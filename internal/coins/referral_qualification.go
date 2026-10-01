// internal/coins/referral_qualification.go
//
// The 7-day wait, the qualification rule, and the job that advances a referral.
//
// ── the rule, and the one part of it that cannot be implemented ───────────────
//
// 04-implementation-plan.md §5.2 step 6: "Qualification: invitee completes
// profile **and** verifies phone". Two conditions, either of which failing means
// the referral does not pay.
//
// PROFILE is implementable, and is read through the SAME port the profile award
// uses — coins.ProfileCompletion, satisfied by studentdashboard's adapter over the
// twelve checks. Reimplementing the twelve checks here would be a second source of
// truth: a student could be qualified against one definition of "complete" while
// the page they are looking at shows them another, and the drift would be silent
// in both directions.
//
// PHONE VERIFICATION DOES NOT EXIST IN THIS SCHEMA. That is the finding this file
// exists to make unmissable:
//
//   - auth.User has `Phone string` (internal/auth/model.go:28) with no
//     verification state of any kind.
//   - docs/coin-system/01-current-state-and-feasibility.md §3.4: "Grepping for
//     is_phone_verified, is_email_verified, phone_verified, and email_verified
//     across *.go and *.sql returns zero hits. … Phone is collected via
//     UpdateProfileRequest.Phone and **never verified by anything**. This is a
//     genuine blocker for fraud-sensitive coin awards. We cannot gate a referral
//     reward on 'the invitee verified their phone', because the system does not
//     know whether they did."
//   - 04-implementation-plan.md §2.3 makes adding `phone_verified_at` "a launch
//     blocker for the referral earn, not a nice-to-have".
//
// So PhoneVerification below is a port with NO IMPLEMENTATION anywhere in this
// codebase, and cmd/server/main.go wires nil for it.
//
// WHAT IS DELIBERATELY NOT DONE: substituting "user.Phone is non-empty" for
// verification. That is not a weaker version of the rule, it is a different and much
// worse one — anyone can type ten digits into a profile form and qualify, which
// makes the fraud control decorative while looking like it is on. The rule fails
// CLOSED: with no verifier, QualifyPendingReferrals pays nothing, says so in its
// report, and returns ErrQualificationUnverifiable's information as a counted
// reason rather than a silent pass. That is why a referral is credited in no
// deployment today, and it is the correct outcome of an unimplementable rule.
//
// ── the window ────────────────────────────────────────────────────────────────
//
// referral.hold_days (7) is a MINIMUM WAIT measured from the referral's
// created_at. It is not a deadline: nothing expires for being slow. See
// referral.go's windowRemaining for why moving the window from "after
// qualification" to "from attribution" is a real behaviour change.
//
// ── what expires, and what does not ───────────────────────────────────────────
//
// §3.3's "release or clawback" described releasing or reversing a HOLD. With no
// hold there is nothing to release and nothing to claw back: a referral that fails
// the fraud window simply never paid. That removes the whole shape of the question,
// so it is worth being explicit about what is left.
//
//   - Nothing is paid and nothing is taken back automatically, ever. The
//     qualification pass PAYS. It has no path that reverses, deducts or debits —
//     expireReferral writes a status column and a reason and touches no balance,
//     which is why it is safe for a job to run unattended.
//
//     What replaced "clawback" in the state machine is EXPIRED, and it means
//     something narrower than clawback did: a referral that can never qualify, so
//     nothing will ever pay it. It is the terminal non-paid state the student's own
//     page reports, and it exists because "pending" is otherwise a permanent lie
//     about referrals that were never going to pay. The string a student reads off
//     it is "this referral did not qualify", never "you lost" — 06-ui-ux-spec.md §7
//     is explicit that a pending referral is never described as a loss, and the
//     expired_reason is drawn from a closed vocabulary that contains no penalty
//     wording at all.
//
//     NOT a timeout, and that is deliberate. A student referral whose invitee simply
//     has not finished their profile stays pending however long that takes, because
//     any deadline would be an arbitrary number of days chosen by whoever wrote this,
//     and the referrer is not the party who is slow. TestARewardableQualification
//     NeverExpires and TestPendingReferralsCarryNoDeadline pin that, so the decision
//     has to be undone deliberately rather than re-derived.
//
//   - A PAID referral can still be reversed by an operator, inside
//     EconomyConfig.ClawbackWindowDays (180), through Ledger.Reverse with
//     ReasonGrantReversal, which is what writes ReferralClawedBack. So
//     clawback-after-payment IS reachable — by hand, with a recorded reason, which is
//     what "a clawback without a recorded reason is the thing a ledger exists to make
//     impossible" (referral_model.go) requires. No automated clawback driver exists,
//     and deliberately so: a job that silently removes a student's coins is a
//     different risk class from one that adds them, and 05 §3.3's 180-day window is a
//     fraud-investigation horizon rather than a timer.
//
// ── why this is not the reconciler ────────────────────────────────────────────
//
// internal/coins/reconcile.go runs five ledger invariants on a ticker. It is a
// READ-ONLY checker: five aggregate queries, a report, a log line, and a promise it
// never breaks ("It never calls logger.Fatal. A data-integrity problem must not
// take the server down, and killing the process would destroy the evidence").
//
// A pass that GRANTS COINS is a different kind of job, in three ways that are each
// enough on their own:
//
//  1. Blast radius. A failed invariant is one wrong row. A failed qualification pass
//     is N wrong balances, and the failure mode of "the wallet timed out mid-loop"
//     is a partially-paid batch rather than an alert.
//  2. Blast radius of the FIX. The reconciler's answer to a problem is to be
//     re-run. This job's answer is to be re-run, and each re-run moves money. A
//     loop that is restarted after a deploy lands in the same table with the same
//     pending rows, so "just run it again" is not a safe instinct for either, but
//     only one of them is the instinct an operator will reach for.
//  3. Cadence and cost. The reconciler is hourly and its cost is five aggregates
//     against a small table. This is per-pending-row with two cross-module port
//     calls each, and it wants to run far more often than hourly, because a referral
//     whose invitee finished their profile at lunchtime should not wait until
//     tonight to pay. One loop would mean one interval chosen to suit whichever
//     mattered more, and it would be the wrong one for the other.
//
// So it is a SEPARATE ticker with its own interval and timeout —
// StartReferralQualifier below — and StartReconciler is untouched. Its name says
// what it is; a `runOnce` that both checks invariants and pays referrals would be
// a function whose name lies about half its behaviour.
//
// ── idempotency, which matters more here than anywhere else ──────────────────
//
// This runs on a timer against every pending referral, so "runs twice" is not a
// hypothetical, it is what happens on every deploy and every process restart. Two
// independent mechanisms make a second pass pay nothing:
//
//  1. the pass only considers status = 'pending', and SettleReferral writes
//     'qualified' in the SAME transaction as the money — so a second pass does not
//     select the row at all;
//  2. if it somehow does, reward_grant's UNIQUE (user_id, award_code) with
//     ReferralAwardCode(referralID) returns no row, and the transaction rolls back.
//
// Mechanism 1 is what makes the common case free. Mechanism 2 is what makes the
// invariant hold even when 1 is bypassed, which is why the test drives both: it runs
// the pass twice and asserts one payout, and then separately forces a second
// settlement of an already-qualified referral and asserts the balance did not move.
// TestRunningTheQualificationPassTwicePaysOnce is in referral_qualification_pg_test.go.
package coins

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"studsphere/backend/internal/shared/logger"
)

