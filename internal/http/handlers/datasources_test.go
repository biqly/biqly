package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/metadata"
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
