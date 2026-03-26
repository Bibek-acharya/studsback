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
		}

		scholarshipProviderAuth := v1.Group("/scholarship-providers/auth")
		{
			scholarshipProviderAuth.POST("/register", handlers.ScholarshipProviderRegister)
			scholarshipProviderAuth.POST("/login", handlers.ScholarshipProviderLogin)
		}

		// Public college routes (no authentication required)
		colleges := v1.Group("/colleges")
		{
			colleges.GET("", handlers.GetColleges)
			colleges.GET("/:id", handlers.GetCollegeByID)
		}

		universities := v1.Group("/universities")
		{
			universities.GET("", handlers.GetUniversities)
			universities.GET("/:id", handlers.GetUniversityByID)
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
			education.GET("/events", handlers.GetEducationEvents)
		}

		tools := v1.Group("/tools")
		{
			tools.POST("/scholarship-finder/recommendations", handlers.GetScholarshipFinderRecommendations)
			tools.POST("/college-recommender/recommendations", handlers.GetCollegeRecommenderRecommendations)
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

		// Protected routes (authentication required)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/profile", handlers.GetProfile)
			protected.POST("/preferences", handlers.SavePreferences)
			protected.POST("/counselling/bookings", handlers.CreateCounsellingBooking)
			protected.GET("/counselling/bookings/my", handlers.GetMyCounsellingBookings)

			// Admin only routes
			admin := protected.Group("/admin")
			admin.Use(middleware.RoleMiddleware("admin"))
			{
				// Admin college management
				admin.POST("/colleges", handlers.CreateCollege)
				admin.PUT("/colleges/:id", handlers.UpdateCollege)
				admin.DELETE("/colleges/:id", handlers.DeleteCollege)

				// Other admin routes
				admin.GET("/users", func(c *gin.Context) {
					c.JSON(200, gin.H{"message": "Admin users endpoint"})
				})
			}
		}
	}
}
