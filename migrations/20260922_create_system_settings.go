package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// CreateSystemSettings creates the minimal key-value settings table used for
// global site toggles (e.g. find_college_ad_cards). GORM AutoMigrate also
// owns this table via system.SystemSetting; this mirrors the college-ad
// tables migration pattern for the SQL-migration path.
func CreateSystemSettings(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS system_settings (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			key VARCHAR NOT NULL UNIQUE,
			value TEXT DEFAULT ''
		);
	`).Error; err != nil {
		return err
	}

	logger.Info("System settings migration completed")
	return nil
}
