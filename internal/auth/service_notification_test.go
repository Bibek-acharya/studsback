package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"

	"studsphere/backend/internal/college"
	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/utils"
)

type captureNotifier struct {
	Last     *notification.NotifyRequest
	Calls    []notification.NotifyRequest
	Roles    [][]string
	Audience []notification.Ref
}

func (c *captureNotifier) Notify(_ context.Context, req notification.NotifyRequest) error {
	c.Last = &req
	c.Calls = append(c.Calls, req)
	return nil
}

func (c *captureNotifier) NotifyTx(ctx context.Context, _ *gorm.DB, req notification.NotifyRequest) error {
	return c.Notify(ctx, req)
}

func (c *captureNotifier) ForRoles(_ context.Context, roles ...string) ([]notification.Ref, error) {
	c.Roles = append(c.Roles, roles)
	return c.Audience, nil
}

// assertTemplates enforces the missingkey=error data contract: every key
// referenced by the registry's Title/Body templates must be present in Data.
func assertTemplates(t *testing.T, req notification.NotifyRequest) {
	t.Helper()
	def, ok := notification.Registry[req.EventKey]
	if !ok {
		t.Fatalf("unknown event key %q", req.EventKey)
	}
	if _, err := notification.ResolveTemplate(def.TitleTpl, req.Data); err != nil {
		t.Errorf("title template for %s: %v", req.EventKey, err)
	}
	if _, err := notification.ResolveTemplate(def.BodyTpl, req.Data); err != nil {
		t.Errorf("body template for %s: %v", req.EventKey, err)
	}
}

func newAuthNotificationService(t *testing.T) (*Service, *captureNotifier) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &InstitutionUser{}, &ScholarshipProviderUser{}, &UserSession{}, &InstitutionSubscription{}, &college.College{}, &institution.InstitutionSettings{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	oldConfig := config.AppConfig
	config.AppConfig = &config.Config{
		SMTPHost: "localhost",
		SMTPPort: "1",
		SMTPUser: "test@example.com",
		SMTPPass: "password",
	}
	notif := &captureNotifier{}
	SetNotifier(notif)
	t.Cleanup(func() {
		config.AppConfig = oldConfig
		SetNotifier(nil)
	})
	return NewService(NewRepository(db)), notif
}

