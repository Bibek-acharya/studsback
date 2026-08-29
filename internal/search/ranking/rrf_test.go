package ranking

import (
	"testing"

	"studsphere/backend/internal/search/retrieval"
)

func TestRRFScore_BothSources(t *testing.T) {
	scorer := NewRRFScorer(60)
	score := scorer.Score([]int{1, 3})
	expected := 1.0/61 + 1.0/63
	if score != expected {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

func TestRRFScore_SingleSource(t *testing.T) {
	scorer := NewRRFScorer(60)
	score := scorer.Score([]int{5})
	expected := 1.0 / 65
	if score != expected {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

func TestRRFScore_NoRanks(t *testing.T) {
	scorer := NewRRFScorer(60)
	score := scorer.Score([]int{})
	if score != 0 {
		t.Errorf("expected 0, got %f", score)
	}
}

func TestRankCandidates_DualSource(t *testing.T) {
	scorer := NewRRFScorer(60)
	candidates := []retrieval.Candidate{
		{ID: 1, Type: retrieval.EntityCollege, Title: "A"},
		{ID: 2, Type: retrieval.EntityCollege, Title: "B"},
		{ID: 3, Type: retrieval.EntityCourse, Title: "C"},
	}
	// ID 1 appears in both sources (rank 1 from meili, rank 3 from vector)
	// ID 2 appears only in meili (rank 5)
	// ID 3 appears only in vector (rank 2)
	sourceRanks := map[string][]int{
		"college:1": {1, 3},
		"college:2": {5},
		"course:3":  {2},
	}

	ranked := scorer.RankCandidates(candidates, sourceRanks)
	if len(ranked) != 3 {
		t.Fatalf("expected 3, got %d", len(ranked))
	}
	// ID 1: 1/61 + 1/63 ≈ 0.0322
	// ID 3: 1/62 ≈ 0.0161
	// ID 2: 1/65 ≈ 0.0154
	if ranked[0].ID != 1 {
		t.Errorf("expected ID 1 first, got ID %d", ranked[0].ID)
	}
	if ranked[1].ID != 3 {
		t.Errorf("expected ID 3 second, got ID %d", ranked[1].ID)
	}
}

func TestRankCandidates_SingleSource(t *testing.T) {
	scorer := NewRRFScorer(60)
	candidates := []retrieval.Candidate{
		{ID: 1, Type: retrieval.EntityCollege, Title: "A"},
		{ID: 2, Type: retrieval.EntityCollege, Title: "B"},
	}
	sourceRanks := map[string][]int{
		"college:1": {1},
		"college:2": {5},
	}

	ranked := scorer.RankCandidates(candidates, sourceRanks)
	if ranked[0].ID != 1 {
		t.Errorf("expected ID 1 first, got ID %d", ranked[0].ID)
	}
}
