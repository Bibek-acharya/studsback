// internal/coins/profile_award.go
//
// The profile-completion award: the first earn mechanic, and the reference for
// every earn mechanic after it.
//
// ── why a ladder and not a boolean ────────────────────────────────────────────
//
// awards.profile_complete is 25, paid as awards.profile_instalment x
// awards.profile_instalments = 5 x 5. The instalments exist so the ledger sees
// repeated grants (each with its own journal and its own lot) and so a profile
// page can show "3 of 5 steps". A student who jumps from an empty profile to a
// full one in one save therefore earns five instalments, not one award of 25, and
// earns all of them — the ladder is about SIZE, not about pacing.
//
// ── why the award is driven by crossing, not by value ─────────────────────────
//
// ProfileCompletion is recomputed from the user row on every request and can go
// DOWN: delete your address and the percentage drops. So "am I at 100% now" is not
// an awardable event, and "was I at 100% last time" is not either, because the
// answer to the second question is stored nowhere.
//
// The rule is therefore: for every step n the student has NOT already been paid,
// award it if and only if the current completion has reached step n's threshold.
// Grant-then-never-revoke means a drop from 100% to 90% and back to 100% pays
// nothing the second time, because all five rows already exist and the claim
// insert for each one returns no row. There is no "last award" state to keep
// correct, and no read-modify-write anywhere.
//
// ── where the award is called from, and why that is the hard part ─────────────
//
// Not from here. This package exposes one method, AwardProfileSteps, and
// internal/auth calls it after each write that can move one of the twelve
// completion fields. The enumeration of those paths, and why the chokepoint is a
// port on the auth service rather than the auth repository, is documented at
// internal/auth/profile_award.go. Read that before changing this.
package coins

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"studsphere/backend/internal/notification"
)

// ProfileStepCodePrefix is the award_code prefix for the ladder. The step number
// is appended: "PROFILE_STEP:1" ... "PROFILE_STEP:5".
//
// The prefix is what makes the ladder addressable one step at a time inside the
// single UNIQUE (user_id, award_code) constraint. There is no separate
// "steps_paid" counter to fall out of step with the rows: the rows ARE the
// counter, and stepsEarnedTo is a pure function of them.
const ProfileStepCodePrefix = "PROFILE_STEP:"

// ProfileStepAwardCode is the award_code for step n (1-based).
func ProfileStepAwardCode(step int) string {
	return ProfileStepCodePrefix + strconv.Itoa(step)
}

// ProfileStepThresholdPercent is the completion percentage at which step n is
// earned.
//
// Steps divide the 0..100 range evenly, so with the configured five instalments
// the thresholds are 20/40/60/80/100 and a student must finish the profile to
// collect the last one. The arithmetic is ceil(n * 100 / instalments) rather than
// n * (100/instalments) because integer division on the divisor alone silently
// misplaces the final step for any instalment count that does not divide 100 —
// with 3 instalments, 100/3 is 33 and step 3 would be paid at 99%, leaving a
// completion of 100% that had not earned the whole award.
func ProfileStepThresholdPercent(step int, instalments int64) int {
	if step <= 0 || instalments <= 0 {
		return 0
	}
	n := int64(step)
	return int((n*100 + instalments - 1) / instalments)
}

// stepsEarned is how many steps a completion percentage has earned: the largest
// n whose threshold the percentage has reached.
//
// Deliberately NOT a function of any stored counter, so it cannot disagree with
// the reward_grant rows. AwardProfileSteps then awards every step in
// 1..stepsEarned that has no row yet, which is what makes the ladder resumable:
// a student who was paid for steps 1-2, then jumped to 100%, collects 3, 4 and 5
// and nothing else.
//
// The one thing this must never become is `if stepsEarned > stepsPaid { award
// ONE }`. That shape pays the top step only, so a student who jumped 0% -> 100%
// in a single save would collect 5 coins instead of 25 — the exact failure
// 04-implementation-plan.md §5.1's "5 instalments" language exists to prevent.
func stepsEarned(percent int, instalments int64) int {
	if percent <= 0 || instalments <= 0 {
		return 0
	}
	earned := 0
	for step := 1; step <= int(instalments); step++ {
		if percent < ProfileStepThresholdPercent(step, instalments) {
			break
		}
		earned = step
	}
	return earned
}

// ProfileCompletion is the read the award depends on: a percentage, 0..100.
//
// It is a PORT, declared here and satisfied by an adapter in cmd/server/main.go
// over studentdashboard. That module owns the twelve checks, and reimplementing
// them here would be a second source of truth that drifts — the student would be
// paid against one definition of "complete" while the page shows them another.
// The same reason the unlock API's ProfileEligibility adapter exists.
type ProfileCompletion interface {
	ProfileCompletionPercent(ctx context.Context, userID uint) (int, error)
}

