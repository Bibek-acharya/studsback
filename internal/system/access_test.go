package system

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

// Who may read the platform inquiry inbox and rewrite the ad configuration, as
// pinned by the tests below:
//
//	platform admin (admin / superadmin / super_admin) — all nine routes
//	everyone else, including institution, scholarship_provider,
//	scholarship_provider_subuser and student             — nothing
//
// Both are first-party platform data. The inbox is the one the module itself
// notifies to ForRoles("superadmin", "admin") and nobody else, and the
// non-admin principal that legitimately needs an inbox already has a correctly
// scoped route in internal/institution. Ad has no institution column at all — it
// describes a slot on the platform's own pages. access.go has the full argument;
// these tests pin what follows from it.

// sharedRoleMWList is the role list cmd/server/main.go builds for roleMW, in the
// same order with the same aliases. Reproduced rather than imported because
// main.go is not importable, and it matters that it is exact: this is the list
// that admitted every tenant to the inbox and to the ad config.
var sharedRoleMWList = []string{
	"admin", "super_admin", "scholarship_provider", "scholarship-provider",
	"Scholarship Provider", "scholarship_provider_subuser", "institution",
}

// seededInbox builds a service over a database holding two visitor inquiries and
// one ad.
//
// The inquiries are the PII: names, emails, phone numbers and message bodies
// submitted through POST /system/contact, which has no auth, by people with no
// account on the platform. One carries an institution_id and one does not, which
// is the realistic shape — the public form lets a visitor name an institution, and
// most never do.
func seededInbox(t *testing.T) (*Service, *ContactInquiry, *Ad) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ContactInquiry{}, &Ad{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	guest := ContactInquiry{
		Name: "Grace Hopper", Email: "grace@example.com", Phone: "9800000000",
		Subject: "Admission requirements", Message: "What are the entry requirements?",
		Type: "admission", Status: "new",
	}
	claimed := ContactInquiry{
		InstitutionID: ptrUint(9), Name: "Alan Turing", Email: "alan@example.com",
		Phone: "9800000001", Subject: "Tour", Message: "Can we visit the campus?",
		Type: "general", Status: "new",
	}
	ad := Ad{Title: "Spring Campaign", Page: "landing", Position: "popup", Active: true}

	for _, row := range []interface{}{&guest, &claimed, &ad} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// A notifier rather than nil: SubmitContactInquiry calls s.notifier.ForRoles
	// with no nil check (service.go), so a nil notifier panics on the public
	// contact form. Not this module's bug to fix here — the test just has to
	// supply one to exercise the route.
	return NewService(NewRepository(db), &captureNotifier{}), &guest, &ad
}

func ptrUint(v uint) *uint { return &v }

// adCreateBody and contactBody are the minimum that satisfies AdRequest's and
// ContactInquiryRequest's binding tags. Discovered the hard way: a body that fails
// validation returns 400 from the handler before the service is ever reached,
// which would have let a test pass without the guard being involved.
const (
	adCreateBody = `{"title":"Autumn","link_url":"https://example.com","page":"landing","position":"showcase"}`
	// The update route binds the same AdRequest as create, so a partial body like
	// {"title":"..."} is rejected 400 by the handler before the service is reached.
	adUpdateBody = `{"title":"Renamed","link_url":"https://example.com","page":"landing","position":"popup"}`
	contactBody  = `{"name":"Visitor","email":"v@example.com","subject":"Hello","message":"Hello there"}`
)

func adRequest(title string) AdRequest {
	return AdRequest{Title: title, LinkURL: "https://example.com", Page: "landing", Position: "showcase"}
}

// nonAdminViewers is every role roleMW used to admit here and that must reach
// nothing. The institution is the case from the brief — it could read and delete
// every visitor's contact message and rewrite the site-wide ad config.
func nonAdminViewers() []Viewer {
	return []Viewer{
		{UserID: 40, Role: "institution"},
		{UserID: 41, Role: "scholarship_provider"},
		{UserID: 42, Role: "scholarship_provider_subuser"},
		{UserID: 7, Role: "student"},
		{UserID: 0, Role: ""},
	}
}

