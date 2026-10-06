// internal/coins/unlock_indexes.go
//
// Everything about the entitlement schema that GORM AutoMigrate cannot create.
//
// It is a separate function from EnsurePostgresIndexes on purpose. The ledger
// and the entitlements have genuinely different failure modes: the ledger's
// missing constraints would let a balance go wrong, while the entitlement's
// would let one user hold two of the same thing. Keeping them in two functions
// means a review of "what does AutoMigrate leave unenforced" reads two short
// lists rather than one that nobody finishes.
//
// It reuses addConstraint and addCheck from ensure_indexes.go rather than
// restating the DO $$ idiom, because that idiom has a specific trap documented
// there (building it by concatenating backtick raw strings next to SQL
// containing quotes and ::regclass yields an unterminated literal) and a second
// copy of it is a second chance at that bug.
//
// main.go MUST call this in the same `if !config.IsSQLite` branch as
// coins.EnsurePostgresIndexes. Skipping it yields two tables that look right and
// enforce nothing at all — no uniqueness, no source/journal agreement, and
// without the foreign key a COINS unlock may name a journal that never existed.
// That is the same class of outage as internal/notification/ensure_indexes.go,
// where a fresh boot produced a server whose every preferences PUT failed.
package coins

import (
	"strings"

	"gorm.io/gorm"
)

