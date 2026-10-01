// internal/coins/referral_cap_model.go
//
// referral_cap_slot: how the monthly referral cap is made a database invariant
// rather than a service-layer comparison.
//
// ── why the cap is not a SELECT COUNT(*) in Go ────────────────────────────────
//
// 05-economy-and-fraud.md §2.4 sets monthly_cap = 10 successful referrals per
// user per calendar month, and says the count cap is primary and the coin ceiling
// is derived from it. A count check written in Go looks like:
//
//	paid := countQualifiedThisMonth(referrerID)
//	if paid >= monthlyCap { return ErrCapReached }
//
// and is wrong in a specific, expensive way. Between the count and the write, two
// concurrent qualifications for the SAME referrer both read 9 and both insert an
// eleventh. The per-user advisory lock does not save it either: it serialises the
// two transactions, so the second one re-reads a count of 10 — provided the count
// is re-read INSIDE the transaction, which a caller has to remember, and which is
// exactly the kind of "remember" this codebase has already been bitten by twice
// (the liability=false default, and the UNIQUE (account_id, account_seq)
// constraint that was documented in the spec and never created).
//
// So the cap is a claim table. Paying a referral means INSERTing a row whose
// (referrer_user_id, month_key, slot) is unique, and the CHECK bounds slot at 10.
// An eleventh successful referral in a month has no slot to take, so the insert is
// refused by the database. That is the same shape as reward_grant's
// ON CONFLICT claim and profile_award.go's awardStep, for the same reason.
//
// ── the CHECK ceiling and the configurable cap ───────────────────────────────
//
// chk_referral_cap_slot bounds slot at 10, the launch cap, as a literal — a CHECK
// cannot read coin_economy. referral.monthly_cap in the config may therefore be
// LOWERED at runtime and the query honours it (claimMonthlySlot is called with
// the configured cap and refuses an ordinal above it), but RAISING it above 10
// requires widening the CHECK.
//
// That asymmetry is deliberate and is the honest reading of §2.4: the config knob
// exists so an operator can tighten a cap without a deploy during an incident,
// and the schema ceiling exists so a runaway config edit cannot turn a 10-per-
// month cap into an unlimited faucet. The two agree at launch, and TestReferral-
// CapCeilingAndConfigAgree pins that they do.
package coins

import "time"

// ReferralCapSlot is one consumed slot of one referrer's monthly referral cap.
//
// The row's existence IS the claim. There is no counter to lose track of and no
// read-modify-write: the rows are the counter, exactly as reward_grant's rows are
// the profile ladder's counter.
type ReferralCapSlot struct {
	// ReferrerUserID is the student the slot belongs to.
	ReferrerUserID uint `gorm:"not null;primaryKey;autoIncrement:false" json:"referrer_user_id"`
	// MonthKey is the calendar month in UTC, 'YYYY-MM'.
	//
	// UTC rather than local time, and worth being explicit about: the cap is
	// "per calendar month", and a referrer in Nepal (UTC+5:45) whose cap reset at
	// local midnight could open the new month at 18:15 UTC the previous day —
	// which means two months' caps overlapping by six hours, twice a year, at the
	// exact moment the cap resets. One timezone for every user removes the class
	// of bug entirely.
	MonthKey string `gorm:"type:char(7);primaryKey;autoIncrement:false" json:"month_key"`
	// Slot is the ordinal within the month, 1..monthly_cap. Bounded by
	// chk_referral_cap_slot.
	Slot int `gorm:"not null;primaryKey;autoIncrement:false" json:"slot"`
	// ReferralID is the referral that consumed the slot. UNIQUE on its own, and
	// that is what stops one referral consuming two slots and therefore being
	// counted twice against two different months — the multi-month version of the
	// double-claim that user_referral_referred_uniq already prevents for a pair.
	//
	// The uniqueness rides on the struct tag rather than on an
	// addConstraint() in referral_indexes.go, for one reason that has bitten this
	// codebase before: GORM builds a composite unique index only when EVERY column
	// names the same index, and having the database own it separately means two
	// objects (a constraint and a tag-generated constraint) over one column that
	// AutoMigrate then tries to reconcile on the next run — which fails with
	// "constraint does not exist" (42704) the moment the two disagree about naming.
	// AutoMigrate owns it, so there is only ever one.
	ReferralID uint `gorm:"not null;uniqueIndex:referral_cap_slot_referral_uniq" json:"referral_id"`
	// Coins is what the slot was worth, snapshotted. A cap is a bound on COINS as
	// well as a count once the award value changes mid-month, and the derived
	// lifetime ceiling needs a running total that does not change retroactively
	// when an operator re-prices.
	Coins int64 `gorm:"not null" json:"coins"`
	// AwardedAt is when the slot was claimed.
	AwardedAt time.Time `gorm:"not null" json:"awarded_at"`
}

func (ReferralCapSlot) TableName() string { return "referral_cap_slot" }

// ReferralCapSlotModels is the AutoMigrate slice for the cap-slot table.
var ReferralCapSlotModels = []any{&ReferralCapSlot{}}

// ReferralCapSlotCeiling is the largest ordinal chk_referral_cap_slot permits.
//
// It is a constant rather than a literal inside the SQL so that the ceiling is
// one number in the codebase, referred to by the comment that explains it, and
// asserted by a test against the constraint the database actually has.
const ReferralCapSlotCeiling = 10

// referralCapSlotConstraints is the DDL EnsureReferralIndexes applies to the cap
// table, kept here so the ceiling constant and the CHECK are in the same file. See
// referral_indexes.go for the rest of the referral constraints.
//
// chk_referral_cap_slot_ceiling is the one named after the ceiling it enforces,
// rather than folding the bound into chk_referral_cap_slot. A constraint a reader
// has to parse an expression to find is a constraint nobody checks.
const referralCapSlotCeilingCheck = `slot <= 10`
