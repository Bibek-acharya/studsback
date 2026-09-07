// internal/notification/banner_admin_test.go
package notification_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/system"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// bannerDB opens the shared Postgres test DB (same DSN contract as the
// internal harness: skip, never fail, without TEST_DATABASE_DSN). Only the
// public_notifications table is needed; rows are self-cleaned by marker.
func bannerDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; notification SQL integration tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// system.PublicNotification is the notification-owned model via alias —
	// one table, one AutoMigrate entry.
	if err := db.AutoMigrate(&system.PublicNotification{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM public_notifications WHERE title LIKE 'banner-e2e-%'`)
	})
	return db
}

// bannerRouter mounts the real system module (guest GET) and the real
// notification module (admin CRUD guarded by the production middleware) on one
// engine — exactly the production wiring shape at /api/v1.
func bannerRouter(t *testing.T, db *gorm.DB, role string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_role", role)
		c.Set("user_id", uint(1))
		c.Next()
	})
	passThrough := func(c *gin.Context) { c.Next() }
	sysH := system.NewHandler(system.NewService(system.NewRepository(db), nil))
	system.RegisterRoutes(r, passThrough, passThrough, sysH)
	notifH := notification.NewHandler(notification.NewService(db))
	notifH.RegisterRoutes(r.Group("/api/v1"), middleware.RequireRole("superadmin", "super_admin"))
	return r
}

func TestPublicBannerAdminCRUD(t *testing.T) {
	db := bannerDB(t)
	r := bannerRouter(t, db, "superadmin")

	// CREATE → 201, row persisted active.
	body := `{"title":"banner-e2e-maintenance","message":"Sunday 02:00-04:00","type":"warning","link":"/news/m","icon":"fa-wrench","color":"text-amber-600","bg_color":"bg-amber-100"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/system/notifications", strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			ID    uint   `json:"id"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	if created.Data.ID == 0 || created.Data.Title != "banner-e2e-maintenance" || created.Data.Type != "warning" {
		t.Fatalf("create payload wrong: %+v", created.Data)
	}
	var n int64
	db.Table("public_notifications").Where("title = ? AND active = ?", "banner-e2e-maintenance", true).Count(&n)
	if n != 1 {
		t.Fatalf("persisted rows=%d want 1", n)
	}

	// Guest list shows it with the full banner shape.
	guestItems := func() []map[string]any {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system/notifications", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("guest list status=%d body=%s", w.Code, w.Body.String())
		}
		var resp struct {
			Data []map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.Data
	}
	findBanner := func(items []map[string]any) map[string]any {
		for _, it := range items {
			if it["title"] == "banner-e2e-maintenance" {
				return it
			}
		}
		return nil
	}
	banner := findBanner(guestItems())
	if banner == nil {
		t.Fatal("guest list missing created banner")
	}
	for _, k := range []string{"id", "created_at", "title", "message", "type", "link", "icon", "color", "bg_color"} {
		if _, ok := banner[k]; !ok {
			t.Fatalf("guest banner field %q missing", k)
		}
	}
	if banner["message"] != "Sunday 02:00-04:00" || banner["icon"] != "fa-wrench" || banner["bg_color"] != "bg-amber-100" {
		t.Fatalf("guest banner values wrong: %v", banner)
	}

	// PUT deactivate → guest list excludes it.
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/system/notifications/%d", created.Data.ID), strings.NewReader(`{"active":false}`)))
	if w2.Code != http.StatusOK {
		t.Fatalf("deactivate status=%d body=%s", w2.Code, w2.Body.String())
	}
	if findBanner(guestItems()) != nil {
		t.Fatal("guest list still shows deactivated banner")
	}

	// GET /all (superadmin) includes the inactive banner.
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/api/v1/system/notifications/all", nil))
	if w3.Code != http.StatusOK {
		t.Fatalf("list-all status=%d body=%s", w3.Code, w3.Body.String())
	}
	var all struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &all)
	if findBanner(all.Data) == nil {
		t.Fatal("list-all missing inactive banner")
	}

	// DELETE → soft delete: row gone from /all, deleted_at set in DB.
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/system/notifications/%d", created.Data.ID), nil))
	if w4.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w4.Code, w4.Body.String())
	}
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, httptest.NewRequest(http.MethodGet, "/api/v1/system/notifications/all", nil))
	_ = json.Unmarshal(w5.Body.Bytes(), &all)
	if findBanner(all.Data) != nil {
		t.Fatal("list-all still shows soft-deleted banner")
	}
	db.Table("public_notifications").Where("title = ? AND deleted_at IS NOT NULL", "banner-e2e-maintenance").Count(&n)
	if n != 1 {
		t.Fatalf("soft-deleted rows=%d want 1", n)
	}
}

func TestPublicBannerAdminRejectsNonSuperadmin(t *testing.T) {
	db := bannerDB(t)
	r := bannerRouter(t, db, "student")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/system/notifications", strings.NewReader(`{}`)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("student create status=%d want 403", w.Code)
	}
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/system/notifications/all", nil))
	if w2.Code != http.StatusForbidden {
		t.Fatalf("student list-all status=%d want 403", w2.Code)
	}
}
