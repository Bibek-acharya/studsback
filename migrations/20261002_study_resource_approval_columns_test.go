package migrations

import (
	"testing"

	"studsphere/backend/internal/studyresources"
)

// The migration's CHECK must name exactly the vocabulary the state machine writes.
//
// This is the drift that would bite quietly: the state machine writes
// ApprovalPending/Approved/Rejected from Go constants, the migration builds its
// IN list from the same slice, and if either side changed alone the server would
// start refusing its own moderation decisions with a check violation at runtime.
func TestApprovalStatusConstraintVocabularyMatchesTheModel(t *testing.T) {
	want := "'pending_review', 'approved', 'rejected'"
	got := quotedList(studyresources.ApprovalStatuses)
	if got != want {
		t.Errorf("quotedList(ApprovalStatuses) = %q, want %q", got, want)
	}
	if len(studyresources.ApprovalStatuses) != 3 {
		t.Errorf("%d statuses, want exactly 3 — the constraint is a closed set", len(studyresources.ApprovalStatuses))
	}
}

// A nil handle is a wiring mistake and must be an error rather than a panic, which
// is what every other migration in this package does.
func TestAddStudyResourceApprovalColumnsRefusesNoDatabase(t *testing.T) {
	if err := AddStudyResourceApprovalColumns(nil); err == nil {
		t.Error("a nil handle was accepted")
	}
}
