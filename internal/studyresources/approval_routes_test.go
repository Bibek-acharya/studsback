// internal/studyresources/approval_routes_test.go
//
// The §5.3 HTTP surface: a student's submission, the moderation queue, and the
// two decisions.
//
// What is pinned here and not in approval_test.go is the ROUTING — which group
// each handler is mounted on, and therefore who can reach it. approval_test.go
// proves the state machine; this proves that a student cannot reach an admin
// handler and that the queue is not on the public route.

package studyresources

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

func approvalRouteServer(t *testing.T, role string, userID uint) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// authMW stands in for the real one: it establishes the two context values
	// every handler below reads, which is the whole contract the real middleware
	// provides.
	auth := func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Next()
	}
	// A real repository, because MyUploads actually queries. NewService(nil) would
	// panic in the handler rather than fail an assertion, and a panic in a routing
	// test hides the thing it exists to check.
	reads, approval := approvalFixture(t, &countingGrant{})
	RegisterRoutes(r, auth, middleware.RequireRole("superadmin", "super_admin"), NewHandler(reads).WithApproval(approval))
	return r
}

func callRoute(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// The moderation decisions must be unreachable to a student. The role gate does
// this, and the assertion is on the STATUS because a handler that ran anyway
// would answer 404 for a nonexistent id — which is a different failure and would
// be masked by a looser check.
func TestAStudentCannotReachTheModerationDecisions(t *testing.T) {
	for _, role := range []string{"user", "student", "institution", "scholarship_provider"} {
		r := approvalRouteServer(t, role, 42)
		for _, tc := range []struct{ method, path string }{
			{"POST", "/api/v1/admin/study-resources/1/approve"},
			{"POST", "/api/v1/admin/study-resources/1/reject"},
			{"GET", "/api/v1/admin/study-resources/pending"},
		} {
			w := callRoute(r, tc.method, tc.path)
			if w.Code != http.StatusForbidden {
				t.Errorf("role %q reached %s %s: got %d, want 403 — body %s",
					role, tc.method, tc.path, w.Code, w.Body.String())
			}
		}
	}
}

// And the student CAN submit, on a session-only route with no role gate. This is
// the positive half: a submission route that needs superadmin would make the whole
// earn mechanic unreachable.
func TestAStudentCanSubmitWithoutAnAdminRole(t *testing.T) {
	r := approvalRouteServer(t, "user", 42)
	// No file in the body, so the handler stops at the file check — the point is
	// that the route matched and did NOT answer 403.
	w := callRoute(r, "POST", "/api/v1/study-resources")
	if w.Code == http.StatusForbidden {
		t.Errorf("a student was refused the submission route: %s", w.Body.String())
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("a file-less submission answered %d, want 400 from the file check — if it is 404 or 403 the route is not mounted as intended (body %s)",
			w.Code, w.Body.String())
	}
	if w := callRoute(r, "GET", "/api/v1/study-resources/mine"); w.Code != http.StatusOK {
		t.Errorf("a student's my-uploads answered %d, want 200 — body %s", w.Code, w.Body.String())
	}
}
