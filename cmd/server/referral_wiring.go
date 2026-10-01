// cmd/server/referral_wiring.go
//
// The adapter that lets internal/auth record a referral without either module
// importing the other.
//
// Same shape and same reason as profile_award_wiring.go: auth owns the trigger (it
// is the only module that inserts into the three account tables) and declares a
// one-method port; coins owns the attribution table and the fraud constraints; the
// adapter that satisfies one from the other lives HERE because neither may import
// the other. internal/auth importing internal/coins would make the auth module —
// which every other module in the codebase depends on, directly or through the
// middleware that reads its session tokens — depend on the coin economy, so a
// coins build failure would become an auth build failure.
package main

import (
	"context"
	"errors"
	"fmt"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/coins"
)

// referralSubjectKinds maps this module's account-table vocabulary onto the coin
// system's.
//
// EXHAUSTIVE, and it has to be. Both sides declare the same three subjects for the
// same reason (neither module may import the other), and a duplicated vocabulary
// that drifts is a silent mis-mapping: a new account table attributes as the wrong
// subject, which means an attribution row pointing at an id from a different table
// — the exact defect the referral table's referred_kind column exists to prevent.
//
// The exhaustion is asserted rather than assumed. TestReferralSubjectMappingCovers
// EveryAuthSubject in this package's tests walks both lists and fails if either
// has an entry the other does not, so adding a fourth account table breaks the
// build rather than producing a wrong attribution.
var referralSubjectKinds = map[string]string{
	auth.ReferralSubjectUser:        coins.SubjectUser,
	auth.ReferralSubjectInstitution: coins.SubjectInstitution,
	auth.ReferralSubjectProvider:    coins.SubjectProvider,
}

// referralAttributorAdapter satisfies auth.ReferralAttributor over the coin
// system's referral service.
type referralAttributorAdapter struct {
	svc *coins.ReferralService
}

func (a *referralAttributorAdapter) ApplyReferral(ctx context.Context, subject auth.ReferralSubject) (auth.ReferralAttribution, error) {
	if a == nil || a.svc == nil {
		// No coin economy wired. An error rather than a silent success: the caller
		// logs it, and a silent success here would make a missing referral
		// indistinguishable from a signup that carried no code. Compare
		// profileAwarderAdapter, which errors for the same reason.
		return auth.ReferralAttribution{}, errors.New("auth: referral attributor is not wired")
	}
	kind, ok := referralSubjectKinds[subject.Kind]
	if !ok {
		// An unmapped subject is a bug in this file or a new account table that
		// nobody wired. Refusing is right: attributing it as the wrong subject would
		// write a row pointing at an id from a table it does not belong to, and the
		// uniqueness constraint would then refuse a legitimate attribution of the
		// real account for that id.
		return auth.ReferralAttribution{}, fmt.Errorf("auth: no coin-system referral subject for %q", subject.Kind)
	}
	res, err := a.svc.ApplyReferral(ctx, coins.Attribution{
		ReferredKind:   kind,
		ReferredUserID: subject.ID,
		ReferralCode:   subject.Code,
		SourcePath:     subject.Path,
	})
	if err != nil {
		return auth.ReferralAttribution{}, err
	}
	return auth.ReferralAttribution{Applied: res.Attributed, Reason: res.Reason}, nil
}

var (
	_ auth.ReferralAttributor       = (*referralAttributorAdapter)(nil)
	_ coins.ReferralAttributionPort = (*coins.ReferralService)(nil)
)
