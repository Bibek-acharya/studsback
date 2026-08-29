package indexer

import (
	"testing"
)

func TestReindexAll_CreatesIndexes(t *testing.T) {
	if len(IndexConfigs) != 11 {
		t.Errorf("expected 11 index configs, got %d", len(IndexConfigs))
	}
}
