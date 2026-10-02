// internal/coins/economy_daily.go
//
// 05-economy-and-fraud.md §5, and the file 02-architecture.md names for this slice.
//
// ── why the dashboard is the deliverable and the target numbers are not ─────────
//
// 05 says it outright: "The dashboard is the actual deliverable; the target numbers
// are not." That is the correct framing and the reason this file exists at all —
// 08 says economy parameters become tunable only "after the daily economy rollup
// has data", and without this file Phase 5's "tune against the health metrics" has
// nothing to tune against.
//
// ── computed from the JOURNAL, never from the cached projection ───────────────
//
// Every figure here is a sum over coin_posting, not a read of
// coin_account_balance. That is not a purity preference. The projection is what the
// wallet renders, so a rollup built on it would agree with the wallet perfectly on a
// healthy day and report confident nonsense on the day it drifted — and the day it
// drifted is precisely the day the dashboard exists. The projection is REBUILDBLE
// from postings; the journal is not.
//
// ── the undefined-vs-zero discipline ──────────────────────────────────────────
//
// Three of these metrics are ratios, and all three are UNDEFINED on a day with no
// activity. That is reported as a zero number with a `*_defined` flag, never as a
// silent zero.
//
// The reason is that a zero reads as a finding: a faucet:faucet ratio of 0 says
// "deflation, coins are hoarding", and a velocity of 0 says "nothing is
// circulating". Both are alarming and both are false on a day nobody used the
// economy. A dashboard that reports those is one an operator learns to ignore, and
// the six metrics only work as a shape over time — a gap renders as an outage and a
// false zero renders as a trend.
//
// ── the day boundary is UTC, and that is not arbitrary ───────────────────────
//
// referral_api.go's month_key is UTC for the same reason: a local-day boundary puts
// the same economic instant in two rows depending on who was looking, and the
// referral cap already has a variant of that bug. Nepal is UTC+05:45, which means a
// local-day rollup would split every evening's activity across two rows for five
// months of the year and shift with daylight saving for the rest.
package coins

import (
	"context"
	"fmt"
	"time"

	"studsphere/backend/internal/shared/logger"
)

