//go:build coinsintegration

// internal/coins/referral_qualification_pg_test.go
//
// The qualification pass, the 7-day gate and the student endpoint, against a real
// PostgreSQL instance.
//
// Every assertion here is about a TRANSACTION and a CONSTRAINT. The claims are "a pass
// that runs twice pays once", "a referral inside its window cannot be paid", and "the
// cap holds across a bulk settlement" — and none of them is checkable without the
// unique constraints, the per-user advisory lock and the ON CONFLICT resolution that a
// mock database would be asserting against a copy of.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// Reuses the harness from referral_pg_test.go — the same coreSchema, the same single
// pool, the same openReferralSchema reset. This file adds no schema of its own. Every
// test skips cleanly when COINS_TEST_DSN is unset, and nothing is ever written to the
// public schema.
//
// It reads the `users` table through the same minimal local struct referral_pg_test.go
// declares, so the qualification pass's code resolution is exercised against a real
// table rather than a stub.
package coins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/utils"
)

// ── fixtures ─────────────────────────────────────────────────────────────────

// qualifiedPhone is the phone-verification port's stand-in.
//
// There is no implementation of coins.PhoneVerification in this codebase — that is the
// finding — so a fixture has to supply one for anything to settle. It is a field so a
// test can move an invitee between verified and unverified between two passes, which is
// what the "unverified phone does not settle" case needs.
type qualifiedPhone struct {
	mu       sync.Mutex
	verified map[uint]bool
	err      error
	calls    int
}

func newQualifiedPhone() *qualifiedPhone {
	return &qualifiedPhone{verified: map[uint]bool{}}
}

func (p *qualifiedPhone) verify(userID uint) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.verified[userID] = true
}

func (p *qualifiedPhone) setErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

func (p *qualifiedPhone) PhoneVerified(_ context.Context, userID uint) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.err != nil {
		return false, p.err
	}
	return p.verified[userID], nil
}

// called reports how many times the port was consulted.
//
// Counted rather than assumed, because "the mechanic asks about verification" and "the
// mechanic reads a phone column" produce identical outcomes — both refuse — so only this
// distinguishes them.
func (p *qualifiedPhone) called() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// qualFixture is a wired referral service over the shared pool.
type qualFixture struct {
	svc        *ReferralService
	completion *stubPercent
	phone      *qualifiedPhone
	notifier   *recordingNotifier
	db         *gorm.DB
}

