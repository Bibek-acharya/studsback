//go:build coinsintegration

// internal/coins/unlock_pg_test.go
//
// The entitlement domain against a real PostgreSQL instance. Not mocked, because
// the load-bearing claims in this file are about what the DATABASE refuses to
// accept, and a Go mock accepts everything:
//
//   - that the partial UNIQUE (user_id, resource_type, resource_id) WHERE
//     revoked_at IS NULL refuses a second LIVE row for the same resource. This is
//     the fraud control; a service-layer check that a bug can bypass is not a
//     fraud control.
//   - and, in the other direction, that a revoked row does NOT hold the slot, so
//     a revocation is recoverable. A mock would let both of those "pass"
//     trivially; only the real index can distinguish them.
//   - that ON CONFLICT with the matching predicate resolves against that
//     PARTIAL index, because Postgres will not infer one from a bare column list.
//     Getting this wrong is a 42P10 on every unlock write in the system, and it
//     cannot be observed without a real planner.
//   - that chk_resource_unlock_source_funding refuses a COINS unlock with no
//     journal AND an ALLOWANCE unlock with one, in both directions.
//   - that the derived `used` count and the ceiling cannot both be read by two
//     concurrent requests, i.e. TestConsumeAllowanceWithOneRemainingConsumesItExactlyOnce.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// This file owns exactly one schema, coins_entitlement_test, creates it at the
// start of each test and drops it when the test ends. Nothing is ever created in
// the public schema and nothing is created in a schema this file did not name.
// Every test skips when COINS_TEST_DSN is unset, so `go test ./...` needs no
// database.
//
// The search_path goes in the DSN rather than in a `SET search_path` statement,
// and that difference is load-bearing rather than stylistic: a SET applies to ONE
// pooled connection, so a concurrency test would have some of its eight
// goroutines writing to the public schema and some failing to find the tables.
// ledger_pg_test.go documents the same trap; see its file header.
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

// entitlementSchema is the one schema this file owns.
const entitlementSchema = "coins_entitlement_test"

// openEntitlementSchema creates coins_entitlement_test, migrates the ledger AND
// the entitlement models into it, applies every constraint AutoMigrate cannot
// create, and returns a pool whose every connection is pinned to it.
//
// Both model sets go in because resource_unlock.journal_id has a foreign key to
// coin_journal(id). A COINS unlock is only provable against a journal that
// actually exists, so the parent table has to be there.
//
// Both Ensure functions are called, not just the entitlement one: the allowance
// path takes the ledger's per-user advisory lock, and running it against a
// schema with no coin_account would still work, but the point of these tests is
// that the entitlement domain and the ledger are wired the way main.go wires
// them, so they are wired the same way here.
//
// One pool per test rather than one per run, and it is closed at the end of the
// test (see openTestDB in schema_pg_test.go for why an unclosed *sql.DB leaks
// connections and surfaces later as "sorry, too many clients" in an unrelated
// test).
func openEntitlementSchema(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping StudsToken entitlement integration test")
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
	if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + entitlementSchema + ` CASCADE`).Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := admin.Exec(`CREATE SCHEMA ` + entitlementSchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(`DROP SCHEMA IF EXISTS ` + entitlementSchema + ` CASCADE`).Error
	})

	// search_path in the DSN, not a SET: every connection in the pool is pinned,
	// which is what makes a concurrent test safe.
	pool, err := gorm.Open(postgres.Open(dsn+" search_path="+entitlementSchema), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres pinned to %s: %v", entitlementSchema, err)
	}
	sqlPool, err := pool.DB()
	if err != nil {
		t.Fatalf("sql handle: %v", err)
	}
	// A connection is held for the whole transaction, so a pool that is too small
	// silently serialises the concurrency test and it stops testing anything. Too
	// large exhausts a shared development server; 20 covers the widest fan-out
	// here with headroom.
	sqlPool.SetMaxOpenConns(20)
	sqlPool.SetMaxIdleConns(2)
	sqlPool.SetConnMaxIdleTime(30 * time.Second)
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

// testEntitlements wires a Service over the test database and an in-memory
// economy config. Every allowance figure in these tests comes from here, never
// from a literal at the call site, which is the same rule the ledger enforces on
// its callers.
func testEntitlements(t *testing.T, db *gorm.DB, mutate func(*EconomyConfig)) *Service {
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
	// The version store is unused by the entitlement methods — it exists for the
	// admin config slice — but the Service holds one, so a fake is passed rather
	// than a nil that would panic if an entitlement method ever appended a row.
	return NewServiceWithRepository(NewRepository(db), NewConfigStore(settings), &fakeVersions{})
}

// ── row helpers ──────────────────────────────────────────────────────────────

// spendJournal creates a coin_journal to hang a COINS unlock on, without moving
// any coins. resource_unlock.journal_id has a foreign key to it, so a COINS
// unlock cannot be tested without a real parent row — which is the point: the
// FK is one of the two ways a COINS unlock proves it is real.
func spendJournal(t *testing.T, db *gorm.DB, key string) string {
	t.Helper()
	return grantFixtureJournal(t, db, key)
}

func unlockRows(t *testing.T, db *gorm.DB, userID uint) []ResourceUnlock {
	t.Helper()
	var rows []ResourceUnlock
	if err := db.Raw(
		`SELECT `+unlockColumns+` FROM resource_unlock WHERE user_id = ? ORDER BY id`, userID,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read unlocks for user %d: %v", userID, err)
	}
	return rows
}

func countUnlocks(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count (%s): %v", query, err)
	}
	return n
}

// constraintPresent reports whether pg_constraint carries a named constraint.
func constraintPresent(t *testing.T, db *gorm.DB, name, table string) bool {
	t.Helper()
	var n int64
	if err := db.Raw(
		`SELECT count(*) FROM pg_constraint WHERE conname = ? AND conrelid = ?::regclass`,
		name, table,
	).Scan(&n).Error; err != nil {
		t.Fatalf("look up constraint %s on %s: %v", name, table, err)
	}
	return n == 1
}

// indexPresent reports whether an index of that name exists in the pinned
// schema.
//
// pg_indexes rather than pg_class, because the name alone is ambiguous across
// schemas and this file owns exactly one of them: a test that matched an index
// in the public schema would report success for an object this file never
// created. The relname join is what pins it to the search_path connection.
func indexPresent(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int64
	if err := db.Raw(
		`SELECT count(*) FROM pg_indexes
		  WHERE schemaname = current_schema() AND indexname = ?`, name,
	).Scan(&n).Error; err != nil {
		t.Fatalf("look up index %s: %v", name, err)
	}
	return n == 1
}

// ── the schema exists and is enforcing ───────────────────────────────────────

func TestEnsureEntitlementIndexesCreatesBothTables(t *testing.T) {
	db := openEntitlementSchema(t)
	for _, table := range []string{"resource_unlock", "user_free_allowance"} {
		var found bool
		if err := db.Raw(`SELECT to_regclass(?) IS NOT NULL`, table).Scan(&found).Error; err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if !found {
			t.Errorf("table %s was not created by AutoMigrate", table)
		}
	}
}

// Every constraint AutoMigrate cannot create. If this fails, a fresh
// `go run ./cmd/server` produced entitlement tables that enforce nothing — which
// is the documented failure mode of
// internal/notification/ensure_indexes.go, where a fresh boot produced a server
// whose every preferences PUT failed with 42P10.
func TestEnsureEntitlementIndexesCreatesEveryConstraint(t *testing.T) {
	db := openEntitlementSchema(t)

	checks := map[string]string{
		"chk_resource_unlock_type":           "resource_unlock",
		"chk_resource_unlock_source":         "resource_unlock",
		"chk_resource_unlock_source_funding": "resource_unlock",
		"chk_resource_unlock_coins_paid":     "resource_unlock",
		"chk_resource_unlock_revocation":     "resource_unlock",
		"uq_user_free_allowance_user":        "user_free_allowance",
		"fk_resource_unlock_journal":         "resource_unlock",
	}
	for name, table := range checks {
		if !constraintPresent(t, db, name, table) {
			t.Errorf("constraint %s is missing from %s", name, table)
		}
	}

	// The partial index behind the derived `used` count. Without it, every
	// allowance check is a scan of the user's whole unlock history, and this is
	// the query that runs on every gate check.
	if !indexPresent(t, db, "resource_unlock_allowance_open_idx") {
		t.Error("index resource_unlock_allowance_open_idx is missing; the derived used count has no index behind it")
	}
	// The fraud control itself, now a partial unique index rather than a
	// constraint. It is checked as an index and not as a constraint because that
	// is what it now is — a gorm uniqueIndex tag cannot express the predicate,
	// and see the header of unlock_indexes.go for why AutoMigrate cannot make
	// it at all.
	if !indexPresent(t, db, "resource_unlock_live_uniq") {
		t.Error("index resource_unlock_live_uniq is missing; nothing at all is enforcing one live unlock per resource")
	}
	// The revocation-history read. Both other resource_unlock indexes are
	// partial on revoked_at IS NULL and cannot return a revoked row at all.
	if !indexPresent(t, db, "resource_unlock_user_revoked_idx") {
		t.Error("index resource_unlock_user_revoked_idx is missing; the revocation history is a sequential scan")
	}
}

