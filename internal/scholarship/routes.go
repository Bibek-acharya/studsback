package scholarship

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		education := v1.Group("/education")
		{
			education.GET("/scholarships", h.GetScholarships)
			education.GET("/scholarships/:id", h.GetScholarshipByID)
			education.GET("/scholarships/:id/similar", h.GetSimilarScholarships)

			education.POST("/scholarships/:id/apply", h.ApplyScholarship)
		}

		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.GET("/scholarships/my-applications", h.GetMyApplications)
			protected.GET("/scholarships/applications/:id", h.GetApplication)
			protected.PUT("/scholarships/applications/:id", h.UpdateApplication)
			protected.DELETE("/scholarships/applications/:id", h.DeleteApplication)
		}

		admin := v1.Group("/admin")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.GET("/scholarships", h.GetAllApplications)
			admin.GET("/scholarships/:id", h.GetScholarshipByID)
			admin.POST("/scholarships", h.AdminCreateScholarship)
			admin.PUT("/scholarships/:id", h.AdminUpdateScholarship)
			admin.DELETE("/scholarships/:id", h.AdminDeleteScholarship)

			admin.GET("/scholarship-applications", h.GetAllApplications)
			admin.GET("/scholarship-applications/:id", h.GetApplication)
			admin.PUT("/scholarship-applications/:id/status", h.UpdateApplicationStatus)
			admin.GET("/scholarship-applications/scholarship/:scholarshipId", h.GetApplicationsByScholarship)
		}
	}
}
