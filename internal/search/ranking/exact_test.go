package ranking

import (
	"testing"

	"studsphere/backend/internal/search/retrieval"
)

func TestExactMatch_TitleExact(t *testing.T) {
	m := NewExactMatcher(ExactMatchConfig{
		TitleExact:  0.1,
		TitlePhrase: 0.05,
		TitlePrefix: 0.03,
		TokenMatch:  0.01,
		DescMatch:   0.005,
	})

	candidates := []retrieval.Candidate{
		{ID: 1, Title: "Kathmandu University", Score: 0.5},
		{ID: 2, Title: "Tribhuvan University", Score: 0.5},
	}

	result := m.Boost(candidates, "Kathmandu University")
	if result[0].ID != 1 {
		t.Errorf("expected ID 1 first, got ID %d", result[0].ID)
	}
	if result[0].Score != 0.6 {
		t.Errorf("expected score 0.6, got %f", result[0].Score)
	}
}

func TestExactMatch_TitlePrefix(t *testing.T) {
	m := NewExactMatcher(ExactMatchConfig{TitlePrefix: 0.03})

	candidates := []retrieval.Candidate{
		{ID: 1, Title: "Kathmandu University School of Engineering", Score: 0.5},
	}

	result := m.Boost(candidates, "Kathmandu")
	if result[0].Score != 0.53 {
		t.Errorf("expected 0.53, got %f", result[0].Score)
	}
}

func TestExactMatch_NoMatch(t *testing.T) {
	m := NewExactMatcher(ExactMatchConfig{TitleExact: 0.1})

	candidates := []retrieval.Candidate{
		{ID: 1, Title: "Something Else", Score: 0.5},
	}

	result := m.Boost(candidates, "Kathmandu")
	if result[0].Score != 0.5 {
		t.Errorf("expected 0.5 (unchanged), got %f", result[0].Score)
	}
}