// The old shape must be gone, not merely superseded. Leaving it would be worse
// than never having changed it: a full UNIQUE over the same three columns
// silently re-forbids every re-buy after a revocation, so the bug this change
// exists to fix would come straight back with no error and no trace of why.
func TestEnsureEntitlementIndexesRemovesTheOldFullUniqueConstraint(t *testing.T) {
	db := openEntitlementSchema(t)

	if constraintPresent(t, db, "resource_unlock_uniq", "resource_unlock") {
		t.Error("resource_unlock_uniq is still a CONSTRAINT; the full unique key over revoked rows is still in force")
	}
	if indexPresent(t, db, "resource_unlock_uniq") {
		t.Error("an index named resource_unlock_uniq still exists; a full unique key over revoked rows is still in force")
	}
	// And the replacement is partial, rather than being a differently-named
	// index that happens to have the same wrong scope. This is the assertion
	// that would catch a future edit swapping CREATE UNIQUE INDEX for a bare
	// one, which would look correct in a diff.
	var def string
	if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE indexname = 'resource_unlock_live_uniq'`).Scan(&def).Error; err != nil {
		t.Fatalf("read the live index definition: %v", err)
	}
	if !strings.Contains(def, "UNIQUE INDEX") {
		t.Errorf("resource_unlock_live_uniq is not a unique index: %s", def)
	}
	if !strings.Contains(def, "revoked_at IS NULL") {
		t.Errorf("resource_unlock_live_uniq is not partial on revoked_at IS NULL, so revoked rows still occupy the cap: %s", def)
	}
}

// The retirement must work against BOTH spellings, because the object being
// dropped was a CONSTRAINT and a constraint owns an index that a bare
// DROP INDEX refuses to touch. An older schema has the constraint; a schema
// somebody has been poking at by hand may have the index. This builds both and
// asserts EnsureEntitlementIndexes removes both.
//
// It is the only test here that constructs the old object deliberately — every
// other test starts from a schema that never had it, where both statements are
// no-ops and a missing DROP would go unnoticed.
func TestEnsureEntitlementIndexesRetiresTheOldObjectUnderBothSpellings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		asIndex bool
	}{
		{name: "as a constraint", asIndex: false},
		{name: "as a plain index", asIndex: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openEntitlementSchema(t)

			if tc.asIndex {
				mustUnlockExec(t, db, `CREATE UNIQUE INDEX resource_unlock_uniq
					ON resource_unlock (user_id, resource_type, resource_id)`)
			} else {
				mustUnlockExec(t, db, `ALTER TABLE resource_unlock
					ADD CONSTRAINT resource_unlock_uniq UNIQUE (user_id, resource_type, resource_id)`)
			}
			// Whichever way it was made, the pre-change behaviour must be back:
			// a re-buy after a revocation is refused.
			now := time.Now().UTC()
			journal := spendJournal(t, db, "retire-old-object")
			mustUnlockExec(t, db,
				`INSERT INTO resource_unlock
					(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
				 VALUES (800, 'study_resource', 1, ?, 'COINS', 40, ?)`, journal, now)
			mustUnlockExec(t, db,
				`UPDATE resource_unlock SET revoked_at = ?, revoke_reason = 'fraud'
				  WHERE user_id = 800`, now)
			if err := db.Exec(
				`INSERT INTO resource_unlock
					(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
				 VALUES (800, 'study_resource', 1, ?, 'COINS', 40, ?)`,
				journal, now).Error; err == nil {
				t.Fatal("the fixture is wrong: a re-buy was allowed before EnsureEntitlementIndexes ran")
			}

			if err := EnsureEntitlementIndexes(db); err != nil {
				t.Fatalf("EnsureEntitlementIndexes over the old object: %v", err)
			}
			if constraintPresent(t, db, "resource_unlock_uniq", "resource_unlock") {
				t.Error("the constraint survived EnsureEntitlementIndexes")
			}
			if indexPresent(t, db, "resource_unlock_uniq") {
				t.Error("the index survived EnsureEntitlementIndexes")
			}
			// And the behaviour flipped: the same insert that was refused a
			// moment ago now succeeds, which is the point of the whole exercise.
			mustUnlockExec(t, db,
				`INSERT INTO resource_unlock
					(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
				 VALUES (800, 'study_resource', 1, ?, 'COINS', 40, ?)`, journal, now)
			if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = 800`); n != 2 {
				t.Errorf("user 800 has %d rows, want 2 — the revoked row and the re-bought one", n)
			}
		})
	}
}

// The CHECK value list is a SQL string and ResourceTypes is a Go slice: one
// list, two representations, and this is the only thing that stops them
// drifting. A new class added to ResourceTypes and not to the CHECK would be
// unluckable in practice — the insert would be refused with a message about a
// CHECK constraint nobody can connect to a feature.
func TestEnsureEntitlementIndexesRestatesTheResourceTypeCheck(t *testing.T) {
	db := openEntitlementSchema(t)
	var def string
	if err := db.Raw(
		`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		  WHERE conname = 'chk_resource_unlock_type' AND conrelid = 'resource_unlock'::regclass`,
	).Scan(&def).Error; err != nil {
		t.Fatalf("read the constraint definition: %v", err)
	}
	for _, class := range ResourceTypes {
		if !strings.Contains(def, "'"+class+"'") {
			t.Errorf("chk_resource_unlock_type does not allow %q; the definition is %s", class, def)
		}
	}
	if strings.Contains(def, "'user_referral'") {
		t.Error("chk_resource_unlock_type allows user_referral, which is a journal ref_type and not an unlockable class")
	}
}

// EnsureEntitlementIndexes must be safe to run repeatedly: main.go calls it on
// every boot, and the ledger's equivalent has an idempotency test for the same
// reason.
func TestEnsureEntitlementIndexesIsIdempotent(t *testing.T) {
	db := openEntitlementSchema(t)
	for i := 0; i < 3; i++ {
		if err := EnsureEntitlementIndexes(db); err != nil {
			t.Fatalf("EnsureEntitlementIndexes call %d: %v", i+1, err)
		}
	}
	if !indexPresent(t, db, "resource_unlock_live_uniq") {
		t.Error("resource_unlock_live_uniq is missing after three runs")
	}
	if !indexPresent(t, db, "resource_unlock_user_revoked_idx") {
		t.Error("resource_unlock_user_revoked_idx is missing after three runs")
	}
	// Idempotency cuts both ways here. The old object is dropped with IF EXISTS
	// and a second run must not fail complaining that it is already gone —
	// main.go calls this on every boot, so "the DROP was already done" is the
	// normal case after the first, not an edge case.
	if constraintPresent(t, db, "resource_unlock_uniq", "resource_unlock") {
		t.Error("resource_unlock_uniq came back as a constraint after three runs")
	}
	if indexPresent(t, db, "resource_unlock_uniq") {
		t.Error("resource_unlock_uniq came back as an index after three runs")
	}
}

// ── the fraud control ────────────────────────────────────────────────────────

