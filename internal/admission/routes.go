package admission

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.POST("/admissions", h.Create)
			protected.GET("/admissions/my", h.GetMyAdmissions)
			protected.GET("/admissions/:id", h.GetByID)
			protected.PUT("/admissions/:id", h.Update)
			protected.DELETE("/admissions/:id", h.Delete)
		}

		admin := v1.Group("/admin/admissions")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.GET("", h.GetAll)
			admin.GET("/:id", h.GetByID)
			admin.PUT("/:id/status", h.UpdateStatus)
			admin.GET("/college/:collegeId", h.GetByCollegeID)
		}
	}
}