// EnsureEntitlementIndexes creates the CHECK constraints, the UNIQUE
// constraints, the partial UNIQUE index behind the fraud control, the
// non-unique index behind the revocation-history read, and the foreign key
// behind the entitlement domain. It is idempotent, a no-op on SQLite, and a
// no-op when the tables do not exist yet.
//
// It also retires the shape the fraud control used to have: the full UNIQUE
// constraint named resource_unlock_uniq, which covered revoked rows as well as
// live ones and so made a revocation permanent. See the statements below for
// why that was wrong and what replaces it.
//
// It is not optional, and that is the whole reason this file exists. See
// TestEnsureIndexesIsNoOpOnSQLite and the ledger's equivalent.
func EnsureEntitlementIndexes(db *gorm.DB) error {
	if db == nil || db.Dialector == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(db.Dialector.Name()), "sqlite") {
		return nil
	}
	var exists bool
	if err := db.Raw(`SELECT to_regclass('resource_unlock') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return nil
	}

	stmts := []string{
		// ── the fraud control ─────────────────────────────────────────────
		// ONE LIVE UNLOCK per (user, resource_type, resource_id). The word doing
		// the work is LIVE, and that is a deliberate change from the shape this
		// index had before: it used to be a full UNIQUE constraint covering every
		// row, revoked ones included, so a revocation permanently consumed the
		// slot and the student could never unlock that resource again.
		//
		// The fraud cap this exists to provide is "one live unlock per resource",
		// and only live rows are access. A revoked row grants nothing, so it must
		// not occupy the cap: it is a record of a decision that has already been
		// undone, and counting it would let an admin's mistake — revoking a
		// student's unlock for something that was not fraud — permanently strip a
		// purchase with no path back. Re-buying costs the buyer a second time, so
		// the cap is not weakened by letting them: two live rows still cannot
		// exist, and the second payment is still visible as a second journal.
		//
		// The history survives, which is the other half of the point. Revocation
		// is a reversal rather than a delete (02-architecture.md §5) and a DELETE
		// or an overwrite would answer neither "was this ever granted" nor "when
		// and why was it taken away". The new row is a SECOND row, not a
		// replacement, so the revoked one is left exactly as the record.
		//
		// Ordering: the new index is built BEFORE the old object is dropped, so
		// there is never a moment at which the table enforces nothing. If this
		// statement fails the old constraint is still in place and the boot still
		// fails loudly, which is the right outcome — see the caveat below.
		`CREATE UNIQUE INDEX IF NOT EXISTS resource_unlock_live_uniq
		   ON resource_unlock (user_id, resource_type, resource_id)
		  WHERE revoked_at IS NULL`,

		// ── retiring the old object ───────────────────────────────────────
		// Two statements, both no-op-safe on a schema that never had it, and both
		// needed because the two spellings are not interchangeable: the old
		// object was a CONSTRAINT, and a constraint owns an index that a bare
		// DROP INDEX refuses to touch ("cannot drop index ... because constraint
		// ... on table ... requires it"). Conversely, on a schema where it was
		// created as a plain index, ALTER TABLE ... DROP CONSTRAINT is a no-op
		// and this DROP INDEX is the only thing that removes it. Whichever
		// spelling is absent, the other one does the work.
		//
		// Both use IF EXISTS rather than the DO $$ shape addConstraint uses,
		// deliberately: addConstraint's idiom exists to avoid adding a constraint
		// twice, and these are REMOVING. There is nothing to be idempotent about
		// beyond the missing-object case, which IF EXISTS covers in one line.
		//
		// ── the honest caveat, and why this stays a loud failure ────────────
		//
		// ResourceUnlock carries no `uniqueIndex` struct tag, so a schema built by
		// AutoMigrate alone — main.go's !config.IsSQLite block skipped, or a
		// deployment where EnsureEntitlementIndexes has never run — has NO
		// uniqueness on these columns at all, not even the old one. Such a schema
		// can hold duplicates of any kind, including two live rows for one triple.
		//
		// CREATE UNIQUE INDEX above therefore fails on one with 23505 and the
		// key values in the message, at every boot, naming the operator exactly
		// what they have to look at. That is the correct outcome and it is left
		// that way on purpose. Adding repair logic to make it start cleanly would
		// mean choosing which of the two rows survives, and that choice destroys
		// the revocation history the partial index exists to preserve — trading a
		// corruption an operator can see for a silent merge they cannot. No
		// repair is written here, deliberately.
		//
		// Nothing needs cleaning up before the build either. A revoked-plus-live
		// pair cannot pre-exist: the old full index spanned revoked rows, so the
		// second of the two was already refused at insert time.
		`ALTER TABLE resource_unlock DROP CONSTRAINT IF EXISTS resource_unlock_uniq`,
		`DROP INDEX IF EXISTS resource_unlock_uniq`,

		// ── the revocation-history read ───────────────────────────────────
		// Serves "this user's unlocks, live first, revoked alongside" — the one
		// read a partial index cannot answer. resource_unlock_allowance_open_idx
		// is itself partial on `revoked_at IS NULL` and so physically cannot
		// return a revoked row at all, and resource_unlock_live_uniq is partial
		// for the same reason. Without this a revocation history is a sequential
		// scan of the whole table, which is exactly the query an admin opens
		// during a fraud review and exactly the one that gets slower as the
		// table grows.
		//
		// Plain and non-unique on purpose: uniqueness here would be the old
		// constraint under a new name. user_id leads because it is the only
		// selective column, and revoked_at follows so a per-user ordering by it
		// is index-ordered rather than a sort.
		`CREATE INDEX IF NOT EXISTS resource_unlock_user_revoked_idx
		   ON resource_unlock (user_id, revoked_at)`,

		// One allowance per user. This is what makes EnsureAllowance idempotent
		// at the database rather than by a SELECT-then-INSERT that races.
		//
		// NOT created here any more: it is declared as `uniqueIndex` on
		// UserFreeAllowance.UserID, so AutoMigrate owns it. The addConstraint that used
		// to be here made a SECOND unique object that AutoMigrate could not reconcile,
		// and a hand-named unique constraint on an AutoMigrate-owned table is exactly
		// what turned every boot after the first into a Fatal. See the model's comment.

		// ── CHECK constraints ──────────────────────────────────────────────
		// resource_type is a class, not free text. A typo here is not a harmless
		// string: it creates an unlock that no gate will ever match, so the
		// student pays and gets nothing back, and the row is invisible to every
		// query that looks for a real class. The three values restate
		// ResourceTypes; the values list is what 03-api-contract.md §2.3 sends.
		addCheck("chk_resource_unlock_type", `resource_unlock`,
			`resource_type IN ('study_resource', 'video', 'mock_test')`),
		addCheck("chk_resource_unlock_source", `resource_unlock`,
			`source IN ('COINS', 'ALLOWANCE')`),

		// The funding triple, as one constraint.
		//
		// COINS must name the journal that settled it and may have paid any
		// non-negative amount. ALLOWANCE must name no journal — no coins moved,
		// so there is nothing to point at — and must have paid nothing. These
		// are the two ways an unlock can lie about whether money changed hands,
		// and both are unrecoverable later: there is no journal to reconstruct
		// from, and no way to charge the student retroactively.
		addCheck("chk_resource_unlock_source_funding", `resource_unlock`, `
			(source = 'COINS'     AND journal_id IS NOT NULL AND coins_paid >= 0)
		 OR (source = 'ALLOWANCE' AND journal_id IS NULL     AND coins_paid = 0)`),
		// Stated on its own as well, because 02-architecture.md §5 declares
		// CHECK (coins_paid >= 0) and a reader checking the schema against the
		// spec should find it under that name rather than have to notice it is
		// implied by the line above.
		addCheck("chk_resource_unlock_coins_paid", `resource_unlock`, `coins_paid >= 0`),

		// A revocation is a pair or it is nothing. revoked_at without a reason
		// is a clawback nobody can explain; a reason without a timestamp cannot
		// be ordered against anything.
		addCheck("chk_resource_unlock_revocation", `resource_unlock`, `
			(revoked_at IS NULL     AND revoke_reason IS NULL)
		 OR (revoked_at IS NOT NULL AND revoke_reason IS NOT NULL)`),

		// ── the derived `used` count ──────────────────────────────────────
		// This index exists for exactly one query: the count of a user's
		// un-revoked allowance unlocks per class, which is how "used" is derived
		// rather than stored. It is partial on `revoked_at IS NULL` so that
		// revoking a document does not stop the count from being an index scan,
		// and it leads with user_id because that is the only selective column.
		`CREATE INDEX IF NOT EXISTS resource_unlock_allowance_open_idx
		   ON resource_unlock (user_id, source, resource_type) WHERE revoked_at IS NULL`,

		// ── foreign keys ──────────────────────────────────────────────────
		// ON DELETE RESTRICT is the default and is stated explicitly, matching
		// the ledger's four foreign keys. A journal cannot be deleted — the
		// append-only trigger blocks it, and ON DELETE RESTRICT is the second of
		// two independent guards rather than the only one.
		//
		// resource_unlock.user_id deliberately has NO foreign key to users. Users
		// live in another module, and the coin tables do not FK to them either
		// (coin_account.owner_user_id and coin_lot are keyed only by the user id),
		// because a coin table that cannot be created until the users module's
		// table exists couples two deployments that have no other coupling, and
		// a user-row delete would then cascade into a ledger that is supposed to
		// be the permanent record. The precedent is the rule; this comment is
		// here so nobody "fixes" the omission later.
		addConstraint("fk_resource_unlock_journal", `resource_unlock`,
			`FOREIGN KEY (journal_id) REFERENCES coin_journal(id) ON DELETE RESTRICT`),
	}

	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
