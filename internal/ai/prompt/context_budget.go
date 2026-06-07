package prompt

import (
	"strings"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/config"
)

const (
	defaultContextWindowTokens = 128_000
	charsPerTokenEstimate      = 4
	systemPromptTokenReserve   = 512
	minInputTokenBudget        = 8_000
)

// ModelContextProfile describes provider context limits for prompt budgeting.
type ModelContextProfile struct {
	ContextWindowTokens int
	Source              string // "registry", "num_ctx", "default"
}

// Stats PromptStats summarizes prompt size for logging and API responses.
type Stats struct {
	PromptRunes           int    `json:"prompt_runes"`
	EstPromptTokens       int    `json:"est_prompt_tokens"`
	EstCompletionReserve  int    `json:"est_completion_reserve,omitempty"`
	ContextWindowTokens   int    `json:"context_window_tokens,omitempty"`
	MaxPromptRunes        int    `json:"max_prompt_runes,omitempty"`
	PromptBuildDurationMs int64  `json:"prompt_build_duration_ms,omitempty"`
	ContextTier           int    `json:"context_tier,omitempty"`
	ContextTierLabel      string `json:"context_tier_label,omitempty"`
	Model                 string `json:"model,omitempty"`
}

// EstimateTokens approximates tokenizer output (~4 Latin chars per token).
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	runes := utf8.RuneCountInString(text)
	byRunes := (runes + 3) / 4
	byBytes := (len(text) + 3) / 4
	if byRunes > byBytes {
		return byRunes
	}
	return byBytes
}

// LookupModelContextProfile resolves context window from model name, with optional
// Ollama num_ctx override when the registry has no entry.
func LookupModelContextProfile(model string, numCtx int) ModelContextProfile {
	key := normalizeModelKey(model)
	if w, ok := modelContextRegistry[key]; ok {
		return ModelContextProfile{ContextWindowTokens: w, Source: "registry"}
	}
	for prefix, w := range modelContextPrefixRegistry {
		if strings.HasPrefix(key, prefix) {
			return ModelContextProfile{ContextWindowTokens: w, Source: "registry"}
		}
	}
	if numCtx > 0 {
		return ModelContextProfile{ContextWindowTokens: numCtx, Source: "num_ctx"}
	}
	return ModelContextProfile{ContextWindowTokens: defaultContextWindowTokens, Source: "default"}
}

// EffectiveMaxPromptRunes derives the prompt rune cap from model context window,
// completion reserve, and the configured BI_AI_MAX_PROMPT_RUNES ceiling.
func EffectiveMaxPromptRunes(cfg config.AIConfig, model string) int {
	profile := LookupModelContextProfile(model, cfg.Generation.NumCtx)
	inputTokens := inputTokenBudget(cfg, profile.ContextWindowTokens)
	runes := inputTokens * charsPerTokenEstimate
	if cfg.Generation.MaxPromptInputRunes > 0 && cfg.Generation.MaxPromptInputRunes < runes {
		return cfg.Generation.MaxPromptInputRunes
	}
	if runes < 16_000 {
		return 16_000
	}
	return runes
}

func inputTokenBudget(cfg config.AIConfig, contextWindow int) int {
	if contextWindow <= 0 {
		contextWindow = defaultContextWindowTokens
	}
	reserve := cfg.Generation.MaxTokens
	if reserve <= 0 {
		reserve = 4096
	}
	if reserve > contextWindow/2 {
		reserve = contextWindow / 2
	}
	budget := contextWindow - reserve - systemPromptTokenReserve
	if budget < minInputTokenBudget {
		return minInputTokenBudget
	}
	return budget
}

// RunesForTier scales the base rune budget by progressive context tier.
func RunesForTier(baseRunes, tier int, cfg config.AIConfig, model string) int {
	if baseRunes <= 0 {
		baseRunes = EffectiveMaxPromptRunes(cfg, model)
	}
	mult := progressiveBudgetMultiplier(tier)
	scaled := int(float64(baseRunes) * mult)
	ceiling := EffectiveMaxPromptRunes(cfg, model)
	if scaled > ceiling {
		return ceiling
	}
	if scaled < 16_000 {
		return 16_000
	}
	return scaled
}

func progressiveBudgetMultiplier(tier int) float64 {
	switch tier {
	case 0:
		return 1.0
	case 1:
		return 1.35
	default:
		return 1.75
	}
}

func contextTierLabel(tier int) string {
	switch tier {
	case 0:
		return "compact"
	case 1:
		return "standard"
	default:
		return "expanded"
	}
}

// ContextTierLabel returns the human-readable label for a context tier.
func ContextTierLabel(tier int) string {
	return contextTierLabel(tier)
}

// MeasurePrompt builds stats for a built prompt string.
func MeasurePrompt(prompt, model string, tier int, cfg config.AIConfig) Stats {
	profile := LookupModelContextProfile(model, cfg.Generation.NumCtx)
	maxRunes := EffectiveMaxPromptRunes(cfg, model)
	return Stats{
		PromptRunes:          utf8.RuneCountInString(prompt),
		EstPromptTokens:      EstimateTokens(prompt),
		EstCompletionReserve: completionReserveTokens(cfg),
		ContextWindowTokens:  profile.ContextWindowTokens,
		MaxPromptRunes:       maxRunes,
		ContextTier:          tier,
		ContextTierLabel:     contextTierLabel(tier),
		Model:                strings.TrimSpace(model),
	}
}

func completionReserveTokens(cfg config.AIConfig) int {
	r := cfg.Generation.MaxTokens
	if r <= 0 {
		return 4096
	}
	return r
}

func normalizeModelKey(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

var modelContextRegistry = map[string]int{
	"gpt-4o":                     128_000,
	"gpt-4o-mini":                128_000,
	"gpt-4-turbo":                128_000,
	"gpt-4":                      128_000,
	"gpt-3.5-turbo":              16_385,
	"claude-3-5-sonnet-20241022": 200_000,
	"claude-3-5-sonnet-latest":   200_000,
	"claude-3-5-haiku-latest":    200_000,
	"claude-3-opus-20240229":     200_000,
	"gemma2:9b":                  8192,
	"gemma2:27b":                 8192,
}

var modelContextPrefixRegistry = map[string]int{
	"gpt-4o":     128_000,
	"gpt-4":      128_000,
	"claude-3-5": 200_000,
	"claude-3":   200_000,
	"gemini-1.5": 1_000_000,
	"gemini-2":   1_000_000,
	"llama3":     128_000,
	"llama3.1":   128_000,
	"llama3.2":   128_000,
	"mistral":    32_768,
	"mixtral":    32_768,
	"qwen2.5":    128_000,
	"deepseek":   64_000,
}
