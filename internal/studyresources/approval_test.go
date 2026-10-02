// internal/studyresources/approval_test.go
//
// 04 §5.3: upload and approval.
//
// The load-bearing property is not "there is an approve endpoint". It is
//
//	coins move on PUBLICATION, never on upload
//
// and that has two independent reasons, both of which are enforced below:
//
//  1. paying before moderation creates an incentive to upload material the
//     platform cannot clear, and
//  2. it puts copyright exposure on the platform for content nobody has reviewed
//     (07-compliance-nepal.md).
//
// So a pending resource is NOT published and pays NOTHING, and the grant is
// inseparable from the publish: one transaction, because a resource that is
// published without its uploader paid is a support ticket, and one that is paid
// but not published is a student with coins for material they cannot open.
package studyresources

import (
	"errors"
	"testing"
)

// The properties under test are named in the comments on each function; the
// vocabulary, the port and the state machine itself live in approval.go, which is
// where a reader looks for them.

func TestANewUploadIsPendingUnpublishedAndPaysNothing(t *testing.T) {
	grant := &countingGrant{}
	_, svc := approvalFixture(t, grant)

	student, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if student.ApprovalStatus != ApprovalPending {
		t.Errorf("a fresh upload is %q, want %q", student.ApprovalStatus, ApprovalPending)
	}
	// THE property. Not published.
	if student.IsPublished {
		t.Error("a pending upload is published; the whole earn-on-publication rule is bypassed")
	}
	// And not paid, at any point.
	if grant.calls != 0 {
		t.Errorf("an upload paid %d times; coins move on publication only", grant.calls)
	}
}

func TestApprovingPublishesAndPaysInOneStep(t *testing.T) {
	grant := &countingGrant{}
	_, svc := approvalFixture(t, grant)

	pending, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	approved, err := svc.Approve(pending.ID, 7)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}

	if !approved.IsPublished {
		t.Error("an approved resource is not published")
	}
	if approved.ApprovalStatus != ApprovalApproved {
		t.Errorf("status = %q, want %q", approved.ApprovalStatus, ApprovalApproved)
	}
	// The reviewer is recorded, and it is the REVIEWER — the admin, not the
	// uploader. 04 §5.3 names reviewer identity as a requirement precisely so
	// "who approved this" is answerable later.
	if approved.ReviewedBy == nil || *approved.ReviewedBy != 7 {
		t.Errorf("reviewed_by = %v, want 7 (the reviewer, not the uploader)", approved.ReviewedBy)
	}
	if approved.ReviewedAt == nil {
		t.Error("reviewed_at was not recorded")
	}
	if grant.calls != 1 {
		t.Fatalf("approval paid %d times, want exactly 1", grant.calls)
	}
	if grant.userID != 42 || grant.resourceID != pending.ID {
		t.Errorf("paid user %d for resource %d, want user 42 resource %d", grant.userID, grant.resourceID, pending.ID)
	}
}

// Approving twice must not pay twice. Two independent defences, and this pins
// the first: the state machine refuses the second decision outright.
func TestApprovingTwiceRefusesTheSecondAndPaysOnce(t *testing.T) {
	grant := &countingGrant{}
	_, svc := approvalFixture(t, grant)

	pending, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Approve(pending.ID, 7); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	if _, err := svc.Approve(pending.ID, 7); !errors.Is(err, ErrApprovalNotPending) {
		t.Errorf("second approve got %v, want ErrApprovalNotPending", err)
	}
	if grant.calls != 1 {
		t.Errorf("a double approve paid %d times, want 1", grant.calls)
	}
}

// A grant that is wired to nothing must NOT fall back to publishing unpaid. This
// is the failure mode that would be invisible in testing and obvious to a
// student: an admin approves, the resource goes live, and nobody is ever paid.
func TestApprovalRefusesWhenTheGrantIsNotWired(t *testing.T) {
	_, svc := approvalFixture(t, nil)

	pending, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err = svc.Approve(pending.ID, 7)
	if !errors.Is(err, ErrApprovalUnconfigured) {
		t.Fatalf("approve with no grant got %v, want ErrApprovalUnconfigured", err)
	}
	// And nothing moved: not published, still pending, not paid.
	reloaded := svc.mustLoad(pending.ID)
	if reloaded.IsPublished || reloaded.ApprovalStatus != ApprovalPending {
		t.Errorf("a refused approval still published the resource: published=%v status=%q",
			reloaded.IsPublished, reloaded.ApprovalStatus)
	}
}

