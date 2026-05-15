package ai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

const llmHTTPMaxAttempts = 4

// execRetry runs op with exponential backoff (up to llmHTTPMaxAttempts). op returns
// retry=true for transient HTTP/network failures.
func execRetry[T any](ctx context.Context, op func() (T, error, bool)) (T, error) {
	var zero T
	var lastErr error
	for attempt := range llmHTTPMaxAttempts {
		if attempt > 0 {
			delay := time.Duration(250*(1<<uint(attempt-1))) * time.Millisecond
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
	if errors.As(err, &opErr) {
		return true
	}
	return false
}
