package studyresources

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

// This file is the executable form of the question "can any route other than
// the gated one hand out a study resource's bytes or its private rows?".
//
// It answers three ways, because each fails differently:
//
//   - a route inventory, so a second handler added to a public group shows up
//     as a diff rather than as a silent second way in;
//   - a middleware census, so a route that loses its auth guard is caught by
//     name instead of by reading routes.go;
//   - behavioural checks on the routes that can reach object storage, proving
//     the refusal happens before the object is opened.

// registeredRoutes is the exact route surface RegisterRoutes mounts. It is
// deliberately a complete literal rather than a subset: the whole point is that
// adding a handler shows up as a failing test, which is the moment someone has
// to decide whether the new route is gated.
func registeredRoutes(t *testing.T, r *gin.Engine) []string {
	t.Helper()
	var out []string
	for _, route := range r.Routes() {
		out = append(out, route.Method+" "+route.Path)
	}
	sort.Strings(out)
	return out
}

func TestStudyResourceRouteSurfaceIsExactlyThis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	got := registeredRoutes(t, func() *gin.Engine {
		r := gin.New()
		RegisterRoutes(r, passThrough, passThrough, NewHandler(nil))
		return r
	}())

	want := []string{
		// Superadmin CRUD. Note there is deliberately no admin route that
		// serves a study resource's bytes: admin can create, list, update,
		// replace the file and delete, but the only ways to READ an object's
		// content are /:id/download (gated) and /:id/stream (token-gated).
		"DELETE /api/v1/admin/study-resources/:id",
		"GET /api/v1/admin/study-resources",
		"POST /api/v1/admin/study-resources",
		"POST /api/v1/admin/study-resources/:id/file",
		// The §5.3 moderation queue and its two decisions. Approve publishes AND
		// pays the uploader in one step; reject never pays and requires a reason.
		"GET /api/v1/admin/study-resources/pending",
		"POST /api/v1/admin/study-resources/:id/approve",
		"POST /api/v1/admin/study-resources/:id/reject",
		"PUT /api/v1/admin/study-resources/:id",
		// The student upload and my-uploads surface: authMW alone, no role gate,
		// because the whole design is that a student's submission is NOT an admin
		// action and arrives pending, unpublished and unpaid.
		"POST /api/v1/study-resources",
		"GET /api/v1/study-resources/mine",
		// Public metadata browsing. No bytes, no drafts.
		"GET /api/v1/study-resources",
		"GET /api/v1/study-resources/:id",
		// The two session-gated routes: the document download and the mint.
		"GET /api/v1/study-resources/:id/download",
		"GET /api/v1/study-resources/:id/playback-token",
		// Inline video playback, authorized by the playback token rather than
		// by a session. Deliberately not behind authMW — see routes.go.
		"GET /api/v1/study-resources/:id/stream",
		// The document SAMPLE: public like the metadata list, and never a path
		// to the full bytes — it serves at most PreviewSamplePages pages and
		// always withholds at least one (preview.go). It is the second
		// deliberate session-guard exception, named here for the same reason
		// the stream route is.
		"GET /api/v1/study-resources/:id/preview",
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("route count = %d, want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("route[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func passThrough(c *gin.Context) { c.Next() }

// Every route that can reach object storage must be on the classified list
// below. The stream route is the one deliberate exception to the session guard
// and it is named rather than excluded silently, so adding a second exception
// requires editing this list.
//
// Whether each entry really is guarded is asserted behaviourally below, because
// gin's RouteInfo does not expose a group's middleware chain and a helper that
// cannot fail would only look like a check.
func TestEveryObjectServingRouteIsClassified(t *testing.T) {
	canServeBytes := map[string]string{
		"GET /api/v1/study-resources/:id/download":       "session (authMW), then the coin gate",
		"GET /api/v1/study-resources/:id/stream":         "a resource-bound playback token, never a session",
		"GET /api/v1/study-resources/:id/playback-token": "session (authMW), then the coin gate BEFORE minting",
		// The second anonymous byte-serving route, deliberate like stream: the
		// sample is the shop window, and its guard is not WHO calls but WHAT it
		// can serve — at most PreviewSamplePages of a published PDF, with at
		// least one page always withheld (preview.go, samplePageCount).
		"GET /api/v1/study-resources/:id/preview": "no session — public by design; it can only ever serve a page-bounded sample, never the full bytes",
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, passThrough, passThrough, NewHandler(nil))

	for _, route := range r.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := canServeBytes[key]; !ok {
			continue
		}
		if reason := canServeBytes[key]; reason == "" {
			t.Errorf("%s is byte-serving but has no recorded authorization reason", key)
		}
	}
	for key := range canServeBytes {
		if !routeRegistered(r, key) {
			t.Errorf("expected byte-serving route %s to exist; the surface changed", key)
		}
	}
}

func routeRegistered(r *gin.Engine, key string) bool {
	for _, route := range r.Routes() {
		if route.Method+" "+route.Path == key {
			return true
		}
	}
	return false
}

// roleSession mints a real session JWT carrying the given role, so a role check
// is exercised against the same claims the production middleware reads.
func roleSession(t *testing.T, role string) string {
	t.Helper()
	token, err := utils.GenerateToken(11, "someone@example.com", role, 0)
	if err != nil {
		t.Fatalf("mint session token for role %q: %v", role, err)
	}
	return token
}

// The behavioural version of the check above, which is the one that actually
// proves something: a router whose auth middleware rejects every session-less
// request must still serve /:id/download and /:id/playback-token to nobody.
func TestSessionOnlyRoutesRefuseAnAnonymousCaller(t *testing.T) {
	db, video := playbackFixture(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	authMW := func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
	}
	RegisterRoutes(r, authMW, nil, NewHandler(NewService(NewRepository(db))))

	for _, path := range []string{
		"/api/v1/study-resources/" + itoaResource(video.ID) + "/download",
		"/api/v1/study-resources/" + itoaResource(video.ID) + "/playback-token",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s with no session = %d, want 401 (body %s)", path, rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); strings.Contains(body, video.FilePath) {
			t.Errorf("GET %s leaked the object key: %s", path, body)
		}
	}
}

// A playback grant authorizes ONE route: the stream. It must not be laundered
// into a session credential, because the session is what /:id/download and
// /:id/playback-token accept — a grant that worked as a session would be a
// second way past the coin gate, not past the token check.
//
// The two directions are separate facts and both are asserted: a session JWT
// does not work as a grant (playback_test.go), and here a grant does not work
// as a session.
func TestPlaybackGrantIsNotASessionCredential(t *testing.T) {
	withTestJWTSecret(t)
	db, video := playbackFixture(t)

	grant := playbackTokenFor(t, 11, video.ID)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// The real Auth middleware, so the only question is whether a playback
	// grant validates as a session JWT. It is signed with a key DERIVED from
	// the session secret, so it must not.
	RegisterRoutes(r, middleware.Auth(), nil, NewHandler(NewService(NewRepository(db))))

	cases := []struct {
		name  string
		apply func(*http.Request)
	}{
		{name: "authorization bearer header", apply: func(rq *http.Request) {
			rq.Header.Set("Authorization", "Bearer "+grant)
		}},
		{name: "session cookie", apply: func(rq *http.Request) {
			rq.AddCookie(&http.Cookie{Name: "token", Value: grant})
		}},
		{name: "legacy token query param", apply: func(rq *http.Request) {
			rq.URL.RawQuery = "token=" + grant
		}},
		{name: "playback query param on a session route", apply: func(rq *http.Request) {
			rq.URL.RawQuery = playbackTokenQueryParam + "=" + grant
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{
				"/api/v1/study-resources/" + itoaResource(video.ID) + "/download",
				"/api/v1/study-resources/" + itoaResource(video.ID) + "/playback-token",
			} {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				tc.apply(req)
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("GET %s with a playback grant = %d, want 401 (a grant is not a session) (body %s)",
						path, rec.Code, rec.Body.String())
				}
			}
		})
	}
}

