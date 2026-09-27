// internal/coins/ledger_test.go
//
// Property tests for the FEFO allocator. No database, no clock, no I/O: the
// allocator is a pure function, which is the reason it is a separate file.
//
// These are PROPERTIES, not examples. The example-based shape of this test is
// TestAllocateFEFOSpendOf120AcrossThreeBuckets, and it exists to pin the
// documented numbers; everything else generates corpora and asserts invariants
// that must hold for every input. 04-implementation-plan.md §3.3 calls the
// property tier mandatory, and the reason is written into the first test: an
// allocator that overwrote its running total instead of accumulating it turned
// a spend of 120 into 145, no database constraint caught it, and it would have
// been a direct overcharge of real users. An example test would not have caught
// it either; a sum property does.
package coins

import (
	"errors"
	"math/rand"
	"sort"
	"testing"
	"time"
)

// ── fixtures ─────────────────────────────────────────────────────────────────

// testNow is a fixed instant so every expiry in these tests is a constant
// offset from it rather than a wall-clock race.
var testNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func daysOut(d int) *time.Time {
	t := testNow.AddDate(0, 0, d)
	return &t
}

func lot(id uint, accountID uint, bucket string, granted, consumed int64, expiresInDays *int) LotBalance {
	var expiresAt *time.Time
	if expiresInDays != nil {
		expiresAt = daysOut(*expiresInDays)
	}
	return LotBalance{
		ID:        id,
		AccountID: accountID,
		Bucket:    bucket,
		Granted:   granted,
		Consumed:  consumed,
		ExpiresAt: expiresAt,
	}
}

func intp(v int) *int { return &v }

// threeBucketLots is the worked example from 02-architecture.md §4: 100 FREE
// expiring in 30 days, 50 EARNED expiring in 365, 25 never expiring.
func threeBucketLots() []LotBalance {
	return []LotBalance{
		lot(1, 10, BucketFree, 100, 0, intp(30)),
		lot(2, 11, BucketEarned, 50, 0, intp(365)),
		lot(3, 12, BucketEarned, 25, 0, nil),
	}
}

// referenceFEFO is an independent implementation of the ordering, written here
// rather than reused from the package. A property test that asserts the
// allocator agrees with fefoOrder is asserting that two copies of the same idea
// agree; this one is spelled out differently on purpose, and it is the thing the
// "exactly the FEFO prefix" property is measured against.
func referenceFEFO(lots []LotBalance, now time.Time) []LotBalance {
	open := make([]LotBalance, 0, len(lots))
	for _, l := range lots {
		if l.Granted <= l.Consumed {
			continue
		}
		if l.ExpiresAt != nil && !l.ExpiresAt.After(now) {
			continue
		}
		open = append(open, l)
	}
	sort.SliceStable(open, func(i, j int) bool {
		a, b := open[i], open[j]
		// Never-expiring last.
		switch {
		case a.ExpiresAt == nil && b.ExpiresAt == nil:
		case a.ExpiresAt == nil:
			return false
		case b.ExpiresAt == nil:
			return true
		default:
			if !a.ExpiresAt.Equal(*b.ExpiresAt) {
				return a.ExpiresAt.Before(*b.ExpiresAt)
			}
		}
		return a.ID < b.ID
	})
	return open
}

// availableAt is the total spendable value of a set of lots.
func availableAt(lots []LotBalance, now time.Time) int64 {
	return AvailableInLots(lots, now)
}

// ── the accumulator bug, named ───────────────────────────────────────────────

// TestAllocateFEFOSpendOf120AcrossThreeBuckets is the documented case:
// "spending 120 consumed 100 from FREE and 20 from EARNED, leaving PURCHASED
// untouched" (02-architecture.md §4).
//
// It is an example test on purpose — it pins the spec's own numbers so a change
// in FEFO behaviour is visible as a failure against a known expected value — and
// it doubles as the regression test for the accumulator bug. 145 is the wrong
// answer the original implementation produced; the assertion that makes that
// impossible is the exact 100/20/0 split plus the total.
func TestAllocateFEFOSpendOf120AcrossThreeBuckets(t *testing.T) {
	alloc, err := AllocateFEFO(threeBucketLots(), 120, testNow)
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}

	want := []LotConsumption{{LotID: 1, Amount: 100}, {LotID: 2, Amount: 20}}
	if len(alloc.Consumptions) != len(want) {
		t.Fatalf("got %d consumptions %+v, want %d %+v", len(alloc.Consumptions), alloc.Consumptions, len(want), want)
	}
	for i, w := range want {
		if alloc.Consumptions[i] != w {
			t.Errorf("consumption %d = %+v, want %+v", i, alloc.Consumptions[i], w)
		}
	}
	if alloc.Taken != 120 {
		t.Errorf("Taken = %d, want 120. A total other than the request is the accumulator bug: "+
			"the loop must do taken += inc and post the per-lot increment, never the running total.", alloc.Taken)
	}
	if got := alloc.Total(); got != 120 {
		t.Errorf("Total() = %d, want 120", got)
	}
	// The never-expiring lot is untouched, and the spent-from list is FEFO order.
	if len(alloc.Consumptions) == 3 || alloc.Consumptions[len(alloc.Consumptions)-1].LotID == 3 {
		t.Error("the never-expiring lot must be left alone when expiring lots still cover the spend")
	}
}

