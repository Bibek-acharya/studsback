package university

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Who may change the university directory, as pinned by the tests below:
//
//	platform admin (admin / superadmin / super_admin) — the four privileged routes
//	any authenticated tenant, including institution — the directory LIST only
//	everyone else                                    — nothing
//
// The split is the design, not an oversight, and it is the part most likely to be
// "tidied up" into a single admin-only group by a later reader. The list is on
// roleMW because it is the same handler the public GET /api/v1/universities
// mounts with no auth at all, and narrowing it would break the institution
// dashboard's ProfilePage.tsx, which fetches it with the institution token to fill
// its university picker. The single read and the three writes are on a narrower
// gate because the read goes through FindByIDFull and the writes are curation of a
// public reference page. access.go has the full argument.

// sharedRoleMWList is the role list cmd/server/main.go builds for roleMW, in the
// same order with the same aliases. Reproduced rather than imported because
// main.go is not importable, and it matters that it is exact: this is the list
// that admitted every tenant to the directory writes.
var sharedRoleMWList = []string{
	"admin", "super_admin", "scholarship_provider", "scholarship-provider",
	"Scholarship Provider", "scholarship_provider_subuser", "institution",
}

// seededUniversities builds a service over a database holding one published and
// one unpublished university.
//
// The unpublished one is load-bearing: AdminGetUniversityByID goes through
// FindByIDFull, not the public FindByID, so that is the row the privileged read
// can see and the public one cannot. Without it the privileged read would be
// indistinguishable from the public one and gating it would be unjustified.
func seededUniversities(t *testing.T) (*Service, *University, *University) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// CollegeUniversityCourse is migrated and seeded because FindCollegesByUniversityID
	// falls back to `university_affiliations @> ?::jsonb` when a university has no
	// course-mapping rows, and that is Postgres syntax SQLite cannot parse. One
	// mapping row per university keeps the query on its `id IN ?` branch, which is
	// portable. Without this the reads 500 on the operator and the tests would be
	// measuring a syntax error rather than the guard.
	//
	// The mapped college_id deliberately points at a row that does not exist: the
	// guard being tested is about who may call these methods, not about college
	// resolution, and an empty result from the IN branch is enough.
	// university.College is a projection of the colleges table owned by
	// internal/college; AutoMigrate here creates just enough of it for the join
	// query to run. The seeded mappings point at college ids that are deliberately
	// absent, so the branch returns no rows either way.
	if err := db.AutoMigrate(&University{}, &College{}, &CollegeUniversityCourse{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	published := University{Name: "Tribhuvan University", Location: "Kirtipur", Status: "published", Rank: 1}
	draft := University{Name: "Midland University", Location: "Elsewhere", Status: "draft"}
	for _, u := range []*University{&published, &draft} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("seed university: %v", err)
		}
		mapping := CollegeUniversityCourse{UniversityID: u.ID, CollegeID: 900 + u.ID, CourseID: u.ID}
		if err := db.Create(&mapping).Error; err != nil {
			t.Fatalf("seed course mapping: %v", err)
		}
	}
	return NewService(NewRepository(db)), &published, &draft
}

// nonAdminViewers is every role roleMW used to admit here and that must reach
// nothing on the four privileged routes.
func nonAdminViewers() []Viewer {
	return []Viewer{
		{UserID: 40, Role: "institution"},
		{UserID: 41, Role: "scholarship_provider"},
		{UserID: 42, Role: "scholarship_provider_subuser"},
		{UserID: 7, Role: "student"},
		{UserID: 0, Role: ""},
	}
}

// TestUniversityCatalogueIsAdminOnlyAtTheService is the core of the fix: the
// service is the authority, and every non-admin role is refused by the privileged
// read and all three writes.
//
// The "and nothing was written" assertions matter as much as the errors. A guard
// that ran after the update or the delete would return the right error and still
// be a working mutation.
func TestUniversityCatalogueIsAdminOnlyAtTheService(t *testing.T) {
	for _, viewer := range nonAdminViewers() {
		svc, published, _ := seededUniversities(t)

		// The privileged read — the one that sees rows the public route does not.
		uni, colleges, err := svc.AdminGetUniversityByID(viewer, published.ID)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q did the privileged read (err=%v)", viewer.Role, err)
		}
		if uni != nil || colleges != nil {
			t.Fatalf("role %q got a university back: %+v", viewer.Role, uni)
		}

		created, err := svc.CreateUniversity(viewer, CreateUniversityRequest{Name: "Sneaky University"})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q created a university (err=%v)", viewer.Role, err)
		}
		if created != nil {
			t.Fatalf("role %q got a university back from create: %+v", viewer.Role, created)
		}

		updated, err := svc.UpdateUniversity(viewer, published.ID, UpdateUniversityRequest{Name: strPtr("Rewritten by a tenant")})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q updated university %d (err=%v)", viewer.Role, published.ID, err)
		}
		if updated != nil {
			t.Fatalf("role %q got an updated university: %+v", viewer.Role, updated)
		}

		if err := svc.DeleteUniversity(viewer, published.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q deleted university %d (err=%v)", viewer.Role, published.ID, err)
		}

		assertUniversitiesIntact(t, svc, viewer.Role)
	}
}

