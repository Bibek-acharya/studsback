// migrations/20261005_backfill_starter_allowances.go
//
// The starter allowance, for everyone who arrived before the grant hook.
//
// user_free_allowance rows are created two ways: EnsureAllowance, wired into
// the two student-creation paths (internal/auth/starter_allowance.go), and
// ConsumeAllowance's lazy create at first gated unlock. Both miss the same
// set: every account registered before the hook existed, while the gates are
// off. Without this migration those students open the wallet and see no
// starter unlocks at all — the card renders nothing for a never-granted row —
// and the first thing the new economy tells them is a price, with no sign
// they were given anything.
//
// One INSERT...SELECT, not a per-user service loop: the row count is the user
// table, the grant is identical for everyone, and the two invariants the
// service path maintains are maintained here directly:
//
//   - ONCE PER LIFETIME. ON CONFLICT (user_id) DO NOTHING against
//     user_free_allowance_user_uniq — the same constraint EnsureAllowance
//     resolves against. A student who already has a row (granted by the hook
//     between deploy and migration, or by a gated unlock) keeps THEIR row:
//     its granted_at, its expiry and its quantities, exactly as
//     EnsureAllowance's read-back branch would have kept them.
//   - A SNAPSHOT OF THE CONFIG AT GRANT TIME. The quantities and the expiry
//     window come from the live economy config (loaded through the coins
//     package's own ConfigStore, so defaults merge the way the runtime path
//     merges them), not from literals restated here. make_interval(days => n)
//     is exactly allowanceExpiry's AddDate(0, 0, days).
//
// Idempotent by construction: the second run conflicts on every row and
// writes nothing. Postgres-only: the unique index it conflicts against is
// created by EnsureEntitlementIndexes, which only runs on Postgres, and
// make_interval has no SQLite spelling — so on SQLite this is a no-op with a
// log line, matching how every other Postgres-shaped migration here behaves
// on the test dialect.
package migrations

import (
	"studsphere/backend/internal/coins"
	"studsphere/backend/internal/shared/logger"
	"studsphere/backend/internal/system"

	"gorm.io/gorm"
)

// BackfillStarterAllowances grants the configured starter allowance to every
// live student account that does not already hold one.
func BackfillStarterAllowances(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		logger.Info("Skipping starter allowance backfill on non-Postgres dialect", "dialect", db.Dialector.Name())
		return nil
	}

	cfg, err := coins.NewConfigStore(system.NewRepository(db)).Load()
	if err != nil {
		return err
	}

	res := db.Exec(`
		INSERT INTO user_free_allowance
			(user_id, granted_at, expires_at, document_unlocks, video_unlocks, mock_test_unlocks)
		SELECT u.id,
			NOW(),
			NOW() + make_interval(days => ?),
			?, ?, ?
		FROM users u
		WHERE u.deleted_at IS NULL
		ON CONFLICT (user_id) DO NOTHING`,
		cfg.Allowance.ExpiresInDays,
		cfg.Allowance.DocumentUnlocks,
		cfg.Allowance.VideoUnlocks,
		cfg.Allowance.MockTestUnlocks,
	)
	if res.Error != nil {
		return res.Error
	}
	logger.Info("Starter allowance backfill completed", "granted", res.RowsAffected)
	return nil
}
