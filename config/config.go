package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	JWTSecret          string
	JWTExpiry          string
	GinMode            string
	SuperAdminEmail    string
	SuperAdminPassword string
	SuperAdminRole     string
	SuperAdminFirst    string
	SuperAdminLast     string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string
	DBSSLMode          string
}

var AppConfig *Config

func LoadConfig() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dbHost := getEnv("DB_HOST", "localhost")
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	// Neon requires TLS. Force sslmode=require for neon hostnames to prevent boot-time failures.
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
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
