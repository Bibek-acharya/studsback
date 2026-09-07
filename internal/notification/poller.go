// internal/notification/poller.go
package notification

import (
	"time"

	"gorm.io/gorm"
)

// StartPoller runs the lease-claim loop until stop() is called.
func StartPoller(db *gorm.DB, interval time.Duration) (stop func()) {
	return startPoller(NewService(db), interval)
}

func startPoller(svc *Service, interval time.Duration) (stop func()) {
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				tick(svc)
			case <-done:
				return
			}
		}
	}()
	return func() { ticker.Stop(); done <- struct{}{} }
}

func tick(svc *Service) {
	rows, token, err := svc.repo.ClaimDueOutbox(50)
	if err != nil || len(rows) == 0 {
		return
	}
	for _, row := range rows {
		if err := dispatchOutboxRow(svc, row); err != nil {
			// Retry with backoff: release the claim, push available_at out.
			svc.repo.db.Exec(`UPDATE notification_outbox
				SET last_error = ?, available_at = now() + make_interval(secs => ?),
				    claimed_at = NULL, lease_expires_at = NULL
				WHERE id = ? AND claim_token = ?`,
				err.Error(), backoffSeconds(row.Attempts), row.ID, token)
			continue
		}
		if err := svc.repo.CompleteOutbox(row.ID, token); err != nil {
			_ = err // claim lost — another worker finished it (doc 03 §3)
		}
	}
}

// dispatchOutboxRow processes one claimed row. Fanout rows expand their
// campaign's audience through the service (occurrence keys make replays
// harmless); dispatch/anonymous_email rows are complete at emission and P2
// replaces their no-op with email delivery (doc 12 §2).
func dispatchOutboxRow(svc *Service, row NotificationOutbox) error {
	if row.Kind == "fanout" {
		return svc.expandFanoutRow(row)
	}
	return nil
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
