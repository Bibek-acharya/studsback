package education

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Course{}, &Affiliation{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Exec(`CREATE TABLE institution_programs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		institution_id INTEGER NOT NULL,
		global_course_id INTEGER NOT NULL,
		fee TEXT,
		eligibility TEXT,
		capacity INTEGER DEFAULT 0,
		status TEXT DEFAULT 'active',
		who_should_choose BLOB DEFAULT '[]',
		features BLOB DEFAULT '[]',
		full_time_courses BLOB DEFAULT '[]',
		fee_items BLOB DEFAULT '[]',
		overrides BLOB DEFAULT '{}',
		nullified_fields BLOB DEFAULT '[]',
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create institution_programs: %v", err)
	}
	return db
}

func seedServiceData(db *gorm.DB) {
	aff := Affiliation{Name: "Tribhuvan University"}
	db.Create(&aff)

	careersJSON, _ := json.Marshal([]CareerItem{{Title: "Software Engineer", Icon: "code", Color: "blue"}})
	faqsJSON, _ := json.Marshal([]FaqItem{{Question: "What is CS?", Answer: "Computer Science"}})

	courses := []Course{
		{
			Title:           "BSc Computer Science",
			Level:           "Bachelor",
			AffiliationID:   &aff.ID,
			IsGlobal:        true,
			Status:          "published",
			Field:           "Science",
			Duration:        "4 years",
			Description:     "A comprehensive CS program",
			BannerURL:       "https://example.com/banner.jpg",
			ScholarshipDesc: "Merit-based",
			Careers:         careersJSON,
			FAQs:            faqsJSON,
		},
		{
			Title:                    "+2 Science",
			Level:                    "+2",
			IsGlobal:                 true,
			Status:                   "published",
			Field:                    "Science",
			Duration:                 "2 years",
			NonUniversityAffiliation: "NEB",
		},
		{
			Title:         "MBA",
			Level:         "Master",
			AffiliationID: &aff.ID,
			IsGlobal:      true,
			Status:        "published",
			Field:         "Management",
			Duration:      "2 years",
		},
	}
	for _, c := range courses {
		db.Create(&c)
	}
}

func TestResolveCourse_ReturnsMergedData(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	var course Course
	db.Where("title = ?", "BSc Computer Science").First(&course)

	overridesJSON, _ := json.Marshal(CourseOverrides{})
	instCareersJSON, _ := json.Marshal([]PersonaItem{{Icon: "laptop", Title: "Tech Enthusiast", ShortDesc: "Loves tech"}})
	instFeaturesJSON, _ := json.Marshal([]FeatureItem{{Title: "Lab Access", ShortDesc: "24/7 lab"}})
	nullifiedJSON, _ := json.Marshal([]string{})

	db.Exec(`INSERT INTO institution_programs (institution_id, global_course_id, fee, eligibility, capacity, status, who_should_choose, features, full_time_courses, fee_items, overrides, nullified_fields) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		10, course.ID, "500000", "Intermediate", 60, "active",
		instCareersJSON, instFeaturesJSON, []byte("[]"), []byte("[]"),
		overridesJSON, nullifiedJSON,
	)

	resolved, err := svc.ResolveCourse(course.ID, 10)
	if err != nil {
		t.Fatalf("ResolveCourse() error = %v", err)
	}
	if resolved.Title != "BSc Computer Science" {
		t.Errorf("Title = %q, want %q", resolved.Title, "BSc Computer Science")
	}
	if resolved.InstitutionID != 10 {
		t.Errorf("InstitutionID = %d, want 10", resolved.InstitutionID)
	}
	if resolved.Fee != "500000" {
		t.Errorf("Fee = %q, want 500000", resolved.Fee)
	}
	if resolved.Eligibility != "Intermediate" {
		t.Errorf("Eligibility = %q, want Intermediate", resolved.Eligibility)
	}
	if resolved.Capacity != 60 {
		t.Errorf("Capacity = %d, want 60", resolved.Capacity)
	}
	if resolved.AffiliationName != "Tribhuvan University" {
		t.Errorf("AffiliationName = %q, want Tribhuvan University", resolved.AffiliationName)
	}
	if len(resolved.WhoShouldChoose) != 1 {
		t.Errorf("WhoShouldChoose len = %d, want 1", len(resolved.WhoShouldChoose))
	}
	if len(resolved.Features) != 1 {
		t.Errorf("Features len = %d, want 1", len(resolved.Features))
	}
	if len(resolved.Careers) != 1 {
		t.Errorf("Careers len = %d, want 1", len(resolved.Careers))
	}
	if len(resolved.FAQs) != 1 {
		t.Errorf("FAQs len = %d, want 1", len(resolved.FAQs))
	}
}

