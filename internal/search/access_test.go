package search

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Who may trigger an embedding reindex, as pinned by the tests below:
//
//	platform admin (admin / superadmin / super_admin) — both routes, both spellings
//	every other authenticated role                   — nothing
//	an anonymous caller                              — nothing
//
// The second row is the one that was not in the brief. This module mounted the
// same handler TWICE, and the copy on the bare /api/v1 group had no middleware at
// all — no auth, no role. A destructive global operation was reachable by anyone on
// the internet. access.go has the full argument.

// sharedRoleMWList is the role list cmd/server/main.go builds for roleMW, in the
// same order with the same aliases. Reproduced rather than imported because
// main.go is not importable, and it matters that it is exact: this is the list
// that admitted every tenant to a platform-wide destructive sweep.
var sharedRoleMWList = []string{
	"admin", "super_admin", "scholarship_provider", "scholarship-provider",
	"Scholarship Provider", "scholarship_provider_subuser", "institution",
}

// nonAdminViewers is every role roleMW used to admit here and that must reach
// nothing. The anonymous zero Viewer is in the list because that is the shape the
// formerly-public route produced, and because a zero Viewer must deny rather than
// pass through on the strength of having no role.
func nonAdminViewers() []Viewer {
	return []Viewer{
		{UserID: 0, Role: ""}, // anonymous
		{UserID: 40, Role: "institution"},
		{UserID: 41, Role: "scholarship_provider"},
		{UserID: 42, Role: "scholarship_provider_subuser"},
		{UserID: 7, Role: "student"},
		{UserID: 40, Role: "institutionx"},
	}
}

// testService builds a SearchService with no retrievers. The guard is checked
// before anything touches the database or the embedding provider, so a reindex test
// never needs a real index, a real DB, or an API key.
//
// A real sqlite handle is supplied because GetVectorStatus probes pgvector with a
// raw query, and the public-surface test below calls it. config.IsSQLite short
// circuits that probe, so the query never runs — but config.GetDB() is still called
// to build it, and a nil *gorm.DB panics there.
func testService(t *testing.T) *SearchService {
	t.Helper()
	return NewSearchService(testDB(t), nil, nil, false)
}

// testDB is a real sqlite handle. GetVectorStatus probes pgvector with a raw query
// and the history routes need a repository, so the public-surface test could not be
// written against a nil DB without asserting on a panic instead of a status code.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&SearchHistory{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// testHandler builds a handler with both dependencies the routes can reach: the
// service, and the history repository that cmd/server/main.go attaches with
// SetHistoryRepository.
func testHandler(t *testing.T) *Handler {
	t.Helper()
	db := testDB(t)

	// GetVectorStatus reads config.GetDB() — the process-wide handle, not the one on
	// the service — and probes pgvector through it. config.IsSQLite short-circuits
	// the probe, but GetDB is still called to build the query, and a nil *gorm.DB
	// panics there. Pointing the global at a real handle is the minimum needed to
	// assert a status code instead of catching a panic, and it is restored
	// afterwards so no other test in this package inherits it.
	originalDB, originalIsSQLite := config.DB, config.IsSQLite
	config.DB, config.IsSQLite = db, true
	t.Cleanup(func() { config.DB, config.IsSQLite = originalDB, originalIsSQLite })

	h := NewHandler(NewSearchService(db, nil, nil, false))
	h.SetHistoryRepository(NewSearchHistoryRepository(db))
	return h
}

// withEmbeddingDisabled stands up the minimal config the handler reads once it is
// PAST the guard.
//
// embedding.IsEnabled() dereferences config.AppConfig, which is nil until
// config.Load runs — and in cmd/server/main.go it is loaded long before any router
// is built. So this is not a production path, only a test one: a test that lets an
// admin through the guard and asserts the handler's own 202 would otherwise panic in
// IsEnabled rather than reaching the assertion.
//
// The tests that assert a REFUSAL never call this, which is itself the point: they
// prove the guard returns before the handler does anything at all.
func withEmbeddingDisabled(t *testing.T) {
	t.Helper()
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}
	original := config.AppConfig.EmbeddingEnabled
	config.AppConfig.EmbeddingEnabled = false
	t.Cleanup(func() { config.AppConfig.EmbeddingEnabled = original })
}

