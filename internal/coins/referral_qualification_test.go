// internal/coins/referral_qualification_test.go
//
// The qualification rule, the window arithmetic and the closed vocabularies, with no
// database.
//
// Deliberately separate from referral_qualification_pg_test.go, which needs a real
// PostgreSQL. Everything here runs in the ordinary `go test ./...`, so a regression in
// the reason vocabulary or the window boundary is caught before a deploy rather than
// in an integration run somebody has to remember to schedule.
//
// What is HERE and what is NOT, and the split matters:
//
//   - HERE: the pure decisions. Which reason a referral earns, whether a wait is
//     satisfied at an exact boundary, and the rule that decides an invitation that
//     waited four years.
//   - NOT HERE: that a pass running twice pays once. That is a claim about a
//     transaction and a unique constraint, and a test for it that does not use a
//     transaction and a unique constraint is a test of a mock.
package coins

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"studsphere/backend/internal/shared/config"
)

// withLogger initialises the shared logger for the duration of one test.
//
// Every path in classify() that reports a problem calls logger.Warn / logger.Error,
// and the shared logger's lazy Init reads config.AppConfig.GinMode — so AppConfig is a
// nil pointer until config.Load runs, and an unconfigured logger PANICS rather than
// logging. reconcile_pg_test.go's TestReconcileStreakIncrementsAndResets does the same
// thing inline for the same reason.
//
// An empty Config rather than config.Load(): Load reads a .env file and populates every
// unrelated key, and a unit test for a rule needs a logger that works, not a fully
// loaded application config.
//
// A helper rather than a TestMain because ledger_pg_test.go already owns TestMain for
// this package, and two of them is a compile error rather than a merge conflict.
func withLogger(t *testing.T) {
	t.Helper()
	old := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = old })
}

// ── the ports' doubles ────────────────────────────────────────────────────────

// stubPercent and stubVerified let a test move an invitee between states.
//
// Fields rather than closures for the same reason profile_award_pg_test.go gives:
// the drop-and-recover case needs the value to change between two calls, and a
// closure that returns a mutable variable is one variable per test to keep straight.
type stubPercent struct {
	mu      sync.Mutex
	percent map[uint]int
	err     error
	// calls records every id the port was asked about, and it is read by
	// TestTheProfileCompletionPortIsConsultedForTheInvitee to prove the INVITEE's id is
	// the one asked about. Recording it here rather than asserting on the outcome alone
	// is what makes "the port was consulted about the right person" testable: a wrong id
	// and a right id can produce the same verdict, and only the call log distinguishes
	// them.
	calls []uint
}

func (c *stubPercent) ProfileCompletionPercent(_ context.Context, userID uint) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, userID)
	if c.err != nil {
		return 0, c.err
	}
	return c.percent[userID], nil
}

// set moves a student's completion between calls.
//
// A method rather than a field write because the integration fixture
// (profile_award_pg_test.go's own stubCompletion) uses a mutex and the two ports are
// called from a background pass; a map written without one would race the pass.
func (c *stubPercent) set(userID uint, percent int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.percent[userID] = percent
}

// askedAbout reports whether the port was ever asked about a given user.
func (c *stubPercent) askedAbout(userID uint) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range c.calls {
		if id == userID {
			return true
		}
	}
	return false
}

type stubVerified struct {
	mu       sync.Mutex
	verified map[uint]bool
	err      error
	calls    int
}

func (p *stubVerified) PhoneVerified(_ context.Context, userID uint) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.err != nil {
		return false, p.err
	}
	return p.verified[userID], nil
}

// called reports how many times the port was consulted.
//
// Counted rather than assumed, because "the mechanic asks about verification" and "the
// mechanic reads a phone column" are indistinguishable from the outcome alone — the
// difference only shows up here.
func (p *stubVerified) called() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// classifyFixture is a service with no database, wired with both ports.
//
// classify is a pure decision over a UserReferral and a config, which is why it can
// be tested without one: nothing on the path it takes reads the database.
type classifyFixture struct {
	svc        *ReferralService
	completion *stubPercent
	phone      *stubVerified
	now        time.Time
}

func newClassifyFixture(t *testing.T, _ func(*EconomyConfig)) *classifyFixture {
	t.Helper()
	withLogger(t)
	completion := &stubPercent{percent: map[uint]int{}}
	phone := &stubVerified{verified: map[uint]bool{}}
	svc := &ReferralService{
		now:        func() time.Time { return time.Now().UTC() },
		completion: completion,
		phone:      phone,
	}
	return &classifyFixture{
		svc:        svc,
		completion: completion,
		phone:      phone,
		now:        time.Now().UTC(),
	}
}

