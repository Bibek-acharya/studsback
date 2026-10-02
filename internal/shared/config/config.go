package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       string
	GinMode    string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	JWTSecret string
	JWTExpiry string

	SuperAdminEmail    string
	SuperAdminPassword string
	SuperAdminRole     string
	SuperAdminFirst    string
	SuperAdminLast     string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string
	CookieDomain       string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string

	EmbeddingEnabled   bool
	EmbeddingAPIKey    string
	EmbeddingBaseURL   string
	EmbeddingModel     string
	VectorDimension    int
	EmbeddingBatchSize int

	LLMEnabled       bool
	LLMBaseURL       string
	LLMModel         string
	LLMAPIKey        string
	LLMMaxConcurrent int

	GeminiAPIKey string
	GeminiModel  string

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// StudyResourceVideoMaxSizeMB caps video-lecture uploads in megabytes.
	// The four document/image resource types keep their fixed 20MB cap.
	StudyResourceVideoMaxSizeMB int
	// StudyResourceVideoTranscodeTimeout bounds one ffmpeg normalization run
	// (upload -> broadly playable H.264/AAC MP4).
	StudyResourceVideoTranscodeTimeout time.Duration

	// CoinsReconcileInterval is how often the StudsToken ledger re-checks its
	// five double-entry invariants. The spec says nightly; the default is
	// hourly because the whole value of the check is timeliness, five
	// aggregate queries are cheap at launch scale, and a nightly interval means
	// discovering drift up to 24 hours late. The per-check duration is logged
	// on every healthy pass, so the point at which this becomes expensive is
	// visible before it becomes a problem.
	CoinsReconcileInterval time.Duration
	// CoinsReconcileTimeout bounds one full reconciliation pass, so a slow
	// check cannot overlap the next tick or hold a connection indefinitely.
	CoinsReconcileTimeout time.Duration

	// CoinsReferralQualifyInterval is how often the referral qualification pass
	// runs. It pays the referrals whose invitee has qualified and whose 7-day
	// window has closed.
	//
	// DELIBERATELY SHORTER than the reconciliation interval, and the reason is that
	// the two jobs answer different questions on different clocks. Reconciliation is a
	// read-only invariant check over aggregates, so hourly is plenty. Qualification
	// moves money, so the interval is also the delay between a student finishing
	// their profile and seeing the coins they earned — and an earn mechanic that pays
	// on a nightly lag reads as broken even when it is working. 15 minutes keeps that
	// delay invisible without paying for a ticker: the pass is a single indexed
	// SELECT that returns nothing when no referral is waiting.
	CoinsReferralQualifyInterval time.Duration
	// CoinsReferralQualifyTimeout bounds one qualification pass. Each referral in a
	// pass is its own transaction holding its own per-user advisory lock, so this is
	// what stops a degraded database from leaving a pass running across several ticks
	// with a transaction open.
	CoinsReferralQualifyTimeout time.Duration

	// CoinsExpirySweepInterval is how often the expiry sweep runs.
	//
	// HOURLY, and the reason is that expiry has no user-visible delay in the way
	// qualification does. A lapsed lot is already unspendable the moment its
	// ExpiresAt passes — openLotQuery excludes it — so the sweep is not what stops a
	// student spending expired coins, it is what makes the BALANCE they are shown
	// true. That is worth doing promptly and not worth doing often, because the pass
	// is a write per expired lot and there is nothing to write until one lapses.
	//
	// The same hour as the reconciler is deliberate rather than convenient: the two
	// are complementary halves of one question ("is this balance true?"), so
	// running them on the same clock means one log line's worth of elapsed time can
	// be read as a single story.
	CoinsExpirySweepInterval time.Duration
	// CoinsExpirySweepTimeout bounds one sweep pass. Each lot in a pass is its own
	// transaction holding its own per-user advisory lock, and InUserTx sets
	// lock_timeout = 3s, so this is what stops a degraded database from leaving a
	// pass running across several ticks.
	CoinsExpirySweepTimeout time.Duration

	EsewaTestMode     bool
	EsewaMerchantCode string
	EsewaSecretKey    string
	EsewaSuccessURL   string
	EsewaFailureURL   string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// NATS
	NATSURL    string
	NATSStream string
}

