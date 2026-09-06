// cmd/migrate/main.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_DSN required")
		os.Exit(1)
	}
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name text PRIMARY KEY, applied_at timestamptz DEFAULT now())`).Error; err != nil {
		fmt.Fprintln(os.Stderr, "tracking table:", err)
		os.Exit(1)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "glob:", err)
		os.Exit(1)
	}
	sort.Strings(files)
	for _, f := range files {
		name := filepath.Base(f)
		var n int64
		db.Table("schema_migrations").Where("name = ?", name).Count(&n)
		if n > 0 {
			continue
		}
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(1)
		}
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			fmt.Fprintf(os.Stderr, "apply %s: %v\n", name, err)
			os.Exit(1)
		}
		if err := db.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name).Error; err != nil {
			fmt.Fprintln(os.Stderr, "record:", err)
			os.Exit(1)
		}
		fmt.Println("applied", name)
	}
}
