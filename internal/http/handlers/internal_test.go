package handlers

import (
	"bytes"
	"context"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/stretchr/testify/require"
)

// TestInternalHealth covers the health endpoint independently of any
// downstream wiring: deps may be nil because Health never reads them.
func TestInternalHealth(t *testing.T) {
	t.Parallel()
	h := &InternalHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/internal/health", nil)
	h.Health(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var body internalapi.HealthResponse
	if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status field: got %q, want ok", body.Status)
	}
	if body.Service == "" {
		t.Errorf("service field should be set for log correlation")
	}
}

// TestRequireInternalQueryParam verifies the helper returns ok=false and writes a
// 400 with the canonical envelope when the parameter is missing or blank.
func TestRequireInternalQueryParam(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		rawURL  string
		paramOK bool
	}{
		{"missing", "/x", false},
		{"empty", "/x?datasource_id=", false},
		{"blank", "/x?datasource_id=%20", false},
		{"ok", "/x?datasource_id=ds_1", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.rawURL, nil)
			_, ok := requireInternalQueryParam(w, r, "datasource_id")
			if ok != tt.paramOK {
				t.Errorf("ok: got %v, want %v", ok, tt.paramOK)
			}
			if !tt.paramOK {
				if w.Code != http.StatusBadRequest {
					t.Errorf("status: got %d, want 400", w.Code)
				}
				var env internalapi.Error
				if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&env); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if env.Code != internalapi.CodeInvalidRequest {
					t.Errorf("code: got %q, want %q", env.Code, internalapi.CodeInvalidRequest)
				}
				if !strings.Contains(env.Error, "datasource_id") {
					t.Errorf("error should mention param name: %q", env.Error)
				}
			}
		})
	}
}

// TestRequireQueryParam verifies the general-purpose query parameter helper
// returns ok=false and writes a 400 when the parameter is missing or blank.
func TestRequireQueryParam(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		rawURL  string
		paramOK bool
	}{
		{"missing", "/x", false},
		{"empty", "/x?datasource_id=", false},
		{"blank", "/x?datasource_id=%20", false},
		{"ok", "/x?datasource_id=ds_1", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.rawURL, nil)
			v, ok := requireQueryParam(w, r, "datasource_id")
			if ok != tt.paramOK {
				t.Errorf("ok: got %v, want %v", ok, tt.paramOK)
			}
			if tt.paramOK && v != "ds_1" {
				t.Errorf("value: got %q, want ds_1", v)
			}
			if !tt.paramOK {
				if w.Code != http.StatusBadRequest {
					t.Errorf("status: got %d, want 400", w.Code)
				}
				var env struct {
					Error string `json:"error"`
				}
				if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&env); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if !strings.Contains(env.Error, "datasource_id is required") {
					t.Errorf("error: got %q, want containing 'datasource_id is required'", env.Error)
				}
			}
		})
	}
}

func assertInvalidRequestMissingField(t *testing.T, handler func(http.ResponseWriter, *http.Request), path string, body any, field string) {
	t.Helper()
	payload, err := sonic.ConfigStd.Marshal(body)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, bytes.NewReader(payload))
	handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	var env internalapi.Error
	if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != internalapi.CodeInvalidRequest {
		t.Errorf("code: got %q, want %q", env.Code, internalapi.CodeInvalidRequest)
	}
	if field != "" && !strings.Contains(env.Error, field) {
		t.Errorf("error should mention the missing field %q: %q", field, env.Error)
	}
}

// TestCreateAIHistory_RejectsMissingDatasourceID stops at the validator
// without invoking the repository, so this stays a pure-handler test.
func TestCreateAIHistory_RejectsMissingDatasourceID(t *testing.T) {
	t.Parallel()
	assertInvalidRequestMissingField(t, (&InternalHandler{}).CreateAIHistory, "/internal/history/ai", internalapi.AIHistoryRequest{}, "datasource_id")
}

// TestCreateQueryHistory_RejectsMissingDatasourceID mirrors the AI variant —
// same contract, separate path.
func TestCreateQueryHistory_RejectsMissingDatasourceID(t *testing.T) {
	t.Parallel()
	h := &InternalHandler{}
	body, err := sonic.ConfigStd.Marshal(internalapi.QueryHistoryRequest{})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/history/query", bytes.NewReader(body))
	h.CreateQueryHistory(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	var env internalapi.Error
	if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != internalapi.CodeInvalidRequest {
		t.Errorf("code: got %q, want %q", env.Code, internalapi.CodeInvalidRequest)
	}
}

func TestCreateEvalResults_RejectsMissingRunID(t *testing.T) {
	t.Parallel()
	assertInvalidRequestMissingField(t, (&InternalHandler{}).CreateEvalResults, "/internal/eval-results", internalapi.EvalResultsRequest{}, "run_id")
}

func TestCreateEvalResults_RejectsMissingProvider(t *testing.T) {
	t.Parallel()
	h := &InternalHandler{}
	body, err := sonic.ConfigStd.Marshal(internalapi.EvalResultsRequest{RunID: "run_1"})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/eval-results", bytes.NewReader(body))
	h.CreateEvalResults(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	var env internalapi.Error
	if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(env.Error, "provider") {
		t.Errorf("error should mention the missing field: %q", env.Error)
	}
}

// TestCreateAIHistory_RejectsMalformedJSON keeps invalid bodies on the 400
// path (handled by the generic decodeJSON helper). Re-asserted here so a
// future refactor of decodeJSON can't silently regress this surface.
func TestCreateAIHistory_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	h := &InternalHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/history/ai", strings.NewReader("not-json"))
	h.CreateAIHistory(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
}

// TestWriteInternalAPIErrorMsg covers the wire format directly so the
// envelope contract is asserted independently of any specific endpoint.
func TestWriteInternalAPIErrorMsg(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeInternalAPIErrorMsg(w, http.StatusNotFound, internalapi.CodeNotFound, "thing gone")

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("content-type: got %q, want application/json", ct)
	}
	var env internalapi.Error
	if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != internalapi.CodeNotFound || env.Error != "thing gone" {
		t.Errorf("envelope: %+v", env)
	}
}
