// Package config provides application configuration loaded from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	HTTP     HTTPConfig
	Metadata MetadataConfig
	Redis    RedisConfig
	Query    QueryConfig
	Security SecurityConfig
	AI       AIConfig
}

// HTTPConfig holds HTTP server configuration.
type HTTPConfig struct {
	Host string
	Port int
}

// MetadataConfig holds metadata database connection configuration.
type MetadataConfig struct {
	DSN string
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	DSN string
}

// QueryConfig holds query execution limits and timeouts.
type QueryConfig struct {
	TimeoutSeconds    int
	MaxRows           int
	MaxRuntimeSeconds int
}

// SecurityConfig holds encryption key settings.
type SecurityConfig struct {
	EncryptionKey string
}

// AIConfig holds AI provider configuration.
type AIConfig struct {
	Provider      string
	APIKey        string
	BaseURL       string
	Model         string
	MaxTokens     int
	Temperature   float64
	RateLimitPerMinute int
	// MaxPromptInputRunes caps the semantic-model section of NL→query prompts (~4 chars/rune ≈ 1 token).
	MaxPromptInputRunes int
	// DescribeMaxCellRunes truncates each sampled cell in AI Describe before sending to the LLM.
	DescribeMaxCellRunes int
	// DescribeMaxSampleRows is a hard cap on rows sampled for Describe (wide tables × many columns).
	DescribeMaxSampleRows int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{
			Host: getEnv("BI_HTTP_HOST", "0.0.0.0"),
			Port: getEnvAsInt("BI_HTTP_PORT", 8888),
		},
		Metadata: MetadataConfig{
			DSN: getEnv("BI_METADATA_DB_DSN", "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"),
		},
		Redis: RedisConfig{
			DSN: getEnv("BI_REDIS_DSN", "redis://localhost:6379"),
		},
		Query: QueryConfig{
			TimeoutSeconds:    getEnvAsInt("BI_QUERY_TIMEOUT_SECONDS", 30),
			MaxRows:           getEnvAsInt("BI_QUERY_MAX_ROWS", 10000),
			MaxRuntimeSeconds: getEnvAsInt("BI_QUERY_MAX_RUNTIME_SECONDS", 60),
		},
		Security: SecurityConfig{
			EncryptionKey: getEnv("BI_ENCRYPTION_KEY", "change-this-to-a-secure-32-byte-key!!"),
		},
		AI: AIConfig{
			Provider:           getEnv("BI_AI_PROVIDER", "openai"),
			APIKey:             getEnv("BI_AI_API_KEY", ""),
			BaseURL:            getEnv("BI_AI_BASE_URL", ""),
			Model:              getEnv("BI_AI_MODEL", "gpt-4o"),
			MaxTokens:          getEnvAsInt("BI_AI_MAX_TOKENS", 4096),
			Temperature:        getEnvAsFloat("BI_AI_TEMPERATURE", 0.0),
			RateLimitPerMinute: getEnvAsInt("BI_AI_RATE_LIMIT_PER_MINUTE", 20),
			MaxPromptInputRunes:   getEnvAsInt("BI_AI_MAX_PROMPT_RUNES", 80000),
			DescribeMaxCellRunes:  getEnvAsInt("BI_AI_DESCRIBE_MAX_CELL_RUNES", 500),
			DescribeMaxSampleRows: getEnvAsInt("BI_AI_DESCRIBE_MAX_SAMPLE_ROWS", 12),
		},
	}

	if cfg.Metadata.DSN == "" {
		return nil, fmt.Errorf("BI_METADATA_DB_DSN is required")
	}
	if cfg.Security.EncryptionKey == "" {
		return nil, fmt.Errorf("BI_ENCRYPTION_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

// HTTPAddr returns the full HTTP listen address.
func (c *Config) HTTPAddr() string {
	return fmt.Sprintf("%s:%d", c.HTTP.Host, c.HTTP.Port)
}

// QueryTimeout returns the query timeout as time.Duration.
func (c *Config) QueryTimeout() time.Duration {
	return time.Duration(c.Query.TimeoutSeconds) * time.Second
}

// MaxQueryRuntime returns the maximum query runtime as time.Duration.
func (c *Config) MaxQueryRuntime() time.Duration {
	return time.Duration(c.Query.MaxRuntimeSeconds) * time.Second
}
