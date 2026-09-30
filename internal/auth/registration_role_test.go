package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// privilegedRoles is exactly the allow-list main.go hands to
// middleware.RequireRole for the admin groups that serve gated content with no
// coin check: GET /api/v1/admin/mock-tests/:id returns the full answer key,
// and GET /api/v1/admin/study-resources lists drafts and their object keys.
var privilegedRoles = []string{"superadmin", "super_admin"}

func registrationTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &InstitutionUser{}, &ScholarshipProviderUser{}, &UserSession{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	previous := config.AppConfig
	// SMTP port 1 is deliberately unreachable: utils.SendOTPEmail then logs the
	// OTP, which is how this test completes the real verification flow without
	// reaching into private package state.
	config.AppConfig = &config.Config{
		JWTSecret: "test-secret",
		JWTExpiry: "24h",
		SMTPHost:  "localhost",
		SMTPPort:  "1",
		SMTPUser:  "test@example.com",
		SMTPPass:  "password",
	}
	t.Cleanup(func() { config.AppConfig = previous })
	return NewService(NewRepository(db))
}

// completeRegistration drives the genuine public sign-up chain — Register, then
// VerifyOTP — and returns the session token the API hands back.
//
// Register stages the pending account in the OTP store under a code it
// generates internally and mails out, so the code is re-staged here under a
// known value. Everything that matters still comes from Register's own output:
// the staged record is read back out of the store rather than constructed, so
// the role under test is the one the public endpoint actually chose.
func completeRegistration(t *testing.T, svc *Service, email, password, claimedRole string) string {
	t.Helper()

	if _, err := svc.Register(RegisterRequest{
		Email:     email,
		Password:  password,
		FirstName: "Mallory",
		LastName:  "Attacker",
		Role:      claimedRole,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	otpType, staged := utils.GetOTPData(email)
	if staged == nil {
		t.Fatal("register staged no pending account")
	}
	utils.StoreOTPWithType(email, "424242", otpType, staged)

	verified, err := svc.VerifyOTP(email, "424242")
	if err != nil {
		t.Fatalf("verify otp: %v", err)
	}
	if verified.Token == "" {
		t.Fatal("verification returned no session token")
	}
	return verified.Token
}

// A public self-registration must not be able to choose its own authorization
// role.
//
// POST /api/v1/auth/register binds a `role` field straight out of the request
// body, and Register used to persist it verbatim. Every admin surface that
// serves gated content with no coin check is guarded by
// middleware.RequireRole("superadmin","super_admin") reading the role claim out
// of the session JWT — and that claim is minted from the stored row. So a
// self-registered account that asked for "superadmin" walked straight past the
// guard and could read the complete mock-test answer key, mock_test gate
// included, plus every draft study resource and its object key, having paid
// nothing.
//
// The dedicated admin sign-up (SuperadminRegister) sets its own role behind an
// access code, so the public student path has no legitimate use for the field.
func TestSelfRegistrationCannotChooseItsAuthorizationRole(t *testing.T) {
	for _, claimed := range []string{
		"superadmin", "super_admin", "admin", "institution",
		"scholarship_provider", "scholarship_provider_subuser",
	} {
		t.Run("claimed role "+claimed, func(t *testing.T) {
			svc := registrationTestService(t)
			email := "escalate-" + claimed + "@example.com"

			if _, err := svc.Register(RegisterRequest{
				Email:     email,
				Password:  "password123",
				FirstName: "Mallory",
				LastName:  "Attacker",
				Role:      claimed,
			}); err != nil {
				t.Fatalf("register: %v", err)
			}

			// The pending registration is what VerifyOTP persists, so this is
			// the role that would become the account.
			_, data := utils.GetOTPData(email)
			pending, ok := data.(User)
			if !ok {
				t.Fatalf("pending registration data = %T, want User", data)
			}
			if pending.Role == claimed {
				t.Fatalf("pending role = %q, want it ignored: a public registration must not mint a %q account", pending.Role, claimed)
			}
			if pending.Role != "student" {
				t.Errorf("pending role = %q, want the student default", pending.Role)
			}
		})
	}
}

// The consequence the test above exists to prevent: the session a
// self-registered account receives must not satisfy the admin role guard, on a
// group wired exactly like the admin mock-test group in main.go.
func TestSelfRegisteredSessionIsRefusedByTheAdminRoleGuard(t *testing.T) {
	svc := registrationTestService(t)
	token := completeRegistration(t, svc, "escalate@example.com", "password123", "superadmin")

	claims, err := utils.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate the issued session token: %v", err)
	}
	if claims.Role != "student" {
		t.Fatalf("issued session carries role %q; the admin guard would accept it", claims.Role)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	admin := r.Group("/api/v1/admin/mock-tests")
	admin.Use(middleware.Auth())
	admin.Use(middleware.RequireRole(privilegedRoles...))
	admin.GET("/:id", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"correct_option_id": 42}) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/mock-tests/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); strings.Contains(body, "correct_option_id") {
		t.Fatalf("the admin answer key was served to a self-registered account: %s", body)
	}
}

