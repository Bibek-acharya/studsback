package faq

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/faq", h.ListCategories)
		v1.GET("/faq/:id", h.GetCategory)

		admin := v1.Group("/admin/faq")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.POST("/categories", h.CreateCategory)
			admin.PUT("/categories/:id", h.UpdateCategory)
			admin.DELETE("/categories/:id", h.DeleteCategory)
			admin.POST("/items", h.CreateItem)
			admin.PUT("/items/:id", h.UpdateItem)
			admin.DELETE("/items/:id", h.DeleteItem)
		}
	}
}
