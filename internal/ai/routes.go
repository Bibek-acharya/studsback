package ai

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1/ai")
	{
		v1.POST("/chat", h.Chat)
		v1.GET("/models", h.ListModels)
	}
}
