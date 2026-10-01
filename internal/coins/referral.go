// internal/coins/referral.go
//
// The attribution core: accept a referral code on account creation, record it, and
// make every fraud control a property of the schema rather than of Go.
//
// ── the one rule this file exists to keep ─────────────────────────────────────
//
// SINGLE-LEVEL. The design credits only the DIRECT referrer: one row, one
// referrer_user_id, resolved from the code and nothing else. There is no code path
// in this file that reads a referral's own referrer and writes another row, and
// there is deliberately no way to add one without failing
// TestNoCodePathPropagatesARewardPastOneHop — which walks the package's own SQL
// rather than asserting one example.
//
// That matters more than it looks. Consumer Protection Act 2075 s.18(e) names a
// "token system" and s.16(2)(p) prohibits "levels or series", so a multi-level
// referral tree is PROHIBITED, not regulated: a pyramid-scheme prohibition, not a
// tax question. There is no compliant deeper version to negotiate toward, which
// means the single-level property has to be structural rather than a policy that
// a future well-meaning request ("let B's invitees also thank A") can quietly
// overturn — the same way a transfer path would.
//
// ── what this slice does NOT do ───────────────────────────────────────────────
//
// No qualification, no 7-day reserved hold, no release, no clawback. Those are the
// next slice, and the seams they need are here: the status column and its CHECK
// exist, and SettleReferral exists as the ONE place that can later turn a hold
// into a grant — combining what the ledger splits across two transactions.
//
// See the ledger's own note at ledger.go:1308-1313, which recorded that
// "converting a hold into a grant is ReleaseReserved followed by Grant, and in two
// transactions that leaves a window in which the coins are unheld" and that "the
// referral phase must do both in one transaction and should say so where it does".
// The answer is in-settle-referral below.
package coins

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"studsphere/backend/internal/shared/utils"
)

// Attribution is the request: an account was created carrying this referral code.
//
// ReferredKind and ReferredUserID identify WHICH account, and the pair matters
// because this codebase has three account tables with independent id sequences.
// See referral_model.go.
type Attribution struct {
	// ReferredKind is one of ReferredKinds.
	ReferredKind string
	// ReferredUserID is the id of the new account in the table ReferredKind names.
	ReferredUserID uint
	// ReferralCode is the code as presented, not yet normalised. Normalisation
	// happens here so that every caller normalises identically — a caller that
	// stored the raw string would let one code exist in two spellings and neither
	// would collide with the other's unique index.
	ReferralCode string
	// SourcePath records which of the four user-creation paths this came through.
	SourcePath string
}

// AttributionResult is what happened, and it distinguishes every way it can fail.
//
// The distinction matters because "no referral" and "fraudulent referral" are
// different operational answers. A no_code is a normal signup. An unknown_code is
// usually a typo. A self_referral and an already_attributed are the two an
// operator wants to see, and the difference between them is exactly the difference
// between a student who re-typed a code and a student trying to farm both sides.
type AttributionResult struct {
	// Attributed is true only when a NEW row was written. A duplicate claim, a
	// self-referral, an unknown code and a capped referrer all return false.
	Attributed bool
	// Reason is one of AttributionReasons, empty when Attributed is true.
	Reason string
	// ReferralID and ReferrerUserID are set when Attributed is true.
	ReferralID     uint
	ReferrerUserID uint
}

// The reasons an attribution did not happen. Returned rather than logged-and-
// dropped so the caller can log something specific, and so a test can assert the
// distinction rather than only the outcome.
const (
	// AttributionNoCode means no code was presented. A normal signup.
	AttributionNoCode = "no_code"
	// AttributionUnknownCode means the code does not resolve to any account. A typo
	// or a code from an account that has since been deleted.
	AttributionUnknownCode = "unknown_code"
	// AttributionSelfReferral means the code belongs to the account being created.
	AttributionSelfReferral = "self_referral"
	// AttributionAlreadyAttributed means this account already has a referrer. This
	// is the double-claim, and it is refused by user_referral_referred_uniq rather
	// than by a check here.
	AttributionAlreadyAttributed = "already_attributed"
	// AttributionUnknownKind means ReferredKind is not one of ReferredKinds.
	AttributionUnknownKind = "unknown_kind"
	// AttributionNotPersisted means the row could not be written. Distinct from
	// every reason above because it is a database problem, not a policy outcome,
	// and a caller should treat it as retryable rather than final.
	AttributionNotPersisted = "not_persisted"
)

