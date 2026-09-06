package institution

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"studsphere/backend/internal/education"
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

func TestBookingConfirmNotifiesStudent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&InstitutionCounsellingSession{}, &InstitutionCounsellingBooking{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), education.NewRepository(db), nil, notif)

	session := &InstitutionCounsellingSession{InstitutionID: 1, Title: "Career Guidance", Status: "scheduled"}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}
	booking := &InstitutionCounsellingBooking{SessionID: session.ID, UserID: 7, Status: "pending"}
	if err := db.Create(booking).Error; err != nil {
		t.Fatalf("seed booking: %v", err)
	}

	if _, err := svc.UpdateBookingStatus(1, booking.ID, "confirmed", "", ""); err != nil {
		t.Fatalf("confirm booking: %v", err)
	}
	if notif.Last == nil || notif.Last.EventKey != notification.EventCounsellingBookingConfirmed {
		t.Fatalf("expected %s, got %+v", notification.EventCounsellingBookingConfirmed, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: booking.UserID}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestCreatePublicBookingNotifiesInstitution(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&InstitutionCounsellingSession{}, &InstitutionCounsellingBooking{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), education.NewRepository(db), nil, notif)

	session := &InstitutionCounsellingSession{
		InstitutionID: 5,
		Title:         "Career Guidance",
		ScheduledAt:   time.Date(2026, 9, 10, 14, 0, 0, 0, time.UTC),
		MaxSeats:      10,
		Status:        "scheduled",
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}

	if _, err := svc.CreatePublicBooking(7, PublicCounsellingBookingRequest{
		SessionID:        session.ID,
		ProgramLevel:     "Bachelor",
		InterestedCourse: "BSc Computer Science",
		SessionMode:      "online",
		StudentName:      "Test Student",
		StudentPhone:     "9800000000",
		StudentEmail:     "student@example.com",
	}); err != nil {
		t.Fatalf("create booking: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventCounsellingBookingCreated {
		t.Fatalf("expected %s, got %+v", notification.EventCounsellingBookingCreated, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "institution", ID: 5}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

// testDBInstitutionPostgres skips (never fails) without a Postgres DSN — the
// admission-status path resolves the college via a jsonb query that only
// PostgreSQL supports. Set TEST_DATABASE_DSN to run it.
func testDBInstitutionPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; institution admission-status notification test requires PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&College{}, &Admission{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.Exec(`ALTER TABLE colleges ADD COLUMN IF NOT EXISTS university_affiliations jsonb DEFAULT '[]'`).Error; err != nil {
		t.Fatalf("add university_affiliations: %v", err)
	}
	return db
}

func TestInstitutionStatusUpdateNotifiesApplicant(t *testing.T) {
	db := testDBInstitutionPostgres(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), education.NewRepository(db), nil, notif)

	const instID = uint(910001)
	const collegeID = uint(910002)
	if err := db.Exec(`INSERT INTO colleges (id, name, university_affiliations) VALUES (?, 'Test College', ?)`,
		collegeID, fmt.Sprintf("[%d]", instID)).Error; err != nil {
		t.Fatalf("seed college: %v", err)
	}

	uid := uint(7)
	app := &Admission{
		CollegeID:    collegeID,
		ProgramName:  "BSc Computer Science",
		ProgramLevel: "Bachelor",
		StudentName:  "Test Student",
		StudentEmail: "student@example.com",
		StudentPhone: "9800000000",
		Status:       "pending",
		UserID:       &uid,
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("seed admission: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM admissions WHERE id = ?`, app.ID)
		db.Exec(`DELETE FROM colleges WHERE id = ?`, collegeID)
	})

	if _, err := svc.UpdateAdmissionStatus(instID, app.ID, UpdateAdmissionStatusRequest{Status: "shortlisted"}); err != nil {
		t.Fatalf("update status: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventApplicationStatusChanged {
		t.Fatalf("expected %s, got %+v", notification.EventApplicationStatusChanged, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: uid}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}