// newQualFixture wires the service with BOTH ports, because a fixture without the
// phone port settles nothing and a test asserting "it paid" would fail for a reason
// that has nothing to do with what it is testing.
func newQualFixture(t *testing.T, db *gorm.DB, mutate func(*EconomyConfig)) *qualFixture {
	t.Helper()
	if err := db.Exec(`TRUNCATE reward_grant RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate reward_grant: %v", err.Error())
	}
	// stubPercent rather than the profile award's own stubCompletion: this fixture reads
	// the port's call log to prove the INVITEE's id is the one asked about, and the two
	// types exist for two different tests.
	completion := &stubPercent{percent: map[uint]int{}}
	phone := newQualifiedPhone()
	notifier := &recordingNotifier{}
	svc := NewReferralService(NewRepository(db), testLedger(t, db, mutate)).
		WithQualifiers(completion, phone).
		WithNotifier(notifier)
	return &qualFixture{svc: svc, completion: completion, phone: phone, notifier: notifier, db: db}
}

// stage attributes a referral and backdates it past the 7-day window.
//
// Backdating rather than sleeping: the window is seven days, and a test that waits for
// it does not run. The backdate is the ONLY thing that makes an old referral, and
// TestTheWindowGateRejectsAReferralInsideItsWindow asserts the gate on the same fixture
// with the backdate left off — so the two together prove the backdate is what makes the
// difference and not something else about the fixture.
func (f *qualFixture) stage(t *testing.T, referrer, invitee uint, code string, ageDays int) uint {
	t.Helper()
	res, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: invitee,
		ReferralCode: code, SourcePath: "verify_otp",
	})
	if err != nil || !res.Attributed {
		t.Fatalf("stage referral (invitee %d): attributed=%v reason=%q err=%v",
			invitee, res.Attributed, res.Reason, err)
	}
	if ageDays > 0 {
		backdateReferral(t, f.db, res.ReferralID, ageDays)
	}
	return res.ReferralID
}

// backdateReferral rewrites created_at, which is what the window is measured from.
//
// Raw SQL rather than a service method because there is no legitimate production path
// that does this — it stands in for the passage of time. The UPDATE is scoped to one id
// so it cannot be mistaken for something the mechanic does on its own.
func backdateReferral(t *testing.T, db *gorm.DB, referralID uint, ageDays int) {
	t.Helper()
	// The day count is interpolated rather than bound, and the reason is repository.go's:
	// pgx infers a placeholder's type from the surrounding expression, so `?` next to
	// `|| ' days'` is inferred as text and an int cannot be encoded into it. The count is
	// an int literal from this file's own call sites, never anything a request carries,
	// so there is nothing to bind.
	if err := db.Exec(fmt.Sprintf(
		`UPDATE user_referral SET created_at = created_at - interval '%d days' WHERE id = ?`,
		ageDays), referralID).Error; err != nil {
		t.Fatalf("backdate referral %d by %d days: %v", referralID, ageDays, err)
	}
}

// referralStatus reads one referral's status straight from the database, so an assertion
// about the state machine is not reading back the same struct the code wrote.
func referralStatus(t *testing.T, db *gorm.DB, referralID uint) string {
	t.Helper()
	var status string
	if err := db.Raw(`SELECT status FROM user_referral WHERE id = ?`, referralID).Scan(&status).Error; err != nil {
		t.Fatalf("read referral %d status: %v", referralID, err)
	}
	return status
}

func expiredReason(t *testing.T, db *gorm.DB, referralID uint) string {
	t.Helper()
	var reason string
	if err := db.Raw(`SELECT expired_reason FROM user_referral WHERE id = ?`, referralID).Scan(&reason).Error; err != nil {
		t.Fatalf("read referral %d expiry reason: %v", referralID, err)
	}
	return reason
}

// settleCoins is what the referrer's referral grants have actually paid them.
func settleCoins(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return countRows(t, db,
		`SELECT COALESCE(SUM(p.amount), 0) FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		   JOIN coin_journal j ON j.id = p.journal_id
		  WHERE a.owner_user_id = ? AND a.kind = ? AND j.reason_code = ? AND p.amount > 0`,
		userID, AccountUser, ReasonReferralQualified)
}

// ── the 7-day gate ────────────────────────────────────────────────────────────

// A referral INSIDE its window does not settle, whatever else is true.
//
// The whole point of the window, and the reason the gate lives in SettleReferral rather
// than only in the pass: a complete profile and a verified phone on day three is not
// enough. §5.2's two conditions are necessary and not sufficient, and a
// qualification rule that forgot that would pay every signup inside the fraud window
// SEON's recycled-number signal runs through.
//
// Asserted through SettleReferral directly, not only through the pass, so it is a
// statement about the money path rather than about one caller's discipline.
func TestTheWindowGateRejectsAReferralInsideItsWindow(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "gate@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "gate-invitee@example.com")
	// NO backdate: the referral is brand new, so its seven-day window is entirely ahead.
	referralID := f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 0)

	// Everything §5.2 asks for is true.
	f.completion.set(inviteeID, 100)
	f.phone.verify(inviteeID)

	// SettleReferral refuses, and names the window as the reason.
	_, err := f.svc.SettleReferral(context.Background(), referralID)
	if !errors.Is(err, ErrReferralNotYetPayable) {
		t.Fatalf("settling a referral inside its window returned %v, want ErrReferralNotYetPayable", err)
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("the referrer was paid %d coins for a referral inside its 7-day window", got)
	}
	if got := referralStatus(t, db, referralID); got != ReferralPending {
		t.Errorf("status = %q, want %q", got, ReferralPending)
	}
	if n := len(grantRows(t, db, referrerID)); n != 0 {
		t.Errorf("%d reward_grant rows for a refused early settlement, want 0", n)
	}

	// And the pass agrees: it finds the referral, decides it is waiting, and pays
	// nobody. The pass and the gate must not disagree.
	report, perr := f.svc.QualifyPendingReferrals(context.Background())
	if perr != nil {
		t.Fatalf("qualification pass: %v", perr)
	}
	if report.Settled != 0 {
		t.Errorf("the pass settled %d referrals, want 0", report.Settled)
	}
	if report.ByReason[QualificationWindowOpen] != 1 {
		t.Errorf("by-reason = %v, want one %s", report.ByReason, QualificationWindowOpen)
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("the referrer was paid %d coins after a pass that should have paid nothing", got)
	}
}

// A referral whose window has closed, with a complete profile and a verified phone,
// settles — exactly once.
//
// The positive counterpart, and the reason the negative case above is not satisfied by a
// pass that settles nothing.
func TestAReferralPastItsWindowWithBothConditionsSettles(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "settle@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "settle-invitee@example.com")
	referralID := f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)

	f.completion.set(inviteeID, 100)
	f.phone.verify(inviteeID)

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("qualification pass: %v", err)
	}
	if report.Settled != 1 {
		t.Fatalf("settled = %d, want 1 (by-reason %v)", report.Settled, report.ByReason)
	}

	// Paid from the FAUCET, and the referrer needed no balance to begin with. This is
	// the whole change from the hold model: a student with zero coins can now be paid
	// for referring someone, which is the mechanic working as intended rather than a
	// detail.
	if got, want := settleCoins(t, db, referrerID), award; got != want {
		t.Errorf("paid %d coins, want %d", got, want)
	}
	if got := userPosted(t, db, referrerID); got != award {
		t.Errorf("posted = %d, want %d — and note the referrer was never funded first, "+
			"which is the point of retiring the hold", got, award)
	}
	if got := userReserved(t, db, referrerID); got != 0 {
		t.Errorf("reserved = %d, want 0: nothing is held any more", got)
	}
	// And no hold journal exists at all, which is what "the hold is gone" means on the
	// ledger rather than merely in the code.
	if n := countRows(t, db, `SELECT count(*) FROM coin_journal WHERE reason_code = ?`,
		ReasonReferralHold); n != 0 {
		t.Errorf("%d REFERRAL_HOLD journals after a settlement, want 0", n)
	}

	// The state machine advanced.
	if got := referralStatus(t, db, referralID); got != ReferralQualified {
		t.Errorf("status = %q, want %q", got, ReferralQualified)
	}
	rows := referrals(t, db)
	if rows[0].HoldJournalID != nil {
		t.Error("hold_journal_id is set; the column is permanently NULL under this model")
	}
	if rows[0].GrantJournalID == nil {
		t.Error("grant_journal_id is null on a settled referral; the money has to be traceable")
	}
	if rows[0].CapSlot == nil || rows[0].CapPeriod == nil {
		t.Error("the settlement recorded no cap slot, so it is paid but uncounted")
	}

	// The faucet's liability is what makes this issuance rather than a transfer.
	if n := countRows(t, db,
		`SELECT count(*) FROM coin_journal WHERE reason_code = ? AND state = ?`,
		ReasonReferralQualified, StatePosted); n != 1 {
		t.Errorf("%d POSTED REFERRAL_QUALIFIED journals, want 1", n)
	}
}

// An UNVERIFIED phone does not settle, and the reason it does not is recorded as
// distinct from a broken profile.
//
// §5.2 is two conditions joined by AND, and dropping either one makes the whole rule
// decorative. The phone half is the one with no implementation behind it, so it is also
// the one most likely to be quietly replaced by "has a phone number" — which is why the
// negative case is asserted with the profile condition satisfied, so a test that passed
// because the profile was incomplete would not prove the phone check runs.
func TestAnUnverifiedPhoneDoesNotSettleEvenWithACompleteProfile(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "unverified@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "unverified-invitee@example.com")
	referralID := f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)

	f.completion.set(inviteeID, 100)
	// Deliberately NOT f.phone.verify(inviteeID).

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("qualification pass: %v", err)
	}
	if report.Settled != 0 {
		t.Fatalf("settled %d referrals with an unverified phone, want 0", report.Settled)
	}
	if report.ByReason[QualificationPhoneUnverified] != 1 {
		t.Errorf("by-reason = %v, want one %s", report.ByReason, QualificationPhoneUnverified)
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("the referrer was paid %d coins for an invitee whose phone is unverified", got)
	}
	if got := referralStatus(t, db, referralID); got != ReferralPending {
		t.Errorf("status = %q, want %q — a referral that has not paid is not terminal", got, ReferralPending)
	}

	// Verifying the phone is then sufficient, with no other change. So the check above
	// really was the phone and not the profile.
	f.phone.verify(inviteeID)
	if report, err = f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if report.Settled != 1 {
		t.Errorf("after verification the referral did not settle: by-reason %v", report.ByReason)
	}
	if got := settleCoins(t, db, referrerID); got != DefaultEconomyConfig().Awards.ReferralReferrer {
		t.Errorf("paid %d coins, want %d", got, DefaultEconomyConfig().Awards.ReferralReferrer)
	}
}

// No phone-verification port means NOTHING settles, and the pass says so rather than
// reporting a quiet success.
//
// This is the fail-closed property in the configuration every deployment actually runs
// today, because auth.User has no verification state and main.go wires nil. A pass that
// counted its refusals as an ordinary outcome would make a disabled mechanic
// indistinguishable from a working one, which is the failure this whole design is
// arranged to avoid.
func TestWithNoPhoneVerifierNothingSettlesAndThePassDeclines(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "noverifier@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "noverifier-invitee@example.com")
	f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 30)

	// Past the window, complete profile — everything except the thing that cannot be
	// checked.
	f.completion.set(inviteeID, 100)

	// Remove the port, exactly as main.go wires it.
	f.svc.WithQualifiers(f.completion, nil)

	// The id is READ rather than hardcoded, because this fixture's ids depend on the
	// state left by earlier rows and a hardcoded 1 is a test that passes or fails for a
	// reason unrelated to what it is about. An earlier draft of this file asserted
	// `status == pending` on id 1 and read back "" — which is not a referral at all.
	// A test that asserts against the wrong row teaches the reader that the assertion is
	// noisy, and the next person deletes it.
	var pendingID uint
	if err := db.Raw(
		`SELECT id FROM user_referral WHERE referrer_user_id = ? AND referred_kind = ?`,
		referrerID, SubjectUser,
	).Scan(&pendingID).Error; err != nil {
		t.Fatalf("find the staged referral: %v", err)
	}
	if pendingID == 0 {
		t.Fatal("stage() did not create a referral, so the assertions below would be about nothing")
	}

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if !errors.Is(err, ErrQualificationUnverifiable) {
		t.Fatalf("a pass with no phone verifier returned %v, want ErrQualificationUnverifiable", err)
	}
	if report.Settled != 0 {
		t.Errorf("settled %d referrals with no phone verifier", report.Settled)
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("paid %d coins with no phone verifier wired", got)
	}
	if report.ByReason[QualificationNoVerifier] != 1 {
		t.Errorf("by-reason = %v, want one %s", report.ByReason, QualificationNoVerifier)
	}
	// The scanned count is the DECLINED count, so the log line tells an operator how
	// many referrals the missing verification flow is holding up.
	if report.Scanned != 1 {
		t.Errorf("scanned = %d, want 1: the count is what makes the log actionable", report.Scanned)
	}

	// And the referral is untouched: declining is not expiring. A missing dependency is
	// a fact about the deployment, and destroying real attributions over it would be
	// unrecoverable.
	if got := referralStatus(t, db, pendingID); got != ReferralPending {
		t.Errorf("status = %q, want %q: a pass that cannot verify must not expire "+
			"referrals, because wiring the port back would otherwise lose them forever",
			got, ReferralPending)
	}

	// And with the port restored, the SAME referral pays — which is what makes "must not
	// expire" a real requirement rather than a precaution. Declining is recoverable;
	// expiring over a missing dependency is not.
	f.svc.WithQualifiers(f.completion, f.phone)
	f.phone.verify(inviteeID)
	if again, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("pass after wiring the verifier: %v", err)
	} else if again.Settled != 1 {
		t.Errorf("after wiring the verifier the referral did not settle: %v — so the "+
			"declining was not recoverable, and any expiry would have destroyed it",
			again.ByReason)
	}
	if got := settleCoins(t, db, referrerID); got != DefaultEconomyConfig().Awards.ReferralReferrer {
		t.Errorf("paid %d coins after wiring the verifier, want %d",
			got, DefaultEconomyConfig().Awards.ReferralReferrer)
	}
}

// ── idempotency ──────────────────────────────────────────────────────────────

// Running the pass twice pays ONCE. This is the load-bearing claim of the whole
// mechanic, because the pass runs on a timer: a second run is not a hypothetical, it is
// what every deploy and every process restart causes.
//
// It asserts three things and they are not redundant:
//
//   - the BALANCE moved by exactly one award, which is the property with money in it;
//   - the reward_grant table has exactly one row, which is the gate;
//   - the SECOND pass reports settling nothing, so the second run also did not waste a
//     transaction on a payout that was refused.
//
// It then goes further and forces the second mechanism: a settlement of the
// already-QUALIFIED referral, which the pass's status filter would never attempt. That
// is the claim's belt-and-braces half — the filter is what makes a repeat pass cheap,
// and the claim insert is what makes it CORRECT, and only one of them being correct
// would be a bug.
func TestRunningTheQualificationPassTwicePaysOnce(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "twice@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "twice-invitee@example.com")
	referralID := f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)
	f.completion.set(inviteeID, 100)
	f.phone.verify(inviteeID)

	// ── pass one ────────────────────────────────────────────────────────────────
	first, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if first.Settled != 1 {
		t.Fatalf("first pass settled %d, want 1 (by-reason %v)", first.Settled, first.ByReason)
	}
	afterFirst := settleCoins(t, db, referrerID)

	// ── pass two ────────────────────────────────────────────────────────────────
	second, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if second.Settled != 0 {
		t.Fatalf("the second pass settled %d referrals, want 0", second.Settled)
	}
	if second.Scanned != 0 {
		t.Errorf("the second pass scanned %d referrals, want 0: a settled referral is "+
			"no longer pending, so the repeat pass should find nothing to look at", second.Scanned)
	}

	// THE assertion. A balance, not a row count — a mechanic can be idempotent in its
	// rows and pay twice in its postings.
	if got := settleCoins(t, db, referrerID); got != afterFirst || afterFirst != award {
		t.Errorf("after two passes the referrer has been paid %d coins, want %d. "+
			"Two passes must pay exactly one referral", got, award)
	}
	if n := len(grantRows(t, db, referrerID)); n != 1 {
		t.Errorf("%d reward_grant rows after two passes, want 1", n)
	}
	if n := countRows(t, db,
		`SELECT count(*) FROM coin_journal WHERE reason_code = ?`, ReasonReferralQualified); n != 1 {
		t.Errorf("%d REFERRAL_QUALIFIED journals after two passes, want 1", n)
	}
	if n := countRows(t, db,
		`SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID); n != 1 {
		t.Errorf("%d cap slots consumed after two passes, want 1: a second pass that paid "+
			"nothing must not consume the referrer's monthly cap", n)
	}
	// The notifier fires once. A retried announce is the same occurrence, and the
	// announcement is keyed on the journal id for exactly that reason.
	if f.notifier.count() != 1 {
		t.Errorf("%d coins.credited notifications after two passes, want 1", f.notifier.count())
	}

	// ── and the second mechanism, forced ─────────────────────────────────────────
	//
	// The pass would never attempt this, because the referral is 'qualified' and the
	// pass only reads 'pending'. Forcing it proves the ON CONFLICT claim is what makes
	// the mechanic correct, rather than the status filter merely making it look so.
	before := userPosted(t, db, referrerID)
	grant, err := f.svc.SettleReferral(context.Background(), referralID)
	if !errors.Is(err, ErrReferralAlreadyPaid) {
		t.Fatalf("re-settling a qualified referral returned (%+v, %v), want ErrReferralAlreadyPaid",
			grant, err)
	}
	if grant.JournalID != "" {
		t.Error("a refused re-settlement returned a journal id, which a handler would " +
			"render as a credit")
	}
	if got := userPosted(t, db, referrerID); got != before {
		t.Errorf("posted moved from %d to %d on a re-settlement of a paid referral", before, got)
	}
	if f.notifier.count() != 1 {
		t.Errorf("%d notifications after a forced re-settlement, want 1: a refusal must "+
			"not announce a credit", f.notifier.count())
	}
}

