package jobs

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

// Who may read a job application, as pinned by the tests below:
//
//	platform admin (admin / superadmin / super_admin)  — any application
//	the applicant who submitted it                          — their own, read only
//	everyone else, including institution and
//	scholarship_provider accounts                          — nothing
//
// Job carries no tenant column, so there is no "posting organisation" to grant
// anything: the postings on /careers belong to the platform and the platform's
// admins run them. access.go has the full argument; these tests pin the
// behaviour that follows from it.

// adminViewer is the operator the superadmin dashboard authenticates as.
func adminViewer() Viewer {
	return Viewer{UserID: 1, Role: "superadmin"}
}

func institutionViewer() Viewer {
	return Viewer{UserID: 40, Role: "institution"}
}

func providerViewer() Viewer {
	return Viewer{UserID: 41, Role: "scholarship_provider"}
}

func subuserViewer() Viewer {
	return Viewer{UserID: 42, Role: "scholarship_provider_subuser"}
}

// applicantViewer is a plain signed-in student. "student" is the default role on
// internal/auth.User and is deliberately absent from roleMW, which is why the
// applicant routes must not be gated on it.
func applicantViewer(userID uint) Viewer {
	return Viewer{UserID: userID, Role: "student"}
}

// seeded builds a service over a fresh in-memory database with one published
// job and three applications: one belonging to applicant 7, two belonging to
// nobody (guest submissions through the public form).
func seededJobsAccess(t *testing.T) (*Service, *Job, []*JobApplication) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Job{}, &JobApplication{}, &jobsUserRow{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	job := Job{Title: "Backend Engineer", Department: "Engineering", Description: "Build things", JobType: "full-time", Status: "published"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}

	applicant := uint(7)
	mine := JobApplication{JobID: job.ID, FullName: "Ada Lovelace", Email: "ada@example.com", Phone: "9800000000", ResumeURL: "resumes/mine.pdf", CoverLetterURL: "resumes/mine_cl.pdf", Status: "pending", ApplicantUserID: &applicant}
	guestA := JobApplication{JobID: job.ID, FullName: "Grace Hopper", Email: "grace@example.com", Phone: "9800000001", ResumeURL: "resumes/a.pdf", Status: "pending"}
	guestB := JobApplication{JobID: job.ID, FullName: "Alan Turing", Email: "alan@example.com", Phone: "9800000002", ResumeURL: "resumes/b.pdf", Status: "pending"}
	for _, app := range []*JobApplication{&mine, &guestA, &guestB} {
		if err := db.Create(app).Error; err != nil {
			t.Fatalf("seed application: %v", err)
		}
	}

	return NewServiceWithDB(NewRepository(db), db, nil), &job, []*JobApplication{&mine, &guestA, &guestB}
}

// TestInstitutionCannotReadApplicantOfAnotherJob is the test that matters.
//
// There is no second institution and no second job in this schema to hang a
// cross-tenant pair on, so the test states the property directly: an institution
// account holds no ownership of any Job, therefore no application is readable by
// it — not the applicant's own, not a guest's, and not one attached to a job at
// all.
func TestInstitutionCannotReadApplicantOfAnotherJob(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	for i, app := range apps {
		got, err := svc.GetApplicationByID(institutionViewer(), app.ID)
		if !errors.Is(err, ErrApplicationNotFound) {
			t.Fatalf("application %d: institution read it (err=%v) — the roleMW leak is open", i, err)
		}
		if got != nil {
			t.Fatalf("application %d: institution got a record back: %+v", i, got)
		}
	}
}

// TestScholarshipProviderCannotReadApplicants pins the boundary drawn in
// access.go: a scholarship provider owns ProviderScholarship rows in
// internal/scholarshipprovider, never a Job. It is not an admin and it is not
// the applicant, so it reads nothing — including the subuser variant, which
// roleMW also admitted.
func TestScholarshipProviderCannotReadApplicants(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	for _, viewer := range []Viewer{providerViewer(), subuserViewer()} {
		for i, app := range apps {
			if _, err := svc.GetApplicationByID(viewer, app.ID); !errors.Is(err, ErrApplicationNotFound) {
				t.Fatalf("role %q application %d: readable (err=%v)", viewer.Role, i, err)
			}
		}
	}
}

// TestAdminCanReadAnyApplicant is the other half. A fix that only denies would
// pass everything above while breaking the product, and the superadmin dashboard
// is the only legitimate consumer of these routes.
func TestAdminCanReadAnyApplicant(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", " admin "} {
		viewer := Viewer{UserID: 1, Role: role}
		for i, app := range apps {
			got, err := svc.GetApplicationByID(viewer, app.ID)
			if err != nil {
				t.Fatalf("role %q application %d: %v", role, i, err)
			}
			if got.ID != app.ID || got.ResumeURL != app.ResumeURL {
				t.Fatalf("role %q application %d: got %+v want %+v", role, i, got, app)
			}
		}
	}
}

