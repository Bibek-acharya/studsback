package coins

import (
	"encoding/json"
	"strings"
	"testing"
)

// The PUBLIC coin table — the disclosure Consumer Protection Act 2075 s.16(2)(n)
// requires, and 04 §6 names as a Phase 4 deliverable.
//
// Every assertion here is about what the PUBLIC may see, so the tests are written
// mostly as NEGATIVE assertions: the surface is easy to get right by accident and
// then leak by a later field being added to the struct. A test that only checked the
// values would pass on a response that also published the fraud thresholds.
//
// The rule the whole file is built on: the public table is a PROJECTION of the
// economy config, not the config. Every excluded field is excluded on purpose, and
// each exclusion has a reason recorded next to it in public_table.go.

// THE test. A configured economy, and exactly the fields the Act requires and
// nothing else.
func TestThePublicTableDisclosesPriceIncludedAndTimeAndNothingMore(t *testing.T) {
	cfg := DefaultEconomyConfig()
	// A recognisable config, so a field that silently kept the DEFAULT instead of
	// reading through would be visible as a wrong number rather than a right one.
	cfg.Prices = PriceConfig{StudyResource: 40, Video: 90, MockTest: 25}
	cfg.Expiry = ExpiryConfig{FreeDays: 90, EarnedDays: 365, ActivityExtendDays: 180}
	cfg.Allowance = AllowanceConfig{
		DocumentUnlocks: 3, VideoUnlocks: 1, MockTestUnlocks: 1, ExpiresInDays: 90,
	}

	got := PublicCoinTableFrom(cfg)

	if got.Prices.StudyResource != 40 || got.Prices.Video != 90 || got.Prices.MockTest != 25 {
		t.Errorf("prices = %+v, want 40/90/25 — s.16(2)(n) requires the price", got.Prices)
	}
	if got.Expiry.IncludedDays != 90 || got.Expiry.EarnedDays != 365 {
		t.Errorf("expiry = %+v, want 90/365 — s.16(2)(n) requires the time", got.Expiry)
	}
	if got.Included.DocumentUnlocks != 3 {
		t.Errorf("included document unlocks = %d, want 3 — 04 §6 names what is included",
			got.Included.DocumentUnlocks)
	}
	if got.Included.ExpiresInDays != 90 {
		t.Errorf("the included allowance's own expiry = %d, want 90", got.Included.ExpiresInDays)
	}
}

// The exclusions, each asserted by KEY NAME because a struct field that is merely
// zero still serialises. This is the test that fails the day someone adds a field to
// EconomyConfig and PublicCoinTableFrom grows a line for it without thinking.
func TestThePublicTableDisclosesNoEarnRateNoFraudCapAndNoInternalSwitch(t *testing.T) {
	cfg := DefaultEconomyConfig()
	cfg.Awards = AwardConfig{
		ProfileComplete: 200, ProfileInstalment: 50, ProfileInstalments: 4,
		ReferralReferrer: 300, ReferralReferred: 200, ResourceApproved: 80,
	}
	cfg.Referral = ReferralConfig{MonthlyCap: 10, LifetimeCoinCap: 5000, HoldDays: 7}
	cfg.ClawbackWindowDays = 14
	cfg.UnlockEndpointEnabled = true

	encoded, err := json.Marshal(PublicCoinTableFrom(cfg))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(encoded)

	// EARN RATES ARE NOT PRICES. s.16(2)(n) obliges a provider to state what it
	// CHARGES. An earn rate is what the platform gives, and publishing the referral
	// payout hands an attacker the return on a fabricated referral — which is exactly
	// what 05 §5's fraud ratio exists to watch, and a fraud ratio whose threshold is
	// published is not a control.
	//
	// The key names are checked, not the values: a `referral_referrer: 0` would still
	// tell a reader the mechanic exists and is a field the API promised.
	for _, banned := range []string{
		"profile_complete", "profile_instalment", "profile_instalments",
		"referral_referrer", "referral_referred", "resource_approved",
		"monthly_cap", "lifetime_coin_cap", "hold_days",
		"clawback_window_days", "unlock_endpoint_enabled",
		// The public keys must not carry the config's banned vocabulary either.
		"free_days", "refundable", "cash_outable", "purchasable", "transferable",
	} {
		if strings.Contains(body, banned) {
			t.Errorf("the public table discloses %q — an earn rate or a fraud threshold, "+
				"neither of which s.16(2)(n) requires and either of which is a fraud signal", banned)
		}
	}
}

