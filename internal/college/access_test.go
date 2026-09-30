package college

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Who may change the college catalogue, as pinned by the tests below:
//
//	platform admin (admin / superadmin / super_admin) — every route
//	the institution that administers the college           — its OWN
//	                                                    college's map pin
//	everyone else, including an institution acting on a
//	college that is not its own, scholarship_provider,
//	scholarship_provider_subuser and student              — nothing
//
// This is the one module of the four where the answer is NOT simply "admins only",
// and the reason is a field rather than a route name: InstitutionUser.CollegeID.
// An institution account administers one college, and this module has always
// treated that column as "my college" — UpdateInstitutionCollegeLocation reads it
// and refuses with "No college associated with your account" when it is zero.
//
// So the map pin is tenant-scoped and the rest of the catalogue is platform-admin
// only. The negative half matters as much as the positive: an institution account
// must not be able to move a COMPETITOR's pin, which is what the arbitrary :id on
// PUT /admin/colleges/:id/location allowed before this. access.go has the full
// argument; these tests pin the behaviour.

// sharedRoleMWList is the role list cmd/server/main.go builds for roleMW, in the
// same order with the same aliases. Reproduced rather than imported because
// main.go is not importable, and it matters that it is exact: this is the list
// that admitted every tenant to the catalogue.
var sharedRoleMWList = []string{
	"admin", "super_admin", "scholarship_provider", "scholarship-provider",
	"Scholarship Provider", "scholarship_provider_subuser", "institution",
}

// stubTenants stands in for institution.Repository. It exists so the ownership rule
// can be tested directly against the field that decides it, without standing up
// the institution module's schema. The production wiring passes the real
// institutionRepo; see cmd/server/main.go.
type stubTenants struct {
	// byUserID maps an authenticated user id to the college it administers, which
	// is exactly what FindInstitutionUserByID(...).CollegeID answers.
	byUserID map[uint]uint
}

func (s stubTenants) FindInstitutionUserByID(id uint) (*institution.InstitutionUser, error) {
	collegeID, ok := s.byUserID[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	return &institution.InstitutionUser{ID: id, CollegeID: collegeID}, nil
}

// seededColleges builds a service over a database holding two colleges, and wires
// the tenant lookup so institution user 40 administers college #1 and institution
// user 41 administers college #2.
//
// Two colleges is the minimum that makes a cross-tenant test mean anything: with
// one, "can this institution touch the college" and "can it touch some college"
// are the same question.
func seededColleges(t *testing.T) (*Service, *College, *College) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&College{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// Both start UNVERIFIED: ApproveCollege errors on an already-verified college,
	// and the anti-lockout test is about who may call it, not about the starting
	// state. Leaving them verified would have made the admin path fail for a reason
	// that has nothing to do with authorisation.
	mine := College{Name: "Kathmandu University", Location: "Lalitpur", CollegeType: "private"}
	theirs := College{Name: "Some Other College", Location: "Pokhara", CollegeType: "private"}
	for _, c := range []*College{&mine, &theirs} {
		lat, lng := 27.7, 85.3
		c.Latitude, c.Longitude = &lat, &lng
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("seed college: %v", err)
		}
	}

	svc := NewService(NewRepository(db)).WithTenantLookup(stubTenants{
		byUserID: map[uint]uint{40: mine.ID, 41: theirs.ID},
	})
	return svc, &mine, &theirs
}

// owningInstitution is the viewer that administers the first seeded college — the
// legitimate tenant principal this module must not lock out.
func owningInstitution() Viewer { return Viewer{UserID: 40, Role: "institution"} }

// otherInstitution administers a different college and must reach nothing on the
// first one.
func otherInstitution() Viewer { return Viewer{UserID: 41, Role: "institution"} }

// tenantlessViewers is every caller that owns no college and must reach nothing:
// a scholarship provider and its subuser (both in the roleMW allow-list), a
// student, and a caller with no context at all.
func tenantlessViewers() []Viewer {
	return []Viewer{
		{UserID: 43, Role: "scholarship_provider"},
		{UserID: 44, Role: "scholarship_provider_subuser"},
		{UserID: 7, Role: "student"},
		{UserID: 999, Role: "institution"}, // an institution with no college attached
		{UserID: 0, Role: ""},
	}
}