// TestPostingSpendWritesPerLotIncrementsNotTheRunningTotal is the direct
// regression test for the failure in 02-architecture.md §4.
//
// The bug is not visible in the total, and it is not visible in the journal: the
// posting legs still sum to zero, because the sink leg was written from the same
// wrong number. It is visible in exactly one place — the amount of each user leg
// versus the amount actually burned from that lot. So this asserts the legs
// themselves, leg by leg, and separately asserts the four totals that all have to
// agree: user legs, sink leg, net, and the per-lot deltas.
func TestPostingSpendWritesPerLotIncrementsNotTheRunningTotal(t *testing.T) {
	lots := threeBucketLots()
	alloc, err := AllocateFEFO(lots, 120, testNow)
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	const sink = 99

	legs, spentFrom := postingSpend(alloc, indexLots(lots), sink)
	if len(legs) != 3 {
		t.Fatalf("got %d legs, want 3 (two lot legs plus the system leg)", len(legs))
	}

	// Leg 1 is the whole 100 off the FREE account. Leg 2 is 20, NOT 120, off the
	// EARNED account — posting 120 there is the overcharge.
	wantLegs := []struct {
		seq    int16
		acct   uint
		amount int64
		lotID  uint
	}{
		{1, 10, -100, 1},
		{2, 11, -20, 2},
		{3, sink, 120, 0},
	}
	for i, w := range wantLegs {
		got := legs[i]
		if got.Seq != w.seq || got.AccountID != w.acct || got.Amount != w.amount {
			t.Errorf("leg %d = {seq:%d acct:%d amount:%d}, want {seq:%d acct:%d amount:%d}",
				i, got.Seq, got.AccountID, got.Amount, w.seq, w.acct, w.amount)
		}
		if w.lotID == 0 {
			if got.LotID != nil {
				t.Errorf("the system leg must not be attributed to a lot, got lot %d", *got.LotID)
			}
			continue
		}
		if got.LotID == nil || *got.LotID != w.lotID {
			t.Errorf("leg %d lot = %v, want %d", i, got.LotID, w.lotID)
		}
	}

	// Every total that has to agree, asserted independently so a reader can see
	// which one broke.
	var userLegs, net int64
	for _, l := range legs {
		net += l.Amount
		if l.AccountID != sink {
			userLegs += l.Amount
		}
	}
	if userLegs != -120 {
		t.Errorf("user legs sum to %d, want -120. A sum of -145 is the accumulator bug and it "+
			"would leave the cache and the lots disagreeing while the journal still netted to zero.", userLegs)
	}
	if net != 0 {
		t.Errorf("legs sum to %d, want 0: every journal must net to exactly zero", net)
	}
	if alloc.Total() != 120 {
		t.Errorf("alloc.Total() = %d, want 120", alloc.Total())
	}

	// The cache delta the caller will apply must be exactly the spend, and the
	// per-account deltas must be the per-lot increments.
	posted := map[uint]int64{}
	for _, c := range alloc.Consumptions {
		lot := lots[c.LotID-1]
		posted[lot.AccountID] -= c.Amount
	}
	posted[sink] += alloc.Total()
	if posted[10] != -100 || posted[11] != -20 || posted[sink] != 120 {
		t.Errorf("cache deltas %+v, want account 10 = -100, account 11 = -20, sink = +120", posted)
	}

	// And the response body's spent_from agrees with the legs.
	if len(spentFrom) != 2 || spentFrom[0].Amount != 100 || spentFrom[1].Amount != 20 {
		t.Errorf("spentFrom = %+v, want [100, 20]", spentFrom)
	}
}

// ── table-driven behaviour ───────────────────────────────────────────────────