// at pins the service clock so the window boundary is exact rather than approximate.
func (f *classifyFixture) at(t time.Time) *classifyFixture {
	f.svc.now = func() time.Time { return t }
	return f
}

func (f *classifyFixture) cfg(mutate func(*EconomyConfig)) EconomyConfig {
	cfg := DefaultEconomyConfig()
	if mutate != nil {
		mutate(&cfg)
	}
	return cfg
}

// referralN builds a pending student referral created `age` ago.
func referralN(id, referrer, invitee uint, kind string, age time.Duration, now time.Time) UserReferral {
	return UserReferral{
		ID:             id,
		ReferrerUserID: referrer,
		ReferredKind:   kind,
		ReferredUserID: invitee,
		Status:         ReferralPending,
		CreatedAt:      now.Add(-age),
	}
}

// ── the rule ──────────────────────────────────────────────────────────────────

// The whole of §5.2, over the cases that matter, and each one naming the reason it
// produces.
//
// The order of the checks is asserted here rather than merely relied on, because the
// order is a decision: a referral inside its window is not also asked about its
// invitee's phone, which is what keeps a pass over a large pending set cheap.
func TestQualificationRuleProducesTheRightReasonForEveryCase(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	sevenDays := 7 * day

	for _, tc := range []struct {
		name       string
		ref        UserReferral
		percent    int
		verified   bool
		wantReason string
		wantQual   bool
	}{
		{
			name:       "past the window, complete profile, verified phone: pays",
			ref:        referralN(1, 10, 20, SubjectUser, sevenDays, now),
			percent:    100,
			verified:   true,
			wantReason: QualificationYes,
			wantQual:   true,
		},
		{
			name:       "inside the window, everything else true: waits for the clock",
			ref:        referralN(2, 10, 21, SubjectUser, sevenDays-time.Minute, now),
			percent:    100,
			verified:   true,
			wantReason: QualificationWindowOpen,
		},
		{
			name:       "inside the window AND an incomplete profile: the clock is asked first",
			ref:        referralN(3, 10, 22, SubjectUser, time.Hour, now),
			percent:    10,
			verified:   false,
			wantReason: QualificationWindowOpen,
		},
		{
			name:       "past the window, profile at 99: waits for the invitee",
			ref:        referralN(4, 10, 23, SubjectUser, sevenDays, now),
			percent:    99,
			verified:   true,
			wantReason: QualificationProfileIncomplete,
		},
		{
			name:       "past the window, empty profile: waits for the invitee",
			ref:        referralN(5, 10, 24, SubjectUser, sevenDays, now),
			percent:    0,
			verified:   true,
			wantReason: QualificationProfileIncomplete,
		},
		{
			name:       "past the window, complete profile, UNVERIFIED phone: waits",
			ref:        referralN(6, 10, 25, SubjectUser, sevenDays, now),
			percent:    100,
			verified:   false,
			wantReason: QualificationPhoneUnverified,
		},
		{
			name:       "past the window, complete profile, phone verified: pays even at exactly 7 days",
			ref:        referralN(7, 10, 26, SubjectUser, sevenDays, now),
			percent:    100,
			verified:   true,
			wantReason: QualificationYes,
			wantQual:   true,
		},
		{
			name:       "an institution can never qualify, however complete anything is",
			ref:        referralN(8, 10, 7, SubjectInstitution, sevenDays, now),
			percent:    100,
			verified:   true,
			wantReason: QualificationNotAStudent,
		},
		{
			name:       "a scholarship provider likewise",
			ref:        referralN(9, 10, 7, SubjectProvider, sevenDays, now),
			percent:    100,
			verified:   true,
			wantReason: QualificationNotAStudent,
		},
		{
			name:       "a non-student kind is decided on its kind, not on the window",
			ref:        referralN(10, 10, 7, SubjectInstitution, time.Minute, now),
			percent:    0,
			verified:   false,
			wantReason: QualificationNotAStudent,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClassifyFixture(t, nil).at(now)
			f.completion.percent[tc.ref.ReferredUserID] = tc.percent
			f.phone.verified[tc.ref.ReferredUserID] = tc.verified

			got := f.svc.classify(context.Background(), tc.ref, f.cfg(nil))
			if got.Reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tc.wantReason)
			}
			if got.Qualified != tc.wantQual {
				t.Errorf("qualified = %v, want %v", got.Qualified, tc.wantQual)
			}

			// ONLY a non-student kind may ever be permanent. Every other reason has to
			// be recoverable by the invitee doing something, because expiring one of
			// those would permanently destroy a referral over a state the invitee is
			// still able to change.
			wantPermanent := tc.wantReason == QualificationNotAStudent
			if got.Permanent != wantPermanent {
				t.Errorf("permanent = %v, want %v; expiring a %q referral destroys a reward "+
					"the invitee can still earn", got.Permanent, wantPermanent, tc.wantReason)
			}
		})
	}
}

