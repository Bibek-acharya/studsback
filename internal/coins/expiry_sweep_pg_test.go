//go:build coinsintegration

// internal/coins/expiry_sweep_pg_test.go
//
// 04 §5 (Phase 5), item 1: the expiry sweep.
//
// THE RULE UNDER TEST: a lot whose expiry has passed must stop counting toward the
// student's spendable balance, by being BURNED — debited from the user and credited
// to expired_burn through an EXPIRE journal — and never by being deleted.
//
// The deletion half matters more than it looks. Deleting the lot would leave the
// cached posted_balance unchanged, so the student's headline figure would keep
// counting coins they cannot spend, and the reconciler would find a balance that
// matches its postings while the wallet disagrees with both. An EXPIRE journal is
// the only way all three agree.
//
// Everything here is Postgres-only. The properties are enforced by
// coin_journal's UNIQUE (scope, idempotency_key), coin_lot's indexes, and the
// `FOR UPDATE SKIP LOCKED` semantics — none of which SQLite has.

package coins

import (
	"context"
	"testing"
	"time"
)

func TestTheSweepBurnsAnExpiredLotAndNeverDeletesIt(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	// A grant whose lot expires an hour ago.
	grant, lotID := env.grantExpiring(t, 4242, -time.Hour)

	report, err := env.sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Expired != 1 {
		t.Fatalf("expired = %d, want 1 (report %+v)", report.Expired, report)
	}

	// The lot is STILL THERE. Burned, not deleted.
	var lot CoinLot
	if err := env.pool.First(&lot, lotID).Error; err != nil {
		t.Fatalf("the expired lot was DELETED; expiry must burn, never delete: %v", err)
	}
	if lot.Granted != grant.Amount {
		t.Errorf("lot.granted changed to %d, want %d — the audit trail is the record", lot.Granted, grant.Amount)
	}

	// An EXPIRE journal, and its two legs net to zero with the user DOWN.
	var journal CoinJournal
	if err := env.pool.Where("idempotency_key = ?", "expire:lot:"+u64str(lotID)).First(&journal).Error; err != nil {
		t.Fatalf("no EXPIRE journal for the lot: %v", err)
	}
	if journal.EntryType != EntryExpire {
		t.Errorf("entry_type = %q, want %q", journal.EntryType, EntryExpire)
	}
	if journal.State != StatePosted {
		t.Errorf("state = %q, want %q", journal.State, StatePosted)
	}

	var legs []CoinPosting
	if err := env.pool.Where("journal_id = ?", journal.ID).Order("seq").Find(&legs).Error; err != nil {
		t.Fatalf("read postings: %v", err)
	}
	if len(legs) != 2 {
		t.Fatalf("%d postings, want 2", len(legs))
	}
	var sum int64
	for _, l := range legs {
		sum += l.Amount
	}
	if sum != 0 {
		t.Errorf("the postings sum to %d; every journal must net to zero", sum)
	}
	if legs[0].Amount >= 0 {
		t.Errorf("the first leg is %d; the user must go DOWN on expiry", legs[0].Amount)
	}
	if legs[1].Amount <= 0 {
		t.Errorf("the second leg is %d; expired_burn must go UP", legs[1].Amount)
	}

	// And the student's spendable balance is now zero.
	if got := env.available(t, 4242); got != 0 {
		t.Errorf("available after expiry = %d, want 0", got)
	}
	// The faucet returned to ZERO. This is the leg that is easy to omit and the
	// reason it matters: expiry is the platform taking back coins it issued, so
	// the liability it carries shrinks by the same amount. Without the third leg
	// the burn would be a gift — the student's balance drops, the faucet stays
	// negative, and the global SUM(posted_balance) invariant fails.
	if got := env.systemBalance(t, SystemEarnedFaucet); got != 0 {
		t.Errorf("earned_faucet = %d, want 0 — a burn returns the liability to the faucet, reversing the original grant", got)
	}
	if got := env.systemBalance(t, SystemExpiredBurn); got != grant.Amount {
		t.Errorf("expired_burn = %d, want %d", got, grant.Amount)
	}
}

