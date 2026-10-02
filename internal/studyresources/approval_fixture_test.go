// internal/studyresources/approval_fixture_test.go
//
// The harness approval_test.go drives. Kept apart so that file reads as the table
// of properties and this reads as the fixtures.
//
// The database is SQLITE on purpose: the state machine under test is ordinary
// column writes, and the money half of this slice is exercised in internal/coins
// against Postgres by the grant's own idempotency key. What is pinned HERE is
// that approving calls the grant at all, once, with the uploader's id — which is
// the seam that a "publish without paying" regression would break.

package studyresources

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// countingGrant records what it was asked to pay, so a test can assert both the
// COUNT (was it paid twice?) and the ARGS (was the right student paid?).
type countingGrant struct {
	calls      int
	userID     uint
	resourceID uint
	title      string
	err        error
}

func (g *countingGrant) GrantResourceApproved(_ context.Context, userID, resourceID uint, title string) error {
	g.calls++
	g.userID, g.resourceID, g.title = userID, resourceID, title
	return g.err
}

func submittedUpload() StudyResource {
	return StudyResource{
		Title:        "Organic Chemistry Notes",
		Course:       "BSc Chemistry",
		Year:         "2024",
		ResourceType: TypeSyllabus,
		FileName:     "chem.pdf",
		FilePath:     "study-resources/chem.pdf",
		FileURL:      "/uploads/study-resources/chem.pdf",
		FileSize:     1024,
		MimeType:     "application/pdf",
	}
}

// approvalFixture returns BOTH objects, because the properties under test span
// both: the state machine writes, and Service is what the public read path uses.
//
// Handing back one type for both would either mean folding the approval port into
// Service (which approval.go argues against) or re-testing the public reads through
// a type that does not serve them.
func approvalFixture(t *testing.T, grant ApprovalGrant) (*Service, *ApprovalService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewRepository(db)
	return NewService(repo), newApprovalService(repo, grant)
}
