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
		// status is not a harmless string here: the next slice's qualification and
		// clawback sweeps filter ON these values, so a row that says 'pening' is a
		// referral that qualifies for nothing and is clawed back for nothing,
		// invisibly.
		addCheck("chk_user_referral_status", `user_referral`,
			`status IN ('pending', 'qualified', 'clawedback', 'rejected')`),
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
	return nil
}
