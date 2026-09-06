// internal/notification/service_test.go
package notification

import (
	"context"
	"testing"
)

func TestNotifyTxWritesInboxAndOutboxAtomically(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	err := svc.NotifyTx(context.Background(), db, NotifyRequest{
		EventKey:   EventApplicationStatusChanged,
		Recipients: []Ref{{Type: "user", ID: 42}},
		Data:       map[string]any{"status": "Shortlisted", "program": "BSc CS"},
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	rows, _, unread, _ := NewRepository(db).ListInbox("user", 42, 1, 20, "", false, false)
	if len(rows) != 1 || unread != 1 {
		t.Fatalf("inbox rows=%d unread=%d", len(rows), unread)
	}
	if rows[0].Title != "Application Status Updated" || rows[0].Body != "Your application for BSc CS moved to Shortlisted." {
		t.Fatalf("template render wrong: %q / %q", rows[0].Title, rows[0].Body)
	}
	var outboxPending int64
	db.Table("notification_outbox").Where("done = false").Count(&outboxPending)
	if outboxPending != 1 {
		t.Fatalf("outbox pending rows=%d want 1", outboxPending)
	}
}

func TestNotifyUnknownKeyReturnsError(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	err := svc.Notify(context.Background(), NotifyRequest{EventKey: "bogus.key", Recipients: []Ref{{Type: "user", ID: 1}}})
	if err == nil {
		t.Fatal("unknown event key must return an error")
	}
	var n int64
	db.Table("account_notifications").Count(&n)
	if n != 0 {
		t.Fatal("failed emission must not write rows")
	}
}

func TestOccurrenceKeySuppressesCallerRetries(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	req := NotifyRequest{EventKey: EventAccountApproved, Recipients: []Ref{{Type: "institution", ID: 5}}, OccurrenceKey: "approve:5"}
	if err := svc.Notify(context.Background(), req); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := svc.Notify(context.Background(), req); err != nil {
		t.Fatalf("retry: %v", err)
	}
	var n int64
	db.Table("account_notifications").Where("account_type='institution' AND account_id=5").Count(&n)
	if n != 1 {
		t.Fatalf("rows=%d want 1 (retry must be idempotent)", n)
	}
}

func TestDedupeKeyWindowSuppressesWithinWindow(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	req := NotifyRequest{EventKey: EventAccountProfileIncomplete, Recipients: []Ref{{Type: "provider", ID: 6}}, DedupeKey: "nudge:6"}
	if err := svc.Notify(context.Background(), req); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := svc.Notify(context.Background(), req); err != nil {
		t.Fatalf("second: %v", err)
	}
	var n int64
	db.Table("account_notifications").Where("account_type='provider' AND account_id=6").Count(&n)
	if n != 1 {
		t.Fatalf("rows=%d want 1 (window suppression)", n)
	}
}

func TestResolveAccountForAllRoles(t *testing.T) {
	cases := []struct {
		role               string
		userID, providerID uint
		wantType           string
		wantID             uint
		wantOK             bool
	}{
		{"student", 1, 0, "user", 1, true},
		{"institution", 2, 0, "institution", 2, true},
		{"scholarship_provider", 3, 3, "provider", 3, true},
		{"scholarship_provider_subuser", 4, 9, "provider", 9, true},
		{"superadmin", 5, 0, "user", 5, true},
		{"super_admin", 6, 0, "user", 6, true},
		{"admin", 7, 0, "user", 7, true},
		{"", 8, 0, "", 0, false},
	}
	for _, c := range cases {
		gotType, gotID, ok := ResolveAccount(c.role, c.userID, c.providerID)
		if gotType != c.wantType || gotID != c.wantID || ok != c.wantOK {
			t.Errorf("ResolveAccount(%q,%d,%d) = (%q,%d,%v)", c.role, c.userID, c.providerID, gotType, gotID, ok)
		}
	}
}

func TestRecipientsForRoleIncludesBothSuperadminSpellings(t *testing.T) {
	db := testDB(t)
	// The test database shares the server schema — users table already exists
	// from the main AutoMigrate. Insert ephemeral rows and clean them up.
	// first_name/last_name are NOT NULL without defaults in the real schema.
	db.Exec(`INSERT INTO users (email, first_name, last_name, role, status, created_at, updated_at) VALUES
		('notif-a@test.local','Notif','A','superadmin','active',now(),now()),
		('notif-b@test.local','Notif','B','super_admin','active',now(),now()),
		('notif-c@test.local','Notif','C','student','active',now(),now())`)
	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE email LIKE 'notif-%@test.local'`) })

	svc := NewService(db)
	recs, err := svc.RecipientsForRole("superadmin", "admin")
	if err != nil {
		t.Fatalf("recipients: %v", err)
	}
	found := 0
	for _, r := range recs {
		var role string
		db.Raw(`SELECT role FROM users WHERE id = ?`, r.ID).Scan(&role)
		if role == "superadmin" || role == "super_admin" {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected both superadmin spellings among %d recipients, found %d", len(recs), found)
	}
}
