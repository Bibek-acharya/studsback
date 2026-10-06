// internal/auth/starter_allowance_test.go
//
// One test per account-creation path, proving the starter allowance is granted
// exactly where a student account is born and nowhere else.
//
// The shape and the reason are the same as referral_attribution_test.go's: the
// grant is triggered by explicit calls in THIS module, and a missing call is
// not a bug anything else can catch — the coins package's tests prove
// EnsureAllowance works WHEN CALLED (and that it is idempotent); what they
// cannot see is a creation path that never calls it, which is the student who
// opens a wallet with no starter unlocks in it.
//
// The granter is a stub for the same reason the attributor is: these tests
// assert "this path requests the grant", not "the row is written correctly" —
// that is internal/coins' pg-tested business, and the stub keeps these tests
// on SQLite in the ordinary `go test ./...`.
package auth

import (
	"context"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/utils"
)

// stubGranter records every grant request and can be made to fail, so a test
// can assert both that a path grants and that a coin-system failure does not
// fail the signup it follows (grantStarterAllowance's contract).
type stubGranter struct {
	userIDs []uint
	Err     error
}

func (g *stubGranter) GrantStarterAllowance(_ context.Context, userID uint) error {
	g.userIDs = append(g.userIDs, userID)
	return g.Err
}

// newAllowanceAuthService builds the same fixture as the referral tests and
// wires the grant stub on top of it.
func newAllowanceAuthService(t *testing.T) (*Service, *stubGranter) {
	t.Helper()
	svc, _ := newReferralAuthService(t)
	granter := &stubGranter{}
	SetStarterAllowanceGranter(granter)
	t.Cleanup(func() { SetStarterAllowanceGranter(nil) })
	return svc, granter
}

func assertGrantedOnce(t *testing.T, g *stubGranter, id uint, path string) {
	t.Helper()
	if len(g.userIDs) == 0 {
		t.Fatalf("user-creation path %s did not grant the starter allowance: the grant "+
			"is triggered by explicit calls in this module, and a missing one leaves the "+
			"wallet empty with nothing else reporting it", path)
	}
	if len(g.userIDs) > 1 {
		t.Fatalf("user-creation path %s granted %d times (%v); once per lifetime means one call",
			path, len(g.userIDs), g.userIDs)
	}
	if g.userIDs[0] != id {
		t.Errorf("path %s granted user %d, want %d", path, g.userIDs[0], id)
	}
}

func assertNotGranted(t *testing.T, g *stubGranter, because string) {
	t.Helper()
	if len(g.userIDs) != 0 {
		t.Fatalf("starter allowance granted %d time(s) (%v) but should not have: %s",
			len(g.userIDs), g.userIDs, because)
	}
}

// The student OTP registration grants, sized and timed by the economy. This is
// the path every email/password signup comes down.
func TestVerifyOTPStudentGrantsStarterAllowance(t *testing.T) {
	svc, granter := newAllowanceAuthService(t)
	user := completeReferralRegistration(t, svc, "allowance-otp@example.com", "")

	assertGrantedOnce(t, granter, user.ID, "verify_otp")
}

// The Google student registration grants too — its own creation branch, for
// the same reason attribution has its own: a fix applied to one branch is not
// applied to the other.
func TestGoogleStudentGrantsStarterAllowance(t *testing.T) {
	svc, granter := newAllowanceAuthService(t)

	res, err := svc.GoogleLoginOrRegister("google-allowance", "allowance-google@example.com",
		"New", "Student", "", "")
	if err != nil {
		t.Fatalf("google login: %v", err)
	}
	assertGrantedOnce(t, granter, res.UserID, "google_login")
}

// An EXISTING student signing in with Google must not be re-granted. The call
// lives inside the creation branch, not after it — the same shape as
// attribution's fraud guard, and although EnsureAllowance would swallow the
// duplicate, an unconditional call is a coin-system transaction on every
// Google sign-in forever.
func TestGoogleSignInOfAnExistingStudentDoesNotRegrant(t *testing.T) {
	svc, granter := newAllowanceAuthService(t)

	if _, err := svc.GoogleLoginOrRegister("google-allowance-existing", "allowance-existing@example.com",
		"Existing", "Student", "", ""); err != nil {
		t.Fatalf("first sign-in: %v", err)
	}
	if len(granter.userIDs) != 1 {
		t.Fatalf("the creating sign-in granted %d times, want 1", len(granter.userIDs))
	}
	granter.userIDs = nil

	if _, err := svc.GoogleLoginOrRegister("google-allowance-existing", "allowance-existing@example.com",
		"Existing", "Student", "", ""); err != nil {
		t.Fatalf("second sign-in: %v", err)
	}
	assertNotGranted(t, granter,
		"the grant is creation-branch only; a sign-in is not a creation")
}

