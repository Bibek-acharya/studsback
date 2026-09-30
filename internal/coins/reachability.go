// Reachability is the invariant that the earn loop and the spend loop can meet:
// a student can earn something, and what they earn can buy something.
//
// Why it needed its own file. The prices and the awards in this config were
// derived from two different models and nobody reconciled them. The prices came
// out of the CAC table in 05-economy-and-fraud.md §2.2, which prices an item by
// what an unlock is worth against acquisition cost. The awards came out of an
// effort model, which prices an act by how much work it is. Each is defensible
// alone; together they shipped a loop that does not close:
//
//   - the cheapest item costs 40 and the easiest award pays 25, so the easiest
//     thing a student can do buys nothing at all;
//   - a video costs 90 and the largest single award pays 80, so no single act of
//     earning can buy a video even on the admin-gated upload path.
//
// The loop only closed by combining the hardest award with a second one.
//
// WHAT THIS FILE DOES AND DOES NOT DECIDE. It adds the mechanism: the two rules
// below are named, they are stated once, they are evaluated in one pure
// function, and an admin cannot persist a combination that breaks them. It does
// NOT change a single figure. The numbers are the product owner's to set and
// that decision is explicitly deferred, so the shipped defaults still break both
// rules — which is why the enforcement is asymmetric (hard on write, loud on
// load) rather than uniform. See ValidateReachability and
// ReachabilityWarning for the split and the reason for it.
package coins

import (
	"fmt"
	"strings"
)

// The two rules, as named constants rather than as prose, because they are the
// vocabulary the admin rejection, the startup warning and the tests all share:
// an operator can be told "you broke <rule>" and look it up, and a test can
// assert on the name rather than on a substring of a sentence that may be
// reworded.
const (
	// RuleAwardReachesCheapestPrice is rule 1. The lowest-value single award a
	// student can earn must be able to buy the cheapest item. This is what
	// stops "the easiest thing you can do is worth less than the cheapest thing
	// you can buy", which is the failure that makes an earn loop feel broken to
	// the one person it is aimed at.
	RuleAwardReachesCheapestPrice = "every_award_reaches_cheapest_price"

	// RuleNoPriceExceedsLargestAward is rule 2. No price may exceed the largest
	// single award. One act of earning must be able to buy any one item, so a
	// student never has to stack rewards — or wait for a second, rarer act — to
	// reach something affordable.
	RuleNoPriceExceedsLargestAward = "no_price_exceeds_largest_award"
)

// ReachabilityWarningName is the stable token the startup warning is prefixed
// with. It is a constant because the point of it is that an operator (or an
// alert) can grep for one string; a token embedded in a sentence that gets
// reworded is not greppable. Do not change it without treating that as a
// breaking change to the runbook.
const ReachabilityWarningName = "STUDS_TOKEN_ECONOMY_REACHABILITY_VIOLATED"

// ReachabilityViolation is one broken rule, carrying the arithmetic that broke
// it. Rule and Field are for code and logs; Message is for whoever has to fix
// it without reading this file.
type ReachabilityViolation struct {
	// Rule is the stable name from the constants above.
	Rule string
	// Field is the JSON path an admin would edit to resolve it, so the
	// rejection lands on the field they sent rather than on a synthetic key.
	// Where two fields could equally be the fix (the tied lowest award), the
	// first in the declared order is named here and the rest are named in the
	// message.
	Field string
	// Message states the failing comparison with the actual figures and the
	// exact shortfall, then says what the rule is for.
	Message string
}

// amountEntry is a configured amount paired with the JSON path an admin edits.
type amountEntry struct {
	path  string
	value int64
}

// singleAwards is the set of payouts a student can earn in ONE act.
//
// awards.profile_instalment is deliberately NOT in this list. It is a slice of
// the profile_complete payout, not a separate act: completing a profile pays
// awards.profile_complete coins, delivered as awards.profile_instalments
// instalments of awards.profile_instalment coins (see
// AwardConfig.instalmentProduct). Including the instalment would report the
// smallest slice of a ladder as the smallest award, which would turn rule 1 into
// a complaint about instalment granularity — "pay 40 coins in instalments of 5"
// — rather than about whether the loop closes. awards.profile_instalments is a
// count, not an amount, so it is not an award either.
//
// The order is fixed rather than map order, so the reported field and the
// reported message are deterministic when two awards tie for the minimum.
func singleAwards(a AwardConfig) []amountEntry {
	return []amountEntry{
		{"awards.profile_complete", a.ProfileComplete},
		{"awards.referral_referrer", a.ReferralReferrer},
		{"awards.referral_referred", a.ReferralReferred},
		{"awards.resource_approved", a.ResourceApproved},
	}
}

