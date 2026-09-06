// internal/notification/model_test.go
package notification

import "testing"

func TestSchemaHasRequiredTablesAndIndexes(t *testing.T) {
	db := testDB(t)
	for _, table := range []string{
		"account_notifications", "notification_outbox",
		"notification_broadcasts", "notification_dedupe_leases",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
	for _, idx := range []string{"idx_an_inbox", "idx_an_cat", "uq_an_occurrence", "uq_an_legacy", "uq_broadcast_idem", "uq_lease"} {
		var count int64
		db.Raw(`SELECT count(*) FROM pg_indexes WHERE indexname = ?`, idx).Scan(&count)
		if count == 0 {
			t.Fatalf("missing index %s", idx)
		}
	}
}
