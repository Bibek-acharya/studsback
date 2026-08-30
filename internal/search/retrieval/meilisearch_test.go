package retrieval

import (
	"context"
	"errors"
	"testing"

	"github.com/meilisearch/meilisearch-go"
)

type fakeIndexClient struct {
	search func(string, *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error)
}

func (f fakeIndexClient) Search(query string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
	return f.search(query, req)
}

type fakeIndexProvider struct {
	client IndexClient
}

func (f fakeIndexProvider) Index(string) IndexClient {
	return f.client
}

func TestSearchIndexUsesStrictMatchingAndScoreThreshold(t *testing.T) {
	client := fakeIndexClient{search: func(_ string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
		if req.MatchingStrategy != meilisearch.All {
			t.Fatalf("expected all-words matching, got %q", req.MatchingStrategy)
		}
		if req.RankingScoreThreshold != lexicalScoreThreshold {
			t.Fatalf("expected threshold %f, got %f", lexicalScoreThreshold, req.RankingScoreThreshold)
		}
		return &meilisearch.SearchResponse{Hits: []interface{}{
			map[string]interface{}{"id": float64(1), "title": "CSIT", "_rankingScore": 0.87},
		}}, nil
	}}
	retriever := NewMeilisearchRetriever(fakeIndexProvider{client: client}, "test_")

	hits, err := retriever.searchIndex(context.Background(), EntityCourse, SearchRequest{Query: "CSIT", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	candidate := hitToCandidate(hits[0], EntityCourse)
	if candidate.LexicalScore != 0.87 {
		t.Fatalf("expected lexical score 0.87, got %f", candidate.LexicalScore)
	}
}

func TestSearchReturnsErrorWhenEveryIndexFails(t *testing.T) {
	client := fakeIndexClient{search: func(string, *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
		return nil, errors.New("index unavailable")
	}}
	retriever := NewMeilisearchRetriever(fakeIndexProvider{client: client}, "test_")

	_, err := retriever.Search(context.Background(), SearchRequest{Query: "engineering", Limit: 10})
	if err == nil {
		t.Fatal("expected retrieval error")
	}
}

func TestSearchIndexUsesGuardedRelaxedFallback(t *testing.T) {
	requests := 0
	client := fakeIndexClient{search: func(_ string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
		requests++
		if requests == 1 {
			return &meilisearch.SearchResponse{}, nil
		}
		if req.MatchingStrategy != meilisearch.Frequency {
			t.Fatalf("expected frequency fallback, got %q", req.MatchingStrategy)
		}
		if req.RankingScoreThreshold != relaxedLexicalScoreThreshold {
			t.Fatalf("expected relaxed threshold %f, got %f", relaxedLexicalScoreThreshold, req.RankingScoreThreshold)
		}
		return &meilisearch.SearchResponse{Hits: []interface{}{
			map[string]interface{}{"id": float64(1), "title": "Computer Engineering", "_rankingScore": 0.6},
		}}, nil
	}}
	retriever := NewMeilisearchRetriever(fakeIndexProvider{client: client}, "test_")

	hits, err := retriever.searchIndex(context.Background(), EntityCourse, SearchRequest{
		Query: "computer science engineering",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || requests != 2 {
		t.Fatalf("expected one fallback hit after two requests, hits=%d requests=%d", len(hits), requests)
	}
}

func TestBuildFiltersOnlyUsesSupportedEntityAttributes(t *testing.T) {
	retriever := NewMeilisearchRetriever(nil, "")
	filters := SearchFilters{Location: "Kathmandu", Type: "course", RatingMin: 4, University: "TU"}

	if got := retriever.buildFilters(EntityCourse, filters); got != "affiliation = \"TU\"" {
		t.Fatalf("unexpected course filters: %q", got)
	}
	if got := retriever.buildFilters(EntityCollege, filters); got == "" {
		t.Fatal("expected supported college filters")
	}
}
