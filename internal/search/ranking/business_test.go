package ranking

import (
	"testing"

	"studsphere/backend/internal/search/retrieval"
)

func TestBusinessBoost_FreshnessNews(t *testing.T) {
	booster := NewBusinessBooster(BusinessBoostConfig{
		News: EntityBoost{FreshnessBoost: 0.03},
	})

	candidates := []retrieval.Candidate{
		{ID: 1, Type: retrieval.EntityNews, Score: 0.5},
	}

	result := booster.Boost(candidates)
	if result[0].Score != 0.53 {
		t.Errorf("expected 0.53, got %f", result[0].Score)
	}
}

func TestBusinessBoost_NoBoost(t *testing.T) {
	booster := NewBusinessBooster(BusinessBoostConfig{})

	candidates := []retrieval.Candidate{
		{ID: 1, Type: retrieval.EntityCollege, Score: 0.5},
	}

	result := booster.Boost(candidates)
	if result[0].Score != 0.5 {
		t.Errorf("expected 0.5 (unchanged), got %f", result[0].Score)
	}
}
