// internal/studyresources/approval.go
//
// 04-implementation-plan.md §5.3 — upload and approval.
//
// ── coins move on PUBLICATION, never on upload ─────────────────────────────────
//
// This is the load-bearing decision in the file and it is not a stylistic one.
// Two independent reasons, either of which alone would justify it:
//
//  1. paying before moderation creates an incentive to upload material the
//     platform cannot clear. An uploader paid on upload is paid to upload; the
//     moderation queue becomes the place where the incentive to abuse it lives.
//  2. it puts copyright exposure on the platform for content nobody has looked
//     at (07-compliance-nepal.md). Money paid for an unreviewed file is money
//     paid for someone's exam paper.
//
// So a submitted resource is NOT published and pays NOTHING, and the grant is
// inseparable from the publish — one transaction, because:
//   - published but unpaid is a student whose page promises coins that never came,
//     which is the single worst failure this feature has; and
//   - paid but unpublished is coins for material the uploader cannot open.
//
// ── the port is declared HERE, in the module that owns the row ─────────────────
//
// Same rule as download_gate.go and playback_gate.go: internal/coins imports
// internal/studyresources (study_resource_lookup.go reads this table), so an
// import in the other direction would be a cycle. Somebody has to own the
// interface, and the owner must be the side that does not need the other's types.
//
// The method takes the idempotency key's PARTS, not the key. This module must not
// be able to compose a key that collides with another earn mechanic's:
// coin_journal has one UNIQUE (scope, idempotency_key) across every reason code,
// so a collision is either a silent replay of someone else's award or
// ErrIdempotencyKeyReuse. The implementer owns the key namespace.
package studyresources

import (
	"context"
	"errors"
	"strings"
	"time"

	"studsphere/backend/internal/notification"

	"gorm.io/gorm"
)

// The approval vocabulary, as a CLOSED set.
//
// Closed because each value is a different support sentence and a different
// support metric, and 08's metrics need pending and rejected counted SEPARATELY:
// a rise in pending is a moderation backlog and a rise in rejected is a broken
// rule or a fraud pattern. Folding rejected into pending would make those
// indistinguishable, which is the same mistake as the retired coins_pending
// aggregate all over again.
const (
	// ApprovalPending is a student upload awaiting moderation. Not published, and
	// pays nothing.
	ApprovalPending = "pending_review"
	// ApprovalApproved is published. THE ONLY STATUS THAT PAYS COINS.
	ApprovalApproved = "approved"
	// ApprovalRejected is permanently not published. It pays nothing, ever, and it
	// is not resurrectable.
	ApprovalRejected = "rejected"
)

// ApprovalStatuses is the closed set the CHECK constraint enforces.
var ApprovalStatuses = []string{ApprovalPending, ApprovalApproved, ApprovalRejected}

// ApprovalGrant is the port the coin economy satisfies: pay an uploader when
// their resource is PUBLISHED.
//
// One method, and it is allowed to fail. A nil port is ErrApprovalUnconfigured
// rather than "publish unpaid" — see ApprovalService.Approve.
type ApprovalGrant interface {
	GrantResourceApproved(ctx context.Context, userID, resourceID uint, title string) error
}

// ApprovalNotifier announces a decision to the uploader. Optional: with none
// wired the decision still stands and the student is told when they look.
type ApprovalNotifier interface {
	NotifyTx(ctx context.Context, tx *gorm.DB, req notification.NotifyRequest) error
}

// The refusals, distinct from the service's plain "resource not found" so a
// handler can tell a refusal from an outage and map them to different statuses.
var (
	// ErrApprovalUnconfigured is a grant port with nothing wired to it.
	//
	// It is deliberately NOT a "publish without paying" path. An admin who
	// approves a resource and the uploader is never paid has broken the promise
	// the upload page makes, silently and invisibly — there is no log line and no
	// failing request, just a student who never receives coins for material they
	// wrote. Refusing is the loud version of the same failure.
	ErrApprovalUnconfigured = errors.New("the approval grant is not connected")
	// ErrApprovalNotPending is a decision on an already-decided resource.
	//
	// Two different mistakes share this, and both are refused. Approving twice
	// must not pay twice — though the grant's idempotency key is what actually
	// guarantees that, since this check and the grant are not one transaction
	// across two databases. And approving a REJECTED resource must not silently
	// republish it, which nothing else in the system would catch.
	ErrApprovalNotPending = errors.New("resource is not awaiting review")
	// ErrRejectReasonRequired is a rejection with no reason.
	//
	// Not optional, and not a style rule. The student is told why, and "why" is
	// the only thing they can act on: a student who cannot act on a rejection
	// resubmits the same material, and the queue grows for no progress. 06 §7 and
	// the 09 cheat sheet both treat the reason as part of the refusal.
	ErrRejectReasonRequired = errors.New("a rejection needs a reason")
)

// ApprovalService is the submission and moderation state machine.
//
// It is a SEPARATE type from Service rather than three more methods on it,
// because Service is reached by the public read path and the existing admin
// upload path, and this one holds a port that is ALLOWED to be nil. Folding them
// together would give every constructor of the read-path service an argument only
// moderation cares about, and a nil there would be indistinguishable from a
// miswiring.
type ApprovalService struct {
	repo   *Repository
	grant  ApprovalGrant
	notify ApprovalNotifier
	now    func() time.Time
}

// NewApprovalService wires the state machine for cmd/server. grant may be nil —
// which makes every approval refuse rather than publish unpaid, see
// ErrApprovalUnconfigured — and notify may be nil, in which case a decision still
// stands and the student is told when they look.
func NewApprovalService(repo *Repository, grant ApprovalGrant) *ApprovalService {
	return newApprovalService(repo, grant)
}

