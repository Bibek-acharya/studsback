//go:build coinsintegration

// internal/coins/economy_daily_pg_test.go
//
// 05 §5, against Postgres: the daily rollup and the health metrics.
//
// 05's own framing is "the dashboard is the actual deliverable; the target numbers
// are not". So these tests pin that each metric is COMPUTED FROM THE JOURNAL — the
// append-only record — and not from the cached projection. A rollup built on the
// projection would agree with it perfectly on a healthy day and report confident
// nonsense on the day it drifted, which is the one day the dashboard exists for.
//
// The metrics are also asserted against hand-computed expectations rather than
// against a second implementation of the same arithmetic. A fixture with one grant,
// one spend and one expiry has figures a person can check.

package coins

import (
	"context"
	"testing"
	"time"
)

// THE test. One grant, one spend, one expiry, on one day, and every figure checked
// by hand.
func TestTheDailyRollupComputesEveryHealthFigureFromTheJournal(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()
	day := env.today()

	// The figures are DERIVED from the grants' own amounts rather than hardcoded. The
	// default award is the profile instalment (5), not the 100 this scenario is
	// narrated in — and a hardcoded 100 would be a test that passes only while the
	// economy config is unchanged, which is the wrong way round.
	//
	// The spend is the full award because the ledger's minimum priced unlock may
	// exceed it; what matters to the assertions is that SOME coins moved, and the
	// ratios are checked against the spend's own reported amount.
	// Granted LIVE, because the FEFO allocator only considers open lots — a lot
	// created already-expired has nothing to spend and the test fails on the fixture
	// rather than on the rollup. The expiry is applied AFTER the spend, which is also
	// the real sequence: coins are earned spendable, then lapse.
	grant, lotID := env.grantExpiring(t, 4242, 7*24*time.Hour)
	spend, err := env.ledger.Spend(ctx, SpendRequest{
		UserID: 4242, ReasonCode: ReasonResourceUnlock, IdempotencyKey: "daily-spend",
		RefType: RefStudyResource, RefID: refID(lotID),
	})
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	spent := spend.Amount
	// A second student with a live grant, so the populations are not 1 by accident and
	// the payer-conversion fraction is meaningful.
	second, _ := env.grantExpiring(t, 5151, 7*24*time.Hour)

	// Now the first student's remaining lot lapses and the sweep burns it.
	if err := env.pool.Model(&CoinLot{}).Where("id = ?", lotID).
		UpdateColumn("expires_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire the lot: %v", err)
	}
	if _, err := env.sweeper.Sweep(ctx, ExpiryBatchDefault); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	var burned int64
	env.pool.Raw(`SELECT COALESCE(SUM(amount), 0) FROM coin_posting p
		JOIN coin_account a ON a.id = p.account_id
		WHERE a.owner_user_id = ? AND a.kind = 'USER'`, 4242).Scan(&burned)
	burned = -burned
	expired := grant.Amount - spent

	row, err := env.sweeper.RollupDay(ctx, day)
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}

	wantIssued := grant.Amount + second.Amount
	if row.CoinsIssued != wantIssued {
		t.Errorf("coins_issued = %d, want %d", row.CoinsIssued, wantIssued)
	}
	if row.CoinsSpent != spent {
		t.Errorf("coins_spent = %d, want %d", row.CoinsSpent, spent)
	}
	// Expired is a DISTINCT column from spent, not folded in: a student who spent
	// coins chose to, and one whose coins lapsed did not.
	if row.CoinsExpired != expired {
		t.Errorf("coins_expired = %d, want %d — the remainder of the lapsed lot", row.CoinsExpired, expired)
	}

	// The faucet:faucet ratio, which is 05's first metric and detects both hoarding
	// and inflation. Expiry counts as a SINK: a coin destroyed by expiry has left
	// circulation exactly as a spent one has.
	wantSink := spent + expired
	if wantSink <= 0 {
		t.Fatalf("the fixture moved no coins; spent=%d expired=%d", spent, expired)
	}
	wantRatio := float64(wantIssued) / float64(wantSink)
	if diff := row.FaucetSinkRatio - wantRatio; diff > 0.001 || diff < -0.001 {
		t.Errorf("faucet_sink_ratio = %v, want %v (issued %d / sink %d)",
			row.FaucetSinkRatio, wantRatio, wantIssued, wantSink)
	}

	wantVelocity := float64(wantIssued) / float64(spent)
	if diff := row.Velocity - wantVelocity; diff > 0.001 || diff < -0.001 {
		t.Errorf("velocity = %v, want %v", row.Velocity, wantVelocity)
	}

	if row.JournalsPosted != 4 {
		t.Errorf("journals_posted = %d, want 4 (two grants, one spend, one expire)", row.JournalsPosted)
	}
	if row.StudentsHolding != 1 {
		// 4242 was burned to zero by the sweep; 5151 still holds their award. This
		// asserts the rollup counts the POPULATION as it stands now rather than as it
		// stood at the start of the day — which is what a balance-derived count does.
		t.Errorf("students_holding = %d, want 1 (the burn emptied the other student)", row.StudentsHolding)
	}
	if row.StudentsPaying != 1 {
		t.Errorf("students_paying = %d, want 1", row.StudentsPaying)
	}
	if !row.PayerConversionDefined {
		t.Error("payer_conversion is undefined on a day with one holder and one payer")
	} else if diff := row.PayerConversion - 1.0; diff > 0.001 || diff < -0.001 {
		t.Errorf("payer_conversion = %v, want 1.0", row.PayerConversion)
	}
}

