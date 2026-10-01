// internal/auth/profile_award.go
//
// Where the profile-completion award is triggered, and why it is triggered here
// rather than anywhere more automatic.
//
// ── the enumeration, which is the whole reason this file exists ───────────────
//
// ProfileCompletion (internal/studentdashboard/service.go) scores twelve checks:
// first_name, last_name, phone, date_of_birth, gender, nationality, address, bio,
// preferences.onboarding_completed, image_url, an education entry, and email.
// Every one of those is written somewhere, and "written somewhere" is where this
// award can be lost. The list below is the audit; a path not on it is a bug, and
// a path on it that does not call awardProfile is a lost award.
//
//	#  completion field(s)            write path
//	1  first/last/phone/dob/gender/  UpdateProfile (service.go:565)
//	   nationality/address/bio/
//	   image_url
//	2  image_url                     UploadProfilePicture handler (handler.go:437)
//	                                  -> calls UpdateProfile, so it is #1
//	3  image_url                     GoogleLoginOrRegister (service.go:490)
//	                                  -> picture download, SaveUser at :520
//	4  preferences.onboarding_       SavePreferences (service.go:625)
//	   _completed
//	5  education entry count         CreateEducationEntry (service.go:1817)
//	6  education entry count         UpdateEducationEntry (service.go:1849)
//	7  education entry count         DeleteEducationEntry (service.go:1883)
//	8  email + onboarding prefs      VerifyOTP (service.go:439) at user creation
//
// That is the set. Note what is NOT in it, because the obvious guess is wrong:
//
//   - Login (service.go:177) SaveUsers, but only LastLoginAt. Not a completion
//     field, and calling the award on every login would be a coin-system query
//     on the hottest read path in the app.
//   - ChangePassword, GenerateTOTPSecret, EnableTOTP, DisableTOTP,
//     DeactivateAccount, QueueDeletion, CancelDeletion, ResetPassword: all
//     SaveUser, none touching a completion field.
//   - SuperadminRegister (service.go:1736) creates a USER row but is a staff
//     account with no student profile to complete.
//
// ── why not hook auth.Repository.SaveUser ─────────────────────────────────────
//
// SaveUser is the obvious chokepoint and it is the wrong one, for two separate
// reasons, either of which alone disqualifies it.
//
// First, DIRECTION. SaveUser is the lowest layer of the auth module: it is a thin
// GORM wrapper with no knowledge of any other module. An award is the coin
// economy reaching into a profile write, and putting that call inside SaveUser
// inverts the dependency — internal/auth would import internal/coins, and every
// future caller of SaveUser (there are twelve) would silently acquire a
// coin-system transaction. The codebase already draws this line deliberately:
// internal/coins does not import internal/auth, and the profile lookup crosses
// the boundary through an adapter defined in cmd/server/main.go. SaveUser would
// be the one place that rule is broken.
//
// Second, and worse: SaveUser is INCOMPLETE, and silently so. Preferences moves
// through UpdatePreferences (repository.go:47), which is a different method, and
// the education count moves through the education_entries table via
// CreateEducationEntry. Neither calls SaveUser. An award hooked only to SaveUser
// would pay a student who filled in the profile form and never pay a student who
// finished onboarding and added an education entry — which is the precise
// "lost award, and an incentive to use one specific form" failure the award is
// supposed to avoid. A chokepoint that misses half the writers is worse than no
// chokepoint, because it looks handled.
//
// GORM model hooks (AfterSave) were rejected for the same two reasons plus one:
// they are invisible at the call site, they cannot participate in the award's own
// transaction, and a struct tag that pays money is something nobody will find
// during review.
//
// ── what makes "no path skips the award" more than a promise ─────────────────
//
// The call is cheap and safe to repeat, so over-calling is the safe failure. Each
// call re-reads completion and attempts a claim insert per step; for an unchanged
// profile every insert hits ON CONFLICT and returns no row, so a redundant call
// costs a handful of no-op statements and writes nothing. That means a future
// writer who calls awardProfile on a path that does not touch a completion field
// costs nothing, and the per-path tests in internal/coins are what turn "we
// enumerated the paths" into "each one is proven".
package auth

import (
	"context"
	"log"
)

// ProfileAwarder is the coin economy's profile award, as this module sees it.
//
// Declared here, in the module that OWNS the trigger, and satisfied by
// internal/coins. The direction matters and is the same one the notifier already
// uses in this module: auth depends on the port, main wires the implementation,
// and neither side imports the other. A narrower port than a whole *coins.Service
// on purpose — one method, so wiring it to a stub in a test is one line and so
// this module cannot grow a dependency on coin-economy internals by accident.
type ProfileAwarder interface {
	// AwardProfileSteps pays every ladder step this user's profile completion
	// has reached and that has not been paid before. It is idempotent, safe to
	// call from any path at any time, and must never block the profile write it
	// follows — see awardProfile.
	AwardProfileSteps(ctx context.Context, userID uint) ([]ProfileAwardStep, error)
}

// ProfileAwardStep is one step an award call paid, echoed back so callers can
// report it. A distinct type from the coins one on purpose: this module's callers
// must not be able to reach into coin internals through the seam.
type ProfileAwardStep struct {
	Step  int
	Coins int64
}

// profileAwarderInstance is nil until main wires it. Every call site tolerates
// nil, which keeps this module's own tests — and the seeder, and any future
// consumer — working without a coin economy configured.
var profileAwarderInstance ProfileAwarder

// SetProfileAwarder wires the award. Called once from main.
func SetProfileAwarder(a ProfileAwarder) { profileAwarderInstance = a }

// awardProfile is the single funnel every profile-writing path calls.
//
// It returns nothing and swallows nothing silently: a failure to award is logged
// and NOT propagated. That is a deliberate trade. The alternative — failing the
// profile save because the award could not be paid — means a coin-system problem
// (a locked table, a config read failure) makes students unable to edit their
// profile, which is a far worse outcome than a delayed award, and the award is
// self-healing: the next profile write retries, and because the claim insert is
// per step and idempotent, a retry after a partial failure pays exactly the steps
// that were missed and nothing twice.
//
// ctx is the caller's context so a request that is cancelled mid-award does not
// leave a half-paid ladder, and background for the paths with no request context
// (the Google login picture download).
func (s *Service) awardProfile(ctx context.Context, userID uint) {
	if profileAwarderInstance == nil || userID == 0 {
		return
	}
	if _, err := profileAwarderInstance.AwardProfileSteps(ctx, userID); err != nil {
		log.Printf("auth: profile award for user %d did not complete: %v", userID, err)
	}
}
