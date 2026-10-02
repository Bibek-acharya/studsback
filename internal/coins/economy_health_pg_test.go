//go:build coinsintegration

// internal/coins/economy_health_pg_test.go
//
// 05 §5 read as a dashboard rather than as a table.
//
// The one thing worth testing hard here is the aggregation rule, because an
// aggregate of ratios is the error that produces a dashboard which reports a stable
// economy straight through a period in which it tripled: average the daily
// faucet:sink ratios and a day that issued five coins weighs exactly as much as a day
// that issued five hundred thousand. That is why the fixture below is DELIBERATELY
// LOPESIDED — the two days have very different sizes — so a mean-of-ratios and a
// ratio-of-totals cannot both pass.
//
// The second thing worth testing is that TODAY is excluded. A partial day in a trend
// makes every metric dip at midnight, which an operator reads as a change in the
// economy when nothing happened.

package coins

import (
	"context"
	"testing"
	"time"
)

// storeDay writes one rollup row directly.
//
// Direct INSERT rather than RollupDay because these tests are about the WINDOW
// arithmetic over stored rows, and driving them through the journal would couple the
// test to the rollup's own correctness — the thing economy_daily_pg_test.go already
// covers.
func storeDay(t *testing.T, env *expiryEnv, day time.Time, issued, referral, spent, expired int64) {
	t.Helper()
	row := CoinEconomyDaily{
		Day: truncateToUTCDay(day), MetricVersion: CurrentMetricVersion,
		CoinsIssued: issued, CoinsIssuedByReferral: referral,
		CoinsSpent: spent, CoinsExpired: expired,
	}
	if issued > 0 && spent+expired > 0 {
		row.FaucetSinkRatio = float64(issued) / float64(spent+expired)
		row.FaucetSinkRatioDefined = true
	}
	if issued > 0 && spent > 0 {
		row.Velocity = float64(issued) / float64(spent)
		row.VelocityDefined = true
	}
	if issued > 0 {
		row.ReferralShareOfIssuance = float64(referral) / float64(issued)
		row.ReferralShareOfIssuanceDefined = true
		row.ReferralShareHealthy = row.ReferralShareOfIssuance <= ReferralShareTarget
	}
	if err := env.pool.Create(&row).Error; err != nil {
		t.Fatalf("store %s: %v", row.Day.Format("2006-01-02"), err)
	}
}

// THE test. Two days of very different size, and the window figures must come from
// the TOTALS.
func TestTheWindowFiguresAreRatiosOfTotalsNotMeansOfDailyRatios(t *testing.T) {
	env := newExpiryEnv(t)
	day := env.today()

	// Day 1: small. Issued 10, sink 10 → ratio 1.0.
	storeDay(t, env, day.AddDate(0, 0, -1), 10, 0, 5, 5)
	// Day 2: large. Issued 1000, sink 1000 → ratio 1.0.
	storeDay(t, env, day.AddDate(0, 0, -2), 1000, 0, 500, 500)

	health, err := env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	// Both daily ratios are 1.0, so a mean-of-ratios would also be 1.0 here — which
	// is why the fixture needs a second, asymmetric figure. See below.
	if health.Summary.CoinsIssued != 1010 || health.Summary.CoinsSpent != 505 || health.Summary.CoinsExpired != 505 {
		t.Fatalf("window totals = issued %d spent %d expired %d, want 1010/505/505",
			health.Summary.CoinsIssued, health.Summary.CoinsSpent, health.Summary.CoinsExpired)
	}

	// Now make day 2 LOPESIDED so the two rules diverge. Day 1 issues 10 and sinks 1
	// (ratio 10); day 2 issues 1000 and sinks 999 (ratio 1.001).
	env.pool.Exec(`DELETE FROM coin_economy_daily`)
	storeDay(t, env, day.AddDate(0, 0, -1), 10, 0, 1, 0)       // ratio 10.0
	storeDay(t, env, day.AddDate(0, 0, -2), 1000, 0, 499, 500) // ratio 1.001

	health, err = env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	issued := health.Summary.CoinsIssued
	sink := health.Summary.CoinsSpent + health.Summary.CoinsExpired
	if issued != 1010 || sink != 1000 {
		t.Fatalf("totals = issued %d sink %d, want 1010/1000", issued, sink)
	}
	wantRatio := float64(issued) / float64(sink)
	if diff := health.Summary.FaucetSinkRatio - wantRatio; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("faucet:sink over the window = %v, want %v (the ratio of TOTALS)",
			health.Summary.FaucetSinkRatio, wantRatio)
	}
	// The mean of the two daily ratios would be (10 + 1.001) / 2 = 5.5, which is
	// nowhere near 1.01 — and 5.5 would read as "the economy is inflating fivefold".
	var meanOfRatios float64
	for _, d := range health.Days {
		meanOfRatios += d.FaucetSinkRatio
	}
	meanOfRatios /= float64(len(health.Days))
	if health.Summary.FaucetSinkRatio == meanOfRatios {
		t.Errorf("the window figure %v equals the mean of daily ratios %v; one of them "+
			"is the wrong rule and this fixture was built so exactly one can pass",
			health.Summary.FaucetSinkRatio, meanOfRatios)
	}
	t.Logf("window ratio-of-totals %v vs mean-of-daily-ratios %v", health.Summary.FaucetSinkRatio, meanOfRatios)

	// And the window velocity is over totals for the same reason.
	wantVelocity := float64(issued) / float64(health.Summary.CoinsSpent)
	if diff := health.Summary.Velocity - wantVelocity; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("window velocity = %v, want %v", health.Summary.Velocity, wantVelocity)
	}
}

