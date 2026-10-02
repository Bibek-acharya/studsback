// internal/coins/resource_award.go
//
// 04-implementation-plan.md §5.3, step 4: the 80-coin grant an uploader receives
// when their resource is PUBLISHED.
//
// internal/studyresources owns the decision and declares the port
// (ApprovalGrant, in approval.go); this file satisfies it. The direction is fixed
// by the import graph — coins already imports studyresources for its resource
// lookup, so the port had to be declared on the side that does not need coins'
// types. Same rule as the download gate, applied the other way round.
//
// ── why this exists at all rather than a call from main.go ────────────────────
//
// The award could have been a closure assembled in cmd/server/main.go. It is not,
// for one reason: the IDEMPOTENCY KEY. It has to be built from the resource id
// with a prefix nothing else uses, and that fact is only checkable by someone who
// can see every key producer in the package. A closure in main.go would put the
// key outside every reach of that check, and the failure it invites is silent —
// one student's second publication swallowed as a replay of the first.

package coins

import (
	"context"
	"strconv"
)

// resourceApprovedKey is the idempotency key for one published resource's award.
//
// KEYED ON THE RESOURCE, NOT THE USER, and that is deliberate even though the
// payee is the user. A user-keyed key would be "resource-approved:<userID>", and a
// student who publishes two resources would find the second award treated as a
// replay of the first — paid once, silently. The resource id is the thing being
// awarded for, so it is what the key names.
//
// The prefix is this package's to keep. coin_journal has ONE UNIQUE
// (scope, idempotency_key) across every reason code, so a collision here is either
// ErrIdempotencyKeyReuse or — worse — a silent replay of a different award.
// resource_award_test.go pins the prefix against every other producer.
func resourceApprovedKey(resourceID uint) string {
	return "resource-approved:" + strconv.FormatUint(uint64(resourceID), 10)
}

// GrantResourceApproved pays the uploader for a resource that has been APPROVED.
//
// It returns the ledger's own GrantResult, including Replayed, so the caller can
// tell a payment from a no-op. studyresources' ApprovalGrant port discards it —
// the state machine does not need to know — but the pg test does, because the
// idempotency claim below is asserted on that flag rather than on a call count.
//
// It is a package function rather than a method on Ledger so that
// internal/studyresources can be handed a plain func with no dependency on this
// package's types — the port takes a method, so main.go supplies a one-line
// closure, and this signature is what that closure wraps.
//
// The ledger argument is explicit and may be nil, which refuses. That is the
// correct answer for a money path: studyresources checks its own port for nil
// before calling, and this is the second refusal for the case where a ledger is
// constructed with no handle — a miswiring that must be loud rather than a
// published resource whose uploader is never paid.
//
// Idempotent by construction. A second call with the same resource id produces
// the same key, the UNIQUE index reports the conflict, and Ledger.Grant's replay
// path returns the ORIGINAL journal with Replayed set — so no second payment and
// no error. That is what makes approval safe to retry after a partial failure.
// NewResourceApprovedAward adapts the function above to the method shape
// internal/studyresources' ApprovalGrant port declares, so cmd/server can pass a
// value rather than writing a closure inline at the wiring site.
//
// It exists as a named constructor rather than a closure because the wiring site in
// main.go should read as a list of ports being connected, not as a lambda — a
// closure there would hide the one line in this package that decides the
// idempotency key.
func NewResourceApprovedAward(ledger *Ledger) *ResourceApprovedAward {
	return &ResourceApprovedAward{ledger: ledger}
}

// ResourceApprovedAward is the port-shaped view of this module's upload award.
type ResourceApprovedAward struct {
	ledger *Ledger
}

// GrantResourceApproved is the method the port declares. The ledger may be nil,
// which refuses — see the package function.
func (a *ResourceApprovedAward) GrantResourceApproved(ctx context.Context, userID, resourceID uint, title string) error {
	if a == nil {
		return ErrNoDatabase
	}
	_, err := GrantResourceApproved(ctx, a.ledger, userID, resourceID, title)
	return err
}

func GrantResourceApproved(ctx context.Context, ledger *Ledger, userID, resourceID uint, title string) (GrantResult, error) {
	if ledger == nil {
		return GrantResult{}, ErrNoDatabase
	}
	if userID == 0 {
		return GrantResult{}, ErrInvalidArgument
	}
	if resourceID == 0 {
		return GrantResult{}, ErrInvalidArgument
	}

	refType := RefStudyResource
	refID := uint64(resourceID)
	// The title travels as metadata rather than in the fingerprint. It is not
	// part of the award's identity: an admin correcting a typo must not be able to
	// turn a replay into ErrIdempotencyKeyReuse, because that would make a
	// cosmetic edit look like a different payment.
	grant, err := ledger.Grant(ctx, GrantRequest{
		UserID:         userID,
		ReasonCode:     ReasonResourceApproved,
		IdempotencyKey: resourceApprovedKey(resourceID),
		RefType:        &refType,
		RefID:          &refID,
		CreatedBy:      "system:resource_award",
		Metadata: map[string]any{
			"title":          title,
			"resource_id":    resourceID,
			"award":          "resource_approved",
			"moderated":      true,
			"earning_reason": "Your resource was approved and published.",
		},
	})
	if err != nil {
		return GrantResult{}, err
	}
	return grant, nil
}
