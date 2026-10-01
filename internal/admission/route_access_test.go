package admission

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/notification"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// The ROUTE-level guard for /admin/admissions.
//
// access_test.go pins the role list; this pins that the routes actually use it,
// and that the applicant routes actually refuse a non-owner. Those are separate
// claims, and the vulnerability lived in exactly the gap: the list looked plausible
// and the gate was the wide shared roleMW.

func seededAdmissions(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Admission{}, &College{}, &User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	applicant := uint(42)
	orphan := Admission{
		CollegeID: 7, ProgramName: "BSc CS", ProgramLevel: "undergraduate",
		StudentName: "Grace Hopper", StudentEmail: "grace@example.com",
		StudentPhone: "9800000000", Gender: "female", City: "Kirtipur",
		Address: "12 Lalitpur", EntranceScore: "88", Status: "pending",
		UserID: &applicant,
	}
	// An application submitted with NO account. Before the fix this one skipped the
	// ownership comparison entirely, because the handler's check was conditional on
	// UserID being non-nil — so it was readable by any authenticated user.
	anonymous := Admission{
		CollegeID: 7, ProgramName: "BSc Physics", ProgramLevel: "undergraduate",
		StudentName: "Ada Lovelace", StudentEmail: "ada@example.com",
		StudentPhone: "9800000001", Gender: "female", City: "Baneshwor",
		Address: "3 Lalitpur", EntranceScore: "91", Status: "pending",
	}
	for _, row := range []interface{}{&orphan, &anonymous} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	repo := NewRepository(db)
	svc := NewService(repo, &noopNotifier{})
	return NewHandler(svc), db
}

// noopNotifier satisfies the full notification.Notifier interface. UpdateStatus
// notifies the applicant, and a nil notifier would panic there rather than fail,
// so a stub is required — these tests are about authorisation, not notification.
type noopNotifier struct{}

func (noopNotifier) Notify(ctx context.Context, req notification.NotifyRequest) error { return nil }

func (noopNotifier) NotifyTx(ctx context.Context, tx *gorm.DB, req notification.NotifyRequest) error {
	return nil
}

func (noopNotifier) ForRoles(ctx context.Context, roles ...string) ([]notification.Ref, error) {
	return nil, nil
}

func admissionRoutes(h *Handler, role string, userID uint) *gin.Engine {
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

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// THE load-bearing test: an ordinary paying tenant gets 403 and no applicant PII
// from any of the four operator routes.
func TestTenantCannotReadOrWriteTheAdmissionsQueue(t *testing.T) {
	h, db := seededAdmissions(t)
	var row Admission
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("read: %v", err)
	}

	for _, role := range []string{"institution", "scholarship_provider", "scholarship-provider", "scholarship_provider_subuser"} {
		r := admissionRoutes(h, role, 99)
		for _, tc := range []struct{ method, path, body string }{
			{"GET", "/api/v1/admin/admissions", ""},
			{"GET", "/api/v1/admin/admissions/" + u64s(row.ID), ""},
			{"GET", "/api/v1/admin/admissions/college/7", ""},
			{"PUT", "/api/v1/admin/admissions/" + u64s(row.ID) + "/status", `{"status":"approved","notes":"x"}`},
		} {
			w := do(r, tc.method, tc.path, tc.body)
			if w.Code != http.StatusForbidden {
				t.Errorf("role %q %s %s returned %d, want 403 — body %s",
					role, tc.method, tc.path, w.Code, w.Body.String())
			}
			// The disclosure this guards: an applicant's name, email, phone and
			// address. Asserted on the body rather than the status code alone,
			// because a 403 with the row in the payload is still a leak.
			for _, pii := range []string{"Grace Hopper", "grace@example.com", "9800000000", "12 Lalitpur"} {
				if strings.Contains(w.Body.String(), pii) {
					t.Errorf("role %q %s %s leaked %q in a %d body", role, tc.method, tc.path, pii, w.Code)
				}
			}
		}
	}

	// And the refusal left the application alone.
	var after Admission
	if err := db.First(&after, row.ID).Error; err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if after.Status != "pending" {
		t.Errorf("a refused tenant write changed the status to %q", after.Status)
	}
	if after.ReviewedBy != nil {
		t.Errorf("a refused tenant write stamped reviewed_by = %v", *after.ReviewedBy)
	}
}

// The operator keeps the whole queue. The fix must not cost them the route.
func TestPlatformAdminStillReadsTheQueueAndSetsStatus(t *testing.T) {
	h, db := seededAdmissions(t)
	var row Admission
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	r := admissionRoutes(h, "superadmin", 1)

	if w := do(r, "GET", "/api/v1/admin/admissions", ""); w.Code != http.StatusOK {
		t.Fatalf("admin list returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	w := do(r, "GET", "/api/v1/admin/admissions/college/7", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin college list returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Grace Hopper") {
		t.Errorf("the operator's college list lost its own applicant: %s", w.Body.String())
	}
	w = do(r, "PUT", "/api/v1/admin/admissions/"+u64s(row.ID)+"/status", `{"status":"approved","notes":"strong application"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("admin status write returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	var after Admission
	if err := db.First(&after, row.ID).Error; err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if after.Status != "approved" {
		t.Errorf("status = %q, want %q", after.Status, "approved")
	}
	if after.ReviewedBy == nil || *after.ReviewedBy != 1 {
		t.Errorf("reviewed_by = %v, want 1 (the operator, not the applicant)", after.ReviewedBy)
	}
}

// The applicant can read their own, and only their own.
func TestApplicantReadsTheirOwnApplicationAndNotAnother(t *testing.T) {
	h, db := seededAdmissions(t)
	var mine Admission
	if err := db.Where("user_id = ?", 42).First(&mine).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	var anon Admission
	if err := db.Where("user_id IS NULL").First(&anon).Error; err != nil {
		t.Fatalf("read anonymous: %v", err)
	}
	// A student, not an operator: they satisfy the applicant group but not the
	// admin gate, which is the pair that must both be true of nobody.
	r := admissionRoutes(h, "user", 42)

	if w := do(r, "GET", "/api/v1/admissions/"+u64s(mine.ID), ""); w.Code != http.StatusOK {
		t.Fatalf("applicant reading their own application returned %d, want 200 — body %s", w.Code, w.Body.String())
	}
	if w := do(r, "GET", "/api/v1/admin/admissions", ""); w.Code != http.StatusForbidden {
		t.Errorf("an applicant reading the operator queue got %d, want 403", w.Code)
	}
	// Someone else's, and one submitted without an account.
	if w := do(r, "GET", "/api/v1/admissions/"+u64s(anon.ID), ""); w.Code != http.StatusNotFound {
		t.Errorf("applicant reading an accountless application got %d, want 404", w.Code)
	}
	if w := do(r, "GET", "/api/v1/admissions/999999", ""); w.Code != http.StatusNotFound {
		t.Errorf("applicant reading a missing id got %d, want 404", w.Code)
	}
}

func u64s(v uint) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
