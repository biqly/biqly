package ai

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/config"
)

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("empty = %d, want 0", got)
	}
	if got := EstimateTokens("abcdabcdabcdabcd"); got != 4 {
		t.Fatalf("16 chars = %d tokens, want 4", got)
	}
}

func TestLookupModelContextProfile(t *testing.T) {
	p := LookupModelContextProfile("gpt-4o-mini", 0)
	if p.ContextWindowTokens != 128_000 || p.Source != "registry" {
		t.Fatalf("gpt-4o-mini profile = %+v", p)
	}
	p = LookupModelContextProfile("custom-local", 32000)
	if p.ContextWindowTokens != 32000 || p.Source != "num_ctx" {
		t.Fatalf("num_ctx profile = %+v", p)
	}
}

func TestEffectiveMaxPromptRunes_RespectsEnvCap(t *testing.T) {
	cfg := config.AIConfig{
		Model:               "gpt-4o",
		MaxTokens:           4096,
		MaxPromptInputRunes: 50_000,
	}
	if got := EffectiveMaxPromptRunes(cfg, cfg.Model); got != 50_000 {
		t.Fatalf("EffectiveMaxPromptRunes = %d, want env cap 50000", got)
	}
}

func TestPromptRunesForTier_ExpandsOnRetry(t *testing.T) {
	cfg := config.AIConfig{Model: "gpt-4o", MaxTokens: 4096, MaxPromptInputRunes: 100_000}
	base := 40_000
	compact := PromptRunesForTier(base, 0, cfg, cfg.Model)
	expanded := PromptRunesForTier(base, 2, cfg, cfg.Model)
	if expanded <= compact {
		t.Fatalf("expanded runes %d should exceed compact %d", expanded, compact)
	}
}

func TestApplyContextTier_ExpandsFewShot(t *testing.T) {
	base := processOptions{
		fewShot: make([]FewShotExample, 10),
	}
	for i := range base.fewShot {
		base.fewShot[i] = FewShotExample{Question: "q"}
	}
	tier0 := applyContextTier(base, 0)
	tier2 := applyContextTier(base, 2)
	if len(tier0.fewShot) >= len(tier2.fewShot) {
		t.Fatalf("tier0 few-shot %d should be fewer than tier2 %d", len(tier0.fewShot), len(tier2.fewShot))
	}
}

func TestMeasurePrompt(t *testing.T) {
	cfg := config.AIConfig{Model: "gpt-4o", MaxTokens: 4096}
	text := strings.Repeat("x", 4000)
	stats := MeasurePrompt(text, "gpt-4o", 1, cfg)
	if stats.PromptRunes != 4000 {
		t.Fatalf("PromptRunes = %d, want 4000", stats.PromptRunes)
	}
	if stats.EstPromptTokens != 1000 {
		t.Fatalf("EstPromptTokens = %d, want 1000", stats.EstPromptTokens)
	}
	if stats.ContextTierLabel != "standard" {
		t.Fatalf("tier label = %q, want standard", stats.ContextTierLabel)
	}
}