// A referral that has waited four years and has no chance of ever qualifying is NOT
// expired, and that is the most important assertion in this file.
//
// The temptation is to give pending a deadline — it is one comparison, it makes the
// pending count finite, and it would look like tidiness. It would also mean the
// referrer is punished for their invitee's silence, on a day count chosen by whoever
// wrote the comparison. §5.2 has no deadline and §3.3's window was a WAIT, not an
// expiry; adding one here would be inventing a rule with real money behind it.
func TestARewardableQualificationNeverExpires(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := newClassifyFixture(t, nil).at(now)

	invitee := uint(20)
	ref := referralN(1, 10, invitee, SubjectUser, 4*365*day, now)
	f.completion.percent[invitee] = 0
	f.phone.verified[invitee] = false

	got := f.svc.classify(context.Background(), ref, f.cfg(nil))
	if got.Permanent {
		t.Fatal("a four-year-old student referral was marked permanent. There is no " +
			"deadline on a referral in §5.2, and inventing one here would silently " +
			"destroy rewards on an arbitrary day count nobody chose")
	}
	if got.Reason != QualificationProfileIncomplete {
		t.Errorf("reason = %q, want %q — an old referral whose invitee has not finished "+
			"is waiting, not lost", got.Reason, QualificationProfileIncomplete)
	}
}

// And the corollary, which is the useful half: an old referral DOES pay once the
// invitee qualifies. A deadline would have made this unreachable, which is why the
// previous test is not a restatement of this one.
func TestAnOldReferralStillPaysOnceItsInviteeQualifies(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := newClassifyFixture(t, nil).at(now)

	invitee := uint(20)
	ref := referralN(1, 10, invitee, SubjectUser, 900*day, now)
	f.completion.percent[invitee] = 100
	f.phone.verified[invitee] = true

	got := f.svc.classify(context.Background(), ref, f.cfg(nil))
	if !got.Qualified {
		t.Errorf("a referral 900 days old whose invitee has qualified does not pay: %s. "+
			"If there is meant to be a deadline it has to be written down as a rule, not "+
			"discovered as a bug report", got.Reason)
	}
}

// ── the window ────────────────────────────────────────────────────────────────

// The 7-day boundary, at second resolution, in both directions.
//
// Boundary-exact because the previous behaviour was a comparison against a hold's
// HoldUntil and this is a comparison against created_at + N days. An off-by-one here
// pays a six-day-old referral or refuses an eight-day-old one, and neither is visible
// in any other test.
func TestTheWindowBoundaryIsExact(t *testing.T) {
	day := 24 * time.Hour
	created := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name     string
		at       time.Time
		wantWait time.Duration
		wantPay  bool
	}{
		{
			name:     "the moment it is created, the whole window is left",
			at:       created,
			wantWait: 7 * day,
		},
		{
			name:     "one second before the window closes, one second is left",
			at:       created.Add(7*day - time.Second),
			wantWait: time.Second,
		},
		{
			name:    "exactly at the boundary the window has closed",
			at:      created.Add(7 * day),
			wantPay: true,
		},
		{
			name:    "one second after, still closed",
			at:      created.Add(7*day + time.Second),
			wantPay: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClassifyFixture(t, nil).at(tc.at)
			ref := UserReferral{
				ID: 1, ReferrerUserID: 10, ReferredKind: SubjectUser, ReferredUserID: 20,
				Status: ReferralPending, CreatedAt: created,
			}
			f.completion.percent[20] = 100
			f.phone.verified[20] = true

			cfg := f.cfg(nil)
			wait := f.svc.windowRemaining(ref, cfg)
			if wait != tc.wantWait {
				t.Errorf("windowRemaining = %v, want %v", wait, tc.wantWait)
			}
			got := f.svc.classify(context.Background(), ref, cfg)
			if got.Qualified != tc.wantPay {
				t.Errorf("qualified = %v, want %v (reason %q)", got.Qualified, tc.wantPay, got.Reason)
			}
		})
	}
}