// This is the constraint the whole domain exists for, asserted at the rows
// rather than through the service. A fraud ring that farms a thousand
// accounts still cannot hold two LIVE unlocks of the same resource per account,
// so the worst case is bounded by what it actually paid for.
//
// It is asserted with raw SQL on purpose: going through RecordCoinUnlock would
// be testing that the service returns an error, and the claim is that the
// DATABASE does.
func TestLiveUnlockUniquenessIsEnforcedByTheDatabase(t *testing.T) {
	db := openEntitlementSchema(t)
	now := time.Now().UTC()
	journal := spendJournal(t, db, "unlock-unique-fixture")

	mustUnlockExec(t, db,
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (500, 'study_resource', 812, ?, 'COINS', 40, ?)`,
		journal, now)

	// The same user, class and resource, both still LIVE: refused, even though it
	// is a differently-priced purchase. This is the fraud cap and it is the one
	// thing that must not have loosened.
	if err := db.Exec(
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (500, 'study_resource', 812, ?, 'COINS', 60, ?)`,
		spendJournal(t, db, "unlock-unique-fixture-2"), now).Error; err == nil {
		t.Error("a second LIVE unlock for the same (user, class, resource) was allowed")
	}

	// One allowance row per user, for the same idempotency reason the journal's
	// idempotency key has one.
	mustUnlockExec(t, db,
		`INSERT INTO user_free_allowance (user_id, granted_at, expires_at, document_unlocks, video_unlocks, mock_test_unlocks)
		 VALUES (500, ?, ?, 3, 1, 1)`, now, now.AddDate(0, 0, 30))
	if err := db.Exec(
		`INSERT INTO user_free_allowance (user_id, granted_at, expires_at, document_unlocks, video_unlocks, mock_test_unlocks)
		 VALUES (500, ?, ?, 9, 9, 9)`, now, now.AddDate(0, 0, 30)).Error; err == nil {
		t.Error("a second allowance row for one user was allowed, so a re-registration could re-grant the starter allowance")
	}

	// The three freedoms that remain: another user, another class, another
	// resource.
	mustUnlockExec(t, db,
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (501, 'study_resource', 812, ?, 'COINS', 40, ?)`, journal, now)
	mustUnlockExec(t, db,
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (500, 'video', 812, ?, 'COINS', 90, ?)`, journal, now)
	mustUnlockExec(t, db,
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (500, 'study_resource', 813, ?, 'COINS', 40, ?)`, journal, now)
	if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock`); n != 4 {
		t.Errorf("total unlock rows = %d, want 4 — the other three combinations are all legitimate", n)
	}
}