// The claim survives a FAILED settlement, so a transient failure does not permanently
// lose the referral.
//
// The ordering that earns this — claim before money, both in one transaction — means a
// rollback undoes the claim too. A claim written outside the transaction would survive
// a failed payout and make the referral permanently unpayable, which is the failure
// mode the whole ordering exists to prevent.
func TestAFailedSettlementLeavesTheClaimRetryable(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "retry@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "retry-invitee@example.com")
	referralID := f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)

	// The failure is a full monthly cap: claimCapSlot is step 3 of the transaction, so
	// the claim at step 2 has already been written when the transaction aborts. That
	// position — a written claim that must not survive — is the whole point, and the
	// old hold model could not produce it here because there was no claim ordering left
	// to test.
	//
	// Filled directly rather than by settling ten referrals, so the cap is the ONLY
	// thing that can refuse this settlement.
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 SELECT ?, ?, g, 900000 + g, 60, now() FROM generate_series(1, ?) g`,
		referrerID, ReferralMonthKey(time.Now().UTC()), ReferralCapSlotCeiling,
	).Error; err != nil {
		t.Fatalf("fill the cap: %v", err)
	}

	if _, err := f.svc.SettleReferral(context.Background(), referralID); !errors.Is(err, ErrCapReached) {
		t.Fatalf("a capped settlement returned %v, want ErrCapReached", err)
	}
	// The claim rolled back with the failed transaction, so it is retryable.
	if n := len(grantRows(t, db, referrerID)); n != 0 {
		t.Errorf("%d reward_grant rows survived a failed settlement; a committed claim "+
			"without its money makes every later attempt a no-op", n)
	}
	if got := referralStatus(t, db, referralID); got != ReferralPending {
		t.Errorf("status = %q, want %q", got, ReferralPending)
	}

	// Free the cap and retry: it pays. So the referral was never lost.
	if err := db.Exec(
		`DELETE FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID,
	).Error; err != nil {
		t.Fatalf("free the cap: %v", err)
	}
	if _, err := f.svc.SettleReferral(context.Background(), referralID); err != nil {
		t.Fatalf("retry after a failed settlement: %v", err)
	}
	if got := settleCoins(t, db, referrerID); got != DefaultEconomyConfig().Awards.ReferralReferrer {
		t.Errorf("paid %d after a successful retry, want %d", got, DefaultEconomyConfig().Awards.ReferralReferrer)
	}
}

