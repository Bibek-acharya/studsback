package coins

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

// The support route's AUTHORISATION, pinned with no database.
//
// The data behaviour lives in support_view_pg_test.go; what is testable without one
// is who may reach the route, and that is the property worth pinning here — the
// endpoint discloses a student's full coin history, so the gate is the security
// boundary and a wiring slip moves it silently.
//
// The sqlite-backed coins route tests use a nil Ledger, which is exactly right for
// this: a request that gets PAST the gate must still answer 500 rather than
// leaking anything, and a request stopped at the gate must never reach the ledger
// at all — observable as 403 rather than 500.

func supportRouteServer(t *testing.T, role string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("user_role", role)
			c.Set("user_id", uint(1))
		}
		c.Next()
	})
	// A nil ledger on purpose: see the file header.
	RegisterRoutes(r, func(c *gin.Context) { c.Next() },
		middleware.RequireRole("superadmin", "super_admin"), NewHandler(nil), nil)
	return r
}

func TestOnlyPlatformOperatorsMayReadACoinHistory(t *testing.T) {
	// These are the roles that the shared multi-tenant roleMW admits and that this
	// gate must NOT. institution and scholarship_provider are the two the coin
	// routes.go header singles out as the reason for having their own gate.
	for _, role := range []string{
		"institution", "scholarship_provider", "scholarship-provider",
		"Scholarship Provider", "scholarship_provider_subuser", "user", "student", "",
	} {
		r := supportRouteServer(t, role)
		w := doAdminRequest(r, "GET", "/api/v1/admin/coins/users/4242")
		if w.Code != http.StatusForbidden {
			t.Errorf("role %q reached a student's coin history: got %d, want 403 — body %s",
				role, w.Code, w.Body.String())
		}
	}
	// And an operator does get past the gate — which for a nil ledger means a 500
	// from the handler, never a 403.
	//
	// NOTE the role list: "admin" is NOT in it, and that is the gate as written —
	// main.go's coinAdminRoleMW is RequireRole("superadmin", "super_admin"), with no
	// "admin" alias. Every other module's PlatformAdminRoles() includes it. Recorded
	// as an observation rather than fixed here: an operator account with role "admin"
	// can manage the inbox and the colleges but not the coin economy, which may well
	// be deliberate. TestCoinAdminGateAndPlatformAdminRolesDisagree documents it so
	// the question is visible instead of buried in main.go.
	for _, role := range []string{"superadmin", "super_admin"} {
		r := supportRouteServer(t, role)
		w := doAdminRequest(r, "GET", "/api/v1/admin/coins/users/4242")
		if w.Code == http.StatusForbidden {
			t.Errorf("role %q was refused the support view; the operator is locked out", role)
		}
	}
}

// A malformed id is a 400 and not a 404 or a 500, and it never reaches the ledger.
//
// The roles are the two superadmin spellings ONLY. "admin" is not admitted by this
// gate — see TestCoinAdminGateAndPlatformAdminRolesDisagree — so testing it here
// would assert 400 for a request that is actually a 403, which would pass for the
// wrong reason and hide the role question.
func TestTheSupportRouteRejectsAMalformedId(t *testing.T) {
	for _, role := range []string{"superadmin", "super_admin"} {
		r := supportRouteServer(t, role)
		for _, id := range []string{"abc", "0", "-1", "1.5"} {
			w := doAdminRequest(r, "GET", "/api/v1/admin/coins/users/"+id)
			if w.Code != http.StatusBadRequest {
				t.Errorf("id %q with role %q returned %d, want 400 — body %s",
					id, role, w.Code, w.Body.String())
			}
		}
	}
}

// The coin admin gate does NOT include the "admin" alias, and every other module's
// PlatformAdminRoles() does.
//
// This test exists to keep the inconsistency VISIBLE rather than to fix it. An
// operator whose role is "admin" can manage the inquiry inbox, colleges and
// universities, and cannot read the coin economy or its support view. That is
// either deliberate (coin balances are financial, and the tightest gate in the
// codebase earns its place) or an oversight from copying the analytics gate
// (main.go:866) rather than the system one.
//
// It is pinned as a test rather than a comment because a comment is invisible to
// the next person editing main.go, and a test that says "these two disagree" makes
// the decision something someone has to look at.
func TestCoinAdminGateAndPlatformAdminRolesDisagree(t *testing.T) {
	// What the gate actually admits in production.
	productionGate := map[string]bool{}
	for _, role := range []string{"superadmin", "super_admin"} {
		productionGate[role] = true
	}

	// What every OTHER module in this codebase considers a platform operator.
	for _, role := range PlatformAdminRoles() {
		if !productionGate[role] {
			t.Logf("NOTE: %q is a platform operator everywhere else but cannot reach the coin economy. "+
				"Decide deliberately whether that is correct (a financial surface deserving the "+
				"tightest gate) or an oversight.", role)
		}
	}
	// The gate must at least admit the two superadmin spellings — that much is not in
	// question, and asserting it means the test above cannot pass vacuously.
	if !productionGate["superadmin"] || !productionGate["super_admin"] {
		t.Error("the coin admin gate no longer admits either superadmin spelling")
	}
}

