package scholarshipprovider

import (
	"context"
	"os"
	"testing"
	"time"

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

// testDBProvider skips (never fails) without a Postgres DSN — the module is
// PostgreSQL-only. Set TEST_DATABASE_DSN to run the SQL integration tests.
func testDBProvider(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; scholarshipprovider notification tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&ProviderScholarship{}, &ProviderApplication{}, &ProviderInterview{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`TRUNCATE provider_scholarships, provider_applications, provider_interviews`)
	})
	return db
}

func TestApplicationStatusNotifiesOrgAndApplicant(t *testing.T) {
	db := testDBProvider(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	uid := uint(7)
	ps := &ProviderScholarship{ProviderID: 1, Title: "Test Scholarship", Status: "published"}
	if err := db.Create(ps).Error; err != nil {
		t.Fatalf("seed scholarship: %v", err)
	}
	app := &ProviderApplication{ScholarshipID: ps.ID, UserID: &uid, FullName: "Test Student", Email: "student@example.com", Status: "pending"}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("seed application: %v", err)
	}

	if _, err := svc.UpdateApplicationStatus(1, app.ID, UpdateApplicationStatusRequest{Status: "shortlisted"}); err != nil {
		t.Fatalf("update status: %v", err)
	}
	if len(notif.Calls) != 2 {
		t.Fatalf("expected 2 emissions (org + applicant), got %d: %+v", len(notif.Calls), notif.Calls)
	}
	org, applicant := notif.Calls[0], notif.Calls[1]
	if org.EventKey != notification.EventApplicationStatusChanged || len(org.Recipients) != 1 || org.Recipients[0] != (notification.Ref{Type: "provider", ID: 1}) {
		t.Fatalf("org copy wrong: %+v", org)
	}
	if applicant.EventKey != notification.EventApplicationShortlisted || len(applicant.Recipients) != 1 || applicant.Recipients[0] != (notification.Ref{Type: "user", ID: uid}) {
		t.Fatalf("applicant copy wrong: %+v", applicant)
	}
	assertTemplates(t, org)
	assertTemplates(t, applicant)
}

func TestInterviewNotifiesStudent(t *testing.T) {
	db := testDBProvider(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	uid := uint(7)
	ps := &ProviderScholarship{ProviderID: 1, Title: "Test Scholarship", Status: "published"}
	if err := db.Create(ps).Error; err != nil {
		t.Fatalf("seed scholarship: %v", err)
	}
	app := &ProviderApplication{ScholarshipID: ps.ID, UserID: &uid, FullName: "Test Student", Email: "student@example.com", Status: "approved"}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("seed application: %v", err)
	}

	req := CreateInterviewRequest{
		ApplicationID: app.ID,
		ScheduledAt:   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		Duration:      30,
		Type:          "online",
	}
	if _, err := svc.CreateInterview(1, req); err != nil {
		t.Fatalf("create interview: %v", err)
	}
	var student *notification.NotifyRequest
	for i := range notif.Calls {
		if len(notif.Calls[i].Recipients) == 1 && notif.Calls[i].Recipients[0].Type == "user" {
			student = &notif.Calls[i]
		}
	}
	if student == nil {
		t.Fatalf("no student copy emitted, calls: %+v", notif.Calls)
	}
	if student.EventKey != notification.EventApplicationInterviewScheduled || student.Recipients[0] != (notification.Ref{Type: "user", ID: uid}) {
		t.Fatalf("student copy wrong: %+v", student)
	}
	assertTemplates(t, *student)
}
