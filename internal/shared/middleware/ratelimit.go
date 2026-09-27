package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"studsphere/backend/internal/shared/httpx"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

// Promoted from the package-local limiter in internal/college/handler.go: the
// pattern was right, the scope was not. It stays in-process on purpose — a
// shared store (Redis) would let limits survive a restart and span replicas,
// but it is also a new failure mode on the hot path of every auth request, and
// nothing here needs cross-replica coordination yet. One server, one process,
// one mutex.

// maxTrackedKeys caps how many keys a Limiter holds. A flood of one-shot keys
// (a rotating email field, a spoofed X-Forwarded-For) would otherwise grow the
// map without limit. Past the cap the least recently used keys are evicted, so
// the bound is hard rather than asymptotic: the map holds at most this many
// keys, each holding at most `limit` timestamps.
const maxTrackedKeys = 10000

// maxKeyBodyBytes caps how much of a request body a JSON key function reads, so
// a large upload cannot be buffered by the limiter. Anything past the cap stays
// readable by the handler (the limiter puts what it read back in front of the
// remainder), it just cannot be used as a key.
const maxKeyBodyBytes = 1 << 20

// rateLimitedMessage is the 429 body. One message for every rule on purpose:
// naming the limit that tripped leaks the budget an attacker is probing for.
const rateLimitedMessage = "Rate limit exceeded. Please try again later."

// Limiter is a fixed-window counter: a key may be used `limit` times per
// `window`, and the window slides — old timestamps are dropped on access, so
// the allowance recovers gradually rather than resetting on a boundary. A
// boundary reset is trivially doubled by an attacker straddling it, which is
// why the college limiter works the same way.
//
// It is not safe for keys that must not collide across limiters: the key
// namespace is the caller's, so a per-email key and a per-IP key on the same
// Limiter share buckets. Use one Limiter per rule.
type Limiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration

	// lastSweep bounds how long an idle key lingers. Sweeping on a window
	// interval means a key is dropped within one window of its last request
	// even if it is never touched again, which is what keeps the map bounded
	// for keys nobody comes back to.
	lastSweep time.Time

	// now is the clock, injected so tests can move time instead of sleeping.
	now func() time.Time
}

// NewLimiter returns a limiter allowing limit uses of a key per window. A
// non-positive limit blocks every request, so treat it as a configuration bug
// rather than "unlimited".
func NewLimiter(limit int, window time.Duration) *Limiter {
	l := &Limiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		now:      time.Now,
	}
	l.lastSweep = l.now()
	return l
}

// Allow reports whether key may be used again, counting the call either way.
func (l *Limiter) Allow(key string) bool {
	allowed, _ := l.allow(key)
	return allowed
}

// Tracked reports how many keys the limiter currently holds. Cleanup's job is
// to shrink this back towards the number of keys still in use.
func (l *Limiter) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.requests)
}

// allow also reports how long until the key has room again, so the middleware
// can emit a truthful Retry-After instead of guessing at the window.
func (l *Limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) >= l.window {
		l.sweepLocked(now)
		l.lastSweep = now
	}

	cutoff := now.Add(-l.window)
	// Copy rather than filter in place: the live slice is handed back out
	// below, and a rejected request must not leave a shorter window behind.
	live := make([]time.Time, 0, len(l.requests[key]))
	for _, t := range l.requests[key] {
		if t.After(cutoff) {
			live = append(live, t)
		}
	}

	if len(live) >= l.limit {
		l.requests[key] = live
		if len(l.requests) > maxTrackedKeys {
			l.trimLocked(now)
		}
		// live is non-empty here, and its oldest entry is inside the window, so
		// this is always positive.
		return false, live[0].Add(l.window).Sub(now)
	}

	l.requests[key] = append(live, now)
	if len(l.requests) > maxTrackedKeys {
		l.trimLocked(now)
	}
	return true, 0
}

// sweepLocked drops keys with nothing left inside the window. The per-key
// timestamps are left alone: allow trims the one key it is about to read, and
// rewriting every slice here would cost allocations for no benefit.
func (l *Limiter) sweepLocked(now time.Time) {
	cutoff := now.Add(-l.window)
	for key, times := range l.requests {
		live := false
		for _, t := range times {
			if t.After(cutoff) {
				live = true
				break
			}
		}
		if !live {
			delete(l.requests, key)
		}
	}
}

