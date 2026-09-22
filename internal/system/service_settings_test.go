package system

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testSettingsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&SystemSetting{}, &institutionUsersRow{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// institutionUsersRow is a projection of the institution_users table for the
// type-counts query, kept local so tests don't import the auth module.
type institutionUsersRow struct {
	ID               uint           `gorm:"primarykey"`
	OrganizationType string         `gorm:"column:organization_type"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (institutionUsersRow) TableName() string { return "institution_users" }

func boolPtr(b bool) *bool { return &b }

func TestGetCollegeAdCardSettingsDefaultsAllEnabled(t *testing.T) {
	svc := NewService(NewRepository(testSettingsDB(t)), nil)

	settings, err := svc.GetCollegeAdCardSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if !settings.Trending || !settings.ByType || !settings.Rating {
		t.Fatalf("defaults wrong: %+v, want all true", settings)
	}
}

func TestUpdateCollegeAdCardSettingsPartialUpdatePersists(t *testing.T) {
	db := testSettingsDB(t)
	svc := NewService(NewRepository(db), nil)

	// Disable by_type only; the other keys stay at their defaults.
	updated, err := svc.UpdateCollegeAdCardSettings(UpdateCollegeAdCardSettingsRequest{ByType: boolPtr(false)})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if !updated.Trending || updated.ByType || !updated.Rating {
		t.Fatalf("update response wrong: %+v", updated)
	}

	// A fresh read (new service over the same DB) sees the persisted value.
	reloaded := NewService(NewRepository(db), nil)
	settings, err := reloaded.GetCollegeAdCardSettings()
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if !settings.Trending || settings.ByType || !settings.Rating {
		t.Fatalf("persisted settings wrong: %+v", settings)
	}

	// A second partial update leaves previously-set keys untouched.
	updated2, err := reloaded.UpdateCollegeAdCardSettings(UpdateCollegeAdCardSettingsRequest{Trending: boolPtr(false)})
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if updated2.Trending || updated2.ByType || !updated2.Rating {
		t.Fatalf("second update response wrong: %+v", updated2)
	}

	// Exactly one settings row exists (update-in-place, no duplicates).
	var n int64
	db.Model(&SystemSetting{}).Where("key = ?", collegeAdCardSettingsKey).Count(&n)
	if n != 1 {
		t.Fatalf("settings rows=%d want 1", n)
	}
}

func TestGetCollegeTypeCountsFiltersAndOrders(t *testing.T) {
	db := testSettingsDB(t)
	rows := []institutionUsersRow{
		{OrganizationType: "Public"},
		{OrganizationType: "Public"},
		{OrganizationType: "Community"},
		{OrganizationType: ""}, // excluded: empty type
	}
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed institution: %v", err)
		}
	}
	// Soft-deleted row: excluded by deleted_at IS NULL.
	if err := db.Create(&institutionUsersRow{OrganizationType: "Public"}).Error; err != nil {
		t.Fatalf("seed deleted institution: %v", err)
	}
	if err := db.Delete(&institutionUsersRow{}, 5).Error; err != nil {
		t.Fatalf("soft delete institution: %v", err)
	}

	svc := NewService(NewRepository(db), nil)
	counts, err := svc.GetCollegeTypeCounts()
	if err != nil {
		t.Fatalf("type counts: %v", err)
	}
	if len(counts) != 2 {
		t.Fatalf("counts=%+v, want 2 rows", counts)
	}
	if counts[0].Type != "Public" || counts[0].Count != 2 {
		t.Fatalf("first row wrong: %+v, want Public/2", counts[0])
	}
	if counts[1].Type != "Community" || counts[1].Count != 1 {
		t.Fatalf("second row wrong: %+v, want Community/1", counts[1])
	}
}

func TestSubmitAndListCollegeAdFeedbackIncludesRating(t *testing.T) {
	db := testSettingsDB(t)
	if err := db.AutoMigrate(&CollegeRecommendationFeedback{}); err != nil {
		t.Fatalf("auto migrate feedback: %v", err)
	}
	svc := NewService(NewRepository(db), nil)

	if err := svc.SubmitCollegeAdFeedback(CollegeAdFeedbackRequest{
		Helpful: true,
		Rating:  4,
		Reasons: []string{"Helpful", "Clear"},
		Comment: "  nice page  ",
	}); err != nil {
		t.Fatalf("submit feedback with rating: %v", err)
	}
	// Recommendation feedback (no rating) keeps rating=0.
	if err := svc.SubmitCollegeAdFeedback(CollegeAdFeedbackRequest{
		Helpful: false,
		Reasons: []string{"Off-topic"},
	}); err != nil {
		t.Fatalf("submit feedback without rating: %v", err)
	}

	out, err := svc.GetCollegeAdFeedback()
	if err != nil {
		t.Fatalf("list feedback: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("items=%d want 2", len(out.Items))
	}
	// Newest first: unrated (id 2) before rated (id 1).
	if out.Items[0].Rating != 0 || out.Items[1].Rating != 4 {
		t.Fatalf("ratings=[%d,%d] want [0,4] newest first", out.Items[0].Rating, out.Items[1].Rating)
	}
	if out.Items[1].Reasons != "Helpful,Clear" {
		t.Fatalf("reasons=%q want Helpful,Clear", out.Items[1].Reasons)
	}
	if out.Items[1].Comment != "nice page" {
		t.Fatalf("comment=%q want trimmed", out.Items[1].Comment)
	}
	if out.Stats.Total != 2 || out.Stats.HelpfulCount != 1 || out.Stats.NotHelpfulCount != 1 {
		t.Fatalf("stats wrong: %+v", out.Stats)
	}
}
