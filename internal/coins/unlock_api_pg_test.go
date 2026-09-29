//go:build coinsintegration

// internal/coins/unlock_api_pg_test.go
//
// The wallet API against a real PostgreSQL instance. Not mocked, because every
// load-bearing claim in this file is about what the DATABASE did:
//
//   - that a purchase really spends the configured price and really writes the
//     entitlement, in the same transaction. A mock of the ledger proves nothing
//     about whether the two commits are atomic.
//   - that two concurrent requests for the SAME resource with DIFFERENT keys
//     produce exactly one unlock and exactly one debit. This is the claim that
//     the single-transaction composition in coinPurchase exists for, and only a
//     real unique index, a real advisory lock and eight real connections can
//     produce the race that would break it.
//   - that an idempotent replay does not double-charge, and that a key reused
//     with a different payload is a 409 rather than a second charge.
//   - that a re-buy after a revocation succeeds, which is only true because
//     resource_unlock_live_uniq is PARTIAL on revoked_at IS NULL (4fe3cde).
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// This file owns exactly one schema, coins_unlock_api_test, creates it at the
// start of each test and drops it when the test ends. Nothing is ever created in
// the public schema. Every test skips when COINS_TEST_DSN is unset, so
// `go test ./...` needs no database.
//
// The search_path goes in the DSN rather than in a SET, because a SET applies to
// ONE pooled connection and the concurrency test would then have some of its
// goroutines writing to the public schema. unlock_pg_test.go documents the same
// trap for its own schema.
package coins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// unlockAPISchema is the one schema this file owns.
const unlockAPISchema = "coins_unlock_api_test"

// openUnlockAPISchema creates coins_unlock_api_test, migrates the ledger AND the
// entitlement models into it, applies every constraint AutoMigrate cannot
// create, and returns a pool whose every connection is pinned to it.
//
// Both model sets go in because resource_unlock.journal_id has a foreign key to
// coin_journal(id), and both Ensure functions run because the unlock path takes
// the ledger's per-user advisory lock and coin_account has to exist for it to be
// the same lock it is everywhere else.
func openUnlockAPISchema(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping StudsToken wallet API integration test")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if sqlDB, err := admin.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + unlockAPISchema + ` CASCADE`).Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := admin.Exec(`CREATE SCHEMA ` + unlockAPISchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(`DROP SCHEMA IF EXISTS ` + unlockAPISchema + ` CASCADE`).Error
	})

	pool, err := gorm.Open(postgres.Open(dsn+" search_path="+unlockAPISchema), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres pinned to %s: %v", unlockAPISchema, err)
	}
	sqlPool, err := pool.DB()
	if err != nil {
		t.Fatalf("sql handle: %v", err)
	}
	// A connection is held for the whole transaction, so a pool that is too small
	// silently serialises the concurrency test and it stops testing anything. 20
	// covers the widest fan-out here with headroom.
	sqlPool.SetMaxOpenConns(20)
	sqlPool.SetMaxIdleConns(2)
	sqlPool.SetConnMaxLifetime(2 * time.Minute)
	t.Cleanup(func() { _ = sqlPool.Close() })

	models := append(append([]any{}, LedgerModels...), EntitlementModels...)
	if err := pool.AutoMigrate(models...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := EnsurePostgresIndexes(pool); err != nil {
		t.Fatalf("ensure ledger indexes: %v", err)
	}
	if err := EnsureEntitlementIndexes(pool); err != nil {
		t.Fatalf("ensure entitlement indexes: %v", err)
	}
	return pool
}

// walletAPI wires the real thing: a Service, a Ledger and the UnlockAPI over
// them, sharing one repository and one config store exactly as main.go does.
//
// The write path is switched ON here, because the dark behaviour is a config
// default and is asserted in unlock_api_test.go; these tests are about what
// happens when a student actually buys something.
type walletAPI struct {
	api     *UnlockAPI
	repo    *Repository
	service *Service
	ledger  *Ledger
}

func newWalletAPI(t *testing.T, db *gorm.DB, mutate func(*EconomyConfig)) *walletAPI {
	t.Helper()
	cfg := DefaultEconomyConfig()
	cfg.UnlockEndpointEnabled = true
	if mutate != nil {
		mutate(&cfg)
	}
	if err := ValidateEconomyConfig(cfg); err != nil {
		t.Fatalf("the test config does not validate: %v", err)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal test config: %v", err)
	}
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = string(encoded)

	store := NewConfigStore(settings)
	repo := NewRepository(db)
	service := NewServiceWithRepository(repo, store, &fakeVersions{})
	ledger := NewLedger(repo, store)
	return &walletAPI{
		api:     NewUnlockAPI(service, ledger),
		repo:    repo,
		service: service,
		ledger:  ledger,
	}
}

// grantCoins funds a user through the ledger's own Grant, so the fixtures are
// the real thing: a journal, postings, a cached balance and a lot, all of which
// the wallet reads then report.
func grantCoins(t *testing.T, w *walletAPI, userID uint, reason string, key string) int64 {
	t.Helper()
	res, err := w.ledger.Grant(context.Background(), GrantRequest{
		UserID:         userID,
		ReasonCode:     reason,
		IdempotencyKey: key,
		CreatedBy:      "test",
	})
	if err != nil {
		t.Fatalf("grant %s to user %d: %v", reason, userID, err)
	}
	return res.Amount
}

