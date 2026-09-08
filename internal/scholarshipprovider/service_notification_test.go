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
	if err := db.AutoMigrate(&ProviderScholarship{}, &ProviderApplication{}, &ProviderInterview{}, &ProviderReview{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`TRUNCATE provider_scholarships, provider_applications, provider_interviews, provider_reviews`)
	})
	return db
}

// paymentRow is a projection of scholarship.Payment covering only the columns
// the dispute-status flow touches. Kept local so AutoMigrate doesn't pull the
// scholarship module's association tables.
type paymentRow struct {
	ID              uint       `gorm:"primarykey"`
	ApplicationID   uint       `gorm:"index"`
	ScholarshipID   uint       `gorm:"index"`
	UserID          *uint      `gorm:"index"`
	Method          string     `gorm:"not null"`
	Amount          float64    `gorm:"not null"`
	Status          string     `gorm:"default:pending"`
	ReceiptURL      string
	TransactionID   string
	PaidAt          *time.Time
	ApprovedAt      *time.Time
	ApprovedBy      uint       `gorm:"index"`
	RejectionReason string
	DisputeStatus   string     `gorm:"default:pending"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (paymentRow) TableName() string { return "scholarship_payments" }

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

	// Fallback branch: any status outside shortlisted/approved/rejected
	// (under_review, pending) still emits the applicant copy — with the
	// status key, since EventApplicationStatusChanged's template needs it.
	if _, err := svc.UpdateApplicationStatus(1, app.ID, UpdateApplicationStatusRequest{Status: "under_review"}); err != nil {
		t.Fatalf("update status (fallback): %v", err)
	}
	if len(notif.Calls) != 4 {
		t.Fatalf("expected 4 emissions total (2 per status update), got %d: %+v", len(notif.Calls), notif.Calls)
	}
	fbOrg, fbApplicant := notif.Calls[2], notif.Calls[3]
	if fbOrg.EventKey != notification.EventApplicationStatusChanged || fbOrg.Recipients[0] != (notification.Ref{Type: "provider", ID: 1}) {
		t.Fatalf("fallback org copy wrong: %+v", fbOrg)
	}
	if fbApplicant.EventKey != notification.EventApplicationStatusChanged || fbApplicant.Recipients[0] != (notification.Ref{Type: "user", ID: uid}) {
		t.Fatalf("fallback applicant copy wrong: %+v", fbApplicant)
	}
	if fbApplicant.Data["status"] != "under_review" {
		t.Fatalf("fallback applicant copy missing status data: %+v", fbApplicant.Data)
	}
	assertTemplates(t, fbOrg)
	assertTemplates(t, fbApplicant)
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

func TestUpdateDisputeStatusNotifiesStudent(t *testing.T) {
	db := testDBProvider(t)
	if err := db.AutoMigrate(&paymentRow{}); err != nil {
		t.Fatalf("automigrate payments: %v", err)
	}
	t.Cleanup(func() { db.Exec(`TRUNCATE scholarship_payments`) })
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	uid := uint(7)
	saID := uint(99)
	ps := &ProviderScholarship{ProviderID: 1, Title: "Test Scholarship", Status: "published"}
	if err := db.Create(ps).Error; err != nil {
		t.Fatalf("seed scholarship: %v", err)
	}
	app := &ProviderApplication{ScholarshipID: ps.ID, UserID: &uid, ScholarshipApplicationID: &saID, FullName: "Test Student", Email: "student@example.com", Status: "approved"}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("seed application: %v", err)
	}
	if err := db.Create(&paymentRow{ApplicationID: saID, ScholarshipID: ps.ID, Method: "esewa", Amount: 100}).Error; err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	if err := svc.UpdateDisputeStatus(1, app.ID, "resolved"); err != nil {
		t.Fatalf("update dispute status: %v", err)
	}
	if notif.Last == nil || notif.Last.EventKey != notification.EventApplicationStatusChanged {
		t.Fatalf("expected %s, got %+v", notification.EventApplicationStatusChanged, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: uid}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["program"] != "Test Scholarship" || notif.Last.Data["status"] != "resolved" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)

	var payment paymentRow
	if err := db.Order("id asc").First(&payment).Error; err != nil {
		var cnt int64
		db.Model(&paymentRow{}).Count(&cnt)
		t.Fatalf("load payment (rows=%d): %v", cnt, err)
	}
	if payment.DisputeStatus != "resolved" {
		t.Fatalf("dispute status not persisted: %+v", payment)
	}
}

func TestDeleteReviewNotifiesReviewer(t *testing.T) {
	db := testDBProvider(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	uid := uint(7)
	review := &ProviderReview{ProviderID: 1, AuthorName: "Test Student", UserID: &uid}
	if err := db.Create(review).Error; err != nil {
		t.Fatalf("seed review: %v", err)
	}

	if err := svc.DeleteReview(1, review.ID); err != nil {
		t.Fatalf("delete review: %v", err)
	}
	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialReviewModerated {
		t.Fatalf("expected %s, got %+v", notification.EventSocialReviewModerated, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: uid}) {
		t.Fatalf("recipient wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestDeleteReviewSkipsUnlinkedReviewer(t *testing.T) {
	db := testDBProvider(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)

	review := &ProviderReview{ProviderID: 1, AuthorName: "Anonymous"}
	if err := db.Create(review).Error; err != nil {
		t.Fatalf("seed review: %v", err)
	}

	if err := svc.DeleteReview(1, review.ID); err != nil {
		t.Fatalf("delete review: %v", err)
	}
	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions without a linked reviewer, got %+v", notif.Calls)
	}
}