// PhoneVerification is the port §5.2's second condition needs and this schema
// cannot answer.
//
// It is a PORT, declared here and to be satisfied by an adapter in cmd/server over
// whatever module ends up owning phone verification — for the same reason
// ProfileCompletion is a port: internal/coins may not import internal/auth, and the
// verification state will live wherever the OTP-to-phone flow is written.
//
// The method takes the referrer's INVITEE's user id and answers whether THAT
// ACCOUNT's phone is verified. There is deliberately no variant that takes a phone
// number, because the interesting question is not "is this a number" (which any
// form answers) and not "is this number verified anywhere" (which is a fraud-table
// question, not an account-state one).
//
// NO IMPLEMENTATION EXISTS. See the file header.
type PhoneVerification interface {
	PhoneVerified(ctx context.Context, userID uint) (bool, error)
}

// The qualification outcomes, as closed vocabulary rather than free text.
//
// They are a closed set because each one is a different support sentence and a
// different metric bucket, and 08 §"Referral qualification rate" needs
// "Rejected and never-qualified invites are tracked separately, because a drop in
// one means a fraud pattern and a drop in the other means a broken rule". A string
// built at a call site would make that impossible to aggregate.
const (
	// QualificationYes means both conditions of §5.2 hold and the referral may be
	// paid once its window has closed.
	QualificationYes = "qualified"
	// QualificationNotAStudent means the referred account is not a student account.
	// PERMANENT — see classify.
	QualificationNotAStudent = "invitee_not_a_student"
	// QualificationWindowOpen means the referral.hold_days wait has not elapsed.
	// The normal state of most pending rows.
	QualificationWindowOpen = "waiting_for_window"
	// QualificationProfileIncomplete means the invitee's profile completion is below
	// 100%. Recoverable: the student can finish it, so the referral stays pending.
	QualificationProfileIncomplete = "waiting_for_profile"
	// QualificationPhoneUnverified means the invitee's phone is not verified.
	// Recoverable in principle once the verification flow exists.
	QualificationPhoneUnverified = "waiting_for_phone"
	// QualificationNoVerifier means this build has no PhoneVerification port, so
	// the phone condition CANNOT BE EVALUATED. Not a student outcome at all — it is
	// a fact about the deployment, and it is counted separately so that "every
	// referral is stuck at QualificationPhoneUnverified" and "every referral is
	// stuck because we cannot check" cannot be mistaken for each other.
	QualificationNoVerifier = "no_phone_verifier"
)