// The public detail and list routes hand out file_path and file_url, which name
// the object. That is metadata, not a byte path, and this is the invariant that
// makes it metadata: whatever key the public routes disclose must be one the
// public object route refuses to serve.
//
// Without this, adding "file_url is handy for the frontend" to the public DTO
// would be indistinguishable from adding a bypass.
func TestPublicRoutesOnlyDiscloseKeysThePublicObjectRouteRefuses(t *testing.T) {
	db, video := playbackFixture(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, middleware.Auth(), nil, NewHandler(NewService(NewRepository(db))))

	for _, path := range []string{
		"/api/v1/study-resources/" + itoaResource(video.ID),
		"/api/v1/study-resources",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200 (body %s)", path, rec.Code, rec.Body.String())
		}

		// And the keys this resource uses must be refused, whether they arrive
		// bare or behind the legacy /uploads prefix.
		for _, disclosed := range []string{video.FilePath, video.FileURL} {
			if !storage.IsPrivateKey(disclosed) {
				t.Errorf("object key %q is disclosed publicly but is NOT refused by the public object route", disclosed)
			}
		}

		if strings.Contains(rec.Body.String(), "%PDF") {
			t.Errorf("GET %s returned file content", path)
		}
	}
}

// The deny list itself, spelled out, so a prefix that is dropped or misspelt
// fails here rather than quietly making the public object route a way past a
// gate. The traversal entry is the shape a caller can actually construct.
func TestPublicObjectDenyListCoversEveryPrivatePrefix(t *testing.T) {
	for _, key := range []string{
		storage.StudyResourcePrefix + "anything",
		storage.PrivateVideoPrefix + "anything.mp4",
		storage.PrivatePrefix + "anything",
		"../private/x.mp4",
		"scholarship/../private/study-resources/lecture.mp4",
		"/uploads/private/study-resources/lecture.mp4",
		"/uploads/study-resources/syllabus.pdf",
	} {
		if !storage.IsPrivateKey(key) {
			t.Errorf("IsPrivateKey(%q) = false, want true", key)
		}
	}
	// A prefix that merely shares leading characters is not private.
	for _, key := range []string{"study-resources-legacy/x.pdf", "privately/x.pdf", "publications/x.pdf"} {
		if storage.IsPrivateKey(key) {
			t.Errorf("IsPrivateKey(%q) = true, want false (an unrelated asset was classified private)", key)
		}
	}
}

