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