// TestInboxAndAdConfigAreAdminOnlyAtTheService is the core of the fix: the
// service is the authority, and every non-admin role is refused by all nine
// routes.
//
// The "and nothing was read or written" assertions matter as much as the errors.
// A guard placed after the query or the delete would return the right error, hand
// back the PII, and still pass every error check here.
func TestInboxAndAdConfigAreAdminOnlyAtTheService(t *testing.T) {
	for _, viewer := range nonAdminViewers() {
		svc, inquiry, ad := seededInbox(t)

		inquiries, total, err := svc.GetContactInquiriesAsAdmin(viewer, 1, 20, "", "")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q listed the inbox (err=%v)", viewer.Role, err)
		}
		if inquiries != nil || total != 0 {
			t.Fatalf("role %q got inbox contents: %d rows %+v", viewer.Role, total, inquiries)
		}

		got, err := svc.GetContactInquiryByIDAsAdmin(viewer, inquiry.ID)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q read inquiry %d (err=%v)", viewer.Role, inquiry.ID, err)
		}
		if got != nil {
			t.Fatalf("role %q read inquiry %d: %+v", viewer.Role, inquiry.ID, got)
		}

		updated, err := svc.UpdateContactInquiryStatusAsAdmin(viewer, inquiry.ID, "closed")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q changed inquiry status (err=%v)", viewer.Role, err)
		}
		if updated != nil {
			t.Fatalf("role %q updated an inquiry: %+v", viewer.Role, updated)
		}

		if err := svc.DeleteContactInquiryAsAdmin(viewer, inquiry.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q deleted inquiry %d (err=%v)", viewer.Role, inquiry.ID, err)
		}

		ads, adTotal, err := svc.GetAds(viewer, 1, 20, "", "", nil)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q listed ads (err=%v)", viewer.Role, err)
		}
		if ads != nil || adTotal != 0 {
			t.Fatalf("role %q got the ad config: %d rows %+v", viewer.Role, adTotal, ads)
		}

		gotAd, err := svc.GetAdByID(viewer, ad.ID)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q read ad %d (err=%v)", viewer.Role, ad.ID, err)
		}
		if gotAd != nil {
			t.Fatalf("role %q read ad %d: %+v", viewer.Role, ad.ID, gotAd)
		}

		created, err := svc.CreateAd(viewer, adRequest("Sneaky Campaign"))
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q created an ad (err=%v)", viewer.Role, err)
		}
		if created != nil {
			t.Fatalf("role %q created an ad: %+v", viewer.Role, created)
		}

		rewritten, err := svc.UpdateAd(viewer, ad.ID, adRequest("Rewritten by a tenant"))
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q rewrote ad %d (err=%v)", viewer.Role, ad.ID, err)
		}
		if rewritten != nil {
			t.Fatalf("role %q rewrote an ad: %+v", viewer.Role, rewritten)
		}

		if err := svc.DeleteAd(viewer, ad.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q deleted ad %d (err=%v)", viewer.Role, ad.ID, err)
		}

		assertInboxIntact(t, svc, viewer.Role)
		assertAdsIntact(t, svc, viewer.Role)
	}
}

// TestOnlyAnAdminCanDestroyAVisitorsMessage is the destructive half stated on its
// own: whoever else can reach these routes, they cannot delete a visitor's
// contact message or the platform's ad configuration.
//
// Asserts the rows are still readable afterwards, not merely that the error is
// right. A guard that ran after the delete would return the right error and still
// destroy the record.
func TestOnlyAnAdminCanDestroyAVisitorsMessage(t *testing.T) {
	for _, viewer := range nonAdminViewers() {
		svc, inquiry, ad := seededInbox(t)

		if err := svc.DeleteContactInquiryAsAdmin(viewer, inquiry.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q deleted inquiry %d (err=%v)", viewer.Role, inquiry.ID, err)
		}
		if err := svc.DeleteAd(viewer, ad.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q deleted ad %d (err=%v)", viewer.Role, ad.ID, err)
		}

		// Both still there, to an ordinary query.
		var liveInquiries, liveAds int64
		svc.repo.db.Model(&ContactInquiry{}).Count(&liveInquiries)
		svc.repo.db.Model(&Ad{}).Count(&liveAds)
		if liveInquiries != 2 {
			t.Fatalf("role %q removed visitor messages: %d left, want 2", viewer.Role, liveInquiries)
		}
		if liveAds != 1 {
			t.Fatalf("role %q removed the ad config: %d left, want 1", viewer.Role, liveAds)
		}
	}
}

