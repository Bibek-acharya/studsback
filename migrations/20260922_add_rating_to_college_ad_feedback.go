package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// AddRatingToCollegeAdFeedback adds the rating column (1-5, 0 = not provided)
// to college_recommendation_feedback. Idempotent: safe to run repeatedly.
func AddRatingToCollegeAdFeedback(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE college_recommendation_feedback
			ADD COLUMN IF NOT EXISTS rating SMALLINT NOT NULL DEFAULT 0
	`).Error; err != nil {
		return err
	}

	logger.Info("College ad feedback rating migration completed")
	return nil
}
