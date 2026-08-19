package education

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Course{}, &Affiliation{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedCourses(db *gorm.DB) {
	aff1 := Affiliation{Name: "Tribhuvan University"}
	aff2 := Affiliation{Name: "Kathmandu University"}
	db.Create(&aff1)
	db.Create(&aff2)

	courses := []Course{
		{Title: "BSc Computer Science", Level: "Bachelor", AffiliationID: &aff1.ID, IsGlobal: true, Status: "published", Field: "Science"},
		{Title: "BBA", Level: "Bachelor", AffiliationID: &aff2.ID, IsGlobal: true, Status: "published", Field: "Management"},
		{Title: "MSc IT", Level: "Master", AffiliationID: &aff1.ID, IsGlobal: true, Status: "published", Field: "Science"},
		{Title: "+2 Science", Level: "+2", NonUniversityAffiliation: "NEB", IsGlobal: true, Status: "published", Field: "Science"},
		{Title: "+2 Management", Level: "+2", NonUniversityAffiliation: "NEB", IsGlobal: true, Status: "published", Field: "Management"},
		{Title: "Diploma Engineering", Level: "Diploma", NonUniversityAffiliation: "CTEVT", IsGlobal: true, Status: "published", Field: "Engineering"},
		{Title: "Draft Course", Level: "Bachelor", AffiliationID: &aff1.ID, IsGlobal: true, Status: "draft"},
		{Title: "Private Course", Level: "Bachelor", AffiliationID: &aff1.ID, IsGlobal: false, Status: "published"},
	}
	for _, c := range courses {
		db.Create(&c)
	}
}

func TestFindCoursesByLevel_ReturnsMatchingPublished(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	courses, total, err := repo.FindCoursesByLevel("Bachelor", 1, 10)
	if err != nil {
		t.Fatalf("FindCoursesByLevel() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(courses) != 2 {
		t.Errorf("len(courses) = %d, want 2", len(courses))
	}
	for _, c := range courses {
		if c.Level != "Bachelor" {
			t.Errorf("course %q has level %q, want Bachelor", c.Title, c.Level)
		}
		if c.Status != "published" {
			t.Errorf("course %q has status %q, want published", c.Title, c.Status)
		}
		if !c.IsGlobal {
			t.Errorf("course %q is not global", c.Title)
		}
	}
}

func TestFindCoursesByLevel_Pagination(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	courses, total, err := repo.FindCoursesByLevel("Bachelor", 1, 1)
	if err != nil {
		t.Fatalf("FindCoursesByLevel() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(courses) != 1 {
		t.Errorf("len(courses) = %d, want 1", len(courses))
	}
}

func TestFindCoursesByLevel_EmptyResult(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	courses, total, err := repo.FindCoursesByLevel("PhD", 1, 10)
	if err != nil {
		t.Fatalf("FindCoursesByLevel() error = %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(courses) != 0 {
		t.Errorf("len(courses) = %d, want 0", len(courses))
	}
}

func TestFindCoursesByAffiliation_ReturnsMatching(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	var aff Affiliation
	db.Where("name = ?", "Tribhuvan University").First(&aff)

	courses, total, err := repo.FindCoursesByAffiliation(aff.ID, 1, 10)
	if err != nil {
		t.Fatalf("FindCoursesByAffiliation() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2 (BSc + MSc)", total)
	}
	if len(courses) != 2 {
		t.Errorf("len(courses) = %d, want 2", len(courses))
	}
	for _, c := range courses {
		if c.AffiliationID == nil || *c.AffiliationID != aff.ID {
			t.Errorf("course %q affiliation_id mismatch", c.Title)
		}
	}
}

func TestFindCoursesByAffiliation_OtherUniversity(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	var aff Affiliation
	db.Where("name = ?", "Kathmandu University").First(&aff)

	courses, total, err := repo.FindCoursesByAffiliation(aff.ID, 1, 10)
	if err != nil {
		t.Fatalf("FindCoursesByAffiliation() error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1 (BBA)", total)
	}
	if len(courses) != 1 {
		t.Errorf("len(courses) = %d, want 1", len(courses))
	}
	if courses[0].Title != "BBA" {
		t.Errorf("title = %q, want BBA", courses[0].Title)
	}
}

func TestFindSecondaryCourses_ReturnsNoAffiliation(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	courses, total, err := repo.FindSecondaryCourses(1, 10)
	if err != nil {
		t.Fatalf("FindSecondaryCourses() error = %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 (+2 Sci, +2 Mgmt, Diploma)", total)
	}
	if len(courses) != 3 {
		t.Errorf("len(courses) = %d, want 3", len(courses))
	}
	for _, c := range courses {
		if c.AffiliationID != nil {
			t.Errorf("course %q has non-nil affiliation_id", c.Title)
		}
	}
}

func TestFindSecondaryCourses_Pagination(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	courses, total, err := repo.FindSecondaryCourses(1, 2)
	if err != nil {
		t.Fatalf("FindSecondaryCourses() error = %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(courses) != 2 {
		t.Errorf("len(courses) = %d, want 2", len(courses))
	}
}

func TestFindCourseByIDWithAffiliation_ReturnsCourseAndAffiliation(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	var course Course
	db.Where("title = ?", "BSc Computer Science").First(&course)

	result, aff, err := repo.FindCourseByIDWithAffiliation(course.ID)
	if err != nil {
		t.Fatalf("FindCourseByIDWithAffiliation() error = %v", err)
	}
	if result == nil {
		t.Fatal("course is nil")
	}
	if result.Title != "BSc Computer Science" {
		t.Errorf("title = %q, want BSc Computer Science", result.Title)
	}
	if aff == nil {
		t.Fatal("affiliation is nil, expected non-nil")
	}
	if aff.Name != "Tribhuvan University" {
		t.Errorf("affiliation name = %q, want Tribhuvan University", aff.Name)
	}
}

func TestFindCourseByIDWithAffiliation_SecondaryCourseNoAffiliation(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	var course Course
	db.Where("title = ?", "+2 Science").First(&course)

	result, aff, err := repo.FindCourseByIDWithAffiliation(course.ID)
	if err != nil {
		t.Fatalf("FindCourseByIDWithAffiliation() error = %v", err)
	}
	if result == nil {
		t.Fatal("course is nil")
	}
	if result.Title != "+2 Science" {
		t.Errorf("title = %q, want +2 Science", result.Title)
	}
	if aff != nil {
		t.Errorf("affiliation should be nil for secondary course, got %+v", aff)
	}
}

func TestFindCourseByIDWithAffiliation_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	result, aff, err := repo.FindCourseByIDWithAffiliation(999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("course should be nil, got %+v", result)
	}
	if aff != nil {
		t.Errorf("affiliation should be nil, got %+v", aff)
	}
}

func TestFindCourseByIDWithAffiliation_DanglingAffiliationID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	bogusID := uint(9999)
	course := Course{Title: "Orphan Course", Level: "Bachelor", AffiliationID: &bogusID, IsGlobal: true, Status: "published"}
	db.Create(&course)

	result, aff, err := repo.FindCourseByIDWithAffiliation(course.ID)
	if err == nil {
		t.Fatal("expected error for dangling affiliation_id, got nil")
	}
	if result == nil {
		t.Fatal("course should still be returned")
	}
	if result.Title != "Orphan Course" {
		t.Errorf("title = %q, want Orphan Course", result.Title)
	}
	if aff != nil {
		t.Errorf("affiliation should be nil for dangling id, got %+v", aff)
	}
}
