// Package aiclient — see doc.go.
package aiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/biqly/biqly/pkg/common/httpclient"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/biqly/biqly/pkg/internalapi"
)

// v0 out-of-scope public endpoints (examples, feedback, glossary, usage, eval).
// Track in docs/microservice-migration-checklist when adding methods here.

const defaultUserAgent = "biqly-aiclient/0.1"

// HTTPDoer is the minimum surface this package needs from net/http.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client talks to the AI Service over /api/ai/*. It is safe for concurrent use
// by multiple goroutines once constructed.
type Client struct {
	baseURL     string
	httpClient  HTTPDoer
	authToken   string
	caller      string
	userAgent   string
	retryPolicy httpclient.RetryPolicy
	breaker     *httpclient.CircuitBreaker
}

// Option configures a Client at construction time.
type Option func(*Client)

// WithHTTPClient overrides the underlying transport.
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

// WithCaller sends X-Internal-Caller so the AI service can audit which peer
// initiated the request (for example "query" or "catalog").
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

// New returns a ready-to-use Client. baseURL is the scheme://host[:port] of the
// AI Service; do NOT include /api/ai — methods build full paths themselves.
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

// BaseURL exposes the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if c.baseURL == "" {
		return errors.New("aiclient: baseURL is empty")
	}
	u := c.baseURL + "/api/ai" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var bodyBytes []byte
	if body != nil {
		buf := &bytes.Buffer{}
		if err := sonic.ConfigStd.NewEncoder(buf).Encode(body); err != nil {
			return fmt.Errorf("aiclient: encode request body: %w", err)
		}
		bodyBytes = buf.Bytes()
	}

	if err := c.breaker.Allow(); err != nil {
		return fmt.Errorf("aiclient: %s %s: %w", method, u, err)
	}
	resp, err := httpclient.DoWithRetry(ctx, c.retryPolicy, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("aiclient: build request: %w", err)
		}
		c.setHeaders(ctx, req, len(bodyBytes) > 0)
		return req, nil
	}, c.httpClient.Do)
	c.breaker.Record(resp, err)
	if err != nil {
		return fmt.Errorf("aiclient: %s %s: %w", method, u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeErrorResponse(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return fmt.Errorf("aiclient: drain response body: %w", err)
		}
		return nil
	}
	if err := sonic.ConfigStd.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("aiclient: decode response: %w", err)
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

// legacyErrorEnvelope is the older public /api/ai error shape from writeError.
type legacyErrorEnvelope struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func decodeErrorResponse(resp *http.Response) error {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return fmt.Errorf("aiclient: read error response: %w", err)
	}
	var env internalapi.Error
	if len(raw) > 0 && bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		if err := sonic.ConfigStd.Unmarshal(raw, &env); err != nil {
			env = internalapi.Error{}
		}
	}
	if env.Code == "" {
		var legacy legacyErrorEnvelope
		if err := sonic.ConfigStd.Unmarshal(raw, &legacy); err != nil {
			legacy = legacyErrorEnvelope{}
		}
		if legacy.Error != "" {
			env.Error = legacy.Error
		}
		if env.Error == "" && legacy.Message != "" {
			env.Error = legacy.Message
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

func clarificationFromResponse(resp *QueryResponse) error {
	if resp != nil && resp.Clarification != nil && resp.Clarification.NeedsClarification {
		return newClarificationError(resp)
	}
	return nil
}
