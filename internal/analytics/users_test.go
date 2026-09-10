// studsback/internal/analytics/users_test.go
package analytics

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; analytics SQL tests require PostgreSQL")
	}
	// Isolate analytics twins in their own database so parallel packages
	// creating minimal same-named twins (without created_at) cannot clash
	// with the full analytics schema, and TRUNCATEs stay analytics-owned.
	parsed, err := url.Parse(dsn)
	if err != nil || strings.TrimPrefix(parsed.Path, "/") == "" {
		t.Fatalf("bad TEST_DATABASE_DSN: %v", err)
	}
	base := strings.TrimPrefix(parsed.Path, "/")
	isolated := base + "_analytics"
	maint := *parsed
	maint.Path = "/postgres"
	admin, err := gorm.Open(postgres.Open(maint.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open maintenance db: %v", err)
	}
	sqlDB, err := admin.DB()
	if err != nil {
		t.Fatalf("maintenance pool: %v", err)
	}
	defer sqlDB.Close()
	if err := admin.Exec(`CREATE DATABASE "` + isolated + `"`).Error; err != nil &&
		!strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "42P04") {
		t.Fatalf("create isolated db: %v", err)
	}
	iso := *parsed
	iso.Path = "/" + isolated
	db, err := gorm.Open(postgres.Open(iso.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS users (id bigserial PRIMARY KEY, role text DEFAULT 'student', status text DEFAULT 'active', deleted_at timestamptz, created_at timestamptz)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS institution_users (id bigserial PRIMARY KEY, status text, deleted_at timestamptz, created_at timestamptz)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS scholarship_provider_users (id bigserial PRIMARY KEY, status text, deleted_at timestamptz, created_at timestamptz)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_sessions (id bigserial PRIMARY KEY, user_id bigint, last_active_at timestamptz)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS education_entries (id bigserial PRIMARY KEY, user_id bigint, created_at timestamptz)`)
	t.Cleanup(func() {
		db.Exec(`TRUNCATE users, institution_users, scholarship_provider_users, user_sessions, education_entries`)
	})
	return db
}

func TestUsersTotalsAndSeries(t *testing.T) {
	db := testDB(t)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	db.Exec(`INSERT INTO users (role, status, created_at) VALUES ('student','active',?), ('student','active',?), ('superadmin','active',?)`, day.Add(-48*time.Hour), day.Add(-24*time.Hour), day.Add(-24*time.Hour))
	var studentIDs []int64
	db.Raw(`SELECT id FROM users WHERE role = 'student' ORDER BY created_at`).Scan(&studentIDs)
	db.Exec(`INSERT INTO institution_users (status, created_at) VALUES ('approved',?), ('pending',?)`, day.Add(-72*time.Hour), day.Add(-12*time.Hour))
	db.Exec(`INSERT INTO scholarship_provider_users (status, created_at) VALUES ('approved',?)`, day.Add(-24*time.Hour))
	now := time.Now().UTC()
	db.Exec(`INSERT INTO user_sessions (user_id, last_active_at) VALUES (?,?), (?,?)`, studentIDs[0], now, studentIDs[0], now.Add(-time.Hour))
	db.Exec(`INSERT INTO user_sessions (user_id, last_active_at) VALUES (?,?)`, studentIDs[1], now.Add(-30*24*time.Hour))
	db.Exec(`INSERT INTO education_entries (user_id, created_at) VALUES (?,?), (?,?)`, studentIDs[0], day.Add(-47*time.Hour), studentIDs[0], day.Add(-46*time.Hour))

	svc := NewService(db, NewUsageTracker(0))
	out, err := svc.Users(day.Add(-72*time.Hour), day.Add(24*time.Hour), "day")
	if err != nil {
		t.Fatalf("users: %v", err)
	}
	if out.Totals.Students != 2 || out.Totals.Institutions != 1 || out.Totals.Providers != 1 {
		t.Fatalf("totals wrong: %+v", out.Totals)
	}
	if out.Totals.PendingInstitutions != 1 {
		t.Fatalf("pending institutions wrong: %+v", out.Totals)
	}
	if out.Totals.Active7d != 1 {
		t.Fatalf("active7d=%d want 1 (duplicate sessions for user 1 must count once)", out.Totals.Active7d)
	}
	// Two students created in window, one activated (two entries for user 1
	// must still count as a single activation): 1/2 = 50%.
	if out.Totals.ActivationPct != 50 {
		t.Fatalf("activation_pct=%v want 50", out.Totals.ActivationPct)
	}
	if len(out.Series) == 0 || out.Series[0].Values["students"] == 0 {
		t.Fatalf("series missing student buckets: %+v", out.Series)
	}
}