// TestCollegeCatalogueIsPlatformAdminOnly is the core of the fix for the six
// routes no tenant owns: create, full profile update, delete, approve,
// toggle-featured and image upload.
//
// The "and nothing was written" assertions matter as much as the errors. A guard
// that returned ErrForbidden after the mutation had already run would pass every
// error check here and still be a working delete.
func TestCollegeCatalogueIsPlatformAdminOnly(t *testing.T) {
	for _, viewer := range append(tenantlessViewers(), otherInstitution(), owningInstitution()) {
		svc, mine, _ := seededColleges(t)

		created, err := svc.CreateCollege(viewer, CreateCollegeRequest{Name: "Sneaky College", Location: "Nowhere", UniversityID: 1})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("viewer %+v created a college (err=%v)", viewer, err)
		}
		if created != nil {
			t.Fatalf("viewer %+v got a college back: %+v", viewer, created)
		}

		updated, err := svc.UpdateCollege(viewer, mine.ID, UpdateCollegeRequest{Name: "Rewritten by a tenant"})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("viewer %+v updated college %d (err=%v)", viewer, mine.ID, err)
		}
		if updated != nil {
			t.Fatalf("viewer %+v updated a college: %+v", viewer, updated)
		}

		if err := svc.DeleteCollege(viewer, mine.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("viewer %+v deleted college %d (err=%v)", viewer, mine.ID, err)
		}

		if _, err := svc.ApproveCollege(viewer, mine.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("viewer %+v approved college %d (err=%v)", viewer, mine.ID, err)
		}
		if _, err := svc.ToggleCollegeFeatured(viewer, mine.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("viewer %+v featured college %d (err=%v)", viewer, mine.ID, err)
		}
		if _, err := svc.UploadCollegeImage(viewer, nil); !errors.Is(err, ErrForbidden) {
			t.Fatalf("viewer %+v uploaded an image (err=%v)", viewer, err)
		}

		assertCollegesIntact(t, svc, mine, mine.ID, 2)
	}
}

// TestAnInstitutionCannotMoveACompetitorsPin is the tenant-scoped rule stated as
// its own case, and it is the hole this module actually had.
//
// Before the fix, PUT /admin/colleges/:id/location sat behind roleMW with no
// ownership check and took an arbitrary :id. Any institution account could put
// any college's coordinates wherever it liked, corrupting a competitor's position
// in the find-college map. Asserted as "not this college", because "no college" is
// already covered by TestCollegeCatalogueIsPlatformAdminOnly.
func TestAnInstitutionCannotMoveACompetitorsPin(t *testing.T) {
	before := map[uint][2]float64{}

	svc, mine, theirs := seededColleges(t)
	before[mine.ID] = [2]float64{*mine.Latitude, *mine.Longitude}
	before[theirs.ID] = [2]float64{*theirs.Latitude, *theirs.Longitude}

	// Institution 41 owns `theirs`. It reaches its own pin and nothing else.
	if err := svc.UpdateCollegeLocation(otherInstitution(), theirs.ID, 28.2, 83.9); err != nil {
		t.Fatalf("institution could not move its OWN college: %v", err)
	}
	// Institution 40 owns `mine`. It must not touch `theirs`.
	if err := svc.UpdateCollegeLocation(owningInstitution(), theirs.ID, 1.1, 2.2); !errors.Is(err, ErrForbidden) {
		t.Fatalf("institution moved a competitor's pin (err=%v)", ownerErr(err))
	}
	// And the reverse, so the rule is not accidentally one-directional.
	if err := svc.UpdateCollegeLocation(otherInstitution(), mine.ID, 1.1, 2.2); !errors.Is(err, ErrForbidden) {
		t.Fatalf("institution moved a rival's pin (err=%v)", ownerErr(err))
	}

	// `mine` was never moved by anyone.
	live, err := svc.repo.FindByID(mine.ID)
	if err != nil {
		t.Fatalf("reload college %d: %v", mine.ID, err)
	}
	if *live.Latitude != before[mine.ID][0] || *live.Longitude != before[mine.ID][1] {
		t.Fatalf("college %d was moved by a non-owner: lat=%v lng=%v, want %v",
			mine.ID, *live.Latitude, *live.Longitude, before[mine.ID])
	}
}

// ownerErr keeps the failure message readable without shadowing the loop variable
// in the callers above.
func ownerErr(err error) error { return err }

