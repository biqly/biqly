package httpclient

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when a client is temporarily refusing calls after
// repeated transient failures.
var ErrCircuitOpen = errors.New("httpclient: circuit open")

// CircuitBreakerConfig controls a small consecutive-failure circuit breaker.
type CircuitBreakerConfig struct {
	FailureThreshold int
	OpenDuration     time.Duration
}

// DefaultCircuitBreakerConfig returns conservative service-client defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDuration:     30 * time.Second,
	}
}

// CircuitBreaker tracks transient failures for one upstream.
type CircuitBreaker struct {
	mu               sync.Mutex
	failureThreshold int
	openDuration     time.Duration
	failures         int
	openUntil        time.Time
	halfOpenInFlight bool
}

// NewCircuitBreaker creates a circuit breaker. Non-positive values use defaults.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	defaults := DefaultCircuitBreakerConfig()
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = defaults.FailureThreshold
	}
	if cfg.OpenDuration <= 0 {
		cfg.OpenDuration = defaults.OpenDuration
	}
	return &CircuitBreaker{
		failureThreshold: cfg.FailureThreshold,
		openDuration:     cfg.OpenDuration,
	}
}

// Allow returns ErrCircuitOpen while the breaker is open.
func (b *CircuitBreaker) Allow() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if now.Before(b.openUntil) {
		return ErrCircuitOpen
	}
	if !b.openUntil.IsZero() {
		if b.halfOpenInFlight {
			return ErrCircuitOpen
		}
		b.halfOpenInFlight = true
		return nil
	}
	if b.halfOpenInFlight {
		return ErrCircuitOpen
	}
	return nil
}

// Record updates breaker state after a response.
func (b *CircuitBreaker) Record(resp *http.Response, err error) {
	if b == nil {
		return
	}
	if shouldRetry(resp, err) {
		b.recordFailure()
		return
	}
	b.recordSuccess()
}

func (b *CircuitBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.halfOpenInFlight {
		b.halfOpenInFlight = false
		b.failures = b.failureThreshold
		b.openUntil = time.Now().Add(b.openDuration)
		return
	}
	b.failures++
	if b.failures >= b.failureThreshold {
		b.openUntil = time.Now().Add(b.openDuration)
	}
}

func (b *CircuitBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openUntil = time.Time{}
	b.halfOpenInFlight = false
}