// exhaustDocuments burns the whole document allowance for a user.
//
// It exists because ConsumeAllowance CREATES the starter allowance when there is
// none, so any test that wants the COIN path has to say so explicitly. Without
// it, a test named "a purchase spends coins" would silently be testing the
// allowance instead and would pass while the purchase path was broken.
func exhaustDocuments(t *testing.T, w *walletAPI, userID uint, firstID uint64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := w.service.EnsureAllowance(context.Background(), userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	status, err := w.service.RemainingAllowance(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	for i := int64(0); i < status.Remaining(ResourceTypeStudyResource); i++ {
		if _, err := w.service.ConsumeAllowance(context.Background(), userID,
			ResourceTypeStudyResource, firstID+uint64(i), now); err != nil {
			t.Fatalf("burning document allowance %d of %d: %v", i+1,
				status.Remaining(ResourceTypeStudyResource), err)
		}
	}
	after, err := w.service.RemainingAllowance(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance after burning: %v", err)
	}
	if after.Remaining(ResourceTypeStudyResource) != 0 {
		t.Fatalf("the document allowance is %d, want 0", after.Remaining(ResourceTypeStudyResource))
	}
}

// walletRouter mounts the four routes over a real API with a stub auth
// middleware, so the tests exercise the real handlers, the real status mapping
// and the real JSON rather than calling methods directly.
func walletRouter(w *walletAPI, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterWalletRoutes(router, func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}, w.api)
	return router
}

// postUnlock is one POST /api/v1/coins/unlock with a fresh router per call, so a
// concurrency test does not share a gin engine's state.
type unlockResponse struct {
	Status int
	Body   UnlockResponse
	Error  struct {
		Code string                 `json:"code"`
		Data *InsufficientCoinsData `json:"data"`
	}
	Raw string
}

func postUnlock(t *testing.T, w *walletAPI, userID uint, key, body string) unlockResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	walletRouter(w, userID).ServeHTTP(recorder, request)

	out := unlockResponse{Status: recorder.Code, Raw: recorder.Body.String()}
	// Every response is the {success, message, data, error} envelope, so the
	// payload is always under `data` — for the 200 as much as for the 402. The
	// 200 branch and the error branch decode different halves of it.
	if recorder.Code == http.StatusOK {
		var envelope struct {
			Success bool           `json:"success"`
			Data    UnlockResponse `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode 200 body: %v (%s)", err, out.Raw)
		}
		if !envelope.Success {
			t.Fatalf("a 200 reported success: false (%s)", out.Raw)
		}
		out.Body = envelope.Data
		return out
	}
	var envelope struct {
		Error struct {
			Code string                 `json:"code"`
			Data *InsufficientCoinsData `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error body: %v (%s)", err, out.Raw)
	}
	out.Error = envelope.Error
	return out
}

const studyResourceBody = `{"resource_type":"study_resource","resource_id":812}`

// ── row helpers ──────────────────────────────────────────────────────────────

func apiCount(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count (%s): %v", query, err)
	}
	return n
}

func apiUserAvailable(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	var total int64
	if err := db.Raw(
		`SELECT COALESCE(SUM(b.posted_balance - b.reserved), 0)
		   FROM coin_account_balance b
		   JOIN coin_account a ON a.id = b.account_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'`, userID,
	).Scan(&total).Error; err != nil {
		t.Fatalf("read available for user %d: %v", userID, err)
	}
	return total
}

// apiUnlockRowsFor is scoped to one resource, because a test that burns the
// starter allowance as part of its fixture has other rows that are not its
// subject.
func apiUnlockRowsFor(t *testing.T, db *gorm.DB, userID uint, resourceType string, resourceID uint64) []ResourceUnlock {
	t.Helper()
	var rows []ResourceUnlock
	if err := db.Raw(
		`SELECT `+unlockColumns+` FROM resource_unlock
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ? ORDER BY id`,
		userID, resourceType, resourceID,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read unlocks for user %d %s/%d: %v", userID, resourceType, resourceID, err)
	}
	return rows
}

func apiUnlockRows(t *testing.T, db *gorm.DB, userID uint) []ResourceUnlock {
	t.Helper()
	var rows []ResourceUnlock
	if err := db.Raw(
		`SELECT `+unlockColumns+` FROM resource_unlock WHERE user_id = ? ORDER BY id`, userID,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read unlocks for user %d: %v", userID, err)
	}
	return rows
}

func apiSpendJournals(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return apiCount(t, db,
		`SELECT count(*) FROM coin_journal j
		   WHERE j.scope = 'user' AND j.entry_type = 'SPEND'
		     AND EXISTS (SELECT 1 FROM coin_posting p
		                  JOIN coin_account a ON a.id = p.account_id
		                 WHERE p.journal_id = j.id AND a.owner_user_id = ? AND a.kind = 'USER')`, userID)
}

// ── the happy path ───────────────────────────────────────────────────────────

