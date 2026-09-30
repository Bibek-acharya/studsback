package jobs

import (
	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
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

	superadmin := v1.Group("/superadmin/jobs")
	superadmin.Use(authMW)
	superadmin.Use(roleMW)
	{
		superadmin.GET("", h.ListAllJobs)
		superadmin.POST("", h.CreateJob)
		superadmin.GET("/:id", h.GetJob)
		superadmin.PUT("/:id", h.UpdateJob)
		superadmin.DELETE("/:id", h.DeleteJob)
	}

	// GET /:id/applicants is mounted here rather than in the roleMW group above
	// for one reason: consistency of the answer. It returns name, email and
	// phone for every applicant to the posting — the same disclosure as the
	// resume route and a one-call replacement for the walk over ids — so it is
	// service-scoped like them. Left in the roleMW group it answered 403 with
	// roleMW's "Insufficient permissions" while the per-id routes answered 404,
	// which is two different answers for one rule and reintroduces exactly the
	// distinction the 404 choice exists to remove.
	applicantData := v1.Group("/superadmin/jobs")
	applicantData.Use(authMW)
	{
		applicantData.GET("/:id/applicants", h.ListApplications)
	}

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
	{
		applicants.PUT("/:id/status", h.UpdateApplicantStatus)
		applicants.PUT("/:id/notes", h.UpdateApplicantNotes)
		applicants.POST("/:id/email", h.SendApplicantEmail)
		applicants.GET("/:id/resume", h.ServeResume)
		applicants.GET("/:id/cover-letter", h.ServeCoverLetter)
	}
}