func TestAllocateFEFOTableDriven(t *testing.T) {
	tests := []struct {
		name    string
		lots    []LotBalance
		amount  int64
		want    []LotConsumption
		wantErr error
	}{
		{
			name:   "single lot covers it exactly",
			lots:   []LotBalance{lot(1, 10, BucketEarned, 100, 0, intp(365))},
			amount: 100,
			want:   []LotConsumption{{LotID: 1, Amount: 100}},
		},
		{
			name:   "a single lot partially covers it",
			lots:   []LotBalance{lot(1, 10, BucketEarned, 100, 0, intp(365))},
			amount: 30,
			want:   []LotConsumption{{LotID: 1, Amount: 30}},
		},
		{
			name: "FEFO crosses buckets by expiry, not by bucket",
			lots: []LotBalance{
				lot(1, 10, BucketEarned, 50, 0, intp(365)),
				lot(2, 11, BucketFree, 100, 0, intp(30)),
			},
			amount: 120,
			want:   []LotConsumption{{LotID: 2, Amount: 100}, {LotID: 1, Amount: 20}},
		},
		{
			name: "equal expiries fall back to lot id",
			lots: []LotBalance{
				lot(9, 10, BucketEarned, 10, 0, intp(30)),
				lot(4, 10, BucketEarned, 10, 0, intp(30)),
			},
			amount: 15,
			want:   []LotConsumption{{LotID: 4, Amount: 10}, {LotID: 9, Amount: 5}},
		},
		{
			name: "an already-partly-consumed lot contributes only its remainder",
			lots: []LotBalance{
				lot(1, 10, BucketEarned, 100, 70, intp(30)),
				lot(2, 10, BucketEarned, 100, 0, intp(60)),
			},
			amount: 50,
			want:   []LotConsumption{{LotID: 1, Amount: 30}, {LotID: 2, Amount: 20}},
		},
		{
			name:   "an expired lot is invisible to FEFO",
			lots:   []LotBalance{lot(1, 10, BucketFree, 100, 0, intp(-1))},
			amount: 10,
			// The only lot expired, so there is nothing to spend: the error names
			// the shortfall rather than silently re-granting.
			wantErr: ErrInsufficientCoins,
		},
		{
			name:   "a fully consumed lot is skipped",
			lots:   []LotBalance{lot(1, 10, BucketEarned, 100, 100, intp(30))},
			amount: 10,
			want:   []LotConsumption{},
			// Nothing open at all: ErrInsufficientCoins with zero available.
			wantErr: ErrInsufficientCoins,
		},
		{
			name: "one more than the total is refused and changes nothing",
			lots: []LotBalance{
				lot(1, 10, BucketFree, 100, 0, intp(30)),
				lot(2, 11, BucketEarned, 25, 0, nil),
			},
			amount:  126,
			wantErr: ErrInsufficientCoins,
		},
		{
			name:    "zero is rejected before any arithmetic",
			lots:    threeBucketLots(),
			amount:  0,
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "negative is rejected before any arithmetic",
			lots:    threeBucketLots(),
			amount:  -1,
			wantErr: ErrInvalidArgument,
		},
		{
			name:    "no lots at all",
			lots:    nil,
			amount:  1,
			wantErr: ErrInsufficientCoins,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]LotBalance(nil), tc.lots...)

			alloc, err := AllocateFEFO(tc.lots, tc.amount, testNow)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error %v, want one matching %v", err, tc.wantErr)
				}
				// A refused allocation carries nothing at all. There is no
				// partial result for a caller to apply by mistake.
				if len(alloc.Consumptions) != 0 || alloc.Taken != 0 {
					t.Fatalf("a refused allocation must be empty, got taken=%d consumptions=%+v",
						alloc.Taken, alloc.Consumptions)
				}
			} else {
				if err != nil {
					t.Fatalf("allocate: %v", err)
				}
				if len(alloc.Consumptions) != len(tc.want) {
					t.Fatalf("got %+v, want %+v", alloc.Consumptions, tc.want)
				}
				for i := range tc.want {
					if alloc.Consumptions[i] != tc.want[i] {
						t.Errorf("consumption %d = %+v, want %+v", i, alloc.Consumptions[i], tc.want[i])
					}
				}
				if alloc.Taken != tc.amount {
					t.Errorf("Taken = %d, want the requested %d", alloc.Taken, tc.amount)
				}
			}

			// The input is never mutated, on any path. A caller that passes its
			// live slice and gets it back reordered has a subtle bug waiting.
			if len(before) != len(tc.lots) {
				t.Fatalf("input length changed: %d -> %d", len(before), len(tc.lots))
			}
			for i := range before {
				if before[i] != tc.lots[i] {
					t.Fatalf("input lot %d was mutated: %+v -> %+v", i, before[i], tc.lots[i])
				}
			}
		})
	}
}

// ── the properties ───────────────────────────────────────────────────────────

// TestAllocateFEFOProperties is the randomised corpus. Every property in
// 04-implementation-plan.md §3.3 is asserted here, over generated lot shapes and
// generated amounts, and additionally after every spend in a generated SEQUENCE of
// spends, because the accumulator bug only shows up once a lot has been partially
// burned: a single spend from fresh lots cannot distinguish `taken += inc` from
// `taken = inc` when the first lot always covers the whole request.
func TestAllocateFEFOProperties(t *testing.T) {
	const (
		seeds        = 60
		lotsPerShape = 7
		spendsPerSeq = 12
	)
	for seed := 1; seed <= seeds; seed++ {
		rng := rand.New(rand.NewSource(int64(seed)))
		shape := randomLotShape(rng, lotsPerShape)
		total := availableAt(shape, testNow)

		// Go 1.22+ gives each iteration its own loop variable, so seed and shape
		// are captured correctly and do not need re-declaring.
		t.Run("seed_"+itoa(seed), func(t *testing.T) {
			t.Run("amounts across the whole range", func(t *testing.T) {
				for _, amount := range amountsToTry(rng, total) {
					assertAllocationProperties(t, shape, amount, total)
				}
			})

			t.Run("a sequence of spends", func(t *testing.T) {
				// Mirrored state: what the database would hold after each spend.
				state := append([]LotBalance(nil), shape...)
				// Lots in the generated shape may already be partly consumed, so
				// the running total starts from what they had given up, not from
				// zero. Without this the cross-check below compares two different
				// things and fails on the first spend of every seed.
				spent := consumedTotal(state)

				for i := 0; i < spendsPerSeq; i++ {
					available := availableAt(state, testNow)
					if available <= 0 {
						break
					}
					amount := int64(rng.Intn(int(available)) + 1)
					alloc, err := AllocateFEFO(state, amount, testNow)
					if err != nil {
						t.Fatalf("spend %d of %d: %v", i, amount, err)
					}
					state = applyAllocation(state, alloc)
					spent += amount

					// Every property, re-asserted against the mutated state. This
					// is where `taken = inc` dies: after the first lot is partly
					// burned, the arithmetic diverges from the request.
					assertStateProperties(t, state)
					if got := consumedTotal(state); got != spent {
						t.Fatalf("after %d spends the lots have given up %d but %d were spent; "+
							"the allocator's running total and the amounts burned from the lots disagree",
							i+1, got, spent)
					}
				}

				// Draining the remainder must leave every lot fully consumed and
				// the two totals equal.
				if available := availableAt(state, testNow); available > 0 {
					alloc, err := AllocateFEFO(state, available, testNow)
					if err != nil {
						t.Fatalf("drain the remainder (%d): %v", available, err)
					}
					state = applyAllocation(state, alloc)
				}
				for _, l := range state {
					// Open lots must be drained. A lot that was already closed or
					// that has expired was never spendable, so it is left alone by
					// design.
					if !l.openAt(testNow) {
						continue
					}
					if l.Consumed != l.Granted {
						t.Errorf("open lot %d is left at consumed=%d of granted=%d after draining; "+
							"spending exactly the available total must leave every lot fully consumed",
							l.ID, l.Consumed, l.Granted)
					}
				}
			})

			t.Run("independent of the order rows come back in", func(t *testing.T) {
				for _, amount := range amountsToTry(rand.New(rand.NewSource(7)), total) {
					if amount == 0 {
						continue
					}
					want, wantErr := AllocateFEFO(shape, amount, testNow)
					for shuffle := 0; shuffle < 8; shuffle++ {
						shuffled := shuffled(shape, rand.New(rand.NewSource(int64(shuffle)+int64(seed))))
						got, gotErr := AllocateFEFO(shuffled, amount, testNow)
						if (wantErr == nil) != (gotErr == nil) {
							t.Fatalf("shuffled input for amount %d changed the outcome: %v vs %v",
								amount, wantErr, gotErr)
						}
						if wantErr != nil {
							continue
						}
						if len(got.Consumptions) != len(want.Consumptions) {
							t.Fatalf("shuffled input for amount %d gave %+v, want %+v",
								amount, got.Consumptions, want.Consumptions)
						}
						for i := range want.Consumptions {
							if got.Consumptions[i] != want.Consumptions[i] {
								t.Fatalf("shuffled input for amount %d gave %+v, want %+v: "+
									"the allocation must not depend on the physical row order",
									amount, got.Consumptions, want.Consumptions)
							}
						}
					}
				}
			})
		})
	}
}

// assertStateProperties asserts the invariants that must hold of a lot set at
// any point in a sequence of spends, independently of any one allocation.
func assertStateProperties(t *testing.T, lots []LotBalance) {
	t.Helper()
	for _, l := range lots {
		if l.Consumed < 0 {
			t.Fatalf("lot %d has consumed=%d, which is negative", l.ID, l.Consumed)
		}
		if l.Consumed > l.Granted {
			t.Fatalf("lot %d reached consumed=%d of granted=%d: no lot is ever over-consumed",
				l.ID, l.Consumed, l.Granted)
		}
	}
}

// assertAllocationProperties is the body every generated case goes through.
func assertAllocationProperties(t *testing.T, lots []LotBalance, amount, totalAvailable int64) {
	t.Helper()

	// Property: a non-positive request is rejected before any arithmetic, so a
	// zero-amount spend can never produce a posting.
	if amount <= 0 {
		alloc, err := AllocateFEFO(lots, amount, testNow)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("amount %d: error %v, want ErrInvalidArgument", amount, err)
		}
		if len(alloc.Consumptions) != 0 || alloc.Taken != 0 {
			t.Fatalf("a rejected amount produced consumptions %+v", alloc.Consumptions)
		}
		return
	}

	alloc, err := AllocateFEFO(lots, amount, testNow)

	// Property: a request the lots cannot cover changes nothing, and says so with
	// the one sentinel the funnel is designed around.
	if amount > totalAvailable {
		if !errors.Is(err, ErrInsufficientCoins) {
			t.Fatalf("amount %d with %d available: error %v, want ErrInsufficientCoins", amount, totalAvailable, err)
		}
		if len(alloc.Consumptions) != 0 || alloc.Taken != 0 {
			t.Fatalf("an unaffordable request changed something: taken=%d consumptions=%+v",
				alloc.Taken, alloc.Consumptions)
		}
		return
	}
	if err != nil {
		t.Fatalf("amount %d with %d available should be affordable: %v", amount, totalAvailable, err)
	}

	// Property: sum(consumed_delta) == requested_amount, always.
	var sum int64
	for _, c := range alloc.Consumptions {
		sum += c.Amount
	}
	if sum != amount {
		t.Fatalf("sum(consumed_delta) = %d, want the requested %d. This is the accumulator bug: "+
			"taken += inc, and post the per-lot increment.", sum, amount)
	}
	if alloc.Taken != amount {
		t.Fatalf("Allocation.Taken = %d, want %d", alloc.Taken, amount)
	}
	if alloc.Requested != amount {
		t.Fatalf("Allocation.Requested = %d, want %d", alloc.Requested, amount)
	}

	byID := indexLots(lots)

	// Property: no lot is ever over-consumed, and no leg is ever zero. A zero leg
	// would violate CHECK (amount <> 0) on coin_posting.
	for _, c := range alloc.Consumptions {
		if c.Amount <= 0 {
			t.Fatalf("lot %d has a consumption of %d; a zero or negative leg is never written", c.LotID, c.Amount)
		}
		lot := byID[c.LotID]
		if lot.Consumed+c.Amount > lot.Granted {
			t.Fatalf("lot %d would reach consumed=%d of granted=%d",
				c.LotID, lot.Consumed+c.Amount, lot.Granted)
		}
	}
	// And the state after applying it is still legal.
	assertStateProperties(t, applyAllocation(append([]LotBalance(nil), lots...), alloc))

	// Property: the touched lots are exactly the FEFO prefix of the open-lot
	// ordering, and the loop stops the instant the request is satisfied.
	order := referenceFEFO(lots, testNow)
	var covered int64
	expected := 0
	for _, l := range order {
		if covered >= amount {
			break
		}
		inc := l.Granted - l.Consumed
		if remaining := amount - covered; inc > remaining {
			inc = remaining
		}
		if inc <= 0 {
			continue
		}
		covered += inc
		expected++
	}
	if len(alloc.Consumptions) != expected {
		t.Fatalf("touched %d lots, want the FEFO prefix of length %d: %+v over %+v",
			len(alloc.Consumptions), expected, alloc.Consumptions, order)
	}
	prefix := make([]uint, 0, expected)
	for i := 0; i < expected; i++ {
		prefix = append(prefix, order[i].ID)
	}
	for i, c := range alloc.Consumptions {
		if c.LotID != prefix[i] {
			t.Fatalf("consumption %d burned lot %d, want lot %d: the touched set must be the FEFO prefix %v",
				i, c.LotID, prefix[i], prefix)
		}
	}

	// Property: never-expiring lots are touched last. If any touched lot never
	// expires, then no untouched lot may expire.
	touched := make(map[uint]bool, len(alloc.Consumptions))
	for _, c := range alloc.Consumptions {
		touched[c.LotID] = true
	}
	for _, l := range order {
		if !touched[l.ID] {
			continue
		}
		if l.ExpiresAt == nil {
			for _, later := range order {
				if !touched[later.ID] && later.ExpiresAt != nil {
					t.Fatalf("lot %d (never expires) was burned while lot %d (expires %v) was left: "+
						"never-expiring coins must be touched last",
						l.ID, later.ID, later.ExpiresAt.Format(time.RFC3339))
				}
			}
		}
	}

	// Property: exactly the available total leaves every lot fully consumed. Only
	// the OPEN lots are covered by that claim: a lot that was already closed, or
	// that has expired and is invisible to FEFO, was never spendable and so is
	// not expected to be drained. Asserting on those would be asserting that
	// expiry does not work.
	if amount == totalAvailable {
		exhausted := indexLots(applyAllocation(append([]LotBalance(nil), lots...), alloc))
		for _, l := range order {
			got := exhausted[l.ID]
			if got.Consumed != got.Granted {
				t.Fatalf("open lot %d left at consumed=%d of granted=%d after spending the whole available %d",
					l.ID, got.Consumed, got.Granted, amount)
			}
		}
	}
}

// applyAllocation mirrors what applySpendDeltas does to the database, so the
// sequence test can assert the properties against state that has already been
// spent from. It returns the mutated copy.
func applyAllocation(lots []LotBalance, alloc Allocation) []LotBalance {
	byID := indexLots(lots)
	for _, c := range alloc.Consumptions {
		lot := byID[c.LotID]
		lot.Consumed += c.Amount
		byID[c.LotID] = lot
	}
	out := make([]LotBalance, 0, len(lots))
	for _, l := range lots {
		out = append(out, byID[l.ID])
	}
	return out
}

func consumedTotal(lots []LotBalance) int64 {
	var total int64
	for _, l := range lots {
		total += l.Consumed
	}
	return total
}

// randomLotShape generates a lot set with a mix of expiries, never-expiring lots,
// already-partly-consumed lots and at least one lot that is already closed or
// expired, so the corpus includes the skip paths and not only the happy one.
func randomLotShape(rng *rand.Rand, n int) []LotBalance {
	lots := make([]LotBalance, 0, n)
	granted := int64(0)
	for i := 0; i < n; i++ {
		amount := int64(rng.Intn(60) + 1)
		var expiresInDays *int
		switch rng.Intn(4) {
		case 0:
			expiresInDays = nil // never expires
		case 1:
			d := rng.Intn(90) + 1
			expiresInDays = &d
		case 2:
			d := -(rng.Intn(10) + 1) // already expired: must be invisible
			expiresInDays = &d
		default:
			d := rng.Intn(400) + 1
			expiresInDays = &d
		}
		consumed := int64(0)
		if rng.Intn(3) == 0 {
			// Partly consumed, sometimes fully.
			consumed = int64(rng.Intn(int(amount) + 1))
		}
		// Two accounts so the multi-account shape is exercised, matching a user
		// with a FREE and an EARNED account.
		accountID := uint(10 + i%2)
		bucket := BucketFree
		if accountID == 11 {
			bucket = BucketEarned
		}
		lots = append(lots, lot(uint(100+i), accountID, bucket, amount, consumed, expiresInDays))
		if expiresInDays == nil || *expiresInDays > 0 {
			if amount > consumed {
				granted += amount - consumed
			}
		}
	}
	return lots
}

// amountsToTry spans the boundaries that break accumulators: 1, the exact total,
// one over, one under, and a spread in between.
func amountsToTry(rng *rand.Rand, total int64) []int64 {
	amounts := []int64{0, 1, 2, total, total - 1, total + 1, total * 2}
	if total > 4 {
		amounts = append(amounts, total/2, total/2+1, total/3)
	}
	for i := 0; i < 12; i++ {
		amounts = append(amounts, int64(rng.Intn(int(total)+2)))
	}
	return amounts
}

func shuffled(lots []LotBalance, rng *rand.Rand) []LotBalance {
	out := append([]LotBalance(nil), lots...)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

// ── never-expiring last, asserted on its own ─────────────────────────────────

// TestAllocateFEFONeverExpiringLotsAreTouchedLast states the consumer-favourable
// half of FEFO directly: a user must never watch coins evaporate while untouched
// permanent ones sit in their balance. 02-architecture.md §4 calls this out as
// the reason FEFO is chosen over LIFO, so it gets its own test rather than being
// left to the generated corpus.
func TestAllocateFEFONeverExpiringLotsAreTouchedLast(t *testing.T) {
	lots := []LotBalance{
		lot(1, 12, BucketEarned, 25, 0, nil), // never expires
		lot(2, 11, BucketEarned, 50, 0, intp(365)),
		lot(3, 10, BucketFree, 100, 0, intp(30)),
		lot(4, 13, BucketEarned, 10, 0, nil), // never expires
	}

	// 30 consumes the FREE lot alone.
	alloc, err := AllocateFEFO(lots, 30, testNow)
	if err != nil {
		t.Fatalf("allocate 30: %v", err)
	}
	if len(alloc.Consumptions) != 1 || alloc.Consumptions[0].LotID != 3 {
		t.Fatalf("spending 30 burned %+v, want only the 30-day FREE lot (id 3)", alloc.Consumptions)
	}

	// 130 takes the FREE 100, the 365-day 30, and stops: neither never-expiring
	// lot is touched even though 35 coins of permanent value are available.
	alloc, err = AllocateFEFO(lots, 130, testNow)
	if err != nil {
		t.Fatalf("allocate 130: %v", err)
	}
	want := []LotConsumption{{LotID: 3, Amount: 100}, {LotID: 2, Amount: 30}}
	if len(alloc.Consumptions) != len(want) {
		t.Fatalf("spending 130 burned %+v, want %+v: never-expiring lots come last", alloc.Consumptions, want)
	}
	for i := range want {
		if alloc.Consumptions[i] != want[i] {
			t.Errorf("consumption %d = %+v, want %+v", i, alloc.Consumptions[i], want[i])
		}
	}

	// Only when there is nothing else does a never-expiring lot get used.
	alloc, err = AllocateFEFO(lots, 185, testNow)
	if err != nil {
		t.Fatalf("allocate 185: %v", err)
	}
	if len(alloc.Consumptions) != 4 {
		t.Fatalf("spending 185 burned %+v, want all four lots including the never-expiring ones", alloc.Consumptions)
	}
	last := alloc.Consumptions[len(alloc.Consumptions)-1]
	if last.LotID != 1 && last.LotID != 4 {
		t.Errorf("the last lot burned is %d, want a never-expiring lot (1 or 4)", last.LotID)
	}
}

// TestAllocateFEFOSpendExactlyAvailableClosesEveryLot is the boundary the
// property list calls out separately, stated on a fixed shape: spending exactly
// the available total must consume every open lot completely, with no remainder
// stranded in a lot.
func TestAllocateFEFOSpendExactlyAvailableClosesEveryLot(t *testing.T) {
	lots := []LotBalance{
		lot(1, 10, BucketFree, 40, 0, intp(30)),
		lot(2, 10, BucketFree, 25, 10, intp(30)), // 15 left
		lot(3, 11, BucketEarned, 7, 0, intp(365)),
		lot(4, 12, BucketEarned, 33, 0, nil),
		lot(5, 12, BucketEarned, 50, 50, intp(365)), // already closed, contributes nothing
	}
	available := availableAt(lots, testNow)
	if available != 40+15+7+33 {
		t.Fatalf("available = %d, want %d", available, 40+15+7+33)
	}

	alloc, err := AllocateFEFO(lots, available, testNow)
	if err != nil {
		t.Fatalf("allocate the whole balance: %v", err)
	}
	after := applyAllocation(append([]LotBalance(nil), lots...), alloc)
	for _, l := range after {
		if l.Consumed != l.Granted {
			t.Errorf("lot %d left at consumed=%d of granted=%d", l.ID, l.Consumed, l.Granted)
		}
	}
	if got := availableAt(after, testNow); got != 0 {
		t.Errorf("available after draining = %d, want 0", got)
	}
	if len(alloc.Consumptions) != 4 {
		t.Errorf("touched %d lots, want the 4 with value (the already-closed lot is skipped)", len(alloc.Consumptions))
	}
}

// TestAvailableInLotsIgnoresExpiredAndClosed is the availability mirror of the
// skip rules, asserted directly because the spend path uses it to report a
// shortfall and a wrong answer there is a wrong error message in the funnel.
func TestAvailableInLotsIgnoresExpiredAndClosed(t *testing.T) {
	lots := []LotBalance{
		lot(1, 10, BucketFree, 100, 0, intp(30)),
		lot(2, 10, BucketFree, 100, 100, intp(30)), // closed
		lot(3, 10, BucketFree, 100, 0, intp(-1)),   // expired
		lot(4, 11, BucketEarned, 40, 15, nil),      // 25 left
	}
	if got, want := AvailableInLots(lots, testNow), int64(125); got != want {
		t.Fatalf("AvailableInLots = %d, want %d", got, want)
	}
}

// ── the pure helpers around the allocator ────────────────────────────────────

// TestFingerprintIsStableAndDiscriminating covers the idempotency comparison.
// A fingerprint that is not stable makes every retry look like a collision; one
// that does not discriminate lets two different requests share a journal, which
// is the silent half of the §12.2 failure.
func TestFingerprintIsStableAndDiscriminating(t *testing.T) {
	base := fingerprint("SPEND", "42", "RESOURCE_UNLOCK", "study_resource:812", "40")

	if string(fingerprint("SPEND", "42", "RESOURCE_UNLOCK", "study_resource:812", "40")) != string(base) {
		t.Error("the same request must produce the same fingerprint")
	}
	if len(base) != 32 {
		t.Errorf("fingerprint is %d bytes, want a 32-byte sha256", len(base))
	}

	// Every field is load-bearing, including the resolved amount and the user.
	for _, other := range [][]string{
		{"SPEND", "43", "RESOURCE_UNLOCK", "study_resource:812", "40"},
		{"GRANT", "42", "RESOURCE_UNLOCK", "study_resource:812", "40"},
		{"SPEND", "42", "RESOURCE_UNLOCK", "video:812", "40"},
		{"SPEND", "42", "RESOURCE_UNLOCK", "study_resource:813", "40"},
		{"SPEND", "42", "RESOURCE_UNLOCK", "study_resource:812", "60"},
		{"SPEND", "42", "RESOURCE_UNLOCK", "study_resource", "40"},
	} {
		if string(fingerprint(other...)) == string(base) {
			t.Errorf("fingerprint collides with the base for %q", other)
		}
	}

	// Field boundaries cannot be forged by moving a separator: the NUL separator
	// is what stops ["ab","c"] and ["a","bc"] hashing the same.
	if string(fingerprint("ab", "c")) == string(fingerprint("a", "bc")) {
		t.Error("field boundaries are ambiguous in the fingerprint")
	}
}

// TestSpendPriceResolvesFromConfigNotTheCaller pins the §12.1 rule in the one
// place a caller could get around it: the price is a function of the config and
// the class, and an unknown class is a rejection rather than a zero.
func TestSpendPriceResolvesFromConfigNotTheCaller(t *testing.T) {
	cfg := DefaultEconomyConfig()
	cfg.Prices.StudyResource = 41
	cfg.Prices.Video = 91
	cfg.Prices.MockTest = 61

	for _, tc := range []struct {
		reasonCode string
		refType    string
		want       int64
	}{
		{ReasonResourceUnlock, RefStudyResource, 41},
		{ReasonResourceUnlock, RefVideo, 91},
		{ReasonResourceUnlock, RefMockTest, 61},
	} {
		got, err := spendPrice(cfg, tc.reasonCode, tc.refType)
		if err != nil {
			t.Fatalf("spendPrice(%s, %s): %v", tc.reasonCode, tc.refType, err)
		}
		if got != tc.want {
			t.Errorf("spendPrice(%s, %s) = %d, want %d", tc.reasonCode, tc.refType, got, tc.want)
		}
	}

	for _, tc := range []struct{ reasonCode, refType string }{
		{ReasonResourceUnlock, "study_resources"},
		{ReasonResourceUnlock, ""},
		{ReasonResourceUnlock, "PURCHASED"},
		{ReasonGrantReversal, RefVideo},
	} {
		if _, err := spendPrice(cfg, tc.reasonCode, tc.refType); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("spendPrice(%q, %q) = %v, want ErrInvalidArgument", tc.reasonCode, tc.refType, err)
		}
	}
}

