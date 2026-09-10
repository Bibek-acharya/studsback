// studsback/internal/analytics/usage_test.go
package analytics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestUsageSnapshotCountsStatusesAndTopPaths(t *testing.T) {
	tracker := NewUsageTracker(30 * time.Minute)
	r := gin.New()
	r.Use(tracker.Middleware())
	r.GET("/api/v1/users/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	r.GET("/public", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public", nil))

	snap := tracker.Snapshot()
	if snap.TotalRequests != 4 {
		t.Fatalf("total=%d want 4 (non-/api/ excluded)", snap.TotalRequests)
	}
	if snap.ServerErrors != 1 {
		t.Fatalf("5xx=%d want 1", snap.ServerErrors)
	}
	if len(snap.TopEndpoints) == 0 || snap.TopEndpoints[0].Path != "/api/v1/users/:id" || snap.TopEndpoints[0].Count != 3 {
		t.Fatalf("top paths wrong (want templated path): %+v", snap.TopEndpoints)
	}
}

func TestHealthShape(t *testing.T) {
	db := testDB(t)
	db.Exec(`CREATE TABLE IF NOT EXISTS notification_outbox (id bigserial PRIMARY KEY, done boolean DEFAULT false)`)
	db.Exec(`INSERT INTO notification_outbox (done) VALUES (false)`)
	t.Cleanup(func() { db.Exec(`TRUNCATE notification_outbox`) })

	h, err := NewService(db, NewUsageTracker(30*time.Minute)).Health()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if h.Database.SizeBytes <= 0 {
		t.Fatalf("db size missing: %+v", h.Database)
	}
	if h.Queues.OutboxPending != 1 {
		t.Fatalf("outbox pending=%d want 1", h.Queues.OutboxPending)
	}
	if h.Queues.Email.Available {
		t.Fatal("asynq must be unavailable in tests")
	}
}