// A purchase spends real coins, writes the entitlement, and leaves the two
// joined by a journal id. It also proves the price came from config: the body
// names an amount of 1, and the wallet is debited the configured 40.
func TestUnlockSpendsTheConfiguredPriceAndRecordsTheEntitlement(t *testing.T) {
	const userID = 900
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, func(c *EconomyConfig) { c.Prices.StudyResource = 40 })
	granted := grantCoins(t, w, userID, ReasonResourceApproved, "unlock-happy-grant")
	if granted != 80 {
		t.Fatalf("the grant was %d, want the configured 80; the fixture is wrong", granted)
	}
	exhaustDocuments(t, w, userID, 9001)

	// A body that TRIES to name the price. If any part of the path read it, the
	// wallet would be debited 1.
	res := postUnlock(t, w, userID, "unlock-happy-1",
		`{"resource_type":"study_resource","resource_id":812,"amount":1,"price":1,"coins":1}`)

	if res.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", res.Status, res.Raw)
	}
	if !res.Body.Unlocked || res.Body.AlreadyUnlocked {
		t.Errorf("body = %+v, want unlocked with already_unlocked false", res.Body)
	}
	if res.Body.UsedAllowance {
		t.Error("used_allowance = true; this student has no allowance row and the purchase is the point")
	}
	if res.Body.CoinsPaid != 40 {
		t.Errorf("coins_paid = %d, want the CONFIGURED 40 — a body-supplied amount of 1 must never reach the spend", res.Body.CoinsPaid)
	}
	if res.Body.BalanceAfter != 40 {
		t.Errorf("balance_after = %d, want 80 - 40", res.Body.BalanceAfter)
	}
	if len(res.Body.SpentFrom) != 1 || res.Body.SpentFrom[0].Coins != 40 {
		t.Errorf("spent_from = %+v, want one 40-coin lot", res.Body.SpentFrom)
	}

	// The rows agree with the response.
	if got := apiUserAvailable(t, db, userID); got != 40 {
		t.Errorf("wallet = %d, want 40 after a 40-coin purchase from 80", got)
	}
	// Scoped to the resource, not to the user: exhausting the document allowance
	// wrote three rows of its own, and they are not the subject of this test.
	rows := apiUnlockRowsFor(t, db, userID, ResourceTypeStudyResource, 812)
	if len(rows) != 1 {
		t.Fatalf("unlock rows for the resource = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Source != UnlockSourceCoins {
		t.Errorf("source = %q, want %q", row.Source, UnlockSourceCoins)
	}
	if row.CoinsPaid != 40 {
		t.Errorf("stored coins_paid = %d, want the 40 snapshot", row.CoinsPaid)
	}
	if row.JournalID == nil {
		t.Fatal("a COINS unlock with no journal: chk_resource_unlock_source_funding should have refused it")
	}
	// The journal is a real SPEND that really moved the coins, and the unlock
	// points at it — the two are reconcilable, which is the property that makes
	// a support ticket answerable.
	if n := apiSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want 1", n)
	}
	var journalUserLeg int64
	if err := db.Raw(
		`SELECT COALESCE(SUM(p.amount), 0) FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		   JOIN coin_journal j ON j.id = p.journal_id
		  WHERE j.id = ? AND a.owner_user_id = ? AND a.kind = 'USER'`,
		*row.JournalID, userID,
	).Scan(&journalUserLeg).Error; err != nil {
		t.Fatalf("read the journal's user legs: %v", err)
	}
	if journalUserLeg != -40 {
		t.Errorf("the journal moved %d for the user, want -40", journalUserLeg)
	}
}

// ── already owned ────────────────────────────────────────────────────────────

// A second request for the same resource is a 200 with coins_paid 0 and no second
// debit. This is the ordinary mobile retry: §2.3 says a 403 here would render a
// failure for an outcome that succeeded.
func TestAlreadyOwnedIsA200WithNoSecondDebit(t *testing.T) {
	const userID = 901
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-owned-grant")

	// Both requests are forced onto the coin path, so the assertion is about
	// the purchase and not about the allowance.
	exhaustDocuments(t, w, userID, 9001)
	before := apiUserAvailable(t, db, userID)

	first := postUnlock(t, w, userID, "unlock-owned-a", studyResourceBody)
	if first.Status != http.StatusOK {
		t.Fatalf("first status = %d, want 200 (%s)", first.Status, first.Raw)
	}
	if first.Body.CoinsPaid != 40 || first.Body.UsedAllowance {
		t.Fatalf("first body = %+v, want a 40-coin purchase", first.Body)
	}
	afterFirst := apiUserAvailable(t, db, userID)

	second := postUnlock(t, w, userID, "unlock-owned-b", studyResourceBody)
	if second.Status != http.StatusOK {
		t.Fatalf("second status = %d, want 200 (%s)", second.Status, second.Raw)
	}
	if !second.Body.AlreadyUnlocked {
		t.Errorf("already_unlocked = false on a second purchase of one resource")
	}
	if second.Body.CoinsPaid != 0 {
		t.Errorf("coins_paid = %d, want 0 for an already-owned resource", second.Body.CoinsPaid)
	}
	if second.Body.BalanceAfter != afterFirst {
		t.Errorf("balance_after = %d, want the unchanged %d", second.Body.BalanceAfter, afterFirst)
	}
	if len(second.Body.SpentFrom) != 0 {
		t.Errorf("spent_from = %+v, want empty; nothing was spent", second.Body.SpentFrom)
	}
	if got := apiUserAvailable(t, db, userID); got != afterFirst {
		t.Errorf("wallet = %d, want %d — the retry was charged again", got, afterFirst)
	}
	if got := apiSpendJournals(t, db, userID); got != 1 {
		t.Errorf("spend journals = %d, want 1", got)
	}
	if rows := apiUnlockRowsFor(t, db, userID, ResourceTypeStudyResource, 812); len(rows) != 1 {
		t.Errorf("unlock rows for the resource = %d, want 1", len(rows))
	}
	if before == afterFirst {
		t.Error("the first purchase did not debit anything; the fixture is wrong")
	}
}

// ── the allowance path ───────────────────────────────────────────────────────