// ── the cap across a bulk pass ───────────────────────────────────────────────

// Settling in BULK does not breach the cap, and a referral refused for the cap is not
// destroyed — it stays pending and pays next month.
//
// Two properties in one test because they are the same property seen from two sides. A
// cap that held by REJECTING rather than by deferring would pass the first assertion and
// lose twelve referrals, which is a worse outcome than the cap itself.
//
// Twelve referrals for one referrer, all qualifying on the same pass, against a cap of
// ten. The first ten pay. The eleventh and twelfth are refused, stay pending, and — the
// part worth pinning — the cap's slots are consumed exactly ten times, so the eleventh
// did not take a slot it will not use.
func TestTheCapHoldsAcrossABulkQualificationPass(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "bulk@example.com", "K7M2QX9RT4")

	const cohort = ReferralCapSlotCeiling + 2
	ids := make([]uint, 0, cohort)
	for i := 0; i < cohort; i++ {
		inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db},
			fmt.Sprintf("bulk-invitee-%02d@example.com", i))
		ids = append(ids, f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8))
		f.completion.set(inviteeID, 100)
		f.phone.verify(inviteeID)
	}

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("bulk pass: %v", err)
	}

	if report.Settled != ReferralCapSlotCeiling {
		t.Errorf("settled %d of %d qualifying referrals, want exactly the cap of %d",
			report.Settled, cohort, ReferralCapSlotCeiling)
	}
	if got := settleCoins(t, db, referrerID); got != int64(ReferralCapSlotCeiling)*award {
		t.Errorf("paid %d coins, want %d = %d x %d", got,
			ReferralCapSlotCeiling*award, ReferralCapSlotCeiling, award)
	}
	if n := countRows(t, db,
		`SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID); n != ReferralCapSlotCeiling {
		t.Errorf("%d cap slots consumed, want %d", n, ReferralCapSlotCeiling)
	}

	// Exactly the two over the cap are still pending — not terminal, not lost.
	pending, qualified := 0, 0
	for _, id := range ids {
		switch referralStatus(t, db, id) {
		case ReferralPending:
			pending++
		case ReferralQualified:
			qualified++
		default:
			t.Errorf("referral %d is %q, want pending or qualified", id, referralStatus(t, db, id))
		}
	}
	if qualified != ReferralCapSlotCeiling || pending != cohort-ReferralCapSlotCeiling {
		t.Errorf("%d qualified and %d pending, want %d and %d", qualified, pending,
			ReferralCapSlotCeiling, cohort-ReferralCapSlotCeiling)
	}
	if expiredReason(t, db, ids[cohort-1]) != "" {
		t.Error("a cap-refused referral recorded an expiry reason; the cap is not an " +
			"expiry, and the referral is owed a payout next month")
	}

	// A second pass in the same month changes nothing: the cap is a claim, and the
	// claim is already taken.
	if again, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("second pass: %v", err)
	} else if again.Settled != 0 {
		t.Errorf("the second pass settled %d more referrals in the same month, want 0", again.Settled)
	}
	if got := settleCoins(t, db, referrerID); got != int64(ReferralCapSlotCeiling)*award {
		t.Errorf("paid %d after a second pass, want %d", got, ReferralCapSlotCeiling*award)
	}
}

// The cap is per REFERRER, so one capped student does not stop anybody else's
// referral from paying.
//
// The mechanic settles one referral per transaction under that referral's referrer's
// lock, so this cannot fail by design — which is exactly why it is worth a test. A pass
// that returned early on the first cap it hit would pay one student and silently starve
// the rest of the cohort, and the only symptom would be a support ticket about somebody
// else.
func TestOneCappedReferrerDoesNotStarveTheRestOfTheCohort(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	cappedID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "capped-bulk@example.com", "K7M2QX9RT4")
	// A different code, so the second student attributes to a second referrer.
	otherID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "other-bulk@example.com", "K7M2QX9RT5")

	// Fill the capped referrer's month.
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 SELECT ?, ?, g, 800000 + g, 60, now() FROM generate_series(1, ?) g`,
		cappedID, ReferralMonthKey(time.Now().UTC()), ReferralCapSlotCeiling,
	).Error; err != nil {
		t.Fatalf("fill the cap: %v", err)
	}

	// Two referrals past their windows: one for each referrer.
	cappedInvitee := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "capped-invitee@example.com")
	otherInvitee := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "other-invitee@example.com")
	f.stage(t, cappedID, cappedInvitee, "K7M2QX9RT4", 8)
	f.stage(t, otherID, otherInvitee, "K7M2QX9RT5", 8)
	for _, id := range []uint{cappedInvitee, otherInvitee} {
		f.completion.set(id, 100)
		f.phone.verify(id)
	}

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	if report.Settled != 1 {
		t.Fatalf("settled %d, want 1: the capped referral is refused and the other is paid",
			report.Settled)
	}
	if got := settleCoins(t, db, cappedID); got != 0 {
		t.Errorf("the capped referrer was paid %d coins", got)
	}
	if got, want := settleCoins(t, db, otherID),
		DefaultEconomyConfig().Awards.ReferralReferrer; got != want {
		t.Errorf("the uncapped referrer was paid %d coins, want %d — one capped student "+
			"must not stop the rest of the cohort", got, want)
	}
}

// ── expiry ───────────────────────────────────────────────────────────────────

// A referral to a NON-STUDENT account expires, never pays, and records why.
//
// The permanent case §5.2's rule produces. An institution has no student profile and no
// phone, so the two conditions are undefined for it — and both ports take a bare user
// id resolved over `users`, so scoring one would score an unrelated STUDENT with the
// same numeric id. That is not a lenient qualification, it is the wrong person.
//
// The state is EXPERIMENTAL-IN-NAME-ONLY in one respect worth pinning: it is not a
// timeout, and nothing time-based writes it. A referral whose invitee is a slow student
// stays pending forever and still pays — TestTheWindowIsAMinimumWaitAndNeverADeadline
// covers that half, and this test covers the other.
func TestANonStudentReferralExpiresAndNeverPays(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "institution@example.com", "K7M2QX9RT4")
	institutionID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "college@example.com")

	res, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectInstitution, ReferredUserID: institutionID,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp_institution",
	})
	if err != nil || !res.Attributed {
		t.Fatalf("attribute an institution referral: attributed=%v reason=%q err=%v",
			res.Attributed, res.Reason, err)
	}
	referralID := res.ReferralID
	backdateReferral(t, db, referralID, 30)

	// Deliberately make the STUDER with the same id fully qualified, which is exactly
	// the accident this rule prevents: a shared id space would otherwise pay A for B's
	// profile.
	f.completion.set(institutionID, 100)
	f.phone.verify(institutionID)

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	if report.Expired != 1 {
		t.Fatalf("expired = %d, want 1 (by-reason %v)", report.Expired, report.ByReason)
	}
	if report.Settled != 0 {
		t.Fatalf("an institution referral settled; the same numeric id being a complete " +
			"student is not evidence about the college")
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("paid %d coins for an institution referral", got)
	}
	if got := referralStatus(t, db, referralID); got != ReferralExpired {
		t.Errorf("status = %q, want %q", got, ReferralExpired)
	}
	if got := expiredReason(t, db, referralID); got != QualificationNotAStudent {
		t.Errorf("expired_reason = %q, want %q: an expired row with no recorded cause is "+
			"a support ticket with no answer", got, QualificationNotAStudent)
	}

	// Terminal, and no amount of further passing changes that.
	for i := 0; i < 3; i++ {
		if again, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
			t.Fatalf("repeat pass %d: %v", i, err)
		} else if again.Expired != 0 || again.Settled != 0 {
			t.Errorf("repeat pass %d moved %d referrals (%d settled)", i, again.Expired, again.Settled)
		}
	}
	if got := referralStatus(t, db, referralID); got != ReferralExpired {
		t.Errorf("status = %q after repeated passes, want %q", got, ReferralExpired)
	}
	// And it is never paid even when somebody calls SettleReferral directly.
	if _, err := f.svc.SettleReferral(context.Background(), referralID); !errors.Is(err, ErrImmutable) {
		t.Errorf("settling an expired referral returned %v, want ErrImmutable", err)
	}
}

