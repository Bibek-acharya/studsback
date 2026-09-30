package coins

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// compliantEconomyConfig is a reachability-compliant variant of the shipped
// defaults, built HERE rather than in DefaultEconomyConfig.
//
// This is deliberate and it is the whole reason several of these tests can be
// written at all. The figures in DefaultEconomyConfig break both rules — the
// product owner deferred setting them — so any test that wants to exercise the
// ACCEPT side of the rules has to supply compliant figures of its own. Putting
// them here would change what the shipped config is, and changing the shipped
// config is the decision this change is explicitly not making.
//
// It moves the awards only, not the prices, and it lands exactly on both
// boundaries: the lowest award (40) equals the cheapest price (40), and the
// largest award (90) equals the dearest price (90). So every "this passes"
// assertion below is an assertion that equality is inside the rule.
func compliantEconomyConfig() EconomyConfig {
	cfg := DefaultEconomyConfig()
	cfg.Prices = PriceConfig{
		StudyResource: 40,
		Video:         90,
		MockTest:      60,
	}
	// 8 x 5 = 40, which is the lowest award and exactly meets the 40-coin
	// document. The ladder still has to balance, which is why this is an
	// instalment of 8 and not a bare profile_complete of 40.
	cfg.Awards = AwardConfig{
		ProfileComplete:    40,
		ProfileInstalment:  8,
		ProfileInstalments: 5,
		ReferralReferrer:   60,
		ReferralReferred:   40,
		ResourceApproved:   90,
	}
	return cfg
}

// seedConfig writes cfg into the fake settings store as the stored coin_economy
// row, which is what Load reads as its merge base.
func seedConfig(t *testing.T, settings *fakeSettings, cfg EconomyConfig) {
	t.Helper()
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("encode seed config: %v", err)
	}
	settings.values[EconomyConfigSettingKey] = string(encoded)
}

// violationByRule indexes the violations so a test can assert on one rule
// without depending on the order the two are reported in.
func violationByRule(violations []ReachabilityViolation, rule string) *ReachabilityViolation {
	for i := range violations {
		if violations[i].Rule == rule {
			return &violations[i]
		}
	}
	return nil
}

// requireSubstrings fails with the offending string when want is missing from
// got. The messages this check produces are the operator's only route to fixing
// a non-compliant config, so the numbers in them are part of the contract and
// are asserted rather than eyeballed.
func requireSubstrings(t *testing.T, label, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("%s does not mention %q\nfull text: %s", label, w, got)
		}
	}
}

// The shipped defaults are NOT clean, and this test is the one that says so.
//
// A test asserting the current config validates would be the wrong test: the
// point of the change is that the current config is broken and is reported as
// such. What is asserted here is the arithmetic of both violations — which
// award, which price, and by how much — because an operator has to be able to
// act on the message without reading reachability.go.
func TestCheckReachabilityDetectsBothCurrentViolations(t *testing.T) {
	violations := CheckReachability(DefaultEconomyConfig())
	if len(violations) != 2 {
		t.Fatalf("violations=%d want 2: the defaults must be reported as breaking both rules, got %+v", len(violations), violations)
	}

	// Rule 1: profile completion pays 25 and the cheapest item costs 40.
	low := violationByRule(violations, RuleAwardReachesCheapestPrice)
	if low == nil {
		t.Fatalf("rule %s was not reported; violations were %+v", RuleAwardReachesCheapestPrice, violations)
	}
	if low.Field != "awards.profile_complete" {
		t.Errorf("rule %s names field %q, want awards.profile_complete (the first of the tied lowest awards)", RuleAwardReachesCheapestPrice, low.Field)
	}
	// 40 - 25 = 15. Both tied awards are named, because either is a valid fix.
	requireSubstrings(t, "rule 1 message", low.Message,
		"awards.profile_complete", "awards.referral_referred", // both tied lowest awards
		"= 25",                          // the award
		"15",                            // the shortfall
		"prices.study_resource", "= 40", // the cheapest price
	)

	// Rule 2: a video costs 90 and the largest single award pays 80.
	high := violationByRule(violations, RuleNoPriceExceedsLargestAward)
	if high == nil {
		t.Fatalf("rule %s was not reported; violations were %+v", RuleNoPriceExceedsLargestAward, violations)
	}
	if high.Field != "prices.video" {
		t.Errorf("rule %s names field %q, want prices.video (the only class above the largest award)", RuleNoPriceExceedsLargestAward, high.Field)
	}
	// 90 - 80 = 10.
	requireSubstrings(t, "rule 2 message", high.Message,
		"prices.video", "= 90",
		"10",                               // the overshoot
		"awards.resource_approved", "= 80", // the largest single award
	)

	// And ValidateEconomyConfig's field rules are NOT what is complaining.
	// The defaults break the invariant and nothing else, so a rejection of them
	// on those grounds is about the loop and not about a bad field.
	err := ValidateReachability(DefaultEconomyConfig())
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("ValidateReachability error = %v, want a *ValidationError", err)
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Error("the reachability rejection does not match ErrInvalidConfig, so the admin handler would not map it to 400")
	}
	if rules := ruleFields(t, err); len(rules) != 2 ||
		rules[0].Field != "awards.profile_complete" || rules[1].Field != "prices.video" {
		t.Errorf("rejection rules = %+v, want awards.profile_complete then prices.video", rules)
	}
	if !strings.Contains(err.Error(), "15") || !strings.Contains(err.Error(), "10") {
		t.Errorf("the joined rejection message lost the arithmetic: %s", err.Error())
	}
}

