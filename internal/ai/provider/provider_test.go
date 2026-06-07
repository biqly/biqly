package provider

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/config"
)

func TestNewProviderRoutesByName(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		wantType string
	}{
		{"openai default", "openai", "*ai.Client"},
		{"openai-compatible", "openai-compatible", "*ai.Client"},
		{"empty falls back to openai", "", "*ai.Client"},
		{"anthropic", "anthropic", "*ai.AnthropicProvider"},
		{"case-insensitive", "Anthropic", "*ai.AnthropicProvider"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewProvider(config.AIConfig{
				Connection: config.AIConnectionConfig{
					Provider: c.provider,
					APIKey:   "x",
					Model:    "m",
				},
			})
			if err != nil {
				t.Fatalf("NewProvider(%q) error = %v, want nil", c.provider, err)
			}
			got := strings.TrimPrefix(typeName(p), "*ai.")
			want := strings.TrimPrefix(c.wantType, "*ai.")
			if got != want {
				t.Errorf("NewProvider(%q): got %s, want %s", c.provider, got, want)
			}
		})
	}
}

func TestNewProviderRejectsUnknown(t *testing.T) {
	if _, err := NewProvider(config.AIConfig{
		Connection: config.AIConnectionConfig{Provider: "cohere"},
	}); err == nil {
		t.Errorf("expected error for unsupported provider, got nil")
	}
}

// typeName returns the dynamic type of v in the form "*pkg.Type" so the test
// can assert the factory wired the right adapter without reflect import noise.
func typeName(v any) string {
	switch v.(type) {
	case *Client:
		return "*ai.Client"
	case *AnthropicProvider:
		return "*ai.AnthropicProvider"
	default:
		return "unknown"
	}
}
