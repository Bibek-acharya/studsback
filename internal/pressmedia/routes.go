package pressmedia

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, superadminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/media-press", h.ListItems)
		v1.GET("/media-press/:id", h.GetItem)

		admin := v1.Group("/superadmin/media-press")
		admin.Use(authMW)
		admin.Use(superadminRoleMW)
		{
			admin.POST("", h.CreateItem)
			admin.GET("", h.AdminListItems)
			admin.GET("/:id", h.GetItem)
			admin.PUT("/:id", h.UpdateItem)
			admin.DELETE("/:id", h.DeleteItem)
			admin.POST("/:id/image", h.UploadImage)
		}
	}
}
