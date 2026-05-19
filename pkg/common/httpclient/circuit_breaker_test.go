package httpclient

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	t.Parallel()
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDuration:     time.Minute,
	})
	breaker.Record(&http.Response{StatusCode: http.StatusServiceUnavailable}, nil)
	if err := breaker.Allow(); err != nil {
		t.Fatalf("Allow() after first failure: %v", err)
	}
	breaker.Record(&http.Response{StatusCode: http.StatusBadGateway}, nil)
	if err := breaker.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Allow() after threshold: got %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreakerResetsOnSuccess(t *testing.T) {
	t.Parallel()
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDuration:     time.Minute,
	})
	breaker.Record(&http.Response{StatusCode: http.StatusServiceUnavailable}, nil)
	breaker.Record(&http.Response{StatusCode: http.StatusOK}, nil)
	breaker.Record(&http.Response{StatusCode: http.StatusServiceUnavailable}, nil)
	if err := breaker.Allow(); err != nil {
		t.Fatalf("Allow() should remain closed after success reset: %v", err)
	}
}
