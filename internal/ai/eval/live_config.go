package eval

import (
	"os"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/config"
)

// LiveAIConfigFromEnv builds an AIConfig for opt-in live eval from BI_AI_QUERY_* with
// BI_AI_* fallbacks. Does not require metadata DB provider rows.
func LiveAIConfigFromEnv() config.AIConfig {
	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{
			Provider: firstNonEmpty(
				os.Getenv("BI_AI_QUERY_PROVIDER"),
				os.Getenv("BI_AI_PROVIDER"),
			),
			APIKey: firstNonEmpty(
				os.Getenv("BI_AI_QUERY_API_KEY"),
				os.Getenv("BI_AI_API_KEY"),
			),
			BaseURL: firstNonEmpty(
				os.Getenv("BI_AI_QUERY_BASE_URL"),
				os.Getenv("BI_AI_BASE_URL"),
			),
			Model: firstNonEmpty(
				os.Getenv("BI_AI_QUERY_MODEL"),
				os.Getenv("BI_AI_MODEL"),
			),
			HTTPTimeoutSeconds: envIntDefault("BI_AI_HTTP_TIMEOUT_SECONDS", 300),
		},
		Generation: config.AIGenerationConfig{
			MaxTokens:           envIntDefault("BI_AI_MAX_TOKENS", 2048),
			Temperature:         envFloatDefault("BI_AI_TEMPERATURE", 0),
			MaxRetries:          envIntDefault("BI_AI_MAX_RETRIES", 1),
			MaxPromptInputRunes: envIntDefault("BI_AI_MAX_PROMPT_RUNES", 80000),
			MultiCandidateCount: envIntDefault("BI_AI_MULTI_CANDIDATE_COUNT", 1),
		},
	}
	return cfg.EffectiveQueryConfig()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func envIntDefault(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envFloatDefault(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