// A student with ZERO coins and an unused starter unlock succeeds, pays nothing
// and is told used_allowance: true. This is the case that fails if the balance
// is checked before the allowance, and it is the one the allowance exists for.
func TestAllowanceCoversAStudentWithNoCoins(t *testing.T) {
	const userID = 902
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, func(c *EconomyConfig) {
		c.Allowance.DocumentUnlocks = 3
		c.Allowance.ExpiresInDays = 30
	})
	now := time.Now().UTC()
	if _, err := w.service.EnsureAllowance(context.Background(), userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	if got := apiUserAvailable(t, db, userID); got != 0 {
		t.Fatalf("the fixture student holds %d coins; the point is that they hold none", got)
	}

	res := postUnlock(t, w, userID, "unlock-allowance-1", studyResourceBody)
	if res.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", res.Status, res.Raw)
	}
	if !res.Body.UsedAllowance {
		t.Error("used_allowance = false; the allowance covered this")
	}
	if res.Body.CoinsPaid != 0 {
		t.Errorf("coins_paid = %d, want 0", res.Body.CoinsPaid)
	}
	if res.Body.BalanceAfter != 0 {
		t.Errorf("balance_after = %d, want 0", res.Body.BalanceAfter)
	}
	if len(res.Body.SpentFrom) != 0 {
		t.Errorf("spent_from = %+v, want empty — no coins moved", res.Body.SpentFrom)
	}
	// No journal at all: an allowance unlock that names one would claim coins
	// were charged for it.
	if n := apiSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0 for an allowance unlock", n)
	}
	rows := apiUnlockRowsFor(t, db, userID, ResourceTypeStudyResource, 812)
	if len(rows) != 1 {
		t.Fatalf("unlock rows for the resource = %d, want 1", len(rows))
	}
	if rows[0].Source != UnlockSourceAllowance || rows[0].JournalID != nil || rows[0].CoinsPaid != 0 {
		t.Errorf("stored row = %+v, want ALLOWANCE with no journal and coins_paid 0", rows[0])
	}
	// And the allowance is now spent.
	status, err := w.service.RemainingAllowance(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	if got := status.Remaining(ResourceTypeStudyResource); got != 2 {
		t.Errorf("remaining documents = %d, want 2 of 3", got)
	}
}

// ── concurrency ──────────────────────────────────────────────────────────────

// The load-bearing test of this slice. Eight concurrent requests for the SAME
// resource, each with its OWN idempotency key — the double-tap, not the retry —
// must produce exactly one unlock and exactly one debit.
//
// What it is really asserting is the single-transaction composition. Across two
// transactions the loser's spend commits before its unlock insert is refused, so
// the loser is charged for something it does not get and Reverse will not refund
// it. In one transaction the loser re-reads the live unlock under the same
// per-user advisory lock and writes nothing.
func TestConcurrentDuplicateUnlocksProduceOneUnlockAndOneDebit(t *testing.T) {
	const (
		userID     = 903
		resourceID = uint64(812)
		fan        = 8
	)
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	// Enough for every request to be affordable, so the only thing stopping a
	// second debit is the system and not the balance.
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-race-grant")
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-race-grant-2")
	// Exhaust the document allowance so all eight requests are forced onto the
	// coin path. If they were allowed to use it, the winner would be decided by
	// the allowance machinery rather than by the purchase composition.
	exhaustDocuments(t, w, userID, 9031)
	before := apiUserAvailable(t, db, userID)
	price := DefaultEconomyConfig().Prices.StudyResource

	type outcome struct {
		status  int
		paid    int64
		already bool
		raw     string
	}
	results := make([]outcome, fan)
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for i := 0; i < fan; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			// Every goroutine blocks on the same WaitGroup, so they collide as
			// tightly as the connection pool allows rather than trickling in.
			start.Wait()
			res := postUnlock(t, w, userID, fmt.Sprintf("unlock-race-%d", i), studyResourceBody)
			results[i] = outcome{
				status:  res.Status,
				paid:    res.Body.CoinsPaid,
				already: res.Body.AlreadyUnlocked,
				raw:     res.Raw,
			}
		}(i)
	}
	start.Done()
	done.Wait()

	// Every request succeeded: the losers are ALREADY_OWNED 200s, not errors.
	// A 402 or a 500 here would mean the race produced a failure rather than a
	// duplicate answer, which is a different and also unacceptable outcome.
	for i, got := range results {
		if got.status != http.StatusOK {
			t.Errorf("request %d = %d, want 200 for every duplicate of a race (%s)", i, got.status, got.raw)
		}
	}

	// Exactly one unlock, and exactly one debit.
	rows := apiUnlockRows(t, db, userID)
	paid := 0
	for _, row := range rows {
		if row.ResourceID == resourceID {
			paid++
		}
	}
	if paid != 1 {
		t.Errorf("live unlocks for %s/%d = %d, want exactly 1", ResourceTypeStudyResource, resourceID, paid)
	}
	if n := apiSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want exactly 1 — a duplicate was charged", n)
	}
	if after := apiUserAvailable(t, db, userID); after != before-price {
		t.Errorf("wallet = %d, want %d (one debit of %d), so %d coins went missing or were taken twice",
			after, before-price, price, before-after)
	}

	// And exactly one request reports a charge. The others say already_unlocked
	// with coins_paid 0, which is what the frontend needs in order to render a
	// success rather than a failure.
	charged := 0
	for i, got := range results {
		if got.paid > 0 {
			charged++
			if got.paid != price {
				t.Errorf("request %d charged %d, want the configured %d", i, got.paid, price)
			}
		} else if !got.already {
			t.Errorf("request %d neither charged nor reported already_unlocked: %+v", i, got)
		}
	}
	if charged != 1 {
		t.Errorf("requests reporting a charge = %d, want exactly 1", charged)
	}
}

