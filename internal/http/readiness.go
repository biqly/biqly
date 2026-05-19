package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/app"
)

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
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func readinessDBCheck(ctx context.Context, deps *app.Dependencies) readinessCheck {
	if deps == nil || deps.MetadataDB == nil {
		return readinessCheck{Status: "ok"}
	}
	if err := deps.MetadataDB.PingContext(ctx); err != nil {
		return readinessCheck{Status: "error", Error: err.Error()}
	}
	return readinessCheck{Status: "ok"}
}

func readinessHTTPCheck(ctx context.Context, baseURL string) readinessCheck {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/health", nil)
	if err != nil {
		return readinessCheck{Status: "error", Error: err.Error()}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return readinessCheck{Status: "error", Error: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readinessCheck{Status: "error", Error: resp.Status}
	}
	return readinessCheck{Status: "ok"}
}
