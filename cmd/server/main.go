package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"studsphere/backend/internal/admission"
	"studsphere/backend/internal/ai"
	"studsphere/backend/internal/analytics"
	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/chat"
	"studsphere/backend/internal/coins"
	"studsphere/backend/internal/college"
	"studsphere/backend/internal/counselling"
	"studsphere/backend/internal/downloadcenter"
	"studsphere/backend/internal/education"
	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/embedding"
	"studsphere/backend/internal/faq"
	"studsphere/backend/internal/feedback"
	"studsphere/backend/internal/follow"
	"studsphere/backend/internal/forum"
	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/jobs"
	"studsphere/backend/internal/location"
	"studsphere/backend/internal/messaging"
	"studsphere/backend/internal/messaging/domain"
	"studsphere/backend/internal/mocktests"
	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/pressmedia"
	"studsphere/backend/internal/projectshiksha"
	"studsphere/backend/internal/review"
	"studsphere/backend/internal/scholarship"
	"studsphere/backend/internal/scholarshipprovider"
	"studsphere/backend/internal/search"
	"studsphere/backend/internal/search/indexer"
	"studsphere/backend/internal/search/retrieval"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"
	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/shared/seeder"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/studentdashboard"
	"studsphere/backend/internal/studyresources"
	"studsphere/backend/internal/system"
	"studsphere/backend/internal/tools"
	"studsphere/backend/internal/university"
	"studsphere/backend/migrations"

	"github.com/gin-gonic/gin"
	"github.com/meilisearch/meilisearch-go"
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

// profileCompletionAdapter answers coins.ProfileEligibility from the student
// dashboard's own twelve checks.
//
// The indirection exists so internal/coins never learns the shape of a user or of
// a profile: the wallet asks one boolean question, and the only module that can
// answer it correctly is the one that already computes it. A second
// implementation inside coins would be free to drift, and the drift would appear
// as a student being told to complete a profile the dashboard shows complete.
type profileCompletionAdapter struct {
	svc *studentdashboard.Service
}

func (a *profileCompletionAdapter) ProfileComplete(ctx context.Context, userID uint) (bool, error) {
	return a.svc.ProfileComplete(ctx, userID)
}

// objectFetcher reads an object from storage. It matches storage.Get and is
// injected by the uploads tests.
type objectFetcher func(objectKey string) (io.Reader, *storage.ObjectInfo, error)

// newUploadsHandler serves object-storage assets through the legacy public
// /uploads route. Study-resource objects are private to their domain API and
// are rejected here; unrelated assets retain their existing behavior.
func newUploadsHandler(fetch objectFetcher) gin.HandlerFunc {
	return func(c *gin.Context) {
		objectKey := c.Param("filepath")
		if objectKey == "" || objectKey == "/" {
			c.Status(http.StatusNotFound)
			return
		}
		objectKey = strings.TrimPrefix(objectKey, "/")

		// Never expose private study-resource objects through the global route.
		if storage.IsPrivateKey(objectKey) {
			c.Status(http.StatusNotFound)
			return
		}

		reader, info, err := fetch(objectKey)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if closer, ok := reader.(io.Closer); ok {
			defer closer.Close()
		}

		contentType := "application/octet-stream"
		if info != nil && info.ContentType != "" {
			contentType = info.ContentType
		}

		filename := objectKey
		if idx := strings.LastIndex(objectKey, "/"); idx >= 0 {
			filename = objectKey[idx+1:]
		}
		if c.Query("dl") == "1" {
			c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		}

		size := int64(-1)
		if info != nil && info.Size > 0 {
			size = info.Size
		}
		c.DataFromReader(http.StatusOK, size, contentType, reader, nil)
	}
}