// ── idempotency ──────────────────────────────────────────────────────────────

// The same key and the same payload replays: one journal, one debit, one
// unlock. 02-architecture.md §12.2 — the effect happens exactly once, and the
// caller may not learn the outcome, which is why the retry is the answer rather
// than more locks.
func TestIdempotentReplayDoesNotDoubleCharge(t *testing.T) {
	const userID = 904
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-replay-grant")
	exhaustDocuments(t, w, userID, 9041)
	before := apiUserAvailable(t, db, userID)
	price := int64(40)

	first := postUnlock(t, w, userID, "unlock-replay-key", studyResourceBody)
	if first.Status != http.StatusOK {
		t.Fatalf("first status = %d, want 200 (%s)", first.Status, first.Raw)
	}
	if first.Body.CoinsPaid != price {
		t.Fatalf("first charged %d, want %d", first.Body.CoinsPaid, price)
	}
	afterFirst := apiUserAvailable(t, db, userID)

	second := postUnlock(t, w, userID, "unlock-replay-key", studyResourceBody)
	if second.Status != http.StatusOK {
		t.Fatalf("replay status = %d, want 200 (%s)", second.Status, second.Raw)
	}
	if second.Body.CoinsPaid != 0 {
		t.Errorf("replay charged %d, want 0", second.Body.CoinsPaid)
	}
	if got := apiUserAvailable(t, db, userID); got != afterFirst {
		t.Errorf("wallet = %d, want %d — the replay was charged again", got, afterFirst)
	}
	if n := apiSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want 1", n)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM coin_journal WHERE idempotency_key = ?`, "unlock-replay-key"); n != 1 {
		t.Errorf("journals with the client's key = %d, want 1 — the key was not passed through verbatim", n)
	}
	if before-afterFirst != price {
		t.Errorf("the first purchase moved %d, want %d", before-afterFirst, price)
	}
}

// The same key with a DIFFERENT payload is a conflict, not a replay. 02
// §12.2: "A reused key with a different payload is an error, not a replay… so a
// key collision cannot silently return the wrong result."
func TestIdempotencyKeyReuseWithADifferentPayloadIs409(t *testing.T) {
	const userID = 905
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-reuse-grant")
	exhaustDocuments(t, w, userID, 9051)

	first := postUnlock(t, w, userID, "unlock-reuse-key", studyResourceBody)
	if first.Status != http.StatusOK {
		t.Fatalf("first status = %d, want 200 (%s)", first.Status, first.Raw)
	}
	afterFirst := apiUserAvailable(t, db, userID)

	// Same key, DIFFERENT payload: a different resource in the same class, so
	// the request reaches the spend and actually presents the key. (A different
	// CLASS would be covered by the untouched mock-test allowance and would never
	// reach the ledger, so the test would pass without exercising anything.)
	second := postUnlock(t, w, userID, "unlock-reuse-key",
		`{"resource_type":"study_resource","resource_id":8999}`)
	if second.Status != http.StatusConflict {
		t.Fatalf("reused key = %d, want 409 (%s)", second.Status, second.Raw)
	}
	if second.Error.Code != CodeIdempotencyKeyReuse {
		t.Errorf("code = %q, want %q", second.Error.Code, CodeIdempotencyKeyReuse)
	}
	if got := apiUserAvailable(t, db, userID); got != afterFirst {
		t.Errorf("wallet = %d, want %d — a conflicting key moved coins", got, afterFirst)
	}
	if n := apiSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want 1", n)
	}
	if n := apiCount(t, db,
		`SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 8999`,
		userID); n != 0 {
		t.Errorf("a conflicting key produced %d unlocks for the other resource, want 0", n)
	}
}

// ── the revocation window ────────────────────────────────────────────────────

