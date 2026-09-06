// internal/notification/repository_test.go
package notification

import (
	"testing"
	"time"
)

func TestInsertNotificationsAndOccurrenceIdempotency(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	row := AccountNotification{AccountType: "user", AccountID: 1,
		EventKey: EventApplicationStatusChanged, Category: "application",
		Priority: PriorityCritical, Title: "T", Body: "B", OccurrenceKey: "ok:1"}
	if err := repo.InsertNotifications(db, []AccountNotification{row}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	dup := row
	dup.ID = 0
	if err := repo.InsertNotifications(db, []AccountNotification{dup}); err == nil {
		t.Fatal("duplicate occurrence key must be rejected by uq_an_occurrence")
	}
}

func TestListInboxUnreadFirstAndCounts(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	now := time.Now()
	rows := []AccountNotification{
		{AccountType: "user", AccountID: 7, EventKey: EventAccountWelcome, Category: "account", Title: "old read", ReadAt: &now},
		{AccountType: "user", AccountID: 7, EventKey: EventApplicationStatusChanged, Category: "application", Priority: PriorityCritical, Title: "unread"},
	}
	if err := repo.InsertNotifications(db, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, total, unread, err := repo.ListInbox("user", 7, 1, 20, "", false, false)
	if err != nil || total != 2 || unread != 1 {
		t.Fatalf("list: %v total=%d unread=%d", err, total, unread)
	}
	if got[0].Title != "unread" {
		t.Fatalf("unread-first order violated: first=%q", got[0].Title)
	}
}

func TestMarkReadOwnershipAndBulk(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	rows := []AccountNotification{
		{AccountType: "user", AccountID: 9, EventKey: EventAccountWelcome, Category: "account", Title: "a"},
		{AccountType: "user", AccountID: 8, EventKey: EventAccountWelcome, Category: "account", Title: "b"},
	}
	if err := repo.InsertNotifications(db, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n, _ := repo.MarkRead("user", 9, rows[0].ID); n != 1 {
		t.Fatalf("owner mark-read affected %d", n)
	}
	if n, _ := repo.MarkRead("user", 9, rows[1].ID); n != 0 {
		t.Fatalf("cross-account mark-read affected %d; must be 0", n)
	}
	if n, _ := repo.MarkAllRead("user", 9); n != 0 {
		t.Fatalf("mark-all after single read should affect 0, got %d", n)
	}
}

func TestLeaseAcquisitionSemantics(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	// First call inserts: inserted=true, notificationID nil.
	id1, nid1, inserted, err := repo.AcquireLease(db, "provider", 3, "nudge", time.Hour)
	if err != nil || !inserted || nid1 != nil {
		t.Fatalf("first acquire: id=%d nid=%v inserted=%v err=%v", id1, nid1, inserted, err)
	}
	// Active lease: zero rows → inserted=false signals suppression.
	if _, _, inserted2, err := repo.AcquireLease(db, "provider", 3, "nudge", time.Hour); err != nil || inserted2 {
		t.Fatalf("active lease must suppress: inserted=%v err=%v", inserted2, err)
	}
	// Expired lease re-acquires as a renewal (row returned, inserted=false).
	db.Exec(`UPDATE notification_dedupe_leases SET expires_at = now() - interval '1 minute' WHERE id = ?`, id1)
	id3, _, inserted3, err := repo.AcquireLease(db, "provider", 3, "nudge", time.Hour)
	if err != nil || inserted3 || id3 != id1 {
		t.Fatalf("expired renewal: id=%d/%d inserted=%v err=%v", id3, id1, inserted3, err)
	}
}
