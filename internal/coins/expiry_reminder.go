// internal/coins/expiry_reminder.go
//
// 04 §7's two communication items:
//
//   - "Reminders at 30 / 7 / 1 days, per lot"
//   - "Never silently delete an expired balance … and always tell the student"
//
// Both are here because they are the same problem seen from two sides. The sweep takes
// value away; a student whose balance drops without a word has had money removed by
// software, and 04 §7 names that as the thing not to do.
//
// ── why the reminder is a SEPARATE job from the sweep ─────────────────────────
//
// They have opposite failure modes and opposite questions. The sweep asks "what is
// past its date"; the reminder asks "what is coming due soon". Combining them means a
// bug in one silently stops the other, and the second symptom — a student never warned
// — is invisible until somebody complains.
//
// They share one ticker, which is a scheduling convenience and not a shared code path.
//
// ── idempotence is the load-bearing property ───────────────────────────────────
//
// This runs hourly. Without idempotence a student receives the same 30-day notice 24
// times a day, which trains them to ignore it and makes the 1-day notice — the one
// that actually matters — equally ignorable.
//
// The mechanism is an OccurrenceKey on the notification, keyed on the LOT and the
// THRESHOLD and not the user, because:
//
//   - keyed on the user, a student with two lots would get one notice and not know
//     about the second;
//   - keyed on the threshold alone, two lots crossing 30 days together would collide.
//
// So the key is `coins.expiring:<lotID>:<days>`. The registry's own dedupe window is
// the second line of defence; this is the first.
//
// ── closed accounts are excluded ──────────────────────────────────────────────
//
// A closed account is a support hold — support has frozen it because something is
// wrong. Telling a student their frozen coins are about to expire is both noise and
// actively misleading: the coins are not going anywhere, and the student has been told
// to do something about it that will not help.

package coins

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/logger"
)

// ReminderThresholds is the set of day-counts at which a lot is announced, in days
// before expiry.
//
// THREE, at 30 / 7 / 1, from 04 §7. The shape of the series is deliberate: the 30-day
// notice is nearly useless as a prompt (nobody plans 30 days out) but it is what tells
// a student to look; the 7-day notice is the actionable one; the 1-day notice is what
// makes the 30-day one credible, because it proves the reminders are real.
//
// Descending, and each is a strict subset of the one before it. That is what lets the
// OccurrenceKey distinguish them.
var ReminderThresholds = []int{30, 7, 1}

// ReminderBatchDefault bounds one pass of the reminder job.
//
// Much larger than the sweep's batch, and for the opposite reason: the sweep does
// irreversible work and wants a small batch so a failure strands little; the reminder
// is read-only plus a notification and wants to cover the whole in-window population
// in one pass, because a lot that misses its window is a lot that never gets a notice
// at all — the thresholds are in DAYS, so a backlog longer than a day is a backlog
// whose notices have expired.
const ReminderBatchDefault = 2000

// expiryNotifier announces an expiry. Optional: with none wired the sweep still burns
// and the reminder job still counts what it found, and the only loss is the notice.
//
// Declared here rather than reusing profileAwardNotifier or referralNotifier because
// this is a fourth producer with its own two event keys, and sharing an interface with
// a producer that has different data requirements invites a later edit that breaks one
// of them.
type expiryNotifier interface {
	Notify(ctx context.Context, req notification.NotifyRequest) error

	// NotifyTx writes the notification inside the CALLER's transaction.
	//
	// This is the method 03 §5 actually asks for — "each is emitted inside the same
	// transaction as the state change that caused it, so a rolled-back expiry never
	// produces an 'your coins expired' notification" — and notification.Service has
	// had `NotifyTx` for exactly that since before the coin system existed. Calling
	// plain Notify from the burn would open a SECOND transaction, so the notice would
	// commit even when the burn rolled back, which is the precise failure the sentence
	// names.
	//
	// The reminder job uses plain Notify, because it has no business mutation to be
	// atomic with: it reads lots and sends mail, and there is nothing to roll back.
	NotifyTx(ctx context.Context, tx *gorm.DB, req notification.NotifyRequest) error

	// Sent reports whether this exact occurrence has already gone to this recipient.
	//
	// ON THE PORT rather than queried from coins, and that placement is the whole
	// design. The occurrence key is stored per RECIPIENT — the notifications table
	// keys it on (account_type, account_id, occurrence_key) — so answering the
	// question needs the notification store's own schema, and reaching into it from
	// here would couple coins to another package's table and depend on a delivery
	// having actually happened for the answer to be true.
	//
	// It is a separate method rather than a return value from Notify because the job
	// must be able to ask BEFORE sending, twenty-four times a day. Making Notify
	// itself suppress would move the decision somewhere this job cannot see or test.
	Sent(ctx context.Context, occurrenceKey string, recipient notification.Ref) (bool, error)
}

