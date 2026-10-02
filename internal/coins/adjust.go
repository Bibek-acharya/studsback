// internal/coins/adjust.go
//
// 03-api-contract.md §3.2: the signed correction.
//
// ── why this is not a "set balance" endpoint ──────────────────────────────────
//
// The endpoint that reads "the balance is now 1,000,000" is the endpoint whose abuse
// is undetectable. There is nothing in such a request to review: no cause, no
// direction, no history. If an operator's session is compromised, that is one
// request away from unlimited value.
//
// An adjustment MOVES the balance by a signed amount, carries a reason from a closed
// set, and names its author. It appears in a list of money movements with a stated
// cause, it is reversible in principle (reverse the journal), and it is subject to
// the same no-overdraft invariant as every other write.
//
// So there is no Bucket field on the request either, and that is deliberate: a
// correction belongs in the bucket the value came from. Letting a caller name it is
// how a goodwill credit lands in FREE — and FREE never expires — when it should
// behave like the earned value it replaces. adjust_test.go asserts the field's
// absence, because adding it is a one-line change nothing else would notice.
//
// ── which account the movement lands on ───────────────────────────────────────
//
// EARNED, always, and for the same reason: every correction concerns value that was
// earned or spent, and FREE has no server-side producer in this build at all. A
// correction into FREE would be the first, and it would be a permanent one.
//
// The consequence is honest and worth stating: a DEBIT that a student has already
// spent is refused by the no-overdraft CHECK rather than allowed to go negative. That
// is correct — the ledger does not lend — and it means the fix for "we wrongly took
// 50 coins they had already spent" is a goodwill CREDIT plus a note, not a negative
// balance. Overdrawing would make every balance in the system mean something weaker.
//
// ── idempotency ───────────────────────────────────────────────────────────────
//
// The caller supplies the key, per 03 §3.2. That is the opposite of the earn paths,
// which derive the key from the fact being recorded — and the difference is
// deliberate. An adjustment has no natural fact to key on (two corrections of the
// same size to the same user for the same reason are two real events), so the key is
// the caller's to supply and its uniqueness is theirs to manage. A replay returns the
// ORIGINAL journal with Replayed set.
//
// The fingerprint carries the user, the reason, the amount and the AUTHOR. Including
// the author is what makes a reused key with a different operator an
// ErrIdempotencyKeyReuse rather than a silent success — two operators colliding on
// one key must not have one of them told "already done" when their own request was
// never applied.
package coins

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AdjustRequest is a signed correction. There is no Bucket field, and no way to
// express "the balance is now X".
type AdjustRequest struct {
	UserID uint
	// Amount is SIGNED. Positive credits the student, negative debits them. A
	// magnitude with a separate direction field would allow the two to disagree.
	Amount int64
	// Reason must be one of AdjustmentReasons.
	Reason string
	// CreatedBy is "admin:<id>" per 03 §3.2. It is mandatory: a correction with no
	// author is unattributable, which is the thing this endpoint exists to prevent.
	CreatedBy string
	// IdempotencyKey is supplied by the CALLER, not derived. See the file header.
	IdempotencyKey string
	// Note is free text and is NOT a reason code. It exists so the closed-set reason
	// ("DUPLICATE_AWARD") can be accompanied by the specifics ("the profile award
	// paid twice on 2 Oct after a double-submit"). It is metadata and is never
	// grouped by.
	Note string
}

// AdjustResult is what a correction did, or what it had already done.
type AdjustResult struct {
	JournalID string    `json:"journal_id"`
	Amount    int64     `json:"amount"`
	Reason    string    `json:"reason"`
	CreatedBy string    `json:"created_by"`
	Available int64     `json:"available"`
	Replayed  bool      `json:"replayed"`
	AppliedAt time.Time `json:"applied_at"`
}

