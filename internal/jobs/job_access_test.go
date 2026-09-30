package jobs

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

// Who may change the job catalogue, as pinned by the tests below:
//
//	platform admin (admin / superadmin / super_admin) — every CRUD route
//	everyone else, including institution, scholarship_provider,
//	scholarship_provider_subuser and student            — nothing
//
// This is the second half of the module. The applicant routes were scoped in
// 4097dfc; these five routes were mounted behind the shared roleMW and had no
// service-layer check at all, so an ordinary paying customer could create, edit
// and delete postings on the platform's own /careers page. Delete is the
// destructive one: it cascades over every application to the posting and destroys
// each applicant's stored resume and cover letter. access.go has the full
// argument; these tests pin the behaviour that follows from it.

const (
	createJobBody = `{"title":"Backend Engineer","department":"Engineering",` +
		`"description":"Build things","job_type":"full-time","status":"published"}`
	updateJobBody = `{"title":"Rewritten by a tenant"}`
)

// sharedRoleMWList is the role list cmd/server/main.go builds for roleMW, in the
// same order and with the same aliases. It is reproduced here rather than imported
// because main.go is not importable, and it matters that it is exact: this is the
// list that admitted every tenant to the catalogue.
var sharedRoleMWList = []string{
	"admin", "super_admin", "scholarship_provider", "scholarship-provider",
	"Scholarship Provider", "scholarship_provider_subuser", "institution",
}

// jobCrudSeed returns a service over a database holding one published job with
// three applications, plus a recorder of every object path the service asked the
// object store to remove. The recorder is what makes the cascade observable
// without a live object store, and it is what distinguishes "refused before the
// delete" from "refused after it".
func jobCrudSeed(t *testing.T) (*Service, *Job, []*JobApplication, *[]string) {
	t.Helper()

	svc, job, apps := seededJobsAccess(t)

	deleted := &[]string{}
	svc.deleteObject = func(objectPath string) error {
		*deleted = append(*deleted, objectPath)
		return nil
	}
	return svc, job, apps, deleted
}

// nonAdminViewers is every role roleMW used to admit here and that must not reach
// a catalogue mutation. The institution is the case from the brief; the provider
// and its subuser were in the same allow-list; the student is here because they
// are the applicant, which is precisely why roleMW could never be this module's
// rule — it names no student role at all.
func nonAdminViewers() []Viewer {
	return []Viewer{
		institutionViewer(),
		providerViewer(),
		subuserViewer(),
		applicantViewer(7),
		{UserID: 0, Role: "student"},
		{UserID: 0, Role: ""},
	}
}

// TestJobCrudIsAdminOnlyAtTheService is the core of the fix: the service is the
// authority, and every non-admin role is refused by every catalogue method.
//
// The "and nothing was written" assertions matter as much as the errors. A guard
// that returned ErrJobForbidden after the mutation had already run would pass
// every error check here and still be a working delete, so each case reloads the
// rows it was aimed at.
func TestJobCrudIsAdminOnlyAtTheService(t *testing.T) {
	for _, viewer := range nonAdminViewers() {
		svc, job, _, _ := jobCrudSeed(t)

		created, err := svc.CreateJob(viewer, CreateJobRequest{
			Title: "Sneaky Posting", Department: "Engineering", Description: "x",
			JobType: "full-time", Status: "published",
		})
		if !errors.Is(err, ErrJobForbidden) {
			t.Fatalf("role %q created a job (err=%v)", viewer.Role, err)
		}
		if created != nil {
			t.Fatalf("role %q got a job back from create: %+v", viewer.Role, created)
		}

		listed, err := svc.ListAllJobs(viewer, "", "", 1, 10)
		if !errors.Is(err, ErrJobForbidden) {
			t.Fatalf("role %q listed the catalogue (err=%v)", viewer.Role, err)
		}
		if listed != nil {
			t.Fatalf("role %q got a listing: %+v", viewer.Role, listed)
		}

		got, err := svc.GetJobByID(viewer, job.ID)
		if !errors.Is(err, ErrJobForbidden) {
			t.Fatalf("role %q read job %d (err=%v)", viewer.Role, job.ID, err)
		}
		if got != nil {
			t.Fatalf("role %q got job %d: %+v", viewer.Role, job.ID, got)
		}

		title := "Rewritten by a tenant"
		updated, err := svc.UpdateJob(viewer, job.ID, UpdateJobRequest{Title: &title})
		if !errors.Is(err, ErrJobForbidden) {
			t.Fatalf("role %q updated job %d (err=%v)", viewer.Role, job.ID, err)
		}
		if updated != nil {
			t.Fatalf("role %q got an updated job: %+v", viewer.Role, updated)
		}

		if err := svc.DeleteJob(viewer, job.ID); !errors.Is(err, ErrJobForbidden) {
			t.Fatalf("role %q deleted job %d (err=%v)", viewer.Role, job.ID, err)
		}

		// A refusal that still mutated would be worse than no check at all.
		var reloaded Job
		if err := svc.repo.db.First(&reloaded, job.ID).Error; err != nil {
			t.Fatalf("role %q: job %d is gone: %v", viewer.Role, job.ID, err)
		}
		if reloaded.Title != job.Title {
			t.Fatalf("role %q changed the posting: %q", viewer.Role, reloaded.Title)
		}
		assertLiveApplications(t, svc, job.ID, 3, viewer.Role)
	}
}

