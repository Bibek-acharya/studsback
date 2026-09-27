package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// fixedClock is a hand-advanced clock, so the window and cleanup tests assert
// real behaviour instead of racing a sleep long enough to be flaky.
type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}

func (c *fixedClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

// newTestLimiter returns a limiter of limit uses per window wired to clock, so
// a test can spend its budget and then step past the window deterministically.
func newTestLimiter(limit int, window time.Duration, clock *fixedClock) *Limiter {
	l := NewLimiter(limit, window)
	l.now = clock.Now
	l.lastSweep = clock.now
	return l
}

// count returns how many uses of key the limiter has recorded, and whether each
// was allowed. It reports the limiter state directly rather than going through
// the middleware, so a counter test cannot be confused by a wiring test.
func count(l *Limiter, key string, times int) (allowed, rejected int) {
	for i := 0; i < times; i++ {
		if l.Allow(key) {
			allowed++
		} else {
			rejected++
		}
	}
	return allowed, rejected
}

func TestLimiterAllowsUpToLimitThenRejects(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(3, time.Hour, clock)

	allowed, rejected := count(l, "ip:1", 5)

	if allowed != 3 || rejected != 2 {
		t.Errorf("got %d allowed / %d rejected, want 3 / 2", allowed, rejected)
	}
}

func TestLimiterResetsAfterWindow(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(2, time.Hour, clock)

	if allowed, _ := count(l, "ip:1", 2); allowed != 2 {
		t.Fatalf("first window: %d allowed, want 2", allowed)
	}
	if l.Allow("ip:1") {
		t.Fatal("third request inside the window was allowed")
	}

	// One nanosecond short of the window: the budget must not be back yet.
	clock.Advance(time.Hour - time.Nanosecond)
	if l.Allow("ip:1") {
		t.Error("request one nanosecond before the window elapsed was allowed")
	}

	clock.Advance(time.Nanosecond)
	if !l.Allow("ip:1") {
		t.Error("request after the window elapsed was rejected")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(1, time.Hour, clock)

	if !l.Allow("ip:1") {
		t.Fatal("first use of ip:1 was rejected")
	}
	if l.Allow("ip:1") {
		t.Fatal("second use of ip:1 was allowed")
	}

	// A different key must not inherit the exhausted budget, in either order.
	if !l.Allow("ip:2") {
		t.Error("ip:2 rejected because ip:1 spent its budget")
	}
	if !l.Allow("email:a@b.com") {
		t.Error("email key rejected because ip:1 spent its budget")
	}
}

func TestLimiterCleanupDropsExpiredKeys(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(5, time.Hour, clock)

	for i := 0; i < 50; i++ {
		l.Allow("key-" + strconv.Itoa(i))
	}
	if tracked := l.Tracked(); tracked != 50 {
		t.Fatalf("Tracked() = %d before the window, want 50", tracked)
	}

	// Nothing new arrives, so only the sweep can shrink the map. The window has
	// to elapse twice: once to retire the entries, once for the next allow to
	// notice the window has passed since the last sweep.
	clock.Advance(2 * time.Hour)
	l.Allow("ip:fresh")

	if got := l.Tracked(); got != 1 {
		t.Errorf("Tracked() = %d after the window elapsed, want 1 (only the fresh key)", got)
	}
}

func TestLimiterTrimsToMaxTrackedKeys(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	// A window long enough that nothing expires, so the only thing standing
	// between the flood and an unbounded map is trimLocked.
	l := newTestLimiter(5, 100*time.Hour, clock)

	total := maxTrackedKeys + 200
	for i := 0; i < total; i++ {
		l.Allow("key-" + strconv.Itoa(i))
	}

	if got := l.Tracked(); got > maxTrackedKeys {
		t.Errorf("Tracked() = %d, want at most maxTrackedKeys (%d)", got, maxTrackedKeys)
	}
}

// newLimitedEngine mounts lim under key on a route that reports whether the
// request got through, so a 429 is distinguishable from any other status.
func newLimitedEngine(l *Limiter, key KeyFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/limited", RateLimit(l, key), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func post(r *gin.Engine, body, remoteAddr string) *httptest.ResponseRecorder {
	return postTo(r, "/limited", body, remoteAddr)
}

func postTo(r *gin.Engine, path, body, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimitMiddlewareRejectsOverLimit(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	r := newLimitedEngine(newTestLimiter(2, time.Hour, clock), ClientIPKey)

	// httptest gives every request the same remote address unless one is set, so
	// the first two here share a key and the third must be refused.
	for i := 0; i < 2; i++ {
		if w := post(r, `{}`, "10.0.0.1:1234"); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}

	w := post(r, `{}`, "10.0.0.1:1234")
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit status = %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), rateLimitedMessage) {
		t.Errorf("body = %q, want the shared rate limit message", w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header missing on a 429")
	}
}

func TestRateLimitMiddlewareSeparatesClientIPs(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	r := newLimitedEngine(newTestLimiter(1, time.Hour, clock), ClientIPKey)

	if w := post(r, `{}`, "10.0.0.1:1234"); w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w.Code)
	}
	if w := post(r, `{}`, "10.0.0.1:1234"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("repeat from the same IP: status = %d, want 429", w.Code)
	}

	// The NAT case: a different client behind the same shared budget is a
	// different key only if the IP differs, so this is the case the plan cares
	// about — it must not inherit the first IP's exhaustion.
	if w := post(r, `{}`, "10.0.0.2:1234"); w.Code != http.StatusOK {
		t.Errorf("different IP: status = %d, want 200", w.Code)
	}
}

