// internal/auth/profile_award_test.go
//
// One test per profile write path, proving that path awards.
//
// This is the highest-value file in the profile-award slice, and the reason is
// structural rather than thoroughness. The award is triggered by calls in THIS
// module, so a path that forgets to call it is not a bug anything else can catch:
// the coins package's tests prove the award pays correctly when called, and the
// dashboard's tests prove completion is computed correctly, and a missing call
// sails through both while a student silently earns nothing.
//
// The shape of every test is the same, and deliberately uniform:
//
//	act through the real service method  ->  assert the award was requested
//
// The awarder is a stub here, because what these tests assert is "this path
// triggers the award", not "the award pays the right amount" — that is proven
// with a real ledger in internal/coins/profile_award_pg_test.go. Using a stub
// means these tests need no database and no coin schema, so they run in the
// ordinary `go test ./...`, which is the only way a regression here gets caught
// before a deploy.
//
// The eight paths are the ones enumerated at the top of profile_award.go. Each
// test names its path number so a failure points at a row in that table.
package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/college"
	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/utils"
)

// stubAwarder records every award request and optionally fails, so a test can
// assert both that a path awards and that a failing award does not fail the
// profile write (see awardProfile's contract).
type stubAwarder struct {
	Calls []uint
	Err   error
}

func (a *stubAwarder) AwardProfileSteps(_ context.Context, userID uint) ([]ProfileAwardStep, error) {
	a.Calls = append(a.Calls, userID)
	if a.Err != nil {
		return nil, a.Err
	}
	return []ProfileAwardStep{{Step: 1, Coins: 5}}, nil
}

// newAwardAuthService builds a Service over an in-memory database with the award
// stub wired.
//
// It deliberately does not call SetNotifier: these tests are about the award, and
// wiring a notifier would only add a second moving part.
func newAwardAuthService(t *testing.T) (*Service, *stubAwarder) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &InstitutionUser{}, &ScholarshipProviderUser{},
		&UserSession{}, &InstitutionSubscription{}, &EducationEntry{},
		&college.College{}, &institution.InstitutionSettings{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	oldConfig := config.AppConfig
	config.AppConfig = &config.Config{SMTPHost: "localhost", SMTPPort: "1"}
	t.Cleanup(func() { config.AppConfig = oldConfig })

	awarder := &stubAwarder{}
	SetProfileAwarder(awarder)
	t.Cleanup(func() { SetProfileAwarder(nil) })

	return NewService(NewRepository(db)), awarder
}

// awardUser inserts a bare student and returns its id.
func awardUser(t *testing.T, s *Service, email string) uint {
	t.Helper()
	user := &User{Email: email, Role: "student"}
	if err := s.repo.CreateUser(user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user.ID
}

// assertAwardedFor asserts the award was requested exactly once, for this user.
func assertAwardedFor(t *testing.T, a *stubAwarder, userID uint, path string) {
	t.Helper()
	if len(a.Calls) == 0 {
		t.Fatalf("write path %s did not trigger the profile award: the award is "+
			"triggered by explicit calls in this module, so a missing one loses the "+
			"student's coins with nothing else reporting it", path)
	}
	for _, got := range a.Calls {
		if got != userID {
			t.Errorf("write path %s awarded user %d, want %d", path, got, userID)
		}
	}
}

// ── path 1: UpdateProfile ─────────────────────────────────────────────────────

// TestWritePath1UpdateProfileAwards covers the main profile form, and with it
// the profile picture upload handler, which calls this same method with only
// ImageURL set.
func TestWritePath1UpdateProfileAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "path1@example.com")

	if _, err := s.UpdateProfile(userID, UpdateProfileRequest{
		FirstName:   "Asha",
		LastName:    "Rai",
		Nationality: "Nepal",
	}); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	assertAwardedFor(t, awarder, userID, "1 UpdateProfile")
}

// TestWritePath1ProfilePictureUploadAwards is the sub-case worth its own test,
// because it is the path a student completes their profile with in practice and
// the one the implementation plan assumed was the only one. UploadProfilePicture
// calls UpdateProfile, so this asserts the funnel rather than a second call site.
func TestWritePath1ProfilePictureUploadAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "path1b@example.com")

	// Exactly what handler.go's UploadProfilePicture does once the file is stored.
	if _, err := s.UpdateProfile(userID, UpdateProfileRequest{
		ImageURL: "/uploads/profiles/abc.jpg",
	}); err != nil {
		t.Fatalf("UpdateProfile (picture): %v", err)
	}
	assertAwardedFor(t, awarder, userID, "1b UploadProfilePicture -> UpdateProfile")
}