// TestPlatformAdminsCanStillManageJobs is the anti-lockout test, written
// explicitly because a fix that only denies would pass every other test in this
// file while breaking the product. The superadmin dashboard is the only
// legitimate consumer of the catalogue routes, so "admins still work" is a
// requirement rather than a nicety.
//
// Every admin spelling the codebase uses is exercised, including the casing and
// padding variants, because RequireRole and IsPlatformAdmin both normalise and the
// gate and the service must agree on that.
func TestPlatformAdminsCanStillManageJobs(t *testing.T) {
	for _, role := range []string{"admin", "superadmin", "super_admin", "SuperAdmin", " admin "} {
		viewer := Viewer{UserID: 1, Role: role}
		svc, job, _, deleted := jobCrudSeed(t)

		created, err := svc.CreateJob(viewer, CreateJobRequest{
			Title: "New Posting", Department: "Engineering", Description: "Build things",
			JobType: "full-time", Status: "published",
		})
		if err != nil {
			t.Fatalf("role %q create: %v", role, err)
		}
		if created.ID == 0 || created.Title != "New Posting" {
			t.Fatalf("role %q create: got %+v", role, created)
		}

		listed, err := svc.ListAllJobs(viewer, "", "", 1, 10)
		if err != nil {
			t.Fatalf("role %q list: %v", role, err)
		}
		if listed.Total != 2 {
			t.Fatalf("role %q list: got %d jobs, want the seeded one plus the new one", role, listed.Total)
		}

		if _, err := svc.GetJobByID(viewer, job.ID); err != nil {
			t.Fatalf("role %q read: %v", role, err)
		}

		title := "Senior Backend Engineer"
		updated, err := svc.UpdateJob(viewer, job.ID, UpdateJobRequest{Title: &title})
		if err != nil {
			t.Fatalf("role %q update: %v", role, err)
		}
		if updated.Title != title {
			t.Fatalf("role %q update: got %q", role, updated.Title)
		}

		if err := svc.DeleteJob(viewer, job.ID); err != nil {
			t.Fatalf("role %q delete: %v", role, err)
		}

		// The delete really happened: the posting and its applications are gone.
		// A service that refused everything would pass every check above and fail
		// here.
		var reloaded Job
		if err := svc.repo.db.First(&reloaded, job.ID).Error; err == nil {
			t.Fatalf("role %q delete: job %d still present", role, job.ID)
		}
		assertLiveApplications(t, svc, job.ID, 0, role)
		assertDocumentsRemoved(t, svc, deleted, job.ID, []string{
			"resumes/mine.pdf", "resumes/mine_cl.pdf", "resumes/a.pdf", "resumes/b.pdf",
		})
	}
}

// TestDeleteJobCascadesOverApplicationsAndTheirFiles is the behaviour the delete
// route exists for, asserted in full because the guard is only worth anything if
// the thing it guards still works for an admin — and because a cascade that
// quietly stopped deleting would be its own data-retention bug.
//
// The rows are soft-deleted (JobApplication carries a DeletedAt), so the assertion
// is that they are no longer visible to an ordinary query, not that the bytes are
// gone. The objects are a hard delete in the object store, asserted on exact paths.
func TestDeleteJobCascadesOverApplicationsAndTheirFiles(t *testing.T) {
	svc, job, apps, deleted := jobCrudSeed(t)

	if err := svc.DeleteJob(adminViewer(), job.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}

	// The posting is gone from the admin read and from the public one.
	if _, err := svc.GetJobByID(adminViewer(), job.ID); err == nil {
		t.Fatalf("deleted job %d is still readable by an admin", job.ID)
	}
	if _, err := svc.GetPublishedJobByID(job.ID); err == nil {
		t.Fatalf("deleted job %d is still served by the public route", job.ID)
	}

	assertLiveApplications(t, svc, job.ID, 0, "admin")
	assertDocumentsRemoved(t, svc, deleted, job.ID, []string{
		"resumes/mine.pdf", "resumes/mine_cl.pdf", "resumes/a.pdf", "resumes/b.pdf",
	})

	// Cross-checked against the seeded rows rather than trusting the recorder
	// alone: every stored document on every application must have been removed,
	// which is 3 resumes and 1 cover letter for the seed.
	wantCount := 0
	for _, app := range apps {
		if app.ResumeURL != "" {
			wantCount++
		}
		if app.CoverLetterURL != "" {
			wantCount++
		}
	}
	if got := len(*deleted); got != wantCount {
		t.Fatalf("removed %d documents, want %d (one per stored file on %d applications): %v",
			got, wantCount, len(apps), *deleted)
	}
}

