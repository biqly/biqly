package config

import (
	"testing"
	"time"
)

func conn(fields ...any) AIConnectionConfig {
	c := AIConnectionConfig{}
	for i := 0; i+1 < len(fields); i += 2 {
		k, ok := fields[i].(string)
		if !ok {
			continue
		}
		switch k {
		case "provider":
			if v, ok := fields[i+1].(string); ok {
				c.Provider = v
			}
		case "api_key":
			if v, ok := fields[i+1].(string); ok {
				c.APIKey = v
			}
		case "base_url":
			if v, ok := fields[i+1].(string); ok {
				c.BaseURL = v
			}
		case "model":
			if v, ok := fields[i+1].(string); ok {
				c.Model = v
			}
		case "http_timeout":
			if v, ok := fields[i+1].(int); ok {
				c.HTTPTimeoutSeconds = v
			}
		}
	}
	return c
}

func TestAIConfig_EmbeddingAPIKey(t *testing.T) {
	c := AIConfig{
		Connection: conn("api_key", "main"),
		Embedding:  EmbeddingConfig{APIKey: "  emb  "},
	}
	if got := c.ResolvedEmbedding().APIKey; got != "emb" {
		t.Fatalf("want dedicated trimmed key, got %q", got)
	}
	c2 := AIConfig{Connection: conn("api_key", "only")}
	if got := c2.ResolvedEmbedding().APIKey; got != "only" {
		t.Fatalf("want fallback to main, got %q", got)
	}
}

func TestAIConfig_HTTPTimeout(t *testing.T) {
	c := AIConfig{Connection: AIConnectionConfig{HTTPTimeoutSeconds: 12}}
	if got := c.HTTPTimeout(); got != 12*time.Second {
		t.Fatalf("configured timeout: got %s", got)
	}
	if got := (AIConfig{}).HTTPTimeout(); got != 300*time.Second {
		t.Fatalf("default timeout: got %s", got)
	}
	if got := (AIConfig{Connection: AIConnectionConfig{HTTPTimeoutSeconds: -1}}).HTTPTimeout(); got != 300*time.Second {
		t.Fatalf("non-positive timeout should default: got %s", got)
	}
}

func TestAIConfig_EmbeddingHTTPTimeout(t *testing.T) {
	c := AIConfig{
		Connection: AIConnectionConfig{HTTPTimeoutSeconds: 12},
		Embedding:  EmbeddingConfig{HTTPTimeoutSeconds: 45},
	}
	if got := c.ResolvedEmbedding().HTTPTimeout; got != 45*time.Second {
		t.Fatalf("configured embedding timeout: got %s", got)
	}
	fallback := AIConfig{Connection: AIConnectionConfig{HTTPTimeoutSeconds: 12}}
	if got := fallback.ResolvedEmbedding().HTTPTimeout; got != 12*time.Second {
		t.Fatalf("embedding timeout should fall back to AI timeout: got %s", got)
	}
	if got := (AIConfig{}).ResolvedEmbedding().HTTPTimeout; got != 300*time.Second {
		t.Fatalf("default embedding timeout: got %s", got)
	}
}

func TestAIConfig_TranslationHTTPTimeout(t *testing.T) {
	c := AIConfig{
		Connection:  AIConnectionConfig{HTTPTimeoutSeconds: 12},
		Translation: TranslationConfig{HTTPTimeoutSeconds: 45},
	}
	if got := c.ResolvedTranslation().HTTPTimeout; got != 45*time.Second {
		t.Fatalf("configured translation timeout: got %s", got)
	}
	fallback := AIConfig{Connection: AIConnectionConfig{HTTPTimeoutSeconds: 12}}
	if got := fallback.ResolvedTranslation().HTTPTimeout; got != 12*time.Second {
		t.Fatalf("translation timeout should fall back to AI timeout: got %s", got)
	}
	if got := (AIConfig{}).ResolvedTranslation().HTTPTimeout; got != 120*time.Second {
		t.Fatalf("default translation timeout: got %s", got)
	}
}

func TestAIConfig_RequestTimeout(t *testing.T) {
	c := AIConfig{
		Connection:  AIConnectionConfig{HTTPTimeoutSeconds: 12},
		Embedding:   EmbeddingConfig{HTTPTimeoutSeconds: 45},
		Translation: TranslationConfig{HTTPTimeoutSeconds: 60},
	}
	if got := c.RequestTimeout(); got != 90*time.Second {
		t.Fatalf("request timeout should include the largest AI subrequest timeout plus buffer: got %s", got)
	}
	chatOnly := AIConfig{Connection: AIConnectionConfig{HTTPTimeoutSeconds: 12}}
	if got := chatOnly.RequestTimeout(); got != 42*time.Second {
		t.Fatalf("request timeout should include chat timeout plus buffer: got %s", got)
	}
}