// TestGrantSpecsResolveAmountAndBucketFromConfig is the same rule on the grant
// side: the bucket is resolved too, because the bucket is what chooses the
// expiry, and a caller that could name FREE would be choosing a 30-day lifetime
// over a 365-day one.
func TestGrantSpecsResolveAmountAndBucketFromConfig(t *testing.T) {
	cfg := DefaultEconomyConfig()

	for reason, wantAmount := range map[string]int64{
		ReasonProfileComplete:   cfg.Awards.ProfileInstalment,
		ReasonReferralQualified: cfg.Awards.ReferralReferrer,
		ReasonResourceApproved:  cfg.Awards.ResourceApproved,
	} {
		spec, ok := grantSpecs[reason]
		if !ok {
			t.Fatalf("%s has no grant spec, so it cannot be granted at all", reason)
		}
		if got := spec.amount(cfg); got != wantAmount {
			t.Errorf("%s amount = %d, want %d from the config", reason, got, wantAmount)
		}
		if spec.bucket != BucketEarned {
			t.Errorf("%s bucket = %s, want %s", reason, spec.bucket, BucketEarned)
		}
		if got := spec.expiryDays(cfg); got != cfg.Expiry.EarnedDays {
			t.Errorf("%s expiry = %d days, want %d from the config", reason, got, cfg.Expiry.EarnedDays)
		}
	}

	// The profile award is paid in instalments, so a single grant must be the
	// instalment and never the whole ladder. Paying the total in one journal
	// would make a mid-ladder failure unresumable.
	if grantSpecs[ReasonProfileComplete].amount(cfg) >= cfg.Awards.ProfileComplete {
		t.Error("the profile grant must pay one instalment, not the whole ladder")
	}

	// A reason code that is not a grant has no spec, which is what makes an
	// unknown code a rejection instead of a silent zero.
	if _, ok := grantSpecs[ReasonResourceUnlock]; ok {
		t.Error("a spend reason must not be grantable")
	}
	if _, ok := grantSpecs[ReasonReferralHold]; ok {
		t.Error("a hold must not be a grant: a hold posts nothing")
	}
}

