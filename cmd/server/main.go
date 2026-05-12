package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"studsphere/backend/internal/admission"
	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/college"
	"studsphere/backend/internal/counselling"
	"studsphere/backend/internal/education"
	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/forum"
	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/projectshiksha"
	"studsphere/backend/internal/review"
	"studsphere/backend/internal/scholarship"
	"studsphere/backend/internal/scholarshipprovider"
	"studsphere/backend/internal/search"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"
	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/shared/seeder"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/studentdashboard"
	"studsphere/backend/internal/system"
	"studsphere/backend/internal/tools"
	"studsphere/backend/internal/university"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	config.Load()

	if err := logger.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	gin.SetMode(config.AppConfig.GinMode)

	logger.Info("Initializing MinIO client...")
	if err := storage.Init(); err != nil {
		logger.Warn("MinIO not available, uploads will fail", "error", err)
	} else {
		logger.Info("MinIO client initialized successfully")
	}

	logger.Info("Initializing database connection...")
	config.ConnectDatabase()

	db := config.GetDB()

	logger.Info("Running database migrations...")
	if err := db.AutoMigrate(
		&auth.User{},
		&auth.InstitutionUser{},
		&auth.ScholarshipProviderUser{},
		&auth.EducationEntry{},
		&university.University{},
		&college.College{},
		&counselling.CounsellingBooking{},
		&scholarship.Scholarship{},
		&scholarship.ScholarshipApplication{},
		&scholarship.Payment{},
		&education.Exam{},
		&education.Course{},
		&education.CollegeUniversityCourse{},
		&education.News{},
		&education.Event{},
		&education.Blog{},
		&forum.ForumPost{},
		&forum.ForumCommunity{},
		&forum.ForumCommunityMember{},
		&forum.ForumComment{},
		&forum.ForumVote{},
		&forum.ForumSave{},
		&forum.ForumPollVote{},
		&admission.Admission{},
		&scholarshipprovider.ProviderScholarship{},
		&scholarshipprovider.ProviderApplication{},
		&scholarshipprovider.ProviderInterview{},
		&scholarshipprovider.ProviderMessage{},
		&scholarshipprovider.ProviderSettings{},
		&scholarshipprovider.ProviderNotification{},
		&scholarshipprovider.ProviderNews{},
		&scholarshipprovider.ProviderEvent{},
		&scholarshipprovider.ProviderBlog{},
		&scholarshipprovider.ProviderCalendarEvent{},
		&scholarshipprovider.ProviderResult{},
		&scholarshipprovider.WrittenExam{},
		&scholarshipprovider.WrittenExamResult{},
		&scholarshipprovider.ProviderAccess{},
		&scholarshipprovider.ProviderAccessUser{},
		&scholarshipprovider.ProviderService{},
		&scholarshipprovider.ProviderSector{},
		&scholarshipprovider.ProviderProject{},
		&scholarshipprovider.ProviderGalleryImage{},
		&scholarshipprovider.ProviderReview{},
		&scholarshipprovider.ProviderVolunteer{},
		&scholarshipprovider.VolunteerApplication{},
		&studentdashboard.Message{},
		&studentdashboard.CalendarEvent{},
		&studentdashboard.SphereInvite{},
		&studentdashboard.Bookmark{},
		&studentdashboard.Notification{},
		&institution.InstitutionProgram{},
		&institution.InstitutionMedia{},
		&institution.InstitutionCounsellingSession{},
		&institution.InstitutionCounsellingBooking{},
		&institution.InstitutionEntrance{},
		&institution.InstitutionEntranceApplicant{},
		&institution.InstitutionEvent{},
		&institution.InstitutionNews{},
		&institution.InstitutionQMS{},
		&institution.InstitutionMessage{},
		&institution.InstitutionSettings{},
		&review.Review{},
		&review.ReviewHelpful{},
		&review.ReviewReport{},
		&projectshiksha.ShikshaApplication{},
		&projectshiksha.ShikshaPayment{},
		&system.ContactInquiry{},
		&system.Ad{},
		&system.CarouselSlide{},
		&system.PublicNotification{},
	); err != nil {
		logger.Fatal("Failed to migrate database", "error", err)
	} else {
		if err := allowAnonymousScholarshipApplications(db); err != nil {
			logger.Fatal("Failed to update scholarship application user_id nullability", "error", err)
		}
		if err := fixMissingColumns(db); err != nil {
			logger.Fatal("Failed to fix missing columns", "error", err)
		}
		// Cleanup dangling sub-users with provider_id = 0 from previous bug
		if err := db.Exec("DELETE FROM provider_access_users WHERE provider_id = 0").Error; err != nil {
			logger.Warn("Failed to cleanup dangling sub-users", "error", err)
		}
		logger.Info("Database migrations completed successfully")
	}

	logger.Info("Seeding super admin account...")
	if err := seeder.SeedSuperAdmin(db); err != nil {
		logger.Fatal("Failed to seed super admin account", "error", err)
	}
	logger.Info("Super admin account seeded successfully")

	logger.Info("Initializing email queue...")
	if err := emailqueue.InitAsynq(); err != nil {
		logger.Warn("Failed to initialize email queue (Redis may not be running)", "error", err)
	} else {
		logger.Info("Email queue initialized successfully")

		go func() {
			if err := emailqueue.StartWorker(); err != nil {
				logger.Error("Failed to start email worker", "error", err)
			}
		}()
		logger.Info("Email queue worker started in background")
	}

	logger.Info("Seeding database...")
	if err := seeder.Seed(db); err != nil {
		logger.Warn("Failed to seed database", "error", err)
	} else {
		logger.Info("Database seeding completed")
	}

	logger.Info("Initializing module handlers...")
	admissionHandler := initModule(admission.NewRepository(db), admission.NewService, admission.NewHandler)
	authHandler := initModule(auth.NewRepository(db), auth.NewService, auth.NewHandler)
	collegeHandler := initModule(college.NewRepository(db), college.NewService, college.NewHandler)
	counsellingHandler := initModule(counselling.NewRepository(db), counselling.NewService, counselling.NewHandler)
	educationHandler := initModule(education.NewRepository(db), education.NewService, education.NewHandler)
	forumHandler := initModule(forum.NewRepository(db), forum.NewService, forum.NewHandler)
	institutionHandler := initModule(institution.NewRepository(db), institution.NewService, institution.NewHandler)
	projectShikshaHandler := initModule(projectshiksha.NewRepository(db), projectshiksha.NewService, projectshiksha.NewHandler)
	reviewHandler := initModule(review.NewRepository(db), review.NewService, review.NewHandler)
	scholarshipRepo := scholarship.NewRepository(db)
	scholarshipSvc := scholarship.NewService(scholarshipRepo, db)
	scholarshipHandler := scholarship.NewHandler(scholarshipSvc, scholarship.NewPaymentService(db))
	scholarshipPHandler := initModule(scholarshipprovider.NewRepository(db), scholarshipprovider.NewService, scholarshipprovider.NewHandler)

	auth.SetScholarshipProviderHandler(scholarshipPHandler)
	studentDashHandler := initModule(studentdashboard.NewRepository(db), studentdashboard.NewService, studentdashboard.NewHandler)
	systemHandler := initModule(system.NewRepository(db), system.NewService, system.NewHandler)
	toolsHandler := initModule(tools.NewRepository(db), tools.NewService, tools.NewHandler)
	universityHandler := initModule(university.NewRepository(db), university.NewService, university.NewHandler)
	searchHandler := search.NewHandler(search.NewService(db))
	logger.Info("All module handlers initialized")

	logger.Info("Setting up router...")
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger())
	router.Use(corsMiddleware())

	router.GET("/uploads/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")
		if filepath == "" || filepath == "/" {
			c.Status(http.StatusNotFound)
			return
		}
		filepath = strings.TrimPrefix(filepath, "/")

		reader, info, err := storage.Get(filepath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		ct := info.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}

		filename := filepath
		if idx := strings.LastIndex(filepath, "/"); idx >= 0 {
			filename = filepath[idx+1:]
		}

		if c.Query("dl") == "1" {
			c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		}

		c.DataFromReader(http.StatusOK, -1, ct, reader, nil)
	})
	router.Static("/docs", "./docs")
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(302, "/docs/index.html")
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "Server is running"})
	})

	authMW := middleware.Auth()
	roleMW := middleware.RequireRole("admin", "super_admin", "scholarship_provider", "scholarship-provider", "Scholarship Provider", "scholarship_provider_subuser", "institution")

	admission.RegisterRoutes(router, authMW, roleMW, admissionHandler)
	auth.RegisterRoutes(router, authMW, roleMW, authHandler)
	college.RegisterRoutes(router, authMW, roleMW, collegeHandler)
	counselling.RegisterRoutes(router, authMW, roleMW, counsellingHandler)
	education.RegisterRoutes(router, authMW, roleMW, educationHandler)
	forum.RegisterRoutes(router, authMW, roleMW, forumHandler)
	institution.RegisterRoutes(router, authMW, roleMW, institutionHandler)
	projectshiksha.RegisterRoutes(router, authMW, roleMW, projectShikshaHandler)
	review.RegisterRoutes(router, authMW, roleMW, reviewHandler)
	scholarship.RegisterRoutes(router, authMW, roleMW, scholarshipHandler)
	scholarshipprovider.RegisterRoutes(router, authMW, roleMW, scholarshipPHandler)
	scholarshipprovider.RegisterPublicRoutes(router, scholarshipPHandler)
	scholarshipprovider.RegisterMessageRoutes(router, authMW, scholarshipPHandler)
	studentdashboard.RegisterRoutes(router, authMW, roleMW, studentDashHandler)
	system.RegisterRoutes(router, authMW, roleMW, systemHandler)
	tools.RegisterRoutes(router, authMW, roleMW, toolsHandler)
	university.RegisterRoutes(router, authMW, roleMW, universityHandler)
	search.RegisterRoutes(router, authMW, roleMW, searchHandler)

	logger.Info("All routes registered", "port", config.AppConfig.Port)

	go func() {
		if err := router.Run(":" + config.AppConfig.Port); err != nil {
			logger.Fatal("Failed to start server", "error", err)
		}
	}()

	logger.Info("Server started successfully", "port", config.AppConfig.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	emailqueue.StopWorker()
	emailqueue.CloseAsynq()
	time.Sleep(1 * time.Second)
	logger.Info("Server exited")
}

