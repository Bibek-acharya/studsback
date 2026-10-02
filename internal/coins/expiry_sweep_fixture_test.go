//go:build coinsintegration

// internal/coins/expiry_sweep_fixture_test.go
//
// The Postgres harness expiry_sweep_pg_test.go drives.

package coins

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"studsphere/backend/internal/shared/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// expiryEnv is a real ledger over a real Postgres schema, because every property
// under test is index or locking behaviour.
type expiryEnv struct {
	pool    *gorm.DB
	ledger  *Ledger
	sweeper *ExpirySweeper
}

func newExpiryEnv(t *testing.T) *expiryEnv {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping the expiry sweep integration test")
	}
	// The shared logger's lazy Init reads config.AppConfig.GinMode, so a nil there
	// panics rather than failing cleanly. Same guard as ledger_pg_test.go's
	// TestMain, needed here because this file builds its own environment.
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}

	schema := "coins_expiry_sweep"
	pool, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	pool.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`)
	pool.Exec(`CREATE SCHEMA ` + schema)
	pool.Exec(`SET search_path TO ` + schema)
	if err := pool.AutoMigrate(LedgerModels...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := pool.AutoMigrate(RewardGrantModels...); err != nil {
		t.Fatalf("automigrate grants: %v", err)
	}
	if err := EnsurePostgresIndexes(pool); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	t.Cleanup(func() { pool.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`) })

	repo := NewRepository(pool)
	settings := newFakeSettings()
	encoded, err := json.Marshal(DefaultEconomyConfig())
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	settings.SetSystemSetting(EconomyConfigSettingKey, string(encoded))
	ledger := NewLedger(repo, NewConfigStore(settings))

	return &expiryEnv{
		pool:    pool,
		ledger:  ledger,
		sweeper: NewExpirySweeper(repo, ledger),
	}
}

// grantExpiring grants the default profile award and rewinds the lot's expiry, so
// the sweep can be tested without waiting a year for a real expiry.
func (e *expiryEnv) grantExpiring(t *testing.T, userID uint, offset time.Duration) (GrantResult, uint) {
	t.Helper()
	grant, err := e.ledger.Grant(context.Background(), GrantRequest{
		UserID:         userID,
		ReasonCode:     ReasonProfileComplete,
		IdempotencyKey: "expiry-fixture:" + u64str(userID) + ":" + time.Now().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("seed grant for %d: %v", userID, err)
	}
	expires := time.Now().UTC().Add(offset)
	if err := e.pool.Model(&CoinLot{}).Where("id = ?", grant.LotID).
		UpdateColumn("expires_at", expires).Error; err != nil {
		t.Fatalf("set lot expiry: %v", err)
	}
	return grant, grant.LotID
}

func (e *expiryEnv) sweep(ctx context.Context) (ExpireReport, error) {
	return e.sweeper.Sweep(ctx, 0)
}

func (e *expiryEnv) sweepBounded(ctx context.Context, limit int) (ExpireReport, error) {
	return e.sweeper.Sweep(ctx, limit)
}

// available reads the spendable balance the way the wallet does: the cached
// projection, which is the table the student actually sees.
func (e *expiryEnv) available(t *testing.T, userID uint) int64 {
	t.Helper()
	var account CoinAccount
	if err := e.pool.Where("owner_user_id = ? AND bucket = ?", userID, BucketEarned).First(&account).Error; err != nil {
		t.Fatalf("read account: %v", err)
	}
	var balance CoinAccountBalance
	if err := e.pool.Where("account_id = ?", account.ID).First(&balance).Error; err != nil {
		t.Fatalf("read balance: %v", err)
	}
	return balance.PostedBalance - balance.Reserved
}

func (e *expiryEnv) systemBalance(t *testing.T, systemType string) int64 {
	t.Helper()
	var account CoinAccount
	if err := e.pool.Where("system_type = ?", systemType).First(&account).Error; err != nil {
		t.Fatalf("read system account %s: %v", systemType, err)
	}
	var balance CoinAccountBalance
	if err := e.pool.Where("account_id = ?", account.ID).First(&balance).Error; err != nil {
		t.Fatalf("read system balance %s: %v", systemType, err)
	}
	return balance.PostedBalance
}

func u64str(v uint) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}

func (e *expiryEnv) setLotConsumed(t *testing.T, lotID uint, consumed int64) {
	t.Helper()
	if err := e.pool.Model(&CoinLot{}).Where("id = ?", lotID).
		UpdateColumn("consumed", consumed).Error; err != nil {
		t.Fatalf("mark lot %d consumed=%d: %v", lotID, consumed, err)
	}
}
