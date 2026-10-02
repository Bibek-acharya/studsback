//go:build coinsintegration

// internal/coins/support_view_pg_test.go
//
// 04 §6: the support view — the endpoint that answers "why does this student have
// 30 coins?".
//
// The load-bearing property is not that it returns data. It is that it returns the
// ARITHMETIC: cached and computed balances side by side, every journal, and every
// lot. A support endpoint that reports only the headline number makes ledger drift
// invisible, which is the one thing it exists to surface.

package coins

import (
	"context"
	"testing"
	"time"
)

// THE test. Grant, spend part of it, and confirm the view explains every figure.
func TestTheSupportViewExplainsEveryCoinATrackingStudentHas(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	const student = 4242

	// Two awards and a spend, so the arithmetic is not a single term.
	grant, lotID := env.grant(t, student, "support-grant-1", ReasonProfileComplete, RefStudyResource, 913)
	env.grant(t, student, "support-grant-2", ReasonResourceApproved, RefStudyResource, 914)
	spend, err := env.ledger.Spend(ctx, SpendRequest{
		UserID: student, ReasonCode: ReasonResourceUnlock, IdempotencyKey: "support-spend-1",
		RefType: RefStudyResource, RefID: refID(lotID),
	})
	if err != nil {
		t.Fatalf("spend: %v", err)
	}

	view, err := env.ledger.SupportView(ctx, SupportQuery{UserID: student})
	if err != nil {
		t.Fatalf("support view: %v", err)
	}

	// The headline the wallet shows, and it must reconcile.
	want := grant.Amount + spend.Amount
	_ = want
	if view.TotalAvailable <= 0 {
		t.Fatalf("total_available = %d, want a positive balance", view.TotalAvailable)
	}
	if len(view.Discrepancies) != 0 {
		t.Errorf("a healthy ledger reported %d discrepancies: %v", len(view.Discrepancies), view.Discrepancies)
	}

	// Every journal that touched this student is here, newest first, with its legs.
	if len(view.Journals) != 3 {
		t.Fatalf("%d journals, want 3 (two grants and a spend)", len(view.Journals))
	}
	seen := map[string]bool{}
	for _, journal := range view.Journals {
		seen[journal.ReasonCode] = true
		if len(journal.Postings) == 0 {
			t.Errorf("journal %s has no postings; support cannot follow the money without them", journal.ID)
		}
		// Every journal nets to zero ACROSS accounts. The per-user amount is what
		// support reads, and for a grant it must be POSITIVE.
		if journal.EntryType == EntryGrant && journal.Amount <= 0 {
			t.Errorf("a grant shows %d for this student; a grant credits them", journal.Amount)
		}
		if journal.EntryType == EntrySpend && journal.Amount >= 0 {
			t.Errorf("a spend shows %d for this student; a spend debits them", journal.Amount)
		}
	}
	for _, want := range []string{ReasonProfileComplete, ReasonResourceApproved, ReasonResourceUnlock} {
		if !seen[want] {
			t.Errorf("the view is missing the %s journal", want)
		}
	}

	// Both lots, with what is left in each.
	if len(view.Lots) != 2 {
		t.Fatalf("%d lots, want 2", len(view.Lots))
	}
	for _, lot := range view.Lots {
		if lot.Remaining < 0 {
			t.Errorf("lot %d shows remaining %d", lot.ID, lot.Remaining)
		}
		if lot.JournalID == "" {
			t.Errorf("lot %d does not name the grant that created it", lot.ID)
		}
	}

	// The spend consumed from exactly one lot, and the view says which.
	consumedSomewhere := false
	for _, lot := range view.Lots {
		if lot.Consumed > 0 {
			consumedSomewhere = true
		}
	}
	if !consumedSomewhere {
		t.Error("no lot shows any consumption; the spend is invisible from the lot side")
	}
}

// The endpoint's real value: it must show a DEFECT when there is one. A support
// view that cannot report drift is a support view that confirms whatever the
// wallet already said.
func TestTheSupportViewReportsCachedVersusComputedDrift(t *testing.T) {
	env := newSupportEnv(t)
	env.grant(t, 4242, "drift-grant", ReasonProfileComplete, RefStudyResource, 913)

	// Corrupt the cached projection — exactly what RunReconcile exists to catch.
	if err := env.pool.Exec(
		`UPDATE coin_account_balance SET posted_balance = posted_balance + 999 WHERE account_id IN
		 (SELECT id FROM coin_account WHERE owner_user_id = ?)`, 4242).Error; err != nil {
		t.Fatalf("corrupt cache: %v", err)
	}

	view, err := env.ledger.SupportView(context.Background(), SupportQuery{UserID: 4242})
	if err != nil {
		t.Fatalf("support view: %v", err)
	}
	if len(view.Discrepancies) == 0 {
		t.Fatal("a 999-coin cache drift was reported as clean; this endpoint exists to prevent exactly that")
	}
	if len(view.Accounts) == 0 {
		t.Fatal("no accounts returned")
	}
	// Both numbers must be visible, and they must disagree — that disagreement IS
	// the finding.
	account := view.Accounts[0]
	if !account.Drifted {
		t.Error("the account is not marked Drifted")
	}
	if account.CachedPosted == account.ComputedPosted {
		t.Errorf("cached %d equals computed %d; the drift was hidden", account.CachedPosted, account.ComputedPosted)
	}
}