// Adjust moves a student's EARNED balance by a signed amount, in one journal.
//
// READ-ONLY REFUSALS first, all before the config load or the transaction:
// an unknown reason, a missing author, a zero amount, a zero user, a missing key.
// The order matters: a request with no reason is refused on THAT, not on whatever
// the next check happens to catch, because the reason is what a reviewer reads.
//
// The negative direction relies on the ledger's existing no-overdraft CHECK rather
// than on a clamp here. A clamped correction would tell the operator the balance was
// corrected when it was not, which is the one outcome worse than a refusal.
func (l *Ledger) Adjust(ctx context.Context, req AdjustRequest) (AdjustResult, error) {
	// REQUEST VALIDATION BEFORE THE WIRING CHECK, and that order is the point.
	//
	// These four tests build a &Ledger{} on purpose — it has no repository, so every
	// one of them would otherwise get ErrNoDatabase and pass for the wrong reason,
	// having asserted nothing about validation. A request with no reason must be
	// refused on THAT, whatever the deployment looks like: the reason is what a
	// reviewer reads, and "your database is misconfigured" is not an answer to "why
	// did you not tell me why you made this correction".
	//
	// A nil ledger still answers ErrNoDatabase, just after the request is known to
	// be well-formed. That order is invisible in production, where both are true, and
	// decisive in a test.
	if !IsAdjustmentReason(req.Reason) {
		return AdjustResult{}, fmt.Errorf("%w: %q is not an adjustment reason", ErrInvalidArgument, req.Reason)
	}
	if strings.TrimSpace(req.CreatedBy) == "" {
		return AdjustResult{}, fmt.Errorf("%w: an adjustment must name who made it", ErrInvalidArgument)
	}
	if req.Amount == 0 {
		return AdjustResult{}, fmt.Errorf("%w: an adjustment of zero writes a journal with no movement", ErrInvalidArgument)
	}
	if req.UserID == 0 {
		return AdjustResult{}, fmt.Errorf("%w: an adjustment needs a user id", ErrInvalidArgument)
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return AdjustResult{}, fmt.Errorf("%w: an adjustment needs an idempotency key", ErrInvalidArgument)
	}
	if l == nil || l.repo == nil {
		return AdjustResult{}, ErrNoDatabase
	}

	now := l.now().UTC()
	fp := fingerprint(
		EntryAdjust, req.Reason, strconv.FormatUint(uint64(req.UserID), 10),
		strconv.FormatInt(req.Amount, 10), req.CreatedBy,
	)

	var result AdjustResult
	err := l.repo.InUserTx(ctx, req.UserID, func(tx *TxContext) error {
		accountID, err := tx.EnsureUserAccount(req.UserID, BucketEarned)
		if err != nil {
			return err
		}
		faucet, err := tx.SystemAccountID(SystemEarnedFaucet)
		if err != nil {
			return err
		}

		metadata := map[string]any{
			"reason":     req.Reason,
			"amount":     req.Amount,
			"account_id": accountID,
			"bucket":     BucketEarned,
			"direction":  directionOf(req.Amount),
			"admin_note": strings.TrimSpace(req.Note),
			"adjustment": true,
		}
		journal := &CoinJournal{
			ID:                 uuid.NewString(),
			EntryType:          EntryAdjust,
			State:              StatePosted,
			Scope:              ScopeUser,
			IdempotencyKey:     req.IdempotencyKey,
			RequestFingerprint: fp,
			ReasonCode:         req.Reason,
			EffectiveAt:        now,
			CreatedAt:          now,
			CreatedBy:          req.CreatedBy,
			Metadata:           metadata,
		}
		created, err := tx.InsertJournal(journal)
		if err != nil {
			return err
		}
		if !created {
			return replayAdjust(tx, req, fp, &result)
		}

		// Two legs, and the faucet always takes the other side. A correction that
		// credited a student without debiting the faucet would be the platform
		// creating value with no journal entry anywhere else, which is precisely
		// what "no arbitrary minting" means.
		if err := tx.InsertPostings(journal.ID, []PostingLeg{
			{Seq: 1, AccountID: accountID, Amount: req.Amount},
			{Seq: 2, AccountID: faucet, Amount: -req.Amount},
		}); err != nil {
			return err
		}
		if err := tx.ApplyPosted(map[uint]int64{accountID: req.Amount, faucet: -req.Amount}, journal.ID); err != nil {
			return err
		}
		available, err := availableForUser(tx, req.UserID)
		if err != nil {
			return err
		}
		result = AdjustResult{
			JournalID: journal.ID, Amount: req.Amount, Reason: req.Reason,
			CreatedBy: req.CreatedBy, Available: available, AppliedAt: now,
		}
		return nil
	})
	if err != nil {
		return AdjustResult{}, err
	}
	return result, nil
}

// replayAdjust resolves a duplicate idempotency key.
//
// Same fingerprint: the original journal is the answer and NOTHING moves. The amount
// comes from the request rather than the journal, because the fingerprint has
// already established that they are the same value.
func replayAdjust(tx *TxContext, req AdjustRequest, fp []byte, out *AdjustResult) error {
	existing, err := tx.FindJournalByKey(ScopeUser, req.IdempotencyKey)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("%w: idempotency key %q conflicted but the journal could not be read back", ErrImmutable, req.IdempotencyKey)
	}
	if !bytes.Equal(existing.RequestFingerprint, fp) {
		return fmt.Errorf("%w: key %q was already used for a different adjustment", ErrIdempotencyKeyReuse, req.IdempotencyKey)
	}
	if existing.EntryType != EntryAdjust || existing.ReasonCode != req.Reason {
		return fmt.Errorf("%w: key %q belongs to a %s/%s journal", ErrIdempotencyKeyReuse, req.IdempotencyKey, existing.EntryType, existing.ReasonCode)
	}
	available, err := availableForUser(tx, req.UserID)
	if err != nil {
		return err
	}
	*out = AdjustResult{
		JournalID: existing.ID, Amount: req.Amount, Reason: req.Reason,
		CreatedBy: existing.CreatedBy, Available: available,
		Replayed: true, AppliedAt: existing.CreatedAt,
	}
	return nil
}

// directionOf is the human word for a sign, for the journal metadata and the
// support view. A negative amount stored as a number is correct; it is unreadable
// in a log without this.
func directionOf(amount int64) string {
	if amount > 0 {
		return "credit"
	}
	return "debit"
}

// IsAdjustmentReason reports whether a reason code is one an operator may choose.
//
// Exported because the HTTP layer and any future dashboard must validate against the
// same set rather than re-listing it, and because a test in another package may want
// to assert the vocabulary.
func IsAdjustmentReason(reason string) bool {
	for _, known := range AdjustmentReasons {
		if reason == known {
			return true
		}
	}
	return false
}
