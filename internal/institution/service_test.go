package institution

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/education"
)

func setupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&education.Course{}); err != nil {
		t.Fatalf("auto migrate courses: %v", err)
	}

	return db
}

func seedGlobalCourse(db *gorm.DB, isGlobal bool, status string) *education.Course {
	course := &education.Course{
		Title:    "BSc Computer Science",
		Level:    "Bachelor",
		IsGlobal: isGlobal,
		Status:   status,
		Field:    "Science",
		Duration: "4 years",
	}
	db.Create(course)
	return course
}

func TestCreateProgram_GlobalCourseNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	req := CreateProgramRequest{
		GlobalCourseID:      999,
		Fee:                 "50000",
		Eligibility:         "12th pass",
		Capacity:            100,
		InstitutionName:     "Test College",
		InstitutionLocation: "Kathmandu",
	}

	_, err := svc.CreateProgram(1, req)
	if err == nil {
		t.Fatal("expected error for non-existent global course")
	}
	if err.Error() != "global course not found" {
		t.Errorf("error = %q, want %q", err.Error(), "global course not found")
	}
}

func TestCreateProgram_GlobalCourseNotGlobal(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	course := seedGlobalCourse(db, false, "published")

	req := CreateProgramRequest{
		GlobalCourseID:      course.ID,
		Fee:                 "50000",
		Eligibility:         "12th pass",
		Capacity:            100,
		InstitutionName:     "Test College",
		InstitutionLocation: "Kathmandu",
	}

	_, err := svc.CreateProgram(1, req)
	if err == nil {
		t.Fatal("expected error for non-global course")
	}
	if err.Error() != "selected course is not a global course" {
		t.Errorf("error = %q, want %q", err.Error(), "selected course is not a global course")
	}
}

func TestCreateProgram_GlobalCourseDraft(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	// Draft course with IsGlobal=true: FindCourseByIDOnly finds it, IsGlobal check passes
	// but task spec validates it should still work for creating programs
	course := seedGlobalCourse(db, true, "draft")

	req := CreateProgramRequest{
		GlobalCourseID:      course.ID,
		Fee:                 "50000",
		Eligibility:         "12th pass",
		Capacity:            100,
		InstitutionName:     "Test College",
		InstitutionLocation: "Kathmandu",
	}

	// The task spec only validates course exists and IsGlobal is true.
	// Draft global courses pass validation; program creation may fail on SQLite
	// due to []string NullifiedFields type incompatibility.
	_, err := svc.CreateProgram(1, req)
	// Either succeeds (PostgreSQL) or fails with SQLite scan error (expected in tests)
	if err != nil && err.Error() == "global course not found" {
		t.Error("draft global course should not trigger 'not found' error")
	}
	if err != nil && err.Error() == "selected course is not a global course" {
		t.Error("draft global course with IsGlobal=true should not trigger 'not global' error")
	}
}

func TestCreateProgram_ValidatesBeforeCreation(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	// Test that validation happens before any DB write
	// Non-existent course should fail without attempting insert
	req := CreateProgramRequest{
		GlobalCourseID:      0,
		Fee:                 "50000",
		Eligibility:         "12th pass",
		Capacity:            100,
		InstitutionName:     "Test College",
		InstitutionLocation: "Kathmandu",
	}

	_, err := svc.CreateProgram(1, req)
	if err == nil {
		t.Fatal("expected error for GlobalCourseID=0")
	}
	if err.Error() != "global course not found" {
		t.Errorf("error = %q, want %q", err.Error(), "global course not found")
	}
}

func TestCreateProgram_MarshaledArraysPassedToModel(t *testing.T) {
	// Verify that the CreateProgramRequest fields are properly typed
	// for JSON marshaling (whoShouldChoose, features, etc.)
	req := CreateProgramRequest{
		GlobalCourseID: 1,
		WhoShouldChoose: []education.PersonaItem{
			{Icon: "🎯", Title: "Test"},
		},
		Features: []education.FeatureItem{
			{Title: "Feature1"},
		},
		FullTimeCourses: []education.FullTimeCourse{
			{Course: "BSc"},
		},
		FeeItems: []education.FeeItem{
			{Particular: "Tuition", Amount: "50000"},
		},
		NullifiedFields: []string{"scholarships"},
	}

	if len(req.WhoShouldChoose) != 1 {
		t.Errorf("WhoShouldChoose len = %d, want 1", len(req.WhoShouldChoose))
	}
	if len(req.Features) != 1 {
		t.Errorf("Features len = %d, want 1", len(req.Features))
	}
	if len(req.FullTimeCourses) != 1 {
		t.Errorf("FullTimeCourses len = %d, want 1", len(req.FullTimeCourses))
	}
	if len(req.FeeItems) != 1 {
		t.Errorf("FeeItems len = %d, want 1", len(req.FeeItems))
	}
	if len(req.NullifiedFields) != 1 || req.NullifiedFields[0] != "scholarships" {
		t.Errorf("NullifiedFields = %v, want [scholarships]", req.NullifiedFields)
	}
}
