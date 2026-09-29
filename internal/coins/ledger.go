// internal/coins/ledger.go
//
// The transactional core: Grant, Spend, Reverse, Reserve, ReleaseReserved.
//
// ── what this file is responsible for ────────────────────────────────────────
//
// Deciding. repository.go holds every SQL statement; this file holds every
// decision about what should happen. The split is the one 02-architecture.md §8
// describes as "an internal decomposition of the repository layer, not a new
// architectural layer".
//
// ── amounts are never accepted from a caller ─────────────────────────────────
//
// 02-architecture.md §12.1: "Never trust a client-supplied amount. The client
// sends {"event":"referral_qualified"}; the server resolves the coin value from
// reason_code against its own config. A client that can say what a coin is worth
// is an exploit."
//
// That rule is enforced by the signatures below rather than by convention. Not
// one exported method takes an amount. A grant names a reason code and the size
// comes from EconomyConfig.Awards; a spend names a reason code and a class and
// the size comes from EconomyConfig.Prices; a hold names an event and the size
// comes from EconomyConfig.Referral. Adding an Amount field to any of these
// structs would be the exploit, and the comment on each one says so.
//
// A caller may not choose the bucket either. grantSpecs resolves it, because the
// bucket chooses the expiry policy: a caller that could name FREE would be
// choosing a 30-day expiry over a 365-day one, which is a pricing decision.
//
// ── idempotency ──────────────────────────────────────────────────────────────
//
// Every money-moving method follows one shape, and it is the shape in
// 04-implementation-plan.md §3.2:
//
//	BEGIN
//	SET LOCAL lock_timeout, idle_in_transaction_session_timeout
//	pg_advisory_xact_lock('coin:user:' || user)
//	INSERT INTO coin_journal ... ON CONFLICT (scope, idempotency_key) DO NOTHING RETURNING id
//	  -- no row => replay: compare fingerprint, return the original or 409
//	... lots, postings, balances ...
//	COMMIT
//
// The unique index does the work. There is no SELECT-then-INSERT anywhere in
// this file, and there must not be one.
//
// ── what is not here ─────────────────────────────────────────────────────────
//
// Entitlements (resource_unlock), awards (reward_grant) and referrals
// (user_referral) are later phases. The seam is deliberate and documented at
// both ends: this file can place and lift a hold, and the referral state machine
// that decides which happens is not in it.
package coins

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ── reason codes ─────────────────────────────────────────────────────────────
//
// The business event a journal records. They are the ledger's vocabulary, not
// the caller's: an unknown code is ErrInvalidArgument rather than a free-text
// reason, because a reason_code the server does not recognise cannot have had an
// amount resolved for it.
const (
	// ReasonProfileComplete pays ONE instalment of the profile award. The award
	// ladder (5 x 5 = 25) is validated on every config write, and paying it in
	// separate journals means each instalment has its own idempotency key and a
	// mid-ladder failure is resumable rather than all-or-nothing.
	ReasonProfileComplete = "PROFILE_COMPLETE"
	// ReasonReferralQualified converts a qualified referral into coins. The hold
	// it was funded from is lifted separately, by the referral phase.
	ReasonReferralQualified = "REFERRAL_QUALIFIED"
	// ReasonResourceApproved pays an uploader when their resource is published.
	ReasonResourceApproved = "RESOURCE_APPROVED"
	// ReasonResourceUnlock is the single write path for spending. The class
	// (ref_type) selects the price.
	ReasonResourceUnlock = "RESOURCE_UNLOCK"
	// ReasonReferralHold is the posting-free journal that records an outstanding
	// hold. See Reserve.
	ReasonReferralHold = "REFERRAL_HOLD"
	// ReasonGrantReversal is the audit reason on a clawback.
	ReasonGrantReversal = "GRANT_REVERSAL"
)

// Reference types. These are the ref_type values, and they double as the spend
// class names from 03-api-contract.md §2.3.
const (
	RefStudyResource = "study_resource"
	RefVideo         = "video"
	RefMockTest      = "mock_test"
	RefUserReferral  = "user_referral"
)

// SpendableBuckets is the set of buckets a spend may draw on. PURCHASED is
// absent: decision D1 forbids buying coins, and the value is still permitted by
// the CHECK constraints, so revisiting D1 is a one-line change here and nowhere
// else.
var SpendableBuckets = []string{BucketFree, BucketEarned}

// grantSpec is the server's answer to "what is this event worth, in which bucket
// does it land, and how long does it live".
type grantSpec struct {
	amount func(EconomyConfig) int64
	bucket string
	// expiryDays is the lot lifetime, always from config. Zero would mean
	// "never expires", which no configured bucket wants and which Grant refuses
	// rather than creating a permanent lot by accident.
	expiryDays func(EconomyConfig) int64
}

// grantSpecs is the whole economy as far as this package is concerned. One map,
// so "what can be granted, for how much, expiring when" is a single read rather
// than a switch with a default that silently grants something.
//
// Every configured award is an earned reward, so every grant lands in EARNED.
// That means this phase's grants do not create FREE lots, and
// EconomyConfig.Expiry.FreeDays is consequently unused here. FREE accounts are
// still created on first spend and are still spendable, so the multi-bucket FEFO
// path is live — it just has no server-side producer yet. Whoever adds the first
// FREE-granting event adds one line here and changes nothing else.
var grantSpecs = map[string]grantSpec{
	ReasonProfileComplete: {
		amount:     func(c EconomyConfig) int64 { return c.Awards.ProfileInstalment },
		bucket:     BucketEarned,
		expiryDays: func(c EconomyConfig) int64 { return c.Expiry.EarnedDays },
	},
	ReasonReferralQualified: {
		amount:     func(c EconomyConfig) int64 { return c.Awards.ReferralReferrer },
		bucket:     BucketEarned,
		expiryDays: func(c EconomyConfig) int64 { return c.Expiry.EarnedDays },
	},
	ReasonResourceApproved: {
		amount:     func(c EconomyConfig) int64 { return c.Awards.ResourceApproved },
		bucket:     BucketEarned,
		expiryDays: func(c EconomyConfig) int64 { return c.Expiry.EarnedDays },
	},
}

// holdReasons are the reason codes whose journals carry no postings.
//
// A hold moves no coins, so it has nothing to post; but it must still be a
// journal, because it has to be idempotent per referral, it has to record who
// placed it and when it is expected to resolve, and a release has to be able to
// find the exact amount to un-hold. The state machine is the one the schema
// already provides: PENDING while outstanding, REVERSED once resolved.
//
// They are enumerated rather than derived so the invariant test can assert that a
// journal with no postings is always one of these. Without that, "every journal
// nets to zero" silently becomes "every journal that has postings nets to zero",
// which is a much weaker claim than it reads.
var holdReasons = map[string]struct{}{
	ReasonReferralHold: {},
}

// IsHoldReason reports whether a reason code is one of the posting-free holds.
func IsHoldReason(reasonCode string) bool {
	_, ok := holdReasons[reasonCode]
	return ok
}

// Ledger is the transactional core. It holds a repository and the economy
// config, and nothing else: no cache, no user state, and a clock that is
// injectable so a test can pin "now" for the expiry boundary.
type Ledger struct {
	repo   *Repository
	config *ConfigStore
	now    func() time.Time
}

