package provider

import (
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/config"
)

func TestHTTPProviderNew(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com/v1", "sk-test")
	if p.client == nil {
		t.Fatal("expected non-nil http.Client")
	}
	if p.client.Timeout != time.Second {
		t.Fatalf("timeout = %v, want 1s", p.client.Timeout)
	}
	if p.baseURL != "https://api.example.com/v1" {
		t.Fatalf("baseURL = %q", p.baseURL)
	}
	if p.apiKey != "sk-test" {
		t.Fatalf("apiKey = %q", p.apiKey)
	}
}

func TestHTTPProviderURLConstructs(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com/v1", "")
	if got := p.url("/chat/completions"); got != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("url = %q", got)
	}
}

func TestHTTPProviderURLToleratesTrailingSlash(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com/v1/", "")
	if got := p.url("/chat/completions"); got != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("url = %q", got)
	}
}

func TestHTTPProviderURLToleratesFullPath(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com/v1/chat/completions", "")
	if got := p.url("/chat/completions"); got != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("url should not double-path: %q", got)
	}
}

func TestHTTPProviderURLAddsLeadingSlash(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com/v1", "")
	if got := p.url("embeddings"); !strings.HasSuffix(got, "/embeddings") {
		t.Fatalf("url = %q, expected trailing /embeddings", got)
	}
}

func TestBearerAuthHeadersWithKey(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com", "sk-test")
	h := p.bearerAuthHeaders()
	if h == nil {
		t.Fatal("expected non-nil headers")
	}
	if h["Authorization"] != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", h["Authorization"])
	}
}

func TestBearerAuthHeadersEmptyKey(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com", "")
	h := p.bearerAuthHeaders()
	if h != nil {
		t.Fatalf("expected nil headers for empty key, got %v", h)
	}
}

func TestHTTPProviderCloseIdempotent(t *testing.T) {
	t.Parallel()
	p := newHTTPProvider(time.Second, "https://api.example.com", "sk-test")
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second call should not panic.
	if err := p.Close(); err != nil {
		t.Fatalf("Close second call: %v", err)
	}
}

func TestAnthropicCloseIdempotent(t *testing.T) {
	t.Parallel()
	p := NewAnthropicProvider(config.AIConfig{
		Connection: config.AIConnectionConfig{
			Provider: "anthropic",
			APIKey:   "x",
			Model:    "claude-3-5-sonnet-latest",
			BaseURL:  "https://api.anthropic.com/v1",
		},
	})
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close second call: %v", err)
	}
}

func TestClientCloseIdempotent(t *testing.T) {
	t.Parallel()
	c := NewClient(config.AIConfig{
		Connection: config.AIConnectionConfig{
			Provider: "openai",
			APIKey:   "x",
			Model:    "gpt-4o-mini",
			BaseURL:  "https://api.openai.com/v1",
		},
	})
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close second call: %v", err)
	}
}

func TestOllamaOptionsEnabled(t *testing.T) {
	t.Parallel()
	c := &Client{topP: 0.9, numCtx: 4096}
	opts := c.ollamaOptions(0.7)
	if opts == nil {
		t.Fatal("expected non-nil ollamaOptions")
	}
	if opts.TopP == nil || *opts.TopP != 0.9 {
		t.Fatalf("TopP = %v, want 0.9", opts.TopP)
	}
	if opts.NumCtx == nil || *opts.NumCtx != 4096 {
		t.Fatalf("NumCtx = %v, want 4096", opts.NumCtx)
	}
}

func TestOllamaOptionsOnlyTopP(t *testing.T) {
	t.Parallel()
	c := &Client{topP: 0.5, numCtx: 0}
	opts := c.ollamaOptions(0.3)
	if opts == nil {
		t.Fatal("expected non-nil")
	}
	if opts.TopP == nil || *opts.TopP != 0.5 {
		t.Fatalf("TopP = %v", opts.TopP)
	}
	if opts.NumCtx != nil {
		t.Fatal("expected NumCtx to be nil")
	}
}

func TestOllamaOptionsOnlyNumCtx(t *testing.T) {
	t.Parallel()
	c := &Client{topP: 0, numCtx: 2048}
	opts := c.ollamaOptions(0.7)
	if opts == nil {
		t.Fatal("expected non-nil")
	}
	if opts.NumCtx == nil || *opts.NumCtx != 2048 {
		t.Fatalf("NumCtx = %v", opts.NumCtx)
	}
	if opts.TopP != nil {
		t.Fatal("expected TopP to be nil")
	}
}

func TestOllamaOptionsDisabled(t *testing.T) {
	t.Parallel()
	c := &Client{topP: 0, numCtx: 0}
	if opts := c.ollamaOptions(0.7); opts != nil {
		t.Fatal("expected nil for zero values")
	}
}
