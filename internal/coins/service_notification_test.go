//go:build coinsintegration

// internal/coins/service_notification_test.go
//
// The debit notification, against a real PostgreSQL instance, in the
// service_notification_test.go convention of every other module: one file per
// module for "does this business event announce itself, and does the announcement
// survive the thing it is announcing".
//
// Two claims, and neither is observable without a transaction:
//
//   - an unlock emits EXACTLY ONE coins.debited to the student who paid. The
//     internal/notification convention tests emission with a captureNotifier,
//     which cannot answer this one: what matters here is that the row is written
//     over the purchase's own transaction, which only a real registry, a real
//     service and a real commit can show.
//   - a notification that cannot be written takes the purchase down with it, and
//     leaves nothing behind. Not just no journal: no lot consumed, no entitlement,
//     and no inbox row — because the row is written inside the transaction that
//     is being rolled back. That last part is why the failing notifier below
//     calls the REAL one first and fails afterwards: injecting a failure before
//     the write would prove the rollback but not that the row goes with it.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// Reuses openUnlockAPISchema, so this file owns no schema of its own: it creates
// and drops exactly one, coins_unlock_api_test, and writes nothing to the public
// schema. Every test skips when COINS_TEST_DSN is unset.
package coins

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/notification"
)

// apiNotificationsFor is the inbox rows this student has for one event, newest
// last. Scoped by account AND event so a test cannot pass on a row another
// emission left behind.
func apiNotificationsFor(t *testing.T, db *gorm.DB, userID uint, eventKey string) []notificationRow {
	t.Helper()
	var rows []notificationRow
	if err := db.Raw(
		`SELECT id, account_type, account_id, event_key, category, priority, title, body, link, occurrence_key
		   FROM account_notifications
		  WHERE account_type = 'user' AND account_id = ? AND event_key = ?
		  ORDER BY id`, userID, eventKey,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read notifications for user %d: %v", userID, err)
	}
	return rows
}

// notificationRow is the subset of account_notifications this file asserts on,
// read with Raw so the test does not depend on the notification module's model.
type notificationRow struct {
	ID            uint
	AccountType   string
	AccountID     uint
	EventKey      string
	Category      string
	Priority      string
	Title         string
	Body          string
	Link          string
	OccurrenceKey string
}

// apiOpenLotValue is what the student's lots are still worth: granted minus
// consumed. It is the assertion that a rolled-back purchase did not BURN coins —
// the cached balance can agree with itself after a rollback while the lots
// underneath it have been consumed, and the balance alone would not notice.
func apiOpenLotValue(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	var total int64
	if err := db.Raw(
		`SELECT COALESCE(SUM(l.granted - l.consumed), 0)
		   FROM coin_lot l
		   JOIN coin_account a ON a.id = l.account_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'`, userID,
	).Scan(&total).Error; err != nil {
		t.Fatalf("read lot value for user %d: %v", userID, err)
	}
	return total
}

// ── the emission ─────────────────────────────────────────────────────────────

