package config

import (
	"fmt"
	"log"

	github_sqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

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
		
		// Fallback to SQLite
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
	}
}

func GetDB() *gorm.DB {
	return DB
}