// NewLedger wires the ledger to its repository and config store.
func NewLedger(repo *Repository, config *ConfigStore) *Ledger {
	return &Ledger{repo: repo, config: config, now: func() time.Time { return time.Now().UTC() }}
}

// ── requests and results ─────────────────────────────────────────────────────
//
// These are not DTOs. dto.go owns the wire contract; these are Go types with no
// json tags that happen to be returned by a method. The result types exist
// because the handler will need balance_after and spent_from, and because a
// caller forced to re-read the balance to learn what it just did is a caller
// that gets the answer wrong under concurrency.

// GrantRequest asks for the value of a business event to be granted. It has no
// amount field. See the file header.
type GrantRequest struct {
	UserID         uint
	ReasonCode     string
	IdempotencyKey string
	// RefType and RefID name the thing that earned the award. A RefType requires
	// a RefID: CHECK (ref_type IS NULL OR ref_id IS NOT NULL).
	RefType   *string
	RefID     *uint64
	CreatedBy string
	Metadata  map[string]any
}

// GrantResult is what a grant did, or what it had already done.
type GrantResult struct {
	JournalID string
	// Amount and Bucket are the values the ORIGINAL grant used, read back from
	// the journal's metadata on a replay. Recomputing them from the current
	// config would report today's price for a movement made at yesterday's.
	Amount      int64
	Bucket      string
	LotID       uint
	ExpiresAt   *time.Time
	Available   int64
	Replayed    bool
	JournalTime time.Time
}

// SpendRequest asks for a spend. It has no amount field: the price comes from
// EconomyConfig.Prices, selected by ReasonCode and RefType.
type SpendRequest struct {
	UserID         uint
	ReasonCode     string
	IdempotencyKey string
	// RefType is the class being bought (study_resource | video | mock_test) and
	// is REQUIRED for ReasonResourceUnlock: the class is what the price is keyed
	// on. It is not an amount, and it cannot be turned into one.
	RefType   string
	RefID     *uint64
	CreatedBy string
	Metadata  map[string]any
}

// SpentFrom is one lot a spend burned, for the response body's spent_from and for
// the "why is my balance going down" support question.
type SpentFrom struct {
	LotID     uint
	AccountID uint
	Bucket    string
	Amount    int64
	ExpiresAt *time.Time
}

// SpendResult is what a spend did, or what it had already done.
type SpendResult struct {
	JournalID string
	Amount    int64
	SpentFrom []SpentFrom
	// Available is posted - reserved AFTER the spend, across every bucket.
	Available int64
	// Required and AvailableBefore are the price and the balance the spend
	// decided on, for a caller that has to explain what it just did.
	//
	// A SpendResult only ever describes a spend that HAPPENED. On refusal the
	// result is the zero value and the same two figures travel out on the typed
	// error instead — ErrInsufficient — because the 402 body in
	// 03-api-contract.md §2.3 is built after the transaction has rolled back, and
	// a result that is empty exactly when the handler needs it is a result the
	// handler has to work around with a second read.
	Required        int64
	AvailableBefore int64
	Replayed        bool
	JournalTime     time.Time
}

// ReverseRequest claws a journal back. It has no amount: a reversal negates the
// whole original, because a partial reversal of a grant is an adjustment, not a
// reversal, and 03-api-contract.md §3.2 makes adjustments a separate entry type
// with its own endpoint and its own recorded reason.
type ReverseRequest struct {
	// JournalID is the original to reverse. Its user is resolved from its
	// postings, because a journal does not carry an owner column and the lock has
	// to be taken on the right user before anything is read for a decision.
	JournalID string
	// ReasonCode is mandatory. A clawback without a recorded reason is the thing
	// the ledger exists to make impossible.
	ReasonCode     string
	IdempotencyKey string
	CreatedBy      string
	Metadata       map[string]any
}

// ReverseResult is what a reversal did, or what it had already done.
type ReverseResult struct {
	JournalID      string
	ReversesID     string
	Amount         int64
	Available      int64
	LotsRemoved    int
	LotsShrunk     int
	Replayed       bool
	JournalTime    time.Time
	OriginalReason string
}

// ReserveRequest places a hold on coins that are promised but not yet granted.
// No amount: it is the configured value of the event being held.
type ReserveRequest struct {
	UserID         uint
	ReasonCode     string
	IdempotencyKey string
	// RefType and RefID identify the held event, and both are required: a hold
	// with no reference could not be released later, because a release finds the
	// hold by (reason, ref).
	RefType   string
	RefID     uint64
	CreatedBy string
	Metadata  map[string]any
}

// ReserveResult is the outstanding hold.
type ReserveResult struct {
	JournalID string
	Amount    int64
	// AccountID is the bucket the hold sits on, which is the account a release
	// takes it off.
	AccountID uint
	Available int64
	// HoldUntil is when the hold is expected to resolve. It is advisory: nothing
	// sweeps it, and the referral phase decides what a lapsed hold means.
	HoldUntil   time.Time
	Replayed    bool
	JournalTime time.Time
}

// ReleaseReservedRequest lifts a hold. It is identified by the hold it releases,
// not by a key of its own; see ReleaseReserved for why.
type ReleaseReservedRequest struct {
	UserID     uint
	ReasonCode string
	RefType    string
	RefID      uint64
	CreatedBy  string
	Metadata   map[string]any
}

// ReleaseResult is the lifted hold.
type ReleaseResult struct {
	// JournalID is the HOLD's journal. A release is not a new journal: it is the
	// one mutation the journal trigger permits, applied to the hold itself.
	JournalID  string
	Amount     int64
	AccountID  uint
	Available  int64
	Replayed   bool
	ReleasedAt time.Time
}

// ── Grant ────────────────────────────────────────────────────────────────────

