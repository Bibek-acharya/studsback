package studyresources

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