// Qualification is one referral's verdict, and the two fields are read together:
// Reason is what to tell support, Permanent says whether this referral can ever
// change its mind.
type Qualification struct {
	// Qualified is true only when both of §5.2's conditions hold.
	Qualified bool
	// Reason is one of the Qualification* constants.
	Reason string
	// Permanent is true when the referral can NEVER qualify whatever else happens.
	// It is what separates "your friend has not finished their profile" from
	// "this can never pay", and it is the ONLY thing that authorises writing
	// ReferralExpired.
	Permanent bool
}

// QualificationReport is what one pass did.
//
// A report rather than a bare error, because the interesting outcomes are not
// errors: a pass that finds nothing to do has SUCCEEDED, and a pass that cannot
// verify anything has also not failed — it has declined, loudly. Collapsing both
// into nil would make "the mechanic is working" and "the mechanic is switched off"
// indistinguishable in the log, which is the exact confusion this pass must not
// create.
type QualificationReport struct {
	// StartedAt and Duration bracket the pass.
	StartedAt time.Time
	Duration  time.Duration
	// Scanned is how many pending referrals the pass looked at.
	Scanned int
	// Settled is how many it PAID. The number that matters.
	Settled int
	// Expired is how many it moved to a terminal never-pay state.
	Expired int
	// Skipped is how many were still legitimately pending — inside the window, or
	// waiting on the invitee.
	Skipped int
	// ByReason is the full breakdown, so a support question about one student can be
	// answered from the log line rather than by re-running a query.
	ByReason map[string]int
	// Err is non-nil only for a pass that could not run at all. A single referral's
	// failure does NOT set it: one unreachable invitee must not stop the other 999,
	// and it is counted in Failed instead.
	Err error
}

// Failed counts the referrals whose evaluation itself failed — the completion port
// errored, the settlement hit the cap, the row vanished under us. Distinct from
// Err because a pass with failures is still a pass that ran.
func (r QualificationReport) Failed() int {
	if r.ByReason == nil {
		return 0
	}
	return r.ByReason[qualificationFailedReason]
}

// qualificationFailedReason is the internal bucket for "this referral's evaluation
// broke", kept out of the closed Qualification* vocabulary because it is not a
// student outcome and must never be reported as one.
const qualificationFailedReason = "failed"

