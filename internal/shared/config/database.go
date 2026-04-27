package config

import (
	"fmt"
	"log"
	"strings"

	github_sqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var IsSQLite bool

func ConnectDatabase() {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		AppConfig.DBHost,
		AppConfig.DBUser,
		AppConfig.DBPassword,
		AppConfig.DBName,
		AppConfig.DBPort,
		AppConfig.DBSSLMode,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Println("PostgreSQL connection failed, falling back to local SQLite database...")
		IsSQLite = true

		sqliteDB, sqliteErr := gorm.Open(github_sqlite.Open("studsphere_fallback.db"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})

		if sqliteErr != nil {
			log.Fatal("Failed to connect to both PostgreSQL and SQLite fallback:", sqliteErr)
		}

		DB = sqliteDB
		log.Println("Database connection established using SQLite (studsphere_fallback.db)")
	} else {
		log.Println("Database connection established using PostgreSQL")
		enablePgvector()
	}
}

func enablePgvector() {
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		log.Printf("Warning: Failed to enable pgvector extension: %v. Vector search disabled.", err)
	}
}

func EnsurePgvectorExtension() {
	if IsSQLite {
		return
	}
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			log.Println("pgvector extension not available (permission denied).")
			log.Println("Vector search will be disabled. To enable:")
			log.Println("  Option 1: Run as PostgreSQL superuser:")
			log.Println("    psql -U postgres -d studsphere -c \"CREATE EXTENSION vector;\"")
			log.Println("  Option 2: Grant CREATE privilege:")
			log.Println("    psql -U postgres -d studsphere -c \"GRANT CREATE ON DATABASE studsphere TO studsphere_user;\"")
			log.Println("  Option 3: Make your user a superuser:")
			log.Println("    psql -U postgres -c \"ALTER USER studsphere_user WITH SUPERUSER;\"")
		} else {
			log.Printf("Warning: Could not enable pgvector: %v", err)
		}
	}
}

func IsPGVectorReady() bool {
	if IsSQLite {
		return false
	}
	var extName string
	if err := DB.Raw("SELECT extname FROM pg_extension WHERE extname = 'vector'").Scan(&extName).Error; err != nil {
		return false
	}
	return extName == "vector"
}

func CreateVectorIndexes() {
	if IsSQLite || !IsPGVectorReady() {
		return
	}

	indexes := []struct {
		table    string
		column   string
		indexName string
		lists    int
	}{
		{"colleges", "embedding", "idx_colleges_embedding", 10},
		{"courses", "embedding", "idx_courses_embedding", 10},
		{"exams", "embedding", "idx_exams_embedding", 10},
		{"scholarships", "embedding", "idx_scholarships_embedding", 10},
		{"news", "embedding", "idx_news_embedding", 10},
		{"events", "embedding", "idx_events_embedding", 10},
		{"blogs", "embedding", "idx_blogs_embedding", 10},
	}

	for _, idx := range indexes {
		sql := fmt.Sprintf(
			"CREATE INDEX IF NOT EXISTS %s ON %s USING ivfflat (%s vector_cosine_ops) WITH (lists = %d)",
			idx.indexName, idx.table, idx.column, idx.lists,
		)
		if err := DB.Exec(sql).Error; err != nil {
			log.Printf("Warning: Failed to create vector index on %s.%s: %v", idx.table, idx.column, err)
		}
	}
	log.Println("Vector search indexes created/verified")
}

func IsPostgreSQL() bool {
	return !IsSQLite
}

func GetDB() *gorm.DB {
	return DB
}

func isPostgresDialect(db *gorm.DB) bool {
	name := db.Dialector.Name()
	return strings.Contains(strings.ToLower(name), "postgres")
}
