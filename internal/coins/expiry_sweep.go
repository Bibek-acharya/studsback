// internal/coins/expiry_sweep.go
//
// 04-implementation-plan.md §5 (Phase 5), item 1: the expiry sweep.
//
// ── what "expired" means here ─────────────────────────────────────────────────
//
// A lot carries its own ExpiresAt. The wallet and the FEFO allocation already
// EXCLUDE expired lots from what may be spent (repository.go's openLotQuery), so an
// expired lot is already unspendable without this file existing.
//
// That is exactly why this file has to exist. If expiry were only "excluded from
// the spend query", the student's cached posted_balance would keep counting coins
// they cannot spend, the wallet's headline figure and its bucket figures would
// disagree, and the reconciler — which checks the cache against the postings —
// would pass while all three told the student different things.
//
// So expiry is a POSTING: the remainder is debited from the user's account and
// credited to expired_burn, through an EXPIRE journal, atomically with the cached
// balance. After that the three agree again, and the student's number is true.
//
// NEVER DELETE. The lot row is the audit trail — it says this student was granted
// 80 coins on a date, 30 were spent, and 50 lapsed. Deleting it destroys the only
// record that makes a balance explainable, which is the property the whole ledger
// exists to provide.
//
// ── idempotency ───────────────────────────────────────────────────────────────
//
// This runs on a timer, so "runs twice" is not hypothetical — it is every deploy
// and every process restart. The key is `expire:lot:<id>`, one per lot, and
// coin_journal's UNIQUE (scope, idempotency_key) does the work. A second sweep over
// an already-burned lot finds no open lot, because the lot is fully consumed, so
// the common case never even reaches the journal insert.
//
// ── why the selection is a SKIP LOCKED batch, not a bulk UPDATE ───────────────
//
// Each lot's burn has to run inside InUserTx, which takes pg_advisory_xact_lock on
// the OWNER. A bulk UPDATE would take no such lock and would race a concurrent
// spend on the same lot, producing a balance that is neither. So the selection
// claims rows with FOR UPDATE SKIP LOCKED and then processes them one at a time —
// which is also what lets several workers run in parallel without colliding, and
// why SKIP LOCKED rather than a blocking lock.
//
// The batch bound is not an optimisation. InUserTx sets lock_timeout = '3s', so an
// unbounded sweep over a table with a million expired lots would hold a user lock
// for the length of the whole table and fail on exactly the day the economy is
// largest. A sweep that hits its bound says so in the report rather than looking
// like a clean pass.
package coins

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"studsphere/backend/internal/shared/logger"

	"github.com/google/uuid"
)

// ReasonCoinExpired is the journal reason for a lot lapsing.
//
// It is not in grantSpecs, correctly: expiry is not a grant, and a lookup in that
// map would let something ask the ledger to PAY for an expiry.
const ReasonCoinExpired = "COIN_EXPIRED"

// ExpiryBatchDefault is the per-pass lot bound. Sized so a pass finishes well
// inside InUserTx's 3s lock_timeout even with the per-lot round trips, and so a
// backlog is drained in passes rather than in one long transaction.
const ExpiryBatchDefault = 500

// ExpireReport is what one pass did.
//
// A report rather than a bare error, for the same reason RunReconcile returns one:
// a pass that found nothing has SUCCEEDED, and a caller that cannot tell that from
// a failure will alert on it.
type ExpireReport struct {
	// Scanned is how many candidate lots the selection returned.
	Scanned int
	// Expired is how many this pass actually burned.
	Expired int
	// CoinsBurned is the total debited from students.
	CoinsBurned int64
	// Skipped is how many candidates were already fully consumed — a spent lot has
	// nothing to burn, and burning zero would be a journal the posting CHECK
	// rejects.
	Skipped int
	// Failed is how many candidates could not be burned. The pass CONTINUES past a
	// failure rather than stopping, because one bad lot must not strand the rest of
	// the backlog forever; a persistently failing lot shows up here on every pass.
	Failed int
	// Truncated is true when the batch bound stopped the selection with more work
	// waiting. Its absence would make a partial pass indistinguishable from a clean
	// one, which is how a backlog silently stops draining.
	Truncated bool
	// RanAt is when the pass started.
	RanAt time.Time
}

