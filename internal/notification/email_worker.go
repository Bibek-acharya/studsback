// internal/notification/email_worker.go
package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"studsphere/backend/internal/emailqueue"
)

// Package-level handler deps, set once at startup (and by tests).
var (
	svc  *Service
	repo *Repository
	db   *gorm.DB
)

// InitWorker sets the package-level dependencies for the Asynq task handlers.
// Call from main.go before emailqueue.StartWorker().
func InitWorker(s *Service, r *Repository, database *gorm.DB) {
	svc = s
	repo = r
	db = database
}

// HandleProcessTask processes a notification:process task: loads the outbox
// row, re-checks prefs, and for each pending email delivery claims → renders →
// enqueues the email:deliver wrapper → marks handed_off.
func HandleProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload struct {
		OutboxID uint `json:"outbox_id"`
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal process payload: %w", err)
	}

	var outbox NotificationOutbox
	if err := db.Where("id = ?", payload.OutboxID).First(&outbox).Error; err != nil {
		return fmt.Errorf("load outbox: %w", err) // asynq retries
	}

	var req NotifyRequest
	if outbox.Kind == "fanout" {
		var bp struct {
			BroadcastID uint `json:"broadcast_id"`
		}
		if err := json.Unmarshal(outbox.Payload, &bp); err != nil {
			return fmt.Errorf("unmarshal broadcast payload: %w", err)
		}
		var campaign NotificationBroadcast
		if err := db.First(&campaign, bp.BroadcastID).Error; err != nil {
			return fmt.Errorf("load broadcast: %w", err)
		}
		req = fanoutNotifyRequest(outbox.ID, &campaign)
	} else if err := json.Unmarshal(outbox.Payload, &req); err != nil {
		return fmt.Errorf("unmarshal notify request: %w", err)
	}

	deliveries, err := repo.PendingDeliveriesForOutbox([]string{req.CorrelationID})
	if err != nil {
		return fmt.Errorf("load deliveries: %w", err)
	}

	def := Registry[req.EventKey]
	for _, d := range deliveries {
		// Re-check prefs at dispatch time (doc 03 §3 fanout note).
		_, emailOn, err := svc.ResolveChannels(Ref{Type: d.AccountType, ID: d.AccountID}, req.EventKey)
		if err != nil {
			return fmt.Errorf("resolve channels: %w", err)
		}
		if !emailOn {
			// Pref changed since emission → skip.
			_ = repo.CompleteDelivery(d.ID, "skipped", "")
			continue
		}

		// Claim delivery (CAS pending → dispatching).
		claimed, err := repo.ClaimDelivery(d.ID, "", 2*time.Minute)
		if err != nil {
			return fmt.Errorf("claim delivery: %w", err)
		}
		if !claimed {
			continue // already dispatched or skipped
		}

		subject, htmlBody, err := renderEmail(def, req)
		if err != nil {
			_ = repo.CompleteDelivery(d.ID, "failed", err.Error())
			continue
		}

		// Enqueue email:deliver wrapper (deterministic ID dedups retries).
		deliverTaskID := fmt.Sprintf("%s:%s", TaskTypeEmailDeliver, d.DeliveryKey)
		deliverPayload, _ := json.Marshal(map[string]any{
			"delivery_key": d.DeliveryKey,
			"subject":      subject,
			"html":         htmlBody,
			"account_type": d.AccountType,
			"account_id":   d.AccountID,
		})
		deliverTask := asynq.NewTask(TaskTypeEmailDeliver, deliverPayload)
		if _, err := EnqueueFunc(deliverTask, asynq.TaskID(deliverTaskID), asynq.MaxRetry(25)); err != nil {
			// Revert to pending and propagate: asynq retries the process task
			// (the outbox row is already done); the retry re-finds this row
			// while still pending — handed_off rows are status-filtered out.
			_ = repo.CompleteDelivery(d.ID, "pending", err.Error())
			return fmt.Errorf("enqueue email:deliver %s: %w", d.DeliveryKey, err)
		}

		_ = repo.CompleteDelivery(d.ID, "handed_off", "")
	}

	return nil
}

// HandleEmailDeliverTask processes an email:deliver task: resolves the
// recipient address and hands the rendered email to emailqueue.
func HandleEmailDeliverTask(ctx context.Context, task *asynq.Task) error {
	var p struct {
		DeliveryKey string `json:"delivery_key"`
		Subject     string `json:"subject"`
		HTML        string `json:"html"`
		AccountType string `json:"account_type"`
		AccountID   uint   `json:"account_id"`
	}
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}

	to, err := repo.EmailForAccount(p.AccountType, p.AccountID)
	if err != nil {
		return fmt.Errorf("resolve email: %w", err)
	}
	if to == "" {
		return fmt.Errorf("resolve email: no address for %s/%d", p.AccountType, p.AccountID)
	}

	return emailqueue.EnqueueGenericEmail(to, p.Subject, p.HTML)
}