// TestApplicantCanReadOwnApplication covers the gap the brief asked about.
//
// Before this change it was not possible at all: JobApplication recorded no
// account, and the group was behind roleMW, which lists no student role. The
// submission is now stamped with the submitter's id and the read routes accept
// it. Reads only — the write routes still refuse the applicant.
func TestApplicantCanReadOwnApplication(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	got, err := svc.GetApplicationByID(applicantViewer(7), apps[0].ID)
	if err != nil {
		t.Fatalf("applicant reading own application: %v", err)
	}
	if got.ResumeURL != apps[0].ResumeURL || got.CoverLetterURL != apps[0].CoverLetterURL {
		t.Fatalf("applicant got the wrong document: %+v", got)
	}

	// The other two are guests. Applicant 7 does not thereby own them.
	for _, guest := range apps[1:] {
		if _, err := svc.GetApplicationByID(applicantViewer(7), guest.ID); !errors.Is(err, ErrApplicationNotFound) {
			t.Fatalf("applicant read a guest application (id %d): err=%v", guest.ID, err)
		}
	}
}

// TestGuestApplicationIsNeverSelfOwned guards the one place a wrong
// implementation would be silently exploitable: if SubmittedBy() returned 0 for
// a nil ApplicantUserID and that 0 were compared without a guard, then every
// unauthenticated viewer would own every guest application — turning the new
// self-read rule into the same cross-tenant leak it replaced.
func TestGuestApplicationIsNeverSelfOwned(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	for _, viewer := range []Viewer{
		{UserID: 0, Role: "student"},
		{UserID: 0, Role: "institution"},
		{UserID: 0, Role: ""},
	} {
		for i, app := range apps {
			if _, err := svc.GetApplicationByID(viewer, app.ID); !errors.Is(err, ErrApplicationNotFound) {
				t.Fatalf("user_id=0 role=%q read application %d: err=%v", viewer.Role, i, err)
			}
		}
	}
}

// TestSubmitApplicationRecordsAuthenticatedApplicant pins where the self-read
// comes from: the id must come from the session, never from the request body.
func TestSubmitApplicationRecordsAuthenticatedApplicant(t *testing.T) {
	svc, job, _ := seededJobsAccess(t)

	app, err := svc.SubmitApplication(job.ID, "Ada Lovelace", "ada2@example.com", "9800000000", "resumes/r.pdf", "", 7)
	if err != nil {
		t.Fatalf("submit as applicant: %v", err)
	}
	if app.SubmittedBy() != 7 {
		t.Fatalf("applicant id not recorded: %+v", app)
	}

	// The guest path is unchanged and yields no owner.
	guest, err := svc.SubmitApplication(job.ID, "Grace Hopper", "grace2@example.com", "9800000000", "resumes/g.pdf", "", 0)
	if err != nil {
		t.Fatalf("submit as guest: %v", err)
	}
	if guest.SubmittedBy() != 0 || guest.ApplicantUserID != nil {
		t.Fatalf("guest submission acquired an owner: %+v", guest.ApplicantUserID)
	}
}

