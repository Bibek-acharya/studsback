// internal/notification/ensure_indexes_test.go
package notification

import (
	"testing"

	"gorm.io/gorm"
)

func indexExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM pg_indexes WHERE indexname = ?`, name).Scan(&n).Error; err != nil {
		t.Fatalf("pg_indexes query: %v", err)
	}
	return n == 1
}

func TestEnsurePostgresIndexes(t *testing.T) {
	db := testDB(t)

	// Best-effort strip of the objects: proves Ensure works on a schema
	// that lacks them (fresh AutoMigrate-only boot).
	for _, q := range []string{
		`DROP INDEX IF EXISTS uq_pref`,
		`DROP INDEX IF EXISTS uq_del_notification`,
		`DROP INDEX IF EXISTS uq_del_digest`,
		`DROP INDEX IF EXISTS uq_del_anonymous`,
		`ALTER TABLE notification_deliveries DROP CONSTRAINT IF EXISTS chk_del_subject`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("drop %q: %v", q, err)
		}
	}

	if err := EnsurePostgresIndexes(db); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	for _, idx := range []string{"uq_pref", "uq_del_notification", "uq_del_digest", "uq_del_anonymous"} {
		if !indexExists(t, db, idx) {
			t.Fatalf("missing index %s after ensure", idx)
		}
	}
	var chk int64
	if err := db.Raw(`SELECT count(*) FROM pg_constraint WHERE conname = 'chk_del_subject'`).Scan(&chk).Error; err != nil {
		t.Fatalf("pg_constraint query: %v", err)
	}
	if chk != 1 {
		t.Fatal("missing chk_del_subject constraint after ensure")
	}

	// Idempotent: second run must not error and must not duplicate.
	if err := EnsurePostgresIndexes(db); err != nil {
		t.Fatalf("second ensure: %v", err)
	}

	// Upsert path (the 42P10 crash) works on the boot-created index.
	repo := NewRepository(db)
	b := true
	if err := repo.UpsertPreference(NotificationPreference{AccountType: "user", AccountID: 999001, PrefKey: "test.key", Email: &b}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertPreference(NotificationPreference{AccountType: "user", AccountID: 999001, PrefKey: "test.key", Email: &b}); err != nil {
		t.Fatalf("second upsert (ON CONFLICT): %v", err)
	}

	// Partial-index uniqueness enforced by the boot-created index.
	nid := uint(1)
	d1 := NotificationDelivery{DeliveryKind: "notification", NotificationID: &nid, DeliveryKey: "ensure-test-key", AccountType: "user", AccountID: 999001, Channel: "email", Status: "pending"}
	if err := repo.InsertDelivery(nil, d1); err != nil {
		t.Fatalf("first delivery insert: %v", err)
	}
	if err := repo.InsertDelivery(nil, d1); err == nil {
		t.Fatal("expected duplicate delivery_key to fail under uq_del_notification")
	}
}