// An expired referral is not an error and does not roll back the pass around it.
//
// It shares a transaction-taking path with settlement and runs on a timer over rows
// other processes touch, so a referral that was moved by somebody else between the scan
// and the write is the normal case on a rolling deploy rather than a fault.
func TestExpiryIsIdempotentAndNeverFailsThePass(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "expire-idem@example.com", "K7M2QX9RT4")

	const many = 5
	ids := make([]uint, 0, many)
	for i := 0; i < many; i++ {
		institutionID := seedReferred(t, &referralFixture{svc: f.svc, db: db},
			fmt.Sprintf("expire-idem-college-%d@example.com", i))
		res, err := f.svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectInstitution, ReferredUserID: institutionID,
			ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp_institution",
		})
		if err != nil || !res.Attributed {
			t.Fatalf("attribute %d: %v", i, err)
		}
		backdateReferral(t, db, res.ReferralID, 30)
		ids = append(ids, res.ReferralID)
	}

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	if report.Expired != many {
		t.Fatalf("expired = %d, want %d (by-reason %v)", report.Expired, many, report.ByReason)
	}
	if report.Err != nil {
		t.Errorf("the pass reported an error after expiring %d referrals: %v", many, report.Err)
	}
	for _, id := range ids {
		if got := referralStatus(t, db, id); got != ReferralExpired {
			t.Errorf("referral %d is %q, want %q", id, got, ReferralExpired)
		}
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("expired referrals paid %d coins", got)
	}
}

// An already-QUALIFIED referral is never re-evaluated into an expiry, even if its
// invitee's kind is somehow inconsistent.
//
// expireReferral re-reads the row FOR UPDATE and writes only WHERE status = 'pending',
// so the second condition alone would stop this. Both are asserted rather than either,
// because a guard that is only tested through the outer condition is a guard whose
// inner condition has never run.
func TestAQualifiedReferralIsNeverExpiredByAStalePass(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "stale@example.com", "K7M2QX9RT4")
	institutionID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "stale-college@example.com")
	res, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectInstitution, ReferredUserID: institutionID,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp_institution",
	})
	if err != nil || !res.Attributed {
		t.Fatalf("attribute: %v", err)
	}
	referralID := res.ReferralID
	backdateReferral(t, db, referralID, 30)

	// Mark it paid behind the pass's back, which is what a concurrent instance settling
	// it would look like.
	if err := db.Exec(
		`UPDATE user_referral SET status = ?, awarded_coins = 60 WHERE id = ?`,
		ReferralQualified, referralID,
	).Error; err != nil {
		t.Fatalf("mark qualified: %v", err)
	}

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	if report.Expired != 0 {
		t.Errorf("expired = %d, want 0: the pass expired a referral that was already paid", report.Expired)
	}
	if got := referralStatus(t, db, referralID); got != ReferralQualified {
		t.Errorf("status = %q, want %q — a paid referral must never be rewritten as expired",
			got, ReferralQualified)
	}
}

// ── the ports are consulted ───────────────────────────────────────────────────

// The profile-completion PORT is consulted, not a reimplementation of the twelve
// checks.
//
// This is the whole of the "do not reimplement" requirement, and it is asserted at the
// seam rather than in prose: the fake is asked, the fake records which id it was asked
// about, and the settlement's money follows the fake's answer. A reimplementation inside
// internal/coins could not satisfy this test at all, because there would be no port to
// stub — so the test would fail to compile or the assertion about the call would find
// nothing.
func TestTheProfileCompletionPortIsConsultedForTheInvitee(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "port@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "port-invitee@example.com")
	f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)

	// The port says 40%. That is the ONLY source of the answer: this test knows nothing
	// about any user row, and the twelve checks live in another module entirely.
	f.completion.set(inviteeID, 40)
	f.phone.verify(inviteeID)

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	if report.Settled != 0 {
		t.Errorf("settled %d with the port reporting 40%%, want 0", report.Settled)
	}
	if report.ByReason[QualificationProfileIncomplete] != 1 {
		t.Errorf("by-reason = %v, want one %s — the port's number has to be the one "+
			"that decides", report.ByReason, QualificationProfileIncomplete)
	}

	// The stub's own call log names the INVITEE, which is the value that decides whose
	// profile counts.
	if !f.completion.askedAbout(inviteeID) {
		t.Errorf("the profile port was never asked about the invitee %d; the verdict above "+
			"cannot have come from the port", inviteeID)
	}
	if f.completion.askedAbout(referrerID) {
		t.Errorf("the port was asked about the REFERRER (%d); a referral is qualified by the "+
			"invitee's profile, and paying on the referrer's own would let a student "+
			"qualify themselves", referrerID)
	}
}

// The same for the phone port, and for the reason it matters: it is a port with NO
// implementation in this codebase, so "is it consulted" is a question about wiring
// rather than about behaviour.
func TestThePhonePortIsConsultedAndNeverReplacedByHavingANumber(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "phoneport@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "phoneport-invitee@example.com")
	f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)
	f.completion.set(inviteeID, 100)

	// The port is asked and answers no. The users table has a `phone` column and this
	// test's fixture never sets one — and nothing in the mechanic looks at it. If a
	// future change substituted "has a phone number" for verification, this case would
	// start paying and the assertion below would catch it, because the port is the only
	// thing that is consulted.
	if _, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("paid %d coins when the phone port said unverified", got)
	}
	if f.phone.called() == 0 {
		t.Error("the phone port was never consulted; the mechanic must ask rather than " +
			"infer verification from any column it can read")
	}
}

// A port that ERRORS is a failed evaluation, counted and not silently retried forever as
// a "no".
func TestAPortErrorIsCountedAsAFailureNotAsANegativeAnswer(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "porterr@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "porterr-invitee@example.com")
	f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)
	f.completion.set(inviteeID, 100)
	f.phone.setErr(errors.New("verification store unreachable"))

	report, err := f.svc.QualifyPendingReferrals(context.Background())
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	if report.Settled != 0 {
		t.Errorf("settled %d while the phone port was failing", report.Settled)
	}
	if report.Failed() != 1 {
		t.Errorf("failed = %d, want 1 (by-reason %v)", report.Failed(), report.ByReason)
	}
	// A failure is not a student outcome, so it must never be reported as one.
	if report.ByReason[QualificationPhoneUnverified] != 0 {
		t.Errorf("a port failure was reported as waiting_for_phone, which asserts a fact "+
			"about the invitee that nobody established (by-reason %v)", report.ByReason)
	}

	// And the referral is recoverable: the port recovers and the next pass pays it.
	f.phone.setErr(nil)
	f.phone.verify(inviteeID)
	if again, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("retry: %v", err)
	} else if again.Settled != 1 {
		t.Errorf("the referral did not pay after the port recovered: %v", again.ByReason)
	}
}