// TestApplicantEmailCannotBeClaimedByEmailAddress is why the owner is the
// session id and not a lookup on the submitted address.
//
// The naive alternative is to resolve the submitter from the address they typed:
// "we know this address belongs to user 7, so this application is user 7's".
// That is wrong in both directions — the person submitting is not necessarily
// the person who owns the address, and it makes two different accounts' records
// collapse onto one owner. The mutation that proves it is that lookup, and this
// test is seeded so the lookup would actually resolve.
func TestApplicantEmailCannotBeClaimedByEmailAddress(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	// A real account owning the address that will be reused.
	if err := svc.repo.db.Create(&jobsUserRow{ID: 7, Email: apps[0].Email}).Error; err != nil {
		t.Fatalf("seed user owning the address: %v", err)
	}

	// A second posting, so the per-(job, email) dedupe does not stop it.
	other := Job{Title: "Frontend Engineer", Department: "Engineering", Description: "Build things", JobType: "full-time", Status: "published"}
	if err := svc.repo.db.Create(&other).Error; err != nil {
		t.Fatalf("seed second job: %v", err)
	}

	mine, err := svc.SubmitApplication(other.ID, "Someone Else", apps[0].Email, "9800000009", "resumes/impostor.pdf", "", 99)
	if err != nil {
		t.Fatalf("submit reusing another applicant's address: %v", err)
	}

	// Ownership followed the session, not the typed address.
	if mine.SubmittedBy() != 99 {
		t.Fatalf("application submitted by 99 is owned by %d — ownership was resolved from the email address", mine.SubmittedBy())
	}

	// The account that owns the address must not thereby own this submission.
	if got, err := svc.GetApplicationByID(applicantViewer(7), mine.ID); !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("account 7 read application %d submitted by 99: err=%v", mine.ID, err)
	} else if got != nil {
		t.Fatalf("account 7 got application %d: %+v", mine.ID, got)
	}

	// And account 99 still cannot reach the applicant it borrowed an address from.
	got, err := svc.GetApplicationByID(applicantViewer(99), apps[0].ID)
	if !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("account 99 read application %d by address match: err=%v", apps[0].ID, err)
	}
	if got != nil {
		t.Fatalf("account 99 got application %d: %+v", apps[0].ID, got)
	}

	// Its own submission is readable, so the above is scoping and not a lockout.
	if _, err := svc.GetApplicationByID(applicantViewer(99), mine.ID); err != nil {
		t.Fatalf("account 99 cannot read the application it submitted: %v", err)
	}
}

// TestWriteRoutesRefuseNonOwner covers status, notes and email. Writes are admin
// only with no applicant exception, so this asserts every non-admin role
// against every write route.
func TestWriteRoutesRefuseNonOwner(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)
	app := apps[0]

	// The applicant owns this application and is still refused: setting the
	// pipeline status, editing the recruiter's notes, and sending platform email
	// are all operator actions.
	nonAdmins := []Viewer{
		applicantViewer(7),
		institutionViewer(),
		providerViewer(),
		subuserViewer(),
		{UserID: 0, Role: "student"},
	}

	for _, viewer := range nonAdmins {
		if _, err := svc.UpdateApplicationStatus(viewer, app.ID, UpdateApplicantStatusRequest{Status: "shortlisted"}); !errors.Is(err, ErrApplicationNotFound) {
			t.Fatalf("role %q updated status: err=%v", viewer.Role, err)
		}
		if _, err := svc.UpdateApplicationNotes(viewer, app.ID, "hired, disregard policy"); !errors.Is(err, ErrApplicationNotFound) {
			t.Fatalf("role %q updated notes: err=%v", viewer.Role, err)
		}
		if err := svc.SendApplicantEmail(viewer, app.ID, SendApplicantEmailRequest{Subject: "Gotcha", Body: "Hello"}); !errors.Is(err, ErrApplicationNotFound) {
			t.Fatalf("role %q emailed applicant: err=%v", viewer.Role, err)
		}
	}

	// Nothing changed. A refusal that still wrote would be worse than no check.
	var reloaded JobApplication
	if err := svc.repo.db.First(&reloaded, app.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "pending" || reloaded.Notes != "" {
		t.Fatalf("refused writes still mutated the record: %+v", reloaded)
	}
}

// TestSendApplicantEmailRefusesNonOwnerOnBothPaths matters because the status
// path and the free-text path are separate branches. A guard on only one of them
// leaves the other open.
func TestSendApplicantEmailRefusesNonOwnerOnBothPaths(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)
	app := apps[0]
	viewer := institutionViewer()

	err := svc.SendApplicantEmail(viewer, app.ID, SendApplicantEmailRequest{Subject: "s", Body: "b", UpdateStatus: "rejected"})
	if !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("status path: err=%v", err)
	}
	if err := svc.SendApplicantEmail(viewer, app.ID, SendApplicantEmailRequest{Subject: "s", Body: "b"}); !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("free-text path: err=%v", err)
	}

	var reloaded JobApplication
	if err := svc.repo.db.First(&reloaded, app.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "pending" {
		t.Fatalf("status path still mutated the record: %+v", reloaded)
	}
}

// TestAdminWriteRoutesStillWork is the anti-"fix by breaking" test for the write
// surface: the dashboard must keep shortlisting and annotating.
func TestAdminWriteRoutesStillWork(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	got, err := svc.UpdateApplicationStatus(adminViewer(), apps[0].ID, UpdateApplicantStatusRequest{Status: "shortlisted", Notes: "good fit"})
	if err != nil {
		t.Fatalf("admin status update: %v", err)
	}
	if got.Status != "shortlisted" || got.Notes != "good fit" {
		t.Fatalf("status/notes not applied: %+v", got)
	}
	if got, err := svc.UpdateApplicationNotes(adminViewer(), apps[0].ID, "call on monday"); err != nil {
		t.Fatalf("admin notes update: %v", err)
	} else if got.Notes != "call on monday" {
		t.Fatalf("notes not applied: %+v", got)
	}
}

