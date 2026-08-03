package jobs

import "github.com/gin-gonic/gin"

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
			public.POST("/:id/apply", h.SubmitApplication)
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
		superadmin.GET("/:id/applicants", h.ListApplications)
	}

	applicants := v1.Group("/superadmin/jobs/applicants")
	applicants.Use(authMW)
	applicants.Use(roleMW)
	{
		applicants.PUT("/:id/status", h.UpdateApplicantStatus)
		applicants.PUT("/:id/notes", h.UpdateApplicantNotes)
		applicants.POST("/:id/email", h.SendApplicantEmail)
		applicants.GET("/:id/resume", h.ServeResume)
		applicants.GET("/:id/cover-letter", h.ServeCoverLetter)
	}
}
