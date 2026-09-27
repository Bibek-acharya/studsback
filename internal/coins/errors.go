package coins

import "errors"

// Domain errors surfaced to the handler. Package-level sentinels matched with
// errors.Is, per the house style at internal/mocktests/service.go:13-23.
//
// The ledger sentinels listed in 03-api-contract.md §4 (ErrInsufficientCoins,
// ErrAccountFrozen, …) are deliberately absent: they belong to the accounts and
// journal slice, which is not built here.
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
