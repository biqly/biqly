package routing

import (
	"context"
	"math"
)

// Embedder produces a vector embedding per input text. The router only needs
// these two methods; it declares the interface locally (structural typing) so
// the routing package does not depend on the concrete embedder implementation.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
}

// CosineSimilarity returns cosine similarity in [-1, 1]. Returns 0 if either
// vector is empty or zero-norm so callers can treat it as a no-op contribution.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
