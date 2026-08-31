package search

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/search", h.Search)
		v1.GET("/search/suggest", h.Suggest)
		v1.GET("/search/vector-status", h.GetVectorStatus)
		v1.POST("/search/reindex", h.Reindex)

		history := v1.Group("/search/history")
		history.Use(authMW)
		{
			history.GET("", h.GetSearchHistory)
			history.POST("", h.SaveSearchHistory)
		}

		admin := v1.Group("/admin/search")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.POST("/reindex", h.Reindex)
		}
	}
}