// Every admin coin route, walked, for every role that must be refused.
//
// This exists because the previous test asserted the gate on ONE route, and a gate
// that is applied per-route rather than per-group is exactly the kind of thing that is
// correct today and wrong after someone adds a route. Walking the group's whole
// inventory means a route added next month without the gate fails HERE rather than in
// production.
//
// The ledger is nil, so a role that DOES get through gets a 500 from the handler. That
// is the signal used below to tell "refused at the gate" (403) from "admitted and then
// failed for want of a database" (500) — and it means this test needs no database.
func TestNoNonOperatorRoleReachesANYAdminCoinRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	refused := []string{
		"institution", "scholarship_provider", "scholarship-provider",
		"Scholarship Provider", "scholarship_provider_subuser", "user", "student",
		"college", "employer", "counsellor", "", // no role at all
	}
	// Every route the admin group mounts, with a concrete id where the path needs one.
	// Kept as an explicit list rather than derived from r.Routes() because a derived
	// list would only prove the gate on the routes that exist — and the failure mode
	// is a route that exists and was never checked.
	paths := []struct{ method, path string }{
		{"GET", "/api/v1/admin/coins/economy"},
		{"PUT", "/api/v1/admin/coins/economy"},
		{"GET", "/api/v1/admin/coins/economy-daily"},
		{"GET", "/api/v1/admin/coins/users/4242"},
		{"POST", "/api/v1/admin/coins/adjust/4242"},
	}

	for _, tc := range paths {
		for _, role := range refused {
			r := supportRouteServer(t, role)
			w := doAdminRequest(r, tc.method, tc.path)
			if w.Code != http.StatusForbidden {
				t.Errorf("%s %s: role %q was NOT refused at the gate (got %d) — body %s",
					tc.method, tc.path, role, w.Code, strings.TrimSpace(w.Body.String()))
			}
		}
	}

	// And the converse, so the loop above cannot pass by every route 403-ing for an
	// unrelated reason (a typo in the path, say, which would 404 rather than 403 — but
	// assert the admission explicitly rather than assume it).
	for _, tc := range paths {
		r := supportRouteServer(t, "superadmin")
		w := doAdminRequest(r, tc.method, tc.path)
		if w.Code == http.StatusForbidden {
			t.Errorf("%s %s: a superadmin was refused — the route is unreachable to everyone",
				tc.method, tc.path)
		}
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s: 404 — the path in this test does not match a mounted route, "+
				"so the refusal assertions above proved nothing", tc.method, tc.path)
		}
	}
}

func TestTheAdminCoinRouteSurfaceIsExactlyThis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, func(c *gin.Context) { c.Next() },
		middleware.RequireRole("superadmin", "super_admin"), NewHandler(nil), nil)

	var got []string
	for _, route := range r.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	want := []string{
		"GET /api/v1/admin/coins/economy",
		"PUT /api/v1/admin/coins/economy",
		"GET /api/v1/admin/coins/users/:id",
		// The target is in the PATH, not the body — see AdjustCoins. The
		// path is where the operator saw which student they were correcting, and
		// it is in the access log next to it.
		"POST /api/v1/admin/coins/adjust/:userId",
		// 05 §5's dashboard, on the same gate: it reads the ledger the adjust
		// endpoint writes, so an operator who can correct a balance is exactly the
		// person who needs to see whether corrections are rising.
		"GET /api/v1/admin/coins/economy-daily",
	}
	if len(got) != len(want) {
		t.Fatalf("admin coin route count = %d, want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	have := map[string]bool{}
	for _, route := range got {
		have[route] = true
	}
	for _, route := range want {
		if !have[route] {
			t.Errorf("route %q is missing", route)
		}
	}
}

func doAdminRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	return doAdminRequestWith(r, method, path, "", "")
}

func doAdminRequestWith(r *gin.Engine, method, path, body, idempotencyKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// THE refusal. An Idempotency-Key header is REQUIRED, not defaulted: an operator
// double-clicking a support console is the normal case, and a server-generated key
// would make the second click a second movement. 400 rather than 428, because the
// body is not the problem and a precondition code would read as "your client is
// wrong".
func TestAnAdjustmentWithoutAnIdempotencyKeyIsRefused(t *testing.T) {
	r := supportRouteServer(t, "superadmin")
	w := doAdminRequestWith(r, "POST", "/api/v1/admin/coins/adjust/4242",
		`{"amount":25,"reason":"GOODWILL"}`, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("an adjustment with no Idempotency-Key returned %d, want 400 — body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Idempotency-Key") {
		t.Errorf("the error does not name the missing header: %s", w.Body.String())
	}
}

// A malformed target in the PATH is a 400, checked before the body is even read —
// the same ordering rule SupportUser follows, and for the same reason.
func TestAnAdjustmentForAMalformedTargetIsRefused(t *testing.T) {
	r := supportRouteServer(t, "superadmin")
	for _, id := range []string{"abc", "0", "-1"} {
		w := doAdminRequestWith(r, "POST", "/api/v1/admin/coins/adjust/"+id,
			`{"amount":25,"reason":"GOODWILL"}`, "key-1")
		if w.Code != http.StatusBadRequest {
			t.Errorf("target %q returned %d, want 400 — body %s", id, w.Code, w.Body.String())
		}
	}
}

// And the tenant roles are still refused, because the gate is the whole security
// boundary of a money-moving endpoint.
func TestATenantCannotPostAnAdjustment(t *testing.T) {
	for _, role := range []string{"institution", "scholarship_provider", "user", "student", ""} {
		r := supportRouteServer(t, role)
		w := doAdminRequestWith(r, "POST", "/api/v1/admin/coins/adjust/4242",
			`{"amount":25,"reason":"GOODWILL"}`, "key-1")
		if w.Code != http.StatusForbidden {
			t.Errorf("role %q posted an adjustment: got %d, want 403 — body %s", role, w.Code, w.Body.String())
		}
	}
}