// Grant issues the configured value of a business event and opens a lot for it.
//
// The shape, and why each step is where it is:
//
//  1. validate and resolve, BEFORE any transaction. An unknown reason code, a
//     zero award or a missing key must not open a transaction, and must not take
//     a user's advisory lock to find out.
//  2. load the config before the transaction, not inside it. The config store may
//     issue its own query on a different connection while this one holds a user
//     lock, and holding a lock across a round trip to another module's table is
//     how a 3-second lock_timeout starts firing in production. The cache makes
//     this free on the hot path.
//  3. inside the transaction: the advisory lock, the journal insert, the
//     accounts, the postings, the cached balance, and the lot.
//
// A grant debits earned_faucet. Coins the platform has issued are a liability of
// the platform, so the faucet goes negative, and the global SUM(posted_balance)
// stays at zero.
func (l *Ledger) Grant(ctx context.Context, req GrantRequest) (GrantResult, error) {
	if err := validateUser(req.UserID); err != nil {
		return GrantResult{}, err
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return GrantResult{}, err
	}
	spec, ok := grantSpecs[req.ReasonCode]
	if !ok {
		return GrantResult{}, fmt.Errorf("%w: %q is not a grant this ledger can make", ErrInvalidArgument, req.ReasonCode)
	}
	if err := validateRef(req.RefType, req.RefID); err != nil {
		return GrantResult{}, err
	}
	if l.config == nil {
		return GrantResult{}, ErrNoDatabase
	}
	cfg, err := l.config.Load()
	if err != nil {
		return GrantResult{}, err
	}
	amount := spec.amount(cfg)
	if amount <= 0 {
		// A configured award of zero is a misconfiguration, not a free grant.
		// Honouring it would write a zero posting, which CHECK (amount <> 0)
		// rejects anyway.
		return GrantResult{}, fmt.Errorf("%w: reason %s is configured to award %d coins", ErrInvalidArgument, req.ReasonCode, amount)
	}
	days := spec.expiryDays(cfg)
	if days <= 0 {
		return GrantResult{}, fmt.Errorf("%w: the expiry for bucket %s is configured to %d days", ErrInvalidArgument, spec.bucket, days)
	}

	now := l.now().UTC()
	expiresAt := now.AddDate(0, 0, int(days))
	fp := fingerprint(
		EntryGrant, strconv.FormatUint(uint64(req.UserID), 10), req.ReasonCode,
		refFingerprint(req.RefType, req.RefID), strconv.FormatInt(amount, 10),
	)

	var result GrantResult
	err = l.repo.InUserTx(ctx, req.UserID, func(tx *TxContext) error {
		journal := &CoinJournal{
			ID:                 uuid.NewString(),
			EntryType:          EntryGrant,
			State:              StatePosted,
			Scope:              ScopeUser,
			IdempotencyKey:     req.IdempotencyKey,
			RequestFingerprint: fp,
			ReasonCode:         req.ReasonCode,
			RefType:            req.RefType,
			RefID:              req.RefID,
			EffectiveAt:        now,
			CreatedAt:          now,
			CreatedBy:          req.CreatedBy,
			Metadata:           grantMetadata(req.Metadata, amount, spec.bucket, &expiresAt),
		}
		created, err := tx.InsertJournal(journal)
		if err != nil {
			return err
		}
		if !created {
			return replayGrant(tx, req, fp, &result)
		}

		userAccount, err := tx.EnsureUserAccount(req.UserID, spec.bucket)
		if err != nil {
			return err
		}
		faucet, err := tx.SystemAccountID(SystemEarnedFaucet)
		if err != nil {
			return err
		}
		// Two legs: the user up, the faucet down by the same amount. Nets to zero
		// by construction, and InsertPostings refuses it if it does not.
		if err := tx.InsertPostings(journal.ID, []PostingLeg{
			{Seq: 1, AccountID: userAccount, Amount: amount},
			{Seq: 2, AccountID: faucet, Amount: -amount},
		}); err != nil {
			return err
		}
		if err := tx.ApplyPosted(map[uint]int64{userAccount: amount, faucet: -amount}, journal.ID); err != nil {
			return err
		}

		// The lot is created after the postings, so there is never a lot for a
		// journal that did not post.
		lot := &CoinLot{
			AccountID: userAccount,
			JournalID: journal.ID,
			Bucket:    spec.bucket,
			Granted:   amount,
			ExpiresAt: &expiresAt,
			CreatedAt: now,
		}
		if err := tx.CreateLot(lot); err != nil {
			return err
		}

		available, err := availableForUser(tx, req.UserID)
		if err != nil {
			return err
		}
		result = GrantResult{
			JournalID:   journal.ID,
			Amount:      amount,
			Bucket:      spec.bucket,
			LotID:       lot.ID,
			ExpiresAt:   &expiresAt,
			Available:   available,
			JournalTime: now,
		}
		return nil
	})
	if err != nil {
		return GrantResult{}, err
	}
	return result, nil
}

// replayGrant resolves a duplicate idempotency key.
//
// Same fingerprint: the original journal is the answer and NOTHING moves. The
// amounts come from the journal's own metadata, not from a recomputation, because
// the config that produced them may have changed since.
//
// Different fingerprint: ErrIdempotencyKeyReuse. 02-architecture.md §12.2 — "A
// reused key with a different payload is an error, not a replay. Compare
// request_fingerprint and reject on mismatch, so a key collision cannot silently
// return the wrong result." The fingerprint carries the user id, so a key reused
// across two users lands here rather than returning one user's journal to
// another.
func replayGrant(tx *TxContext, req GrantRequest, fp []byte, out *GrantResult) error {
	existing, err := tx.FindJournalByKey(ScopeUser, req.IdempotencyKey)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("%w: idempotency key %q conflicted but the journal could not be read back", ErrImmutable, req.IdempotencyKey)
	}
	if !bytes.Equal(existing.RequestFingerprint, fp) {
		return fmt.Errorf("%w: key %q was already used for a different request", ErrIdempotencyKeyReuse, req.IdempotencyKey)
	}
	if existing.EntryType != EntryGrant || existing.ReasonCode != req.ReasonCode {
		return fmt.Errorf("%w: key %q belongs to a %s/%s journal", ErrIdempotencyKeyReuse, req.IdempotencyKey, existing.EntryType, existing.ReasonCode)
	}

	amount, bucket, err := replayGrantMetadata(existing)
	if err != nil {
		return err
	}
	available, err := availableForUser(tx, req.UserID)
	if err != nil {
		return err
	}
	*out = GrantResult{
		JournalID:   existing.ID,
		Amount:      amount,
		Bucket:      bucket,
		Available:   available,
		Replayed:    true,
		JournalTime: existing.CreatedAt,
	}
	return nil
}

// ── Spend ────────────────────────────────────────────────────────────────────

