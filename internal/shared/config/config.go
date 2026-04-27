package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	GinMode  string
	DBHost   string
	DBPort   string
	DBUser   string
	DBPassword string
	DBName   string
	DBSSLMode string

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

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string

	MinIOEndpoint   string
	MinIOAPIEndpoint string
	MinIOAccessKey  string
	MinIOSecretKey  string
	MinIOBucket     string
	MinIOUseSSL     bool

	EmbeddingEnabled    bool
	EmbeddingAPIKey     string
	EmbeddingBaseURL    string
	EmbeddingModel      string
	VectorDimension     int
	EmbeddingBatchSize  int
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
		DBSSLMode:          dbSSLMode,
		SMTPHost:           getEnv("SMTP_HOST", "smtp.hostinger.com"),
		SMTPPort:           getEnv("SMTP_PORT", "587"),
		SMTPUser:           getEnv("SMTP_USER", "system@studsphere.com"),
		SMTPPass:           getEnv("SMTP_PASS", "Systemtask@200"),
		MinIOEndpoint:      getEnv("MINIO_ENDPOINT", "storage.studsphere.com"),
		MinIOAPIEndpoint:   getEnv("MINIO_API_ENDPOINT", ""),
		MinIOAccessKey:     getEnv("MINIO_ACCESS_KEY", "studsphere"),
		MinIOSecretKey:     getEnv("MINIO_SECRET_KEY", "studsphere123"),
		MinIOBucket:        getEnv("MINIO_BUCKET", "studsphere"),
		MinIOUseSSL:        getEnv("MINIO_USE_SSL", "true") == "true",
		EmbeddingEnabled:    getEnv("EMBEDDING_ENABLED", "false") == "true",
		EmbeddingAPIKey:     getEnv("EMBEDDING_API_KEY", ""),
		EmbeddingBaseURL:    getEnv("EMBEDDING_BASE_URL", "https://api.openai.com/v1"),
		EmbeddingModel:      getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
		VectorDimension:     getEnvInt("VECTOR_DIMENSION", 1536),
		EmbeddingBatchSize:  getEnvInt("EMBEDDING_BATCH_SIZE", 20),
	}
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
