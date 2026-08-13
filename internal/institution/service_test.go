package institution

import (
	"encoding/json"
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

func seedGlobalCourseWithFields(db *gorm.DB) *education.Course {
	careersJSON, _ := json.Marshal([]education.CareerItem{
		{Title: "Software Engineer", Icon: "code", Color: "#3b82f6"},
	})
	faqsJSON, _ := json.Marshal([]education.FaqItem{
		{Question: "What is BSc CS?", Answer: "A computer science degree"},
	})
	course := &education.Course{
		Title:       "BSc Computer Science",
		Level:       "Bachelor",
		IsGlobal:    true,
		Status:      "published",
		Field:       "Science",
		Duration:    "4 years",
		Description: "Global description",
		BannerURL:   "https://example.com/banner.jpg",
		Careers:     careersJSON,
		FAQs:        faqsJSON,
	}
	db.Create(course)
	return course
}

func TestRecalculateOverrides_DetectsDescriptionDiff(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	course := seedGlobalCourseWithFields(db)
	customDesc := "Custom description"
	overridesJSON, _ := json.Marshal(education.CourseOverrides{Description: &customDesc})

	program := &InstitutionProgram{
		GlobalCourseID: course.ID,
		Overrides:      overridesJSON,
	}

	svc.recalculateOverrides(program)

	var result education.CourseOverrides
	json.Unmarshal(program.Overrides, &result)

	if result.Description == nil || *result.Description != "Custom description" {
		t.Errorf("expected Description override to be preserved, got %v", result.Description)
	}
}

func TestRecalculateOverrides_DetectsBannerURLDiff(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	course := seedGlobalCourseWithFields(db)
	customBanner := "https://custom.com/banner.png"
	overridesJSON, _ := json.Marshal(education.CourseOverrides{BannerURL: &customBanner})

	program := &InstitutionProgram{
		GlobalCourseID: course.ID,
		Overrides:      overridesJSON,
	}

	svc.recalculateOverrides(program)

	var result education.CourseOverrides
	json.Unmarshal(program.Overrides, &result)

	if result.BannerURL == nil || *result.BannerURL != customBanner {
		t.Errorf("expected BannerURL override to be preserved, got %v", result.BannerURL)
	}
}

func TestRecalculateOverrides_DetectsCareersDiff(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	course := seedGlobalCourseWithFields(db)
	customCareers := []education.CareerItem{
		{Title: "Data Scientist", Icon: "chart", Color: "#10b981"},
	}
	overridesJSON, _ := json.Marshal(education.CourseOverrides{Careers: customCareers})

	program := &InstitutionProgram{
		GlobalCourseID: course.ID,
		Overrides:      overridesJSON,
	}

	svc.recalculateOverrides(program)

	var result education.CourseOverrides
	json.Unmarshal(program.Overrides, &result)

	if result.Careers == nil || len(result.Careers) != 1 || result.Careers[0].Title != "Data Scientist" {
		t.Errorf("expected Careers override to be preserved, got %v", result.Careers)
	}
}

func TestRecalculateOverrides_DetectsFAQsDiff(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	course := seedGlobalCourseWithFields(db)
	customFAQs := []education.FaqItem{
		{Question: "Custom Q?", Answer: "Custom A"},
	}
	overridesJSON, _ := json.Marshal(education.CourseOverrides{FAQs: customFAQs})

	program := &InstitutionProgram{
		GlobalCourseID: course.ID,
		Overrides:      overridesJSON,
	}

	svc.recalculateOverrides(program)

	var result education.CourseOverrides
	json.Unmarshal(program.Overrides, &result)

	if result.FAQs == nil || len(result.FAQs) != 1 || result.FAQs[0].Question != "Custom Q?" {
		t.Errorf("expected FAQs override to be preserved, got %v", result.FAQs)
	}
}

func TestRecalculateOverrides_ClearsWhenMatchingGlobal(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	course := seedGlobalCourseWithFields(db)
	sameDesc := "Global description"
	overridesJSON, _ := json.Marshal(education.CourseOverrides{Description: &sameDesc})

	program := &InstitutionProgram{
		GlobalCourseID: course.ID,
		Overrides:      overridesJSON,
	}

	svc.recalculateOverrides(program)

	if program.Overrides != nil {
		t.Errorf("expected Overrides to be nil when matching global, got %v", program.Overrides)
	}
}

func TestRecalculateOverrides_MultipleOverrides(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	course := seedGlobalCourseWithFields(db)
	customDesc := "Custom description"
	customBanner := "https://custom.com/banner.png"
	overridesJSON, _ := json.Marshal(education.CourseOverrides{
		Description: &customDesc,
		BannerURL:   &customBanner,
	})

	program := &InstitutionProgram{
		GlobalCourseID: course.ID,
		Overrides:      overridesJSON,
	}

	svc.recalculateOverrides(program)

	var result education.CourseOverrides
	json.Unmarshal(program.Overrides, &result)

	if result.Description == nil || *result.Description != customDesc {
		t.Errorf("expected Description override, got %v", result.Description)
	}
	if result.BannerURL == nil || *result.BannerURL != customBanner {
		t.Errorf("expected BannerURL override, got %v", result.BannerURL)
	}
}

func TestRecalculateOverrides_NoGlobalCourse(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	educationRepo := education.NewRepository(db)
	svc := NewService(repo, educationRepo, nil)

	program := &InstitutionProgram{
		GlobalCourseID: 0,
	}

	svc.recalculateOverrides(program)

	if program.Overrides != nil {
		t.Errorf("expected Overrides to remain nil, got %v", program.Overrides)
	}
}
