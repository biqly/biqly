package app

import (
	"context"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
)

func TestQueryCatalogAdapter_ImplementsQueryServicePorts(t *testing.T) {
	t.Parallel()
	var historyCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/internal/models/orders":
			_ = sonic.ConfigStd.NewEncoder(w).Encode(semantic.SemanticModel{ID: "orders", DatasourceID: "ds_1"})
		case "/internal/datasources/ds_1":
			_ = sonic.ConfigStd.NewEncoder(w).Encode(metadata.Datasource{ID: "ds_1", Type: "postgres"})
		case "/internal/history/query":
			historyCalled = true
			_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string]string{"id": "hist_1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	adapter := newQueryCatalogAdapter(catalogclient.New(server.URL))
	model, err := adapter.GetPublishedFullModel(context.Background(), "orders")
	if err != nil {
		t.Fatalf("GetPublishedFullModel: %v", err)
	}
	if model.ID != "orders" {
		t.Fatalf("model id: got %q", model.ID)
	}

	ds, err := adapter.GetDatasource(context.Background(), "ds_1")
	if err != nil {
		t.Fatalf("GetDatasource: %v", err)
	}
	if ds.ID != "ds_1" || ds.Type != "postgres" {
		t.Fatalf("datasource: %+v", ds)
	}

	if err := adapter.CreateQueryHistory(context.Background(), &query.HistoryEntry{ID: "hist_1"}); err != nil {
		t.Fatalf("CreateQueryHistory: %v", err)
	}
	if !historyCalled {
		t.Fatal("history endpoint was not called")
	}
}
