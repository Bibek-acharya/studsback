//go:build coinsintegration

// internal/coins/expiry_reminder_pg_test.go
//
// 04 §7's "reminders at 30 / 7 / 1 days, per lot", and the other half of "never
// silently delete an expired balance … and always tell the student".
//
// Against Postgres, because the property that matters most is IDEMPOTENCE across
// repeated runs of an hourly job, and a fake notifier records calls without ever
// being able to show that the second pass sent nothing.

package coins

import (
	"context"
	"testing"
	"time"

	"studsphere/backend/internal/notification"

	"gorm.io/gorm"
)

// expiryReminderNotifier captures what the reminder job announced.
//
// It records EVERY call including the ones a real registry would suppress, because
// the OccurrenceKey is what does the suppressing in production — and asserting that
// the job does not even ATTEMPT a second call is a stronger and simpler property than
// asserting the registry would have dropped it.
type expiryReminderNotifier struct {
	calls []notification.NotifyRequest
	// sent is the store `Sent` reads, so the double actually ACCEPTS its own writes
	// rather than pretending every lookup is a miss. A double that always answered
	// false would make the idempotence test pass for the wrong reason — it would be
	// testing that the job asks, not that asking twice yields one notice.
	sent map[string]bool
	// sentErr makes `Sent` fail, for the test that a lookup error must not skip the
	// pass. A missing 1-day notice is worse than a duplicate one.
	sentErr error
	// onNotifyTx receives the burn's own transaction, so a test can read the
	// transaction's UNCOMMITTED state — the only way to prove the notice was emitted
	// inside it rather than after it.
	onNotifyTx func(tx *gorm.DB)
}

func newExpiryReminderNotifier() *expiryReminderNotifier {
	return &expiryReminderNotifier{sent: map[string]bool{}}
}

func (r *expiryReminderNotifier) Notify(_ context.Context, req notification.NotifyRequest) error {
	r.calls = append(r.calls, req)
	if r.sent != nil {
		r.sent[req.OccurrenceKey] = true
	}
	return nil
}

// NotifyTx satisfies the port's transactional half, and — this is the point — does
// NOT behave the same as Notify.
//
// It gets the caller's *gorm.DB, so it can read the burn's UNCOMMITTED writes. That is
// the whole difference between the two methods, and a double that simply delegated
// would erase it: with the delegation, replacing `NotifyTx(ctx, tx.DB(), ...)` back to
// `Notify(ctx, ...)` in the sweep changed no observable behaviour and every test still
// passed. Falsifying that way is how a transaction-boundary requirement survives
// review with nothing holding it up.
//
// So NotifyTx runs a separate hook that receives the transaction, and a test asserts
// the hook SAW state that only exists inside it.
func (r *expiryReminderNotifier) NotifyTx(ctx context.Context, tx *gorm.DB, req notification.NotifyRequest) error {
	if r.onNotifyTx != nil {
		r.onNotifyTx(tx)
	}
	return r.Notify(ctx, req)
}

func (r *expiryReminderNotifier) Sent(_ context.Context, key string, _ notification.Ref) (bool, error) {
	if r.sentErr != nil {
		return false, r.sentErr
	}
	return r.sent[key], nil
}

func (r *expiryReminderNotifier) forEvent(key string) []notification.NotifyRequest {
	var out []notification.NotifyRequest
	for _, c := range r.calls {
		if c.EventKey == key {
			out = append(out, c)
		}
	}
	return out
}