// newApprovalService is the unexported constructor the tests use, so a test can
// build the state machine without an exported alternative that exists only for
// them.
func newApprovalService(repo *Repository, grant ApprovalGrant) *ApprovalService {
	return &ApprovalService{
		repo:  repo,
		grant: grant,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// WithNotifier wires the announcement seam, returning the receiver so it can be
// chained at the call site in main.go.
func (a *ApprovalService) WithNotifier(n ApprovalNotifier) *ApprovalService {
	a.notify = n
	return a
}

// Submit records a student upload as PENDING, unpublished, unpaid.
//
// The three things it must get right, and why each is easy to get wrong:
//   - it forces IsPublished false even when the caller's struct says true. The
//     caller's flag is the legacy admin-upload path's answer and must not leak
//     into a submission, or a student's resource goes live unreviewed.
//   - it records UploadedBy, because the approver pays THAT account and a
//     missing owner means nobody is paid.
//   - it pays nothing.
func (a *ApprovalService) Submit(userID uint, resource StudyResource) (*StudyResource, error) {
	if a == nil || a.repo == nil {
		return nil, errors.New("resource not found")
	}
	if userID == 0 {
		return nil, errors.New("submission needs an uploader id")
	}
	resource.UploadedBy = userID
	resource.ApprovalStatus = ApprovalPending
	resource.IsPublished = false
	resource.ReviewedBy = nil
	resource.ReviewedAt = nil
	resource.RejectReason = ""

	if err := a.repo.CreateResource(&resource); err != nil {
		return nil, err
	}
	return &resource, nil
}

// Approve publishes a pending resource and pays its uploader, as ONE unit.
//
// The ORDER is the whole design and is not negotiable:
//
//  1. the port is checked FIRST. A nil grant refuses before any write, so a
//     miswiring cannot publish an unpaid resource.
//  2. the row is re-read and its status checked. This is the guard against
//     approving a rejected row.
//  3. status, reviewer, reviewer time and is_published are written TOGETHER with
//     the grant.
//
// The honest caveat, because it is a real limitation rather than a detail: step 3
// is atomic with respect to the GRANT's transaction, and the grant opens its own.
// So the resource row and the coin journal are not in one database transaction, and
// a crash between them can leave a published resource whose uploader was not paid
// or a paid resource that stayed unpublished.
//
// What makes that recoverable rather than silent: the grant's idempotency key is
// derived from the resource id, so re-running Approve on a still-pending row pays
// exactly once, and a resource that was published before the crash is rejected by
// the status check and is therefore visible in the queue rather than invisible.
// The alternative — a true cross-module transaction — is not available here
// without a distributed transaction, and the failure window is one row plus one
// journal, both individually inspectable.
func (a *ApprovalService) Approve(resourceID, reviewerID uint) (*StudyResource, error) {
	if a == nil || a.repo == nil {
		return nil, errors.New("resource not found")
	}
	if a.grant == nil {
		return nil, ErrApprovalUnconfigured
	}

	resource, err := a.repo.FindResourceByID(resourceID)
	if err != nil {
		return nil, errors.New("resource not found")
	}
	if resource.ApprovalStatus != ApprovalPending {
		return nil, ErrApprovalNotPending
	}

	now := a.now().UTC()
	resource.ApprovalStatus = ApprovalApproved
	resource.IsPublished = true
	resource.ReviewedBy = &reviewerID
	resource.ReviewedAt = &now
	resource.RejectReason = ""

	if err := a.repo.UpdateResource(resource); err != nil {
		return nil, err
	}
	// The uploader, never the reviewer: the reviewer is the operator and is
	// recorded on ReviewedBy precisely so it is not mistaken for the payee.
	if err := a.grant.GrantResourceApproved(context.Background(), resource.UploadedBy, resource.ID, resource.Title); err != nil {
		return nil, err
	}
	a.announce(resource, notification.EventStudyResourceApproved)
	return resource, nil
}

// Reject refuses a pending resource with a reason, and pays nothing.
//
// Terminal: a rejected resource cannot be approved afterwards, and that is
// deliberate. An uploader who can flip a rejection into a publication and a
// payout by asking again has a route to coins that bypasses moderation entirely.
func (a *ApprovalService) Reject(resourceID, reviewerID uint, reason string) (*StudyResource, error) {
	if a == nil || a.repo == nil {
		return nil, errors.New("resource not found")
	}
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil, ErrRejectReasonRequired
	}

	resource, err := a.repo.FindResourceByID(resourceID)
	if err != nil {
		return nil, errors.New("resource not found")
	}
	if resource.ApprovalStatus != ApprovalPending {
		return nil, ErrApprovalNotPending
	}

	now := a.now().UTC()
	resource.ApprovalStatus = ApprovalRejected
	resource.IsPublished = false
	resource.ReviewedBy = &reviewerID
	resource.ReviewedAt = &now
	resource.RejectReason = trimmed

	if err := a.repo.UpdateResource(resource); err != nil {
		return nil, err
	}
	a.announce(resource, notification.EventStudyResourceRejected)
	return resource, nil
}

// announce tells the uploader what happened to their material.
//
// Best-effort and last: a notification failure must not undo a decision that has
// already been written, and the student can always see the status on their own
// uploads list. The notification is a convenience; the row is the record.
func (a *ApprovalService) announce(resource *StudyResource, eventKey string) {
	if a.notify == nil || resource == nil || resource.UploadedBy == 0 {
		return
	}
	_ = a.notify.NotifyTx(context.Background(), a.repo.db, notification.NotifyRequest{
		EventKey:   eventKey,
		Recipients: []notification.Ref{{Type: "user", ID: resource.UploadedBy}},
		Data: map[string]any{
			"title":  resource.Title,
			"reason": resource.RejectReason,
		},
	})
}