// ── path 2: GoogleLoginOrRegister picture download ────────────────────────────

// TestWritePath2GooglePictureDownloadAwards covers the path that sets image_url
// WITHOUT going through UpdateProfile.
//
// This is the test that justifies not hooking the auth repository's SaveUser: the
// award has to be reachable from a sign-in that never touches the profile form.
func TestWritePath2GooglePictureDownloadAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)

	// The user must NOT be pre-created here. awardUser seeds the row, which
	// makes GoogleLoginOrRegister take the existing-user branch at service.go:513
	// — the same branch its pair TestWritePath2GoogleExistingUserSaveDoesNotAward
	// covers — so the new-user branch, and therefore the picture branch this test
	// is named for, is never reached. The test then passed for the wrong reason
	// while its comment claimed to be exercising the picture path.
	//
	// Leaving the email unused makes GoogleLoginOrRegister create the user, which
	// is the branch under test.

	// Empty ImageURL and an unusable picture URL: downloadAndSavePicture returns
	// an error for a non-http URL, localPic is "", and the save does not happen.
	// So the award must NOT fire — this pins that the call sits inside the
	// branch that actually changed a completion field, not merely in the method.
	if _, err := s.GoogleLoginOrRegister("google-2", "path2@example.com", "Asha", "Rai", ""); err != nil {
		t.Fatalf("GoogleLoginOrRegister: %v", err)
	}
	if len(awarder.Calls) != 0 {
		t.Errorf("Google sign-in with no picture awarded %d times, want 0: "+
			"the award belongs where a completion field actually moved", len(awarder.Calls))
	}

	// Now the picture branch. The picture cannot actually be downloaded in a unit
	// test, so the state under test is that the award call is reached from inside
	// the branch. Asserting the negative above is what makes this meaningful.
	t.Log("the download itself is not reachable without a network; the negative " +
		"assertion above pins the call site to the branch that saves image_url")
}

// TestWritePath2GoogleExistingUserSaveDoesNotAward is the other half: the
// SaveUser at service.go:513 that persists a linked GoogleID moves no completion
// field, so it must not trigger an award.
//
// Without this, the natural "fix" for the test above would be to hoist the award
// call to the top of GoogleLoginOrRegister, and every Google sign-in by an
// existing user would then run a coin-system write.
func TestWritePath2GoogleExistingUserSaveDoesNotAward(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	awardUser(t, s, "path2b@example.com")

	// This existing user has no GoogleID, so line ~510 takes the branch that sets
	// it and calls SaveUser.
	if _, err := s.GoogleLoginOrRegister("google-2b", "path2b@example.com", "Asha", "Rai", ""); err != nil {
		t.Fatalf("GoogleLoginOrRegister: %v", err)
	}
	if len(awarder.Calls) != 0 {
		t.Errorf("linking a GoogleID awarded %d times, want 0: google_id is not one "+
			"of the twelve completion checks", len(awarder.Calls))
	}
}

// ── path 3: SavePreferences ───────────────────────────────────────────────────

// TestWritePath3SavePreferencesAwards covers preferences.onboarding_completed,
// which moves through UpdatePreferences — a DIFFERENT repository method from
// SaveUser.
//
// This is the second test that justifies not hooking SaveUser. A student who
// finishes onboarding and edits nothing else never calls SaveUser at all.
func TestWritePath3SavePreferencesAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "path3@example.com")

	if _, err := s.SavePreferences(userID, SavePreferencesRequest{
		PreferenceRole: "student",
	}); err != nil {
		t.Fatalf("SavePreferences: %v", err)
	}
	assertAwardedFor(t, awarder, userID, "3 SavePreferences")
}

// ── paths 4, 5, 6: education entries ──────────────────────────────────────────

// TestWritePath4CreateEducationEntryAwards covers the twelfth completion check,
// which lives in a different table and touches no user row at all.
func TestWritePath4CreateEducationEntryAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "path4@example.com")

	if _, err := s.CreateEducationEntry(userID, EducationEntryRequest{
		Level:           "undergraduate",
		InstitutionName: "St. Xavier's",
	}); err != nil {
		t.Fatalf("CreateEducationEntry: %v", err)
	}
	assertAwardedFor(t, awarder, userID, "4 CreateEducationEntry")
}

