// internal/notification/poller_test.go
package notification

import (
	"testing"
	"time"
)

func TestPollerClaimsAndCompletesDueRows(t *testing.T) {
	db := testDB(t)
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