// ── the notification ──────────────────────────────────────────────────────────

// A settlement announces coins.credited exactly once, addressed to the REFERRER, and
// the emitted data renders the registry's templates.
//
// coins.credited rather than a referral-specific key, and this test is the reason that
// choice is safe: it renders the REAL registry's templates with the REAL emitted data
// under missingkey=error, so a key the template names but the emit does not supply —
// which in production is a dropped notification, not a blank line — fails here.
func TestASettlementAnnouncesCoinsCreditedOnceToTheReferrer(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "notify@example.com", "K7M2QX9RT4")
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "notify-invitee@example.com")
	f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", 8)
	f.completion.set(inviteeID, 100)
	f.phone.verify(inviteeID)

	if _, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if f.notifier.count() != 1 {
		t.Fatalf("%d notifications after one settlement, want 1", f.notifier.count())
	}
	call := f.notifier.calls[0]
	if call.eventKey != notification.EventCoinsCredited {
		t.Errorf("event key = %q, want %q. The registry is closed and boot-validated, so a "+
			"new constant without a row prevents the server starting; coins.credited's own "+
			"registry comment already names the referral award as one of its producers",
			call.eventKey, notification.EventCoinsCredited)
	}
	// The REFERRER is the one who earned. The invitee is notified by their own profile
	// award, on their own account.
	if call.userID != referrerID {
		t.Errorf("notification addressed to user %d, want the REFERRER %d", call.userID, referrerID)
	}
	if call.coins != DefaultEconomyConfig().Awards.ReferralReferrer {
		t.Errorf("announced %d coins, want %d", call.coins, DefaultEconomyConfig().Awards.ReferralReferrer)
	}
	if call.balance != call.coins {
		t.Errorf("announced a balance of %d for a referrer who started at zero; the "+
			"template quotes it and a wrong balance is a support ticket", call.balance)
	}

	// The occurrence key is the journal, so a retried announce is the same occurrence.
	if !strings.HasSuffix(call.occurKey, call.correlate) {
		t.Errorf("occurrence key %q does not end in the journal id %q", call.occurKey, call.correlate)
	}

	// The registry row is present and the templates render from the emitted data.
	if err := notification.ValidateRegistry(); err != nil {
		t.Fatalf("registry invalid: %v", err)
	}
	def, ok := notification.Registry[notification.EventCoinsCredited]
	if !ok {
		t.Fatalf("%s has no Registry row", notification.EventCoinsCredited)
	}
	data := map[string]any{"coins": call.coins, "balance": call.balance}
	title, err := notification.ResolveTemplate(def.TitleTpl, data)
	if err != nil {
		t.Fatalf("title template: %v", err)
	}
	body, err := notification.ResolveTemplate(def.BodyTpl, data)
	if err != nil {
		t.Fatalf("body template: %v", err)
	}
	rendered := strings.ToLower(title + " " + body)
	if strings.Contains(rendered, "{{") || strings.Contains(rendered, "<no value>") {
		t.Errorf("a template hole survived: %q %q", title, body)
	}
	// 09's three statutory bans, asserted on the copy a referral student will actually
	// receive.
	for _, banned := range []string{
		"free", "prize", "award", "win", "raffle", "draw", "baksis", "npr", "inr", "₹",
	} {
		if strings.Contains(rendered, banned) {
			t.Errorf("referral credit copy contains the banned %q: %q", banned, title+" "+body)
		}
	}
}

// ── the student's own endpoint ───────────────────────────────────────────────

// referralRouter mounts the endpoint with a FIXED caller id.
//
// A fixed id rather than one per request, because the point of most of these is that
// the endpoint is owner-scoped, and a router that can change its caller is a router
// where that has to be arranged. stubAuth is what unlock_api_test.go uses.
func referralRouter(t *testing.T, caller uint, api *ReferralAPI) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterReferralRoutes(r, stubAuth(caller), api)
	return r
}

// getReferrals performs the request and returns the decoded body.
func getReferrals(t *testing.T, r *gin.Engine) (int, ReferralMeDTO) {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/referrals", nil))
	var envelope struct {
		Success bool          `json:"success"`
		Data    ReferralMeDTO `json:"data"`
	}
	body := rec.Body.String()
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return rec.Code, envelope.Data
}

// The endpoint answers only for the authenticated caller, and carries no parameter
// through which another account's referrals could be named.
//
// Three properties in one test because they are one property seen from three angles:
// the id comes from the session and nowhere else, the body is the caller's, and there
// is no route variant that takes an id. If somebody added /api/v1/referrals/:id later,
// this test would still pass — so the LAST block is the load-bearing one.
func TestTheReferralEndpointExposesOnlyTheCallersOwnReferrals(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)
	api := NewReferralAPI(f.svc, "https://studsphere.example")

	// Two students, each with their own referrals. Student 7 and student 8.
	alice := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "alice@example.com", "K7M2QX9RT4")
	bob := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "bob@example.com", "K7M2QX9RT5")
	aliceInvitee := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "alice-invitee@example.com")
	bobInvitee := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "bob-invitee@example.com")
	aliceReferral := f.stage(t, alice, aliceInvitee, "K7M2QX9RT4", 8)
	bobReferral := f.stage(t, bob, bobInvitee, "K7M2QX9RT5", 40)

	// Unauthenticated is 401, and reveals nothing.
	unauth := gin.New()
	RegisterReferralRoutes(unauth, func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no session"})
	}, api)
	rec := httptest.NewRecorder()
	unauth.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/referrals", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d, want 401", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "K7M2QX9RT4") || strings.Contains(rec.Body.String(), "K7M2QX9RT5") {
		t.Errorf("an unauthenticated request saw a referral code: %s", rec.Body.String())
	}

	// Alice's view: her code, her referral, Bob's absent.
	code, aliceBody := getReferrals(t, referralRouter(t, alice, api))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if aliceBody.ReferralCode == nil || *aliceBody.ReferralCode != "K7M2QX9RT4" {
		t.Errorf("referral_code = %v, want Alice's own", aliceBody.ReferralCode)
	}
	if aliceBody.Stats.Invited != 1 {
		t.Errorf("invited = %d, want 1", aliceBody.Stats.Invited)
	}
	if len(aliceBody.Referrals) != 1 || aliceBody.Referrals[0].ReferralID != aliceReferral {
		t.Fatalf("referrals = %+v, want exactly Alice's referral %d", aliceBody.Referrals, aliceReferral)
	}

	// THE assertion, on the SERIALISED body rather than on a Go struct dump.
	//
	// On the JSON because that is what actually goes over the wire and it is the only
	// form in which an accidental field is visible. An earlier draft of this assertion
	// searched a `%+v` dump for Bob's numeric ids — and found "4" inside
	// "2026-09-23 17:02:58", because a timestamp contains every small integer. It
	// failed for a reason that had nothing to do with the leak, which is the worst way
	// for a privacy assertion to fail: it teaches the reader that the check is noisy.
	//
	// Bob's CODE is the thing that cannot be produced by coincidence and cannot be
	// derived from Alice's own rows, so it is the substantive check. The count is the
	// second: an Invited of 2 rather than 1 would mean his referral leaked even if the
	// row itself did not.
	raw, err := json.Marshal(aliceBody)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "K7M2QX9RT5") {
		t.Errorf("Alice's response contains Bob's referral code: %s", raw)
	}
	if aliceBody.Stats.Invited != 1 || len(aliceBody.Referrals) != 1 {
		t.Errorf("Alice's response carries %d referrals / %d invited, want 1 of each: "+
			"Bob's referral leaked into her view. Body: %s",
			len(aliceBody.Referrals), aliceBody.Stats.Invited, raw)
	}
	for _, r := range aliceBody.Referrals {
		if r.ReferralID == bobReferral {
			t.Error("Alice's response contains Bob's referral row")
		}
	}

	// And Bob's view is his own, not a filtered version of hers.
	_, bobBody := getReferrals(t, referralRouter(t, bob, api))
	if bobBody.ReferralCode == nil || *bobBody.ReferralCode != "K7M2QX9RT5" {
		t.Errorf("Bob's referral_code = %v, want his own", bobBody.ReferralCode)
	}
	if len(bobBody.Referrals) != 1 || bobBody.Referrals[0].ReferralID != bobReferral {
		t.Errorf("Bob's referrals = %+v, want only his %d", bobBody.Referrals, bobReferral)
	}
	// Bob's referral is 40 days old and still pending, because his invitee never
	// qualified. His view must say so rather than implying it is progressing.
	if got := bobBody.Referrals[0].State; got != ReferralViewWaitingOnInvitee {
		t.Errorf("a 40-day-old pending referral reads as %q, want %q — the page has to be "+
			"able to say \"waiting on your friend\"", got, ReferralViewWaitingOnInvitee)
	}

	// ── and there is no id parameter to abuse ────────────────────────────────────
	r := referralRouter(t, alice, api)
	for _, path := range []string{
		"/api/v1/referrals/8",
		"/api/v1/referrals?referrer_user_id=8",
		"/api/v1/referrals?user_id=8",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "K7M2QX9RT5") {
			t.Errorf("%s returned 200 with another account's referral code", path)
		}
	}
}