// TestOnlyAnAdminCanCauseTheCascade is the destructive half of the fix stated on
// its own: whatever else changes about who may manage the catalogue, an ordinary
// tenant must not be able to destroy another person's documents with one request.
//
// The recorder is asserted empty, not merely "the error is right". A guard placed
// after the file loop would return the right error and destroy everything, and
// would still pass every other test in this file.
func TestOnlyAnAdminCanCauseTheCascade(t *testing.T) {
	for _, viewer := range nonAdminViewers() {
		svc, job, _, deleted := jobCrudSeed(t)

		if err := svc.DeleteJob(viewer, job.ID); !errors.Is(err, ErrJobForbidden) {
			t.Fatalf("role %q deleted job %d (err=%v)", viewer.Role, job.ID, err)
		}
		if len(*deleted) != 0 {
			t.Fatalf("role %q destroyed documents: %v", viewer.Role, *deleted)
		}

		// Still there: the posting and its applications.
		if _, err := svc.repo.FindJobByID(job.ID); err != nil {
			t.Fatalf("role %q removed the posting: %v", viewer.Role, err)
		}
		assertLiveApplications(t, svc, job.ID, 3, viewer.Role)
	}
}

// TestJobCrudRoutesRefuseNonAdminsAtTheEdge covers the route half with the
// production shape: authMW plus a gate built from PlatformAdminRoles, exactly as
// cmd/server/main.go now does.
//
// Each of the five routes is exercised per role, because a guard applied to four
// of them is a hole in the fifth.
func TestJobCrudRoutesRefuseNonAdminsAtTheEdge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, role := range []string{"institution", "scholarship_provider", "scholarship_provider_subuser", "student"} {
		svc, job, _, deleted := jobCrudSeed(t)

		r := gin.New()
		RegisterRoutes(r, viewerMW(role, 40), middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

		if w := doRequest(r, http.MethodGet, "/api/v1/superadmin/jobs", ""); w.Code != http.StatusForbidden {
			t.Fatalf("role %q listed the catalogue: %d %q", role, w.Code, w.Body.String())
		}
		if w := doJSON(r, http.MethodPost, "/api/v1/superadmin/jobs", createJobBody); w.Code != http.StatusForbidden {
			t.Fatalf("role %q created a job: %d %q", role, w.Code, w.Body.String())
		}
		if w := doJSON(r, http.MethodPut, "/api/v1/superadmin/jobs/"+itoa(job.ID), updateJobBody); w.Code != http.StatusForbidden {
			t.Fatalf("role %q updated a job: %d %q", role, w.Code, w.Body.String())
		}
		if w := doRequest(r, http.MethodGet, "/api/v1/superadmin/jobs/"+itoa(job.ID), ""); w.Code != http.StatusForbidden {
			t.Fatalf("role %q read a job: %d %q", role, w.Code, w.Body.String())
		}
		if w := doRequest(r, http.MethodDelete, "/api/v1/superadmin/jobs/"+itoa(job.ID), ""); w.Code != http.StatusForbidden {
			t.Fatalf("role %q deleted a job: %d %q", role, w.Code, w.Body.String())
		}

		// The middleware stopped these before the handler, so the service never
		// ran: nothing created, nothing destroyed.
		var jobs int64
		svc.repo.db.Model(&Job{}).Count(&jobs)
		if jobs != 1 {
			t.Fatalf("role %q reached the service: %d jobs exist, want the seeded one", role, jobs)
		}
		assertLiveApplications(t, svc, job.ID, 3, role)
		if len(*deleted) != 0 {
			t.Fatalf("role %q destroyed documents: %v", role, *deleted)
		}
	}
}

