package auth

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/college"
	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/config"
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
	if err := db.AutoMigrate(&User{}, &InstitutionUser{}, &ScholarshipProviderUser{}, &college.College{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	config.AppConfig = &config.Config{
		SMTPHost: "localhost",
		SMTPPort: "1",
		SMTPUser: "test@example.com",
		SMTPPass: "password",
	}
	notif := &captureNotifier{}
	SetNotifier(notif)
	t.Cleanup(func() { SetNotifier(nil) })
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

func TestInstitutionRegisterNotifiesSuperadmins(t *testing.T) {
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

	if notif.Last == nil || notif.Last.EventKey != notification.EventSystemInstitutionPending {
		t.Fatalf("expected %s, got %+v", notification.EventSystemInstitutionPending, notif.Last)
	}
	if notif.Last.Data["name"] != "Test Institute" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertForRolesSuperadminAdmin(t, notif)
	assertTemplates(t, *notif.Last)
}

func TestScholarshipProviderRegisterNotifiesSuperadmins(t *testing.T) {
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

	if notif.Last == nil || notif.Last.EventKey != notification.EventSystemProviderPending {
		t.Fatalf("expected %s, got %+v", notification.EventSystemProviderPending, notif.Last)
	}
	if notif.Last.Data["name"] != "Scholarship Nepal" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertForRolesSuperadminAdmin(t, notif)
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
