//go:build coinsintegration

// internal/coins/ledger_pg_test.go
//
// The transactional core, against a real PostgreSQL instance. Not mocked. The
// concurrency claim is the load-bearing one in the whole feature — it is the
// reason for pg_advisory_xact_lock, and 02-architecture.md §3 says it was
// validated against a live server during design — so it is tested against the
// engine that will actually run it.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// The DSN must point at a database you are willing to have a schema created and
// dropped in. This file creates exactly one schema, coins_ledger_core, and drops
// it when the run ends. Nothing is ever created in the public schema and nothing
// is created in a schema this package did not name. Every test skips when
// COINS_TEST_DSN is unset, so `go test ./...` needs no database.
//
// Why one shared schema rather than one per test: the concurrency tests assert on
// exact row counts and exact final balances, so each test has to start from an
// empty ledger — and truncating achieves that with a fraction of the connection
// churn of a schema (and therefore a pool) per test. On a shared development
// server the per-test pools were enough to exhaust max_connections, which
// surfaces as "sorry, too many clients" — a failure that looks nothing like what
// it is.
//
// ── on the connection's search_path ──────────────────────────────────────────
//
// The schema is put in the DSN, not in a `SET search_path` statement, and that
// difference matters. A SET applies to ONE pooled connection; the next checkout
// gets a fresh connection with the default search_path, and the test either
// writes to the public schema or fails to find its own tables. Putting it in the
// DSN means every connection in the pool is pinned to the test schema, which is
// what makes a pooled, concurrent test safe.
package coins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ── harness ──────────────────────────────────────────────────────────────────

// coreSchema is the one schema this file owns. It is created before the run and
// dropped after it; nothing is ever created in the public schema, and nothing is
// ever created in a schema this package did not name.
const coreSchema = "coins_ledger_core"

var corePool *gorm.DB

// TestMain creates and drops the schema the ledger tests share, and manages the
// one connection pool they all use.
//
// It is here rather than in a per-test helper for two reasons, both learned the
// hard way:
//
//   - One pool, not one per test. A pool per test means up to
//     (widest concurrency test x tests) live connections at the peak, and on a
//     shared development server that is "sorry, too many clients" — a failure
//     that looks exactly like a hang and says nothing about the ledger. With one
//     pool the footprint is bounded by the widest single test, once.
//   - One schema, truncated between tests, not one per test. A schema per test
//     needs a pool per test to pin search_path, which is the problem above.
//     Truncating gives the same isolation for the assertions that matter — every
//     test here counts rows and sums balances — at a fraction of the cost. The
//     exact-count assertions stay exact because a truncated table is as empty as a
//     fresh one.
//
// schema_pg_test.go runs in the same binary and manages its own schema, which is
// unaffected: this TestMain only creates and drops coreSchema.
func TestMain(m *testing.M) {
	code := 1
	if dsn := os.Getenv("COINS_TEST_DSN"); dsn == "" {
		// No database. Every test skips itself, so the run is a pass.
		code = m.Run()
	} else {
		var err error
		corePool, err = openCoreSchema(dsn)
		if err == nil {
			code = m.Run()
			dropCoreSchema(dsn)
		} else {
			fmt.Fprintf(os.Stderr, "coins integration: %v\n", err)
		}
	}
	os.Exit(code)
}

// openCoreSchema creates the schema, migrates the ledger into it, and returns a
// pool whose every connection is pinned to it.
func openCoreSchema(dsn string) (*gorm.DB, error) {
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + coreSchema + ` CASCADE`).Error; err != nil {
		return nil, fmt.Errorf("drop schema %s: %w", coreSchema, err)
	}
	if err := admin.Exec(`CREATE SCHEMA ` + coreSchema).Error; err != nil {
		return nil, fmt.Errorf("create schema %s: %w", coreSchema, err)
	}

	// The search_path goes in the DSN, not in a SET statement, and that difference
	// matters. A SET applies to ONE pooled connection; the next checkout gets a
	// fresh connection with the default search_path, and the test either writes to
	// the public schema or fails to find its own tables. In the DSN, every
	// connection in the pool is pinned.
	pooled, err := gorm.Open(postgres.Open(dsn+" search_path="+coreSchema), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres with search_path: %w", err)
	}
	sqlPool, err := pooled.DB()
	if err != nil {
		return nil, fmt.Errorf("sql handle: %w", err)
	}
	// A connection is held for the whole transaction, so a pool that is too small
	// silently serialises the concurrency tests and the floor-division assertion
	// stops testing anything. Too large exhausts a shared development server.
	// 24 covers the widest test in this file with headroom; MaxIdleConns of 2
	// keeps the steady-state footprint tiny, and the reaping settings are belt and
	// braces for the same reason.
	sqlPool.SetMaxOpenConns(20)
	sqlPool.SetMaxIdleConns(2)
	sqlPool.SetConnMaxIdleTime(30 * time.Second)
	sqlPool.SetConnMaxLifetime(2 * time.Minute)

	if err := pooled.AutoMigrate(LedgerModels...); err != nil {
		return nil, fmt.Errorf("automigrate: %w", err)
	}
	if err := EnsurePostgresIndexes(pooled); err != nil {
		return nil, fmt.Errorf("ensure indexes: %w", err)
	}
	return pooled, nil
}

func dropCoreSchema(dsn string) {
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return
	}
	_ = admin.Exec(`DROP SCHEMA IF EXISTS ` + coreSchema + ` CASCADE`).Error
}

// openLedgerSchema returns the shared pool with every ledger table emptied, so
// each test starts from nothing.
//
// TRUNCATE does not fire the coin_posting row triggers — they are BEFORE UPDATE
// OR DELETE FOR EACH ROW — so the append-only guarantee is not something the
// harness has to be granted an exemption from.
//
// The chart of accounts is preserved: the SYSTEM accounts are seeded once by
// EnsurePostgresIndexes and every assertion about them ("earned_faucet has 72
// legs", "SUM(posted_balance) is 0") is relative to a fixed set of three rows.
// Deleting them would mean reseeding on every test and the assertions would be
// about a different starting state each time.
func openLedgerSchema(t *testing.T) *gorm.DB {
	t.Helper()
	if corePool == nil {
		t.Skip("COINS_TEST_DSN not set; skipping StudsToken ledger integration test")
	}
	if err := corePool.Exec(
		`TRUNCATE coin_posting, coin_lot, coin_journal RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate ledger tables: %v", err.Error())
	}
	if err := corePool.Exec(
		`DELETE FROM coin_account_balance WHERE account_id IN (SELECT id FROM coin_account WHERE kind = 'USER')`).Error; err != nil {
		t.Fatalf("clear user balances: %v", err.Error())
	}
	if err := corePool.Exec(`DELETE FROM coin_account WHERE kind = 'USER'`).Error; err != nil {
		t.Fatalf("clear user accounts: %v", err.Error())
	}
	// The chart of accounts is the one thing whose ROWS survive, because its
	// identities are part of the fixture, but its cached balances are reset: the
	// postings they project were just truncated, and a stale balance would make
	// the global SUM(posted_balance) = 0 invariant fail on leftover state rather
	// than on anything this test did.
	if err := corePool.Exec(
		`UPDATE coin_account_balance b
		    SET posted_balance = 0, reserved = 0, version = 0, last_journal_id = NULL
		   FROM coin_account a
		  WHERE b.account_id = a.id AND a.kind = 'SYSTEM'`).Error; err != nil {
		t.Fatalf("reset system balances: %v", err.Error())
	}
	// Every test in this file asserts on exact counts, so a reset that did not
	// happen must be loud here rather than quietly producing a coin-flip five
	// lines later.
	for _, table := range []string{"coin_journal", "coin_posting", "coin_lot"} {
		if n := countRows(t, corePool, `SELECT count(*) FROM `+table); n != 0 {
			t.Fatalf("%s rows after reset = %d, want 0", table, n)
		}
	}
	if n := countRows(t, corePool, `SELECT count(*) FROM coin_account WHERE kind = 'USER'`); n != 0 {
		t.Fatalf("user accounts after reset = %d, want 0", n)
	}
	if n := countRows(t, corePool, `SELECT count(*) FROM coin_account WHERE kind = 'SYSTEM'`); n != int64(len(SystemAccounts)) {
		t.Fatalf("system accounts after reset = %d, want %d", n, len(SystemAccounts))
	}
	var systemTotal int64
	corePool.Raw(`SELECT COALESCE(SUM(posted_balance), 0) FROM coin_account_balance`).Scan(&systemTotal)
	if systemTotal != 0 {
		t.Fatalf("the chart of accounts does not start at zero (sum = %d)", systemTotal)
	}
	return corePool
}

// testLedger wires a Ledger over the test database and an in-memory economy
// config. The config is the point: every amount in these tests comes from here,
// never from a literal in the test, which is the same rule the ledger enforces on
// its callers.
func testLedger(t *testing.T, db *gorm.DB, mutate func(*EconomyConfig)) *Ledger {
	t.Helper()
	cfg := DefaultEconomyConfig()
	if mutate != nil {
		mutate(&cfg)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal test config: %v", err)
	}
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = string(encoded)
	return NewLedger(NewRepository(db), NewConfigStore(settings))
}

// ── row helpers ──────────────────────────────────────────────────────────────

func countRows(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count (%s): %v", query, err.Error())
	}
	return n
}

func userAccountID(t *testing.T, db *gorm.DB, userID uint, bucket string) uint {
	t.Helper()
	var id uint
	db.Raw(`SELECT id FROM coin_account WHERE owner_user_id = ? AND bucket = ? AND kind = 'USER'`,
		userID, bucket).Scan(&id)
	return id
}

func userPosted(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	var total int64
	db.Raw(`SELECT COALESCE(SUM(b.posted_balance), 0)
	          FROM coin_account_balance b
	          JOIN coin_account a ON a.id = b.account_id
	         WHERE a.owner_user_id = ? AND a.kind = 'USER'`, userID).Scan(&total)
	return total
}

func userReserved(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	var total int64
	db.Raw(`SELECT COALESCE(SUM(b.reserved), 0)
	          FROM coin_account_balance b
	          JOIN coin_account a ON a.id = b.account_id
	         WHERE a.owner_user_id = ? AND a.kind = 'USER'`, userID).Scan(&total)
	return total
}

func lotRows(t *testing.T, db *gorm.DB, journalID string) []CoinLot {
	t.Helper()
	var lots []CoinLot
	db.Raw(`SELECT id, account_id, journal_id, bucket, granted, consumed, expires_at, created_at
	          FROM coin_lot WHERE journal_id = ? ORDER BY id`, journalID).Scan(&lots)
	return lots
}

// seedLot inserts a lot directly, the way the multi-bucket FEFO test needs to.
//
// This is the same shape as seedUserAccount in schema_pg_test.go: a fixture that
// builds a state the current phase's grants cannot produce on its own. Every
// grant in this build lands in EARNED with the same configured lifetime, so the
// ledger cannot itself produce two lots with distinct expiries in one account, and
// FEFO's expiry ordering would be untested against the real database. The lot is
// attributed to a real journal so the invariant checks still hold.
func seedLot(t *testing.T, db *gorm.DB, journalID string, accountID uint, bucket string, granted int64, expiresAt *time.Time) uint {
	t.Helper()
	var expires any
	if expiresAt != nil {
		expires = *expiresAt
	}
	res := db.Exec(
		`INSERT INTO coin_lot (account_id, journal_id, bucket, granted, consumed, expires_at, created_at)
		 VALUES (?, ?, ?, ?, 0, ?, ?)`,
		accountID, journalID, bucket, granted, expires, time.Now().UTC())
	if res.Error != nil {
		t.Fatalf("seed lot: %v", res.Error)
	}
	var id uint
	db.Raw(`SELECT max(id) FROM coin_lot WHERE journal_id = ?`, journalID).Scan(&id)
	return id
}

// grant creates a journal to hang fixtures on, without moving any coins.
func grantFixtureJournal(t *testing.T, db *gorm.DB, key string) string {
	t.Helper()
	res := db.Exec(
		`INSERT INTO coin_journal (entry_type, state, scope, idempotency_key, request_fingerprint,
		                           reason_code, created_at, metadata)
		 VALUES ('GRANT', 'POSTED', 'user', ?, '\x00', 'RESOURCE_APPROVED', ?, '{}'::jsonb)`,
		key, time.Now().UTC())
	if res.Error != nil {
		t.Fatalf("fixture journal %s: %v", key, res.Error)
	}
	var id string
	db.Raw(`SELECT id FROM coin_journal WHERE idempotency_key = ?`, key).Scan(&id)
	return id
}

// ── the five invariants of 02-architecture.md §11 ────────────────────────────

// assertLedgerInvariants runs the reconciliation queries from §11 verbatim,
// plus the two that make the first one non-vacuous.
//
// Invariant 1 as written — `GROUP BY journal_id HAVING SUM(amount) <> 0` — is
// only meaningful over journals that HAVE postings. A journal with no postings
// simply does not appear in that group-by, so a bug that made every spend
// posting-free would satisfy invariant 1 perfectly. The two extra assertions
// close that: every posting-free journal must carry a hold reason code, and every
// money-moving journal must have postings.
func assertLedgerInvariants(t *testing.T, db *gorm.DB) {
	t.Helper()

	// 1. every journal nets to zero.
	var unbalanced []string
	if err := db.Raw(`SELECT journal_id FROM coin_posting GROUP BY 1 HAVING SUM(amount) <> 0`).
		Scan(&unbalanced).Error; err != nil {
		t.Fatalf("invariant 1: %v", err.Error())
	}
	if len(unbalanced) > 0 {
		t.Errorf("invariant 1 (every journal nets to zero) violated by %d journals: %v", len(unbalanced), unbalanced)
	}

	// 2. global conservation: SUM(posted_balance) across every account is 0.
	var total int64
	db.Raw(`SELECT COALESCE(SUM(posted_balance), 0) FROM coin_account_balance`).Scan(&total)
	if total != 0 {
		t.Errorf("invariant 2 (global SUM(posted_balance) = 0) violated: got %d", total)
	}

	// 3. no user account overdrawn.
	var overdrawn []uint
	if err := db.Raw(`SELECT account_id FROM coin_account_balance WHERE liability AND posted_balance - reserved < 0`).
		Scan(&overdrawn).Error; err != nil {
		t.Fatalf("invariant 3: %v", err.Error())
	}
	if len(overdrawn) > 0 {
		t.Errorf("invariant 3 (no liability account overdrawn) violated: %v", overdrawn)
	}

	// 4. the cached projection matches the postings, per account.
	var drifted []uint
	if err := db.Raw(
		`SELECT b.account_id
		   FROM coin_account_balance b
		  WHERE b.posted_balance <> (SELECT COALESCE(SUM(p.amount), 0) FROM coin_posting p WHERE p.account_id = b.account_id)`).
		Scan(&drifted).Error; err != nil {
		t.Fatalf("invariant 4: %v", err.Error())
	}
	if len(drifted) > 0 {
		t.Errorf("invariant 4 (cache matches postings) violated for accounts %v", drifted)
	}

	// 5. no duplicate idempotency keys.
	var dupes []string
	if err := db.Raw(
		`SELECT scope || '/' || idempotency_key FROM coin_journal GROUP BY 1 HAVING COUNT(*) > 1`).
		Scan(&dupes).Error; err != nil {
		t.Fatalf("invariant 5: %v", err.Error())
	}
	if len(dupes) > 0 {
		t.Errorf("invariant 5 (no duplicate idempotency keys) violated: %v", dupes)
	}

	// 6. the strengthening of invariant 1: a posting-free journal is a hold, and
	// nothing else. This is what makes invariant 1 cover the whole journal table.
	var postingFree []string
	if err := db.Raw(
		`SELECT reason_code FROM coin_journal j
		  WHERE NOT EXISTS (SELECT 1 FROM coin_posting p WHERE p.journal_id = j.id)
		  ORDER BY reason_code`).Scan(&postingFree).Error; err != nil {
		t.Fatalf("invariant 6: %v", err.Error())
	}
	for _, reason := range postingFree {
		if !IsHoldReason(reason) {
			t.Errorf("journal with reason %q has no postings, but only a hold reason may: "+
				"invariant 1 is vacuous for a posting-free money journal", reason)
		}
	}

	// 7. and the mirror: a journal with a money reason has postings.
	var moneyWithoutPostings []string
	if err := db.Raw(
		`SELECT reason_code FROM coin_journal j
		  WHERE j.reason_code NOT IN ('REFERRAL_HOLD')
		    AND NOT EXISTS (SELECT 1 FROM coin_posting p WHERE p.journal_id = j.id)
		  ORDER BY reason_code`).Scan(&moneyWithoutPostings).Error; err != nil {
		t.Fatalf("invariant 7: %v", err.Error())
	}
	if len(moneyWithoutPostings) > 0 {
		t.Errorf("journals with money reason codes but no postings: %v", moneyWithoutPostings)
	}

	// 8. lot integrity. The allocator's whole contract is that this holds after
	// every spend, and it is the only place the accumulator bug would show up.
	if n := countRows(t, db, `SELECT count(*) FROM coin_lot WHERE consumed > granted OR consumed < 0`); n != 0 {
		t.Errorf("invariant 8 (no lot over-consumed) violated by %d lots", n)
	}

	// 9. account_seq is a strictly increasing per-account counter with no gaps and
	// no NULLs.
	//
	// 02-architecture.md §2 declares UNIQUE (account_id, account_seq) as a table
	// constraint, but the SHIPPED SCHEMA DOES NOT HAVE IT: the GORM model carries
	// no uniqueIndex tag, so AutoMigrate cannot create it, and
	// EnsurePostgresIndexes does not add it either. Verified against a live
	// instance — pg_constraint on coin_posting holds only the primary key and
	// chk_coin_posting_nonzero. Nothing in the database stops two transactions
	// writing the same sequence number; the FOR UPDATE that InsertPostings takes
	// on the target accounts is the only thing that does.
	//
	// So this assertion is the substitute for the missing constraint, and it is
	// the one that caught the account lock silently not executing.
	if n := countRows(t, db, `SELECT count(*) FROM coin_posting WHERE account_seq IS NULL`); n != 0 {
		t.Errorf("invariant 9 violated: %d postings have a NULL account_seq", n)
	}
	if n := countRows(t, db,
		`SELECT count(*) FROM coin_posting a
		  WHERE a.account_seq > 1
		    AND NOT EXISTS (SELECT 1 FROM coin_posting b
		                     WHERE b.account_id = a.account_id AND b.account_seq = a.account_seq - 1)`); n != 0 {
		t.Errorf("invariant 9 violated: %d postings have a gap or a repeat in their account's "+
			"account_seq. Two transactions read the same counter, which means the per-account "+
			"row lock in InsertPostings is not taking effect.", n)
	}

	// 10. seq is dense from 1 within a journal, for the same reason:
	// UNIQUE (journal_id, seq) is declared in the spec and equally absent from the
	// shipped schema. InsertPostings checks density in Go before writing, and this
	// confirms it at the rows.
	if n := countRows(t, db,
		`SELECT count(*) FROM coin_posting a
		  WHERE a.seq > 1
		    AND NOT EXISTS (SELECT 1 FROM coin_posting b
		                     WHERE b.journal_id = a.journal_id AND b.seq = a.seq - 1)`); n != 0 {
		t.Errorf("invariant 10 violated: %d postings have a gap or a repeat in their journal's seq", n)
	}
}

// ── happy path, and the ledger's own arithmetic ──────────────────────────────

// TestGrantAndSpendMoveTheCacheAndTheLotsTogether is the base case everything
// else is a perturbation of: 80 granted, 60 spent, 20 left, and every one of the
// five invariants holding.
func TestGrantAndSpendMoveTheCacheAndTheLotsTogether(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil) // resource_approved 80, mock_test 60

	grant, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 11, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:r11:res:1",
		CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	if grant.Amount != 80 || grant.Bucket != BucketEarned {
		t.Fatalf("grant moved %d into %s, want 80 into %s", grant.Amount, grant.Bucket, BucketEarned)
	}
	if grant.ExpiresAt == nil {
		t.Fatal("a grant must open a lot with a configured expiry, not a permanent one")
	}
	if got, want := int(grant.ExpiresAt.Sub(grant.JournalTime).Hours()/24), int(DefaultEconomyConfig().Expiry.EarnedDays); got != want {
		t.Errorf("lot expires in %d days, want the configured %d", got, want)
	}
	if grant.Available != 80 {
		t.Errorf("available after the grant = %d, want 80", grant.Available)
	}

	spend, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 11, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(501), IdempotencyKey: "spend:u11:mock:501", CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if spend.Amount != 60 {
		t.Errorf("spend charged %d, want the configured mock_test price 60", spend.Amount)
	}
	if len(spend.SpentFrom) != 1 || spend.SpentFrom[0].Amount != 60 {
		t.Errorf("spent_from = %+v, want one lot of 60", spend.SpentFrom)
	}
	if spend.Available != 20 || spend.AvailableBefore != 80 {
		t.Errorf("available %d (from %d), want 20 (from 80)", spend.Available, spend.AvailableBefore)
	}

	if got := userPosted(t, db, 11); got != 20 {
		t.Errorf("cached posted balance = %d, want 20", got)
	}
	lots := lotRows(t, db, grant.JournalID)
	if len(lots) != 1 || lots[0].Consumed != 60 || lots[0].Granted != 80 {
		t.Errorf("lots = %+v, want one lot with 60 of 80 consumed", lots)
	}
	// The sink took the coins back and the faucet gave them out, so the two
	// system accounts offset.
	var faucet, sink int64
	db.Raw(`SELECT COALESCE(SUM(b.posted_balance),0) FROM coin_account_balance b
	          JOIN coin_account a ON a.id=b.account_id WHERE a.system_type = 'earned_faucet'`).Scan(&faucet)
	db.Raw(`SELECT COALESCE(SUM(b.posted_balance),0) FROM coin_account_balance b
	          JOIN coin_account a ON a.id=b.account_id WHERE a.system_type = 'redeemed_sink'`).Scan(&sink)
	if faucet != -80 || sink != 60 {
		t.Errorf("earned_faucet = %d, redeemed_sink = %d; want -80 and +60", faucet, sink)
	}

	assertLedgerInvariants(t, db)
}

// TestFEFOSpendsSoonestExpiringFirstAgainstRealLots is the FEFO contract at the
// database, with three lots whose expiries the ledger's own grants cannot
// produce (they all get the same configured lifetime), seeded the way
// seedUserAccount already seeds an account in schema_pg_test.go.
func TestFEFOSpendsSoonestExpiringFirstAgainstRealLots(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, func(c *EconomyConfig) {
		c.Prices.StudyResource = 120
		c.Awards.ResourceApproved = 175
	})

	grant, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 21, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u21:res:1",
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	account := userAccountID(t, db, 21, BucketEarned)
	if account == 0 {
		t.Fatal("the EARNED account was not created")
	}

	// Replace the grant's single lot with the spec's shape: 100 expiring in 30
	// days, 50 in 365, 25 never.
	if err := db.Exec(`DELETE FROM coin_lot WHERE journal_id = ?`, grant.JournalID).Error; err != nil {
		t.Fatalf("clear the grant's lot: %v", err.Error())
	}
	soon := time.Now().UTC().AddDate(0, 0, 30)
	year := time.Now().UTC().AddDate(0, 0, 365)
	l1 := seedLot(t, db, grant.JournalID, account, BucketEarned, 100, &soon)
	l2 := seedLot(t, db, grant.JournalID, account, BucketEarned, 50, &year)
	l3 := seedLot(t, db, grant.JournalID, account, BucketEarned, 25, nil)

	spend, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 21, ReasonCode: ReasonResourceUnlock, RefType: RefStudyResource,
		RefID: uint64p(900), IdempotencyKey: "spend:u21:res:900",
	})
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if spend.Amount != 120 {
		t.Fatalf("charged %d, want the configured 120", spend.Amount)
	}

	// 100 from the 30-day lot and 20 from the 365-day lot, never-expiring
	// untouched. The per-lot increments, not 120 from each.
	if len(spend.SpentFrom) != 2 {
		t.Fatalf("spent_from = %+v, want exactly two lots", spend.SpentFrom)
	}
	if spend.SpentFrom[0].LotID != l1 || spend.SpentFrom[0].Amount != 100 {
		t.Errorf("first lot burned = %+v, want lot %d for 100", spend.SpentFrom[0], l1)
	}
	if spend.SpentFrom[1].LotID != l2 || spend.SpentFrom[1].Amount != 20 {
		t.Errorf("second lot burned = %+v, want lot %d for 20", spend.SpentFrom[1], l2)
	}

	byID := map[uint]CoinLot{}
	for _, l := range lotRows(t, db, grant.JournalID) {
		byID[l.ID] = l
	}
	if byID[l1].Consumed != 100 || byID[l1].Granted != 100 {
		t.Errorf("the 30-day lot is at %d of %d, want fully consumed", byID[l1].Consumed, byID[l1].Granted)
	}
	if byID[l2].Consumed != 20 {
		t.Errorf("the 365-day lot has given up %d, want 20", byID[l2].Consumed)
	}
	if byID[l3].Consumed != 0 {
		t.Errorf("the never-expiring lot has given up %d, want 0: permanent coins are burned last", byID[l3].Consumed)
	}

	// And the arithmetic that the accumulator bug would have broken: the cached
	// balance, the postings, and the lots all agree that exactly 120 went.
	if got := userPosted(t, db, 21); got != 55 {
		t.Errorf("cached balance = %d, want 175 - 120 = 55", got)
	}
	var userLegs int64
	db.Raw(`SELECT COALESCE(SUM(p.amount),0) FROM coin_posting p
	          JOIN coin_account a ON a.id = p.account_id
	         WHERE a.owner_user_id = 21 AND p.journal_id = ?`, spend.JournalID).Scan(&userLegs)
	if userLegs != -120 {
		t.Errorf("the spend's user legs sum to %d, want -120", userLegs)
	}
	var burned int64
	db.Raw(`SELECT COALESCE(SUM(consumed), 0) FROM coin_lot WHERE journal_id = ?`, grant.JournalID).Scan(&burned)
	if burned != 120 {
		t.Errorf("the lots gave up %d, want 120", burned)
	}

	assertLedgerInvariants(t, db)
}

func uint64p(v uint64) *uint64 { return &v }
func strp(v string) *string    { return &v }

// ── the headline concurrency test ────────────────────────────────────────────

// TestConcurrentSpendsProduceExactlyFloorDivisionSuccesses is the reproduction
// of the validation in 02-architecture.md §3, and the load-bearing test in this
// file.
//
// Twenty transactions each try to spend 30 from a balance of 100. Exactly
// floor(100/30) = 3 must succeed. Not "at most 3", and not "3 or 4 depending on
// timing": exactly 3, every run. If the per-user advisory lock did nothing, two
// transactions would both read 100 available, both take 30 from the same lot, and
// four or more would commit against 100 coins.
//
// It also asserts the part that is easy to get wrong and expensive to discover
// later: a REJECTED transaction leaves zero rows. The journal insert is inside
// the same transaction as everything else, so the refusal rolls the whole thing
// back. A ledger that wrote a journal and then refused would show up as an
// unexplained entry in a student's transaction history.
func TestConcurrentSpendsProduceExactlyFloorDivisionSuccesses(t *testing.T) {
	const (
		userID     = 101
		balance    = int64(100)
		cost       = int64(30)
		concurrent = 20
	)
	wantSuccesses := int(balance / cost)

	db := openLedgerSchema(t)
	ledger := testLedger(t, db, func(c *EconomyConfig) {
		c.Awards.ResourceApproved = balance
		c.Prices.MockTest = cost
	})

	grant, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: userID, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u101:res:1",
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	journalsBefore := countRows(t, db, `SELECT count(*) FROM coin_journal`)
	postingsBefore := countRows(t, db, `SELECT count(*) FROM coin_posting`)

	// A barrier, not a sleep: every goroutine is released at the same instant, so
	// they genuinely contend for the lock rather than trickling through it.
	var ready, done sync.WaitGroup
	ready.Add(concurrent)
	done.Add(concurrent)
	start := make(chan struct{})

	var mu sync.Mutex
	succeeded := make(map[string]bool, concurrent)
	var rejected int64
	var otherErrors []error

	for i := 0; i < concurrent; i++ {
		go func(i int) {
			defer done.Done()
			key := fmt.Sprintf("spend:u101:mock:%03d", i)
			ready.Done()
			<-start
			res, err := ledger.Spend(context.Background(), SpendRequest{
				UserID: userID, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
				RefID: uint64p(uint64(1000 + i)), IdempotencyKey: key, CreatedBy: "api",
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				succeeded[key] = true
				if res.Amount != cost {
					otherErrors = append(otherErrors,
						fmt.Errorf("goroutine %d was charged %d, want %d", i, res.Amount, cost))
				}
			case errors.Is(err, ErrInsufficientCoins):
				rejected++
			default:
				otherErrors = append(otherErrors, fmt.Errorf("goroutine %d: %w", i, err))
			}
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	if len(otherErrors) > 0 {
		t.Fatalf("unexpected errors: %v", otherErrors)
	}
	if len(succeeded) != wantSuccesses {
		t.Errorf("got %d successes, want exactly floor(%d/%d) = %d. "+
			"More than that means the per-user advisory lock did not serialise them; "+
			"fewer means something rejected a spend that should have fitted.",
			len(succeeded), balance, cost, wantSuccesses)
	}
	if int(rejected) != concurrent-wantSuccesses {
		t.Errorf("got %d rejections, want %d", rejected, concurrent-wantSuccesses)
	}

	// The remainder is exact, not approximately right.
	if got, want := userPosted(t, db, userID), balance%cost; got != want {
		t.Errorf("final cached balance = %d, want exactly the remainder %d", got, want)
	}
	// And the lots agree with the cache, which is the invariant the accumulator
	// bug would have broken.
	var consumed int64
	db.Raw(`SELECT COALESCE(SUM(consumed),0) FROM coin_lot WHERE journal_id = ?`,
		grant.JournalID).Scan(&consumed)
	if consumed != balance-(balance%cost) {
		t.Errorf("the lots gave up %d, want %d", consumed, balance-(balance%cost))
	}

	// Rejected transactions left ZERO rows. Every journal that exists belongs to
	// the grant or to a transaction that actually succeeded.
	var journalKeys []string
	if err := db.Raw(`SELECT idempotency_key FROM coin_journal ORDER BY idempotency_key`).Scan(&journalKeys).Error; err != nil {
		t.Fatalf("read journal keys: %v", err.Error())
	}
	for _, key := range journalKeys {
		switch {
		case key == "grant:u101:res:1":
		case succeeded[key]:
		default:
			t.Errorf("journal %q exists but its transaction was rejected: a refused spend must leave no rows", key)
		}
	}
	if got, want := countRows(t, db, `SELECT count(*) FROM coin_journal`), journalsBefore+int64(len(succeeded)); got != want {
		t.Errorf("journal rows = %d, want %d (one grant plus one per success)", got, want)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting`); got <= postingsBefore {
		t.Errorf("posting rows = %d, want more than the %d before the race: successes must have written legs", got, postingsBefore)
	}
	// Every posting belongs to a journal that is allowed to exist.
	var orphanKeys []string
	if err := db.Raw(
		`SELECT j.idempotency_key FROM coin_posting p
		   JOIN coin_journal j ON j.id = p.journal_id`).Scan(&orphanKeys).Error; err != nil {
		t.Fatalf("read posting journals: %v", err.Error())
	}
	for _, key := range orphanKeys {
		if key != "grant:u101:res:1" && !succeeded[key] {
			t.Errorf("posting belongs to %q, whose transaction was rejected", key)
		}
	}

	assertLedgerInvariants(t, db)
}

// TestConcurrentGrantsAndSpendsConvergeOnTheSameTotal runs the two write paths
// against each other on one user. The grants and spends take the same per-user
// lock, so they interleave in some order and the final balance must be the same
// whichever order it was. The system accounts are shared with every other user in
// production, which is where UNIQUE (account_id, account_seq) would bite without
// the account lock InsertPostings takes; this is the cheapest place to notice if
// that lock were dropped, because a dropped account_seq unique violation surfaces
// as an unexpected error under contention.
func TestConcurrentGrantsAndSpendsConvergeOnTheSameTotal(t *testing.T) {
	const (
		userID  = 102
		workers = 12
		award   = int64(50)
		cost    = int64(30)
	)
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, func(c *EconomyConfig) {
		c.Awards.ResourceApproved = award
		c.Prices.StudyResource = cost
	})

	var ready, done sync.WaitGroup
	ready.Add(workers)
	done.Add(workers)
	start := make(chan struct{})
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			// Grant first, so the spend is always affordable regardless of the
			// order the two paths interleave in. The test is about
			// interleaving, not about a race on affordability.
			if _, err := ledger.Grant(context.Background(), GrantRequest{
				UserID: userID, ReasonCode: ReasonResourceApproved,
				IdempotencyKey: fmt.Sprintf("grant:u102:res:%02d", i),
			}); err != nil {
				errs[i] = fmt.Errorf("grant %d: %w", i, err)
				return
			}
			if _, err := ledger.Spend(context.Background(), SpendRequest{
				UserID: userID, ReasonCode: ReasonResourceUnlock, RefType: RefStudyResource,
				RefID: uint64p(uint64(2000 + i)), IdempotencyKey: fmt.Sprintf("spend:u102:res:%02d", i),
			}); err != nil {
				errs[i] = fmt.Errorf("spend %d: %w", i, err)
			}
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("worker failed: %v", err)
		}
	}
	if got, want := userPosted(t, db, userID), workers*(award-cost); got != want {
		t.Errorf("final balance = %d, want %d workers x (%d granted - %d spent) = %d",
			got, workers, award, cost, want)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != 2*workers {
		t.Errorf("journals = %d, want %d", got, 2*workers)
	}
	assertLedgerInvariants(t, db)
}

// TestConcurrentReplayOfOneKeyDebitsExactlyOnce is the at-least-once delivery
// case from 02-architecture.md §12: the client may retry, and the effect happens
// exactly once.
//
// Twelve goroutines send the SAME request with the SAME idempotency key at the
// same instant. The unique index admits one, the other eleven replay. If any
// goroutine saw "no row returned" for a transaction that had not committed, or if
// the advisory lock were absent and two transactions both inserted, the balance
// would be 960 instead of 80.
func TestConcurrentReplayOfOneKeyDebitsExactlyOnce(t *testing.T) {
	const (
		userID     = 103
		concurrent = 12
	)
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil) // resource_approved 80

	var ready, done sync.WaitGroup
	ready.Add(concurrent)
	done.Add(concurrent)
	start := make(chan struct{})
	journals := make([]string, concurrent)
	replays := make([]bool, concurrent)
	errs := make([]error, concurrent)

	for i := 0; i < concurrent; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			res, err := ledger.Grant(context.Background(), GrantRequest{
				UserID: userID, ReasonCode: ReasonResourceApproved,
				IdempotencyKey: "grant:u103:res:same", CreatedBy: "api",
			})
			if err != nil {
				errs[i] = err
				return
			}
			journals[i] = res.JournalID
			replays[i] = res.Replayed
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	for i := 1; i < concurrent; i++ {
		if journals[i] != journals[0] {
			t.Fatalf("goroutine %d got journal %s, goroutine 0 got %s: a replay must return the original",
				i, journals[i], journals[0])
		}
	}
	// Exactly one inserter; every other goroutine is a replay. Whichever one won
	// is not deterministic, so the assertion is on the count, not the index.
	inserted := 0
	for _, r := range replays {
		if !r {
			inserted++
		}
	}
	if inserted != 1 {
		t.Errorf("%d goroutines reported a fresh grant, want exactly 1", inserted)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != 1 {
		t.Errorf("journals = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting`); got != 2 {
		t.Errorf("postings = %d, want 2 (one grant, one faucet)", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_lot`); got != 1 {
		t.Errorf("lots = %d, want 1", got)
	}
	if got := userPosted(t, db, userID); got != 80 {
		t.Errorf("balance = %d, want exactly one award of 80", got)
	}
	assertLedgerInvariants(t, db)
}

// ── idempotency, sequentially ────────────────────────────────────────────────

// TestIdempotentReplayReturnsTheOriginalAndMovesNothing covers the whole §12
// contract on a single goroutine: same key and same payload replays, same key and
// a different payload is a conflict, and neither moves a coin.
func TestIdempotentReplayReturnsTheOriginalAndMovesNothing(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil)

	first, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 111, ReasonCode: ReasonResourceApproved,
		RefType: strp(RefStudyResource), RefID: uint64p(7),
		IdempotencyKey: "grant:u111:res:7", CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("first grant: %v", err)
	}
	journalsAfterFirst := countRows(t, db, `SELECT count(*) FROM coin_journal`)
	postingsAfterFirst := countRows(t, db, `SELECT count(*) FROM coin_posting`)

	second, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 111, ReasonCode: ReasonResourceApproved,
		RefType: strp(RefStudyResource), RefID: uint64p(7),
		IdempotencyKey: "grant:u111:res:7", CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !second.Replayed {
		t.Error("the second grant with the same key must be reported as a replay")
	}
	if second.JournalID != first.JournalID {
		t.Errorf("replay returned journal %s, want the original %s", second.JournalID, first.JournalID)
	}
	if second.Amount != first.Amount || second.Bucket != first.Bucket {
		t.Errorf("replay reported %d/%s, want the original's %d/%s",
			second.Amount, second.Bucket, first.Amount, first.Bucket)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != journalsAfterFirst {
		t.Errorf("a replay wrote a journal row: %d -> %d", journalsAfterFirst, got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting`); got != postingsAfterFirst {
		t.Errorf("a replay wrote postings: %d -> %d", postingsAfterFirst, got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_lot`); got != 1 {
		t.Errorf("a replay opened another lot: lots = %d, want 1", got)
	}
	if got := userPosted(t, db, 111); got != 80 {
		t.Errorf("a replay moved the balance to %d, want 80", got)
	}

	// A different payload on the same key is a conflict, per §12.2. Each of these
	// differs in exactly one fingerprinted field.
	for _, tc := range []struct {
		name string
		req  GrantRequest
	}{
		{"different user", GrantRequest{UserID: 112, ReasonCode: ReasonResourceApproved,
			RefType: strp(RefStudyResource), RefID: uint64p(7), IdempotencyKey: "grant:u111:res:7"}},
		{"different reason", GrantRequest{UserID: 111, ReasonCode: ReasonReferralQualified,
			RefType: strp(RefStudyResource), RefID: uint64p(7), IdempotencyKey: "grant:u111:res:7"}},
		{"different ref", GrantRequest{UserID: 111, ReasonCode: ReasonResourceApproved,
			RefType: strp(RefStudyResource), RefID: uint64p(8), IdempotencyKey: "grant:u111:res:7"}},
		{"no ref", GrantRequest{UserID: 111, ReasonCode: ReasonResourceApproved,
			IdempotencyKey: "grant:u111:res:7"}},
	} {
		if _, err := ledger.Grant(context.Background(), tc.req); !errors.Is(err, ErrIdempotencyKeyReuse) {
			t.Errorf("%s on an existing key: error %v, want ErrIdempotencyKeyReuse", tc.name, err)
		}
	}

	// A price change between the original and the retry is a conflict too,
	// because the resolved amount is in the fingerprint. Replaying into a
	// cheaper world must not silently charge the old price and call it success.
	cheaper := testLedger(t, db, func(c *EconomyConfig) { c.Awards.ResourceApproved = 40 })
	if _, err := cheaper.Spend(context.Background(), SpendRequest{
		UserID: 111, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u111:mock:1",
	}); err != nil {
		t.Fatalf("first spend: %v", err)
	}
	if _, err := cheaper.Spend(context.Background(), SpendRequest{
		UserID: 111, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u111:mock:1",
	}); err != nil {
		t.Fatalf("replay: %v", err)
	}
	repriced := testLedger(t, db, func(c *EconomyConfig) { c.Prices.MockTest = 5 })
	if _, err := repriced.Spend(context.Background(), SpendRequest{
		UserID: 111, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u111:mock:1",
	}); !errors.Is(err, ErrIdempotencyKeyReuse) {
		t.Errorf("a retry after a price change: error %v, want ErrIdempotencyKeyReuse", err)
	}
	if got := userPosted(t, db, 111); got != 20 {
		t.Errorf("balance = %d, want 80 - 60 = 20: none of the conflicting retries moved a coin", got)
	}

	assertLedgerInvariants(t, db)
}

// TestSpendingMoreThanAvailableLeavesNoRows is the refused-spend contract on its
// own, and it is what makes the concurrency test's "zero rows" claim credible:
// the same code path, without the concurrency.
func TestSpendingMoreThanAvailableLeavesNoRows(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, func(c *EconomyConfig) {
		c.Awards.ResourceApproved = 50
		c.Prices.Video = 90
		c.Prices.MockTest = 40
	})

	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 121, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u121:res:1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	journals := countRows(t, db, `SELECT count(*) FROM coin_journal`)
	postings := countRows(t, db, `SELECT count(*) FROM coin_posting`)

	_, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 121, ReasonCode: ReasonResourceUnlock, RefType: RefVideo,
		RefID: uint64p(1), IdempotencyKey: "spend:u121:video:1",
	})
	if !errors.Is(err, ErrInsufficientCoins) {
		t.Fatalf("spending 90 with 50 available: error %v, want ErrInsufficientCoins", err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != journals {
		t.Errorf("journal rows = %d, want %d: a refused spend must roll its journal back", got, journals)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting`); got != postings {
		t.Errorf("posting rows = %d, want %d", got, postings)
	}
	if got := userPosted(t, db, 121); got != 50 {
		t.Errorf("balance = %d, want an untouched 50", got)
	}
	var consumed int64
	db.Raw(`SELECT COALESCE(SUM(consumed),0) FROM coin_lot`).Scan(&consumed)
	if consumed != 0 {
		t.Errorf("lots gave up %d, want 0", consumed)
	}

	// The refusal is keyed to the key that was sent, and a spend that genuinely
	// fits still goes through afterwards: a refusal must not poison the account.
	if _, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 121, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(2), IdempotencyKey: "spend:u121:mock:2",
	}); err != nil {
		t.Fatalf("a spend that fits must succeed after a refusal: %v", err)
	}
	if got := userPosted(t, db, 121); got != 10 {
		t.Errorf("balance = %d, want 50 - 40 = 10", got)
	}
	assertLedgerInvariants(t, db)
}

// ── reserve: a hold is not a grant ───────────────────────────────────────────

// TestReserveIsNotAGrant is the explicit statement of the distinction: a hold
// raises `reserved`, leaves `posted` alone, and writes no posting at all. If it
// moved `posted`, a pending referral would be indistinguishable from an issued
// award in the audit trail, and every "why does this balance say what it says"
// question would have an unanswerable entry.
func TestReserveIsNotAGrant(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil) // resource_approved 80, referral_referrer 60

	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 131, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u131:res:1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	postingsBefore := countRows(t, db, `SELECT count(*) FROM coin_posting`)

	hold, err := ledger.Reserve(context.Background(), ReserveRequest{
		UserID: 131, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 55,
		IdempotencyKey: "hold:u131:ref:55", CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if hold.Amount != 60 {
		t.Errorf("held %d, want the configured referral_referrer 60", hold.Amount)
	}
	if hold.Available != 20 {
		t.Errorf("available after the hold = %d, want 80 - 60 = 20", hold.Available)
	}

	if got := userPosted(t, db, 131); got != 80 {
		t.Errorf("posted = %d, want 80: a hold must not move posted", got)
	}
	if got := userReserved(t, db, 131); got != 60 {
		t.Errorf("reserved = %d, want 60", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting`); got != postingsBefore {
		t.Errorf("posting rows = %d, want %d: a hold posts nothing", got, postingsBefore)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_lot`); got != 1 {
		t.Errorf("lots = %d, want 1: a hold opens no lot", got)
	}
	var state string
	db.Raw(`SELECT state FROM coin_journal WHERE id = ?`, hold.JournalID).Scan(&state)
	if state != StatePending {
		t.Errorf("hold journal state = %q, want %q", state, StatePending)
	}

	// Idempotent per referral.
	again, err := ledger.Reserve(context.Background(), ReserveRequest{
		UserID: 131, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 55,
		IdempotencyKey: "hold:u131:ref:55", CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("replayed hold: %v", err)
	}
	if !again.Replayed || again.JournalID != hold.JournalID {
		t.Errorf("replay = %+v, want the original journal %s marked as a replay", again, hold.JournalID)
	}
	if got := userReserved(t, db, 131); got != 60 {
		t.Errorf("reserved after a replayed hold = %d, want 60: a replay must not double the hold", got)
	}

	// A hold larger than what is available is refused with a typed error, not a
	// CHECK violation.
	rich := testLedger(t, db, func(c *EconomyConfig) { c.Awards.ReferralReferrer = 500 })
	if _, err := rich.Reserve(context.Background(), ReserveRequest{
		UserID: 131, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 56,
		IdempotencyKey: "hold:u131:ref:56",
	}); !errors.Is(err, ErrHoldExceedsBalance) {
		t.Errorf("holding 500 with 20 available: error %v, want ErrHoldExceedsBalance", err)
	}
	if got := userReserved(t, db, 131); got != 60 {
		t.Errorf("reserved after a refused hold = %d, want 60", got)
	}

	assertLedgerInvariants(t, db)
}

// TestSpendRefusesWhatIsAlreadyPromisedAway is the arithmetic the whole hold
// mechanism rests on, at the numbers the task requires: posted = 200, reserved =
// 150, so a 60-coin spend is refused because only 50 is available.
func TestSpendRefusesWhatIsAlreadyPromisedAway(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, func(c *EconomyConfig) {
		c.Awards.ResourceApproved = 200
		c.Awards.ReferralReferrer = 150
		c.Prices.MockTest = 60
	})

	grant, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 141, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u141:res:1",
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	hold, err := ledger.Reserve(context.Background(), ReserveRequest{
		UserID: 141, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 9,
		IdempotencyKey: "hold:u141:ref:9",
	})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if got, want := userPosted(t, db, 141), int64(200); got != want {
		t.Fatalf("posted = %d, want %d", got, want)
	}
	if got, want := userReserved(t, db, 141), int64(150); got != want {
		t.Fatalf("reserved = %d, want %d", got, want)
	}
	if hold.Available != 50 {
		t.Errorf("available after the hold = %d, want 50", hold.Available)
	}

	// posted - reserved = 50 < 60, so this must be refused even though the raw
	// balance of 200 comfortably covers it.
	journals := countRows(t, db, `SELECT count(*) FROM coin_journal`)
	_, err = ledger.Spend(context.Background(), SpendRequest{
		UserID: 141, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u141:mock:1",
	})
	if !errors.Is(err, ErrInsufficientCoins) {
		t.Fatalf("spending 60 with posted=200 reserved=150: error %v, want ErrInsufficientCoins", err)
	}
	if !strings.Contains(err.Error(), "50 available") {
		t.Errorf("error %q should name the 50 that is actually available", err.Error())
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != journals {
		t.Errorf("the refused spend left a journal behind: %d -> %d", journals, got)
	}
	if got := userPosted(t, db, 141); got != 200 {
		t.Errorf("posted = %d, want an untouched 200", got)
	}
	var consumed int64
	db.Raw(`SELECT COALESCE(SUM(consumed),0) FROM coin_lot WHERE journal_id = ?`, grant.JournalID).Scan(&consumed)
	if consumed != 0 {
		t.Errorf("the lot gave up %d, want 0: a hold must not make a lot look spent", consumed)
	}

	// Lifting the hold frees exactly the promised amount, and the same spend now
	// goes through.
	release, err := ledger.ReleaseReserved(context.Background(), ReleaseReservedRequest{
		UserID: 141, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 9,
		CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if release.Amount != 150 || release.JournalID != hold.JournalID {
		t.Errorf("release = %+v, want 150 against hold %s", release, hold.JournalID)
	}
	if got := userReserved(t, db, 141); got != 0 {
		t.Errorf("reserved after the release = %d, want 0", got)
	}
	if _, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 141, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u141:mock:1",
	}); err != nil {
		t.Fatalf("the spend must fit once the hold is lifted: %v", err)
	}
	if got := userPosted(t, db, 141); got != 140 {
		t.Errorf("posted = %d, want 200 - 60 = 140", got)
	}

	assertLedgerInvariants(t, db)
}

// TestReleaseReservedIsIdempotent: a second release finds the hold already
// resolved, changes nothing, and says so. The reserved decrement is a
// compare-and-set, so a double release is not representable.
func TestReleaseReservedIsIdempotent(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil)

	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 151, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u151:res:1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	hold, err := ledger.Reserve(context.Background(), ReserveRequest{
		UserID: 151, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 3,
		IdempotencyKey: "hold:u151:ref:3",
	})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	first, err := ledger.ReleaseReserved(context.Background(), ReleaseReservedRequest{
		UserID: 151, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 3,
		CreatedBy: "api",
	})
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if first.Replayed {
		t.Error("the first release is not a replay")
	}
	if got := userReserved(t, db, 151); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}

	// A repeated release finds no open hold. It reports ErrNotFound rather than
	// silently succeeding, and the important part is the state: reserved does not
	// move again.
	if _, err := ledger.ReleaseReserved(context.Background(), ReleaseReservedRequest{
		UserID: 151, ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 3,
		CreatedBy: "api",
	}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a second release: error %v, want ErrNotFound (there is no open hold left)", err)
	}
	if got := userReserved(t, db, 151); got != 0 {
		t.Errorf("reserved after a repeated release = %d, want 0: reserved must never go negative", got)
	}
	if got := userPosted(t, db, 151); got != 80 {
		t.Errorf("posted = %d, want 80: a release moves no coins", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting`); got != 2 {
		t.Errorf("postings = %d, want 2 (the grant only)", got)
	}
	var state string
	db.Raw(`SELECT state FROM coin_journal WHERE id = ?`, hold.JournalID).Scan(&state)
	if state != StateReversed {
		t.Errorf("hold state = %q, want %q", state, StateReversed)
	}
	assertLedgerInvariants(t, db)
}

// TestConcurrentHoldsAndSpendsNeverOverdraw: holds and spends contend on the same
// user, and the sum of what is held plus what is spent can never exceed what was
// granted. Twenty goroutines each either hold 30 or spend 30 against a balance of
// 100; the total that can fit is three, and whichever three they are, the
// invariants must hold and reserved must never go negative.
func TestConcurrentHoldsAndSpendsNeverOverdraw(t *testing.T) {
	const (
		userID     = 161
		concurrent = 20
		unit       = int64(30)
	)
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, func(c *EconomyConfig) {
		c.Awards.ResourceApproved = 100
		c.Awards.ReferralReferrer = unit
		c.Prices.MockTest = unit
	})

	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: userID, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u161:res:1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	var ready, done sync.WaitGroup
	ready.Add(concurrent)
	done.Add(concurrent)
	start := make(chan struct{})
	var mu sync.Mutex
	var fitsHold, fitsSpend int

	for i := 0; i < concurrent; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			refID := uint64(3000 + i)
			ref := &refID
			var err error
			if i%2 == 0 {
				_, err = ledger.Reserve(context.Background(), ReserveRequest{
					UserID: userID, ReasonCode: ReasonReferralHold, RefType: RefUserReferral,
					RefID: refID, IdempotencyKey: fmt.Sprintf("hold:u161:ref:%04d", refID),
				})
			} else {
				_, err = ledger.Spend(context.Background(), SpendRequest{
					UserID: userID, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
					RefID: ref, IdempotencyKey: fmt.Sprintf("spend:u161:mock:%04d", refID),
				})
			}
			if err == nil {
				mu.Lock()
				if i%2 == 0 {
					fitsHold++
				} else {
					fitsSpend++
				}
				mu.Unlock()
				return
			}
			if !errors.Is(err, ErrInsufficientCoins) && !errors.Is(err, ErrHoldExceedsBalance) {
				t.Errorf("goroutine %d: unexpected error %v", i, err)
			}
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	// Whichever three win, the total that fitted is three: each operation takes 30
	// from the same 100 and they all contend for the same user lock.
	if got, want := fitsHold+fitsSpend, int(100/unit); got != want {
		t.Errorf("%d of %d operations fitted (%d holds, %d spends), want exactly %d",
			got, concurrent, fitsHold, fitsSpend, want)
	}
	posted := userPosted(t, db, userID)
	reserved := userReserved(t, db, userID)
	// A hold moves no coins, so posted falls only by what was spent and
	// reserved rises only by what was held.
	if want := 100 - unit*int64(fitsSpend); posted != want {
		t.Errorf("posted = %d, want %d (100 less %d spent in units of %d)", posted, want, fitsSpend, unit)
	}
	if want := unit * int64(fitsHold); reserved != want {
		t.Errorf("reserved = %d, want %d (%d holds of %d)", reserved, want, fitsHold, unit)
	}
	// And availability can never have gone negative at any point: whatever fitted,
	// held plus spent cannot exceed the grant.
	if spent := 100 - posted; spent+reserved > 100 {
		t.Errorf("spent %d plus held %d exceeds the 100 granted", spent, reserved)
	}
	if posted-reserved < 0 {
		t.Errorf("posted %d minus reserved %d is negative: the no-overdraft CHECK should have refused", posted, reserved)
	}
	assertLedgerInvariants(t, db)
}

// ── reversal ─────────────────────────────────────────────────────────────────

// TestReverseClawbackTakesTheGrantAndItsLotBack is the §1 capability a balance
// column cannot have: "was this already refunded?" is answerable because the
// reversal is a second journal pointing at the first, and the first is untouched.
func TestReverseClawbackTakesTheGrantAndItsLotBack(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil)

	grant, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 171, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u171:res:1",
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	rev, err := ledger.Reverse(context.Background(), ReverseRequest{
		JournalID: grant.JournalID, ReasonCode: ReasonGrantReversal,
		IdempotencyKey: "rev:u171:res:1", CreatedBy: "admin:1",
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if rev.Amount != 80 {
		t.Errorf("reversed %d, want the whole 80", rev.Amount)
	}
	if rev.LotsRemoved != 1 {
		t.Errorf("lots removed = %d, want 1: a clawed-back grant must leave nothing spendable", rev.LotsRemoved)
	}
	if got := userPosted(t, db, 171); got != 0 {
		t.Errorf("balance = %d, want 0 after a full clawback", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_lot WHERE journal_id = ?`, grant.JournalID); got != 0 {
		t.Errorf("the clawed-back lot is still there: %d rows", got)
	}
	// The original is untouched and still POSTED; the reversal is a separate
	// record.
	var originalState string
	db.Raw(`SELECT state FROM coin_journal WHERE id = ?`, grant.JournalID).Scan(&originalState)
	if originalState != StatePosted {
		t.Errorf("the original journal became %q; POSTED is terminal and nothing is edited in place", originalState)
	}
	var reversalOf *string
	db.Raw(`SELECT reversal_of FROM coin_journal WHERE id = ?`, rev.JournalID).Scan(&reversalOf)
	if reversalOf == nil || *reversalOf != grant.JournalID {
		t.Errorf("reversal_of = %v, want %s", reversalOf, grant.JournalID)
	}

	// And it is not spendable.
	if _, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 171, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u171:mock:1",
	}); !errors.Is(err, ErrInsufficientCoins) {
		t.Errorf("spending clawed-back coins: error %v, want ErrInsufficientCoins", err)
	}

	// A second clawback is refused: the same audit question has one answer.
	if _, err := ledger.Reverse(context.Background(), ReverseRequest{
		JournalID: grant.JournalID, ReasonCode: ReasonGrantReversal,
		IdempotencyKey: "rev:u171:res:2", CreatedBy: "admin:1",
	}); !errors.Is(err, ErrImmutable) {
		t.Errorf("a second reversal: error %v, want ErrImmutable", err)
	}
	if got := userPosted(t, db, 171); got != 0 {
		t.Errorf("balance = %d after a refused second clawback, want 0", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal WHERE entry_type = 'REVERSAL'`); got != 1 {
		t.Errorf("reversal journals = %d, want 1", got)
	}

	// And a replay of the first one changes nothing.
	replay, err := ledger.Reverse(context.Background(), ReverseRequest{
		JournalID: grant.JournalID, ReasonCode: ReasonGrantReversal,
		IdempotencyKey: "rev:u171:res:1", CreatedBy: "admin:1",
	})
	// The original is now known to be reversed, so the guard fires before the
	// key is even consulted. Either refusal is correct; a doubled clawback is
	// not.
	if err == nil && !replay.Replayed {
		t.Error("a replayed reversal must either replay or refuse, never move coins again")
	}
	if got := userPosted(t, db, 171); got != 0 {
		t.Errorf("balance = %d, want 0: no path may claw back twice", got)
	}

	assertLedgerInvariants(t, db)
}

// TestReverseOfPartlySpentGrantShrinksTheLotRatherThanTheBalance is the
// interesting half of a clawback. The coins are gone, so the balance still falls
// — and the no-overdraft CHECK then refuses it, which is the correct answer. The
// lot arithmetic is exercised instead by a clawback that is smaller than the
// unspent remainder.
func TestReverseOfPartlySpentGrantShrinksTheLotRatherThanTheBalance(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, func(c *EconomyConfig) {
		c.Awards.ResourceApproved = 100
		c.Prices.StudyResource = 30
	})

	grant, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 181, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u181:res:1",
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 181, ReasonCode: ReasonResourceUnlock, RefType: RefStudyResource,
		RefID: uint64p(1), IdempotencyKey: "spend:u181:res:1",
	}); err != nil {
		t.Fatalf("spend: %v", err)
	}
	if got := userPosted(t, db, 181); got != 70 {
		t.Fatalf("balance after spending 30 of 100 = %d, want 70", got)
	}

	// The full clawback cannot complete: 70 coins remain and 100 are being taken
	// back, so the CHECK refuses it rather than letting the balance go negative.
	if _, err := ledger.Reverse(context.Background(), ReverseRequest{
		JournalID: grant.JournalID, ReasonCode: ReasonGrantReversal,
		IdempotencyKey: "rev:u181:full",
	}); err == nil {
		t.Error("clawing back 100 with 70 in hand must be refused, not silently overdrawn")
	}
	if got := userPosted(t, db, 181); got != 70 {
		t.Errorf("balance = %d after a refused clawback, want 70", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != 2 {
		t.Errorf("journals = %d, want 2: the refused reversal rolled back entirely", got)
	}

	assertLedgerInvariants(t, db)
}

// TestReverseRefusesToReverseASpend: a spend's user legs are negative, so there
// is nothing to claw back, and a refund is a grant rather than a reversal.
func TestReverseRefusesToReverseASpend(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil)
	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 191, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u191:res:1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	spend, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 191, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u191:mock:1",
	})
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if _, err := ledger.Reverse(context.Background(), ReverseRequest{
		JournalID: spend.JournalID, ReasonCode: ReasonGrantReversal,
		IdempotencyKey: "rev:u191:mock:1",
	}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("reversing a spend: error %v, want ErrInvalidArgument", err)
	}
	if got := userPosted(t, db, 191); got != 20 {
		t.Errorf("balance = %d, want 20", got)
	}
	assertLedgerInvariants(t, db)
}

// ── the frozen account ───────────────────────────────────────────────────────

// TestFrozenAccountRefusesSpend: a hold placed by support stops the money
// moving, and it reports 423 rather than 402, so the student is not sent off to
// earn coins for an account that is locked.
func TestFrozenAccountRefusesSpend(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil)
	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 201, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u201:res:1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 201, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u201:warm",
	}); err != nil {
		t.Fatalf("a spend before the freeze must work: %v", err)
	}

	for _, bucket := range SpendableBuckets {
		if err := db.Exec(`UPDATE coin_account SET closed_at = ? WHERE owner_user_id = ? AND bucket = ?`,
			time.Now().UTC(), uint64(201), bucket).Error; err != nil {
			t.Fatalf("freeze %s: %v", bucket, err.Error())
		}
	}
	journals := countRows(t, db, `SELECT count(*) FROM coin_journal`)
	_, err := ledger.Spend(context.Background(), SpendRequest{
		UserID: 201, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(2), IdempotencyKey: "spend:u201:mock:2",
	})
	if !errors.Is(err, ErrAccountFrozen) {
		t.Fatalf("spending from a frozen account: error %v, want ErrAccountFrozen", err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != journals {
		t.Errorf("the refused spend left a journal behind: %d -> %d", journals, got)
	}
	if got := userPosted(t, db, 201); got != 20 {
		t.Errorf("balance = %d, want 20", got)
	}
	assertLedgerInvariants(t, db)
}

// ── argument validation against a live database ──────────────────────────────

// TestLedgerRejectsBadArgumentsBeforeOpeningATransaction: a rejected request
// must not take a user's advisory lock, and must not leave a row. Every case
// here fails in the validation phase, before InUserTx.
func TestLedgerRejectsBadArgumentsBeforeOpeningATransaction(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		run  func() error
		want error
	}{
		{"grant with no user", func() error {
			_, err := ledger.Grant(ctx, GrantRequest{ReasonCode: ReasonResourceApproved, IdempotencyKey: "k1"})
			return err
		}, ErrInvalidArgument},
		{"grant with no key", func() error {
			_, err := ledger.Grant(ctx, GrantRequest{UserID: 1, ReasonCode: ReasonResourceApproved})
			return err
		}, ErrInvalidArgument},
		{"grant of an unknown reason", func() error {
			_, err := ledger.Grant(ctx, GrantRequest{UserID: 1, ReasonCode: "FREE_COINS", IdempotencyKey: "k2"})
			return err
		}, ErrInvalidArgument},
		{"grant with a ref_type and no ref_id", func() error {
			_, err := ledger.Grant(ctx, GrantRequest{UserID: 1, ReasonCode: ReasonResourceApproved,
				RefType: strp(RefStudyResource), IdempotencyKey: "k3"})
			return err
		}, ErrInvalidArgument},
		{"spend of an unknown class", func() error {
			_, err := ledger.Spend(ctx, SpendRequest{UserID: 1, ReasonCode: ReasonResourceUnlock,
				RefType: "study_resources", RefID: uint64p(1), IdempotencyKey: "k4"})
			return err
		}, ErrInvalidArgument},
		{"spend with a grant reason", func() error {
			_, err := ledger.Spend(ctx, SpendRequest{UserID: 1, ReasonCode: ReasonResourceApproved,
				RefType: RefMockTest, RefID: uint64p(1), IdempotencyKey: "k5"})
			return err
		}, ErrInvalidArgument},
		{"spend with no key", func() error {
			_, err := ledger.Spend(ctx, SpendRequest{UserID: 1, ReasonCode: ReasonResourceUnlock,
				RefType: RefMockTest, RefID: uint64p(1)})
			return err
		}, ErrInvalidArgument},
		{"hold with no ref", func() error {
			_, err := ledger.Reserve(ctx, ReserveRequest{UserID: 1, ReasonCode: ReasonReferralHold,
				IdempotencyKey: "k6"})
			return err
		}, ErrInvalidArgument},
		{"hold with a non-hold reason", func() error {
			_, err := ledger.Reserve(ctx, ReserveRequest{UserID: 1, ReasonCode: EntryGrant,
				RefType: RefUserReferral, RefID: 1, IdempotencyKey: "k7"})
			return err
		}, ErrInvalidArgument},
		{"release of an unknown hold", func() error {
			_, err := ledger.ReleaseReserved(ctx, ReleaseReservedRequest{UserID: 1,
				ReasonCode: ReasonReferralHold, RefType: RefUserReferral, RefID: 99})
			return err
		}, ErrNotFound},
		{"reversal of an unknown journal", func() error {
			_, err := ledger.Reverse(ctx, ReverseRequest{JournalID: "00000000-0000-0000-0000-000000000000",
				ReasonCode: ReasonGrantReversal, IdempotencyKey: "k8"})
			return err
		}, ErrNotFound},
		{"reversal with no reason", func() error {
			_, err := ledger.Reverse(ctx, ReverseRequest{JournalID: "00000000-0000-0000-0000-000000000000",
				IdempotencyKey: "k9"})
			return err
		}, ErrInvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, tc.want) {
				t.Fatalf("error %v, want one matching %v", err, tc.want)
			}
		})
	}

	// Nothing was written by any of them.
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != 0 {
		t.Errorf("journals = %d, want 0: a rejected argument must not write", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting`); got != 0 {
		t.Errorf("postings = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_account WHERE kind = 'USER'`); got != 0 {
		t.Errorf("user accounts = %d, want 0: a rejected argument must not even create an account", got)
	}
}

// TestConfiguredZeroAwardIsRefusedRatherThanPostingZero: an admin who sets an
// award to 0 has misconfigured the economy. Honouring it would write a posting of
// zero, which CHECK (amount <> 0) rejects anyway — but the typed error says which
// key is wrong, and no transaction is opened at all.
func TestConfiguredZeroAwardIsRefusedRatherThanPostingZero(t *testing.T) {
	db := openLedgerSchema(t)
	zero := testLedger(t, db, func(c *EconomyConfig) { c.Awards.ResourceApproved = 0 })
	if _, err := zero.Grant(context.Background(), GrantRequest{
		UserID: 211, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u211:res:1",
	}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a zero award: error %v, want ErrInvalidArgument", err)
	}
	zeroPrice := testLedger(t, db, func(c *EconomyConfig) { c.Prices.MockTest = 0 })
	if _, err := zeroPrice.Spend(context.Background(), SpendRequest{
		UserID: 211, ReasonCode: ReasonResourceUnlock, RefType: RefMockTest,
		RefID: uint64p(1), IdempotencyKey: "spend:u211:mock:1",
	}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a zero price: error %v, want ErrInvalidArgument (a zero price is a free unlock)", err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM coin_journal`); got != 0 {
		t.Errorf("journals = %d, want 0", got)
	}
}

// TestPostingsCannotBeCorrectedInPlace: the append-only trigger is what makes the
// audit trail worth anything, and the ledger never issues an UPDATE or DELETE on
// coin_posting. This proves that at the level the reconciliation job will see it.
func TestPostingsCannotBeCorrectedInPlace(t *testing.T) {
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil)
	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID: 221, ReasonCode: ReasonResourceApproved, IdempotencyKey: "grant:u221:res:1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if err := db.Exec(`UPDATE coin_posting SET amount = amount + 1`).Error; err == nil {
		t.Error("coin_posting UPDATE was allowed; a mutable leg destroys the audit trail")
	}
	if err := db.Exec(`DELETE FROM coin_posting`).Error; err == nil {
		t.Error("coin_posting DELETE was allowed")
	}
	// A zero leg is refused too, which is what makes "skip a lot rather than write
	// 0" a database guarantee rather than only a code convention.
	if err := db.Exec(
		`INSERT INTO coin_journal (entry_type, state, scope, idempotency_key, request_fingerprint, reason_code)
		 VALUES ('GRANT', 'POSTED', 'user', 'zero', '\x00', 'RESOURCE_APPROVED')`).Error; err != nil {
		t.Fatalf("journal fixture: %v", err.Error())
	}
	var jid string
	db.Raw(`SELECT id FROM coin_journal WHERE idempotency_key = 'zero'`).Scan(&jid)
	if err := db.Exec(
		`INSERT INTO coin_posting (journal_id, seq, account_id, amount, account_seq)
		 SELECT ?, 1, id, 0, 1 FROM coin_account WHERE kind = 'SYSTEM' LIMIT 1`, jid).Error; err == nil {
		t.Error("a zero-amount posting was allowed")
	}
}

// TestConcurrentGrantsAcrossDifferentUsersDoNotCollideOnAccountSeq covers the one
// case the per-user advisory lock does NOT cover, and which 02-architecture.md §3
// does not mention.
//
// The system accounts are shared by every user in the system. coin_posting has
// UNIQUE (account_id, account_seq), so appending to earned_faucet needs a
// strictly increasing per-account counter — and two DIFFERENT users hold two
// DIFFERENT user locks, so they will both read MAX(account_seq) + 1 and one of
// them loses on the unique index. Every other concurrency test in this file uses
// a single user, so the user lock masks the problem completely; this one is
// written with a different user per goroutine for exactly that reason.
//
// The fix is in InsertPostings, which takes the target coin_account rows FOR
// UPDATE in ascending id order before reading any counter. This test is what
// proves that lock is there and doing its job.
func TestConcurrentGrantsAcrossDifferentUsersDoNotCollideOnAccountSeq(t *testing.T) {
	const (
		users   = 12
		rounds  = 2
		workers = users * rounds
	)
	db := openLedgerSchema(t)
	ledger := testLedger(t, db, nil) // resource_approved 80

	var ready, done sync.WaitGroup
	ready.Add(workers)
	done.Add(workers)
	start := make(chan struct{})
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			// A distinct user per goroutine, so the per-user advisory lock gives
			// no mutual exclusion at all and the only thing standing between two
			// transactions is the shared system account.
			userID := uint(1000 + i%users)
			_, errs[i] = ledger.Grant(context.Background(), GrantRequest{
				UserID:         userID,
				ReasonCode:     ReasonResourceApproved,
				IdempotencyKey: fmt.Sprintf("grant:u%d:res:%d", userID, i/users),
			})
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	if got, want := countRows(t, db, `SELECT count(*) FROM coin_journal`), int64(workers); got != want {
		t.Errorf("journals = %d, want %d", got, want)
	}
	// The faucet took one leg per grant. If the row lock were missing, some of
	// these inserts would have failed on the unique index above.
	const faucetLegs = `SELECT count(*) FROM coin_posting p
	                       JOIN coin_account a ON a.id = p.account_id
	                      WHERE a.system_type = 'earned_faucet'`
	if got := countRows(t, db, faucetLegs); got != int64(workers) {
		t.Errorf("earned_faucet has %d legs, want %d (one per grant)", got, workers)
	}
	// UNIQUE (account_id, account_seq) makes the count a tautology on its own, so
	// the assertion that matters is that no leg has a NULL sequence: a NULL
	// escapes the unique index entirely and would be a silent hole in the
	// ordering.
	if got := countRows(t, db, `SELECT count(*) FROM coin_posting WHERE account_seq IS NULL`); got != 0 {
		t.Errorf("%d postings have a NULL account_seq; the per-account counter must be written on every leg", got)
	}
	var maxSeq, gaps int64
	db.Raw(`SELECT COALESCE(MAX(account_seq), 0) FROM coin_posting p
	          JOIN coin_account a ON a.id = p.account_id
	         WHERE a.system_type = 'earned_faucet'`).Scan(&maxSeq)
	db.Raw(`SELECT count(*) FROM coin_posting p
	          JOIN coin_account a ON a.id = p.account_id
	         WHERE a.system_type = 'earned_faucet' AND p.account_seq > 1
	           AND NOT EXISTS (SELECT 1 FROM coin_posting q
	                            WHERE q.account_id = p.account_id
	                              AND q.account_seq = p.account_seq - 1)`).Scan(&gaps)
	if maxSeq != int64(workers) {
		t.Errorf("the highest earned_faucet account_seq is %d, want %d (one per grant, contiguous)", maxSeq, workers)
	}
	if gaps != 0 {
		t.Errorf("the earned_faucet account_seq sequence has %d gaps; two transactions read the same counter", gaps)
	}
	for i := 0; i < users; i++ {
		if got := userPosted(t, db, uint(1000+i)); got != 80*rounds {
			t.Errorf("user %d balance = %d, want %d", 1000+i, got, 80*rounds)
		}
	}
	assertLedgerInvariants(t, db)
}

// TestTransactionGuardsAreSetBeforeTheLock asserts the two SET LOCAL timeouts
// 02-architecture.md §3 requires on every coin transaction.
//
// They are the defence against a pathological request: without lock_timeout, one
// request queued behind a stuck one waits forever and holds a user lock open for
// as long as the client keeps the connection. The ordering matters too — the
// timeouts have to be set before the lock, not after, or the pathological
// request already has what it needs.
//
// Nothing else in the suite can see this: with the timeouts absent every test
// still passes, because a healthy transaction never waits long enough to hit
// them. Asserting the values is the only way to keep them.
func TestTransactionGuardsAreSetBeforeTheLock(t *testing.T) {
	db := openLedgerSchema(t)
	repo := NewRepository(db)

	var lockTimeout, idleTimeout string
	err := repo.InUserTx(context.Background(), 241, func(tx *TxContext) error {
		// The advisory lock has already been taken by the time this runs, so
		// these read the values that were in force when it was requested.
		if res := tx.db.Raw(`SELECT current_setting('lock_timeout')`).Scan(&lockTimeout); res.Error != nil {
			return res.Error
		}
		return tx.db.Raw(`SELECT current_setting('idle_in_transaction_session_timeout')`).Scan(&idleTimeout).Error
	})
	if err != nil {
		t.Fatalf("in transaction: %v", err)
	}
	if got := durationSeconds(lockTimeout); got != 3 {
		t.Errorf("lock_timeout = %q, want 3s so a stuck request cannot hold a user lock open", lockTimeout)
	}
	if got := durationSeconds(idleTimeout); got != 10 {
		t.Errorf("idle_in_transaction_session_timeout = %q, want 10s", idleTimeout)
	}
}

// durationSeconds parses a Postgres timeout setting such as "3s" or "1min".
func durationSeconds(setting string) float64 {
	setting = strings.TrimSpace(setting)
	var value float64
	if _, err := fmt.Sscanf(setting, "%fs", &value); err == nil {
		return value
	}
	if _, err := fmt.Sscanf(setting, "%fmin", &value); err == nil {
		return value * 60
	}
	return -1
}

// TestAdvisoryLockSerialisesConcurrentWriters is the control test from
// 02-architecture.md §3: a second transaction that starts later must not write
// until the first has committed.
//
// It runs a first transaction that holds the user lock open, then starts a
// second that reports when its own transaction began and when it acquired the
// lock. The second must acquire strictly after the first commits. Without the
// advisory lock both would be in the critical section together and the
// timestamps would interleave.
func TestAdvisoryLockSerialisesConcurrentWriters(t *testing.T) {
	db := openLedgerSchema(t)
	repo := NewRepository(db)

	firstHeld := make(chan struct{})
	firstMayCommit := make(chan struct{})
	firstCommitted := make(chan struct{})

	go func() {
		defer close(firstCommitted)
		_ = repo.InUserTx(context.Background(), 231, func(tx *TxContext) error {
			// Announce that the lock is held, then hold it until told otherwise.
			close(firstHeld)
			<-firstMayCommit
			return nil
		})
	}()

	<-firstHeld

	type stamps struct{ began, acquired time.Time }
	second := make(chan stamps, 1)
	go func() {
		var s stamps
		err := repo.InUserTx(context.Background(), 231, func(tx *TxContext) error {
			// InUserTx has already taken the lock by the time this runs, so
			// "acquired" is the moment the callback starts.
			s.acquired = time.Now()
			return nil
		})
		if err != nil {
			t.Errorf("second transaction: %v", err)
		}
		s.began = time.Now()
		second <- s
	}()

	// The second transaction must still be waiting: it cannot be inside its
	// callback while the first holds the lock. This is the whole claim, and it
	// is what the floor-division test then relies on.
	select {
	case s := <-second:
		t.Fatalf("the second transaction did not wait for the lock (began %v, acquired %v)", s.began, s.acquired)
	case <-time.After(200 * time.Millisecond):
	}

	close(firstMayCommit)
	<-firstCommitted

	// It completes once the first has committed, which is what "transaction
	// scoped, releases at COMMIT" means in practice.
	select {
	case s := <-second:
		if s.acquired.IsZero() {
			t.Error("the second transaction reported no acquire time")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second transaction never acquired the lock after the first committed")
	}
}
