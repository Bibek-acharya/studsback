package follow

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	follow := r.Group("/api/v1/follow")
	follow.Use(authMW)
	{
		follow.POST("/institution/:id", h.Follow)
		follow.DELETE("/institution/:id", h.Unfollow)
		follow.GET("/status/:institutionId", h.Status)
		follow.GET("/list", h.FollowedList)
	}
}