// TestListApplicationsIsAdminOnly: this route is the enumeration shortcut. One
// call returns name, email and phone for every applicant to a posting, so it
// has to be scoped even though the per-id routes are.
func TestListApplicationsIsAdminOnly(t *testing.T) {
	svc, job, _ := seededJobsAccess(t)

	if _, err := svc.ListApplications(adminViewer(), job.ID, "", "", 1, 10); err != nil {
		t.Fatalf("admin listing: %v", err)
	}

	for _, viewer := range []Viewer{institutionViewer(), providerViewer(), subuserViewer(), applicantViewer(7), {UserID: 0, Role: "student"}} {
		got, err := svc.ListApplications(viewer, job.ID, "", "", 1, 10)
		if !errors.Is(err, ErrApplicationNotFound) {
			t.Fatalf("role %q listed applicants: err=%v", viewer.Role, err)
		}
		if got != nil {
			t.Fatalf("role %q got a listing: %+v", viewer.Role, got)
		}
	}
}

// TestEnumerationYieldsNothing walks sequential ids the way the original attack
// did and asserts that nothing is granted to a caller with no claim on the
// record. The applicant's own id is the one legitimate hit, so it is allowed
// exactly that and no more — a check that returned "nothing at all" for everyone
// would be satisfied by a service that denies applicants too, and that is the
// failure mode where a security fix quietly becomes a lockout.
func TestEnumerationYieldsNothing(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)

	maxID := apps[len(apps)-1].ID

	// No claim on any record: every id must be refused.
	for _, viewer := range []Viewer{institutionViewer(), providerViewer(), subuserViewer(), {UserID: 0, Role: "student"}} {
		var granted []uint
		for id := uint(1); id <= maxID+5; id++ {
			if got, err := svc.GetApplicationByID(viewer, id); err == nil && got != nil {
				granted = append(granted, id)
			}
		}
		if len(granted) != 0 {
			t.Fatalf("role %q enumerated and got ids %v (records exist at ids 1..%d)", viewer.Role, granted, maxID)
		}
	}

	// One legitimate hit, and nothing else.
	var granted []uint
	for id := uint(1); id <= maxID+5; id++ {
		if got, err := svc.GetApplicationByID(applicantViewer(7), id); err == nil && got != nil {
			granted = append(granted, id)
		}
	}
	if len(granted) != 1 || granted[0] != apps[0].ID {
		t.Fatalf("applicant got ids %v, want exactly [%d]", granted, apps[0].ID)
	}
}

// TestAbsenceAndNonOwnershipAreIndistinguishable pins the 404-vs-403 decision
// where it is observable. Same sentinel, same shape, for an id nobody has and an
// id that belongs to somebody else.
func TestAbsenceAndNonOwnershipAreIndistinguishable(t *testing.T) {
	svc, _, apps := seededJobsAccess(t)
	viewer := institutionViewer()

	existing := apps[1].ID
	absent := apps[1].ID + 1000

	gotExisting, errExisting := svc.GetApplicationByID(viewer, existing)
	gotAbsent, errAbsent := svc.GetApplicationByID(viewer, absent)

	if !errors.Is(errExisting, ErrApplicationNotFound) || !errors.Is(errAbsent, ErrApplicationNotFound) {
		t.Fatalf("errors differ: existing=%v absent=%v", errExisting, errAbsent)
	}
	if errExisting.Error() != errAbsent.Error() {
		t.Fatalf("messages leak existence: %q vs %q", errExisting, errAbsent)
	}
	if (gotExisting == nil) != (gotAbsent == nil) {
		t.Fatalf("result shape differs: existing=%v absent=%v", gotExisting, gotAbsent)
	}

	// The admin sees the difference, so 404 is not "the id does not exist".
	if _, err := svc.GetApplicationByID(adminViewer(), absent); !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("admin reading a nonexistent id: err=%v", err)
	}
	if _, err := svc.GetApplicationByID(adminViewer(), existing); err != nil {
		t.Fatalf("admin reading an existing id: %v", err)
	}
}

