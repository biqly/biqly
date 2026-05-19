package ai

import (
	"context"
	"log/slog"
)

// GenerationResult is the outcome of a single LLM completion call.
type GenerationResult struct {
	Content string
	Usage   *TokenUsage
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

func tokenUsageFromGeneration(stats PromptStats, result GenerationResult) *TokenUsage {
	if u := result.Usage; u != nil && (u.Prompt > 0 || u.Completion > 0) {
		return newTokenUsage(u.Prompt, u.Completion, u.Total)
	}
	return tokenUsageEstimate(stats, result.Content)
}

func logLLMCompletion(ctx context.Context, provider, model string, estPromptTokens int, result GenerationResult) {
	fromAPI := result.Usage != nil && (result.Usage.Prompt > 0 || result.Usage.Completion > 0)
	usage := tokenUsageFromGeneration(PromptStats{EstPromptTokens: estPromptTokens}, result)

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
