// studsback/internal/analytics/ops_test.go
package analytics

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func migrateOpsTwins(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS forum_reports (id bigserial PRIMARY KEY, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS review_reports (id bigserial PRIMARY KEY, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS contact_inquiries (id bigserial PRIMARY KEY, status text DEFAULT 'new', created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS feedback (id bigserial PRIMARY KEY, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS notification_broadcasts (id bigserial PRIMARY KEY, status text, audience text, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS account_notifications (id bigserial PRIMARY KEY, category text, created_at timestamptz)`,
	} {
		db.Exec(ddl)
	}
	t.Cleanup(func() {
		db.Exec(`TRUNCATE forum_reports, review_reports, contact_inquiries, feedback, notification_broadcasts, account_notifications`)
	})
}

func TestOpsTotalsAndBroadcasts(t *testing.T) {
	db := testDB(t)
	migrateOpsTwins(t, db)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	db.Exec(`INSERT INTO forum_reports (created_at) VALUES (?), (?)`, day.Add(-48*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO review_reports (created_at) VALUES (?)`, day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO contact_inquiries (status, created_at) VALUES ('new',?), ('replied',?)`, day.Add(-24*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO feedback (created_at) VALUES (?)`, day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO notification_broadcasts (status, audience, created_at) VALUES ('completed','all',?), ('failed','user',?)`, day.Add(-24*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO account_notifications (category, created_at) VALUES ('moderation',?), ('system',?)`, day.Add(-24*time.Hour), day.Add(-24*time.Hour))

	out, err := NewService(db, NewUsageTracker(0)).Ops(day.Add(-72*time.Hour), day.Add(24*time.Hour), "day")
	if err != nil {
		t.Fatalf("ops: %v", err)
	}
	if out.Totals.ForumReports != 2 || out.Totals.ReviewReports != 1 || out.Totals.Feedback != 1 {
		t.Fatalf("totals wrong: %+v", out.Totals)
	}
	if out.Totals.Broadcasts != 2 || out.Totals.BroadcastsFailed != 1 {
		t.Fatalf("broadcasts wrong: %+v", out.Totals)
	}
	if out.InquiriesByStatus["new"] != 1 || out.InquiriesByStatus["replied"] != 1 {
		t.Fatalf("inquiry statuses wrong: %+v", out.InquiriesByStatus)
	}
	if len(out.RecentBroadcasts) != 2 {
		t.Fatalf("recent broadcasts wrong: %+v", out.RecentBroadcasts)
	}
	if len(out.Series) == 0 {
		t.Fatal("empty series")
	}
}
