// cmd/server/profile_award_wiring.go
//
// The adapters that let the coin economy ask the student dashboard how complete a
// profile is, without either module importing the other.
//
// This is the same shape as profileCompletionAdapter in main.go, and the reason
// it is a second adapter rather than a shared one is worth a note: the unlock API
// needs a BOOLEAN ("is this profile finished?", for ways_to_earn) and the award
// needs a PERCENTAGE ("which of the five steps does this reach?", for the ladder).
// Two questions, two port methods, one implementation of the twelve checks
// underneath in studentdashboard. Sharing one adapter would mean a single method
// returning a percentage with the boolean caller doing `>= 100` itself, which
// moves the definition of "finished" out of the module that owns it and into a
// caller that has no business stating it.
package main

import (
	"context"
	"errors"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/coins"
	"studsphere/backend/internal/studentdashboard"
)

// profilePercentAdapter satisfies coins.ProfileCompletion.
type profilePercentAdapter struct {
	svc *studentdashboard.Service
}

func (a *profilePercentAdapter) ProfileCompletionPercent(ctx context.Context, userID uint) (int, error) {
	if a == nil || a.svc == nil {
		// No lookup wired. Returning an error makes the award fail loudly rather
		// than silently reporting 0% — a wired-but-empty service would look
		// exactly like a student with an empty profile and would pay nothing,
		// forever, which is the quietest available way to break an earn mechanic.
		return 0, errors.New("coins: profile completion adapter is not wired")
	}
	return a.svc.ProfileCompletionPercent(ctx, userID)
}

// profileAwarderAdapter satisfies auth.ProfileAwarder over the coin economy's
// award service.
//
// It exists because the auth port deliberately returns auth's own step type
// rather than the coins one, so the module that owns the trigger cannot reach
// into coin internals through the seam. This is the only place the two shapes
// meet.
type profileAwarderAdapter struct {
	svc *coins.ProfileAwardService
}

func (a *profileAwarderAdapter) AwardProfileSteps(ctx context.Context, userID uint) ([]auth.ProfileAwardStep, error) {
	if a == nil || a.svc == nil {
		return nil, errors.New("auth: profile awarder is not wired")
	}
	steps, err := a.svc.AwardProfileSteps(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]auth.ProfileAwardStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, auth.ProfileAwardStep{Step: s.Step, Coins: s.Coins})
	}
	return out, nil
}

var (
	_ coins.ProfileCompletion = (*profilePercentAdapter)(nil)
	_ auth.ProfileAwarder     = (*profileAwarderAdapter)(nil)
)