var AppConfig *Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dbHost := getEnv("DB_HOST", "localhost")
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	if strings.Contains(strings.ToLower(dbHost), "neon.tech") && strings.ToLower(dbSSLMode) != "require" {
		log.Println("DB_HOST points to Neon; forcing DB_SSLMODE=require")
		dbSSLMode = "require"
	}

	AppConfig = &Config{
		Port:               getEnv("PORT", "8080"),
		DBHost:             dbHost,
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "studsphere_user"),
		DBPassword:         getEnv("DB_PASSWORD", "studsphere_pass"),
		DBName:             getEnv("DB_NAME", "studsphere"),
		JWTSecret:          getEnv("JWT_SECRET", "your-secret-key"),
		JWTExpiry:          getEnv("JWT_EXPIRY", "24h"),
		GinMode:            getEnv("GIN_MODE", "debug"),
		SuperAdminEmail:    getEnv("SUPER_ADMIN_EMAIL", "admin@studsphere.com"),
		SuperAdminPassword: getEnv("SUPER_ADMIN_PASSWORD", "Admin@1234"),
		SuperAdminRole:     getEnv("SUPER_ADMIN_ROLE", "super_admin"),
		SuperAdminFirst:    getEnv("SUPER_ADMIN_FIRST_NAME", "Super"),
		SuperAdminLast:     getEnv("SUPER_ADMIN_LAST_NAME", "Admin"),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		CookieDomain:       getEnv("COOKIE_DOMAIN", ""),
		DBSSLMode:          dbSSLMode,
		SMTPHost:           getEnv("SMTP_HOST", "smtp.hostinger.com"),
		SMTPPort:           getEnv("SMTP_PORT", "587"),
		SMTPUser:           getEnv("SMTP_USER", "system@studsphere.com"),
		SMTPPass:           getEnv("SMTP_PASS", "Systemtask@200"),
		EmbeddingEnabled:   getEnv("EMBEDDING_ENABLED", "false") == "true",
		EmbeddingAPIKey:    getEnv("EMBEDDING_API_KEY", ""),
		EmbeddingBaseURL:   getEnv("EMBEDDING_BASE_URL", "https://api.openai.com/v1"),
		EmbeddingModel:     getEnv("EMBEDDING_MODEL", "liquid/lfm-2.5-embedding-350m:free"),
		VectorDimension:    getEnvInt("VECTOR_DIMENSION", 1024),
		EmbeddingBatchSize: getEnvInt("EMBEDDING_BATCH_SIZE", 20),

		LLMEnabled:       getEnv("LLM_ENABLED", "false") == "true",
		LLMBaseURL:       getEnv("LLM_BASE_URL", "https://openrouter.ai/api/v1"),
		LLMModel:         getEnv("LLM_MODEL", "openai/gpt-4o-mini"),
		LLMAPIKey:        getAPIKey(),
		LLMMaxConcurrent: getEnvInt("LLM_MAX_CONCURRENT_REQUESTS", 4),

		GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-2.0-flash-lite"),

		MinioEndpoint:  getEnv("MINIO_ENDPOINT", ""),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", ""),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", ""),
		MinioBucket:    getEnv("MINIO_BUCKET", "studsphere-storage"),
		MinioUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",

		StudyResourceVideoMaxSizeMB: getEnvInt("STUDY_RESOURCE_VIDEO_MAX_SIZE_MB", 200),
		StudyResourceVideoTranscodeTimeout: getEnvDuration(
			"STUDY_RESOURCE_VIDEO_TRANSCODE_TIMEOUT", 10*time.Minute,
		),

		CoinsReconcileInterval:       getEnvDuration("COINS_RECONCILE_INTERVAL", time.Hour),
		CoinsReconcileTimeout:        getEnvDuration("COINS_RECONCILE_TIMEOUT", 2*time.Minute),
		CoinsReferralQualifyInterval: getEnvDuration("COINS_REFERRAL_QUALIFY_INTERVAL", 15*time.Minute),
		CoinsReferralQualifyTimeout:  getEnvDuration("COINS_REFERRAL_QUALIFY_TIMEOUT", 2*time.Minute),
		CoinsExpirySweepInterval:     getEnvDuration("COINS_EXPIRY_SWEEP_INTERVAL", time.Hour),
		CoinsExpirySweepTimeout:      getEnvDuration("COINS_EXPIRY_SWEEP_TIMEOUT", 5*time.Minute),

		EsewaTestMode:     getEnv("ESEWA_TEST_MODE", "true") == "true",
		EsewaMerchantCode: getEnv("ESEWA_MERCHANT_CODE", "EPAYTEST"),
		EsewaSecretKey:    getEnv("ESEWA_SECRET_KEY", "8gBm/:&EnhH.1/q"),
		EsewaSuccessURL:   getEnv("ESEWA_SUCCESS_URL", "http://localhost:3000/scholarship-apply/project-shiksha/success"),
		EsewaFailureURL:   getEnv("ESEWA_FAILURE_URL", "http://localhost:3000/scholarship-apply/project-shiksha/payment"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),
		NATSURL:       getEnv("NATS_URL", "nats://localhost:4222"),
		NATSStream:    getEnv("NATS_STREAM_NAME", "messaging"),
	}
}

func getAPIKey() string {
	if key := os.Getenv("LLM_API_KEY"); key != "" {
		return key
	}
	return os.Getenv("OPENROUTER_API_KEY")
}

func (c *Config) EsewaGatewayURL() string {
	if c.EsewaTestMode {
		return "https://rc-epay.esewa.com.np/api/epay/main/v2/form"
	}
	return "https://epay.esewa.com.np/api/epay/main/v2/form"
}

func (c *Config) EsewaStatusAPIURL() string {
	if c.EsewaTestMode {
		return "https://rc.esewa.com.np/api/epay/transaction/status/"
	}
	return "https://esewa.com.np/api/epay/transaction/status/"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

// getEnvDuration parses a Go duration value ("10m", "90s", "1h"). Unparsable
// or non-positive values fall back to defaultValue so a typo can never turn
// into an unbounded or instantly-expiring timeout.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(value)); err == nil && d > 0 {
			return d
		}
	}
	return defaultValue
}
