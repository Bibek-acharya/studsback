package scholarshipprovider

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		scholarshipProvider := v1.Group("/scholarship-providers")
		scholarshipProvider.Use(authMW)
		scholarshipProvider.Use(roleMW)
		{
			scholarshipProvider.GET("/dashboard", h.GetDashboard)
			scholarshipProvider.GET("/analytics", h.GetAnalytics)

			scholarshipProvider.POST("/scholarships", h.CreateScholarship)
			scholarshipProvider.GET("/scholarships", h.GetScholarships)
			scholarshipProvider.GET("/scholarships/:id", h.GetScholarshipByID)
			scholarshipProvider.PUT("/scholarships/:id", h.UpdateScholarship)
			scholarshipProvider.DELETE("/scholarships/:id", h.DeleteScholarship)

			scholarshipProvider.GET("/applications", h.GetApplications)
			scholarshipProvider.GET("/applications/:id", h.GetApplicationByID)
			scholarshipProvider.PUT("/applications/:id/evaluate", h.EvaluateApplication)
			scholarshipProvider.PUT("/applications/:id/status", h.UpdateApplicationStatus)

			scholarshipProvider.GET("/interviews", h.GetInterviews)
			scholarshipProvider.POST("/interviews", h.CreateInterview)
			scholarshipProvider.PUT("/interviews/:id", h.UpdateInterview)

			scholarshipProvider.GET("/messages", h.GetMessages)
			scholarshipProvider.POST("/messages", h.CreateMessage)
			scholarshipProvider.GET("/messages/:id", h.GetMessageByID)

			scholarshipProvider.GET("/profile", h.GetProfile)
			scholarshipProvider.PUT("/profile", h.UpdateProfile)
			scholarshipProvider.GET("/settings", h.GetSettings)
			scholarshipProvider.PUT("/settings", h.UpdateSettings)

			scholarshipProvider.GET("/notifications", h.GetNotifications)
			scholarshipProvider.PUT("/notifications/:id/read", h.MarkNotificationRead)
			scholarshipProvider.PUT("/notifications/read-all", h.MarkAllNotificationsRead)
		}
	}
}
