package projectshiksha

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all Project Shiksha routes
func RegisterRoutes(router *gin.Engine, authMW gin.HandlerFunc, roleMW gin.HandlerFunc, h *Handler) {
	// Public routes
	applications := router.Group("/api/v1/project-shiksha/applications")
	{
		applications.POST("", h.CreateApplication)
		applications.GET("/:id", h.GetApplication)
		applications.GET("/roll-number/:roll_number", h.GetApplicationByRollNumber)
		applications.GET("/admit-card/:roll_number", h.GetAdmitCard)
	}

	// Payment routes (public but could be protected)
	payments := router.Group("/api/v1/project-shiksha/payments")
	{
		payments.POST("", h.ProcessPayment)
	}

	// Protected admin routes
	admin := router.Group("/api/v1/admin/project-shiksha")
	admin.Use(authMW)
	admin.Use(roleMW)
	{
		// Application management
		admin.GET("/applications", h.ListApplications)
		admin.PUT("/applications/:id/status", h.UpdateApplicationStatus)
		admin.DELETE("/applications/:id", h.DeleteApplication)
		
		// Payment management
		admin.POST("/payments/verify", h.VerifyPayment)
		
		// Statistics
		admin.GET("/stats", h.GetStats)
	}
}
