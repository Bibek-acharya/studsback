// internal/coins/reconcile.go
package coins

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// The reconciliation job proves the ledger's invariants still hold, on a
// schedule, in production.
//
// This is deliberately not a test suite. A test proves the invariants held on
// the day the test ran. This proves they still hold, and says so when they stop.
// The five checks below are the ones from 02-architecture.md section 11.
//
// Two of them exist because of failures that have actually happened in this
// codebase's wider history. A US payments company was assessed a 60-90M dollar
// shortfall across 100k+ consumers because its own records did not match its
// partner banks' records. A Federal Reserve enforcement action against another
// firm turned on recordkeeping. The cheapest insurance available here is five
// aggregate queries, and this file is that insurance.
//
// It never calls logger.Fatal. A data-integrity problem must not take the
// server down, and killing the process would destroy the evidence.

// InvariantName identifies one check in a report.
type InvariantName string

const (
	// InvariantJournalsBalance: every journal's postings sum to exactly zero.
	InvariantJournalsBalance InvariantName = "journals_net_zero"
	// InvariantGlobalConservation: the sum of every account's posted_balance
	// is exactly zero, because every grant has a matching system leg.
	InvariantGlobalConservation InvariantName = "global_conservation"
	// InvariantNoOverdraft: no liability account is below zero after reserved
	// is subtracted. System accounts are excluded by the liability flag and are
	// legitimately signed against us.
	InvariantNoOverdraft InvariantName = "no_liability_overdraft"
	// InvariantCacheMatchesPostings: coin_account_balance equals the sum of
	// its coin_posting legs. This is the drift check, and it is the one that
	// would have caught the missing UNIQUE constraints on the first day.
	InvariantCacheMatchesPostings InvariantName = "cache_matches_postings"
	// InvariantIdempotencyKeysUnique: no two journals share
	// (scope, idempotency_key). Backed by a unique index, so a violation means
	// the index is absent rather than that a duplicate slipped through.
	InvariantIdempotencyKeysUnique InvariantName = "idempotency_keys_unique"
)

// AllInvariants is the check set, in the order a report renders them.
var AllInvariants = []InvariantName{
	InvariantJournalsBalance,
	InvariantGlobalConservation,
	InvariantNoOverdraft,
	InvariantCacheMatchesPostings,
	InvariantIdempotencyKeysUnique,
}

// InvariantResult is one check's outcome. Violations is a count; Sample is a
// short excerpt of offending rows so an operator can act without a second query.
type InvariantResult struct {
	Name       InvariantName
	Violations int64
	Sample     string
	Duration   time.Duration
	Err        error
}

// ReconcileReport is one full pass over the five invariants.
type ReconcileReport struct {
	CheckedAt time.Time
	Duration  time.Duration
	Results   []InvariantResult
	Healthy   bool
}

// Failure returns the first failing result, or nil when the report is healthy.
func (r ReconcileReport) Failure() *InvariantResult {
	for i := range r.Results {
		if !r.Results[i].OK() {
			return &r.Results[i]
		}
	}
	return nil
}

// OK reports whether this individual check passed.
func (r InvariantResult) OK() bool { return r.Err == nil && r.Violations == 0 }

const reconcileSampleLimit = 5