// qualificationScanLimit bounds one pass.
//
// A bound, not a batch loop: the pass settles each referral in its own transaction
// and each one takes the referrer's advisory lock, so an unbounded pass holds N
// transactions' worth of work in one interval and would overlap the next tick if it
// ran long. Bounded, one pass per tick, and a row that is not reached this tick is
// reached next tick — which costs nothing because a pending referral pays whenever
// it is looked at, not whenever a particular pass ran.
//
// 500 is deliberately larger than the cap (10 settlements per referrer per month,
// and a month has 43,200 seven-day windows' worth of referral ages). It is not a
// throughput promise; it is the point at which the log line becomes visibly
// truncated rather than silently partial.
const qualificationScanLimit = 500

// QualifyPendingReferrals advances every pending referral as far as it can go, and
// pays the ones that can be paid.
//
// It is the ONLY caller of SettleReferral, and it is safe to run any number of
// times. See the file header for the two mechanisms that make a second run pay
// nothing.
//
// The error it returns is for a pass that COULD NOT RUN — no config, no database.
// A referral that cannot be settled is counted, not returned: one capped referrer
// must not prevent the rest of the cohort from being paid, because the cap is
// per-referrer and the next referral belongs to somebody else.
func (s *ReferralService) QualifyPendingReferrals(ctx context.Context) (QualificationReport, error) {
	started := time.Now()
	report := QualificationReport{
		StartedAt: started.UTC(),
		ByReason:  map[string]int{},
	}
	finish := func() (QualificationReport, error) {
		report.Duration = time.Since(started)
		return report, report.Err
	}

	if s == nil || s.repo == nil || s.ledger == nil {
		report.Err = ErrNoDatabase
		return finish()
	}
	cfg, err := s.ledger.config.Load()
	if err != nil {
		report.Err = err
		return finish()
	}
	if cfg.Awards.ReferralReferrer <= 0 {
		report.Err = fmt.Errorf("%w: awards.referral_referrer is %d, so no referral can pay",
			ErrInvalidConfig, cfg.Awards.ReferralReferrer)
		return finish()
	}

	// THE FAIL-CLOSED CHECK, before any row is touched.
	//
	// With no PhoneVerification port, §5.2's second condition cannot be evaluated
	// and NO referral may be paid — not even one whose profile is complete, and not
	// even one past its window. Counting them and returning is the whole behaviour,
	// and it happens before the SELECT so that a deployment missing the
	// verification flow does not do per-row work whose answer it must then discard.
	if s.phone == nil {
		if report.Scanned, err = s.countPending(); err != nil {
			report.Err = err
			return finish()
		}
		report.ByReason[QualificationNoVerifier] = report.Scanned
		report.Err = fmt.Errorf("%w: %d pending referrals cannot be qualified without a phone "+
			"verification check; see referral_qualification.go", ErrQualificationUnverifiable, report.Scanned)
		return finish()
	}

	pending, err := s.pendingForQualification(qualificationScanLimit)
	if err != nil {
		report.Err = err
		return finish()
	}
	report.Scanned = len(pending)

	for _, ref := range pending {
		verdict := s.classify(ctx, ref, cfg)
		report.ByReason[verdict.Reason]++

		if verdict.Permanent {
			// The one write this pass makes that is not a payment.
			if err := s.expireReferral(ctx, ref, verdict.Reason); err != nil {
				report.ByReason[verdict.Reason]--
				report.ByReason[qualificationFailedReason]++
				logger.Error("StudsToken referral could not be expired",
					"referral_id", ref.ID, "reason", verdict.Reason, "error", err)
				continue
			}
			report.Expired++
			continue
		}
		if !verdict.Qualified {
			// Inside the window, or waiting on the invitee. The overwhelming
			// majority of pending rows on any given pass are here.
			report.Skipped++
			continue
		}

		if _, err := s.SettleReferral(ctx, ref.ID); err != nil {
			switch {
			case errors.Is(err, ErrReferralAlreadyPaid), errors.Is(err, ErrReferralNotYetPayable):
				// Raced with another instance's pass, or the row moved between the
				// scan and the settlement. Not a failure: the other pass paid it, or
				// the clock has not actually reached eligible_at yet.
				report.ByReason[verdict.Reason]--
				report.ByReason[QualificationWindowOpen]++
				report.Skipped++
			case errors.Is(err, ErrCapReached):
				// A cap, which is a policy outcome. The referral stays PENDING and
				// keeps its place in the queue, so it is paid in a later month rather
				// than lost — which is what "the cap is a count of successful
				// referrals" requires.
				report.ByReason[verdict.Reason]--
				report.ByReason[qualificationFailedReason]++
				report.Skipped++
				logger.Info("StudsToken referral skipped: monthly cap reached",
					"referral_id", ref.ID, "referrer_user_id", ref.ReferrerUserID)
			default:
				report.ByReason[verdict.Reason]--
				report.ByReason[qualificationFailedReason]++
				logger.Error("StudsToken referral settlement failed",
					"referral_id", ref.ID, "referrer_user_id", ref.ReferrerUserID, "error", err)
			}
			continue
		}
		report.Settled++
	}

	return finish()
}

