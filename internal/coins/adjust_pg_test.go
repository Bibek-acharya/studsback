//go:build coinsintegration

// internal/coins/adjust_pg_test.go
//
// 03 §3.2: the signed correction, against Postgres.
//
// The properties here are enforced by the DATABASE — the no-overdraft CHECK, the
// unique idempotency key, the append-only trigger on coin_posting — so SQLite
// cannot test any of them.

package coins

import (
	"context"
	"testing"
)

// THE test. A correction moves the balance, is attributable, is reversible in
// record, and leaves every invariant intact.
func TestAnAdjustmentMovesTheBalanceAndIsFullyAttributable(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	const student = 4242
	const adminID = 7

	// Start from a real grant so the student is not at zero — a correction applied to
	// an empty balance is the degenerate case and hides overdraft behaviour.
	env.grant(t, student, "adjust-base", ReasonProfileComplete, RefStudyResource, 913)
	before := env.available(t, student)

	result, err := env.ledger.Adjust(ctx, AdjustRequest{
		UserID:         student,
		Amount:         25,
		Reason:         AdjustGoodwill,
		CreatedBy:      "admin:7",
		IdempotencyKey: "adjust-key-1",
		Note:           "our refund flow missed their case",
	})
	if err != nil {
		t.Fatalf("adjust: %v", err)
	}
	if result.Amount != 25 || result.Replayed {
		t.Errorf("amount=%d replayed=%v, want 25 / false", result.Amount, result.Replayed)
	}
	if got := env.available(t, student); got != before+25 {
		t.Errorf("available = %d, want %d — a credit must move the balance", got, before+25)
	}

	// The journal is the audit record and must be complete: who, why, how much, which
	// direction, and the free-text note the closed-set reason cannot carry.
	var journal CoinJournal
	if err := env.pool.Where("id = ?", result.JournalID).First(&journal).Error; err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if journal.CreatedBy != "admin:7" {
		t.Errorf("created_by = %q, want %q", journal.CreatedBy, "admin:7")
	}
	if journal.ReasonCode != AdjustGoodwill {
		t.Errorf("reason_code = %q, want %q", journal.ReasonCode, AdjustGoodwill)
	}
	if journal.EntryType != EntryAdjust {
		t.Errorf("entry_type = %q, want %q", journal.EntryType, EntryAdjust)
	}
	if journal.Metadata["direction"] != "credit" {
		t.Errorf("direction metadata = %v, want %q", journal.Metadata["direction"], "credit")
	}
	if note, _ := journal.Metadata["admin_note"].(string); note == "" {
		t.Error("the operator's note was dropped; the reason code alone does not say what happened")
	}

	// Two legs netting to zero, user up and faucet down. The faucet leg is what makes
	// this a correction rather than a mint.
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
		t.Errorf("postings sum to %d, want 0", sum)
	}
	if legs[0].Amount != 25 || legs[1].Amount != -25 {
		t.Errorf("legs are %d and %d, want +25 and -25", legs[0].Amount, legs[1].Amount)
	}
}

// A replay must move nothing. An operator clicking twice on a support console is the
// normal case, not an edge case.
func TestAReplayedAdjustmentMovesNothing(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	const student = 4242
	env.grant(t, student, "replay-base", ReasonProfileComplete, RefStudyResource, 913)
	before := env.available(t, student)

	req := AdjustRequest{
		UserID: student, Amount: 40, Reason: AdjustGoodwill,
		CreatedBy: "admin:7", IdempotencyKey: "replay-key",
	}
	first, err := env.ledger.Adjust(ctx, req)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := env.ledger.Adjust(ctx, req)
	if err != nil {
		t.Fatalf("a replay returned %v; it must be a replay, not an error", err)
	}
	if !second.Replayed {
		t.Error("the second adjustment was not reported as a replay")
	}
	if second.JournalID != first.JournalID {
		t.Errorf("the replay returned journal %s, want the original %s", second.JournalID, first.JournalID)
	}
	if got := env.available(t, student); got != before+40 {
		t.Errorf("available = %d, want %d — a replay must move nothing", got, before+40)
	}
	var journals int64
	env.pool.Model(&CoinJournal{}).Where("idempotency_key = ?", "replay-key").Count(&journals)
	if journals != 1 {
		t.Errorf("%d journals for one key, want 1", journals)
	}
}

// The SAME key with a DIFFERENT payload is refused, not silently accepted. This is
// what stops two operators colliding on one key and one of them being told "already
// done" about a request that was never applied.
func TestAReusedKeyWithADifferentPayloadIsRefused(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	const student = 4242
	env.grant(t, student, "reuse-base", ReasonProfileComplete, RefStudyResource, 913)

	if _, err := env.ledger.Adjust(ctx, AdjustRequest{
		UserID: student, Amount: 40, Reason: AdjustGoodwill,
		CreatedBy: "admin:7", IdempotencyKey: "shared-key",
	}); err != nil {
		t.Fatalf("first: %v", err)
	}

	for _, tc := range []struct {
		name string
		req  AdjustRequest
	}{
		{"a different amount", AdjustRequest{UserID: student, Amount: 41, Reason: AdjustGoodwill, CreatedBy: "admin:7", IdempotencyKey: "shared-key"}},
		{"a different reason", AdjustRequest{UserID: student, Amount: 40, Reason: AdjustDataFix, CreatedBy: "admin:7", IdempotencyKey: "shared-key"}},
		{"a different AUTHOR", AdjustRequest{UserID: student, Amount: 40, Reason: AdjustGoodwill, CreatedBy: "admin:9", IdempotencyKey: "shared-key"}},
		{"a different user", AdjustRequest{UserID: 5151, Amount: 40, Reason: AdjustGoodwill, CreatedBy: "admin:7", IdempotencyKey: "shared-key"}},
	} {
		_, err := env.ledger.Adjust(ctx, tc.req)
		if err == nil {
			t.Errorf("%s reused an existing key and was accepted", tc.name)
		}
	}
}

