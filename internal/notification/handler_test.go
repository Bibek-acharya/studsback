// internal/notification/handler_test.go
package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
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
	h.RegisterRoutes(r.Group("/api/v1"), nil)
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

func TestArchiveOwnershipAndMissing(t *testing.T) {
	db := testDB(t)
	h := NewHandler(NewService(db))
	rows := []AccountNotification{
		{AccountType: "user", AccountID: 42, EventKey: EventAccountWelcome, Category: "account", Title: "mine"},
		{AccountType: "institution", AccountID: 7, EventKey: EventApplicationReceived, Category: "application", Title: "foreign"},
	}
	if err := NewRepository(db).InsertNotifications(nil, rows); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := studentRouter(h, 42)

	// Archive own notification → 200.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/notifications/%d/archive", rows[0].ID), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("own archive status=%d body=%s", w.Code, w.Body.String())
	}

	// Archive a foreign account's notification → 404.
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/notifications/%d/archive", rows[1].ID), nil))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("foreign archive status=%d want 404", w2.Code)
	}

	// Archive a nonexistent notification → 404.
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodPut, "/api/v1/notifications/999999/archive", nil))
	if w3.Code != http.StatusNotFound {
		t.Fatalf("missing archive status=%d want 404", w3.Code)
	}

	// Unarchive own → 200; foreign unarchive → 404 too.
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/notifications/%d/unarchive", rows[0].ID), nil))
	if w4.Code != http.StatusOK {
		t.Fatalf("own unarchive status=%d body=%s", w4.Code, w4.Body.String())
	}
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/notifications/%d/unarchive", rows[1].ID), nil))
	if w5.Code != http.StatusNotFound {
		t.Fatalf("foreign unarchive status=%d want 404", w5.Code)
	}
}

func providerRouter(h *Handler, providerID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// Distinct values (real middleware sets user_id = users-table ID,
		// provider_id = claims.ProviderID) prove the proxy scopes by provider_id.
		c.Set("user_role", "scholarship_provider")
		c.Set("user_id", providerID*10)
		c.Set("provider_id", providerID)
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"), nil)
	return r
}

func TestProviderProxyServesLegacyShape(t *testing.T) {
	db := testDB(t)
	h := NewHandler(NewService(db))
	if err := NewRepository(db).InsertNotifications(nil, []AccountNotification{
		{AccountType: "provider", AccountID: 9, EventKey: EventApplicationReceived,
			Category: "application", Title: "New Application Received", Body: "Rita applied."},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := providerRouter(h, 9)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/scholarship-providers/notifications?page=1&limit=20", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Notifications []map[string]any `json:"notifications"`
			UnreadCount   int              `json:"unread_count"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	item := resp.Data.Notifications[0]
	for _, k := range []string{"provider_id", "message", "type", "read", "created_at"} {
		if _, ok := item[k]; !ok {
			t.Fatalf("provider legacy field %q missing", k)
		}
	}
	if item["provider_id"] != float64(9) || item["message"] != "Rita applied." || resp.Data.UnreadCount != 1 {
		t.Fatalf("proxy values wrong: %v unread=%d", item, resp.Data.UnreadCount)
	}
}

func TestBroadcastCreatesCampaignAndFansOut(t *testing.T) {
	db := testDB(t)
	// The audience query unions the institution/provider account tables —
	// minimal stubs are created by testDB (this test only reads them, so no
	// row cleanup is needed).
	db.Exec(`INSERT INTO users (email, first_name, last_name, role, status, created_at, updated_at) VALUES
		('notif-u1@test.local','Notif','U1','student','active',now(),now()),
		('notif-u2@test.local','Notif','U2','student','active',now(),now())`)
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE email LIKE 'notif-u%@test.local'`) })

	svc := NewService(db)
	// Expansion enqueues the fanout row's process task — stub the enqueuer.
	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue })
	svc.SetEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		return &asynq.TaskInfo{ID: "test"}, nil
	})
	h := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_role", "superadmin")
		c.Set("user_id", uint(1))
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"), nil)

	body := `{"title":"Maintenance","body":"Sunday 02:00-04:00","link":"/news/m","audience":["all"],"idempotency_key":"maint-1"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/notifications/broadcast", strings.NewReader(body)))
	if w.Code != http.StatusAccepted {
		t.Fatalf("broadcast status=%d body=%s", w.Code, w.Body.String())
	}
	// Same idempotency key returns the same campaign, not a second one.
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/api/v1/notifications/broadcast", strings.NewReader(body)))
	if w2.Code != http.StatusOK {
		t.Fatalf("idempotent replay status=%d", w2.Code)
	}
	var first, second struct {
		Data struct {
			BroadcastID uint `json:"broadcast_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &first)
	_ = json.Unmarshal(w2.Body.Bytes(), &second)
	if first.Data.BroadcastID == 0 || first.Data.BroadcastID != second.Data.BroadcastID {
		t.Fatalf("idempotency broken: %d vs %d", first.Data.BroadcastID, second.Data.BroadcastID)
	}
	// Expand fan-out synchronously (the poller does this in production).
	if err := svc.ExpandFanoutPending(context.Background()); err != nil {
		t.Fatalf("expand: %v", err)
	}
	var n int64
	db.Table("account_notifications").Where("account_type='user' AND event_key='system.announcement'").Count(&n)
	if n != 2 {
		t.Fatalf("fanout rows=%d want 2", n)
	}
	// Replay expansion — occurrence keys must make it harmless.
	if err := svc.ExpandFanoutPending(context.Background()); err != nil {
		t.Fatalf("re-expand: %v", err)
	}
	db.Table("account_notifications").Where("account_type='user' AND event_key='system.announcement'").Count(&n)
	if n != 2 {
		t.Fatalf("fanout replay duplicated rows: %d", n)
	}
}
