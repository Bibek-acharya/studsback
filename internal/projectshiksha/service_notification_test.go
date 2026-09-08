package projectshiksha

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"studsphere/backend/internal/notification"
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

func testDBShiksha(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&ShikshaApplication{}, &ShikshaPayment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedShikshaApp(t *testing.T, db *gorm.DB, userID *uint) *ShikshaApplication {
	t.Helper()
	app := &ShikshaApplication{
		FullName: "Ada Lovelace", Gender: "female", DOBBS: "2060-01-01", DOBAD: "2004-01-01",
		Phone: "9800000000", Email: "ada@example.com", SEESchoolType: "community",
		SchoolName: "Some School", PermProvince: "3", PermDistrict: "Kathmandu",
		PermMunicipality: "Kathmandu", GuardianName: "Guardian", GuardianPhone: "9800000001",
		PaymentStatus: "pending", Status: "submitted", UserID: userID,
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return app
}

func TestCreateApplicationNotifiesSuperadmins(t *testing.T) {
	db := testDBShiksha(t)
	notif := &captureNotifier{Audience: []notification.Ref{{Type: "user", ID: 1}}}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.CreateApplication(CreateApplicationRequest{
		FullName: "Ada Lovelace", Gender: "female", DOBBS: "2060-01-01", DOBAD: "2004-01-01",
		Phone: "9800000000", Email: "ada@example.com", SEESchoolType: "community",
		SchoolName: "Some School", PermProvince: "3", PermDistrict: "Kathmandu",
		PermMunicipality: "Kathmandu", GuardianName: "Guardian", GuardianPhone: "9800000001",
	}); err != nil {
		t.Fatalf("create application: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSystemInquiryReceived {
		t.Fatalf("expected %s, got %+v", notification.EventSystemInquiryReceived, notif.Last)
	}
	if notif.Last.Data["name"] != "Ada Lovelace" || notif.Last.Data["email"] != "ada@example.com" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	if _, ok := notif.Last.Data["subject"]; !ok {
		t.Fatalf("subject key missing from data: %+v", notif.Last.Data)
	}
	if len(notif.Roles) != 1 || len(notif.Roles[0]) != 1 || notif.Roles[0][0] != "superadmin" {
		t.Fatalf("ForRoles called with %v, want [superadmin]", notif.Roles)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 1}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestCreateApplicationSkipsWhenAudienceEmpty(t *testing.T) {
	db := testDBShiksha(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.CreateApplication(CreateApplicationRequest{
		FullName: "Ada Lovelace", Gender: "female", DOBBS: "2060-01-01", DOBAD: "2004-01-01",
		Phone: "9800000000", Email: "ada@example.com", SEESchoolType: "community",
		SchoolName: "Some School", PermProvince: "3", PermDistrict: "Kathmandu",
		PermMunicipality: "Kathmandu", GuardianName: "Guardian", GuardianPhone: "9800000001",
	}); err != nil {
		t.Fatalf("create application: %v", err)
	}

	if notif.Last != nil {
		t.Fatalf("unexpected emission with empty audience: %+v", notif.Last)
	}
	if len(notif.Roles) != 1 {
		t.Fatalf("ForRoles not consulted: %v", notif.Roles)
	}
}

func TestUpdateApplicationStatusNotifiesApplicant(t *testing.T) {
	db := testDBShiksha(t)
	uid := uint(7)
	app := seedShikshaApp(t, db, &uid)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if err := svc.UpdateApplicationStatus(app.ID, "accepted"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventProjectshikshaStatusChanged {
		t.Fatalf("expected %s, got %+v", notification.EventProjectshikshaStatusChanged, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 7}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["status"] != "accepted" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestUpdateApplicationStatusSkipsWithoutAccount(t *testing.T) {
	db := testDBShiksha(t)
	app := seedShikshaApp(t, db, nil)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if err := svc.UpdateApplicationStatus(app.ID, "accepted"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	if notif.Last != nil {
		t.Fatalf("unexpected emission without account: %+v", notif.Last)
	}
}

func TestProcessPaymentKhaltiNotifiesApplicant(t *testing.T) {
	db := testDBShiksha(t)
	uid := uint(7)
	app := seedShikshaApp(t, db, &uid)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	if _, err := svc.ProcessPayment(app.ID, "khalti", 100, "txn-1"); err != nil {
		t.Fatalf("process payment: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventProjectshikshaStatusChanged {
		t.Fatalf("expected %s, got %+v", notification.EventProjectshikshaStatusChanged, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 7}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["status"] != "completed" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	if len(notif.Calls) != 1 {
		t.Fatalf("expected exactly one emission, got %d", len(notif.Calls))
	}
	assertTemplates(t, *notif.Last)
}