// A revocation does not permanently block a re-buy. resource_unlock_live_uniq
// is PARTIAL on revoked_at IS NULL (4fe3cde), so the revoked row releases the
// slot and a fresh purchase makes a SECOND row at full price, leaving the
// revocation in place as the record of the first decision.
func TestReBuyAfterARevocationSucceedsAtFullPrice(t *testing.T) {
	const userID = 906
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-revoke-grant")
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-revoke-grant-2")
	now := time.Now().UTC()
	if _, err := w.service.EnsureAllowance(context.Background(), userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	// Exhaust the allowance so the re-buy has to be a purchase.
	exhaustDocuments(t, w, userID, 9061)

	first := postUnlock(t, w, userID, "unlock-revoke-a", studyResourceBody)
	if first.Status != http.StatusOK || first.Body.CoinsPaid != 40 {
		t.Fatalf("first = %d %+v (%s), want a 40-coin purchase", first.Status, first.Body, first.Raw)
	}
	if err := w.service.Revoke(context.Background(), userID, ResourceTypeStudyResource, 812, "chargeback", now); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// The revoked row grants nothing.
	has, err := w.service.HasAccess(context.Background(), userID, ResourceTypeStudyResource, 812)
	if err != nil {
		t.Fatalf("HasAccess: %v", err)
	}
	if has {
		t.Fatal("HasAccess = true after a revocation")
	}

	// And the re-buy is allowed, at full price.
	second := postUnlock(t, w, userID, "unlock-revoke-b", studyResourceBody)
	if second.Status != http.StatusOK {
		t.Fatalf("re-buy = %d, want 200 (%s)", second.Status, second.Raw)
	}
	if second.Body.AlreadyUnlocked {
		t.Error("already_unlocked = true; a revoked row grants no access")
	}
	if second.Body.CoinsPaid != 40 {
		t.Errorf("re-buy charged %d, want the FULL 40 — a revocation is not a free re-grant", second.Body.CoinsPaid)
	}
	// Two rows for the triple: the revoked one and the live one. The history is
	// the record and is not overwritten.
	if n := apiCount(t, db,
		`SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_type = ? AND resource_id = ?`,
		userID, ResourceTypeStudyResource, 812); n != 2 {
		t.Errorf("rows for the triple = %d, want 2 (the revoked row survives)", n)
	}
	if n := apiCount(t, db,
		`SELECT count(*) FROM resource_unlock
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ? AND revoked_at IS NULL`,
		userID, ResourceTypeStudyResource, 812); n != 1 {
		t.Errorf("LIVE rows for the triple = %d, want 1", n)
	}
	if n := apiSpendJournals(t, db, userID); n != 2 {
		t.Errorf("spend journals = %d, want 2 — the re-buy is a second real payment", n)
	}
}

// ── the designed refusals ────────────────────────────────────────────────────

// A student with no coins and no allowance gets the 402 object, with the gap
// computed from the real wallet rather than from a message.
func TestInsufficientCoinsBodyCarriesTheGap(t *testing.T) {
	const userID = 907
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	// A grant so "available" is a real figure rather than a zero. The amount is
	// the CONFIGURING: ReasonProfileComplete pays one instalment, which is 5, and
	// a test that hard-codes 25 would be asserting against a number the ledger
	// never produced.
	granted := grantCoins(t, w, userID, ReasonProfileComplete, "unlock-402-grant")
	exhaustDocuments(t, w, userID, 9071)

	res := postUnlock(t, w, userID, "unlock-402-1", studyResourceBody)
	if res.Status != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (%s)", res.Status, res.Raw)
	}
	if res.Error.Code != CodeInsufficientCoins {
		t.Errorf("code = %q, want %q", res.Error.Code, CodeInsufficientCoins)
	}
	if res.Error.Data == nil {
		t.Fatal("the 402 carries no designed object")
	}
	data := res.Error.Data
	if data.Required != 40 {
		t.Errorf("required = %d, want the configured 40", data.Required)
	}
	if data.Available != granted {
		t.Errorf("available = %d, want the wallet's real %d", data.Available, granted)
	}
	if data.Shortfall != 40-granted {
		t.Errorf("shortfall = %d, want %d", data.Shortfall, 40-granted)
	}
	// A refusal writes nothing: no unlock, no journal, no posting.
	if n := apiCount(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 812`, userID); n != 0 {
		t.Errorf("a refused purchase wrote %d unlock rows", n)
	}
	if n := apiSpendJournals(t, db, userID); n != 0 {
		t.Errorf("a refused purchase wrote %d spend journals", n)
	}
	if got := apiUserAvailable(t, db, userID); got != granted {
		t.Errorf("wallet = %d, want the unchanged %d", got, granted)
	}
}

// A lapsed allowance that no purchase covers is the 423, not the 402, and the
// designed object still travels with it.
func TestLapsedAllowanceThatNoPurchaseCoversIs423(t *testing.T) {
	const userID = 908
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, func(c *EconomyConfig) { c.Allowance.ExpiresInDays = 1 })
	granted := time.Now().UTC().AddDate(0, 0, -2)
	if _, err := w.service.EnsureAllowance(context.Background(), userID, granted); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}

	// The API's clock is real time, and the allowance lapsed two days ago.
	res := postUnlock(t, w, userID, "unlock-423-1", studyResourceBody)
	if res.Status != http.StatusLocked {
		t.Fatalf("status = %d, want 423 (%s)", res.Status, res.Raw)
	}
	if res.Error.Code != CodeAllowanceExpired {
		t.Errorf("code = %q, want %q", res.Error.Code, CodeAllowanceExpired)
	}
	if res.Error.Data == nil || res.Error.Data.Required != 40 {
		t.Errorf("the 423 carries no gap: %+v", res.Error.Data)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 812`, userID); n != 0 {
		t.Errorf("a refused purchase wrote %d unlock rows", n)
	}
}

