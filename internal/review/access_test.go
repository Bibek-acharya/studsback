package review

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/notification"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// The review module's authorisation, pinned at the ROUTE level.
//
// access_test.go pins the role list; this pins which groups use it and that the
// tenant read still works. The vulnerability was that three surfaces shared one
// wide gate, and that the tenant DELETE was unscoped rather than merely
// over-gated — a claim only a route-level test can make, because the service
// method it called has no caller argument at all.

type reviewNotifier struct{}

func (reviewNotifier) Notify(context.Context, notification.NotifyRequest) error { return nil }

func (reviewNotifier) NotifyTx(ctx context.Context, _ *gorm.DB, req notification.NotifyRequest) error {
	return nil
}

func (reviewNotifier) ForRoles(context.Context, ...string) ([]notification.Ref, error) {
	return nil, nil
}

func seededReviews(t *testing.T) (*Handler, *gorm.DB, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// ReviewHelpful is migrated too: GetCollegeReviews reads vote counts through
	// it, and without the table the college read 500s for reasons unrelated to
	// authorisation — which is exactly how a real regression hides behind a
	// "looks like it was already failing" shrug.
	if err := db.AutoMigrate(&Review{}, &ReviewReport{}, &ReviewHelpful{}, &DateReport{}, &auth.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// A review OF institution 9, naming a student. The other institution's review
	// is the one an institution account must never be able to touch.
	mine := Review{
		UserID: 42, InstitutionID: 9, CollegeID: 7, CollegeName: "St. Xavier",
		StudentType: "current", Course: "BSc CS", BatchYear: 2024,
		Ratings: []byte(`{"overall":4.5}`), Pros: "Good teachers here.",
		Cons: "Fees are high.", Email: "grace@example.com", IsPublished: true,
	}
	theirs := Review{
		UserID: 43, InstitutionID: 77, CollegeID: 8, CollegeName: "Other College",
		StudentType: "alumni", Course: "BSc Physics", BatchYear: 2023,
		Ratings: []byte(`{"overall":3.0}`), Pros: "Fine for physics.",
		Cons: "Lab closures.", Email: "alan@example.com", IsPublished: true,
	}
	// A university review, the other admin surface.
	university := Review{
		UserID: 44, UniversityID: 3, StudentType: "alumni", BatchYear: 2022,
		Ratings: []byte(`{"overall":5}`), Pros: "Excellent research.",
		Cons: "Cold winters.", IsPublished: true,
	}
	report := DateReport{
		UniversityID: 3, UniversityName: "TU", Contact: "9800000000",
		Feedback: "The site is broken", Status: "pending",
	}
	for _, row := range []interface{}{&mine, &theirs, &university, &report} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	svc := NewService(NewRepository(db), &reviewNotifier{})
	return NewHandler(svc), db, mine.ID, theirs.ID
}

func reviewRoutes(h *Handler, role string, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Next()
	})
	RegisterRoutes(r, func(c *gin.Context) { c.Next() }, func(c *gin.Context) { c.Next() }, h)
	return r
}

