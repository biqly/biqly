package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/go-chi/chi/v5"
)

func TestInternalIntegration_Endpoints(t *testing.T) {
	encDSN := mustEncryptDSN(t, plaintextProbeDSN)
	catalog := integrationCatalogFixture(encDSN)
	env := newInternalIntegrationEnv(t, catalog, integrationQueryRunner{})

	t.Run("GET /internal/health", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/health", nil, integrationToken, "catalog")
		assertStatus(t, rec, http.StatusOK)
		raw := rec.Body.Bytes()
		assertGoldenJSON(t, "health.json", raw)
		var body internalapi.HealthResponse
		decodeJSONBytes(t, raw, &body)
		if body.Status != "ok" || body.Service == "" {
			t.Fatalf("health body: %+v", body)
		}
	})

	t.Run("GET /internal/models/{id} happy", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/models/"+integrationModel, nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		var body semantic.SemanticModel
		decodeJSONBody(t, rec, &body)
		if body.ID != integrationModel || len(body.Dimensions) == 0 {
			t.Fatalf("model: %+v", body)
		}
	})

	t.Run("GET /internal/models/{id} 404", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/models/missing", nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusNotFound)
		raw := rec.Body.Bytes()
		assertGoldenJSON(t, "model_not_found.json", raw)
		assertAPIErrorBytes(t, raw, internalapi.CodeNotFound)
	})

	t.Run("GET /internal/models", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/models?datasource_id="+integrationDSID, nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		var body []semantic.SemanticModel
		decodeJSONBody(t, rec, &body)
		if len(body) != 1 || body[0].ID != integrationModel {
			t.Fatalf("models: %+v", body)
		}
	})

	t.Run("GET /internal/datasources/{id}", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/datasources/"+integrationDSID, nil, integrationToken, "query")
		assertStatus(t, rec, http.StatusOK)
		raw := rec.Body.String()
		if strings.Contains(raw, plaintextProbeDSN) || strings.Contains(raw, "supersecret") {
			t.Fatalf("response leaks plaintext DSN: %s", raw)
		}
		var body metadata.Datasource
		decodeJSONBody(t, rec, &body)
		if body.ID != integrationDSID || body.Type != "postgres" {
			t.Fatalf("datasource: %+v", body)
		}
	})

	t.Run("GET /internal/datasources/{id}/tables", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/datasources/"+integrationDSID+"/tables?datasource_id="+integrationDSID, nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		var body []metadata.Table
		decodeJSONBody(t, rec, &body)
		if len(body) != 1 || body[0].TableName != "orders" {
			t.Fatalf("tables: %+v", body)
		}
	})

	t.Run("GET /internal/datasources/{id}/columns", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/datasources/"+integrationDSID+"/columns?datasource_id="+integrationDSID, nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		var body []metadata.Column
		decodeJSONBody(t, rec, &body)
		if len(body) != 1 || body[0].ColumnName != "id" {
			t.Fatalf("columns: %+v", body)
		}
	})

	t.Run("GET /internal/datasources/{id}/relations", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/datasources/"+integrationDSID+"/relations?datasource_id="+integrationDSID, nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		var body []metadata.Relation
		decodeJSONBody(t, rec, &body)
		if len(body) != 1 || body[0].FromTable != "orders" {
			t.Fatalf("relations: %+v", body)
		}
	})

	t.Run("GET /internal/few-shot", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/few-shot?datasource_id="+integrationDSID+"&model_id="+integrationModel, nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		var body []metadata.FewShotCuratedRow
		decodeJSONBody(t, rec, &body)
		if len(body) != 1 || body[0].Question == "" {
			t.Fatalf("few-shot: %+v", body)
		}
	})

	t.Run("GET /internal/glossary", func(t *testing.T) {
		rec := env.do(t, http.MethodGet, "/internal/glossary?datasource_id="+integrationDSID+"&model_id="+integrationModel, nil, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		var body []metadata.BusinessGlossaryRow
		decodeJSONBody(t, rec, &body)
		if len(body) != 1 || body[0].Term != "revenue" {
			t.Fatalf("glossary: %+v", body)
		}
	})

	t.Run("POST /internal/history/ai validation", func(t *testing.T) {
		rec := env.do(t, http.MethodPost, "/internal/history/ai", []byte(`{"entry":{}}`), integrationToken, "ai")
		assertStatus(t, rec, http.StatusBadRequest)
		assertAPIError(t, rec, internalapi.CodeInvalidRequest)
	})

	t.Run("POST /internal/history/ai", func(t *testing.T) {
		rec := env.do(t, http.MethodPost, "/internal/history/ai", []byte(`{"entry":{"datasource_id":"`+integrationDSID+`","question":"q"}}`), integrationToken, "ai")
		assertStatus(t, rec, http.StatusCreated)
		var body internalapi.AIHistoryResponse
		decodeJSONBody(t, rec, &body)
		if body.ID == "" {
			t.Fatalf("ai history id empty: %+v", body)
		}
	})

	t.Run("POST /internal/history/query", func(t *testing.T) {
		payload, _ := json.Marshal(internalapi.QueryHistoryRequest{
			Entry: query.HistoryEntry{DatasourceID: integrationDSID, Status: "success"},
		})
		rec := env.do(t, http.MethodPost, "/internal/history/query", payload, integrationToken, "query")
		assertStatus(t, rec, http.StatusCreated)
		var body internalapi.QueryHistoryResponse
		decodeJSONBody(t, rec, &body)
		if body.ID == "" {
			t.Fatalf("query history id empty: %+v", body)
		}
	})

	t.Run("POST /internal/eval-results", func(t *testing.T) {
		payload, _ := json.Marshal(internalapi.EvalResultsRequest{
			RunID: "run_1", Provider: "openai", Model: "gpt-4o",
			Results: []ai.EvalResultWithMetrics{},
		})
		rec := env.do(t, http.MethodPost, "/internal/eval-results", payload, integrationToken, "ai")
		assertStatus(t, rec, http.StatusCreated)
		var body internalapi.EvalResultsResponse
		decodeJSONBody(t, rec, &body)
		if body.RunID != "run_1" || body.TotalCases != 0 {
			t.Fatalf("eval results: %+v", body)
		}
	})

	t.Run("POST /internal/query/compile", func(t *testing.T) {
		payload, _ := json.Marshal(internalapi.CompileRequest{LogicalQuery: integrationLogicalQuery()})
		rec := env.do(t, http.MethodPost, "/internal/query/compile", payload, integrationToken, "ai")
		assertStatus(t, rec, http.StatusOK)
		raw := rec.Body.Bytes()
		assertGoldenJSON(t, "compile_ok.json", raw)
		var body internalapi.CompileResponse
		decodeJSONBytes(t, raw, &body)
		if body.SQL == "" || body.Fingerprint == "" {
			t.Fatalf("compile: %+v", body)
		}
	})

	t.Run("POST /internal/query/run", func(t *testing.T) {
		payload, _ := json.Marshal(internalapi.RunRequest{LogicalQuery: integrationLogicalQuery()})
		rec := env.do(t, http.MethodPost, "/internal/query/run", payload, integrationToken, "query")
		assertStatus(t, rec, http.StatusOK)
		var body internalapi.RunResponse
		decodeJSONBody(t, rec, &body)
		if body.RowCount != 1 || body.SQL == "" {
			t.Fatalf("run: %+v", body)
		}
	})

	t.Run("POST /internal/query/dry-run", func(t *testing.T) {
		payload, _ := json.Marshal(internalapi.DryRunRequest{LogicalQuery: integrationLogicalQuery()})
		rec := env.do(t, http.MethodPost, "/internal/query/dry-run", payload, integrationToken, "query")
		assertStatus(t, rec, http.StatusOK)
		var body internalapi.DryRunResponse
		decodeJSONBody(t, rec, &body)
		if body.SQL == "" || body.Fingerprint == "" {
			t.Fatalf("dry-run: %+v", body)
		}
	})
}

