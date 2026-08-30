package search

import (
	"testing"

	"studsphere/backend/internal/search/retrieval"
)

func TestFilterRelevantCandidates(t *testing.T) {
	candidates := []retrieval.Candidate{
		{ID: 1, LexicalScore: 0.19},
		{ID: 2, LexicalScore: 0.20},
		{ID: 3, VectorScore: 0.55},
		{ID: 4, VectorScore: 0.54},
	}

	filtered := filterRelevantCandidates(candidates, "engineering")
	if len(filtered) != 2 || filtered[0].ID != 2 || filtered[1].ID != 3 {
		t.Fatalf("unexpected relevant candidates: %#v", filtered)
	}
}

func TestFilterRelevantCandidates_AllowsStructuredBrowse(t *testing.T) {
	candidates := []retrieval.Candidate{{ID: 1}, {ID: 2}}
	filtered := filterRelevantCandidates(candidates, "")
	if len(filtered) != len(candidates) {
		t.Fatalf("expected all browse candidates, got %d", len(filtered))
	}
}

func TestMergeCandidatesCombinesRetrieverEvidence(t *testing.T) {
	meili := []retrieval.Candidate{
		{ID: 1, Type: retrieval.EntityCollege, LexicalScore: 0.8},
	}
	vector := []retrieval.Candidate{
		{ID: 1, Type: retrieval.EntityCollege, VectorScore: 0.7},
	}

	merged := mergeCandidates(meili, vector)
	if len(merged) != 1 {
		t.Fatalf("expected one merged candidate, got %d", len(merged))
	}
	if merged[0].LexicalScore != 0.8 || merged[0].VectorScore != 0.7 {
		t.Fatalf("retrieval evidence was not combined: %#v", merged[0])
	}
}

func TestDeduplicateCandidatesCollapsesCollegeAndInstitutionProfiles(t *testing.T) {
	candidates := []retrieval.Candidate{
		{ID: 1, Type: retrieval.EntityInstitution, Title: "  Example College ", Location: "Kathmandu"},
		{ID: 2, Type: retrieval.EntityCollege, Title: "example   college", Location: "kathmandu"},
		{ID: 3, Type: retrieval.EntityCollege, Title: "Example College", Location: "Pokhara"},
	}

	deduplicated := deduplicateCandidates(candidates)
	if len(deduplicated) != 2 {
		t.Fatalf("expected two distinct organizations, got %d", len(deduplicated))
	}
}