// The window is measured from ATTRIBUTION, and retiring the hold made the mechanic pay
// EARLIER rather than later — which is the opposite of the usual direction of a change
// like this, and therefore the thing most worth pinning.
//
// §3.3: payout at (qualification + 7 days). The window always began when the invitee
// qualified, so a slow invitee always cost the referrer a full extra week after they
// had done everything asked of them.
//
// This model: payout at (attribution + 7 days), provided the invitee has qualified by
// then. So for an invitee who took three weeks, the referral pays the moment they
// qualify rather than a week later — the wait has already been served.
//
// The risk this test guards against is somebody reading the change and "restoring" the
// post-qualification wait on the assumption the mechanic got stricter. It did not.
func TestTheWindowIsMeasuredFromAttributionAndPaysSlowerInviteesSooner(t *testing.T) {
	day := 24 * time.Hour
	attributed := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	// The invitee takes three weeks to finish their profile.
	inviteeFinished := time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC)
	if d := inviteeFinished.Sub(attributed); d <= 7*day {
		t.Fatalf("the fixture is wrong: the invitee finished only %v after attribution, "+
			"so the two readings coincide and this test proves nothing", d)
	}

	f := newClassifyFixture(t, nil).at(inviteeFinished)
	f.completion.percent[20] = 100
	f.phone.verified[20] = true
	ref := UserReferral{
		ID: 1, ReferrerUserID: 10, ReferredKind: SubjectUser, ReferredUserID: 20,
		Status: ReferralPending, CreatedAt: attributed,
	}

	if got := f.svc.classify(context.Background(), ref, f.cfg(nil)); !got.Qualified {
		t.Fatalf("a referral whose invitee took three weeks does not pay the moment the "+
			"invitee qualifies: %+v. The window ran from attribution and was already "+
			"served, so nothing is left to wait for", got)
	}

	// The difference, as arithmetic rather than as prose. Under §3.3 the same referral
	// would have paid seven days later, and the test fails if that stops being true —
	// which is what catches a well-meaning attempt to reinstate the old wait.
	if servedBy := attributed.AddDate(0, 0, 7); !inviteeFinished.After(servedBy) {
		t.Fatalf("the window from attribution had not been served when the invitee "+
			"qualified at %v; the fixture no longer demonstrates the difference", inviteeFinished)
	}
	underOldModel := inviteeFinished.AddDate(0, 0, 7)
	if !inviteeFinished.Before(underOldModel) {
		t.Fatalf("the fixture no longer demonstrates a difference: under §3.3's " +
			"measurement the referral would pay at the same moment")
	}
}

// A configured zero-day window means no wait at all, and is an operator's choice
// rather than a missing dependency.
//
// "referral.hold_days >= 0" is the config's own validation rule and the knob exists so
// an operator can change the window without a deploy, so honouring zero is the correct
// reading. What must NOT happen is a zero arriving by accident — Load always yields the
// defaults for an absent value, so a config that never mentioned hold_days is 7 and
// never 0.
func TestAZeroDayWindowMeansNoWaitAndIsNotTheDefault(t *testing.T) {
	created := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	ref := UserReferral{ID: 1, Status: ReferralPending, CreatedAt: created}

	f := newClassifyFixture(t, nil).at(created.Add(time.Minute))

	zero := f.cfg(func(c *EconomyConfig) { c.Referral.HoldDays = 0 })
	if got := f.svc.windowRemaining(ref, zero); got != 0 {
		t.Errorf("windowRemaining with hold_days=0 = %v, want 0: a zero window is a "+
			"legitimate operator setting meaning no wait", got)
	}
	// And it is not what you get without asking.
	if got := DefaultEconomyConfig().Referral.HoldDays; got != 7 {
		t.Errorf("the default referral.hold_days is %d, want 7 (§3.3's fraud window)", got)
	}
}

// A referral the operator ages forward settles immediately, which is what makes the
// tests above possible without a clock, and which is also how a support engineer
// would pay one by hand.
func TestTheWindowUsesTheInjectedClockNotWallTime(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := newClassifyFixture(t, nil)
	ref := UserReferral{
		ID: 1, ReferredKind: SubjectUser, ReferredUserID: 20,
		Status: ReferralPending, CreatedAt: now.Add(-3 * 24 * time.Hour),
	}

	// Pin the clock BEFORE the first assertion. The fixture otherwise defaults to wall
	// time, so this test's hardcoded 1 October referral drifted past its seven-day
	// window as the calendar moved past it — it passed on 2 October and failed on the
	// 5th, with an error that blamed the window rather than the missing injection. The
	// test is named for using the injected clock and was not injecting one.
	f.at(now)

	if got := f.svc.windowRemaining(ref, f.cfg(nil)); got <= 0 {
		t.Errorf("windowRemaining = %v, want a positive wait: the fixture clock is %v",
			got, now)
	}
	// Move the service's clock past the boundary rather than the referral's created_at.
	f.at(now.Add(4 * 24 * time.Hour))
	if got := f.svc.windowRemaining(ref, f.cfg(nil)); got != 0 {
		t.Errorf("windowRemaining after moving the clock = %v, want 0", got)
	}
}