// TestTheInstitutionMapPinRouteStillWorks is the anti-lockout test for the tenant
// half, written explicitly.
//
// This is the requirement, not a nicety. The institution dashboard has a live page
// (app/institution-zone/dashboard/college-location/page.tsx) that calls
// apiService.updateInstitutionCollegeLocation, and this module has always had a
// route for it. A fix that made the module uniformly admin-only would have passed
// every negative test in this file while removing a feature customers pay for.
func TestTheInstitutionMapPinRouteStillWorks(t *testing.T) {
	svc, mine, _ := seededColleges(t)

	// Through the tenant-scoped service rule, the owning institution succeeds.
	if err := svc.UpdateCollegeLocation(owningInstitution(), mine.ID, 27.68, 85.32); err != nil {
		t.Fatalf("owning institution could not move its own pin: %v", err)
	}

	live, err := svc.repo.FindByID(mine.ID)
	if err != nil {
		t.Fatalf("reload college: %v", err)
	}
	if *live.Latitude != 27.68 || *live.Longitude != 85.32 {
		t.Fatalf("pin not written: lat=%v lng=%v, want 27.68 85.32", *live.Latitude, *live.Longitude)
	}
}

// TestPlatformAdminsCanStillManageColleges is the anti-lockout test for the admin
// half. The superadmin dashboard is the only legitimate consumer of the six
// catalogue routes, so "admins still work" is a requirement.
//
// Every admin spelling the codebase uses is exercised, including casing and padding
// variants, because RequireRole and IsPlatformAdmin both normalise and the gate and
// the service must agree on that.
func TestPlatformAdminsCanStillManageColleges(t *testing.T) {
	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", " admin "} {
		viewer := Viewer{UserID: 1, Role: role}
		svc, mine, _ := seededColleges(t)

		created, err := svc.CreateCollege(viewer, CreateCollegeRequest{Name: "New College", Location: "Bhaktapur", UniversityID: 1})
		if err != nil {
			t.Fatalf("role %q create: %v", role, err)
		}
		if created.ID == 0 || created.Name != "New College" {
			t.Fatalf("role %q create: got %+v", role, created)
		}

		updated, err := svc.UpdateCollege(viewer, mine.ID, UpdateCollegeRequest{Name: "Renamed by admin"})
		if err != nil {
			t.Fatalf("role %q update: %v", role, err)
		}
		if updated.Name != "Renamed by admin" {
			t.Fatalf("role %q update: got %q", role, updated.Name)
		}

		approved, err := svc.ApproveCollege(viewer, mine.ID)
		if err != nil {
			t.Fatalf("role %q approve: %v", role, err)
		}
		if !approved.Verified {
			t.Fatalf("role %q approve: college not verified: %+v", role, approved)
		}

		featured, err := svc.ToggleCollegeFeatured(viewer, mine.ID)
		if err != nil {
			t.Fatalf("role %q featured: %v", role, err)
		}
		if !featured.Featured {
			t.Fatalf("role %q featured: college not featured: %+v", role, featured)
		}

		// The map pin, admin side: any college, not just one the admin "owns".
		if err := svc.UpdateCollegeLocation(viewer, theirsFor(t, svc).ID, 28.2, 83.9); err != nil {
			t.Fatalf("role %q location: %v", role, err)
		}

		if err := svc.DeleteCollege(viewer, mine.ID); err != nil {
			t.Fatalf("role %q delete: %v", role, err)
		}
		var gone int64
		svc.repo.db.Model(&College{}).Where("id = ?", mine.ID).Count(&gone)
		if gone != 0 {
			t.Fatalf("role %q delete: college %d still present", role, mine.ID)
		}
	}
}

// theirsFor is the second seeded college, re-read so the admin test can move a
// college it does not own — the point being that admins are not subject to the
// ownership comparison at all.
func theirsFor(t *testing.T, svc *Service) *College {
	t.Helper()
	var c College
	if err := svc.repo.db.Order("id asc").Offset(1).First(&c).Error; err != nil {
		t.Fatalf("load second college: %v", err)
	}
	return &c
}

