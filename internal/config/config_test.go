package config

import (
	"testing"
	"time"
)

func TestAIConfig_EffectiveEmbeddingAPIKey(t *testing.T) {
	c := AIConfig{APIKey: "main", EmbeddingAPIKey: "  emb  "}
	if got := c.EffectiveEmbeddingAPIKey(); got != "emb" {
		t.Fatalf("want dedicated trimmed key, got %q", got)
	}
	c2 := AIConfig{APIKey: "only"}
	if got := c2.EffectiveEmbeddingAPIKey(); got != "only" {
		t.Fatalf("want fallback to main, got %q", got)
	}
}

func TestAIConfig_AIHTTPTimeout(t *testing.T) {
	c := AIConfig{HTTPTimeoutSeconds: 12}
	if got := c.AIHTTPTimeout(); got != 12*time.Second {
		t.Fatalf("configured timeout: got %s", got)
	}
	if got := (AIConfig{}).AIHTTPTimeout(); got != 300*time.Second {
		t.Fatalf("default timeout: got %s", got)
	}
	if got := (AIConfig{HTTPTimeoutSeconds: -1}).AIHTTPTimeout(); got != 300*time.Second {
		t.Fatalf("non-positive timeout should default: got %s", got)
	}
}

func TestAIConfig_EffectiveEmbeddingBaseURL(t *testing.T) {
	c := AIConfig{EmbeddingBaseURL: "https://embed.example/v1/"}
	if got := c.EffectiveEmbeddingBaseURL(); got != "https://embed.example/v1" {
		t.Fatalf("trim: got %q", got)
	}
	c2 := AIConfig{BaseURL: "https://chat.example/v1", Provider: "openai"}
	if got := c2.EffectiveEmbeddingBaseURL(); got != "https://chat.example/v1" {
		t.Fatalf("fallback to BaseURL: got %q", got)
	}
	c3 := AIConfig{Provider: "openai"}
	if got := c3.EffectiveEmbeddingBaseURL(); got != "https://api.openai.com/v1" {
		t.Fatalf("openai default: got %q", got)
	}
}

func TestAIConfig_EmbeddingsConfigured(t *testing.T) {
	if (AIConfig{}).EmbeddingsConfigured() {
		t.Fatal("empty config should not enable embeddings")
	}
	ok := AIConfig{
		Provider:       "openai",
		APIKey:         "k",
		EmbeddingModel: "text-embedding-3-small",
	}
	if !ok.EmbeddingsConfigured() {
		t.Fatal("model + key + default openai base should enable")
	}
	noURL := AIConfig{
		Provider:       "anthropic",
		APIKey:         "k",
		BaseURL:        "",
		EmbeddingModel: "x",
	}
	if noURL.EmbeddingsConfigured() {
		t.Fatal("anthropic without any base URL should not enable (no default host for embeddings)")
	}
}