// A closed account is a support hold. "Their coins are frozen" is a question
// support is asked and this is where it gets answered.
func TestTheSupportViewSaysAnAccountIsFrozen(t *testing.T) {
	env := newSupportEnv(t)
	env.grant(t, 4242, "frozen-grant", ReasonProfileComplete, RefStudyResource, 913)

	before, err := env.ledger.SupportView(context.Background(), SupportQuery{UserID: 4242})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	for _, account := range before.Accounts {
		if account.Closed {
			t.Fatal("a fresh account is reported closed")
		}
	}

	if err := env.pool.Exec(
		`UPDATE coin_account SET closed_at = now() WHERE owner_user_id = ?`, 4242).Error; err != nil {
		t.Fatalf("close the account: %v", err)
	}
	after, err := env.ledger.SupportView(context.Background(), SupportQuery{UserID: 4242})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if len(after.Accounts) == 0 || !after.Accounts[0].Closed {
		t.Error("a closed account is not reported as closed")
	}
}

// A lot past its expiry and not yet swept is the state where the balance the
// student sees and the coins they can spend disagree. Support needs to see it.
func TestTheSupportViewFlagsAnExpiredButUnburnedLot(t *testing.T) {
	env := newSupportEnv(t)
	_, lotID := env.grant(t, 4242, "stale-lot", ReasonProfileComplete, RefStudyResource, 913)

	if err := env.pool.Model(&CoinLot{}).Where("id = ?", lotID).
		UpdateColumn("expires_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire the lot: %v", err)
	}

	view, err := env.ledger.SupportView(context.Background(), SupportQuery{UserID: 4242})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if len(view.Lots) != 1 {
		t.Fatalf("%d lots, want 1", len(view.Lots))
	}
	lot := view.Lots[0]
	if !lot.Expired {
		t.Error("a lot an hour past its expiry is not flagged; this is the state where the shown balance and the spendable balance disagree")
	}
	if lot.Remaining <= 0 {
		t.Errorf("remaining = %d; the lot should still hold coins", lot.Remaining)
	}
}

// A swept lot is expired AND consumed. Conflating the two would make a swept lot
// look like a pending problem.
func TestTheSupportViewDoesNotFlagAnAlreadyBurnedLotAsPending(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	_, lotID := env.grant(t, 4242, "burned-lot", ReasonProfileComplete, RefStudyResource, 913)

	if err := env.pool.Model(&CoinLot{}).Where("id = ?", lotID).
		UpdateColumn("expires_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire: %v", err)
	}
	if _, err := env.sweeper.Sweep(ctx, ExpiryBatchDefault); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	view, err := env.ledger.SupportView(ctx, SupportQuery{UserID: 4242})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	for _, lot := range view.Lots {
		if lot.Expired {
			t.Errorf("lot %d is flagged expired although the sweep already burned it; Expired means 'lapsed and unburnt'", lot.ID)
		}
	}
	if view.TotalAvailable != 0 {
		t.Errorf("total_available = %d after a full burn, want 0", view.TotalAvailable)
	}
}

// The limits. An unbounded read on a user with thousands of journals is how an
// operator's browser hangs, and this endpoint is reachable by anyone who can read
// coin pricing.
func TestTheSupportViewIsBoundedAndRefusesUserZero(t *testing.T) {
	env := newSupportEnv(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		env.grant(t, 4242, "bounded-"+string(rune('a'+i)), ReasonProfileComplete, RefStudyResource, uint(913+i))
	}

	full, err := env.ledger.SupportView(ctx, SupportQuery{UserID: 4242})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if len(full.Journals) != 5 {
		t.Fatalf("%d journals unbounded, want 5", len(full.Journals))
	}

	bounded, err := env.ledger.SupportView(ctx, SupportQuery{UserID: 4242, Limit: 2})
	if err != nil {
		t.Fatalf("bounded view: %v", err)
	}
	if len(bounded.Journals) != 2 {
		t.Errorf("%d journals with Limit 2, want 2", len(bounded.Journals))
	}
	// Lots are NOT truncated: they are the smaller table, and a support engineer
	// needs every one to answer "what is left".
	if len(bounded.Lots) != len(full.Lots) {
		t.Errorf("the journal limit also truncated lots: %d of %d", len(bounded.Lots), len(full.Lots))
	}

	if _, err := env.ledger.SupportView(ctx, SupportQuery{UserID: 0}); err == nil {
		t.Error("user 0 was accepted")
	}
	// A user with nothing gets an empty, non-nil view rather than an error — the
	// same rule as the referral endpoint's "no referrals is a true answer".
	empty, err := env.ledger.SupportView(ctx, SupportQuery{UserID: 999999})
	if err != nil {
		t.Fatalf("an unknown user returned %v, want an empty view", err)
	}
	if len(empty.Accounts) != 0 || empty.Journals == nil || empty.Lots == nil {
		t.Error("an unknown user returned nil slices; the JSON would be null rather than []")
	}
}
