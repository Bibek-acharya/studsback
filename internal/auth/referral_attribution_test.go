// internal/auth/referral_attribution_test.go
//
// One test per user-creation path, proving that path attributes a referral.
//
// This is the highest-value file in the slice, and the reason is structural rather
// than thoroughness. Attribution is triggered by explicit calls in THIS module, at
// six sites spread over four functions. A site that forgets to call applyAttribution
// is not a bug anything else can catch: the coins package's tests prove attribution
// pays correctly WHEN CALLED, and a missing call sails through all of them while
// every student who signed up through that path silently credits nobody. The
// inverse holds too — a path that attributes when it should not is a fraud route,
// so these tests assert both that the call happens on a genuine signup and that it
// does NOT happen on a login of an existing account.
//
// The attributor is a stub here, because what these tests assert is "this path
// triggers attribution", not "the attribution is recorded correctly". That is proven
// against a real database in internal/coins/referral_pg_test.go, driven through the
// real auth service. Using a stub means these tests need no coin schema and no
// database beyond SQLite, so they run in the ordinary `go test ./...` — which is the
// only way a regression here is caught before a deploy.
//
// The shape of every test is the same, and deliberately uniform:
//
//	act through the real service method  ->  assert the attribution was requested
package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/utils"
)

// errSimulatedCoinOutage stands in for a coin system that cannot answer — a locked
// table, an unmigrated schema, a config read failure. Anything at all.
var errSimulatedCoinOutage = errors.New("coins: simulated attribution outage")

// stubAttributor records every attribution request and can be made to fail, so a
// test can assert both that a path attributes and that a coin-system failure does
// not fail the signup it follows (applyAttribution's contract).
type stubAttributor struct {
	Calls []ReferralSubject
	Err   error
	// Applied is what the stub reports back, so a test can drive the
	// already-attributed branch without a database.
	Applied bool
}

func (a *stubAttributor) ApplyReferral(_ context.Context, subject ReferralSubject) (ReferralAttribution, error) {
	a.Calls = append(a.Calls, subject)
	if a.Err != nil {
		return ReferralAttribution{}, a.Err
	}
	return ReferralAttribution{Applied: a.Applied, Reason: "stub"}, nil
}

// attributedCodes returns the codes this stub was asked to attribute, in order.
func (a *stubAttributor) attributedCodes() []string {
	out := make([]string, 0, len(a.Calls))
	for _, c := range a.Calls {
		out = append(out, c.Code)
	}
	return out
}

// newReferralAuthService builds a Service over an in-memory database with the
// attribution stub wired.
//
// SMTP port 1 is deliberately unreachable, so utils.SendOTPEmail logs the OTP
// rather than reaching a mail server. That is how the OTP tests can drive the real
// verification flow without reaching into private package state, and it is the same
// trick registrationTestService uses.
func newReferralAuthService(t *testing.T) (*Service, *stubAttributor) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &InstitutionUser{}, &ScholarshipProviderUser{},
		&UserSession{}, &InstitutionSubscription{}, &EducationEntry{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	previous := config.AppConfig
	config.AppConfig = &config.Config{
		JWTSecret: "test-secret",
		JWTExpiry: "24h",
		SMTPHost:  "localhost",
		SMTPPort:  "1",
		SMTPUser:  "test@example.com",
		SMTPPass:  "password",
	}
	t.Cleanup(func() { config.AppConfig = previous })

	attributor := &stubAttributor{Applied: true}
	SetReferralAttributor(attributor)
	t.Cleanup(func() { SetReferralAttributor(nil) })

	return NewService(NewRepository(db)), attributor
}

// assertAttributedFor asserts exactly one attribution was requested, for this
// account and this code.
func assertAttributedFor(t *testing.T, a *stubAttributor, kind string, id uint, code, path string) {
	t.Helper()
	if len(a.Calls) == 0 {
		t.Fatalf("user-creation path %s did not attribute: attribution is triggered by "+
			"explicit calls in this module, so a missing one loses the referral with "+
			"nothing else reporting it", path)
	}
	if len(a.Calls) > 1 {
		t.Fatalf("user-creation path %s attributed %d times (%v); a signup attributes once",
			path, len(a.Calls), a.Calls)
	}
	got := a.Calls[0]
	if got.Kind != kind {
		t.Errorf("path %s attributed kind = %q, want %q — the account was created in a "+
			"different table and a User-keyed attribution would record the wrong one", path, got.Kind, kind)
	}
	if got.ID != id {
		t.Errorf("path %s attributed id %d, want %d", path, got.ID, id)
	}
	if got.Code != code {
		t.Errorf("path %s attributed code %q, want %q", path, got.Code, code)
	}
	if got.Path != path {
		t.Errorf("path label = %q, want %q", got.Path, path)
	}
}