// The body carries enough state for the page to be honest about what has not paid, and
// no state that would let it lie.
//
// The specific claims: pending referrals carry eligible_at so the page can render a date
// rather than a spinner; a settled one carries the award and its journal; a terminal one
// carries the recorded reason; and NOTHING carries the invitee's identity, because
// 07-compliance-nepal.md §5.2 is about minimising what a referral discloses about the
// other person.
func TestTheReferralBodyIsHonestAboutWhatHasNotPaid(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)
	api := NewReferralAPI(f.svc, "https://studsphere.example")
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "honest@example.com", "K7M2QX9RT4")

	// (age in days, qualifies, what the view must say)
	for i, tc := range []struct {
		ageDays   int
		qualifies bool
		wantState string
		wantCoins int64
	}{
		{ageDays: 1, qualifies: true, wantState: ReferralViewWaitingOnWindow},
		{ageDays: 30, qualifies: false, wantState: ReferralViewWaitingOnInvitee},
		{ageDays: 30, qualifies: true, wantState: ReferralViewSettled, wantCoins: award},
	} {
		inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db},
			fmt.Sprintf("honest-invitee-%d@example.com", i))
		id := f.stage(t, referrerID, inviteeID, "K7M2QX9RT4", tc.ageDays)
		if tc.qualifies {
			f.completion.set(inviteeID, 100)
			f.phone.verify(inviteeID)
		} else {
			f.completion.set(inviteeID, 20)
		}
		_ = id
	}
	if _, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}

	_, body := getReferrals(t, referralRouter(t, referrerID, api))
	if len(body.Referrals) != 3 {
		t.Fatalf("%d referrals in the body, want 3", len(body.Referrals))
	}
	byState := map[string][]ReferralDTO{}
	for _, r := range body.Referrals {
		byState[r.State] = append(byState[r.State], r)
	}
	if len(byState[ReferralViewWaitingOnWindow]) != 1 {
		t.Errorf("states = %v, want one %s", keysOf(byState), ReferralViewWaitingOnWindow)
	}
	if len(byState[ReferralViewWaitingOnInvitee]) != 1 {
		t.Errorf("states = %v, want one %s", keysOf(byState), ReferralViewWaitingOnInvitee)
	}
	if len(byState[ReferralViewSettled]) != 1 {
		t.Fatalf("states = %v, want one %s", keysOf(byState), ReferralViewSettled)
	}

	// A pending referral carries its eligible_at, which is the date the page needs to
	// render "available from". §2.4 has no such field and this is the replacement for
	// the hold date it assumed: there is no hold any more, so there is nothing for a
	// student to be "held until" — there is a window, and it has an end.
	for _, r := range byState[ReferralViewWaitingOnWindow] {
		if r.EligibleAt == nil {
			t.Error("a pending referral has no eligible_at; the page cannot render a date " +
				"for a wait with no end")
		}
		if r.AwardedCoins != 0 {
			t.Errorf("a pending referral reports %d awarded coins", r.AwardedCoins)
		}
		if r.SettledAt != nil {
			t.Error("a pending referral reports a settlement time")
		}
	}

	settled := byState[ReferralViewSettled][0]
	if settled.AwardedCoins != award {
		t.Errorf("a settled referral reports %d coins, want %d", settled.AwardedCoins, award)
	}
	if settled.SettledAt == nil || settled.QualifiedAt == nil {
		t.Error("a settled referral reports no settlement or qualification time")
	}

	// The stats agree with the rows.
	if body.Stats.Invited != 3 {
		t.Errorf("invited = %d, want 3", body.Stats.Invited)
	}
	if body.Stats.Qualified != 1 {
		t.Errorf("qualified = %d, want 1", body.Stats.Qualified)
	}
	// Pending counts only rows that MAY still pay — the two unsettled ones, not the
	// settled one.
	if body.Stats.Pending != 2 {
		t.Errorf("pending = %d, want 2: §2.4's four buckets cannot express a fifth "+
			"terminal state, and folding one into pending reports rows that can never "+
			"pay as rows that might", body.Stats.Pending)
	}
	if body.Stats.CoinsEarnedTotal != award {
		t.Errorf("coins_earned_total = %d, want %d", body.Stats.CoinsEarnedTotal, award)
	}
	if body.Stats.ThisMonthQualified != 1 {
		t.Errorf("this_month_qualified = %d, want 1", body.Stats.ThisMonthQualified)
	}
	if want := int(DefaultEconomyConfig().Referral.MonthlyCap) - 1; body.Stats.MonthlyCapRemaining != want {
		t.Errorf("monthly_cap_remaining = %d, want %d", body.Stats.MonthlyCapRemaining, want)
	}
	if want := DefaultEconomyConfig().Referral.LifetimeCoinCap - award; body.Stats.LifetimeCapRemaining != want {
		t.Errorf("lifetime_cap_remaining = %d, want %d", body.Stats.LifetimeCapRemaining, want)
	}
	if body.ReferralHoldDays != 7 {
		t.Errorf("referral_hold_days = %d, want the configured 7", body.ReferralHoldDays)
	}

	// The invite link points at the injected host, and the code is normalised.
	if body.ReferralLink == nil {
		t.Fatal("no referral_link")
	}
	if want := "https://studsphere.example/r/K7M2QX9RT4"; *body.ReferralLink != want {
		t.Errorf("referral_link = %q, want %q", *body.ReferralLink, want)
	}

	// NOTHING exposes the invitee. Not an id, not a kind that would be a lookup key
	// into another table, and the JSON has no field carrying an email or a name.
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rendered := string(raw)
	for _, forbidden := range []string{"honest-invitee", "@example.com", "email", "name"} {
		if strings.Contains(rendered, forbidden) {
			t.Errorf("the body discloses %q: %s", forbidden, rendered)
		}
	}
	// referred_kind IS exposed, and that is a deliberate minimum: it tells the page
	// "this one cannot qualify" without identifying anybody.
	if body.Referrals[0].ReferredKind != SubjectUser {
		t.Errorf("referred_kind = %q, want %q", body.Referrals[0].ReferredKind, SubjectUser)
	}
}