// TestPlatformAdminsCanStillManageUniversities is the anti-lockout test, written
// explicitly. The superadmin dashboard's AddUniversitySection.tsx and
// UniversityAffiliationSection.tsx are live consumers of all four routes, so
// "admins still work" is a requirement rather than a nicety.
//
// Every admin spelling the codebase uses is exercised, including casing and
// padding variants, because RequireRole and IsPlatformAdmin both normalise and the
// gate and the service must agree on that.
func TestPlatformAdminsCanStillManageUniversities(t *testing.T) {
	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", " admin "} {
		viewer := Viewer{UserID: 1, Role: role}
		svc, published, draft := seededUniversities(t)

		// The privileged read sees the draft, which is the point of it existing.
		uni, _, err := svc.AdminGetUniversityByID(viewer, draft.ID)
		if err != nil {
			t.Fatalf("role %q privileged read: %v", role, err)
		}
		if uni == nil || uni.Name != "Midland University" {
			t.Fatalf("role %q privileged read: got %+v", role, uni)
		}

		created, err := svc.CreateUniversity(viewer, CreateUniversityRequest{Name: "New University"})
		if err != nil {
			t.Fatalf("role %q create: %v", role, err)
		}
		if created.ID == 0 || created.Name != "New University" {
			t.Fatalf("role %q create: got %+v", role, created)
		}

		updated, err := svc.UpdateUniversity(viewer, published.ID, UpdateUniversityRequest{Name: strPtr("Renamed by admin")})
		if err != nil {
			t.Fatalf("role %q update: %v", role, err)
		}
		if updated.Name != "Renamed by admin" {
			t.Fatalf("role %q update: got %q", role, updated.Name)
		}

		if err := svc.DeleteUniversity(viewer, published.ID); err != nil {
			t.Fatalf("role %q delete: %v", role, err)
		}

		// The delete really happened. A service that refused everything would pass
		// every check above and fail here.
		var gone int64
		svc.repo.db.Model(&University{}).Where("id = ?", published.ID).Count(&gone)
		if gone != 0 {
			t.Fatalf("role %q delete: university %d still present", role, published.ID)
		}
	}
}

// TestUniversityGateCannotBeWidenedByTheCaller is the defence-in-depth test, and
// the reason the service checks are not redundant with the middleware.
//
// It mounts the production shape but hands RegisterRoutes the ACTUAL shared roleMW
// list from cmd/server/main.go — the one that admitted every tenant. The middleware
// lets an institution through, exactly as it always did, and the delete must still
// be refused by the service. If this fails, the module is relying on its caller to
// pass the right list.
func TestUniversityGateCannotBeWidenedByTheCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, published, _ := seededUniversities(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...), nil, NewHandler(svc))

	w := doRequest(r, http.MethodDelete, "/api/v1/admin/universities/"+strconv.Itoa(int(published.ID)), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution delete a university: %d %q", w.Code, w.Body.String())
	}
	// The create route, which the service refuses with ErrForbidden.
	w = doJSON(r, http.MethodPost, "/api/v1/admin/universities", `{"name":"Sneaky"}`)
	if !strings.Contains(w.Body.String(), ErrForbidden.Error()) {
		t.Fatalf("shared roleMW let the institution create a university: %d %q", w.Code, w.Body.String())
	}
	// The privileged read, which is the one that leaks unpublished rows.
	w = doRequest(r, http.MethodGet, "/api/v1/admin/universities/"+strconv.Itoa(int(published.ID)), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution do the privileged read: %d %q", w.Code, w.Body.String())
	}

	// The refusals came from the service and not the middleware. RequireRole's body
	// names the role; the service's does not.
	if strings.Contains(w.Body.String(), "Role:") {
		t.Fatalf("refusal came from the middleware, not the service: %q", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), ErrForbidden.Error()) {
		t.Fatalf("want the service's %q, got %q", ErrForbidden, w.Body.String())
	}

	assertUniversitiesIntact(t, svc, "institution")
}

