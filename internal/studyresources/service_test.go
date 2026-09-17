package studyresources

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedResources(db *gorm.DB) {
	resources := []StudyResource{
		{Title: "Physics Past Questions 2081", ResourceType: "Past Questions", Course: "Physics", Year: "2081", FileName: "physics.pdf", FilePath: "study-resources/a.pdf", FileURL: "/uploads/study-resources/a.pdf"},
		{Title: "Math Study Notes", Description: "Calculus notes", ResourceType: "Study Notes", Course: "Mathematics", Year: "2080", FileName: "math.pdf", FilePath: "study-resources/b.pdf", FileURL: "/uploads/study-resources/b.pdf"},
		{Title: "Entrance Model Questions", ResourceType: "Model Questions", Course: "Engineering", Year: "2081", FileName: "model.pdf", FilePath: "study-resources/c.pdf", FileURL: "/uploads/study-resources/c.pdf"},
	}
	for i := range resources {
		db.Create(&resources[i])
	}
}

func TestFindAll_FiltersByType(t *testing.T) {
	db := setupTestDB(t)
	seedResources(db)
	repo := NewRepository(db)

	resources, total, err := repo.FindAll(ResourceFilters{Type: "Past Questions"}, 1, 20)
	if err != nil {
		t.Fatalf("FindAll error: %v", err)
	}
	if total != 1 || len(resources) != 1 {
		t.Fatalf("total=%d len=%d, want 1", total, len(resources))
	}
	if resources[0].ResourceType != "Past Questions" {
		t.Errorf("type = %q", resources[0].ResourceType)
	}
}

func TestFindAll_SearchesTitleDescriptionCourse(t *testing.T) {
	db := setupTestDB(t)
	seedResources(db)
	repo := NewRepository(db)

	// Matches via description
	resources, total, err := repo.FindAll(ResourceFilters{Search: "calculus"}, 1, 20)
	if err != nil {
		t.Fatalf("FindAll error: %v", err)
	}
	if total != 1 || resources[0].Title != "Math Study Notes" {
		t.Fatalf("search by description failed: total=%d", total)
	}

	// Matches via course
	resources, total, err = repo.FindAll(ResourceFilters{Search: "engineer"}, 1, 20)
	if err != nil {
		t.Fatalf("FindAll error: %v", err)
	}
	if total != 1 || resources[0].Course != "Engineering" {
		t.Fatalf("search by course failed: total=%d", total)
	}
}

func TestUpdateResource_PartialUpdate(t *testing.T) {
	db := setupTestDB(t)
	seedResources(db)
	svc := NewService(NewRepository(db))

	db.Model(&StudyResource{}).Order("id ASC").First(&StudyResource{})
	var first StudyResource
	db.First(&first)

	title := "Updated Title"
	updated, err := svc.UpdateResource(first.ID, UpdateResourceRequest{Title: &title})
	if err != nil {
		t.Fatalf("UpdateResource error: %v", err)
	}
	if updated.Title != title {
		t.Errorf("title = %q, want %q", updated.Title, title)
	}
	if updated.ResourceType != first.ResourceType {
		t.Errorf("other fields should be unchanged, got type %q", updated.ResourceType)
	}
}

func TestIncrementDownloads(t *testing.T) {
	db := setupTestDB(t)
	seedResources(db)
	repo := NewRepository(db)

	var first StudyResource
	db.First(&first)

	if err := repo.IncrementDownloads(first.ID); err != nil {
		t.Fatalf("IncrementDownloads error: %v", err)
	}
	if err := repo.IncrementDownloads(first.ID); err != nil {
		t.Fatalf("IncrementDownloads error: %v", err)
	}

	var refreshed StudyResource
	db.First(&refreshed, first.ID)
	if refreshed.Downloads != 2 {
		t.Errorf("downloads = %d, want 2", refreshed.Downloads)
	}
}

