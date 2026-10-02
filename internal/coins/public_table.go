// internal/coins/public_table.go
//
// The public coin table — Consumer Protection Act 2075 s.16(2)(n), and 04 §6's
// "public coin table page".
//
// ── why this is a PROJECTION and not the config ──────────────────────────────────
//
// The obvious implementation is to serve EconomyConfig and let the frontend pick the
// fields it needs. That is the wrong shape, and the reason is that a projection is
// reviewed once while an exposed config is re-exposed by every future field.
//
// s.16(2)(n) obliges a professional service provider to specify price, quality, venue
// and time. That is the whole obligation. It says nothing about what the platform
// PAYS, so every earn rate is outside it — and the referral payout in particular is
// the return on a fabricated referral, which is what 05 §5's fraud ratio exists to
// watch. A published fraud threshold is not a control.
//
// So this file enumerates the disclosure and the file's tests enumerate the
// EXCLUSIONS. Both matter: a test that only checked the values would pass on a
// response that also published the monthly referral cap.
//
// ── the price is published WITH the switch that makes it real ───────────────────
//
// The single most important field here is charges_apply, and it exists because of CPA
// s.16(2)(c)(3) — advertising "in false and misleading manner … even when no benefit
// is obtained as declared", which 07 calls the functional analogue of a free-claims
// rule. A table that printed "40 StudsTokens" while the document gate was off would be
// advertising a charge that is not being made.
//
// Every gate currently ships dark, so this is not hypothetical: as configured today,
// the honest table says no charge applies to anything.
//
// ── structure is published as BOOLEANS, not as prose ───────────────────────────
//
// 07's mitigation list: the product name contains "Token" and the statute names
// "token system" in its pyramid-scheme prohibition, so the first thing a reader should
// find is the STRUCTURE. A boolean cannot drift from the code that enforces it. A
// paragraph of prose can, silently, and the drift is invisible until a regulator reads
// both.
//
// ── no prose at all ─────────────────────────────────────────────────────────────
//
// There is not one sentence of user-facing copy in this file, and that is on purpose.
// 09's three hard bans — "free", currency beside a coin figure, and the Income Tax
// Act's windfall words — are enforced at the point the data is produced, because the
// .tsx can be edited by anyone and the API is what a third-party client reads too.
// The wording is the frontend's; the claims are this file's.

package coins

// PublicCoinTable is what an unauthenticated reader may know about the economy.
//
// It is a DTO with no reference to EconomyConfig so that adding a field to the config
// cannot widen it by accident: the compiler refuses, which is the property that makes
// this a projection rather than a filtered view.
type PublicCoinTable struct {
	// Prices is the coin cost of unlocking one item of each class — the "price" limb
	// of s.16(2)(n). Zero means nothing in that class is chargeable.
	Prices PriceConfig `json:"prices"`

	// ChargesApply says whether each published price is ACTUALLY being charged.
	//
	// Always published alongside Prices, never instead of them, because a price
	// without its qualifier is the misleading advertisement. See the file header.
	ChargesApply PublicChargeState `json:"charges_apply"`

	// Included is the free-allowance entitlement: what an account has without
	// unlocking. 04 §6 names "what is included" as a required element.
	//
	// The field is called `included` and NOT `allowance` because 09 bans the word
	// "free" outright when any payment is on the other side, and "allowance" is
	// neutral while a key named `free_unlocks` would not be.
	Included AllowanceConfig `json:"included"`

	// Expiry is the "time" limb: how long coins live, and how long the included
	// unlocks live.
	Expiry PublicExpiry `json:"expiry"`

	// Terms is the structure. Every field is a permanent product invariant from
	// ADR-001 except ExpiredCoinsRedeemed, which exists so the page can say
	// something true rather than stay silent.
	Terms PublicTerms `json:"terms"`
}

// PublicChargeState is per-class: does the published price apply right now?
//
// A struct of three bools rather than a map so a class cannot be added to the config
// without this being updated to match, and a gate can never be published without a
// qualifier.
type PublicChargeState struct {
	StudyResource bool `json:"study_resource"`
	Video         bool `json:"video"`
	MockTest      bool `json:"mock_test"`
}

// PublicExpiry is the expiry policy as the page states it.
//
// The key names deliberately do NOT reuse the config's vocabulary — `free_days`
// becomes `included_days` — and that is not cosmetic. A JSON key is what a frontend
// turns into a visible label, so a key containing a word 09 bans is a banned word one
// mapping away from a human being. The internal config keeps its vocabulary; this is
// a projection, and renaming here costs nothing upstream.
type PublicExpiry struct {
	// IncludedDays is the life of the coins in the included starter allowance.
	//
	// Named `included_days` and NOT `free_days` — see the struct comment. The config
	// keeps its own vocabulary; this projection does not, because a key is what a
	// frontend turns into a visible label.
	IncludedDays int64 `json:"included_days"`
	// EarnedDays is the life of coins a student earned by qualifying.
	EarnedDays int64 `json:"earned_days"`
	// EarnedDaysExtended is EarnedDays plus the qualifying-activity extension.
	//
	// Published as its own number rather than left for the frontend to add up,
	// because 06 §10 requires the page to state the EXTENDED figure: a student told
	// "12 months" who then reads a rule that extends it to 18 has been told less than
	// they are owed. The frontend should never be doing arithmetic on a policy.
	EarnedDaysExtended int64 `json:"earned_days_extended"`
	// ActivityExtendDays is how much qualifying activity buys. Zero means the
	// extension does not exist and the page must not mention it.
	ActivityExtendDays int64 `json:"activity_extend_days"`
}