// profileAwardNotifier announces a credit. Optional: with no notifier wired the
// coins still move and no announcement is sent.
type profileAwardNotifier interface {
	Notify(ctx context.Context, req notification.NotifyRequest) error
}

// ProfileAward is one step that this call actually paid for.
//
// Returned per step rather than as a total because callers want to announce
// per step ("+5 StudsTokens"), and because a single value would hide how many
// steps a 0 -> 100 jump paid.
type ProfileAward struct {
	Step           int
	Code           string
	Coins          int64
	JournalID      string
	Available      int64
	ProfilePercent int
}

// ProfileAwardService pays the profile-completion ladder.
//
// One service rather than a function on the Ledger, because the award is not a
// ledger operation: it is a decision (which steps has this student earned?) plus
// a ledger grant per step. The Ledger stays the only thing that moves coins, and
// this service only decides when to ask.
type ProfileAwardService struct {
	repo       *Repository
	ledger     *Ledger
	completion ProfileCompletion
	notifier   profileAwardNotifier
	now        func() time.Time
}

// NewProfileAwardService wires the awarder.
//
// ledger and completion are both required: without the ledger there is nothing to
// pay with, and without the completion read there is nothing to decide against.
// notifier is optional and defaults to nil.
func NewProfileAwardService(repo *Repository, ledger *Ledger, completion ProfileCompletion, notifier profileAwardNotifier) *ProfileAwardService {
	s := &ProfileAwardService{
		repo:       repo,
		ledger:     ledger,
		completion: completion,
		notifier:   notifier,
		now:        func() time.Time { return time.Now().UTC() },
	}
	return s
}

// WithNotifier replaces the announcement seam. Used by tests to count emissions
// without a notification outbox.
func (s *ProfileAwardService) WithNotifier(n profileAwardNotifier) *ProfileAwardService {
	s.notifier = n
	return s
}

// AwardProfileSteps pays every ladder step the student's completion has reached
// and that has not been paid before.
//
// Safe to call on every profile write, from every path, repeatedly: a student
// whose profile has not changed gets zero rows and zero coins. Safe to call
// concurrently: the claim insert and the ledger's per-user advisory lock make
// each step exactly-once.
//
// Returns the steps it paid in this call. An error means the ladder stopped
// early, and the steps returned alongside it DID commit — so a caller must treat
// the partial result as real and may safely call again.
func (s *ProfileAwardService) AwardProfileSteps(ctx context.Context, userID uint) ([]ProfileAward, error) {
	if s == nil || s.repo == nil || s.ledger == nil {
		return nil, ErrNoDatabase
	}
	if s.completion == nil {
		return nil, fmt.Errorf("%w: the profile award needs a completion lookup", ErrInvalidArgument)
	}
	if userID == 0 {
		return nil, fmt.Errorf("%w: the profile award needs a user id", ErrInvalidArgument)
	}

	cfg, err := s.ledger.config.Load()
	if err != nil {
		return nil, err
	}
	instalments := cfg.Awards.ProfileInstalments
	if instalments <= 0 {
		return nil, fmt.Errorf("%w: awards.profile_instalments is %d", ErrInvalidArgument, instalments)
	}

	percent, err := s.completion.ProfileCompletionPercent(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read profile completion for user %d: %w", userID, err)
	}
	target := stepsEarned(percent, instalments)
	if target == 0 {
		return nil, nil
	}

	// Each step is its own transaction. Five separate transactions, not one, and
	// that is the point: the ledger's own note on ReasonProfileComplete says a
	// mid-ladder failure should be resumable rather than all-or-nothing, and one
	// transaction wrapping the whole ladder would roll back steps the student had
	// genuinely earned because of an unrelated failure on step 5. Resuming is
	// free here precisely because the per-step claim is idempotent.
	var paid []ProfileAward
	for step := 1; step <= target; step++ {
		award, err := s.awardStep(ctx, userID, step, percent)
		if err != nil {
			return paid, err
		}
		if award == nil {
			continue // already paid by an earlier save, or by a concurrent one
		}
		paid = append(paid, *award)
	}
	return paid, nil
}

