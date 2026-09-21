package downloadcenter

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, superadminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/downloads", h.ListItems)
		v1.GET("/downloads/:id/download", h.DownloadItem)

		admin := v1.Group("/superadmin/downloads")
		admin.Use(authMW)
		admin.Use(superadminRoleMW)
		{
			admin.POST("", h.CreateItem)
			admin.GET("", h.AdminListItems)
			admin.GET("/:id", h.GetItem)
			admin.PUT("/:id", h.UpdateItem)
			admin.DELETE("/:id", h.DeleteItem)
		}
	}
}