// TestJobCrudGateCannotBeWidenedByTheCaller is the defence-in-depth test, and the
// reason the service checks are not redundant with the middleware.
//
// It mounts the production shape but hands RegisterRoutes the ACTUAL shared roleMW
// list from cmd/server/main.go — the one that admitted every tenant. The
// middleware now lets an institution through, exactly as it always did, and the
// request must still be refused. If this test fails, the module is relying on its
// caller to pass the right list, which is the fragility that let the original hole
// exist in the first place.
func TestJobCrudGateCannotBeWidenedByTheCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, job, _, deleted := jobCrudSeed(t)

	r := gin.New()
	RegisterRoutes(r, viewerMW("institution", 40), middleware.RequireRole(sharedRoleMWList...), NewHandler(svc))

	w := doRequest(r, http.MethodDelete, "/api/v1/superadmin/jobs/"+itoa(job.ID), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("shared roleMW let the institution reach the delete: %d %q", w.Code, w.Body.String())
	}

	// The refusal came from the service and not from the middleware, which is
	// what distinguishes "the gate is narrow" from "the gate was ignored".
	// RequireRole's body names the role; the service's does not.
	body := w.Body.String()
	if !strings.Contains(body, ErrJobForbidden.Error()) {
		t.Fatalf("want the service's %q, got %q", ErrJobForbidden, body)
	}
	if strings.Contains(body, "Role:") {
		t.Fatalf("refusal came from the middleware, not the service: %q", body)
	}
	if len(*deleted) != 0 {
		t.Fatalf("documents destroyed with a widened gate: %v", *deleted)
	}
	if _, err := svc.repo.FindJobByID(job.ID); err != nil {
		t.Fatalf("job %d removed with a widened gate: %v", job.ID, err)
	}
}

