package system

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

func testDBSystem(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&ContactInquiry{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestSubmitContactInquiryNotifiesSuperadmins(t *testing.T) {
	db := testDBSystem(t)
	notif := &captureNotifier{Audience: []notification.Ref{{Type: "user", ID: 1}}}
	svc := NewService(NewRepository(db), notif)

	inquiry, err := svc.SubmitContactInquiry(ContactInquiryRequest{
		Name:    "Ann Visitor",
		Email:   "ann@example.com",
		Subject: "Admission question",
		Message: "Hello, I have a question.",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if inquiry == nil {
		t.Fatal("nil inquiry")
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSystemInquiryReceived {
		t.Fatalf("expected %s, got %+v", notification.EventSystemInquiryReceived, notif.Last)
	}
	if len(notif.Roles) != 1 || len(notif.Roles[0]) != 2 || notif.Roles[0][0] != "superadmin" || notif.Roles[0][1] != "admin" {
		t.Fatalf("ForRoles called with %v, want [superadmin admin]", notif.Roles)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 1}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

// usersRow is a projection of auth.User covering only the columns the
// inquirer-account lookup touches. Kept local to avoid importing the auth
// module.
type usersRow struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Email     string         `gorm:"index" json:"email"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (usersRow) TableName() string { return "users" }

func seedInquirer(t *testing.T, db *gorm.DB, id uint, email string) {
	t.Helper()
	if err := db.Create(&usersRow{ID: id, Email: email}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func TestUpdateContactInquiryStatusNotifiesRegisteredInquirer(t *testing.T) {
	db := testDBSystem(t)
	db.AutoMigrate(&usersRow{})
	seedInquirer(t, db, 5, "ann@example.com")
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	db.Create(&ContactInquiry{Name: "Ann", Email: "ann@example.com", Subject: "Admission question", Status: "new"})

	_, err := svc.UpdateContactInquiryStatus(1, "resolved")
	if err != nil {
		t.Fatalf("update status: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSystemInquiryReplied {
		t.Fatalf("expected %s, got %+v", notification.EventSystemInquiryReplied, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 5}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["subject"] != "Admission question" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestUpdateContactInquiryStatusSkipsGuestInquirer(t *testing.T) {
	db := testDBSystem(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	db.Create(&ContactInquiry{Name: "Guest", Email: "guest@example.com", Subject: "Question", Status: "new"})

	if _, err := svc.UpdateContactInquiryStatus(1, "resolved"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions for guest inquirer, got %+v", notif.Calls)
	}
}
