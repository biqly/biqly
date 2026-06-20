package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestMain(m *testing.M) {
	// Mirror SetupTracing: a W3C propagator and a recording provider so the
	// otelhttp transport injects trace context on outbound requests.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	code := m.Run()
	_ = tp.Shutdown(context.Background())
	os.Exit(code)
}

func TestNewServiceTransportTuning(t *testing.T) {
	t.Parallel()
	transport := NewServiceTransport()
	if transport.MaxIdleConns != 100 {
		t.Fatalf("MaxIdleConns: got %d, want 100", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 20 {
		t.Fatalf("MaxIdleConnsPerHost: got %d, want 20", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Fatalf("IdleConnTimeout: got %s, want 90s", transport.IdleConnTimeout)
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("ResponseHeaderTimeout: got %s, want 30s", transport.ResponseHeaderTimeout)
	}
}

func TestNewServiceClientPropagatesTraceContext(t *testing.T) {
	t.Parallel()
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"

	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("traceparent")
	}))
	defer srv.Close()

	// Simulate an inbound request that already carries a trace; the outbound
	// call must continue the same trace (same trace-id, fresh client span-id).
	ctx := propagation.TraceContext{}.Extract(context.Background(),
		propagation.MapCarrier{"traceparent": "00-" + traceID + "-00f067aa0ba902b7-01"})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := NewServiceClient().Do(req)
	if err != nil {
		t.Fatalf("Do(): %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}

	if !strings.HasPrefix(got, "00-"+traceID+"-") {
		t.Fatalf("traceparent: got %q, want trace-id %s propagated", got, traceID)
	}
}

func TestDoWithRetryRetriesTransientStatuses(t *testing.T) {
	t.Parallel()
	attempts := 0
	resp, err := DoWithRetry(
		context.Background(),
		RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Nanosecond, MaxBackoff: time.Nanosecond},
		func() (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test", http.NoBody)
		},
		func(_ *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(nilReader{})}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(nilReader{})}, nil
		},
	)
	if err != nil {
		t.Fatalf("DoWithRetry() error: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("close response body: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK || attempts != 3 {
		t.Fatalf("status/attempts: got %d/%d, want 200/3", resp.StatusCode, attempts)
	}
}

func TestDoWithRetryDoesNotRetryContextCancellation(t *testing.T) {
	t.Parallel()
	attempts := 0
	resp, err := DoWithRetry(
		context.Background(),
		RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Nanosecond, MaxBackoff: time.Nanosecond},
		func() (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test", http.NoBody)
		},
		func(_ *http.Request) (*http.Response, error) {
			attempts++
			return nil, context.Canceled
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error: got %v, want context.Canceled", err)
	}
	if resp != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Fatalf("close response body: %v", err)
			}
		}()
	}
	if attempts != 1 {
		t.Fatalf("attempts: got %d, want 1", attempts)
	}
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp *http.Response
		err  error
		want bool
	}{
		{name: "transport error retries", err: io.ErrUnexpectedEOF, want: true},
		{name: "context canceled does not retry", err: context.Canceled, want: false},
		{name: "deadline exceeded does not retry", err: context.DeadlineExceeded, want: false},
		{name: "bad gateway retries", resp: &http.Response{StatusCode: http.StatusBadGateway}, want: true},
		{name: "service unavailable retries", resp: &http.Response{StatusCode: http.StatusServiceUnavailable}, want: true},
		{name: "gateway timeout retries", resp: &http.Response{StatusCode: http.StatusGatewayTimeout}, want: true},
		{name: "too many requests does not retry", resp: &http.Response{StatusCode: http.StatusTooManyRequests}, want: false},
		{name: "ok does not retry", resp: &http.Response{StatusCode: http.StatusOK}, want: false},
		{name: "nil response and nil error does not retry", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetry(tt.resp, tt.err); got != tt.want {
				t.Fatalf("shouldRetry(%v, %v) = %v, want %v", tt.resp, tt.err, got, tt.want)
			}
		})
	}
}

type nilReader struct{}

func (nilReader) Read(_ []byte) (int, error) { return 0, io.EOF }