// assertNotAttributed asserts the path did not attribute at all, and says why the
// call site matters.
func assertNotAttributed(t *testing.T, a *stubAttributor, because string) {
	t.Helper()
	if len(a.Calls) != 0 {
		t.Fatalf("attribution ran %d time(s) (%v) but should not have: %s",
			len(a.Calls), a.Calls, because)
	}
}

// referralCode is a syntactically valid code for tests. The stub never looks it up,
// so its content is irrelevant — what matters is that the call site passed SOMETHING
// through, and a realistic-shaped string makes a missing pass obvious in a diff.
const referralCode = "K7M2QX9RT4"

// ── paths 1-3: VerifyOTP, which is three paths in one function ────────────────

// completeReferralRegistration drives the genuine public sign-up chain for a
// student — Register, then SendOTP (the mainline "send code again" call), then
// VerifyOTP — and returns the created account.
//
// SendOTP is in the chain deliberately and it is the point of the test rather than
// ceremony. Register stages the pending account WITH the referral code and does not
// mail anything; the frontend then calls /send-otp, which re-stages the entry under a
// fresh code. A re-stage that carried back only the account would drop the referral
// code here — an under-counted referral on the most common path in the product,
// caused by a line that looks like a harmless copy. Driving the real SendOTP is the
// only way to catch it.
//
// The staged entry is read back out of the store and re-staged under a known OTP,
// exactly as registration_role_test.go's completeRegistration does, so what VerifyOTP
// persists is what the public endpoints actually produced.
func completeReferralRegistration(t *testing.T, svc *Service, email, referral string) *User {
	t.Helper()

	if _, err := svc.Register(RegisterRequest{
		Email:          email,
		Password:       "password123",
		FirstName:      "Invitee",
		LastName:       "Person",
		ReferralCode:   referral,
		EducationLevel: "+2",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// The mainline path: the student clicks "verify account" and the frontend asks
	// for the OTP.
	if err := svc.SendOTP(email, "verification"); err != nil {
		t.Fatalf("send otp: %v", err)
	}

	otpType, staged, stagedReferral, found := utils.GetOTPStaged(email)
	if !found {
		t.Fatal("no pending registration after send-otp")
	}
	if stagedReferral != referral {
		t.Fatalf("referral code after send-otp = %q, want %q: send-otp re-stages the "+
			"pending account and must carry the referral code through", stagedReferral, referral)
	}
	utils.StoreOTPWithReferral(email, "424242", otpType, staged, stagedReferral)

	verified, err := svc.VerifyOTP(email, "424242")
	if err != nil {
		t.Fatalf("verify otp: %v", err)
	}
	if verified.Token == "" {
		t.Fatal("verification returned no session token")
	}
	user, ok := verified.User.(User)
	if !ok {
		t.Fatalf("verified account is %T, want User", verified.User)
	}
	return &user
}

// TestPath1VerifyOTPStudentAttributes is path 1 of 6: an OTP registration that
// creates a row in `users`.
func TestPath1VerifyOTPStudentAttributes(t *testing.T) {
	svc, attributor := newReferralAuthService(t)
	user := completeReferralRegistration(t, svc, "path1@example.com", referralCode)

	assertAttributedFor(t, attributor, ReferralSubjectUser, user.ID, referralCode, "verify_otp")
}

// TestPath2VerifyOTPInstitutionAttributes is path 2 of 6. It writes to
// institution_users, NOT users.
//
// This is the test that would have caught the §10.1 under-specification. A
// User-keyed attribution cannot express "this row was created in
// institution_users", so with the spec's shape this path either attributes against
// the wrong table's id space or does not attribute at all — and a student who invites
// a college is one of the most natural referrals there is.
func TestPath2VerifyOTPInstitutionAttributes(t *testing.T) {
	svc, attributor := newReferralAuthService(t)

	if _, err := svc.InstitutionRegister(InstitutionRegisterRequest{
		InstitutionName: "Kathmandu Model College",
		Email:           "path2@example.com",
		ReferralCode:    referralCode,
		ContactNumber:   "9800000000",
		Province:        "Bagmati",
		District:        "Kathmandu",
	}); err != nil {
		t.Fatalf("institution register: %v", err)
	}
	if err := svc.SendOTP("path2@example.com", "verification"); err != nil {
		t.Fatalf("send otp: %v", err)
	}
	otpType, staged, stagedReferral, found := utils.GetOTPStaged("path2@example.com")
	if !found {
		t.Fatal("no pending institution registration")
	}
	utils.StoreOTPWithReferral("path2@example.com", "424242", otpType, staged, stagedReferral)

	verified, err := svc.VerifyOTP("path2@example.com", "424242")
	if err != nil {
		t.Fatalf("verify otp: %v", err)
	}
	inst, ok := verified.User.(InstitutionUser)
	if !ok {
		t.Fatalf("verified account is %T, want InstitutionUser", verified.User)
	}
	assertAttributedFor(t, attributor, ReferralSubjectInstitution, inst.ID, referralCode, "verify_otp_institution")
}

// TestPath3VerifyOTPProviderAttributes is path 3 of 6, in
// scholarship_provider_users.
func TestPath3VerifyOTPProviderAttributes(t *testing.T) {
	svc, attributor := newReferralAuthService(t)

	if _, err := svc.ScholarshipProviderRegister(ScholarshipProviderRegisterRequest{
		ProviderName:       "Path3 Scholarship Trust",
		RegistrationNumber: "REG-PATH3-001",
		Email:              "path3@example.com",
		ReferralCode:       referralCode,
	}); err != nil {
		t.Fatalf("provider register: %v", err)
	}
	if err := svc.SendOTP("path3@example.com", "verification"); err != nil {
		t.Fatalf("send otp: %v", err)
	}
	otpType, staged, _, found := utils.GetOTPStaged("path3@example.com")
	if !found {
		t.Fatal("no pending provider registration")
	}
	utils.StoreOTPWithReferral("path3@example.com", "424242", otpType, staged, referralCode)

	verified, err := svc.VerifyOTP("path3@example.com", "424242")
	if err != nil {
		t.Fatalf("verify otp: %v", err)
	}
	provider, ok := verified.User.(ScholarshipProviderUser)
	if !ok {
		t.Fatalf("verified account is %T, want ScholarshipProviderUser", verified.User)
	}
	assertAttributedFor(t, attributor, ReferralSubjectProvider, provider.ID, referralCode, "verify_otp_provider")
}

// ── paths 4-6: the three Google callbacks ─────────────────────────────────────

// TestPath4GoogleLoginOrRegisterAttributes is path 4 of 6.
//
// picture is empty so the flow stays on the creation branch without a network call
// to Google's CDN, which is what downloadAndSavePicture would otherwise attempt.
func TestPath4GoogleLoginOrRegisterAttributes(t *testing.T) {
	svc, attributor := newReferralAuthService(t)

	res, err := svc.GoogleLoginOrRegister("google-path4", "path4@example.com",
		"Invitee", "Person", "", referralCode)
	if err != nil {
		t.Fatalf("google login: %v", err)
	}
	assertAttributedFor(t, attributor, ReferralSubjectUser, res.UserID, referralCode, "google_login")
}

// TestPath5InstitutionGoogleAttributes is path 5 of 6, in institution_users.
func TestPath5InstitutionGoogleAttributes(t *testing.T) {
	svc, attributor := newReferralAuthService(t)

	inst, _, err := svc.InstitutionGoogleLoginOrRegister("google-path5",
		"path5@example.com", "Path Five College", referralCode)
	if err != nil {
		t.Fatalf("institution google login: %v", err)
	}
	assertAttributedFor(t, attributor, ReferralSubjectInstitution, inst.ID, referralCode, "google_institution")
}

// TestPath6ProviderGoogleAttributes is path 6 of 6, in
// scholarship_provider_users.
func TestPath6ProviderGoogleAttributes(t *testing.T) {
	svc, attributor := newReferralAuthService(t)

	provider, _, err := svc.ScholarshipProviderGoogleLoginOrRegister("google-path6",
		"path6@example.com", "Path Six Trust", referralCode)
	if err != nil {
		t.Fatalf("provider google login: %v", err)
	}
	assertAttributedFor(t, attributor, ReferralSubjectProvider, provider.ID, referralCode, "google_provider")
}

// ── the paths that must NOT attribute ─────────────────────────────────────────

// A Google sign-in by an account that ALREADY EXISTS must not attribute, even when
// the request carries a code.
//
// This is fraud mechanism 3 in 05-economy-and-fraud.md §3.2 — "A creates B, refers
// A→B" is the self-referral case, and the generalisation is an established account
// re-entering an invite code to try to be credited on both sides. UNIQUE
// (referred_kind, referred_user_id) is the constraint that ultimately refuses it.
//
// But the constraint is the backstop, not the design: attributing here would send
// every Google sign-in of an already-referred student at the coin system, on every
// login, forever, which is a coin-system query on the hottest read path in the app
// and would show up as an unexplained referral-attempt flood in the logs.
func TestGoogleSignInOfAnExistingAccountDoesNotAttribute(t *testing.T) {
	svc, attributor := newReferralAuthService(t)

	first, err := svc.GoogleLoginOrRegister("google-existing", "existing@example.com",
		"Invitee", "Person", "", referralCode)
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}
	if len(attributor.Calls) != 1 {
		t.Fatalf("the creating sign-in attributed %d times, want 1", len(attributor.Calls))
	}
	attributor.Calls = nil

	// Same Google id, same account, and a code — this is the fraud attempt.
	again, err := svc.GoogleLoginOrRegister("google-existing", "existing@example.com",
		"Invitee", "Person", "", "ZZZZZZZZZZ")
	if err != nil {
		t.Fatalf("second sign-in: %v", err)
	}
	if again.UserID != first.UserID {
		t.Fatalf("sign-in created a second account: %d then %d", first.UserID, again.UserID)
	}
	assertNotAttributed(t, attributor,
		"an existing account re-entering a code is the two-sided referral fraud the "+
			"UNIQUE(referred_kind, referred_user_id) constraint exists to refuse")
}

// The same for the other two Google paths, because each has its own creation branch
// and a fix applied to one is not applied to the others. These are short because the
// property is identical; they exist so a future edit to either path that moves the
// call site out of the creation branch fails here.
func TestInstitutionAndProviderGoogleSignInOfExistingAccountsDoNotAttribute(t *testing.T) {
	t.Run("institution", func(t *testing.T) {
		svc, attributor := newReferralAuthService(t)
		if _, _, err := svc.InstitutionGoogleLoginOrRegister("google-inst-existing",
			"inst-existing@example.com", "Existing College", referralCode); err != nil {
			t.Fatalf("first sign-in: %v", err)
		}
		attributor.Calls = nil

		if _, _, err := svc.InstitutionGoogleLoginOrRegister("google-inst-existing",
			"inst-existing@example.com", "Existing College", "ZZZZZZZZZZ"); err != nil {
			t.Fatalf("second sign-in: %v", err)
		}
		assertNotAttributed(t, attributor, "an existing institution re-entering a code")
	})

	t.Run("provider", func(t *testing.T) {
		svc, attributor := newReferralAuthService(t)
		if _, _, err := svc.ScholarshipProviderGoogleLoginOrRegister("google-prov-existing",
			"prov-existing@example.com", "Existing Trust", referralCode); err != nil {
			t.Fatalf("first sign-in: %v", err)
		}
		attributor.Calls = nil

		if _, _, err := svc.ScholarshipProviderGoogleLoginOrRegister("google-prov-existing",
			"prov-existing@example.com", "Existing Trust", "ZZZZZZZZZZ"); err != nil {
			t.Fatalf("second sign-in: %v", err)
		}
		assertNotAttributed(t, attributor, "an existing provider re-entering a code")
	})
}

// The overwhelming majority of signups carry no code. applyAttribution returns
// before calling the port for those, which keeps a coin-system query off the signup
// path for every signup that never came from an invite — the same reasoning
// profile_award.go gives for not calling the award on Login.
func TestSignupWithNoCodeDoesNotReachTheCoinSystem(t *testing.T) {
	svc, attributor := newReferralAuthService(t)

	if _, err := svc.GoogleLoginOrRegister("google-nocode", "nocode@example.com",
		"Ordinary", "Student", "", ""); err != nil {
		t.Fatalf("google login: %v", err)
	}
	completeReferralRegistration(t, svc, "nocode-otp@example.com", "")

	assertNotAttributed(t, attributor,
		"a signup with no code has nothing to attribute, and calling the coin system "+
			"for it would put a database round trip on the main signup path")
}

// A coin-system failure must not fail the signup. The account is worth more than the
// referral, and a locked table or an unmigrated schema must not stop a student
// creating an account — the argument applyAttribution makes, asserted here.
func TestAttributionFailureDoesNotFailTheSignup(t *testing.T) {
	svc, attributor := newReferralAuthService(t)
	attributor.Err = errSimulatedCoinOutage

	res, err := svc.GoogleLoginOrRegister("google-outage", "outage@example.com",
		"Invitee", "Person", "", referralCode)
	if err != nil {
		t.Fatalf("a coin-system outage failed the signup: %v", err)
	}
	if res.Token == "" {
		t.Fatal("no session token issued")
	}
	// And the account really exists — a signup that returned an error without
	// rolling anything back would be worse than the one being tested for.
	user, err := svc.repo.FindUserByEmail("outage@example.com")
	if err != nil {
		t.Fatalf("the account was not created: %v", err)
	}
	if user.ID != res.UserID {
		t.Fatalf("created account %d does not match the issued session's user %d", user.ID, res.UserID)
	}
	if len(attributor.Calls) != 1 {
		t.Fatalf("attribution ran %d times, want 1 (the attempt must be visible in the logs)",
			len(attributor.Calls))
	}
}

// ── the enumeration itself ────────────────────────────────────────────────────

// TestUserCreationPathsAreAllAttributed is the test that keeps the enumeration
// honest, and it is the closest thing here to a guard against the whole class of bug
// this file exists for.
//
// It reads this module's own source and counts the INSERT sites into the three
// account tables. Every one must either call applyAttribution or be on the explicit
// exclusion list with a reason — and the list has exactly two entries, both of which
// are staff-facing account creation with no invite link to capture.
//
// So a future fifth account-creation method shows up as a source change that fails
// this test rather than as a signup route that quietly credits nobody. That is the
// only mechanism by which "a path not on the list is a bug" can be enforced rather
// than asserted: the plan calls four paths a high-value test, but the list is only
// as good as its ability to notice a fifth.
//
// Source-scanning rather than reflection because the count is a property of the code
// and reflection cannot see it: a call to applyAttribution is a statement in a
// function body, and there is no runtime handle on "did this function attribute".
func TestUserCreationPathsAreAllAttributed(t *testing.T) {
	src, err := readOwnSource("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}

	// Every repository call that WRITES an account row, with how many times each
	// appears in service.go as it stands.
	//
	//	users                       VerifyOTP, GoogleLoginOrRegister, SuperadminRegister   3
	//	institution_users           VerifyOTP, InstitutionGoogleLoginOrRegister,
	//	                            CreateInstitution (superadmin console)                  3
	//	scholarship_provider_users  VerifyOTP, ScholarshipProviderGoogleLoginOrRegister   2
	//
	// ClaimRegister is NOT in this table and that is correct rather than an
	// omission: it does not insert anything, it stages an InstitutionUser in the OTP
	// store, and VerifyOTP's institution branch writes the row. So the four staging
	// routes (Register, InstitutionRegister, ScholarshipProviderRegister,
	// ClaimRegister) reach creation indirectly, which is exactly why this scan of
	// the CreateX calls is not sufficient on its own and the staging routes are
	// asserted separately below.
	//
	// The multiplicity is the point. A count of 1 for any of these would mean a
	// creation path had been merged or renamed; a count of 3 or 2 would mean a new
	// one appeared. Either way this fails and asks for the enumeration in
	// internal/auth/referral.go to be updated, which is the only way "a path not on
	// the list is a bug" gets enforced rather than merely asserted.
	//
	// Note that CreateInstitution reaches the repository through the same
	// s.repo.CreateInstitutionUser call as everyone else, so it is counted here and
	// excluded by FUNCTION below rather than being invisible to this scan. An
	// earlier draft of this test assumed a creation path would use a different
	// receiver or an inline struct, which is how it came to undercount by one and
	// pass while a path was unattributed — a good illustration of why the
	// exclusion is asserted per-function below rather than inferred from the text.
	want := map[string]int{
		"s.repo.CreateUser(":                    3,
		"s.repo.CreateInstitutionUser(":         3,
		"s.repo.CreateScholarshipProviderUser(": 2,
	}
	for pattern, wantN := range want {
		if got := strings.Count(src, pattern); got != wantN {
			t.Errorf("%s appears %d times, want %d. A new account-creation site "+
				"in service.go is either an unattributed signup or needs the "+
				"enumeration in internal/auth/referral.go updated to say so",
				pattern, got, wantN)
		}
	}

	// One applyAttribution per attributed creation site: six.
	attributionCalls := strings.Count(src, "s.applyAttribution(")
	if attributionCalls != 6 {
		t.Errorf("service.go calls s.applyAttribution %d times, want 6 — one per "+
			"attributed user-creation path. See internal/auth/referral.go for the list",
			attributionCalls)
	}

	// The four staging routes must all stage the code. These are the paths that
	// reach a creation site indirectly, by way of the OTP store rather than by
	// calling CreateX themselves — which is precisely why a scan of the CreateX
	// calls alone would have found them and why they need their own assertion.
	for _, fn := range []string{
		"func (s *Service) Register(",
		"func (s *Service) InstitutionRegister(",
		"func (s *Service) ScholarshipProviderRegister(",
		"func (s *Service) ClaimRegister(",
	} {
		body := sourceAfter(src, fn)
		if body == "" {
			t.Errorf("%s no longer exists; internal/auth/referral.go lists it as a "+
				"code-staging signup route", fn)
			continue
		}
		if !strings.Contains(body, "StoreOTPWithReferral(") {
			t.Errorf("%s stages the pending account with utils.StoreOTP rather than "+
				"StoreOTPWithReferral, so the invite code is dropped on this route. "+
				"Four registration routes reach VerifyOTP this way and all four "+
				"need it", fn)
		}
	}

	// The documented exclusions, asserted to still exist and still not attribute.
	// If one is ever removed this fails and forces the enumeration to be revisited,
	// rather than leaving a comment describing a method that no longer exists.
	for _, fn := range []string{"func (s *Service) CreateInstitution(", "func (s *Service) SuperadminRegister("} {
		if !strings.Contains(src, fn) {
			t.Errorf("%s no longer exists; internal/auth/referral.go documents it as a "+
				"deliberately unattributed account-creation path and that list must be updated", fn)
			continue
		}
		if strings.Contains(sourceAfter(src, fn), "applyAttribution(") {
			t.Errorf("%s now calls applyAttribution. If that is deliberate — a staff "+
				"tool that can attribute a referral is a legitimate thing to build — "+
				"update the exclusion list in internal/auth/referral.go and say why",
				fn)
		}
	}
}

// sourceAfter returns the text from marker to the end of that function, so a check
// can ask "does this function attribute" without parsing Go.
func sourceAfter(src, marker string) string {
	i := strings.Index(src, marker)
	if i < 0 {
		return ""
	}
	rest := src[i:]
	// A function body ends at the next top-level declaration, which in this file is
	// always a line starting at column zero with "func " or a type/var/const.
	if next := strings.Index(rest[1:], "\nfunc "); next >= 0 {
		return rest[:next+1]
	}
	return rest
}

// readOwnSource reads a file from this package's own directory.
//
// Deliberately reading source rather than reflecting over the compiled package: the
// property under test is about statements in function bodies, and the only way to
// see them is to read them. The path is derived from the test's own working
// directory, which `go test` sets to the package's source directory — so this does
// not depend on a relative path from the repository root, and does not read any file
// outside this package.
func readOwnSource(name string) (string, error) {
	// `go test` runs with the package's source directory as the working directory,
	// so this is a same-directory read and not a path relative to the repository
	// root — which keeps it correct under `go test ./...`, under an IDE, and under
	// `go test` from inside the package.
	raw, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