// pricedClasses is the set of things a student can spend coins on, in the
// declared class order the rest of the package uses (study_resource, video,
// mock_test). Same determinism reason as singleAwards.
func pricedClasses(p PriceConfig) []amountEntry {
	return []amountEntry{
		{"prices.study_resource", p.StudyResource},
		{"prices.video", p.Video},
		{"prices.mock_test", p.MockTest},
	}
}

// lowestOf returns the smallest value in entries and the paths that share it,
// in declared order. Callers pass a non-empty slice: singleAwards has four
// entries and pricedClasses has three, both fixed at compile time.
func lowestOf(entries []amountEntry) (int64, []string) {
	best := entries[0].value
	for _, e := range entries[1:] {
		if e.value < best {
			best = e.value
		}
	}
	return best, pathsAt(entries, best)
}

// highestOf returns the largest value in entries and the paths that share it,
// in declared order. Same non-empty precondition as lowestOf.
func highestOf(entries []amountEntry) (int64, []string) {
	best := entries[0].value
	for _, e := range entries[1:] {
		if e.value > best {
			best = e.value
		}
	}
	return best, pathsAt(entries, best)
}

func pathsAt(entries []amountEntry, value int64) []string {
	paths := make([]string, 0, 1)
	for _, e := range entries {
		if e.value == value {
			paths = append(paths, e.path)
		}
	}
	return paths
}

// namePaths renders one or more paths as "a", "a or b" — so a tie is reported as
// a tie rather than silently resolving to whichever came first in the slice.
func namePaths(paths []string) string {
	switch len(paths) {
	case 0:
		return "(none)"
	case 1:
		return paths[0]
	default:
		return strings.Join(paths[:len(paths)-1], ", ") + " or " + paths[len(paths)-1]
	}
}

// coinWord renders an amount with its noun, so a one-coin shortfall reads as
// "1 coin short" rather than "1 coins short". It is a message-quality fix, but
// the one-coin case is the boundary case this whole file is about and a typo
// there is the kind of thing that makes an operator distrust the message.
func coinWord(n int64) string {
	if n == 1 {
		return "1 coin"
	}
	return fmt.Sprintf("%d coins", n)
}

// CheckReachability evaluates both rules against cfg and returns every
// violation, or nil when the loop closes.
//
// It is a pure function with no database, no cache and no logger, which is what
// makes it safe to call from the admin write path and from the startup check
// and testable on its own. It is O(1) — seven numbers — and the admin write
// path is not hot, so there is nothing to cache.
//
// Both rules are inequalities, and both are satisfied at equality: an award
// exactly equal to the cheapest price passes, and a price exactly equal to the
// largest award passes. Paying exactly the cost of the thing is the boundary
// case that must work — a shortfall of one coin is a violation and an exact
// match is not, and which side of that line you land on is the whole content of
// the rule.
//
// Rule 1 produces at most one violation (the lowest award is a single number),
// rule 2 one per class priced above the largest award. Both are reported rather
// than short-circuiting, so an admin sees every edit their request needs in one
// 400 instead of discovering the second problem on the second attempt.
func CheckReachability(cfg EconomyConfig) []ReachabilityViolation {
	awards := singleAwards(cfg.Awards)
	prices := pricedClasses(cfg.Prices)

	lowestAward, lowestAwardPaths := lowestOf(awards)
	cheapestPrice, cheapestPricePaths := lowestOf(prices)
	largestAward, largestAwardPaths := highestOf(awards)

	var violations []ReachabilityViolation

	if lowestAward < cheapestPrice {
		violations = append(violations, ReachabilityViolation{
			Rule:  RuleAwardReachesCheapestPrice,
			Field: lowestAwardPaths[0],
			Message: fmt.Sprintf(
				"%s: the lowest single award %s = %d coins is %s short of the cheapest price %s = %d; "+
					"raise the lowest award to at least %d, or lower the cheapest price to at most %d, "+
					"so that the easiest thing a student can earn buys the cheapest thing they can",
				RuleAwardReachesCheapestPrice,
				namePaths(lowestAwardPaths), lowestAward,
				coinWord(cheapestPrice-lowestAward),
				namePaths(cheapestPricePaths), cheapestPrice,
				cheapestPrice, lowestAward,
			),
		})
	}

	for _, p := range prices {
		if p.value > largestAward {
			violations = append(violations, ReachabilityViolation{
				Rule:  RuleNoPriceExceedsLargestAward,
				Field: p.path,
				Message: fmt.Sprintf(
					"%s: %s = %d costs %s more than the largest single award %s = %d; "+
						"lower this price to at most %d, or raise the largest single award to at least %d, "+
						"so that one act of earning can buy any one item",
					RuleNoPriceExceedsLargestAward,
					p.path, p.value,
					coinWord(p.value-largestAward),
					namePaths(largestAwardPaths), largestAward,
					largestAward, p.value,
				),
			})
		}
	}

	return violations
}