// awards.profile_instalment is a slice of the profile payout, not an award in
// its own right, and the difference is the whole content of rule 1. If the
// 5-coin instalment counted, the reported "lowest award" would be 5 rather than
// 25 and the rule would be a complaint about instalment granularity rather than
// about whether the loop closes.
func TestTheProfileInstalmentIsNotCountedAsASingleAward(t *testing.T) {
	v := violationByRule(CheckReachability(DefaultEconomyConfig()), RuleAwardReachesCheapestPrice)
	if v == nil {
		t.Fatal("rule 1 was not reported at all")
	}
	if strings.Contains(v.Message, "awards.profile_instalment ") {
		t.Errorf("the instalment was counted as an award: %s", v.Message)
	}
	requireSubstrings(t, "rule 1 message", v.Message, "= 25")
	if strings.Contains(v.Message, "= 5 coins") {
		t.Errorf("the 5-coin instalment was reported as the lowest award: %s", v.Message)
	}
}

// Both rules are satisfied AT equality. An award exactly equal to the cheapest
// price passes and one coin short fails; a price exactly equal to the largest
// award passes and one coin more fails. Which side of that line an amount falls
// on is the entire content of the rule, and coins are indivisible, so this is
// the boundary that matters.
func TestReachabilityBoundariesAreInclusive(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*EconomyConfig)
		wantErr bool
		wantSub string
	}{
		{
			name:   "lowest award exactly equal to the cheapest price passes",
			mutate: func(*EconomyConfig) {},
		},
		{
			// 39 < 40. ReferralReferred rather than ProfileComplete, because
			// dropping the profile total by one would break the instalment
			// ladder and this test is about the boundary, not about that rule.
			name:    "lowest award one coin short of the cheapest price fails",
			mutate:  func(c *EconomyConfig) { c.Awards.ReferralReferred = 39 },
			wantErr: true,
			wantSub: "is 1 coin short of the cheapest price",
		},
		{
			// Mock test sits between the two boundaries, so moving it must not
			// trip either rule.
			name:    "a price above the cheapest and below the largest is not a violation",
			mutate:  func(c *EconomyConfig) { c.Prices.MockTest = 41 },
			wantErr: false,
		},
		{
			name:    "a price exactly equal to the largest award passes",
			mutate:  func(c *EconomyConfig) { c.Prices.MockTest = 90 },
			wantErr: false,
		},
		{
			name:    "a price one coin above the largest award fails",
			mutate:  func(c *EconomyConfig) { c.Prices.MockTest = 91 },
			wantErr: true,
			wantSub: "costs 1 coin more than the largest single award",
		},
		{
			// The mirror of the first case, so the inclusive edge of rule 2 is
			// proven from both directions rather than only from above.
			name:    "largest award exactly equal to the dearest price passes",
			mutate:  func(c *EconomyConfig) {},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := compliantEconomyConfig()
			tc.mutate(&cfg)

			err := ValidateReachability(cfg)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("boundary case rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("boundary case accepted; one coin outside the rule is a violation")
			}
			if tc.wantSub != "" {
				requireSubstrings(t, "rejection", err.Error(), tc.wantSub)
			}
		})
	}
}

