// studsback/internal/analytics/supply_test.go
package analytics

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func migrateSupplyTwins(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS colleges (id bigserial PRIMARY KEY, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS scholarships (id bigserial PRIMARY KEY, title text, status text DEFAULT 'draft', deadline timestamptz, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS events (id bigserial PRIMARY KEY, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS blogs (id bigserial PRIMARY KEY, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS news (id bigserial PRIMARY KEY, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS bookmarks (id bigserial PRIMARY KEY, item_type text, item_id bigint, created_at timestamptz)`,
		`CREATE TABLE IF NOT EXISTS user_follows (id bigserial PRIMARY KEY, target_type text, target_id bigint, created_at timestamptz)`,
	} {
		db.Exec(ddl)
	}
	t.Cleanup(func() {
		db.Exec(`TRUNCATE colleges, scholarships, events, blogs, news, bookmarks, user_follows, institution_users, scholarship_provider_users`)
	})
}

func TestSupplyTotalsAgingAndHygiene(t *testing.T) {
	db := testDB(t)
	migrateSupplyTwins(t, db)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	db.Exec(`INSERT INTO colleges (created_at) VALUES (?), (?)`, day.Add(-48*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO scholarships (title, status, deadline, created_at) VALUES ('Old Grant','published',?,?), ('New Grant','published',?,?), ('Draft','draft',?,?)`,
		day.Add(-24*time.Hour), day.Add(-72*time.Hour), day.Add(30*24*time.Hour), day.Add(-24*time.Hour), day.Add(30*24*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO events (created_at) VALUES (?)`, day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO blogs (created_at) VALUES (?)`, day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO news (created_at) VALUES (?)`, day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO bookmarks (item_type, item_id, created_at) VALUES ('college',7,?), ('college',7,?), ('scholarship',3,?)`,
		day.Add(-24*time.Hour), day.Add(-24*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO user_follows (target_type, target_id, created_at) VALUES ('institution',9,?), ('institution',9,?)`,
		day.Add(-24*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO institution_users (status, created_at) VALUES ('pending',?), ('pending',?)`,
		time.Now().UTC().Add(-2*time.Hour), time.Now().UTC().Add(-48*time.Hour))

	out, err := NewService(db, NewUsageTracker(0)).Supply(day.Add(-72*time.Hour), day.Add(24*time.Hour), "day")
	if err != nil {
		t.Fatalf("supply: %v", err)
	}
	if out.Totals.Colleges != 2 || out.Totals.ScholarshipsPublished != 2 || out.Totals.Events != 1 || out.Totals.Blogs != 1 || out.Totals.News != 1 {
		t.Fatalf("totals wrong: %+v", out.Totals)
	}
	if out.ApprovalAging.Institutions.Lt24h != 1 || out.ApprovalAging.Institutions.D1_3 != 1 {
		t.Fatalf("aging wrong: %+v", out.ApprovalAging)
	}
	if len(out.TopBookmarked) != 2 || out.TopBookmarked[0].Count != 2 {
		t.Fatalf("top bookmarked wrong: %+v", out.TopBookmarked)
	}
	if len(out.TopFollowed) != 1 || out.TopFollowed[0].Count != 2 {
		t.Fatalf("top followed wrong: %+v", out.TopFollowed)
	}
	if len(out.StaleScholarships) != 1 || out.StaleScholarships[0].Title != "Old Grant" {
		t.Fatalf("stale scholarships wrong: %+v", out.StaleScholarships)
	}
}