// ValidateReachability enforces the two reachability rules as a *ValidationError,
// the same type ValidateEconomyConfig produces, so a caller — and the admin
// handler's errors.Is(err, ErrInvalidConfig) mapping to 400 — cannot tell which
// rule refused the write. A rejected write is rejected whole: nothing is
// persisted and no version row is appended.
//
// WHY THIS IS NOT INSIDE ValidateEconomyConfig. It is a real rule about the
// config and the natural place to look for it is the validator, so the absence is
// worth stating rather than leaving as a trap. Three things put it here
// instead:
//
//  1. ValidateEconomyConfig answers "is this field value legal". These rules
//     answer "does the loop close" — a statement about the config as a whole,
//     between seven numbers that no single field owns. Every other rule names a
//     field that is wrong; these name a relationship.
//  2. ValidateEconomyConfig is called from the gate tests to assert that a
//     gate-only edit is otherwise a valid config. Folding the invariant in
//     would make those tests fail on figures they are not about, and the fix
//     would be to either rewrite them or weaken them — both worse than a second
//     named entry point.
//  3. Keeping them apart is what lets the startup path call the same rule as a
//     warning while this path calls it as an error. One evaluator, two
//     severities, one seam (see ReachabilityWarning).
//
// The consequence of (3), stated so nobody discovers it the hard way: the shipped
// defaults and the stored coin_economy row both break these rules today, and the
// merge base for an admin write is the stored config. So while the stored
// config is non-compliant, EVERY admin write is rejected — including a write
// that only edits something unrelated — because the merged result still breaks
// the rules. That is deliberate: the first write has to bring the figures into
// compliance, and it must do it in the same request. The alternative (only
// reject a write that makes an existing violation worse) would let an admin
// persist a broken combination through the back door, which is precisely what
// this rule exists to prevent. The way out is one admin write carrying
// compliant figures, not a code change.
func ValidateReachability(cfg EconomyConfig) error {
	violations := CheckReachability(cfg)
	if len(violations) == 0 {
		return nil
	}
	// The preamble goes FIRST, and it is not decoration. The only production
	// caller validates the MERGED result of an admin write, so a violation here
	// may pre-date the request entirely: an admin who edited only
	// clawback_window_days is refused for a price they never touched. Without
	// this line the rejection reads as if their own edit caused it, which is
	// baffling and sends them looking for the wrong field.
	fields := make([]FieldError, 0, len(violations)+1)
	fields = append(fields, FieldError{Field: "reachability", Message: reachabilityPreamble})
	for _, v := range violations {
		fields = append(fields, FieldError{Field: v.Field, Message: v.Message})
	}
	return &ValidationError{Fields: fields}
}

// reachabilityPreamble leads every reachability rejection. It says three things,
// in order: what is being checked, that the violation may pre-date the request,
// and what the way out is.
const reachabilityPreamble = "the merged economy breaks the coin reachability invariant: " +
	"the lowest single award must buy the cheapest item, and no price may exceed the " +
	"largest single award. A violation named below may PREDATE this request — if the " +
	"stored config is already non-compliant, every write is refused until one request " +
	"brings the figures into compliance, even a write that only edited unrelated " +
	"fields. Fix the figures named below in the same request."

