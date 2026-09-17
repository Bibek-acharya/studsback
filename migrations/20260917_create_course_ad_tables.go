package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

func CreateCourseAdTables(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS course_ad_cards (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			position VARCHAR DEFAULT '',
			course_id BIGINT NOT NULL,
			institution_id BIGINT,
			subtitle TEXT DEFAULT '',
			active BOOLEAN DEFAULT TRUE,
			priority INTEGER DEFAULT 0,
			clicks INTEGER DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_course_ad_cards_position ON course_ad_cards(position);
		CREATE INDEX IF NOT EXISTS idx_course_ad_cards_deleted_at ON course_ad_cards(deleted_at);

		CREATE TABLE IF NOT EXISTS course_ad_card_institutions (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			card_id BIGINT NOT NULL,
			institution_id BIGINT NOT NULL,
			order_index INTEGER DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_course_ad_card_institutions_card_id ON course_ad_card_institutions(card_id);

		CREATE TABLE IF NOT EXISTS course_ad_card_mou_companies (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			card_id BIGINT NOT NULL,
			name VARCHAR NOT NULL,
			logo_url TEXT DEFAULT '',
			company_url TEXT DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_course_ad_card_mou_companies_card_id ON course_ad_card_mou_companies(card_id);
	`).Error; err != nil {
		return err
	}

	logger.Info("Course ad tables migration completed")
	return nil
}
