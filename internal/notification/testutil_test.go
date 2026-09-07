// internal/notification/testutil_test.go
package notification

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDB skips (never fails) without a Postgres DSN — the module is
// PostgreSQL-only. Set TEST_DATABASE_DSN to run the SQL integration tests.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; notification SQL integration tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&AccountNotification{}, &NotificationOutbox{}, &NotificationBroadcast{}, &NotificationDedupeLease{}, &NotificationPreference{}, &NotificationDelivery{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	// Apply the shipped SQL migrations so tests verify the real schema —
	// AutoMigrate cannot create the composite/partial indexes below.
	for _, f := range []string{
		"../../migrations/20260903-01-notification-tables.sql",
		"../../migrations/20260903-02-notification-indexes.sql",
		"../../migrations/20260907-01-notification-preferences.sql",
		"../../migrations/20260907-02-notification-deliveries.sql",
	} {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatalf("apply migration %s: %v", f, err)
		}
	}
	t.Cleanup(func() {
		db.Exec(`TRUNCATE account_notifications, notification_outbox, notification_broadcasts, notification_dedupe_leases, notification_preferences, notification_deliveries`)
	})
	return db
}