// ReachabilityWarning renders the one startup warning this invariant produces.
// It returns "" when the loop closes, so the caller can log the string
// unconditionally guarded by a single emptiness check.
//
// Returned as a string rather than logged from here so that the coins package
// keeps no dependency on the logger, and so the exact text an operator sees is
// an ordinary function return that a test can assert on.
//
// ============================ READ THIS BEFORE "FIXING" IT ============================
//
// WHY THIS IS A WARNING AND NOT AN ERROR. The obvious edit to this file is to
// make a non-empty result fatal: return it from Load, or call logger.Fatal at
// the startup call site. Do not make that edit as a cleanup. It is right for the
// wrong reason and it takes the server down.
//
// The stored coin_economy row and DefaultEconomyConfig both break both rules
// right now, and every gate is switched off — the economy has been built but
// not launched. Promoting the check to fatal therefore does not refuse to start
// a broken LIVE economy; it refuses to start at all, over an economy nobody has
// launched, on a server whose other, working features would then also be
// unavailable. A documented broken combination that an operator sees in one log
// line is strictly less bad than a process that will not boot because the
// StudsToken pricing decision has not been made yet.
//
// Note also that the invariant being violated is not itself customer-harmful
// while the gates are dark: nothing is charged and nothing is earned, because
// nothing is reachable. The failure mode it prevents is a student discovering
// mid-launch that the reward they were promised buys nothing. There is no
// student to disappoint yet.
//
// WHAT WOULD HAVE TO BE TRUE FOR IT TO BECOME AN ERROR. All of these, not some:
//
//   - the figures are set. DefaultEconomyConfig and the stored row satisfy both
//     rules. Today they do not, and the fix is the product decision, not code.
//   - the gates have been read in production. Every gate and
//     unlock_endpoint_enabled is still an unexercised rollback path
//     (04-implementation-plan.md §4.4). Failing startup on a config path that has
//     never run live is failing on a hypothetical.
//   - there is a migration, not just a check. Making it fatal means every
//     environment — including a developer's local database and any CI fixture
//     seeded from an older dump — has to have been migrated to compliant
//     figures first, or it will not start.
//
// When those hold, the promotion is two deliberate edits: return the error from
// the startup call site in cmd/server/main.go, and delete
// TestLoadingTheCurrentNonCompliantConfigStillSucceeds in reachability_test.go,
// which exists precisely so that this edit cannot be made by accident.
//
// =========================================================================================
func ReachabilityWarning(cfg EconomyConfig) string {
	violations := CheckReachability(cfg)
	if len(violations) == 0 {
		return ""
	}

	parts := make([]string, 0, len(violations))
	for i, v := range violations {
		parts = append(parts, fmt.Sprintf("(%d/%d) %s", i+1, len(violations), v.Message))
	}

	return fmt.Sprintf(
		"%s: the loaded %s config breaks the coin reachability invariant in %d place(s): %s. "+
			"%s "+
			"The prices and the awards were derived separately (05-economy-and-fraud.md §2.2 prices from CAC, "+
			"the awards from effort) and never reconciled; the figures are the product owner's to set and have "+
			"not been changed here.",
		ReachabilityWarningName,
		EconomyConfigSettingKey,
		len(violations),
		strings.Join(parts, "; "),
		reachabilityUrgency(cfg),
	)
}

// reachabilityUrgency is the last sentence of the warning: whether anything can
// actually reach a student under this configuration right now.
//
// It is here rather than in the caller because the answer needs the config, and
// because it is the sentence that decides whether the operator treats the log
// line as a launch blocker or a ticket. With everything dark the broken figures
// cannot be reached by anyone, so the fix is "before the gates go on"; with
// something live the same figures are charging students today, which is a
// different and more urgent sentence.
func reachabilityUrgency(cfg EconomyConfig) string {
	live := make([]string, 0, 4)
	if cfg.Gates.StudyResource {
		live = append(live, "gates_enabled.study_resource")
	}
	if cfg.Gates.Video {
		live = append(live, "gates_enabled.video")
	}
	if cfg.Gates.MockTest {
		live = append(live, "gates_enabled.mock_test")
	}
	if cfg.UnlockEndpointEnabled {
		live = append(live, "unlock_endpoint_enabled")
	}

	if len(live) == 0 {
		return "Nothing is live yet: every gates_enabled class and unlock_endpoint_enabled are off, " +
			"so no student can be charged under these figures. Fix them before switching any of them on."
	}
	return "LIVE RIGHT NOW: " + namePaths(live) + " is switched on, so these figures are already " +
		"reachable by students. Treat this as an incident, not a ticket."
}
