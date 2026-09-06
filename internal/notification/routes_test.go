package notification

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

func TestV2EnabledFlag(t *testing.T) {
	t.Setenv("NOTIFICATIONS_V2", "")
	if !V2Enabled() {
		t.Fatal("default must be on")
	}
	t.Setenv("NOTIFICATIONS_V2", "on")
	if !V2Enabled() {
		t.Fatal("on must enable v2")
	}
	t.Setenv("NOTIFICATIONS_V2", "off")
	if V2Enabled() {
		t.Fatal("off must disable v2")
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