// The rule is not "reject everything": a config that satisfies both rules is
// accepted and persisted. Without this the write path would simply be closed,
// which would pass every rejection test above.
//
// The figures are compliantEconomyConfig's, supplied by the test. The shipped
// defaults are untouched.
func TestUpdateEconomyConfigAcceptsAWriteThatSatisfiesReachability(t *testing.T) {
	svc, settings, versions, _ := testService(t)
	seedConfig(t, settings, compliantEconomyConfig())

	updated, err := svc.UpdateEconomyConfig(UpdateEconomyConfigRequest{
		// Raises the largest single award so the 90-coin video becomes
		// reachable, which is the deferred numbers decision being made for real
		// rather than by a code change.
		Awards: &UpdateAwardsRequest{ResourceApproved: i64(120)},
	}, 7)
	if err != nil {
		t.Fatalf("a write that satisfies both rules was rejected: %v", err)
	}
	if settings.writes != 1 {
		t.Fatalf("writes=%d want 1", settings.writes)
	}
	if len(versions.rows) != 1 {
		t.Fatalf("version rows=%d want 1", len(versions.rows))
	}
	if updated.Awards.ResourceApproved != 120 || updated.Prices.Video != 90 {
		t.Fatalf("update not applied: %+v", updated)
	}
	if err := ValidateReachability(updated); err != nil {
		t.Fatalf("the persisted config does not satisfy the invariant: %v", err)
	}
}

// An admin write that breaks a rule is refused whole: nothing persisted, no
// audit row, and the stored config untouched. The rejection has to carry the
// numbers, because the admin is looking at a form and the log.
func TestUpdateEconomyConfigRejectsAWriteThatBreaksReachability(t *testing.T) {
	svc, settings, versions, _ := testService(t)
	base := compliantEconomyConfig()
	seedConfig(t, settings, base)
	if _, err := svc.GetEconomyConfig(); err != nil {
		t.Fatalf("prime: %v", err)
	}

	// 500 is above the 90-coin largest single award, so this breaks rule 2 and
	// only rule 2: the cheapest price is still 40 and the lowest award is still
	// 40. An isolated single-rule rejection proves the rule is the reason and
	// not a coincidence of the base config.
	_, err := svc.UpdateEconomyConfig(UpdateEconomyConfigRequest{
		Prices: &UpdatePricesRequest{MockTest: i64(500)},
	}, 3)
	if err == nil {
		t.Fatal("a write pricing a class above the largest single award was accepted")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error %v does not match ErrInvalidConfig, so the handler would not map it to 400", err)
	}

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error = %v, want a *ValidationError", err)
	}
	rules := ruleFields(t, err)
	if len(rules) != 1 {
		t.Fatalf("rule fields=%d want 1 (only rule 2 is broken by this write): %+v", len(rules), rules)
	}
	if rules[0].Field != "prices.mock_test" {
		t.Errorf("rejection names field %q, want prices.mock_test", rules[0].Field)
	}
	// 500 - 90 = 410, and both sides of the comparison are named.
	requireSubstrings(t, "rejection", rules[0].Message,
		RuleNoPriceExceedsLargestAward,
		"prices.mock_test", "= 500",
		"410",
		"awards.resource_approved", "= 90",
	)

	if settings.writes != 0 {
		t.Errorf("writes=%d want 0: a rejected write must not persist", settings.writes)
	}
	if len(versions.rows) != 0 {
		t.Errorf("version rows=%d want 0 for a rejected write", len(versions.rows))
	}
	got, err := svc.GetEconomyConfig()
	if err != nil {
		t.Fatalf("get after rejection: %v", err)
	}
	if got != base {
		t.Errorf("the rejected write changed the config: %+v", got)
	}
}