func call(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// THE load-bearing test. A tenant must not be able to delete another institution's
// review, or another tenant's, or one naming an individual student.
func TestTenantCannotDeleteAnyReview(t *testing.T) {
	h, db, mineID, theirsID := seededReviews(t)
	universityReview := Review{}
	if err := db.Where("university_id > 0").First(&universityReview).Error; err != nil {
		t.Fatalf("read the university review: %v", err)
	}

	for _, role := range []string{"institution", "scholarship_provider", "scholarship-provider", "scholarship_provider_subuser"} {
		r := reviewRoutes(h, role, 9)
		for _, id := range []uint{theirsID, mineID, universityReview.ID} {
			w := call(r, "DELETE", "/api/v1/institution/reviews/"+utoa(id))
			if w.Code == http.StatusOK || w.Code == http.StatusNoContent {
				t.Errorf("role %q deleted review %d: %d — body %s", role, id, w.Code, w.Body.String())
			}
			// The route is gone, so any code other than a success is a pass; what
			// must never happen is the row disappearing.
			if err := db.First(&Review{}, id).Error; err != nil {
				t.Errorf("role %q deleted review %d anyway: %v", role, id, err)
			}
		}
	}
	_ = mineID
}

// The moderation queues are the platform's, not a tenant's.
func TestTenantCannotReachTheReviewModerationQueues(t *testing.T) {
	h, db, _, _ := seededReviews(t)
	var report DateReport
	if err := db.First(&report).Error; err != nil {
		t.Fatalf("read the date report: %v", err)
	}

	for _, role := range []string{"institution", "scholarship_provider", "scholarship_provider_subuser"} {
		r := reviewRoutes(h, role, 9)
		for _, tc := range []struct{ method, path string }{
			{"GET", "/api/v1/admin/university-reviews/3"},
			{"GET", "/api/v1/admin/date-reports"},
			{"PUT", "/api/v1/admin/date-reports/" + utoa(report.ID)},
			{"DELETE", "/api/v1/admin/date-reports/" + utoa(report.ID)},
		} {
			w := call(r, tc.method, tc.path)
			if w.Code != http.StatusForbidden {
				t.Errorf("role %q %s %s returned %d, want 403 — body %s", role, tc.method, tc.path, w.Code, w.Body.String())
			}
			// DateReport.Contact is a phone number from a PUBLIC, unauthenticated
			// submission. Assert on the body, not only the code.
			if strings.Contains(w.Body.String(), "9800000000") {
				t.Errorf("role %q %s %s leaked a submitter's phone number", role, tc.method, tc.path)
			}
		}
	}
	if err := db.First(&report, report.ID).Error; err != nil {
		t.Errorf("a refused delete removed the date report: %v", err)
	}
}

// The positive paths, which a sweep that only proves refusals would happily break.
func TestInstitutionKeepsItsOwnReviewReads(t *testing.T) {
	h, _, _, _ := seededReviews(t)
	r := reviewRoutes(h, "institution", 9)

	w := call(r, "GET", "/api/v1/institution/reviews")
	if w.Code != http.StatusOK {
		t.Fatalf("an institution reading its own reviews got %d, want 200 — body %s", w.Code, w.Body.String())
	}
	// Its own review is in there; the other institution's is not.
	if !strings.Contains(w.Body.String(), "Good teachers here.") {
		t.Errorf("the institution lost its own review: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Fine for physics.") {
		t.Errorf("institution 9 was served institution 77's review: %s", w.Body.String())
	}
	if w := call(r, "GET", "/api/v1/institution/reviews/college/7"); w.Code != http.StatusOK {
		t.Errorf("institution reading a college's reviews got %d, want 200", w.Code)
	}
}

func TestPlatformAdminKeepsEveryModerationRoute(t *testing.T) {
	h, db, _, theirsID := seededReviews(t)
	var report DateReport
	if err := db.First(&report).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	r := reviewRoutes(h, "superadmin", 1)

	if w := call(r, "GET", "/api/v1/admin/university-reviews/3"); w.Code != http.StatusOK {
		t.Errorf("admin university reviews got %d, want 200 — body %s", w.Code, w.Body.String())
	}
	w := call(r, "GET", "/api/v1/admin/date-reports")
	if w.Code != http.StatusOK {
		t.Fatalf("admin date reports got %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "9800000000") {
		t.Errorf("the operator lost the submitter's contact: %s", w.Body.String())
	}
	// And the delete still works for the operator — the route is gated, not gone.
	if w := call(r, "DELETE", "/api/v1/admin/university-reviews/"+utoa(theirsID)); w.Code != http.StatusOK {
		t.Errorf("admin deleting a review got %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if err := db.First(&Review{}, theirsID).Error; err == nil {
		t.Error("the admin's delete did not take effect")
	}
	_ = report
}

func utoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }
