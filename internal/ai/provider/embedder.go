package provider

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/config"
)

// OpenAIEmbedder calls the OpenAI /v1/embeddings endpoint (or any
// OpenAI-compatible host). The model name comes from BI_AI_EMBEDDING_MODEL.
type OpenAIEmbedder struct {
	base baseEmbedder
}

// NewOpenAIEmbedder configures an embedder using BI_AI_EMBEDDING_* with
// fallback to the main LLM BaseURL/APIKey. Call only when ResolvedEmbedding().Configured().
func NewOpenAIEmbedder(cfg config.AIConfig) *OpenAIEmbedder {
	emb := cfg.ResolvedEmbedding()
	http := newHTTPProvider(emb.HTTPTimeout, emb.BaseURL, emb.APIKey)
	return &OpenAIEmbedder{
		base: baseEmbedder{
			http:  http,
			model: emb.Model,
			hooks: embeddingHooks{
				path:    "/embeddings",
				headers: func(p httpProvider) map[string]string { return p.bearerAuthHeaders() },
				marshal: marshalOpenAIEmbeddingRequest(emb.Model),
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
		return sonic.ConfigStd.Marshal(openAIEmbeddingRequest{Input: texts, Model: model})
	}
}

func parseOpenAIEmbeddingResponse(body []byte, count int) ([][]float32, error) {
	var parsed openAIEmbeddingResponse
	if err := sonic.ConfigStd.Unmarshal(body, &parsed); err != nil {
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
