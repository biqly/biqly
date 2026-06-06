package http

import (
	"context"
	"errors"
	"github.com/bytedance/sonic"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/app"
)

// readinessHTTPClient talks to upstream /health endpoints. Configured to
// refuse redirects entirely — a probe of an internal service should never
// be silently forwarded somewhere else (SSRF surface). Total wall-clock is
// bounded by the request context (~2s) so a hung upstream cannot stall the
// readiness handler.
var readinessHTTPClient = &http.Client{
	Timeout: 2 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type readinessResponse struct {
	Status string                    `json:"status"`
	Checks map[string]readinessCheck `json:"checks"`
}

type readinessCheck struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ReadinessHandler reports whether this process can serve traffic.
func ReadinessHandler(deps *app.Dependencies, upstreams map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		resp := readinessResponse{
			Status: "ok",
			Checks: map[string]readinessCheck{
				"metadata_db": readinessDBCheck(ctx, deps),
			},
		}
		for name, baseURL := range upstreams {
			if strings.TrimSpace(baseURL) == "" {
				continue
			}
			resp.Checks[name] = readinessHTTPCheck(ctx, baseURL)
		}

		statusCode := http.StatusOK
		for _, check := range resp.Checks {
			if check.Status != "ok" {
				resp.Status = "degraded"
				statusCode = http.StatusServiceUnavailable
				break
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = sonic.ConfigStd.NewEncoder(w).Encode(resp)
	}
}

func readinessDBCheck(ctx context.Context, deps *app.Dependencies) readinessCheck {
	if deps == nil || deps.MetadataDB == nil {
		return readinessCheck{Status: "ok"}
	}
	if err := deps.MetadataDB.PingContext(ctx); err != nil {
		return readinessCheck{Status: "error", Error: redactReadinessError(err)}
	}
	return readinessCheck{Status: "ok"}
}

func readinessHTTPCheck(ctx context.Context, baseURL string) readinessCheck {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/health", http.NoBody)
	if err != nil {
		return readinessCheck{Status: "error", Error: "invalid upstream URL"}
	}
	resp, err := readinessHTTPClient.Do(req)
	if err != nil {
		return readinessCheck{Status: "error", Error: redactReadinessError(err)}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readinessCheck{Status: "error", Error: resp.Status}
	}
	return readinessCheck{Status: "ok"}
}

// redactReadinessError returns a coarse error category for the readiness
// JSON response. The full error text contains hostnames, ports, and TLS
// chain details that should not leak to anonymous /ready callers — they
// live in slog/stderr instead.
func redactReadinessError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "timeout"
	default:
		return "unavailable"
	}
}
