//go:build coinsintegration

// internal/coins/resource_award_pg_test.go
//
// The §5.3 upload award against Postgres, because the property that matters is
// enforced by the DATABASE and SQLite does not have it.
//
// What SQLite cannot test here: the UNIQUE (scope, idempotency_key) on
// coin_journal, the per-user advisory lock, and coin_account's balance
// arithmetic. The idempotency claim in resource_award.go rests entirely on that
// unique index, so testing it on SQLite would be testing a different database's
// behaviour and calling it a pass.
//
// COINS_TEST_DSN gates the whole file. Every test skips cleanly when unset.

package coins

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"studsphere/backend/internal/shared/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// THE load-bearing test. Approving the same resource twice must pay once, and the
// mechanism is the unique index rather than any check in Go — so this asserts on
// the ledger's own Replayed flag and on the balance, not on a call count.
func TestPublishingTheSameResourceTwicePaysOnce(t *testing.T) {
	env := newResourceAwardEnv(t)
	ctx := context.Background()

	first, err := GrantResourceApproved(ctx, env.ledger, env.userID, env.resourceID, "Notes")
	if err != nil {
		t.Fatalf("first approval: %v", err)
	}
	if first.Amount != 80 {
		t.Errorf("the award paid %d, want 80", first.Amount)
	}
	if first.Replayed {
		t.Error("the first approval was reported as a replay")
	}

	second, err := GrantResourceApproved(ctx, env.ledger, env.userID, env.resourceID, "Notes")
	if err != nil {
		t.Fatalf("a second approval returned %v; it must be a replay, not an error", err)
	}
	if !second.Replayed {
		t.Error("the second approval was NOT reported as a replay")
	}
	if second.JournalID != first.JournalID {
		t.Errorf("the replay returned journal %s, want the original %s", second.JournalID, first.JournalID)
	}
	if second.Amount != first.Amount {
		t.Errorf("the replay reported %d coins, want the original %d — the amount must be read from the journal, not recomputed",
			second.Amount, first.Amount)
	}

	// And the balance moved exactly once, which is the assertion that would catch a
	// double pay even if both calls claimed to be first.
	if got := env.available(t, env.userID); got != 80 {
		t.Errorf("balance after two approvals = %d, want 80", got)
	}
	var journals int64
	env.pool.Model(&CoinJournal{}).
		Where("reason_code = ?", ReasonResourceApproved).
		Count(&journals)
	if journals != 1 {
		t.Errorf("%d journals for one published resource, want 1", journals)
	}
}

// A student who publishes two resources is paid twice. This is the case a
// user-keyed idempotency key would silently swallow, which is why the key is
// built from the resource id.
func TestTwoResourcesFromOneStudentPayTwice(t *testing.T) {
	env := newResourceAwardEnv(t)
	ctx := context.Background()

	first, err := GrantResourceApproved(ctx, env.ledger, env.userID, env.resourceID, "Notes")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := GrantResourceApproved(ctx, env.ledger, env.userID, env.resourceID+1, "Past papers")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Replayed {
		t.Error("a second, DIFFERENT resource was reported as a replay; the key is user-keyed when it must be resource-keyed")
	}
	if second.JournalID == first.JournalID {
		t.Error("two resources produced the same journal")
	}
	if got := env.available(t, env.userID); got != 160 {
		t.Errorf("balance = %d, want 160 (two 80-coin awards)", got)
	}
}

// The journal must name the resource, so the wallet can answer "why did I get 80
// coins" — which is 09's whole support question. A grant with no ref is an award
// a student cannot explain.
func TestTheAwardJournalNamesTheResourceItWasFor(t *testing.T) {
	env := newResourceAwardEnv(t)

	grant, err := GrantResourceApproved(context.Background(), env.ledger, env.userID, env.resourceID, "Organic Chemistry Notes")
	if err != nil {
		t.Fatalf("grant: %v", err)
	}

	var journal CoinJournal
	if err := env.pool.Where("id = ?", grant.JournalID).First(&journal).Error; err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if journal.ReasonCode != ReasonResourceApproved {
		t.Errorf("reason = %q, want %q", journal.ReasonCode, ReasonResourceApproved)
	}
	if journal.RefType == nil || *journal.RefType != RefStudyResource {
		t.Errorf("ref_type = %v, want %q", journal.RefType, RefStudyResource)
	}
	if journal.RefID == nil || *journal.RefID != uint64(env.resourceID) {
		t.Errorf("ref_id = %v, want %d", journal.RefID, env.resourceID)
	}
	// One journal, two postings, netting to zero: the student up, the faucet down.
	var legs int64
	var total int64
	env.pool.Model(&CoinPosting{}).Where("journal_id = ?", journal.ID).Find(&PostingLeg{})
	env.pool.Model(&CoinPosting{}).Where("journal_id = ?", journal.ID).Count(&legs)
	env.pool.Model(&CoinPosting{}).Where("journal_id = ?", journal.ID).Select("COALESCE(SUM(amount), 0)").Scan(&total)
	if legs != 2 {
		t.Errorf("%d postings, want 2", legs)
	}
	if total != 0 {
		t.Errorf("the postings sum to %d; a grant must net to zero", total)
	}
	// And a lot exists, so the coins actually expire rather than being permanent.
	var lots int64
	env.pool.Model(&CoinLot{}).Where("journal_id = ?", journal.ID).Count(&lots)
	if lots != 1 {
		t.Errorf("%d lots for the grant, want 1", lots)
	}
}