func TestInternalIntegration_AuthMiddleware(t *testing.T) {
	env := newInternalIntegrationEnv(t, integrationCatalogFixture("enc"), integrationQueryRunner{})

	rec := env.do(t, http.MethodGet, "/internal/models/"+integrationModel, nil, "", "ai")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: got %d want 401", rec.Code)
	}
	assertAPIError(t, rec, internalapi.CodeUnauthorized)

	unset := newInternalIntegrationEnvUnsetToken(t, integrationCatalogFixture("enc"), integrationQueryRunner{})
	rec = unset.do(t, http.MethodGet, "/internal/health", nil, "", "ai")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unset token: got %d want 403", rec.Code)
	}
	assertAPIError(t, rec, internalapi.CodeUnauthorized)
}

func TestInternalIntegration_AuditMiddleware(t *testing.T) {
	env := newInternalIntegrationEnv(t, integrationCatalogFixture("enc"), integrationQueryRunner{})
	rec := env.do(t, http.MethodGet, "/internal/health", nil, integrationToken, "ai")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if !strings.Contains(env.audit.String(), `"event_type":"internal_request"`) {
		t.Fatalf("audit log missing internal_request: %s", env.audit.String())
	}
}

func newInternalIntegrationEnvUnsetToken(t *testing.T, catalog integrationCatalog, queryRunner internalQueryRunner) *internalIntegrationEnv {
	t.Helper()
	var auditBuf bytes.Buffer
	auditLogger := audit.NewLogger(slog.New(slog.NewJSONHandler(&auditBuf, nil)))
	internalHandler := &InternalHandler{meta: catalog, semantic: catalog, eval: integrationEvalRepo{}}
	r := chi.NewRouter()
	r.Route("/internal", func(r chi.Router) {
		r.Use(InternalAuditMiddleware(auditLogger))
		r.Use(InternalTokenMiddleware(""))
		r.Get("/health", internalHandler.Health)
		r.Get("/models/{id}", internalHandler.GetFullModel)
	})
	return &internalIntegrationEnv{handler: r, audit: &auditBuf}
}

