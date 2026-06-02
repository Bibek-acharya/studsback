package feedback

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		// Public routes — no auth required
		v1.GET("/public/feedback", h.GetPublicFeedbacks)
		v1.POST("/public/testimonials", h.SubmitTestimonial)

		// Submit feedback — any authenticated user
		v1.POST("/feedback", authMW, h.SubmitFeedback)
		v1.GET("/feedback/status", authMW, h.CheckFeedbackStatus)

		// Admin routes — superadmin only
		v1.GET("/feedback", authMW, roleMW, h.ListFeedback)
		v1.DELETE("/feedback/:id", authMW, roleMW, h.DeleteFeedback)
	}
}
