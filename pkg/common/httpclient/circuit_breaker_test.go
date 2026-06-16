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

func TestCircuitBreakerAllowsSingleHalfOpenProbe(t *testing.T) {
	t.Parallel()
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenDuration:     10 * time.Millisecond,
	})
	breaker.Record(&http.Response{StatusCode: http.StatusServiceUnavailable}, nil)
	time.Sleep(20 * time.Millisecond)

	if err := breaker.Allow(); err != nil {
		t.Fatalf("first half-open probe Allow(): %v", err)
	}
	if err := breaker.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("second half-open probe Allow() = %v, want ErrCircuitOpen", err)
	}

	breaker.Record(&http.Response{StatusCode: http.StatusOK}, nil)
	if err := breaker.Allow(); err != nil {
		t.Fatalf("Allow() after successful half-open probe: %v", err)
	}
}

func TestCircuitBreakerReopensOnHalfOpenFailure(t *testing.T) {
	t.Parallel()
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenDuration:     10 * time.Millisecond,
	})
	breaker.Record(&http.Response{StatusCode: http.StatusServiceUnavailable}, nil)
	time.Sleep(20 * time.Millisecond)

	if err := breaker.Allow(); err != nil {
		t.Fatalf("half-open probe Allow(): %v", err)
	}
	breaker.Record(&http.Response{StatusCode: http.StatusBadGateway}, nil)
	if err := breaker.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Allow() after failed half-open probe = %v, want ErrCircuitOpen", err)
	}
}