// A coin purchase announces itself exactly once, to the student who paid, and
// only when coins actually moved.
func TestUnlockEmitsExactlyOneCoinsDebitedNotification(t *testing.T) {
	const userID = 920
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "notify-happy-grant")
	exhaustDocuments(t, w, userID, 9201)
	before := apiUserAvailable(t, db, userID)

	if rows := apiNotificationsFor(t, db, userID, notification.EventCoinsDebited); len(rows) != 0 {
		t.Fatalf("the fixture already has %d notifications", len(rows))
	}

	res := postUnlock(t, w, userID, "notify-happy-1", studyResourceBody)
	if res.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", res.Status, res.Raw)
	}
	if res.Body.CoinsPaid != 40 {
		t.Fatalf("coins_paid = %d, want the configured 40", res.Body.CoinsPaid)
	}

	rows := apiNotificationsFor(t, db, userID, notification.EventCoinsDebited)
	if len(rows) != 1 {
		t.Fatalf("coins.debited rows = %d, want exactly 1", len(rows))
	}
	row := rows[0]
	if row.AccountType != "user" || row.AccountID != userID {
		t.Errorf("the notification went to %s/%d, want the unlocking student user/%d",
			row.AccountType, row.AccountID, userID)
	}
	// Rendered from the registry's templates, not stored raw.
	if row.Title != "StudsTokens used" {
		t.Errorf("title = %q, want %q", row.Title, "StudsTokens used")
	}
	if row.Body == "" {
		t.Fatal("the body is empty; the template did not render")
	}
	for _, banned := range []string{"free", "prize", "award", "win", "raffle", "draw", "npr"} {
		if strings.Contains(strings.ToLower(row.Title+" "+row.Body), banned) {
			t.Errorf("the rendered copy contains the banned %q: %q", banned, row.Body)
		}
	}
	// One emission per purchase, keyed on the journal that settled it.
	if row.OccurrenceKey == "" {
		t.Error("occurrence_key is empty; a retried emit has no identity to suppress")
	}

	// The already-owned retry is the interesting negative: it is a 200 and it is
	// NOT a second debit, so it must not be a second announcement either.
	second := postUnlock(t, w, userID, "notify-happy-2", studyResourceBody)
	if second.Status != http.StatusOK || !second.Body.AlreadyUnlocked {
		t.Fatalf("the retry = %d %+v (%s), want an already-owned 200", second.Status, second.Body, second.Raw)
	}
	if rows := apiNotificationsFor(t, db, userID, notification.EventCoinsDebited); len(rows) != 1 {
		t.Errorf("coins.debited rows after a retry = %d, want 1 — a purchase that moved no coins announced itself", len(rows))
	}

	// The dispatch row the email path polls for, once.
	if n := apiCount(t, db,
		`SELECT count(*) FROM notification_outbox WHERE kind = 'dispatch' AND done = false`); n != 1 {
		t.Errorf("pending dispatch rows = %d, want 1", n)
	}
	// The email row, because 09 lists this event as one that emails. It is
	// PENDING: EmailDefault is true and the student has no preference row, so
	// the channel resolves on. The row existing is the claim; whether it is
	// delivered is the worker's business.
	if n := apiCount(t, db, `SELECT count(*) FROM notification_deliveries WHERE status = 'pending'`); n != 1 {
		t.Errorf("pending email delivery rows = %d, want 1", n)
	}

	// The balance moved by exactly the announced amount.
	if after := apiUserAvailable(t, db, userID); after != before-40 {
		t.Errorf("wallet = %d, want %d (one 40-coin debit)", after, before-40)
	}
}

// An allowance unlock moves no coins, so it announces nothing. A "you spent 40
// StudsTokens" for a purchase that cost nothing is the exact email a student
// would escalate on.
func TestAnAllowanceUnlockAnnouncesNothing(t *testing.T) {
	const userID = 921
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	now := time.Now().UTC()
	if _, err := w.service.EnsureAllowance(context.Background(), userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}

	res := postUnlock(t, w, userID, "notify-allowance-1", studyResourceBody)
	if res.Status != http.StatusOK || !res.Body.UsedAllowance {
		t.Fatalf("the unlock = %d %+v (%s), want an allowance unlock", res.Status, res.Body, res.Raw)
	}
	if rows := apiNotificationsFor(t, db, userID, notification.EventCoinsDebited); len(rows) != 0 {
		t.Errorf("an allowance unlock emitted %d coins.debited rows, want 0: %+v", len(rows), rows)
	}
}

