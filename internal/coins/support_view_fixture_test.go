//go:build coinsintegration

// internal/coins/support_view_fixture_test.go
//
// The Postgres harness for the support view.

package coins

import (
	"context"
	"encoding/json"
	"os"
	"studsphere/backend/internal/shared/config"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// supportEnv is a real ledger over a real Postgres schema: the cached-versus-computed
// comparison the endpoint exists for is meaningless without both tables present.
type supportEnv struct {
	pool    *gorm.DB
	ledger  *Ledger
	sweeper *ExpirySweeper
}

func newSupportEnv(t *testing.T) *supportEnv {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping the support view integration test")
	}
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}

	schema := "coins_support_view"
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

	return &supportEnv{pool: pool, ledger: ledger, sweeper: NewExpirySweeper(repo, ledger)}
}

// grant makes one grant and returns its result and lot id.
func (e *supportEnv) grant(t *testing.T, userID uint, key, reason, refTypeName string, refIDValue uint) (GrantResult, uint) {
	t.Helper()
	grant, err := e.ledger.Grant(context.Background(), GrantRequest{
		UserID:         userID,
		ReasonCode:     reason,
		IdempotencyKey: key,
		RefType:        refType(refTypeName),
		RefID:          refID(refIDValue),
	})
	if err != nil {
		t.Fatalf("grant %s: %v", key, err)
	}
	if grant.LotID == 0 {
		t.Fatalf("grant %s returned LotID 0 — CreateLot is not populating the id", key)
	}
	return grant, grant.LotID
}

func refType(s string) *string { return &s }

func refID(v uint) *uint64 {
	id := uint64(v)
	return &id
}
