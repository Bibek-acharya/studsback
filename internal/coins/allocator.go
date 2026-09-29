// internal/coins/allocator.go
//
// The FEFO lot allocator. It is the single piece of the ledger with arithmetic
// in it, and it is therefore the single piece with a bug class the database
// cannot catch.
//
// The bug is named in 02-architecture.md §4 and in 04-implementation-plan.md
// §3.3: the first implementation of this loop OVERWROTE its running total
// instead of accumulating it, so a spend of 120 became 145. No constraint caught
// it. The postings still summed to zero, because the system leg was written
// from the same wrong total, and the cached balance still matched the postings,
// because it was updated from the same wrong total. The only artefact that
// disagreed was the sum of the lot rows — which nothing in the write path reads.
//
// It fails silently and it overcharges real users. So three things are load
// bearing here and each is stated in the code rather than only in a comment:
//
//  1. `taken += inc` — accumulate. Never `taken = inc`.
//
//  2. The caller posts the PER-LOT increment (`-inc`), never the running total
//     (`-taken`). postingSpend below builds one posting per lot from
//     LotConsumption.Amount, and Allocation deliberately does not expose `taken`
//     as a per-lot quantity for it to be misused as one.
//
//  3. The loop breaks as soon as `taken >= amount`. A spend that is already
//     satisfied must not keep eating the next lot, which is how an off-by-one
//     becomes a real overcharge.
//
// The function is pure: no clock of its own beyond the `now` it is handed, no
// database, no globals, and it copies its input before sorting so the caller's
// slice is never reordered. That is what makes the property tests in
// ledger_test.go possible, and it is also why the allocator is a separate file.
package coins

import (
	"fmt"
	"sort"
	"time"
)

// LotBalance is the allocator's read-only view of one grant lot. It carries only
// what the decision needs, so the allocator cannot accidentally grow a
// dependency on the rest of the lot row.
type LotBalance struct {
	ID        uint
	AccountID uint
	Bucket    string
	Granted   int64
	Consumed  int64
	// ExpiresAt is nil for a lot that never expires. FEFO sorts nil LAST, which
	// is the consumer-favourable end of the rule: a user never watches coins
	// evaporate while untouched permanent ones sit in their balance.
	ExpiresAt *time.Time
}

// remaining is the spendable value left in the lot. A lot at or past its grant
// contributes nothing rather than a negative, so a corrupt row cannot make the
// allocator take a negative slice out of the next lot.
func (l LotBalance) remaining() int64 {
	if l.Granted <= l.Consumed {
		return 0
	}
	return l.Granted - l.Consumed
}

// openAt reports whether the lot is spendable at now: not already consumed, and
// not past its expiry.
//
// The repository filters both in SQL, so this is redundant on the normal path.
// It is here because the allocator must be safe for any caller: a defensive skip
// costs nothing, whereas an expired lot silently becoming spendable is a
// correctness bug that only shows up as an unexplained balance.
func (l LotBalance) openAt(now time.Time) bool {
	if l.remaining() <= 0 {
		return false
	}
	if l.ExpiresAt != nil && !l.ExpiresAt.After(now) {
		return false
	}
	return true
}

// LotConsumption is one lot's share of a single spend. Amount is strictly
// positive, and the sum of every Amount in an Allocation is exactly the
// requested amount. It is the per-lot INCREMENT, never a running total: that
// distinction is the whole point of this file.
type LotConsumption struct {
	LotID  uint
	Amount int64
}

// Allocation is a successful FEFO allocation.
type Allocation struct {
	// Requested is what the caller asked for, echoed so the caller can assert
	// against it rather than recomputing.
	Requested int64
	// Taken is the total consumed across all lots. On success it always equals
	// Requested. It exists for the invariant test and for error messages, not as
	// something to post: posting `taken` to each lot is exactly the bug this
	// file exists to prevent.
	Taken int64
	// Consumptions is the FEFO prefix of the open-lot ordering, in the order the
	// lots are burned. Never empty on success.
	Consumptions []LotConsumption
}

// Total is the sum of the per-lot increments. It is the only quantity that may
// be posted against a single account as an aggregate, and even then the
// caller should post per lot so the lot_id attribution survives.
func (a Allocation) Total() int64 {
	var sum int64
	for _, c := range a.Consumptions {
		sum += c.Amount
	}
	return sum
}

// AllocateFEFO decides which lots pay for `amount` coins, soonest-expiring
// first. It is a pure function over the lots it is given.
//
// The contract, which ledger_test.go asserts as properties rather than examples:
//
//   - On success, sum of every LotConsumption.Amount == amount. Always.
//   - Every LotConsumption.Amount is strictly positive and strictly less than
//     or equal to that lot's remaining value, so no lot is ever over-consumed.
//   - The consumed lots are exactly the FEFO PREFIX of the open-lot ordering:
//     it never skips a soon-expiring lot for a later one, and it stops the
//     instant the amount is satisfied.
//   - The result does not depend on the order the caller supplied the lots in.
//     See fefoBefore for why the allocator re-sorts rather than trusting its
//     caller.
//   - A request larger than the total available changes nothing: it returns an
//     error and a zero Allocation, so there is no partial result for a caller to
//     apply by mistake.
//   - A non-positive amount is rejected before anything is computed, so there is
//     no path on which a zero-amount spend produces a posting (coin_posting has
//     CHECK (amount <> 0), and a zero posting is a bug either way).
//
// The error is ErrInsufficientCoins with the shortfall in the message; it is
// never ErrInvalidArgument, because a too-small balance is a designed state in
// the funnel (402) and not a malformed request.
func AllocateFEFO(lots []LotBalance, amount int64, now time.Time) (Allocation, error) {
	// Rejected first, before any arithmetic. A zero or negative amount is a
	// programming error at the call site, and computing on it would produce an
	// empty allocation that looks like a legitimate "nothing to spend".
	if amount <= 0 {
		return Allocation{}, fmt.Errorf("%w: a spend must be for a positive number of coins, got %d", ErrInvalidArgument, amount)
	}

	// Sorted into a copy: fefoBefore is a total order, but the caller's slice
	// order is not ours to disturb, and the repository's rows are only *already*
	// FEFO-ordered by luck of the query plan.
	ordered := make([]LotBalance, len(lots))
	copy(ordered, lots)
	sort.SliceStable(ordered, func(i, j int) bool { return fefoBefore(ordered[i], ordered[j]) })

	alloc := Allocation{Requested: amount}
	var taken int64
	for _, lot := range ordered {
		// Break, not continue: once the amount is satisfied the remaining lots
		// must not be touched at all. `continue` here would still be correct
		// because every later `inc` would be zero, but it would keep walking
		// rows and the explicit break is the statement of intent.
		if taken >= amount {
			break
		}
		if !lot.openAt(now) {
			continue
		}

		// inc is this lot's SHARE, never the running total. `inc` is bounded
		// above by what is still owed, which is what stops a lot being consumed
		// past the point where the spend is satisfied — and it is the line the
		// original bug sat on.
		inc := lot.remaining()
		if owed := amount - taken; inc > owed {
			inc = owed
		}
		if inc <= 0 {
			continue
		}

		taken += inc
		alloc.Consumptions = append(alloc.Consumptions, LotConsumption{LotID: lot.ID, Amount: inc})
	}

	if taken != amount {
		// No partial result. Returning one would invite a caller that ignores
		// the error to burn `taken` coins and think it had been asked for
		// `amount`.
		// Typed, so the 402 body can be built from the error without a second
		// read. This path is reachable only when the cached projection and the
		// lots disagree, and previously it reported available=0 in that case.
		return Allocation{}, fmt.Errorf("%w across %d open lot(s)",
			ErrInsufficient(amount, taken), len(alloc.Consumptions))
	}
	alloc.Taken = taken
	return alloc, nil
}

// fefoBefore is the FEFO order: soonest expiry first, never-expiring last, and
// lot id ascending as the tiebreak so the order is total and stable.
//
// Lot ids are unique, so the tiebreak never fires for two distinct lots with the
// same instant expiry. It is still there, and it is still total, because the
// allocator is a general function: two lots granted in the same transaction
// share a timestamp, and an order that left them interchangeable would make the
// allocation depend on the physical row order. The property test
// TestAllocateFEFOIsIndependentOfInputOrder exists to keep that honest.
//
// This mirrors the SQL in repository.openLots (`ORDER BY expires_at NULLS LAST,
// id`) and the partial index coin_lot_open_idx. The index makes the database
// return rows in this order cheaply; the sort here makes the decision correct
// even when it does not.
func fefoBefore(a, b LotBalance) bool {
	switch {
	case a.ExpiresAt == nil && b.ExpiresAt == nil:
		// Both never expire: fall through to the id tiebreak.
	case a.ExpiresAt == nil:
		// a never expires, b does: b is burned first. This single case is the
		// consumer-favourable half of FEFO.
		return false
	case b.ExpiresAt == nil:
		return true
	case !a.ExpiresAt.Equal(*b.ExpiresAt):
		return a.ExpiresAt.Before(*b.ExpiresAt)
	}
	return a.ID < b.ID
}

// fefoOrder returns the lots in the order AllocateFEFO burns them. Exported
// behaviour for the property tests and for the balance endpoint's spend_order,
// which has to show the student the same order the spend will use.
func fefoOrder(lots []LotBalance) []LotBalance {
	ordered := make([]LotBalance, len(lots))
	copy(ordered, lots)
	sort.SliceStable(ordered, func(i, j int) bool { return fefoBefore(ordered[i], ordered[j]) })
	return ordered
}

// AvailableInLots totals the spendable value of the given lots at now. It is the
// lot-side view of availability, used to report a shortfall that names the real
// cause ("your coins expired") rather than the cached balance, which only knows
// what the postings said.
func AvailableInLots(lots []LotBalance, now time.Time) int64 {
	var total int64
	for _, lot := range lots {
		if lot.openAt(now) {
			total += lot.remaining()
		}
	}
	return total
}
