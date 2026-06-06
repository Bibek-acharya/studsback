package university

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	universities := r.Group("/api/v1/universities")
	{
		universities.GET("", h.GetUniversities)
		universities.GET("/:id", h.GetUniversityByID)
		universities.GET("/:id/:tab", h.GetUniversityTab)
	}

	admin := r.Group("/api/v1/admin")
	admin.Use(authMW)
	admin.Use(roleMW)
	{
		admin.GET("/universities", h.GetUniversities)
		admin.GET("/universities/:id", h.AdminGetUniversityByID)
		admin.POST("/universities", h.CreateUniversity)
		admin.PUT("/universities/:id", h.UpdateUniversity)
		admin.DELETE("/universities/:id", h.DeleteUniversity)
	}
}
