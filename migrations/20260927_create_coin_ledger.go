package migrations

import (
	"studsphere/backend/internal/coins"
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// CreateCoinLedger brings the StudsToken ledger schema up to date on the
// SQL-migration path.
//
// This is a thin wrapper, deliberately. The ledger's tables are created by
// AutoMigrate over coins.LedgerModels, and everything AutoMigrate cannot create
// — the partial indexes, every CHECK constraint, the append-only triggers, and
// the chart-of-accounts seed — lives in coins.EnsurePostgresIndexes. Rather than
// restate that DDL a third time, this migration calls the same two functions.
//
// That is the point. The notification module carries its schema in a .sql file,
// an ensure_indexes.go, and the models, and the two hand-maintained copies drifted
// badly enough to cause a production incident documented at
// internal/notification/ensure_indexes.go: a fresh-boot server where every
// preferences PUT failed with 42P10 ON CONFLICT without matching constraint. A
// third copy of the ledger schema that nothing needs would invite the same class
// of bug.
//
// Idempotent: AutoMigrate and EnsurePostgresIndexes are both safe to re-run, and
// EnsurePostgresIndexes is already invoked on every boot from main.go, so this is
// a no-op in the normal server path and only matters for an operator running the
// migration path by hand.
func CreateCoinLedger(db *gorm.DB) error {
	if err := db.AutoMigrate(coins.LedgerModels...); err != nil {
		return err
	}
	// reward_grant carries the once-only award ledger, not double entry, so it is
	// a separate slice — see coins.RewardGrantModels for why lumping it in with
	// the five ledger tables would put an award table on the reconciliation path.
	if err := db.AutoMigrate(coins.RewardGrantModels...); err != nil {
		return err
	}
	if err := coins.EnsurePostgresIndexes(db); err != nil {
		return err
	}
	logger.Info("Coin ledger migration completed")
	return nil
}
