package coins

import (
	"context"
	"errors"
	"time"

	"studsphere/backend/internal/shared/config"
)

// init makes config.AppConfig non-nil before any test runs.
//
// The package's TestMain does this too — but it lives in ledger_pg_test.go, behind the
// `coinsintegration` build tag, so it is ABSENT from an untagged `go test`. Without
// this, any untagged test that reaches a logging path panics on a nil dereference
// rather than merely logging to nowhere: logger.Init reads config.AppConfig.GinMode.
//
// init rather than a TestMain because a package may only have one, and the tagged one
// also manages a schema this package's untagged tests do not need. Both are
// idempotent, so the tagged run simply finds the work already done.
func init() {
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}
}

// recordingSweeper stands in for *ExpirySweeper in the schedule tests.
//
// It exists to make two things observable that a real sweeper hides behind a database:
// WHICH DAYS the loop asked for, and what it does after one of them fails. Both are
// behavioural, and both are the difference between a rollup that keeps its series
// current and one that silently stops.
type recordingSweeper struct {
	// nowFunc is the injected clock. A field rather than a method so a test can
	// assign it in one expression rather than defining a type per test.
	nowFunc func() time.Time
	// failOn is a day (YYYY-MM-DD) whose rollup returns an error.
	failOn string
	// days records every day the loop ATTEMPTED, in order — attempts, not successes.
	days []time.Time
}

func (r *recordingSweeper) nowUTC() time.Time {
	if r.nowFunc == nil {
		return time.Now().UTC()
	}
	return r.nowFunc()
}

func (r *recordingSweeper) RollupDay(_ context.Context, day time.Time) (CoinEconomyDaily, error) {
	day = truncateToUTCDay(day)
	r.days = append(r.days, day)
	if r.failOn != "" && day.Format("2006-01-02") == r.failOn {
		return CoinEconomyDaily{}, errors.New("the database was briefly unavailable")
	}
	return CoinEconomyDaily{Day: day, MetricVersion: CurrentMetricVersion}, nil
}
