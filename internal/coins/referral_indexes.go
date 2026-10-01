// internal/coins/referral_indexes.go
//
// Everything about user_referral that GORM AutoMigrate cannot create.
//
// A separate function from EnsurePostgresIndexes and EnsureEntitlementIndexes
// because the failure modes differ again: the ledger's missing constraints let a
// balance go wrong, the entitlement's let one user hold two of the same thing, and
// this one lets one account be counted twice — which is money. Four separate
// concerns deserve four auditable lists rather than one nobody finishes.
//
// It reuses addConstraint and addCheck from ensure_indexes.go rather than
// restating the DO $$ idiom, for the reason documented there: building that idiom
// by concatenating backtick raw strings next to SQL containing quotes yields an
// unterminated literal.
//
// main.go MUST call this in the same `if !config.IsSQLite` branch as
// coins.EnsurePostgresIndexes. Skipping it yields a table with a plain index
// instead of the unique one, no CHECK on the state machine, and no cap-slot
// ceiling — which looks like a working referral programme and under-pays silently.
// Same class of outage as internal/notification/ensure_indexes.go.
package coins

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// EnsureReferralIndexes creates the CHECK constraints, the partial UNIQUE indexes
// behind the phone and device fraud controls, and the CHECK ceiling on the monthly
// cap-slot table. It is idempotent, a no-op on SQLite, and a no-op when
// user_referral does not exist yet.
//
// It does NOT create two things that ride on GORM struct tags, and both omissions
// are load-bearing:
//
//   - user_referral_referred_uniq, the composite UNIQUE on (referred_kind,
//     referred_user_id). The attribution INSERT's ON CONFLICT names those columns,
//     and 02-architecture.md's notification precedent (42P10, ON CONFLICT without a
//     matching constraint) says what happens when the constraint lives somewhere
//     other than where the query expects it. AutoMigrate has to own it.
//   - referral_cap_slot_referral_uniq, the UNIQUE on referral_id. Same reason, plus
//     a sharper one discovered by running it: a constraint created here over a
//     column AutoMigrate already has a tag for produces TWO objects over one
//     column, and the next AutoMigrate fails reconciling them with 42704
//     "constraint ... does not exist". One owner per constraint.
func EnsureReferralIndexes(db *gorm.DB) error {
	if db == nil || db.Dialector == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(db.Dialector.Name()), "sqlite") {
		return nil
	}
	var exists bool
	if err := db.Raw(`SELECT to_regclass('user_referral') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return nil
	}

	stmts := []string{
		// ── the fraud controls ─────────────────────────────────────────────
		// One successful referral per invited PHONE NUMBER, ever, and per invited
		// DEVICE, ever (05-economy-and-fraud.md §2.4). These are the two controls
		// that prevent the same person being counted repeatedly, which is the
		// actual fraud mechanism; a cap only bounds loss.
		//
		// Partial on `phone_hash IS NOT NULL` because the column is nullable and
		// NULLs compare equal in a btree — a plain UNIQUE over a nullable column
		// admits at most one NULL row, which would make the second signup with no
		// recorded phone fail. The same partial-index reasoning as
		// resource_unlock_live_uniq in unlock_indexes.go.
		//
		// DELIBERATELY NOT scoped to a state. §2.4 says "at most one successful
		// referral per invited phone number, EVER" and the word is ever: a
		// clawed-back row has to keep burning that phone hash or a fraudster
		// simply re-registers after the clawback and farms the same device again.
		// Only qualification populates the hash, so a pending row — the only kind
		// this slice writes — never occupies the slot.
		`CREATE UNIQUE INDEX IF NOT EXISTS user_referral_phone_uniq
		   ON user_referral (phone_hash) WHERE phone_hash IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS user_referral_device_uniq
		   ON user_referral (device_hash) WHERE device_hash IS NOT NULL`,

		// ── CHECK constraints ───────────────────────────────────────────────
		// The state machine is a closed set rather than free text. A typo in a
		// status is not a harmless string here: the qualification pass and the
		// student's own endpoint both filter ON these values, so a row that says
		// 'pening' is a referral that qualifies for nothing and is clawed back for
		// nothing, invisibly.
		//
		// The status CHECK is REPLACED rather than added-if-absent, and that is
		// applied after this list rather than inside it — see replaceCheck. The
		// vocabulary is expected to grow ('expired' arrived with the qualification
		// pass), and addConstraint's IF NOT EXISTS is keyed on the CONSTRAINT NAME
		// rather than on its expression, so a schema that already carries
		// chk_user_referral_status would keep the four-value expression forever and
		// refuse every expiry write. A constraint whose definition can silently
		// disagree with the code that owns it is worse than one brief lock at boot.
		addCheck("chk_user_referral_kind", `user_referral`,
			`referred_kind IN ('user', 'institution', 'provider')`),

		// Self-referral. §10.1 writes CHECK (referrer_user_id <> referred_user_id)
		// and that is right for a student, but for an INSTITUTION the ids come
		// from a different sequence, so `7 <> 7` is comparing two unrelated
		// accounts and would refuse a legitimate attribution for no reason. The
		// rule is only meaningful within one id space, hence the kind guard.
		//
		// The other half of the 3-cycle (A→B→C→A) is stopped by
		// user_referral_referred_uniq, not by this: each account can be referred
		// exactly once, so a ring cannot close.
		addCheck("chk_user_referral_no_self", `user_referral`,
			`referred_kind <> 'user' OR referrer_user_id <> referred_user_id`),

		// A negative award is a sign the caller read the wrong config, and a
		// negative cap slot is the same. Neither is representable.
		addCheck("chk_user_referral_awarded_coins", `user_referral`,
			`awarded_coins >= 0`),
		addCheck("chk_user_referral_cap_pair", `user_referral`,
			`(cap_period IS NULL AND cap_slot IS NULL)
		  OR (cap_period IS NOT NULL AND cap_slot IS NOT NULL)`),

		// ── the monthly cap ────────────────────────────────────────────────
		// THE CAP IS A SCHEMA CONSTRAINT, NOT A GO CHECK. Read
		// referral_cap_model.go for why a SELECT COUNT(*) in Go is wrong here.
		//
		// referral_cap_slot is one row per paid referral: (referrer, YYYY-MM,
		// ordinal). Its PRIMARY KEY — declared as GORM primaryKey tags, so
		// AutoMigrate owns it, because the claim insert's ON CONFLICT names those
		// three columns and an ON CONFLICT without a matching constraint is
		// 42P10 at runtime — makes "one referral per (referrer, month, slot)" a
		// property of the database. The two CHECKs below make the slot a BOUNDED
		// ordinal, so "no more than N successful referrals in a calendar month" is
		// arithmetic the database refuses rather than a comparison a future caller
		// might forget to write.
		addCheck("chk_referral_cap_slot", `referral_cap_slot`,
			`slot >= 1 AND month_key ~ '^[0-9]{4}-[0-9]{2}$' AND coins > 0`),
		// The ceiling is a LITERAL, not coin_economy.referral.monthly_cap, because
		// a CHECK constraint cannot read another table. So referral.monthly_cap may
		// be LOWERED at runtime — claimCapSlot is called with the configured value
		// and refuses an ordinal above it — but RAISING it past this needs the
		// constraint widened. That asymmetry is deliberate and is the whole point
		// of having a schema ceiling behind a config knob: the knob exists so an
		// operator can tighten a cap mid-incident without a deploy, and the
		// ceiling exists so a runaway config edit cannot turn a 10-per-month cap
		// into an unbounded faucet.
		addCheck("chk_referral_cap_slot_ceiling", `referral_cap_slot`,
			referralCapSlotCeilingCheck),
	}

	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	// The status CHECK runs after the list rather than inside it, because it is a
	// comparison followed by a conditional write rather than one statement. See
	// replaceCheck.
	return replaceCheck(db, "chk_user_referral_status", `user_referral`,
		`status IN ('pending', 'qualified', 'expired', 'clawedback', 'rejected')`)
}

// replaceCheck makes a CHECK's definition follow the code that owns it.
//
// addConstraint is idempotent on the constraint NAME, which is right for a
// definition that never changes and wrong for one that does: a CHECK added by an
// earlier build keeps its earlier expression, and every write the new build makes is
// then refused by a constraint whose name is still the one it created. That failure
// surfaces as one INSERT error on the first affected row in production, which is the
// worst place to discover it. 'expired' is exactly that case: a new value in an
// existing closed set, on a table that already carries the constraint.
//
// So: read what the constraint currently says, and only touch it when it differs.
// The alternative — DROP IF EXISTS then ADD on every boot — leaves a window in which
// the constraint does not exist at all, on a rolling deploy where another instance is
// still serving traffic. That window is small, and it is exactly the window in which a
// write the constraint exists to refuse would succeed, so it is not a risk worth
// taking for a comparison that costs one catalogue query per boot.
//
// Cost is one query plus, on a mismatch, one brief ACCESS EXCLUSIVE lock to replace
// the constraint. It runs once per process start, on a table this codebase creates
// from scratch at boot anyway.
func replaceCheck(db *gorm.DB, name, table, expression string) error {
	var existing *string
	if err := db.Raw(
		`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		  WHERE conname = ? AND conrelid = ?::regclass`, name, table,
	).Scan(&existing).Error; err != nil {
		return fmt.Errorf("read constraint %s: %w", name, err)
	}
	if existing != nil && checkLiteralSet(*existing) == checkLiteralSet(expression) {
		return nil
	}
	if err := db.Exec(`ALTER TABLE ` + table + ` DROP CONSTRAINT IF EXISTS ` + name).Error; err != nil {
		return fmt.Errorf("drop constraint %s: %w", name, err)
	}
	return db.Exec(addCheck(name, table, expression)).Error
}

// checkLiteralSet reduces a CHECK body to the ORDERED SET of string literals it
// compares against, so the comparison is about MEANING rather than formatting.
//
// Postgres renders an IN list as `status = ANY (ARRAY['pending'::text,
// 'qualified'::text, ...])`, which shares no characters with the source
// `status IN ('pending', 'qualified', ...)`. Comparing the source text would never
// match and would replace the constraint on every single boot, which is the exact
// failure mode this function exists to avoid.
//
// Comparing the literal sequence is enough for the closed vocabularies this guards:
// every use of replaceCheck is a membership test over string literals, and a change
// to that set is exactly the change that has to be detected. Ordered rather than
// sorted, because pg_get_constraintdef preserves the order the expression listed them
// in, so a reorder is not a change worth taking a lock for.
//
// Deliberately NOT a general CHECK differ. If somebody changes the SHAPE of an
// expression rather than its vocabulary, this will not notice — and the honest fix
// then is to name the constraint differently, so it becomes a new object rather than a
// silent replacement of a live one.
func checkLiteralSet(expression string) string {
	var literals []string
	inQuote := false
	current := strings.Builder{}
	for _, r := range expression {
		switch {
		case r == '\'':
			if inQuote {
				literals = append(literals, current.String())
				current.Reset()
			}
			inQuote = !inQuote
		case inQuote:
			current.WriteRune(r)
		}
	}
	return strings.Join(literals, ",")
}
