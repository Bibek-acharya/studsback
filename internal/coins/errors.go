package coins

import "errors"

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