// TestCollegeGateCannotBeWidenedByTheCaller is the defence-in-depth test, and the
// reason the service checks are not redundant with the middleware.
//
// It mounts the production shape but hands RegisterRoutes the ACTUAL shared roleMW
// list from cmd/server/main.go — the one that admitted every tenant. The middleware
// lets an institution through, exactly as it always did, and both the catalogue
// write and the cross-tenant pin must still be refused by the service. If this
// fails, the module is relying on its caller to pass the right list.
func TestCollegeGateCannotBeWidenedByTheCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, mine, theirs := seededColleges(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...), NewHandler(svc, nil))

	// A catalogue route the institution owns nothing on.
	w := doJSON(r, http.MethodDelete, "/api/v1/admin/colleges/"+strconv.Itoa(int(mine.ID)), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution delete a college: %d %q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), ErrForbidden.Error()) {
		t.Fatalf("want the service's %q, got %q", ErrForbidden, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Role:") {
		t.Fatalf("refusal came from the middleware, not the service: %q", w.Body.String())
	}

	// The cross-tenant pin, which is the hole that actually existed.
	w = doJSON(r, http.MethodPut, "/api/v1/admin/colleges/"+strconv.Itoa(int(theirs.ID))+"/location",
		`{"latitude":1.1,"longitude":2.2}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution move a rival's pin: %d %q", w.Code, w.Body.String())
	}

	// Nothing moved and nothing was deleted.
	live, err := svc.repo.FindByID(theirs.ID)
	if err != nil {
		t.Fatalf("reload college: %v", err)
	}
	if *live.Latitude != *theirs.Latitude || *live.Longitude != *theirs.Longitude {
		t.Fatalf("rival's pin was moved with a widened gate: lat=%v lng=%v", *live.Latitude, *live.Longitude)
	}
	if _, err := svc.repo.FindByID(mine.ID); err != nil {
		t.Fatalf("college %d was deleted with a widened gate: %v", mine.ID, err)
	}
}

// TestCollegeCatalogueRoutesRefuseTenantsAtTheEdge covers the route half with the
// production shape. Each route is exercised per role, because a guard applied to
// five of them is a hole in the sixth.
func TestCollegeCatalogueRoutesRefuseTenantsAtTheEdge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, role := range []string{"institution", "scholarship_provider", "scholarship_provider_subuser", "student"} {
		svc, mine, _ := seededColleges(t)
		r := gin.New()
		RegisterRoutes(r, viewerMW(role, 40), middleware.RequireRole(sharedRoleMWList...), NewHandler(svc, nil))

		id := strconv.Itoa(int(mine.ID))
		for _, tc := range []struct{ method, path, body string }{
			{http.MethodPost, "/api/v1/admin/colleges", `{"name":"Sneaky","location":"Nowhere","university_id":1}`},
			{http.MethodPut, "/api/v1/admin/colleges/" + id, `{"name":"Rewritten"}`},
			{http.MethodDelete, "/api/v1/admin/colleges/" + id, ""},
			{http.MethodPut, "/api/v1/admin/colleges/" + id + "/approve", `{}`},
			{http.MethodPut, "/api/v1/admin/colleges/" + id + "/featured", `{}`},
		} {
			w := doJSON(r, tc.method, tc.path, tc.body)
			if w.Code != http.StatusForbidden {
				t.Fatalf("role %q %s %s: got %d, want 403 (%q)", role, tc.method, tc.path, w.Code, w.Body.String())
			}
		}

		assertCollegesIntact(t, svc, mine, mine.ID, 2)
	}
}

// TestAdminCollegeRoutesStillWork is the anti-lockout test at the edge,
// complementing TestPlatformAdminsCanStillManageColleges at the service. Were the
// middleware narrower than PlatformAdminRoles, the service tests would keep passing
// while the dashboard got a 403 from the gate — a fix that ships broken.
func TestAdminCollegeRoutesStillWork(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, mine, _ := seededColleges(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("superadmin", 1), middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc, nil))

	id := strconv.Itoa(int(mine.ID))
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/colleges/"+id+"/location", `{"latitude":27.7,"longitude":85.3}`); w.Code != http.StatusOK {
		t.Fatalf("admin location: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/colleges", `{"name":"Fresh","location":"Bhaktapur","university_id":1}`); w.Code != http.StatusCreated {
		t.Fatalf("admin create: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/colleges/"+id, `{"name":"Renamed"}`); w.Code != http.StatusOK {
		t.Fatalf("admin update: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/colleges/"+id+"/approve", `{}`); w.Code != http.StatusOK {
		t.Fatalf("admin approve: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/colleges/"+id+"/featured", `{}`); w.Code != http.StatusOK {
		t.Fatalf("admin featured: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodDelete, "/api/v1/admin/colleges/"+id, ""); w.Code != http.StatusOK {
		t.Fatalf("admin delete: %d %q", w.Code, w.Body.String())
	}
}

// TestTheOwningInstitutionsOwnPinStillWorksAtTheEdge is the tenant anti-lockout
// test at the edge.
//
// The gate on the admin group is deliberately still the wide roleMW, because an
// institution account has to reach the pin route. So this is the test that proves
// the widening was safe: the owning institution gets through the gate and is
// let through by the service, while a rival gets through the gate and is refused
// by it.
func TestTheOwningInstitutionsOwnPinStillWorksAtTheEdge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, mine, theirs := seededColleges(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...), NewHandler(svc, nil))

	// Its own pin: succeeds.
	w := doJSON(r, http.MethodPut, "/api/v1/admin/colleges/"+strconv.Itoa(int(mine.ID))+"/location",
		`{"latitude":27.68,"longitude":85.32}`)
	if w.Code != http.StatusOK {
		t.Fatalf("owning institution refused its own pin: %d %q", w.Code, w.Body.String())
	}

	// A rival's pin: refused by the service, past the gate.
	w = doJSON(r, http.MethodPut, "/api/v1/admin/colleges/"+strconv.Itoa(int(theirs.ID))+"/location",
		`{"latitude":1.1,"longitude":2.2}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("institution moved a rival's pin: %d %q", w.Code, w.Body.String())
	}
}

