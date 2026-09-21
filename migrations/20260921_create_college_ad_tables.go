package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func CreateCollegeAdTables(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS college_ad_trending_items (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			kind VARCHAR NOT NULL,
			college_id BIGINT NOT NULL,
			headline VARCHAR DEFAULT '',
			priority INTEGER DEFAULT 0,
			active BOOLEAN DEFAULT TRUE
		);
		CREATE INDEX IF NOT EXISTS idx_college_ad_trending_items_kind ON college_ad_trending_items(kind);
		CREATE INDEX IF NOT EXISTS idx_college_ad_trending_items_deleted_at ON college_ad_trending_items(deleted_at);

		CREATE TABLE IF NOT EXISTS college_recommendation_feedback (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			helpful BOOLEAN NOT NULL,
			reasons TEXT DEFAULT '',
			comment TEXT DEFAULT ''
		);
	`).Error; err != nil {
		return err
	}

	logger.Info("College ad tables migration completed")
	return nil
}
