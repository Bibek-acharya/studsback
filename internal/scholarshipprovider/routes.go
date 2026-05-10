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
			public.GET("/blogs", h.GetPublicBlogs)
			public.GET("/blogs/:id", h.GetPublicBlogByID)
			public.GET("/providers/:id", h.GetPublicProviderProfile)
			public.GET("/volunteers", h.GetPublicVolunteers)
			public.GET("/volunteers/:id", h.GetPublicVolunteerByID)
			public.POST("/volunteers/:id/apply", h.ApplyVolunteer)
		}
	}
}

func RegisterMessageRoutes(r *gin.Engine, authMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("/scholarship-providers")
		protected.Use(authMW)
		{
			protected.POST("/messages/from-user", h.SendMessageFromUser)
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
			scholarshipProvider.GET("/analytics/detailed", h.GetDetailedAnalytics)

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
			scholarshipProvider.PUT("/applications/:id/payment", h.ApproveApplicationPayment)

			scholarshipProvider.GET("/interviews", h.GetInterviews)
			scholarshipProvider.POST("/interviews", h.CreateInterview)
			scholarshipProvider.PUT("/interviews/:id", h.UpdateInterview)

			scholarshipProvider.GET("/messages", h.GetMessages)
			scholarshipProvider.POST("/messages", h.CreateMessage)
			scholarshipProvider.GET("/messages/:id", h.GetMessageByID)
			scholarshipProvider.PUT("/messages/:id/read", h.MarkMessageRead)
			scholarshipProvider.GET("/users/:id", h.GetUser)

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

			scholarshipProvider.POST("/services", h.CreateService)
			scholarshipProvider.GET("/services", h.GetServices)
			scholarshipProvider.GET("/services/:id", h.GetServiceByID)
			scholarshipProvider.PUT("/services/:id", h.UpdateService)
			scholarshipProvider.DELETE("/services/:id", h.DeleteService)

			scholarshipProvider.POST("/sectors", h.CreateSector)
			scholarshipProvider.GET("/sectors", h.GetSectors)
			scholarshipProvider.GET("/sectors/:id", h.GetSectorByID)
			scholarshipProvider.PUT("/sectors/:id", h.UpdateSector)
			scholarshipProvider.DELETE("/sectors/:id", h.DeleteSector)

			scholarshipProvider.POST("/projects", h.CreateProject)
			scholarshipProvider.GET("/projects", h.GetProjects)
			scholarshipProvider.GET("/projects/:id", h.GetProjectByID)
			scholarshipProvider.PUT("/projects/:id", h.UpdateProject)
			scholarshipProvider.DELETE("/projects/:id", h.DeleteProject)

			scholarshipProvider.POST("/gallery", h.CreateGalleryImage)
			scholarshipProvider.GET("/gallery", h.GetGalleryImages)
			scholarshipProvider.GET("/gallery/:id", h.GetGalleryImageByID)
			scholarshipProvider.PUT("/gallery/:id", h.UpdateGalleryImage)
			scholarshipProvider.DELETE("/gallery/:id", h.DeleteGalleryImage)

			scholarshipProvider.POST("/reviews", h.CreateReview)
			scholarshipProvider.GET("/reviews", h.GetReviews)
			scholarshipProvider.GET("/reviews/:id", h.GetReviewByID)
			scholarshipProvider.PUT("/reviews/:id", h.UpdateReview)
			scholarshipProvider.DELETE("/reviews/:id", h.DeleteReview)

			scholarshipProvider.POST("/volunteers", h.CreateVolunteer)
			scholarshipProvider.GET("/volunteers", h.GetVolunteers)
			scholarshipProvider.GET("/volunteers/:id", h.GetVolunteerByID)
			scholarshipProvider.PUT("/volunteers/:id", h.UpdateVolunteer)
			scholarshipProvider.DELETE("/volunteers/:id", h.DeleteVolunteer)
			scholarshipProvider.PUT("/volunteers/:id/toggle", h.ToggleVolunteerActive)
			scholarshipProvider.GET("/volunteers/:id/applications", h.GetVolunteerApplications)
			scholarshipProvider.GET("/volunteers/applications", h.GetAllVolunteerApplications)
			scholarshipProvider.PUT("/volunteers/applications/:id/shortlist", h.ShortlistVolunteerApplication)
			scholarshipProvider.PUT("/volunteers/applications/:id/unshortlist", h.UnshortlistVolunteerApplication)
			scholarshipProvider.PUT("/volunteers/applications/:id/reject", h.RejectVolunteerApplication)

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
			scholarshipProvider.POST("/written-exams", h.CreateWrittenExam)
			scholarshipProvider.GET("/written-exams", h.GetWrittenExams)
			scholarshipProvider.GET("/written-exams/:id", h.GetWrittenExamByID)
			scholarshipProvider.PUT("/written-exams/:id", h.UpdateWrittenExam)
			scholarshipProvider.DELETE("/written-exams/:id", h.DeleteWrittenExam)
			scholarshipProvider.POST("/written-exams/:id/results", h.AddWrittenExamResult)
			scholarshipProvider.PUT("/written-exams/:id/results/:resultId", h.UpdateWrittenExamResult)
			scholarshipProvider.DELETE("/written-exams/:id/results/:resultId", h.DeleteWrittenExamResult)

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