// While the stored config is non-compliant, EVERY admin write is refused —
// including one that edits something unrelated — because the merge base is
// non-compliant and a merge can only add violations, never remove them.
//
// This is a consequence with teeth, so it is pinned: it is the forcing function
// that makes the deferred figures decision get made in one admin write, and it
// is also why an admin screen will have to send the figures together. If this
// test starts failing because a write was accepted on top of a non-compliant
// base, the invariant has been weakened, not the config fixed.
func TestUpdateEconomyConfigRejectsEveryWriteWhileTheStoredConfigIsNonCompliant(t *testing.T) {
	svc, settings, _, _ := testService(t)
	// No seed: Load falls back to DefaultEconomyConfig, which is the current
	// real state of a deployment that has never been configured.
	if violations := CheckReachability(DefaultEconomyConfig()); len(violations) == 0 {
		t.Skip("the defaults are compliant now; this test is about the non-compliant state and no longer applies")
	}

	_, err := svc.UpdateEconomyConfig(UpdateEconomyConfigRequest{
		// Touches nothing the invariant is about. Still refused.
		ClawbackWindowDays: i64(90),
	}, 5)
	if err == nil {
		t.Fatal("a write on top of a non-compliant base was accepted; the invariant is not being enforced on the result")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error %v does not match ErrInvalidConfig", err)
	}
	requireSubstrings(t, "rejection", err.Error(),
		RuleAwardReachesCheapestPrice, RuleNoPriceExceedsLargestAward)
	if settings.writes != 0 {
		t.Errorf("writes=%d want 0", settings.writes)
	}

	// And the way out is one write carrying compliant figures, not a code
	// change. This is the same transition the accepting test above makes from
	// the other direction.
	fixed := compliantEconomyConfig()
	if _, err := svc.UpdateEconomyConfig(UpdateEconomyConfigRequest{
		Prices: &UpdatePricesRequest{
			StudyResource: i64(fixed.Prices.StudyResource),
			Video:         i64(fixed.Prices.Video),
			MockTest:      i64(fixed.Prices.MockTest),
		},
		Awards: &UpdateAwardsRequest{
			ProfileComplete:    i64(fixed.Awards.ProfileComplete),
			ProfileInstalment:  i64(fixed.Awards.ProfileInstalment),
			ProfileInstalments: i64(fixed.Awards.ProfileInstalments),
			ReferralReferrer:   i64(fixed.Awards.ReferralReferrer),
			ReferralReferred:   i64(fixed.Awards.ReferralReferred),
			ResourceApproved:   i64(fixed.Awards.ResourceApproved),
		},
	}, 5); err != nil {
		t.Fatalf("the write that brings the config into compliance was rejected: %v", err)
	}
	if settings.writes != 1 {
		t.Fatalf("writes=%d want 1 after the compliant write", settings.writes)
	}
}

// DO NOT DELETE THIS TEST WITHOUT DELIBERATE DECISION.
//
// It is the only thing standing between a future edit and a server that will not
// boot. The stored coin_economy row and the defaults it falls back to both break
// both reachability rules today, and Load is the path every reader in the
// process goes through. If the invariant is ever promoted from a warning to an
// error at load time, this test fails first — and that is the intent: the
// promotion is a product decision (see the "WHAT WOULD HAVE TO BE TRUE FOR IT TO
// BECOME AN ERROR" block on coins.ReachabilityWarning) that must be made on
// purpose, with the figures fixed, the gates read in production, and every
// environment migrated, rather than landed as a drive-by refactor.
//
// The second half asserts that the loaded config is STILL non-compliant, so
// this test cannot go quietly vacuous: the day the figures are fixed, it fails
// and says so.
func TestLoadingTheCurrentNonCompliantConfigStillSucceeds(t *testing.T) {
	tests := []struct {
		name  string
		store func(*fakeSettings)
	}{
		{
			// The defaults it falls back to when nothing is stored — a fresh
			// environment, and every environment until an admin writes.
			name:  "falling back to the defaults",
			store: func(*fakeSettings) {},
		},
		{
			// The stored row. Unmarshalled onto the defaults, so this is the
			// shape a row written by an older build reads back as.
			name: "reading the stored coin_economy row",
			store: func(s *fakeSettings) {
				s.values[EconomyConfigSettingKey] = `{"prices":{"study_resource":40,"video":90,"mock_test":60},` +
					`"awards":{"profile_complete":25,"profile_instalment":5,"profile_instalments":5,` +
					`"referral_referrer":60,"referral_referred":25,"resource_approved":80}}`
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settings := newFakeSettings()
			tc.store(settings)

			cfg, err := NewConfigStore(settings).Load()
			if err != nil {
				t.Fatalf("loading a non-compliant config failed and the server would not start: %v", err)
			}
			if cfg.Prices.Video != 90 || cfg.Awards.ResourceApproved != 80 {
				t.Fatalf("the stored config did not load: %+v", cfg)
			}

			// And it IS still non-compliant, so this test cannot pass by
			// accident against a config that was quietly fixed.
			if violations := CheckReachability(cfg); len(violations) == 0 {
				t.Fatal("the loaded config is reachability-compliant now. If the figures were fixed, " +
					"re-read the promotion criteria on coins.ReachabilityWarning and decide whether the " +
					"check can become fatal — and delete this test deliberately when it does.")
			}

			// Read it twice: the cached path must be as forgiving as the first
			// read, or the check is only protecting the first caller after boot.
			if _, err := NewConfigStore(settings).Load(); err != nil {
				t.Fatalf("second load failed: %v", err)
			}
		})
	}
}

