// internal/notification/delivery_test.go
package notification

import (
	"context"
	"testing"
)

func TestNotifyCreatesEmailDeliveryWhenEnabled(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)

	db.Exec(`INSERT INTO users (id, email, first_name, last_name, role, created_at, updated_at)
		VALUES (42, 'test@example.com', 'Test', 'User', 'student', now(), now())
		ON CONFLICT (id) DO NOTHING`)

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

	db.Exec(`INSERT INTO users (id, email, first_name, last_name, role, created_at, updated_at)
		VALUES (42, 'test@example.com', 'Test', 'User', 'student', now(), now())
		ON CONFLICT (id) DO NOTHING`)
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

	db.Exec(`INSERT INTO users (id, email, first_name, last_name, role, created_at, updated_at)
		VALUES (42, 'test@example.com', 'Test', 'User', 'student', now(), now())
		ON CONFLICT (id) DO NOTHING`)

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
