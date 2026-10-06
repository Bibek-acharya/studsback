// internal/auth/starter_allowance.go
//
// Where the StudsToken starter allowance is granted, and why it is granted
// here rather than anywhere more automatic.
//
// The allowance (internal/coins' user_free_allowance: a handful of free
// unlocks per class, once per lifetime) exists so a brand-new student can
// open their first documents before they have earned a single coin. The coin
// economy owns the RULES; auth owns the only moment every account definitely
// passes through — its creation. That is the same split as the referral
// attribution (referral.go) and the profile award (profile_award.go), and
// this file deliberately mirrors both.
//
// ── which creation paths grant ───────────────────────────────────────────────
//
// The grant fires on the paths that create a row in `users` — the student
// table, the only kind of account that unlocks study resources:
//
//	#  path                        where
//	1  VerifyOTP (student)         service.go, ReferralSubjectUser branch
//	2  GoogleLoginOrRegister       service.go, the CreateUser branch
//
// The other four creation paths — institution and scholarship-provider, OTP
// and Google — create rows in institution_users / scholarship_provider_users,
// which never unlock resources, so they never receive an allowance. A
// superadmin account (SuperadminRegister) creates a USER row but is a staff
// account: it is excluded for the same reason profile_award.go excludes it,
// and a staff member testing with their own account loses nothing.
//
// ── why not inside the coin economy ──────────────────────────────────────────
//
// The economy's own fallback is ConsumeAllowance's lazy create: a missing row
// is created at first unlock. That is unreachable while the gates are off and
// invisible on the wallet until it happens — a student told they have starter
// unlocks should be able to SEE them from day one, which is what the wallet's
// StarterAllowanceCard renders from. Granting at creation makes the row exist
// before the first login completes.
//
// ── failure semantics ────────────────────────────────────────────────────────
//
// Identical to applyAttribution: a grant failure is logged and NEVER
// propagated. The alternative is that a coin-system problem — a locked table,
// an unmigrated schema — makes students unable to create accounts, which is
// far worse than a missing allowance. And unlike a referral, a missing grant
// is fully recoverable: EnsureAllowance is idempotent, so the backfill
// migration (migrations.BackfillStarterAllowances) or the lazy create in
// ConsumeAllowance will produce exactly the row this call would have.
package auth

import (
	"context"
	"log"
)

// StarterAllowanceGranter is the coin economy's one-time grant, as this
// module sees it. One method, on purpose: it is idempotent (UNIQUE (user_id)
// resolved ON CONFLICT DO NOTHING inside EnsureAllowance), cheap, and safe to
// call from every creation path — over-calling is the safe failure, exactly
// as with ReferralAttributor.
type StarterAllowanceGranter interface {
	// GrantStarterAllowance creates the user's starter allowance row sized
	// from the current economy config, or returns the existing one unchanged.
	// It must never refuse a policy outcome: an already-granted user is a
	// success, not an error.
	GrantStarterAllowance(ctx context.Context, userID uint) error
}

// starterAllowanceGranterInstance is nil until main wires it. Every call site
// tolerates nil, which keeps this module's own tests and any deployment
// without a coin economy working unchanged — the same shape and the same
// reason as referralAttributorInstance and profileAwarderInstance.
var starterAllowanceGranterInstance StarterAllowanceGranter

// SetStarterAllowanceGranter wires the grant. Called once from main.
func SetStarterAllowanceGranter(g StarterAllowanceGranter) { starterAllowanceGranterInstance = g }

// grantStarterAllowance is the funnel every student-creation path calls.
//
// It returns nothing and swallows nothing silently: a failure is logged with
// the path and NOT propagated, for the reason given in the file header. The
// path argument exists for the same reason applyAttribution's does — a silent
// grant failure is otherwise indistinguishable from a signup on a path that
// deliberately does not grant.
func (s *Service) grantStarterAllowance(ctx context.Context, userID uint, path string) {
	if starterAllowanceGranterInstance == nil {
		return
	}
	if userID == 0 {
		log.Printf("auth: starter allowance grant on path %s has no account id; nothing was granted", path)
		return
	}
	if err := starterAllowanceGranterInstance.GrantStarterAllowance(ctx, userID); err != nil {
		log.Printf("auth: starter allowance grant on path %s did not complete for user %d: %v",
			path, userID, err)
	}
}
