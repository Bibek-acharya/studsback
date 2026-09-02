package migrations

import (
	"fmt"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func AddEmbeddingMetadata(db *gorm.DB) error {
	if config.IsSQLite {
		logger.Info("SQLite detected, skipping embedding metadata migration")
		return nil
	}

	var tables []string
	if err := db.Raw(`
		SELECT DISTINCT table_name FROM information_schema.columns
		WHERE column_name = 'embedding' AND table_schema = 'public'
	`).Scan(&tables).Error; err != nil {
		return err
	}

	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS embedded_at TIMESTAMPTZ", table)).Error; err != nil {
			return fmt.Errorf("failed to add embedded_at to %s: %w", table, err)
		}
	}

	logger.Info("Embedding metadata migration completed", "tables", len(tables))
	return nil
}
