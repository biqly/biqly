package handlers

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	internalsemantic "github.com/biqly/biqly/internal/semantic"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

func newDBTImportRequest(t *testing.T, datasourceID string, files map[string]string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, content := range files {
		part, err := writer.CreateFormFile(name, name)
		if err != nil {
			t.Fatalf("CreateFormFile(%q): %v", name, err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	path := "/api/catalog/dbt/import"
	if datasourceID != "" {
		path += "?datasource_id=" + datasourceID
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func assertDBTImportBadRequest(t *testing.T, req *http.Request, wantError string) {
	t.Helper()

	rec := httptest.NewRecorder()
	(&SemanticHandler{}).ImportDbtProject(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ImportDbtProject(%s) status = %d, want %d; body = %s", req.URL, rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), wantError) {
		t.Errorf("ImportDbtProject(%s) error body = %q, want containing %q", req.URL, rec.Body.String(), wantError)
	}
}

func TestImportDbtProjectRejectsMultipartRequestWithoutDatasourceID(t *testing.T) {
	t.Parallel()

	assertDBTImportBadRequest(t, newDBTImportRequest(t, "", map[string]string{
		"manifest": `{"nodes":{}}`,
	}), "datasource_id is required")
}

func TestImportDbtProjectRejectsMultipartRequestWithoutManifest(t *testing.T) {
	t.Parallel()

	assertDBTImportBadRequest(t, newDBTImportRequest(t, "datasource-1", map[string]string{
		"catalog": `{"nodes":{}}`,
	}), "manifest file is required")
}

func TestImportDbtProjectRejectsMalformedMultipartRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/catalog/dbt/import?datasource_id=datasource-1", strings.NewReader("not a multipart body"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")

	assertDBTImportBadRequest(t, req, "invalid multipart request")
}

func TestImportDbtProjectRejectsInvalidManifestFile(t *testing.T) {
	t.Parallel()

	assertDBTImportBadRequest(t, newDBTImportRequest(t, "datasource-1", map[string]string{
		"manifest": `{`,
		"catalog":  `{"nodes":{}}`,
	}), "parse manifest")
}

func TestImportDbtProjectCreatesDraftModelFromMultipartFiles(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	datasourceID := uuid.NewString()
	if _, err := db.ExecContext(ctx, `INSERT INTO datasources (id, name, type, dsn_encrypted) VALUES ($1, $2, 'postgres', 'enc')`, datasourceID, "dbt-import-test"); err != nil {
		t.Fatalf("seed datasource: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM semantic_models WHERE datasource_id = $1`, datasourceID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM datasources WHERE id = $1`, datasourceID)
	})

	handler := NewSemanticHandler(&app.CatalogDeps{
		SemanticRepo: internalsemantic.NewRepository(db),
		MetaRepo:     metadata.NewRepository(db),
	})
	req := newDBTImportRequest(t, datasourceID, map[string]string{
		"manifest": `{
			"nodes": {
				"model.project.orders": {
					"unique_id": "model.project.orders",
					"resource_type": "model",
					"name": "orders",
					"schema": "public",
					"columns": {"id": {"name": "id", "data_type": "integer"}}
				}
			}
		}`,
		"catalog": `{"nodes":{"model.project.orders":{"columns":{"id":{"name":"id","type":"integer"}}}}}`,
	})
	rec := httptest.NewRecorder()

	handler.ImportDbtProject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ImportDbtProject status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var response dbtImportResponse
	if err := sonic.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.ImportedModels) != 1 {
		t.Fatalf("imported_models = %d, want 1; response = %+v", len(response.ImportedModels), response)
	}
	model := response.ImportedModels[0].Model
	if model.Status != internalsemantic.ModelStatusDraft {
		t.Errorf("imported model status = %q, want %q", model.Status, internalsemantic.ModelStatusDraft)
	}
	if model.DatasourceID != datasourceID {
		t.Errorf("imported model datasource_id = %q, want %q", model.DatasourceID, datasourceID)
	}
}

func TestPersistDbtModelDeletesDraftWhenEnumPersistenceFails(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{{Pattern: "INSERT INTO semantic_models"}}
	state.execs = []execMock{{Pattern: "INSERT INTO enum_mappings", Err: errors.New("enum mapping write failed")}}

	model := &internalsemantic.SemanticModel{
		ID:           uuid.NewString(),
		DatasourceID: uuid.NewString(),
		Name:         "orders",
		BaseTable:    "orders",
		IsActive:     true,
		Dimensions: []internalsemantic.Dimension{{
			ID:        uuid.NewString(),
			Name:      "status",
			ColumnRef: "orders.status",
			Type:      "string",
			IsActive:  true,
			EnumValues: []internalsemantic.EnumMapping{{
				RawValue: "paid",
				Label:    "paid",
			}},
		}},
	}
	h := NewSemanticHandler(&app.CatalogDeps{SemanticRepo: internalsemantic.NewRepository(db)})

	_, _, err := h.persistDbtModel(context.Background(), model, semanticCatalogAdapter{})
	if err == nil {
		t.Fatal("persistDbtModel error = nil, want enum persistence error")
	}

	for _, call := range state.calls {
		if strings.Contains(call.Op, "DELETE FROM semantic_models") {
			return
		}
	}
	t.Errorf("persistDbtModel error = %v; did not delete draft after enum persistence failed; calls = %+v", err, state.calls)
}