// TestUniversityCatalogueRoutesRefuseTenantsAtTheEdge covers the route half with
// the production shape. Each route is exercised per role, because a guard applied
// to three of them is a hole in the fourth.
func TestUniversityCatalogueRoutesRefuseTenantsAtTheEdge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, role := range []string{"institution", "scholarship_provider", "scholarship_provider_subuser", "student"} {
		svc, published, _ := seededUniversities(t)
		r := gin.New()
		RegisterRoutes(r, viewerMW(role, 40), nil, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

		id := strconv.Itoa(int(published.ID))
		for _, tc := range []struct{ method, path, body string }{
			{http.MethodGet, "/api/v1/admin/universities/" + id, ""},
			{http.MethodPost, "/api/v1/admin/universities", `{"name":"Sneaky"}`},
			{http.MethodPut, "/api/v1/admin/universities/" + id, `{"name":"Rewritten"}`},
			{http.MethodDelete, "/api/v1/admin/universities/" + id, ""},
		} {
			w := doJSON(r, tc.method, tc.path, tc.body)
			if w.Code != http.StatusForbidden {
				t.Fatalf("role %q %s %s: got %d, want 403 (%q)", role, tc.method, tc.path, w.Code, w.Body.String())
			}
		}

		assertUniversitiesIntact(t, svc, role)
	}
}

// TestTheDirectoryListStaysReachableByTenants is the anti-lockout test for the
// half of this module that is deliberately NOT narrowed, and the one a later
// reader is most likely to break by "tidying" the two groups into one.
//
// The institution dashboard's ProfilePage.tsx fetches
// /api/v1/admin/universities?limit=500 with the institution token to populate its
// university picker. If that list moved onto the admin gate, the picker would come
// back empty for every institution in the country. This asserts it still answers
// for an institution account, and that the response is the published directory
// rather than something privileged.
//
// Falsified by adding admin.Use(adminRoleMW) to the list group in routes.go: the
// request then 403s and this fails, with the narrow gate genuinely supplied rather
// than nil.
func TestTheDirectoryListStaysReachableByTenants(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, published, draft := seededUniversities(t)
	r := gin.New()
	// BOTH gates are supplied and both are real. Handing adminRoleMW as nil would
	// make a wiring regression invisible: gin tolerates a nil handler, so a routes.go
	// that put the list on the narrow gate would still have let this request through
	// and the test would still have passed. Supplying the narrow gate is what makes
	// it bite.
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...),
		middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	w := doRequest(r, http.MethodGet, "/api/v1/admin/universities?limit=500", "")
	if w.Code != http.StatusOK {
		t.Fatalf("institution could not read the directory list: %d %q", w.Code, w.Body.String())
	}

	// Same answer as the public route, which has no auth at all — that is the
	// argument for not gating it.
	publicRouter := gin.New()
	RegisterRoutes(publicRouter, viewerMW("", 0), middleware.RequireRole(sharedRoleMWList...),
		middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))
	pw := doRequest(publicRouter, http.MethodGet, "/api/v1/universities?limit=500", "")
	if pw.Code != http.StatusOK {
		t.Fatalf("public directory read: %d %q", pw.Code, pw.Body.String())
	}
	if w.Body.String() != pw.Body.String() {
		t.Fatalf("the admin list is not the public list:\nadmin:  %s\npublic: %s", w.Body.String(), pw.Body.String())
	}

	// And the published row is genuinely in it, so this is not passing on an empty
	// response.
	if !strings.Contains(w.Body.String(), published.Name) {
		t.Fatalf("published university missing from the list: %s", w.Body.String())
	}
	// Both rows are in it, published and draft alike. Not asserted as desirable —
	// see the note below and the finding in the report — but pinned because it is
	// the reason the "gating this list closes nothing" argument holds, and a
	// reader who assumed the list was publication-filtered would be reasoning from
	// a false premise when deciding whether to narrow the gate.
	//
	// FINDING, not fixed here: GET /api/v1/universities applies no status filter
	// unless ?status= is passed, so the PUBLIC route serves draft universities. That
	// is pre-existing and unrelated to the role question, and it is reported rather
	// than fixed, because deciding which rows a public directory should show is a
	// product call.
	if !strings.Contains(w.Body.String(), draft.Name) {
		t.Fatalf("draft row is absent from the list, so this test no longer covers "+
			"what the admin list actually returns: %s", w.Body.String())
	}
}

