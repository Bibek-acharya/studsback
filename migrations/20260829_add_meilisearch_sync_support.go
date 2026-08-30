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

	// Reset existing cursors once when the indexed card metadata schema changes.
	const cardMetadataVersion = "__search_card_metadata_v1__"
	result := db.Exec(`
		INSERT INTO sync_state (table_name, last_synced_at, last_sync_id)
		VALUES (?, '1970-01-01 00:00:00', 0)
		ON CONFLICT (table_name) DO NOTHING
	`, cardMetadataVersion)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		if err := db.Exec("DELETE FROM sync_state WHERE table_name <> ?", cardMetadataVersion).Error; err != nil {
			return err
		}
		logger.Info("Search sync cursors reset for card metadata reindex")
	}

	// Create sync indexes for efficient incremental queries
	tables := []string{"colleges", "courses", "scholarships", "news", "events", "exams", "blogs", "site_pages", "universities", "admission_pages", "institution_users"}
	for _, table := range tables {
		idxName := "idx_" + table + "_sync"
		if err := db.Exec(`
			CREATE INDEX IF NOT EXISTS ` + idxName + ` ON ` + table + ` (updated_at ASC, id ASC)
			WHERE deleted_at IS NULL;
		`).Error; err != nil {
			logger.Warn("Failed to create sync index", "table", table, "error", err)
		}
	}

	logger.Info("Meilisearch sync support migration completed")
	return nil
}
