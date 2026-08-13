package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"studsphere/backend/internal/admission"
	"studsphere/backend/internal/ai"
	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/chat"
	"studsphere/backend/internal/college"
	"studsphere/backend/internal/counselling"
	"studsphere/backend/internal/education"
	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/faq"
	"studsphere/backend/internal/feedback"
	"studsphere/backend/internal/follow"
	"studsphere/backend/internal/jobs"
	"studsphere/backend/internal/forum"
	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/location"
	"studsphere/backend/internal/messaging"
	"studsphere/backend/internal/messaging/domain"
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
	"studsphere/backend/migrations"
	"studsphere/backend/internal/studentdashboard"
	"studsphere/backend/internal/system"
	"studsphere/backend/internal/tools"
	"studsphere/backend/internal/university"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// instProgramRepoAdapter wraps institution.Repository to satisfy education.InstitutionProgramRepo.
type instProgramRepoAdapter struct {
	repo *institution.Repository
}

func (a *instProgramRepoAdapter) FindProgramByGlobalCourse(institutionID, globalCourseID uint) (*education.ResolvedProgram, error) {
	p, err := a.repo.FindProgramByGlobalCourse(institutionID, globalCourseID)
	if err != nil {
		return nil, err
	}
	return &education.ResolvedProgram{
		InstitutionID:   p.InstitutionID,
		Fee:             p.Fee,
		Eligibility:     p.Eligibility,
		Capacity:        p.Capacity,
		Status:          p.Status,
		WhoShouldChoose: p.WhoShouldChoose,
		Features:        p.Features,
		FullTimeCourses: p.FullTimeCourses,
		FeeItems:        p.FeeItems,
		Overrides:       p.Overrides,
		NullifiedFields: p.NullifiedFields,
	}, nil
}

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

	if !config.IsSQLite {
		if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
			logger.Warn("pgvector extension not available, vector search will be disabled", "error", err)
		}
	}

	logger.Info("Running database migrations...")
	if err := db.AutoMigrate(
		&auth.User{},
		&auth.InstitutionUser{},
		&auth.ScholarshipProviderUser{},
		&auth.EducationEntry{},
		&auth.UserSession{},
		&auth.ProfileDocument{},
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
		&jobs.Job{},
		&jobs.JobApplication{},
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
		&institution.InstitutionBlog{},
		&institution.InstitutionQMS{},
		&institution.AdmissionPage{},
		&institution.InstitutionSettings{},
		&review.Review{},
		&review.ReviewHelpful{},
		&review.ReviewReport{},
		&review.DateReport{},
		&follow.UserFollow{},
		&projectshiksha.ShikshaApplication{},
		&projectshiksha.ShikshaPayment{},
		&system.ContactInquiry{},
		&auth.InstitutionSubscription{},
		&system.Ad{},
		&system.CarouselSlide{},
		&system.PublicNotification{},
		&chat.SitePage{},
		&feedback.Feedback{},
		&faq.FAQCategory{},
		&faq.FAQItem{},
		&domain.Conversation{},
		&domain.Message{},
		&domain.Participant{},
		&domain.Attachment{},
		&domain.PendingUpload{},
		&domain.OutboxEvent{},
	); err != nil {
		logger.Fatal("Failed to migrate database", "error", err)
	} else {
		db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_universities_name ON universities(name) WHERE deleted_at IS NULL`);
		if err := allowAnonymousScholarshipApplications(db); err != nil {
			logger.Fatal("Failed to update scholarship application user_id nullability", "error", err)
		}
		if err := fixMissingColumns(db); err != nil {
			logger.Fatal("Failed to fix missing columns", "error", err)
		}
		if err := migrations.AddUniversityAffiliations(db); err != nil {
			logger.Fatal("Failed to run university affiliations migration", "error", err)
		}
		// Cleanup dangling sub-users with provider_id = 0 from previous bug
		if err := db.Exec("DELETE FROM provider_access_users WHERE provider_id = 0").Error; err != nil {
			logger.Warn("Failed to cleanup dangling sub-users", "error", err)
		}
		logger.Info("Database migrations completed successfully")

		if err := initVectorSearch(db); err != nil {
			logger.Warn("Failed to initialize vector search", "error", err)
		} else {
			logger.Info("Vector search initialized successfully")
		}
	}

	// Initialize Redis for messaging
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.RedisAddr,
		Password: config.AppConfig.RedisPassword,
		DB:       config.AppConfig.RedisDB,
	})

	// Initialize NATS for messaging
	natsConn, err := nats.Connect(config.AppConfig.NATSURL)
	if err != nil {
		logger.Warn("Failed to connect to NATS, messaging will be HTTP-only", "error", err)
	} else {
		logger.Info("Connected to NATS")
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

		// Register the admit card PDF generation handler from the scholarship package.
		// This must happen before StartWorker so the handler is registered on the mux.
		emailqueue.RegisterHandler(emailqueue.TypeSendAdmitCard, scholarship.HandleAdmitCardTask)

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

	if err := chat.SeedSitePages(db); err != nil {
		logger.Warn("Failed to seed site pages", "error", err)
	} else {
		logger.Info("Site pages seeded successfully")
	}

	logger.Info("Initializing module handlers...")
	systemRepo := system.NewRepository(db)
	systemSvc := system.NewService(systemRepo)

	institutionRepo := institution.NewRepository(db)
	admissionHandler := initModule(admission.NewRepository(db), admission.NewService, admission.NewHandler)
	authHandler := initModule(auth.NewRepository(db), auth.NewService, auth.NewHandler)
	collegeRepo := college.NewRepository(db)
	collegeSvc := college.NewService(collegeRepo)
	collegeHandler := college.NewHandler(collegeSvc, institutionRepo)
	counsellingHandler := initModule(counselling.NewRepository(db), counselling.NewService, counselling.NewHandler)

	educationRepo := education.NewRepository(db)
	instProgramAdapter := &instProgramRepoAdapter{repo: institutionRepo}
	educationSvc := education.NewService(educationRepo, instProgramAdapter, systemSvc)
	educationHandler := education.NewHandler(educationSvc)

	feedbackHandler := initModule(feedback.NewRepository(db), feedback.NewService, feedback.NewHandler)

	forumHandler := initModule(forum.NewRepository(db), forum.NewService, forum.NewHandler)

	institutionSvc := institution.NewService(institutionRepo, educationRepo, systemSvc)
	institutionHandler := institution.NewHandler(institutionSvc, systemSvc)

	projectShikshaHandler := initModule(projectshiksha.NewRepository(db), projectshiksha.NewService, projectshiksha.NewHandler)
	faqHandler := initModule(faq.NewRepository(db), faq.NewService, faq.NewHandler)
	reviewHandler := initModule(review.NewRepository(db), review.NewService, review.NewHandler)
	scholarshipRepo := scholarship.NewRepository(db)
	scholarshipSvc := scholarship.NewService(scholarshipRepo, db, systemSvc)
	scholarshipHandler := scholarship.NewHandler(scholarshipSvc, scholarship.NewPaymentService(db))

	go func() {
		time.Sleep(10 * time.Second)
		ticker := time.NewTicker(48 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-24 * time.Hour)
			count, err := scholarshipSvc.PurgeOldDraftApplications(cutoff)
			if err != nil {
				logger.Error("Failed to purge old draft applications", "error", err)
			} else if count > 0 {
				logger.Info("Purged old draft applications", "count", count)
			}
		}
	}()
	logger.Info("Draft application cleanup cron started")

	scholarshipPHandler := initModule(scholarshipprovider.NewRepository(db), scholarshipprovider.NewService, scholarshipprovider.NewHandler)

	auth.SetScholarshipProviderHandler(scholarshipPHandler)
	auth.SetInstitutionService(institutionSvc)
	studentDashHandler := initModule(studentdashboard.NewRepository(db), studentdashboard.NewService, studentdashboard.NewHandler)
	admission.SetNotifyStudentFunc(studentDashHandler.GetService().CreateNotification)
	institution.SetNotifyStudentFunc(studentDashHandler.GetService().CreateNotification)
	systemHandler := system.NewHandler(systemSvc)
	toolsHandler := initModule(tools.NewRepository(db), tools.NewService, tools.NewHandler)
	universityHandler := initModule(university.NewRepository(db), university.NewService, university.NewHandler)
	searchHandler := search.NewHandler(search.NewService(db))
	chatService := chat.NewService(db)
	chatHandler := chat.NewHandler(chatService)
	aiService := ai.NewService(db)
	aiHandler := ai.NewHandler(aiService)
	locationHandler := location.NewHandler(location.NewService())
	followRepo := follow.NewRepository(db)
	followService := follow.NewService(followRepo)
	followHandler := follow.NewHandler(followService)
	jobsHandler := jobs.NewHandler(jobs.NewServiceWithDB(jobs.NewRepository(db), db))
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

	router.GET("/api/v1/proxy-image", func(c *gin.Context) {
		imageURL := c.Query("url")
		if imageURL == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		parsedURL, err := url.Parse(imageURL)
		if err != nil || !parsedURL.IsAbs() {
			c.Status(http.StatusBadRequest)
			return
		}

		allowedDomains := map[string]bool{
			"projectshiksha.hundredgroupnepal.org": true,
			"api.qrserver.com":                     true,
			"chart.googleapis.com":                 true,
		}
		if !allowedDomains[parsedURL.Host] {
			c.Status(http.StatusForbidden)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(imageURL)
		if err != nil {
			c.Status(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Status(http.StatusBadGateway)
			return
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType(body)
		}

		c.DataFromReader(http.StatusOK, int64(len(body)), contentType, bytes.NewReader(body), map[string]string{
			"Access-Control-Allow-Origin": "*",
			"Cache-Control":               "public, max-age=86400",
		})
	})

	authMW := middleware.Auth()
	roleMW := middleware.RequireRole("admin", "super_admin", "scholarship_provider", "scholarship-provider", "Scholarship Provider", "scholarship_provider_subuser", "institution")

	admission.RegisterRoutes(router, authMW, roleMW, admissionHandler)
	auth.RegisterRoutes(router, authMW, roleMW, authHandler)
	college.RegisterRoutes(router, authMW, roleMW, collegeHandler)
	counselling.RegisterRoutes(router, authMW, roleMW, counsellingHandler)
	education.RegisterRoutes(router, authMW, roleMW, educationHandler)
	feedback.RegisterRoutes(router, authMW, roleMW, feedbackHandler)
	forum.RegisterRoutes(router, authMW, roleMW, forumHandler)
	institution.RegisterRoutes(router, authMW, roleMW, institutionHandler)
	projectshiksha.RegisterRoutes(router, authMW, roleMW, projectShikshaHandler)
	faq.RegisterRoutes(router, authMW, roleMW, faqHandler)
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
	chat.RegisterRoutes(router, chatHandler)
	ai.RegisterRoutes(router, aiHandler)
	location.RegisterRoutes(router, locationHandler)
	follow.RegisterRoutes(router, authMW, followHandler)
	jobs.RegisterRoutes(router, authMW, roleMW, jobsHandler)

	// Setup messaging routes
	api := router.Group("/api/v1")
	messaging.SetupRoutes(api, db, redisClient, natsConn, authMW)

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

func hasColumn(db *gorm.DB, table, column string) (bool, error) {
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(?)", table).Scan(&cols).Error; err != nil {
		return false, err
	}
	for _, c := range cols {
		if c.Name == column {
			return true, nil
		}
	}
	return false, nil
}

func addColumnIfMissing(db *gorm.DB, table, definition string) error {
	if config.IsSQLite {
		colName := strings.Split(definition, " ")[0]
		exists, err := hasColumn(db, table, colName)
		if err != nil || exists {
			return err
		}
		return db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, definition)).Error
	}
	return db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s", table, definition)).Error
}

func dropColumnIfExists(db *gorm.DB, table, column string) error {
	if config.IsSQLite {
		exists, err := hasColumn(db, table, column)
		if err != nil || !exists {
			return err
		}
		return db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)).Error
	}
	return db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", table, column)).Error
}

func fixMissingColumns(db *gorm.DB) error {
	cols := []struct {
		table string
		def   string // used for ADD COLUMN
	}{
		{"institution_users", "profile_status VARCHAR(20) DEFAULT 'draft'"},
		{"provider_applications", "see_gpa TEXT"},
		{"institution_users", "contact_email TEXT DEFAULT ''"},
		{"institution_users", "contact_phone TEXT DEFAULT ''"},
		{"institution_users", "map_url TEXT DEFAULT ''"},
		{"scholarships", "institution_id INTEGER DEFAULT 0"},
		{"scholarships", "short_desc TEXT DEFAULT ''"},
		{"scholarships", "status TEXT DEFAULT 'draft'"},
		{"scholarships", "deleted_at TIMESTAMP"},
		{"institution_users", "facebook_url TEXT DEFAULT ''"},
		{"institution_users", "instagram_url TEXT DEFAULT ''"},
		{"institution_users", "tiktok_url TEXT DEFAULT ''"},
		{"institution_users", "youtube_url TEXT DEFAULT ''"},
		{"institution_users", "linkedin_url TEXT DEFAULT ''"},
		{"institution_users", "level TEXT DEFAULT ''"},
		{"institution_users", "university_affiliations JSONB DEFAULT '[]'"},
		{"institution_users", "non_university_affiliation TEXT DEFAULT ''"},
		{"colleges", "card_image_url TEXT DEFAULT ''"},
		{"colleges", "banner_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "about_text TEXT DEFAULT ''"},
		{"scholarship_provider_users", "mission TEXT DEFAULT ''"},
		{`scholarship_provider_users`, `"values" TEXT DEFAULT ''`},
		{"scholarship_provider_users", "logo_url TEXT"},
		{"scholarship_provider_users", "address TEXT DEFAULT ''"},
		{"scholarship_provider_users", "pan_number TEXT DEFAULT ''"},
		{"scholarship_provider_users", "founder_name TEXT DEFAULT ''"},
		{"scholarship_provider_users", "founder_role TEXT DEFAULT ''"},
		{"scholarship_provider_users", "founder_message TEXT DEFAULT ''"},
		{"scholarship_provider_users", "founder_image_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "facebook_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "instagram_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "youtube_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "linkedin_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "map_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "brochure_url TEXT DEFAULT ''"},
		{"scholarship_provider_users", "banner_url TEXT DEFAULT ''"},
		{"provider_scholarships", "exam_date TEXT DEFAULT ''"},
		{"provider_scholarships", "exam_time TEXT DEFAULT ''"},
		{"scholarships", "exam_date TEXT DEFAULT ''"},
		{"scholarships", "exam_time TEXT DEFAULT ''"},
		{"scholarship_applications", "roll_number TEXT DEFAULT ''"},
		{"provider_applications", "roll_number TEXT DEFAULT ''"},
		{"ads", "location TEXT DEFAULT ''"},
		{"scholarships", "slug TEXT"},
		{"provider_scholarships", "slug TEXT"},
		{"provider_volunteers", "slug TEXT"},
		{"courses", "is_global BOOLEAN DEFAULT FALSE"},
		{"courses", "status TEXT DEFAULT 'draft'"},
		{"courses", "created_by INTEGER DEFAULT 0"},
		{"courses", "source_program_id INTEGER DEFAULT NULL"},
		{"institution_programs", "global_course_id INTEGER DEFAULT NULL"},
		{"institution_programs", "overrides JSONB DEFAULT '{}'"},
		{"institution_programs", "nullified_fields JSONB DEFAULT '[]'"},
		{"admission_pages", "level TEXT DEFAULT ''"},
	}
	for _, c := range cols {
		if err := addColumnIfMissing(db, c.table, c.def); err != nil {
			return err
		}
	}
	drops := []struct {
		table  string
		column string
	}{
		{"institution_events", "title"},
		{"institution_events", "date"},
		{"institution_events", "image"},
		{"institution_news", "excerpt"},
		{"institution_news", "image"},
		{"institution_news", "category"},
		{"institution_news", "published"},
	}
	for _, d := range drops {
		if err := dropColumnIfExists(db, d.table, d.column); err != nil {
			return err
		}
	}
	if !config.IsSQLite {
		if err := db.Exec(`CREATE SEQUENCE IF NOT EXISTS scholarship_roll_number_seq START WITH 50`).Error; err != nil {
			return err
		}
		db.Exec(`ALTER TABLE reviews ALTER COLUMN course DROP NOT NULL`)
		db.Exec(`ALTER TABLE reviews ALTER COLUMN "level" DROP NOT NULL`)
		db.Exec(`ALTER TABLE reviews ALTER COLUMN summary_title DROP NOT NULL`)
	}
	db.Exec(`UPDATE scholarships SET slug = 'scholarship-' || id WHERE slug IS NULL OR slug = ''`)
	db.Exec(`UPDATE provider_scholarships SET slug = 'provider-scholarship-' || id WHERE slug IS NULL OR slug = ''`)
	db.Exec(`UPDATE provider_volunteers SET slug = 'volunteer-' || id WHERE slug IS NULL OR slug = ''`)
	db.Exec(`UPDATE institution_users SET profile_status = 'published' WHERE profile_status = 'draft' AND id IN (SELECT institution_id FROM institution_settings WHERE public_profile = true)`)

	if !config.IsSQLite {
		db.Exec(`UPDATE institution_news SET slug = 'inst-' || id || '-' || LOWER(REPLACE(REPLACE(REPLACE(title, ' ', '-'), '''', ''), '&', '')) WHERE (slug IS NULL OR slug = '') AND deleted_at IS NULL`)
		db.Exec(`UPDATE institution_events SET slug = 'inst-' || id || '-' || LOWER(REPLACE(REPLACE(REPLACE(name, ' ', '-'), '''', ''), '&', '')) WHERE (slug IS NULL OR slug = '') AND deleted_at IS NULL`)
		db.Exec(`UPDATE institution_blogs SET slug = 'inst-' || id || '-' || LOWER(REPLACE(REPLACE(REPLACE(title, ' ', '-'), '''', ''), '&', '')) WHERE (slug IS NULL OR slug = '') AND deleted_at IS NULL`)
		db.Exec(`UPDATE admission_pages SET level = COALESCE(data->'overview_data'->>'level', '') WHERE (level IS NULL OR level = '') AND deleted_at IS NULL`)
	}
	return nil
}

func initVectorSearch(db *gorm.DB) error {
	if config.IsSQLite {
		logger.Info("SQLite does not support pgvector, skipping vector search init")
		return nil
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		logger.Warn("pgvector extension not available, skipping vector search init", "error", err)
		return nil
	}
	dim := config.AppConfig.VectorDimension
	tables := []string{"colleges", "courses", "exams", "scholarships", "news", "events", "blogs", "site_pages", "institution_entrances"}
	for _, table := range tables {
		var colType string
		db.Raw(fmt.Sprintf("SELECT data_type FROM information_schema.columns WHERE table_name = '%s' AND column_name = 'embedding'", table)).Scan(&colType)
		if colType == "" {
			if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN embedding vector(%d)", table, dim)).Error; err != nil {
				return fmt.Errorf("failed to add embedding column to %s: %w", table, err)
			}
			logger.Info("Added embedding column", "table", table, "dim", dim)
		}
	}
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
