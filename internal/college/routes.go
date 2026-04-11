package college

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		colleges := v1.Group("/colleges")
		{
			colleges.GET("", h.GetColleges)
			colleges.GET("/filter-counts", h.GetCollegeFilterCounts)
			colleges.GET("/featured", h.GetFeaturedColleges)
			colleges.GET("/:id", h.GetCollegeByID)
		}

		admin := v1.Group("/admin/colleges")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.GET("", h.GetColleges)
			admin.GET("/:id", h.GetCollegeByID)
			admin.POST("", h.CreateCollege)
			admin.PUT("/:id", h.UpdateCollege)
			admin.DELETE("/:id", h.DeleteCollege)
			admin.PUT("/:id/approve", h.ApproveCollege)
			admin.PUT("/:id/featured", h.ToggleCollegeFeatured)
		}
	}
}