// THE test. A lot inside each window gets exactly one notice, and a lot outside every
// window gets none.
func TestRemindersFireOncePerThresholdAndNeverOutsideIt(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := newReminderSweeper(env, notifier)

	// Four lots, one in each band plus one far out.
	//
	// The bands are (d-1 days, d days], so a lot is announced from the moment it
	// CROSSES the threshold until the next one. 29 days out is therefore INSIDE the
	// 30-day band — it crossed 30 somewhere in the last day — and an earlier draft of
	// this test asserted it should not be, which was wrong about the design rather
	// than catching a bug. In practice the job runs hourly, so it fires at 30d minus
	// minutes; a lot sitting at 29d only reaches the band if the job was down, and a
	// late notice there is the right trade against a missed one. The OccurrenceKey
	// stops the second day re-sending it.
	//
	// Out of band therefore means genuinely between the windows: 45 days is before
	// any of them and 20 sits in the gap between the 30-day and 7-day bands.
	for _, spec := range []struct {
		user  uint
		days  int
		label string
	}{
		{4242, 30, "30 days — inside the 30-day band"},
		{4243, 45, "45 days — before every band"},
		{4244, 7, "7 days — inside the 7-day band"},
		{4245, 20, "20 days — between the 30-day and 7-day bands"},
		{4246, 1, "1 day — inside the 1-day band"},
	} {
		env.grantExpiring(t, spec.user, time.Duration(spec.days)*24*time.Hour)
	}

	report, err := sweeper.RemindExpiring(context.Background())
	if err != nil {
		t.Fatalf("remind: %v", err)
	}

	calls := notifier.forEvent(notification.EventCoinsExpiring)
	if len(calls) != 3 {
		t.Fatalf("%d expiry reminders sent, want 3 (the 30-day, 7-day and 1-day lots): %+v",
			len(calls), report)
	}
	gotUsers := map[uint]bool{}
	for _, c := range calls {
		gotUsers[c.Recipients[0].ID] = true
	}
	for _, user := range []uint{4243, 4245} {
		if gotUsers[user] {
			t.Errorf("a lot outside every band was reminded about (user %d)", user)
		}
	}

	// The notice must carry the DAYS so the template can say "in N days", and the lot's
	// own remaining balance rather than the student's total.
	byUser := map[uint]notification.NotifyRequest{}
	for _, c := range calls {
		byUser[c.Recipients[0].ID] = c
	}
	thirty, ok := byUser[4242]
	if !ok {
		t.Fatal("the 30-day lot got no reminder")
	}
	if got := thirty.Data["days"]; got == nil {
		t.Error("the reminder carries no `days`; the template interpolates {{.days}} with " +
			"missingkey=error, so this would be a DROPPED notification rather than a bad one")
	}
	if thirty.Data["coins"] == nil {
		t.Error("the reminder carries no `coins`; the same missing-key drop applies")
	}
	if thirty.Data["expires_at"] == nil {
		t.Error("the reminder carries no `expires_at`; 06 §1.4 requires expiry to be a " +
			"DATE rather than a bare day count")
	}
}