// TestPlatformAdminsCanStillRunTheInboxAndAds is the anti-lockout test, written
// explicitly. A fix that only denies would pass every other test in this file
// while breaking the product: the superadmin dashboard is the only legitimate
// consumer of the inbox and the ad manager, and both are live UIs
// (contact.api.ts, adminAdApi.ts, PopupManagementTab.tsx, ShowcaseBannerTab.tsx).
//
// Every admin spelling the codebase uses is exercised, including the casing and
// padding variants, because RequireRole and IsPlatformAdmin both normalise and the
// gate and the service must agree on that.
func TestPlatformAdminsCanStillRunTheInboxAndAds(t *testing.T) {
	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", " admin "} {
		viewer := Viewer{UserID: 1, Role: role}
		svc, inquiry, ad := seededInbox(t)

		inquiries, total, err := svc.GetContactInquiriesAsAdmin(viewer, 1, 20, "", "")
		if err != nil {
			t.Fatalf("role %q inbox list: %v", role, err)
		}
		if total != 2 || len(inquiries) != 2 {
			t.Fatalf("role %q inbox list: got %d rows, want the 2 seeded", role, total)
		}

		got, err := svc.GetContactInquiryByIDAsAdmin(viewer, inquiry.ID)
		if err != nil {
			t.Fatalf("role %q inquiry read: %v", role, err)
		}
		if got.Email != "grace@example.com" {
			t.Fatalf("role %q read the wrong inquiry: %+v", role, got)
		}

		closed, err := svc.UpdateContactInquiryStatusAsAdmin(viewer, inquiry.ID, "closed")
		if err != nil {
			t.Fatalf("role %q status update: %v", role, err)
		}
		if closed.Status != "closed" {
			t.Fatalf("role %q status update: got %q", role, closed.Status)
		}

		ads, adTotal, err := svc.GetAds(viewer, 1, 20, "", "", nil)
		if err != nil {
			t.Fatalf("role %q ad list: %v", role, err)
		}
		if adTotal != 1 || len(ads) != 1 {
			t.Fatalf("role %q ad list: got %d rows, want the seeded one", role, adTotal)
		}

		gotAd, err := svc.GetAdByID(viewer, ad.ID)
		if err != nil {
			t.Fatalf("role %q ad read: %v", role, err)
		}
		if gotAd.Title != "Spring Campaign" {
			t.Fatalf("role %q read the wrong ad: %+v", role, gotAd)
		}

		created, err := svc.CreateAd(viewer, adRequest("Autumn Campaign"))
		if err != nil {
			t.Fatalf("role %q ad create: %v", role, err)
		}
		if created.ID == 0 || created.Title != "Autumn Campaign" {
			t.Fatalf("role %q ad create: got %+v", role, created)
		}

		rewritten, err := svc.UpdateAd(viewer, ad.ID, adRequest("Spring Campaign 2027"))
		if err != nil {
			t.Fatalf("role %q ad update: %v", role, err)
		}
		if rewritten.Title != "Spring Campaign 2027" {
			t.Fatalf("role %q ad update: got %q", role, rewritten.Title)
		}

		if err := svc.DeleteAd(viewer, ad.ID); err != nil {
			t.Fatalf("role %q ad delete: %v", role, err)
		}
		if err := svc.DeleteContactInquiryAsAdmin(viewer, inquiry.ID); err != nil {
			t.Fatalf("role %q inquiry delete: %v", role, err)
		}

		// The writes really happened. A service that refused everything would
		// pass every check above and fail here.
		var liveInquiries, liveAds int64
		svc.repo.db.Model(&ContactInquiry{}).Count(&liveInquiries)
		svc.repo.db.Model(&Ad{}).Count(&liveAds)
		if liveInquiries != 1 {
			t.Fatalf("role %q inquiry delete: %d left, want 1", role, liveInquiries)
		}
		if liveAds != 1 {
			t.Fatalf("role %q ad delete: %d left, want only the newly created one", role, liveAds)
		}
	}
}