// TestPublicFindCollegeSurfaceIsUnchanged is the cross-check against a fix that
// over-reaches. The find-college surface is public by design and takes no Viewer,
// so nothing here moved it — but "the college catalogue is platform-admin only"
// read broadly enough would mean locking the whole find-college page to admins.
func TestPublicFindCollegeSurfaceIsUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, mine, _ := seededColleges(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("", 0), middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc, nil))

	if w := doRequest(r, http.MethodGet, "/api/v1/colleges", ""); w.Code != http.StatusOK {
		t.Fatalf("public college list: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/colleges/"+strconv.Itoa(int(mine.ID)), ""); w.Code != http.StatusOK {
		t.Fatalf("public college read: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/map/colleges", ""); w.Code != http.StatusOK {
		t.Fatalf("public map: %d %q", w.Code, w.Body.String())
	}
}

// TestPlatformAdminRolesCoversEveryAdminSpelling pins the list itself. Removing
// one locks the dashboard out; adding a tenant role here would hand the whole
// catalogue to that tenant for every module that builds its gate from this.
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

// TestAnInstitutionWithoutATenantLookupIsDenied is the nil-safety property of
// TenantCollegeID. A Service built without WithTenantLookup must deny every
// non-admin, not panic and not admit: the safe direction for a missing dependency
// is closed, never open.
func TestAnInstitutionWithoutATenantLookupIsDenied(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&College{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	c := College{Name: "Lonely", Location: "X"}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// No WithTenantLookup call.
	svc := NewService(NewRepository(db))
	if got := svc.TenantCollegeID(owningInstitution()); got != 0 {
		t.Fatalf("TenantCollegeID with no lookup = %d, want 0", got)
	}
	if err := svc.UpdateCollegeLocation(owningInstitution(), c.ID, 1, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("service with no tenant lookup admitted an institution (err=%v)", err)
	}
	// An admin is unaffected — the rule is not "deny everyone".
	if err := svc.UpdateCollegeLocation(Viewer{UserID: 1, Role: "superadmin"}, c.ID, 1, 1); err != nil {
		t.Fatalf("admin refused by a service with no tenant lookup: %v", err)
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

// assertCollegesIntact checks that every seeded college is still present and still
// carries its original name. Counted with an ordinary query, so a soft delete shows
// up as a missing row.
func assertCollegesIntact(t *testing.T, svc *Service, want *College, id uint, count int64) {
	t.Helper()
	var live int64
	svc.repo.db.Model(&College{}).Count(&live)
	if live != count {
		t.Fatalf("college %d: %d colleges left, want %d", id, live, count)
	}
	var reloaded College
	if err := svc.repo.db.First(&reloaded, id).Error; err != nil {
		t.Fatalf("college %d was removed: %v", id, err)
	}
	if reloaded.Name != want.Name {
		t.Fatalf("college %d was renamed to %q, want %q", id, reloaded.Name, want.Name)
	}
}