// A student with no account must not be created by an approval. UploadedBy = 0 is
// a data defect, and paying it would mint an account for user zero.
func TestAnAwardToAZeroUserRefusesAndWritesNothing(t *testing.T) {
	env := newResourceAwardEnv(t)

	if _, err := GrantResourceApproved(context.Background(), env.ledger, 0, env.resourceID, "Notes"); err == nil {
		t.Fatal("a zero user id was accepted")
	}
	var journals int64
	env.pool.Model(&CoinJournal{}).Count(&journals)
	if journals != 0 {
		t.Errorf("a refused grant wrote %d journals, want 0", journals)
	}
}

// resourceAwardEnv is a real ledger over a real Postgres schema, because the
// properties under test are index and lock behaviour.
type resourceAwardEnv struct {
	pool       *gorm.DB
	ledger     *Ledger
	userID     uint
	resourceID uint
}

func newResourceAwardEnv(t *testing.T) *resourceAwardEnv {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping the §5.3 upload award integration test")
	}
	// config.AppConfig is read by the shared logger's lazy Init, and a nil deref
	// there panics rather than failing a test cleanly. Same fix as
	// ledger_pg_test.go's TestMain, applied here because this file builds its own
	// environment.
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}

	schema := "coins_resource_award"
	pool, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	pool.Exec(`CREATE SCHEMA IF NOT EXISTS ` + schema)
	pool.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`)
	pool.Exec(`CREATE SCHEMA ` + schema)
	pool.Exec(`SET search_path TO ` + schema)
	if err := pool.AutoMigrate(LedgerModels...); err != nil {
		t.Fatalf("automigrate ledger: %v", err)
	}
	if err := pool.AutoMigrate(RewardGrantModels...); err != nil {
		t.Fatalf("automigrate grants: %v", err)
	}
	if err := EnsurePostgresIndexes(pool); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	t.Cleanup(func() { pool.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`) })

	repo := NewRepository(pool)
	// The config store is built over the same in-memory fake the rest of this
	// package's tests use, with the DEFAULT config written into it — so the award
	// amount under test is the shipped default (80) rather than a number this file
	// invented.
	settings := newFakeSettings()
	encoded, err := json.Marshal(DefaultEconomyConfig())
	if err != nil {
		t.Fatalf("encode default config: %v", err)
	}
	if err := settings.SetSystemSetting(EconomyConfigSettingKey, string(encoded)); err != nil {
		t.Fatalf("store config: %v", err)
	}
	// The account is created by the grant itself — Ledger.Grant calls
	// tx.EnsureUserAccount — so there is deliberately no seed step here. A test
	// that pre-created the account would exercise a different branch than the one
	// a real first-ever approval takes.
	ledger := NewLedger(repo, NewConfigStore(settings))
	return &resourceAwardEnv{pool: pool, ledger: ledger, userID: 4242, resourceID: 913}
}

// available reads the student's spendable balance the way the wallet does: the
// CACHED projection, not SUM over postings. That is deliberate — this is the table
// the wallet renders, so a test asserting on it is asserting on what a student
// actually sees.
func (e *resourceAwardEnv) available(t *testing.T, userID uint) int64 {
	t.Helper()
	var account CoinAccount
	if err := e.pool.Where("owner_user_id = ? AND bucket = ?", userID, BucketEarned).First(&account).Error; err != nil {
		t.Fatalf("read account: %v", err)
	}
	var balance CoinAccountBalance
	if err := e.pool.Where("account_id = ?", account.ID).First(&balance).Error; err != nil {
		t.Fatalf("read cached balance: %v", err)
	}
	return balance.PostedBalance - balance.Reserved
}

var _ = sqlite.Open // the sqlite import is here only for the shared test helper set
