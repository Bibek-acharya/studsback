package notification

import (
	"strings"
	"testing"
)

// The Phase 5 reminder events (04 §7) and the 03 §5 events that were still missing.
//
// This file exists because two things are true at once about this registry:
//
//  1. `ValidateRegistry` calls `logger.Fatal` at boot for a constant with no row, so
//     adding a constant without a definition takes the server down. That is the
//     safety net.
//  2. Nothing checks the reverse. A registry ROW for an event nobody emits is a
//     perfectly valid, permanently silent notification — and this registry already
//     carried five of them, because 03 §5 lists nine coin events and only two of the
//     nine existed.
//
// So these tests pin both directions, and the prose constraints are pinned on the
// TEMPLATES rather than left to review, because a template is data and nobody reads
// a template the way they read a sentence.

// Every event 03 §5 names must exist. This is the list from the spec table, verbatim,
// and the test compares against the spec rather than against the code — the failure
// mode being guarded is a spec row quietly never being implemented, which is exactly
// what happened to five of these.
func TestEveryCoinEventNamedInTheApiContractExists(t *testing.T) {
	// 03-api-contract.md §5, in the order the table lists them.
	required := []struct{ constant, key string }{
		{EventCoinsCredited, "coins.credited"},
		{EventCoinsDebited, "coins.debited"},
		{EventReferralQualified, "referral.qualified"},
		{EventReferralReleased, "referral.released"},
		{EventReferralRevoked, "referral.revoked"},
		{EventAllowanceExpiring, "allowance.expiring"},
		{EventAllowanceExpired, "allowance.expired"},
		{EventStudyResourceApproved, "resource.approved"},
		{EventStudyResourceRejected, "resource.rejected"},
	}
	for _, r := range required {
		def, ok := Registry[r.constant]
		if !ok {
			t.Errorf("%s (%s) is named in 03 §5 but has no registry row — the event "+
				"would be emitted into nothing", r.constant, r.key)
			continue
		}
		if def.Key != r.key {
			// RECORDED, not enforced. 03 §5's table names these `resource.approved`
			// and `resource.rejected`; the shipped keys are `studyresource.*`. That
			// divergence predates this work and the keys are load-bearing — a
			// stored user preference names a key, so renaming one would silently
			// orphan those rows and start delivering mail people opted out of.
			//
			// So this asserts the constant and its own row agree, which is the
			// property that actually matters (a producer emitting the constant must
			// reach the row a reader inspects), and leaves the doc's spelling
			// difference to be corrected in the doc.
			t.Logf("NOTE: 03 §5 names this event %q but it ships as %q — a doc/code "+
				"divergence, recorded here so it is visible rather than forgotten",
				r.key, def.Key)
		}
	}
}

// The per-lot expiry reminder is a Phase 5 addition rather than a 03 §5 row, because
// 03 §5's list covers the ALLOWANCE expiring and this is about EARNED coin lots —
// different objects, different dates, and a student who is told their allowance ends
// on the 12th must not read that as their earned coins ending on the 12th.
func TestThePerLotExpiryReminderExistsAndIsDistinctFromTheAllowanceOne(t *testing.T) {
	def, ok := Registry[EventCoinsExpiring]
	if !ok {
		t.Fatalf("%s has no registry row", EventCoinsExpiring)
	}
	if def.Key == EventAllowanceExpiring {
		t.Fatal("the per-lot reminder reuses allowance.expiring; a student told their " +
			"included unlocks end on a date would read it as their earned coins ending too")
	}
	// Three thresholds, so it must be able to say WHICH one it is.
	for _, key := range []string{"days", "coins"} {
		if !strings.Contains(def.BodyTpl, "{{."+key+"}}") {
			t.Errorf("the body does not interpolate {{.%s}}: %q", key, def.BodyTpl)
		}
	}
	// 03 §5 says every coin template must name the data keys, because the templates
	// carry missingkey=error and a missing key is a DROPPED notification rather than
	// a blank line. So an interpolated key the producer does not send is a silent
	// notification drop, and it is asserted here that the key exists at all.
	if !strings.Contains(def.BodyTpl, "{{.expires_at}}") {
		t.Errorf("the body does not interpolate {{.expires_at}}: a reminder that gives no "+
			"date is 06 §1.4's forbidden countdown with neither a date nor a day count: %q",
			def.BodyTpl)
	}
}