// trimLocked re-imposes maxTrackedKeys. Expired keys go first, which is enough
// unless the key space is being churned faster than the window retires it; then
// the least recently used keys are evicted, oldest first. Evicting is a
// deliberate trade: it can hand an active key a fresh budget, and losing that
// is better than an unbounded map at 10k+ live keys.
func (l *Limiter) trimLocked(now time.Time) {
	l.sweepLocked(now)
	if len(l.requests) <= maxTrackedKeys {
		return
	}

	type tracked struct {
		key    string
		newest time.Time
	}
	live := make([]tracked, 0, len(l.requests))
	for key, times := range l.requests {
		var newest time.Time
		for _, t := range times {
			if t.After(newest) {
				newest = t
			}
		}
		live = append(live, tracked{key: key, newest: newest})
	}
	slices.SortFunc(live, func(a, b tracked) int { return a.newest.Compare(b.newest) })

	for _, entry := range live {
		if len(l.requests) <= maxTrackedKeys {
			return
		}
		delete(l.requests, entry.key)
	}
}

// KeyFunc extracts the identifier a request is counted against. An empty
// string is a valid answer meaning "no identifier on this request" — see
// JSONFieldKey for why that must not be read as "exempt".
type KeyFunc func(c *gin.Context) string

// ClientIPKey counts by client IP. This is the weakest key available: campus
// networks in India and Nepal put thousands of students behind a single NAT, so
// an IP-only limit punishes exactly the users the product is for, and a SIM
// farm defeats it outright. Prefer adding a second key — see
// docs/coin-system/04-implementation-plan.md §2.2.
func ClientIPKey(c *gin.Context) string {
	return c.ClientIP()
}

// UserIDKey counts by the authenticated user id stored by Auth. It requires Auth
// to run first, so wire it after the auth middleware on protected routes.
// Requests with no resolvable id share the empty key, which Auth would have
// rejected anyway.
func UserIDKey(c *gin.Context) string {
	userID, ok := httpx.CurrentUserID(c)
	if !ok || userID == 0 {
		return ""
	}
	return fmt.Sprintf("user:%d", userID)
}

// JSONFieldKey counts by a top-level string field of a JSON body, e.g. an
// email. The value is trimmed and lower-cased first, otherwise
// "A@B.com" and "a@b.com" are two budgets for one mailbox.
//
// Reading the body is the cost here, and it is not free: the bytes are pushed
// back in front of the unread remainder so the handler still binds normally.
//
// A missing field, a non-string value, or an unparseable body all yield the
// empty key, and every such request shares one bucket. The alternative —
// skipping the rule — is a hole: dropping the email buys an attacker the
// unlimited IP budget. Sharing a bucket fails closed and costs nothing real,
// because the handler's own binding rejects those requests anyway.
func JSONFieldKey(field string) KeyFunc {
	return func(c *gin.Context) string {
		body, err := readRequestBody(c)
		if err != nil {
			return ""
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(body, &payload); err != nil {
			return ""
		}
		raw, ok := payload[field]
		if !ok {
			return ""
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return ""
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return ""
		}
		return field + ":" + value
	}
}

// readRequestBody reads at most maxKeyBodyBytes of the request body and puts
// them back, so a key function can be applied to a request that a handler will
// subsequently bind.
func readRequestBody(c *gin.Context) ([]byte, error) {
	if c.Request == nil || c.Request.Body == nil {
		return nil, io.ErrUnexpectedEOF
	}
	body := c.Request.Body
	head, err := io.ReadAll(io.LimitReader(body, maxKeyBodyBytes))
	if err != nil {
		return nil, err
	}
	c.Request.Body = replayBody{Reader: io.MultiReader(bytes.NewReader(head), body), Closer: body}
	return head, nil
}

// replayBody serves the bytes a key function consumed before handing the rest
// of the original body to the handler, while still closing the original.
type replayBody struct {
	io.Reader
	io.Closer
}

// Limit pairs one counter with the key its requests are counted against. The
// two are separate because one Limiter holds one counter per key, so a route
// wanting both a per-IP and a per-email budget needs two Limits.
type Limit struct {
	Limiter *Limiter
	Key     KeyFunc
}

// RateLimit rejects a request once limiter has seen limit uses of the key that
// fn extracts.
func RateLimit(limiter *Limiter, key KeyFunc) gin.HandlerFunc {
	return RateLimitAll(Limit{Limiter: limiter, Key: key})
}

// RateLimitAll applies every rule and lets the request through only if all of
// them pass, which is how /send-otp carries "5/hour/IP and 3/hour/email": one
// middleware, two independent counters.
//
// Rules are evaluated in order, and a rejection stops there, so a request
// refused by the IP rule never spends email budget. That ordering also decides
// which limit a caller is told about, and only through the shared message.
func RateLimitAll(rules ...Limit) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, rule := range rules {
			// A half-built rule is a wiring bug; skipping it is the same as not
			// having added the rule at all, which is at least visible in review.
			if rule.Limiter == nil || rule.Key == nil {
				continue
			}
			allowed, retryAfter := rule.Limiter.allow(rule.Key(c))
			if allowed {
				continue
			}
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			}
			response.Error(c, http.StatusTooManyRequests, rateLimitedMessage)
			c.Abort()
			return
		}
		c.Next()
	}
}