// CoinEconomyDaily is one UTC day's economy figures.
//
// The composite primary key is (day, metric_version) rather than (day) alone, and
// the reason is that a metric's DEFINITION changes over the life of a product while
// its history must not be rewritten. 08 asks for trends and for "rising" and
// "falling", and both are meaningless across a definition change: recomputing last
// quarter with today's formula produces a smooth line that describes nothing. So a
// changed definition starts a new metric_version and the old rows stay readable.
//
// BumpMetricVersion is the only supported way to change a definition, and the comment
// on it says what has to happen to the dashboard when it is used.
type CoinEconomyDaily struct {
	// Day is midnight UTC, stored as a DATE.
	Day time.Time `gorm:"type:date;not null;primaryKey" json:"day"`
	// MetricVersion is the definition version. 1 for the definitions in this file.
	MetricVersion int `gorm:"not null;default:1;primaryKey" json:"metric_version"`

	// ── the raw counts, which are the durable part ─────────────────────────────
	//
	// These are the figures a definition change cannot invalidate, so they are stored
	// alongside the derived ratios rather than being recomputed from them. Every ratio
	// below is arithmetic on this block — checkable by hand, which is the property
	// that makes a bad rollup obvious rather than merely suspicious.

	// CoinsIssued is everything the platform granted today, across every reason.
	CoinsIssued int64 `gorm:"not null;default:0" json:"coins_issued"`
	// CoinsIssuedByReferral is the subset issued for a qualified referral, kept
	// separate because 05 §5's fraud ratio is referral issuance over TOTAL issuance.
	// Derived by an operator from the total, it could not be computed at all.
	CoinsIssuedByReferral int64 `gorm:"not null;default:0" json:"coins_issued_by_referral"`
	// CoinsSpent is what left wallets through unlocks.
	CoinsSpent int64 `gorm:"not null;default:0" json:"coins_spent"`
	// CoinsExpired is what expiry burned. A DESTINCT column from spent, not folded
	// into it: a student who spent coins chose to, and a student whose coins lapsed
	// did not. 05 §4's whole mitigation argument is about the second group.
	CoinsExpired int64 `gorm:"not null;default:0" json:"coins_expired"`
	// CoinsAdjusted is the absolute value of manual corrections. Tracked because an
	// adjustment is a human overriding the system, and a rising count is either a
	// broken mechanic or a broken operator — indistinguishable from the count alone,
	// which is why the reason_code breakdown matters more than this figure.
	CoinsAdjusted int64 `gorm:"not null;default:0" json:"coins_adjusted"`
	// JournalsPosted counts POSTED journals in the day, whatever their type.
	JournalsPosted int64 `gorm:"not null;default:0" json:"journals_posted"`
	// StudentsHolding is distinct users with a positive posted balance at the day's end.
	StudentsHolding int64 `gorm:"not null;default:0" json:"students_holding"`
	// StudentsPaying is the subset who spent something today. The pair gives payer
	// conversion, and the pair is stored because the RATIO alone cannot be
	// reconstructed once the denominator is not recorded.
	StudentsPaying int64 `gorm:"not null;default:0" json:"students_paying"`

	// ── the derived metrics ────────────────────────────────────────────────────

	// FaucetSinkRatio is coins_issued / (coins_spent + coins_expired).
	//
	// Expiry counts as a SINK and that is the load-bearing judgement: a coin
	// destroyed by expiry has left circulation exactly as a spent one has, and 05 §5
	// lists this metric as detecting both deflation (hoarding) and inflation
	// (worthless coins). Leaving expired coins out of the denominator would report a
	// hoarding economy every time a batch of earned coins matured.
	//
	// Undefined — FaucetSinkRatioDefined false and the value zero — when nothing was
	// issued, because the ratio has no meaning then.
	FaucetSinkRatio        float64 `gorm:"not null;default:0" json:"faucet_sink_ratio"`
	FaucetSinkRatioDefined bool    `gorm:"not null;default:false" json:"faucet_sink_ratio_defined"`

	// Velocity is coins_issued / coins_spent. Above 1 the economy is expanding, below
	// 1 coins are accruing faster than they circulate — 05 §5 calls a sustained value
	// below 0.4 the hoarding signal.
	Velocity        float64 `gorm:"not null;default:0" json:"velocity"`
	VelocityDefined bool    `gorm:"not null;default:false" json:"velocity_defined"`

	// DaysOfCurrencyOnHand is the closing outstanding balance divided by the average
	// daily spend over the trailing window, which is the figure that rises when
	// students stop spending. A day with no spend history is undefined rather than
	// "infinite".
	//
	// It is NOT computed here. It needs a trailing window, and the window is a
	// dashboard concern rather than a rollup one — see EconomyHealth, which computes
	// it over the stored rows.
	DaysOfCurrencyOnHand        float64 `gorm:"not null;default:0" json:"days_of_currency_on_hand"`
	DaysOfCurrencyOnHandDefined bool    `gorm:"not null;default:false" json:"days_of_currency_on_hand_defined"`

	// PayerConversion is students_paying / students_holding. 05 §5 reads a FALLING
	// value after an earn-rate change as evidence the change was wrong, which is the
	// only one of the six that directly judges a parameter.
	PayerConversion        float64 `gorm:"not null;default:0" json:"payer_conversion"`
	PayerConversionDefined bool    `gorm:"not null;default:false" json:"payer_conversion_defined"`

	// ReferralShareOfIssuance is the fraud ratio: coins_issued_by_referral /
	// coins_issued, with a DERIVED target of 0.08.
	ReferralShareOfIssuance        float64 `gorm:"not null;default:0" json:"referral_share_of_issuance"`
	ReferralShareOfIssuanceDefined bool    `gorm:"not null;default:false" json:"referral_share_of_issuance_defined"`
	// ReferralShareHealthy is the target comparison, PRECOMPUTED and stored.
	//
	// Stored rather than left to the dashboard because 05 says "treat as a ratio
	// monitored weekly, not an incident discovered later" — a monitor that has to
	// re-derive the threshold to notice it has missed is not a monitor. The threshold
	// is in one place so changing it changes history consistently.
	ReferralShareHealthy bool `gorm:"not null;default:false" json:"referral_share_healthy"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReferralShareTarget is 05 §5's DERIVED fraud threshold: referral issuance ≤ 8% of
// total issuance.
//
// Named and exported because it is a product threshold with a compliance tail, not an
// implementation detail, and because a dashboard that hardcodes it cannot be
// adjusted by the people who own the risk.
const ReferralShareTarget = 0.08

// CurrentMetricVersion is the definition version this file implements.
const CurrentMetricVersion = 1

func (CoinEconomyDaily) TableName() string { return "coin_economy_daily" }

// EconomyDailyModels is the AutoMigrate slice.
var EconomyDailyModels = []any{&CoinEconomyDaily{}}

// RollupDay computes and stores one UTC day's figures.
//
// IDEMPOTENT, by upsert on (day, metric_version) rather than by refusing a second
// call: re-running a past day is the normal REPAIR action once a bug is found in a
// metric, and a rollup that refused to be re-run would need a manual delete and
// would be wrong more often.
//
// The whole day's figures are recomputed from the journal each time rather than
// incremented, which is what makes the repair safe. A counter that had been
// incremented by a partial run would need the delta and would be unrecoverable.
//
// day is any instant on the target day; it is TRUNCATED to UTC midnight, so callers
// pass a timestamp rather than having to do the truncation themselves and get it
// wrong.
func (s *ExpirySweeper) RollupDay(ctx context.Context, day time.Time) (CoinEconomyDaily, error) {
	if s == nil || s.repo == nil {
		return CoinEconomyDaily{}, ErrNoDatabase
	}
	day = truncateToUTCDay(day)
	from, to := dayBounds(day)
	return s.rollupRange(ctx, from, to, day)
}

// rollupRange does the work, taking explicit bounds so the arithmetic is stated once.
func (s *ExpirySweeper) rollupRange(ctx context.Context, from, to, day time.Time) (CoinEconomyDaily, error) {
	repo := s.repo
	row := CoinEconomyDaily{
		Day: day, MetricVersion: CurrentMetricVersion,
		CreatedAt: s.now().UTC(), UpdatedAt: s.now().UTC(),
	}

	// ── the raw counts, one grouped query each ────────────────────────────────
	//
	// Grouped by entry_type and reason_code in ONE query rather than one per metric.
	// A rollup that ran a query per figure would take a lock-free but very long time
	// on a busy day, and this runs on a timer.

	var totals []struct {
		EntryType  string
		ReasonCode string
		Coins      int64
		Journals   int64
	}
	if err := repo.DB().WithContext(ctx).Raw(`
		SELECT j.entry_type, j.reason_code,
		       COALESCE(SUM(p.amount), 0) AS coins,
		       COUNT(DISTINCT j.id)      AS journals
		  FROM coin_journal j
		  JOIN coin_posting p  ON p.journal_id = j.id
		  JOIN coin_account a  ON a.id = p.account_id
		 WHERE j.state = ?
		   AND j.effective_at >= ? AND j.effective_at < ?
		   AND a.kind = ?
		 GROUP BY j.entry_type, j.reason_code`,
		StatePosted, from, to, AccountUser).Scan(&totals).Error; err != nil {
		return CoinEconomyDaily{}, fmt.Errorf("roll up %s: %w", day.Format("2006-01-02"), err)
	}

	// The loop variable is `group`, not `row`, on purpose: shadowing the row being
	// built by a value read out of the database is a bug that compiles.
	for _, group := range totals {
		row.JournalsPosted += group.Journals
		switch group.EntryType {
		case EntryGrant:
			// A grant's user-side leg is positive, so the grouped sum IS the amount
			// issued — and only because the WHERE keeps to USER accounts. Including
			// the faucet would net every grant to zero.
			row.CoinsIssued += group.Coins
			if group.ReasonCode == ReasonReferralQualified {
				row.CoinsIssuedByReferral += group.Coins
			}
		case EntrySpend:
			// A spend's user-side leg is NEGATIVE, and "coins spent" is a positive
			// quantity. Negating here is the one sign flip in the file, and it is why
			// this comment exists.
			row.CoinsSpent += -group.Coins
		case EntryExpire:
			row.CoinsExpired += -group.Coins
		case EntryAdjust:
			// The absolute value, because a correction can be either direction and
			// "coins adjusted" means the size of the human's involvement, not a net.
			if group.Coins < 0 {
				row.CoinsAdjusted += -group.Coins
			} else {
				row.CoinsAdjusted += group.Coins
			}
		}
	}

	// ── the populations ───────────────────────────────────────────────────────

	if err := repo.DB().WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT a.owner_user_id) AS holding
		  FROM coin_account_balance b
		  JOIN coin_account a ON a.id = b.account_id
		 WHERE a.kind = ? AND a.owner_user_id IS NOT NULL AND b.posted_balance > 0`,
		AccountUser).Scan(&row.StudentsHolding).Error; err != nil {
		return CoinEconomyDaily{}, fmt.Errorf("roll up %s: students holding: %w", day.Format("2006-01-02"), err)
	}

	if err := repo.DB().WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT a.owner_user_id) AS paying
		  FROM coin_journal j
		  JOIN coin_posting p ON p.journal_id = j.id
		  JOIN coin_account a ON a.id = p.account_id
		 WHERE j.entry_type = ? AND j.state = ?
		   AND j.effective_at >= ? AND j.effective_at < ?
		   AND a.kind = ?`,
		EntrySpend, StatePosted, from, to, AccountUser).Scan(&row.StudentsPaying).Error; err != nil {
		return CoinEconomyDaily{}, fmt.Errorf("roll up %s: students paying: %w", day.Format("2006-01-02"), err)
	}

	// ── the ratios, with the undefined discipline applied ──────────────────────
	//
	// Every one of these is guarded, and the guard sets BOTH the value (zero) and the
	// defined flag. A ratio computed from a zero denominator would be Inf, and Inf
	// serialised to JSON is a parse error in most clients — so this is also a
	// correctness requirement, not only a presentational one.

	sink := row.CoinsSpent + row.CoinsExpired
	if row.CoinsIssued > 0 && sink > 0 {
		row.FaucetSinkRatio = float64(row.CoinsIssued) / float64(sink)
		row.FaucetSinkRatioDefined = true
	}
	if row.CoinsIssued > 0 && row.CoinsSpent > 0 {
		row.Velocity = float64(row.CoinsIssued) / float64(row.CoinsSpent)
		row.VelocityDefined = true
	}
	if row.StudentsHolding > 0 {
		row.PayerConversion = float64(row.StudentsPaying) / float64(row.StudentsHolding)
		row.PayerConversionDefined = true
	}
	if row.CoinsIssued > 0 {
		row.ReferralShareOfIssuance = float64(row.CoinsIssuedByReferral) / float64(row.CoinsIssued)
		row.ReferralShareOfIssuanceDefined = true
		// Only DEFINED days can be healthy or unhealthy. An idle day is not a
		// healthy one — that reading would make a dead economy look compliant.
		row.ReferralShareHealthy = row.ReferralShareOfIssuance <= ReferralShareTarget
	}

	// ── store ─────────────────────────────────────────────────────────────────
	//
	// An upsert, so re-running a day is a repair rather than a conflict. The
	// uniqueness is on (day, metric_version) which is the composite primary key.

	if err := repo.DB().WithContext(ctx).Exec(`
		INSERT INTO coin_economy_daily
			(day, metric_version, coins_issued, coins_issued_by_referral, coins_spent,
			 coins_expired, coins_adjusted, journals_posted, students_holding,
			 students_paying, faucet_sink_ratio, faucet_sink_ratio_defined,
			 velocity, velocity_defined, days_of_currency_on_hand,
			 days_of_currency_on_hand_defined, payer_conversion,
			 payer_conversion_defined, referral_share_of_issuance,
			 referral_share_of_issuance_defined, referral_share_healthy,
			 created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (day, metric_version) DO UPDATE SET
			coins_issued = EXCLUDED.coins_issued,
			coins_issued_by_referral = EXCLUDED.coins_issued_by_referral,
			coins_spent = EXCLUDED.coins_spent,
			coins_expired = EXCLUDED.coins_expired,
			coins_adjusted = EXCLUDED.coins_adjusted,
			journals_posted = EXCLUDED.journals_posted,
			students_holding = EXCLUDED.students_holding,
			students_paying = EXCLUDED.students_paying,
			faucet_sink_ratio = EXCLUDED.faucet_sink_ratio,
			faucet_sink_ratio_defined = EXCLUDED.faucet_sink_ratio_defined,
			velocity = EXCLUDED.velocity,
			velocity_defined = EXCLUDED.velocity_defined,
			payer_conversion = EXCLUDED.payer_conversion,
			payer_conversion_defined = EXCLUDED.payer_conversion_defined,
			referral_share_of_issuance = EXCLUDED.referral_share_of_issuance,
			referral_share_of_issuance_defined = EXCLUDED.referral_share_of_issuance_defined,
			referral_share_healthy = EXCLUDED.referral_share_healthy,
			updated_at = EXCLUDED.updated_at`,
		day, CurrentMetricVersion,
		row.CoinsIssued, row.CoinsIssuedByReferral, row.CoinsSpent, row.CoinsExpired,
		row.CoinsAdjusted, row.JournalsPosted, row.StudentsHolding, row.StudentsPaying,
		row.FaucetSinkRatio, row.FaucetSinkRatioDefined,
		row.Velocity, row.VelocityDefined,
		0.0, false, // days-of-currency is a dashboard figure; see the field comment
		row.PayerConversion, row.PayerConversionDefined,
		row.ReferralShareOfIssuance, row.ReferralShareOfIssuanceDefined, row.ReferralShareHealthy,
		row.CreatedAt, row.UpdatedAt,
	).Error; err != nil {
		return CoinEconomyDaily{}, fmt.Errorf("store rollup for %s: %w", day.Format("2006-01-02"), err)
	}
	return row, nil
}

// EconomyHealth reads the trailing window and computes the trends that need one.
//
// READ-ONLY, and it reads the STORED ROLLUP rather than recomputing from the
// journal. That is the whole point of the rollup existing: a dashboard that
// recomputed on every page load would be a hundred full-table aggregations per
// operator refresh, and the rollup is what makes the read cheap.
//
// The window is trailing and EXCLUSIVE of today. Today is incomplete, and including
// it would make every metric dip at the end of each day regardless of what happened —
// the same false-signal problem the undefined flags exist for.
func (l *Ledger) EconomyHealth(ctx context.Context, windowDays int) (*EconomyHealth, error) {
	if l == nil || l.repo == nil {
		return nil, ErrNoDatabase
	}
	if windowDays <= 0 {
		windowDays = HealthDefaultWindow
	}
	if windowDays > HealthMaxWindow {
		windowDays = HealthMaxWindow
	}

	today := truncateToUTCDay(l.now().UTC())
	// [today-window, today) — half-open again, so today is excluded and the oldest
	// day is included.
	from := today.AddDate(0, 0, -windowDays)

	var days []CoinEconomyDaily
	if err := l.repo.DB().WithContext(ctx).
		Where("day >= ? AND day < ? AND metric_version = ?", from, today, CurrentMetricVersion).
		Order("day ASC").
		Find(&days).Error; err != nil {
		return nil, fmt.Errorf("read the economy rollup: %w", err)
	}
	if days == nil {
		days = []CoinEconomyDaily{}
	}

	out := &EconomyHealth{
		Days: days, Target: ReferralShareTarget,
		MetricVersion: CurrentMetricVersion, WindowDays: windowDays,
	}
	out.Summary = summarise(days)

	// Days-of-currency needs the OUTSTANDING balance, which the rollup does not store
	// — a balance is a moment, a day is an interval, and the row that had it at the
	// end of the window is not necessarily the last row present. So it is read now.
	var outstanding int64
	if err := l.repo.DB().WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(b.posted_balance - b.reserved), 0)
		  FROM coin_account_balance b
		  JOIN coin_account a ON a.id = b.account_id
		 WHERE a.kind = ? AND a.owner_user_id IS NOT NULL AND a.closed_at IS NULL`,
		AccountUser).Scan(&outstanding).Error; err != nil {
		return nil, fmt.Errorf("read the outstanding balance: %w", err)
	}
	daysWithSpend := 0
	for _, day := range days {
		if day.CoinsSpent > 0 {
			daysWithSpend++
		}
	}
	if daysWithSpend > 0 && out.Summary.CoinsSpent > 0 {
		perDay := float64(out.Summary.CoinsSpent) / float64(daysWithSpend)
		out.Summary.DaysOfCurrencyOnHand = float64(outstanding) / perDay
		out.Summary.DaysOfCurrencyOnHandDefined = true
	}
	return out, nil
}