// THE COMPLIANCE-CRITICAL ONE. A published price for a class whose gate is OFF is an
// advertisement for a charge that is not being made.
//
// This is CPA 2075 s.16(2)(c)(3) — advertising a service "in false and misleading
// manner … even when no benefit is obtained as declared" — which 07 calls the
// functional analogue of a free-claims rule. Right now every gate is dark, so a table
// that printed "40 StudsTokens" with no qualification would be advertising a price
// for content a student still gets at no charge.
//
// So charges_apply travels WITH prices, always, and the frontend cannot render one
// without the other.
func TestThePublicTableSaysWhetherEachPriceActuallyApplies(t *testing.T) {
	cfg := DefaultEconomyConfig()
	cfg.Gates = GatesEnabledConfig{StudyResource: false, Video: true, MockTest: false}
	cfg.UnlockEndpointEnabled = true

	got := PublicCoinTableFrom(cfg)

	if got.ChargesApply.StudyResource {
		t.Error("study resources are published at 40 StudsTokens but the gate is off")
	}
	if !got.ChargesApply.Video {
		t.Error("video is gated on but the table does not say the charge applies")
	}
	if got.ChargesApply.MockTest {
		t.Error("mock tests are ungated but the table claims a charge applies")
	}

	// And the inverse trap: a gate being ON is not enough. The document path also has
	// its own unlock-endpoint switch, and turning the gate live without it would leave
	// the site charging at the download while the unlock endpoint refuses — two prices
	// for one document. The table reports the DOCUMENT class as ungated unless both
	// are on, which is the pair main.go's comment calls the real switch.
	cfg.Gates = GatesEnabledConfig{StudyResource: true}
	cfg.UnlockEndpointEnabled = false
	got = PublicCoinTableFrom(cfg)
	if got.ChargesApply.StudyResource {
		t.Error("the document gate is on but the unlock endpoint is dark, so a document " +
			"has no single price; the table must not claim one")
	}
	cfg.UnlockEndpointEnabled = true
	got = PublicCoinTableFrom(cfg)
	if !got.ChargesApply.StudyResource {
		t.Error("both document switches are on, so the charge does apply")
	}
}

// The structural facts, as BOOLEANS rather than prose.
//
// 07's mitigation list says the first thing a reader should find is the STRUCTURE, not
// the product name — the name contains "Token" and the Consumer Protection Act names
// "token system" in its pyramid-scheme prohibition. Publishing these as booleans rather
// than as copy means the frontend cannot drift from them: a paragraph of prose can
// quietly stop matching the code, and a boolean cannot.
func TestThePublicTableStatesTheStructureAsBooleans(t *testing.T) {
	got := PublicCoinTableFrom(DefaultEconomyConfig())
	terms := got.Terms

	// ADR-001: coins are never purchasable, transferable or cash-outable. These three
	// are load-bearing across the whole product, not restatements of policy.
	if terms.CanBeBought || terms.CanBeSentToOthers || terms.CanBeConvertedToMoney {
		t.Errorf("the table claims StudsTokens can be bought, sent or cashed out: %+v", terms)
	}
	// 07 mitigation #2: state the single-level structure proactively, so the reader
	// finds the structure before the name. A number, not a word — the word "level"
	// together with "token" is the phrase that invites the question.
	if terms.ReferralLevels != 1 {
		t.Errorf("referral_levels = %d, want 1 — 07 records that a multi-level tree is "+
			"not a grey area under s.16(2)(p) and s.18(e)", terms.ReferralLevels)
	}
	// 09: "Spent StudsTokens stay spent." There is no refund path at all.
	if !terms.SpendsAreFinal {
		t.Error("the table does not say spends are final; the ledger refuses Reverse outright")
	}
	// The one that is NOT a permanent rule, and is here so the frontend can say
	// something true about it rather than nothing.
	if terms.ExpiredCoinsRecoverable {
		t.Error("expired coins are not recovered for anything; they are burned")
	}
}