// TestReindexIsAdminOnlyAtTheService is the core of the fix: the service is the
// authority, for both the plain and the destructive force variant.
//
// force is exercised explicitly because it is the one that runs
// `UPDATE <table> SET embedding = NULL` across seventeen tables including other
// tenants' rows. A guard that only covered the plain reindex would still have left
// the destructive one open.
func TestReindexIsAdminOnlyAtTheService(t *testing.T) {
	svc := testService(t)
	for _, viewer := range nonAdminViewers() {
		for _, force := range []bool{false, true} {
			if err := svc.authorizeReindex(viewer, force); !errors.Is(err, ErrForbidden) {
				t.Fatalf("viewer %+v authorized a reindex (force=%t, err=%v)", viewer, force, err)
			}
		}
		// And the status read, which is the operational half of the same feature.
		if err := svc.authorizeReindex(viewer, false); !errors.Is(err, ErrForbidden) {
			t.Fatalf("viewer %+v authorized the status read (err=%v)", viewer, err)
		}
	}
}

// TestPlatformAdminsCanStillReindex is the anti-lockout test, written explicitly.
//
// The superadmin dashboard's SettingsSection.tsx calls /api/v1/admin/search/reindex
// and /reindex/status, so "admins still work" is a requirement rather than a
// nicety. A guard that refused everyone would pass every other test in this file
// while removing the feature.
//
// Every admin spelling the codebase uses is exercised, including casing and padding
// variants, because RequireRole and IsPlatformAdmin both normalise and the gate and
// the service must agree on that.
func TestPlatformAdminsCanStillReindex(t *testing.T) {
	svc := testService(t)
	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", " admin "} {
		viewer := Viewer{UserID: 1, Role: role}
		for _, force := range []bool{false, true} {
			if err := svc.authorizeReindex(viewer, force); err != nil {
				t.Fatalf("role %q refused a reindex (force=%t): %v", role, force, err)
			}
		}
	}
}

// TestReindexRoutesRefuseNonAdminsAtTheEdge covers the route half with the
// production shape, on BOTH spellings of the route. The short alias
// /api/v1/search/reindex is the one that used to have no middleware at all, so it
// gets its own assertion rather than being assumed to follow the admin one.
func TestReindexRoutesRefuseNonAdminsAtTheEdge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, role := range []string{"institution", "scholarship_provider", "scholarship_provider_subuser", "student"} {
		r := gin.New()
		RegisterRoutes(r, viewerMW(role, 40), nil, middleware.RequireRole(PlatformAdminRoles()...), testHandler(t))

		for _, path := range []string{
			"/api/v1/admin/search/reindex",
			"/api/v1/admin/search/reindex?force=true",
			"/api/v1/search/reindex",
			"/api/v1/search/reindex?force=true",
		} {
			w := doRequest(r, http.MethodPost, path, "")
			if w.Code != http.StatusForbidden {
				t.Fatalf("role %q POST %s: got %d, want 403 (%q)", role, path, w.Code, w.Body.String())
			}
		}

		if w := doRequest(r, http.MethodGet, "/api/v1/admin/search/reindex/status", ""); w.Code != http.StatusForbidden {
			t.Fatalf("role %q read the reindex status: got %d, want 403 (%q)", role, w.Code, w.Body.String())
		}
	}
}

