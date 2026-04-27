package embedding

import "github.com/pgvector/pgvector-go"

type Embeddable interface {
	EmbeddingText() string
}

func NewVector(dims int) pgvector.Vector {
	return pgvector.NewVector(make([]float32, dims))
}
