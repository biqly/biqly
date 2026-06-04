package queryclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/biqly/biqly/pkg/common/httpclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/biqly/biqly/pkg/internalapi"
)

const defaultUserAgent = "biqly-queryclient/0.1"

// HTTPDoer is the minimum surface this package needs from net/http.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client talks to the Query Engine over /internal/query/*. It is safe for
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

// WithHTTPClient overrides the underlying transport. Use this to bound query
// execution time at the transport layer; the server enforces its own ceiling
// but defensive client-side transport timeouts shield callers from upstream stalls.
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
func WithAuthToken(token string) Option {
	return func(cl *Client) { cl.authToken = strings.TrimSpace(token) }
}

// WithCaller sends X-Internal-Caller so Query can audit which peer service
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
// the Query Engine; do NOT include /internal/query — methods build full paths.
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

// do is the single HTTP path. POST-only because every query endpoint takes a
// LogicalQuery body.
func (c *Client) do(ctx context.Context, path string, body, out any) error {
	if c.baseURL == "" {
		return errors.New("queryclient: baseURL is empty")
	}
	u := c.baseURL + "/internal/query" + path

	var bodyBytes []byte
	if body != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return fmt.Errorf("queryclient: encode request body: %w", err)
		}
		bodyBytes = buf.Bytes()
	}

	if err := c.breaker.Allow(); err != nil {
		return fmt.Errorf("queryclient: POST %s: %w", u, err)
	}
	resp, err := httpclient.DoWithRetry(ctx, c.retryPolicy, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("queryclient: build request: %w", err)
		}
		c.setHeaders(ctx, req, len(bodyBytes) > 0)
		return req, nil
	}, c.httpClient.Do)
	c.breaker.Record(resp, err)
	if err != nil {
		return fmt.Errorf("queryclient: POST %s: %w", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeErrorResponse(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("queryclient: decode response: %w", err)
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

func decodeErrorResponse(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	var env internalapi.Error
	if len(raw) > 0 && bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		_ = json.Unmarshal(raw, &env)
	}
	if env.Error == "" {
		env.Error = strings.TrimSpace(string(raw))
		if env.Error == "" {
			env.Error = http.StatusText(resp.StatusCode)
		}
	}
	return newAPIErrorFromResponse(resp.StatusCode, env)
}
