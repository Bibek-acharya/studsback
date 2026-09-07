// internal/notification/delivery_test.go
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// fakeEnqueuer is the SetEnqueuer signature alias used by worker tests.
type fakeEnqueuer = func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)

// seedUser42 inserts the fixture recipient and cleans it up afterwards —
// an uncleaned active user leaks into broadcast audience tests (shared DB).
func seedUser42(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Exec(`INSERT INTO users (id, email, first_name, last_name, role, created_at, updated_at)
		VALUES (42, 'test@example.com', 'Test', 'User', 'student', now(), now())
		ON CONFLICT (id) DO NOTHING`)
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE id = 42`) })
}

func TestNotifyCreatesEmailDeliveryWhenEnabled(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)

	seedUser42(t, db)

	err := svc.Notify(context.Background(), NotifyRequest{
		EventKey:   EventApplicationStatusChanged,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"program": "CS", "status": "shortlisted"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var delivery NotificationDelivery
	err = db.Where("account_type = ? AND account_id = ? AND channel = ?",
		"user", 42, "email").First(&delivery).Error
	if err != nil {
		t.Fatalf("expected delivery row: %v", err)
	}
	if delivery.Status != "pending" {
		t.Errorf("expected status pending, got %s", delivery.Status)
	}
	if delivery.DeliveryKind != "notification" {
		t.Errorf("expected kind notification, got %s", delivery.DeliveryKind)
	}
	if delivery.NotificationID == nil {
		t.Error("expected notification_id set")
	}
	if delivery.CorrelationID == "" {
		t.Error("expected correlation_id auto-filled")
	}
}

func TestNotifySkipsEmailDeliveryWhenPrefOff(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)

	seedUser42(t, db)
	boolPtr := func(b bool) *bool { return &b }
	db.Create(&NotificationPreference{
		AccountType: "user", AccountID: 42,
		PrefKey: "*", Email: boolPtr(false),
	})

	err := svc.Notify(context.Background(), NotifyRequest{
		EventKey:   EventApplicationStatusChanged,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"program": "CS", "status": "shortlisted"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var delivery NotificationDelivery
	err = db.Where("account_type = ? AND account_id = ? AND channel = ?",
		"user", 42, "email").First(&delivery).Error
	if err != nil {
		t.Fatalf("expected delivery row: %v", err)
	}
	if delivery.Status != "skipped" {
		t.Errorf("expected status skipped (pref off), got %s", delivery.Status)
	}
}

func TestNotifySkipsEmailDeliveryWhenEmailDefaultOff(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)

	seedUser42(t, db)

	err := svc.Notify(context.Background(), NotifyRequest{
		EventKey:   EventContentCreatedOwn,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"what": "Blog", "title": "Test"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&NotificationDelivery{}).Where("account_type = ? AND account_id = ?",
		"user", 42).Count(&count)
	if count != 0 {
		t.Errorf("expected no delivery rows for email-default-off event, got %d", count)
	}
}

func TestProcessTaskHandOffsEmail(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	InitWorker(svc, NewRepository(db), db)
	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue; InitWorker(nil, nil, nil) })

	// Install fake enqueuer to capture email:deliver tasks.
	var enqueued []*asynq.Task
	svc.SetEnqueuer(fakeEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		enqueued = append(enqueued, task)
		return &asynq.TaskInfo{ID: task.Type()}, nil
	}))

	// Seed: user with email, inbox row, pending delivery, outbox row.
	seedUser42(t, db)

	// Create inbox row + outbox row + pending delivery (mimic what NotifyTx does).
	inbox := AccountNotification{
		AccountType: "user", AccountID: 42,
		EventKey: EventApplicationStatusChanged, Category: "application",
		Priority: "normal", Title: "Application Update",
		Body: "Your application moved to shortlisted.",
	}
	db.Create(&inbox)

	corrID := uuid.New().String()
	outbox := NotificationOutbox{
		Kind: "dispatch",
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

	// Call process handler directly.
	payload, _ := json.Marshal(map[string]any{"outbox_id": outbox.ID})
	task := asynq.NewTask(TaskTypeProcess, payload)
	err := HandleProcessTask(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}

	// Verify: delivery is handed_off and email:deliver enqueued.
	db.First(&delivery, delivery.ID)
	if delivery.Status != "handed_off" {
		t.Errorf("expected handed_off, got %s", delivery.Status)
	}
	if len(enqueued) != 1 {
		t.Fatalf("expected 1 enqueued email:deliver task, got %d", len(enqueued))
	}
	if enqueued[0].Type() != TaskTypeEmailDeliver {
		t.Errorf("expected %s, got %s", TaskTypeEmailDeliver, enqueued[0].Type())
	}
}

func TestProcessTaskSkipsPrefOff(t *testing.T) {
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

	// Seed: user with email pref OFF, inbox, outbox, delivery pending
	// (pref turned off after emission — the dispatch-time re-check must skip).
	seedUser42(t, db)
	boolPtr := func(b bool) *bool { return &b }
	db.Create(&NotificationPreference{
		AccountType: "user", AccountID: 42,
		PrefKey: "*", Email: boolPtr(false),
	})
	inbox := AccountNotification{
		AccountType: "user", AccountID: 42,
		EventKey: EventApplicationStatusChanged, Category: "application",
		Priority: "normal", Title: "test", Body: "test",
	}
	db.Create(&inbox)
	outbox := NotificationOutbox{
		Kind: "dispatch",
		Payload: datatypes.JSON(mustJSON(NotifyRequest{
			EventKey:   EventApplicationStatusChanged,
			Recipients: []Ref{{Type: "user", ID: 42}},
			Data:       map[string]any{"program": "CS", "status": "shortlisted"},
		})),
	}
	db.Create(&outbox)
	delivery := NotificationDelivery{
		NotificationID: &inbox.ID, DeliveryKind: "notification",
		DeliveryKey: fmt.Sprintf("%d:email", inbox.ID),
		AccountType: "user", AccountID: 42, Channel: "email",
		Status: "pending",
	}
	db.Create(&delivery)

	payload, _ := json.Marshal(map[string]any{"outbox_id": outbox.ID})
	task := asynq.NewTask(TaskTypeProcess, payload)
	err := HandleProcessTask(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	// Verify: pref re-check flips the delivery to skipped, no enqueue.
	db.First(&delivery, delivery.ID)
	if delivery.Status != "skipped" {
		t.Errorf("expected skipped, got %s", delivery.Status)
	}
	if len(enqueued) != 0 {
		t.Errorf("expected no email:deliver tasks, got %d", len(enqueued))
	}
}

func TestProcessTaskEnqueueFailureReturnsError(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	InitWorker(svc, NewRepository(db), db)
	origEnqueue := EnqueueFunc
	t.Cleanup(func() { EnqueueFunc = origEnqueue; InitWorker(nil, nil, nil) })
	svc.SetEnqueuer(fakeEnqueuer(func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		return nil, errors.New("redis connection refused")
	}))

	seedUser42(t, db)
	inbox := AccountNotification{
		AccountType: "user", AccountID: 42,
		EventKey: EventApplicationStatusChanged, Category: "application",
		Priority: "normal", Title: "test", Body: "test",
	}
	db.Create(&inbox)
	corrID := uuid.New().String()
	outbox := NotificationOutbox{
		Kind: "dispatch",
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

	payload, _ := json.Marshal(map[string]any{"outbox_id": outbox.ID})
	task := asynq.NewTask(TaskTypeProcess, payload)
	err := HandleProcessTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error so asynq retries the process task")
	}
	// Verify: delivery reverted to pending (retryable on the next process run).
	db.First(&delivery, delivery.ID)
	if delivery.Status != "pending" {
		t.Errorf("expected pending after enqueue failure, got %s", delivery.Status)
	}
}

func TestReopenExpiredDeliveriesSweep(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	inbox := AccountNotification{
		AccountType: "user", AccountID: 42, EventKey: EventApplicationStatusChanged,
		Category: "application", Title: "t", Body: "b",
	}
	db.Create(&inbox)
	db.Create(&NotificationDelivery{
		NotificationID: &inbox.ID, DeliveryKind: "notification",
		DeliveryKey: fmt.Sprintf("%d:email", inbox.ID),
		AccountType: "user", AccountID: 42, Channel: "email",
		Status: "dispatching", CorrelationID: "corr-sweep",
	})
	// Simulate a crashed worker: lease already expired.
	db.Exec(`UPDATE notification_deliveries SET dispatch_expires_at = now() - interval '1 minute'`)

	n, err := repo.ReopenExpiredDeliveries()
	if err != nil || n != 1 {
		t.Fatalf("reopen: %v %d", err, n)
	}
	var d NotificationDelivery
	db.Where("correlation_id = ?", "corr-sweep").First(&d)
	if d.Status != "pending" {
		t.Errorf("expected pending after sweep, got %s", d.Status)
	}
	if d.DispatchExpiresAt != nil {
		t.Error("expected dispatch_expires_at cleared")
	}
}