// 09's hard bans, enforced on the RESPONSE rather than on the frontend.
//
// The frontend can be edited by anyone and the copy can drift; these are the strings
// that make a claim the Act treats as an offence or the Income Tax Act treats as a
// taxable windfall gain, so they are checked where they are produced. A test that
// checked the .tsx would pass while the API served the banned word to a screen reader
// or a third-party client.
func TestThePublicTableContainsNoBannedWord(t *testing.T) {
	cfg := DefaultEconomyConfig()
	encoded, err := json.Marshal(PublicCoinTableFrom(cfg))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := strings.ToLower(string(encoded))

	// "free" is the one that carries a criminal penalty. The allowed spellings are
	// "included" and "starter" — this API emits no prose at all, so any occurrence is
	// a mistake.
	for _, banned := range []string{"free", "prize", "award", "winner", "raffle", "baksis", "cash out", "refundable"} {
		if strings.Contains(body, banned) {
			t.Errorf("the public table contains %q — 09 bans it, and the API is the only "+
				"place that cannot be edited past review", banned)
		}
	}
	// Currency beside a coin figure is the second hard ban. No currency unit may
	// appear anywhere in the payload.
	for _, banned := range []string{"npr", "rs.", "rupee", "₨", "$"} {
		if strings.Contains(body, banned) {
			t.Errorf("the public table contains the currency marker %q beside a coin figure; "+
				"StudsTokens have no money value, so the claim would be false on the product's own terms", banned)
		}
	}
}

// A zero or absent config must produce a readable table, not a division by zero or a
// panic. This state is reachable: the config lives in system_settings and a fresh
// database has no row, so Load returns the defaults — but a partially-written row, or
// a config written by an older build, can carry zeros.
func TestThePublicTableSurvivesADegenerateConfig(t *testing.T) {
	for name, cfg := range map[string]EconomyConfig{
		"all zero": {},
		"negative": {Prices: PriceConfig{StudyResource: -5}, Expiry: ExpiryConfig{FreeDays: -1}},
		"defaults": DefaultEconomyConfig(),
	} {
		t.Run(name, func(t *testing.T) {
			got := PublicCoinTableFrom(cfg)
			// A negative price in storage would render as "costs -5 StudsTokens", which
			// is a price nobody can read. Validation refuses to WRITE one, but this is
			// the load path and it must not launder a bad stored value into published
			// copy — it clamps to zero and the row shows as not charging.
			if got.Prices.StudyResource < 0 || got.Prices.Video < 0 || got.Prices.MockTest < 0 {
				t.Errorf("a negative price reached the public table: %+v", got.Prices)
			}
			if got.Expiry.IncludedDays < 0 || got.Expiry.EarnedDays < 0 {
				t.Errorf("a negative expiry reached the public table: %+v", got.Expiry)
			}
			// And it must serialise, because a page that 500s on an empty economy is
			// worse than one that says nothing is chargeable yet.
			if _, err := json.Marshal(got); err != nil {
				t.Errorf("marshal: %v", err)
			}
		})
	}
}

// The one derived figure. "Earned coins last 12 months, extended to 18 on qualifying
// activity" is two numbers a student can check, and 06 §10 requires the extended
// figure rather than the base one so nobody is told a shorter life than they get.
func TestTheExtendedExpiryIsPublishedAndNotShorterThanTheBase(t *testing.T) {
	cfg := DefaultEconomyConfig()
	cfg.Expiry = ExpiryConfig{FreeDays: 90, EarnedDays: 365, ActivityExtendDays: 180}

	got := PublicCoinTableFrom(cfg)

	if got.Expiry.EarnedDaysExtended != 365+180 {
		t.Errorf("earned_days_extended = %d, want %d (365 + 180)",
			got.Expiry.EarnedDaysExtended, 365+180)
	}
	if got.Expiry.EarnedDaysExtended < got.Expiry.EarnedDays {
		t.Errorf("the extended life (%d) is shorter than the base life (%d); a student "+
			"would be told they lose coins sooner than the policy gives them",
			got.Expiry.EarnedDaysExtended, got.Expiry.EarnedDays)
	}

	// And a config where the extension is zero must not claim an extension.
	cfg.Expiry.ActivityExtendDays = 0
	got = PublicCoinTableFrom(cfg)
	if got.Expiry.EarnedDaysExtended != got.Expiry.EarnedDays {
		t.Errorf("with no extension configured, earned_days_extended = %d, want the base %d",
			got.Expiry.EarnedDaysExtended, got.Expiry.EarnedDays)
	}
}
