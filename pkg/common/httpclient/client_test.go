package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestNewServiceClientUsesServiceTransport(t *testing.T) {
	t.Parallel()
	client := NewServiceClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type: got %T, want *http.Transport", client.Transport)
	}
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

type nilReader struct{}

func (nilReader) Read(_ []byte) (int, error) { return 0, io.EOF }