// THE idempotence test. An hourly job runs this repeatedly; the second run must be
// silent, and so must the twenty-first.
func TestTheReminderJobIsSilentOnEveryRunAfterTheFirst(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := newReminderSweeper(env, notifier)

	env.grantExpiring(t, 4242, 30*24*time.Hour)

	if _, err := sweeper.RemindExpiring(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := len(notifier.forEvent(notification.EventCoinsExpiring))
	if first != 1 {
		t.Fatalf("first run sent %d reminders, want 1", first)
	}

	// Twenty more passes, as an hourly job would do across the same day.
	for i := 0; i < 20; i++ {
		if _, err := sweeper.RemindExpiring(context.Background()); err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
	}
	if got := len(notifier.forEvent(notification.EventCoinsExpiring)); got != first {
		t.Errorf("21 runs sent %d reminders, want %d — the job re-sends on every pass, "+
			"so a student gets the same notice 21 times", got, first)
	}
}

// The OccurrenceKey is the actual mechanism, so it is asserted rather than assumed.
// Keyed on lot AND threshold, which is the whole point: a student approaching 30 days
// and then 7 days must get BOTH notices, and the two must not collide.
func TestTheOccurrenceKeySeparatesLotsAndThresholds(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := newReminderSweeper(env, notifier)

	// One student, two lots, so the key cannot be just the user id.
	env.grantExpiring(t, 4242, 30*24*time.Hour)
	env.grantExpiring(t, 4242, 7*24*time.Hour)

	if _, err := sweeper.RemindExpiring(context.Background()); err != nil {
		t.Fatalf("remind: %v", err)
	}
	calls := notifier.forEvent(notification.EventCoinsExpiring)
	if len(calls) != 2 {
		t.Fatalf("%d reminders for one student's two lots, want 2", len(calls))
	}
	if calls[0].OccurrenceKey == calls[1].OccurrenceKey {
		t.Errorf("two different notices share the key %q, so the registry's dedupe would "+
			"silently drop one of them", calls[0].OccurrenceKey)
	}
	// Same lot, two thresholds, as a lot ages from 30 days out to 7. Distinct keys.
	env.pool.Exec(`UPDATE coin_lot SET expires_at = expires_at + interval '22 days'`)
	before := len(notifier.forEvent(notification.EventCoinsExpiring))
	if _, err := sweeper.RemindExpiring(context.Background()); err != nil {
		t.Fatalf("second remind: %v", err)
	}
	if after := len(notifier.forEvent(notification.EventCoinsExpiring)); after <= before {
		t.Errorf("a lot crossing from 30 days to 8 sent nothing new (%d notices)", after)
	}
}

// A lot with nothing left in it must NOT be reminded about. Telling someone "5
// StudsTokens expire" is noise; telling someone "0 StudsTokens expire" is worse, and a
// fully-consumed lot that is still open is a real state — it just has nothing in it.
func TestAFullySpentLotIsNotRemindedAbout(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := newReminderSweeper(env, notifier)

	_, lotID := env.grantExpiring(t, 4242, 30*24*time.Hour)
	// Mark it consumed without moving money — the fixture has a helper for this and
	// the point is the REMAINING count, not the ledger.
	env.setLotConsumed(t, lotID, 1)

	if _, err := sweeper.RemindExpiring(context.Background()); err != nil {
		t.Fatalf("remind: %v", err)
	}
	for _, c := range notifier.forEvent(notification.EventCoinsExpiring) {
		if c.Data["coins"] == int64(0) || c.Data["coins"] == 0 {
			t.Errorf("a fully-spent lot was reminded about: %+v", c.Data)
		}
	}
}

// A lot inside the window but CLOSED must not be reminded about either. A closed
// account is a support hold, and a student whose coins are frozen mid-hold should not
// be told they are about to expire.
func TestALotInAClosedAccountIsNotRemindedAbout(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := newReminderSweeper(env, notifier)

	env.grantExpiring(t, 4242, 30*24*time.Hour)
	var accountID uint
	env.pool.Raw(`SELECT id FROM coin_account WHERE owner_user_id = 4242 LIMIT 1`).Scan(&accountID)
	if accountID == 0 {
		t.Fatal("no account for 4242")
	}
	if err := env.pool.Exec(`UPDATE coin_account SET closed_at = now() WHERE id = ?`, accountID).Error; err != nil {
		t.Fatalf("close the account: %v", err)
	}

	if _, err := sweeper.RemindExpiring(context.Background()); err != nil {
		t.Fatalf("remind: %v", err)
	}
	for _, c := range notifier.forEvent(notification.EventCoinsExpiring) {
		if c.Recipients[0].ID == 4242 {
			t.Error("a lot in a CLOSED account was reminded about; a support hold is not " +
				"a reason to tell a student their coins are running out")
		}
	}
}

// The sweep must tell the student their coins expired. 04 §7: "never silently delete
// an expired balance … and always tell the student."
func TestTheExpirySweepAnnouncesWhatItBurned(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := env.sweeper.WithNotifier(notifier)

	grant, lotID := env.grantExpiring(t, 4242, -time.Hour)
	env.setLotConsumed(t, lotID, 1)
	burned := grant.Amount - 1

	report, err := sweeper.Sweep(context.Background(), ExpiryBatchDefault)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Expired != 1 {
		t.Fatalf("burned %d lots, want 1", report.Expired)
	}

	calls := notifier.forEvent(notification.EventCoinsExpired)
	if len(calls) != 1 {
		t.Fatalf("%d expiry notices sent, want 1 — a balance was taken away silently", len(calls))
	}
	call := calls[0]
	if call.Recipients[0].ID != 4242 {
		t.Errorf("the notice went to user %d, want 4242", call.Recipients[0].ID)
	}
	// The AMOUNT burned, not the amount granted and not the configured expiry. A
	// student told "25 StudsTokens expired" when 25 went is right; told "5 expired"
	// when 25 went, it looks like a coin theft.
	if got := call.Data["coins"]; got != burned {
		t.Errorf("notice says %v coins expired, want %d", got, burned)
	}
	// And the balance they are left with, because after this the number on screen has
	// changed and they need to be able to reconcile it.
	if call.Data["balance"] == nil {
		t.Error("the notice carries no `balance`; the template interpolates {{.balance}} " +
			"with missingkey=error, so this is a dropped notification")
	}
}

// The expiry notice is emitted AFTER the postings have moved the balance, and it
// carries the balance the student is LEFT with.
//
// The observable proof of the ordering is the balance in the notice: it is read
// through tx.AvailableInTx, which can only be correct if ApplyPosted has already run.
// A notice emitted before the postings would carry the PRE-burn figure, and a student
// reading "25 StudsTokens expired, your balance is 30" against a wallet showing 5
// concludes the system is lying to them — which, at that instant, it would be.
//
// An earlier draft of this test observed `consumed == granted` from inside the
// notifier and it read 1 of 5. That was a TEST artifact rather than a bug: the hook
// queried through the fixture's pool, a different connection, which cannot see another
// transaction's uncommitted writes. Asserting through the notification's own data
// avoids the trap rather than fighting it.
func TestTheExpiryNoticeCarriesTheBalanceTheStudentIsLeftWith(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := env.sweeper.WithNotifier(notifier)

	grant, lotID := env.grantExpiring(t, 4242, -time.Hour)
	env.setLotConsumed(t, lotID, 1)
	burned := grant.Amount - 1

	// The balance before the sweep, read through the same port the notice uses.
	var before int64
	env.pool.Raw(`
		SELECT COALESCE(SUM(b.posted_balance - b.reserved), 0)
		  FROM coin_account_balance b JOIN coin_account a ON a.id = b.account_id
		 WHERE a.kind = 'USER' AND a.owner_user_id = ?`, 4242).Scan(&before)

	if _, err := sweeper.Sweep(context.Background(), ExpiryBatchDefault); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	calls := notifier.forEvent(notification.EventCoinsExpired)
	if len(calls) != 1 {
		t.Fatalf("%d expiry notices sent for one burn", len(calls))
	}

	// The amount is what was ACTUALLY taken, not what the lot was granted. A student
	// told "5 expired" when 25 went reasonably reads as coin theft, and their wallet
	// shows the larger figure.
	if got := calls[0].Data["coins"]; got != burned {
		t.Errorf("the notice says %v coins expired, want %d — the amount burned, not the "+
			"amount granted", got, burned)
	}

	// And the balance AFTER the burn, which is the whole ordering proof.
	notified, ok := calls[0].Data["balance"].(int64)
	if !ok {
		t.Fatalf("the notice carries no usable `balance`: %#v — the template "+
			"interpolates {{.balance}} with missingkey=error, so a missing key is a "+
			"DROPPED notification rather than a blank line", calls[0].Data["balance"])
	}
	if want := before - burned; notified != want {
		t.Errorf("the notice says the balance is %d, want %d (was %d, %d burned). "+
			"A pre-burn figure here means the notice is emitted before the postings, "+
			"so a student would be told a balance the wallet will never show",
			notified, want, before, burned)
	}
}

// THE transaction-boundary test.
//
// The burn must emit its notice through NotifyTx, which enlists in the burn's own
// transaction. The proof is that the notifier can read state that exists ONLY inside
// that transaction: the lot marked fully consumed by ConsumeLot, one statement earlier.
//
// Reading the same row through an outside connection shows the pre-transaction value,
// which is why an earlier version of this test — observing consumed/granted through
// the fixture's pool — reported 1 of 5 and looked like a bug in the sweep. It was the
// wrong vantage point, and this is the right one.
func TestTheExpiryNoticeIsEmittedInsideTheBurnTransaction(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := env.sweeper.WithNotifier(notifier)

	_, lotID := env.grantExpiring(t, 4242, -time.Hour)
	env.setLotConsumed(t, lotID, 1)

	type seen struct {
		consumed int64
		granted  int64
		called   bool
	}
	var inside seen
	notifier.onNotifyTx = func(tx *gorm.DB) {
		inside.called = true
		var row struct {
			Consumed int64
			Granted  int64
		}
		tx.Raw(`SELECT consumed, granted FROM coin_lot WHERE id = ?`, lotID).Scan(&row)
		inside.consumed, inside.granted = row.Consumed, row.Granted
	}

	if _, err := sweeper.Sweep(context.Background(), ExpiryBatchDefault); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if !inside.called {
		t.Fatal("the expiry notice was not sent through NotifyTx, so it was emitted in its " +
			"OWN transaction and would survive a rollback of the burn — which is exactly " +
			"the failure 03 §5's 'emitted inside the same transaction' clause exists to " +
			"prevent")
	}
	if inside.consumed != inside.granted {
		t.Errorf("inside the transaction the lot read consumed=%d of granted=%d; the "+
			"notice must go AFTER ConsumeLot, or a failure between them leaves a student "+
			"told their coins expired while the burn rolled back",
			inside.consumed, inside.granted)
	}
}

// A lot with nothing left in it is skipped by the sweep and produces no notice.
// Burning zero is a journal the posting CHECK rejects, so the sweep skips it — and a
// notice for a burn that did not happen is a false statement about a balance.
func TestAFullyConsumedLotProducesNoExpiryNotice(t *testing.T) {
	env := newExpiryEnv(t)
	notifier := newExpiryReminderNotifier()
	sweeper := env.sweeper.WithNotifier(notifier)

	grant, lotID := env.grantExpiring(t, 4242, -time.Hour)
	env.setLotConsumed(t, lotID, grant.Amount)

	report, err := sweeper.Sweep(context.Background(), ExpiryBatchDefault)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Expired != 0 {
		t.Errorf("burned %d lots from a fully-consumed one", report.Expired)
	}
	if n := len(notifier.forEvent(notification.EventCoinsExpired)); n != 0 {
		t.Errorf("%d expiry notices sent for a burn that never happened", n)
	}
}

// A nil notifier must be a no-op, not a panic. main.go wires one, and a missing
// constructor call should cost an operator their expiry notices, not a crashed job.
func TestTheReminderJobToleratesNoNotifier(t *testing.T) {
	env := newExpiryEnv(t)
	sweeper := env.sweeper.WithNotifier(nil)
	env.grantExpiring(t, 4242, 30*24*time.Hour)

	report, err := sweeper.RemindExpiring(context.Background())
	if err != nil {
		t.Fatalf("remind with no notifier: %v", err)
	}
	if report.Reminded == 0 {
		t.Error("the report claims nothing was reminded; the job should still count the " +
			"lots it found even when it could not announce them")
	}
}

// newReminderSweeper builds a sweeper over the expiry fixture's ledger.
func newReminderSweeper(env *expiryEnv, notifier *expiryReminderNotifier) *ExpirySweeper {
	return env.sweeper.WithNotifier(notifier)
}