// Rejection is the mirror image: a reason, never published, never paid.
func TestRejectionNeedsAReasonAndPaysNothing(t *testing.T) {
	grant := &countingGrant{}
	_, svc := approvalFixture(t, grant)
	pending, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := svc.Reject(pending.ID, 7, ""); !errors.Is(err, ErrRejectReasonRequired) {
		t.Errorf("a rejection with no reason got %v, want ErrRejectReasonRequired", err)
	}
	if _, err := svc.Reject(pending.ID, 7, "   "); !errors.Is(err, ErrRejectReasonRequired) {
		t.Errorf("a whitespace-only reason got %v, want ErrRejectReasonRequired", err)
	}

	rejected, err := svc.Reject(pending.ID, 7, "Someone else's material. Please upload your own notes.")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.IsPublished {
		t.Error("a rejected resource is published")
	}
	if rejected.ApprovalStatus != ApprovalRejected {
		t.Errorf("status = %q, want %q", rejected.ApprovalStatus, ApprovalRejected)
	}
	if rejected.RejectReason == "" {
		t.Error("the reject reason was not stored; the student is told nothing about why")
	}
	if grant.calls != 0 {
		t.Errorf("a rejection paid %d times; it must never pay", grant.calls)
	}
}

// A rejected resource must not be resurrectable by approving it. There is no
// idempotency key protecting this and no other layer that would catch it.
func TestARejectedResourceCannotThenBeApproved(t *testing.T) {
	grant := &countingGrant{}
	_, svc := approvalFixture(t, grant)
	pending, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := svc.Reject(pending.ID, 7, "Not yours."); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := svc.Approve(pending.ID, 7); !errors.Is(err, ErrApprovalNotPending) {
		t.Errorf("approving a rejected resource got %v, want ErrApprovalNotPending", err)
	}
	if grant.calls != 0 {
		t.Errorf("a rejected resource paid %d times", grant.calls)
	}
}

// A pending resource must not be reachable on the public list, by id, or as a
// filter facet. The status column is new; every read that filtered on
// is_published alone would now leak pending rows.
func TestAPendingResourceIsInvisibleEverywherePublic(t *testing.T) {
	reads, svc := approvalFixture(t, &countingGrant{})
	pending, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := reads.GetPublishedResource(pending.ID); err == nil {
		t.Error("a pending resource is readable through the published-by-id lookup")
	}
	rows, _, err := reads.GetResources(ResourceFilters{PublishedOnly: true}, 1, 20)
	if err != nil {
		t.Fatalf("public list: %v", err)
	}
	for _, r := range rows {
		if r.ID == pending.ID {
			t.Error("a pending resource appeared on the public list")
		}
	}
	years, courses, err := reads.DistinctFacets()
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	for _, year := range years {
		if year == pending.Year {
			t.Errorf("a pending resource's year %q leaked into the public facets", year)
		}
	}
	for _, course := range courses {
		if course == pending.Course {
			t.Errorf("a pending resource's course %q leaked into the public facets", course)
		}
	}
}

// The uploader sees their own pending resource. Otherwise "awaiting review" is
// unanswerable and support gets the ticket.
func TestTheUploaderCanSeeTheirOwnPendingResource(t *testing.T) {
	reads, svc := approvalFixture(t, &countingGrant{})
	pending, err := svc.Submit(42, submittedUpload())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	rows, total, err := reads.GetResources(ResourceFilters{UploaderID: 42}, 1, 20)
	if err != nil {
		t.Fatalf("own uploads: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != pending.ID {
		t.Fatalf("the uploader sees %d rows (total %d), want their own 1", len(rows), total)
	}
	// And the admin queue sees it, which is what makes moderation possible.
	queue, queueTotal, err := reads.GetResources(ResourceFilters{ApprovalStatus: ApprovalPending}, 1, 20)
	if err != nil {
		t.Fatalf("pending queue: %v", err)
	}
	if queueTotal != 1 || len(queue) != 1 || queue[0].ID != pending.ID {
		t.Fatalf("the moderation queue holds %d rows (total %d), want 1", len(queue), queueTotal)
	}
}
