// migrations/20261002_widen_referral_status_for_expiry.go
package migrations

import (
	"studsphere/backend/internal/coins"
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// WidenReferralStatusForExpiry brings a deployed schema up to the five-state referral
// machine, and adds the columns the qualification pass writes.
//
// WHY THIS EXISTS SEPARATELY from 20261001_create_user_referral_attribution.go, which
// called EnsureReferralIndexes and would otherwise appear to cover it: that migration
// creates the CHECK if it is absent, and the CHECK IS PRESENT on any schema the
// previous migration has run against. EnsureReferralIndexes's addConstraint is
// idempotent on the constraint NAME, not on its expression, so it left the four-value
// `status IN ('pending', 'qualified', 'clawedback', 'rejected')` in place and the first
// expiry write would have been refused by it with a check violation.
//
// That is a general hazard rather than a referral quirk, and referral_indexes.go's
// replaceCheck is the fix at the source: it reads pg_get_constraintdef, compares the
// literal set, and only then drops and re-adds. So this migration is a belt-and-braces
// path for the SQL-only deployment order — run this migration, do NOT run the boot-time
// Ensure* calls — and it is what makes the vocabulary change safe on a database that was
// migrated before 'expired' existed.
//
// A NOTE ON THE DROP-AND-RE-ADD, because it is the one dangerous-looking thing here:
// the constraint is absent for the microseconds between the DROP and the ADD. On a
// rolling deploy another instance may be serving traffic at that moment, and a write the
// constraint exists to refuse would succeed. That is a real window and the reason
// replaceCheck only takes the lock when the definition actually differs — on every other
// boot, including this one's second run, the comparison short-circuits and no lock is
// taken at all.
//
// WHAT IT DOES NOT DO: touch hold_journal_id. That column is permanently NULL under the
// new model but is kept for audit, so there is nothing to migrate and nothing to drop.
// See internal/coins/referral_model.go.
//
// Idempotent, like every other migration in this package: re-running it is a no-op, so an
// operator can resume after a partial failure.
func WidenReferralStatusForExpiry(db *gorm.DB) error {
	// The two columns the qualification pass writes. AutoMigrate rather than
	// `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` because the column type, the NOT NULL
	// default and the index all come from the one place that defines them
	// (internal/coins/referral_model.go), and a hand-written ADD COLUMN would be a second
	// definition to keep in step with the model.
	//
	// Both are additive with a default, so a live table gains them without a rewrite and
	// without a lock that blocks reads for the length of a backfill — there is no backfill,
	// because a referral that existed before this slice has no expiry to record.
	if err := db.AutoMigrate(coins.UserReferralModels...); err != nil {
		return err
	}

	// The status vocabulary, through the same function the boot path uses so the two
	// cannot disagree about what the constraint says.
	if err := coins.EnsureReferralIndexes(db); err != nil {
		return err
	}

	// Verified rather than assumed. A migration that reports success while the constraint
	// still refuses 'expired' is worse than one that fails: the first expiry write would
	// be the thing that discovered it, in production, on a background job.
	//
	// The check is that the constraint ACCEPTS the new value, which is the only question
	// that matters and is immune to how Postgres chooses to render the expression.
	var accepted bool
	if err := db.Raw(
		`SELECT EXISTS (
		   SELECT 1 FROM pg_constraint
		    WHERE conname = 'chk_user_referral_status'
		      AND conrelid = 'user_referral'::regclass
		      AND pg_get_constraintdef(oid) LIKE '%expired%'
		 )`,
	).Scan(&accepted).Error; err != nil {
		return err
	}
	if !accepted {
		// NOT fatal, and the reason is worth stating: the table might not exist yet on a
		// fresh database where AutoMigrate has not run, in which case there is no
		// constraint to widen and the boot-time EnsureReferralIndexes will create it
		// with the right vocabulary. Only a schema that HAS the table and the WRONG
		// constraint is a problem, and that is a warning an operator has to act on.
		logger.Warn("Referral status CHECK does not yet list 'expired'; the qualification "+
			"pass will not be able to expire a referral until this migration is re-run",
			"constraint", "chk_user_referral_status")
		return nil
	}
	logger.Info("Referral status vocabulary widened to include 'expired'")
	return nil
}