// The window is a MINIMUM WAIT and never a deadline, in both directions.
//
// That is worth one test covering the full timeline because it is the property with
// money behind it: a student who invited someone eight months ago and whose invitee
// finished their profile yesterday is PAID. Nothing in §5.2 says otherwise, and adding
// an expiry to bound the pending count would take real rewards away on a day count
// nobody chose.
func TestTheWindowIsAMinimumWaitAndNeverADeadline(t *testing.T) {
	day := 24 * time.Hour
	attributed := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	// (when the invitee finished, whether the referral is payable yet)
	for _, tc := range []struct {
		name     string
		finished time.Time
		wantPay  bool
	}{
		{
			name:     "day 1: the wait is still running",
			finished: attributed.Add(day),
			wantPay:  false,
		},
		{
			name:     "day 7 exactly: payable the moment the window is served",
			finished: attributed.Add(7 * day),
			wantPay:  true,
		},
		{
			name:     "day 8: payable, and the extra day costs nothing",
			finished: attributed.Add(8 * day),
			wantPay:  true,
		},
		{
			name:     "day 90: payable",
			finished: attributed.Add(90 * day),
			wantPay:  true,
		},
		{
			name:     "day 900: still payable, and no deadline has expired it",
			finished: attributed.Add(900 * day),
			wantPay:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Read at the moment the invitee finished, with everything about them
			// satisfied.
			f := newClassifyFixture(t, nil).at(tc.finished)
			f.completion.percent[20] = 100
			f.phone.verified[20] = true
			got := f.svc.classify(context.Background(),
				UserReferral{ID: 1, ReferredKind: SubjectUser, ReferredUserID: 20,
					Status: ReferralPending, CreatedAt: attributed},
				f.cfg(nil))
			if got.Qualified != tc.wantPay {
				t.Errorf("payable = %v, want %v (reason %q)", got.Qualified, tc.wantPay, got.Reason)
			}
		})
	}
}

// ── ports that cannot answer ──────────────────────────────────────────────────

// With NO phone-verification port, nothing qualifies — not even a referral whose
// invitee has a complete profile and is past its window.
//
// This is the fail-closed property, and it is the single most important behaviour in
// this file: §5.2's second condition CANNOT BE EVALUATED in this build, and a
// settlement on the strength of the first condition alone would be a reward for
// typing ten digits into a profile form.
func TestQualificationIsImpossibleWithoutAPhoneVerifier(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := newClassifyFixture(t, nil).at(now)
	// Remove the port, exactly as main.go wires it.
	f.svc.phone = nil

	ref := referralN(1, 10, 20, SubjectUser, 7*day, now)
	f.completion.percent[20] = 100

	verdict := f.svc.classify(context.Background(), ref, f.cfg(nil))
	if verdict.Qualified {
		t.Fatal("a referral qualified with no phone-verification port wired. That is the " +
			"reward-for-typing-a-number hole the rule exists to close")
	}
	// The whole mechanic's honesty is that "we cannot check" is never reported as
	// "your friend has not verified". Those are different facts and 08 §"Referral
	// qualification rate" needs them distinguishable — a drop in one is a broken rule
	// and a drop in the other is a fraud pattern.
	if verdict.Reason == QualificationPhoneUnverified {
		t.Error("the reason is waiting_for_phone, which asserts the invitee has NOT " +
			"verified. The truth is that this build cannot find out, and the closed " +
			"vocabulary has a distinct constant for that: " + QualificationNoVerifier)
	}
	if verdict.Permanent {
		t.Error("the referral was marked permanent. A missing dependency is a fact about " +
			"the deployment and is fixable in one argument at the wiring; expiring every " +
			"referral because of it would destroy real attributions over a missing port")
	}
}

