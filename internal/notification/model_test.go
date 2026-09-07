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

func TestSchemaHasPreferencesTable(t *testing.T) {
	db := testDB(t)
	var count int64
	db.Raw(`SELECT count(*) FROM information_schema.tables
		WHERE table_name = 'notification_preferences'`).Scan(&count)
	if count != 1 {
		t.Fatal("missing notification_preferences table")
	}
	// verify unique index
	var idxCount int64
	db.Raw(`SELECT count(*) FROM pg_indexes
		WHERE tablename = 'notification_preferences' AND indexname = 'uq_pref'`).Scan(&idxCount)
	if idxCount != 1 {
		t.Fatal("missing uq_pref index")
	}
}
