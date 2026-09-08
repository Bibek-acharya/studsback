// internal/notification/poller_test.go
package notification

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/datatypes"
)

func TestPollerClaimsAndCompletesDueRows(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	// Fake enqueuer so dispatch row enqueue succeeds (no real Redis in tests).
	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue })
	svc.SetEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		return &asynq.TaskInfo{ID: "test"}, nil
	})
	if err := NewRepository(db).InsertOutbox(nil, NotificationOutbox{Kind: "dispatch", Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stop := StartPoller(db, 50*time.Millisecond)
	defer stop()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var done int64
		db.Table("notification_outbox").Where("done = true").Count(&done)
		if done >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("poller did not complete the row within 3s")
}

func TestLeaseExpiryReclaimsAbandonedRow(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	if err := repo.InsertOutbox(nil, NotificationOutbox{Kind: "dispatch", Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Simulate a crashed poller: claim, never complete, lease already expired.
	rows, token, err := repo.ClaimDueOutbox(10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("claim: %v %d", err, len(rows))
	}
	db.Exec(`UPDATE notification_outbox SET lease_expires_at = now() - interval '1 minute' WHERE id = ?`, rows[0].ID)
	if err := repo.CompleteOutbox(rows[0].ID, token); err == nil {
		t.Fatal("completing with an expired lease must fail (claim lost)")
	}
	again, _, err := repo.ClaimDueOutbox(10)
	if err != nil || len(again) != 1 {
		t.Fatalf("expired lease must be reclaimable: %v %d", err, len(again))
	}
}

// TestPollerExpandsFanoutRow drives one synchronous tick over a seeded
// fanout row (the production path CreateBroadcast relies on): the tick must
// expand the campaign's audience and only then mark the row done.
func TestPollerExpandsFanoutRow(t *testing.T) {
	db := testDB(t)
	// Account stubs come from testDB (same adaptation as
	// TestBroadcastCreatesCampaignAndFansOut).
	db.Exec(`INSERT INTO users (email, first_name, last_name, role, status, created_at, updated_at) VALUES
		('poller-fan@test.local','Poller','Fan','student','active',now(),now())`)
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE email = 'poller-fan@test.local'`) })

	campaign := NotificationBroadcast{
		Title: "Poller fanout", Body: "hello", Priority: "normal",
		Audience: datatypes.JSON(`["all"]`), Status: "sending", CreatedBy: 1,
	}
	if err := db.Create(&campaign).Error; err != nil {
		t.Fatalf("seed campaign: %v", err)
	}
	if err := NewRepository(db).InsertOutbox(nil, NotificationOutbox{
		Kind:    "fanout",
		Payload: datatypes.JSON(fmt.Sprintf(`{"broadcast_id":%d}`, campaign.ID)),
	}); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}

	tick(NewService(db)) // one synchronous tick — no background loop

	var n int64
	db.Table("account_notifications").
		Where("account_type='user' AND event_key='system.announcement' AND broadcast_id = ?", campaign.ID).
		Count(&n)
	if n < 1 {
		t.Fatalf("tick did not expand fanout: %d recipient rows", n)
	}
	var done NotificationBroadcast
	if err := db.First(&done, campaign.ID).Error; err != nil {
		t.Fatalf("reload campaign: %v", err)
	}
	if done.SentCount != int(n) {
		t.Fatalf("sent_count=%d want %d (only actual inserts count)", done.SentCount, n)
	}
	var out NotificationOutbox
	if err := db.Where("kind = 'fanout' AND payload->>'broadcast_id' = ?", fmt.Sprint(campaign.ID)).First(&out).Error; err != nil {
		t.Fatalf("load outbox row: %v", err)
	}
	if !out.Done {
		t.Fatal("fanout outbox row not marked done after tick")
	}
}

func TestDispatchRowEnqueuesProcessTask(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)

	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue })

	var enqueued []*asynq.Task
	var lastOpts []asynq.Option
	svc.SetEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		enqueued = append(enqueued, task)
		lastOpts = opts
		return &asynq.TaskInfo{ID: task.Type()}, nil
	})

	payload := mustJSON(NotifyRequest{
		EventKey:   EventApplicationStatusChanged,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"program": "CS", "status": "shortlisted"},
	})
	db.Create(&NotificationOutbox{
		Kind:    "dispatch",
		Payload: datatypes.JSON(payload),
	})

	var outbox NotificationOutbox
	db.Last(&outbox)

	err := dispatchOutboxRow(svc, outbox)
	if err != nil {
		t.Fatal(err)
	}

	if len(enqueued) != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", len(enqueued))
	}
	if enqueued[0].Type() != TaskTypeProcess {
		t.Errorf("expected type %s, got %s", TaskTypeProcess, enqueued[0].Type())
	}
	// Deterministic TaskID: retries must dedupe on the outbox row.
	wantID := fmt.Sprintf("%s:%d", TaskTypeProcess, outbox.ID)
	if got := taskIDFromOpts(lastOpts); got != wantID {
		t.Errorf("expected task ID %s, got %s", wantID, got)
	}
}

func TestDispatchRowEnqueueFailureReturnsError(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)

	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue })

	svc.SetEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		return nil, errors.New("redis connection refused")
	})

	payload := mustJSON(NotifyRequest{
		EventKey:   EventApplicationStatusChanged,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"program": "CS", "status": "shortlisted"},
	})
	db.Create(&NotificationOutbox{Kind: "dispatch", Payload: datatypes.JSON(payload)})
	var outbox NotificationOutbox
	db.Last(&outbox)

	err := dispatchOutboxRow(svc, outbox)
	if err == nil {
		t.Fatal("expected error on enqueue failure")
	}
}

// TestSweepRekicksStuckPendingDelivery: a pending delivery whose process task
// exhausted its retries is stranded — the poller sweep must re-enqueue the
// original notification:process task so the delivery can hand off.
func TestSweepRekicksStuckPendingDelivery(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	InitWorker(svc, NewRepository(db), db)
	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue; InitWorker(nil, nil, nil) })

	type enq struct {
		task *asynq.Task
		id   string
	}
	var enqueued []enq
	svc.SetEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		enqueued = append(enqueued, enq{task, taskIDFromOpts(opts)})
		return &asynq.TaskInfo{ID: task.Type()}, nil
	})

	seedUser42(t, db)
	corrID := uuid.New().String()
	inbox := AccountNotification{
		AccountType: "user", AccountID: 42,
		EventKey: EventApplicationStatusChanged, Category: "application",
		Priority: "normal", Title: "t", Body: "b",
		OccurrenceKey: "application.status_changed:sweep-occ",
	}
	db.Create(&inbox)
	outbox := NotificationOutbox{
		Kind: "dispatch", Done: true,
		OccurrenceKey: "application.status_changed:sweep-occ",
		Payload: datatypes.JSON(mustJSON(NotifyRequest{
			EventKey:      EventApplicationStatusChanged,
			Recipients:    []Ref{{Type: "user", ID: 42}},
			Data:          map[string]any{"program": "CS", "status": "shortlisted"},
			CorrelationID: corrID,
		})),
	}
	db.Create(&outbox)
	delivery := NotificationDelivery{
		NotificationID: &inbox.ID, DeliveryKind: "notification",
		DeliveryKey: fmt.Sprintf("%d:email", inbox.ID),
		AccountType: "user", AccountID: 42, Channel: "email",
		Status: "pending", CorrelationID: corrID,
	}
	db.Create(&delivery)
	// The process task burned its retries a while ago.
	db.Exec(`UPDATE notification_deliveries SET created_at = now() - interval '11 minutes' WHERE id = ?`, delivery.ID)

	tick(svc)

	var kicked *enq
	for i := range enqueued {
		if enqueued[i].task.Type() == TaskTypeProcess {
			kicked = &enqueued[i]
		}
	}
	if kicked == nil {
		t.Fatal("sweep did not re-enqueue a process task for the stuck delivery")
	}
	wantID := fmt.Sprintf("%s:%d", TaskTypeProcess, outbox.ID)
	if kicked.id != wantID {
		t.Errorf("expected task ID %s, got %s", wantID, kicked.id)
	}

	// Synthetic second process run: the delivery hands off.
	if err := HandleProcessTask(context.Background(), kicked.task); err != nil {
		t.Fatal(err)
	}
	var d NotificationDelivery
	db.First(&d, delivery.ID)
	if d.Status != "handed_off" {
		t.Errorf("expected handed_off after re-kick process run, got %s", d.Status)
	}
}