// TestInboxAndAdGateCannotBeWidenedByTheCaller is the defence-in-depth test, and
// the reason the service checks are not redundant with the middleware.
//
// It mounts the production shape but hands RegisterRoutes the ACTUAL shared
// roleMW list from cmd/server/main.go — the one that admitted every tenant. The
// middleware now lets an institution through, exactly as it always did, and the
// request must still be refused. If this fails, the module is relying on its caller
// to pass the right list, which is the fragility that let the original hole exist.
func TestInboxAndAdGateCannotBeWidenedByTheCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, inquiry, ad := seededInbox(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...), nil, NewHandler(svc))

	// The destructive one first.
	w := doRequest(r, http.MethodDelete, "/api/v1/admin/inquiries/"+strconv.Itoa(int(inquiry.ID)), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution delete the inquiry: %d %q", w.Code, w.Body.String())
	}
	// The read, because leaking the PII is the other half of the hole.
	w = doRequest(r, http.MethodGet, "/api/v1/admin/inquiries", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution read the inbox: %d %q", w.Code, w.Body.String())
	}
	w = doJSON(r, http.MethodPost, "/api/v1/admin/ads", adCreateBody)
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution create an ad: %d %q", w.Code, w.Body.String())
	}

	// The refusals came from the service and not the middleware, which is what
	// distinguishes "the gate is narrow" from "the gate was ignored". RequireRole's
	// body names the role; the service's does not.
	if strings.Contains(w.Body.String(), "Role:") {
		t.Fatalf("refusal came from the middleware, not the service: %q", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), ErrForbidden.Error()) {
		t.Fatalf("want the service's %q, got %q", ErrForbidden, w.Body.String())
	}

	// Nothing was read or written.
	var liveInquiries, liveAds int64
	svc.repo.db.Model(&ContactInquiry{}).Count(&liveInquiries)
	svc.repo.db.Model(&Ad{}).Count(&liveAds)
	if liveInquiries != 2 || liveAds != 1 {
		t.Fatalf("widened gate changed state: %d inquiries, %d ads", liveInquiries, liveAds)
	}
	_ = ad
}

// TestInboxAndAdRoutesRefuseNonAdminsAtTheEdge covers the route half with the
// production shape: authMW plus a gate built from PlatformAdminRoles, exactly as
// cmd/server/main.go now does.
//
// Each of the nine routes is exercised per role, because a guard applied to eight
// of them is a hole in the ninth.
func TestInboxAndAdRoutesRefuseNonAdminsAtTheEdge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, role := range []string{"institution", "scholarship_provider", "scholarship_provider_subuser", "student"} {
		svc, inquiry, ad := seededInbox(t)
		r := gin.New()
		RegisterRoutes(r, viewerMW(role, 40), nil, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

		id := strconv.Itoa(int(inquiry.ID))
		adID := strconv.Itoa(int(ad.ID))

		for _, tc := range []struct {
			method, path, body string
		}{
			{http.MethodGet, "/api/v1/admin/inquiries", ""},
			{http.MethodGet, "/api/v1/admin/inquiries/" + id, ""},
			{http.MethodPut, "/api/v1/admin/inquiries/" + id + "/status", `{"status":"closed"}`},
			{http.MethodDelete, "/api/v1/admin/inquiries/" + id, ""},
			{http.MethodGet, "/api/v1/admin/ads", ""},
			{http.MethodGet, "/api/v1/admin/ads/" + adID, ""},
			{http.MethodPost, "/api/v1/admin/ads", adCreateBody},
			{http.MethodPut, "/api/v1/admin/ads/" + adID, adUpdateBody},
			{http.MethodDelete, "/api/v1/admin/ads/" + adID, ""},
		} {
			w := doJSON(r, tc.method, tc.path, tc.body)
			if w.Code != http.StatusForbidden {
				t.Fatalf("role %q %s %s: got %d, want 403 (%q)", role, tc.method, tc.path, w.Code, w.Body.String())
			}
		}

		// The middleware stopped these before the handler, so nothing was touched.
		assertInboxIntact(t, svc, role)
		assertAdsIntact(t, svc, role)
	}
}

