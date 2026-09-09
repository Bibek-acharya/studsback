// internal/notification/ensure_indexes.go
package notification

import (
	"strings"

	"gorm.io/gorm"
)

// EnsurePostgresIndexes creates the composite/partial indexes and CHECK
// constraint that only exist in SQL migrations (cmd/migrate path). A fresh
// `go run ./cmd/server` runs GORM AutoMigrate only, which cannot create
// them — every preferences PUT then 500s with
// `42P10 ON CONFLICT without matching constraint`.
// Statements mirror migrations/20260907-01-notification-preferences.sql and
// migrations/20260907-02-notification-deliveries.sql. Idempotent; no-op on
// SQLite (dev fallback) and when the tables don't exist yet.
func EnsurePostgresIndexes(db *gorm.DB) error {
	if db == nil || db.Dialector == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(db.Dialector.Name()), "sqlite") {
		return nil
	}
	var tableExists bool
	if err := db.Raw(`SELECT to_regclass('notification_preferences') IS NOT NULL`).Scan(&tableExists).Error; err != nil {
		return err
	}
	if !tableExists {
		return nil
	}
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_pref ON notification_preferences (account_type, account_id, pref_key)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_del_notification ON notification_deliveries (delivery_key) WHERE delivery_kind = 'notification' AND channel = 'email' AND status <> 'failed'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_del_digest ON notification_deliveries (delivery_key) WHERE delivery_kind = 'digest' AND channel = 'email' AND status <> 'failed'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_del_anonymous ON notification_deliveries (delivery_key) WHERE delivery_kind = 'anonymous' AND channel = 'email' AND status <> 'failed'`,
		`DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_del_subject'
        AND conrelid = 'notification_deliveries'::regclass
    ) THEN
        ALTER TABLE notification_deliveries
            ADD CONSTRAINT chk_del_subject CHECK (
                (delivery_kind = 'notification' AND notification_id IS NOT NULL AND digest_batch_id IS NULL)
                OR (delivery_kind = 'digest' AND notification_id IS NULL AND digest_batch_id IS NOT NULL)
                OR (delivery_kind = 'anonymous' AND notification_id IS NULL AND digest_batch_id IS NULL)
            );
    END IF;
END $$`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
