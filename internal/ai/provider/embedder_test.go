package provider

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/config"
)

func TestNewOpenAIEmbedderConfigured(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-test",
		},
		Embedding: config.EmbeddingConfig{
			Model:              "text-embedding-3-small",
			BaseURL:            "https://api.openai.com/v1",
			APIKey:             "***",
			HTTPTimeoutSeconds: 30,
		},
	}
	e := NewOpenAIEmbedder(cfg)
	if e == nil {
		t.Fatal("NewOpenAIEmbedder returned nil")
	}
	if model := e.Model(); model != "text-embedding-3-small" {
		t.Fatalf("Model() = %q, want text-embedding-3-small", model)
	}
}

func TestOpenAIEmbedderModel(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-test",
		},
		Embedding: config.EmbeddingConfig{
			Model:              "text-embedding-3-large",
			BaseURL:            "https://api.openai.com/v1",
			APIKey:             "***",
			HTTPTimeoutSeconds: 30,
		},
	}
	e := NewOpenAIEmbedder(cfg)
	if model := e.Model(); model != "text-embedding-3-large" {
		t.Fatalf("Model() = %q", model)
	}
}

func TestOpenAIEmbedderCloseIdempotent(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-test",
		},
		Embedding: config.EmbeddingConfig{
			Model:              "text-embedding-3-small",
			BaseURL:            "https://api.openai.com/v1",
			APIKey:             "***",
			HTTPTimeoutSeconds: 30,
		},
	}
	e := NewOpenAIEmbedder(cfg)
	if err := e.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := e.Close(); err != nil {
		t.Fatalf("Close second call: %v", err)
	}
}

func TestEmbedEmptyTexts(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-test",
		},
		Embedding: config.EmbeddingConfig{
			Model:              "text-embedding-3-small",
			BaseURL:            "https://api.openai.com/v1",
			APIKey:             "***",
			HTTPTimeoutSeconds: 30,
		},
	}
	e := NewOpenAIEmbedder(cfg)
	result, err := e.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("Embed empty: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for empty texts, got %v", result)
	}

	result, err = e.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Embed empty slice: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for empty slice, got %v", result)
	}
}

func TestMarshalOpenAIEmbeddingRequest(t *testing.T) {
	t.Parallel()
	marshal := marshalOpenAIEmbeddingRequest("text-embedding-3-small")
	body, err := marshal([]string{"hello", "world"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty body")
	}
}

func TestParseOpenAIEmbeddingResponse(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"data": [
			{"index": 0, "embedding": [0.1, 0.2, 0.3]},
			{"index": 1, "embedding": [0.4, 0.5, 0.6]}
		]
	}`)
	result, err := parseOpenAIEmbeddingResponse(body, 2)
	if err != nil {
		t.Fatalf("parseOpenAIEmbeddingResponse: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if len(result[0]) != 3 || len(result[1]) != 3 {
		t.Fatalf("expected 3-dim vectors, got %d and %d", len(result[0]), len(result[1]))
	}
}

func TestParseOpenAIEmbeddingResponseError(t *testing.T) {
	t.Parallel()
	body := []byte(`{"error": {"message": "rate limit exceeded"}}`)
	_, err := parseOpenAIEmbeddingResponse(body, 1)
	if err == nil || err.Error() != "embedding API error: rate limit exceeded" {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}

func TestParseOpenAIEmbeddingResponseInvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := parseOpenAIEmbeddingResponse([]byte(`{bad json}`), 1)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestParseOpenAIEmbeddingResponseOutOfRangeIndex(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"data": [
			{"index": 99, "embedding": [0.1]},
			{"index": -1, "embedding": [0.2]}
		]
	}`)
	result, err := parseOpenAIEmbeddingResponse(body, 2)
	if err != nil {
		t.Fatalf("parseOpenAIEmbeddingResponse: %v", err)
	}
	if result[0] != nil || result[1] != nil {
		t.Fatal("expected both entries to be nil due to out-of-range indices")
	}
}

func TestParseOpenAIResponseError(t *testing.T) {
	t.Parallel()
	_, err := parseOpenAIResponse([]byte(`{"error": {"message": "invalid API key"}}`))
	if err == nil || err.Error() != "API error: invalid API key" {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestParseOpenAIResponseNoChoices(t *testing.T) {
	t.Parallel()
	_, err := parseOpenAIResponse([]byte(`{"choices": []}`))
	if err == nil || err.Error() != "no choices in response" {
		t.Fatalf("expected 'no choices' error, got %v", err)
	}
}

func TestParseOpenAIResponseInvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := parseOpenAIResponse([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestMarshalOpenAIRequest(t *testing.T) {
	t.Parallel()
	c := NewClient(config.AIConfig{
		Connection: config.AIConnectionConfig{
			Provider: "openai",
			APIKey:   "x",
			Model:    "gpt-4o-mini",
			BaseURL:  "https://api.openai.com/v1",
		},
	})
	marshal := c.marshalOpenAIRequest("gpt-4o-mini", 4096)
	body, err := marshal("hello", 0.7)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty body")
	}
}

func TestMarshalAnthropicRequest(t *testing.T) {
	t.Parallel()
	marshal := marshalAnthropicRequest("claude-3-5-sonnet-latest", 4096)
	body, err := marshal("hello", 0.7)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty body")
	}
}

func TestParseAnthropicResponseError(t *testing.T) {
	t.Parallel()
	_, err := parseAnthropicResponse([]byte(`{"error": {"type": "authentication_error", "message": "bad key"}}`))
	if err == nil || err.Error() != "anthropic API error: bad key" {
		t.Fatalf("expected anthropic API error, got %v", err)
	}
}

func TestParseAnthropicResponseInvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := parseAnthropicResponse([]byte(`{bad`))
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestParseAnthropicResponseNoText(t *testing.T) {
	t.Parallel()
	_, err := parseAnthropicResponse([]byte(`{"content": [{"type": "image", "text": ""}]}`))
	if err == nil || err.Error() != "no text content in Anthropic response" {
		t.Fatalf("expected 'no text content' error, got %v", err)
	}
}

func TestParseAnthropicResponseFinishReasonMaxTokens(t *testing.T) {
	t.Parallel()
	gen, err := parseAnthropicResponse([]byte(`{
		"content": [{"type": "text", "text": "hello"}],
		"stop_reason": "max_tokens",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`))
	if err != nil {
		t.Fatalf("parseAnthropicResponse: %v", err)
	}
	if gen.FinishReason != "length" {
		t.Fatalf("FinishReason = %q, want 'length'", gen.FinishReason)
	}
	if gen.Usage == nil || gen.Usage.Prompt != 10 || gen.Usage.Completion != 5 {
		t.Fatalf("usage = %+v", gen.Usage)
	}
}

func TestParseAnthropicResponseFinishReasonStop(t *testing.T) {
	t.Parallel()
	gen, err := parseAnthropicResponse([]byte(`{
		"content": [{"type": "text", "text": "world"}],
		"stop_reason": "end_turn"
	}`))
	if err != nil {
		t.Fatalf("parseAnthropicResponse: %v", err)
	}
	if gen.FinishReason != "stop" {
		t.Fatalf("FinishReason = %q, want 'stop'", gen.FinishReason)
	}
}
