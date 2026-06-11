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
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

func TestValidateDisplayExpression(t *testing.T) {
	cases := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{name: "empty clears", expr: "", wantErr: false},
		{name: "column token", expr: "author_name", wantErr: false},
		{name: "columns and literal", expr: `author_name + " " + screen_name`, wantErr: false},
		{name: "single quoted literal", expr: `first_name + ' ' + last_name`, wantErr: false},
		{name: "unclosed literal", expr: `author_name + " " +`, wantErr: true},
		{name: "function call", expr: `concat(author_name, screen_name)`, wantErr: true},
		{name: "bad token", expr: `author-name`, wantErr: true},
		{name: "too long", expr: strings.Repeat("a", maxDisplayExpressionRunes+1), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDisplayExpression(tc.expr)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("validateDisplayExpression(%q) error = %v, want error presence = %t", tc.expr, err, tc.wantErr)
			}
		})
	}
}

func TestUpdateTableDescriptionRejectsInvalidDisplayExpression(t *testing.T) {
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

	handler := NewMetadataHandler(&app.CatalogDeps{MetaRepo: metadata.NewRepository(db)})
	router := chi.NewRouter()
	router.Patch("/metadata/tables/{id}", handler.UpdateTableDescription)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/metadata/tables/table-1", bytes.NewBufferString(`{"display_expression":"concat(author_name, screen_name)"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("UpdateTableDescription invalid display_expression status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, call := range state.calls {
		if strings.Contains(call.Op, "UPDATE tables SET display_expression") {
			t.Fatalf("UpdateTableDescription executed display expression update despite invalid expression: %s", call.Op)
		}
	}
}
