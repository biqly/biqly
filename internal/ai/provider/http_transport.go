package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func readResponseBody(resp *http.Response) ([]byte, error) {
	limited := io.LimitReader(resp.Body, maxResponseSize)
	body, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, fmt.Errorf("read response: %w", readErr)
	}
	return body, nil
}

type httpPostSpec struct {
	URL     string
	Headers map[string]string
	Body    []byte
}

func execHTTPPost(ctx context.Context, client *http.Client, spec httpPostSpec) (status int, body []byte, err error) {
	if err := CheckEgress(spec.URL); err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spec.URL, bytes.NewReader(spec.Body))
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	respBody, err := readResponseBody(resp)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBody, nil
}

func execHTTPPostRetry(ctx context.Context, client *http.Client, spec httpPostSpec, handle func(status int, body []byte) (GenerationResult, error, bool), onRetry func()) (GenerationResult, error) {
	return execLLMHTTPRetry(ctx, func() (GenerationResult, error, bool) {
		status, respBody, err := execHTTPPost(ctx, client, spec)
		if err != nil {
			return GenerationResult{}, err, isRetriableNetErr(err)
		}
		return handle(status, respBody)
	}, onRetry)
}

func execHTTPPostRetryBytes(ctx context.Context, client *http.Client, spec httpPostSpec, handle func(status int, body []byte) ([]byte, error, bool), onRetry func()) ([]byte, error) {
	return execRetry(ctx, func() ([]byte, error, bool) {
		status, respBody, err := execHTTPPost(ctx, client, spec)
		if err != nil {
			return nil, err, isRetriableNetErr(err)
		}
		return handle(status, respBody)
	}, onRetry)
}