// A registration that sends no role at all is still a student, so the fix
// cannot be "reject any request that carries a role" — the onboarding persona
// the client sends in that field has to keep working.
func TestRegistrationWithoutARoleStillYieldsAStudent(t *testing.T) {
	svc := registrationTestService(t)

	if _, err := svc.Register(RegisterRequest{
		Email:          "ordinary@example.com",
		Password:       "password123",
		FirstName:      "Ordinary",
		LastName:       "Student",
		EducationLevel: "+2",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, data := utils.GetOTPData("ordinary@example.com")
	pending, ok := data.(User)
	if !ok {
		t.Fatalf("pending registration data = %T, want User", data)
	}
	if pending.Role != "student" {
		t.Errorf("pending role = %q, want student", pending.Role)
	}
	// The persona keeps its historical default too, so the onboarding UI is
	// unaffected for a registration that sends no role.
	if pending.Preferences == nil || pending.Preferences.Role != "student" {
		t.Errorf("preferences role = %+v, want the student default", pending.Preferences)
	}
}

// The onboarding persona the client sends in `role` is stored on the
// preferences blob, not on the authorization column. The fix must keep that
// behaviour: dropping the privilege escalation is not a reason to discard the
// student's own choice of persona.
func TestRegistrationKeepsTheClientRoleAsAnOnboardingPersona(t *testing.T) {
	svc := registrationTestService(t)

	if _, err := svc.Register(RegisterRequest{
		Email:          "persona@example.com",
		Password:       "password123",
		FirstName:      "Persona",
		LastName:       "Student",
		Role:           "science_student",
		EducationLevel: "+2",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, data := utils.GetOTPData("persona@example.com")
	pending, ok := data.(User)
	if !ok {
		t.Fatalf("pending registration data = %T, want User", data)
	}
	if pending.Role != "student" {
		t.Errorf("authorization role = %q, want student", pending.Role)
	}
	if pending.Preferences == nil {
		t.Fatal("preferences were dropped")
	}
	if pending.Preferences.Role != "science_student" {
		t.Errorf("preferences role = %q, want the client persona preserved", pending.Preferences.Role)
	}
	if pending.Preferences.Preferences["education_level"] != "+2" {
		t.Errorf("education_level = %v, want +2", pending.Preferences.Preferences["education_level"])
	}
}

// The session token must carry the persisted authorization role, not whatever
// the registration request asked for. This is the last link in the chain: the
// role claim is what RequireRole reads.
func TestIssuedSessionCarriesThePersistedAuthorizationRole(t *testing.T) {
	svc := registrationTestService(t)
	token := completeRegistration(t, svc, "sessions@example.com", "password123", "superadmin")

	claims, err := utils.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate issued session token: %v", err)
	}
	for _, privileged := range privilegedRoles {
		if strings.EqualFold(claims.Role, privileged) {
			t.Fatalf("issued session carries privileged role %q", claims.Role)
		}
	}

	stored, err := svc.repo.FindUserByEmail("sessions@example.com")
	if err != nil {
		t.Fatalf("load the persisted account: %v", err)
	}
	if stored.Role != claims.Role {
		t.Fatalf("stored role %q does not match the issued claim %q", stored.Role, claims.Role)
	}
}

// The persona must survive a registration that carries no education level.
//
// education_level has no `binding:"required"`, so a request may omit it, and
// completeRegistration omits it. When the persona assignment was nested inside
// that guard — which is how it naturally reads while making User.Role
// server-owned — the persona was silently dropped for exactly the registrations
// that were thinnest. The security property held throughout, so nothing failed
// and nothing said so: a security fix that quietly became a data-loss fix.
//
// Both halves are asserted together, because either alone would pass against a
// broken implementation. The authorization role must be server-owned AND the
// client's choice must survive.
func TestAPersonaSurvivesARegistrationWithNoEducationLevel(t *testing.T) {
	svc := registrationTestService(t)
	// completeRegistration sends no education_level, which is the case under test.
	completeRegistration(t, svc, "persona-only@example.com", "password123", "mock_test_student")

	user, err := svc.repo.FindUserByEmail("persona-only@example.com")
	if err != nil {
		t.Fatalf("load the persisted account: %v", err)
	}

	// The security half: the client's choice must not be the authorization role.
	if user.Role != "student" {
		t.Fatalf("User.Role = %q, want the server-owned \"student\"", user.Role)
	}

	// The data half: the persona must still be recorded somewhere durable.
	if user.Preferences == nil {
		t.Fatal("Preferences is nil — the persona was dropped for a registration with no education level")
	}
	if user.Preferences.Role != "mock_test_student" {
		t.Errorf("Preferences.Role = %q, want the client's persona %q preserved",
			user.Preferences.Role, "mock_test_student")
	}
	// CompletedAt means onboarding finished. With no education level the
	// onboarding data is absent, so it must not claim to be complete.
	if user.Preferences.CompletedAt != nil {
		t.Error("CompletedAt is set with no education level — onboarding is not finished")
	}
}

// And the privilege-escalation attempt is still refused on the thinnest possible
// request, which is the one a fuzzer reaches for first: no education level, and
// the most privileged role in the deployment.
func TestAPrivilegeEscalationWithoutAnEducationLevelIsStillRefused(t *testing.T) {
	svc := registrationTestService(t)
	token := completeRegistration(t, svc, "escalate-thin@example.com", "password123", "superadmin")

	claims, err := utils.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate issued session token: %v", err)
	}
	for _, privileged := range privilegedRoles {
		if strings.EqualFold(claims.Role, privileged) {
			t.Fatalf("issued session carries privileged role %q", claims.Role)
		}
	}

	stored, err := svc.repo.FindUserByEmail("escalate-thin@example.com")
	if err != nil {
		t.Fatalf("load the persisted account: %v", err)
	}
	if stored.Role != "student" {
		t.Fatalf("User.Role = %q — a public registration minted a privileged role", stored.Role)
	}
	// The persona IS preserved verbatim here, which looks alarming and is not:
	// it is display data, it is the same field a student legitimately picks, and
	// middleware.RequireRole never reads it. Asserted so that a future change
	// cannot quietly start sanitizing it and break onboarding while still
	// passing the check above.
	if stored.Preferences == nil || stored.Preferences.Role != "superadmin" {
		t.Errorf("persona = %+v, want it preserved verbatim; sanitizing it would break onboarding and is not what secures this", stored.Preferences)
	}
}