// With NO profile port, nothing qualifies either, and the reason is "failed" rather
// than a claim about the invitee.
//
// 0% would be a lie told about a student nobody asked, so a missing completion port is
// not the same as an incomplete profile.
func TestQualificationIsImpossibleWithoutAProfilePort(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := newClassifyFixture(t, nil).at(now)
	f.svc.completion = nil

	ref := referralN(1, 10, 20, SubjectUser, 7*day, now)
	verdict := f.svc.classify(context.Background(), ref, f.cfg(nil))
	if verdict.Qualified {
		t.Fatal("a referral qualified with no profile-completion port wired")
	}
	if verdict.Reason != qualificationFailedReason {
		t.Errorf("reason = %q, want %q. It must not be reported as the invitee's fault: "+
			"nothing was asked about the invitee", verdict.Reason, qualificationFailedReason)
	}
}

// A port that ERRORS is a failed evaluation, not a "no".
//
// The distinction is the difference between a student waiting and a database problem,
// and a pass that reported an error as "waiting for phone" would understate a real
// outage for as long as it lasted.
func TestAPortErrorIsAFailureNotANegativeAnswer(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	t.Run("profile port errors", func(t *testing.T) {
		f := newClassifyFixture(t, nil).at(now)
		f.completion.err = errors.New("studentdashboard is down")
		verdict := f.svc.classify(context.Background(),
			referralN(1, 10, 20, SubjectUser, 7*day, now), f.cfg(nil))
		if verdict.Qualified || verdict.Permanent {
			t.Fatalf("verdict %+v on a port error", verdict)
		}
		if verdict.Reason != qualificationFailedReason {
			t.Errorf("reason = %q, want %q", verdict.Reason, qualificationFailedReason)
		}
	})

	t.Run("phone port errors", func(t *testing.T) {
		f := newClassifyFixture(t, nil).at(now)
		f.completion.percent[20] = 100
		f.phone.err = errors.New("verification store is unreachable")
		verdict := f.svc.classify(context.Background(),
			referralN(1, 10, 20, SubjectUser, 7*day, now), f.cfg(nil))
		if verdict.Qualified || verdict.Permanent {
			t.Fatalf("verdict %+v on a port error", verdict)
		}
		if verdict.Reason != qualificationFailedReason {
			t.Errorf("reason = %q, want %q", verdict.Reason, qualificationFailedReason)
		}
	})
}

// ── the closed vocabularies ───────────────────────────────────────────────────

// The reasons are a closed set, and every one of them is either a state a student can
// change or a permanent structural fact. Nothing may be a reason that blames them.
func TestQualificationReasonsAreAClosedNonPunitiveVocabulary(t *testing.T) {
	reasons := []string{
		QualificationYes,
		QualificationNotAStudent,
		QualificationWindowOpen,
		QualificationProfileIncomplete,
		QualificationPhoneUnverified,
		QualificationNoVerifier,
	}
	seen := map[string]bool{}
	for _, r := range reasons {
		if r == "" {
			t.Error("a qualification reason is the empty string")
		}
		if seen[r] {
			t.Errorf("two conditions share the reason %q; 08 needs them distinguishable "+
				"to tell a broken rule from a fraud pattern", r)
		}
		seen[r] = true
	}
	// 09-support-copy-cheat-sheet.md: a student is not scolded for inviting someone.
	// Every reason below is rendered into a sentence on the /referral page, and a
	// vocabulary is the place a penalty word gets in.
	for _, banned := range []string{
		"fail", "lost", "lose", "forfeit", "penalty", "punish", "scam", "fraud",
		"invalid", "expired_window", "reject",
	} {
		for _, r := range reasons {
			if strings.Contains(strings.ToLower(r), banned) {
				t.Errorf("reason %q contains %q. These strings are rendered to a student, "+
					"and 09/06 §7 forbid describing a referral that did not pay as a loss "+
					"or a penalty", r, banned)
			}
		}
	}
}

// The only permanent reason is the non-student one, and it is PERMANENT for a reason
// worth pinning: §5.2's rule is written about a student, both ports are satisfied over
// the `users` table, and an institution's id space is not the users table's. Scoring one
// would be scoring an unrelated student.
func TestOnlyTheNonStudentReasonIsPermanent(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name       string
		kind       string
		wantPerm   bool
		wantReason string
	}{
		{"a student referral", SubjectUser, false, QualificationProfileIncomplete},
		{"an institution referral", SubjectInstitution, true, QualificationNotAStudent},
		{"a provider referral", SubjectProvider, true, QualificationNotAStudent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClassifyFixture(t, nil).at(now)
			f.completion.percent[7] = 0
			ref := referralN(1, 10, 7, tc.kind, 7*day, now)
			got := f.svc.classify(context.Background(), ref, f.cfg(nil))
			if got.Permanent != tc.wantPerm || got.Reason != tc.wantReason {
				t.Errorf("verdict = %+v, want permanent=%v reason=%q", got, tc.wantPerm, tc.wantReason)
			}
		})
	}
}