func TestAIConfig_TranslationView(t *testing.T) {
	c := AIConfig{
		Connection: conn("api_key", "main", "base_url", "https://chat.example/v1/"),
		Translation: TranslationConfig{
			Model:   "translategemma:4b",
			BaseURL: "https://translate.example/v1/",
			APIKey:  "  translate  ",
		},
	}
	tr := c.ResolvedTranslation()
	if got := tr.APIKey; got != "translate" {
		t.Fatalf("want dedicated trimmed key, got %q", got)
	}
	if got := tr.BaseURL; got != "https://translate.example/v1" {
		t.Fatalf("want dedicated trimmed base URL, got %q", got)
	}
	if !tr.Configured() {
		t.Fatal("translation model plus base URL should enable translation")
	}

	fallback := AIConfig{
		Connection:  conn("api_key", "main", "base_url", "https://chat.example/v1"),
		Translation: TranslationConfig{Model: "x"},
	}
	trFallback := fallback.ResolvedTranslation()
	if got := trFallback.APIKey; got != "main" {
		t.Fatalf("want main key fallback, got %q", got)
	}
	if got := trFallback.BaseURL; got != "https://chat.example/v1" {
		t.Fatalf("want main base URL fallback, got %q", got)
	}
	if !trFallback.Configured() {
		t.Fatal("translation should use BI_AI_BASE_URL when dedicated URL is empty")
	}

	if (AIConfig{Translation: TranslationConfig{Model: "x"}}).ResolvedTranslation().Configured() {
		t.Fatal("translation model without any base URL should not enable translation")
	}
}

func TestAIConfig_EmbeddingBaseURL(t *testing.T) {
	c := AIConfig{Embedding: EmbeddingConfig{BaseURL: "https://embed.example/v1/"}}
	if got := c.ResolvedEmbedding().BaseURL; got != "https://embed.example/v1" {
		t.Fatalf("trim: got %q", got)
	}
	c2 := AIConfig{Connection: conn("base_url", "https://chat.example/v1", "provider", "openai")}
	if got := c2.ResolvedEmbedding().BaseURL; got != "https://chat.example/v1" {
		t.Fatalf("fallback to BaseURL: got %q", got)
	}
	c3 := AIConfig{Connection: conn("provider", "openai")}
	if got := c3.ResolvedEmbedding().BaseURL; got != "https://api.openai.com/v1" {
		t.Fatalf("openai default: got %q", got)
	}
}

func TestAIConfig_QueryOverrides(t *testing.T) {
	base := AIConfig{
		Connection: conn(
			"provider", "openai-compatible",
			"api_key", "ollama",
			"base_url", "http://local/v1",
			"model", "gemma4:e4b",
			"http_timeout", 300,
		),
	}
	t.Run("no overrides reuses base", func(t *testing.T) {
		got := base.ResolvedQuery()
		if got.Config.Connection.Model != "gemma4:e4b" || got.Config.Connection.BaseURL != "http://local/v1" || got.Config.Connection.APIKey != "ollama" {
			t.Errorf("expected base fields preserved, got %+v", got.Config.Connection)
		}
		if got.Override {
			t.Error("Override should be false for empty overrides")
		}
	})

	t.Run("model override only", func(t *testing.T) {
		c := base
		c.Query.Model = "gpt-4o"
		view := c.ResolvedQuery()
		if !view.Override {
			t.Error("Override should be true when QueryModel set")
		}
		if view.Config.Connection.Model != "gpt-4o" {
			t.Errorf("model not overridden: %q", view.Config.Connection.Model)
		}
		if view.Config.Connection.BaseURL != base.Connection.BaseURL {
			t.Errorf("base URL must fall back when not overridden: %q", view.Config.Connection.BaseURL)
		}
	})

	t.Run("full provider swap", func(t *testing.T) {
		c := base
		c.Query.Provider = "openai"
		c.Query.Model = "gpt-4o"
		c.Query.BaseURL = "https://api.openai.com/v1"
		c.Query.APIKey = "sk-xxx"
		c.Query.HTTPTimeoutSeconds = 60
		conn := c.ResolvedQuery().Config.Connection
		if conn.Provider != "openai" || conn.Model != "gpt-4o" || conn.BaseURL != "https://api.openai.com/v1" || conn.APIKey != "sk-xxx" || conn.HTTPTimeoutSeconds != 60 {
			t.Errorf("expected full override, got %+v", conn)
		}
	})
}

func TestAIConfig_QueryConfigured(t *testing.T) {
	if (AIConfig{}).ResolvedQuery().Configured() {
		t.Fatal("empty config should not enable query LLM")
	}
	if (AIConfig{Connection: conn("model", "gpt-4o")}).ResolvedQuery().Configured() {
		t.Fatal("openai without key or base URL should not enable")
	}
	local := AIConfig{
		Connection: conn(
			"provider", "openai-compatible",
			"base_url", "http://127.0.0.1:12434/v1",
			"model", "local",
		),
	}
	if !local.ResolvedQuery().Configured() {
		t.Fatal("keyless local openai-compatible should enable when base URL set")
	}
	withKey := AIConfig{
		Connection: conn("provider", "openai", "api_key", "sk-test", "model", "gpt-4o"),
	}
	if !withKey.ResolvedQuery().Configured() {
		t.Fatal("api key + model should enable")
	}
	if (AIConfig{
		Connection: conn("provider", "anthropic", "model", "claude-3"),
	}).ResolvedQuery().Configured() {
		t.Fatal("anthropic without API key should not enable")
	}
}

func TestAIConfig_EmbeddingConfigured(t *testing.T) {
	if (AIConfig{}).ResolvedEmbedding().Configured() {
		t.Fatal("empty config should not enable embeddings")
	}
	ok := AIConfig{
		Connection: conn("provider", "openai", "api_key", "k"),
		Embedding:  EmbeddingConfig{Model: "text-embedding-3-small"},
	}
	if !ok.ResolvedEmbedding().Configured() {
		t.Fatal("model + key + default openai base should enable")
	}
	noURL := AIConfig{
		Connection: conn("provider", "anthropic", "api_key", "k"),
		Embedding:  EmbeddingConfig{Model: "x"},
	}
	if noURL.ResolvedEmbedding().Configured() {
		t.Fatal("anthropic without any base URL should not enable (no default host for embeddings)")
	}
}