// uploadsHandler wires the public object route to real object storage.
func uploadsHandler() gin.HandlerFunc {
	return newUploadsHandler(storage.Get)
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

	// Bad event registry means every notification fan-out would misbehave:
	// kill the boot (plan doc "startup validation" intent).
	if err := notification.ValidateRegistry(); err != nil {
		logger.Fatal("Notification event registry invalid", "error", err)
	}

	if !config.IsSQLite {
		if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
			logger.Warn("pgvector extension not available, vector search will be disabled", "error", err)
		}
	}

	embedding.RegisterGORMCallbacks(db)
	embedding.StartWorker()

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
		&college.ComparisonHistory{},
		&counselling.CounsellingBooking{},
		&scholarship.Scholarship{},
		&scholarship.ScholarshipApplication{},
		&scholarship.Payment{},
		&education.Exam{},
		&education.Course{},
		&education.Affiliation{},
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
		&forum.ForumReport{},
		&forum.ForumNotInterested{},
		&admission.Admission{},
		&jobs.Job{},
		&jobs.JobApplication{},
		&scholarshipprovider.ProviderScholarship{},
		&scholarshipprovider.ProviderApplication{},
		&scholarshipprovider.ProviderInterview{},
		&scholarshipprovider.ProviderMessage{},
		&scholarshipprovider.ProviderSettings{},
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
		&system.LandingCourseField{},
		&system.LandingCourseInstitution{},
		&system.CourseAdCard{},
		&system.CourseAdCardInstitution{},
		&system.CourseAdCardMouCompany{},
		&system.CollegeAdTrendingItem{},
		&system.CollegeRecommendationFeedback{},
		&system.AdvertiseRequest{},
		&system.SystemSetting{},
		&chat.SitePage{},
		&feedback.Feedback{},
		&faq.FAQCategory{},
		&faq.FAQItem{},
		&studyresources.StudyResource{},
		&mocktests.MockTest{},
		&mocktests.MockQuestion{},
		&mocktests.MockOption{},
		&mocktests.MockAttempt{},
		&coins.ConfigVersion{},
		&coins.CoinAccount{},
		&coins.CoinAccountBalance{},
		&coins.CoinJournal{},
		&coins.CoinPosting{},
		&coins.CoinLot{},
		&coins.ResourceUnlock{},
		&coins.UserFreeAllowance{},
		&coins.RewardGrant{},
		&coins.UserReferral{},
		&coins.ReferralCapSlot{},
		&pressmedia.PressMediaItem{},
		&downloadcenter.DownloadItem{},
		&domain.Conversation{},
		&domain.Message{},
		&domain.Participant{},
		&domain.Attachment{},
		&domain.PendingUpload{},
		&domain.OutboxEvent{},
		&search.SearchHistory{},
		notification.AccountNotification{}, notification.NotificationOutbox{},
		notification.NotificationBroadcast{}, notification.NotificationDedupeLease{},
		notification.NotificationPreference{},
		notification.NotificationDelivery{},
		notification.PublicNotification{},
		analytics.PageVisit{},
	); err != nil {
		logger.Fatal("Failed to migrate database", "error", err)
	} else {
		db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_universities_name ON universities(name) WHERE deleted_at IS NULL`)
		if !config.IsSQLite {
			if err := notification.EnsurePostgresIndexes(db); err != nil {
				logger.Fatal("Failed to create notification indexes", "error", err)
			}
			// The StudsToken ledger cannot be correct without this. AutoMigrate
			// cannot create partial or expression indexes, CHECK constraints, or
			// triggers, and it will not seed the chart of accounts. Skipping it
			// yields tables that look right and enforce nothing, which is the
			// same class of bug as internal/notification/ensure_indexes.go.
			if err := coins.EnsurePostgresIndexes(db); err != nil {
				logger.Fatal("Failed to create coin ledger constraints", "error", err)
			}
			// The entitlement tables are the same trap with a different symptom.
			// AutoMigrate creates resource_unlock and user_free_allowance and stops
			// there, so without this call they have no UNIQUE on
			// (user_id, resource_type, resource_id) — the one constraint that caps
			// referral fraud — no CHECK keeping source and journal_id consistent, and
			// no foreign key to coin_journal. Tables that look right and enforce
			// nothing is exactly what internal/notification/ensure_indexes.go
			// documents, so this sits beside the ledger's call rather than in the
			// migration list below, where a failure is only a warning.
			if err := coins.EnsureEntitlementIndexes(db); err != nil {
				logger.Fatal("Failed to create coin entitlement constraints", "error", err)
			}
			// The referral table is the same trap with a different symptom. Without
			// this, user_referral exists with the composite UNIQUE from its struct
			// tags and NOTHING else: no CHECK on the state machine, no UNIQUE on
			// phone_hash or device_hash — which is the control that actually
			// prevents the same person being counted twice, as distinct from the cap
			// which only bounds the loss — and no ceiling on the monthly cap slot.
			// That is a referral programme that looks correct in a schema dump and
			// enforces nothing.
			if err := coins.EnsureReferralIndexes(db); err != nil {
				logger.Fatal("Failed to create referral constraints", "error", err)
			}
		}
		if err := allowAnonymousScholarshipApplications(db); err != nil {
			logger.Fatal("Failed to update scholarship application user_id nullability", "error", err)
		}
		if err := fixMissingColumns(db); err != nil {
			logger.Fatal("Failed to fix missing columns", "error", err)
		}
		if err := migrations.AddUniversityAffiliations(db); err != nil {
			logger.Fatal("Failed to run university affiliations migration", "error", err)
		}
		if err := migrations.AddMeilisearchSyncSupport(db); err != nil {
			logger.Warn("Failed to run Meilisearch sync migration", "error", err)
		}
		if err := migrations.AddEmbeddingMetadata(db); err != nil {
			logger.Warn("Failed to run embedding metadata migration", "error", err)
		}
		if err := migrations.AddAdEntityLinksAndFields(db); err != nil {
			logger.Warn("Failed to run ad entity links migration", "error", err)
		}
		if err := migrations.SeedLandingCourseFields(db); err != nil {
			logger.Warn("Failed to seed landing course fields", "error", err)
		}
		if err := migrations.CreateAdvertiseRequests(db); err != nil {
			logger.Warn("Failed to run advertise requests migration", "error", err)
		}
		if err := migrations.CreateCourseAdTables(db); err != nil {
			logger.Warn("Failed to run course ad tables migration", "error", err)
		}
		if err := migrations.CreateCollegeAdTables(db); err != nil {
			logger.Warn("Failed to run college ad tables migration", "error", err)
		}
		if err := migrations.CreateSystemSettings(db); err != nil {
			logger.Warn("Failed to run system settings migration", "error", err)
		}
		if err := migrations.AddRatingToCollegeAdFeedback(db); err != nil {
			logger.Warn("Failed to run college ad feedback rating migration", "error", err)
		}
		if err := migrations.AddStudyResourceVideoColumns(db); err != nil {
			logger.Warn("Failed to run study resource video/publish migration", "error", err)
		}
		if err := migrations.CreateMockTestTables(db); err != nil {
			logger.Warn("Failed to run mock test tables migration", "error", err)
		}
		if err := migrations.CreateCoinLedger(db); err != nil {
			logger.Warn("Failed to run coin ledger migration", "error", err)
		}
		if err := migrations.CreateCoinEconomyConfigVersion(db); err != nil {
			logger.Warn("Failed to run coin economy config version migration", "error", err)
		}
		if err := migrations.CreateStudsTokenEntitlements(db); err != nil {
			logger.Warn("Failed to run StudsToken entitlement migration", "error", err)
		}
		if err := migrations.CreateUserReferralAttribution(db); err != nil {
			logger.Warn("Failed to run referral attribution migration", "error", err)
		}
		// Widen chk_user_referral_status to carry 'expired'. Separate from the migration
		// above because that one called EnsureReferralIndexes when the constraint already
		// existed, and EnsureReferralIndexes is idempotent on the constraint NAME rather
		// than on its expression — so it left the four-value vocabulary in place and the
		// first expiry write would have been refused by it. See
		// migrations/20261002_widen_referral_status_for_expiry.go.
		if err := migrations.WidenReferralStatusForExpiry(db); err != nil {
			logger.Warn("Failed to widen the referral status vocabulary", "error", err)
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

		// Notification worker handlers: outbox dispatch → email delivery.
		notification.InitWorker(notification.NewService(db), notification.NewRepository(db), db)
		emailqueue.RegisterHandler(notification.TaskTypeProcess, notification.HandleProcessTask)
		emailqueue.RegisterHandler(notification.TaskTypeEmailDeliver, notification.HandleEmailDeliverTask)

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
	notificationSvc := notification.NewService(db)
	systemSvc := system.NewService(systemRepo, notificationSvc)

	institutionRepo := institution.NewRepository(db)
	admissionSvc := admission.NewService(admission.NewRepository(db), notificationSvc)
	admissionHandler := admission.NewHandler(admissionSvc)
	authHandler := initModule(auth.NewRepository(db), auth.NewService, auth.NewHandler)
	collegeRepo := college.NewRepository(db)
	// WithTenantLookup is what lets the college module answer "which college does
	// this caller administer?" from institution_users.college_id at the SERVICE
	// layer. The one tenant-scoped rule in that module — the map pin on
	// PUT /admin/colleges/:id/location — is decided by this, so a route that
	// takes an arbitrary :id cannot be pointed at somebody else's college. See
	// internal/college/access.go.
	collegeSvc := college.NewService(collegeRepo).WithTenantLookup(institutionRepo)
	collegeHandler := college.NewHandler(collegeSvc, institutionRepo)
	counsellingHandler := counselling.NewHandler(counselling.NewService(counselling.NewRepository(db), notificationSvc))

	educationRepo := education.NewRepository(db)
	instProgramAdapter := &instProgramRepoAdapter{repo: institutionRepo}
	educationSvc := education.NewService(educationRepo, instProgramAdapter, systemSvc, notificationSvc)
	educationHandler := education.NewHandler(educationSvc)

	feedbackHandler := feedback.NewHandler(feedback.NewService(feedback.NewRepository(db), notificationSvc))

	forumHandler := forum.NewHandler(forum.NewService(forum.NewRepository(db), notificationSvc))

	institutionSvc := institution.NewService(institutionRepo, educationRepo, systemSvc, notificationSvc)
	institutionHandler := institution.NewHandler(institutionSvc, systemSvc)

	projectShikshaHandler := projectshiksha.NewHandler(projectshiksha.NewService(projectshiksha.NewRepository(db), notificationSvc))
	faqHandler := initModule(faq.NewRepository(db), faq.NewService, faq.NewHandler)
	// The study-resource service is kept rather than inlined, because the coin
	// economy's ResourceLookup is answered from it (below) and the download gate
	// needs a handler it can be attached to. One repository, one service, one
	// handler — the same three objects initModule built, held onto by name.
	studyResourcesSvc := studyresources.NewService(studyresources.NewRepository(db))
	studyResourcesHandler := studyresources.NewHandler(studyResourcesSvc)
	pressMediaHandler := initModule(pressmedia.NewRepository(db), pressmedia.NewService, pressmedia.NewHandler)
	// The mock-test service is held rather than inlined for the same reason as the
	// study-resources one above: the coin economy's ResourceLookup reads the
	// mock-tests table (mock_test_lookup.go), and the paper gate needs a handler to
	// attach to.
	mockTestsSvc := mocktests.NewService(mocktests.NewRepository(db))
	mockTestsHandler := mocktests.NewHandler(mockTestsSvc)
	downloadCenterHandler := initModule(downloadcenter.NewRepository(db), downloadcenter.NewService, downloadcenter.NewHandler)
	// Coin economy: the settings key is owned by internal/system, the audit rows
	// by internal/coins, and the config store holds a short-TTL cache over both
	// because pricing is read on every wallet render and every unlock attempt.
	//
	// ONE config store and ONE repository are shared between the admin config
	// surface, the wallet endpoints and the ledger. Sharing the store matters
	// more than it looks: a second store is a second 30-second cache over the
	// same row, so an admin's price change could be visible to the unlock path
	// and invisible to the admin screen that just made it. Sharing the
	// repository is what lets the unlock path open the transaction that spans
	// the spend and the entitlement.
	coinsConfig := coins.NewConfigStore(systemRepo)

	// The coin reachability invariant is checked once, here, and only logged.
	//
	// This is deliberately NOT a logger.Fatal and deliberately not an error
	// return. The stored coin_economy row and DefaultEconomyConfig both break
	// both rules today (25 vs 40, 90 vs 80) and every gate is off, so a fatal
	// check here would refuse to start the whole server over the deferred
	// StudsToken pricing decision — taking the features that do work down with
	// it. One named, greppable warning per boot is the right severity. The
	// full reasoning, and the three things that would have to be true before
	// promoting this to fatal, are in the "WHY THIS IS A WARNING AND NOT AN
	// ERROR" block on coins.ReachabilityWarning — read it before changing this
	// to a Fatal.
	//
	// A read failure is a warning too, for the same reason: this check is
	// observability, and it must not become a new way for the server to refuse
	// to boot. The economy still works if this line never prints.
	if cfg, err := coinsConfig.Load(); err != nil {
		logger.Warn("Coin economy reachability check did not run: config unreadable", "error", err)
	} else if warning := coins.ReachabilityWarning(cfg); warning != "" {
		logger.Warn(warning)
	}

	coinsRepo := coins.NewRepository(db)
	coinsService := coins.NewServiceWithRepository(coinsRepo, coinsConfig, coins.NewVersionStore(db))
	coinsHandler := coins.NewHandler(coinsService)
	coinsLedger := coins.NewLedger(coinsRepo, coinsConfig)
	reviewHandler := review.NewHandler(review.NewService(review.NewRepository(db), notificationSvc))
	scholarshipRepo := scholarship.NewRepository(db)
	scholarshipSvc := scholarship.NewService(scholarshipRepo, db, systemSvc, notificationSvc)
	scholarshipHandler := scholarship.NewHandler(scholarshipSvc, scholarship.NewPaymentService(db, notificationSvc))

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

	scholarshipPSvc := scholarshipprovider.NewService(scholarshipprovider.NewRepository(db), notificationSvc)
	scholarshipPHandler := scholarshipprovider.NewHandler(scholarshipPSvc)

	auth.SetScholarshipProviderHandler(scholarshipPHandler)
	auth.SetInstitutionService(institutionSvc)
	auth.SetNotifier(notificationSvc)
	// Hoisted above the coin wiring: the wallet's 402 body has to ask whether the
	// caller has already finished their profile, and that answer is computed by
	// this module's twelve checks rather than by a second copy of them inside
	// internal/coins. One service, shared, so the two screens cannot disagree.
	studentDashboardSvc := studentdashboard.NewService(studentdashboard.NewRepository(db), notificationSvc)
	studentDashHandler := studentdashboard.NewHandler(studentDashboardSvc)

	// The student-facing wallet: /api/v1/coins/{balance,transactions,allowance,unlock}.
	//
	// Two lookups are wired and they are not the same kind of thing.
	//
	// profileCompletionAdapter is an adapter DEFINED HERE, because the twelve
	// checks that answer it are studentdashboard's and must not be reimplemented
	// inside coins. The resource lookup is the other way round: the answer lives
	// in a table coins needs to read, so the adapter lives in internal/coins
	// (study_resource_lookup.go) and is handed the studyresources service. See
	// that file for why the import points that way rather than this one.
	//
	// The resource lookup now makes 404 RESOURCE_NOT_FOUND reachable for a study
	// resource and a mock test, and supplies the title that names a purchase in the
	// debit receipt and in the wallet history.
	//
	// It is a COMPOSITE of one adapter per owning module rather than a single
	// lookup, and that is the whole point of the shape: internal/coins never learns
	// which table holds which class, so adding press media or the download centre
	// later is one more adapter in this list and no change to the wallet. The two
	// remaining classes are not wired, and an unlock naming one is a 404 until
	// their slice lands rather than a purchase against the wrong table.
	// The profile-completion AWARD: the first earn mechanic.
	//
	// Wired as three seams rather than one call, and the dependency arrows all
	// point one way. auth owns the trigger (it is the only module that writes the
	// twelve completion fields) and declares a one-method port for it; coins owns
	// the ladder and declares a port for the completion percentage; the adapter
	// that satisfies the latter lives HERE because studentdashboard is a third
	// module and neither of the other two may import it.
	//
	// auth is wired before coinsWalletAPI only because studentDashboardSvc is
	// needed for both and is constructed above; there is no ordering requirement
	// between the award and the wallet.
	profileAwardSvc := coins.NewProfileAwardService(
		coinsRepo,
		coinsLedger,
		&profilePercentAdapter{svc: studentDashboardSvc},
		nil, // no notifier: coins.credited is emitted by the award itself
	)
	auth.SetProfileAwarder(&profileAwarderAdapter{svc: profileAwardSvc})

	// Referral attribution: the second earn mechanic, and the only one with a fraud
	// surface.
	//
	// Wired as two seams for the same reason the profile award is. auth owns the
	// TRIGGER — it is the only module that inserts into users,
	// institution_users and scholarship_provider_users — and declares a one-method
	// port. coins owns the ATTRIBUTION table and every uniqueness and cap
	// constraint on it. Neither imports the other; the adapter in
	// referral_wiring.go, in this package, is the only place the two vocabularies
	// meet.
	//
	// NOT an award yet. Attribution records that a code was accepted; it pays
	// nothing. Qualification, the 7-day reserved hold, the release and the clawback
	// are the next slice, and the columns and the single-transaction settlement
	// seam they need already exist (coins.SettleReferral). Wiring this here means
	// the four (really six) user-creation paths start recording referrals the moment
	// this deploys, which is the whole point of doing attribution and payment
	// separately: the relationship is worth capturing from the first day even while
	// the payout is still dark.
	//
	// The two qualification ports are attached here rather than being constructor
	// parameters, and the phone one is nil ON PURPOSE.
	//
	// WithQualifiers takes the SAME profile-completion adapter the profile award uses
	// above, deliberately not a second implementation of the twelve checks: a student
	// paid against one definition of "complete" while the page shows them another is
	// the drift profile_award.go exists to prevent.
	//
	// The phone argument is nil because THERE IS NO PHONE VERIFICATION IN THIS
	// SCHEMA. auth.User carries `Phone string` with no verification state;
	// docs/coin-system/01-current-state-and-feasibility.md §3.4 records that grepping
	// for is_phone_verified / phone_verified returns nothing, and
	// 04-implementation-plan.md §2.3 calls adding `phone_verified_at` "a launch
	// blocker for the referral earn, not a nice-to-have".
	//
	// So §5.2's second condition cannot be evaluated, and the qualification pass
	// therefore pays nothing — it fails CLOSED and counts the referrals it declined,
	// rather than substituting "has a non-empty phone", which would let anyone type
	// ten digits and qualify. The nil is here so that adding the verification flow is
	// a one-argument change at the wiring, in the one place that knows the dependency
	// is missing.
	referralSvc := coins.NewReferralService(coinsRepo, coinsLedger).
		WithQualifiers(
			&profilePercentAdapter{svc: studentDashboardSvc},
			nil, // coins.PhoneVerification: no implementation exists yet
		).
		WithNotifier(notificationSvc)
	auth.SetReferralAttributor(&referralAttributorAdapter{svc: referralSvc})

	coinsWalletAPI := coins.NewUnlockAPI(coinsService, coinsLedger).
		WithProfileEligibility(&profileCompletionAdapter{svc: studentDashboardSvc}).
		WithResourceLookup(coins.NewResourceLookups(
			coins.ClassLookup{
				Classes: []string{coins.ResourceTypeStudyResource, coins.ResourceTypeVideo},
				Lookup:  coins.NewStudyResourceLookup(studyResourcesSvc),
			},
			coins.ClassLookup{
				Classes: []string{coins.ResourceTypeMockTest},
				Lookup:  coins.NewMockTestLookup(mockTestsSvc),
			},
		))

	// The download gate, and the one place a study-resource file is paid for.
	//
	// Attaching it here rather than at construction is what keeps the dependency
	// one-way: internal/coins imports internal/studyresources to read the table,
	// and internal/studyresources declares the port this satisfies rather than
	// importing coins back. See internal/studyresources/download_gate.go.
	//
	// The gate is inert until an admin sets gates_enabled.study_resource, and so
	// is the unlock endpoint until unlock_endpoint_enabled. Turning the document
	// path live means turning on BOTH: the gate is what makes a purchase
	// meaningful, and the endpoint is what stops a student spending coins
	// through a route that is cheaper than the one they are being charged on.
	//
	// The three gates below are the SAME *UnlockAPI answering three ports, and each
	// has its own switch: gates_enabled.study_resource for downloads,
	// gates_enabled.video for playback, gates_enabled.mock_test for papers. They
	// are wired independently on purpose — the kill switch is only a kill switch
	// if the classes can be turned on and off one at a time — and all three ship
	// dark, so a deployment that has set none of them behaves byte-identically to
	// one that has not heard of the coin economy.
	studyResourcesHandler.WithDownloadGate(coins.NewDownloadGate(coinsWalletAPI)).
		WithPlaybackGate(coins.NewPlaybackGate(coinsWalletAPI))
	mockTestsHandler.WithPaperGate(coins.NewPaperGate(coinsWalletAPI))
	systemHandler := system.NewHandler(systemSvc)
	toolsHandler := initModule(tools.NewRepository(db), tools.NewService, tools.NewHandler)
	universityHandler := initModule(university.NewRepository(db), university.NewService, university.NewHandler)
	searchHandler := initSearchHandler(db)
	searchHandler.SetHistoryRepository(search.NewSearchHistoryRepository(db))
	chatService := chat.NewService(db)
	chatHandler := chat.NewHandler(chatService)
	aiService := ai.NewService(db)
	aiHandler := ai.NewHandler(aiService)
	locationHandler := location.NewHandler(location.NewService())
	followRepo := follow.NewRepository(db)
	followService := follow.NewService(followRepo, notificationSvc)
	followHandler := follow.NewHandler(followService)
	jobsHandler := jobs.NewHandler(jobs.NewServiceWithDB(jobs.NewRepository(db), db, notificationSvc))
	logger.Info("All module handlers initialized")

	logger.Info("Setting up router...")
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger())
	router.Use(corsMiddleware())

	// Public object route. Study-resource objects (documents and videos) are
	// private to their domain API and are rejected here; everything else keeps
	// working exactly as before.
	router.GET("/uploads/*filepath", uploadsHandler())
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

	// Module-local gates for the four modules audited behind roleMW.
	//
	// roleMW is a multi-tenant list being used as an authorisation boundary, and
	// for these four it granted too much: a scholarship_provider or institution
	// account could read every visitor's contact message on the platform, rewrite
	// site-wide ad and landing configuration, add and delete colleges and
	// universities, and trigger a full embedding reindex that nulls every
	// embedding column on the platform. roleMW is left exactly as it is — 17
	// modules are wired to it, each configured for its own reasons, and narrowing
	// it globally would break all of them.
	//
	// Each list below is built from the module's own PlatformAdminRoles() rather
	// than being spelled out here, so the middleware at the edge and the
	// service-layer checks in the module cannot drift apart. The service is the
	// authority in all four: these gates are the cheaper first refusal, and a
	// module handed the wide roleMW at the call site is still refused inside.
	//
	// See internal/{system,college,university,search}/access.go for the argument
	// each rule rests on.
	//
	// college is deliberately absent from this list. Its one legitimate non-admin
	// principal — an institution account moving its OWN college's map pin, keyed on
	// institution_users.college_id — has to stay reachable, so that module decides
	// per-record in the service instead of per-role here. See
	// internal/college/access.go.
	systemAdminRoleMW := middleware.RequireRole(system.PlatformAdminRoles()...)
	universityAdminRoleMW := middleware.RequireRole(university.PlatformAdminRoles()...)
	searchAdminRoleMW := middleware.RequireRole(search.PlatformAdminRoles()...)

	usageTracker := analytics.NewUsageTracker(30 * time.Minute)
	router.Use(usageTracker.Middleware())
	analyticsHandler := analytics.NewHandler(analytics.NewService(db, usageTracker))
	analyticsHandler.RegisterRoutes(router, authMW, middleware.RequireRole("superadmin", "super_admin"))
	analyticsHandler.RegisterPublicRoutes(router)

	// NO roleMW. The admission module derives its own operator gate from
	// admission.PlatformAdminRoles(); passing the shared multi-tenant list admitted
	// institution and scholarship_provider accounts to every applicant's PII. The
	// signature no longer accepts one so it cannot be passed back in. The remaining
	// modules below still take it, and each is a separate question — this says
	// nothing about the other 16.
	admission.RegisterRoutes(router, authMW, admissionHandler)
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
	// NO roleMW, same reasoning as admission above: this module's admin group carries
	// applicant and GUARDIAN PII including household income, so it derives its own
	// gate from scholarship.PlatformAdminRoles(). Provider-owned application review
	// lives in internal/scholarshipprovider and is filtered by provider_id.
	scholarship.RegisterRoutes(router, authMW, scholarshipHandler)
	scholarshipprovider.RegisterRoutes(router, authMW, roleMW, scholarshipPHandler)
	scholarshipprovider.RegisterPublicRoutes(router, scholarshipPHandler)
	scholarshipprovider.RegisterMessageRoutes(router, authMW, scholarshipPHandler)
	studentdashboard.RegisterRoutes(router, authMW, roleMW, studentDashHandler)
	system.RegisterRoutes(router, authMW, roleMW, systemAdminRoleMW, systemHandler)
	tools.RegisterRoutes(router, authMW, roleMW, toolsHandler)
	university.RegisterRoutes(router, authMW, roleMW, universityAdminRoleMW, universityHandler)
	search.RegisterRoutes(router, authMW, roleMW, searchAdminRoleMW, searchHandler)
	chat.RegisterRoutes(router, chatHandler)
	ai.RegisterRoutes(router, aiHandler)
	location.RegisterRoutes(router, locationHandler)
	follow.RegisterRoutes(router, authMW, followHandler)
	// Job catalogue: platform admins only, deliberately NOT roleMW. roleMW admits
	// "institution" and "scholarship_provider", and the job catalogue is the
	// platform's own first-party postings — an institution account has no
	// postings of its own here to own, and DELETE on one of them destroys every
	// applicant's stored resume and cover letter (internal/jobs/routes.go,
	// internal/jobs/access.go). The list is the jobs module's own
	// PlatformAdminRoles(), so this gate and the service check it backs cannot
	// disagree about who is a platform admin.
	jobsAdminRoleMW := middleware.RequireRole(jobs.PlatformAdminRoles()...)
	jobs.RegisterRoutes(router, authMW, jobsAdminRoleMW, jobsHandler)

	// Study resources: admins only (superadmin guard, like notifications).
	studyResourcesRoleMW := middleware.RequireRole("superadmin", "super_admin")
	studyresources.RegisterRoutes(router, authMW, studyResourcesRoleMW, studyResourcesHandler)

	// Coin economy config: its own superadmin gate, deliberately NOT roleMW.
	// roleMW admits "institution" and "scholarship_provider", and an
	// institution account must not be able to rewrite coin pricing
	// (docs/coin-system/02-architecture.md §7). "admin" is omitted because it
	// is a phantom role here — it appears in allow-lists but is never assigned
	// to a user.
	coinAdminRoleMW := middleware.RequireRole("superadmin", "super_admin")
	coins.RegisterRoutes(router, authMW, coinAdminRoleMW, coinsHandler)
	// The student wallet sits on authMW alone, not on this gate: every endpoint
	// under /api/v1/coins returns the CALLER's own balance, allowance and
	// history, and moves nothing. POST /unlock is mounted here but dark — it
	// answers 503 until EconomyConfig.UnlockEndpointEnabled is set, which the
	// slice that adds the first resource gate does.
	coins.RegisterWalletRoutes(router, authMW, coinsWalletAPI)

	// The student's own referral summary: their code, their standing against the
	// monthly cap, and their referrals with enough state to be honest about what has
	// not paid out yet. authMW alone, owner-scoped — every query filters on the
	// CALLER's id and there is no parameter through which anybody else's could be
	// named. An institution or scholarship-provider account calling it gets its own
	// (empty) view, exactly as it gets its own empty wallet.
	//
	// The invite base URL is config.AppConfig.FrontendURL rather than a constant
	// here, because a shareable link assembled from a guessed host is a broken invite
	// on every deployment that is not localhost.
	coins.RegisterReferralRoutes(router, authMW,
		coins.NewReferralAPI(referralSvc, config.AppConfig.FrontendURL))

	// Mock tests: separate domain with nested questions/options. Browsing is
	// public, submit + attempt results require auth, CRUD is superadmin-only.
	// The handler is the one built at line 515, not a fresh one: that is the object
	// the paper gate is attached to, and building a second here would silently drop
	// the gate from the registered routes.
	mocktests.RegisterRoutes(router, authMW, studyResourcesRoleMW, mockTestsHandler)

	// Media & press + download center: superadmin-guarded like study resources.
	pressMediaRoleMW := middleware.RequireRole("superadmin", "super_admin")
	downloadCenterRoleMW := middleware.RequireRole("superadmin", "super_admin")
	pressmedia.RegisterRoutes(router, authMW, pressMediaRoleMW, pressMediaHandler)
	downloadcenter.RegisterRoutes(router, authMW, downloadCenterRoleMW, downloadCenterHandler)

	// Setup messaging routes
	api := router.Group("/api/v1")
	messaging.SetupRoutes(api, db, redisClient, natsConn, authMW, notificationSvc)

	// Notification routes are always v2-owned (P2.5: NOTIFICATIONS_V2 retired,
	// legacy off-branch removed). The poller drains the outbox for process
	// lifetime.
	notificationsAPI := api.Group("")
	notificationsAPI.Use(authMW)
	notificationHandler := notification.NewHandler(notification.NewService(db))
	notificationHandler.RegisterRoutes(notificationsAPI, middleware.RequireRole("superadmin", "super_admin"))
	// Outbox drain + broadcast fan-out expansion; stop func discarded for
	// process lifetime.
	go notification.StartPoller(db, 2*time.Second)

	// StudsToken ledger reconciliation. Re-checks the five double-entry
	// invariants on a ticker: journals net to zero, global conservation, no
	// liability overdraft, the cached balance matches the postings, and no
	// duplicate idempotency keys. It logs and never kills the process, because
	// a data-integrity problem must not destroy the evidence or take the server
	// down. The first pass is deferred by one interval so a rolling deploy does
	// not have every instance reconcile at once. Stop func discarded for
	// process lifetime, matching the poller above.
	go coins.StartReconciler(
		db,
		config.AppConfig.CoinsReconcileInterval,
		config.AppConfig.CoinsReconcileTimeout,
	)

	// Referral qualification: the pass that pays a referral whose invitee has
	// qualified and whose 7-day window has closed.
	//
	// A SEPARATE ticker from the reconciler above, deliberately, and
	// internal/coins/referral_qualification.go's header gives the three reasons: a
	// read-only invariant checker that also paid money would make its "healthy" report
	// conflate "the ledger is consistent" with "a payout worked", "just run it again"
	// would mean "move money again", and the two want opposite cadences — the
	// reconciler hourly is plenty, this wants to be quick because it is the delay
	// between a student finishing their profile and seeing what they earned.
	//
	// It is constructed with the SAME referralSvc the attribution port holds, because
	// it is the same referral state machine: attribution records the relationship,
	// this advances it. Two services over one table would be two answers to "is this
	// referral paid".
	//
	// Nothing is wired for phone verification, so this pass currently settles nothing
	// and says so in every log line it writes. That is the intended behaviour of an
	// unimplementable rule — see referral_qualification.go.
	go coins.StartReferralQualifier(
		referralSvc,
		config.AppConfig.CoinsReferralQualifyInterval,
		config.AppConfig.CoinsReferralQualifyTimeout,
	)

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
		{"news", "featured BOOLEAN DEFAULT FALSE"},
		{"news", "published BOOLEAN DEFAULT TRUE"},
		{"events", "end_date TIMESTAMP"},
		{"events", "status VARCHAR(20) DEFAULT 'upcoming'"},
		{"events", "application_link TEXT"},
		{"events", "registration_deadline TIMESTAMP"},
		{"provider_events", "application_link TEXT"},
		{"provider_events", "registration_deadline TIMESTAMP"},
		{"institution_events", "application_link TEXT"},
		{"institution_events", "registration_deadline TIMESTAMP"},
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
		// Range is required for authenticated byte-range video streaming.
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Range")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func initSearchHandler(db *gorm.DB) *search.Handler {
	meiliCfg := search.LoadMeiliConfig()
	meiliClient := search.NewMeiliClient(meiliCfg)

	if !meiliClient.IsHealthy() {
		logger.Warn("Meilisearch not available, hybrid search will be degraded")
		return search.NewHandler(nil)
	}

	logger.Info("Meilisearch connected", "host", meiliCfg.Host)

	meiliRetriever := retrieval.NewMeilisearchRetriever(meiliIndexAdapter{client: meiliClient.Client}, meiliClient.IndexPrefix)

	var vecRetriever *retrieval.VectorRetriever
	if config.AppConfig.EmbeddingEnabled {
		vecRetriever = retrieval.NewVectorRetriever(db, embeddingAdapter{})
	}

	searchSvc := search.NewSearchService(db, meiliRetriever, vecRetriever, config.AppConfig.EmbeddingEnabled)

	// Start sync worker in background
	idx := indexer.NewMeiliIndexer(meiliClient)
	syncInterval := 5 * time.Second
	if v := os.Getenv("SEARCH_SYNC_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			syncInterval = d
		}
	}
	syncBatchSize := 500
	if v := os.Getenv("SEARCH_SYNC_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			syncBatchSize = n
		}
	}

	syncWorker := indexer.NewSyncWorker(db, idx, syncInterval, syncBatchSize)

	// Apply settings before synchronization starts so filters are valid immediately.
	go func() {
		ctx := context.Background()
		if err := idx.CreateIndexes(ctx); err != nil {
			logger.Warn("Failed to create Meilisearch indexes", "error", err)
			return
		}
		logger.Info("Meilisearch indexes created/updated")
		syncWorker.Start(ctx)
	}()

	return search.NewHybridHandler(searchSvc, meiliClient)
}

type embeddingAdapter struct{}

func (embeddingAdapter) GenerateEmbedding(text string) ([]float32, error) {
	return embedding.GenerateEmbedding(text)
}

type meiliIndexAdapter struct {
	client meilisearch.ServiceManager
}

func (a meiliIndexAdapter) Index(uid string) retrieval.IndexClient {
	return a.client.Index(uid)
}
