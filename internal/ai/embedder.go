package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/biqly/biqly/internal/config"
)

// Embedder produces a vector embedding per input text. Implementations are
// expected to preserve order: out[i] corresponds to texts[i].
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
}

// OpenAIEmbedder calls the OpenAI /v1/embeddings endpoint (or any
// OpenAI-compatible host). The model name comes from BI_AI_EMBEDDING_MODEL.
type OpenAIEmbedder struct {
	base baseEmbedder
}

// NewOpenAIEmbedder configures an embedder using BI_AI_EMBEDDING_* with
// fallback to the main LLM BaseURL/APIKey. Call only when EmbeddingsConfigured().
func NewOpenAIEmbedder(cfg config.AIConfig) *OpenAIEmbedder {
	model := strings.TrimSpace(cfg.EmbeddingModel)
	http := newHTTPProvider(cfg.EmbeddingHTTPTimeout(), cfg.EffectiveEmbeddingBaseURL(), cfg.EffectiveEmbeddingAPIKey())
	return &OpenAIEmbedder{
		base: baseEmbedder{
			http:  http,
			model: model,
			hooks: embeddingHooks{
				path:    "/embeddings",
				headers: func(p httpProvider) map[string]string { return p.bearerAuthHeaders() },
				marshal: marshalOpenAIEmbeddingRequest(model),
				parse:   parseOpenAIEmbeddingResponse,
			},
		},
	}
}

// Model returns the embedding model identifier (e.g. text-embedding-3-small).
func (e *OpenAIEmbedder) Model() string { return e.base.model }

// Close closes idle HTTP keepalive connections held by the embedder.
func (e *OpenAIEmbedder) Close() error {
	return e.base.http.Close()
}

type openAIEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed returns one embedding vector per input text. Order is preserved.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return e.base.embed(ctx, texts)
}

func marshalOpenAIEmbeddingRequest(model string) func([]string) ([]byte, error) {
	return func(texts []string) ([]byte, error) {
		return json.Marshal(openAIEmbeddingRequest{Input: texts, Model: model})
	}
}

func parseOpenAIEmbeddingResponse(body []byte, count int) ([][]float32, error) {
	var parsed openAIEmbeddingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s", parsed.Error.Message)
	}
	out := make([][]float32, count)
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(out) {
			continue
		}
		out[d.Index] = d.Embedding
	}
	return out, nil
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