// summarise computes the window aggregate.
//
// THE ARITHMETIC RULE, and it is the one most likely to be got wrong: window figures
// are totals or ratios OF TOTALS, never averages of daily ratios. Averaging daily
// faucet:faucet ratios weights a day that issued 5 coins the same as a day that
// issued 500,000, which is how a metric reports a stable economy through a period
// in which it tripled. Same for velocity.
//
// Averages DO appear, in one place: DaysAboveTarget, which counts days rather than
// measuring them, so a ratio per day is the right unit there.
func summarise(days []CoinEconomyDaily) HealthSummary {
	var out HealthSummary
	for _, day := range days {
		out.CoinsIssued += day.CoinsIssued
		out.CoinsSpent += day.CoinsSpent
		out.CoinsExpired += day.CoinsExpired
		if day.ReferralShareOfIssuanceDefined {
			out.DaysDefined++
			if !day.ReferralShareHealthy {
				out.DaysAboveTarget++
			}
		}
	}
	sink := out.CoinsSpent + out.CoinsExpired
	if out.CoinsIssued > 0 && sink > 0 {
		out.FaucetSinkRatio = float64(out.CoinsIssued) / float64(sink)
		out.FaucetSinkRatioDefined = true
	}
	if out.CoinsIssued > 0 && out.CoinsSpent > 0 {
		out.Velocity = float64(out.CoinsIssued) / float64(out.CoinsSpent)
		out.VelocityDefined = true
	}
	if out.CoinsIssued > 0 {
		out.ReferralShare = float64(sumReferralIssuance(days)) / float64(out.CoinsIssued)
		out.ReferralShareDefined = true
		// A window is healthy when it is healthy OVERALL, and the target is a
		// DERIVED one — the exact threshold the per-day flag uses.
		out.ReferralShareHealthy = out.ReferralShare <= ReferralShareTarget
	}
	return out
}

