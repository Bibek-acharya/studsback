package jobs

import (
	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the careers surface and the job-catalogue admin surface.
//
// adminRoleMW is passed in rather than built here, and it is deliberately NOT the
// shared roleMW from cmd/server/main.go. That gate is
// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution") —
// a multi-tenant role list shared by 18 modules, each of which was wired to it
// for its own reasons. It is far wider than this one, and behind it an ordinary
// paying customer could create, rewrite and delete postings on the platform's own
// /careers page — and deleting a posting cascades through DeleteJob over every
// applicant's resume and cover letter, destroying other people's documents.
//
// So the catalogue gets its own gate, exactly like coin config (coins/routes.go)
// and study resources, built from jobs.PlatformAdminRoles() so the middleware
// here and the service checks in access.go cannot drift. roleMW itself is left
// alone: narrowing a list 18 modules depend on would break all of them.
//
// The gate is not the whole answer. Every catalogue handler passes a Viewer into
// the service, which re-checks the role, so passing a wider gate back in at the
// call site cannot re-open this — the service is the authority and the middleware
// is the cheaper first refusal. TestJobCrudGateCannotBeWidenedByTheCaller pins
// that.
func RegisterRoutes(r *gin.Engine, authMW, adminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		public := v1.Group("/careers")
		{
			public.GET("", h.ListPublishedJobs)
			public.GET("/departments", h.GetDepartments)
			public.GET("/:id", h.GetPublishedJob)
			// OptionalAuth, not auth: the careers form is public and an
			// applicant must still be able to send it signed out. When a token
			// IS present the submission is stamped with that account, which is
			// what later lets the applicant read their own application. Absent
			// a token nothing changes — the submission is a guest's.
			public.POST("/:id/apply", middleware.OptionalAuth(), h.SubmitApplication)
		}
	}

	// Job CRUD: platform admins only, and the guard is the module-local
	// adminRoleMW rather than the shared roleMW. See the note on RegisterRoutes
	// and the reasoning in access.go. DELETE /:id is the one to read twice — it
	// takes every application to the posting and their stored documents with it.
	jobAdmin := v1.Group("/superadmin/jobs")
	jobAdmin.Use(authMW)
	jobAdmin.Use(adminRoleMW)
	{
		jobAdmin.GET("", h.ListAllJobs)
		jobAdmin.POST("", h.CreateJob)
		jobAdmin.GET("/:id", h.GetJob)
		jobAdmin.PUT("/:id", h.UpdateJob)
		jobAdmin.DELETE("/:id", h.DeleteJob)
	}

	// GET /:id/applicants is mounted here rather than in the catalogue group above
	// for one reason: consistency of the answer. It returns name, email and
	// phone for every applicant to the posting — the same disclosure as the
	// resume route and a one-call replacement for the walk over ids — so it is
	// service-scoped like them. Left in the shared roleMW group it answered 403
	// with roleMW's "Insufficient permissions" while the per-id routes answered
	// 404, which is two different answers for one rule and reintroduces exactly
	// the distinction the 404 choice exists to remove.
	applicantData := v1.Group("/superadmin/jobs")
	applicantData.Use(authMW)
	{
		applicantData.GET("/:id/applicants", h.ListApplications)
	}

	// The applicants group is behind authMW alone and takes no role gate at all,
	// deliberately. adminRoleMW is not applied here either: it names admins only,
	// and the applicant reading their own application is entitled on this module
	// without being one. A gate here would answer 403 to the applicant before the
	// service could admit them.
	applicants := v1.Group("/superadmin/jobs/applicants")
	applicants.Use(authMW)
	// roleMW is deliberately NOT applied here, and its absence is the point.
	//
	// It used to be, and roleMW is
	// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution")
	// — a shared list spanning 18 modules, chosen for each module's own needs and
	// far wider than this one. Behind it, every handler below read an applicant by
	// id with no ownership check, so any institution or scholarship_provider
	// account could pull any applicant's resume and cover letter platform-wide by
	// walking sequential ids.
	//
	// roleMW cannot be the guard for this group even once the service checks
	// exist, because it excludes the one principal that is entitled here but is
	// not an administrator: the applicant reading their own application. Keeping
	// it would mean the guard and the real rule disagree — roleMW says
	// institution, the service says nobody — and the looser of the two would
	// decide who reaches the handler.
	//
	// So the guard is authMW alone and the service is the authority: it admits a
	// platform admin or the applicant, and answers 404 to everyone else
	// including every role roleMW used to let in. That is strictly narrower than
	// what roleMW permitted except for self-reads, so nothing roleMW protected
	// is exposed by dropping it.
	//
	// Two different answers in one module is not an inconsistency here, because
	// these are two different rules: this group admits an applicant, the
	// catalogue group admits only operators. Each answers within itself
	// consistently — 404 for "not yours, or not there" on applicant records,
	// 403 for "not an operator" on the catalogue. See ErrJobForbidden in
	// access.go for why the catalogue refusal is a 403.
	{
		applicants.PUT("/:id/status", h.UpdateApplicantStatus)
		applicants.PUT("/:id/notes", h.UpdateApplicantNotes)
		applicants.POST("/:id/email", h.SendApplicantEmail)
		applicants.GET("/:id/resume", h.ServeResume)
		applicants.GET("/:id/cover-letter", h.ServeCoverLetter)
	}
}