// The other half of the change, and the reason it was made: a revocation must
// not be permanent. Under the old full UNIQUE, a revoked row held its slot
// forever, so a student whose unlock was revoked in error lost it with no path
// back — no reinstatement operation existed and no second row could be inserted.
//
// Now a fresh LIVE row is allowed once the old one is revoked. Three properties
// are asserted together, because any one of them alone would pass for the wrong
// reason: the re-buy is allowed, the cap still holds (a SECOND live row is
// still refused), and the revoked row is intact rather than overwritten.
func TestRevokedUnlockDoesNotOccupyTheLiveSlot(t *testing.T) {
	const userID = 510
	db := openEntitlementSchema(t)
	// Truncated to microseconds because that is timestamptz's resolution and Go
	// has nanoseconds. Without it the round-trip through Postgres loses the last
	// few digits and the equality assertion below is comparing the fixture
	// against a rounding artefact rather than against the stored value.
	now := time.Now().UTC().Truncate(time.Microsecond)
	revokedAt := now.Add(-time.Hour)
	journal := spendJournal(t, db, "revoked-slot-fixture")

	mustUnlockExec(t, db,
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (?, 'study_resource', 812, ?, 'COINS', 40, ?)`,
		userID, journal, now)
	mustUnlockExec(t, db,
		`UPDATE resource_unlock SET revoked_at = ?, revoke_reason = 'revoked by mistake'
		  WHERE user_id = ?`, revokedAt, userID)

	// The re-buy. This is the statement the old index refused.
	mustUnlockExec(t, db,
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (?, 'study_resource', 812, ?, 'COINS', 40, ?)`,
		userID, spendJournal(t, db, "revoked-slot-fixture-2"), now)

	// The cap is still one LIVE row. Allowing the re-buy is not the same as
	// allowing duplicates, and a regression that dropped the predicate from the
	// CREATE entirely would pass the assertion above while breaking this one.
	if err := db.Exec(
		`INSERT INTO resource_unlock
			(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
		 VALUES (?, 'study_resource', 812, ?, 'COINS', 40, ?)`,
		userID, spendJournal(t, db, "revoked-slot-fixture-3"), now).Error; err == nil {
		t.Error("a second LIVE row was allowed after the re-buy; the cap is one live row, not one row ever")
	}

	// The history survives, byte for byte. A re-buy is a second row, not an
	// overwrite: "was this ever granted", "when" and "why" all still have
	// answers, which is the entire reason a revocation is a reversal rather than
	// a delete.
	var history []ResourceUnlock
	if err := db.Raw(
		`SELECT `+unlockColumns+` FROM resource_unlock
		  WHERE user_id = ? AND revoked_at IS NOT NULL`, userID,
	).Scan(&history).Error; err != nil {
		t.Fatalf("read the revoked row: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("the triple has %d revoked rows, want 1", len(history))
	}
	old := history[0]
	if old.RevokeReason == nil || *old.RevokeReason != "revoked by mistake" {
		t.Errorf("the revoked row's reason = %v, want 'revoked by mistake' — the re-buy overwrote the history", old.RevokeReason)
	}
	if old.RevokedAt == nil || !old.RevokedAt.Equal(revokedAt) {
		t.Errorf("the revoked row's timestamp = %v, want %v — the re-buy overwrote the history", old.RevokedAt, revokedAt)
	}

	// Two rows for the triple, exactly one of them live. The revoked row still
	// names the journal it was bought under; the re-buy is a second, separately
	// paid purchase and an auditor can see both.
	if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 2 {
		t.Errorf("user %d has %d rows, want 2 — the revoked row and the re-bought one", userID, n)
	}
	if n := countUnlocks(t, db,
		`SELECT count(*) FROM resource_unlock WHERE user_id = ? AND revoked_at IS NULL`, userID); n != 1 {
		t.Errorf("user %d has %d LIVE rows, want exactly 1", userID, n)
	}
	// The first purchase keeps its own journal. If the two rows had been merged
	// rather than added to, one of these journals would be gone.
	if n := countUnlocks(t, db,
		`SELECT count(*) FROM resource_unlock WHERE user_id = ? AND journal_id = ?`, userID, journal); n != 1 {
		t.Errorf("the original purchase's journal appears on %d rows, want 1", n)
	}
}

// ── ON CONFLICT inference: the regression that matters most ─────────────────

// The one that would have taken production down, and the reason InsertUnlock
// carries `WHERE revoked_at IS NULL` on its ON CONFLICT.
//
// Postgres cannot infer a PARTIAL unique index from a bare column list. The
// inference clause has to carry a predicate the planner can match against the
// index's own. With the bare list the statement does not merely fail to dedupe —
// it fails to PREPARE, every time, on every call:
//
//	ERROR: there is no unique or exclusion constraint matching
//	       the ON CONFLICT specification   (SQLSTATE 42P10)
//
// 42P10 is the same code internal/notification/ensure_indexes.go documents as a
// production incident, and the blast radius is every unlock write in the system:
// RecordCoinUnlock and ConsumeAllowance both route through InsertUnlock, so
// nothing would work at all.
//
// Asserted at the level the bug lives at — the actual statement, through the
// actual repository method — because a test that only inserted twice serially
// would pass against the broken version too: two sequential inserts never
// conflict, so the missing predicate would never be exercised. The concurrency
// is the point.
func TestOnConflictInferenceResolvesAgainstThePartialIndex(t *testing.T) {
	const (
		userID     = 520
		concurrent = 8
	)
	ctx := context.Background()
	db := openEntitlementSchema(t)
	repo := NewRepository(db)
	svc := testEntitlements(t, db, nil)
	now := time.Now().UTC()
	journal := spendJournal(t, db, "on-conflict-inference-fixture")

	t.Run("the bare column list is refused by the planner", func(t *testing.T) {
		// Stated as a fact rather than left implied. If a future Postgres
		// learned to infer through a partial index this would start failing,
		// which is fine: it means the explicit predicate in InsertUnlock has
		// become redundant and the comment there can be revisited. What must
		// not happen is this test being quietly deleted.
		err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (?, 'study_resource', 9991, ?, 'COINS', 40, ?)
			 ON CONFLICT (user_id, resource_type, resource_id) DO NOTHING`,
			userID, journal, now).Error
		if err == nil {
			t.Fatal("a bare ON CONFLICT column list resolved against the partial index; " +
				"either Postgres changed or the index is not partial, and InsertUnlock's explicit " +
				"predicate should be re-examined")
		}
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() != "42P10" {
			t.Errorf("the bare inference failed with %v (SQLSTATE %s), want 42P10 — the test is not "+
				"proving what it claims if it fails for a different reason", err, pgErr.SQLState())
		}
		if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE resource_id = 9991`); n != 0 {
			t.Errorf("the failing statement left %d rows behind", n)
		}
	})

	t.Run("the predicated inference takes the conflict path", func(t *testing.T) {
		// EIGHT concurrent RecordCoinUnlock calls for ONE resource, which is the
		// mobile-retry storm the ON CONFLICT exists for. Exactly one must create
		// a row; the other seven must come back with that same row and no error.
		//
		// A 23505 here means the conflict was NOT resolved — either the
		// predicate is missing and the statement failed to prepare, or something
		// else is going wrong. Either way it surfaces as an error here rather
		// than as ErrAlreadyUnlocked, which is the whole difference the ON
		// CONFLICT is buying.
		var ready, done sync.WaitGroup
		ready.Add(concurrent)
		done.Add(concurrent)
		start := make(chan struct{})

		var mu sync.Mutex
		var ids []uint
		var conflicts []error

		for i := 0; i < concurrent; i++ {
			go func() {
				defer done.Done()
				ready.Done()
				<-start
				row, err := svc.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, 4242, journal, 40, now)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					conflicts = append(conflicts, err)
					return
				}
				ids = append(ids, row.ID)
			}()
		}
		ready.Wait()
		close(start)
		done.Wait()

		if len(conflicts) > 0 {
			t.Errorf("%d of %d concurrent RecordCoinUnlock calls failed: %v\n"+
				"A raw unique violation or 42P10 here means the ON CONFLICT did not resolve against "+
				"resource_unlock_live_uniq; the inferred predicate in InsertUnlock is load-bearing.",
				len(conflicts), concurrent, conflicts[0])
		}
		if len(ids) != concurrent {
			t.Fatalf("%d of %d calls returned a row, want %d — a conflicted insert must return the existing row, not nil",
				len(ids), concurrent, concurrent)
		}
		for _, id := range ids {
			if id != ids[0] {
				t.Fatalf("concurrent calls returned different rows (%v); every one of them must be the same single row", ids)
			}
		}
		if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 4242`, userID); n != 1 {
			t.Errorf("%d rows exist for one concurrently-unlocked resource, want exactly 1", n)
		}
	})

	t.Run("the same predicate does not swallow a revoked row", func(t *testing.T) {
		// The converse, and the reason the predicate is the RIGHT one rather
		// than merely a legal one. If ON CONFLICT were inference-free — say
		// written as `ON CONFLICT DO NOTHING` with no target at all — it would
		// suppress the insert against ANY conflict, and the re-buy after a
		// revocation would silently do nothing while the caller was told
		// nothing. Named inference with a predicate can only conflict on a
		// live row, which is exactly the set that should be suppressed.
		const rbUser = 521
		rbJournal := spendJournal(t, db, "on-conflict-revoked-fixture")
		if _, err := svc.RecordCoinUnlock(ctx, rbUser, ResourceTypeStudyResource, 7001, rbJournal, 40, now); err != nil {
			t.Fatalf("first RecordCoinUnlock: %v", err)
		}
		if err := svc.Revoke(ctx, rbUser, ResourceTypeStudyResource, 7001, "revoked", now); err != nil {
			t.Fatalf("Revoke: %v", err)
		}
		again, err := svc.RecordCoinUnlock(ctx, rbUser, ResourceTypeStudyResource, 7001, rbJournal, 40, now)
		if err != nil {
			t.Fatalf("RecordCoinUnlock after a revocation = %v, want success — a revoked row must not block the re-buy", err)
		}
		if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 7001`, rbUser); n != 2 {
			t.Errorf("%d rows for the revoked-then-rebought resource, want 2", n)
		}
		_ = again
	})

	t.Run("the repository's own conflict path resolves rather than erroring", func(t *testing.T) {
		// InsertUnlock called directly, which is the one place the SQL is
		// visible. A regression in the predicate fails here with a 42P10 that
		// names the statement, rather than somewhere downstream.
		const directUser = 522
		directJournal := spendJournal(t, db, "on-conflict-direct-fixture")
		mustUnlockExec(t, db,
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (?, 'study_resource', 8100, ?, 'COINS', 40, ?)`,
			directUser, directJournal, now)

		var firstID, secondID uint
		err := repo.InUserTx(ctx, directUser, func(tx *TxContext) error {
			created, err := tx.InsertUnlock(&ResourceUnlock{
				UserID: directUser, ResourceType: ResourceTypeStudyResource, ResourceID: 8100,
				JournalID: &directJournal, Source: UnlockSourceCoins, CoinsPaid: 40, UnlockedAt: now,
			})
			if err != nil {
				return err
			}
			if created {
				t.Error("InsertUnlock created a second live row for a triple that already had one")
			}
			firstID = 1
			existing, err := tx.ReadUnlock(directUser, ResourceTypeStudyResource, 8100)
			if err != nil {
				return err
			}
			if existing == nil {
				t.Fatal("ReadUnlock returned nil after a conflicted insert; the conflict path cannot report who holds the row")
			}
			secondID = existing.ID
			return nil
		})
		if err != nil {
			t.Fatalf("the conflicted insert path returned %v; it must resolve to created=false, not an error", err)
		}
		if firstID == 0 || secondID == 0 {
			t.Fatal("the fixture did not run")
		}
	})
}

// ── source and journal_id must agree ─────────────────────────────────────────

// The two impossibilities, in both directions, asserted at the database.
//
// The spec names exactly these two, and both are unrecoverable afterwards: a
// COINS unlock with no journal is money that moved and left no trace of what
// for, and an ALLOWANCE unlock naming a journal claims coins were charged for an
// entitlement that was free.
func TestSourceAndJournalIDMustAgree(t *testing.T) {
	db := openEntitlementSchema(t)
	now := time.Now().UTC()
	journal := spendJournal(t, db, "funding-agreement-fixture")

	t.Run("coins without a journal is refused", func(t *testing.T) {
		if err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (600, 'study_resource', 1, NULL, 'COINS', 40, ?)`, now).Error; err == nil {
			t.Error("a COINS unlock with a NULL journal_id was allowed")
		}
	})

	t.Run("allowance with a journal is refused", func(t *testing.T) {
		if err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (601, 'study_resource', 1, ?, 'ALLOWANCE', 0, ?)`, journal, now).Error; err == nil {
			t.Error("an ALLOWANCE unlock with a journal_id was allowed")
		}
	})

	t.Run("allowance claiming coins were paid is refused", func(t *testing.T) {
		// The third way to lie about the same thing: no journal, but a price.
		// coins_paid = 0 is part of the ALLOWANCE shape, not just a default.
		if err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (602, 'study_resource', 1, NULL, 'ALLOWANCE', 40, ?)`, now).Error; err == nil {
			t.Error("an ALLOWANCE unlock with coins_paid 40 was allowed")
		}
	})

	t.Run("a negative snapshot is refused", func(t *testing.T) {
		if err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (603, 'study_resource', 1, ?, 'COINS', -1, ?)`, journal, now).Error; err == nil {
			t.Error("a negative coins_paid was allowed; a snapshot of a charge can never be negative")
		}
	})

	t.Run("an unknown class is refused", func(t *testing.T) {
		if err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (604, 'documents', 1, ?, 'COINS', 40, ?)`, journal, now).Error; err == nil {
			t.Error("a resource_type outside the three classes was allowed, which creates an unlock no gate will ever match")
		}
	})

	t.Run("an unknown source is refused", func(t *testing.T) {
		if err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (605, 'study_resource', 1, ?, 'FREEBIE', 0, ?)`, journal, now).Error; err == nil {
			t.Error("an unknown source was allowed")
		}
	})

	t.Run("a journal that does not exist is refused", func(t *testing.T) {
		ghost := "00000000-0000-0000-0000-0000000000ff"
		if err := db.Exec(
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (606, 'study_resource', 1, ?, 'COINS', 40, ?)`, ghost, now).Error; err == nil {
			t.Error("a COINS unlock naming a journal that does not exist was allowed")
		}
	})

	t.Run("a half-present revocation is refused", func(t *testing.T) {
		// revoked_at with no reason is a clawback nobody can explain, and a
		// reason with no timestamp cannot be ordered against anything.
		mustUnlockExec(t, db,
			`INSERT INTO resource_unlock
				(user_id, resource_type, resource_id, journal_id, source, coins_paid, unlocked_at)
			 VALUES (607, 'study_resource', 1, ?, 'COINS', 40, ?)`, journal, now)
		if err := db.Exec(
			`UPDATE resource_unlock SET revoked_at = ? WHERE user_id = 607`, now).Error; err == nil {
			t.Error("a revocation with a timestamp and no reason was allowed")
		}
		if err := db.Exec(
			`UPDATE resource_unlock SET revoke_reason = 'oops' WHERE user_id = 607`).Error; err == nil {
			t.Error("a revocation with a reason and no timestamp was allowed")
		}
	})
}