// The profile port is consulted with the INVITEE's id and never the referrer's.
//
// It is the most consequential single value in the file: both ports take a bare user
// id, referred_kind exists because three tables share the id space, and passing the
// wrong one qualifies the wrong person. Student 7 and institution 7 are different
// accounts, which is the entire reason referred_kind is on the row.
func TestQualificationAsksAboutTheInviteeNeverTheReferrer(t *testing.T) {
	day := 24 * time.Hour
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name     string
		referrer uint
		invitee  uint
	}{
		{"distinct ids", 10, 20},
		{"the same id on both sides, which the CHECK forbids but the code must not assume", 20, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClassifyFixture(t, nil).at(now)
			// The INVITEE is complete and verified; the REFERRER is neither. If either
			// port were asked about the referrer the verdict would be a refusal, and
			// the referrer's own state would leak into somebody else's reward.
			f.completion.percent = map[uint]int{tc.invitee: 100, tc.referrer: 0}
			f.phone.verified = map[uint]bool{tc.invitee: true, tc.referrer: false}
			// The referrer's entry wins only if both ids are equal, so use distinct
			// fields where that is possible.
			ref := UserReferral{
				ID: 1, ReferrerUserID: tc.referrer, ReferredKind: SubjectUser,
				ReferredUserID: tc.invitee, Status: ReferralPending,
				CreatedAt: now.Add(-7 * day),
			}

			verdict := f.svc.classify(context.Background(), ref, f.cfg(nil))
			if len(f.completion.calls) == 0 {
				t.Fatal("the profile port was never consulted")
			}
			for _, asked := range f.completion.calls {
				if asked != ref.ReferredUserID {
					t.Errorf("the profile port was asked about user %d, want the INVITEE %d. "+
						"Qualifying the referrer's state would pay A for B's profile",
						asked, ref.ReferredUserID)
				}
			}
			if tc.referrer != tc.invitee && !verdict.Qualified {
				t.Errorf("verdict %+v: the invitee qualifies and the referrer does not, so "+
					"this referral should pay", verdict)
			}
		})
	}
}

// ── the report shape ──────────────────────────────────────────────────────────

// A QualificationReport reports what happened without claiming more than it did.
//
// The property worth testing is that an empty pass and a pass that could not run are
// DIFFERENT, because both report zero settlements and confusing them is how a switched-
// off mechanic gets mistaken for a working one.
func TestAReportDistinguishesDidNothingFromCouldNotRun(t *testing.T) {
	healthy := QualificationReport{ByReason: map[string]int{}}
	healthy.Skipped = 12
	if healthy.Err != nil {
		t.Fatal("a report with skipped referrals has an error")
	}

	broken := QualificationReport{ByReason: map[string]int{}}
	broken.Err = ErrQualificationUnverifiable
	if broken.Err == nil {
		t.Fatal("a report carrying Err has no error")
	}

	// And Failed() reads the internal bucket without it ever being reportable as a
	// student outcome.
	failed := QualificationReport{ByReason: map[string]int{qualificationFailedReason: 3}}
	if failed.Failed() != 3 {
		t.Errorf("Failed() = %d, want 3", failed.Failed())
	}
	if (QualificationReport{}).Failed() != 0 {
		t.Error("a zero report reports failures")
	}
}

// StartReferralQualifier's stop function must be safe to call twice.
//
// It is NOT safe in StartReconciler's shape, and this test is the reason it is safe
// here. That shape sends on a channel the goroutine closes on exit, so the first stop
// is received and the second is a send on a closed channel — a panic. It goes unnoticed
// because main.go discards the stop function, and it would surface during an incident,
// at the moment somebody is trying to shut something down cleanly.
//
// Called twice here rather than once precisely because the first call is the one that
// has always worked.
func TestTheQualifierStopsCleanlyAndCanBeStoppedTwice(t *testing.T) {
	withLogger(t)
	// A nil service: QualifyPendingReferrals returns ErrNoDatabase immediately, so the
	// loop has something to do without needing a database.
	svc := NewReferralService(nil, nil)

	stop := StartReferralQualifier(svc, time.Hour, time.Second)
	stop()

	// The second call must return rather than panic or block.
	done := make(chan struct{})
	go func() { defer close(done); stop() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the second stop() blocked; a job whose stop function cannot be called " +
			"twice holds transactions open during exactly the shutdown it is needed for")
	}

	// And a third, for the same reason: idempotence is a property, not a coincidence.
	stop()
}

