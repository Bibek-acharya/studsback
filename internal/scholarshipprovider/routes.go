package scholarshipprovider

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(r *gin.Engine, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		public := v1.Group("/public")
		{
			public.GET("/news", h.GetPublicNews)
			public.GET("/news/:id", h.GetPublicNewsByID)
			public.GET("/events", h.GetPublicEvents)
			public.GET("/events/:id", h.GetPublicEventByID)
		}
	}
}

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/scholarship-providers/auth")
		{
			auth.POST("/access-login", h.LoginAccessUserPublic)
		}

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
			scholarshipProvider.GET("/applications/export", h.ExportApplications)
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
			scholarshipProvider.PUT("/change-password", h.ChangePassword)
			scholarshipProvider.PUT("/change-email", h.ChangeEmail)
			scholarshipProvider.GET("/settings", h.GetSettings)
			scholarshipProvider.PUT("/settings", h.UpdateSettings)

			scholarshipProvider.GET("/notifications", h.GetNotifications)
			scholarshipProvider.PUT("/notifications/:id/read", h.MarkNotificationRead)
			scholarshipProvider.PUT("/notifications/read-all", h.MarkAllNotificationsRead)

			scholarshipProvider.POST("/uploads", h.UploadImage)
			scholarshipProvider.POST("/uploads/document", h.UploadDocument)

			scholarshipProvider.POST("/news", h.CreateNews)
			scholarshipProvider.GET("/news", h.GetNews)
			scholarshipProvider.GET("/news/:id", h.GetNewsByID)
			scholarshipProvider.PUT("/news/:id", h.UpdateNews)
			scholarshipProvider.DELETE("/news/:id", h.DeleteNews)

			scholarshipProvider.POST("/events", h.CreateEvent)
			scholarshipProvider.GET("/events", h.GetEvents)
			scholarshipProvider.GET("/events/:id", h.GetEventByID)
			scholarshipProvider.PUT("/events/:id", h.UpdateEvent)
			scholarshipProvider.DELETE("/events/:id", h.DeleteEvent)

			scholarshipProvider.POST("/blogs", h.CreateBlog)
			scholarshipProvider.GET("/blogs", h.GetBlogs)
			scholarshipProvider.GET("/blogs/:id", h.GetBlogByID)
			scholarshipProvider.PUT("/blogs/:id", h.UpdateBlog)
			scholarshipProvider.DELETE("/blogs/:id", h.DeleteBlog)

			scholarshipProvider.POST("/calendar-events", h.CreateCalendarEvent)
			scholarshipProvider.GET("/calendar-events", h.GetCalendarEvents)
			scholarshipProvider.GET("/calendar-events/:id", h.GetCalendarEventByID)
			scholarshipProvider.PUT("/calendar-events/:id", h.UpdateCalendarEvent)
			scholarshipProvider.DELETE("/calendar-events/:id", h.DeleteCalendarEvent)

			scholarshipProvider.POST("/results", h.CreateResult)
			scholarshipProvider.GET("/results", h.GetResults)
			scholarshipProvider.GET("/results/:id", h.GetResultByID)
			scholarshipProvider.PUT("/results/:id", h.UpdateResult)
			scholarshipProvider.DELETE("/results/:id", h.DeleteResult)

			scholarshipProvider.POST("/access", h.CreateAccess)
			scholarshipProvider.GET("/access", h.GetAccess)
			scholarshipProvider.GET("/access/:id", h.GetAccessByID)
			scholarshipProvider.PUT("/access/:id", h.UpdateAccess)
			scholarshipProvider.DELETE("/access/:id", h.DeleteAccess)

			auth := scholarshipProvider.Group("/auth")
			{
				auth.POST("/access-users", h.CreateAccessUser)
				auth.GET("/access-users", h.GetAccessUsers)
				auth.GET("/access-users/:id", h.GetAccessUser)
				auth.PUT("/access-users/:id", h.UpdateAccessUser)
				auth.DELETE("/access-users/:id", h.DeleteAccessUser)
				auth.PUT("/access-users/:id/permissions", h.UpdatePermissions)
				auth.PUT("/access-users/:id/reset-password", h.ResetAccessUserPassword)
			}
		}
	}
}
