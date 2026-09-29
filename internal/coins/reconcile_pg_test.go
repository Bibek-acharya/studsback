//go:build coinsintegration

// internal/coins/reconcile_pg_test.go
//
// The reconciliation job only earns its keep if it CATCHES corruption, so these
// tests deliberately corrupt a healthy ledger in each of the four ways the job
// claims to detect, and assert the matching invariant fails.
//
// A test that only asserts "a clean ledger is healthy" would pass against a
// job whose SQL was quietly wrong, which is the failure mode that makes a
// monitoring job worse than none: it reports all-clear while the ledger drifts.
package coins

import (
	"context"
	"testing"
	"time"

	"studsphere/backend/internal/shared/config"

	"gorm.io/gorm"
)

// resultFor returns the named invariant's result, failing the test if absent.
func resultFor(t *testing.T, r ReconcileReport, name InvariantName) InvariantResult {
	t.Helper()
	for _, res := range r.Results {
		if res.Name == name {
			return res
		}
	}
	t.Fatalf("invariant %s missing from report", name)
	return InvariantResult{}
}

func TestReconcilePassesOnAFreshLedger(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		report := RunReconcile(context.Background(), db)
		if !report.Healthy {
			t.Fatalf("fresh ledger reported unhealthy: %+v", report.Results)
		}
		if len(report.Results) != len(AllInvariants) {
			t.Errorf("ran %d invariants, want %d", len(report.Results), len(AllInvariants))
		}
		for _, r := range report.Results {
			if !r.OK() {
				t.Errorf("%s not ok: violations=%d err=%v", r.Name, r.Violations, r.Err)
			}
		}
	})
}

func TestReconcilePassesAfterARealGrantAndSpend(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		ledger := testLedger(t, db, nil)
		ctx := context.Background()

		// Amounts come from the config, never from a literal, mirroring how the
		// ledger's own callers must behave.
		if _, err := ledger.Grant(ctx, GrantRequest{
			UserID:         5001,
			ReasonCode:     ReasonResourceApproved,
			IdempotencyKey: "recon:grant:5001",
			CreatedBy:      "test",
		}); err != nil {
			t.Fatalf("grant: %v", err)
		}
		// ref_type requires a ref_id: chk_coin_journal_ref enforces it, and the
		// ledger rejects the request rather than writing a dangling reference.
		resourceID := uint64(777)
		if _, err := ledger.Spend(ctx, SpendRequest{
			UserID:         5001,
			ReasonCode:     ReasonResourceUnlock,
			RefType:        "study_resource",
			RefID:          &resourceID,
			IdempotencyKey: "recon:spend:5001",
			CreatedBy:      "test",
		}); err != nil {
			t.Fatalf("spend: %v", err)
		}

		report := RunReconcile(ctx, db)
		if !report.Healthy {
			t.Fatalf("ledger unhealthy after a legitimate grant and spend: %+v", report.Results)
		}
	})
}

// The drift check. Tampering with the cached balance is exactly the failure the
// missing UNIQUE (account_id, account_seq) constraint would have caused, and it
// is the one this job exists to catch.
func TestReconcileCatchesCacheDrift(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		ledger := testLedger(t, db, nil)
		ctx := context.Background()
		if _, err := ledger.Grant(ctx, GrantRequest{
			UserID:         5002,
			ReasonCode:     ReasonResourceApproved,
			IdempotencyKey: "recon:grant:5002",
			CreatedBy:      "test",
		}); err != nil {
			t.Fatalf("grant: %v", err)
		}
		if report := RunReconcile(ctx, db); !report.Healthy {
			t.Fatalf("expected healthy before tampering: %+v", report.Results)
		}

		// Corrupt only the cache, leaving coin_posting untouched.
		if err := db.Exec(
			`UPDATE coin_account_balance SET posted_balance = posted_balance + 999 WHERE liability`,
		).Error; err != nil {
			t.Fatalf("tamper: %v", err)
		}

		report := RunReconcile(ctx, db)
		if report.Healthy {
			t.Fatal("cache drift was not detected")
		}
		drift := resultFor(t, report, InvariantCacheMatchesPostings)
		if drift.OK() {
			t.Error("cache drift must fail InvariantCacheMatchesPostings")
		}
		// Global conservation should also notice, since the sums no longer cancel.
		if got := resultFor(t, report, InvariantGlobalConservation); got.OK() {
			t.Error("global conservation must also fail once the cache is tampered")
		}
	})
}

