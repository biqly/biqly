package provider

import (
	"strings"

	"github.com/biqly/biqly/internal/platform/observability"
)

func recordLLMRetry(provider string) {
	observability.Default().RecordLLMProviderRetry(normalizeProviderName(provider))
}

func recordLLMError(provider string, err error, httpStatus int) {
	observability.Default().RecordLLMProviderError(
		normalizeProviderName(provider),
		observability.ClassifyProviderError(err, httpStatus),
	)
}

func recordLLMProviderTokens(promptTokens, completionTokens int) {
	observability.Default().RecordLLMProviderTokens(promptTokens, completionTokens)
}

func normalizeProviderName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "openai-compatible":
		return "openai"
	case "anthropic":
		return "anthropic"
	default:
		return "other"
	}
}