// ExpirySweeper burns lapsed lots.
type ExpirySweeper struct {
	repo   *Repository
	ledger *Ledger
	now    func() time.Time
}

// NewExpirySweeper wires the sweeper. A nil ledger makes every pass an error
// rather than a silent no-op, for the same reason a nil grant refuses an approval.
func NewExpirySweeper(repo *Repository, ledger *Ledger) *ExpirySweeper {
	return &ExpirySweeper{
		repo:   repo,
		ledger: ledger,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// expireLotKey is the idempotency key for one lot's expiry.
//
// ONE PER LOT, not per lot-and-amount: the amount that lapses is whatever was
// left, which is a function of how much the student spent first. Keying on the
// amount would let a student who spends between two sweeps produce a second key
// for the same lot and be charged for the same remainder twice.
func expireLotKey(lotID uint) string {
	return "expire:lot:" + strconv.FormatUint(uint64(lotID), 10)
}

// Sweep burns every lot that has expired and still has coins in it.
//
// limit of 0 means ExpiryBatchDefault. A negative limit is an error rather than
// "unbounded" — an unbounded sweep is the failure this file's batching exists to
// prevent, and a negative value is a caller's bug rather than a request for it.
func (s *ExpirySweeper) Sweep(ctx context.Context, limit int) (ExpireReport, error) {
	report := ExpireReport{RanAt: s.now()}
	if s == nil || s.repo == nil || s.ledger == nil {
		return report, ErrNoDatabase
	}
	if limit == 0 {
		limit = ExpiryBatchDefault
	}
	if limit < 0 {
		return report, fmt.Errorf("%w: the expiry sweep limit is %d", ErrInvalidArgument, limit)
	}

	now := s.now()
	// Fetch ONE MORE than the limit, so "hit the bound" is knowable without a
	// second COUNT query. That is the whole of Truncated.
	candidates, err := s.repo.ClaimExpiredLots(ctx, now, limit+1)
	if err != nil {
		return report, fmt.Errorf("select expired lots: %w", err)
	}
	report.Scanned = len(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
		report.Truncated = true
	}

	for _, candidate := range candidates {
		remaining := candidate.Lot.Granted - candidate.Lot.Consumed
		if remaining <= 0 {
			// Already spent. There is nothing to burn, and a zero-amount journal
			// is refused by CHECK (amount <> 0) on the posting.
			report.Skipped++
			continue
		}
		if candidate.OwnerUserID == 0 {
			// A lot whose account has no owner. That cannot happen through the
			// ledger — every lot is created beside a user account — so it is a data
			// defect, and counting it is better than burning from the wrong account.
			report.Failed++
			continue
		}
		if err := s.burnLot(ctx, candidate, remaining, now); err != nil {
			report.Failed++
			continue
		}
		report.Expired++
		report.CoinsBurned += remaining
	}
	return report, nil
}

// ExpiringLot is one candidate lot together with the student who owns it.
//
// The owner is RESOLVED IN THE BATCH rather than inside the burn transaction, and
// that placement is the point. A lot carries AccountID, not a user id, so the
// translation needs a second query; doing it inside InUserTx would put a lookup that
// touches no balance inside a held pg_advisory_xact_lock, against the very contract
// InUserTx documents ("one user's work, one lock").
type ExpiringLot struct {
	Lot         CoinLot
	OwnerUserID uint
}

// burnLot writes the EXPIRE journal and moves the balance, atomically.
//
// The order inside the transaction is the same one Grant uses, and for the same
// reasons: journal first, then postings, then the cached balance. A crash between
// the journal insert and the postings leaves a journal with no legs — which the
// reconciler reports as an invariant failure rather than a silent drift — and a
// crash after the postings but before the cache leaves a cache that the
// reconciler rebuilds.
func (s *ExpirySweeper) burnLot(ctx context.Context, candidate ExpiringLot, remaining int64, now time.Time) error {
	lot := candidate.Lot
	err := s.repo.InUserTx(ctx, candidate.OwnerUserID, func(tx *TxContext) error {
		acct, err := tx.EnsureUserAccount(candidate.OwnerUserID, lot.Bucket)
		if err != nil {
			return err
		}
		burn, err := tx.SystemAccountID(SystemExpiredBurn)
		if err != nil {
			return err
		}
		fac, err := tx.SystemAccountID(SystemEarnedFaucet)
		if err != nil {
			return err
		}

		journal := &CoinJournal{
			ID:                 uuid.NewString(),
			EntryType:          EntryExpire,
			State:              StatePosted,
			Scope:              ScopeUser,
			IdempotencyKey:     expireLotKey(lot.ID),
			RequestFingerprint: expiryFingerprint(lot.ID, remaining),
			ReasonCode:         ReasonCoinExpired,
			EffectiveAt:        now,
			CreatedAt:          now,
			CreatedBy:          "system:expiry_sweep",
			Metadata: map[string]any{
				"lot_id":     lot.ID,
				"granted":    lot.Granted,
				"consumed":   lot.Consumed,
				"burned":     remaining,
				"bucket":     lot.Bucket,
				"expires_at": time.Time(lotExpiry(lot)).Format(time.RFC3339),
			},
		}
		created, err := tx.InsertJournal(journal)
		if err != nil {
			return err
		}
		if !created {
			// Another worker won the race for this lot. Nothing to do, and NOT an
			// error: this is the concurrency path working.
			return nil
		}

		if err := tx.InsertPostings(journal.ID, []PostingLeg{
			{Seq: 1, AccountID: acct, Amount: -remaining},
			{Seq: 2, AccountID: burn, Amount: remaining},
		}); err != nil {
			return err
		}
		// The faucet moves too, so the global SUM(posted_balance) stays at zero:
		// the platform issued these coins, and an expiry returns the liability.
		// Without this leg the burn is a gift and the invariant breaks.
		if err := tx.ApplyPosted(map[uint]int64{
			acct: -remaining,
			burn: remaining,
			fac:  remaining,
		}, journal.ID); err != nil {
			return err
		}
		// The lot is marked FULLY CONSUMED rather than deleted or re-expiring. That
		// single column is what makes a second sweep skip it: ClaimExpiredLots
		// filters on consumed < granted.
		return tx.ConsumeLot(lot.ID, remaining)
	})
	return err
}

func lotExpiry(l CoinLot) time.Time {
	if l.ExpiresAt == nil {
		return time.Time{}
	}
	return *l.ExpiresAt
}

// StartExpirySweeper runs the sweep on a ticker until stop() is called.
//
// The first pass is deferred by one interval for the reason StartReconciler gives:
// on a rolling deploy several instances would otherwise all sweep at once, and a
// boot has nothing new to sweep.
func StartExpirySweeper(sweeper *ExpirySweeper, interval, timeout time.Duration) (stop func()) {
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				report, err := sweeper.Sweep(ctx, ExpiryBatchDefault)
				switch {
				case err != nil:
					// An error here is a database problem, not a student-facing one, so
					// it is a WARN and never a Fatal: the server must keep serving wallets
					// while the sweep retries on the next tick.
					logger.Warn("coin expiry sweep failed; expired coins stay visible until the next pass",
						"error", err, "timeout", timeout)
				case report.Expired == 0 && report.Failed == 0:
					logger.Info("coin expiry sweep: nothing to expire",
						"scanned", report.Scanned)
				default:
					// The one line an operator actually reads. Truncated is in it because
					// it is the difference between "drained" and "not draining, silently".
					logger.Info("coin expiry sweep",
						"scanned", report.Scanned,
						"expired", report.Expired,
						"coins_burned", report.CoinsBurned,
						"skipped", report.Skipped,
						"failed", report.Failed,
						"truncated", report.Truncated)
				}
				cancel()
			case <-done:
				return
			}
		}
	}()
	return func() { ticker.Stop(); done <- struct{}{} }
}

// expiryFingerprint is the request fingerprint for a lot's expiry journal.
//
// It carries the lot and the amount, NOT the timestamp. A replay must compare
// equal to the original, and a clock that has advanced between two attempts would
// otherwise turn a retry into ErrIdempotencyKeyReuse — "key reused for a
// different request" — for a request that is identical in every way that matters.
func expiryFingerprint(lotID uint, remaining int64) []byte {
	return fingerprint(
		EntryExpire,
		strconv.FormatUint(uint64(lotID), 10),
		ReasonCoinExpired,
		strconv.FormatInt(remaining, 10),
	)
}
