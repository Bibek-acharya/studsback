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
// constraints, the partial index and the foreign key behind the entitlement
// domain. It is idempotent, a no-op on SQLite, and a no-op when the tables do
// not exist yet.
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
		// One row per (user, resource_type, resource_id), forever, revoked or
		// not. It is deliberately NOT partial on `revoked_at IS NULL`: a revoked
		// unlock still occupies its slot, because a revocation is a reversal
		// rather than a delete (02-architecture.md §5) and "re-buy it" must not
		// be expressible as "insert a second row".
		addConstraint("resource_unlock_uniq", `resource_unlock`,
			`UNIQUE (user_id, resource_type, resource_id)`),

		// One allowance per user. This is what makes EnsureAllowance idempotent
		// at the database rather than by a SELECT-then-INSERT that races.
		addConstraint("uq_user_free_allowance_user", `user_free_allowance`,
			`UNIQUE (user_id)`),

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
