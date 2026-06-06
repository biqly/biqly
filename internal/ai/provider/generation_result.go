package provider

import (
	"context"
	"log/slog"

	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
)

// GenerationResult is the outcome of a single LLM completion call.
type GenerationResult struct {
	Content string
	Usage   *TokenUsage
}

// TokenUsage tracks LLM token consumption.
type TokenUsage struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

// newTokenUsage builds a *TokenUsage from prompt and completion counts,
// deriving the total when the caller does not supply one. Pass total=0 to
// auto-compute as prompt+completion.
func newTokenUsage(prompt, completion, total int) *TokenUsage {
	if total == 0 {
		total = prompt + completion
	}
	return &TokenUsage{
		Prompt:     prompt,
		Completion: completion,
		Total:      total,
	}
}

// TokenUsageEstimate derives token counts from prompt stats and the completion
// text when the provider did not return usage. Returns nil when both are zero.
func TokenUsageEstimate(stats promptpkg.Stats, completion string) *TokenUsage {
	promptTok := stats.EstPromptTokens
	completionTok := promptpkg.EstimateTokens(completion)
	if promptTok == 0 && completionTok == 0 {
		return nil
	}
	return newTokenUsage(promptTok, completionTok, 0)
}

// TokenUsageFromGeneration prefers provider-reported usage and falls back to a
// local estimate derived from prompt stats and the completion text.
func TokenUsageFromGeneration(stats promptpkg.Stats, result GenerationResult) *TokenUsage {
	if u := result.Usage; u != nil && (u.Prompt > 0 || u.Completion > 0) {
		return newTokenUsage(u.Prompt, u.Completion, u.Total)
	}
	return TokenUsageEstimate(stats, result.Content)
}

func logLLMCompletion(ctx context.Context, provider, model string, estPromptTokens int, result GenerationResult) {
	fromAPI := result.Usage != nil && (result.Usage.Prompt > 0 || result.Usage.Completion > 0)
	usage := TokenUsageFromGeneration(promptpkg.Stats{EstPromptTokens: estPromptTokens}, result)

	args := []any{
		"provider", provider,
		"model", model,
		"est_prompt_tokens", estPromptTokens,
		"tokens_from_api", fromAPI,
	}
	if usage != nil {
		args = append(args,
			"prompt_tokens", usage.Prompt,
			"completion_tokens", usage.Completion,
			"total_tokens", usage.Total,
		)
	}
	slog.InfoContext(ctx, "llm completion", args...)
}