func TestRateLimitMiddlewareLeavesRequestBodyReadable(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}

	// Two key functions read the same body in sequence, so this is the case
	// where a naive reader would hand the handler an empty stream.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var bound string
	r.POST("/bind", RateLimitAll(
		Limit{Limiter: newTestLimiter(5, time.Hour, clock), Key: JSONFieldKey("email")},
		Limit{Limiter: newTestLimiter(5, time.Hour, clock), Key: JSONFieldKey("type")},
	), func(c *gin.Context) {
		var payload map[string]any
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		bound, _ = payload["email"].(string)
		c.Status(http.StatusOK)
	})

	body := `{"email":"Student@Example.com","type":"verification"}`
	if w := postTo(r, "/bind", body, "10.0.0.1:1234"); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if bound != "Student@Example.com" {
		t.Errorf("handler bound %q, want the untouched body value", bound)
	}
}

func TestJSONFieldKeyNormalisesValue(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	r := newLimitedEngine(newTestLimiter(1, time.Hour, clock), JSONFieldKey("email"))

	if w := post(r, `{"email":"Student@Example.com"}`, "10.0.0.1:1234"); w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w.Code)
	}
	// Case and padding must not buy a second budget for one mailbox.
	if w := post(r, `{"email":"  student@EXAMPLE.com "}`, "10.0.0.2:1234"); w.Code != http.StatusTooManyRequests {
		t.Errorf("case-variant email: status = %d, want 429", w.Code)
	}
}

func TestJSONFieldKeySharesBucketWhenFieldIsMissing(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(1, time.Hour, clock)
	r := newLimitedEngine(l, JSONFieldKey("email"))

	// No email at all: the request still spends the shared empty-key budget
	// rather than escaping the rule, and the handler's binding rejects it later.
	if w := post(r, `{"type":"verification"}`, "10.0.0.1:1234"); w.Code != http.StatusOK {
		t.Fatalf("first keyless request: status = %d, want 200", w.Code)
	}
	if w := post(r, `not json`, "10.0.0.1:1234"); w.Code != http.StatusTooManyRequests {
		t.Errorf("second keyless request: status = %d, want 429", w.Code)
	}
	// A real email is a different key, so it is unaffected.
	if w := post(r, `{"email":"a@b.com"}`, "10.0.0.1:1234"); w.Code != http.StatusOK {
		t.Errorf("keyed request: status = %d, want 200", w.Code)
	}
}

func TestRateLimitAllRequiresEveryRule(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	byIP := newTestLimiter(2, time.Hour, clock)
	byEmail := newTestLimiter(1, time.Hour, clock)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/send-otp", RateLimitAll(
		Limit{Limiter: byIP, Key: ClientIPKey},
		Limit{Limiter: byEmail, Key: JSONFieldKey("email")},
	), func(c *gin.Context) { c.Status(http.StatusOK) })

	send := func(ip, email string) int {
		req := httptest.NewRequest(http.MethodPost, "/send-otp", strings.NewReader(`{"email":"`+email+`"}`))
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// The email rule is the tighter one, so it refuses first and a second IP
	// with a fresh mailbox is still allowed.
	if code := send("10.0.0.1", "a@b.com"); code != http.StatusOK {
		t.Fatalf("first send-otp: status = %d, want 200", code)
	}
	if code := send("10.0.0.1", "a@b.com"); code != http.StatusTooManyRequests {
		t.Errorf("repeat email: status = %d, want 429", code)
	}
	if code := send("10.0.0.2", "c@d.com"); code != http.StatusOK {
		t.Errorf("fresh IP and email: status = %d, want 200", code)
	}

	// The IP rule trips at 2: the third request from one IP fails even though
	// its email has budget.
	if code := send("10.0.0.2", "e@f.com"); code != http.StatusOK {
		t.Fatalf("second request from 10.0.0.2: status = %d, want 200", code)
	}
	if code := send("10.0.0.2", "g@h.com"); code != http.StatusTooManyRequests {
		t.Errorf("over IP limit: status = %d, want 429", code)
	}
}

func TestUserIDKeyUsesAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	if got := UserIDKey(c); got != "" {
		t.Errorf("UserIDKey without Auth = %q, want the shared empty key", got)
	}

	c.Set("user_id", uint(42))
	if got := UserIDKey(c); got != "user:42" {
		t.Errorf("UserIDKey = %q, want %q", got, "user:42")
	}
}
