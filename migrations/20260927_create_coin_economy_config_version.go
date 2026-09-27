package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// CreateCoinEconomyConfigVersion creates the audit table that records every
// coin-economy config write: the actor, the timestamp, and the full previous
// and new JSON objects. GORM AutoMigrate already owns this table via
// coins.ConfigVersion; this mirrors the documented-shape pattern of the other
// table migrations so a fresh SQL-only bootstrap works.
//
// created_at is a plain btree index, not a partial or expression index, so
// AutoMigrate can create it and no EnsurePostgresIndexes helper is required
// (docs/coin-system/02-architecture.md §9).
//
// Idempotent: IF NOT EXISTS throughout, and additive only — no column is
// dropped and no existing row is rewritten.
func CreateCoinEconomyConfigVersion(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS coin_economy_config_version (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			previous_json TEXT DEFAULT '',
			new_json TEXT DEFAULT '',
			changed_by_user_id BIGINT NOT NULL DEFAULT 0,
			changed_by VARCHAR(64) DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_coin_economy_config_version_created_at ON coin_economy_config_version(created_at)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	logger.Info("Coin economy config version migration completed")
	return nil
}