// TestApplicantRoutesUseTheServiceAsTheGuard covers the route-level half.
//
// A student token must reach the service rather than being turned away by
// middleware, because otherwise the applicant's own read is unreachable however
// correct the service is. The two shapes are mounted side by side and told apart
// by their answers: roleMW answers 403 with its own message, the service answers
// 404 with "application not found". Only the second is production.
//
// The 200 path for a document is not asserted here — storage.Get needs an
// object store, so ServeResume cannot be driven to 200 in a unit test. The
// applicant's own read is pinned at the service level instead, by
// TestApplicantCanReadOwnApplication.
func TestApplicantRoutesUseTheServiceAsTheGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	studentToken := func(c *gin.Context) {
		c.Set("user_id", uint(7))
		c.Set("user_role", "student")
		c.Next()
	}

	// Production shape: the group is behind authMW only.
	svc, _, apps := seededJobsAccess(t)
	production := gin.New()
	RegisterRoutes(production, studentToken, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	// The applicant themselves, on the write route, gets the service's answer
	// and not the middleware's — which proves the guard let them through.
	w := doJSON(production, http.MethodPut,
		"/api/v1/superadmin/jobs/applicants/"+itoa(apps[0].ID)+"/status",
		`{"status":"shortlisted"}`)
	if w.Code == http.StatusForbidden {
		t.Fatalf("applicant was stopped by middleware (403); the group's guard is still roleMW")
	}
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "application not found") {
		t.Fatalf("want the service's 404, got %d %q", w.Code, w.Body.String())
	}

	// The old shape, for contrast: the same handler behind authMW + roleMW. It is
	// mounted by hand because RegisterRoutes no longer applies roleMW to this
	// group, which is the change being pinned — passing a wider roleMW in cannot
	// re-widen it, and that is the property.
	old := gin.New()
	oldGroup := old.Group("/api/v1/superadmin/jobs/applicants")
	oldGroup.Use(studentToken)
	oldGroup.Use(middleware.RequireRole(PlatformAdminRoles()...))
	oldGroup.PUT("/:id/status", NewHandler(svc).UpdateApplicantStatus)

	w = doJSON(old, http.MethodPut,
		"/api/v1/superadmin/jobs/applicants/"+itoa(apps[0].ID)+"/status",
		`{"status":"shortlisted"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected the roleMW shape to answer 403, got %d", w.Code)
	}

	// And the resume route is behind the same guard as the write routes.
	w = doRequest(production, http.MethodGet, "/api/v1/superadmin/jobs/applicants/"+itoa(apps[0].ID)+"/resume", "")
	if w.Code == http.StatusForbidden {
		t.Fatalf("applicant blocked from the resume route by middleware, not by the service")
	}
}

// TestInstitutionRoutesAnswerNotFoundNotForbidden pins the 404 choice at the
// edge. roleMW's message is what an existence oracle would look like from the
// outside; the service must not emit it for these routes.
func TestInstitutionRoutesAnswerNotFoundNotForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _, apps := seededJobsAccess(t)

	r := gin.New()
	RegisterRoutes(r, func(c *gin.Context) {
		c.Set("user_id", uint(40))
		c.Set("user_role", "institution")
		c.Next()
	}, middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	id := itoa(apps[1].ID) // an application that exists
	absent := itoa(apps[1].ID + 1000)

	for _, path := range []string{
		"/api/v1/superadmin/jobs/applicants/" + id + "/resume",
		"/api/v1/superadmin/jobs/applicants/" + id + "/cover-letter",
	} {
		w := doRequest(r, http.MethodGet, path, "")
		if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "Insufficient permissions") {
			t.Fatalf("GET %s: got %d %q", path, w.Code, w.Body.String())
		}
		w = doRequest(r, http.MethodGet, "/api/v1/superadmin/jobs/applicants/"+absent+"/resume", "")
		if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "Insufficient permissions") {
			t.Fatalf("GET absent id: got %d %q", w.Code, w.Body.String())
		}
	}

	for _, path := range []string{
		"/api/v1/superadmin/jobs/applicants/" + id + "/status",
		"/api/v1/superadmin/jobs/applicants/" + id + "/notes",
		"/api/v1/superadmin/jobs/applicants/" + id + "/email",
	} {
		method := http.MethodPost
		if strings.HasSuffix(path, "/notes") {
			method = http.MethodPut
		}
		w := doJSON(r, method, path, `{"status":"shortlisted","notes":"x","subject":"s","body":"b"}`)
		if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "Insufficient permissions") {
			t.Fatalf("%s %s: got %d %q", method, path, w.Code, w.Body.String())
		}
	}

	// The whole-job listing, which is the one-shot enumeration shortcut.
	listing := doRequest(r, http.MethodGet, "/api/v1/superadmin/jobs/"+itoa(apps[1].JobID)+"/applicants", "")
	if listing.Code != http.StatusNotFound {
		t.Fatalf("institution listed applicants: %d %q", listing.Code, listing.Body.String())
	}
}

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }

func doRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	return doRequest(r, method, path, body)
}
