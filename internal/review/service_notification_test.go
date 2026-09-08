package review

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"studsphere/backend/internal/auth"
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

func testDBReview(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&auth.User{}, &Review{}, &ReviewReport{}, &institutionUsersRow{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// institutionUsersRow is a projection of institution.InstitutionUser covering
// only the columns the recipient-resolution query touches. Kept local to avoid
// importing the institution module.
type institutionUsersRow struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Status    string         `gorm:"default:'pending'" json:"status"`
	CollegeID uint           `gorm:"default:0" json:"college_id"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (institutionUsersRow) TableName() string { return "institution_users" }

func TestReportReviewNotifiesSuperadmins(t *testing.T) {
	db := testDBReview(t)
	notif := &captureNotifier{Audience: []notification.Ref{{Type: "user", ID: 1}}}
	svc := NewService(NewRepository(db), notif)

	reviewer := auth.User{Email: "reviewer@example.com", FirstName: "Ada", LastName: "Lovelace"}
	if err := db.Create(&reviewer).Error; err != nil {
		t.Fatalf("seed reviewer: %v", err)
	}
	review := &Review{
		UserID:       reviewer.ID,
		UniversityID: 7,
		StudentType:  "current",
		BatchYear:    2024,
		Ratings:      []byte(`{"overall":4}`),
		Pros:         "Good",
		Cons:         "Far",
		IsPublished:  true,
	}
	if err := db.Create(review).Error; err != nil {
		t.Fatalf("seed review: %v", err)
	}

	if err := svc.ReportReview(review.ID, 42, "Spam content"); err != nil {
		t.Fatalf("report: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialReviewReported {
		t.Fatalf("expected %s, got %+v", notification.EventSocialReviewReported, notif.Last)
	}
	if notif.Last.Data["review_id"] != review.ID || notif.Last.Data["reason"] != "Spam content" {
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

func seedReviewer(t *testing.T, db *gorm.DB) auth.User {
	t.Helper()
	reviewer := auth.User{Email: "reviewer@example.com", FirstName: "Ada", LastName: "Lovelace"}
	if err := db.Create(&reviewer).Error; err != nil {
		t.Fatalf("seed reviewer: %v", err)
	}
	return reviewer
}

func TestSubmitReviewNotifiesClaimedInstitution(t *testing.T) {
	db := testDBReview(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	reviewer := seedReviewer(t, db)
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}

	if _, err := svc.SubmitReview(reviewer.ID, CreateReviewRequest{
		CollegeID:   3,
		CollegeName: "Test College",
		StudentType: "current",
		BatchYear:   2024,
		Ratings:     map[string]float64{"overall": 4},
		Pros:        "Great faculty and campus",
		Cons:        "Far from the city",
		Email:       "reviewer@example.com",
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialReviewReceived {
		t.Fatalf("expected %s, got %+v", notification.EventSocialReviewReceived, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "institution", ID: 42}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	if notif.Last.Data["name"] != "Ada Lovelace" {
		t.Fatalf("data wrong: %+v", notif.Last.Data)
	}
	if notif.Last.Data["rating"] != "4" {
		t.Fatalf("rating wrong: %+v", notif.Last.Data["rating"])
	}
	assertTemplates(t, *notif.Last)
}

func TestSubmitReviewSkipsPendingInstitution(t *testing.T) {
	db := testDBReview(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	reviewer := seedReviewer(t, db)
	// Claimed but unapproved — D-Q13: no emission.
	if err := db.Create(&institutionUsersRow{ID: 43, Status: "pending", CollegeID: 5}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}

	if _, err := svc.SubmitReview(reviewer.ID, CreateReviewRequest{
		CollegeID:   5,
		CollegeName: "Pending College",
		StudentType: "current",
		BatchYear:   2024,
		Ratings:     map[string]float64{"overall": 4},
		Pros:        "Great faculty and campus",
		Cons:        "Far from the city",
		Email:       "reviewer@example.com",
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if len(notif.Calls) != 0 {
		t.Fatalf("expected no emissions, got %+v", notif.Calls)
	}
}

func TestSubmitReviewOmitsRatingKeyWhenAbsent(t *testing.T) {
	db := testDBReview(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	reviewer := seedReviewer(t, db)
	if err := db.Create(&institutionUsersRow{ID: 42, Status: "approved", CollegeID: 3}).Error; err != nil {
		t.Fatalf("seed institution user: %v", err)
	}

	// No "overall" in ratings — the "rating" key must be absent (the registry
	// body guards with {{if .rating}}); the template must still resolve.
	if _, err := svc.SubmitReview(reviewer.ID, CreateReviewRequest{
		CollegeID:   3,
		CollegeName: "Test College",
		StudentType: "current",
		BatchYear:   2024,
		Ratings:     map[string]float64{"faculty": 4},
		Pros:        "Great faculty and campus",
		Cons:        "Far from the city",
		Email:       "reviewer@example.com",
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialReviewReceived {
		t.Fatalf("expected %s, got %+v", notification.EventSocialReviewReceived, notif.Last)
	}
	if _, ok := notif.Last.Data["rating"]; ok {
		t.Fatalf("rating key must be absent, got %+v", notif.Last.Data)
	}
	assertTemplates(t, *notif.Last)
}

func TestAdminDeleteReviewNotifiesReviewer(t *testing.T) {
	db := testDBReview(t)
	notif := &captureNotifier{}
	svc := NewService(NewRepository(db), notif)
	reviewer := seedReviewer(t, db)
	review := &Review{
		UserID:      reviewer.ID,
		CollegeID:   3,
		StudentType: "current",
		BatchYear:   2024,
		Ratings:     []byte(`{"overall":4}`),
		Pros:        "Good",
		Cons:        "Far",
		IsPublished: true,
	}
	if err := db.Create(review).Error; err != nil {
		t.Fatalf("seed review: %v", err)
	}

	if err := svc.AdminDeleteReview(review.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}

	if notif.Last == nil || notif.Last.EventKey != notification.EventSocialReviewModerated {
		t.Fatalf("expected %s, got %+v", notification.EventSocialReviewModerated, notif.Last)
	}
	if len(notif.Last.Recipients) != 1 || notif.Last.Recipients[0] != (notification.Ref{Type: "user", ID: reviewer.ID}) {
		t.Fatalf("recipients wrong: %+v", notif.Last.Recipients)
	}
	assertTemplates(t, *notif.Last)
}