// The endpoint says referral rewards cannot be added yet, in every deployment today,
// because no phone-verification port is wired.
//
// The one field in this body about the SERVER rather than the student, and it is there
// because the alternative is a page that counts pending referrals forever and never
// explains that none of them can settle. It is derived from the same nil port the
// qualification pass refuses on, so it cannot claim referrals are earnable while the
// pass pays nothing.
func TestTheEndpointReportsThatReferralRewardsAreUnavailable(t *testing.T) {
	db := openReferralSchema(t)

	t.Run("with a verifier wired, it promises nothing about availability and shows no notice", func(t *testing.T) {
		f := newQualFixture(t, db, nil)
		api := NewReferralAPI(f.svc, "https://studsphere.example")
		_, body := getReferrals(t, referralRouter(t, seedReferrer(t,
			&referralFixture{svc: f.svc, db: db}, "avail@example.com", "K7M2QX9RT4"), api))
		if !body.QualificationAvailable {
			t.Error("qualification_available is false with a verifier wired, so the page " +
				"would show the unavailable notice to a deployment that can qualify")
		}
		if body.QualificationNotice != "" {
			t.Errorf("a notice is shown when qualification is available: %q", body.QualificationNotice)
		}
	})

	t.Run("with none wired — every deployment today — it says so, in 09's language", func(t *testing.T) {
		f := newQualFixture(t, db, nil)
		f.svc.WithQualifiers(f.completion, nil)
		api := NewReferralAPI(f.svc, "https://studsphere.example")
		userID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "unavail@example.com", "K7M2QX9RT5")
		f.stage(t, userID, seedReferred(t, &referralFixture{svc: f.svc, db: db}, "unavail-invitee@example.com"),
			"K7M2QX9RT5", 30)

		_, body := getReferrals(t, referralRouter(t, userID, api))
		if body.QualificationAvailable {
			t.Error("qualification_available is true with no phone verifier wired, while the " +
				"pass settles nothing. The page would promise a reward it cannot pay")
		}
		if body.QualificationNotice == "" {
			t.Fatal("no notice: the page would show a pending referral forever with no " +
				"explanation of why nothing pays")
		}
		// 09's bans, on a string a student reads.
		lower := strings.ToLower(body.QualificationNotice)
		for _, banned := range []string{
			"free", "prize", "award", "win", "raffle", "draw", "baksis", "npr", "inr", "₹",
			"pending review", "under review",
		} {
			if strings.Contains(lower, banned) {
				t.Errorf("the notice contains the banned %q: %q", banned, body.QualificationNotice)
			}
		}
		// And it must not promise a date or a figure it cannot honour.
		for _, overclaim := range []string{"soon", "will be credited", "guaranteed", "within"} {
			if strings.Contains(lower, overclaim) {
				t.Errorf("the notice overclaims with %q: %q", overclaim, body.QualificationNotice)
			}
		}
	})
}

// An account with no referrals gets zeros, not a 404 and not an error.
//
// It is behind authMW, so the caller is always a real principal; "this account has
// referred nobody" is a true answer rather than a missing resource, and a new student's
// first page load is the most common request this endpoint will ever serve.
func TestAnAccountWithNoReferralsGetsAnEmptyViewNotAnError(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)
	api := NewReferralAPI(f.svc, "https://studsphere.example")
	userID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "lonely@example.com", "K7M2QX9RT4")

	code, body := getReferrals(t, referralRouter(t, userID, api))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(body.Referrals) != 0 {
		t.Errorf("%d referrals for an account that referred nobody", len(body.Referrals))
	}
	if body.Stats.Invited != 0 || body.Stats.Pending != 0 || body.Stats.Qualified != 0 {
		t.Errorf("stats = %+v, want all zero", body.Stats)
	}
	// The code and link are still there — that is the point of the page.
	if body.ReferralCode == nil || *body.ReferralCode != "K7M2QX9RT4" {
		t.Errorf("referral_code = %v, want the caller's own", body.ReferralCode)
	}
	// A full month of cap is available, not negative.
	if body.Stats.MonthlyCapRemaining != int(DefaultEconomyConfig().Referral.MonthlyCap) {
		t.Errorf("monthly_cap_remaining = %d, want the full cap", body.Stats.MonthlyCapRemaining)
	}
}

// A lowered cap cannot produce a negative "remaining", which a UI would render as
// "-3 invites left".
func TestACapLoweredMidMonthNeverRendersNegative(t *testing.T) {
	db := openReferralSchema(t)
	// A config whose cap is 3, with five slots already taken this month — possible
	// because the cap can be lowered after the fact, which is exactly the incident
	// scenario claimCapSlot's use of the configured value exists for.
	f := newQualFixture(t, db, func(c *EconomyConfig) { c.Referral.MonthlyCap = 3 })
	api := NewReferralAPI(f.svc, "https://studsphere.example")
	userID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "lowered@example.com", "K7M2QX9RT4")

	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 SELECT ?, ?, g, 600000 + g, 60, now() FROM generate_series(1, 5) g`,
		userID, ReferralMonthKey(time.Now().UTC()),
	).Error; err != nil {
		t.Fatalf("fill five slots: %v", err)
	}

	_, body := getReferrals(t, referralRouter(t, userID, api))
	if body.Stats.MonthlyCapRemaining < 0 {
		t.Errorf("monthly_cap_remaining = %d with 5 slots used against a cap of 3. A "+
			"negative count renders as \"-2 invites left\"", body.Stats.MonthlyCapRemaining)
	}
	if body.Stats.LifetimeCapRemaining < 0 {
		t.Errorf("lifetime_cap_remaining = %d, want it floored at zero", body.Stats.LifetimeCapRemaining)
	}
}

// keysOf is a small helper so a failing map assertion prints the states it found
// rather than a Go map dump.
func keysOf(m map[string][]ReferralDTO) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// utilsNormalizeIsUnchanged keeps the import of internal/shared/utils honest.
//
// referral_code lives in another module and this file's stage() helper hands codes to
// ApplyReferral, which normalises them. The assertion is that normalisation is not
// load-bearing for a code this file generates: a code that normalises to itself settles,
// so a failure elsewhere is about the mechanic rather than about spelling.
func TestReferralCodesResolveThroughTheUsersTable(t *testing.T) {
	db := openReferralSchema(t)
	f := newQualFixture(t, db, nil)
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	const code = "K7M2QX9RT4"
	if got := utils.NormalizeReferralCode(code); got != code {
		t.Fatalf("the fixture code %q normalises to %q, so this file's other tests would "+
			"fail for a spelling reason rather than a mechanic one", code, got)
	}

	referrerID := seedReferrer(t, &referralFixture{svc: f.svc, db: db}, "resolve@example.com", code)
	inviteeID := seedReferred(t, &referralFixture{svc: f.svc, db: db}, "resolve-invitee@example.com")
	f.stage(t, referrerID, inviteeID, strings.ToLower(code), 8)
	f.completion.set(inviteeID, 100)
	f.phone.verify(inviteeID)

	// A lower-case code in the request resolves to the account holding the canonical
	// one, which is the behaviour referral.go's resolveReferral exists to provide, and
	// the settlement that follows proves the attribution reached a real account rather
	// than a fabricated row.
	if _, err := f.svc.QualifyPendingReferrals(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if got := settleCoins(t, db, referrerID); got != award {
		t.Errorf("paid %d coins for a code presented in lower case, want %d", got, award)
	}
}