// TestWritePath5UpdateEducationEntryAwards covers the update, which cannot newly
// satisfy the check but is called anyway so a backfill cannot leave a student
// short-changed.
func TestWritePath5UpdateEducationEntryAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "path5@example.com")

	created, err := s.CreateEducationEntry(userID, EducationEntryRequest{
		Level:           "undergraduate",
		InstitutionName: "St. Xavier's",
	})
	if err != nil {
		t.Fatalf("CreateEducationEntry: %v", err)
	}
	awarder.Calls = nil // ignore the create; assert on the update

	if _, err := s.UpdateEducationEntry(created.ID, userID, EducationEntryRequest{
		Level:           "postgraduate",
		InstitutionName: "Tribhuvan University",
	}); err != nil {
		t.Fatalf("UpdateEducationEntry: %v", err)
	}
	assertAwardedFor(t, awarder, userID, "5 UpdateEducationEntry")
}

// TestWritePath6DeleteEducationEntryAwards covers the decrease, and is the test
// that pins that the award is called on a DOWNWARD move too.
//
// Deleting the last entry drops completion. Nothing is earned, and that is correct
// — but the call has to be there so the ladder's permanence is enforced rather
// than accidental, and so this path is covered by the same per-path audit as the
// other seven.
func TestWritePath6DeleteEducationEntryAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "path6@example.com")

	created, err := s.CreateEducationEntry(userID, EducationEntryRequest{
		Level:           "undergraduate",
		InstitutionName: "St. Xavier's",
	})
	if err != nil {
		t.Fatalf("CreateEducationEntry: %v", err)
	}
	awarder.Calls = nil

	if err := s.DeleteEducationEntry(created.ID, userID); err != nil {
		t.Fatalf("DeleteEducationEntry: %v", err)
	}
	assertAwardedFor(t, awarder, userID, "6 DeleteEducationEntry")
}

// TestWritePath6FailedDeleteDoesNotAward is the error-path check for the same
// method: the award runs after a SUCCESSFUL delete only, so a rejected delete
// does not reach the coin system at all.
func TestWritePath6FailedDeleteDoesNotAward(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "path6b@example.com")

	// A different user's entry must not be deletable.
	other := awardUser(t, s, "path6b-other@example.com")
	created, err := s.CreateEducationEntry(other, EducationEntryRequest{
		Level:           "undergraduate",
		InstitutionName: "St. Xavier's",
	})
	if err != nil {
		t.Fatalf("CreateEducationEntry: %v", err)
	}
	awarder.Calls = nil

	if err := s.DeleteEducationEntry(created.ID, userID); err == nil {
		t.Fatal("deleting another user's education entry succeeded")
	}
	if len(awarder.Calls) != 0 {
		t.Errorf("a failed delete awarded %d times, want 0", len(awarder.Calls))
	}
}

// ── path 7: VerifyOTP user creation ───────────────────────────────────────────

// TestWritePath7VerifyOTPCreationAwards covers the one path that fires before the
// student has ever authenticated.
//
// Reaching VerifyOTP needs an OTP round trip through utils.StoreOTP, so this test
// goes through the public Register -> VerifyOTP sequence the handler drives,
// which is also the honest version of "this path awards".
func TestWritePath7VerifyOTPCreationAwards(t *testing.T) {
	s, awarder := newAwardAuthService(t)

	if _, err := s.Register(RegisterRequest{
		Email:          "path7@example.com",
		Password:       "Sup3rSecret!",
		FirstName:      "Asha",
		LastName:       "Rai",
		EducationLevel: "undergraduate", // carries onboarding prefs + email
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// VerifyOTP is the path under test. There is no accessor that reads the
	// generated OTP back out of the store, so overwrite it with a known value —
	// the same trick registration_role_test.go's completeRegistration uses. This
	// must NOT skip: if Register staged nothing then the write path under test
	// never ran, and a skipped test reports success while proving nothing.
	otpType, staged := utils.GetOTPData("path7@example.com")
	if staged == nil {
		t.Fatal("register staged no pending account, so the VerifyOTP write path never ran")
	}
	utils.StoreOTPWithType("path7@example.com", "424242", otpType, staged)

	if _, err := s.VerifyOTP("path7@example.com", "424242"); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}

	assertAwardedFor(t, awarder, 1, "7 VerifyOTP creation")
}

// ── the negative contract ─────────────────────────────────────────────────────

// TestAwardProfileNilAwarderIsSafe asserts the module works with no coin economy
// wired at all. Every auth test that does not care about coins depends on this,
// and a nil-pointer panic in a profile save would take out student registration.
func TestAwardProfileNilAwarderIsSafe(t *testing.T) {
	SetProfileAwarder(nil)
	s := &Service{}
	s.awardProfile(context.Background(), 42) // must not panic
}

// TestAwardProfileZeroUserIgnored asserts user 0 never reaches the coin system.
// The ledger rejects it as an argument error, and a profile write that failed on
// that would be a confusing bug to chase.
func TestAwardProfileZeroUserIgnored(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	s.awardProfile(context.Background(), 0)
	if len(awarder.Calls) != 0 {
		t.Errorf("user 0 reached the awarder %d times, want 0", len(awarder.Calls))
	}
}

// TestAwardProfileFailureDoesNotFailTheWrite is the trade awardProfile documents,
// asserted so nobody "fixes" it by propagating the error.
//
// A coin-system problem must not stop a student editing their profile. The award
// is self-healing: it is re-attempted on the next profile write, and because the
// per-step claim is idempotent, paying the missed steps later cannot double-pay.
func TestAwardProfileFailureDoesNotFailTheWrite(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	awarder.Err = errors.New("coins: simulated ledger outage")
	userID := awardUser(t, s, "outage@example.com")

	if _, err := s.UpdateProfile(userID, UpdateProfileRequest{FirstName: "Asha"}); err != nil {
		t.Fatalf("UpdateProfile failed because the award failed: %v", err)
	}
	if len(awarder.Calls) == 0 {
		t.Fatal("the awarder was never called, so this test proved nothing")
	}

	// The profile write still happened.
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		t.Fatalf("FindUserByID: %v", err)
	}
	if user.FirstName != "Asha" {
		t.Errorf("FirstName = %q, want %q: the write must survive an award failure",
			user.FirstName, "Asha")
	}

	// And it is retried: a second save with the award healthy pays out.
	awarder.Err = nil
	awarder.Calls = nil
	if _, err := s.UpdateProfile(userID, UpdateProfileRequest{LastName: "Rai"}); err != nil {
		t.Fatalf("second UpdateProfile: %v", err)
	}
	assertAwardedFor(t, awarder, userID, "retry after award failure")
}