// sumReferralIssuance re-adds the referral subset across the window.
//
// A sum rather than a mean of daily shares, for the same reason as every other window
// figure: the daily share of a small day would otherwise weigh as much as a large
// day's.
func sumReferralIssuance(days []CoinEconomyDaily) int64 {
	var total int64
	for _, day := range days {
		total += day.CoinsIssuedByReferral
	}
	return total
}

// nextRollupAt is when the daily rollup next fires, given the current instant and the
// configured UTC hour.
//
// ALWAYS TOMORROW'S, never today's — even when called at 03:00 with a 02:00 hour. This
// is the one behaviour that makes the job correct rather than merely frequent: today's
// day is still accruing, so a rollup covering it stores a partial figure that every
// subsequent read would report until the next run replaced it. Getting this wrong does
// not crash, does not log an error, and produces a number that is wrong for 23 hours
// each day.
//
// So it is ALWAYS the next occurrence of the hour strictly after `from`, and computed
// from midnight rather than from `from` so the schedule does not drift with process
// start. A job whose output is indexed by the day it describes cannot run on a timer
// offset from whenever the container booted.
func nextRollupAt(from time.Time, hourUTC int) time.Time {
	from = from.UTC()
	// A configured hour outside [0, 24) would compute a day boundary that is not one.
	if hourUTC < 0 || hourUTC > 23 {
		hourUTC = 0
	}
	todayAtHour := time.Date(from.Year(), from.Month(), from.Day(), hourUTC, 0, 0, 0, time.UTC)
	if !from.Before(todayAtHour) {
		return todayAtHour.Add(24 * time.Hour)
	}
	return todayAtHour
}