// The write path is dark, and nothing at all happens when it is off: no
// entitlement, no journal, and a message that says why.
func TestUnlockIs503WhileTheFlagIsOffAndWritesNothing(t *testing.T) {
	const userID = 909
	db := openUnlockAPISchema(t)
	cfg := DefaultEconomyConfig()
	cfg.UnlockEndpointEnabled = false
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = string(encoded)
	repo := NewRepository(db)
	store := NewConfigStore(settings)
	ledger := NewLedger(repo, store)
	service := NewServiceWithRepository(repo, store, &fakeVersions{})
	w := &walletAPI{api: NewUnlockAPI(service, ledger), repo: repo, service: service, ledger: ledger}
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-dark-grant")
	before := apiUserAvailable(t, db, userID)

	res := postUnlock(t, w, userID, "unlock-dark-1", studyResourceBody)
	if res.Status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 while the write path is dark (%s)", res.Status, res.Raw)
	}
	if res.Error.Code != CodeEndpointDisabled {
		t.Errorf("code = %q, want %q", res.Error.Code, CodeEndpointDisabled)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 0 {
		t.Errorf("a 503 wrote %d unlock rows", n)
	}
	if got := apiUserAvailable(t, db, userID); got != before {
		t.Errorf("wallet = %d, want the unchanged %d", got, before)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM coin_journal WHERE idempotency_key = ?`, "unlock-dark-1"); n != 0 {
		t.Error("a 503 still wrote a journal; the gate is not ahead of the write")
	}
}

// ── the notification seam ────────────────────────────────────────────────────

// A notifier failure aborts the whole purchase. §5: the notification is emitted
// inside the same transaction as the state change that caused it, so a rolled
// back unlock must not leave a "you spent 40 coins" inbox row behind — and,
// more importantly here, a purchase must not commit when its notification could
// not be written.
func TestNotifierFailureRollsBackTheWholePurchase(t *testing.T) {
	const userID = 910
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-notify-grant")
	exhaustDocuments(t, w, userID, 9101)
	before := apiUserAvailable(t, db, userID)

	notifier := &txProbeNotifier{fail: true}
	w.api = w.api.WithNotifier(notifier)

	res := postUnlock(t, w, userID, "unlock-notify-1", studyResourceBody)
	if res.Status == http.StatusOK {
		t.Fatalf("a failed notification still returned 200 (%s)", res.Raw)
	}
	if notifier.calls != 1 {
		t.Errorf("the notifier was called %d times, want 1", notifier.calls)
	}
	if !notifier.sawTransaction {
		t.Error("the notifier was not handed the transaction handle; it cannot be atomic with the unlock")
	}
	// Nothing moved: the journal, the postings, the lot, the balance and the
	// entitlement all rolled back with the notification.
	if n := apiSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0 after a rolled-back purchase", n)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 812`, userID); n != 0 {
		t.Errorf("unlock rows = %d, want 0 after a rolled-back purchase", n)
	}
	if got := apiUserAvailable(t, db, userID); got != before {
		t.Errorf("wallet = %d, want the unchanged %d", got, before)
	}
	// And the same key is still usable afterwards, because the first attempt
	// wrote nothing at all — which is the property that makes a client retry
	// safe rather than having to escalate to support.
	notifier.fail = false
	retry := postUnlock(t, w, userID, "unlock-notify-1", studyResourceBody)
	if retry.Status != http.StatusOK {
		t.Fatalf("the retry after a rolled-back attempt = %d, want 200 (%s)", retry.Status, retry.Raw)
	}
	if retry.Body.CoinsPaid != 40 {
		t.Errorf("the retry charged %d, want 40", retry.Body.CoinsPaid)
	}
}

// txProbeNotifier records that it was handed a live transaction and can be made
// to fail, which is the only way to assert the seam is wired INSIDE the
// transaction rather than after it.
type txProbeNotifier struct {
	calls          int
	sawTransaction bool
	fail           bool
}

func (n *txProbeNotifier) NotifyUnlockTx(_ context.Context, tx *gorm.DB, _ uint, event UnlockEvent) error {
	n.calls++
	n.sawTransaction = tx != nil && tx.Statement != nil && tx.Statement.ConnPool != nil
	if n.fail {
		return errors.New("inbox write failed")
	}
	if event.CoinsPaid != 40 {
		return fmt.Errorf("the notifier was told %d coins, want the configured 40", event.CoinsPaid)
	}
	return nil
}

// ── the read endpoints ───────────────────────────────────────────────────────

