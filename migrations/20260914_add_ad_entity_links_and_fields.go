package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func AddAdEntityLinksAndFields(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE ads
		ADD COLUMN IF NOT EXISTS college_id BIGINT,
		ADD COLUMN IF NOT EXISTS course_id BIGINT,
		ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '',
		ADD COLUMN IF NOT EXISTS accent VARCHAR(7) DEFAULT '';
	`).Error; err != nil {
		return err
	}

	logger.Info("Ad entity links and fields migration completed")
	return nil
}
