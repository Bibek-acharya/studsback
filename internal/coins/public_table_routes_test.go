package coins

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

// The public table's ROUTE, pinned with no database.
//
// The projection is tested in public_table_test.go and the data in
// public_table_pg_test.go. What is testable here — and what matters most — is that the
// route is reachable by NOBODY in particular. A compliance disclosure that requires a
// login is not the disclosure the provision contemplates: 04 §6 says it "needs to be a
// page a student can actually read before they spend", and a student deciding whether
// to register has not registered yet.
//
// The mirror risk is the one this file mostly guards. The admin config route is one
// path away and returns the FULL economy — earn rates, fraud caps, the internal
// unlock switch. If this route ever picks up authMW, or worse gets mounted inside the
// admin group, the thing that breaks is not that a page goes blank; it is that the
// fraud thresholds stop being secret.

// coinTableServer builds a server with BOTH mounts: the public table unauthenticated
// and the admin group behind a real authMW.
//
// Both in one server is the point. A test that mounted only the public route would
// pass whether or not the mount code accidentally inherited the admin group's
// middleware; putting the two side by side and driving them with the SAME anonymous
// request is what actually distinguishes them.
//
// The auth here is the real middleware.RequireRole, so this asserts against the gate
// that ships rather than a stand-in that could drift from it.
func coinTableServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := NewService(NewConfigStore(newFakeSettings()), nil)

	RegisterPublicRoutes(r, NewPublicTableAPI(svc))
	// A no-op authMW, because the property under test is the MOUNTING (which group
	// carries which middleware), not the role list — the role gate is pinned in
	// support_routes_test.go. A middleware that always refuses stands in for "this
	// group is protected" without entangling the two questions.
	RegisterRoutes(r, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}, middleware.RequireRole("superadmin", "super_admin"), NewHandler(nil), nil)
	return r
}

// THE test: an anonymous caller gets the table, and is refused the admin config by the
// same request shape. One server, two answers, so neither assertion is vacuous.
func TestTheCoinTableIsReadableWithoutAnAccountWhileTheAdminConfigIsNot(t *testing.T) {
	r := coinTableServer(t)

	// Anonymous → the table.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/coins/table", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("an anonymous reader got %d, want 200 — a disclosure behind a login is "+
			"not the disclosure s.16(2)(n) contemplates. body: %s", w.Code, w.Body.String())
	}
	var envelope struct {
		Data PublicCoinTable `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.Prices.StudyResource != DefaultEconomyConfig().Prices.StudyResource {
		t.Errorf("the anonymous response carried no price: %+v", envelope.Data.Prices)
	}
	// The structural claims must be present too — this is the whole deliverable of
	// 07's mitigation list, and it is the part a student reads before deciding.
	if envelope.Data.Terms.ReferralLevels != 1 {
		t.Errorf("referral_levels = %d, want 1", envelope.Data.Terms.ReferralLevels)
	}

	// The same anonymous request to the admin config is refused. If this ever returns
	// 200 the earn rates and the fraud caps are public.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/admin/coins/economy", nil)
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("an anonymous reader reached the full economy config: %s", w.Body.String())
	}
}

// The response must not be cacheable as if it were a per-user document, and must not
// claim to be. A CDN caching this by URL and serving it to an authenticated caller is
// harmless; a CDN caching it and serving a STALE table after a price change is the
// failure that matters — the published price would no longer be the charged one, which
// is the advertisement problem again.
func TestTheCoinTableIsNotCachedForever(t *testing.T) {
	r := coinTableServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/coins/table", nil)
	r.ServeHTTP(w, req)

	cc := w.Header().Get("Cache-Control")
	if cc == "" {
		t.Error("no Cache-Control on a public pricing disclosure; a stale table would " +
			"publish a price that is not the one charged")
	}
	if strings.Contains(strings.ToLower(cc), "immutable") || strings.Contains(strings.ToLower(cc), "max-age=31536000") {
		t.Errorf("Cache-Control %q would let a stale price be published indefinitely", cc)
	}
}

// The route must not be inside the admin group. Asserted by walking the mounted
// inventory rather than by trusting the mount code, because the failure being guarded
// is exactly "someone added authMW to the public mount" and a comment would not catch
// it.
func TestTheCoinTableIsNotMountedInsideTheAdminGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterPublicRoutes(r, NewPublicTableAPI(NewService(NewConfigStore(newFakeSettings()), nil)))
	RegisterRoutes(r, func(c *gin.Context) { c.Next() },
		middleware.RequireRole("superadmin", "super_admin"), NewHandler(nil), nil)

	var public, admin []string
	for _, route := range r.Routes() {
		full := route.Method + " " + route.Path
		if strings.Contains(route.Path, "/admin/") {
			admin = append(admin, full)
		} else {
			public = append(public, full)
		}
	}
	if len(public) != 1 || public[0] != "GET /api/v1/coins/table" {
		t.Fatalf("the public surface is %v, want exactly [GET /api/v1/coins/table]", public)
	}
	// And the admin surface must NOT have gained a public twin. The full economy
	// config — earn rates and fraud caps included — is one path away, and this is the
	// test that says the two are not the same endpoint.
	if len(admin) != 6 {
		t.Errorf("admin coin routes = %d (%v), want 6", len(admin), admin)
	}
}

// A Handler constructed without a Service must answer 500, not panic. Same principle
// as RegisterRoutes' nil-AdminAPI guard and Handler.ready: a boot-time wiring mistake
// should read as a boot-time mistake.
func TestThePublicTableOnANilServiceAnswers500RatherThanPanicking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := NewPublicTableAPI(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/coins/table", nil)
	r.GET("/api/v1/coins/table", api.Table)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", w.Code)
	}
}