// The three reads are always available, expose only the caller's own wallet, and
// agree with the rows. The wallet is put through a whole life — two grants, a
// burnt allowance, a purchase — so every field has something real to report.
func TestReadEndpointsReportTheCallersOwnWallet(t *testing.T) {
	const userID = 911
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "unlock-read-grant")
	grantCoins(t, w, userID, ReasonProfileComplete, "unlock-read-grant-2")
	// The whole document allowance is burnt, so the used count is 3 of 3 and a
	// purchase is possible.
	exhaustDocuments(t, w, userID, 9111)
	purchase := postUnlock(t, w, userID, "unlock-read-purchase", studyResourceBody)
	if purchase.Status != http.StatusOK || purchase.Body.CoinsPaid != 40 {
		t.Fatalf("the purchase = %d %+v (%s)", purchase.Status, purchase.Body, purchase.Raw)
	}
	router := walletRouter(w, userID)

	// GET /balance
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/coins/balance", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /balance = %d (%s)", recorder.Code, recorder.Body.String())
	}
	var balance struct {
		Data BalanceResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &balance); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	want := apiUserAvailable(t, db, userID)
	if balance.Data.TotalAvailable != want {
		t.Errorf("total_available = %d, want the projection's %d", balance.Data.TotalAvailable, want)
	}
	if len(balance.Data.Buckets) == 0 {
		t.Error("buckets is empty although the wallet holds a lot")
	}
	if len(balance.Data.SpendOrder) != len(balance.Data.Buckets) {
		t.Error("spend_order and buckets disagree; they are built from the same rows")
	}
	if balance.Data.Allowance == nil {
		t.Fatal("the balance body omits the allowance block")
	}
	if balance.Data.Allowance.DocumentUnlocks != 3 || balance.Data.Allowance.DocumentUsed != 3 {
		t.Errorf("allowance = %+v, want 3 granted and 3 used", *balance.Data.Allowance)
	}
	if balance.Data.Allowance.VideoUnlocks != 1 || balance.Data.Allowance.VideoUsed != 0 {
		t.Errorf("video allowance = %+v, want 1 granted and 0 used", *balance.Data.Allowance)
	}

	// GET /allowance is the same object on its own.
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/coins/allowance", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /allowance = %d (%s)", recorder.Code, recorder.Body.String())
	}
	var allowance struct {
		Data AllowanceDTO `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &allowance); err != nil {
		t.Fatalf("decode allowance: %v", err)
	}
	if allowance.Data.DocumentUsed != 3 {
		t.Errorf("document_used = %d, want 3", allowance.Data.DocumentUsed)
	}

	// GET /transactions, newest first, and it agrees with the journal.
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/coins/transactions", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /transactions = %d (%s)", recorder.Code, recorder.Body.String())
	}
	var history struct {
		Data TransactionsResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode transactions: %v", err)
	}
	// Two grants and the spend. The three burnt allowance unlocks are
	// entitlements, not movements, and are deliberately absent: §2.2 is a
	// history of what moved coins, and an allowance unlock moved none.
	if len(history.Data.Items) != 3 {
		t.Fatalf("items = %d, want the 2 grants and the 1 spend (%s)", len(history.Data.Items), recorder.Body.String())
	}
	if history.Data.Items[0].EntryType != EntrySpend {
		t.Errorf("the newest entry is %q, want the SPEND", history.Data.Items[0].EntryType)
	}
	if history.Data.Items[0].Amount != -40 {
		t.Errorf("the spend's amount = %d, want -40 — the sign is the user's perspective, not the journal's",
			history.Data.Items[0].Amount)
	}
	if history.Data.Items[0].Ref == nil || history.Data.Items[0].Ref.Type != ResourceTypeStudyResource || history.Data.Items[0].Ref.ID != 812 {
		t.Errorf("the spend's ref = %+v, want study_resource 812", history.Data.Items[0].Ref)
	}
	if !strings.HasPrefix(history.Data.Items[0].Description, "Unlocked:") {
		t.Errorf("the spend's description = %q, want it to read as an unlock", history.Data.Items[0].Description)
	}
	if history.Data.Items[1].Ref != nil {
		t.Errorf("a grant with no ref rendered %+v, want null", history.Data.Items[1].Ref)
	}
	if history.Data.Items[0].Amount >= history.Data.Items[1].Amount {
		t.Errorf("items are not newest first: %d then %d", history.Data.Items[0].Amount, history.Data.Items[1].Amount)
	}
	// balance_after is a real running balance: the newest row's is the wallet's
	// current total and it decreases as you walk back in time. Getting this
	// wrong by filtering the window with the cursor would look identical on page
	// one.
	if got := history.Data.Items[0].BalanceAfter; got != want {
		t.Errorf("the newest balance_after = %d, want the wallet's %d", got, want)
	}
	balanceAfterSecond := want - history.Data.Items[0].Amount
	if got := history.Data.Items[1].BalanceAfter; got != balanceAfterSecond {
		t.Errorf("the second row's balance_after = %d, want %d", got, balanceAfterSecond)
	}
	if got := history.Data.Items[2].BalanceAfter; got != balanceAfterSecond-history.Data.Items[1].Amount {
		t.Errorf("the third row's balance_after = %d, want %d", got, balanceAfterSecond-history.Data.Items[1].Amount)
	}
	for _, item := range history.Data.Items {
		if item.JournalID == "" || item.Description == "" || item.CreatedAt.IsZero() {
			t.Errorf("item %+v is missing a journal id, a description or a timestamp", item)
		}
	}
}

// The cursor pages without repeating or skipping, and a spend that lands between
// two pages does not shift the window — which is why this is a keyset and not an
// offset.
func TestTransactionCursorPagesWithoutRepeatingOrSkipping(t *testing.T) {
	const userID = 912
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	for i := 0; i < 5; i++ {
		grantCoins(t, w, userID, ReasonProfileComplete, fmt.Sprintf("unlock-page-grant-%d", i))
	}
	repo := NewRepository(db)

	seen := map[string]bool{}
	cursor := transactionCursor{}
	total := 0
	for page := 0; page < 10; page++ {
		items, next, err := repo.TransactionPage(context.Background(), userID, cursor, 2)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		for _, item := range items {
			if seen[item.JournalID] {
				t.Fatalf("journal %s appeared on two pages", item.JournalID)
			}
			seen[item.JournalID] = true
			total++
		}
		if next == "" {
			break
		}
		decoded, err := decodeTransactionCursor(next)
		if err != nil {
			t.Fatalf("the server issued a cursor it cannot read back: %v", err)
		}
		cursor = decoded
	}
	if total != 5 {
		t.Errorf("paged over %d journals, want 5", total)
	}

	// A new journal between pages does not move the window: the cursor is a key,
	// not an offset, so the rows before it are still exactly the rows before it.
	if _, err := w.ledger.Grant(context.Background(), GrantRequest{
		UserID: userID, ReasonCode: ReasonProfileComplete,
		IdempotencyKey: "unlock-page-grant-late", CreatedBy: "test",
	}); err != nil {
		t.Fatalf("late grant: %v", err)
	}
	cursor = transactionCursor{}
	counted := 0
	for page := 0; page < 10; page++ {
		items, next, err := repo.TransactionPage(context.Background(), userID, cursor, 2)
		if err != nil {
			t.Fatalf("page %d after the late journal: %v", page, err)
		}
		counted += len(items)
		if next == "" {
			break
		}
		cursor, err = decodeTransactionCursor(next)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	if counted != 6 {
		t.Errorf("paged over %d journals after a late arrival, want 6", counted)
	}
}
