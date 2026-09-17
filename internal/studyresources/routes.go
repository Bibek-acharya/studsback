package studyresources

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, superadminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/study-resources", h.ListResources)
		v1.GET("/study-resources/:id", h.GetResource)
		v1.GET("/study-resources/:id/download", h.DownloadResource)

		admin := v1.Group("/admin/study-resources")
		admin.Use(authMW)
		admin.Use(superadminRoleMW)
		{
			admin.POST("", h.CreateResource)
			admin.GET("", h.AdminListResources)
			admin.PUT("/:id", h.UpdateResource)
			admin.POST("/:id/file", h.ReplaceResourceFile)
			admin.DELETE("/:id", h.DeleteResource)
		}
	}
}