// classify is the whole of §5.2, in the order the checks are cheap and the reasons
// are informative.
//
// The order is: permanent first, then the window, then profile, then phone — and it
// matters more than it looks. Checking the window first means the common case
// (every pending referral, on every pass) costs one comparison and no cross-module
// read at all. Checking permanent first means a non-student referral is retired on
// its first sighting rather than occupying a slot in every pass forever.
func (s *ReferralService) classify(ctx context.Context, ref UserReferral, cfg EconomyConfig) Qualification {
	if ref.ReferredKind != SubjectUser {
		// PERMANENT, and this is the finding rather than a shortcut.
		//
		// §5.2's rule — "invitee completes profile AND verifies phone" — is defined
		// only for a STUDENT. Both ports take a bare user id, and both are satisfied
		// over the `users` table by this codebase's wiring. So a referral whose
		// referred_kind is 'institution' would be evaluated against STUDENT #7's
		// profile and STUDENT #7's phone, because institution id 7 is not student id
		// 7 — that is the entire reason referred_kind exists (referral_model.go).
		//
		// So scoring it would not be a lenient qualification, it would be a
		// qualification of the WRONG PERSON, and it would pay a student for inviting
		// a college on the strength of an unrelated student's profile.
		//
		// An institution or provider can never satisfy a rule about completing a
		// student profile, so the referral can never qualify. That is a permanent
		// fact, not a pending one, which is what ReferralExpired is for: the
		// relationship is kept for audit, and the payout is closed with a recorded
		// reason rather than left pending forever.
		//
		// This is a genuine gap in the design rather than a bug in the code, and it is
		// worth stating plainly: §5.2 does not say what an institution referral is
		// worth, and the attribution table admits three kinds. Deciding that is a
		// product call. What the code must not do is guess.
		return Qualification{Reason: QualificationNotAStudent, Permanent: true}
	}
	if s.windowRemaining(ref, cfg) > 0 {
		// Not a judgement about the invitee at all — the fraud window is still open.
		return Qualification{Reason: QualificationWindowOpen}
	}

	// The completion port. nil is refused rather than treated as 0%: a missing port
	// is a wiring fault, and a "0%" would be a claim about a student nobody asked.
	if s.completion == nil {
		logger.Error("StudsToken referral qualification has no profile-completion port",
			"referral_id", ref.ID, "error", ErrNoDatabase)
		return Qualification{Reason: qualificationFailedReason}
	}
	percent, err := s.completion.ProfileCompletionPercent(ctx, ref.ReferredUserID)
	if err != nil {
		logger.Error("StudsToken referral could not read invitee profile completion",
			"referral_id", ref.ID, "invitee_user_id", ref.ReferredUserID, "error", err)
		return Qualification{Reason: qualificationFailedReason}
	}
	// 100 and not "the top step's threshold". §5.2 says "completes profile", and the
	// page's own 100% is what a student is being told. The threshold that matters
	// for the profile AWARD is ProfileStepThresholdPercent(5, 5) = 100 today, so the
	// two agree — but deriving this from the ladder would couple a referral's
	// qualification to a re-pricing of an unrelated award, and a referral paid
	// because someone set profile_instalments to 3 would be wrong.
	if percent < profileCompletePercent {
		return Qualification{Reason: QualificationProfileIncomplete}
	}

	// The phone port, nil-checked even though QualifyPendingReferrals short-circuits
	// above it. Two reasons, and the second is the real one: classify is the whole of
	// §5.2 and is called directly by tests, so a nil here is reachable, and a method
	// that panics on a wiring mistake is a wiring mistake that surfaces as a crashed
	// background job rather than a logged error.
	if s.phone == nil {
		// Deliberately NOT QualificationPhoneUnverified. That asserts a fact about the
		// invitee, and the fact is unknown here — nothing was asked. The distinct
		// constant is what keeps "we cannot check" from being reported as "they did not".
		logger.Warn("StudsToken referral qualification reached the phone check with no "+
			"phone-verification port wired", "referral_id", ref.ID)
		return Qualification{Reason: QualificationNoVerifier}
	}
	verified, err := s.phone.PhoneVerified(ctx, ref.ReferredUserID)
	if err != nil {
		logger.Error("StudsToken referral could not read invitee phone verification",
			"referral_id", ref.ID, "invitee_user_id", ref.ReferredUserID, "error", err)
		return Qualification{Reason: qualificationFailedReason}
	}
	if !verified {
		return Qualification{Reason: QualificationPhoneUnverified}
	}
	return Qualification{Qualified: true, Reason: QualificationYes}
}