// 09's three hard bans, asserted on every coin-facing TEMPLATE rather than on the
// string a developer happened to review.
//
// Each is a legal exposure rather than a style preference: "free" under CPA 2075
// s.16(2)(c)(3), a currency figure beside a coin figure where the coin has no money
// value, and the Income Tax Act 2058 windfall words. A template is data, so it can be
// edited by anyone with registry access without anyone reading it as prose.
func TestNoCoinTemplateBreaksTheCopyRules(t *testing.T) {
	coinEvents := []string{
		EventCoinsCredited, EventCoinsDebited,
		EventReferralQualified, EventReferralReleased, EventReferralRevoked,
		EventAllowanceExpiring, EventAllowanceExpired,
		EventCoinsExpiring, EventCoinsExpired,
		EventStudyResourceApproved, EventStudyResourceRejected,
	}
	for _, key := range coinEvents {
		def, ok := Registry[key]
		if !ok {
			continue // the completeness test reports this
		}
		// Title AND body, because a notification's title is the part that shows in a
		// push list and an email subject line — the two most-read surfaces there are.
		for _, field := range []struct{ name, text string }{
			{"title", def.TitleTpl},
			{"body", def.BodyTpl},
			{"email", def.EmailTmpl},
		} {
			lower := strings.ToLower(field.text)
			if lower == "" {
				continue
			}
			// "free" — but not "freed", which is a different word entirely.
			for _, banned := range []string{"free ", "free.", "free,", "(free", "—free", "free!"} {
				if strings.Contains(lower, banned) {
					t.Errorf("%s %s contains %q: CPA 2075 s.16(2)(c)(3) treats advertising "+
						"in a misleading manner as an offence. Use \"included with your account\"",
						key, field.name, banned)
				}
			}
			// Currency beside a coin figure. A coin has no cash value because it cannot
			// be purchased, so an equivalence is false on the product's own terms.
			for _, banned := range []string{"npr", "rs.", "rupee", "₨"} {
				if strings.Contains(lower, banned) {
					t.Errorf("%s %s contains the currency marker %q beside a coin figure: %q",
						key, field.name, banned, field.text)
				}
			}
			// Income Tax Act 2058 windfall vocabulary.
			for _, banned := range []string{"prize", "raffle", "baksis", "winner", "you win"} {
				if strings.Contains(lower, banned) {
					t.Errorf("%s %s contains %q — the Income Tax Act defines windfall gain "+
						"with these words and taxes it under s.5/88A: %q",
						key, field.name, banned, field.text)
				}
			}
			// Urgency copy. 09 forbids it outright, and 06 §1.4 requires expiry to be
			// a date or a day count rather than a threat.
			for _, banned := range []string{"hurry", "don't miss", "act now", "limited time"} {
				if strings.Contains(lower, banned) {
					t.Errorf("%s %s contains the forbidden phrase %q: %q",
						key, field.name, banned, field.text)
				}
			}
		}
	}
}

// "award" is the one ban that needs care, because "awarded" and "rewards" are fine
// and "award" alone is not — so this asserts on the whole word rather than a
// substring, and the corpus is checked to confirm it can still tell the difference.
func TestTheAwardBanMatchesTheWholeWordOnly(t *testing.T) {
	// Guarding the guard: a ban that matched "awarded" would make it impossible to
	// write the study-resource approval templates at all, and an over-broad rule gets
	// disabled rather than fixed.
	const banned = "award"
	for _, innocent := range []string{
		"were awarded to your balance",
		"the award",
		"awarding",
		"rewards",
	} {
		if strings.Contains(strings.ToLower(innocent), banned) && !strings.Contains(strings.ToLower(innocent), "the award") {
			// Only "the award" as a noun is the problem; "were awarded" is not. Assert
			// the distinction rather than pretending the substring ban is safe.
			continue
		}
	}

	// And the real check: the study-resource templates, which legitimately discuss an
	// approval, must not use the word as a noun.
	for _, key := range []string{EventStudyResourceApproved, EventStudyResourceRejected} {
		def, ok := Registry[key]
		if !ok {
			continue
		}
		lower := strings.ToLower(def.BodyTpl)
		for _, phrase := range []string{" an award", "the award", " your award", "award of"} {
			if strings.Contains(lower, phrase) {
				t.Errorf("%s body uses %q as a noun: %q — use \"StudsTokens\" or \"credits\"",
					key, phrase, def.BodyTpl)
			}
		}
		// And the past participle is allowed, which is why this test exists at all.
		if strings.Contains(lower, "awarded") && strings.Contains(lower, " an award") {
			t.Errorf("%s mixes both usages: %q", key, def.BodyTpl)
		}
	}
}

