package indexer

import (
	"testing"
)

func TestReindexAll_CreatesIndexes(t *testing.T) {
	if len(IndexConfigs) != 10 {
		t.Errorf("expected 10 index configs, got %d", len(IndexConfigs))
	}
}
