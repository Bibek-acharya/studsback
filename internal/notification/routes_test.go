package notification

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

// Rollback retired (P2.5 Task 4): routes register unconditionally — the
// NOTIFICATIONS_V2 env var is inert and must not gate anything.
func TestRoutesAlwaysRegistered(t *testing.T) {
	t.Setenv("NOTIFICATIONS_V2", "off") // legacy var; registration must ignore it
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(nil).RegisterRoutes(r.Group("/api/v1"), nil)

	paths := map[string]bool{}
	for _, ri := range r.Routes() {
		paths[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/notifications",
		"GET /api/v1/notifications/unread-count",
		"PUT /api/v1/notifications/read-all",
		"PUT /api/v1/notifications/:id/read",
	} {
		if !paths[want] {
			t.Errorf("missing route %s (flag must not gate registration)", want)
		}
	}
}

// The broadcast endpoints must reject non-superadmins at the route level:
// the wiring site passes middleware.RequireRole("superadmin", "super_admin").
func TestBroadcastRequiresSuperadmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_role", "student")
		c.Set("user_id", uint(1))
		c.Next()
	})
	NewHandler(nil).RegisterRoutes(r.Group("/api/v1"), middleware.RequireRole("superadmin", "super_admin"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/notifications/broadcast", strings.NewReader(`{}`)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-superadmin broadcast status=%d body=%s", w.Code, w.Body.String())
	}
}

// The public banner admin verbs on /system/notifications must reject
// non-superadmins at the route level, same as the broadcast endpoints.
func TestBannerAdminRequiresSuperadmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_role", "student")
		c.Set("user_id", uint(1))
		c.Next()
	})
	NewHandler(nil).RegisterRoutes(r.Group("/api/v1"), middleware.RequireRole("superadmin", "super_admin"))

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/system/notifications"},
		{http.MethodGet, "/api/v1/system/notifications/all"},
		{http.MethodPut, "/api/v1/system/notifications/1"},
		{http.MethodDelete, "/api/v1/system/notifications/1"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s status=%d want 403", tc.method, tc.path, w.Code)
		}
	}
}