// ── the concurrency claim ────────────────────────────────────────────────────

// THE test. Eight concurrent requests, one allowance remaining, and the answer
// must be one winner and seven refusals.
//
// The naive implementation — `count remaining; if > 0 then insert` — is
// double-spending this. Two goroutines both read "1 left", both find 1 > 0, and
// both insert; the last unlock is consumed twice and the fraud control the
// allowance exists to provide is worth nothing.
//
// The mechanism under test, precisely:
//
//	resource_unlock_live_uniq, via ON CONFLICT (user_id, resource_type,
//	resource_id) WHERE revoked_at IS NULL DO NOTHING RETURNING id, decides "does
//	this user already hold this resource?" inside the insert itself, so that
//	half needs no read at all. The predicate is not optional: without it the
//	statement does not prepare, and everything in the system that writes an
//	unlock fails. See TestOnConflictInferenceResolvesAgainstThePartialIndex.
//
//	The count against the allowance CEILING is not decidable by a constraint —
//	the ceiling is a number in system_settings, not a schema constant, and no
//	index can express "at most the current value of a settings row". That half is
//	decided by serialisation: InUserTx takes the ledger's per-user advisory lock
//	(pg_advisory_xact_lock on 'coin:user:<id>') and then LockFreeAllowance takes
//	FOR UPDATE on the allowance row. Two requests for one user are therefore
//	strictly ordered, and the loser's count runs after the winner's INSERT has
//	committed, so it sees the row.
//
// Note the resource ids are all DIFFERENT. The unique index is not what
// rejects the losers here — a duplicate-resource race would be caught by the
// index and would prove nothing about the count. Every loser is refused
// because there is no allowance left, which is the claim being tested.
//
// The failure mode this test exists to catch is silent: a double-spend here
// shows up as one student with four free document downloads, not as an error.
//
// ── and this test is NOT sufficient on its own ───────────────────────────────
//
// It is worth being explicit about what this does and does not establish, because
// a concurrency test that passes for the wrong reason is worse than none. This
// one asserts an OUTCOME, and an outcome can be reached by luck: the window
// between the count and the insert is a few microseconds, so a naive
// implementation will often pass it by not actually overlapping. It was measured
// against a naive build — same transaction, same count, same insert, no lock —
// and the naive build passed this test. It is
// TestConsumeAllowanceBlocksOnTheAllowanceRow that establishes the mechanism, and
// this one that establishes the outcome.
func TestConsumeAllowanceWithOneRemainingConsumesItExactlyOnce(t *testing.T) {
	const (
		userID     = 700
		concurrent = 8
	)
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, nil)

	now := time.Now().UTC()

	// Three document unlocks, two already burned, so exactly ONE remains. The
	// two are burned through the service so the derived count is a real one.
	if _, err := svc.EnsureAllowance(ctx, userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, uint64(900+i), now); err != nil {
			t.Fatalf("burning the two pre-existing unlocks (%d): %v", i, err)
		}
	}
	status, err := svc.RemainingAllowance(ctx, userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	if got := status.Remaining(ResourceTypeStudyResource); got != 1 {
		t.Fatalf("the fixture is wrong: %d document unlocks remain, want exactly 1", got)
	}

	// A barrier, not a sleep: every goroutine is released at the same instant, so
	// they genuinely contend rather than trickling through the lock one at a time.
	var ready, done sync.WaitGroup
	ready.Add(concurrent)
	done.Add(concurrent)
	start := make(chan struct{})

	var mu sync.Mutex
	var succeeded []uint64
	var exhausted, alreadyUnlocked int
	var otherErrors []error

	for i := 0; i < concurrent; i++ {
		go func(i int) {
			defer done.Done()
			// A distinct resource for every goroutine, so nothing but the
			// allowance can refuse them.
			resourceID := uint64(1000 + i)
			ready.Done()
			<-start
			_, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, resourceID, now)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				succeeded = append(succeeded, resourceID)
			case errors.Is(err, ErrNoAllowanceRemaining):
				exhausted++
			case errors.Is(err, ErrAlreadyUnlocked):
				alreadyUnlocked++
			default:
				otherErrors = append(otherErrors, fmt.Errorf("goroutine %d (resource %d): %w", i, resourceID, err))
			}
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	if len(otherErrors) > 0 {
		t.Fatalf("unexpected errors: %v", otherErrors)
	}
	if len(succeeded) != 1 {
		t.Errorf("got %d successes with exactly 1 unlock remaining, want exactly 1 (winners: %v). "+
			"More than one means the per-user advisory lock did not serialise the count-and-insert, "+
			"which is a double-spend of the last free unlock; none means something refused an unlock that should have fitted.",
			len(succeeded), succeeded)
	}
	if exhausted != concurrent-1 {
		t.Errorf("got %d refusals, want %d", exhausted, concurrent-1)
	}
	if alreadyUnlocked != 0 {
		t.Errorf("%d goroutines were refused with ErrAlreadyUnlocked, want 0 — every goroutine asked for a "+
			"DISTINCT resource, so a duplicate-resource refusal means the uniqueness index is doing "+
			"the work the allowance ceiling should be doing", alreadyUnlocked)
	}

	// The rows, not just the return values. Three active allowance unlocks total:
	// the two burned by the fixture and the one that won.
	if n := countUnlocks(t, db,
		`SELECT count(*) FROM resource_unlock
		  WHERE user_id = ? AND source = 'ALLOWANCE' AND resource_type = ? AND revoked_at IS NULL`,
		userID, ResourceTypeStudyResource); n != 3 {
		t.Errorf("active allowance unlocks for user %d = %d, want 3 — a loser that inserted anyway would make this 4", userID, n)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 3 {
		t.Errorf("total unlock rows for user %d = %d, want 3; a refused transaction must leave nothing behind", userID, n)
	}

	// And the derived count agrees with the rows, which is the whole claim about
	// not storing it.
	after, err := svc.RemainingAllowance(ctx, userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance after the race: %v", err)
	}
	if got := after.Remaining(ResourceTypeStudyResource); got != 0 {
		t.Errorf("document unlocks remaining after the race = %d, want 0", got)
	}
	if got := after.Classes[ResourceTypeStudyResource].Used; got != 3 {
		t.Errorf("derived used count = %d, want 3", got)
	}
}

// The MECHANISM test, and the one that actually establishes that the lock is
// load-bearing. The test above asserts an outcome, and an outcome can be reached
// without the lock: a naive build (same transaction, same count, same insert, no
// lock) was measured and it PASSES that test, because the count-to-insert window
// is a few microseconds and the goroutines often do not genuinely overlap.
//
// So this asserts the serialisation directly, by making it observable.
//
// The shape is the same control test the ledger uses at
// ledger_pg_test.go:TestAdvisoryLockSerialisesConcurrentWriters. A first
// transaction takes the allowance row and holds it; a second ConsumeAllowance
// for the same user is fired and must NOT return until the first commits. That is
// the property, stated as a fact about the database rather than as a statistical
// outcome: the count and the insert are in one transaction that is serialised
// against every other consumer of the same user, so the second one's count is
// guaranteed to run after the first one's INSERT has committed.
//
// A timing assertion is used deliberately. The alternative — asserting the
// outcome N times — cannot distinguish "serialised" from "usually fast enough",
// which is exactly the confusion this test exists to remove.
func TestConsumeAllowanceBlocksOnTheAllowanceRow(t *testing.T) {
	const userID = 710
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, nil)
	now := time.Now().UTC()

	if _, err := svc.EnsureAllowance(ctx, userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}

	// The blocker: a transaction that takes the same per-user advisory lock the
	// service takes, then holds it. Everything a ConsumeAllowance does happens
	// behind that lock, so this is not a synthetic stand-in for it — it IS it.
	blocked := make(chan struct{})
	release := make(chan struct{})
	blockerDone := make(chan error, 1)
	go func() {
		blockerDone <- db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(
				`SELECT pg_advisory_xact_lock(hashtextextended('coin:user:' || CAST(? AS text), 0))`,
				fmt.Sprintf("%d", userID),
			).Error; err != nil {
				return err
			}
			close(blocked)
			<-release
			return nil
		})
	}()
	<-blocked

	// The contended call. It must wait.
	consumeDone := make(chan error, 1)
	go func() {
		_, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 9901, now)
		consumeDone <- err
	}()

	select {
	case err := <-consumeDone:
		t.Fatalf("ConsumeAllowance returned %v while the per-user lock was held by another transaction. "+
			"The count-and-insert is NOT serialised, which is the double-spend this domain exists to prevent: "+
			"two concurrent requests would both read \"1 remaining\" and both insert.", err)
	case <-time.After(500 * time.Millisecond):
		// Correct: it is waiting for the lock. The wait is the proof.
	}

	// Release, and the call must then complete normally rather than timing out
	// against the lock_timeout set by InUserTx.
	close(release)
	select {
	case err := <-consumeDone:
		if err != nil {
			t.Errorf("ConsumeAllowance after the lock was released = %v, want success", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ConsumeAllowance never completed after the blocking transaction committed")
	}
	if err := <-blockerDone; err != nil {
		t.Fatalf("the blocking transaction: %v", err)
	}

	// And the contended consume really did write its row, so the test is not
	// passing because the call did nothing.
	if n := countUnlocks(t, db,
		`SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 9901`, userID); n != 1 {
		t.Errorf("the contended ConsumeAllowance wrote %d rows, want 1", n)
	}
}

// The same guarantee when the race is over the FIRST allowance rather than the
// last: seven concurrent first-timers, one row between them, and exactly one
// consume. This is the case where the allowance row does not exist yet, so it is
// the one place uq_user_free_allowance_user rather than the advisory lock is what
// separates the creators.
func TestConcurrentFirstEverConsumeCreatesOneAllowanceAndOneUnlock(t *testing.T) {
	const (
		userID     = 701
		concurrent = 7
	)
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, nil)
	now := time.Now().UTC()

	var ready, done sync.WaitGroup
	ready.Add(concurrent)
	done.Add(concurrent)
	start := make(chan struct{})
	var mu sync.Mutex
	var succeeded int
	var otherErrors []error

	for i := 0; i < concurrent; i++ {
		go func(i int) {
			defer done.Done()
			resourceID := uint64(2000 + i)
			ready.Done()
			<-start
			_, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeVideo, resourceID, now)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				succeeded++
			case errors.Is(err, ErrNoAllowanceRemaining), errors.Is(err, ErrAlreadyUnlocked):
				// Either refusal is a legitimate outcome: the default video
				// allowance is 1, so at most one can win, and the slot also
				// stops a duplicate resource. What must not happen is two.
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
	// Default video_unlocks is 1, and no allowance row existed at the start.
	if succeeded != 1 {
		t.Errorf("got %d successes for a brand-new user with a default video allowance of 1, want exactly 1", succeeded)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("unlock rows for user %d = %d, want 1", userID, n)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM user_free_allowance WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("allowance rows for user %d = %d, want 1 — two first-timers must not produce two starter allowances", userID, n)
	}
}

// Once two rows for one triple are legal, every read that matched on the triple
// with no filter and no ordering stops having a defined answer, and that is not
// a theoretical concern — it is the change this whole test file exists to cover.
// A read that returns a year-old revoked row to a caller expecting an active
// unlock is a silent wrong answer, so the readers are pinned here.
//
// This is a service-level test rather than a raw-SQL one because the point is
// which row each CALLER receives, and the callers are the service methods.
func TestReadersReturnTheRightRowWhenATripleHoldsARevokedAndALiveOne(t *testing.T) {
	const userID = 530
	ctx := context.Background()
	db := openEntitlementSchema(t)
	repo := NewRepository(db)
	svc := testEntitlements(t, db, nil)
	now := time.Now().UTC()
	first := time.Now().UTC().Add(-72 * time.Hour)
	journalA := spendJournal(t, db, "readers-fixture-a")
	journalB := spendJournal(t, db, "readers-fixture-b")

	// Buy, revoke, buy again — the exact shape the partial index now permits.
	if _, err := svc.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, 555, journalA, 40, first); err != nil {
		t.Fatalf("the first purchase: %v", err)
	}
	if err := svc.Revoke(ctx, userID, ResourceTypeStudyResource, 555, "an old mistake", first); err != nil {
		t.Fatalf("revoking the first: %v", err)
	}
	second, err := svc.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, 555, journalB, 40, now)
	if err != nil {
		t.Fatalf("the re-buy: %v", err)
	}

	// FindUnlock: the LIVE row, even though the revoked one has the same triple
	// and a lower id. Without ORDER BY revoked_at NULLS FIRST this is whatever
	// the planner reached first, and a caller reading the revocation's journal
	// would be told about the wrong purchase.
	row, err := repo.FindUnlock(ctx, userID, ResourceTypeStudyResource, 555)
	if err != nil {
		t.Fatalf("FindUnlock: %v", err)
	}
	if row == nil {
		t.Fatal("FindUnlock returned nil for a resource the user currently holds")
	}
	if row.ID != second.ID {
		t.Errorf("FindUnlock returned row %d, want the live row %d", row.ID, second.ID)
	}
	if row.RevokedAt != nil {
		t.Errorf("FindUnlock returned a revoked row (%v) while a live one exists", row.RevokedAt)
	}
	if row.JournalID == nil || *row.JournalID != journalB {
		t.Errorf("FindUnlock returned journal %v, want the current purchase's %s", row.JournalID, journalB)
	}

	// With the live row revoked too, the triple is entirely revoked and
	// FindUnlock must still answer — with the OLDEST, so a caller asking about
	// the history starts at the beginning of it.
	if err := svc.Revoke(ctx, userID, ResourceTypeStudyResource, 555, "a second mistake", now); err != nil {
		t.Fatalf("revoking the second: %v", err)
	}
	row, err = repo.FindUnlock(ctx, userID, ResourceTypeStudyResource, 555)
	if err != nil {
		t.Fatalf("FindUnlock after both revocations: %v", err)
	}
	if row == nil {
		t.Fatal("FindUnlock returned nil; a revoked row is still a row")
	}
	if row.JournalID == nil || *row.JournalID != journalA {
		t.Errorf("FindUnlock returned journal %v, want the first purchase's %s — the history should read from the start", row.JournalID, journalA)
	}

	// Revoke's own error path. This is the assertion that ReadUnlock could not
	// have been filtered to live-only: a second Revoke updates nothing, and
	// asking for a live row would come back nil and be reported as ErrNotFound
	// — "you never had this" about a resource the user bought twice.
	err = svc.Revoke(ctx, userID, ResourceTypeStudyResource, 555, "a friendlier reason", now.Add(time.Minute))
	if !errors.Is(err, ErrUnlockRevoked) {
		t.Fatalf("a third revocation = %v, want ErrUnlockRevoked — with two revocations on the triple the "+
			"error must name the LATEST one, not the first", err)
	}
	// The first revocation is still the record of the first decision and has not
	// been overwritten by either of the later ones.
	if n := countUnlocks(t, db,
		`SELECT count(*) FROM resource_unlock
		  WHERE user_id = ? AND revoke_reason = 'an old mistake'`, userID); n != 1 {
		t.Errorf("%d rows carry the first revocation's reason, want 1 — a later revocation overwrote it", n)
	}
	if n := countUnlocks(t, db,
		`SELECT count(*) FROM resource_unlock
		  WHERE user_id = ? AND revoke_reason = 'a friendlier reason'`, userID); n != 0 {
		t.Errorf("%d rows carry the third revocation's reason, want 0 — RevokeUnlock is a compare-and-set", n)
	}

	// A triple that was never held at all still answers nil, so ErrNotFound and
	// ErrUnlockRevoked remain distinguishable.
	if err := svc.Revoke(ctx, userID, ResourceTypeStudyResource, 999999, "x", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoking an unheld resource = %v, want ErrNotFound", err)
	}
}

