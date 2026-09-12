package retrieval

import (
	"errors"
	"testing"
	"time"
)

type countingEmbeddingService struct {
	calls int
	vec   []float32
	err   error
}

func (s *countingEmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
	s.calls++
	return s.vec, s.err
}

func TestCachedEmbeddingReusesRepeatQueries(t *testing.T) {
	svc := &countingEmbeddingService{vec: []float32{0.1, 0.2}}
	r := NewVectorRetriever(nil, svc)

	if _, err := r.cachedEmbedding("colleges"); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if _, err := r.cachedEmbedding("colleges"); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if svc.calls != 1 {
		t.Fatalf("expected 1 embedding call, got %d", svc.calls)
	}
}

func TestCachedEmbeddingExpires(t *testing.T) {
	svc := &countingEmbeddingService{vec: []float32{0.1}}
	r := NewVectorRetriever(nil, svc)
	r.embCache["old"] = embCacheEntry{vec: []float32{0.1}, exp: time.Now().Add(-time.Minute)}

	if _, err := r.cachedEmbedding("old"); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if svc.calls != 1 {
		t.Fatalf("expected refresh after expiry, got %d calls", svc.calls)
	}
}

func TestCachedEmbeddingPropagatesError(t *testing.T) {
	svc := &countingEmbeddingService{err: errors.New("api down")}
	r := NewVectorRetriever(nil, svc)

	if _, err := r.cachedEmbedding("q"); err == nil {
		t.Fatal("expected error, got nil")
	}
	if svc.calls != 1 {
		t.Fatalf("expected 1 embedding call, got %d", svc.calls)
	}
}