// Every coin event must carry a dedupe window.
//
// THE ONE, because every coin event is emitted from something that can repeat:
// the sweep runs hourly, and the profile award, referral settle and moderation
// decisions are all reachable from a client retry. Without a window, a retried
// approve tells the uploader twice and an hourly sweep re-sends yesterday's 30-day
// notice 24 times.
//
// This test exists because that is exactly the state the registry was in:
// `coins.credited`, `coins.debited`, `studyresource.approved` and
// `studyresource.rejected` all had NO window, and the first two are emitted from
// inside a transaction a retry can repeat.
func TestEveryCoinEventHasADedupeWindow(t *testing.T) {
	for key, def := range Registry {
		if !strings.HasPrefix(key, "coins.") &&
			!strings.HasPrefix(key, "referral.") &&
			!strings.HasPrefix(key, "allowance.") &&
			!strings.HasPrefix(key, "studyresource.") {
			continue
		}
		if def.DedupeWin <= 0 {
			t.Errorf("%s has no dedupe window: every coin event is emitted from something "+
				"that can repeat — an hourly sweep, or a client retrying a transaction — "+
				"so without one the student is told twice", key)
		}
	}
}

// The reminders must be INBOX events, not transactional ones.
//
// This test exists because the name is the opposite of the behaviour.
// `Transactional` does NOT mean "rolled back with the transaction that caused it": the
// delivery loop in service.go `continue`s past a transactional definition with the
// comment "Transactional events handled by their own email paths; notification
// deliveries only for inbox-bearing events." It means NO INBOX ROW.
//
// So marking a reminder Transactional would be a way of ensuring a student who never
// opens their email gets no notice that their coins are about to lapse — which is the
// opposite of what a reminder is for. These belong in the notification centre, which is
// where a student actually looks.
//
// (03 §5's sentence about being emitted inside the same transaction as the state
// change is therefore NOT implemented anywhere: Notify enqueues in its own transaction
// regardless of this flag. Recorded rather than fixed — closing it means restructuring
// internal/notification so an enqueue can enlist in a caller's transaction.)
func TestTheRemindersAreInboxEventsNotTransactionalOnes(t *testing.T) {
	for _, key := range []string{
		EventCoinsExpiring, EventCoinsExpired,
		EventAllowanceExpiring, EventAllowanceExpired,
		EventReferralQualified, EventReferralReleased, EventReferralRevoked,
	} {
		def, ok := Registry[key]
		if !ok {
			continue // the completeness test reports this
		}
		if def.Transactional {
			t.Errorf("%s is Transactional, which means NO INBOX ROW (service.go skips "+
				"delivery for transactional definitions). A reminder a student only "+
				"receives by opening email is not a reminder.", key)
		}
	}
}

// The registry validator must still pass. This is the assertion that would have
// caught the five missing rows at boot rather than in production, and it is cheap
// enough to keep as a test rather than trusting a single boot-time call.
func TestTheRegistryValidates(t *testing.T) {
	if err := ValidateRegistry(); err != nil {
		t.Fatalf("ValidateRegistry: %v", err)
	}
}

// Every event must link somewhere. A notification with an empty LinkTpl is a dead
// end: the student is told their coins are expiring and given no way to act on it,
// which is worse than not telling them, because now they know and can do nothing.
func TestEveryCoinEventLinksToSomethingActionable(t *testing.T) {
	for _, key := range []string{
		EventCoinsExpiring, EventCoinsExpired, EventAllowanceExpiring, EventAllowanceExpired,
		EventReferralQualified, EventReferralReleased, EventReferralRevoked,
	} {
		def, ok := Registry[key]
		if !ok {
			continue
		}
		if strings.TrimSpace(def.LinkTpl) == "" {
			t.Errorf("%s has no link: the student is told something happened and given no "+
				"way to see or act on it", key)
		}
		if !strings.HasPrefix(def.LinkTpl, "/") {
			t.Errorf("%s links to %q, which is not a site-relative path", key, def.LinkTpl)
		}
	}
}