// A 402 announces nothing either: the refusal is answered in the response body,
// and an inbox row saying a purchase failed would read as a charge.
func TestARefusedUnlockAnnouncesNothing(t *testing.T) {
	const userID = 922
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonProfileComplete, "notify-refused-grant")
	exhaustDocuments(t, w, userID, 9221)

	res := postUnlock(t, w, userID, "notify-refused-1", studyResourceBody)
	if res.Status != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (%s)", res.Status, res.Raw)
	}
	if rows := apiNotificationsFor(t, db, userID, notification.EventCoinsDebited); len(rows) != 0 {
		t.Errorf("a refused purchase emitted %d coins.debited rows, want 0: %+v", len(rows), rows)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM notification_outbox WHERE kind = 'dispatch'`); n != 0 {
		t.Errorf("dispatch rows after a 402 = %d, want 0", n)
	}
}

// ── the rollback ─────────────────────────────────────────────────────────────

// failAfterRealNotifier calls the REAL notifier and then fails.
//
// Failing BEFORE the write would prove the transaction rolls back. Failing AFTER
// it proves the harder half: the inbox row that was really written, over the real
// registry and the real service, is inside the same transaction, so it goes when
// the purchase does. §5 asks for exactly this — "a rolled-back change never
// produces a notification".
type failAfterRealNotifier struct {
	inner unlockNotifier
	calls int
}

func (n *failAfterRealNotifier) NotifyUnlockTx(ctx context.Context, tx *gorm.DB, userID uint, event UnlockEvent) error {
	n.calls++
	if err := n.inner.NotifyUnlockTx(ctx, tx, userID, event); err != nil {
		return err
	}
	return errors.New("inbox dispatch refused after the row was written")
}

// A notification that cannot be written aborts the whole purchase: the spend, the
// entitlement, the lot consumption and the notification itself all roll back
// together, and the client's key is still usable afterwards.
func TestANotificationFailureRollsBackTheSpendTheUnlockAndTheLot(t *testing.T) {
	const userID = 923
	db := openUnlockAPISchema(t)
	w := newWalletAPI(t, db, nil)
	grantCoins(t, w, userID, ReasonResourceApproved, "notify-rollback-grant")
	exhaustDocuments(t, w, userID, 9231)
	before := apiUserAvailable(t, db, userID)
	beforeLots := apiOpenLotValue(t, db, userID)
	if beforeLots == 0 {
		t.Fatal("the fixture granted no lot; there is nothing for a rollback to protect")
	}

	// Wrap whatever NewUnlockAPI wired — the real registryNotifier — so the
	// failure is injected, not simulated.
	real := w.api.notifier
	if real == nil {
		t.Fatal("NewUnlockAPI wired no notifier; the emission seam is dark")
	}
	notifier := &failAfterRealNotifier{inner: real}
	w.api = w.api.WithNotifier(notifier)

	res := postUnlock(t, w, userID, "notify-rollback-1", studyResourceBody)
	if res.Status == http.StatusOK {
		t.Fatalf("a failed notification still returned 200 (%s)", res.Raw)
	}
	if notifier.calls != 1 {
		t.Errorf("the notifier was called %d times, want 1", notifier.calls)
	}

	// Nothing moved.
	if n := apiSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0 after a rolled-back purchase", n)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_id = 812`, userID); n != 0 {
		t.Errorf("unlock rows = %d, want 0 after a rolled-back purchase", n)
	}
	if got := apiUserAvailable(t, db, userID); got != before {
		t.Errorf("wallet = %d, want the unchanged %d", got, before)
	}
	// The lots: the cached balance can agree with itself after a rollback while
	// the lots under it have been burned, so the lots are asserted separately.
	if got := apiOpenLotValue(t, db, userID); got != beforeLots {
		t.Errorf("open lot value = %d, want the unchanged %d — the lot consumption outlived the rollback", got, beforeLots)
	}
	// And the notification that WAS written is gone with the transaction.
	if rows := apiNotificationsFor(t, db, userID, notification.EventCoinsDebited); len(rows) != 0 {
		t.Errorf("a rolled-back purchase left %d inbox rows behind: %+v", len(rows), rows)
	}
	if n := apiCount(t, db, `SELECT count(*) FROM notification_outbox WHERE kind = 'dispatch'`); n != 0 {
		t.Errorf("dispatch rows = %d, want 0 after a rolled-back purchase", n)
	}

	// The client's key is still usable, because the first attempt wrote nothing at
	// all. This is the property that makes a retry safe rather than a support
	// ticket.
	w.api = w.api.WithNotifier(real)
	retry := postUnlock(t, w, userID, "notify-rollback-1", studyResourceBody)
	if retry.Status != http.StatusOK {
		t.Fatalf("the retry = %d, want 200 (%s)", retry.Status, retry.Raw)
	}
	if retry.Body.CoinsPaid != 40 {
		t.Errorf("the retry charged %d, want 40", retry.Body.CoinsPaid)
	}
	rows := apiNotificationsFor(t, db, userID, notification.EventCoinsDebited)
	if len(rows) != 1 {
		t.Errorf("coins.debited rows after the retry = %d, want 1", len(rows))
	}
	if got := apiOpenLotValue(t, db, userID); got != beforeLots-40 {
		t.Errorf("open lot value = %d, want %d after one 40-coin spend", got, beforeLots-40)
	}
}