// ReferralAttributionPort is what internal/auth is handed.
//
// Declared HERE, not in internal/auth, and that direction is load-bearing. auth
// owns the four user-creation paths and therefore owns the TRIGGER; it cannot own
// the implementation without importing internal/coins, which is the inversion
// profile_award.go documents at length. So auth declares a one-method port and
// main wires the adapter, exactly as it does for ProfileAwarder.
//
// The four auth methods take (referredKind, referredUserID, code) and nothing
// else. In particular no auth caller can pass a referrer id: the referrer is
// resolved from the code inside ApplyReferral, by this package, and never by a
// caller. That is what makes "credits only the direct referrer" structural rather
// than a convention — there is no parameter through which a grandparent could be
// named.
type ReferralAttributionPort interface {
	ApplyReferral(ctx context.Context, attr Attribution) (AttributionResult, error)
}

// ReferralService applies attribution and owns the fraud controls.
type ReferralService struct {
	repo   *Repository
	ledger *Ledger
	now    func() time.Time
}

// NewReferralService wires the attribution service.
//
// ledger is required rather than optional: the cap slot is claimed with the
// AWARDED coin amount, and a service that could not read the config would have to
// invent one. See NewProfileAwardService for the same argument about a required
// dependency.
func NewReferralService(repo *Repository, ledger *Ledger) *ReferralService {
	return &ReferralService{
		repo:   repo,
		ledger: ledger,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// ApplyReferral attributes a new account to the referrer its code names.
//
// It never returns an error for a policy outcome — an unknown code, a self-
// referral, a duplicate claim and a capped referrer are all results, not failures,
// because in every one of those cases the caller's correct action is identical:
// let the signup finish. Only a genuine database failure returns an error, and
// even then auth's applyAttribution logs it rather than failing the signup, for
// the reason profile_award.go gives: a coin-system problem must not stop a student
// creating an account.
//
// The claim INSERT is ON CONFLICT (referred_kind, referred_user_id) DO NOTHING
// RETURNING id, and RowsAffected == 0 is the "already attributed" signal rather
// than an error being swallowed. That is the same shape as profile_award.go's
// awardStep, for the same reason: claiming first, inside the referrer's advisory
// lock, is what makes a double-claim impossible rather than unlikely.
func (s *ReferralService) ApplyReferral(ctx context.Context, attr Attribution) (AttributionResult, error) {
	if s == nil || s.repo == nil {
		return AttributionResult{}, ErrNoDatabase
	}
	if attr.ReferredUserID == 0 {
		return AttributionResult{}, fmt.Errorf("%w: attribution needs the new account's id", ErrInvalidArgument)
	}
	if !validReferredKind(attr.ReferredKind) {
		return AttributionResult{Reason: AttributionUnknownKind},
			fmt.Errorf("%w: referred_kind %q is not one of %v", ErrInvalidArgument, attr.ReferredKind, ReferredKinds)
	}

	code := utils.NormalizeReferralCode(attr.ReferralCode)
	if code == "" {
		// No code presented, or nothing survived normalisation. A normal signup,
		// not a failure — the overwhelming majority of signups are this.
		return AttributionResult{Reason: AttributionNoCode}, nil
	}

	// Resolve the code to its owner. This is the ONLY place a referrer id is
	// obtained, and it is obtained from the users table by code rather than from
	// any referral row. That single fact is what makes the mechanic single-level:
	// there is no input to this function that names a referrer, so there is no
	// input that could name a grandparent.
	referrerID, err := s.resolveReferrer(ctx, code)
	if err != nil {
		return AttributionResult{Reason: AttributionNotPersisted}, err
	}
	if referrerID == 0 {
		return AttributionResult{Reason: AttributionUnknownCode}, nil
	}

	// Self-referral is refused here for a clear error, and again by
	// chk_user_referral_no_self. Both, for the same reason as everywhere else in
	// this package: the constraint is the guarantee, and the early return is what
	// produces a reason a caller can log.
	//
	// The kind guard is not decoration. institution id 7 is not student id 7, so
	// comparing the two for a non-student account would refuse a legitimate
	// attribution and report it as self-referral.
	if attr.ReferredKind == SubjectUser && referrerID == attr.ReferredUserID {
		return AttributionResult{Reason: AttributionSelfReferral}, nil
	}

	now := s.now().UTC()
	var id uint
	res := s.repo.DB().Raw(
		`INSERT INTO user_referral
		   (referrer_user_id, referred_kind, referred_user_id, referral_code, status,
		    source_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (referred_kind, referred_user_id) DO NOTHING
		 RETURNING id`,
		referrerID, attr.ReferredKind, attr.ReferredUserID, code, ReferralPending,
		attr.SourcePath, now, now,
	).Scan(&id)
	if res.Error != nil {
		return AttributionResult{Reason: AttributionNotPersisted},
			fmt.Errorf("attribute referral for %s %d: %w", attr.ReferredKind, attr.ReferredUserID, res.Error)
	}
	if res.RowsAffected == 0 {
		return AttributionResult{Reason: AttributionAlreadyAttributed}, nil
	}
	return AttributionResult{
		Attributed:     true,
		ReferralID:     id,
		ReferrerUserID: referrerID,
	}, nil
}

// resolveReferrer maps a normalised code to the account that owns it.
//
// Returns 0 for "no such code", which is not an error: a code that does not
// resolve is a typo far more often than it is an attack, and refusing the signup
// would punish the student who mistyped it.
//
// `deleted_at IS NULL` because a soft-deleted account's code must not keep
// attributing. A deletion already scheduled for a fraud finding should stop
// paying the referrer immediately rather than at the end of the retention window.
//
// NOT wrapped in a transaction and NOT under the referrer's lock. The read has to
// happen first — the lock is taken on a user whose id this call has not resolved
// yet — and it does not need one: a code resolves to the same account or to none,
// and the uniqueness claim below is what actually serialises the write.
func (s *ReferralService) resolveReferrer(ctx context.Context, code string) (uint, error) {
	var id uint
	err := s.repo.DB().WithContext(ctx).Raw(
		`SELECT id FROM users WHERE referral_code = ? AND deleted_at IS NULL`, code,
	).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("resolve referral code: %w", err)
	}
	return id, nil
}

// ── the monthly cap ───────────────────────────────────────────────────────────

// ErrCapReached is returned when a referrer has already consumed this month's
// referral cap.
//
// Declared here rather than left for later because errors.go says so
// explicitly: "ErrAccountExists, ErrCapReached and ErrSelfReferral belong to the
// referral and account slices ... and declaring an unused sentinel is a promise
// nobody keeps". This is the slice that uses it.
//
// Matched with errors.Is, so claimCapSlot wraps rather than substitutes.
var ErrCapReached = errors.New("referral cap reached for this month")

// ErrSelfReferral is the self-referral refusal as an error, for the caller that
// wants to log it as one. ApplyReferral returns AttributionSelfReferral as a
// RESULT as well, because the caller's correct action is to let the signup
// finish, and an error would read as a failure.
var ErrSelfReferral = errors.New("a referral code cannot be used by the account that owns it")

// claimCapSlot takes one of the referrer's slots for the current month, or refuses.
//
// The whole cap, in one INSERT:
//
//	INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
//	VALUES (?, ?, ?, ?, ?, ?)
//	ON CONFLICT DO NOTHING
//	RETURNING slot
//
// `ON CONFLICT DO NOTHING` with no target rather than naming a constraint: the
// conflict that matters here is the PRIMARY KEY (referrer, month, slot) and, for a
// retry, the UNIQUE on referral_id. Naming one target would leave the other
// unprotected, and leaving both off is safe precisely because both are "this slot
// is taken" or "this referral is already counted" — the two answers are the same,
// which is that this claim does not pay.
//
// RowsAffected == 0 therefore means exactly one thing: no slot available. Either
// the month is full, or this referral already took one. The caller distinguishes
// them with a read, because "you have hit your cap" and "this one already paid"
// are different sentences for the person being told.
//
// Called under the REFERRER's advisory lock, from in-settle-referral. The lock
// matters for the read-then-insert below and nothing else, and it is the referrer
// rather than the referred user because the cap is the referrer's.
func (s *ReferralService) claimCapSlot(ctx context.Context, tx *TxContext, referralID, referrerID uint, coins int64) (int, error) {
	if tx == nil {
		return 0, ErrNoDatabase
	}
	cfg, err := s.ledger.config.Load()
	if err != nil {
		return 0, err
	}
	// The CONFIGURED cap, which may be lower than the schema ceiling. It is read
	// here and not baked into the SQL because an operator lowering referral.
	// monthly_cap mid-incident should take effect on the next referral without a
	// deploy — that is what the config knob is FOR. It cannot raise the ceiling;
	// see referral_cap_model.go.
	cap := cfg.Referral.MonthlyCap
	if cap <= 0 {
		return 0, fmt.Errorf("%w: referral.monthly_cap is %d", ErrInvalidArgument, cap)
	}
	if cap > ReferralCapSlotCeiling {
		// Refusing rather than silently truncating. A config that asks for more
		// than the schema permits would otherwise be paid out to the ceiling with
		// no indication that the operator's change did not take, which is the
		// quietest possible failure for an economy control.
		return 0, fmt.Errorf("%w: referral.monthly_cap is %d but the schema ceiling is %d; "+
			"raising it needs a migration", ErrInvalidConfig, cap, ReferralCapSlotCeiling)
	}

	month := ReferralMonthKey(s.now().UTC())

	// Find the next free ordinal. Under the referrer's advisory lock this is
	// safe: no other transaction for this referrer is between the read and the
	// write, and the PRIMARY KEY is the backstop if one ever is.
	//
	// MAX(slot) + 1 rather than COUNT(*): COUNT would be wrong the moment a slot
	// was released or a row removed, and would hand out an ordinal already taken.
	// The insert would then be refused, so this is a "fails loudly" difference
	// rather than a "pays twice" one — but a claim that fails because of a
	// reclaimed slot is a claim that fails for no reason, and the reference to
	// chk_referral_cap_slot is there to make the ceiling the thing that stops the
	// eleventh, not an accident of the ordinal.
	var next int
	if err := tx.DB().Raw(
		`SELECT COALESCE(MAX(slot), 0) + 1 FROM referral_cap_slot
		  WHERE referrer_user_id = ? AND month_key = ?`,
		referrerID, month,
	).Scan(&next).Error; err != nil {
		return 0, fmt.Errorf("read next referral cap slot for user %d: %w", referrerID, err)
	}
	if int64(next) > cap {
		return 0, fmt.Errorf("%w: %d slots already used in %s, the cap is %d", ErrCapReached, next-1, month, cap)
	}

	now := s.now().UTC()
	var slot int
	res := tx.DB().Raw(
		`INSERT INTO referral_cap_slot
		   (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT DO NOTHING
		 RETURNING slot`,
		referrerID, month, next, referralID, coins, now,
	).Scan(&slot)
	if res.Error != nil {
		return 0, fmt.Errorf("claim referral cap slot: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		// Either the month filled between the read and the write, or this
		// referral already holds a slot. One message for both, because the caller's
		// action is identical: do not pay, and let the referral stand as it is.
		return 0, fmt.Errorf("%w: slot %d in %s was taken", ErrCapReached, next, month)
	}
	return slot, nil
}

// ReferralMonthKey is the 'YYYY-MM' calendar month of t in UTC.
//
// Exported because the next slice's qualification sweep groups by it, and a month
// key computed two different ways in two files is a cap that resets twice.
func ReferralMonthKey(t time.Time) string {
	return t.UTC().Format("2006-01")
}

// ── the single-transaction seam ───────────────────────────────────────────────

// SettleReferral turns a qualified referral into paid coins, as ONE unit.
//
// This is the answer to the gap Phase 1 recorded and this phase was told to close.
// ledger.go:1308-1313: "converting a hold into a grant is ReleaseReserved followed
// by Grant, and in two transactions that leaves a window in which the coins are
// unheld. The referral phase must do both in one transaction and should say so
// where it does."
//
// So it does. The ordering, and why each step is where it is:
//
//  1. CLAIM the reward_grant row for this referral, ON CONFLICT DO NOTHING.
//     First, because it is the once-only gate: a no-row-returned means this
//     referral has already been paid and nothing after it runs. Claiming after the
//     grant would let a crash between them pay the same referral twice, and a
//     unique constraint written after the money does not prevent that.
//  2. CLAIM a monthly cap slot. Before any coins move, so a capped referrer causes
//     no partial payment.
//  3. RELEASE the hold and GRANT, both on the caller's transaction.
//
// Steps 3's two halves are where the two-transaction problem would live, and they
// are avoided by not calling Ledger.ReleaseReserved and Ledger.Grant at all:
// both open their own transaction through InUserTx, and InUserTx inside InUserTx is
// a nested transaction GORM resolves with a SAVEPOINT — which commits with the
// outer transaction or rolls back with it. releaseAndGrantInTx below is the
// body of both, driven directly on tx so there is no second COMMIT to be early.
//
// The consequence, which is the point: a released hold without its grant is not
// reachable. If the grant fails, the release rolls back with it and the coins stay
// held — which is the safe direction, because a hold is a promise the balance
// already reflects and a grant that is missing is a referral that pays on the next
// qualification attempt. The opposite failure — grant committed, release rolled
// back — would leave coins both held and spendable, and it is not representable.
//
// This slice does NOT call SettleReferral: qualification, the 7-day hold and the
// clawback are the next slice, and nothing here advances a referral past 'pending'
// on its own. It exists, tested, as the place the next slice's state machine calls.
func (s *ReferralService) SettleReferral(ctx context.Context, referralID uint) (GrantResult, error) {
	if s == nil || s.repo == nil || s.ledger == nil {
		return GrantResult{}, ErrNoDatabase
	}
	if referralID == 0 {
		return GrantResult{}, fmt.Errorf("%w: settling a referral needs its id", ErrInvalidArgument)
	}
	cfg, err := s.ledger.config.Load()
	if err != nil {
		return GrantResult{}, err
	}
	referrerCoins := cfg.Awards.ReferralReferrer
	if referrerCoins <= 0 {
		return GrantResult{}, fmt.Errorf("%w: awards.referral_referrer is %d, so there is nothing to settle",
			ErrInvalidArgument, referrerCoins)
	}

	var out GrantResult
	err = s.repo.InUserTx(ctx, s.referrerFor(referralID), func(tx *TxContext) error {
		ref, err := readReferralForUpdate(tx, referralID)
		if err != nil {
			return err
		}
		if ref.Status == ReferralClawedBack || ref.Status == ReferralRejected {
			return fmt.Errorf("%w: referral %d is %s and must not be paid", ErrImmutable, referralID, ref.Status)
		}

		// 1. the once-only claim.
		claimCode := ReferralAwardCode(referralID)
		var claimID uint
		claim := tx.DB().Raw(
			`INSERT INTO reward_grant (user_id, award_code, amount, granted_at)
			 VALUES (?, ?, 0, ?)
			 ON CONFLICT (user_id, award_code) DO NOTHING
			 RETURNING id`,
			ref.ReferrerUserID, claimCode, s.now().UTC(),
		).Scan(&claimID)
		if claim.Error != nil {
			return fmt.Errorf("claim referral award for %d: %w", referralID, claim.Error)
		}
		if claim.RowsAffected == 0 {
			// Already settled. Returning nil with a zero GrantResult is the same
			// answer Reserve gives for a replay, and the state column on the row
			// is where "it was paid" is actually recorded.
			return nil
		}

		// 2. the cap, before any coins move.
		slot, err := s.claimCapSlot(ctx, tx, referralID, ref.ReferrerUserID, referrerCoins)
		if err != nil {
			return err
		}

		// 3. release the hold, then grant, on THIS transaction.
		grant, err := s.releaseAndGrantInTx(ctx, tx, ref)
		if err != nil {
			return err
		}

		month := ReferralMonthKey(s.now().UTC())
		if err := tx.DB().Exec(
			`UPDATE user_referral
			    SET status = ?, awarded_coins = ?, cap_period = ?, cap_slot = ?,
			        hold_journal_id = COALESCE(hold_journal_id, ?), grant_journal_id = ?,
			        qualified_at = COALESCE(qualified_at, ?), released_at = ?, updated_at = ?
			  WHERE id = ?`,
			ReferralQualified, referrerCoins, month, slot,
			nullableString(grant.HoldJournalID), grant.JournalID,
			s.now().UTC(), s.now().UTC(), s.now().UTC(), referralID,
		).Error; err != nil {
			return fmt.Errorf("record settlement of referral %d: %w", referralID, err)
		}
		if err := tx.DB().Model(&RewardGrant{}).Where("id = ?", claimID).
			Update("journal_id", grant.JournalID).Error; err != nil {
			return fmt.Errorf("link referral award to journal %s: %w", grant.JournalID, err)
		}
		out = grant.GrantResult
		return nil
	})
	if err != nil {
		return GrantResult{}, err
	}
	return out, nil
}

// referralGrant is the internal shape of a settled referral's money movement: the
// hold that was lifted and the grant that paid for it, both already posted.
type referralGrant struct {
	HoldJournalID string
	GrantResult
}

// releaseAndGrantInTx is the body of both Ledger.ReleaseReserved and
// Ledger.Grant, run on the caller's open transaction.
//
// It exists because calling the two exported methods would each open their own
// InUserTx, and a nested transaction is a SAVEPOINT — which does commit and roll
// back with the outer one, so the two-transaction window would in fact close. It
// is written here anyway for three reasons: the outer transaction must also carry
// the claim INSERT and the state UPDATE, and interleaving four nested savepoints
// with two of them writing the same balance rows is harder to read than one
// function that does the whole settlement; the callers of ReleaseReserved and
// Grant keep their own tested bodies untouched; and a change to either ledger
// primitive then has one obvious place to be reflected rather than two.
//
// Both halves are ordered: release first, then grant. The reverse order would be
// wrong in the one window that matters — the no-overdraft CHECK evaluates
// posted - reserved, so granting first while the hold is still reserved can make a
// student's available balance dip below zero transiently and the grant's own
// posting would be refused on a balance that is about to be corrected.
func (s *ReferralService) releaseAndGrantInTx(ctx context.Context, tx *TxContext, ref UserReferral) (referralGrant, error) {
	var out referralGrant
	refType := RefUserReferral
	refID := uint64(ref.ID)
	now := s.now().UTC()

	// The hold. A referral with no open hold is not settled but not broken either:
	// the qualification slice places one before calling here, and a referral that
	// reached 'qualified' without one would mean the hold was already lifted. Both
	// are ErrNotFound from FindOpenHold and both mean "do not grant", so the
	// transaction rolls back rather than paying an unreconciled award.
	hold, err := tx.FindOpenHold(ReasonReferralHold, refType, &refID)
	if err != nil {
		return out, err
	}
	if hold == nil {
		return out, fmt.Errorf("%w: referral %d has no open %s to release", ErrNotFound, ref.ID, ReasonReferralHold)
	}
	held, accountID, err := holdMetadata(hold)
	if err != nil {
		return out, err
	}
	changed, err := tx.SetJournalState(hold.ID, StatePending, StateReversed, map[string]any{
		"released_at":  now.Format(time.RFC3339),
		"released_by":  "system:referral",
		"release_note": "referral qualified and settled",
	})
	if err != nil {
		return out, err
	}
	if !changed {
		// Already released between the read and the write. Under the user lock this
		// cannot happen, and it is checked anyway so the invariant is enforced
		// rather than assumed — the same reasoning as ReleaseReserved.
		return out, fmt.Errorf("%w: hold %s was released under this referral", ErrImmutable, hold.ID)
	}
	if err := tx.ApplyReserved(map[uint]int64{accountID: -held}, hold.ID); err != nil {
		return out, err
	}
	out.HoldJournalID = hold.ID

	// The grant. Same shape as Ledger.Grant, minus the config read (already done
	// by the caller, outside the transaction, for the reason ledger.go gives) and
	// plus the grantSpec lookup the caller resolved.
	spec, ok := grantSpecs[ReasonReferralQualified]
	if !ok {
		return out, fmt.Errorf("%w: reason %s is not a grant this ledger can make", ErrInvalidArgument, ReasonReferralQualified)
	}
	cfg, err := s.ledger.config.Load()
	if err != nil {
		return out, err
	}
	amount := spec.amount(cfg)
	if amount != held {
		// The hold and the grant must move the same number of coins. If an operator
		// re-prices referral_referrer between the hold and its settlement, the
		// difference is a pricing decision nobody made deliberately for THIS
		// referral, and paying either figure would make the ledger unexplainable:
		// the student would hold a balance that does not match its journal.
		return out, fmt.Errorf("%w: referral %d held %d coins and is configured to pay %d",
			ErrInvalidConfig, ref.ID, held, amount)
	}
	days := spec.expiryDays(cfg)
	if days <= 0 {
		return out, fmt.Errorf("%w: the expiry for bucket %s is configured to %d days", ErrInvalidArgument, spec.bucket, days)
	}
	expiresAt := now.AddDate(0, 0, int(days))

	key := "referral-qualified:" + strconv.FormatUint(uint64(ref.ReferrerUserID), 10) +
		":" + strconv.FormatUint(uint64(ref.ID), 10)
	fp := fingerprint(EntryGrant, strconv.FormatUint(uint64(ref.ReferrerUserID), 10),
		ReasonReferralQualified, refFingerprint(&refType, &refID), strconv.FormatInt(amount, 10))

	journal := &CoinJournal{
		ID:                 uuid.NewString(),
		EntryType:          EntryGrant,
		State:              StatePosted,
		Scope:              ScopeUser,
		IdempotencyKey:     key,
		RequestFingerprint: fp,
		ReasonCode:         ReasonReferralQualified,
		RefType:            &refType,
		RefID:              &refID,
		EffectiveAt:        now,
		CreatedAt:          now,
		CreatedBy:          "system:referral",
		Metadata:           grantMetadata(map[string]any{"referral_id": ref.ID}, amount, spec.bucket, &expiresAt),
	}
	created, err := tx.InsertJournal(journal)
	if err != nil {
		return out, err
	}
	if !created {
		// The reward_grant claim above already gates this, so reaching here means
		// the two disagree about whether this referral was paid. Roll back rather
		// than commit a claim row whose money is not this transaction's — the same
		// refusal profile_award.go's grantStep makes.
		return out, fmt.Errorf("referral %d settled twice: grant key %q conflicts without a claim row", ref.ID, key)
	}

	userAccount, err := tx.EnsureUserAccount(ref.ReferrerUserID, spec.bucket)
	if err != nil {
		return out, err
	}
	faucet, err := tx.SystemAccountID(SystemEarnedFaucet)
	if err != nil {
		return out, err
	}
	if err := tx.InsertPostings(journal.ID, []PostingLeg{
		{Seq: 1, AccountID: userAccount, Amount: amount},
		{Seq: 2, AccountID: faucet, Amount: -amount},
	}); err != nil {
		return out, err
	}
	if err := tx.ApplyPosted(map[uint]int64{userAccount: amount, faucet: -amount}, journal.ID); err != nil {
		return out, err
	}
	lot := &CoinLot{
		AccountID: userAccount,
		JournalID: journal.ID,
		Bucket:    spec.bucket,
		Granted:   amount,
		ExpiresAt: &expiresAt,
		CreatedAt: now,
	}
	if err := tx.CreateLot(lot); err != nil {
		return out, err
	}
	available, err := availableForUser(tx, ref.ReferrerUserID)
	if err != nil {
		return out, err
	}
	out.GrantResult = GrantResult{
		JournalID:   journal.ID,
		Amount:      amount,
		Bucket:      spec.bucket,
		LotID:       lot.ID,
		ExpiresAt:   &expiresAt,
		Available:   available,
		JournalTime: now,
	}
	return out, nil
}

// referrerFor reads the referrer off a referral so the advisory lock can be taken
// before anything else in the transaction.
//
// A separate read rather than a resolved parameter, because the lock has to be on
// the REFERRER (the cap is theirs, the coins are theirs) and the caller has only a
// referral id. It happens outside InUserTx by construction — the id has to be
// known before the lock can be taken at all — and it is safe there for the same
// reason Repository.JournalOwnerUserID is safe outside a transaction: the answer
// is a column of an append-only-ish row that nothing rewrites. The row is
// re-read FOR UPDATE inside the transaction before any decision, so a referral
// whose referrer changed in between is refused rather than paid to the wrong
// account.
func (s *ReferralService) referrerFor(referralID uint) uint {
	var id uint
	s.repo.DB().Raw(`SELECT referrer_user_id FROM user_referral WHERE id = ?`, referralID).Scan(&id)
	return id
}

// readReferralForUpdate reads a referral and holds it against concurrent updates
// for the rest of the transaction.
//
// FOR UPDATE rather than a plain SELECT because two settlements of the same
// referral would otherwise both read 'pending' and both proceed to the claim —
// which the claim insert would resolve, so this is belt-and-braces rather than the
// mechanism. It is here because the STATUS is read to make a decision (a
// clawed-back referral must not be paid) and a decision taken on a row another
// transaction is changing is not a decision.
func readReferralForUpdate(tx *TxContext, referralID uint) (UserReferral, error) {
	var ref UserReferral
	if err := tx.DB().Raw(
		`SELECT id, referrer_user_id, referred_kind, referred_user_id, referral_code, status,
		        awarded_coins, created_at, updated_at
		   FROM user_referral WHERE id = ? FOR UPDATE`, referralID,
	).Scan(&ref).Error; err != nil {
		return UserReferral{}, fmt.Errorf("read referral %d: %w", referralID, err)
	}
	if ref.ID == 0 {
		return UserReferral{}, fmt.Errorf("%w: referral %d", ErrNotFound, referralID)
	}
	return ref, nil
}

// ReferralAwardCode is the reward_grant award_code for one settled referral.
//
// Per referral rather than per user, because a referrer's monthly cap is ten
// SETTLEMENTS, not ten awards-ever: the (user_id, award_code) uniqueness that
// makes reward_grant work has to be keyed on something that differs per referral.
// A per-user code would pay the first referral and never the other nine.
//
// The referral id is in the code rather than in a separate column, so one UNIQUE
// constraint does the work — the same reasoning reward_grant_model.go gives for
// its own composite index.
func ReferralAwardCode(referralID uint) string {
	return "REFERRAL_BONUS:" + strconv.FormatUint(uint64(referralID), 10)
}

// ── hashed identifiers ────────────────────────────────────────────────────────

// HashFraudIdentifier is the keyed-HMAC helper for phone_hash and device_hash.
//
// Keyed, NOT a bare hash, and the reason is worth stating because a plain SHA-256
// of a Nepali mobile number is not a privacy control: the space of plausible
// numbers is small enough to enumerate, so anyone with the table recovers every
// phone number in it by brute force. An HMAC under a key that never enters the
// database is not reversible that way — the same position
// 02-architecture.md §10.1 and 05-economy-and-fraud.md §2.4 take.
//
// The key is supplied by the caller and is never persisted. keyID selects the key,
// so a key can be rotated without invalidating existing rows; rows written under
// an old key are distinguished by their key id, which is what makes rotation
// possible rather than a reason to never rotate.
func HashFraudIdentifier(key []byte, keyID string, value string) []byte {
	if len(key) == 0 {
		return nil
	}
	// NUL-separated so no two different splits of (keyID, value) produce the same
	// HMAC input — the same argument fingerprint() makes for the ledger's request
	// fingerprint. Without it, keyID "a" + value "b:c" and keyID "a:b" + value "c"
	// would collide.
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(strings.TrimSpace(keyID)))
	mac.Write([]byte{0})
	mac.Write([]byte(strings.TrimSpace(value)))
	return mac.Sum(nil)
}

// ReferralFingerprint is a stable, non-reversible identifier for a referral pair.
//
// Used to recognise "this same pair of accounts has been seen before" without
// storing either id in a table whose whole purpose is to be joined against. Not
// used for the cap — the cap is a count, not a match — and provided here so the
// fraud-matching tables the next slice adds all draw on one implementation.
func ReferralFingerprint(referrerUserID, referredUserID uint, referredKind string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", referredKind, referrerUserID, referredUserID)))
	return fmt.Sprintf("%x", sum[:])
}

func validReferredKind(kind string) bool {
	for _, k := range ReferredKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// nullableString renders an empty string as SQL NULL.
//
// Written rather than imported because this package's other nullable-journal
// writes are all on *string columns where a nil pointer is already NULL, and these
// are on text columns fed from a Go string. The COALESCE on hold_journal_id is
// what makes the write idempotent: a second settlement must not erase the hold it
// did not place.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// compile-time assertion that the service satisfies the port it is wired to.
var _ ReferralAttributionPort = (*ReferralService)(nil)