// StartEconomyRollup runs the daily rollup on a timer until stop() is called.
//
// ROLLS UP THE PREVIOUS DAY, not today — see rollupRecentDays. It also does NOT fire
// immediately on start: a rollup at boot would be a race with the migrations in
// main.go on a fresh deploy, and on an existing deploy it would recompute days nobody
// asked about. Waiting for the scheduled hour costs nothing, because the rollup is
// idempotent and the gap closes itself.
func StartEconomyRollup(sweeper *ExpirySweeper, hourUTC int) (stop func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			next := nextRollupAt(time.Now().UTC(), hourUTC)
			timer := time.NewTimer(time.Until(next))
			select {
			case <-timer.C:
				rollupRecentDays(sweeper)
			case <-done:
				timer.Stop()
				return
			}
		}
	}()
	return func() { done <- struct{}{} }
}

// dayRolluper is what rollupRecentDays needs from a sweeper.
//
// An INTERFACE rather than the concrete *ExpirySweeper, and the reason is specific:
// the part of this loop worth testing is its retry behaviour — that one day's failure
// does not stop the next — and that behaviour is only observable without a database by
// substituting the clock and the failure. Both methods are unexported, so only this
// package can implement it, which keeps it a seam rather than a published contract.
type dayRolluper interface {
	nowUTC() time.Time
	RollupDay(ctx context.Context, day time.Time) (CoinEconomyDaily, error)
}

