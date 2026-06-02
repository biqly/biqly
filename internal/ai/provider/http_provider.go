package provider

import (
	"net/http"
	"strings"
	"time"
)

// httpProvider holds shared HTTP transport settings for LLM and embedding APIs.
type httpProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func newHTTPProvider(timeout time.Duration, baseURL, apiKey string) httpProvider {
	return httpProvider{
		client:  &http.Client{Timeout: timeout},
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  apiKey,
	}
}

func (p httpProvider) url(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base := strings.TrimRight(p.baseURL, "/")
	// Tolerate base URLs that already include the endpoint path (a common
	// mistake when pasting a full "…/v1/embeddings" or "…/v1/chat/completions"
	// URL) so we don't produce ".../embeddings/embeddings" → 404.
	if strings.HasSuffix(base, path) {
		return base
	}
	return base + path
}

func (p httpProvider) bearerAuthHeaders() map[string]string {
	if p.apiKey == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + p.apiKey}
}

// Close drains the provider's HTTP client by closing idle keepalive
// connections. Used during graceful shutdown so the process exits without
// dangling sockets. Safe to call multiple times.
func (p httpProvider) Close() error {
	if p.client != nil {
		p.client.CloseIdleConnections()
	}
	return nil
}