// A posting that does not net to zero, which double-entry forbids.
func TestReconcileCatchesUnbalancedJournal(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		ledger := testLedger(t, db, nil)
		ctx := context.Background()
		if _, err := ledger.Grant(ctx, GrantRequest{
			UserID:         5003,
			ReasonCode:     ReasonResourceApproved,
			IdempotencyKey: "recon:grant:5003",
			CreatedBy:      "test",
		}); err != nil {
			t.Fatalf("grant: %v", err)
		}

		// The append-only trigger blocks UPDATE/DELETE on coin_posting, so
		// corrupt via a direct insert of an extra leg instead, which is exactly
		// the shape a missing-constraint bug would have produced.
		var journalID string
		var accountID uint
		db.Raw(`SELECT id FROM coin_journal WHERE idempotency_key = 'recon:grant:5003'`).Scan(&journalID)
		db.Raw(`SELECT id FROM coin_account WHERE kind='SYSTEM' LIMIT 1`).Scan(&accountID)
		nextSeq := int16(0)
		db.Raw(`SELECT coalesce(max(seq),0)::smallint FROM coin_posting WHERE journal_id = ?`, journalID).Scan(&nextSeq)
		var maxAcctSeq int64
		db.Raw(`SELECT coalesce(max(account_seq),0) FROM coin_posting`).Scan(&maxAcctSeq)

		if err := db.Exec(
			`INSERT INTO coin_posting (journal_id, seq, account_id, amount, account_seq, created_at)
			 VALUES (?, ?, ?, 4242, ?, ?)`,
			journalID, nextSeq+1, accountID, maxAcctSeq+1, time.Now().UTC(),
		).Error; err != nil {
			t.Fatalf("inject unbalanced leg: %v", err)
		}

		report := RunReconcile(ctx, db)
		if report.Healthy {
			t.Fatal("an unbalanced journal was not detected")
		}
		if got := resultFor(t, report, InvariantJournalsBalance); got.OK() {
			t.Error("must fail InvariantJournalsBalance")
		}
	})
}

func TestReconcileCatchesOverdraft(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		ledger := testLedger(t, db, nil)
		ctx := context.Background()
		if _, err := ledger.Grant(ctx, GrantRequest{
			UserID:         5004,
			ReasonCode:     ReasonResourceApproved,
			IdempotencyKey: "recon:grant:5004",
			CreatedBy:      "test",
		}); err != nil {
			t.Fatalf("grant: %v", err)
		}

		// chk_coin_no_overdraft makes this state UNREACHABLE through normal
		// writes, which is the constraint doing its job rather than a test
		// obstacle. Drop the CHECK to get past it, which is exactly the
		// "constraint is missing" scenario this job is the backstop for, and
		// restore it afterwards.
		if err := db.Exec(
			`ALTER TABLE coin_account_balance DROP CONSTRAINT chk_coin_no_overdraft`,
		).Error; err != nil {
			t.Fatalf("drop overdraft check: %v", err)
		}
		t.Cleanup(func() {
			db.Exec(`DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint
                  WHERE conname = 'chk_coin_no_overdraft'
                    AND conrelid = 'coin_account_balance'::regclass) THEN
    ALTER TABLE coin_account_balance
      ADD CONSTRAINT chk_coin_no_overdraft
      CHECK (NOT liability OR posted_balance - reserved >= 0);
  END IF;
END $$`)
		})
		if err := db.Exec(
			`UPDATE coin_account_balance SET reserved = posted_balance + 50 WHERE liability`,
		).Error; err != nil {
			t.Fatalf("force overdraft via reserved: %v", err)
		}

		report := RunReconcile(ctx, db)
		if report.Healthy {
			t.Fatal("an overdraft was not detected")
		}
		if got := resultFor(t, report, InvariantNoOverdraft); got.OK() {
			t.Error("must fail InvariantNoOverdraft")
		}
	})
}