// TestHoldReasonsAreExactlyThePostingFreeOnes keeps the invariant test's
// enumeration honest. If a posting-free reason is added without being listed, the
// "every journal nets to zero" assertion quietly stops covering it.
func TestHoldReasonsAreExactlyThePostingFreeOnes(t *testing.T) {
	if !IsHoldReason(ReasonReferralHold) {
		t.Error(ReasonReferralHold + " must be a hold reason")
	}
	for _, reason := range []string{
		EntryGrant, EntrySpend, EntryExpire, EntryReversal, EntryAdjust, ReasonResourceUnlock,
	} {
		if IsHoldReason(reason) {
			t.Errorf("%s moves coins and must not be a hold reason", reason)
		}
	}
}

// TestValidateRefMatchesTheCheckConstraint keeps the Go guard in step with
// CHECK (ref_type IS NULL OR ref_id IS NOT NULL), which is the constraint the
// database would raise three statements later.
func TestValidateRefMatchesTheCheckConstraint(t *testing.T) {
	refID := func(v uint64) *uint64 { return &v }
	refType := func(v string) *string { return &v }

	if err := validateRef(nil, nil); err != nil {
		t.Errorf("no ref at all is allowed: %v", err)
	}
	if err := validateRef(refType(RefStudyResource), refID(812)); err != nil {
		t.Errorf("a ref_type with a ref_id is allowed: %v", err)
	}
	if err := validateRef(refType(RefStudyResource), nil); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a ref_type with no ref_id must be refused: %v", err)
	}
	if err := validateRef(refType(RefStudyResource), refID(0)); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a ref_type with a zero ref_id must be refused: %v", err)
	}
	if err := validateRef(nil, refID(812)); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a ref_id with no ref_type must be refused: %v", err)
	}
}

// TestValidateIdempotencyKeyRejectsTheEmptyKey ties the ledger to
// 03-api-contract.md §2.3, where a missing Idempotency-Key is a 400 rather than a
// silently non-idempotent write.
func TestValidateIdempotencyKeyRejectsTheEmptyKey(t *testing.T) {
	for _, key := range []string{"", "   ", "\t\n"} {
		if err := validateIdempotencyKey(key); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("key %q = %v, want ErrInvalidArgument", key, err)
		}
	}
	if err := validateIdempotencyKey("9f2c-unlock-812"); err != nil {
		t.Errorf("a real key must be accepted: %v", err)
	}
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'k'
	}
	if err := validateIdempotencyKey(string(long)); !errors.Is(err, ErrInvalidArgument) {
		t.Error("a key longer than the column must be refused before the database refuses it")
	}
}

// TestSumAvailableSubtractsReserved states the one line the whole hold mechanism
// rests on, with the numbers the task requires: posted=200, reserved=150 must
// leave 50 available.
func TestSumAvailableSubtractsReserved(t *testing.T) {
	balances := map[uint]AccountBalance{
		10: {AccountID: 10, PostedBalance: 200, Reserved: 150},
	}
	if got := balances[10].Available(); got != 50 {
		t.Errorf("Available() = %d, want 50", got)
	}
	if got := sumAvailable(balances); got != 50 {
		t.Errorf("sumAvailable = %d, want 50", got)
	}

	// A hold on one bucket is not spendable from another: the sum is over
	// posted - reserved per account, never over a total of posted minus a total
	// of reserved computed separately from a different set of accounts.
	balances[11] = AccountBalance{AccountID: 11, PostedBalance: 80, Reserved: 0}
	if got := sumAvailable(balances); got != 130 {
		t.Errorf("sumAvailable across two buckets = %d, want 130", got)
	}
}

// TestFrozenAccountsIsDeterministic: a 423 that names a different account on each
// run is useless in a log, and the account set is a map.
func TestFrozenAccountsIsDeterministic(t *testing.T) {
	balances := map[uint]AccountBalance{
		30: {AccountID: 30, Closed: true},
		10: {AccountID: 10, Closed: true},
		20: {AccountID: 20},
	}
	for i := 0; i < 20; i++ {
		frozen, id := frozenAccounts(balances)
		if !frozen || id != 10 {
			t.Fatalf("frozenAccounts = %v, %d; want true, 10", frozen, id)
		}
	}
	if frozen, _ := frozenAccounts(map[uint]AccountBalance{20: {AccountID: 20}}); frozen {
		t.Error("an open account must not be reported as frozen")
	}
}
