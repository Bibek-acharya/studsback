package routes

import (
	"studsphere/backend/handlers"
	"studsphere/backend/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all application routes
func SetupRoutes(router *gin.Engine) {
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Server is running",
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public routes (no authentication required)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
			auth.POST("/send-otp", handlers.SendOTP)
			auth.POST("/verify-otp", handlers.VerifyOTP)
			auth.GET("/google", handlers.GoogleLogin)
			auth.GET("/google/callback", handlers.GoogleCallback)
		}

		institutionAuth := v1.Group("/institutions/auth")
		{
			institutionAuth.POST("/register", handlers.InstitutionRegister)
			institutionAuth.POST("/login", handlers.InstitutionLogin)
			institutionAuth.GET("/google", handlers.InstitutionGoogleLogin)
			institutionAuth.GET("/google/callback", handlers.InstitutionGoogleCallback)
		}

		scholarshipProviderAuth := v1.Group("/scholarship-providers/auth")
		{
			scholarshipProviderAuth.POST("/register", handlers.ScholarshipProviderRegister)
			scholarshipProviderAuth.POST("/login", handlers.ScholarshipProviderLogin)
			scholarshipProviderAuth.GET("/google", handlers.ScholarshipProviderGoogleLogin)
			scholarshipProviderAuth.GET("/google/callback", handlers.ScholarshipProviderGoogleCallback)
		}

		// Public college routes (no authentication required)
		colleges := v1.Group("/colleges")
		{
			colleges.GET("", handlers.GetColleges)
			colleges.GET("/featured", handlers.GetFeaturedColleges)
			colleges.GET("/:id", handlers.GetCollegeByID)
		}

		universities := v1.Group("/universities")
		{
			universities.GET("", handlers.GetUniversities)
			universities.GET("/:id", handlers.GetUniversityByID)
			universities.GET("/:id/:tab", handlers.GetUniversityTab)
		}

		education := v1.Group("/education")
		{
			education.GET("/rankings", handlers.GetEducationRankings)
			education.GET("/exams", handlers.GetEducationExams)
			education.GET("/exams/:id", handlers.GetEducationExamByID)
			education.GET("/scholarships", handlers.GetScholarships)
			education.GET("/scholarships/:id", handlers.GetScholarshipByID)
			education.GET("/scholarships/:id/similar", handlers.GetSimilarScholarships)
			education.POST("/scholarships/:id/apply", middleware.AuthMiddleware(), handlers.ApplyScholarship)
			education.GET("/courses", handlers.GetEducationCourses)
			education.GET("/courses/:id", handlers.GetEducationCourseByID)
			education.GET("/courses/:id/details", handlers.GetEducationCourseDetailsByID)
			education.GET("/admissions", handlers.GetEducationAdmissions)
			education.GET("/news", handlers.GetEducationNews)
			education.GET("/news/:id", handlers.GetEducationNewsByID)
			education.GET("/events", handlers.GetEducationEvents)
			education.GET("/events/:id", handlers.GetEducationEventByID)
			education.GET("/blogs", handlers.GetEducationBlogs)
			education.GET("/blogs/:id", handlers.GetEducationBlogByID)
		}

		tools := v1.Group("/tools")
		{
			tools.POST("/scholarship-finder/recommendations", handlers.GetScholarshipFinderRecommendations)
			tools.POST("/college-recommender/recommendations", handlers.GetCollegeRecommenderRecommendations)
		}

		// Public system routes
		system := v1.Group("/system")
		{
			system.POST("/contact", handlers.SubmitContactInquiry)
			system.GET("/ads", handlers.GetActiveAds)
			system.GET("/carousels", handlers.GetCarousels)
		}

		// Forum routes
		forum := v1.Group("/forum")
		{
			forum.GET("/posts", handlers.GetForumPosts)
			forum.GET("/posts/:id/comments", handlers.GetForumPostComments)
			forum.GET("/communities", handlers.GetForumCommunities)

			// Protected forum interactions
			protectedForum := forum.Group("")
			protectedForum.Use(middleware.AuthMiddleware())
			{
				protectedForum.POST("/posts", handlers.CreateForumPost)
				protectedForum.POST("/posts/:id/like", handlers.LikeForumPost)
				protectedForum.POST("/posts/:id/dislike", handlers.DislikeForumPost)
				protectedForum.POST("/posts/:id/save", handlers.SaveForumPost)
				protectedForum.PUT("/posts/:id", handlers.UpdateForumPost)
				protectedForum.DELETE("/posts/:id", handlers.DeleteForumPost)
				protectedForum.POST("/posts/:id/comments", handlers.CreateForumComment)
				protectedForum.POST("/posts/:id/poll/vote", handlers.VoteForumPoll)
				// File upload
				protectedForum.POST("/upload", handlers.UploadForumMedia)
				// Community membership (toggle join/leave)
				protectedForum.POST("/communities/:id/join", handlers.JoinForumCommunity)
			}
		}

		// Scholarship provider routes (scholarship_provider role required)
		scholarshipProvider := v1.Group("/scholarship-providers")
		scholarshipProvider.Use(middleware.AuthMiddleware())
		scholarshipProvider.Use(middleware.RoleMiddleware("scholarship_provider"))
		{
			scholarshipProvider.GET("/dashboard", handlers.GetScholarshipProviderDashboard)
			scholarshipProvider.GET("/analytics", handlers.GetScholarshipProviderAnalytics)

			scholarshipProvider.POST("/scholarships", handlers.CreateProviderScholarship)
			scholarshipProvider.GET("/scholarships", handlers.GetProviderScholarships)
			scholarshipProvider.GET("/scholarships/:id", handlers.GetProviderScholarshipByID)
			scholarshipProvider.PUT("/scholarships/:id", handlers.UpdateProviderScholarship)
			scholarshipProvider.DELETE("/scholarships/:id", handlers.DeleteProviderScholarship)

			scholarshipProvider.GET("/applications", handlers.GetProviderApplications)
			scholarshipProvider.GET("/applications/:id", handlers.GetProviderApplicationByID)
			scholarshipProvider.PUT("/applications/:id/evaluate", handlers.EvaluateProviderApplication)
			scholarshipProvider.PUT("/applications/:id/status", handlers.UpdateProviderApplicationStatus)

			scholarshipProvider.GET("/interviews", handlers.GetProviderInterviews)
			scholarshipProvider.POST("/interviews", handlers.CreateProviderInterview)
			scholarshipProvider.PUT("/interviews/:id", handlers.UpdateProviderInterview)

			scholarshipProvider.GET("/messages", handlers.GetProviderMessages)
			scholarshipProvider.POST("/messages", handlers.CreateProviderMessage)
			scholarshipProvider.GET("/messages/:id", handlers.GetProviderMessageByID)

			scholarshipProvider.GET("/profile", handlers.GetProviderProfile)
			scholarshipProvider.PUT("/profile", handlers.UpdateProviderProfile)
			scholarshipProvider.GET("/settings", handlers.GetProviderSettings)
			scholarshipProvider.PUT("/settings", handlers.UpdateProviderSettings)
		}

		// Protected routes (authentication required)
		// Grouping manually to avoid nested group complexity that might affect routing performance or consistency
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/profile", handlers.GetProfile)
			protected.PUT("/profile", handlers.UpdateProfile)
			protected.POST("/preferences", handlers.SavePreferences)
			protected.POST("/counselling/bookings", handlers.CreateCounsellingBooking)
			protected.GET("/counselling/bookings/my", handlers.GetMyCounsellingBookings)

			// Admission routes for students
			protected.POST("/admissions", handlers.CreateAdmission)
			protected.GET("/admissions/my", handlers.GetMyAdmissions)
			protected.GET("/admissions/:id", handlers.GetAdmission)
			protected.PUT("/admissions/:id", handlers.UpdateAdmission)
			protected.DELETE("/admissions/:id", handlers.DeleteAdmission)

			// Scholarship application routes for students
			protected.GET("/scholarships/my-applications", handlers.GetMyScholarshipApplications)
			protected.GET("/scholarships/applications/:id", handlers.GetScholarshipApplication)
			protected.PUT("/scholarships/applications/:id", handlers.UpdateScholarshipApplication)
			protected.DELETE("/scholarships/applications/:id", handlers.DeleteScholarshipApplication)

			// Student messaging
			protected.GET("/messages", handlers.GetMessages)
			protected.GET("/messages/:id", handlers.GetMessageByID)
			protected.POST("/messages", handlers.CreateMessage)
			protected.POST("/messages/:id/reply", handlers.ReplyToMessage)
			protected.GET("/messages/contacts", handlers.GetMessageContacts)

			// Student calendar
			protected.GET("/calendar/events", handlers.GetCalendarEvents)
			protected.GET("/calendar/events/:id", handlers.GetCalendarEventByID)
			protected.POST("/calendar/events", handlers.CreateCalendarEvent)
			protected.PUT("/calendar/events/:id", handlers.UpdateCalendarEvent)
			protected.DELETE("/calendar/events/:id", handlers.DeleteCalendarEvent)

			// Student invites
			protected.GET("/invites", handlers.GetInvites)
			protected.GET("/invites/:id", handlers.GetInviteByID)
			protected.PUT("/invites/:id/accept", handlers.AcceptInvite)
			protected.PUT("/invites/:id/decline", handlers.DeclineInvite)
			protected.PUT("/invites/:id/save", handlers.SaveInvite)

			// Student bookmarks
			protected.GET("/bookmarks", handlers.GetBookmarks)
			protected.POST("/bookmarks", handlers.CreateBookmark)
			protected.DELETE("/bookmarks/:id", handlers.DeleteBookmark)
			protected.GET("/bookmarks/:type", handlers.GetBookmarksByType)

			// Student notifications
			protected.GET("/notifications", handlers.GetNotifications)
			protected.PUT("/notifications/:id/read", handlers.MarkNotificationRead)
			protected.PUT("/notifications/read-all", handlers.MarkAllNotificationsRead)
		}

		// Institution routes (institution role required)
		institution := v1.Group("/institution")
		institution.Use(middleware.AuthMiddleware())
		institution.Use(middleware.RoleMiddleware("institution"))
		{
			// Institution dashboard & analytics
			institution.GET("/dashboard", handlers.GetInstitutionDashboard)
			institution.GET("/analytics", handlers.GetInstitutionAnalytics)

			// Institution program management
			institution.GET("/programs", handlers.GetInstitutionPrograms)
			institution.GET("/programs/:id", handlers.GetInstitutionProgramByID)
			institution.POST("/programs", handlers.CreateInstitutionProgram)
			institution.PUT("/programs/:id", handlers.UpdateInstitutionProgram)
			institution.DELETE("/programs/:id", handlers.DeleteInstitutionProgram)

			// Institution college profile
			institution.GET("/profile", handlers.GetInstitutionProfile)
			institution.PUT("/profile", handlers.UpdateInstitutionProfile)
			institution.GET("/media", handlers.GetInstitutionMedia)
			institution.POST("/media", handlers.CreateInstitutionMedia)
			institution.DELETE("/media/:id", handlers.DeleteInstitutionMedia)

			// Institution counselling management
			institution.GET("/counselling/sessions", handlers.GetInstitutionCounsellingSessions)
			institution.GET("/counselling/bookings", handlers.GetInstitutionCounsellingBookings)
			institution.PUT("/counselling/bookings/:id/status", handlers.UpdateInstitutionCounsellingBookingStatus)

			// Institution entrance management
			institution.GET("/entrances", handlers.GetInstitutionEntrances)
			institution.GET("/entrances/:id", handlers.GetInstitutionEntranceByID)
			institution.POST("/entrances", handlers.CreateInstitutionEntrance)
			institution.PUT("/entrances/:id", handlers.UpdateInstitutionEntrance)
			institution.DELETE("/entrances/:id", handlers.DeleteInstitutionEntrance)
			institution.GET("/entrances/:id/applicants", handlers.GetInstitutionEntranceApplicants)

			// Institution events management
			institution.GET("/events", handlers.GetInstitutionEvents)
			institution.GET("/events/:id", handlers.GetInstitutionEventByID)
			institution.POST("/events", handlers.CreateInstitutionEvent)
			institution.PUT("/events/:id", handlers.UpdateInstitutionEvent)
			institution.DELETE("/events/:id", handlers.DeleteInstitutionEvent)

			// Institution news & notices management
			institution.GET("/news", handlers.GetInstitutionNews)
			institution.GET("/news/:id", handlers.GetInstitutionNewsByID)
			institution.POST("/news", handlers.CreateInstitutionNews)
			institution.PUT("/news/:id", handlers.UpdateInstitutionNews)
			institution.DELETE("/news/:id", handlers.DeleteInstitutionNews)

			// Institution QMS management
			institution.GET("/qms", handlers.GetInstitutionQMS)
			institution.GET("/qms/:id", handlers.GetInstitutionQMSByID)
			institution.POST("/qms", handlers.CreateInstitutionQMS)
			institution.PUT("/qms/:id", handlers.UpdateInstitutionQMS)
			institution.DELETE("/qms/:id", handlers.DeleteInstitutionQMS)

			// Institution messaging
			institution.GET("/messages", handlers.GetInstitutionMessages)
			institution.GET("/messages/:id", handlers.GetInstitutionMessageByID)
			institution.POST("/messages", handlers.CreateInstitutionMessage)
			institution.GET("/messages/students", handlers.GetInstitutionMessageStudents)

			// Institution settings
			institution.GET("/settings", handlers.GetInstitutionSettings)
			institution.PUT("/settings", handlers.UpdateInstitutionSettings)
			institution.PUT("/settings/password", handlers.UpdateInstitutionPassword)

			// Institution scholarship management
			institution.GET("/scholarships", handlers.InstitutionGetScholarships)
			institution.POST("/scholarships", handlers.InstitutionCreateScholarship)
			institution.PUT("/scholarships/:id", handlers.InstitutionUpdateScholarship)
			institution.DELETE("/scholarships/:id", handlers.InstitutionDeleteScholarship)

			// Institution admission management
			institution.GET("/admissions", handlers.InstitutionGetAdmissions)
			institution.PUT("/admissions/:id/status", handlers.InstitutionUpdateAdmissionStatus)

			// Institution scholarship application management
			institution.GET("/scholarship-applications", handlers.InstitutionGetScholarshipApplications)
			institution.PUT("/scholarship-applications/:id/status", handlers.InstitutionUpdateScholarshipApplicationStatus)
		}

		// Admin only routes - explicit group under /api/v1/admin
		admin := v1.Group("/admin")
		admin.Use(middleware.AuthMiddleware())
		admin.Use(middleware.RoleMiddleware("admin", "super_admin"))
		{
			// Admin university management
			admin.GET("/universities", handlers.GetUniversities)
			admin.GET("/universities/:id", handlers.GetUniversityByID)
			admin.POST("/universities", handlers.CreateUniversity)
			admin.PUT("/universities/:id", handlers.UpdateUniversity)
			admin.DELETE("/universities/:id", handlers.DeleteUniversity)

			// Admin college management
			admin.GET("/colleges", handlers.GetColleges)
			admin.GET("/colleges/:id", handlers.GetCollegeByID)
			admin.POST("/colleges", handlers.CreateCollege)
			admin.PUT("/colleges/:id", handlers.UpdateCollege)
			admin.DELETE("/colleges/:id", handlers.DeleteCollege)
			admin.PUT("/colleges/:id/approve", handlers.ApproveCollege)
			admin.PUT("/colleges/:id/featured", handlers.ToggleCollegeFeatured)

			// Admin contact inquiries
			admin.GET("/inquiries", handlers.GetContactInquiries)
			admin.GET("/inquiries/:id", handlers.GetContactInquiryByID)
			admin.PUT("/inquiries/:id/status", handlers.UpdateContactInquiryStatus)
			admin.DELETE("/inquiries/:id", handlers.DeleteContactInquiry)

			// Admin ads management
			admin.GET("/ads", handlers.GetAds)
			admin.GET("/ads/:id", handlers.GetAdByID)
			admin.POST("/ads", handlers.CreateAd)
			admin.PUT("/ads/:id", handlers.UpdateAd)
			admin.DELETE("/ads/:id", handlers.DeleteAd)
			admin.POST("/ads/:id/click", handlers.TrackAdClick)

			// Admin carousel management
			admin.GET("/carousels", handlers.GetCarousels)
			admin.GET("/carousels/:id", handlers.GetCarouselSlideByID)
			admin.POST("/carousels", handlers.CreateCarouselSlide)
			admin.PUT("/carousels/:id", handlers.UpdateCarouselSlide)
			admin.DELETE("/carousels/:id", handlers.DeleteCarouselSlide)
			admin.PUT("/carousels/reorder", handlers.ReorderCarouselSlides)

			// Admin admission management
			admin.GET("/admissions", handlers.GetAllAdmissions)
			admin.GET("/admissions/:id", handlers.GetAdmission)
			admin.PUT("/admissions/:id/status", handlers.UpdateAdmissionStatus)
			admin.GET("/admissions/college/:collegeId", handlers.GetCollegeAdmissions)

			// Admin scholarship management
			admin.GET("/scholarships", handlers.GetAllScholarships)
			admin.GET("/scholarships/:id", handlers.GetScholarshipByID)
			admin.POST("/scholarships", handlers.AdminCreateScholarship)
			admin.PUT("/scholarships/:id", handlers.AdminUpdateScholarship)
			admin.DELETE("/scholarships/:id", handlers.AdminDeleteScholarship)

			// Admin scholarship application management
			admin.GET("/scholarship-applications", handlers.GetAllScholarshipApplications)
			admin.GET("/scholarship-applications/:id", handlers.GetScholarshipApplication)
			admin.PUT("/scholarship-applications/:id/status", handlers.AdminUpdateScholarshipApplicationStatus)
			admin.GET("/scholarship-applications/scholarship/:scholarshipId", handlers.GetScholarshipApplicationsByScholarship)

			// Other admin routes
			admin.GET("/users", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Admin users endpoint"})
			})
		}
	}
}