// awardStep pays one step, atomically.
//
// The order is: CLAIM first, then grant, in ONE transaction.
//
//	INSERT INTO reward_grant ... ON CONFLICT (user_id, award_code) DO NOTHING RETURNING id
//	  -- no id => already awarded; skip entirely, no journal, no coins
//	Ledger.Grant(...) inside the SAME transaction (advisory lock already held)
//	UPDATE reward_grant SET journal_id = ...
//	COMMIT
//
// Claiming first is what makes the whole thing exactly-once rather than
// best-effort. If the grant committed but the claim did not, the student would
// hold coins for a step with no row, and the next save would pay it AGAIN —
// double-paying is worse than a lost award, and a unique constraint on a row that
// was written after the money would not prevent it. Because the claim and the
// grant are one transaction, neither can exist without the other.
//
// The grant runs through a Ledger built over this transaction's own handle, which
// is how coinPurchase composes Spend (see spendInTx). The per-user advisory lock
// InUserTx took is still held, so two concurrent profile saves serialise here and
// the loser's claim insert returns no row.
func (s *ProfileAwardService) awardStep(ctx context.Context, userID uint, step int, percent int) (*ProfileAward, error) {
	code := ProfileStepAwardCode(step)
	var out *ProfileAward

	err := s.repo.InUserTx(ctx, userID, func(tx *TxContext) error {
		var id uint
		res := tx.DB().Raw(
			`INSERT INTO reward_grant (user_id, award_code, amount, granted_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT (user_id, award_code) DO NOTHING
			 RETURNING id`,
			userID, code, 0, s.now().UTC(),
		).Scan(&id)
		if res.Error != nil {
			return fmt.Errorf("claim profile step %s for user %d: %w", code, userID, res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}

		grant, err := s.grantStep(ctx, tx, userID, step, code, percent)
		if err != nil {
			return err
		}
		// journal_id is backfilled rather than supplied on the claim insert
		// because the claim has to come first to be the once-only gate. It is
		// nullable for that one statement; the row leaves this transaction with
		// it set, or does not exist at all.
		if err := tx.DB().Model(&RewardGrant{}).
			Where("id = ?", id).
			Update("journal_id", grant.JournalID).Error; err != nil {
			return fmt.Errorf("link profile step %s to journal %s: %w", code, grant.JournalID, err)
		}
		if err := tx.DB().Model(&RewardGrant{}).
			Where("id = ?", id).
			Update("amount", grant.Amount).Error; err != nil {
			return fmt.Errorf("record profile step %s amount: %w", code, err)
		}

		out = &ProfileAward{
			Step:           step,
			Code:           code,
			Coins:          grant.Amount,
			JournalID:      grant.JournalID,
			Available:      grant.Available,
			ProfilePercent: percent,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}

	s.announce(ctx, userID, *out)
	return out, nil
}

// grantStep performs the ledger grant for one step, on the caller's open
// transaction.
func (s *ProfileAwardService) grantStep(ctx context.Context, tx *TxContext, userID uint, step int, code string, percent int) (GrantResult, error) {
	// Idempotency key includes the user id. It must: the unique index on
	// coin_journal is (scope, idempotency_key) and ScopeUser is a CONSTANT
	// (repository.go documents this explicitly), so a key without the user would
	// make student 7's PROFILE_STEP:1 collide with student 8's and come back as
	// a fingerprint mismatch. Embedding the step also means each instalment is
	// independently idempotent, which is what makes the ladder resumable.
	key := "profile-step:" + strconv.FormatUint(uint64(userID), 10) + ":" + strconv.Itoa(step)

	ledger := NewLedger(NewRepository(tx.DB()), s.ledger.config)
	grant, err := ledger.Grant(ctx, GrantRequest{
		UserID:         userID,
		ReasonCode:     ReasonProfileComplete,
		IdempotencyKey: key,
		RefType:        nil,
		RefID:          nil,
		CreatedBy:      "system:profile_award",
		Metadata: map[string]any{
			"award_code": code,
			"step":       step,
			"percent":    percent,
		},
	})
	if err != nil {
		return GrantResult{}, err
	}
	if grant.Replayed {
		// Unreachable while the claim insert gates this, and fatal if it ever
		// happens: it would mean the journal idempotency key fired without the
		// reward_grant row, i.e. two award paths disagree. Roll back rather than
		// commit a claim row whose money is not this transaction's.
		return GrantResult{}, fmt.Errorf("profile step %s replayed without a claim row", code)
	}
	return grant, nil
}

// announce sends coins.credited for one step.
//
// AFTER the transaction, never inside it. The coins are the fact; the
// announcement is a courtesy, and a notification outbox that fails must not roll
// back an award the student earned. It is also why the emission is keyed on the
// journal id: a retried announce is the same occurrence, not a second credit
// notice for one credit.
//
// Data keys are named to match the registry templates exactly, because those
// templates carry missingkey=error and a missing key is a dropped notification
// rather than a blank line.
func (s *ProfileAwardService) announce(ctx context.Context, userID uint, award ProfileAward) {
	if s.notifier == nil {
		return
	}
	_ = s.notifier.Notify(ctx, notification.NotifyRequest{
		EventKey:   notification.EventCoinsCredited,
		Recipients: []notification.Ref{{Type: "user", ID: userID}},
		Data: map[string]any{
			"coins":   award.Coins,
			"balance": award.Available,
			"step":    award.Step,
		},
		OccurrenceKey: notification.EventCoinsCredited + ":" + award.JournalID,
		CorrelationID: award.JournalID,
	})
}