// profileCompletePercent is the completion §5.2's "completes profile" means.
//
// A named constant rather than a literal, so that the referral rule and the page's
// own notion of complete cannot drift by someone editing one of them. The value is
// 100 because ProfileCompletionPercent returns a percentage of the twelve checks
// and the twelve are all-or-nothing: there is no partial state that counts as
// complete. See studentdashboard.ProfileCompletionPercent.
const profileCompletePercent = 100

// pendingForQualification reads the pending referrals a pass will look at.
//
// OLDEST FIRST, deliberately, and it is a fairness property rather than a
// performance one: two students whose invitees qualified on the same day compete for
// the same ten monthly cap slots, and LIFO would pay whoever happened to be scanned
// last. Ordering by id gives the older attribution first claim on the cap, which is
// also the only ordering a student could describe as reasonable if they complained.
func (s *ReferralService) pendingForQualification(limit int) ([]UserReferral, error) {
	var rows []UserReferral
	if err := s.repo.DB().Raw(
		`SELECT id, referrer_user_id, referred_kind, referred_user_id, referral_code, status,
		        awarded_coins, created_at, updated_at
		   FROM user_referral
		  WHERE status = ?
		  ORDER BY id
		  LIMIT ?`, ReferralPending, limit,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read pending referrals: %w", err)
	}
	return rows, nil
}

// countPending is how many referrals a pass declined to look at.
//
// Its own query rather than len(pendingForQualification) because the limit would
// make the number a lie: a deployment with 12,000 pending referrals and no verifier
// would report 500, and the log line is supposed to be the thing an operator reads
// to decide whether the missing verification flow is urgent.
func (s *ReferralService) countPending() (int, error) {
	var n int64
	if err := s.repo.DB().Raw(
		`SELECT count(*) FROM user_referral WHERE status = ?`, ReferralPending,
	).Scan(&n).Error; err != nil {
		return 0, fmt.Errorf("count pending referrals: %w", err)
	}
	return int(n), nil
}

// expireReferral moves a referral that can never qualify to its terminal state, with
// the reason recorded.
//
// ONE transaction, and it is taken on the REFERRER rather than on the referral,
// because the pass runs concurrently across a whole cohort and several referrals in
// one pass can share a referrer. Serialising on the referrer means two instances of
// this job cannot interleave writes against the same account's referral set, which
// is the same choice SettleReferral makes for the same reason.
//
// The row is re-read FOR UPDATE and re-checked before the write. A referral that
// qualified and was paid a second earlier — because its invitee is a student after
// all, or because the kind was corrected — must not be expired out from under a
// settlement that has already committed. Terminal, and already terminal, are both
// no-ops here rather than errors: the pass runs on a timer against rows other
// processes touch, and treating "somebody already got there first" as a failure
// would make every overlapping pass look broken.
func (s *ReferralService) expireReferral(ctx context.Context, ref UserReferral, reason string) error {
	if reason == "" {
		return fmt.Errorf("%w: expiring referral %d with no recorded reason", ErrInvalidArgument, ref.ID)
	}
	return s.repo.InUserTx(ctx, ref.ReferrerUserID, func(tx *TxContext) error {
		current, err := readReferralForUpdate(tx, ref.ID)
		if err != nil {
			return err
		}
		switch {
		case current.Status != ReferralPending:
			// Somebody else moved it — settled, or already terminal. Not an error:
			// this is a timer running over shared state, and treating "someone got
			// here first" as a failure would make every overlapping pass look broken.
			// The write below is guarded on status = 'pending' for the same reason, so
			// the re-read is a cheap assertion rather than the mechanism.
			return nil
		}
		now := s.now().UTC()
		if err := tx.DB().Exec(
			`UPDATE user_referral
			    SET status = ?, expired_at = ?, expired_reason = ?, updated_at = ?
			  WHERE id = ? AND status = ?`,
			ReferralExpired, now, reason, now, ref.ID, ReferralPending,
		).Error; err != nil {
			return fmt.Errorf("expire referral %d: %w", ref.ID, err)
		}
		return nil
	})
}

