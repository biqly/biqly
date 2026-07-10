package http

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/audit"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type recordedRequest struct {
	method  string
	path    string
	query   string
	body    string
	headers http.Header
}

func mcpTestBackend(t *testing.T, status int, respBody string) (http.Handler, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		rec.body = string(body)
		rec.headers = r.Header.Clone()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	})
	return handler, rec
}

func mcpTestSession(t *testing.T, backend http.Handler) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(backend, "Bearer test-token", "test-key")
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestMCPServer_ListsExpectedTools(t *testing.T) {
	backend, _ := mcpTestBackend(t, http.StatusOK, "[]")
	session := mcpTestSession(t, backend)

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	got := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{
		"list_datasources", "list_models", "list_prompt_templates",
		"run_question", "run_logical_query", "list_skills", "run_skill",
		"dry_plan", "dry_run", "metric_query",
		"list_knowledge_files", "read_knowledge_file",
	} {
		if !got[want] {
			t.Errorf("missing tool %q in %v", want, res.Tools)
		}
	}
	if len(res.Tools) != 12 {
		t.Errorf("expected exactly 12 tools (default-deny allow-list), got %d", len(res.Tools))
	}
}

func TestMCPServer_RunQuestionDispatchesGovernedRequest(t *testing.T) {
	backend, rec := mcpTestBackend(t, http.StatusOK, `{"result":{"rows":[]}}`)
	session := mcpTestSession(t, backend)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "run_question",
		Arguments: map[string]any{
			"datasource_id": "ds-1",
			"question":      "how many orders last week",
		},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	if rec.method != http.MethodPost || rec.path != "/api/ai/query/run" {
		t.Errorf("unexpected dispatch: %s %s", rec.method, rec.path)
	}
	if !strings.Contains(rec.body, `"question":"how many orders last week"`) {
		t.Errorf("question missing from dispatched body: %s", rec.body)
	}
	if got := rec.headers.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization not forwarded, got %q", got)
	}
	if got := rec.headers.Get("X-API-Key"); got != "test-key" {
		t.Errorf("X-API-Key not forwarded, got %q", got)
	}
	if got := rec.headers.Get(bimw.ChannelHeader); got != audit.ChannelMCP {
		t.Errorf("channel header = %q, want %q", got, audit.ChannelMCP)
	}
}

func TestMCPServer_ListModelsPassesDatasourceFilter(t *testing.T) {
	backend, rec := mcpTestBackend(t, http.StatusOK, "[]")
	session := mcpTestSession(t, backend)

	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_models",
		Arguments: map[string]any{"datasource_id": "ds 1"},
	}); err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if rec.path != "/api/semantic/models" ||
		(rec.query != "datasource_id=ds+1&include=full" && rec.query != "include=full&datasource_id=ds+1") {
		t.Errorf("unexpected dispatch: %s?%s", rec.path, rec.query)
	}
}

func TestMCPServer_BackendErrorSurfacesAsToolError(t *testing.T) {
	backend, _ := mcpTestBackend(t, http.StatusForbidden, `{"error":"permission denied"}`)
	session := mcpTestSession(t, backend)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_datasources",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError for 403 backend response")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "HTTP 403") {
		t.Errorf("expected HTTP 403 in error content, got %+v", res.Content[0])
	}
}
