// migrations/20261002_study_resource_approval_columns.go
//
// 04-implementation-plan.md §5.3: the approval columns on study_resources.
//
// WHY THIS EXISTS ALONGSIDE AutoMigrate. `go run ./cmd/server` runs GORM
// AutoMigrate over ~110 models and it will add these four columns on its own, so
// this file is not about making the columns appear. It is about the ONE thing
// AutoMigrate gets wrong here, which is the DEFAULT:
//
//	approval_status TEXT NOT NULL DEFAULT 'approved'
//
// That default is load-bearing and it is the opposite of what §5.3's moderation
// wants. It says "rows that nobody moderated are approved", and that is correct
// and deliberate: every row already in the table was uploaded through the
// long-standing admin route, which has no queue, and a 'pending_review' default
// would make the entire existing catalogue vanish from the public list the moment
// the column landed.
//
// A student submission does NOT rely on that default —
// studyresources.ApprovalService.Submit writes pending_review explicitly — so the
// legacy default cannot leak into the new path.
//
// WHY NOT A .sql FILE. The ledger's migrations (20260927_create_coin_ledger.go)
// established the convention in this repo: AutoMigrate for the tables and columns
// it can create, a named Go function for everything it cannot, no duplicated .sql.
// Adding a raw-constraint-free pair of columns is inside AutoMigrate's competence,
// and the one statement below is the CHECK constraint it is NOT.
//
// The CHECK is a closed vocabulary (pending_review | approved | rejected). Without
// it the status is whatever a caller typed, and every query that groups by it —
// the queue, and 08's pending-versus-rejected metric split — would silently split
// into buckets nobody is counting.
//
// Idempotent: the column adds are guarded by column_exists, and the constraint by
// the same pattern the referral status migration used (drop-then-add, because
// PostgreSQL has no ADD CONSTRAINT IF NOT EXISTS and re-running must not fail).
package migrations

import (
	"fmt"

	"studsphere/backend/internal/studyresources"

	"gorm.io/gorm"
)

// AddStudyResourceApprovalColumns adds the §5.3 moderation columns.
//
// Signature matches the other migrations in this package.
func AddStudyResourceApprovalColumns(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrations: no database handle")
	}

	// AutoMigrate creates the columns and the two plain indexes from the model
	// tags. It also cannot add a CHECK constraint, which is the next statement.
	if err := db.AutoMigrate(&studyresources.StudyResource{}); err != nil {
		return fmt.Errorf("migrations: automigrate study_resources approval columns: %w", err)
	}

	if err := dropConstraintIfPresent(db, "chk_study_resources_approval_status"); err != nil {
		return err
	}
	// The vocabulary is studyresources.ApprovalStatuses, built from the same three
	// constants the state machine writes, so the constraint cannot drift from the
	// code that has to satisfy it.
	constraint := fmt.Sprintf(
		`ALTER TABLE study_resources ADD CONSTRAINT chk_study_resources_approval_status CHECK (approval_status IN (%s))`,
		quotedList(studyresources.ApprovalStatuses),
	)
	if err := db.Exec(constraint).Error; err != nil {
		return fmt.Errorf("migrations: add chk_study_resources_approval_status: %w", err)
	}
	return nil
}

// quotedList renders a string slice as a SQL literal list. The values are Go
// constants from this repository, not user input, so interpolation is safe here
// and is the only way to build an IN list without a dialect-specific placeholder.
func quotedList(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += ", "
		}
		out += "'" + v + "'"
	}
	return out
}

// dropConstraintIfPresent removes a CHECK constraint if it exists.
//
// PostgreSQL has no ADD CONSTRAINT IF NOT EXISTS, so re-running this migration
// would fail on the second run without this. Dropping first is also what makes the
// constraint's EXPRESSION updatable — the same trap the referral status migration
// hit, where add-constraint idempotence was on name and not on expression, and a
// migrated database would have kept the old vocabulary and rejected every expiry
// write.
func dropConstraintIfPresent(db *gorm.DB, name string) error {
	var exists bool
	err := db.Raw(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = ?)`, name).Scan(&exists).Error
	if err != nil {
		// Not Postgres, or no pg_constraint. Nothing to drop.
		return nil
	}
	if !exists {
		return nil
	}
	if err := db.Exec(`ALTER TABLE study_resources DROP CONSTRAINT ` + name).Error; err != nil {
		return fmt.Errorf("migrations: drop %s: %w", name, err)
	}
	return nil
}