// Spend charges the configured price of a class and burns the soonest-expiring
// coins first.
//
// Three gates, in this order, and the order is the design:
//
//  1. the account is not frozen;
//  2. posted - reserved covers the price, summed across every bucket. The
//     reserved term is what makes a hold a hold: a user with 200 coins of which
//     150 are promised to a pending referral has 50 available, and a 60-coin
//     spend is refused;
//  3. the FEFO allocation actually covers the price from open, unexpired lots.
//
// Gate 2 is the cached projection and gate 3 is the source of truth, and the gap
// between them is exactly what a nightly reconciliation is for. Both run because
// gate 2 is what the no-overdraft CHECK enforces and gate 3 is what keeps the
// lots honest; neither alone is sufficient.
//
// A refusal returns the zero SpendResult and a typed *InsufficientError carrying
// the two figures gate 2 compared. See SpendResult for why the figures are on
// the error rather than in the result.
//
// The postings are built from the ALLOCATION's per-lot increments, one leg per
// lot carrying its lot_id, plus one aggregate leg on redeemed_sink. Writing
// -taken to each lot instead is the bug in 02-architecture.md §4, and it is why
// postingSpend is a named function with a test rather than a loop at the call
// site.
func (l *Ledger) Spend(ctx context.Context, req SpendRequest) (SpendResult, error) {
	if err := validateUser(req.UserID); err != nil {
		return SpendResult{}, err
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return SpendResult{}, err
	}
	refID := req.RefID
	if err := validateRef(&req.RefType, refID); err != nil {
		return SpendResult{}, err
	}
	if l.config == nil {
		return SpendResult{}, ErrNoDatabase
	}
	cfg, err := l.config.Load()
	if err != nil {
		return SpendResult{}, err
	}
	price, err := spendPrice(cfg, req.ReasonCode, req.RefType)
	if err != nil {
		return SpendResult{}, err
	}
	if price <= 0 {
		return SpendResult{}, fmt.Errorf("%w: %s is configured to cost %d coins", ErrInvalidArgument, req.RefType, price)
	}

	now := l.now().UTC()
	fp := fingerprint(
		EntrySpend, strconv.FormatUint(uint64(req.UserID), 10), req.ReasonCode,
		refFingerprint(&req.RefType, refID), strconv.FormatInt(price, 10),
	)

	var result SpendResult
	err = l.repo.InUserTx(ctx, req.UserID, func(tx *TxContext) error {
		journal := &CoinJournal{
			ID:                 uuid.NewString(),
			EntryType:          EntrySpend,
			State:              StatePosted,
			Scope:              ScopeUser,
			IdempotencyKey:     req.IdempotencyKey,
			RequestFingerprint: fp,
			ReasonCode:         req.ReasonCode,
			RefType:            &req.RefType,
			RefID:              refID,
			EffectiveAt:        now,
			CreatedAt:          now,
			CreatedBy:          req.CreatedBy,
			Metadata:           spendMetadata(req.Metadata, price, req.RefType),
		}
		created, err := tx.InsertJournal(journal)
		if err != nil {
			return err
		}
		if !created {
			return replaySpend(tx, req, fp, &result)
		}

		accountIDs, err := ensureSpendAccounts(tx, req.UserID)
		if err != nil {
			return err
		}
		balances, err := tx.Balances(accountIDs)
		if err != nil {
			return err
		}
		if frozen, id := frozenAccounts(balances); frozen {
			// Checked before the balance so a frozen account reports 423 rather
			// than a 402 that invites the student to go and earn coins.
			return fmt.Errorf("%w: account %d is closed", ErrAccountFrozen, id)
		}
		availableBefore := sumAvailable(balances)
		if availableBefore < price {
			// Returned before a single lot or posting is touched, and the journal
			// rolls back with it. A refused spend leaves ZERO rows, which is the
			// property the concurrency test asserts.
			//
			// The two figures go out ON THE ERROR, not on the result. The 402 body
			// is rendered after this transaction has rolled back, and a zero
			// SpendResult is what a failed Spend returns — so before this was typed,
			// the handler had to re-read the wallet to recover numbers it already
			// had. Unwraps to ErrInsufficientCoins, so nothing downstream changed.
			return ErrInsufficient(price, availableBefore)
		}

		lots, err := tx.OpenLots(accountIDs, now)
		if err != nil {
			return err
		}
		// The allocator owns the FEFO decision. It returns an error rather than a
		// partial allocation, so there is no code path on which a shortfall burns
		// fewer coins than were asked for.
		alloc, err := AllocateFEFO(lots, price, now)
		if err != nil {
			return err
		}
		byID := indexLots(lots)
		sink, err := tx.SystemAccountID(SystemRedeemedSink)
		if err != nil {
			return err
		}
		legs, spentFrom := postingSpend(alloc, byID, sink)
		if err := tx.InsertPostings(journal.ID, legs); err != nil {
			return err
		}
		if err := applySpendDeltas(tx, alloc, byID, sink, journal.ID); err != nil {
			return err
		}

		availableAfter, err := availableForUser(tx, req.UserID)
		if err != nil {
			return err
		}
		result = SpendResult{
			JournalID:       journal.ID,
			Amount:          price,
			SpentFrom:       spentFrom,
			Available:       availableAfter,
			Required:        price,
			AvailableBefore: availableBefore,
			JournalTime:     now,
		}
		return nil
	})
	if err != nil {
		// The zero result beside the error is the point: a SpendResult describes a
		// spend, and describing one that did not happen is how a caller ends up
		// quoting a journal id for a journal that was rolled back. Everything a
		// refusal has to say is on the error.
		return SpendResult{}, err
	}
	return result, nil
}

// postingSpend builds the legs of a spend from the allocation.
//
// It is a pure function of the allocation and the lot table, which is what makes
// the per-lot increment a structural fact rather than a comment: the only Amount
// that ever reaches a user leg is `c.Amount`, the per-lot increment. The
// aggregate appears exactly once, on the final leg against the sink.
//
// Getting this wrong is silent. If the user legs were written as -alloc.Taken the
// postings would still sum to zero (the sink leg is the same total), the cached
// balance would still match, and only the lot rows would disagree — which is
// precisely the failure in 02-architecture.md §4.
func postingSpend(alloc Allocation, lots map[uint]LotBalance, sink uint) ([]PostingLeg, []SpentFrom) {
	legs := make([]PostingLeg, 0, len(alloc.Consumptions)+1)
	spent := make([]SpentFrom, 0, len(alloc.Consumptions))
	for i, c := range alloc.Consumptions {
		lot := lots[c.LotID]
		lotID := c.LotID
		legs = append(legs, PostingLeg{
			Seq:       int16(i + 1),
			AccountID: lot.AccountID,
			Amount:    -c.Amount,
			LotID:     &lotID,
		})
		spent = append(spent, SpentFrom{
			LotID:     c.LotID,
			AccountID: lot.AccountID,
			Bucket:    lot.Bucket,
			Amount:    c.Amount,
			ExpiresAt: lot.ExpiresAt,
		})
	}
	legs = append(legs, PostingLeg{
		Seq:       int16(len(alloc.Consumptions) + 1),
		AccountID: sink,
		Amount:    alloc.Total(),
	})
	return legs, spent
}

// applySpendDeltas burns the lots and moves the cached projection: every burned
// lot's account down by its own increment, the sink up by the total.
//
// The lots are consumed before the cache is touched, so a failure in between
// cannot leave the cache moved and the lot un-consumed. Both are inside the one
// transaction either way, so the ordering is about which error message you get,
// not about correctness.
func applySpendDeltas(tx *TxContext, alloc Allocation, lots map[uint]LotBalance, sink uint, journalID string) error {
	posted := make(map[uint]int64, len(alloc.Consumptions)+1)
	for _, c := range alloc.Consumptions {
		if err := tx.ConsumeLot(c.LotID, c.Amount); err != nil {
			return err
		}
		posted[lots[c.LotID].AccountID] -= c.Amount
	}
	posted[sink] += alloc.Total()
	return tx.ApplyPosted(posted, journalID)
}

func indexLots(lots []LotBalance) map[uint]LotBalance {
	byID := make(map[uint]LotBalance, len(lots))
	for _, l := range lots {
		byID[l.ID] = l
	}
	return byID
}

