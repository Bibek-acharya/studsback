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
