package migrations

import (
	"studsphere/backend/internal/coins"
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// CreateStudsTokenEntitlements brings the StudsToken entitlement schema up to
// date on the SQL-migration path.
//
// A thin wrapper, for the same reason 20260927_create_coin_ledger.go is one. The
// entitlement tables are created by AutoMigrate over coins.EntitlementModels, and
// everything AutoMigrate cannot create — the UNIQUE on
// (user_id, resource_type, resource_id), the UNIQUE on user_id, every CHECK, the
// partial index behind the derived `used` count, and the foreign key to
// coin_journal — lives in coins.EnsureEntitlementIndexes. Restating that DDL
// here would be a second hand-maintained copy, and the notification module
// carries two and drifted badly enough to cause a production incident
// (internal/notification/ensure_indexes.go: a fresh-boot server where every
// preferences PUT failed with 42P10 ON CONFLICT without matching constraint).
//
// Idempotent: AutoMigrate is, and EnsureEntitlementIndexes uses the same
// DO $$ / CREATE ... IF NOT EXISTS shapes as EnsurePostgresIndexes. main.go calls
// it on every boot in the !config.IsSQLite branch, so this is a no-op on the
// normal server path and only matters for an operator running the migration path
// by hand.
func CreateStudsTokenEntitlements(db *gorm.DB) error {
	if err := db.AutoMigrate(coins.EntitlementModels...); err != nil {
		return err
	}
	if err := coins.EnsureEntitlementIndexes(db); err != nil {
		return err
	}
	logger.Info("StudsToken entitlement migration completed")
	return nil
}