// TestTheAnonymousReindexRouteIsClosed is the regression test for the hole that was
// not in the brief: /api/v1/search/reindex was mounted on the bare /api/v1 group
// with no middleware, so it answered any unauthenticated request.
//
// It asserts the route is refused for a caller with NO token at all, and separately
// that it 401s when even authMW is present. Both halves matter — a guard that only
// refused on the role would still let the request reach the handler, and a gate that
// was merely tightened while the service stayed open would still be one caller
// signature away from open again.
func TestTheAnonymousReindexRouteIsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Stands up config.AppConfig. Without the guard running first, a request here
	// reaches embedding.IsEnabled and panics on the nil config — which would still
	// fail the test, but as a panic rather than as the status-code assertion this
	// test exists to make. The ordering property is asserted deliberately, in
	// TestTheCheckRunsBeforeTheEmbeddingFlagIsDisclosed.
	withEmbeddingDisabled(t)

	r := gin.New()
	RegisterRoutes(r, viewerMW("", 0), nil, middleware.RequireRole(PlatformAdminRoles()...), testHandler(t))

	// No role and no user id at all. The gate refuses with "Unauthorized access" —
	// it is RequireRole's own answer when user_role is absent — so the status is
	// what is asserted here, not the body. The service's own message is asserted
	// below on the unmounted-gate mount, where the middleware cannot be speaking.
	for _, path := range []string{"/api/v1/search/reindex", "/api/v1/admin/search/reindex"} {
		w := doRequest(r, http.MethodPost, path+"?force=true", "")
		if w.Code != http.StatusForbidden {
			t.Fatalf("anonymous POST %s: got %d, want 403 (%q)", path, w.Code, w.Body.String())
		}
	}

	// The same two paths, mounted with NO role middleware at all — the shape the
	// public copy had. The service must still refuse, because the service check is
	// the authority and the middleware was never the thing holding the line.
	// viewerMW is still the auth stand-in — it puts user_role and user_id on the
	// context, which is what ViewerFrom reads. Only the ROLE GATE is omitted, since
	// that is the thing that would otherwise refuse the tenant before the handler
	// is entered.
	bare := gin.New()
	RegisterRoutes(bare, viewerMW("institution", 40), nil, nil, testHandler(t))

	for _, path := range []string{"/api/v1/search/reindex", "/api/v1/admin/search/reindex"} {
		w := doRequest(bare, http.MethodPost, path+"?force=true", "")
		if w.Code != http.StatusForbidden {
			t.Fatalf("unmounted-gate POST %s reached the service: got %d (%q)", path, w.Code, w.Body.String())
		}
		// With no middleware at all, this message can only have come from the
		// service. That is what makes this the load-bearing half of the test.
		if !strings.Contains(w.Body.String(), ErrForbidden.Error()) {
			t.Fatalf("unmounted-gate POST %s: want the service's %q, got %q", path, ErrForbidden, w.Body.String())
		}
	}

	// The status read, on the same unmounted-gate router. Without this the service
	// check on ReindexStatus is only ever exercised behind a working middleware, so
	// deleting it from the handler would ship silently: the gate would be the only
	// thing refusing, which is exactly the fragility the service check exists to
	// remove. Falsified by removing the authorizeReindex call from ReindexStatus —
	// every other test in this file still passes.
	sw := doRequest(bare, http.MethodGet, "/api/v1/admin/search/reindex/status", "")
	if sw.Code != http.StatusForbidden {
		t.Fatalf("unmounted-gate status read reached the service: got %d (%q)", sw.Code, sw.Body.String())
	}
	if !strings.Contains(sw.Body.String(), ErrForbidden.Error()) {
		t.Fatalf("unmounted-gate status read: want the service's %q, got %q", ErrForbidden, sw.Body.String())
	}
}

// TestReindexGateCannotBeWidenedByTheCaller is the defence-in-depth test, and the
// reason the service check is not redundant with the middleware.
//
// It mounts the production shape but hands RegisterRoutes the ACTUAL shared roleMW
// list from cmd/server/main.go — the one that admitted every tenant. The middleware
// lets an institution through, exactly as it always did, and the request must still
// be refused. If this fails, the module is relying on its caller to pass the right
// list, which is the fragility that let the original hole exist.
func TestReindexGateCannotBeWidenedByTheCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...), nil, testHandler(t))

	// The destructive one, on both spellings.
	for _, path := range []string{
		"/api/v1/admin/search/reindex?force=true",
		"/api/v1/search/reindex?force=true",
	} {
		w := doRequest(r, http.MethodPost, path, "")
		if w.Code != http.StatusForbidden {
			t.Fatalf("shared roleMW let the institution reindex via %s: %d %q", path, w.Code, w.Body.String())
		}
		// The refusal came from the service, not RequireRole, which is what
		// distinguishes "the gate is narrow" from "the gate was ignored". RequireRole's
		// body names the role; the service's does not.
		if strings.Contains(w.Body.String(), "Role:") {
			t.Fatalf("refusal came from the middleware, not the service: %q", w.Body.String())
		}
		if !strings.Contains(w.Body.String(), ErrForbidden.Error()) {
			t.Fatalf("want the service's %q, got %q", ErrForbidden, w.Body.String())
		}
	}
}

