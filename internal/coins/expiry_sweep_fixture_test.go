//go:build coinsintegration

// internal/coins/expiry_sweep_fixture_test.go
//
// The Postgres harness expiry_sweep_pg_test.go drives.

package coins

import (
	"context"
	"encoding/json"
	"fmt"
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

	// settings and service are held so a test can change the ECONOMY — open a gate,
	// enable the unlock endpoint — and see the annotation react. The shipped default
	// has every gate off, so without a handle to write one, the catalogue-access tests
	// could only ever assert the inert case.
	settings *fakeSettings
	service  *Service
	// config is the shared ConfigStore, so a test can invalidate it after a write.
	config *ConfigStore
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
	// The UNLOCK tables — resource_unlock and user_free_allowance — are entitlements
	// rather than ledger rows, so they are not in LedgerModels, and the catalogue-access
	// tests read both. Migrated BEFORE EnsurePostgresIndexes, and the ordering is
	// load-bearing rather than tidiness: that function creates the partial indexes, and
	// EnsureAllowance's ON CONFLICT needs one of them to exist. Migrating afterwards
	// fails with "no unique or exclusion constraint matching the ON CONFLICT
	// specification".
	if err := pool.AutoMigrate(&ResourceUnlock{}, &UserFreeAllowance{}); err != nil {
		t.Fatalf("automigrate unlock models: %v", err)
	}
	if err := EnsurePostgresIndexes(pool); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	// The rollup table is part of this schema, so the harness migrates it too — the
	// alternative is a harness that passes the sweep tests and fails the rollup ones
	// for a reason that has nothing to do with either.
	if err := pool.AutoMigrate(EconomyDailyModels...); err != nil {
		t.Fatalf("automigrate economy daily: %v", err)
	}
	t.Cleanup(func() { pool.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`) })

	repo := NewRepository(pool)
	// The prices are DROPPED to 1 for this harness. The shipped config prices a
	// study-resource unlock at 40 while the profile award is 5, so without this a
	// fixture that grants one award and spends cannot spend — and the rollup tests
	// need both. The economy's actual numbers are not what these tests are about;
	// hardcoding them here would make every figure in the rollup assertions a
	// function of the config, which is exactly the coupling that made them brittle
	// in the first draft.
	cfg := DefaultEconomyConfig()
	cfg.Prices.StudyResource = 1
	settings := newFakeSettings()
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	settings.SetSystemSetting(EconomyConfigSettingKey, string(encoded))
	// ONE ConfigStore shared by the ledger and the service, and this matters rather
	// than being tidiness. main.go passes the same `coinsConfig` to both, so they see
	// one cache and one invalidation. Two stores here would let a test write the config
	// through the service, invalidate the service's cache, and leave the LEDGER
	// reading the previous values — so a gate-open test would silently assert against
	// the value it just replaced.
	configStore := NewConfigStore(settings)
	ledger := NewLedger(repo, configStore)
	// WithRepository, not NewService: HasAccess and RemainingAllowance both need the
	// repository, and a service without one answers "no database handle". main.go
	// uses the same constructor for exactly this reason.
	service := NewServiceWithRepository(repo, configStore, NewVersionStore(pool))

	return &expiryEnv{
		pool: pool, ledger: ledger, sweeper: NewExpirySweeper(repo, ledger),
		settings: settings, service: service, config: configStore,
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

// grantReason makes one grant under a SPECIFIC reason, so a fixture can set the
// referral share rather than leaving every grant a profile award. The reason is the
// only thing that distinguishes a referral award from any other in the rollup, so a
// fixture that cannot vary it cannot test the fraud ratio.
func (e *expiryEnv) grantReason(t *testing.T, userID uint, reason, key string, expiry time.Duration) (GrantResult, uint) {
	t.Helper()
	grant, err := e.ledger.Grant(context.Background(), GrantRequest{
		UserID:         userID,
		ReasonCode:     reason,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("grant %s: %v", key, err)
	}
	if err := e.pool.Model(&CoinLot{}).Where("id = ?", grant.LotID).
		UpdateColumn("expires_at", time.Now().UTC().Add(expiry)).Error; err != nil {
		t.Fatalf("set lot expiry: %v", err)
	}
	return grant, grant.LotID
}

// today is the current UTC day, truncated. Used by the rollup tests so a fixture
// written "now" lands in the row being asserted on rather than in yesterday's.
func (e *expiryEnv) today() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
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

// setGate opens or closes one class's charge gate.
//
// Written through the settings store rather than by mutating the loaded config,
// because that is how an admin does it in production (PUT /admin/coins/economy) and
// a test that used a different path could pass while the real one is broken.
func (e *expiryEnv) setGate(t *testing.T, class string, on bool) error {
	t.Helper()
	cfg, err := e.service.GetEconomyConfig()
	if err != nil {
		return err
	}
	switch class {
	case ResourceTypeStudyResource:
		cfg.Gates.StudyResource = on
	case ResourceTypeVideo:
		cfg.Gates.Video = on
	case ResourceTypeMockTest:
		cfg.Gates.MockTest = on
	default:
		return fmt.Errorf("unknown class %q", class)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := e.settings.SetSystemSetting(EconomyConfigSettingKey, string(encoded)); err != nil {
		return err
	}
	// The store caches, so a write that does not invalidate it would leave the test
	// asserting against the value it just replaced.
	e.config.Invalidate()
	return nil
}

// setUnlockEndpointEnabled opens POST /coins/unlock.
//
// Separate from setGate because it is a DIFFERENT switch, and the document class
// needs both — see TestADocumentGetsNoBlockUntilTheUnlockEndpointIsAlsoEnabled.
func (e *expiryEnv) setUnlockEndpointEnabled(on bool) error {
	cfg, err := e.service.GetEconomyConfig()
	if err != nil {
		return err
	}
	cfg.UnlockEndpointEnabled = on
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := e.settings.SetSystemSetting(EconomyConfigSettingKey, string(encoded)); err != nil {
		return err
	}
	e.config.Invalidate()
	return nil
}
