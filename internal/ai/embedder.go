package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/config"
)

// Embedder produces a vector embedding per input text. Implementations are
// expected to preserve order: out[i] corresponds to texts[i].
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
}

// OpenAIEmbedder calls the OpenAI /v1/embeddings endpoint (or any
// OpenAI-compatible host). Default model is text-embedding-3-small (1536
// dims, cheap, good for table-name retrieval).
type OpenAIEmbedder struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
}

// NewOpenAIEmbedder configures an embedder against the AI config's BaseURL +
// APIKey. The embedding model defaults to text-embedding-3-small when
// EmbeddingModel is unset.
func NewOpenAIEmbedder(cfg config.AIConfig) *OpenAIEmbedder {
	baseURL := cfg.BaseURL
	if baseURL == "" && cfg.Provider == "openai" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.EmbeddingModel
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		model:      model,
	}
}

// Model returns the embedding model identifier (e.g. text-embedding-3-small).
// Used to record which model produced a stored vector so we can re-embed when
// the model changes.
func (e *OpenAIEmbedder) Model() string { return e.model }

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
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(openAIEmbeddingRequest{Input: texts, Model: e.model})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embedding request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed openAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s", parsed.Error.Message)
	}
	out := make([][]float32, len(texts))
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