// ensureSpendAccounts returns the ids of every account a spend may draw on,
// creating them on first use. A user who has never been granted anything gets
// both, so the FEFO query has a stable account set and the insufficiency check
// has something to read.
func ensureSpendAccounts(tx *TxContext, userID uint) ([]uint, error) {
	ids := make([]uint, 0, len(SpendableBuckets))
	for _, bucket := range SpendableBuckets {
		id, err := tx.EnsureUserAccount(userID, bucket)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// replaySpend resolves a duplicate idempotency key for a spend. Same rule as
// replayGrant: identical fingerprint replays, anything else is a conflict, and
// the reported amount and spent_from come from the original journal.
func replaySpend(tx *TxContext, req SpendRequest, fp []byte, out *SpendResult) error {
	existing, err := tx.FindJournalByKey(ScopeUser, req.IdempotencyKey)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("%w: idempotency key %q conflicted but the journal could not be read back", ErrImmutable, req.IdempotencyKey)
	}
	if !bytes.Equal(existing.RequestFingerprint, fp) {
		return fmt.Errorf("%w: key %q was already used for a different request", ErrIdempotencyKeyReuse, req.IdempotencyKey)
	}
	if existing.EntryType != EntrySpend {
		return fmt.Errorf("%w: key %q belongs to a %s journal", ErrIdempotencyKeyReuse, req.IdempotencyKey, existing.EntryType)
	}

	amount, err := replayAmount(existing)
	if err != nil {
		return err
	}
	postings, err := tx.JournalPostings(existing.ID)
	if err != nil {
		return err
	}
	spent, err := spentFromPostings(tx, postings)
	if err != nil {
		return err
	}
	available, err := availableForUser(tx, req.UserID)
	if err != nil {
		return err
	}
	*out = SpendResult{
		JournalID:   existing.ID,
		Amount:      amount,
		SpentFrom:   spent,
		Available:   available,
		Required:    amount,
		Replayed:    true,
		JournalTime: existing.CreatedAt,
	}
	return nil
}

// spentFromPostings reconstructs spent_from from a spend's own legs: every
// negative user leg is one burned lot, and its amount is the per-lot increment
// recorded at the time. It is why a replay reports what actually happened rather
// than what a fresh FEFO pass would have chosen.
func spentFromPostings(tx *TxContext, postings []CoinPosting) ([]SpentFrom, error) {
	spent := make([]SpentFrom, 0, len(postings))
	for _, p := range postings {
		if p.Amount >= 0 || p.LotID == nil {
			continue
		}
		lot, err := tx.LotByID(*p.LotID)
		if err != nil {
			return nil, err
		}
		if lot == nil {
			continue
		}
		spent = append(spent, SpentFrom{
			LotID:     lot.ID,
			AccountID: lot.AccountID,
			Bucket:    lot.Bucket,
			Amount:    -p.Amount,
			ExpiresAt: lot.ExpiresAt,
		})
	}
	return spent, nil
}

// ── Reverse ──────────────────────────────────────────────────────────────────

// Reverse claws a posted grant back by writing its negation.
//
// The original is never touched. POSTED is terminal and the trigger freezes
// provenance anyway, so "reversed" is the existence of a REVERSAL journal
// pointing at it — which is what makes "was this already refunded?" answerable a
// second time, the reason a balance column cannot work (02-architecture.md §1).
//
// The reversal posts the negation of every original leg, so the reversal journal
// nets to zero on its own terms and the pair nets to nothing at all. The lot the
// grant opened is shrunk by the same amount, floored at its already-consumed
// part, so the coins taken back can never be spent again.
//
// A clawback that lands on coins the user has already spent still reduces the
// balance, and the no-overdraft CHECK then refuses it. That is the correct
// answer — you cannot take back coins the user no longer has — and it arrives as
// a constraint error rather than as a silently negative balance.
//
// Reversing a SPEND is not supported and is not a refund. A refund has to
// re-credit coins, which means creating new lots, which is a grant. It is
// Grant(ReasonX) plus a link to the spend, and it belongs in the entitlement
// slice where a revoked unlock can be re-bought. This method refuses a journal
// whose user legs are negative, with a message that says so.
func (l *Ledger) Reverse(ctx context.Context, req ReverseRequest) (ReverseResult, error) {
	if strings.TrimSpace(req.JournalID) == "" {
		return ReverseResult{}, fmt.Errorf("%w: a reversal needs the journal it reverses", ErrInvalidArgument)
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return ReverseResult{}, err
	}
	if strings.TrimSpace(req.ReasonCode) == "" {
		return ReverseResult{}, fmt.Errorf("%w: a reversal needs a recorded reason", ErrInvalidArgument)
	}
	if IsHoldReason(req.ReasonCode) {
		return ReverseResult{}, fmt.Errorf("%w: a hold is released, not reversed", ErrInvalidArgument)
	}
	if l.repo == nil || l.repo.db == nil {
		return ReverseResult{}, ErrNoDatabase
	}

	// The owner has to be known before the lock, because the lock is per user and
	// a journal does not carry an owner column. Read outside the transaction: the
	// join is over immutable rows (a posting is append-only, and an account's
	// owner never changes), so nothing that happens after this read can change
	// the answer. The journal is re-read, authoritatively, inside the
	// transaction.
	owner, err := l.repo.JournalOwnerUserID(req.JournalID)
	if err != nil {
		return ReverseResult{}, err
	}

	now := l.now().UTC()
	var result ReverseResult
	err = l.repo.InUserTx(ctx, owner, func(tx *TxContext) error {
		original, err := tx.FindJournal(req.JournalID)
		if err != nil {
			return err
		}
		if original == nil {
			return fmt.Errorf("%w: no journal %s", ErrNotFound, req.JournalID)
		}
		if original.State != StatePosted {
			return fmt.Errorf("%w: journal %s is %s, not POSTED", ErrImmutable, req.JournalID, original.State)
		}
		if IsHoldReason(original.ReasonCode) {
			return fmt.Errorf("%w: journal %s is a hold and is released, not reversed", ErrInvalidArgument, req.JournalID)
		}
		if original.ReversalOf != nil {
			return fmt.Errorf("%w: journal %s is itself a reversal", ErrImmutable, req.JournalID)
		}
		reversals, err := tx.CountReversalsOf(original.ID)
		if err != nil {
			return err
		}
		if reversals > 0 {
			return fmt.Errorf("%w: journal %s has already been reversed", ErrImmutable, original.ID)
		}

		postings, err := tx.JournalPostings(original.ID)
		if err != nil {
			return err
		}
		if len(postings) == 0 {
			return fmt.Errorf("%w: journal %s has no postings to negate", ErrImmutable, original.ID)
		}
		userAccounts, err := tx.UserAccountIDs(owner)
		if err != nil {
			return err
		}
		amount := userLegTotal(postings, userAccounts)
		if amount <= 0 {
			return fmt.Errorf("%w: journal %s moved no coins to a user; clawback applies to a grant, and a spend is refunded by re-granting", ErrInvalidArgument, original.ID)
		}
		fp := fingerprint(
			EntryReversal, strconv.FormatUint(uint64(owner), 10), original.ID,
			req.ReasonCode, strconv.FormatInt(amount, 10),
		)

		journal := &CoinJournal{
			ID:                 uuid.NewString(),
			EntryType:          EntryReversal,
			State:              StatePosted,
			Scope:              ScopeUser,
			IdempotencyKey:     req.IdempotencyKey,
			RequestFingerprint: fp,
			ReasonCode:         req.ReasonCode,
			ReversalOf:         &original.ID,
			EffectiveAt:        now,
			CreatedAt:          now,
			CreatedBy:          req.CreatedBy,
			Metadata: map[string]any{
				"amount":           amount,
				"reversal_reason":  req.ReasonCode,
				"reverses_journal": original.ID,
				"reverses_reason":  original.ReasonCode,
				"event":            req.Metadata,
			},
		}
		created, err := tx.InsertJournal(journal)
		if err != nil {
			return err
		}
		if !created {
			existing, err := tx.FindJournalByKey(ScopeUser, req.IdempotencyKey)
			if err != nil {
				return err
			}
			return replayReversal(tx, original, fp, existing, &result)
		}

		// Leg for leg, exactly negated. lot_id is deliberately NOT copied: a
		// grant's own postings carry none (the lot is created after them), and
		// the clawback may delete that lot outright, so a reversal leg pointing
		// at it would be a dangling reference in the append-only table.
		legs := make([]PostingLeg, 0, len(postings))
		for i, p := range postings {
			legs = append(legs, PostingLeg{
				Seq:       int16(i + 1),
				AccountID: p.AccountID,
				Amount:    -p.Amount,
			})
		}
		if err := tx.InsertPostings(journal.ID, legs); err != nil {
			return err
		}
		posted, removed, shrunk, err := shrinkReversedLots(tx, original.ID, postings, amount)
		if err != nil {
			return err
		}
		if err := tx.ApplyPosted(posted, journal.ID); err != nil {
			return err
		}
		available, err := availableForUser(tx, owner)
		if err != nil {
			return err
		}
		result = ReverseResult{
			JournalID:      journal.ID,
			ReversesID:     original.ID,
			Amount:         amount,
			Available:      available,
			LotsRemoved:    removed,
			LotsShrunk:     shrunk,
			JournalTime:    now,
			OriginalReason: original.ReasonCode,
		}
		return nil
	})
	if err != nil {
		return ReverseResult{}, err
	}
	return result, nil
}

// replayReversal resolves a duplicate idempotency key on a reversal.
//
// A reversal is doubly guarded, because it is the one operation that reduces a
// balance: CountReversalsOf already refused a second reversal of the same
// original above, and the key's unique index refuses a second journal for the
// same key. Both have to hold, because either alone leaves a path to a doubled
// clawback — a caller retrying with a fresh key, or two callers racing past the
// state check — and a doubled clawback is a user losing 160 coins instead of 80.
func replayReversal(tx *TxContext, original *CoinJournal, fp []byte, existing *CoinJournal, out *ReverseResult) error {
	if existing == nil {
		return fmt.Errorf("%w: the idempotency key conflicted but the journal could not be read back", ErrImmutable)
	}
	if !bytes.Equal(existing.RequestFingerprint, fp) {
		return fmt.Errorf("%w: the key was already used for a different request", ErrIdempotencyKeyReuse)
	}
	if existing.EntryType != EntryReversal || existing.ReversalOf == nil || *existing.ReversalOf != original.ID {
		return fmt.Errorf("%w: the key belongs to a different journal", ErrIdempotencyKeyReuse)
	}
	amount, err := replayAmount(existing)
	if err != nil {
		return err
	}
	owner := tx.UserID()
	available, err := availableForUser(tx, owner)
	if err != nil {
		return err
	}
	*out = ReverseResult{
		JournalID:      existing.ID,
		ReversesID:     original.ID,
		Amount:         amount,
		Available:      available,
		Replayed:       true,
		JournalTime:    existing.CreatedAt,
		OriginalReason: original.ReasonCode,
	}
	return nil
}

// shrinkReversedLots reduces the lots the reversed grant opened and returns the
// posted deltas for the reversal.
//
// The lots come from the original journal, NOT from the legs being negated: a
// grant's postings carry no lot_id, so a reversal that looked for one would find
// nothing and would leave the clawed-back coins sitting in a lot, still
// spendable. That is the same class of silent failure as the accumulator bug —
// the balance looks right and the money is still there.
//
// The reduction is spread across the grant's lots oldest first, so a multi-lot
// grant gives up its oldest value first, which is the order those coins would
// have been spent in. A lot whose remaining value the reduction fully consumes is
// deleted rather than zeroed, because CHECK (granted > 0) forbids a zero grant
// and the lot has nothing left to represent; the journal and its postings are the
// permanent record, and coin_lot is not append-only.
func shrinkReversedLots(tx *TxContext, journalID string, postings []CoinPosting, amount int64) (map[uint]int64, int, int, error) {
	posted := make(map[uint]int64, len(postings))
	// The delta is the negation of every leg, so a reversed grant takes the user
	// back down and hands the faucet its coins back.
	for _, p := range postings {
		posted[p.AccountID] -= p.Amount
	}

	lots, err := tx.LotsOfJournal(journalID)
	if err != nil {
		return nil, 0, 0, err
	}

	remaining := amount
	removed, shrunk := 0, 0
	for _, lot := range lots {
		if remaining <= 0 {
			break
		}
		openValue := lot.Granted - lot.Consumed
		if openValue <= 0 {
			continue
		}
		apply := openValue
		if apply > remaining {
			apply = remaining
		}
		if apply == openValue && lot.Consumed == 0 {
			// Nothing left to represent: no value, and nothing ever spent.
			if err := tx.DeleteLot(lot.ID); err != nil {
				return nil, 0, 0, err
			}
			removed++
		} else {
			// ShrinkLot floors granted at consumed, so a clawback on
			// partly-spent coins cannot break CHECK (consumed <= granted).
			if err := tx.ShrinkLot(lot.ID, apply); err != nil {
				return nil, 0, 0, err
			}
			shrunk++
		}
		remaining -= apply
	}
	return posted, removed, shrunk, nil
}

// userLegTotal is the number of coins a journal moved to a user, which is the
// amount a reversal takes back.
//
// It is computed from the legs rather than from a stored total, so a reversal can
// never claim a number the postings do not support. A grant's user legs are
// positive and a spend's are negative, which is how a spend is recognised and
// refused without a lookup.
func userLegTotal(postings []CoinPosting, userAccounts map[uint]struct{}) int64 {
	var total int64
	for _, p := range postings {
		if _, ok := userAccounts[p.AccountID]; ok {
			total += p.Amount
		}
	}
	return total
}

// ── Reserve / ReleaseReserved ────────────────────────────────────────────────

// Reserve holds coins that are promised but not yet granted.
//
// **A reserve is not a grant.** It moves nothing: `posted` is untouched, no
// posting is written, and the money has not been issued. It raises `reserved`,
// and availability is posted - reserved everywhere in this file. A user with 200
// posted and 150 reserved has 50 available, and a 60-coin spend is refused. That
// subtraction is the entire mechanism.
//
// It could not be modelled as a grant that is later clawed back, because a grant
// is a real issuance: reversing it would put a phantom movement in the audit
// trail for coins that were never issued, and every "why does this balance say
// what it says" question would have to answer for a movement that never
// economically happened.
//
// The hold IS journalled, though, because it has to be: it must be idempotent per
// referral, it must record who placed it and when it is expected to resolve, and
// a release has to be able to find the exact amount to un-hold. The journal
// carries no postings and stays PENDING until resolved; see holdReasons for why
// that set is enumerated rather than implied.
func (l *Ledger) Reserve(ctx context.Context, req ReserveRequest) (ReserveResult, error) {
	if err := validateUser(req.UserID); err != nil {
		return ReserveResult{}, err
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return ReserveResult{}, err
	}
	if req.ReasonCode != ReasonReferralHold {
		return ReserveResult{}, fmt.Errorf("%w: %q is not a hold this ledger can place", ErrInvalidArgument, req.ReasonCode)
	}
	if strings.TrimSpace(req.RefType) == "" {
		return ReserveResult{}, fmt.Errorf("%w: a hold needs a ref_type so a release can find it", ErrInvalidArgument)
	}
	if req.RefID == 0 {
		return ReserveResult{}, fmt.Errorf("%w: a hold needs a ref_id so a release can find it", ErrInvalidArgument)
	}
	if l.config == nil {
		return ReserveResult{}, ErrNoDatabase
	}
	cfg, err := l.config.Load()
	if err != nil {
		return ReserveResult{}, err
	}
	amount := cfg.Awards.ReferralReferrer
	if amount <= 0 {
		return ReserveResult{}, fmt.Errorf("%w: referral_referrer is configured to %d coins, so there is nothing to hold", ErrInvalidArgument, amount)
	}

	now := l.now().UTC()
	refID := req.RefID
	refType := req.RefType
	holdUntil := now.AddDate(0, 0, int(cfg.Referral.HoldDays))
	fp := fingerprint(
		EntryAdjust, ReasonReferralHold, strconv.FormatUint(uint64(req.UserID), 10),
		refFingerprint(&refType, &refID), strconv.FormatInt(amount, 10),
	)

	var result ReserveResult
	err = l.repo.InUserTx(ctx, req.UserID, func(tx *TxContext) error {
		// Resolved before the journal insert so the amount and the account it
		// sits on are part of the journal's metadata from the first write. A hold
		// that had to be back-filled would have a window in which a release
		// could read a hold with no amount to un-hold.
		accountID, err := tx.EnsureUserAccount(req.UserID, BucketEarned)
		if err != nil {
			return err
		}
		available, err := availableForUser(tx, req.UserID)
		if err != nil {
			return err
		}

		journal := &CoinJournal{
			ID:                 uuid.NewString(),
			EntryType:          EntryAdjust,
			State:              StatePending,
			Scope:              ScopeUser,
			IdempotencyKey:     req.IdempotencyKey,
			RequestFingerprint: fp,
			ReasonCode:         ReasonReferralHold,
			RefType:            &refType,
			RefID:              &refID,
			EffectiveAt:        now,
			CreatedAt:          now,
			CreatedBy:          req.CreatedBy,
			Metadata: map[string]any{
				"amount":     amount,
				"account_id": accountID,
				"hold_days":  cfg.Referral.HoldDays,
				"hold_until": holdUntil.UTC().Format(time.RFC3339),
				"event":      req.Metadata,
			},
		}
		created, err := tx.InsertJournal(journal)
		if err != nil {
			return err
		}
		if !created {
			existing, err := tx.FindJournalByKey(ScopeUser, req.IdempotencyKey)
			if err != nil {
				return err
			}
			if existing == nil {
				return fmt.Errorf("%w: idempotency key %q conflicted but the journal could not be read back", ErrImmutable, req.IdempotencyKey)
			}
			if !bytes.Equal(existing.RequestFingerprint, fp) {
				return fmt.Errorf("%w: key %q was already used for a different request", ErrIdempotencyKeyReuse, req.IdempotencyKey)
			}
			held, heldAccount, err := holdMetadata(existing)
			if err != nil {
				return err
			}
			result = ReserveResult{
				JournalID:   existing.ID,
				Amount:      held,
				AccountID:   heldAccount,
				Available:   available,
				HoldUntil:   holdUntil,
				Replayed:    true,
				JournalTime: existing.CreatedAt,
			}
			return nil
		}

		// Checked before the write so the answer is typed rather than a CHECK
		// violation. The CHECK is still the backstop; this is the version that can
		// say why.
		if available < amount {
			return fmt.Errorf("%w: cannot hold %d with %d available", ErrHoldExceedsBalance, amount, available)
		}
		if err := tx.ApplyReserved(map[uint]int64{accountID: amount}, journal.ID); err != nil {
			return err
		}
		result = ReserveResult{
			JournalID:   journal.ID,
			Amount:      amount,
			AccountID:   accountID,
			Available:   available - amount,
			HoldUntil:   holdUntil,
			JournalTime: now,
		}
		return nil
	})
	if err != nil {
		return ReserveResult{}, err
	}
	return result, nil
}

// ReleaseReserved lifts a hold.
//
// It is identified by the hold it releases — (reason, ref) — and takes no
// idempotency key of its own, because its idempotency is structural: the
// transition is a compare-and-set on the hold's state, so a second call finds
// REVERSED, changes nothing, and returns the same hold journal with Replayed.
// A second `reserved` decrement is not representable.
//
// The mirror image of Reserve: `reserved` falls, `posted` does not move, and no
// posting is written. A clawback of coins that were actually granted is Reverse,
// which is a different operation with a different journal.
//
// Which of the two a referral gets is the referral state machine's decision, and
// that is a later phase. This method is the primitive both are built from, and
// the seam is deliberate: converting a hold into a grant is ReleaseReserved
// followed by Grant, and in two transactions that leaves a window in which the
// coins are unheld. The referral phase must do both in one transaction and should
// say so where it does.
func (l *Ledger) ReleaseReserved(ctx context.Context, req ReleaseReservedRequest) (ReleaseResult, error) {
	if err := validateUser(req.UserID); err != nil {
		return ReleaseResult{}, err
	}
	if !IsHoldReason(req.ReasonCode) {
		return ReleaseResult{}, fmt.Errorf("%w: %q is not a hold reason", ErrInvalidArgument, req.ReasonCode)
	}
	if strings.TrimSpace(req.RefType) == "" || req.RefID == 0 {
		return ReleaseResult{}, fmt.Errorf("%w: a release must identify the hold it releases", ErrInvalidArgument)
	}

	now := l.now().UTC()
	refID := req.RefID
	refType := req.RefType
	var result ReleaseResult
	err := l.repo.InUserTx(ctx, req.UserID, func(tx *TxContext) error {
		hold, err := tx.FindOpenHold(req.ReasonCode, refType, &refID)
		if err != nil {
			return err
		}
		if hold == nil {
			// Either there was never a hold, or it has already been resolved.
			// Both are the same answer, and it is the safe direction: neither
			// releases anything. Distinguishing them needs a lookup of a REVERSED
			// hold, which would then have to be distinguished from a hold for a
			// different event — and a caller that cannot tell "already released"
			// from "never held" still cannot release anything twice.
			return fmt.Errorf("%w: no open %s for user %d ref %s:%d", ErrNotFound, req.ReasonCode, req.UserID, refType, refID)
		}
		amount, accountID, err := holdMetadata(hold)
		if err != nil {
			return err
		}
		available, err := availableForUser(tx, req.UserID)
		if err != nil {
			return err
		}

		// The compare-and-set. changed == false means this hold was resolved
		// between the read and the write — impossible under the user lock, and
		// still checked so the invariant is enforced rather than assumed.
		changed, err := tx.SetJournalState(hold.ID, StatePending, StateReversed, map[string]any{
			"released_at":  now.UTC().Format(time.RFC3339),
			"released_by":  req.CreatedBy,
			"release_note": req.Metadata,
		})
		if err != nil {
			return err
		}
		if !changed {
			result = ReleaseResult{
				JournalID:  hold.ID,
				Amount:     amount,
				AccountID:  accountID,
				Available:  available,
				Replayed:   true,
				ReleasedAt: now,
			}
			return nil
		}
		if err := tx.ApplyReserved(map[uint]int64{accountID: -amount}, hold.ID); err != nil {
			return err
		}
		after, err := availableForUser(tx, req.UserID)
		if err != nil {
			return err
		}
		result = ReleaseResult{
			JournalID:  hold.ID,
			Amount:     amount,
			AccountID:  accountID,
			Available:  after,
			ReleasedAt: now,
		}
		return nil
	})
	if err != nil {
		return ReleaseResult{}, err
	}
	return result, nil
}

// ── pure helpers ─────────────────────────────────────────────────────────────

// spendPrice resolves the price of a class from the economy config.
//
// The class comes from RefType, never from a caller-supplied amount — the rule in
// 02-architecture.md §12.1. An unknown class is ErrInvalidArgument rather than a
// zero price, because a zero price is a free unlock.
func spendPrice(cfg EconomyConfig, reasonCode, refType string) (int64, error) {
	if reasonCode != ReasonResourceUnlock {
		return 0, fmt.Errorf("%w: %q is not a spend reason", ErrInvalidArgument, reasonCode)
	}
	switch refType {
	case RefStudyResource:
		return cfg.Prices.StudyResource, nil
	case RefVideo:
		return cfg.Prices.Video, nil
	case RefMockTest:
		return cfg.Prices.MockTest, nil
	}
	return 0, fmt.Errorf("%w: %q is not a purchasable class", ErrInvalidArgument, refType)
}

// fingerprint is the sha256 of the canonical request, and it is how a replay is
// told apart from a key collision.
//
// The resolved amount is part of it, deliberately. A price change between the
// original request and the retry must be a conflict, not a replay: the caller
// asked to be charged yesterday's price and got yesterday's journal, and
// returning that silently is correct. But a caller who reused a key for a
// different class, a different event or a different user is a bug, and a
// fingerprint that omitted the amount would let two different spends share one
// journal whenever the price had not moved.
//
// Fields are NUL-separated so no two different field splits can produce the same
// input, and the user id is included so a key reused across users is a conflict
// rather than one user's journal being handed to another. That matters because
// ScopeUser is a constant, so the (scope, idempotency_key) index does not
// partition keys by user on its own.
func fingerprint(parts ...string) []byte {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return h.Sum(nil)
}

func validateUser(userID uint) error {
	if userID == 0 {
		return fmt.Errorf("%w: a ledger operation needs a real user id", ErrInvalidArgument)
	}
	return nil
}

func validateIdempotencyKey(key string) error {
	if strings.TrimSpace(key) == "" {
		// 03-api-contract.md §2.3: the header is required, and without it 400.
		// The ledger is where that is enforced, because the ledger is the only
		// thing that can guarantee the key is actually used.
		return fmt.Errorf("%w: an idempotency key is required", ErrInvalidArgument)
	}
	if len(key) > 255 {
		return fmt.Errorf("%w: the idempotency key is %d bytes and the column allows 255", ErrInvalidArgument, len(key))
	}
	return nil
}

func validateRef(refType *string, refID *uint64) error {
	if refType == nil || strings.TrimSpace(*refType) == "" {
		if refID != nil && *refID != 0 {
			return fmt.Errorf("%w: a ref_id needs a ref_type", ErrInvalidArgument)
		}
		return nil
	}
	if refID == nil || *refID == 0 {
		// CHECK (ref_type IS NULL OR ref_id IS NOT NULL) says the same thing
		// three statements later; saying it here names the field.
		return fmt.Errorf("%w: ref_type %q needs a ref_id", ErrInvalidArgument, *refType)
	}
	return nil
}

func refFingerprint(refType *string, refID *uint64) string {
	if refType == nil || strings.TrimSpace(*refType) == "" {
		return ""
	}
	parts := []string{*refType}
	if refID != nil {
		parts = append(parts, strconv.FormatUint(*refID, 10))
	}
	return strings.Join(parts, ":")
}

func grantMetadata(event map[string]any, amount int64, bucket string, expiresAt *time.Time) map[string]any {
	meta := map[string]any{"amount": amount, "bucket": bucket}
	if expiresAt != nil {
		meta["expires_at"] = expiresAt.UTC().Format(time.RFC3339)
	}
	if len(event) > 0 {
		meta["event"] = event
	}
	return meta
}

func spendMetadata(event map[string]any, price int64, class string) map[string]any {
	meta := map[string]any{"amount": price, "class": class}
	if len(event) > 0 {
		meta["event"] = event
	}
	return meta
}

// replayAmount reads the amount a journal actually moved, from its own metadata.
// Recomputing it from the current config would report today's price for a
// movement that happened at yesterday's.
func replayAmount(j *CoinJournal) (int64, error) {
	amount, ok := j.Metadata["amount"].(float64)
	if !ok {
		return 0, fmt.Errorf("%w: journal %s does not record the amount it moved", ErrImmutable, j.ID)
	}
	return int64(amount), nil
}

func replayGrantMetadata(j *CoinJournal) (int64, string, error) {
	amount, err := replayAmount(j)
	if err != nil {
		return 0, "", err
	}
	if bucket, ok := j.Metadata["bucket"].(string); ok && bucket != "" {
		return amount, bucket, nil
	}
	// A journal that predates the metadata still fixes its own bucket through its
	// reason code, because that is where the bucket came from in the first place.
	spec, ok := grantSpecs[j.ReasonCode]
	if !ok {
		return 0, "", fmt.Errorf("%w: journal %s records neither a bucket nor a reason this ledger knows", ErrImmutable, j.ID)
	}
	return amount, spec.bucket, nil
}

func holdMetadata(j *CoinJournal) (int64, uint, error) {
	amount, ok := j.Metadata["amount"].(float64)
	if !ok {
		return 0, 0, fmt.Errorf("%w: hold %s does not record the amount it held", ErrImmutable, j.ID)
	}
	accountID, ok := j.Metadata["account_id"].(float64)
	if !ok || accountID <= 0 {
		return 0, 0, fmt.Errorf("%w: hold %s does not record the account it sits on", ErrImmutable, j.ID)
	}
	return int64(amount), uint(accountID), nil
}

// availableForUser is posted - reserved summed across every one of a user's
// bucket accounts. It is the single definition of "what this user can spend", and
// Grant, Spend, Reserve and ReleaseReserved all use it, so they cannot drift
// apart on what "available" means.
func availableForUser(tx *TxContext, userID uint) (int64, error) {
	balances, err := tx.UserBalances(userID)
	if err != nil {
		return 0, err
	}
	return sumAvailable(indexBalances(balances)), nil
}

func indexBalances(balances []AccountBalance) map[uint]AccountBalance {
	out := make(map[uint]AccountBalance, len(balances))
	for _, b := range balances {
		out[b.AccountID] = b
	}
	return out
}

func sumAvailable(balances map[uint]AccountBalance) int64 {
	var total int64
	for _, b := range balances {
		total += b.Available()
	}
	return total
}

// frozenAccounts reports the lowest-numbered closed account in a set. A closed
// coin_account is a hold placed by support, and nothing may move through it (423
// in 09-support-copy-cheat-sheet.md). Deterministic because the account set is a
// map and an error message that names a different account on each run is useless
// in a log.
func frozenAccounts(balances map[uint]AccountBalance) (bool, uint) {
	ids := make([]uint, 0, len(balances))
	for id := range balances {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if balances[id].Closed {
			return true, id
		}
	}
	return false, 0
}
