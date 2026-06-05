package catalogclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/biqly/biqly/pkg/common/httpclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/biqly/biqly/pkg/internalapi"
)

// defaultUserAgent identifies this client in upstream access logs. Peer
// services may override via WithUserAgent.
const defaultUserAgent = "biqly-catalogclient/0.1"

// HTTPDoer is the minimum surface this package needs from net/http. Tests
// inject a fake; production wires through an *http.Client.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client talks to the Catalog Service over /internal/*. It is safe for
// concurrent use by multiple goroutines once constructed.
type Client struct {
	baseURL     string
	httpClient  HTTPDoer
	authToken   string
	caller      string
	userAgent   string
	retryPolicy httpclient.RetryPolicy
	breaker     *httpclient.CircuitBreaker
}

// Option configures a Client at construction time. Options are applied in
// argument order.
type Option func(*Client)

// WithHTTPClient overrides the underlying transport. The caller owns its
// lifecycle (timeouts, connection pool). A nil value resets to the service
// client defaults.
func WithHTTPClient(c HTTPDoer) Option {
	return func(cl *Client) {
		if c == nil {
			cl.httpClient = httpclient.NewServiceClient()
			return
		}
		cl.httpClient = c
	}
}

// WithAuthToken sends "Authorization: Bearer <token>" on every request.
// Empty string disables the header.
func WithAuthToken(token string) Option {
	return func(cl *Client) { cl.authToken = strings.TrimSpace(token) }
}

// WithCaller sends X-Internal-Caller so Catalog can audit which peer service
// initiated the request (for example "ai", "query", or "catalog").
func WithCaller(caller string) Option {
	return func(cl *Client) { cl.caller = strings.TrimSpace(caller) }
}

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) Option {
	return func(cl *Client) {
		if ua = strings.TrimSpace(ua); ua != "" {
			cl.userAgent = ua
		}
	}
}

// WithRetryPolicy overrides transient failure retry behavior.
func WithRetryPolicy(policy httpclient.RetryPolicy) Option {
	return func(cl *Client) { cl.retryPolicy = policy }
}

// WithCircuitBreaker overrides circuit breaker behavior. A nil value disables it.
func WithCircuitBreaker(breaker *httpclient.CircuitBreaker) Option {
	return func(cl *Client) { cl.breaker = breaker }
}

// New returns a ready-to-use Client. baseURL is the scheme://host[:port] of
// the Catalog Service; do NOT include /internal — methods build full paths
// themselves so future routing changes stay encapsulated.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient:  httpclient.NewServiceClient(),
		userAgent:   defaultUserAgent,
		retryPolicy: httpclient.DefaultRetryPolicy(),
		breaker:     httpclient.NewCircuitBreaker(httpclient.DefaultCircuitBreakerConfig()),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// BaseURL exposes the configured base URL. Useful for logging and tests.
func (c *Client) BaseURL() string { return c.baseURL }

// get issues GET <baseURL>/internal<path>?<query> and decodes the JSON body
// into out. A 2xx with empty body is allowed when out is nil.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

// post issues POST <baseURL>/internal<path> with the JSON-encoded body, and
// decodes the JSON response into out. Both body and out may be nil.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

// do is the single HTTP path. It centralises URL construction, header
// management, body encoding, status-code branching and error envelope parsing
// so per-endpoint methods stay one-liners.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) (err error) {
	if c.baseURL == "" {
		return errors.New("catalogclient: baseURL is empty")
	}
	u := c.baseURL + "/internal" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var bodyBytes []byte
	if body != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return fmt.Errorf("catalogclient: encode request body: %w", err)
		}
		bodyBytes = buf.Bytes()
	}

	if err := c.breaker.Allow(); err != nil {
		return fmt.Errorf("catalogclient: %s %s: %w", method, u, err)
	}
	resp, err := httpclient.DoWithRetry(ctx, c.retryPolicy, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("catalogclient: build request: %w", err)
		}
		c.setHeaders(ctx, req, len(bodyBytes) > 0)
		return req, nil
	}, c.httpClient.Do)
	c.breaker.Record(resp, err)
	if err != nil {
		return fmt.Errorf("catalogclient: %s %s: %w", method, u, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeErrorResponse(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return fmt.Errorf("catalogclient: drain response body: %w", err)
		}
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("catalogclient: decode response: %w", err)
	}
	return nil
}

func (c *Client) setHeaders(ctx context.Context, req *http.Request, hasBody bool) {
	req.Header.Set("Accept", "application/json")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	if c.caller != "" {
		req.Header.Set("X-Internal-Caller", c.caller)
	}
	if id := requestid.FromContext(ctx); id != "" {
		req.Header.Set("X-Request-ID", id)
	}
	if traceparent := tracecontext.TraceparentFromContext(ctx); traceparent != "" {
		req.Header.Set("traceparent", traceparent)
	}
}

// decodeErrorResponse reads the response body, attempts to parse it as an
// internalapi.Error envelope, and returns an APIError. Bodies that don't parse
// cleanly (e.g. proxy HTML pages on a misconfigured ingress) still produce a
// usable APIError carrying the raw text snippet.
func decodeErrorResponse(resp *http.Response) error {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return newAPIErrorFromResponse(resp.StatusCode, internalapi.Error{Error: "read error response: " + err.Error()})
	}
	var env internalapi.Error
	if len(raw) > 0 && bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		if err := json.Unmarshal(raw, &env); err != nil {
			env.Error = strings.TrimSpace(string(raw))
		}
	}
	if env.Error == "" {
		env.Error = strings.TrimSpace(string(raw))
		if env.Error == "" {
			env.Error = http.StatusText(resp.StatusCode)
		}
	}
	return newAPIErrorFromResponse(resp.StatusCode, env)
}