// Today must be EXCLUDED. Its row is still being written to, so including it would
// make every metric dip at midnight — a trend line that drops daily regardless of
// what happened.
func TestTheHealthWindowExcludesTodayBecauseTodayIsIncomplete(t *testing.T) {
	env := newExpiryEnv(t)
	day := env.today()

	storeDay(t, env, day.AddDate(0, 0, -1), 100, 0, 10, 0)
	storeDay(t, env, day, 999999, 0, 1, 0) // today — still accruing

	health, err := env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.Summary.CoinsIssued != 100 {
		t.Errorf("coins_issued = %d, want 100 — today's partial row leaked into the window",
			health.Summary.CoinsIssued)
	}
	for _, d := range health.Days {
		if truncateToUTCDay(d.Day).Equal(day) {
			t.Errorf("today (%s) was returned in the window", day.Format("2006-01-02"))
		}
	}
}

// A window of N days is [today-N, today) — half-open again, so the OLDEST day is
// included and today is not. An off-by-one here silently drops a day off the front of
// every window, which is invisible in the total and visible only as a truncated chart.
func TestTheHealthWindowIsHalfOpenAndIncludesItsOldestDay(t *testing.T) {
	env := newExpiryEnv(t)
	day := env.today()

	storeDay(t, env, day.AddDate(0, 0, -1), 10, 0, 1, 0) // inside a 3-day window
	storeDay(t, env, day.AddDate(0, 0, -3), 20, 0, 2, 0) // the oldest day — inside
	storeDay(t, env, day.AddDate(0, 0, -4), 30, 0, 3, 0) // one too old

	health, err := env.ledger.EconomyHealth(context.Background(), 3)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.Summary.CoinsIssued != 30 {
		t.Errorf("coins_issued = %d, want 30 (days 1 and 3), so the window is [today-3, today)",
			health.Summary.CoinsIssued)
	}
	if health.WindowDays != 3 {
		t.Errorf("window_days = %d, want 3", health.WindowDays)
	}
}

