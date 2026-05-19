package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// RetryPolicy controls short retries for transient service-to-service failures.
type RetryPolicy struct {
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

// DefaultRetryPolicy retries a request at most three times.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseBackoff: 50 * time.Millisecond,
		MaxBackoff:  250 * time.Millisecond,
	}
}

// DoWithRetry runs newRequest and do for each attempt. It retries transport
// errors and gateway/service-timeout responses.
func DoWithRetry(ctx context.Context, policy RetryPolicy, newRequest func() (*http.Request, error), do func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	attempts := policy.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	backoff := policy.BaseBackoff
	if backoff <= 0 {
		backoff = 50 * time.Millisecond
	}
	maxBackoff := policy.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = backoff
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := newRequest()
		if err != nil {
			return nil, err
		}
		resp, err := do(req)
		if !shouldRetry(resp, err) || attempt == attempts {
			return resp, err
		}
		lastErr = err
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		if err := sleepWithContext(ctx, backoff); err != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, err
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	return nil, lastErr
}

func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
	}
	if resp == nil {
		return false
	}
	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