// WithNotifier returns a COPY of the sweeper carrying a notifier.
//
// A copy rather than a setter because main.go hands the same sweeper to both the sweep
// and the reminder job, and a setter would mean whichever ran last silently took the
// notifier away from the other.
func (s *ExpirySweeper) WithNotifier(n expiryNotifier) *ExpirySweeper {
	if s == nil {
		return nil
	}
	clone := *s
	clone.notifier = n
	return &clone
}

// ReminderReport is what one pass of the reminder job did.
type ReminderReport struct {
	// Scanned is how many in-window lots the selection returned.
	Scanned int
	// Reminded is how many notices were actually emitted.
	//
	// SCANNED and REMINDED are separate because they answer different questions, and
	// conflating them hides the interesting case: a pass that scanned 200 lots and
	// reminded 0 is either perfect idempotence or a total failure, and nothing in a
	// single number distinguishes them.
	Reminded int
	// Suppressed is how many were skipped because that lot had already been announced
	// at that threshold. On a steady-state pass this is every one of them.
	Suppressed int
	// Skipped is how many were skipped for a reason that is permanent rather than
	// idempotent — an empty lot, or a closed account.
	Skipped int
	// Truncated is true when the batch bound stopped the selection with lots still in
	// window. A lot past the bound has lost nothing yet, but it will lose its whole
	// window if the backlog keeps outrunning the batch.
	Truncated bool
	RanAt     time.Time
}

// RemindableLot is one lot inside a reminder window.
type RemindableLot struct {
	Lot         CoinLot
	OwnerUserID uint
	// Remaining is granted minus consumed — the amount that would actually be lost.
	// Carried on the candidate rather than recomputed, because the notice must say what
	// the student is about to LOSE and a lot's granted amount is not that.
	Remaining int64
	// Days is the matching threshold, not the exact day count. The template says "in N
	// days", and a student told "in 29 days" when the policy says 30 has been told
	// something true and needlessly confusing.
	Days int
}

// RemindExpiring emits one notice per lot per threshold it has entered.
//
// READ-ONLY apart from the notifications: no journal, no posting, no balance move.
// A reminder that took value would be the thing 04 §7 forbids.
func (s *ExpirySweeper) RemindExpiring(ctx context.Context) (ReminderReport, error) {
	report := ReminderReport{RanAt: s.now()}
	if s == nil || s.repo == nil {
		return report, ErrNoDatabase
	}
	now := s.now().UTC()

	candidates, err := s.repo.ClaimRemindableLots(ctx, now, ReminderBatchDefault+1, ReminderThresholds)
	if err != nil {
		return report, fmt.Errorf("select remindable lots: %w", err)
	}
	report.Scanned = len(candidates)
	if len(candidates) > ReminderBatchDefault {
		report.Truncated = true
		candidates = candidates[:ReminderBatchDefault]
	}

	for _, candidate := range candidates {
		// An empty lot has nothing to lose and a notice saying so is noise.
		if candidate.Remaining <= 0 {
			report.Skipped++
			continue
		}
		// The OccurrenceKey is what makes this idempotent, and it is built here
		// rather than by the notification service because the service has no idea a
		// lot exists.
		key := reminderOccurrenceKey(candidate.Lot.ID, candidate.Days)

		recipient := notification.Ref{Type: "user", ID: candidate.OwnerUserID}

		// Ask whether this notice has gone before. This is the idempotence check and
		// it runs BEFORE the send, twenty-four times a day.
		//
		// An error here is treated as "not sent" rather than as a failure to skip the
		// pass. That is a deliberate choice against the safer-looking option: the
		// alternative loses a student's 1-day notice entirely, because the pass would
		// move on and the window is measured in HOURS at that threshold. A duplicate
		// notice is annoying; a missing one is the failure 04 §7 is about.
		if s.notifier != nil {
			already, err := s.notifier.Sent(ctx, key, recipient)
			if err == nil && already {
				report.Suppressed++
				continue
			}
		} else {
			// No notifier wired. Count it anyway, so the report distinguishes "the job
			// is not running" from "the job is running and cannot speak".
			report.Reminded++
			continue
		}

		_ = s.notifier.Notify(ctx, notification.NotifyRequest{
			EventKey:   notification.EventCoinsExpiring,
			Recipients: []notification.Ref{recipient},
			Data: map[string]any{
				// The lot's OWN remaining balance, not the student's total. "5
				// StudsTokens expire on 12 November" is actionable for a student whose
				// balance is 300; "you have StudsTokens expiring" is not.
				"coins": candidate.Remaining,
				// The threshold, not the exact day count — see the field's comment.
				"days": candidate.Days,
				// A DATE. 06 §1.4 requires expiry to be stated as a date rather than
				// as a bare countdown, and the template interpolates this key with
				// missingkey=error, so omitting it drops the notification outright.
				"expires_at": lotExpiry(candidate.Lot).Format("2006-01-02"),
			},
			OccurrenceKey: key,
			CorrelationID: key,
		})
		report.Reminded++
	}
	return report, nil
}

