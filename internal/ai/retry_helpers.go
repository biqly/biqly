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

// execLLMHTTPRetry runs op with exponential backoff between attempts (same policy as before:
// up to llmHTTPMaxAttempts tries; op returns retry=true for transient HTTP/network failures).
func execLLMHTTPRetry(ctx context.Context, op func() (content string, err error, retry bool)) (string, error) {
	var lastErr error
	for attempt := range llmHTTPMaxAttempts {
		if attempt > 0 {
			delay := time.Duration(250*(1<<uint(attempt-1))) * time.Millisecond
			if err := sleepCtx(ctx, delay); err != nil {
				return "", err
			}
		}
		content, err, retry := op()
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !retry {
			return "", err
		}
	}
	return "", fmt.Errorf("send request: %w", lastErr)
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