// TestAdminReindexRoutesStillReachTheHandler is the anti-lockout test at the edge.
//
// It cannot assert a 202 or a 200 — an accepted reindex would start a real sweep
// against a real database, and the disabled case answers 202 without touching one.
// So it asserts the thing that actually distinguishes success from refusal: an admin
// gets past the guard, and lands on the handler's own answer rather than the
// service's 403. Getting the service's 403 here would be the lockout.
func TestAdminReindexRoutesStillReachTheHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withEmbeddingDisabled(t)

	r := gin.New()
	RegisterRoutes(r, viewerMW("superadmin", 1), nil, middleware.RequireRole(PlatformAdminRoles()...), testHandler(t))

	for _, path := range []string{
		"/api/v1/admin/search/reindex",
		"/api/v1/search/reindex",
		"/api/v1/admin/search/reindex?force=true",
		"/api/v1/search/reindex?force=true",
	} {
		w := doRequest(r, http.MethodPost, path, "")
		if w.Code == http.StatusForbidden {
			t.Fatalf("admin refused POST %s: %q — the guard is over-broad", path, w.Body.String())
		}
		// Reached the handler, which with embedding disabled answers 202.
		if w.Code != http.StatusAccepted {
			t.Fatalf("admin POST %s: got %d, want the handler's 202 (%q)", path, w.Code, w.Body.String())
		}
	}

	// The status read answers for an admin.
	if w := doRequest(r, http.MethodGet, "/api/v1/admin/search/reindex/status", ""); w.Code != http.StatusOK {
		t.Fatalf("admin status read: %d %q", w.Code, w.Body.String())
	}
}

// TestTheCheckRunsBeforeTheEmbeddingFlagIsDisclosed pins the ORDERING, which is a
// security property rather than a style choice.
//
// The handler's disabled branch answers 202 with a message naming
// EMBEDDING_ENABLED. If the role check ran after it, any authenticated tenant could
// call the route and read the platform's embedding configuration off the status
// code. So a tenant must be refused identically whether or not embedding happens to
// be enabled, and must never see the flag's message.
//
// Falsified by moving the authorizeReindex call below the IsEnabled branch: the
// tenant then gets 202 and the flag's message, and this fails. The router is
// mounted with no middleware on purpose — with the gate in place, RequireRole would
// refuse first and this test would pass regardless of the handler's ordering.
func TestTheCheckRunsBeforeTheEmbeddingFlagIsDisclosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withEmbeddingDisabled(t)

	// Embedding explicitly disabled, which is the configuration where the
	// disclosing branch runs.
	//
	// Mounted with NO role middleware, deliberately. With the gate in place the
	// tenant is refused by RequireRole before the handler is entered, so the test
	// would pass no matter where inside Reindex the check sat — it would be
	// measuring the middleware, not the ordering. Removing the gate is what makes
	// the handler's own sequence the thing under test, which is where the flag
	// disclosure would happen.
	// viewerMW is still the auth stand-in — it puts user_role and user_id on the
	// context, which is what ViewerFrom reads. Only the ROLE GATE is omitted, since
	// that is the thing that would otherwise refuse the tenant before the handler
	// is entered.
	bare := gin.New()
	RegisterRoutes(bare, viewerMW("institution", 40), nil, nil, testHandler(t))

	w := doRequest(bare, http.MethodPost, "/api/v1/admin/search/reindex", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("tenant got %d, want 403 (%q)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "EMBEDDING_ENABLED") {
		t.Fatalf("the refusal disclosed the embedding flag: %q", w.Body.String())
	}

	// The admin does see it, which is what makes the check — not the flag — the
	// thing that was ordered first. Also on the unmounted-gate router, so both
	// halves of the comparison run through the same path.
	adminRouter := gin.New()
	RegisterRoutes(adminRouter, viewerMW("superadmin", 1), nil, nil, testHandler(t))
	aw := doRequest(adminRouter, http.MethodPost, "/api/v1/admin/search/reindex", "")
	if !strings.Contains(aw.Body.String(), "EMBEDDING_ENABLED") {
		t.Fatalf("admin did not reach the disabled branch: %d %q", aw.Code, aw.Body.String())
	}
}

