package ai

import (
	"context"
	"testing"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
)

func TestValidPurpose(t *testing.T) {
	for _, p := range AllPurposes {
		if !ValidPurpose(string(p)) {
			t.Errorf("expected %q to be valid", p)
		}
	}
	if ValidPurpose("nonsense") {
		t.Error("expected nonsense purpose to be invalid")
	}
}

func TestValidProviderType(t *testing.T) {
	cases := map[string]bool{
		"openai":            true,
		"OpenAI":            true,
		"openai-compatible": true,
		"anthropic":         true,
		"":                  false,
		"gemini":            false,
	}
	for in, want := range cases {
		if got := ValidProviderType(in); got != want {
			t.Errorf("ValidProviderType(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDefaultBaseURLForType(t *testing.T) {
	if got := DefaultBaseURLForType("openai"); got != "https://api.openai.com/v1" {
		t.Errorf("openai base = %q", got)
	}
	if got := DefaultBaseURLForType("anthropic"); got != "https://api.anthropic.com/v1" {
		t.Errorf("anthropic base = %q", got)
	}
	if got := DefaultBaseURLForType("openai-compatible"); got != "" {
		t.Errorf("openai-compatible base should be empty, got %q", got)
	}
}

func TestMaskSecret(t *testing.T) {
	cases := map[string]string{ //nolint:gosec // G101 false positive: test fixtures, not real credentials
		"":              "",
		"abc":           "••••",
		"abcd12345678":  "••••5678",
		"longtoken9k2d": "••••9k2d",
	}
	for in, want := range cases {
		if got := maskSecret(in); got != want {
			t.Errorf("maskSecret(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChatConfigForPurposeFallback(t *testing.T) {
	fallback := config.AIConfig{
		Provider:            "openai",
		Model:               "gpt-4o",
		BaseURL:             "https://api.openai.com/v1",
		APIKey:              "env-key",
		MaxPromptInputRunes: 80000,
	}
	store := NewProviderStore(nil, nil, &fallback)

	// No resolved model → fallback config, ok=false.
	cfg, ok := store.ChatConfigForPurpose(PurposeQuery)
	if ok {
		t.Fatal("expected ok=false when nothing resolved")
	}
	if cfg.Model != "gpt-4o" || cfg.APIKey != "env-key" {
		t.Errorf("fallback config not returned: %+v", cfg)
	}
}

func TestChatConfigForPurposeResolved(t *testing.T) {
	fallback := config.AIConfig{
		Provider:            "openai",
		Model:               "gpt-4o",
		MaxRetries:          3,
		MultiCandidateCount: 2,
		MaxPromptInputRunes: 80000,
		Query:               config.QueryLLMConfig{Model: "should-be-cleared"},
	}
	store := NewProviderStore(nil, nil, &fallback)
	store.resolved[PurposeQuery] = &resolvedModel{
		ProviderType:        "anthropic",
		BaseURL:             "https://api.anthropic.com/v1",
		APIKey:              "db-key",
		ModelID:             "claude-sonnet-4",
		MaxTokens:           2048,
		Temperature:         0.2,
		MaxPromptInputRunes: 40000,
		HTTPTimeoutSeconds:  90,
	}

	cfg, ok := store.ChatConfigForPurpose(PurposeQuery)
	if !ok {
		t.Fatal("expected ok=true when resolved")
	}
	if cfg.Provider != "anthropic" || cfg.Model != "claude-sonnet-4" || cfg.APIKey != "db-key" {
		t.Errorf("connection fields not overridden: %+v", cfg)
	}
	if cfg.MaxTokens != 2048 || cfg.Temperature != 0.2 || cfg.HTTPTimeoutSeconds != 90 {
		t.Errorf("model tuning not applied: %+v", cfg)
	}
	if cfg.MaxPromptInputRunes != 40000 {
		t.Errorf("expected per-model max prompt runes, got %d", cfg.MaxPromptInputRunes)
	}
	// Non-connection knobs carry through from fallback.
	if cfg.MaxRetries != 3 || cfg.MultiCandidateCount != 2 {
		t.Errorf("fallback tuning lost: %+v", cfg)
	}
	// BI_AI_QUERY_* overrides neutralized so the DB selection is authoritative.
	if cfg.Query.Model != "" {
		t.Errorf("expected QueryModel cleared, got %q", cfg.Query.Model)
	}
}

func TestEffectiveConfigOverlaysEmbeddingAndTranslation(t *testing.T) {
	store := NewProviderStore(nil, nil, &config.AIConfig{Model: "gpt-4o"})
	store.resolved[PurposeEmbedding] = &resolvedModel{
		ModelID:            "text-embedding-3-small",
		BaseURL:            "https://emb.example/v1",
		APIKey:             "emb-key",
		HTTPTimeoutSeconds: 600,
	}
	store.resolved[PurposeTranslation] = &resolvedModel{
		ModelID: "gpt-4o-mini",
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "tr-key",
	}

	cfg := store.EffectiveConfig()
	if cfg.Embedding.Model != "text-embedding-3-small" || cfg.Embedding.BaseURL != "https://emb.example/v1" || cfg.Embedding.APIKey != "emb-key" {
		t.Errorf("embedding overlay missing: %+v", cfg)
	}
	if cfg.Embedding.HTTPTimeoutSeconds != 600 {
		t.Errorf("embedding timeout not applied: %d", cfg.Embedding.HTTPTimeoutSeconds)
	}
	if cfg.Translation.Model != "gpt-4o-mini" || cfg.Translation.APIKey != "tr-key" {
		t.Errorf("translation overlay missing: %+v", cfg)
	}
}

func TestCacheVersionIncrements(t *testing.T) {
	store := NewProviderStore(nil, nil, &config.AIConfig{})
	v0 := store.CacheVersion()
	store.version.Add(1)
	if store.CacheVersion() != v0+1 {
		t.Error("cache version did not advance")
	}
}

// stubProvider records the prompt it last received.
type stubProvider struct {
	last  string
	reply string
}

func (s *stubProvider) Generate(_ context.Context, prompt string) (providerpkg.GenerationResult, error) {
	s.last = prompt
	return providerpkg.GenerationResult{Content: s.reply}, nil
}

func (s *stubProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return s.Generate(ctx, prompt)
}

func TestPurposeProviderFallsBackWhenUnresolved(t *testing.T) {
	fallback := &stubProvider{reply: "from-fallback"}
	store := NewProviderStore(nil, nil, &config.AIConfig{Model: "gpt-4o"})
	pp := NewPurposeProvider(store, PurposeQuery, fallback, nil)

	res, err := pp.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "from-fallback" {
		t.Errorf("expected fallback provider to serve, got %q", res.Content)
	}
	if fallback.last != "hello" {
		t.Errorf("fallback did not receive prompt, got %q", fallback.last)
	}
}

func TestEffectiveConfigForEmbeddings_ClearsEnvWhenUnresolved(t *testing.T) {
	fallback := config.AIConfig{
		Embedding: config.EmbeddingConfig{
			Model:  "env-embed",
			APIKey: "env-key",
		},
		BaseURL: "https://api.openai.com/v1",
	}
	store := NewProviderStore(nil, nil, &fallback)

	cfg := store.EffectiveConfigForEmbeddings()
	if cfg.EmbeddingsConfigured() {
		t.Fatal("expected embeddings disabled when DB has no embedding model")
	}
	if cfg.Embedding.Model != "" {
		t.Errorf("EmbeddingModel = %q, want empty", cfg.Embedding.Model)
	}

	store.resolved[PurposeEmbedding] = &resolvedModel{
		ModelID: "db-embed",
		APIKey:  "db-key",
		BaseURL: "https://embed.example/v1",
	}
	cfg2 := store.EffectiveConfigForEmbeddings()
	if !cfg2.EmbeddingsConfigured() {
		t.Fatal("expected embeddings enabled when DB embedding model is resolved")
	}
	if cfg2.Embedding.Model != "db-embed" {
		t.Errorf("EmbeddingModel = %q, want db-embed", cfg2.Embedding.Model)
	}
}

func TestModelLabelForPurpose(t *testing.T) {
	fallback := config.AIConfig{Model: "env-describe", Query: config.QueryLLMConfig{Model: "qwen-env"}}
	store := NewProviderStore(nil, nil, &fallback)
	store.resolved[PurposeQuery] = &resolvedModel{
		ModelID:     "mimo-v2.5",
		DisplayName: "Mimo v2.5",
	}

	if got := store.ModelLabelForPurpose(PurposeQuery); got != "Mimo v2.5" {
		t.Errorf("ModelLabelForPurpose(query) = %q, want Mimo v2.5", got)
	}
	if got := store.ModelLabelForPurpose(PurposeDescribe); got != "env-describe" {
		t.Errorf("ModelLabelForPurpose(describe) = %q, want env-describe", got)
	}

	store.resolved[PurposeQuery] = &resolvedModel{ModelID: "raw-id", DisplayName: ""}
	if got := store.ModelLabelForPurpose(PurposeQuery); got != "raw-id" {
		t.Errorf("ModelLabelForPurpose(query) = %q, want raw-id", got)
	}
}