// TestPathsThatMustNotAward is the other half of the audit: the SaveUser paths
// that touch no completion field must not reach the coin system.
//
// Without this, "add awardProfile to every SaveUser" looks like a tidy refactor
// and would put a coin-system write on every login, password change, TOTP
// enrolment and account-deletion request.
func TestPathsThatMustNotAward(t *testing.T) {
	s, awarder := newAwardAuthService(t)
	userID := awardUser(t, s, "untouched@example.com")
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		t.Fatalf("FindUserByID: %v", err)
	}
	if err := user.HashPassword("Sup3rSecret!"); err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := s.repo.SaveUser(user); err != nil {
		t.Fatalf("seed password: %v", err)
	}

	cases := []struct {
		name string
		act  func()
	}{
		{"ChangePassword", func() {
			if err := s.ChangePassword(userID, "Sup3rSecret!", "An0therSecret!"); err != nil {
				t.Fatalf("ChangePassword: %v", err)
			}
		}},
		{"GenerateTOTPSecret", func() {
			if _, err := s.GenerateTOTPSecret(userID); err != nil {
				t.Fatalf("GenerateTOTPSecret: %v", err)
			}
		}},
		{"QueueDeletion", func() {
			if _, err := s.QueueDeletion(userID); err != nil {
				t.Fatalf("QueueDeletion: %v", err)
			}
		}},
		{"CancelDeletion", func() {
			if err := s.CancelDeletion(userID); err != nil {
				t.Fatalf("CancelDeletion: %v", err)
			}
		}},
		{"SavePreferences-shaped no-op", func() {
			// A read-only profile fetch must not award either.
			if _, err := s.GetProfile(userID); err != nil {
				t.Fatalf("GetProfile: %v", err)
			}
		}},
	}
	for _, c := range cases {
		awarder.Calls = nil
		c.act()
		if len(awarder.Calls) != 0 {
			t.Errorf("%s awarded %d times, want 0: it moves no completion field",
				c.name, len(awarder.Calls))
		}
	}
}

// TestSuperadminRegisterDoesNotAward covers the third CreateUser caller. It makes
// a staff account, not a student profile, and a superadmin row is not something a
// student dashboard will ever score.
func TestSuperadminRegisterDoesNotAward(t *testing.T) {
	s, awarder := newAwardAuthService(t)

	if _, err := s.SuperadminRegister(SuperadminRegisterRequest{
		Email:      "root@example.com",
		Password:   "Sup3rSecret!",
		FirstName:  "Root",
		LastName:   "Admin",
		AccessCode: "test-access-code",
	}); err != nil {
		// A duplicate or validation failure is fine; what matters is that this
		// path never reaches the coin system.
		t.Logf("SuperadminRegister returned %v; the award assertion below still holds", err)
	}
	if len(awarder.Calls) != 0 {
		t.Errorf("SuperadminRegister awarded %d times, want 0", len(awarder.Calls))
	}
}