// TestTheInboxAndAdGateIsActuallyMountedOnTheNarrowList pins the ROUTE WIRING
// itself, which no other test in this file can distinguish on its own.
//
// Every other test here would still pass if routes.go mounted the inbox on roleMW
// instead of adminRoleMW, because the service refuses the same callers either way —
// which is the defence in depth working, but it means the middleware regression
// would ship silently. So this asserts which of the two refused, by their messages:
// RequireRole answers "Insufficient permissions. Role: <role>" and the service
// answers a bare "insufficient permissions".
//
// Falsified by swapping inbox.Use(adminRoleMW) for inbox.Use(roleMW): the refusals
// then come from the service, the body loses "Role:", and this fails.
func TestTheInboxAndAdGateIsActuallyMountedOnTheNarrowList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, _, _ := seededInbox(t)
	// roleMW is handed the real shared list. If the inbox group is correctly mounted
	// on adminRoleMW, this wider gate is irrelevant to it and the refusal below
	// comes from the narrow middleware.
	r := gin.New()
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...),
		middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	w := doRequest(r, http.MethodGet, "/api/v1/admin/inquiries", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("institution read the inbox: %d %q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Role:") {
		t.Fatalf("refusal did not come from RequireRole, so the inbox group is not "+
			"mounted on the narrow list: %q", w.Body.String())
	}
}