func TestResolveCourse_AppliesOverrides(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	var course Course
	db.Where("title = ?", "BSc Computer Science").First(&course)

	customDesc := "Custom description"
	overridesJSON, _ := json.Marshal(CourseOverrides{Description: &customDesc})
	nullifiedJSON, _ := json.Marshal([]string{})

	db.Exec(`INSERT INTO institution_programs (institution_id, global_course_id, fee, eligibility, capacity, status, who_should_choose, features, full_time_courses, fee_items, overrides, nullified_fields) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		10, course.ID, "600000", "Intermediate", 40, "active",
		[]byte("[]"), []byte("[]"), []byte("[]"), []byte("[]"),
		overridesJSON, nullifiedJSON,
	)

	resolved, err := svc.ResolveCourse(course.ID, 10)
	if err != nil {
		t.Fatalf("ResolveCourse() error = %v", err)
	}
	if resolved.Description != "Custom description" {
		t.Errorf("Description = %q, want Custom description", resolved.Description)
	}
}

func TestResolveCourse_NullifiedFields(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	var course Course
	db.Where("title = ?", "BSc Computer Science").First(&course)

	overridesJSON, _ := json.Marshal(CourseOverrides{})
	nullifiedJSON, _ := json.Marshal([]string{"description", "banner_url"})

	db.Exec(`INSERT INTO institution_programs (institution_id, global_course_id, fee, eligibility, capacity, status, who_should_choose, features, full_time_courses, fee_items, overrides, nullified_fields) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		10, course.ID, "500000", "Intermediate", 60, "active",
		[]byte("[]"), []byte("[]"), []byte("[]"), []byte("[]"),
		overridesJSON, nullifiedJSON,
	)

	resolved, err := svc.ResolveCourse(course.ID, 10)
	if err != nil {
		t.Fatalf("ResolveCourse() error = %v", err)
	}
	if resolved.Description != "" {
		t.Errorf("Description = %q, want empty (nullified)", resolved.Description)
	}
	if resolved.BannerURL != "" {
		t.Errorf("BannerURL = %q, want empty (nullified)", resolved.BannerURL)
	}
}

func TestResolveCourse_ProgramNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	var course Course
	db.Where("title = ?", "BSc Computer Science").First(&course)

	_, err := svc.ResolveCourse(course.ID, 999)
	if err == nil {
		t.Error("ResolveCourse() expected error for missing program, got nil")
	}
}

func TestResolveCourse_CourseNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	_, err := svc.ResolveCourse(999, 10)
	if err == nil {
		t.Error("ResolveCourse() expected error for missing course, got nil")
	}
}

