package provider

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// dialTimeout bounds the TCP connect (and TLS handshake) for each attempt.
// Providers like DeepSeek sit behind a CDN (CloudFront) with rotating IPs;
// when one edge IP is momentarily unreachable, the OS default connect timeout
// (~2m) would let a single attempt consume the whole request budget before the
// retry stack (execRetry / isRetriableNetErr) could reach a healthy IP. A short
// dial timeout makes that attempt fail fast so a retry can succeed.
const dialTimeout = 10 * time.Second

// httpProvider holds shared HTTP transport settings for LLM and embedding APIs.
type httpProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func newHTTPProvider(timeout time.Duration, baseURL, apiKey string) httpProvider {
	return httpProvider{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   dialTimeout,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   dialTimeout,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
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