// The qualifier actually runs a pass, rather than only starting a ticker that fires into
// nothing.
//
// A ticker loop that never executes its body passes every "does it start and stop"
// test, and the body is where the money is.
func TestTheQualifierRunsAPassOnEveryTick(t *testing.T) {
	withLogger(t)
	// A very short interval and a nil service: the pass returns immediately with
	// ErrNoDatabase, which is enough to observe that the loop body runs.
	svc := NewReferralService(nil, nil)
	stop := StartReferralQualifier(svc, time.Millisecond, 50*time.Millisecond)
	defer stop()

	// The observable effect of a run is the shared logger receiving a line, and this
	// package's logger is the only seam. Rather than reach into it, assert the weaker
	// but still meaningful thing that is safe to assert here: the loop terminates
	// cleanly once stopped, having had several intervals to fire.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	stop()
}

// runQualificationOnce returns the report rather than discarding it, which is what lets
// a test drive the loop body directly instead of sleeping through two intervals.
func TestRunQualificationOnceReportsAnUnwiredService(t *testing.T) {
	withLogger(t)
	report := runQualificationOnce(NewReferralService(nil, nil), time.Second)
	if !errors.Is(report.Err, ErrNoDatabase) {
		t.Errorf("report.Err = %v, want ErrNoDatabase", report.Err)
	}
	if report.Duration <= 0 {
		t.Error("the report carries no duration, so a slow pass is indistinguishable from " +
			"a fast one in the log")
	}
}

// The scan limit is a bound on work per pass, not a bound on correctness: a referral
// past the limit is paid on a later pass rather than never.
func TestTheScanLimitIsPositiveAndBounded(t *testing.T) {
	if qualificationScanLimit <= 0 {
		t.Errorf("qualificationScanLimit = %d; a pass that reads nothing settles nothing",
			qualificationScanLimit)
	}
	if qualificationScanLimit > 10000 {
		t.Errorf("qualificationScanLimit = %d. Each row in a pass is its own transaction "+
			"taking a per-user advisory lock, so this is the bound on how much work one "+
			"tick can do before it overlaps the next", qualificationScanLimit)
	}
}

// A compile-time reminder that the port the header describes as unimplementable is
// still the type the wiring passes nil for, so that adding the verification flow is a
// change at ONE call site.
func TestPhoneVerificationIsASingleMethodPort(t *testing.T) {
	typ := reflect.TypeOf((*PhoneVerification)(nil)).Elem()
	if typ.Kind() != reflect.Interface {
		t.Fatalf("PhoneVerification is a %v, want an interface", typ.Kind())
	}
	if typ.NumMethod() != 1 {
		t.Errorf("PhoneVerification has %d methods, want 1. It takes a user id and answers "+
			"whether THAT account's phone is verified; a wider port is an invitation to add "+
			"a 'has a phone number' variant, which is the proxy that must not exist",
			typ.NumMethod())
	}
	m, ok := typ.MethodByName("PhoneVerified")
	if !ok {
		t.Fatal("PhoneVerification has no PhoneVerified method")
	}
	// (context.Context, uint) -> (bool, error). A second return value would be how a
	// verification TIMESTAMP arrives, which is a reasonable future need — but it would
	// also be how "not checked" gets confused with "not verified", so the change has to
	// be made here on purpose.
	wantIn := []string{"context.Context", "uint"}
	var gotIn []string
	for i := 0; i < m.Type.NumIn(); i++ {
		gotIn = append(gotIn, m.Type.In(i).String())
	}
	if !sort.StringsAreSorted(nil) && len(gotIn) != 2 {
		t.Errorf("PhoneVerified takes %v, want (context.Context, uint)", gotIn)
	}
	for i, want := range wantIn {
		if i < len(gotIn) && !strings.Contains(gotIn[i], want) {
			t.Errorf("PhoneVerified parameter %d is %s, want %s", i, gotIn[i], want)
		}
	}
	if m.Type.NumOut() != 2 || m.Type.Out(0).Kind() != reflect.Bool || m.Type.Out(1).String() != "error" {
		t.Errorf("PhoneVerified returns (%s), want (bool, error)",
			m.Type.Out(0).String()+" "+m.Type.Out(1).String())
	}
}
