// migrations/20261001_create_user_referral_attribution.go
package migrations

import (
	"time"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/coins"
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// CreateUserReferralAttribution brings the referral attribution schema up to date
// on the SQL-migration path, and backfills referral codes for existing users.
//
// Three steps, in this order, and the order is load-bearing:
//
//  1. AutoMigrate user_referral and referral_cap_slot. Both are new tables, so
//     there is nothing to conflict with.
//  2. AutoMigrate auth.User for the new referral_code column.
//  3. EnsureReferralIndexes.
//
// The backfill comes LAST and is separate from the schema work, deliberately. A
// migration that creates the column and fills it in one statement holds a lock on
// the users table for as long as the fill takes, and there is no SQL that fills it
// in one statement anyway — the codes come from crypto/rand in Go, because a
// timestamp or a counter would be guessable and enumerable. See
// internal/auth/referral_code.go for why generation is in Go and not in a PL/pgSQL
// function.
//
// It is also idempotent, so an operator can re-run it after a partial failure and
// it will resume rather than duplicate: the write is guarded on
// `referral_code IS NULL`, so a second pass finds nothing to do and changes no
// code a student has already shared.
//
// TWO MIGRATION NOTES worth writing down rather than discovering.
//
// First, the column is NULLABLE. A NOT NULL column with no default cannot be added
// to a table that has rows without a three-step add-with-default / backfill / set
// not null dance, and a column that is NOT NULL from the moment it exists would
// break every signup between deploy and backfill. Nullable plus a batched backfill
// means the system is correct at every point in between: a user with a NULL code
// simply has not been backfilled yet, and the only consequence is that they cannot
// yet share an invite link.
//
// Second, this migration does NOT create the UNIQUE index on users.referral_code by
// hand — AutoMigrate does it from the model's uniqueIndex tag. That is the point
// of putting it there: EnsureReferralIndexes owns the partial and CHECK constraints
// it can express, and the unique index the attribution query's ON CONFLICT depends
// on rides on the model so AutoMigrate creates it at the same moment as the column.
// A hand-written CREATE UNIQUE INDEX here would be a second statement to keep in
// step with the model, which is the drift the ledger's own migration file warns
// about.
func CreateUserReferralAttribution(db *gorm.DB) error {
	if err := db.AutoMigrate(coins.UserReferralModels...); err != nil {
		return err
	}
	if err := db.AutoMigrate(coins.ReferralCapSlotModels...); err != nil {
		return err
	}
	// The users.referral_code column. Migrated through the auth package rather
	// than with raw SQL so the column type, length and unique index all come from
	// the one place that defines them (internal/auth/model.go).
	if err := db.AutoMigrate(&auth.User{}); err != nil {
		return err
	}
	if err := coins.EnsureReferralIndexes(db); err != nil {
		return err
	}

	start := time.Now()
	wrote, err := auth.BackfillReferralCodes(db)
	if err != nil {
		// A partial backfill is NOT fatal and this is the interesting decision in the
		// file. The schema is correct either way — every new signup gets a code from
		// Repository.CreateUser — and a failed backfill means some EXISTING users
		// cannot yet share an invite link, which is a support ticket rather than an
		// outage. Failing the boot would take the whole server down to fix something
		// that is not down, and this migration runs on every boot.
		//
		// It is re-runnable and idempotent, so the recovery is to run it again. What
		// is not acceptable is being silent, hence the count alongside the error:
		// "failed after 40,000 of 90,000" is an actionable log line and "failed" is
		// not.
		logger.Warn("Referral code backfill did not complete; re-run migrations.CreateUserReferralAttribution to resume",
			"error", err, "written", wrote, "elapsed", time.Since(start).String())
		return nil
	}
	logger.Info("Referral code backfill completed", "written", wrote, "elapsed", time.Since(start).String())
	return nil
}
