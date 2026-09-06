package admission

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"studsphere/backend/internal/notification"
)

type captureNotifier struct {
	Last  *notification.NotifyRequest
	Calls []notification.NotifyRequest
}

func (c *captureNotifier) Notify(_ context.Context, req notification.NotifyRequest) error {
	c.Last = &req
	c.Calls = append(c.Calls, req)
	return nil
}

func (c *captureNotifier) NotifyTx(ctx context.Context, _ *gorm.DB, req notification.NotifyRequest) error {
	return c.Notify(ctx, req)
}

func (c *captureNotifier) ForRoles(_ context.Context, _ ...string) ([]notification.Ref, error) {
	return nil, nil
}

// testDBAdmission skips (never fails) without a Postgres DSN — the module is
// PostgreSQL-only. Set TEST_DATABASE_DSN to run the SQL integration tests.
func testDBAdmission(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; admission notification tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&Admission{}, &College{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`TRUNCATE admissions, colleges`)
	})
	return db
}

func seedAdmission(t *testing.T, db *gorm.DB) *Admission {
	t.Helper()
	uid := uint(7)
	app := &Admission{
		UserID:       &uid,
		CollegeID:    1,
		ProgramName:  "BSc Computer Science",
		ProgramLevel: "Bachelor",
		StudentName:  "Test Student",
		StudentEmail: "student@example.com",
		StudentPhone: "9800000000",
		Status:       "pending",
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("seed admission: %v", err)
	}
	return app
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

func TestStatusUpdateNotifiesApplicant(t *testing.T) {
	db := testDBAdmission(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	app := seedAdmission(t, db)

	if _, err := svc.UpdateStatus(app.ID, UpdateAdmissionStatusRequest{Status: "shortlisted"}, 1); err != nil {
		t.Fatalf("update: %v", err)
	}
	if notif.Last == nil || notif.Last.EventKey != notification.EventApplicationStatusChanged {
		t.Fatalf("expected %s, got %+v", notification.EventApplicationStatusChanged, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: *app.UserID}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}
