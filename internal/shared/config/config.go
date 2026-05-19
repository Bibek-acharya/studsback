package config

import (
	"log"
	"os"
	"strconv"
	"strings"

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

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	EsewaTestMode     bool
	EsewaMerchantCode string
	EsewaSecretKey    string
	EsewaSuccessURL   string
	EsewaFailureURL   string
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
		EmbeddingModel:     getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
		VectorDimension:    getEnvInt("VECTOR_DIMENSION", 1536),
		EmbeddingBatchSize: getEnvInt("EMBEDDING_BATCH_SIZE", 20),

		MinioEndpoint:  getEnv("MINIO_ENDPOINT", ""),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", ""),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", ""),
		MinioBucket:    getEnv("MINIO_BUCKET", "studsphere-storage"),
		MinioUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",

		EsewaTestMode:     getEnv("ESEWA_TEST_MODE", "true") == "true",
		EsewaMerchantCode: getEnv("ESEWA_MERCHANT_CODE", "EPAYTEST"),
		EsewaSecretKey:    getEnv("ESEWA_SECRET_KEY", "8gBm/:&EnhH.1/q"),
		EsewaSuccessURL:   getEnv("ESEWA_SUCCESS_URL", "http://localhost:3000/scholarship-apply/project-shiksha/success"),
		EsewaFailureURL:   getEnv("ESEWA_FAILURE_URL", "http://localhost:3000/scholarship-apply/project-shiksha/payment"),
	}
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