func assertSingleRecipient(t *testing.T, notif *captureNotifier, eventKey string, want notification.Ref) {
	t.Helper()
	if notif.Last == nil || notif.Last.EventKey != eventKey {
		t.Fatalf("expected %s, got %+v", eventKey, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != want {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestApproveScholarshipProviderNotifiesProvider(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	provider := &ScholarshipProviderUser{
		ProviderName:       "Scholarship Nepal",
		RegistrationNumber: "REG-APPROVE-1",
		Email:              "provider@example.com",
		Status:             "pending",
		Role:               "scholarship_provider",
	}
	if err := svc.repo.CreateScholarshipProviderUser(provider); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	if err := svc.ApproveScholarshipProvider(provider.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventAccountApproved, notification.Ref{Type: "provider", ID: provider.ID})
}

func TestRejectScholarshipProviderNotifiesProvider(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	provider := &ScholarshipProviderUser{
		ProviderName:       "Scholarship Nepal",
		RegistrationNumber: "REG-REJECT-1",
		Email:              "provider-reject@example.com",
		Status:             "pending",
		Role:               "scholarship_provider",
	}
	if err := svc.repo.CreateScholarshipProviderUser(provider); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	if err := svc.RejectScholarshipProvider(provider.ID); err != nil {
		t.Fatalf("reject: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventAccountRejected, notification.Ref{Type: "provider", ID: provider.ID})
}

func TestApproveInstitutionNotifiesInstitution(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	inst := &InstitutionUser{
		InstitutionName:    "Test Institute",
		RegistrationNumber: "REG-INST-APPROVE-1",
		Email:              "inst@example.com",
		Status:             "pending",
		Role:               "institution",
	}
	if err := svc.repo.CreateInstitutionUser(inst); err != nil {
		t.Fatalf("seed institution: %v", err)
	}

	if err := svc.ApproveInstitution(inst.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventAccountApproved, notification.Ref{Type: "institution", ID: inst.ID})
}

func TestRejectInstitutionNotifiesInstitution(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	inst := &InstitutionUser{
		InstitutionName:    "Test Institute",
		RegistrationNumber: "REG-INST-REJECT-1",
		Email:              "inst-reject@example.com",
		Status:             "pending",
		Role:               "institution",
	}
	if err := svc.repo.CreateInstitutionUser(inst); err != nil {
		t.Fatalf("seed institution: %v", err)
	}

	if err := svc.RejectInstitution(inst.ID); err != nil {
		t.Fatalf("reject: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventAccountRejected, notification.Ref{Type: "institution", ID: inst.ID})
}

func TestApproveClaimRequestNotifiesInstitution(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	claim := &InstitutionUser{
		InstitutionName:    "Claimed Institute",
		RegistrationNumber: "REG-CLAIM-APPROVE-1",
		Email:              "claim@example.com",
		Status:             "pending",
		Role:               "institution",
		Claimed:            false,
	}
	if err := svc.repo.CreateInstitutionUser(claim); err != nil {
		t.Fatalf("seed claim: %v", err)
	}

	if err := svc.ApproveClaimRequest(claim.ID); err != nil {
		t.Fatalf("approve claim: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventAccountApproved, notification.Ref{Type: "institution", ID: claim.ID})
}

func TestRejectClaimRequestNotifiesInstitution(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	claim := &InstitutionUser{
		InstitutionName:    "Claimed Institute",
		RegistrationNumber: "REG-CLAIM-REJECT-1",
		Email:              "claim-reject@example.com",
		Status:             "pending",
		Role:               "institution",
		Claimed:            false,
	}
	if err := svc.repo.CreateInstitutionUser(claim); err != nil {
		t.Fatalf("seed claim: %v", err)
	}

	if err := svc.RejectClaimRequest(claim.ID, "Proof of ownership missing"); err != nil {
		t.Fatalf("reject claim: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventAccountRejected, notification.Ref{Type: "institution", ID: claim.ID})
}

func assertForRolesSuperadminAdmin(t *testing.T, notif *captureNotifier) {
	t.Helper()
	if len(notif.Roles) != 1 || len(notif.Roles[0]) != 2 || notif.Roles[0][0] != "superadmin" || notif.Roles[0][1] != "admin" {
		t.Fatalf("ForRoles called with %v, want [superadmin admin]", notif.Roles)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 1}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
}

func TestInstitutionRegisterDoesNotNotifyYet(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	notif.Audience = []notification.Ref{{Type: "user", ID: 1}}

	resp, err := svc.InstitutionRegister(InstitutionRegisterRequest{
		InstitutionName:    "Test Institute",
		RegistrationNumber: "REG-INST-REGISTER-1",
		Email:              "inst-register@example.com",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp == nil || !resp.RequiresOTP {
		t.Fatalf("unexpected response: %+v", resp)
	}

	// pending-approval emissions moved to VerifyOTP success (Task 3).
	if notif.Last != nil {
		t.Fatalf("expected no emission at register, got %+v", notif.Last)
	}
}

func TestScholarshipProviderRegisterDoesNotNotifyYet(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	notif.Audience = []notification.Ref{{Type: "user", ID: 1}}

	resp, err := svc.ScholarshipProviderRegister(ScholarshipProviderRegisterRequest{
		ProviderName:       "Scholarship Nepal",
		RegistrationNumber: "REG-PROVIDER-REGISTER-1",
		Email:              "provider-register@example.com",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp == nil || !resp.RequiresOTP {
		t.Fatalf("unexpected response: %+v", resp)
	}

	// pending-approval emissions moved to VerifyOTP success (Task 3).
	if notif.Last != nil {
		t.Fatalf("expected no emission at register, got %+v", notif.Last)
	}
}

func TestVerifyOTPStudentRegistrationWelcomesUser(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	utils.StoreOTP("student-welcome@example.com", "123456", User{
		Email: "student-welcome@example.com", FirstName: "Aasha", Role: "student",
	})

	resp, err := svc.VerifyOTP("student-welcome@example.com", "123456")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	user, ok := resp.User.(User)
	if !ok {
		t.Fatalf("unexpected user in response: %T", resp.User)
	}

	assertSingleRecipient(t, notif, notification.EventAccountWelcome, notification.Ref{Type: "user", ID: user.ID})
	if notif.Last.Data["first_name"] != "Aasha" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
}

func TestVerifyOTPProviderRegistrationEmits(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	notif.Audience = []notification.Ref{{Type: "user", ID: 1}}
	provider := ScholarshipProviderUser{
		ProviderName:       "Scholarship Nepal",
		RegistrationNumber: "REG-VERIFY-P1",
		Email:              "prov-verify@example.com",
		Role:               "scholarship_provider",
		Status:             "pending",
	}
	utils.StoreOTP(provider.Email, "123456", provider)

	if _, err := svc.VerifyOTP(provider.Email, "123456"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(notif.Calls) != 2 {
		t.Fatalf("expected 2 emissions, got %+v", notif.Calls)
	}
	welcome, pending := notif.Calls[0], notif.Calls[1]
	if welcome.EventKey != notification.EventAccountWelcome {
		t.Fatalf("expected welcome first, got %+v", welcome)
	}
	if len(welcome.Recipients) != 1 || welcome.Recipients[0] != (notification.Ref{Type: "provider", ID: 1}) {
		t.Fatalf("welcome recipient wrong: %+v", welcome.Recipients)
	}
	if welcome.Data["first_name"] != "Scholarship Nepal" {
		t.Fatalf("welcome data wrong: %+v", welcome.Data)
	}
	if pending.EventKey != notification.EventSystemProviderPending {
		t.Fatalf("expected %s, got %+v", notification.EventSystemProviderPending, pending)
	}
	if pending.Data["name"] != "Scholarship Nepal" {
		t.Fatalf("pending data wrong: %+v", pending.Data)
	}
	assertTemplates(t, welcome)
	assertTemplates(t, pending)

	subject, html := approvalPendingEmail("Scholarship Nepal", "provider")
	if subject != "StudSphere — Application Received" {
		t.Fatalf("subject wrong: %q", subject)
	}
	if !strings.Contains(html, "Hi Scholarship Nepal, we received your provider registration") {
		t.Fatalf("body wrong: %q", html)
	}
}

func TestVerifyOTPProviderRegistrationSkipsPendingWhenNoAudience(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	notif.Audience = nil
	provider := ScholarshipProviderUser{
		ProviderName:       "Scholarship Nepal",
		RegistrationNumber: "REG-VERIFY-P2",
		Email:              "prov-verify2@example.com",
		Role:               "scholarship_provider",
		Status:             "pending",
	}
	utils.StoreOTP(provider.Email, "123456", provider)

	if _, err := svc.VerifyOTP(provider.Email, "123456"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(notif.Calls) != 1 || notif.Calls[0].EventKey != notification.EventAccountWelcome {
		t.Fatalf("expected only welcome, got %+v", notif.Calls)
	}
}

func TestVerifyOTPInstitutionRegistrationEmits(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	notif.Audience = []notification.Ref{{Type: "user", ID: 1}}
	inst := InstitutionUser{
		InstitutionName:    "Test Institute",
		RegistrationNumber: "REG-VERIFY-I1",
		Email:              "inst-verify@example.com",
		Role:               "institution",
		Status:             "pending",
	}
	utils.StoreOTP(inst.Email, "123456", inst)

	if _, err := svc.VerifyOTP(inst.Email, "123456"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(notif.Calls) != 2 {
		t.Fatalf("expected 2 emissions, got %+v", notif.Calls)
	}
	welcome, pending := notif.Calls[0], notif.Calls[1]
	if welcome.EventKey != notification.EventAccountWelcome {
		t.Fatalf("expected welcome first, got %+v", welcome)
	}
	if welcome.Data["first_name"] != "Test Institute" {
		t.Fatalf("welcome data wrong: %+v", welcome.Data)
	}
	if pending.EventKey != notification.EventSystemInstitutionPending {
		t.Fatalf("expected %s, got %+v", notification.EventSystemInstitutionPending, pending)
	}
	assertTemplates(t, welcome)
	assertTemplates(t, pending)

	_, html := approvalPendingEmail("Test Institute", "institution")
	if !strings.Contains(html, "we received your institution registration") {
		t.Fatalf("body wrong: %q", html)
	}
}

func TestVerifyOTPClaimEmitsOnlyWelcome(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	notif.Audience = []notification.Ref{{Type: "user", ID: 1}}
	claim := InstitutionUser{
		InstitutionName:    "Claimed Institute",
		RegistrationNumber: "REG-VERIFY-C1",
		Email:              "claim-verify@example.com",
		Role:               "institution",
		Status:             "pending",
		CollegeID:          7,
	}
	utils.StoreOTP(claim.Email, "123456", claim)

	if _, err := svc.VerifyOTP(claim.Email, "123456"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(notif.Calls) != 1 || notif.Calls[0].EventKey != notification.EventAccountWelcome {
		t.Fatalf("expected only welcome, got %+v", notif.Calls)
	}
	if notif.Calls[0].Data["first_name"] != "Claimed Institute" {
		t.Fatalf("welcome data wrong: %+v", notif.Calls[0].Data)
	}
}

func TestNewSessionNotifiesNewDeviceLogin(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	user := &User{Email: "device-user@example.com", FirstName: "Dev", Role: "student"}
	if err := svc.repo.CreateUser(user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	ua := "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0"

	svc.CreateOrUpdateSession(user.ID, "10.0.0.5", ua, "")

	assertSingleRecipient(t, notif, notification.EventAccountNewDeviceLogin, notification.Ref{Type: "user", ID: user.ID})
	if notif.Last.DedupeKey != fmt.Sprintf("new_device:%d", user.ID) {
		t.Fatalf("dedupe key wrong: %q", notif.Last.DedupeKey)
	}
	if notif.Last.Data["email"] != user.Email {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)

	notif.Last = nil
	svc.CreateOrUpdateSession(user.ID, "10.0.0.5", ua, "")
	if notif.Last != nil {
		t.Fatalf("expected no emission on known device, got %+v", notif.Last)
	}
}

func TestSuspendAndReinstateUserNotify(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	user := &User{Email: "suspend@example.com", Role: "student"}
	if err := svc.repo.CreateUser(user); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if err := svc.SuspendUser(user.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	assertSingleRecipient(t, notif, notification.EventAccountSuspended, notification.Ref{Type: "user", ID: user.ID})
	assertTemplates(t, *notif.Last)

	if err := svc.ReinstateUser(user.ID); err != nil {
		t.Fatalf("reinstate: %v", err)
	}
	assertSingleRecipient(t, notif, notification.EventAccountReinstated, notification.Ref{Type: "user", ID: user.ID})
	assertTemplates(t, *notif.Last)
}

func TestDeletionScheduledAndCancelledNotify(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	user := &User{Email: "deletion@example.com", Role: "student"}
	if err := svc.repo.CreateUser(user); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if _, err := svc.QueueDeletion(user.ID); err != nil {
		t.Fatalf("queue deletion: %v", err)
	}
	assertSingleRecipient(t, notif, notification.EventAccountDeletionScheduled, notification.Ref{Type: "user", ID: user.ID})
	assertTemplates(t, *notif.Last)

	if err := svc.CancelDeletion(user.ID); err != nil {
		t.Fatalf("cancel deletion: %v", err)
	}
	assertSingleRecipient(t, notif, notification.EventAccountDeletionCancelled, notification.Ref{Type: "user", ID: user.ID})
	assertTemplates(t, *notif.Last)
}

func TestTOTPEnableDisableNotifies(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	user := &User{Email: "totp@example.com", Role: "student"}
	if err := svc.repo.CreateUser(user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "StudSphere", AccountName: user.Email})
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	user.TOTPSecret = key.Secret()
	if err := svc.repo.SaveUser(user); err != nil {
		t.Fatalf("save secret: %v", err)
	}

	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if err := svc.EnableTOTP(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	assertSingleRecipient(t, notif, notification.EventAccountTotpChanged, notification.Ref{Type: "user", ID: user.ID})
	assertTemplates(t, *notif.Last)

	code2, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if err := svc.DisableTOTP(user.ID, "", code2); err != nil {
		t.Fatalf("disable: %v", err)
	}
	assertSingleRecipient(t, notif, notification.EventAccountTotpChanged, notification.Ref{Type: "user", ID: user.ID})
}

func TestRecordInstitutionPaymentNotifiesInstitution(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	inst := &InstitutionUser{
		InstitutionName:    "Pay Institute",
		RegistrationNumber: "REG-PAY-1",
		Email:              "pay@example.com",
		Role:               "institution",
	}
	if err := svc.repo.CreateInstitutionUser(inst); err != nil {
		t.Fatalf("seed institution: %v", err)
	}

	if err := svc.RecordInstitutionPayment(inst.ID, time.Now(), 30, 500, ""); err != nil {
		t.Fatalf("record payment: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventPaymentSubscriptionRecorded, notification.Ref{Type: "institution", ID: inst.ID})
	if notif.Last.Data["plan"] == "" {
		t.Fatalf("plan data missing: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestClaimRegisterNotifiesSuperadmins(t *testing.T) {
	svc, notif := newAuthNotificationService(t)
	notif.Audience = []notification.Ref{{Type: "user", ID: 1}}
	if err := svc.repo.db.Create(&college.College{ID: 7, Name: "Test College", Location: "Kathmandu"}).Error; err != nil {
		t.Fatalf("seed college: %v", err)
	}

	resp, err := svc.ClaimRegister(ClaimRegisterRequest{
		CollegeID:          7,
		InstitutionName:    "Claimer Institute",
		RegistrationNumber: "REG-CLAIM-REGISTER-1",
		Email:              "claim-register@example.com",
	})
	if err != nil {
		t.Fatalf("claim register: %v", err)
	}
	if resp == nil || !resp.RequiresOTP {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSystemClaimSubmitted {
		t.Fatalf("expected %s, got %+v", notification.EventSystemClaimSubmitted, notif.Last)
	}
	if notif.Last.Data["college"] != "Test College" || notif.Last.Data["email"] != "claim-register@example.com" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertForRolesSuperadminAdmin(t, notif)
	assertTemplates(t, *notif.Last)
}

func TestCreateInstitutionNotifiesInstitution(t *testing.T) {
	svc, notif := newAuthNotificationService(t)

	inst, err := svc.CreateInstitution(CreateInstitutionRequest{InstitutionName: "Created Institute"})
	if err != nil {
		t.Fatalf("create institution: %v", err)
	}

	assertSingleRecipient(t, notif, notification.EventAccountApproved, notification.Ref{Type: "institution", ID: inst.ID})
	assertTemplates(t, *notif.Last)
}