// ── the service behaviour, against the rows ──────────────────────────────────

func TestEnsureAllowanceIsIdempotentAndKeepsTheOriginalGrant(t *testing.T) {
	const userID = 702
	ctx := context.Background()
	db := openEntitlementSchema(t)
	grantedAt := time.Date(2026, 9, 27, 4, 12, 0, 0, time.UTC)

	first := testEntitlements(t, db, func(c *EconomyConfig) {
		c.Allowance.DocumentUnlocks = 3
		c.Allowance.ExpiresInDays = 30
	})
	row, err := first.EnsureAllowance(ctx, userID, grantedAt)
	if err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	if row.DocumentUnlocks != 3 {
		t.Errorf("document unlocks = %d, want 3 from the config", row.DocumentUnlocks)
	}
	if want := grantedAt.AddDate(0, 0, 30); !row.ExpiresAt.Equal(want) {
		t.Errorf("expires at %s, want %s (granted_at + expires_in_days)", row.ExpiresAt, want)
	}

	// A second call under a config that grants MORE must not resize the existing
	// row. Re-granting is how "once per lifetime" stops being true, and an admin
	// raising a default would otherwise hand five document unlocks to every user
	// who happened to hit a code path again.
	second := testEntitlements(t, db, func(c *EconomyConfig) {
		c.Allowance.DocumentUnlocks = 9
		c.Allowance.ExpiresInDays = 365
	})
	again, err := second.EnsureAllowance(ctx, userID, grantedAt.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("second EnsureAllowance: %v", err)
	}
	if again.ID != row.ID {
		t.Errorf("the second call created a new row (id %d, was %d)", again.ID, row.ID)
	}
	if again.DocumentUnlocks != 3 {
		t.Errorf("document unlocks after a re-grant = %d, want the original 3 — a raised default is not a re-grant", again.DocumentUnlocks)
	}
	if !again.GrantedAt.Equal(grantedAt) {
		t.Errorf("granted_at moved to %s, want the original %s — a re-grant would extend the window forever", again.GrantedAt, grantedAt)
	}
	if want := grantedAt.AddDate(0, 0, 30); !again.ExpiresAt.Equal(want) {
		t.Errorf("expires_at = %s, want the original %s", again.ExpiresAt, want)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM user_free_allowance WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("allowance rows = %d, want 1", n)
	}
}