func initModule[R any, S any, H any](repo R, newService func(R) S, newHandler func(S) H) H {
	return newHandler(newService(repo))
}

func allowAnonymousScholarshipApplications(db *gorm.DB) error {
	if config.IsSQLite {
		return nil
	}
	if err := db.Exec(`ALTER TABLE scholarship_applications ALTER COLUMN user_id DROP NOT NULL`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE provider_applications ALTER COLUMN user_id DROP NOT NULL`).Error; err != nil {
		return err
	}
	return nil
}

func fixMissingColumns(db *gorm.DB) error {
	if err := db.Exec(`ALTER TABLE provider_applications ADD COLUMN IF NOT EXISTS see_gpa TEXT`).Error; err != nil {
		return err
	}
	// Fix missing columns for scholarship_provider_users
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS about_text TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS mission TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS "values" TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS logo_url TEXT`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS address TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS pan_number TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS founder_name TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS founder_role TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS founder_message TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS founder_image_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS facebook_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS instagram_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS youtube_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS linkedin_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS map_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS brochure_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_provider_users ADD COLUMN IF NOT EXISTS banner_url TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE provider_scholarships ADD COLUMN IF NOT EXISTS exam_date TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE provider_scholarships ADD COLUMN IF NOT EXISTS exam_time TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarships ADD COLUMN IF NOT EXISTS exam_date TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarships ADD COLUMN IF NOT EXISTS exam_time TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarship_applications ADD COLUMN IF NOT EXISTS roll_number TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE provider_applications ADD COLUMN IF NOT EXISTS roll_number TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE ads ADD COLUMN IF NOT EXISTS location TEXT DEFAULT ''`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE scholarships ADD COLUMN IF NOT EXISTS slug TEXT`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE provider_scholarships ADD COLUMN IF NOT EXISTS slug TEXT`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE provider_volunteers ADD COLUMN IF NOT EXISTS slug TEXT`).Error; err != nil {
		return err
	}
	db.Exec(`UPDATE scholarships SET slug = 'scholarship-' || id WHERE slug IS NULL OR slug = ''`)
	db.Exec(`UPDATE provider_scholarships SET slug = 'provider-scholarship-' || id WHERE slug IS NULL OR slug = ''`)
	db.Exec(`UPDATE provider_volunteers SET slug = 'volunteer-' || id WHERE slug IS NULL OR slug = ''`)
	return nil
}

func ginLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var statusColor = param.StatusCodeColor()
		var methodColor = param.MethodColor()
		var reset = param.ResetColor()

		icon := "?"
		switch param.Method {
		case "GET":
			icon = "[GET]"
		case "POST":
			icon = "[POST]"
		case "PUT":
			icon = "[PUT]"
		case "DELETE":
			icon = "[DEL]"
		case "PATCH":
			icon = "[PATCH]"
		case "OPTIONS":
			icon = "[OPT]"
		}

		latency := param.Latency
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}

		logger.Info("request",
			"method", param.Method,
			"path", param.Path,
			"status", param.StatusCode,
			"latency", latency.String(),
			"ip", param.ClientIP,
		)

		return fmt.Sprintf("[API] %v |%s %3d %s| %13v | %15s | %s %s%-7s %s %s\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			statusColor, param.StatusCode, reset,
			latency,
			param.ClientIP,
			icon,
			methodColor, param.Method, reset,
			param.Path,
			param.ErrorMessage,
		)
	})
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin == "" {
			origin = config.AppConfig.FrontendURL
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}