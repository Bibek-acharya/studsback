// internal/notification/email_test.go
package notification

import (
	"context"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
)

// TestE2ENotifyToEmailHandOff drives the full pipeline with a fake enqueuer:
// Notify → pending delivery + outbox → poller dispatch → process task →
// handed_off → email:deliver task. The final deliver handler errors without
// a real queue (asynq/Redis) — acceptable; the pipeline up to hand-off is
// what this test verifies.
func TestE2ENotifyToEmailHandOff(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	repo := NewRepository(db)
	InitWorker(svc, repo, db)
	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue; InitWorker(nil, nil, nil) })

	// Track all enqueued tasks.
	var allEnqueued []*asynq.Task
	svc.SetEnqueuer(fakeEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		allEnqueued = append(allEnqueued, task)
		return &asynq.TaskInfo{ID: task.Type()}, nil
	}))

	// Seed user with email.
	seedUser42(t, db)

	// 1. Notify.
	err := svc.Notify(context.Background(), NotifyRequest{
		EventKey:   EventApplicationStatusChanged,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"program": "CS", "status": "shortlisted"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Verify: inbox row + pending delivery + outbox row.
	var inboxCount int64
	db.Model(&AccountNotification{}).Where("account_type = ? AND account_id = ?", "user", 42).Count(&inboxCount)
	if inboxCount != 1 {
		t.Fatalf("expected 1 inbox row, got %d", inboxCount)
	}
	var delivery NotificationDelivery
	if err := db.Where("account_type = ? AND account_id = ? AND channel = ?", "user", 42, "email").First(&delivery).Error; err != nil {
		t.Fatalf("expected delivery row: %v", err)
	}
	if delivery.Status != "pending" {
		t.Fatalf("expected pending delivery, got %s", delivery.Status)
	}
	var outbox NotificationOutbox
	db.Last(&outbox)

	// 3. Simulate poller tick: dispatch outbox → enqueue process task.
	if err := dispatchOutboxRow(svc, outbox); err != nil {
		t.Fatal(err)
	}
	// Verify: notification:process task enqueued.
	if len(allEnqueued) != 1 || allEnqueued[0].Type() != TaskTypeProcess {
		t.Fatalf("expected 1 process task, got %d tasks", len(allEnqueued))
	}

	// 4. Simulate worker: process task.
	processTask := allEnqueued[0]
	if err := HandleProcessTask(context.Background(), processTask); err != nil {
		t.Fatal(err)
	}
	// Verify: delivery handed_off + email:deliver task enqueued.
	db.First(&delivery, delivery.ID)
	if delivery.Status != "handed_off" {
		t.Errorf("expected handed_off, got %s", delivery.Status)
	}
	if len(allEnqueued) != 2 || allEnqueued[1].Type() != TaskTypeEmailDeliver {
		t.Fatalf("expected email:deliver task, got %d tasks", len(allEnqueued))
	}

	// 5. Simulate worker: email:deliver task. Without Redis the queue is nil
	// and the handler returns "asynq not initialized" — acceptable here; any
	// other error is a bug.
	deliverTask := allEnqueued[1]
	if err := HandleEmailDeliverTask(context.Background(), deliverTask); err != nil {
		if !strings.Contains(err.Error(), "asynq not initialized") {
			t.Errorf("unexpected deliver error: %v", err)
		}
	}
}

// TestPollerTickDispatchesOutbox drives one synchronous tick (the poller
// unit seam) over a seeded dispatch row: the tick must enqueue its
// notification:process task and mark the row done.
func TestPollerTickDispatchesOutbox(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	InitWorker(svc, NewRepository(db), db)
	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue; InitWorker(nil, nil, nil) })

	var enqueued []*asynq.Task
	svc.SetEnqueuer(fakeEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		enqueued = append(enqueued, task)
		return &asynq.TaskInfo{ID: task.Type()}, nil
	}))

	// Seed outbox row.
	payload := mustJSON(NotifyRequest{
		EventKey:   EventApplicationStatusChanged,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"program": "CS", "status": "shortlisted"},
	})
	db.Create(&NotificationOutbox{Kind: "dispatch", Payload: payload})

	// Run one tick.
	tick(svc)

	if len(enqueued) != 1 {
		t.Fatalf("expected 1 task from tick, got %d", len(enqueued))
	}
	if enqueued[0].Type() != TaskTypeProcess {
		t.Errorf("expected %s, got %s", TaskTypeProcess, enqueued[0].Type())
	}
	// Verify outbox row completed.
	var outbox NotificationOutbox
	db.Last(&outbox)
	if !outbox.Done {
		t.Error("expected outbox row done after tick")
	}
}
