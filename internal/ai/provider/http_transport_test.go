package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReadResponseBodyNormal(t *testing.T) {
	t.Parallel()
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}
	body, err := readResponseBody(resp)
	if err != nil {
		t.Fatalf("readResponseBody: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("body = %q", string(body))
	}
}

func TestReadResponseBodyLargeResponse(t *testing.T) {
	t.Parallel()
	// Response at the limit (10MB) — should succeed.
	big := strings.NewReader(strings.Repeat("x", 10*1024*1024))
	resp := &http.Response{Body: io.NopCloser(big)}
	body, err := readResponseBody(resp)
	if err != nil {
		t.Fatalf("readResponseBody: %v", err)
	}
	if len(body) != 10*1024*1024 {
		t.Fatalf("body len = %d, want %d", len(body), 10*1024*1024)
	}
}

func TestReadResponseBodyTruncated(t *testing.T) {
	t.Parallel()
	// Slightly over 10MB — the LimitReader will limit it, but ReadAll
	// still returns successfully with 10MB of data.
	slightlyMore := strings.NewReader(strings.Repeat("y", 10*1024*1024+100))
	resp := &http.Response{Body: io.NopCloser(slightlyMore)}
	body, err := readResponseBody(resp)
	if err != nil {
		t.Fatalf("readResponseBody: %v", err)
	}
	if len(body) != 10*1024*1024 {
		t.Fatalf("body len = %d, want %d (truncated to 10MB)", len(body), 10*1024*1024)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestReadResponseBodyError(t *testing.T) {
	t.Parallel()
	resp := &http.Response{Body: io.NopCloser(errReader{})}
	_, err := readResponseBody(resp)
	if err == nil {
		t.Fatal("expected error from readResponseBody")
	}
	if !strings.Contains(err.Error(), "read response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecHTTPPostRetryBytesRetries503(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte(`{"error":"busy"}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	body, err := execHTTPPostRetryBytes(context.Background(), srv.Client(), httpPostSpec{
		URL: srv.URL,
	}, func(status int, respBody []byte) ([]byte, error, bool) {
		if status == http.StatusOK {
			return respBody, nil, false
		}
		apiErr := http.ErrServerClosed
		_ = respBody
		return nil, apiErr, isRetriableHTTPStatus(status)
	}, nil)
	if err != nil {
		t.Fatalf("execHTTPPostRetryBytes: %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
	if len(body) == 0 {
		t.Fatal("expected response body")
	}
}

func TestExecHTTPPostRetryBytesNoRetryOn400(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	_, err := execHTTPPostRetryBytes(context.Background(), srv.Client(), httpPostSpec{
		URL: srv.URL,
	}, func(status int, _ []byte) ([]byte, error, bool) {
		if status == http.StatusOK {
			return nil, nil, false
		}
		return nil, http.ErrServerClosed, isRetriableHTTPStatus(status)
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 400)", calls.Load())
	}
}

func TestExecHTTPPostRetryOnRetryCallback(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	var retryCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"content":"hello"}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	gen, err := execHTTPPostRetry(context.Background(), srv.Client(), httpPostSpec{
		URL: srv.URL,
	}, func(status int, body []byte) (GenerationResult, error, bool) {
		if status == http.StatusOK {
			return GenerationResult{Content: string(body)}, nil, false
		}
		return GenerationResult{}, http.ErrServerClosed, isRetriableHTTPStatus(status)
	}, func() { retryCalls.Add(1) })
	if err != nil {
		t.Fatalf("execHTTPPostRetry: %v", err)
	}
	if gen.Content != `{"content":"hello"}` {
		t.Fatalf("content = %q", gen.Content)
	}
	if retryCalls.Load() != 1 {
		t.Fatalf("retry calls = %d, want 1", retryCalls.Load())
	}
}
