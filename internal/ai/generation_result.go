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

func tokenUsageFromGeneration(stats PromptStats, result GenerationResult) *TokenUsage {
	if u := result.Usage; u != nil && (u.Prompt > 0 || u.Completion > 0) {
		total := u.Total
		if total == 0 {
			total = u.Prompt + u.Completion
		}
		return &TokenUsage{
			Prompt:     u.Prompt,
			Completion: u.Completion,
			Total:      total,
		}
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
