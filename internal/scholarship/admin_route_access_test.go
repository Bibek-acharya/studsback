package scholarship

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// The ROUTE-level guard for /admin.
//
// access_test.go pins the role list; this pins that the group actually uses it.
// They are separate claims, and the vulnerability lived in exactly the gap: the
// group was wired to the shared multi-tenant roleMW, which no test in this module
// asserted anything about.
//
// The PII assertions matter more than the status codes here. ScholarshipApplication
// carries a guardian's name and phone, both parents' occupations, household monthly
// income, family size, and the permanent and temporary address down to ward and
// tole — so a 403 that still serialised the row would be the same disclosure.

func seededApplications(t *testing.T) (*Handler, *gorm.DB, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Scholarship{}, &ScholarshipApplication{}, &User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	applicant := uint(42)
	app := ScholarshipApplication{
		ScholarshipID: 1, UserID: &applicant, FullName: "Grace Hopper",
		Gender: "female", Ethnicity: "hill", DateOfBirthBS: "2007/01/01", Age: 19,
		PhoneNumber: "9800000000", Email: "grace@example.com",
		SchoolName: "St. Xavier", SchoolTole: "Lalitpur-4",
		PermanentWard: " ward-32", PermanentTole: " tole-9",
		GuardianName: "Mary Hopper", GuardianPhone: "9800000009",
		FatherOccupation: "farmer", MotherOccupation: "teacher",
		FamilyMonthlyIncome: 18500, FamilyMembersCount: 5,
		Status: "submitted",
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc := NewService(NewRepository(db), db, nil, &captureNotifier{})
	return NewHandler(svc, NewPaymentService(db, &captureNotifier{})), db, app.ID
}

func adminRoutes(h *Handler, role string, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Next()
	})
	RegisterRoutes(r, func(c *gin.Context) { c.Next() }, h)
	return r
}

func req(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	r2 := httptest.NewRequest(method, path, strings.NewReader(body))
	r2.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, r2)
	return w
}

// THE load-bearing test. An ordinary paying tenant gets 403 and no applicant PII
// from any of the admin routes.
func TestTenantCannotReadTheScholarshipAdminQueue(t *testing.T) {
	h, db, appID := seededApplications(t)
	// The PII a leak would expose. Guardian contact, household income, and the
	// address to ward and tole are the fields no scholarship platform should hand
	// to a tenant that did not receive the application.
	pii := []string{"Grace Hopper", "grace@example.com", "Mary Hopper", "9800000009",
		"18500", "farmer", "ward-32", "tole-9"}

	for _, role := range []string{"institution", "scholarship_provider", "scholarship-provider", "scholarship_provider_subuser"} {
		r := adminRoutes(h, role, 99)
		for _, tc := range []struct{ method, path, body string }{
			{"GET", "/api/v1/admin/scholarship-applications", ""},
			{"GET", "/api/v1/admin/scholarship-applications/" + utoa(appID), ""},
			{"GET", "/api/v1/admin/scholarship-applications/scholarship/1", ""},
			{"GET", "/api/v1/admin/scholarships/list", ""},
			{"PUT", "/api/v1/admin/scholarship-applications/" + utoa(appID) + "/status", `{"status":"approved"}`},
		} {
			w := req(r, tc.method, tc.path, tc.body)
			if w.Code != http.StatusForbidden {
				t.Errorf("role %q %s %s returned %d, want 403 — body %s",
					role, tc.method, tc.path, w.Code, w.Body.String())
			}
			for _, p := range pii {
				if strings.Contains(w.Body.String(), p) {
					t.Errorf("role %q %s %s leaked %q in a %d body", role, tc.method, tc.path, p, w.Code)
				}
			}
		}
	}

	// And the refusals changed nothing.
	var after ScholarshipApplication
	if err := db.First(&after, appID).Error; err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if after.Status != "submitted" {
		t.Errorf("a refused tenant write changed the status to %q", after.Status)
	}
}

// The operator keeps the queue. Narrowing it must not cost them the route.
func TestPlatformAdminStillReadsTheScholarshipQueue(t *testing.T) {
	h, _, appID := seededApplications(t)
	r := adminRoutes(h, "superadmin", 1)

	w := req(r, "GET", "/api/v1/admin/scholarship-applications", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin list returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Grace Hopper") {
		t.Errorf("the operator's list lost its own applicant: %s", w.Body.String())
	}
	if w := req(r, "PUT", "/api/v1/admin/scholarship-applications/"+utoa(appID)+"/status", `{"status":"approved"}`); w.Code != http.StatusOK {
		t.Errorf("admin status write returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
}

// A provider reviewing applications to its OWN scholarships is a real capability
// and lives in internal/scholarshipprovider. This pins that narrowing /admin did
// not touch the applicant-facing routes on this module either.
func TestApplicantRoutesStillServeTheApplicant(t *testing.T) {
	h, db, appID := seededApplications(t)
	r := adminRoutes(h, "user", 42)

	w := req(r, "GET", "/api/v1/scholarships/my-applications", "")
	if w.Code != http.StatusOK {
		t.Fatalf("applicant's own list returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Grace Hopper") {
		t.Errorf("the applicant cannot see their own application: %s", w.Body.String())
	}
	// And not somebody else's.
	other := adminRoutes(h, "user", 77)
	if w := req(other, "GET", "/api/v1/scholarships/applications/"+utoa(appID), ""); w.Code == http.StatusOK {
		t.Errorf("a non-owner read another applicant's application: %s", w.Body.String())
	}
	_ = db
}

func utoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }
