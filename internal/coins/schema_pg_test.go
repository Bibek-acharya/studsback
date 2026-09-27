//go:build coinsintegration

// internal/coins/schema_pg_test.go
//
// Proves the ledger schema is not merely present but ENFORCING, on a real
// PostgreSQL instance. Two of these assertions exist because the corresponding
// bug was actually hit during schema validation and would otherwise have shipped
// silently:
//
//   - the no-overdraft CHECK must apply to USER accounts only. Applied to system
//     accounts it rejects every single grant, because earned_faucet is
//     legitimately negative after an issue.
//   - the one-account-per-user-bucket and one-account-per-system-type
//     constraints must be TWO partial indexes. A single UNIQUE over
//     (owner_user_id, kind, bucket) collides on (NULL, 'SYSTEM', NULL) for every
//     system account.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// The DSN must point at a database you are willing to have tables created and
// dropped in. This test creates and drops its own schema.
package coins

import (
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testSchema = "coins_schema_test"

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping ledger schema integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	return db
}

func withFreshSchema(t *testing.T, fn func(*gorm.DB)) {
	t.Helper()
	db := openTestDB(t)

	if err := db.Exec(`DROP SCHEMA IF EXISTS ` + testSchema + ` CASCADE`).Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := db.Exec(`CREATE SCHEMA ` + testSchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DROP SCHEMA IF EXISTS ` + testSchema + ` CASCADE`)
	})

	if err := db.Exec(`SET search_path TO ` + testSchema).Error; err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if err := db.AutoMigrate(LedgerModels...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := EnsurePostgresIndexes(db); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	// Every subsequent statement must resolve inside the test schema.
	db.Exec(`SET search_path TO ` + testSchema)
	fn(db)
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// seedUserAccount inserts a USER account and returns its id.
func seedUserAccount(t *testing.T, db *gorm.DB, userID uint64) uint {
	t.Helper()
	mustExec(t, db,
		`INSERT INTO coin_account (kind, owner_user_id, bucket, created_at) VALUES ('USER', ?, 'EARNED', ?)`,
		userID, time.Now().UTC())
	var id uint
	db.Raw(`SELECT id FROM coin_account WHERE owner_user_id = ? AND kind='USER'`, userID).Scan(&id)
	if id == 0 {
		t.Fatalf("could not resolve seeded account for user %d", userID)
	}
	return id
}

// indexExists reports whether an index with the given name is present.
func indexExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var found bool
	if err := db.Raw(`SELECT to_regclass(?) IS NOT NULL`, name).Scan(&found).Error; err != nil {
		t.Fatalf("index lookup %s: %v", name, err)
	}
	return found
}

func TestSchemaCreatesEveryLedgerTable(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		for _, tbl := range []string{
			"coin_account", "coin_account_balance", "coin_journal", "coin_posting", "coin_lot",
		} {
			var found bool
			if err := db.Raw(`SELECT to_regclass(?) IS NOT NULL`, tbl).Scan(&found).Error; err != nil {
				t.Fatalf("lookup %s: %v", tbl, err)
			}
			if !found {
				t.Errorf("table %s was not created", tbl)
			}
		}
	})
}

// The five partial/expression indexes AutoMigrate cannot create. If this fails,
// a fresh `go run ./cmd/server` produced tables that enforce nothing.
func TestEnsureIndexesCreatesPartialIndexes(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		for _, name := range []string{
			"coin_account_user_bucket_uniq",
			"coin_account_system_uniq",
			"coin_journal_idem_uniq",
			"coin_journal_ref_idx",
			"coin_posting_lot_idx",
			"coin_lot_open_idx",
		} {
			if !indexExists(t, db, name) {
				t.Errorf("index %s missing; AutoMigrate cannot create it", name)
			}
		}
	})
}

// The bug that was hit: a single UNIQUE over (owner_user_id, kind, bucket)
// collides on (NULL, 'SYSTEM', NULL). With two partial indexes, all three system
// accounts coexist and a user may hold one account per bucket.
func TestSystemAccountsCoexistAndUserBucketsAreUnique(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		// The chart of accounts was already seeded by EnsurePostgresIndexes.
		// Its coexistence IS the assertion: under a single UNIQUE over
		// (owner_user_id, kind, bucket) these three rows collide on
		// (NULL, 'SYSTEM', NULL) and the seed itself would have failed.
		var n int64
		db.Raw(`SELECT count(*) FROM coin_account WHERE kind='SYSTEM'`).Scan(&n)
		if n != int64(len(SystemAccounts)) {
			t.Fatalf("system accounts = %d, want %d", n, len(SystemAccounts))
		}
		for _, s := range SystemAccounts {
			var found int64
			db.Raw(`SELECT count(*) FROM coin_account WHERE kind='SYSTEM' AND system_type = ?`, s).Scan(&found)
			if found != 1 {
				t.Errorf("system account %q = %d rows, want 1", s, found)
			}
		}

		// A duplicate system_type must be rejected, proving the partial index
		// is enforcing rather than merely present.
		if err := db.Exec(
			`INSERT INTO coin_account (kind, system_type, created_at) VALUES ('SYSTEM', ?, ?)`,
			SystemEarnedFaucet, time.Now().UTC()).Error; err == nil {
			t.Error("a duplicate system account was allowed")
		}

		// One user, both grantable buckets, is allowed.
		mustExec(t, db,
			`INSERT INTO coin_account (kind, owner_user_id, bucket, created_at) VALUES ('USER', 42, 'FREE', ?)`,
			time.Now().UTC())
		mustExec(t, db,
			`INSERT INTO coin_account (kind, owner_user_id, bucket, created_at) VALUES ('USER', 42, 'EARNED', ?)`,
			time.Now().UTC())

		// A second FREE account for the same user must be rejected.
		if err := db.Exec(
			`INSERT INTO coin_account (kind, owner_user_id, bucket, created_at) VALUES ('USER', 42, 'FREE', ?)`,
			time.Now().UTC()).Error; err == nil {
			t.Error("a second account for the same user+bucket was allowed")
		}

		// A different user may hold the same bucket.
		mustExec(t, db,
			`INSERT INTO coin_account (kind, owner_user_id, bucket, created_at) VALUES ('USER', 43, 'FREE', ?)`,
			time.Now().UTC())
	})
}

// The other bug that was hit: applying the no-overdraft CHECK to system accounts
// rejects every grant, because earned_faucet is negative after issuing coins.
func TestNoOverdraftCheckAppliesToUserAccountsOnly(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		var faucetID uint
		db.Raw(`SELECT id FROM coin_account WHERE system_type = ?`, SystemEarnedFaucet).Scan(&faucetID)
		if faucetID == 0 {
			t.Fatal("earned_faucet was not seeded")
		}
		var userID uint
		mustExec(t, db,
			`INSERT INTO coin_account (kind, owner_user_id, bucket, created_at) VALUES ('USER', 7, 'EARNED', ?)`,
			time.Now().UTC())
		db.Raw(`SELECT id FROM coin_account WHERE owner_user_id = 7 AND kind='USER'`).Scan(&userID)

		var faucetLiability bool
		db.Raw(`SELECT liability FROM coin_account_balance WHERE account_id = ?`, faucetID).Scan(&faucetLiability)
		if faucetLiability {
			t.Error("system accounts must seed with liability=false, or the CHECK rejects every grant")
		}

		// A user account may not go negative.
		mustExec(t, db, `INSERT INTO coin_account_balance (account_id) VALUES (?)`, userID)
		err := db.Exec(`UPDATE coin_account_balance SET posted_balance = -1 WHERE account_id = ?`, userID).Error
		if err == nil {
			t.Error("a user balance was allowed to go negative")
		}
		mustExec(t, db, `UPDATE coin_account_balance SET posted_balance = 50 WHERE account_id = ?`, userID)

		// The system account may, because it is the outside-the-world side.
		mustExec(t, db, `UPDATE coin_account_balance SET posted_balance = -100 WHERE account_id = ?`, faucetID)
	})
}

// Idempotency is enforced by the index, not by application logic. Two journals
// with the same (scope, idempotency_key) must not coexist.
func TestIdempotencyKeyIsUnique(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		mustExec(t, db, `INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'user', 'grant:u1:ref:1', '\x00', 'REFERRAL', ?)`,
			time.Now().UTC())

		err := db.Exec(`INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'user', 'grant:u1:ref:1', '\x01', 'REFERRAL', ?)`,
			time.Now().UTC()).Error
		if err == nil {
			t.Error("a duplicate (scope, idempotency_key) was allowed")
		}

		// A different key, and the same key under a different scope, are fine.
		mustExec(t, db, `INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'user', 'grant:u1:ref:2', '\x00', 'REFERRAL', ?)`,
			time.Now().UTC())
		mustExec(t, db, `INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'system', 'grant:u1:ref:1', '\x00', 'REFERRAL', ?)`,
			time.Now().UTC())
	})
}

func TestPostingsAreAppendOnly(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		mustExec(t, db, `INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'user', 'k1', '\x00', 'REFERRAL', ?)`,
			time.Now().UTC())
		acctID := seedUserAccount(t, db, 900)
		mustExec(t, db, `INSERT INTO coin_posting
			(journal_id, seq, account_id, amount, account_seq, created_at)
			SELECT id, 1, ?, 5, 1, ? FROM coin_journal WHERE idempotency_key = 'k1'`,
			acctID, time.Now().UTC())

		if err := db.Exec(`UPDATE coin_posting SET amount = 999`).Error; err == nil {
			t.Error("coin_posting UPDATE was allowed; postings must be append-only")
		}
		if err := db.Exec(`DELETE FROM coin_posting`).Error; err == nil {
			t.Error("coin_posting DELETE was allowed; postings must be append-only")
		}
		if err := db.Exec(`INSERT INTO coin_posting (journal_id, seq, account_id, amount, account_seq, created_at)
			SELECT id, 1, ?, 0, 2, ? FROM coin_journal WHERE idempotency_key = 'k1'`, acctID, time.Now().UTC()).Error; err == nil {
			t.Error("a zero-amount posting was allowed")
		}
	})
}

func TestJournalProvenanceIsFrozenAndPostedIsTerminal(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		mustExec(t, db, `INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'user', 'k1', '\x00', 'REFERRAL', ?)`,
			time.Now().UTC())

		if err := db.Exec(`UPDATE coin_journal SET reason_code = 'TAMPERED'`).Error; err == nil {
			t.Error("provenance columns must be frozen")
		}
		if err := db.Exec(`UPDATE coin_journal SET state = 'REVERSED'`).Error; err == nil {
			t.Error("POSTED must be terminal")
		}
		if err := db.Exec(`DELETE FROM coin_journal`).Error; err == nil {
			t.Error("coin_journal DELETE was allowed")
		}
	})
}

func TestLotCannotBeOverConsumed(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		mustExec(t, db, `INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'user', 'k1', '\x00', 'RESOURCE_APPROVED', ?)`,
			time.Now().UTC())
		lotAcct := seedUserAccount(t, db, 901)
		mustExec(t, db, `INSERT INTO coin_lot (account_id, journal_id, bucket, granted, consumed, created_at)
			SELECT ?, id, 'EARNED', 10, 0, ? FROM coin_journal WHERE idempotency_key = 'k1'`,
			lotAcct, time.Now().UTC())

		if err := db.Exec(`UPDATE coin_lot SET consumed = 11`).Error; err == nil {
			t.Error("a lot was allowed to be over-consumed")
		}
		mustExec(t, db, `UPDATE coin_lot SET consumed = 10`)
		if err := db.Exec(`UPDATE coin_lot SET consumed = 11`).Error; err == nil {
			t.Error("a fully consumed lot was allowed to be over-consumed")
		}
	})
}

// FEFO ordering must put never-expiring lots last, so a user never burns
// permanent coins while expiring ones sit untouched.
//
// This asserts behaviour rather than index DDL on purpose: `NULLS LAST` is the
// default sort direction for ASC, so Postgres does not record it in
// pg_indexes.indexdef. An earlier version of this test asserted on that text
// and failed on a correct index. The allocator's query is the contract.
func TestFEFOOrderPutsNeverExpiringLotsLast(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		acct := seedUserAccount(t, db, 902)
		mustExec(t, db, `INSERT INTO coin_journal
			(entry_type, state, scope, idempotency_key, request_fingerprint, reason_code, created_at)
			VALUES ('GRANT', 'POSTED', 'user', 'fefo', '\x00', 'RESOURCE_APPROVED', ?)`,
			time.Now().UTC())
		var jid string
		db.Raw(`SELECT id FROM coin_journal WHERE idempotency_key='fefo'`).Scan(&jid)

		now := time.Now().UTC()
		mustExec(t, db, `INSERT INTO coin_lot (account_id, journal_id, bucket, granted, consumed, expires_at, created_at)
			VALUES (?, ?, 'EARNED', 10, 0, NULL, ?)`, acct, jid, now) // never expires
		mustExec(t, db, `INSERT INTO coin_lot (account_id, journal_id, bucket, granted, consumed, expires_at, created_at)
			VALUES (?, ?, 'FREE', 10, 0, ?, ?)`, acct, jid, now.Add(72*time.Hour), now) // soonest
		mustExec(t, db, `INSERT INTO coin_lot (account_id, journal_id, bucket, granted, consumed, expires_at, created_at)
			VALUES (?, ?, 'EARNED', 10, 0, ?, ?)`, acct, jid, now.Add(240*time.Hour), now) // later

		type row struct {
			ID        uint
			ExpiresAt *time.Time
		}
		var rows []row
		if err := db.Raw(`SELECT id, expires_at FROM coin_lot
			WHERE account_id = ? AND consumed < granted
			ORDER BY expires_at NULLS LAST, id`, acct).Scan(&rows).Error; err != nil {
			t.Fatalf("FEFO query: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("got %d open lots, want 3", len(rows))
		}
		// Assert relative order, not exact timestamps. Go's time.Time carries
		// nanoseconds and Postgres timestamptz stores microseconds, so an
		// exact round-trip comparison fails on precision alone. FEFO is an
		// ordering contract, so that is what this asserts.
		if rows[0].ExpiresAt == nil || rows[1].ExpiresAt == nil {
			t.Fatalf("the two expiring lots must sort before the non-expiring one, got %+v", rows)
		}
		if !rows[0].ExpiresAt.Before(*rows[1].ExpiresAt) {
			t.Errorf("soonest-expiring must sort first, got %v then %v", rows[0].ExpiresAt, rows[1].ExpiresAt)
		}
		if rows[2].ExpiresAt != nil {
			t.Errorf("the never-expiring lot must sort last, got expires_at=%v", rows[2].ExpiresAt)
		}
	})
}

// EnsurePostgresIndexes must be safe to run twice: main.go calls it on every
// boot.
func TestEnsurePostgresIndexesIsIdempotent(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		for i := 0; i < 3; i++ {
			if err := EnsurePostgresIndexes(db); err != nil {
				t.Fatalf("EnsurePostgresIndexes call %d: %v", i+1, err)
			}
		}
		var n int64
		db.Raw(`SELECT count(*) FROM coin_account WHERE kind='SYSTEM'`).Scan(&n)
		if n != int64(len(SystemAccounts)) {
			t.Errorf("chart of accounts = %d after 3 runs, want %d (seed must not duplicate)", n, len(SystemAccounts))
		}
	})
}

func TestEnsureIndexesIsNoOpOnSQLite(t *testing.T) {
	// The dev fallback must not error out trying to create Postgres partial
	// indexes. AutoMigrate alone leaves the tables unenforced there, which is
	// acceptable for a dev fallback but must not be fatal.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := EnsurePostgresIndexes(db); err != nil {
		t.Fatalf("EnsurePostgresIndexes on sqlite: %v", err)
	}
	if err := EnsurePostgresIndexes(nil); err != nil {
		t.Fatalf("EnsurePostgresIndexes(nil): %v", err)
	}
}
