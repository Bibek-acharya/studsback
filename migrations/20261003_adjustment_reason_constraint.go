// migrations/20261003_adjustment_reason_constraint.go
//
// The reason vocabulary for manual coin corrections, enforced by the DATABASE.
//
// 03-api-contract.md §3.2 requires a mandatory `reason_code` on every ADJUST journal.
// The service (coins.Adjust) validates it against a closed set, and this adds the
// CHECK that makes the requirement true for every OTHER writer too.
//
// WHY THE SERVICE CHECK IS NOT ENOUGH. coins.Adjust is currently the only thing that
// writes an ADJUST journal, so its validation covers today's world. The constraint
// covers the next one: a future bulk-correction script, a backfill, an admin
// maintenance path. Those are exactly the paths that write raw rows, and exactly the
// ones where an invented reason string would enter the health metrics as a bucket
// nobody is watching — 05 §5 groups by reason_code, so "SUPPORT_FIX" and "TYPO" would
// both silently join "the operator was in a hurry".
//
// WHY DROP-AND-RE-ADD rather than ADD IF NOT EXISTS. PostgreSQL has no
// `ADD CONSTRAINT IF NOT EXISTS`, so a plain add fails on the second run of this
// migration. Dropping first is also what makes the EXPRESSION updatable, and that is
// the same trap the referral status migration hit: idempotence on the constraint NAME
// is not idempotence on its vocabulary, so a future added reason would be silently
// ignored on any database that already had the constraint.
//
// The drop-and-re-add leaves the constraint absent for the microseconds between the
// two statements. That is safe here and worth saying why: an INSERT that lands in
// that window is validated against nothing and succeeds with any reason, and the
// reconciler does not check reason vocabulary. So this runs at boot before the
// listener starts, not concurrently with traffic — which is true of every migration
// in this package.

package migrations

import (
	"fmt"

	"studsphere/backend/internal/coins"

	"gorm.io/gorm"
)

// AddAdjustmentReasonConstraint installs chk_coin_journal_adjustment_reason.
//
// Idempotent: the drop is guarded by a pg_constraint lookup and the add is the only
// statement that can fail, so re-running against an up-to-date database is a no-op.
func AddAdjustmentReasonConstraint(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrations: no database handle")
	}

	// Guard: the journal table must exist. A migration that assumes a table it did not
	// create should say so rather than fail with a PostgreSQL error about a missing
	// relation.
	var exists bool
	if err := db.Raw(`SELECT to_regclass('coin_journal') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return fmt.Errorf("migrations: check coin_journal exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("migrations: coin_journal does not exist; run the ledger migration first")
	}

	if err := dropConstraintIfPresent(db, "chk_coin_journal_adjustment_reason"); err != nil {
		return err
	}
	// The list comes from the package that validates, not from a copy here, so the
	// constraint and the service cannot be written from two recollections.
	constraint := fmt.Sprintf(`
		ALTER TABLE coin_journal ADD CONSTRAINT chk_coin_journal_adjustment_reason
		CHECK (
			entry_type <> 'ADJUST'
			OR reason_code IN (%s)
		)`, coins.AdjustmentReasonsSQLList())
	if err := db.Exec(constraint).Error; err != nil {
		return fmt.Errorf("migrations: add chk_coin_journal_adjustment_reason: %w", err)
	}
	return nil
}
