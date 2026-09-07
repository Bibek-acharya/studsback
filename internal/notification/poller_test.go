// internal/notification/poller_test.go
package notification

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/datatypes"
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

// TestPollerExpandsFanoutRow drives one synchronous tick over a seeded
// fanout row (the production path CreateBroadcast relies on): the tick must
// expand the campaign's audience and only then mark the row done.
func TestPollerExpandsFanoutRow(t *testing.T) {
	db := testDB(t)
	// Audience stubs — same adaptation as TestBroadcastCreatesCampaignAndFansOut
	// (the shared test DB only carries the notification tables + users).
	db.Exec(`CREATE TABLE IF NOT EXISTS institution_users (id bigserial PRIMARY KEY, status text, deleted_at timestamptz)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS scholarship_provider_users (id bigserial PRIMARY KEY, status text, deleted_at timestamptz)`)
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