// The target comparison, and the count of breaching days. 05 says the fraud ratio is
// "monitored weekly, not an incident discovered later" — a single day's breach is
// often noise, several are not, which is why the summary counts days rather than
// averaging the breach.
func TestTheSummaryCountsBreachingDaysButOnlyDefinedOnes(t *testing.T) {
	env := newExpiryEnv(t)
	day := env.today()

	// 3 days over the 8% target...
	storeDay(t, env, day.AddDate(0, 0, -1), 100, 20, 10, 0) // 20% — breach
	storeDay(t, env, day.AddDate(0, 0, -2), 100, 30, 10, 0) // 30% — breach
	storeDay(t, env, day.AddDate(0, 0, -3), 100, 5, 10, 0)  //  5% — healthy
	// ...and 2 idle days, which are undefined and must NOT be counted as breaches or
	// as healthy days. Counting them as breaches would make an outage look like fraud;
	// counting them as healthy would make an outage look compliant.
	storeDay(t, env, day.AddDate(0, 0, -4), 0, 0, 0, 0)
	storeDay(t, env, day.AddDate(0, 0, -5), 0, 0, 0, 0)

	health, err := env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.Summary.DaysAboveTarget != 2 {
		t.Errorf("days_above_target = %d, want 2", health.Summary.DaysAboveTarget)
	}
	if health.Summary.DaysDefined != 3 {
		t.Errorf("days_defined = %d, want 3 — the two idle days have no share to judge", health.Summary.DaysDefined)
	}
	// The window's own share: (20+30+5) / 300 = 18.3%, over the target.
	want := 55.0 / 300.0
	if diff := health.Summary.ReferralShare - want; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("window referral share = %v, want %v", health.Summary.ReferralShare, want)
	}
	if health.Summary.ReferralShareHealthy {
		t.Errorf("the window share %v reported healthy against a %v target", health.Summary.ReferralShare, ReferralShareTarget)
	}
	// The target travels with the response so the dashboard draws the line it is
	// judged against rather than hardcoding it.
	if health.Target != ReferralShareTarget {
		t.Errorf("the response carried target %v, want %v", health.Target, ReferralShareTarget)
	}
}

// Days is returned OLDEST FIRST, because a chart that has to sort renders in the wrong
// order the first time a client forgets, and a mis-read trend is the one failure these
// metrics cannot have.
func TestTheDaysComeBackOldestFirst(t *testing.T) {
	env := newExpiryEnv(t)
	day := env.today()
	for back := 1; back <= 5; back++ {
		storeDay(t, env, day.AddDate(0, 0, -back), int64(back), 0, 1, 0)
	}

	health, err := env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if len(health.Days) != 5 {
		t.Fatalf("%d days returned, want 5", len(health.Days))
	}
	for i := 1; i < len(health.Days); i++ {
		if !health.Days[i-1].Day.Before(health.Days[i].Day) {
			t.Fatalf("day %d (%s) does not precede day %d (%s); the series would be drawn back to front",
				i-1, health.Days[i-1].Day.Format("2006-01-02"), i, health.Days[i].Day.Format("2006-01-02"))
		}
	}
}

// An empty window must not divide by zero, and must not claim to be healthy. This is
// the state the dashboard is in on day one of every deployment — the very first thing
// an operator will see — so if it renders as "all clear" then the most dangerous false
// reading available is also the first one.
func TestAnEmptyWindowReportsNothingRatherThanAllClear(t *testing.T) {
	env := newExpiryEnv(t)

	health, err := env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.Days == nil {
		t.Error("Days is nil; a JSON client renders null rather than an empty series")
	}
	if len(health.Days) != 0 {
		t.Errorf("%d days on an empty window", len(health.Days))
	}
	if health.Summary.FaucetSinkRatioDefined || health.Summary.VelocityDefined ||
		health.Summary.ReferralShareDefined || health.Summary.DaysOfCurrencyOnHandDefined {
		t.Errorf("an empty window reported a defined metric: %+v", health.Summary)
	}
	if health.Summary.ReferralShareHealthy {
		t.Error("an empty window reported the fraud ratio healthy")
	}
	if health.MetricVersion != CurrentMetricVersion {
		t.Errorf("metric_version = %d, want %d — a client needs it to avoid drawing a trend across a change",
			health.MetricVersion, CurrentMetricVersion)
	}
}