// A second sweep must move nothing. This runs on a timer, so "runs twice" is what
// happens on every deploy — the idempotency key is the whole defence.
func TestTheSweepIsIdempotent(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()
	env.grantExpiring(t, 4242, -time.Hour)

	if _, err := env.sweep(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	report, err := env.sweep(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if report.Expired != 0 {
		t.Errorf("the second sweep expired %d lots, want 0", report.Expired)
	}
	var journals int64
	env.pool.Model(&CoinJournal{}).Where("entry_type = ?", EntryExpire).Count(&journals)
	if journals != 1 {
		t.Errorf("%d EXPIRE journals after two sweeps, want 1", journals)
	}
	if got := env.available(t, 4242); got != 0 {
		t.Errorf("available after two sweeps = %d, want 0 — a double burn would go negative", got)
	}
}

// A lot that has NOT expired is untouched, and neither is one that is fully
// consumed — a spent lot has nothing left to burn and expiring it would create a
// journal for zero coins, which the posting CHECK rejects.
func TestTheSweepLeavesLiveAndSpentLotsAlone(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	// Expires in a week: live.
	live, _ := env.grantExpiring(t, 4242, 7*24*time.Hour)
	// Expired an hour ago but already fully spent: nothing to burn.
	spentGrant, spentLot := env.grantExpiring(t, 5151, -time.Hour)
	env.setLotConsumed(t, spentLot, spentGrant.Amount)

	report, err := env.sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Expired != 0 {
		t.Errorf("expired = %d, want 0", report.Expired)
	}
	if got := env.available(t, 4242); got != live.Amount {
		t.Errorf("a live lot's balance = %d, want %d", got, live.Amount)
	}
	var expireJournals int64
	env.pool.Model(&CoinJournal{}).Where("entry_type = ?", EntryExpire).Count(&expireJournals)
	if expireJournals != 0 {
		t.Errorf("%d EXPIRE journals, want 0", expireJournals)
	}
}

// A partially-spent expired lot burns only what is LEFT. Burning the granted
// amount would take the student's balance below the coins they actually spent, and
// the second posting would fail the no-overdraft CHECK rather than quietly
// overdrawing.
func TestAPartiallySpentLotBurnsOnlyTheRemainder(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	grant, lotID := env.grantExpiring(t, 4242, -time.Hour)
	// Spend part of it. Marked directly on the lot because a real SPEND journal
	// would need a priced resource, and this test is about the sweep's arithmetic.
	spent := int64(grant.Amount / 2)
	env.setLotConsumed(t, lotID, spent)
	remaining := grant.Amount - spent
	if err := env.pool.Model(&CoinAccountBalance{}).
		Where("account_id = (SELECT id FROM coin_account WHERE owner_user_id = ?)", 4242).
		UpdateColumn("posted_balance", remaining).Error; err != nil {
		t.Fatalf("adjust cached balance: %v", err)
	}

	report, err := env.sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Expired != 1 {
		t.Fatalf("expired = %d, want 1", report.Expired)
	}
	if report.CoinsBurned != remaining {
		t.Errorf("coins burned = %d, want %d — the remainder, not the %d granted",
			report.CoinsBurned, remaining, grant.Amount)
	}
	if got := env.available(t, 4242); got != 0 {
		t.Errorf("available = %d, want 0", got)
	}
}

// The batch bound. A sweep with no limit on a table with a million expired lots
// would hold a user advisory lock for the length of the whole table, and
// InUserTx sets lock_timeout = 3s — so an unbounded sweep would fail on exactly
// the day the economy is largest.
func TestTheSweepIsBounded(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		env.grantExpiring(t, uint(6000+i), -time.Hour)
	}

	report, err := env.sweepBounded(ctx, 2)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Expired != 2 {
		t.Errorf("expired = %d, want 2 — the batch bound is the batch bound", report.Expired)
	}
	// Truncated MUST be set: 5 lots expired and 2 were burned, so this pass left
	// work behind. Without the flag an operator reading the log cannot tell a
	// partial pass from a clean one, and a backlog stops draining silently.
	if !report.Truncated {
		t.Error("a sweep that hit its batch limit reported Truncated=false; a partial pass is indistinguishable from a clean one")
	}
	// The rest are still there for the next pass. Asserted as a COUNT of expired
	// lots still holding coins, not as any particular student's balance: the sweep
	// orders by expires_at and these five all expire at effectively the same instant,
	// so which two it burns is not deterministic and a per-student assertion would
	// be asserting on an accident of the index scan.
	var stillExpired int64
	env.pool.Model(&CoinLot{}).
		Where("expires_at IS NOT NULL AND expires_at <= ? AND consumed < granted", time.Now().UTC()).
		Count(&stillExpired)
	if stillExpired != 3 {
		t.Errorf("%d expired lots remain, want 3 — a bounded pass must leave the rest for the next one", stillExpired)
	}
}
