package jobs

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

// jobsUserRow is a projection of the users table covering only the columns
// the applicant-account lookup touches. Kept local to avoid importing the
// auth module into jobs.
type jobsUserRow struct {
	ID        uint
	Email     string
	DeletedAt gorm.DeletedAt
}

func (jobsUserRow) TableName() string { return "users" }

func testDBJobs(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Job{}, &JobApplication{}, &jobsUserRow{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedJob(t *testing.T, db *gorm.DB) *Job {
	t.Helper()
	job := Job{Title: "Backend Engineer", Department: "Engineering", Description: "Build things", JobType: "full-time", Status: "published"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}
	return &job
}

func seedApplication(t *testing.T, db *gorm.DB, job *Job, email string) *JobApplication {
	t.Helper()
	app := JobApplication{JobID: job.ID, FullName: "Ada Lovelace", Email: email, Phone: "9800000000", ResumeURL: "resumes/r.pdf", Status: "pending"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return &app
}

func TestSubmitApplicationNotifiesAdmins(t *testing.T) {
	db := testDBJobs(t)
	job := seedJob(t, db)
	notif := &captureNotifier{Audience: []notification.Ref{{Type: "user", ID: 1}}}
	svc := NewServiceWithDB(NewRepository(db), db, notif)

	if _, err := svc.SubmitApplication(job.ID, "Ada Lovelace", "ada@example.com", "9800000000", "resumes/r.pdf", ""); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventJobsApplicationReceived {
		t.Fatalf("expected %s, got %+v", notification.EventJobsApplicationReceived, notif.Last)
	}
	if notif.Last.Data["name"] != "Ada Lovelace" || notif.Last.Data["title"] != job.Title {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	if len(notif.Roles) != 1 || len(notif.Roles[0]) != 2 || notif.Roles[0][0] != "superadmin" || notif.Roles[0][1] != "admin" {
		t.Fatalf("ForRoles called with %v, want [superadmin admin]", notif.Roles)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 1}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}

func TestSubmitApplicationSkipsWhenAudienceEmpty(t *testing.T) {
	db := testDBJobs(t)
	job := seedJob(t, db)
	notif := &captureNotifier{}
	svc := NewServiceWithDB(NewRepository(db), db, notif)

	if _, err := svc.SubmitApplication(job.ID, "Ada Lovelace", "ada@example.com", "9800000000", "resumes/r.pdf", ""); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if notif.Last != nil {
		t.Fatalf("unexpected emission with empty audience: %+v", notif.Last)
	}
	if len(notif.Roles) != 1 {
		t.Fatalf("ForRoles not consulted: %v", notif.Roles)
	}
}

func TestUpdateApplicationStatusNotifiesApplicant(t *testing.T) {
	db := testDBJobs(t)
	job := seedJob(t, db)
	app := seedApplication(t, db, job, "ada@example.com")
	if err := db.Create(&jobsUserRow{ID: 7, Email: "ada@example.com"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewServiceWithDB(NewRepository(db), db, notif)

	if _, err := svc.UpdateApplicationStatus(app.ID, UpdateApplicantStatusRequest{Status: "shortlisted"}); err != nil {
		t.Fatalf("update status: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventJobsStatusChanged {
		t.Fatalf("expected %s, got %+v", notification.EventJobsStatusChanged, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: 7}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["job_title"] != job.Title || notif.Last.Data["status"] != "shortlisted" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestUpdateApplicationStatusSkipsWithoutAccount(t *testing.T) {
	db := testDBJobs(t)
	job := seedJob(t, db)
	app := seedApplication(t, db, job, "guest@example.com")
	notif := &captureNotifier{}
	svc := NewServiceWithDB(NewRepository(db), db, notif)

	if _, err := svc.UpdateApplicationStatus(app.ID, UpdateApplicantStatusRequest{Status: "shortlisted"}); err != nil {
		t.Fatalf("update status: %v", err)
	}

	if notif.Last != nil {
		t.Fatalf("unexpected emission without account: %+v", notif.Last)
	}
}

// The manual email send must be gone from the status path — email flows
// through the pipeline instead (no double emission). EnqueueGenericEmail
// fails with a nil asynq queue in tests, so any error here means the
// manual call is still present.
func TestSendApplicantEmailStatusPathUsesPipeline(t *testing.T) {
	db := testDBJobs(t)
	job := seedJob(t, db)
	app := seedApplication(t, db, job, "ada@example.com")
	app.Notes = "keep me"
	if err := db.Save(app).Error; err != nil {
		t.Fatalf("seed notes: %v", err)
	}
	if err := db.Create(&jobsUserRow{ID: 7, Email: "ada@example.com"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	notif := &captureNotifier{}
	svc := NewServiceWithDB(NewRepository(db), db, notif)

	if err := svc.SendApplicantEmail(app.ID, SendApplicantEmailRequest{Subject: "s", Body: "b", UpdateStatus: "rejected"}); err != nil {
		t.Fatalf("send applicant email: %v (manual email call still present?)", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventJobsStatusChanged {
		t.Fatalf("expected %s, got %+v", notification.EventJobsStatusChanged, notif.Last)
	}
	if notif.Last.Data["status"] != "rejected" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	var reloaded JobApplication
	if err := db.First(&reloaded, app.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "rejected" || reloaded.Notes != "keep me" {
		t.Fatalf("status/notes wrong: %+v", reloaded)
	}
}

// Custom email without a status change keeps the manual email path.
func TestSendApplicantEmailCustomKeepsManualEmail(t *testing.T) {
	db := testDBJobs(t)
	job := seedJob(t, db)
	app := seedApplication(t, db, job, "ada@example.com")
	notif := &captureNotifier{}
	svc := NewServiceWithDB(NewRepository(db), db, notif)

	err := svc.SendApplicantEmail(app.ID, SendApplicantEmailRequest{Subject: "Hello", Body: "World"})
	if err == nil {
		t.Fatal("expected manual email enqueue error (asynq nil in tests), got nil")
	}
	if notif.Last != nil {
		t.Fatalf("unexpected emission for custom email: %+v", notif.Last)
	}
}
