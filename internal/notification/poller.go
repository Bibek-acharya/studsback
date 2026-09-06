// internal/notification/poller.go
package notification

import (
	"time"

	"gorm.io/gorm"
)

// StartPoller runs the lease-claim loop until stop() is called.
func StartPoller(db *gorm.DB, interval time.Duration) (stop func()) {
	repo := NewRepository(db)
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				tick(repo)
			case <-done:
				return
			}
		}
	}()
	return func() { ticker.Stop(); done <- struct{}{} }
}

func tick(repo *Repository) {
	rows, token, err := repo.ClaimDueOutbox(50)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, row := range rows {
		if err := dispatchOutboxRow(repo, row); err != nil {
			// Retry with backoff: release the claim, push available_at out.
			repo.db.Exec(`UPDATE notification_outbox
				SET last_error = ?, available_at = now() + make_interval(secs => ?),
				    claimed_at = NULL, lease_expires_at = NULL
				WHERE id = ? AND claim_token = ?`,
				err.Error(), backoffSeconds(row.Attempts), row.ID, token)
			continue
		}
		if err := repo.CompleteOutbox(row.ID, token); err != nil {
			_ = err // claim lost — another worker finished it (doc 03 §3)
		}
	}
}

// dispatchOutboxRow is the P1 dispatch: in-app rows already exist at emission,
// so a claimed row is complete. P2 replaces this with email fan-out +
// delivery bookkeeping (doc 12 §2) — the only place that changes.
func dispatchOutboxRow(repo *Repository, row NotificationOutbox) error {
	switch row.Kind {
	case "dispatch", "fanout", "anonymous_email":
		return nil
	default:
		return nil
	}
}

func backoffSeconds(attempts int) float64 {
	s := 300.0 // 5m base (doc 03 §3 backoff)
	for i := 1; i < attempts && s < 3600; i++ {
		s *= 2
	}
	if s > 3600 {
		s = 3600
	}
	return s
}
