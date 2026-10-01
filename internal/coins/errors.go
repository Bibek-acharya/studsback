package coins

import (
	"errors"
	"fmt"
)

// Domain errors surfaced to the handler. Package-level sentinels matched with
// errors.Is, per the house style at internal/mocktests/service.go:13-23.
//
// ErrInvalidConfig is matched by *ValidationError, so the handler maps a
// rejected field to 400 without knowing the concrete type.
// ErrConfigUnreadable is a storage problem, not a request problem: a
// system_settings value this package wrote but cannot parse back. It maps to
// 500, because nothing the caller sent is wrong.
var (
	ErrInvalidConfig    = errors.New("invalid coin economy config")
	ErrConfigUnreadable = errors.New("coin economy config is not readable")
)

// The ledger sentinels, verbatim from 03-api-contract.md §4. Only the ones the
// transactional core can actually produce are declared here; ErrAccountExists,
// ErrCapReached and ErrSelfReferral belong to the referral and account slices,
// which are later phases, and declaring an unused sentinel is a promise nobody
// keeps.
//
// Matched with errors.Is, so every ledger method wraps rather than substitutes:
//
//	ErrInsufficientCoins   402 — the highest-traffic error in the feature, and a
//	                        designed state in the funnel rather than a failure.
//	ErrIdempotencyKeyReuse 409 — a key reused with a different payload is an
//	                        error, not a replay. See 02-architecture.md §12.2.
//	ErrAccountFrozen       423 — a spend against a coin_account whose closed_at
//	                        is set.
//	ErrNotFound            404 — the referenced journal or hold does not exist,
//	                        which for a hold means there is nothing to release.
//	ErrImmutable           409 — a journal that has already been reversed, or a
//	                        system account that was never seeded. Nothing here is
//	                        ever edited in place, so "already done" is the only
//	                        way to hit it.
//	ErrInvalidArgument     400 — a zero or negative amount, a missing user, an
//	                        unknown reason code, an unknown spend class.
//	ErrHoldExceedsBalance  409 — a hold larger than posted-reserved. The
//	                        no-overdraft CHECK would reject it anyway; naming it
//	                        turns a constraint violation into a typed answer.
//
// ErrHoldExceedsBalance is not in 03-api-contract.md §4 because no endpoint
// creates a hold in v1; it is here because Reserve is, and a Reserve that cannot
// be distinguished from a database error is not a usable primitive.
var (
	ErrInsufficientCoins   = errors.New("insufficient coins")
	ErrIdempotencyKeyReuse = errors.New("idempotency key reused with a different payload")
	ErrAccountFrozen       = errors.New("coin account is frozen")
	ErrNotFound            = errors.New("not found")
	ErrImmutable           = errors.New("entry is immutable")
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrHoldExceedsBalance  = errors.New("hold exceeds available balance")
)

// The referral sentinels, for the qualification state machine. All three are
// REFUSALS rather than failures, and all three are distinct because "the mechanic
// did the right thing" and the reason it did are different operational answers —
// the qualification pass counts them separately, and support asks which one
// happened.
//
//	ErrReferralNotYetPayable the referral is inside its referral.hold_days wait, or
//	    its invitee has not qualified yet. THE NORMAL CASE on every pass: almost
//	    every pending referral is inside its wait, and a pass that treats that as an
//	    error has nothing to report.
//	ErrReferralAlreadyPaid a settlement was attempted on a referral that already
//	    has a reward_grant claim. Its own sentinel rather than ErrImmutable so a
//	    caller can tell "we already paid this" from "this must never be paid",
//	    without string-matching an error message.
//	ErrQualificationUnverifiable NO phone-verification port is wired, so this build
//	    cannot establish §5.2's second condition at all. Not ErrNoDatabase and not
//	    ErrInvalidArgument: nothing is broken and nothing the caller sent is wrong,
//	    and the only correct response is to pay nothing. It is exported and given a
//	    sentinel precisely so that state is loud rather than looking like a quiet
//	    pass that found nothing to do.
var (
	ErrReferralNotYetPayable     = errors.New("referral is not yet payable")
	ErrReferralAlreadyPaid       = errors.New("referral has already been paid")
	ErrQualificationUnverifiable = errors.New("referral qualification cannot be verified in this build")
)

