// @title StudSphere API
// @version 1.0
// @description API for the StudSphere education platform - connecting students with universities, colleges, scholarships, and community forums.
// @host localhost:8080
// @BasePath /
// @schemes http https
package main

import (
	"fmt"
	"log"
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/routes"
	"studsphere/backend/seeds"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	config.LoadConfig()

	// Set Gin mode
	gin.SetMode(config.AppConfig.GinMode)

	// Connect to database
	config.ConnectDatabase()

	// Auto migrate models
	if err := config.GetDB().AutoMigrate(
		&models.User{},
		&models.InstitutionUser{},
		&models.University{},
		&models.College{},
		&models.CounsellingBooking{},
		&models.Scholarship{},
		&models.ScholarshipApplication{},
		&models.Exam{},
		&models.Course{},
		&models.CollegeUniversityCourse{},
		&models.News{},
		&models.Event{},
		&models.ForumPost{},
		&models.ForumCommunity{},
		&models.ForumCommunityMember{},
		&models.ForumComment{},
		&models.ForumVote{},
		&models.ForumSave{},
		&models.ForumPollVote{},
		&models.ScholarshipProviderUser{},
		&models.Admission{},
		&models.Blog{},
		&models.ProviderScholarship{},
		&models.ProviderApplication{},
		&models.ProviderInterview{},
		&models.ProviderMessage{},
		&models.ProviderSettings{},
		&models.Message{},
		&models.CalendarEvent{},
		&models.SphereInvite{},
		&models.Bookmark{},
		&models.Notification{},
		&models.InstitutionProgram{},
		&models.InstitutionMedia{},
		&models.InstitutionCounsellingSession{},
		&models.InstitutionCounsellingBooking{},
		&models.InstitutionEntrance{},
		&models.InstitutionEntranceApplicant{},
		&models.InstitutionEvent{},
		&models.InstitutionNews{},
		&models.InstitutionQMS{},
		&models.InstitutionMessage{},
		&models.InstitutionSettings{},
		&models.ContactInquiry{},
		&models.Ad{},
		&models.CarouselSlide{},
	); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	log.Println("Database migration completed")

	// Seed database
	if err := seeds.Seed(); err != nil {
		log.Printf("Warning: Failed to seed database: %v", err)
	}

	// Create Gin router with custom logger
	router := gin.New()
	router.Use(gin.Recovery())

	// Highly readable custom logger middleware
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var statusColor = param.StatusCodeColor()
		var methodColor = param.MethodColor()
		var reset = param.ResetColor()

		// Icon mapping for methods to make them easier to scan
		icon := "❔"
		switch param.Method {
		case "GET":
			icon = "🔍"
		case "POST":
			icon = "➕"
		case "PUT":
			icon = "📝"
		case "DELETE":
			icon = "🗑️"
		case "PATCH":
			icon = "🔧"
		case "OPTIONS":
			icon = "⚙️"
		}

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		return fmt.Sprintf("[API] %v |%s %3d %s| %13v | %15s | %s %s%-7s %s %s\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			statusColor, param.StatusCode, reset,
			param.Latency,
			param.ClientIP,
			icon,
			methodColor, param.Method, reset,
			param.Path,
			param.ErrorMessage,
		)
	}))

	// Setup CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Serve uploaded media files
	router.Static("/uploads", "./uploads")

	// Swagger documentation - serve OpenAPI spec and Swagger UI
	router.Static("/docs", "./docs")
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(302, "/docs/index.html")
	})

	// Setup routes
	routes.SetupRoutes(router)

	// Start server
	log.Printf("Server starting on port %s...", config.AppConfig.Port)
	if err := router.Run(":" + config.AppConfig.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
