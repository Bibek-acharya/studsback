package migrations

import (
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func AddMeilisearchSyncSupport(db *gorm.DB) error {
	if config.IsSQLite {
		logger.Info("SQLite detected, skipping Meilisearch sync indexes")
		return nil
	}

	// Create sync_state table for durable cursor persistence
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sync_state (
			table_name VARCHAR(100) PRIMARY KEY,
			last_synced_at TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:00',
			last_sync_id BIGINT NOT NULL DEFAULT 0
		);
	`).Error; err != nil {
		return err
	}

	// Create sync indexes for efficient incremental queries
	tables := []string{"colleges", "courses", "scholarships", "news", "events", "exams", "blogs", "site_pages", "universities", "admission_pages", "institution_users"}
	for _, table := range tables {
		idxName := "idx_" + table + "_sync"
		if err := db.Exec(`
			CREATE INDEX IF NOT EXISTS `+idxName+` ON `+table+` (updated_at ASC, id ASC)
			WHERE deleted_at IS NULL;
		`).Error; err != nil {
			logger.Warn("Failed to create sync index", "table", table, "error", err)
		}
	}

	logger.Info("Meilisearch sync support migration completed")
	return nil
}
