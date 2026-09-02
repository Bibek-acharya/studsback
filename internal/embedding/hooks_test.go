package embedding

import (
	"testing"

	"studsphere/backend/internal/shared/config"
)

func resetQueue() {
	for {
		select {
		case <-queue:
		default:
			queued.Range(func(k, _ any) bool {
				queued.Delete(k)
				return true
			})
			return
		}
	}
}

func TestEnqueueUpdate_DedupAndTableGate(t *testing.T) {
	prev := config.AppConfig
	config.AppConfig = &config.Config{EmbeddingEnabled: true}
	defer func() { config.AppConfig = prev }()
	resetQueue()
	defer resetQueue()

	EnqueueUpdate("users", 1)

	EnqueueUpdate("courses", 27)
	EnqueueUpdate("courses", 27)
	EnqueueUpdate("courses", 27)
	EnqueueUpdate("courses", 28)
	EnqueueUpdate("colleges", 5)

	if n := len(queue); n != 3 {
		t.Fatalf("queue length = %d, want 3 (users gated out, 3 deduped course/college jobs)", n)
	}
	if _, ok := queued.Load("courses:27"); !ok {
		t.Fatal("dedup key courses:27 missing")
	}
	if _, ok := queued.Load("users:1"); ok {
		t.Fatal("non-embedding table users must not be queued")
	}

	config.AppConfig.EmbeddingEnabled = false
	EnqueueUpdate("courses", 99)
	if _, ok := queued.Load("courses:99"); ok {
		t.Fatal("disabled embedding service must not enqueue")
	}
}