func TestGetCoursesByLevel_Delegates(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	courses, total, err := svc.GetCoursesByLevel("Bachelor", 1, 10)
	if err != nil {
		t.Fatalf("GetCoursesByLevel() error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(courses) != 1 {
		t.Errorf("len = %d, want 1", len(courses))
	}
}

func TestGetCoursesByAffiliation_Delegates(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	var aff Affiliation
	db.Where("name = ?", "Tribhuvan University").First(&aff)

	courses, total, err := svc.GetCoursesByAffiliation(aff.ID, 1, 10)
	if err != nil {
		t.Fatalf("GetCoursesByAffiliation() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(courses) != 2 {
		t.Errorf("len = %d, want 2", len(courses))
	}
}

func TestGetSecondaryCourses_Delegates(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	courses, total, err := svc.GetSecondaryCourses(1, 10)
	if err != nil {
		t.Fatalf("GetSecondaryCourses() error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(courses) != 1 {
		t.Errorf("len = %d, want 1", len(courses))
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice []string
		item  string
		want  bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{nil, "a", false},
	}
	for _, tt := range tests {
		got := contains(tt.slice, tt.item)
		if got != tt.want {
			t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.want)
		}
	}
}

func TestBuildCourseResponse_SetsAffiliation(t *testing.T) {
	course := Course{
		ID:       1,
		Title:    "BSc CS",
		Level:    "Bachelor",
		Field:    "Science",
		Duration: "4 years",
	}
	resp := buildCourseResponse(course, 5, "Tribhuvan University")
	if resp.Affiliation != "Tribhuvan University" {
		t.Errorf("Affiliation = %q, want %q", resp.Affiliation, "Tribhuvan University")
	}
	if resp.Colleges != 5 {
		t.Errorf("Colleges = %d, want 5", resp.Colleges)
	}
}

func TestBuildCourseResponse_EmptyAffiliation(t *testing.T) {
	course := Course{ID: 1, Title: "Test", Level: "+2"}
	resp := buildCourseResponse(course, 0, "")
	if resp.Affiliation != "" {
		t.Errorf("Affiliation = %q, want empty", resp.Affiliation)
	}
}

func TestResolveAffiliationName_FromAffiliationID(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	var course Course
	db.Where("title = ?", "BSc Computer Science").First(&course)

	name := svc.resolveAffiliationName(&course)
	if name != "Tribhuvan University" {
		t.Errorf("resolveAffiliationName() = %q, want %q", name, "Tribhuvan University")
	}
}

func TestResolveAffiliationName_NonUniversityAffiliation(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	course := &Course{
		Title:                    "+2 Science",
		Level:                    "+2",
		NonUniversityAffiliation: "NEB",
	}
	name := svc.resolveAffiliationName(course)
	if name != "NEB" {
		t.Errorf("resolveAffiliationName() = %q, want %q", name, "NEB")
	}
}

func TestResolveAffiliationName_NilAffiliationID(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	course := &Course{Title: "Test", Level: "+2"}
	name := svc.resolveAffiliationName(course)
	if name != "" {
		t.Errorf("resolveAffiliationName() = %q, want empty", name)
	}
}

func TestGetEducationCourses_PopulatesAffiliation(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	courses, err := svc.GetEducationCourses()
	if err != nil {
		t.Fatalf("GetEducationCourses() error = %v", err)
	}

	foundTU := false
	foundNEB := false
	for _, c := range courses {
		if c.Title == "BSc Computer Science" && c.Affiliation == "Tribhuvan University" {
			foundTU = true
		}
		if c.Title == "+2 Science" && c.Affiliation == "NEB" {
			foundNEB = true
		}
		if c.Affiliation == "" {
			t.Errorf("course %q has empty affiliation", c.Title)
		}
	}
	if !foundTU {
		t.Error("BSc Computer Science should have affiliation 'Tribhuvan University'")
	}
	if !foundNEB {
		t.Error("+2 Science should have affiliation 'NEB'")
	}
}

func TestGetEducationCourseByID_PopulatesAffiliation(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	var course Course
	db.Where("title = ?", "BSc Computer Science").First(&course)

	resp, err := svc.GetEducationCourseByID(fmt.Sprintf("%d", course.ID))
	if err != nil {
		t.Fatalf("GetEducationCourseByID() error = %v", err)
	}
	if resp.Affiliation != "Tribhuvan University" {
		t.Errorf("Affiliation = %q, want %q", resp.Affiliation, "Tribhuvan University")
	}
}

func TestSearchGlobalCourses_PopulatesAffiliation(t *testing.T) {
	db := setupServiceTestDB(t)
	seedServiceData(db)
	repo := NewRepository(db)
	svc := NewService(repo, nil)

	courses, err := svc.SearchGlobalCourses("")
	if err != nil {
		t.Fatalf("SearchGlobalCourses() error = %v", err)
	}
	if len(courses) == 0 {
		t.Fatal("expected at least one course")
	}
	for _, c := range courses {
		if c.Affiliation == "" {
			t.Errorf("course %q has empty affiliation", c.Title)
		}
	}
}

func TestFindAffiliationsByIDs_ReturnsMap(t *testing.T) {
	db := setupTestDB(t)
	seedCourses(db)
	repo := NewRepository(db)

	var aff1, aff2 Affiliation
	db.Where("name = ?", "Tribhuvan University").First(&aff1)
	db.Where("name = ?", "Kathmandu University").First(&aff2)

	result, err := repo.FindAffiliationsByIDs([]uint{aff1.ID, aff2.ID})
	if err != nil {
		t.Fatalf("FindAffiliationsByIDs() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("len = %d, want 2", len(result))
	}
	if result[aff1.ID].Name != "Tribhuvan University" {
		t.Errorf("aff1 name = %q, want Tribhuvan University", result[aff1.ID].Name)
	}
	if result[aff2.ID].Name != "Kathmandu University" {
		t.Errorf("aff2 name = %q, want Kathmandu University", result[aff2.ID].Name)
	}
}

func TestFindAffiliationsByIDs_EmptyInput(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	result, err := repo.FindAffiliationsByIDs([]uint{})
	if err != nil {
		t.Fatalf("FindAffiliationsByIDs() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}