func TestHasAccessIsTrueAfterACoinUnlockAndFalseAfterRevoke(t *testing.T) {
	const (
		userID     = 703
		resourceID = uint64(812)
	)
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, nil)
	now := time.Now().UTC()
	journal := spendJournal(t, db, "has-access-fixture")

	// Before anything: no access. The idempotency check every gate calls first.
	has, err := svc.HasAccess(ctx, userID, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("HasAccess before: %v", err)
	}
	if has {
		t.Fatal("HasAccess = true before anything was unlocked")
	}

	unlock, err := svc.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, resourceID, journal, 40, now)
	if err != nil {
		t.Fatalf("RecordCoinUnlock: %v", err)
	}
	if unlock.Source != UnlockSourceCoins || unlock.CoinsPaid != 40 {
		t.Errorf("stored source/coins_paid = %s/%d, want COINS/40", unlock.Source, unlock.CoinsPaid)
	}
	if unlock.JournalID == nil || *unlock.JournalID != journal {
		t.Errorf("stored journal_id = %v, want %s", unlock.JournalID, journal)
	}

	has, err = svc.HasAccess(ctx, userID, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("HasAccess after: %v", err)
	}
	if !has {
		t.Error("HasAccess = false straight after a coin unlock")
	}

	// A different class on the same resource id is a different resource. If this
	// returned true, one paid unlock would grant a video for free.
	has, err = svc.HasAccess(ctx, userID, ResourceTypeVideo, resourceID)
	if err != nil {
		t.Fatalf("HasAccess for another class: %v", err)
	}
	if has {
		t.Error("HasAccess = true for a different resource_type on the same id")
	}

	// Another user is a different user.
	has, err = svc.HasAccess(ctx, userID+1, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("HasAccess for another user: %v", err)
	}
	if has {
		t.Error("HasAccess = true for a different user")
	}

	if err := svc.Revoke(ctx, userID, ResourceTypeStudyResource, resourceID, "chargeback", now); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	has, err = svc.HasAccess(ctx, userID, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("HasAccess after revoke: %v", err)
	}
	if has {
		t.Error("HasAccess = true after a revocation")
	}

	// The row is still there. A revocation is a reversal, not a delete, and the
	// record of what was once granted is the whole reason.
	row, err := NewRepository(db).FindUnlock(ctx, userID, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("FindUnlock: %v", err)
	}
	if row == nil {
		t.Fatal("the unlock row was deleted by Revoke; a revocation must never be a delete")
	}
	if row.RevokedAt == nil || row.RevokeReason == nil || *row.RevokeReason != "chargeback" {
		t.Errorf("revocation = %v / %v, want a timestamp and the reason 'chargeback'", row.RevokedAt, row.RevokeReason)
	}

	// A second revocation changes nothing, and says so.
	if err := svc.Revoke(ctx, userID, ResourceTypeStudyResource, resourceID, "a nicer reason", now.Add(time.Minute)); err == nil {
		t.Error("a second revocation was accepted; the first one is the record")
	} else if !errors.Is(err, ErrUnlockRevoked) {
		t.Errorf("a second revocation = %v, want ErrUnlockRevoked", err)
	}
	row, err = NewRepository(db).FindUnlock(ctx, userID, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("FindUnlock after the second revocation: %v", err)
	}
	if row.RevokeReason == nil || *row.RevokeReason != "chargeback" {
		t.Errorf("the second revocation overwrote the first: reason is now %v, want 'chargeback'", row.RevokeReason)
	}

	// Revoking something the user never had is a different answer, and a caller
	// that cannot tell them apart retries forever.
	if err := svc.Revoke(ctx, userID, ResourceTypeStudyResource, 999999, "chargeback", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoking an unheld resource = %v, want ErrNotFound", err)
	}
}

func TestRecordCoinUnlockIsIdempotentOnARepeat(t *testing.T) {
	const (
		userID     = 704
		resourceID = uint64(812)
	)
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, nil)
	now := time.Now().UTC()
	journal := spendJournal(t, db, "idempotent-unlock-fixture")

	first, err := svc.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, resourceID, journal, 40, now)
	if err != nil {
		t.Fatalf("first RecordCoinUnlock: %v", err)
	}

	// The mobile retry. Same journal, same everything: the original row comes
	// back, nothing is written, and nothing is charged. 03-api-contract.md §2.3
	// makes this the normal case rather than the exception — a 403 here would
	// render a failure for an outcome that succeeded.
	second, err := svc.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, resourceID, journal, 40, now)
	if err != nil {
		t.Fatalf("repeat RecordCoinUnlock: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("the repeat created row %d, want the original %d", second.ID, first.ID)
	}
	if second.UnlockedAt.Unix() != first.UnlockedAt.Unix() {
		t.Errorf("the repeat moved unlocked_at to %s, want the original %s", second.UnlockedAt, first.UnlockedAt)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("unlock rows after a repeat = %d, want 1", n)
	}

	// A DIFFERENT journal for a resource somebody already paid for is not a
	// retry. It is either a double charge or a bug, and returning success would
	// hide both.
	other := spendJournal(t, db, "idempotent-unlock-fixture-2")
	if _, err := svc.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, resourceID, other, 40, now); !errors.Is(err, ErrAlreadyUnlocked) {
		t.Errorf("a second payment for one resource = %v, want ErrAlreadyUnlocked", err)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("unlock rows after a second payment attempt = %d, want 1", n)
	}

	// The retry must not consume an allowance either, and must not be a
	// revocation target. The row is untouched.
	row, err := NewRepository(db).FindUnlock(ctx, userID, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("FindUnlock: %v", err)
	}
	if row.RevokedAt != nil {
		t.Error("a repeat RecordCoinUnlock revoked the row; it is a read, not a mutation")
	}
	if row.CoinsPaid != 40 {
		t.Errorf("coins_paid = %d after the attempts, want the original 40 snapshot", row.CoinsPaid)
	}
}

// A price change must not retroactively re-price history. The unlock is 40 coins
// because that is what the ledger charged, and no config read can change it.
func TestCoinsPaidIsASnapshotNotALookup(t *testing.T) {
	const (
		userID     = 705
		resourceID = uint64(812)
	)
	ctx := context.Background()
	db := openEntitlementSchema(t)
	now := time.Now().UTC()
	journal := spendJournal(t, db, "snapshot-fixture")

	cheap := testEntitlements(t, db, func(c *EconomyConfig) { c.Prices.StudyResource = 40 })
	if _, err := cheap.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, resourceID, journal, 40, now); err != nil {
		t.Fatalf("RecordCoinUnlock at 40: %v", err)
	}

	// The price doubles. The historical unlock must still read 40.
	dear := testEntitlements(t, db, func(c *EconomyConfig) { c.Prices.StudyResource = 80 })
	replay, err := dear.RecordCoinUnlock(ctx, userID, ResourceTypeStudyResource, resourceID, journal, 40, now)
	if err != nil {
		t.Fatalf("replay after the price change: %v", err)
	}
	if replay.CoinsPaid != 40 {
		t.Errorf("coins_paid = %d after the price moved to 80, want the 40 snapshot", replay.CoinsPaid)
	}
	row, err := NewRepository(db).FindUnlock(ctx, userID, ResourceTypeStudyResource, resourceID)
	if err != nil {
		t.Fatalf("FindUnlock: %v", err)
	}
	if row.CoinsPaid != 40 {
		t.Errorf("the stored snapshot is %d, want 40 — re-deriving the price at query time is the bug in 02-architecture.md §5", row.CoinsPaid)
	}
}