// A draft must not be reachable by id on any route, including the ones whose
// gate would otherwise run. The publication check has to come FIRST, so the
// answer is 404 rather than a price — otherwise a gate-before-publication
// ordering would both leak the draft's existence and tell a caller it costs
// coins. This router's auth middleware lets every request through, so a 404
// here can only have come from the publication check.
func TestDraftsAreUnreachableOnEveryRouteEvenWithASession(t *testing.T) {
	withTestJWTSecret(t)
	db, _ := playbackFixture(t)

	var draft StudyResource
	if err := db.First(&draft, "title = ?", "Draft lecture").Error; err != nil {
		t.Fatalf("load draft: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// A middleware that establishes a full session context, so the only thing
	// left to refuse a draft is the publication check. A bare pass-through
	// would leave user_id unset and IssuePlaybackToken would answer 401 before
	// it ever looked at the id, which would prove nothing about ordering.
	signedIn := func(c *gin.Context) {
		c.Set("user_id", uint(11))
		c.Set("user_role", "student")
		c.Next()
	}
	RegisterRoutes(r, signedIn, passThrough, NewHandler(NewService(NewRepository(db))))

	id := itoaResource(draft.ID)
	grant := playbackTokenFor(t, 11, draft.ID)

	for _, tc := range []struct {
		name string
		path string
	}{
		{"public detail", "/api/v1/study-resources/" + id},
		{"download", "/api/v1/study-resources/" + id + "/download"},
		{"playback token", "/api/v1/study-resources/" + id + "/playback-token"},
		{"stream with a matching grant", "/api/v1/study-resources/" + id + "/stream?" + playbackTokenQueryParam + "=" + grant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
			}
			if body := rec.Body.String(); strings.Contains(body, draft.FilePath) {
				t.Errorf("the refusal leaked the draft's object key: %s", body)
			}
		})
	}

	// And the public list must not contain it at all.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/study-resources", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public list = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), draft.FilePath) || strings.Contains(rec.Body.String(), draft.Title) {
		t.Errorf("the public list disclosed a draft: %s", rec.Body.String())
	}
}

// The admin group serves drafts and object keys with no coin check, which is
// only acceptable because it is genuinely superadmin-guarded. This asserts that
// with the real middleware rather than a stub that always aborts.
func TestAdminSurfaceIsRefusedToANonSuperadminSession(t *testing.T) {
	db, _ := playbackFixture(t)
	withTestJWTSecret(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Reproduce main.go: the real Auth middleware, then the real RequireRole
	// with the exact allow-list main.go passes for study resources.
	RegisterRoutes(r, middleware.Auth(), middleware.RequireRole("superadmin", "super_admin"),
		NewHandler(NewService(NewRepository(db))))

	for _, role := range []string{"student", "admin", "institution", "scholarship_provider", ""} {
		t.Run("role "+role, func(t *testing.T) {
			session := roleSession(t, role)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/study-resources", nil)
			req.Header.Set("Authorization", "Bearer "+session)
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("role %q on the admin list = %d, want 403 (body %s)", role, rec.Code, rec.Body.String())
			}
		})
	}
}
