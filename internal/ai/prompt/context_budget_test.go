package prompt

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
		Connection: config.AIConnectionConfig{Model: "gpt-4o"},
		Generation: config.AIGenerationConfig{
			MaxTokens:           4096,
			MaxPromptInputRunes: 50_000,
		},
	}
	if got := EffectiveMaxPromptRunes(cfg, cfg.Connection.Model); got != 50_000 {
		t.Fatalf("EffectiveMaxPromptRunes = %d, want env cap 50000", got)
	}
}

func TestPromptRunesForTier_ExpandsOnRetry(t *testing.T) {
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "gpt-4o"},
		Generation: config.AIGenerationConfig{
			MaxTokens:           4096,
			MaxPromptInputRunes: 100_000,
		},
	}
	base := 40_000
	compact := RunesForTier(base, 0, cfg, cfg.Connection.Model)
	expanded := RunesForTier(base, 2, cfg, cfg.Connection.Model)
	if expanded <= compact {
		t.Fatalf("expanded runes %d should exceed compact %d", expanded, compact)
	}
}

func TestApplyContextTier_ExpandsFewShot(t *testing.T) {
	base := make([]FewShotExample, 10)
	for i := range base {
		base[i] = FewShotExample{Question: "q"}
	}
	tier0 := TailSlice(base, FewShotCap(0))
	tier2 := TailSlice(base, FewShotCap(2))
	if len(tier0) >= len(tier2) {
		t.Fatalf("tier0 few-shot %d should be fewer than tier2 %d", len(tier0), len(tier2))
	}
}

func TestTailPriorTurnsAccountsForResultSummaryBudget(t *testing.T) {
	turns := []ConversationTurn{
		{Question: "q1", ResultSummary: strings.Repeat("a", 900)},
		{Question: "q2", ResultSummary: strings.Repeat("b", 900)},
		{Question: "q3", ResultSummary: "May 20, 2026: 2,932 tweets"},
	}

	compact := TailPriorTurns(turns, 0)
	expanded := TailPriorTurns(turns, 2)

	if len(compact) != 1 {
		t.Fatalf("compact prior turns = %d, want 1", len(compact))
	}
	if compact[0].Question != "q3" {
		t.Fatalf("compact kept %q, want q3", compact[0].Question)
	}
	if len(expanded) <= len(compact) {
		t.Fatalf("expanded prior turns = %d, want more than compact %d", len(expanded), len(compact))
	}
}

func TestMeasurePrompt(t *testing.T) {
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "gpt-4o"},
		Generation: config.AIGenerationConfig{MaxTokens: 4096},
	}
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