// RunReconcile executes every invariant once and returns a report. It does not
// log; logging is the runner's job, so this is usable from a test and from a
// manual invocation.
//
// The SQL is intentionally plain and readable. These run at most hourly against
// a small table at launch scale, and when they get slow the per-check Duration in
// the report says which one, rather than anyone having to guess.
func RunReconcile(ctx context.Context, db *gorm.DB) ReconcileReport {
	started := time.Now()
	report := ReconcileReport{
		CheckedAt: started,
		Healthy:   true,
		Results:   make([]InvariantResult, 0, len(AllInvariants)),
	}

	checks := []struct {
		name InvariantName
		// query returns a single row with a violation count and, optionally,
		// a comma-joined sample of what failed.
		query func(ctx context.Context, db *gorm.DB) (int64, string, error)
	}{
		{
			name: InvariantJournalsBalance,
			query: func(ctx context.Context, db *gorm.DB) (int64, string, error) {
				var out struct {
					Count  int64
					Sample string
				}
				err := db.WithContext(ctx).Raw(`
					SELECT count(*)::bigint AS count,
					       coalesce(string_agg(journal_id::text, ', '), '') AS sample
					  FROM (
					        SELECT journal_id
					          FROM coin_posting
					         GROUP BY journal_id
					        HAVING sum(amount) <> 0
					         LIMIT ` + fmt.Sprint(reconcileSampleLimit) + `
				       ) AS offenders
				`).Scan(&out).Error
				return out.Count, out.Sample, err
			},
		},
		{
			name: InvariantGlobalConservation,
			query: func(ctx context.Context, db *gorm.DB) (int64, string, error) {
				// The violation count is the magnitude of the drift, so a
				// nonzero sum reads as "1 violation" rather than hiding the size.
				var sum int64
				if err := db.WithContext(ctx).Raw(
					`SELECT coalesce(sum(posted_balance), 0)::bigint FROM coin_account_balance`,
				).Scan(&sum).Error; err != nil {
					return 0, "", err
				}
				if sum == 0 {
					return 0, "", nil
				}
				abs := sum
				if abs < 0 {
					abs = -abs
				}
				return abs, fmt.Sprintf("sum(posted_balance) = %d", sum), nil
			},
		},
		{
			name: InvariantNoOverdraft,
			query: func(ctx context.Context, db *gorm.DB) (int64, string, error) {
				var out struct {
					Count  int64
					Sample string
				}
				err := db.WithContext(ctx).Raw(`
					SELECT count(*)::bigint AS count,
					       coalesce(string_agg(account_id::text, ', '), '') AS sample
					  FROM (
					        SELECT account_id
					          FROM coin_account_balance
					         WHERE liability AND posted_balance - reserved < 0
					         LIMIT ` + fmt.Sprint(reconcileSampleLimit) + `
				       ) AS offenders
				`).Scan(&out).Error
				return out.Count, out.Sample, err
			},
		},
		{
			name: InvariantCacheMatchesPostings,
			query: func(ctx context.Context, db *gorm.DB) (int64, string, error) {
				var out struct {
					Count  int64
					Sample string
				}
				err := db.WithContext(ctx).Raw(`
					SELECT count(*)::bigint AS count,
					       coalesce(string_agg(account_id::text, ', '), '') AS sample
					  FROM (
					        SELECT b.account_id
					          FROM coin_account_balance b
					          LEFT JOIN (
					            SELECT account_id, sum(amount)::bigint AS posted
					              FROM coin_posting GROUP BY account_id
					          ) p ON p.account_id = b.account_id
					         WHERE b.posted_balance <> coalesce(p.posted, 0)
					         LIMIT ` + fmt.Sprint(reconcileSampleLimit) + `
				       ) AS offenders
				`).Scan(&out).Error
				return out.Count, out.Sample, err
			},
		},
		{
			name: InvariantIdempotencyKeysUnique,
			query: func(ctx context.Context, db *gorm.DB) (int64, string, error) {
				var out struct {
					Count  int64
					Sample string
				}
				err := db.WithContext(ctx).Raw(`
					SELECT count(*)::bigint AS count,
					       coalesce(string_agg(scope || '/' || idempotency_key, ', '), '') AS sample
					  FROM (
					        SELECT scope, idempotency_key
					          FROM coin_journal
					         GROUP BY scope, idempotency_key
					        HAVING count(*) > 1
					         LIMIT ` + fmt.Sprint(reconcileSampleLimit) + `
				       ) AS offenders
				`).Scan(&out).Error
				return out.Count, out.Sample, err
			},
		},
	}

	for _, check := range checks {
		begin := time.Now()
		count, sample, err := check.query(ctx, db)
		result := InvariantResult{
			Name:       check.name,
			Violations: count,
			Sample:     sample,
			Duration:   time.Since(begin),
			Err:        err,
		}
		if !result.OK() {
			report.Healthy = false
		}
		report.Results = append(report.Results, result)
	}

	report.Duration = time.Since(started)
	return report
}

// consecutiveFailures counts how many runs in a row have been unhealthy. A
// single bad run can be a blip; a streak is an incident, and the streak is the
// number worth alerting on.
var consecutiveFailures atomic.Int64

// ConsecutiveReconcileFailures exposes the current streak for a health endpoint
// or a metrics scrape. Zero means healthy.
func ConsecutiveReconcileFailures() int64 { return consecutiveFailures.Load() }

// defaultReconcileTimeout bounds one full pass. A slow check must not overlap
// the next tick, and it must not hold a connection open indefinitely.
const defaultReconcileTimeout = 2 * time.Minute

// runOnce performs a pass and logs the outcome. Split out from the ticker so a
// test can drive it directly.
func runOnce(db *gorm.DB, timeout time.Duration) ReconcileReport {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	report := RunReconcile(ctx, db)

	if report.Healthy {
		if streak := consecutiveFailures.Swap(0); streak > 0 {
			logger.Info("StudsToken reconciliation recovered",
				"unhealthy_runs_cleared", streak)
		}
		// Log the slowest check even when healthy: this is the signal that
		// tells you when the full sweep is becoming expensive, before it
		// becomes a problem.
		var slowest InvariantResult
		for _, r := range report.Results {
			if slowest.Name == "" || r.Duration > slowest.Duration {
				slowest = r
			}
		}
		logger.Info("StudsToken reconciliation passed",
			"checks", len(report.Results),
			"total_ms", report.Duration.Milliseconds(),
			"slowest_check", string(slowest.Name),
			"slowest_ms", slowest.Duration.Milliseconds())
		return report
	}

	streak := consecutiveFailures.Add(1)
	failed := report.Failure()
	logger.Error("StudsToken reconciliation FAILED",
		"invariant", string(failed.Name),
		"violations", failed.Violations,
		"sample", failed.Sample,
		"err", failed.Err,
		"consecutive_unhealthy_runs", streak)
	return report
}

// StartReconciler runs the reconciliation pass on a ticker until stop() is
// called. Follows the shape of notification.StartPoller.
//
// The first pass is deferred by one interval rather than run at startup: on a
// rolling deploy several instances would otherwise all reconcile at once, and a
// fresh boot has nothing new to say. Anything genuinely broken will still be
// caught one interval later.
func StartReconciler(db *gorm.DB, interval, timeout time.Duration) (stop func()) {
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				runOnce(db, timeout)
			case <-done:
				return
			}
		}
	}()
	return func() { ticker.Stop(); done <- struct{}{} }
}