func (e *internalIntegrationEnv) do(t *testing.T, method, path string, body []byte, token, caller string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequestWithContext(context.Background(), method, path, nil)
	}
	if token != "" {
		r.Header.Set("X-Internal-Token", token)
	}
	if caller != "" {
		r.Header.Set("X-Internal-Caller", caller)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, r)
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status: got %d want %d body=%s", rec.Code, want, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type: got %q want application/json", ct)
	}
}

func assertAPIError(t *testing.T, rec *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	assertAPIErrorBytes(t, rec.Body.Bytes(), wantCode)
}

func assertAPIErrorBytes(t *testing.T, raw []byte, wantCode string) {
	t.Helper()
	var env internalapi.Error
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode error envelope: %v body=%s", err, string(raw))
	}
	if env.Code != wantCode {
		t.Fatalf("code: got %q want %q", env.Code, wantCode)
	}
	if env.Error == "" {
		t.Fatal("error message empty")
	}
}

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	decodeJSONBytes(t, rec.Body.Bytes(), dst)
}

func decodeJSONBytes(t *testing.T, raw []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode: %v body=%s", err, string(raw))
	}
}

func assertGoldenJSON(t *testing.T, name string, got []byte) {
	t.Helper()
	path := "testdata/internal_golden/" + name
	var gotNorm bytes.Buffer
	if err := json.Compact(&gotNorm, got); err != nil {
		t.Fatalf("compact got: %v", err)
	}
	if os.Getenv("UPDATE_INTERNAL_GOLDEN") == "1" {
		if err := os.WriteFile(path, append(gotNorm.Bytes(), '\n'), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	var wantNorm bytes.Buffer
	if err := json.Compact(&wantNorm, want); err != nil {
		t.Fatalf("compact want: %v", err)
	}
	if !bytes.Equal(gotNorm.Bytes(), wantNorm.Bytes()) {
		t.Fatalf("golden %s mismatch:\ngot:  %s\nwant: %s", name, gotNorm.String(), wantNorm.String())
	}
}

func mustEncryptDSN(t *testing.T, plaintext string) string {
	t.Helper()
	enc, err := security.NewEncryptionWithKey([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("encryption: %v", err)
	}
	out, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return out
}