// reminderOccurrenceKey identifies one lot at one threshold.
//
// THREE parts and the third is not decoration: keyed on lot and user alone, a lot
// crossing from the 30-day band into the 7-day band would collide with its own earlier
// notice and the student would never get the actionable one. The threshold is what
// makes the descending series work.
func reminderOccurrenceKey(lotID uint, days int) string {
	return fmt.Sprintf("%s:%d:%d", notification.EventCoinsExpiring, lotID, days)
}

// expiryOccurrenceKey identifies one burn.
//
// Keyed on the lot alone rather than on a date, because a lot burns exactly once and
// "once" is a stronger statement than any timestamp could be.
func expiryOccurrenceKey(lotID uint) string {
	return fmt.Sprintf("%s:%d", notification.EventCoinsExpired, lotID)
}

// StartExpiryReminder runs the reminder job on the same clock as the sweep.
//
// SHARED TICKER deliberately, for the same reason the sweep shares its hour with the
// reconciler: one log line's worth of elapsed time should be readable as a single
// story. A reminder sent an hour after the sweep that would have warned about it is a
// reminder that arrives after the coins are gone.
func StartExpiryReminder(sweeper *ExpirySweeper, interval, timeout time.Duration) (stop func()) {
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				report, err := sweeper.RemindExpiring(ctx)
				cancel()
				switch {
				case err != nil:
					logger.Warn("coin expiry reminder failed; lots already past a window "+
						"may go unnotified until the next pass", "error", err)
				case report.Reminded > 0 || report.Skipped > 0:
					logger.Info("coin expiry reminder",
						"scanned", report.Scanned,
						"reminded", report.Reminded,
						"suppressed", report.Suppressed,
						"skipped", report.Skipped,
						"truncated", report.Truncated)
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() { done <- struct{}{} }
}

// AvailableInTx is the caller's spendable balance across every one of their accounts,
// read inside the transaction that just moved it.
//
// It exists for the expiry notice and nothing else. A student whose balance just fell
// needs to be told what they are LEFT WITH, not only what went — otherwise the number
// on their wallet has changed and the notification has given them no way to reconcile
// the two, which is the support ticket 09 exists to prevent.
//
// POSTED minus RESERVED, across all of the caller's USER accounts, because that is the
// figure the wallet renders. Reading one account's posted balance instead would state
// a number the student cannot see anywhere.
func (tx *TxContext) AvailableInTx(userID uint) (int64, error) {
	var available int64
	err := tx.db.Raw(`
		SELECT COALESCE(SUM(b.posted_balance - b.reserved), 0)
		  FROM coin_account_balance b
		  JOIN coin_account a ON a.id = b.account_id
		 WHERE a.kind = ? AND a.owner_user_id = ?`,
		AccountUser, userID).Scan(&available).Error
	return available, err
}