// DaysOfCurrencyOnHand: outstanding balance over the average daily spend. It is the
// figure that rises when students stop spending, and it must be UNDEFINED when nothing
// was spent — a division by zero there would be +Inf, which is not a number a JSON
// client can parse, and a very large placeholder number would read as infinite supply.
func TestDaysOfCurrencyOnHandIsUndefinedWithoutSpend(t *testing.T) {
	env := newExpiryEnv(t)
	day := env.today()

	// Coins held, but none spent.
	env.grantExpiring(t, 4242, 7*24*time.Hour)
	storeDay(t, env, day.AddDate(0, 0, -1), 100, 0, 0, 0) // issued, never spent

	health, err := env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.Summary.DaysOfCurrencyOnHandDefined {
		t.Errorf("days_of_currency_on_hand = %v on a window with no spend; the division has no meaning",
			health.Summary.DaysOfCurrencyOnHand)
	}

	// Now spend, and the figure becomes defined and is checkable by hand.
	_, lotID := env.grantExpiring(t, 5151, 7*24*time.Hour)
	if _, err := env.ledger.Spend(context.Background(), SpendRequest{
		UserID: 5151, ReasonCode: ReasonResourceUnlock, IdempotencyKey: "doc-spend",
		RefType: RefStudyResource, RefID: refID(lotID),
	}); err != nil {
		t.Fatalf("spend: %v", err)
	}
	spend, err := env.ledger.Spend(context.Background(), SpendRequest{
		UserID: 4242, ReasonCode: ReasonResourceUnlock, IdempotencyKey: "doc-spend-2",
		RefType: RefStudyResource, RefID: refID(lotID),
	})
	if err != nil {
		t.Fatalf("spend: %v", err)
	}

	env.pool.Exec(`DELETE FROM coin_economy_daily`)
	storeDay(t, env, day.AddDate(0, 0, -1), 100, 0, spend.Amount, 0)

	health, err = env.ledger.EconomyHealth(context.Background(), 30)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if !health.Summary.DaysOfCurrencyOnHandDefined {
		t.Fatal("days_of_currency_on_hand is undefined on a window with spend")
	}
	// Hand-checkable: the outstanding balance is what the two grants left after the
	// spends, and the daily average is the spend spread over the one day that spent.
	var outstanding int64
	env.pool.Raw(`SELECT COALESCE(SUM(b.posted_balance - b.reserved), 0)
		FROM coin_account_balance b JOIN coin_account a ON a.id = b.account_id
		WHERE a.kind = 'USER' AND a.owner_user_id IS NOT NULL AND a.closed_at IS NULL`).Scan(&outstanding)
	want := float64(outstanding) / float64(spend.Amount)
	if diff := health.Summary.DaysOfCurrencyOnHand - want; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("days_of_currency_on_hand = %v, want %v (outstanding %d / one day's spend %d)",
			health.Summary.DaysOfCurrencyOnHand, want, outstanding, spend.Amount)
	}
}

// The window is bounded. This route is reachable by anyone who can read coin PRICING,
// and an unbounded `?days=` would return the entire economic history of the platform
// to an operator console on every keystroke of a date picker.
func TestTheWindowIsBoundedAndABadQueryFallsBackToTheDefault(t *testing.T) {
	env := newExpiryEnv(t)

	health, err := env.ledger.EconomyHealth(context.Background(), 0)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.WindowDays != HealthDefaultWindow {
		t.Errorf("window for 0 = %d, want the default %d", health.WindowDays, HealthDefaultWindow)
	}
	if health.WindowDays != 30 {
		t.Errorf("the default window is %d days; 05 §5 states its alert bands as MONTHLY, "+
			"so a shorter default would alert on ordinary day-to-day variation", health.WindowDays)
	}

	health, err = env.ledger.EconomyHealth(context.Background(), -5)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.WindowDays != HealthDefaultWindow {
		t.Errorf("window for -5 = %d, want the default %d", health.WindowDays, HealthDefaultWindow)
	}

	health, err = env.ledger.EconomyHealth(context.Background(), 100000)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.WindowDays != HealthMaxWindow {
		t.Errorf("window for 100000 = %d, want the cap %d", health.WindowDays, HealthMaxWindow)
	}
}

// A nil Ledger must not panic. RegisterRoutes substitutes an empty AdminAPI when the
// constructor is missed, and the handler then has no ledger — the documented way for
// this endpoint to be present but dead.
func TestHealthOnALedgerWithNoDatabaseRefusesRatherThanPanicking(t *testing.T) {
	var l *Ledger
	if _, err := l.EconomyHealth(context.Background(), 30); err != ErrNoDatabase {
		t.Errorf("err = %v, want ErrNoDatabase", err)
	}
}
