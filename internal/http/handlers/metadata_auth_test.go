package handlers

import (
	"bytes"
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

type metadataAccessCall struct {
	UserID       string
	DatasourceID string
	Level        string
}

type fakeMetadataAccessChecker struct {
	allowed bool
	calls   []metadataAccessCall
}

func (f *fakeMetadataAccessChecker) CheckDatasourceAccess(_ context.Context, userID, datasourceID, level string) (bool, error) {
	f.calls = append(f.calls, metadataAccessCall{UserID: userID, DatasourceID: datasourceID, Level: level})
	return f.allowed, nil
}

func TestMetadataTableMutationRequiresDatasourceWriteAccess(t *testing.T) {
	db, state := setupMockDB(t)
	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM tables WHERE id",
			Cols:    metadataTableCols(),
			Rows: [][]driver.Value{
				{"table-1", "ds-1", "schema-1", "public", "orders", "BASE TABLE", nil, nil, nil, nil, now, now},
			},
		},
	}

	checker := &fakeMetadataAccessChecker{allowed: false}
	handler := NewMetadataHandler(&app.CatalogDeps{MetaRepo: metadata.NewRepository(db)})
	handler.SetDatasourceAccessChecker(checker)

	router := chi.NewRouter()
	router.Patch("/metadata/tables/{id}", handler.UpdateTableDescription)

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-1")
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/metadata/tables/table-1", bytes.NewBufferString(`{"description":"owned"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("UpdateTableDescription denied response = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if len(checker.calls) != 1 {
		t.Fatalf("CheckDatasourceAccess calls = %d, want 1", len(checker.calls))
	}
	if got := checker.calls[0]; got != (metadataAccessCall{UserID: "user-1", DatasourceID: "ds-1", Level: "write"}) {
		t.Fatalf("CheckDatasourceAccess call = %+v, want user-1/ds-1/write", got)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, call := range state.calls {
		if call.Op != "" && hasMetadataAuthExec(call.Op) {
			t.Fatalf("UpdateTableDescription executed mutation despite denied access: %s", call.Op)
		}
	}
}

func TestMetadataColumnTranslationRequiresDatasourceWriteAccess(t *testing.T) {
	db, state := setupMockDB(t)
	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM columns WHERE id",
			Cols:    metadataColumnCols(),
			Rows: [][]driver.Value{
				{"col-1", "ds-2", "table-1", "public", "orders", "email", "text", true, nil, nil, nil, nil, nil, nil, false, false, nil, nil, nil, now, nil, nil, nil, nil, nil},
			},
		},
	}

	checker := &fakeMetadataAccessChecker{allowed: false}
	handler := NewMetadataHandler(&app.CatalogDeps{MetaRepo: metadata.NewRepository(db)})
	handler.SetDatasourceAccessChecker(checker)

	router := chi.NewRouter()
	router.Put("/metadata/columns/{id}/translations", handler.PutColumnTranslations)

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-2")
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/metadata/columns/col-1/translations", bytes.NewBufferString(`{"en":{"description":"Email"}}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("PutColumnTranslations denied response = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if len(checker.calls) != 1 {
		t.Fatalf("CheckDatasourceAccess calls = %d, want 1", len(checker.calls))
	}
	if got := checker.calls[0]; got != (metadataAccessCall{UserID: "user-2", DatasourceID: "ds-2", Level: "write"}) {
		t.Fatalf("CheckDatasourceAccess call = %+v, want user-2/ds-2/write", got)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, call := range state.calls {
		if call.Op != "" && hasMetadataAuthExec(call.Op) {
			t.Fatalf("PutColumnTranslations executed mutation despite denied access: %s", call.Op)
		}
	}
}

func TestMetadataTableTranslationRequiresDatasourceReadAccess(t *testing.T) {
	db, state := setupMockDB(t)
	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM tables WHERE id",
			Cols:    metadataTableCols(),
			Rows: [][]driver.Value{
				{"table-2", "ds-3", "schema-1", "public", "customers", "BASE TABLE", nil, nil, nil, nil, now, now},
			},
		},
	}

	checker := &fakeMetadataAccessChecker{allowed: false}
	handler := NewMetadataHandler(&app.CatalogDeps{MetaRepo: metadata.NewRepository(db)})
	handler.SetDatasourceAccessChecker(checker)

	router := chi.NewRouter()
	router.Get("/metadata/tables/{id}/translations", handler.GetTableTranslations)

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-3")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/metadata/tables/table-2/translations", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GetTableTranslations denied response = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if len(checker.calls) != 1 {
		t.Fatalf("CheckDatasourceAccess calls = %d, want 1", len(checker.calls))
	}
	if got := checker.calls[0]; got != (metadataAccessCall{UserID: "user-3", DatasourceID: "ds-3", Level: "read"}) {
		t.Fatalf("CheckDatasourceAccess call = %+v, want user-3/ds-3/read", got)
	}
}

func metadataTableCols() []string {
	return []string{"id", "datasource_id", "schema_id", "schema_name", "table_name", "table_type", "row_estimate", "description", "label", "display_expression", "created_at", "updated_at"}
}

func metadataColumnCols() []string {
	return []string{"id", "datasource_id", "table_id", "schema_name", "table_name", "column_name", "data_type", "nullable", "ordinal_position", "character_maximum_length", "numeric_precision", "numeric_scale", "column_default", "description", "is_primary_key", "is_foreign_key", "referenced_schema", "referenced_table", "referenced_column", "created_at", "pii_type", "pii_confidence", "pii_detected_at", "pii_reviewed_by", "pii_masking_strategy"}
}

func hasMetadataAuthExec(op string) bool {
	return containsAny(op, "UPDATE tables", "UPDATE columns", "INSERT INTO entity_translations")
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
