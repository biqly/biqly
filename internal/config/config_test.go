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

func TestAIConfig_EmbeddingHTTPTimeout(t *testing.T) {
	c := AIConfig{HTTPTimeoutSeconds: 12, EmbeddingHTTPTimeoutSeconds: 45}
	if got := c.EmbeddingHTTPTimeout(); got != 45*time.Second {
		t.Fatalf("configured embedding timeout: got %s", got)
	}
	fallback := AIConfig{HTTPTimeoutSeconds: 12}
	if got := fallback.EmbeddingHTTPTimeout(); got != 12*time.Second {
		t.Fatalf("embedding timeout should fall back to AI timeout: got %s", got)
	}
	if got := (AIConfig{}).EmbeddingHTTPTimeout(); got != 300*time.Second {
		t.Fatalf("default embedding timeout: got %s", got)
	}
}

func TestAIConfig_TranslationHTTPTimeout(t *testing.T) {
	c := AIConfig{HTTPTimeoutSeconds: 12, TranslationHTTPTimeoutSeconds: 45}
	if got := c.TranslationHTTPTimeout(); got != 45*time.Second {
		t.Fatalf("configured translation timeout: got %s", got)
	}
	fallback := AIConfig{HTTPTimeoutSeconds: 12}
	if got := fallback.TranslationHTTPTimeout(); got != 12*time.Second {
		t.Fatalf("translation timeout should fall back to AI timeout: got %s", got)
	}
	if got := (AIConfig{}).TranslationHTTPTimeout(); got != 120*time.Second {
		t.Fatalf("default translation timeout: got %s", got)
	}
}

func TestAIConfig_AIRequestTimeout(t *testing.T) {
	c := AIConfig{HTTPTimeoutSeconds: 12, EmbeddingHTTPTimeoutSeconds: 45, TranslationHTTPTimeoutSeconds: 60}
	if got := c.AIRequestTimeout(); got != 90*time.Second {
		t.Fatalf("request timeout should include the largest AI subrequest timeout plus buffer: got %s", got)
	}
	chatOnly := AIConfig{HTTPTimeoutSeconds: 12}
	if got := chatOnly.AIRequestTimeout(); got != 42*time.Second {
		t.Fatalf("request timeout should include chat timeout plus buffer: got %s", got)
	}
}

func TestAIConfig_EffectiveTranslationConfig(t *testing.T) {
	c := AIConfig{
		APIKey:             "main",
		BaseURL:            "https://chat.example/v1/",
		TranslationModel:   "translategemma:4b",
		TranslationBaseURL: "https://translate.example/v1/",
		TranslationAPIKey:  "  translate  ",
	}
	if got := c.EffectiveTranslationAPIKey(); got != "translate" {
		t.Fatalf("want dedicated trimmed key, got %q", got)
	}
	if got := c.EffectiveTranslationBaseURL(); got != "https://translate.example/v1" {
		t.Fatalf("want dedicated trimmed base URL, got %q", got)
	}
	if !c.TranslationConfigured() {
		t.Fatal("translation model plus base URL should enable translation")
	}

	fallback := AIConfig{APIKey: "main", BaseURL: "https://chat.example/v1", TranslationModel: "x"}
	if got := fallback.EffectiveTranslationAPIKey(); got != "main" {
		t.Fatalf("want main key fallback, got %q", got)
	}
	if got := fallback.EffectiveTranslationBaseURL(); got != "https://chat.example/v1" {
		t.Fatalf("want main base URL fallback, got %q", got)
	}
	if !fallback.TranslationConfigured() {
		t.Fatal("translation should use BI_AI_BASE_URL when dedicated URL is empty")
	}

	if (AIConfig{TranslationModel: "x"}).TranslationConfigured() {
		t.Fatal("translation model without any base URL should not enable translation")
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

func TestAIConfig_EffectiveQueryConfigOverrides(t *testing.T) {
	base := AIConfig{
		Provider:           "openai-compatible",
		APIKey:             "ollama",
		BaseURL:            "http://local/v1",
		Model:              "gemma4:e4b",
		HTTPTimeoutSeconds: 300,
	}
	t.Run("no overrides reuses base", func(t *testing.T) {
		got := base.EffectiveQueryConfig()
		if got.Model != "gemma4:e4b" || got.BaseURL != "http://local/v1" || got.APIKey != "ollama" {
			t.Errorf("expected base fields preserved, got %+v", got)
		}
		if base.HasQueryOverride() {
			t.Error("HasQueryOverride should be false for empty overrides")
		}
	})

	t.Run("model override only", func(t *testing.T) {
		c := base
		c.QueryModel = "gpt-4o"
		if !c.HasQueryOverride() {
			t.Error("HasQueryOverride should be true when QueryModel set")
		}
		got := c.EffectiveQueryConfig()
		if got.Model != "gpt-4o" {
			t.Errorf("model not overridden: %q", got.Model)
		}
		if got.BaseURL != base.BaseURL {
			t.Errorf("base URL must fall back when not overridden: %q", got.BaseURL)
		}
	})

	t.Run("full provider swap", func(t *testing.T) {
		c := base
		c.QueryProvider = "openai"
		c.QueryModel = "gpt-4o"
		c.QueryBaseURL = "https://api.openai.com/v1"
		c.QueryAPIKey = "sk-xxx"
		c.QueryHTTPTimeoutSeconds = 60
		got := c.EffectiveQueryConfig()
		if got.Provider != "openai" || got.Model != "gpt-4o" || got.BaseURL != "https://api.openai.com/v1" || got.APIKey != "sk-xxx" || got.HTTPTimeoutSeconds != 60 {
			t.Errorf("expected full override, got %+v", got)
		}
	})
}

func TestAIConfig_QueryLLMConfigured(t *testing.T) {
	if (AIConfig{}).QueryLLMConfigured() {
		t.Fatal("empty config should not enable query LLM")
	}
	if (AIConfig{Model: "gpt-4o"}).QueryLLMConfigured() {
		t.Fatal("openai without key or base URL should not enable")
	}
	local := AIConfig{
		Provider: "openai-compatible",
		BaseURL:  "http://127.0.0.1:12434/v1",
		Model:    "local",
	}
	if !local.QueryLLMConfigured() {
		t.Fatal("keyless local openai-compatible should enable when base URL set")
	}
	withKey := AIConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4o",
	}
	if !withKey.QueryLLMConfigured() {
		t.Fatal("api key + model should enable")
	}
	if (AIConfig{
		Provider: "anthropic",
		Model:    "claude-3",
	}).QueryLLMConfigured() {
		t.Fatal("anthropic without API key should not enable")
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
