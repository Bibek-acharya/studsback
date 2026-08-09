package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func AddUniversityAffiliations(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE colleges
		ADD COLUMN IF NOT EXISTS university_affiliations JSONB DEFAULT '[]',
		ADD COLUMN IF NOT EXISTS non_university_affiliation TEXT DEFAULT '';
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		UPDATE colleges
		SET university_affiliations = jsonb_build_array(university_id)
		WHERE university_id IS NOT NULL AND university_id > 0;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		ALTER TABLE colleges DROP COLUMN IF EXISTS university_id;
	`).Error; err != nil {
		return err
	}

	logger.Info("University affiliations migration completed")
	return nil
}
