package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/internalapi"
)

func TestAIHandlerUsesCatalogClientForCatalogReadsAndHistory(t *testing.T) {
	t.Parallel()

	calls := map[string]int{}
	upstream := httptest.NewServer(catalogClientTestHandler(t, calls))
	defer upstream.Close()

	handler := &AIHandler{deps: (&app.Dependencies{CatalogClient: catalogclient.New(upstream.URL)}).AIDeps()}
	ctx := context.Background()

	models, err := handler.listSemanticModels(ctx, integrationDSID)
	if err != nil || len(models) != 1 {
		t.Fatalf("list models: len=%d err=%v", len(models), err)
	}
	model, err := handler.loadModel(ctx, integrationDSID, integrationModel)
	if err != nil || model.ID != integrationModel {
		t.Fatalf("load model: model=%#v err=%v", model, err)
	}
	if dialectName := handler.datasourceDialectName(ctx, integrationDSID); dialectName != "postgres" {
		t.Fatalf("dialect: got %q, want postgres", dialectName)
	}
	glossary, err := handler.listBusinessGlossary(ctx, integrationDSID, integrationModel)
	if err != nil || len(glossary) != 1 {
		t.Fatalf("glossary: len=%d err=%v", len(glossary), err)
	}

	lq := integrationLogicalQuery()
	handler.recordAIHistory(ctx, aiQueryRequest{DatasourceID: integrationDSID, Question: "orders by country"}, model, nil, &ai.Response{Result: &ai.AIResult{LogicalQuery: &lq}})

	for _, key := range []string{
		"GET /internal/models",
		"GET /internal/models/" + integrationModel,
		"GET /internal/datasources/" + integrationDSID,
		"GET /internal/glossary",
		"POST /internal/history/ai",
	} {
		if calls[key] != 1 {
			t.Fatalf("catalog call %s: got %d, want 1", key, calls[key])
		}
	}
}

func catalogClientTestHandler(t *testing.T, calls map[string]int) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.Method+" "+r.URL.Path]++
		if writeCatalogClientTestRead(t, w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/internal/history/ai" {
			writeCatalogClientTestHistory(t, w, r)
			return
		}
		t.Fatalf("unexpected catalog request: %s %s", r.Method, r.URL.Path)
	})
}

func writeCatalogClientTestRead(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	t.Helper()
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/internal/models":
		writeUpstreamJSON(t, w, []semantic.SemanticModel{{ID: integrationModel, Name: integrationModel, DatasourceID: integrationDSID, IsActive: true, Status: semantic.ModelStatusPublished}})
	case "/internal/models/" + integrationModel:
		writeUpstreamJSON(t, w, integrationSemanticModel())
	case "/internal/datasources/" + integrationDSID:
		writeUpstreamJSON(t, w, metadata.Datasource{ID: integrationDSID, Type: "postgres"})
	case "/internal/glossary":
		writeUpstreamJSON(t, w, []metadata.BusinessGlossaryRow{{Term: "revenue", MapsToType: "metric", MapsToName: "order_count"}})
	default:
		return false
	}
	return true
}

func writeCatalogClientTestHistory(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var req internalapi.AIHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Fatalf("decode ai history request: %v", err)
	}
	if req.Entry.Question == "" {
		t.Fatal("ai history question was empty")
	}
	writeUpstreamJSON(t, w, internalapi.AIHistoryResponse{ID: "ai_hist_1"})
}
