// cmd/server/starter_allowance_wiring.go
//
// The adapter between auth's StarterAllowanceGranter port and the coin
// economy's Service.EnsureAllowance. auth owns the trigger (it creates the
// account), coins owns the rules (the grant's size, expiry and once-per-
// lifetime idempotency) — the same seam as profile_award_wiring.go and
// referral_wiring.go, in a file of its own for the same reason.
package main

import (
	"context"
	"errors"
	"time"

	"studsphere/backend/internal/coins"
)

type starterAllowanceGranterAdapter struct {
	svc *coins.Service
}

func (a *starterAllowanceGranterAdapter) GrantStarterAllowance(ctx context.Context, userID uint) error {
	if a == nil || a.svc == nil {
		// An error rather than a silent success: the caller logs it, and a
		// silent success would make an unwired deployment indistinguishable
		// from a granted allowance. Compare referralAttributorAdapter.
		return errors.New("auth: starter allowance granter is not wired")
	}
	// The clock is injected here rather than inside auth for the same reason
	// every coins entry point takes `now`: the economy's tests pin expiry
	// arithmetic against fixed times, and auth has no business knowing an
	// expiry exists.
	_, err := a.svc.EnsureAllowance(ctx, userID, time.Now().UTC())
	return err
}