// TestAdminJobRoutesStillWork is the anti-lockout test at the edge, complementing
// TestPlatformAdminsCanStillManageJobs at the service. Were the middleware list
// narrower than PlatformAdminRoles, the service tests would keep passing while the
// dashboard got a 403 from the gate — a fix that ships broken.
func TestAdminJobRoutesStillWork(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, job, _, _ := jobCrudSeed(t)

	r := gin.New()
	RegisterRoutes(r, viewerMW("superadmin", 1), middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	if w := doJSON(r, http.MethodPost, "/api/v1/superadmin/jobs", createJobBody); w.Code != http.StatusCreated {
		t.Fatalf("admin create: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/superadmin/jobs", ""); w.Code != http.StatusOK {
		t.Fatalf("admin list: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/superadmin/jobs/"+itoa(job.ID), ""); w.Code != http.StatusOK {
		t.Fatalf("admin read: %d %q", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/superadmin/jobs/"+itoa(job.ID), updateJobBody); w.Code != http.StatusOK {
		t.Fatalf("admin update: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodDelete, "/api/v1/superadmin/jobs/"+itoa(job.ID), ""); w.Code != http.StatusOK {
		t.Fatalf("admin delete: %d %q", w.Code, w.Body.String())
	}
}

// TestApplicantRoutesAreUnaffectedByTheJobGuard is the cross-check against a fix
// that over-reaches. The applicant self-read was added in 4097dfc and depends on
// that group being behind authMW ALONE — a gate listing platform admins only
// would refuse the applicant who is legitimately reading their own application,
// which is exactly the lockout the previous commit warned about.
//
// It lives here rather than in applicant_access_test.go because what it protects
// is the interaction between the two halves: if a future edit moves the job guard
// onto the applicants group, this fails.
func TestApplicantRoutesAreUnaffectedByTheJobGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc, _, apps, _ := jobCrudSeed(t)

	r := gin.New()
	// The job gate is passed in and must be ignored by the applicants group.
	RegisterRoutes(r, viewerMW("student", 7), middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	w := doRequest(r, http.MethodGet, "/api/v1/superadmin/jobs/applicants/"+itoa(apps[0].ID)+"/resume", "")
	if w.Code == http.StatusForbidden {
		t.Fatalf("applicant 7 was refused their own application by the job gate: %q", w.Body.String())
	}
	// The same gate refuses them on the job catalogue, so the two groups really do
	// differ and this is not passing because the guard is inert.
	if w := doRequest(r, http.MethodDelete, "/api/v1/superadmin/jobs/"+itoa(apps[0].JobID), ""); w.Code != http.StatusForbidden {
		t.Fatalf("applicant 7 deleted a job: %d %q", w.Code, w.Body.String())
	}
}

// TestPublicCareersSurfaceIsUnchanged is the read-scoping check the brief asked
// for: that nothing here moved the PUBLIC routes.
//
// The careers page is first-party and public by design — ListPublishedJobs and
// GetPublishedJobByID take no Viewer at all — and neither was ever behind roleMW,
// so this fix does not touch them. The test pins that they still answer for a
// caller with no role, because "job CRUD is admin only" read too broadly would
// mean locking the careers page to admins. It also pins that they expose only
// published postings, which is what keeps the admin-only listing genuinely
// narrower than the public one rather than a duplicate of it.
func TestPublicCareersSurfaceIsUnchanged(t *testing.T) {
	svc, job, _, _ := jobCrudSeed(t)

	draft := Job{Title: "Unannounced Role", Department: "Engineering", Description: "x", JobType: "full-time", Status: "draft"}
	if err := svc.repo.db.Create(&draft).Error; err != nil {
		t.Fatalf("seed draft: %v", err)
	}

	listed := svc.ListPublishedJobs("", "", 1, 10)
	if listed.Total != 1 || listed.Jobs[0].ID != job.ID {
		t.Fatalf("public listing changed: total=%d jobs=%+v", listed.Total, listed.Jobs)
	}
	if _, err := svc.GetPublishedJobByID(job.ID); err != nil {
		t.Fatalf("public read of a published job: %v", err)
	}
	if _, err := svc.GetPublishedJobByID(draft.ID); err == nil {
		t.Fatal("a draft is served by the public route")
	}

	// The admin-only listing is a superset: it is the only way to see the draft.
	adminListing, err := svc.ListAllJobs(adminViewer(), "", "", 1, 10)
	if err != nil {
		t.Fatalf("admin listing: %v", err)
	}
	if adminListing.Total != 2 {
		t.Fatalf("admin listing: got %d jobs, want the published one and the draft", adminListing.Total)
	}

	// And the public routes are still mounted without a role gate at all.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, viewerMW("", 0), middleware.RequireRole(PlatformAdminRoles()...), NewHandler(svc))

	if w := doRequest(r, http.MethodGet, "/api/v1/careers", ""); w.Code != http.StatusOK {
		t.Fatalf("public careers listing: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/careers/"+itoa(job.ID), ""); w.Code != http.StatusOK {
		t.Fatalf("public careers read: %d %q", w.Code, w.Body.String())
	}
	if w := doRequest(r, http.MethodGet, "/api/v1/careers/"+itoa(draft.ID), ""); w.Code != http.StatusNotFound {
		t.Fatalf("public careers read of a draft: %d %q", w.Code, w.Body.String())
	}
}

// TestPlatformAdminRolesCoversEveryAdminSpelling pins the list itself, since
// cmd/server/main.go builds the gate from it. Adding an admin role here widens the
// catalogue for everyone at once; removing one locks the dashboard out. Both are
// caught by this test rather than by whichever test happens to use that spelling.
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

// viewerMW stands in for middleware.Auth(): it puts a user_id and a role on the
// context, which is the contract authMW establishes and the only part of it the
// guard reads.
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

// assertLiveApplications counts the applications to a job that an ordinary query
// can still see. JobApplication is soft-deleted, so this is how "gone" is asserted
// without reaching for Unscoped.
func assertLiveApplications(t *testing.T, svc *Service, jobID uint, want int64, who string) {
	t.Helper()

	var live int64
	svc.repo.db.Model(&JobApplication{}).Where("job_id = ?", jobID).Count(&live)
	if live != want {
		t.Fatalf("%s: %d live applications to job %d, want %d", who, live, jobID, want)
	}
}

// assertDocumentsRemoved checks the cascade against the exact object paths it is
// supposed to destroy — not just the count, so a cascade that deleted the right
// number of the wrong files, or another job's files, fails here.
func assertDocumentsRemoved(t *testing.T, svc *Service, deleted *[]string, jobID uint, want []string) {
	t.Helper()

	got := map[string]int{}
	for _, objectPath := range *deleted {
		got[objectPath]++
	}
	for _, objectPath := range want {
		switch got[objectPath] {
		case 1:
		case 0:
			t.Fatalf("job %d: document %q survived the cascade; removed %v", jobID, objectPath, *deleted)
		default:
			t.Fatalf("job %d: document %q removed %d times", jobID, objectPath, got[objectPath])
		}
		delete(got, objectPath)
	}
	for objectPath, n := range got {
		t.Fatalf("job %d: deleted %q x%d, which is not one of this job's documents", jobID, objectPath, n)
	}
}