// A DEBIT that would take a student below zero must FAIL. Not clamp, not warn —
// fail, because a clamped correction tells the operator the balance was corrected
// when it was not.
func TestADebitBeyondTheBalanceIsRefusedRatherThanClamped(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	const student = 4242
	grant, _ := env.grant(t, student, "overdraft-base", ReasonProfileComplete, RefStudyResource, 913)

	_, err := env.ledger.Adjust(ctx, AdjustRequest{
		UserID: student, Amount: -(grant.Amount + 1000), Reason: AdjustErroneousSpend,
		CreatedBy: "admin:7", IdempotencyKey: "overdraft-key",
	})
	if err == nil {
		t.Fatalf("a debit of %d against a balance of %d was accepted", grant.Amount+1000, grant.Amount)
	}
	// Nothing moved, and no journal was written — a refused write leaves no trace
	// beyond the error.
	if got := env.available(t, student); got != grant.Amount {
		t.Errorf("available = %d after a refused debit, want %d", got, grant.Amount)
	}
	var journals int64
	env.pool.Model(&CoinJournal{}).Where("idempotency_key = ?", "overdraft-key").Count(&journals)
	if journals != 0 {
		t.Errorf("a refused debit wrote %d journals, want 0", journals)
	}
}

// A debit that EXACTLY empties the balance is allowed. "Refuse to go negative" must
// not read as "refuse to reach zero", or an operator could never take back an award
// that was the whole balance.
func TestADebitThatExactlyEmptiesTheBalanceIsAllowed(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	const student = 4242
	grant, _ := env.grant(t, student, "exact-base", ReasonProfileComplete, RefStudyResource, 913)

	result, err := env.ledger.Adjust(ctx, AdjustRequest{
		UserID: student, Amount: -grant.Amount, Reason: AdjustDuplicateAward,
		CreatedBy: "admin:7", IdempotencyKey: "exact-key",
	})
	if err != nil {
		t.Fatalf("a debit of the exact balance was refused: %v", err)
	}
	if got := env.available(t, student); got != 0 {
		t.Errorf("available = %d, want 0", got)
	}
	_ = result
}

// The correction must survive reconciliation. An endpoint that produces a journal the
// reconciler then reports as drift is worse than no endpoint.
func TestAnAdjustmentLeavesTheLedgerReconcilable(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	const student = 4242
	env.grant(t, student, "recon-base", ReasonProfileComplete, RefStudyResource, 913)

	for i, amount := range []int64{30, -10, 5, 100} {
		if _, err := env.ledger.Adjust(ctx, AdjustRequest{
			UserID: student, Amount: amount, Reason: AdjustGoodwill,
			CreatedBy: "admin:7", IdempotencyKey: "recon-" + string(rune('a'+i)),
		}); err != nil {
			t.Fatalf("adjustment %d: %v", i, err)
		}
	}

	// Every invariant the ledger enforces about itself must still hold.
	report := RunReconcile(ctx, env.pool)
	if failure := report.Failure(); failure != nil {
		t.Errorf("the ledger stopped reconciling after adjustments: %+v", failure)
	}

	// And the balance must equal the sum of the postings, computed independently here
	// rather than through the projection.
	var fromPostings int64
	env.pool.Raw(`
		SELECT COALESCE(SUM(p.amount), 0) FROM coin_posting p
		  JOIN coin_account a ON a.id = p.account_id
		 WHERE a.owner_user_id = ?`, student).Scan(&fromPostings)
	if fromPostings != env.available(t, student) {
		t.Errorf("postings sum to %d but the projection says %d", fromPostings, env.available(t, student))
	}
}

// The reason must be in the closed set ENFORCED BY THE DATABASE, not only validated
// in Go. A future writer that bypasses Adjust must not be able to invent a reason.
//
// This asserts the SERVICE refuses it. The constraint itself is added by the
// migration and asserted there — two layers, tested in two places, because a
// service-only check is bypassable by any future code path and a constraint-only
// check produces an error message nobody can act on.
func TestAnInventedAdjustmentReasonIsRefused(t *testing.T) {
	env := newSupportEnv(t)
	env.grant(t, 4242, "reason-base", ReasonProfileComplete, RefStudyResource, 913)
	_, err := env.ledger.Adjust(context.Background(), AdjustRequest{
		UserID: 4242, Amount: 5, Reason: "OPERATOR_WAS_HURRY",
		CreatedBy: "admin:7", IdempotencyKey: "invented-key",
	})
	if err == nil {
		t.Error("an invented reason code was accepted")
	}
}
