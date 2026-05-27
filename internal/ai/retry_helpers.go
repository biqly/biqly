package ai

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"syscall"
	"time"
)

const llmHTTPMaxAttempts = 4

// execRetry runs op with exponential backoff (up to llmHTTPMaxAttempts). op
// returns retry=true for transient HTTP/network failures. Backoff is
// 250ms × 2^(attempt-1) with ±25% uniform jitter so simultaneously failing
// callers do not synchronize a retry storm against the upstream.
func execRetry[T any](ctx context.Context, op func() (T, error, bool)) (T, error) {
	var zero T
	var lastErr error
	for attempt := range llmHTTPMaxAttempts {
		if attempt > 0 {
			delay := jitteredBackoff(attempt)
			if err := sleepCtx(ctx, delay); err != nil {
				return zero, err
			}
		}
		result, err, retry := op()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !retry {
			return zero, err
		}
	}
	return zero, fmt.Errorf("send request: %w", lastErr)
}

// jitteredBackoff returns the per-attempt delay: 250ms × 2^(attempt-1)
// multiplied by a uniform random factor in [0.75, 1.25].
func jitteredBackoff(attempt int) time.Duration {
	base := time.Duration(250*(1<<uint(attempt-1))) * time.Millisecond
	// factor ∈ [0.75, 1.25)
	factor := 0.75 + cryptoRandomUnitFloat()*0.5
	return time.Duration(float64(base) * factor)
}

func cryptoRandomUnitFloat() float64 {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return 0.5
	}
	return float64(n.Int64()) / 1_000_000
}

func execLLMHTTPRetry(ctx context.Context, op func() (GenerationResult, error, bool)) (GenerationResult, error) {
	return execRetry(ctx, op)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isRetriableHTTPStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetriableNetErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}
