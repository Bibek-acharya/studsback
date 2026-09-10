// studsback/internal/analytics/funnel_test.go
package analytics

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func migrateFunnelTwins(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Exec(`CREATE TABLE IF NOT EXISTS admissions (id bigserial PRIMARY KEY, status text DEFAULT 'pending', created_at timestamptz)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS scholarship_applications (id bigserial PRIMARY KEY, status text DEFAULT 'pending', created_at timestamptz)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS counselling_bookings (id bigserial PRIMARY KEY, status text DEFAULT 'pending', created_at timestamptz)`)
	t.Cleanup(func() {
		db.Exec(`TRUNCATE admissions, scholarship_applications, counselling_bookings`)
	})
}

func TestFunnelBucketsAndConversion(t *testing.T) {
	db := testDB(t)
	migrateFunnelTwins(t, db)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	db.Exec(`INSERT INTO admissions (status, created_at) VALUES ('pending',?), ('approved',?), ('rejected',?)`, day.Add(-48*time.Hour), day.Add(-24*time.Hour), day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO scholarship_applications (status, created_at) VALUES ('pending',?)`, day.Add(-24*time.Hour))
	db.Exec(`INSERT INTO counselling_bookings (status, created_at) VALUES ('confirmed',?), ('cancelled',?)`, day.Add(-24*time.Hour), day.Add(-24*time.Hour))

	out, err := NewService(db, NewUsageTracker(0)).Funnel(day.Add(-72*time.Hour), day.Add(24*time.Hour), "day")
	if err != nil {
		t.Fatalf("funnel: %v", err)
	}
	if out.Totals.Admissions != 3 || out.Totals.ScholarshipApplications != 1 || out.Totals.Bookings != 2 {
		t.Fatalf("totals wrong: %+v", out.Totals)
	}
	// 1 approved of 3 admissions → 33.33%.
	if out.ConversionPct < 33.3 || out.ConversionPct > 33.4 {
		t.Fatalf("conversion=%v want ~33.33", out.ConversionPct)
	}
	if len(out.Series) == 0 {
		t.Fatal("empty series")
	}
}
