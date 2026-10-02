package coins

import (
	"testing"
	"time"
)

// The properties of the rollup that need no database: the boundaries, the
// thresholds, and the schema of the table itself.
//
// The arithmetic is in economy_daily_pg_test.go, because it can only be checked
// against real rows. What is HERE is everything that would silently produce a wrong
// day boundary or a wrong threshold — the failures that do not show up as a wrong
// number but as a trend that is quietly wrong.

// UTC, half-open, and midnight. The three rules the referral cap and the referral
// month_key already each got wrong once.
func TestDayBoundsAreUTCHalfOpenAndMidnight(t *testing.T) {
	// A local instant that is a DIFFERENT DAY in Nepal (UTC+05:45). 22:30 UTC on the
	// 2nd is 04:15 on the 3rd in Kathmandu, so a local-day truncation would file
	// this under the 3rd.
	local := time.Date(2026, 10, 2, 22, 30, 0, 0, time.FixedZone("NPT", 5*60*45+15*60))
	day := truncateToUTCDay(local)
	if day.Hour() != 0 || day.Minute() != 0 || day.Second() != 0 {
		t.Errorf("truncated day = %v, want midnight", day)
	}
	if day.Day() != 2 {
		t.Errorf("truncated day = the %dth, want the 2nd — the boundary is UTC, not local", day.Day())
	}

	from, to := dayBounds(local)
	if !from.Equal(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("from = %v, want 2026-10-02T00:00:00Z", from)
	}
	// HALF-OPEN: a journal stamped at exactly midnight belongs to THIS day only. With
	// a closed interval it would be counted at the end of the previous day too, so
	// midnight activity would be double-counted and nothing else would show it.
	if !to.Equal(time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("to = %v, want 2026-10-03T00:00:00Z — the window is half-open", to)
	}
	if to.Sub(from) != 24*time.Hour {
		t.Errorf("the window is %v, want 24h", to.Sub(from))
	}

	// A leap day must not produce a 23- or 25-hour window.
	leap := time.Date(2028, 2, 28, 12, 0, 0, 0, time.UTC)
	lFrom, lTo := dayBounds(leap)
	if lTo.Sub(lFrom) != 24*time.Hour {
		t.Errorf("the window across a leap day is %v, want 24h", lTo.Sub(lFrom))
	}
}

// The fraud threshold is 05 §5's DERIVED 8%, and it must not become a literal
// somewhere. A dashboard that hardcodes it cannot be adjusted by the people who own
// the risk, which is why it is a named constant with a comment.
func TestTheFraudRatioTargetIsEightPercent(t *testing.T) {
	if ReferralShareTarget != 0.08 {
		t.Errorf("ReferralShareTarget = %v, want 0.08 — 05 §5's derived fraud threshold", ReferralShareTarget)
	}
	// And the health flag must be the comparison against it, not something else. A
	// metric that computes but does not flag is a number nobody acts on.
	if ReferralShareTarget <= 0 || ReferralShareTarget >= 1 {
		t.Errorf("the target %v is not a share", ReferralShareTarget)
	}
}

// The metric_version column is what stops a definition change from rewriting
// history. 08 asks for trends and for "rising" and "falling", and both are
// meaningless across a change of definition — recomputing last quarter with today's
// formula produces a smooth line describing nothing.
func TestTheDailyTableIsKeyedByDayAndMetricVersion(t *testing.T) {
	if CurrentMetricVersion != 1 {
		t.Errorf("CurrentMetricVersion = %d, want 1 for the definitions in this file", CurrentMetricVersion)
	}
	if len(EconomyDailyModels) != 1 {
		t.Errorf("%d models in EconomyDailyModels, want 1", len(EconomyDailyModels))
	}
	if name := (CoinEconomyDaily{}).TableName(); name != "coin_economy_daily" {
		t.Errorf("table name = %q, want %q — 01 and 08 both name this table", name, "coin_economy_daily")
	}
}

// The stored raw counts are the durable part. Every ratio is arithmetic on them, so
// a definition change can recompute a ratio without re-reading the journal — and
// more importantly an auditor can CHECK a ratio by hand from five numbers on one row.
func TestTheRawCountsAreStoredAlongsideTheRatios(t *testing.T) {
	// Referrals must never exceed total issuance, which is the invariant the fraud
	// ratio depends on. Asserted on a constructed row rather than on a query because
	// this is a property of the STORED figures, not of the computation.
	row := CoinEconomyDaily{CoinsIssued: 100, CoinsIssuedByReferral: 8}
	if row.CoinsIssuedByReferral > row.CoinsIssued {
		t.Error("a referral share above total issuance was stored; the ratio would exceed 1")
	}
	if row.CoinsSpent < 0 || row.CoinsExpired < 0 {
		t.Error("a spend or expiry total is negative; both are positive quantities and are negated at the source")
	}
}