// ── the driver ────────────────────────────────────────────────────────────────

// StartReferralQualifier runs the qualification pass on a ticker until stop() is
// called. Follows StartReconciler's shape, and is deliberately a DIFFERENT job —
// read the file header for why the reconciler's loop does not also pay referrals.
//
// The first pass is deferred by one interval rather than run at startup, for
// StartReconciler's reason: on a rolling deploy several instances would otherwise
// all qualify at once, and a fresh boot has nothing new to say. That the second
// instance's pass pays nothing is then a property of the claim rather than of luck,
// which is why the double-pass test exists.
//
// It never calls logger.Fatal. A qualification pass that fails is retried on the
// next tick and reported on this one, and killing the process over it would take
// down the wallet, the unlock endpoint and every other request to fix a job that
// pays nothing until it succeeds.
func StartReferralQualifier(svc *ReferralService, interval, timeout time.Duration) (stop func()) {
	// closed by stop(), and SELECTED on rather than sent to.
	//
	// StartReconciler's shape is `done <- struct{}{}` on a channel the goroutine
	// receives from, and that shape PANICS on a second stop(): the first send is
	// received, the goroutine returns and closes done, and the second send is a send
	// on a closed channel. It is invisible in production because nothing calls stop()
	// twice — main.go discards it — and a stop function that kills the process when
	// called twice is the kind of defect that surfaces during an incident.
	//
	// close() is idempotent and safe from any goroutine, which is what a stop function
	// has to be. A `sync.Once` is belt-and-braces for the ticker as well: Stop is
	// already safe to call twice, and being explicit costs nothing.
	stopCh := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runQualificationOnce(svc, timeout)
			case <-stopCh:
				return
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stopCh) }) }
}

// runQualificationOnce performs a pass and logs the outcome. Split out from the
// ticker so a test can drive it directly, which is what the double-pass test does
// rather than sleeping through two intervals.
func runQualificationOnce(svc *ReferralService, timeout time.Duration) QualificationReport {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	report, err := svc.QualifyPendingReferrals(ctx)
	if err != nil {
		logger.Error("StudsToken referral qualification pass failed",
			"scanned", report.Scanned, "settled", report.Settled, "expired", report.Expired,
			"skipped", report.Skipped, "failed", report.Failed(),
			"duration_ms", report.Duration.Milliseconds(), "error", err)
		return report
	}
	if report.Settled == 0 && report.Expired == 0 {
		// Nothing moved. Still logged, and still with the scanned count, because
		// "the pass ran and paid nobody" and "the pass did not run" have to be
		// distinguishable in the log — that is the whole difference between a
		// mechanic that is working and one that is switched off.
		logger.Info("StudsToken referral qualification pass found nothing to settle",
			"scanned", report.Scanned, "skipped", report.Skipped,
			"duration_ms", report.Duration.Milliseconds())
		return report
	}
	logger.Info("StudsToken referral qualification pass settled referrals",
		"scanned", report.Scanned, "settled", report.Settled, "expired", report.Expired,
		"skipped", report.Skipped, "failed", report.Failed(),
		"by_reason", report.ByReason, "duration_ms", report.Duration.Milliseconds())
	return report
}
