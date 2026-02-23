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
			education.GET("/scholarships", handlers.GetEducationScholarships)
			education.GET("/courses", handlers.GetEducationCourses)
			education.GET("/courses/:id", handlers.GetEducationCourseByID)
			education.GET("/admissions", handlers.GetEducationAdmissions)
		}

		// Protected routes (authentication required)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/profile", handlers.GetProfile)
			protected.POST("/preferences", handlers.SavePreferences)

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