func TestDeleteResource_SoftDeletes(t *testing.T) {
	db := setupTestDB(t)
	seedResources(db)
	svc := NewService(NewRepository(db))

	var first StudyResource
	db.First(&first)

	titles, total, err := svc.GetResources(ResourceFilters{}, 1, 20)
	if err != nil || total != 3 || len(titles) != 3 {
		t.Fatalf("pre-delete list: total=%d len=%d err=%v", total, len(titles), err)
	}

	if err := svc.DeleteResource(first.ID); err != nil {
		t.Fatalf("DeleteResource error: %v", err)
	}

	_, total, err = svc.GetResources(ResourceFilters{}, 1, 20)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if total != 2 {
		t.Errorf("total after delete = %d, want 2", total)
	}
}

func TestDistinctFacets(t *testing.T) {
	db := setupTestDB(t)
	seedResources(db)
	// A soft-deleted resource with unique year/course must be excluded.
	db.Create(&StudyResource{Title: "Archived", ResourceType: "Notes", Course: "Unique Course", Year: "2079", FileName: "f.pdf", FilePath: "study-resources/z.pdf", FileURL: "/uploads/study-resources/z.pdf", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}})
	// A blank course/year must be excluded.
	db.Create(&StudyResource{Title: "No Facets", ResourceType: "Notes", FileName: "f.pdf", FilePath: "study-resources/y.pdf", FileURL: "/uploads/study-resources/y.pdf"})

	svc := NewService(NewRepository(db))

	years, courses, err := svc.DistinctFacets()
	if err != nil {
		t.Fatalf("DistinctFacets error: %v", err)
	}

	wantYears := []string{"2081", "2080"}
	if len(years) != len(wantYears) {
		t.Fatalf("years = %v, want %v", years, wantYears)
	}
	for i, y := range wantYears {
		if years[i] != y {
			t.Errorf("years[%d] = %q, want %q", i, years[i], y)
		}
	}

	wantCourses := []string{"Engineering", "Mathematics", "Physics"}
	if len(courses) != len(wantCourses) {
		t.Fatalf("courses = %v, want %v", courses, wantCourses)
	}
	for i, c := range wantCourses {
		if courses[i] != c {
			t.Errorf("courses[%d] = %q, want %q", i, courses[i], c)
		}
	}
}

func TestReplaceResourceFile_MetadataUpdate(t *testing.T) {
	db := setupTestDB(t)
	seedResources(db)
	svc := NewService(NewRepository(db))

	var first StudyResource
	db.First(&first)

	resource, err := svc.GetResource(first.ID)
	if err != nil {
		t.Fatalf("GetResource error: %v", err)
	}

	resource.FileName = "new-name.docx"
	resource.FilePath = "study-resources/new-key-abc.docx"
	resource.FileURL = "/uploads/study-resources/new-key-abc.docx"
	resource.FileSize = 12345
	resource.MimeType = "application/msword"

	if err := svc.UpdateResourceModel(resource); err != nil {
		t.Fatalf("UpdateResourceModel error: %v", err)
	}

	refreshed, err := svc.GetResource(first.ID)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if refreshed.FileName != "new-name.docx" ||
		refreshed.FilePath != "study-resources/new-key-abc.docx" ||
		refreshed.FileURL != "/uploads/study-resources/new-key-abc.docx" ||
		refreshed.FileSize != 12345 ||
		refreshed.MimeType != "application/msword" {
		t.Errorf("file metadata not persisted: %+v", refreshed)
	}

	// Soft-delete must not occur as a side effect of the replace.
	_, total, err := svc.GetResources(ResourceFilters{}, 1, 20)
	if err != nil || total != 3 {
		t.Errorf("post-replace total = %d (err %v), want 3", total, err)
	}
}

func TestNormalizeObjectKey(t *testing.T) {
	cases := map[string]string{
		"study-resources/a.pdf":  "study-resources/a.pdf",
		"/uploads/study/a.png":   "study/a.png",
		"/uploads/uploads/b.txt": "uploads/b.txt",
	}
	for in, want := range cases {
		if got := normalizeObjectKey(in); got != want {
			t.Errorf("normalizeObjectKey(%q) = %q, want %q", in, got, want)
		}
	}
}
