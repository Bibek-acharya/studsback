package analytics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"studsphere/backend/internal/shared/middleware"
)

func testRouter() *gin.Engine {
	r := gin.New()
	h := NewHandler(NewService(nil, NewUsageTracker(0)))
	h.RegisterRoutes(r,
		func(c *gin.Context) { c.Next() }, // stand-in Auth: no claims needed for guard test
		middleware.RequireRole("superadmin", "super_admin"))
	return r
}

func TestAnalyticsRequiresSuperadmin(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_role", "student")
		c.Set("user_id", uint(1))
	})
	NewHandler(NewService(nil, NewUsageTracker(0))).RegisterRoutes(r,
		func(c *gin.Context) { c.Next() },
		middleware.RequireRole("superadmin", "super_admin"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/superadmin/analytics/users", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", w.Code)
	}
}
