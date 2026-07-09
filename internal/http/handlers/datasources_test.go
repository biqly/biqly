package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

func TestResolveCreateDatasourceMode_structuredWithoutHost(t *testing.T) {
	req := createDatasourceRequest{
		Name: "local file", Type: "sqlite",
		Connection: &connectionRequest{DatabaseName: "/data/app.db"},
	}
	if got := resolveCreateDatasourceMode(&req); got != metadata.DSNModeStructured {
		t.Fatalf("mode = %q, want structured", got)
	}
}

func TestRejectFunctionBlocklistConfig(t *testing.T) {
	recorder := httptest.NewRecorder()
	if rejectFunctionBlocklistConfig(recorder, `{"function_blocklist":["custom_reader"]}`) {
		t.Fatal("rejectFunctionBlocklistConfig() = true, want false")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("response status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestPreserveFunctionBlocklist(t *testing.T) {
	existing := &metadata.Datasource{Config: `{"function_blocklist":["custom_reader"],"timezone":"UTC"}`}
	updated := &metadata.Datasource{Config: `{"max_rows":100}`}
	recorder := httptest.NewRecorder()

	if !preserveFunctionBlocklist(context.Background(), existing, updated, recorder) {
		t.Fatalf("preserveFunctionBlocklist() = false, response = %s", recorder.Body.String())
	}
	custom, err := pkgmetadata.ParseDatasourceFunctionBlocklist(updated.Config)
	if err != nil {
		t.Fatalf("ParseDatasourceFunctionBlocklist() error = %v", err)
	}
	if len(custom) != 1 || custom[0] != "custom_reader" {
		t.Fatalf("custom blocklist = %v, want [custom_reader]", custom)
	}
}