// The startup warning is a named token an operator can grep for or alert on, it
// carries the figures, and it is silent when the loop closes. It also reports
// whether anything can actually reach a student, because that is what decides
// whether the operator treats it as a launch blocker or an incident.
func TestReachabilityWarningIsNamedAndGradesItsUrgency(t *testing.T) {
	if warning := ReachabilityWarning(compliantEconomyConfig()); warning != "" {
		t.Errorf("a compliant config produced a warning: %s", warning)
	}

	warning := ReachabilityWarning(DefaultEconomyConfig())
	if warning == "" {
		t.Fatal("the non-compliant defaults produced no warning")
	}
	requireSubstrings(t, "warning", warning,
		ReachabilityWarningName, // greppable
		EconomyConfigSettingKey,
		RuleAwardReachesCheapestPrice, RuleNoPriceExceedsLargestAward,
		"= 25", "15", "prices.study_resource", "= 40",
		"= 90", "10", "awards.resource_approved", "= 80",
		"Nothing is live yet", // every gate is off, so this is not customer-facing yet
	)
	if !strings.HasPrefix(warning, ReachabilityWarningName+":") {
		t.Errorf("the warning does not start with its name, so grepping for it misses the line: %s", warning)
	}

	// The same broken figures with a gate on are a different sentence: now a
	// student can reach them.
	live := DefaultEconomyConfig()
	live.Gates.StudyResource = true
	requireSubstrings(t, "warning with a gate on", ReachabilityWarning(live),
		"LIVE RIGHT NOW", "gates_enabled.study_resource")
}

// The rejection an admin actually reads. The only production caller validates the
// MERGED result, so a violation may pre-date the request: an admin who edited
// only clawback_window_days is refused for a price they never touched. Without a
// preamble saying so, the rejection reads as though their own edit caused it.
func TestAReachabilityRejectionSaysTheViolationMayPreDateTheRequest(t *testing.T) {
	cfg := DefaultEconomyConfig()
	err := ValidateReachability(cfg)
	if err == nil {
		t.Fatal("the shipped defaults must violate the invariant; if they no longer do, the fixtures are stale")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("rejection is %T, want *ValidationError so the existing 400 mapping applies", err)
	}
	if len(ve.Fields) == 0 {
		t.Fatal("rejection carried no fields")
	}
	first := ve.Fields[0]
	if first.Field != "reachability" {
		t.Errorf("first field is %q, want the preamble to lead", first.Field)
	}
	for _, want := range []string{"PREDATE", "same request", "merged economy"} {
		if !strings.Contains(first.Message, want) {
			t.Errorf("preamble does not say %q:\n%s", want, first.Message)
		}
	}
	// The specific, fixable rules must still follow it, or the admin is told what
	// is wrong and not what to do.
	if len(ve.Fields) < 2 {
		t.Fatalf("rejection has %d fields; the preamble replaced the rules instead of leading them", len(ve.Fields))
	}
	if !strings.Contains(ve.Fields[1].Message, "cheapest price") {
		t.Errorf("second field is not a specific rule:\n%s", ve.Fields[1].Message)
	}
}

// ruleFields strips the leading reachability preamble so a test about the RULES
// is not coupled to the preamble existing, its field name, or its position.
func ruleFields(t *testing.T, err error) []FieldError {
	t.Helper()
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error = %v, want a *ValidationError", err)
	}
	out := make([]FieldError, 0, len(verr.Fields))
	for _, f := range verr.Fields {
		if f.Field == "reachability" {
			continue
		}
		out = append(out, f)
	}
	return out
}
