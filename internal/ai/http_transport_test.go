package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestExecHTTPPostRetryBytesRetries503(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"busy"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]}]}`))
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
	})
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
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 400)", calls.Load())
	}
}