// The streak counter is the number worth alerting on: one bad run can be a blip,
// a run of them is an incident.
func TestReconcileStreakIncrementsAndResets(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		// runOnce logs, and the shared logger reads config.AppConfig.GinMode
		// during lazy Init. AppConfig is a nil pointer until config is loaded,
		// so this would panic. Same pattern as the notification tests.
		oldConfig := config.AppConfig
		config.AppConfig = &config.Config{}
		t.Cleanup(func() { config.AppConfig = oldConfig })

		consecutiveFailures.Store(0)

		// A liability account has to exist before it can be tampered with: a
		// fresh schema holds only the three system accounts, all of which are
		// liability=false, so `WHERE liability` would match nothing.
		ledger := testLedger(t, db, nil)
		if _, err := ledger.Grant(context.Background(), GrantRequest{
			UserID:         5005,
			ReasonCode:     ReasonResourceApproved,
			IdempotencyKey: "recon:grant:5005",
			CreatedBy:      "test",
		}); err != nil {
			t.Fatalf("grant: %v", err)
		}

		// Two unhealthy runs in a row.
		if err := db.Exec(
			`UPDATE coin_account_balance SET posted_balance = posted_balance + 1 WHERE liability`,
		).Error; err != nil {
			t.Fatalf("tamper: %v", err)
		}
		if n := countRows(t, db, `SELECT count(*) FROM coin_account_balance WHERE liability`); n == 0 {
			t.Fatal("expected a liability account to tamper with")
		}
		runOnce(db, time.Minute)
		if got := ConsecutiveReconcileFailures(); got != 1 {
			t.Errorf("streak = %d after one bad run, want 1", got)
		}
		runOnce(db, time.Minute)
		if got := ConsecutiveReconcileFailures(); got != 2 {
			t.Errorf("streak = %d after two bad runs, want 2", got)
		}

		// Repair by rebuilding the cache from the postings, which is the only
		// repair that actually restores the invariant. Zeroing the cache would
		// not: the grant left a real balance, so the cache would still disagree
		// with coin_posting and the drift check would correctly keep failing.
		if err := db.Exec(`
			UPDATE coin_account_balance b
			   SET posted_balance = coalesce(
			         (SELECT sum(amount)::bigint FROM coin_posting
			           WHERE account_id = b.account_id), 0)
		`).Error; err != nil {
			t.Fatalf("repair: %v", err)
		}
		if r := RunReconcile(context.Background(), db); !r.Healthy {
			t.Fatalf("ledger should be healthy after repair: %+v", r.Results)
		}
		runOnce(db, time.Minute)
		if got := ConsecutiveReconcileFailures(); got != 0 {
			t.Errorf("streak = %d after recovery, want 0", got)
		}
	})
}

// A database that cannot answer the check must report an error, not a clean
// pass. Silently treating "could not check" as "healthy" is how a monitoring
// job becomes decorative.
func TestReconcileReportsQueryErrors(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		// Point the check at a table that does not exist.
		if err := db.Exec(`ALTER TABLE coin_posting RENAME TO coin_posting_hidden`).Error; err != nil {
			t.Fatalf("rename: %v", err)
		}
		t.Cleanup(func() {
			db.Exec(`ALTER TABLE coin_posting_hidden RENAME TO coin_posting`)
		})

		report := RunReconcile(context.Background(), db)
		if report.Healthy {
			t.Fatal("a check that could not run must not report healthy")
		}
	})
}

// StartReconciler is what main.go actually calls, so the ticker path needs
// covering rather than only runOnce. A broken tick would leave the job silently
// never running, which looks identical to a healthy ledger.
func TestStartReconcilerTicks(t *testing.T) {
	withFreshSchema(t, func(db *gorm.DB) {
		oldConfig := config.AppConfig
		config.AppConfig = &config.Config{}
		t.Cleanup(func() { config.AppConfig = oldConfig })
		consecutiveFailures.Store(0)

		// Corrupt the cache so a tick has something to report, and assert the
		// streak advances without anyone calling runOnce.
		ledger := testLedger(t, db, nil)
		if _, err := ledger.Grant(context.Background(), GrantRequest{
			UserID:         5006,
			ReasonCode:     ReasonResourceApproved,
			IdempotencyKey: "recon:grant:5006",
			CreatedBy:      "test",
		}); err != nil {
			t.Fatalf("grant: %v", err)
		}
		if err := db.Exec(
			`UPDATE coin_account_balance SET posted_balance = posted_balance + 1 WHERE liability`,
		).Error; err != nil {
			t.Fatalf("tamper: %v", err)
		}

		stop := StartReconciler(db, 30*time.Millisecond, 5*time.Second)
		t.Cleanup(stop)

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if ConsecutiveReconcileFailures() > 0 {
				return // the ticker fired and observed the corruption
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("the reconciler ticker did not fire within 3s")
	})
}