// TestAdminUniversityRoutesStillWork is the anti-lockout test at the edge,
// complementing TestPlatformAdminsCanStillManageUniversities at the service. Were
// the middleware narrower than PlatformAdminRoles, the service tests would keep
// passing while the dashboard got a 403 from the gate — a fix that ships broken.
func TestAdminUniversityRoutesStillWork(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, published, _ := seededUniversities(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("superadmin", 1), nil, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	id := strconv.Itoa(int(published.ID))
	if w := doRequest(r, http.MethodGet, "/api/v1/admin/universities/"+id, ""); w.Code != http.StatusOK {
		t.Fatalf("admin privileged read: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/universities", `{"name":"Fresh University"}`); w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("admin create: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/universities/"+id, `{"name":"Renamed"}`); w.Code != http.StatusOK {
		t.Fatalf("admin update: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodDelete, "/api/v1/admin/universities/"+id, ""); w.Code != http.StatusOK {
		t.Fatalf("admin delete: %d %q", w.Code, w.Body.String())
	}
}

// TestPublicUniversitySurfaceIsUnchanged is the cross-check against a fix that
// over-reaches. The public directory is unauthenticated and takes no Viewer, so
// nothing here moved it.
func TestPublicUniversitySurfaceIsUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, published, _ := seededUniversities(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("", 0), nil, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	if w := doRequest(r, http.MethodGet, "/api/v1/universities", ""); w.Code != http.StatusOK {
		t.Fatalf("public directory: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/universities/"+strconv.Itoa(int(published.ID)), ""); w.Code != http.StatusOK {
		t.Fatalf("public university read: %d %q", w.Code, w.Body.String())
	}
	// /filter-counts is deliberately not exercised here: it aggregates over jsonb
	// columns and is not SQLite-portable, and it takes no Viewer, so it is not part
	// of the property this test is about.
}

// TestPlatformAdminRolesCoversEveryAdminSpelling pins the list itself, since
// cmd/server/main.go builds the gate from it. Removing one locks the dashboard
// out; adding a tenant role would hand that tenant the directory for every module
// that builds its gate from this.
func TestPlatformAdminRolesCoversEveryAdminSpelling(t *testing.T) {
	gate := middleware.RequireRole(PlatformAdminRoles()...)

	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", "SUPER_ADMIN", " admin "} {
		r := gin.New()
		group := r.Group("/probe")
		group.Use(viewerMW(role, 1))
		group.Use(gate)
		group.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })

		if w := doRequest(r, http.MethodGet, "/probe", ""); w.Code != http.StatusOK {
			t.Fatalf("gate refused admin spelling %q: %d %q", role, w.Code, w.Body.String())
		}
	}

	for _, role := range []string{"institution", "scholarship_provider", "scholarship-provider",
		"Scholarship Provider", "scholarship_provider_subuser", "student", "superadminx", ""} {
		r := gin.New()
		group := r.Group("/probe")
		group.Use(viewerMW(role, 40))
		group.Use(gate)
		group.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })

		if w := doRequest(r, http.MethodGet, "/probe", ""); w.Code != http.StatusForbidden {
			t.Fatalf("gate admitted %q: %d %q", role, w.Code, w.Body.String())
		}
	}
}

// viewerMW stands in for middleware.Auth(): it puts a user_id and a role on the
// context, which is the contract authMW establishes and the only part of it these
// guards read.
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
	return doJSON(r, method, path, body)
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func strPtr(s string) *string { return &s }

// assertUniversitiesIntact checks that both seeded universities are still present
// and that neither was created, renamed or deleted.
func assertUniversitiesIntact(t *testing.T, svc *Service, who string) {
	t.Helper()
	var live int64
	svc.repo.db.Model(&University{}).Count(&live)
	if live != 2 {
		t.Fatalf("%s: %d universities left, want the 2 seeded", who, live)
	}
	for _, want := range []string{"Tribhuvan University", "Midland University"} {
		var got int64
		svc.repo.db.Model(&University{}).Where("name = ?", want).Count(&got)
		if got != 1 {
			t.Fatalf("%s: university %q is gone or renamed", who, want)
		}
	}
}