// TestPublicSearchSurfaceIsUnchanged is the cross-check against a fix that
// over-reaches. /search, /search/suggest and /search/history are what students
// search with, they take no Viewer, and nothing here moved them.
//
// /search/vector-status is also still mounted with no gate: it is a liveness probe.
// It is flagged in the report as disclosing that pgvector and Meilisearch are
// installed, which is worth a decision but is not this fix's to make.
func TestPublicSearchSurfaceIsUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	RegisterRoutes(r, viewerMW("", 0), nil, middleware.RequireRole(PlatformAdminRoles()...), testHandler(t))

	// The public reads still answer for a caller with no role.
	if w := doRequest(r, http.MethodGet, "/api/v1/search/vector-status", ""); w.Code != http.StatusOK {
		t.Fatalf("public vector status: %d %q", w.Code, w.Body.String())
	}
	// The history group is still behind authMW alone, for a signed-in student.
	studentRouter := gin.New()
	RegisterRoutes(studentRouter, viewerMW("student", 7), nil, middleware.RequireRole(PlatformAdminRoles()...), testHandler(t))
	if w := doRequest(studentRouter, http.MethodGet, "/api/v1/search/history", ""); w.Code == http.StatusForbidden {
		t.Fatalf("a student was refused their own search history by the reindex gate: %q", w.Body.String())
	}
	// And the same gate refuses that student on the reindex, so the two groups
	// really do differ and this is not passing because the guard is inert.
	if w := doRequest(studentRouter, http.MethodPost, "/api/v1/admin/search/reindex", ""); w.Code != http.StatusForbidden {
		t.Fatalf("a student reached the reindex: %d %q", w.Code, w.Body.String())
	}
}

// TestPlatformAdminRolesCoversEveryAdminSpelling pins the list itself, since
// cmd/server/main.go builds the gate from it. Removing one locks the dashboard
// out; adding a tenant role would hand that tenant a platform-wide destructive
// operation.
func TestPlatformAdminRolesCoversEveryAdminSpelling(t *testing.T) {
	gate := middleware.RequireRole(PlatformAdminRoles()...)

	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", "SUPER_ADMIN", " admin "} {
		r := gin.New()
		group := r.Group("/probe")
		group.Use(viewerMW(role, 1))
		group.Use(gate)
		group.POST("", func(c *gin.Context) { c.Status(http.StatusOK) })

		if w := doRequest(r, http.MethodPost, "/probe", ""); w.Code != http.StatusOK {
			t.Fatalf("gate refused admin spelling %q: %d %q", role, w.Code, w.Body.String())
		}
	}

	for _, role := range []string{"institution", "scholarship_provider", "scholarship-provider",
		"Scholarship Provider", "scholarship_provider_subuser", "student", "superadminx", ""} {
		r := gin.New()
		group := r.Group("/probe")
		group.Use(viewerMW(role, 40))
		group.Use(gate)
		group.POST("", func(c *gin.Context) { c.Status(http.StatusOK) })

		if w := doRequest(r, http.MethodPost, "/probe", ""); w.Code != http.StatusForbidden {
			t.Fatalf("gate admitted %q: %d %q", role, w.Code, w.Body.String())
		}
	}
}

// viewerMW stands in for middleware.Auth(): it puts a user_id and a role on the
// context, which is the contract authMW establishes and the only part of it these
// guards read. Empty role and zero id produce the anonymous case, which must deny.
func viewerMW(role string, userID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role != "" {
			c.Set("user_role", role)
		}
		if userID != 0 {
			c.Set("user_id", userID)
		}
		c.Next()
	}
}

func doRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
