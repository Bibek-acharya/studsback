package indexer

import (
	"testing"
	"time"
)

func TestSyncCursor_AdvancesOnSuccess(t *testing.T) {
	cursor := &SyncCursor{
		Table:      "colleges",
		LastSyncAt: time.Now().Add(-1 * time.Hour),
		LastSyncID: 0,
		BatchSize:  100,
	}

	lastRow := struct {
		UpdatedAt time.Time
		ID        uint
	}{
		UpdatedAt: time.Now(),
		ID:        42,
	}

	cursor.Advance(lastRow.UpdatedAt, lastRow.ID)

	if cursor.LastSyncID != 42 {
		t.Errorf("expected LastSyncID 42, got %d", cursor.LastSyncID)
	}
}

func TestSyncCursor_DoesNotAdvanceOnFailure(t *testing.T) {
	cursor := &SyncCursor{
		Table:      "colleges",
		LastSyncAt: time.Now().Add(-1 * time.Hour),
		LastSyncID: 0,
		BatchSize:  100,
	}

	if cursor.LastSyncID != 0 {
		t.Errorf("expected LastSyncID 0 (unchanged), got %d", cursor.LastSyncID)
	}
}

func TestIsPublished_UniversityRequiresPublishedStatus(t *testing.T) {
	table := syncTable{Name: "universities", StatusFilter: "status = 'published'"}
	if !isPublished(map[string]interface{}{"status": "published"}, table) {
		t.Fatal("expected published university to be indexed")
	}
	if isPublished(map[string]interface{}{"status": "draft"}, table) {
		t.Fatal("expected draft university to be excluded")
	}
}

func TestIsPublished_InstitutionRequiresApprovalAndPublishedProfile(t *testing.T) {
	table := syncTable{
		Name:         "institution_users",
		StatusFilter: "profile_status = 'published' AND status = 'approved'",
	}

	if !isPublished(map[string]interface{}{
		"status":         "approved",
		"profile_status": "published",
	}, table) {
		t.Fatal("expected approved published institution to be indexed")
	}
	if isPublished(map[string]interface{}{
		"status":         "pending",
		"profile_status": "published",
	}, table) {
		t.Fatal("expected pending institution to be excluded")
	}
}

func TestIsPublished_NewsUsesBooleanPublishedField(t *testing.T) {
	table := syncTable{Name: "news", StatusFilter: "published = true"}
	if !isPublished(map[string]interface{}{"published": true}, table) {
		t.Fatal("expected published news to be indexed")
	}
	if isPublished(map[string]interface{}{"published": false}, table) {
		t.Fatal("expected unpublished news to be excluded")
	}
}

func TestIsPublished_EventExcludesCompletedAndExpiredRecords(t *testing.T) {
	table := syncTable{Name: "events", StatusFilter: "status and end-date filter"}
	if isPublished(map[string]interface{}{"status": "completed"}, table) {
		t.Fatal("expected completed event to be excluded")
	}
	if isPublished(map[string]interface{}{
		"status":   "upcoming",
		"end_date": time.Now().Add(-time.Hour),
	}, table) {
		t.Fatal("expected expired event to be excluded")
	}
	if !isPublished(map[string]interface{}{
		"status":   "upcoming",
		"end_date": time.Now().Add(time.Hour),
	}, table) {
		t.Fatal("expected upcoming event to be indexed")
	}
}