// The fraud ratio — 05 §5's third feature-specific metric, and the one with a
// DERIVED target (referral issuance ≤ 8% of total issuance). It needs referral
// issuance counted SEPARATELY from the rest, which is the whole reason it is
// reported here rather than derived by an operator from the total.
func TestTheFraudRatioCountsReferralIssuanceSeparately(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()
	day := env.today()

	// Two profile awards and one referral. Each gets its own user because a user with
	// two grants of the same reason would collide on the fixture's per-user key.
	for _, user := range []uint{1, 2} {
		env.grantExpiring(t, user, 7*24*time.Hour)
	}
	referral, _ := env.grantReason(t, 3, ReasonReferralQualified, "fraud-ratio-referral", 7*24*time.Hour)

	row, err := env.sweeper.RollupDay(ctx, day)
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	// The referral award is configured differently from the profile instalment, so the
	// expected figures come from the grants themselves rather than from a fraction of
	// the total.
	wantIssued := row.CoinsIssued - referral.Amount
	if wantIssued <= 0 {
		t.Fatalf("the profile grants issued nothing; row = %+v", row)
	}
	if row.CoinsIssuedByReferral != referral.Amount {
		t.Errorf("coins_issued_by_referral = %d, want %d", row.CoinsIssuedByReferral, referral.Amount)
	}
	want := float64(referral.Amount) / float64(row.CoinsIssued)
	if diff := row.ReferralShareOfIssuance - want; diff > 0.001 || diff < -0.001 {
		t.Errorf("referral_share = %v, want %v", row.ReferralShareOfIssuance, want)
	}
	// And the DASHBOARD must say the ratio is out of policy, because 1/3 is far above
	// the 8% target. A metric that computes but does not flag is a number nobody acts
	// on, which is the failure 08 warns about for every one of these.
	if row.ReferralShareHealthy {
		t.Errorf("referral share %v reported healthy; the target is 0.08", row.ReferralShareOfIssuance)
	}
}

// A day with no activity must produce a row, not nothing. A dashboard that skips
// empty days renders gaps that look like outages, and 08's "sustained below" and
// "rising trend" tests need the zero days to see the shape.
func TestAnIdleDayRollsUpToZeroesRatherThanNothing(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()
	day := env.today()

	row, err := env.sweeper.RollupDay(ctx, day)
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if row.CoinsIssued != 0 || row.CoinsSpent != 0 {
		t.Errorf("an idle day reported issued=%d spent=%d", row.CoinsIssued, row.CoinsSpent)
	}
	// The ratios are UNDEFINED on an idle day, and must be null rather than 0, 1 or
	// -1. A zero faucet:faucet ratio would read as "deflation" and a zero velocity as
	// "coins are not circulating" — both alarming, both false.
	if row.FaucetSinkRatio != 0 {
		t.Errorf("faucet_sink_ratio = %v on an idle day, want 0 meaning UNDEFINED", row.FaucetSinkRatio)
	}
	if row.FaucetSinkRatioDefined {
		t.Error("the ratio claims to be defined on a day with no issuance")
	}
	if row.ReferralShareHealthy {
		t.Error("an idle day reported the fraud ratio healthy; it has no meaning")
	}
}

// Backfill must be idempotent. Re-running a day's rollup after the fact is the
// normal repair action, and it must not double-count — which is why this asserts on
// the STORED row rather than on a returned value.
func TestRollingUpADayTwiceStoresTheSameFigures(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()
	day := env.today()
	env.grantExpiring(t, 4242, 7*24*time.Hour)

	first, err := env.sweeper.RollupDay(ctx, day)
	if err != nil {
		t.Fatalf("first rollup: %v", err)
	}
	second, err := env.sweeper.RollupDay(ctx, day)
	if err != nil {
		t.Fatalf("second rollup: %v", err)
	}
	if first.CoinsIssued != second.CoinsIssued {
		t.Errorf("issued changed from %d to %d across two rollups of one day", first.CoinsIssued, second.CoinsIssued)
	}
	var stored int64
	env.pool.Model(&CoinEconomyDaily{}).Where("day = ?", day.Format("2006-01-02")).Count(&stored)
	if stored != 1 {
		t.Errorf("%d stored rows for one day, want 1 — the rollup must upsert", stored)
	}
}

// A rollup is a DAY's figures, so a journal stamped at 23:59:59 and one at 00:00:00
// must land in different rows. This is UTC and it is deliberate, for the same reason
// referral_api.go's month_key is UTC: a local-day boundary would put the same
// economic instant in two rows depending on who was looking, and the referral cap
// already had that bug.
func TestTheRollupDayIsUTCAndSplitsAtMidnight(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	_, err := env.ledger.Grant(ctx, GrantRequest{
		UserID: 4242, ReasonCode: ReasonProfileComplete, IdempotencyKey: "late-grant",
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	// Move the journal to just before midnight UTC and roll up both days.
	late := time.Now().UTC().Truncate(time.Hour).Add(-30 * time.Minute)
	late = time.Date(late.Year(), late.Month(), late.Day(), 23, 30, 0, 0, time.UTC)
	if err := env.pool.Model(&CoinJournal{}).Where("idempotency_key = ?", "late-grant").
		UpdateColumn("effective_at", late).Error; err != nil {
		t.Fatalf("move the journal: %v", err)
	}

	laterDay := late.Add(30 * time.Minute)
	row, err := env.sweeper.RollupDay(ctx, laterDay)
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if row.CoinsIssued != 0 {
		t.Errorf("a journal at %s was counted in the rollup for %s", late, laterDay)
	}
	sameDay, err := env.sweeper.RollupDay(ctx, late)
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	if sameDay.CoinsIssued == 0 {
		t.Errorf("a journal at %s was not counted in its own day's rollup", late)
	}
}
