package analytics

import (
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type usageRecord struct {
	path    string
	status  int
	latency time.Duration
	at      time.Time
}

type TopEndpoint struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

type UsageSnapshot struct {
	TotalRequests int64         `json:"total_requests"`
	ServerErrors  int64         `json:"server_errors_5xx"`
	AvgLatencyMs  float64       `json:"avg_latency_ms"`
	P95LatencyMs  float64       `json:"p95_latency_ms"`
	TopEndpoints  []TopEndpoint `json:"top_endpoints"`
}

type UsageTracker struct {
	mu     sync.Mutex
	window time.Duration
	recs   []usageRecord
}

func NewUsageTracker(window time.Duration) *UsageTracker {
	return &UsageTracker{window: window}
}

// Middleware records /api/* requests using the route template (FullPath) so
// :id params never explode cardinality.
func (t *UsageTracker) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if len(c.Request.URL.Path) < 5 || c.Request.URL.Path[:5] != "/api/" {
			return
		}
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		t.mu.Lock()
		t.recs = append(t.recs, usageRecord{path: path, status: c.Writer.Status(), latency: time.Since(start), at: time.Now()})
		if len(t.recs) > 2000 {
			t.recs = t.recs[len(t.recs)-2000:]
		}
		t.mu.Unlock()
	}
}

func (t *UsageTracker) Snapshot() UsageSnapshot {
	cutoff := time.Now().Add(-t.window)
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := t.recs[:0]
	var total, errs int64
	var sum time.Duration
	lats := make([]float64, 0, len(t.recs))
	byPath := map[string]int64{}
	for _, r := range t.recs {
		if r.at.Before(cutoff) {
			continue
		}
		kept = append(kept, r)
		total++
		if r.status >= 500 {
			errs++
		}
		ms := float64(r.latency.Microseconds()) / 1000.0
		sum += r.latency
		lats = append(lats, ms)
		byPath[r.path]++
	}
	t.recs = kept
	snap := UsageSnapshot{TotalRequests: total, ServerErrors: errs}
	if total > 0 {
		snap.AvgLatencyMs = float64(sum.Microseconds()) / 1000.0 / float64(total)
		sort.Float64s(lats)
		snap.P95LatencyMs = lats[int(float64(len(lats)-1)*0.95)]
	}
	for p, n := range byPath {
		snap.TopEndpoints = append(snap.TopEndpoints, TopEndpoint{Path: p, Count: n})
	}
	sort.Slice(snap.TopEndpoints, func(i, j int) bool { return snap.TopEndpoints[i].Count > snap.TopEndpoints[j].Count })
	if len(snap.TopEndpoints) > 10 {
		snap.TopEndpoints = snap.TopEndpoints[:10]
	}
	if snap.TopEndpoints == nil {
		snap.TopEndpoints = []TopEndpoint{}
	}
	return snap
}