// Institution and provider accounts never unlock study resources, so neither
// their OTP nor their Google creation paths may grant. One test per branch,
// because each branch is its own call site that could drift.
func TestNonStudentCreationsDoNotGrantStarterAllowance(t *testing.T) {
	t.Run("institution otp", func(t *testing.T) {
		svc, granter := newAllowanceAuthService(t)
		if _, err := svc.InstitutionRegister(InstitutionRegisterRequest{
			InstitutionName: "Allowance Test College",
			Email:           "allowance-inst@example.com",
			ContactNumber:   "9800000000",
			Province:        "Bagmati",
			District:        "Kathmandu",
		}); err != nil {
			t.Fatalf("institution register: %v", err)
		}
		if err := svc.SendOTP("allowance-inst@example.com", "verification"); err != nil {
			t.Fatalf("send otp: %v", err)
		}
		otpType, staged, stagedReferral, found := utils.GetOTPStaged("allowance-inst@example.com")
		if !found {
			t.Fatal("no pending institution registration")
		}
		utils.StoreOTPWithReferral("allowance-inst@example.com", "424242", otpType, staged, stagedReferral)
		if _, err := svc.VerifyOTP("allowance-inst@example.com", "424242"); err != nil {
			t.Fatalf("verify otp: %v", err)
		}
		assertNotGranted(t, granter, "an institution account is not a student wallet")
	})

	t.Run("provider google", func(t *testing.T) {
		svc, granter := newAllowanceAuthService(t)
		if _, _, err := svc.ScholarshipProviderGoogleLoginOrRegister("google-allowance-provider",
			"allowance-provider@example.com", "Allowance Provider Trust", ""); err != nil {
			t.Fatalf("provider google login: %v", err)
		}
		assertNotGranted(t, granter, "a scholarship-provider account is not a student wallet")
	})
}

// A coin-system failure must not fail the signup — the account is worth more
// than the allowance, and EnsureAllowance's idempotency plus the backfill
// migration make the missed grant recoverable.
func TestAllowanceFailureDoesNotFailTheSignup(t *testing.T) {
	svc, granter := newAllowanceAuthService(t)
	granter.Err = errSimulatedCoinOutage

	res, err := svc.GoogleLoginOrRegister("google-allowance-outage", "allowance-outage@example.com",
		"New", "Student", "", "")
	if err != nil {
		t.Fatalf("a coin-system outage failed the signup: %v", err)
	}
	if res.Token == "" {
		t.Fatal("no session token issued")
	}
	if len(granter.userIDs) != 1 {
		t.Fatalf("grant attempted %d times, want 1 (the attempt must be visible in the logs)",
			len(granter.userIDs))
	}
}

// The enumeration guard, mirroring TestUserCreationPathsAreAllAttributed: the
// grant sites are statements in function bodies, so only reading this module's
// own source can prove there are exactly the documented ones — and that the
// staff paths stay grant-free.
func TestStarterAllowanceGrantSitesAreExactlyThis(t *testing.T) {
	src, err := readOwnSource("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}

	// Two student-creation paths (VerifyOTP's users branch, GoogleLoginOrRegister's
	// CreateUser branch), one call each. A new count means a new grant site,
	// which is either a student path the enumeration in
	// internal/auth/starter_allowance.go does not know about, or a non-student
	// path that must not grant.
	if got := strings.Count(src, "s.grantStarterAllowance("); got != 2 {
		t.Errorf("service.go calls s.grantStarterAllowance %d times, want 2 — one per "+
			"student-creation path. See internal/auth/starter_allowance.go for the list", got)
	}

	// The staff paths must stay grant-free. CreateInstitution writes
	// institution_users, which is not a wallet at all; SuperadminRegister writes
	// a USER row but is a staff account, excluded for the same reason it is
	// excluded from the profile award.
	for _, fn := range []string{"func (s *Service) CreateInstitution(", "func (s *Service) SuperadminRegister("} {
		if !strings.Contains(src, fn) {
			t.Errorf("%s no longer exists; internal/auth/starter_allowance.go documents it as "+
				"a deliberately grant-free account-creation path and that list must be updated", fn)
			continue
		}
		if strings.Contains(sourceAfter(src, fn), "grantStarterAllowance(") {
			t.Errorf("%s now grants the starter allowance; staff and institution accounts are "+
				"not student wallets — update the enumeration in internal/auth/starter_allowance.go if deliberate", fn)
		}
	}
}
