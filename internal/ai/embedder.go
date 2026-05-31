package ai

import (
	"context"
)

// Embedder produces a vector embedding per input text. Implementations are
// expected to preserve order: out[i] corresponds to texts[i].
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
}