// PublicTerms is the structure, as POSITIVE claims.
//
// `can_be_bought: false` rather than `purchasable: false`, and `spends_are_final`
// rather than `refundable_after_spend`, for the same reason as the expiry keys: a
// frontend maps these straight to labels, and a label reading "Refundable: no" puts the
// banned word in front of the reader while "Spent StudsTokens stay spent" does not.
// Every name here is one a developer can render without touching a policy word.
type PublicTerms struct {
	// CanBeBought, CanBeSentToOthers and CanBeConvertedToMoney are all false forever —
	// ADR-001. They are published because 09 requires the page to answer "can I buy
	// these?" without a euphemism, and a reader who has to ask is a reader who
	// suspects.
	CanBeBought           bool `json:"can_be_bought"`
	CanBeSentToOthers     bool `json:"can_be_sent_to_others"`
	CanBeConvertedToMoney bool `json:"can_be_converted_to_money"`
	// SpendsAreFinal is always true. 09's replacement copy is exactly this: "Spent
	// StudsTokens stay spent." The ledger refuses Reverse outright, and 04 §4.4 records
	// the absence of a refund path as deliberate rather than as a gap.
	SpendsAreFinal bool `json:"spends_are_final"`
	// ExpiredCoinsRecoverable is always false: expired coins are burned, not exchanged
	// for anything.
	ExpiredCoinsRecoverable bool `json:"expired_coins_recoverable"`
	// ReferralLevels is 1 — a single-level programme.
	//
	// Published proactively, per 07's mitigation list, so the reader encounters the
	// structure before the name. A number rather than the words "single level",
	// because "level" beside "token" is the phrase that invites the question.
	ReferralLevels int `json:"referral_levels"`
}

// PublicCoinTableFrom projects an economy config onto the public table.
//
// PURE — no database, no clock — because the compliance review of this projection
// should not require a database to exercise, and because the thing most likely to go
// wrong here is a field copied from the wrong place.
//
// The negatives are the projection: a negative stored value becomes zero rather than
// being published. ValidateEconomyConfig refuses to WRITE one, but this is the load
// path, and laundered into public copy a negative price renders as "costs -5
// StudsTokens" — a figure no reader can interpret and an advertisement nobody can act
// on.
func PublicCoinTableFrom(cfg EconomyConfig) PublicCoinTable {
	charges := PublicChargeState{
		// The document class takes BOTH switches, per the Gates field's own comment:
		// the gate makes the resource chargeable, and the unlock endpoint is what a
		// paid unlock is actually spent on. One without the other means there is no
		// single price for a document, and the honest answer is that no charge
		// applies.
		StudyResource: cfg.Gates.StudyResource && cfg.UnlockEndpointEnabled,
		Video:         cfg.Gates.Video,
		MockTest:      cfg.Gates.MockTest,
	}
	// A price of zero is not a price; it is a statement that the class is not
	// chargeable, so it never claims to apply regardless of the switch.
	if clampNonNegative(cfg.Prices.StudyResource) == 0 {
		charges.StudyResource = false
	}
	if clampNonNegative(cfg.Prices.Video) == 0 {
		charges.Video = false
	}
	if clampNonNegative(cfg.Prices.MockTest) == 0 {
		charges.MockTest = false
	}

	earned := clampNonNegative(cfg.Expiry.EarnedDays)
	extension := clampNonNegative(cfg.Expiry.ActivityExtendDays)

	return PublicCoinTable{
		Prices: PriceConfig{
			StudyResource: clampNonNegative(cfg.Prices.StudyResource),
			Video:         clampNonNegative(cfg.Prices.Video),
			MockTest:      clampNonNegative(cfg.Prices.MockTest),
		},
		ChargesApply: charges,
		Included: AllowanceConfig{
			DocumentUnlocks: clampNonNegative(cfg.Allowance.DocumentUnlocks),
			VideoUnlocks:    clampNonNegative(cfg.Allowance.VideoUnlocks),
			MockTestUnlocks: clampNonNegative(cfg.Allowance.MockTestUnlocks),
			ExpiresInDays:   clampNonNegative(cfg.Allowance.ExpiresInDays),
		},
		Expiry: PublicExpiry{
			IncludedDays:       clampNonNegative(cfg.Expiry.FreeDays),
			EarnedDays:         earned,
			EarnedDaysExtended: earned + extension,
			ActivityExtendDays: extension,
		},
		Terms: PublicTerms{
			CanBeBought:           false,
			CanBeSentToOthers:     false,
			CanBeConvertedToMoney: false,
			// 09: "Spent StudsTokens stay spent." The ledger refuses Reverse
			// outright — 04 §4.4 records the absence of a refund path as deliberate.
			SpendsAreFinal: true,
			// Expired coins are burned by the expiry sweep; nothing is exchanged.
			ExpiredCoinsRecoverable: false,
			// Single level. 07 treats a multi-level tree as squarely inside
			// s.16(2)(p) and s.18(e), so this is the number the page leads with.
			ReferralLevels: 1,
		},
	}
}

// clampNonNegative floors a stored amount at zero.
//
// A named function rather than inline clamping at each site so the rule has one
// implementation and the reason is written once.
func clampNonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