// TestAdminInboxAndAdRoutesStillWork is the anti-lockout test at the edge,
// complementing TestPlatformAdminsCanStillRunTheInboxAndAds at the service. Were
// the middleware list narrower than PlatformAdminRoles, the service tests would
// keep passing while the dashboard got a 403 from the gate — a fix that ships
// broken.
func TestAdminInboxAndAdRoutesStillWork(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, inquiry, ad := seededInbox(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("superadmin", 1), nil, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	id := strconv.Itoa(int(inquiry.ID))
	adID := strconv.Itoa(int(ad.ID))

	if w := doRequest(r, http.MethodGet, "/api/v1/admin/inquiries", ""); w.Code != http.StatusOK {
		t.Fatalf("admin inbox list: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/admin/inquiries/"+id, ""); w.Code != http.StatusOK {
		t.Fatalf("admin inquiry read: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/inquiries/"+id+"/status", `{"status":"resolved"}`); w.Code != http.StatusOK {
		t.Fatalf("admin status update: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/admin/ads", ""); w.Code != http.StatusOK {
		t.Fatalf("admin ad list: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/admin/ads/"+adID, ""); w.Code != http.StatusOK {
		t.Fatalf("admin ad read: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/ads", adCreateBody); w.Code != http.StatusCreated {
		t.Fatalf("admin ad create: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/ads/"+adID, adUpdateBody); w.Code != http.StatusOK {
		t.Fatalf("admin ad update: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodDelete, "/api/v1/admin/inquiries/"+id, ""); w.Code != http.StatusOK {
		t.Fatalf("admin inquiry delete: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodDelete, "/api/v1/admin/ads/"+adID, ""); w.Code != http.StatusOK {
		t.Fatalf("admin ad delete: %d %q", w.Code, w.Body.String())
	}
}

// TestPublicContactAndAdReadsAreUnaffected is the cross-check against a fix that
// over-reaches.
//
// The contact form and the ad slots are PUBLIC: POST /system/contact has no auth
// and GET /system/ads serves the active placements to anyone. "The inbox and the
// ad config are platform-admin only" read too broadly would mean locking the
// contact form and the advertising itself to admins, which is the opposite of the
// intent. GetActiveAds and TrackAdClick therefore take no Viewer at all, and this
// pins that they still answer a caller with no role.
func TestPublicContactAndAdReadsAreUnaffected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, _, ad := seededInbox(t)
	r := gin.New()
	RegisterRoutes(r, viewerMW("", 0), nil, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	// The contact form still works for a signed-out prospective student.
	if w := doJSON(r, http.MethodPost, "/api/v1/system/contact", contactBody); w.Code != http.StatusCreated {
		t.Fatalf("public contact form: %d %q", w.Code, w.Body.String())
	}
	// The ad slot is still served to the public, and a public click still counts.
	if w := doRequest(r, http.MethodGet, "/api/v1/system/ads", ""); w.Code != http.StatusOK {
		t.Fatalf("public ads: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/system/ads/"+strconv.Itoa(int(ad.ID))+"/click", ""); w.Code != http.StatusOK {
		t.Fatalf("public ad click: %d %q", w.Code, w.Body.String())
	}

	// The click really counted, so the public write is still live and not inert.
	reloaded, err := svc.repo.FindAdByID(ad.ID)
	if err != nil {
		t.Fatalf("reload ad: %v", err)
	}
	if reloaded.Clicks != 1 {
		t.Fatalf("public click did not count: clicks=%d, want 1", reloaded.Clicks)
	}
}

// TestPlatformAdminRolesCoversEveryAdminSpelling pins the list itself, since
// cmd/server/main.go builds the gate from it. Adding an admin role here widens
// the inbox and the ad config for everyone at once; removing one locks the
// dashboard out. Both are caught by this test rather than by whichever test
// happens to use that spelling.
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

	// The same list must not admit anything a tenant can hold. This is the half
	// that matters and the reason the gate is not roleMW.
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

// TestInstitutionScopedInquiryReadIsUnchanged guards the other half of the
// decision.
//
// The reason the platform inbox can be admin-only without locking institutions out
// is that internal/institution already serves them a correctly scoped one, via
// GetInstitutionInquiries with the id off the caller's own account. That method
// takes no Viewer and must keep working — if a future edit put the admin gate on
// it, institutions would lose access to their own messages, which is the lockout
// this whole change had to avoid.
func TestInstitutionScopedInquiryReadIsUnchanged(t *testing.T) {
	svc, guest, _ := seededInbox(t)

	// Institution 9's own inquiry: visible to it.
	mine, total, err := svc.GetInstitutionInquiries(9, 1, 20, "", "", "")
	if err != nil {
		t.Fatalf("institution 9 read its own inquiries: %v", err)
	}
	if total != 1 {
		t.Fatalf("institution 9 saw %d inquiries, want only its own 1", total)
	}
	if mine[0].ID != guest.ID+1 {
		t.Fatalf("institution 9 got the wrong inquiry: %+v", mine[0])
	}

	// An institution with no inquiries sees none of anybody else's.
	other, otherTotal, err := svc.GetInstitutionInquiries(77, 1, 20, "", "", "")
	if err != nil {
		t.Fatalf("institution 77 read: %v", err)
	}
	if otherTotal != 0 || len(other) != 0 {
		t.Fatalf("institution 77 saw %d inquiries it does not own: %+v", otherTotal, other)
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
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// assertInboxIntact checks that every seeded visitor message is still there and
// still unread by a tenant. Counted with an ordinary query, so a soft delete shows
// up as a missing row.
func assertInboxIntact(t *testing.T, svc *Service, who string) {
	t.Helper()
	var live int64
	svc.repo.db.Model(&ContactInquiry{}).Count(&live)
	if live != 2 {
		t.Fatalf("%s: %d visitor messages left, want both seeded ones", who, live)
	}
}

func assertAdsIntact(t *testing.T, svc *Service, who string) {
	t.Helper()
	var live int64
	svc.repo.db.Model(&Ad{}).Count(&live)
	if live != 1 {
		t.Fatalf("%s: %d ads left, want the seeded one", who, live)
	}
	var reloaded Ad
	if err := svc.repo.db.First(&reloaded, "title = ?", "Spring Campaign").Error; err != nil {
		t.Fatalf("%s: the ad config was rewritten: %v", who, err)
	}
	if reloaded.Title != "Spring Campaign" {
		t.Fatalf("%s: ad title changed to %q", who, reloaded.Title)
	}
}
