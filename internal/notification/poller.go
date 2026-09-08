// internal/notification/poller.go
package notification

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

const (
	TaskTypeProcess      = "notification:process"
	TaskTypeEmailDeliver = "email:deliver"
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
	// Recovery sweep: re-open expired delivery reservations (crashed workers).
	_, _ = svc.repo.ReopenExpiredDeliveries()
	rows, token, err := svc.repo.ClaimDueOutbox(50)
	if err == nil {
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
	// Stranded-pending sweep: re-kick dispatch work for deliveries whose
	// process task exhausted its retries and never handed off.
	rekickStuckPending(svc)
}

// rekickStuckPending re-enqueues the original notification:process task for
// email deliveries stuck pending beyond the window — their process task
// burned all retries, so nothing else will hand them off. An enqueue that
// hits a live TaskID collision (task still queued) fails: skipped, harmless.
func rekickStuckPending(svc *Service) {
	deliveries, err := svc.repo.StuckPendingDeliveries(10)
	if err != nil {
		return
	}
	for _, d := range deliveries {
		if d.NotificationID == nil {
			continue
		}
		var n AccountNotification
		if err := svc.db.First(&n, *d.NotificationID).Error; err != nil || n.OccurrenceKey == "" {
			continue // no occurrence key → no outbox row to re-kick
		}
		var outbox NotificationOutbox
		if err := svc.db.Where("kind = 'dispatch' AND occurrence_key = ?", n.OccurrenceKey).
			Order("id DESC").First(&outbox).Error; err != nil {
			continue
		}
		payload, err := json.Marshal(map[string]any{"outbox_id": outbox.ID})
		if err != nil {
			continue
		}
		// Collision-free ID: asynq archives keep the task hash key forever,
		// so the original (retry-exhausted) task still owns
		// notification:process:<outbox_id> — a deterministic re-kick ID
		// would ErrTaskIDConflict forever. Duplicates are harmless (the
		// process handler filters by delivery status).
		_, _ = EnqueueFunc(
			asynq.NewTask(TaskTypeProcess, payload),
			asynq.TaskID(fmt.Sprintf("%s:%d:rekick:%d", TaskTypeProcess, outbox.ID, time.Now().UnixNano())),
			asynq.MaxRetry(25), asynq.Timeout(10*time.Minute))
	}
}

// dispatchOutboxRow processes one claimed row. Fanout rows expand their
// campaign's audience through the service (occurrence keys make replays
// harmless); dispatch rows enqueue a notification:process task; anonymous_email
// is a P3 no-op seam.
func dispatchOutboxRow(svc *Service, row NotificationOutbox) error {
	if row.Kind == "fanout" {
		return svc.expandFanoutRow(row)
	}
	if row.Kind == "anonymous_email" {
		return nil // P3 seam — transactional email dispatch not routed through outbox yet
	}
	taskID := fmt.Sprintf("%s:%d", TaskTypeProcess, row.ID)
	payload, err := json.Marshal(map[string]any{"outbox_id": row.ID})
	if err != nil {
		return fmt.Errorf("marshal process payload: %w", err)
	}
	task := asynq.NewTask(TaskTypeProcess, payload)
	_, err = EnqueueFunc(task, asynq.TaskID(taskID), asynq.MaxRetry(25), asynq.Timeout(10*time.Minute))
	return err
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