// The entitlement sentinels, for the resource_unlock / user_free_allowance
// domain (02-architecture.md §5). Appended, not reorganised: the block above is
// the ledger's vocabulary, and mixing the two would blur which errors a spend can
// produce and which an unlock can.
//
// Matched with errors.Is, so every method here wraps rather than substitutes.
//
//	ErrNoAllowanceRemaining 402/423 — the starter allowance cannot cover this.
//	    It is deliberately returned TOGETHER WITH ErrAllowanceExpired when the
//	    window has closed (see ConsumeAllowance), because the two answer different
//	    questions for the same student: "you have used your free unlocks" and
//	    "your free unlocks ran out on the 26th" are different sentences, and the
//	    second is the 423 in 03-api-contract.md §2.3. A caller that only wants
//	    "the allowance is not available" matches this one; a handler mapping 423
//	    matches the other.
//	ErrAllowanceExpired    423 — the window has closed. Never returned alone; it
//	    always accompanies ErrNoAllowanceRemaining.
//	ErrAlreadyUnlocked     200 — an unlock already exists for
//	    (user, resource_type, resource_id). It is NOT an error condition at the
//	    API boundary: 03-api-contract.md §2.3 says a mobile retry returns the
//	    success body with already_unlocked: true, because a 403 would render a
//	    failure for an outcome that succeeded. The sentinel exists so the domain
//	    can report it distinctly from a genuine refusal, not so a handler can
//	    4xx it.
//	ErrUnlockRevoked       409 — a second revocation of an already-revoked
//	    unlock. The first revocation is the record, so the second changes
//	    nothing. Deliberately not "already done and fine": overwriting the
//	    timestamp and the reason would destroy exactly the evidence a revocation
//	    exists to preserve.
//	ErrInvalidResourceType 400 — a resource_type that is not one of
//	    study_resource | video | mock_test. It has its own sentinel rather than
//	    riding ErrInvalidArgument because it is the one validation failure in
//	    this slice that names a request field, and the handler wants to say which
//	    field without string-matching an error message.
var (
	ErrNoAllowanceRemaining = errors.New("no starter allowance remaining")
	ErrAllowanceExpired     = errors.New("starter allowance has expired")
	ErrAlreadyUnlocked      = errors.New("resource is already unlocked")
	ErrUnlockRevoked        = errors.New("unlock is already revoked")
	ErrInvalidResourceType  = errors.New("not an unlockable resource type")
)

// ── the typed insufficiency ──────────────────────────────────────────────────
//
// Appended, because it is a refinement of a sentinel declared above rather than
// a new outcome.
//
// Spend decides the 402 in 03-api-contract.md §2.3 by comparing two numbers, and
// §2.3's body is required, available and shortfall. Those numbers were computed
// inside the transaction that has already rolled back by the time the handler
// renders, and a zero SpendResult is the only thing that comes back out of a
// failed Spend — so the handler re-read the wallet to recover them. That read is
// safe (unlock_api.go says why) but it is a read of a DIFFERENT moment from the
// decision, and the whole point of the refusal is to quote the decision.
//
// ErrInsufficient carries both figures out of the failed call. It is not a new
// sentinel: it UNWRAPS to ErrInsufficientCoins, so every
// errors.Is(err, ErrInsufficientCoins) in the status mapping, the code mapping
// and the tests keeps matching exactly as before, and the message is unchanged.
type InsufficientError struct {
	// Required is the price the spend was about: the server-resolved value of
	// EconomyConfig.Prices for the class, never a number from the request.
	Required int64
	// Available is posted - reserved across every spendable bucket, as read
	// inside the transaction that refused the spend, under the per-user advisory
	// lock. It is the figure the decision was made on, not a later one.
	Available int64
}

// Error reproduces the message ledger.go has always produced for this refusal, so
// a log line and a grep of the old string both still work.
func (e *InsufficientError) Error() string {
	return fmt.Sprintf("%s: need %d, %d available", ErrInsufficientCoins, e.Required, e.Available)
}

// Unwrap is what keeps the sentinel matching: errors.Is walks it, so
// errors.Is(ErrInsufficient(40, 25), ErrInsufficientCoins) is true.
func (e *InsufficientError) Unwrap() error { return ErrInsufficientCoins }

// Shortfall is the gap, floored at zero. The payload derives it rather than
// asking for it, so the two cannot disagree; this is here for a caller that has
// the error and not the payload.
func (e *InsufficientError) Shortfall() int64 {
	if e.Available >= e.Required {
		return 0
	}
	return e.Required - e.Available
}

// ErrInsufficient is the error Spend returns when the balance will not cover the
// price, built from the two figures it had already computed.
func ErrInsufficient(required, available int64) error {
	return &InsufficientError{Required: required, Available: available}
}

// InsufficientFigures pulls Required and Available back out of a refusal, and
// reports whether the error was carrying them at all.
//
// ok == false means somebody wrapped the bare ErrInsufficientCoins sentinel. No
// caller in this package does, and a caller that did would be quoting a wallet
// reading taken at the wrong moment — which is the bug the typed error exists to
// remove, not to make survivable.
func InsufficientFigures(err error) (required, available int64, ok bool) {
	var typed *InsufficientError
	if !errors.As(err, &typed) {
		return 0, 0, false
	}
	return typed.Required, typed.Available, true
}
