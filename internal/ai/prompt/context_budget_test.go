package prompt

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/config"
)

func TestEstimateTokens(t *testing.T) {
	t.Parallel()
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("empty = %d, want 0", got)
	}
	if got := EstimateTokens("abcdabcdabcdabcd"); got != 4 {
		t.Fatalf("16 chars = %d tokens, want 4", got)
	}
}

func TestLookupModelContextProfile(t *testing.T) {
	t.Parallel()
	p := LookupModelContextProfile("gpt-4o-mini", 0)
	if p.ContextWindowTokens != 128_000 || p.Source != "registry" {
		t.Fatalf("gpt-4o-mini profile = %+v", p)
	}
	p = LookupModelContextProfile("custom-local", 32000)
	if p.ContextWindowTokens != 32000 || p.Source != "num_ctx" {
		t.Fatalf("num_ctx profile = %+v", p)
	}
	p = LookupModelContextProfile("unknown-model", 0)
	if p.Source != "default" {
		t.Fatalf("default profile source = %q", p.Source)
	}
	p = LookupModelContextProfile("claude-3-5-sonnet-20241022", 0)
	if p.ContextWindowTokens != 200_000 {
		t.Fatalf("claude profile = %+v", p)
	}
}

func TestEffectiveMaxPromptRunes_RespectsEnvCap(t *testing.T) {
	t.Parallel()
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

func TestEffectiveMaxPromptRunesMinFloor(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "mini-local"},
		Generation: config.AIGenerationConfig{MaxTokens: 0},
	}
	runes := EffectiveMaxPromptRunes(cfg, "mini-local")
	if runes < 16000 {
		t.Fatalf("EffectiveMaxPromptRunes = %d, want >= 16000", runes)
	}
}

func TestPromptRunesForTier_ExpandsOnRetry(t *testing.T) {
	t.Parallel()
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

func TestRunesForTierZeroBase(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "gpt-4o"},
		Generation: config.AIGenerationConfig{
			MaxTokens:           4096,
			MaxPromptInputRunes: 100_000,
		},
	}
	runes := RunesForTier(0, 0, cfg, "gpt-4o")
	if runes <= 0 {
		t.Fatalf("RunesForTier(0,0) = %d, want > 0", runes)
	}
}

func TestRunesForTierCeiling(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "gpt-4o"},
		Generation: config.AIGenerationConfig{
			MaxTokens:           4096,
			MaxPromptInputRunes: 100_000,
		},
	}
	runes := RunesForTier(500_000, 2, cfg, "gpt-4o")
	if runes > 100_000 {
		t.Fatalf("RunesForTier over ceiling = %d, want <= 100000", runes)
	}
}

func TestApplyContextTier_ExpandsFewShot(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "gpt-4o"},
		Generation: config.AIGenerationConfig{MaxTokens: 4096},
	}
	text := strings.Repeat("x", 4000)
	stats := MeasurePrompt(text, "gpt-4o", 1, cfg)
	if stats.PromptRunes != 4000 {
		t.Fatalf("PromptRunes = %d, want 4000", stats.PromptRunes)
	}
	if stats.Model != "gpt-4o" {
		t.Fatalf("Model = %q", stats.Model)
	}
}

func TestContextTierLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tier int
		want string
	}{
		{0, "compact"},
		{1, "standard"},
		{2, "expanded"},
		{3, "expanded"},
		{-1, "expanded"},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			if got := ContextTierLabel(tc.tier); got != tc.want {
				t.Errorf("ContextTierLabel(%d) = %q, want %q", tc.tier, got, tc.want)
			}
		})
	}
}

func TestInputTokenBudgetZeroContextWindow(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Model: "unknown"},
		Generation: config.AIGenerationConfig{MaxTokens: 0},
	}
	budget := inputTokenBudget(cfg, 0)
	if budget < 8000 {
		t.Fatalf("inputTokenBudget = %d, want >= 8000", budget)
	}
}

func TestInputTokenBudgetSmallContext(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Generation: config.AIGenerationConfig{MaxTokens: 100_000},
	}
	budget := inputTokenBudget(cfg, 200_000)
	if budget < 99488 {
		t.Fatalf("inputTokenBudget = %d, want approx 99488", budget)
	}
}

func TestCompletionReserveTokens(t *testing.T) {
	t.Parallel()
	cfg := config.AIConfig{
		Generation: config.AIGenerationConfig{MaxTokens: 0},
	}
	if got := completionReserveTokens(cfg); got != 4096 {
		t.Fatalf("completionReserveTokens = %d, want 4096", got)
	}
	cfg2 := config.AIConfig{
		Generation: config.AIGenerationConfig{MaxTokens: 2048},
	}
	if got := completionReserveTokens(cfg2); got != 2048 {
		t.Fatalf("completionReserveTokens = %d, want 2048", got)
	}
}

func TestProgressiveBudgetMultiplier(t *testing.T) {
	t.Parallel()
	if m := progressiveBudgetMultiplier(0); m != 1.0 {
		t.Fatalf("tier 0 = %f", m)
	}
	if m := progressiveBudgetMultiplier(1); m != 1.35 {
		t.Fatalf("tier 1 = %f", m)
	}
	if m := progressiveBudgetMultiplier(2); m != 1.75 {
		t.Fatalf("tier 2 = %f", m)
	}
	if m := progressiveBudgetMultiplier(99); m != 1.75 {
		t.Fatalf("tier default = %f", m)
	}
}

func TestNormalizeModelKey(t *testing.T) {
	t.Parallel()
	if got := normalizeModelKey("GPT-4o"); got != "gpt-4o" {
		t.Fatalf("normalizeModelKey = %q", got)
	}
	if got := normalizeModelKey("  Claude-3-5  "); got != "claude-3-5" {
		t.Fatalf("normalizeModelKey = %q", got)
	}
}