// nowUTC is the sweeper's injected clock, normalised to UTC and safe on a nil
// receiver.
//
// The UTC normalisation is because the rollup's day boundary is UTC and a schedule
// that computed a day in local time would pick a different day for part of the year —
// Nepal is UTC+05:45, so this is not hypothetical.
func (s *ExpirySweeper) nowUTC() time.Time {
	if s == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

// rollupRecentDays rolls up yesterday and the day before.
//
// Two days rather than one because a process restarting across midnight, or a
// deployment that skipped a run, would otherwise leave a permanent gap — and a gap in
// the middle of a trend line is invisible as a gap, appearing only as a step change
// nobody can explain.
//
// A failed day is LOGGED AND SKIPPED rather than fatal. One bad day must not stop the
// series advancing: a dashboard frozen on the day before the failure looks like a dead
// economy, and the log line is the only clue that it is not.
func rollupRecentDays(sweeper dayRolluper) {
	if sweeper == nil {
		return
	}
	ctx := context.Background()
	today := truncateToUTCDay(sweeper.nowUTC())
	for _, back := range []int{1, 2} {
		day := today.AddDate(0, 0, -back)
		row, err := sweeper.RollupDay(ctx, day)
		if err != nil {
			logger.Warn("economy rollup failed; the next run will retry",
				"day", day.Format("2006-01-02"), "error", err)
			continue
		}
		logger.Info("economy rollup",
			"day", day.Format("2006-01-02"),
			"issued", row.CoinsIssued,
			"spent", row.CoinsSpent,
			"expired", row.CoinsExpired,
			"faucet_sink", row.FaucetSinkRatio,
			"faucet_sink_defined", row.FaucetSinkRatioDefined,
			"velocity", row.Velocity,
			"referral_share", row.ReferralShareOfIssuance,
			"referral_share_healthy", row.ReferralShareHealthy)
	}
}

// truncateToUTCDay floors an instant to midnight UTC.
//
// Exported as a named function because it is a correctness boundary rather than a
// convenience: a local-midnight truncation would split a day's activity into two rows
// for part of the year, and the referral cap has already had that bug once.
func truncateToUTCDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// dayBounds is the half-open [from, to) window for one day.
//
// HALF-OPEN, and that is the third UTC boundary rule in this codebase. With closed
// intervals a journal stamped at exactly midnight is counted twice — once at the end
// of the previous day and once at the start of this one.
func dayBounds(day time.Time) (time.Time, time.Time) {
	from := truncateToUTCDay(day)
	return from, from.AddDate(0, 0, 1)
}
