// internal/notification/handler_test.go
package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func studentRouter(h *Handler, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_role", "student")
		c.Set("user_id", userID)
		c.Set("provider_id", uint(0))
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"))
	return r
}

func TestListResponseIsTransitionEnvelope(t *testing.T) {
	db := testDB(t)
	h := NewHandler(NewService(db))
	if err := NewRepository(db).InsertNotifications(nil, []AccountNotification{
		{AccountType: "user", AccountID: 42, EventKey: EventApplicationStatusChanged,
			Category: "application", Priority: PriorityCritical, Title: "T", Body: "B"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := studentRouter(h, 42)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/notifications?page=1&limit=20", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Notifications []map[string]any `json:"notifications"`
			UnreadCount   int              `json:"unread_count"`
			Meta          struct {
				Total int64 `json:"total"`
			} `json:"meta"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	item := resp.Data.Notifications[0]
	// New-contract fields:
	if item["event_key"] != EventApplicationStatusChanged || item["category"] != "application" {
		t.Fatalf("new fields wrong: %v", item)
	}
	// Envelope fields (doc 05 §2.2):
	for _, k := range []string{"message", "type", "read", "user_id", "updated_at"} {
		if _, ok := item[k]; !ok {
			t.Fatalf("envelope field %q missing", k)
		}
	}
	if item["message"] != "B" || item["type"] != "application" || item["read"] != false {
		t.Fatalf("envelope values wrong: message=%v type=%v read=%v", item["message"], item["type"], item["read"])
	}
	if resp.Data.UnreadCount != 1 || resp.Data.Meta.Total != 1 || resp.Message == "" {
		t.Fatalf("envelope meta wrong: unread=%d total=%d message=%q", resp.Data.UnreadCount, resp.Data.Meta.Total, resp.Message)
	}
}

func TestInboxIsolationBetweenRoles(t *testing.T) {
	db := testDB(t)
	h := NewHandler(NewService(db))
	if err := NewRepository(db).InsertNotifications(nil, []AccountNotification{
		{AccountType: "institution", AccountID: 7, EventKey: EventApplicationReceived, Category: "application", Title: "inst-only"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := studentRouter(h, 42)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil))
	var resp struct {
		Data struct {
			Notifications []map[string]any `json:"notifications"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Notifications) != 0 {
		t.Fatalf("student saw institution inbox: %d items", len(resp.Data.Notifications))
	}
}

func TestMarkReadAndReadAll(t *testing.T) {
	db := testDB(t)
	h := NewHandler(NewService(db))
	if err := NewRepository(db).InsertNotifications(nil, []AccountNotification{
		{AccountType: "user", AccountID: 42, EventKey: EventAccountWelcome, Category: "account", Title: "a"},
		{AccountType: "user", AccountID: 42, EventKey: EventAccountWelcome, Category: "account", Title: "b"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := studentRouter(h, 42)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/v1/notifications/read-all", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("read-all status=%d", w.Code)
	}
	if n, _ := NewRepository(db).UnreadCount("user", 42); n != 0 {
		t.Fatalf("unread after read-all = %d", n)
	}
	// Cross-account read must 404.
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPut, "/api/v1/notifications/999999/read", nil))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("missing item status=%d want 404", w2.Code)
	}
}
