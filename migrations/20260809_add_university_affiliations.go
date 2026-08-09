package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func AddUniversityAffiliations(db *gorm.DB) error {
	// Step 1: Add new columns if they don't exist
	if err := db.Exec(`
		ALTER TABLE colleges
		ADD COLUMN IF NOT EXISTS university_affiliations JSONB DEFAULT '[]',
		ADD COLUMN IF NOT EXISTS non_university_affiliation TEXT DEFAULT '';
	`).Error; err != nil {
		return err
	}

	// Step 2: Check if university_id column exists before migrating data
	var columnExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'colleges' AND column_name = 'university_id'
		)
	`).Scan(&columnExists).Error
	if err != nil {
		return err
	}

	// Step 3: Migrate data only if university_id column exists
	if columnExists {
		if err := db.Exec(`
			UPDATE colleges
			SET university_affiliations = jsonb_build_array(university_id)
			WHERE university_id IS NOT NULL AND university_id > 0;
		`).Error; err != nil {
			return err
		}

		// Step 4: Drop old column
		if err := db.Exec(`
			ALTER TABLE colleges DROP COLUMN IF EXISTS university_id;
		`).Error; err != nil {
			return err
		}
	}

	logger.Info("University affiliations migration completed")
	return nil
}
