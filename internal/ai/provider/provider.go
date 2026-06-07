package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/config"
)

// Provider is the abstraction over LLM backends. Each implementation owns its
// own HTTP client, request shape, and authentication scheme — the rest of the
// service treats them all the same: prompt in, completion out (with token usage
// when the provider returns it).
//
// GenerateAt overrides the default temperature for a single call. Used by the
// self-consistency loop to draw varied candidates without rebuilding clients.
type Provider interface {
	Generate(ctx context.Context, prompt string) (GenerationResult, error)
	GenerateAt(ctx context.Context, prompt string, temperature float64) (GenerationResult, error)
}

// NewProvider returns a Provider for the configured backend. Unknown providers
// are an error so callers fail fast at startup rather than mid-request.
func NewProvider(cfg config.AIConfig) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Connection.Provider)) {
	case "", "openai", "openai-compatible":
		return NewClient(cfg), nil
	case "anthropic":
		return NewAnthropicProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported AI provider %q (supported: openai, openai-compatible, anthropic)", cfg.Connection.Provider)
	}
}
