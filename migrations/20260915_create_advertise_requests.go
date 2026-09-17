package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func CreateAdvertiseRequests(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS advertise_requests (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			institution_id BIGINT NOT NULL,
			name VARCHAR NOT NULL,
			designation VARCHAR DEFAULT '',
			contact VARCHAR DEFAULT '',
			email VARCHAR NOT NULL,
			advertise_for VARCHAR NOT NULL,
			status VARCHAR DEFAULT 'pending',
			note TEXT DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_advertise_requests_institution_id ON advertise_requests(institution_id);
		CREATE INDEX IF NOT EXISTS idx_advertise_requests_status ON advertise_requests(status);
		CREATE INDEX IF NOT EXISTS idx_advertise_requests_deleted_at ON advertise_requests(deleted_at);
	`).Error; err != nil {
		return err
	}

	logger.Info("Advertise requests migration completed")
	return nil
}
