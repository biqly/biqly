package http

import (
	"bytes"
	"context"
	"github.com/bytedance/sonic"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
)

func TestRouter_InternalRoutesWriteAuditLog(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Security: config.SecurityConfig{InternalAPIToken: "secret-token"},
		},
		AuditLogger: audit.NewLogger(slog.New(slog.NewJSONHandler(&logs, nil))),
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/internal/health", nil)
	r.Header.Set("X-Internal-Token", "secret-token")
	r.Header.Set("X-Internal-Caller", "ai")
	r.Header.Set("traceparent", sampleAuditTraceparent)
	handler.ServeHTTP(w, r)

	if w.Code != stdhttp.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if !strings.Contains(logs.String(), `"event_type":"internal_request"`) {
		t.Fatalf("audit log missing event_type: %s", logs.String())
	}
	var entry struct {
		Details map[string]any `json:"details"`
	}
	if err := sonic.ConfigStd.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if entry.Details["source"] != "service" {
		t.Fatalf("source: got %#v, want service", entry.Details["source"])
	}
	if entry.Details["caller"] != "ai" {
		t.Fatalf("caller: got %#v, want ai", entry.Details["caller"])
	}
	if entry.Details["path"] != "/internal/health" {
		t.Fatalf("path: got %#v, want /internal/health", entry.Details["path"])
	}
	if entry.Details["status"] != float64(stdhttp.StatusOK) {
		t.Fatalf("status: got %#v, want 200", entry.Details["status"])
	}
	if entry.Details["request_id"] == "" {
		t.Fatalf("request_id should be recorded: %+v", entry.Details)
	}
	if entry.Details["traceparent"] != sampleAuditTraceparent {
		t.Fatalf("traceparent: got %#v, want %q", entry.Details["traceparent"], sampleAuditTraceparent)
	}
}

const sampleAuditTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