func TestConsumeAllowanceDerivesUsedAndHonoursExpiry(t *testing.T) {
	const userID = 706
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, func(c *EconomyConfig) {
		c.Allowance.DocumentUnlocks = 3
		c.Allowance.ExpiresInDays = 30
	})
	granted := time.Date(2026, 9, 27, 4, 12, 0, 0, time.UTC)

	// A user with no allowance row yet: RemainingAllowance reports zeros and
	// creates nothing.
	status, err := svc.RemainingAllowance(ctx, userID, granted)
	if err != nil {
		t.Fatalf("RemainingAllowance before any grant: %v", err)
	}
	if status.TotalRemaining != 0 || status.Expired {
		t.Errorf("a user with no allowance row = %+v, want all zero and not expired", status)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM user_free_allowance WHERE user_id = ?`, userID); n != 0 {
		t.Fatalf("RemainingAllowance created %d allowance rows; a read must not write", n)
	}

	if _, err := svc.EnsureAllowance(ctx, userID, granted); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	// Two burned of three.
	for i := 0; i < 2; i++ {
		if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, uint64(3000+i), granted); err != nil {
			t.Fatalf("ConsumeAllowance %d: %v", i, err)
		}
	}
	status, err = svc.RemainingAllowance(ctx, userID, granted)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	if got := status.Classes[ResourceTypeStudyResource]; got.Granted != 3 || got.Used != 2 || got.Remaining != 1 {
		t.Errorf("documents = %d/%d/%d, want 3/2/1", got.Granted, got.Used, got.Remaining)
	}
	// The video allowance is untouched by two document unlocks, which is the
	// per-class half of "three documents, one video, one mock test".
	if got := status.Classes[ResourceTypeVideo]; got.Granted != 1 || got.Used != 0 || got.Remaining != 1 {
		t.Errorf("video = %d/%d/%d, want 1/0/1", got.Granted, got.Used, got.Remaining)
	}

	// The last one, and then the class is exhausted.
	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 3002, granted); err != nil {
		t.Fatalf("consuming the last document unlock: %v", err)
	}
	_, err = svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 3003, granted)
	if !errors.Is(err, ErrNoAllowanceRemaining) {
		t.Fatalf("a fourth document unlock = %v, want ErrNoAllowanceRemaining", err)
	}

	// A different class still has its own allowance, so a student who used all
	// three documents can still take a free video.
	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeVideo, 4001, granted); err != nil {
		t.Fatalf("a video unlock after exhausting the documents: %v", err)
	}

	// The expiry boundary, against the real rows: ExpiresAt is exactly
	// granted_at + 30 days, and a consume at that instant is refused.
	expires := granted.AddDate(0, 0, 30)
	_, err = svc.ConsumeAllowance(ctx, userID, ResourceTypeMockTest, 5001, expires)
	if !errors.Is(err, ErrNoAllowanceRemaining) {
		t.Errorf("a consume at exactly ExpiresAt = %v, want ErrNoAllowanceRemaining", err)
	}
	if !errors.Is(err, ErrAllowanceExpired) {
		t.Errorf("a consume at exactly ExpiresAt = %v, want it to ALSO match ErrAllowanceExpired — the 423 case is matched on that sentinel", err)
	}
	// And the refusals wrote nothing.
	if n := countUnlocks(t, db,
		`SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id IN (3003, 5001)`, userID); n != 0 {
		t.Errorf("%d rows exist for refused consumes; a refused transaction must leave nothing behind", n)
	}

	// The same read, one microsecond earlier, succeeds — which is what makes the
	// boundary a boundary rather than a rounding artefact.
	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeMockTest, 5002, expires.Add(-time.Microsecond)); err != nil {
		t.Errorf("a consume one microsecond before ExpiresAt = %v, want it to succeed", err)
	}
}

// A revocation gives the allowance unlock back, because "used" is derived from
// un-revoked rows. This is the test that a stored counter would fail, and it is
// why there is no `used` column.
func TestRevokingAnAllowanceUnlockReturnsItToTheAllowance(t *testing.T) {
	const userID = 707
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, func(c *EconomyConfig) { c.Allowance.DocumentUnlocks = 1 })
	now := time.Now().UTC()

	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 6001, now); err != nil {
		t.Fatalf("ConsumeAllowance: %v", err)
	}
	status, err := svc.RemainingAllowance(ctx, userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	if got := status.Remaining(ResourceTypeStudyResource); got != 0 {
		t.Fatalf("remaining after consuming the only unlock = %d, want 0", got)
	}
	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 6002, now); !errors.Is(err, ErrNoAllowanceRemaining) {
		t.Fatalf("a second document unlock = %v, want ErrNoAllowanceRemaining", err)
	}

	if err := svc.Revoke(ctx, userID, ResourceTypeStudyResource, 6001, "resource was removed", now); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	status, err = svc.RemainingAllowance(ctx, userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance after the revocation: %v", err)
	}
	if got := status.Remaining(ResourceTypeStudyResource); got != 1 {
		t.Errorf("remaining after a revocation = %d, want 1 — a derived count notices a revocation, and a stored one would not", got)
	}
	// And it is really usable again.
	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 6002, now); err != nil {
		t.Errorf("consuming after the revocation = %v, want success", err)
	}
}

// Burning the same resource twice is a duplicate, not an exhaustion, and the
// difference matters: a duplicate means the caller should look at what it
// already has, an exhaustion means the student needs to earn coins.
func TestConsumingTheSameResourceTwiceIsADuplicateNotAnExhaustion(t *testing.T) {
	const userID = 708
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, func(c *EconomyConfig) { c.Allowance.DocumentUnlocks = 3 })
	now := time.Now().UTC()

	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 7001, now); err != nil {
		t.Fatalf("first ConsumeAllowance: %v", err)
	}
	_, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 7001, now)
	if !errors.Is(err, ErrAlreadyUnlocked) {
		t.Fatalf("consuming the same resource twice = %v, want ErrAlreadyUnlocked", err)
	}
	// The failed duplicate must not have burned a second allowance.
	status, err := svc.RemainingAllowance(ctx, userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	if got := status.Remaining(ResourceTypeStudyResource); got != 2 {
		t.Errorf("remaining after a duplicate = %d, want 2 — a refused duplicate consumes nothing", got)
	}
	if n := countUnlocks(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("unlock rows = %d, want 1", n)
	}
}

// The read that runs before a spend. It must be cheap, it must be scoped to the
// user, and it must be true — a gate that calls it and is told "no" when the
// answer is "yes" charges a student for something they already own.
func TestHasAccessMatchesTheStoredRows(t *testing.T) {
	const userID = 709
	ctx := context.Background()
	db := openEntitlementSchema(t)
	svc := testEntitlements(t, db, func(c *EconomyConfig) { c.Allowance.DocumentUnlocks = 1 })
	now := time.Now().UTC()

	if _, err := svc.ConsumeAllowance(ctx, userID, ResourceTypeStudyResource, 8001, now); err != nil {
		t.Fatalf("ConsumeAllowance: %v", err)
	}
	// The plan for the rows the method will report on, asserted against the table
	// rather than against itself.
	rows := unlockRows(t, db, userID)
	if len(rows) != 1 {
		t.Fatalf("got %d rows for user %d, want 1", len(rows), userID)
	}
	has, err := svc.HasAccess(ctx, userID, ResourceTypeStudyResource, 8001)
	if err != nil {
		t.Fatalf("HasAccess: %v", err)
	}
	if !has {
		t.Error("HasAccess disagrees with the one stored row")
	}
	if rows[0].RevokedAt != nil {
		t.Error("an allowance unlock should not be revoked")
	}
	if rows[0].JournalID != nil {
		t.Error("an allowance unlock must carry no journal; no coins moved")
	}
	if rows[0].CoinsPaid != 0 {
		t.Errorf("an allowance unlock recorded coins_paid = %d, want 0", rows[0].CoinsPaid)
	}
	if rows[0].Source != UnlockSourceAllowance {
		t.Errorf("source = %q, want %q", rows[0].Source, UnlockSourceAllowance)
	}
}

// ── small helpers ────────────────────────────────────────────────────────────

// mustUnlockExec is the local name for the fixture INSERTs, kept distinct from
// mustExec in schema_pg_test.go so it is obvious at a glance which harness a
// failure came from.
func mustUnlockExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
