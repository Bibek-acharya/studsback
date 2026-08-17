package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func AddNewsFeaturedPublished(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE news
		ADD COLUMN IF NOT EXISTS featured BOOLEAN DEFAULT FALSE,
		ADD COLUMN IF NOT EXISTS published BOOLEAN DEFAULT TRUE;
	`).Error; err != nil {
		return err
	}

	logger.Info("News featured/published migration completed")
	return nil
}
