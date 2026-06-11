package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestCatalogMetadataSearchRequiresDatasourceReadAccess(t *testing.T) {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	registerCatalogMetadataRoutes(router, &app.CatalogDeps{}, bimw.NewAuthClient("http://127.0.0.1:1", "token"))

	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/metadata/tables/search?datasource_id=ds-1&q=orders", stdhttp.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("SearchTables anonymous response = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
}

func TestCatalogMetadataListRequiresDatasourceReadAccess(t *testing.T) {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	registerCatalogMetadataRoutes(router, &app.CatalogDeps{}, bimw.NewAuthClient("http://127.0.0.1:1", "token"))

	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/datasources/ds-1/tables", stdhttp.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("ListTables anonymous response = %d, want %d", rec.Code, stdhttp.StatusUnauthorized)
	}
}
