package http

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/biqly/biqly/internal/audit"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/bytedance/sonic"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpHandler exposes biqly's governed query capability as an MCP server
// (streamable HTTP, stateless). Every tool call is dispatched through the
// monolith's own /api router with the caller's credentials forwarded, so the
// exact same middleware chain applies as for UI/API callers: authentication,
// permission checks, per-datasource access, PII masking, row-level security,
// spend caps, and audit (channel=mcp). There is no second query path.
func mcpHandler(dispatch http.Handler) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return newMCPServer(dispatch, r.Header.Get("Authorization"), r.Header.Get("X-API-Key"))
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}

func newMCPServer(dispatch http.Handler, authorization, apiKey string) *mcp.Server {
	d := &mcpDispatcher{api: dispatch, authorization: authorization, apiKey: apiKey}
	s := mcp.NewServer(&mcp.Implementation{Name: "biqly", Title: "Biqly governed BI", Version: "1.0.0"}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_datasources",
		Description: "List the datasources the caller can access. Returns id, name and " +
			"type for each; use the id with the other tools.",
	}, d.listDatasources)

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_models",
		Description: "List published semantic models (dimensions, metrics, joins) the " +
			"caller can query, optionally filtered by datasource.",
	}, d.listModels)

	mcp.AddTool(s, &mcp.Tool{
		Name: "run_question",
		Description: "Answer a natural-language question against a datasource. The " +
			"backend generates a governed LogicalQuery, compiles it to SQL with " +
			"row-level security and PII masking applied, executes it read-only and " +
			"returns the result rows plus the generated query.",
	}, d.runQuestion)

	mcp.AddTool(s, &mcp.Tool{
		Name: "run_logical_query",
		Description: "Compile and execute a Biqly LogicalQuery JSON document " +
			"(datasource_id, model_id, select, filters, group_by, order_by, limit). " +
			"The backend enforces the semantic model, permissions and read-only " +
			"execution; raw SQL is never accepted.",
	}, d.runLogicalQuery)

	return s
}

// mcpDispatcher performs in-process loopback requests against the monolith
// router on behalf of MCP tool calls, forwarding the caller's credentials.
type mcpDispatcher struct {
	api           http.Handler
	authorization string
	apiKey        string
}

type mcpListModelsInput struct {
	DatasourceID string `json:"datasource_id,omitempty" jsonschema:"optional datasource id to filter models by"`
}

type mcpRunQuestionInput struct {
	DatasourceID string `json:"datasource_id" jsonschema:"datasource id to answer the question against"`
	Question     string `json:"question" jsonschema:"the natural-language question to answer"`
	ModelID      string `json:"model_id,omitempty" jsonschema:"optional semantic model id; omitted = automatic routing"`
}

type mcpRunLogicalQueryInput struct {
	LogicalQuery map[string]any `json:"logical_query" jsonschema:"the LogicalQuery document to compile and execute"`
}

func (d *mcpDispatcher) listDatasources(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	return d.call(ctx, http.MethodGet, "/api/datasources", nil)
}

func (d *mcpDispatcher) listModels(ctx context.Context, _ *mcp.CallToolRequest, in mcpListModelsInput) (*mcp.CallToolResult, any, error) {
	path := "/api/semantic/models"
	if ds := strings.TrimSpace(in.DatasourceID); ds != "" {
		path += "?datasource_id=" + url.QueryEscape(ds)
	}
	return d.call(ctx, http.MethodGet, path, nil)
}

func (d *mcpDispatcher) runQuestion(ctx context.Context, _ *mcp.CallToolRequest, in mcpRunQuestionInput) (*mcp.CallToolResult, any, error) {
	body := map[string]any{
		"datasource_id": in.DatasourceID,
		"question":      in.Question,
	}
	if in.ModelID != "" {
		body["model_id"] = in.ModelID
	}
	return d.call(ctx, http.MethodPost, "/api/ai/query/run", body)
}

func (d *mcpDispatcher) runLogicalQuery(ctx context.Context, _ *mcp.CallToolRequest, in mcpRunLogicalQueryInput) (*mcp.CallToolResult, any, error) {
	return d.call(ctx, http.MethodPost, "/api/query/run", map[string]any{"logical_query": in.LogicalQuery})
}

func (d *mcpDispatcher) call(ctx context.Context, method, path string, body any) (*mcp.CallToolResult, any, error) {
	var reader *bytes.Reader
	if body != nil {
		encoded, err := sonic.ConfigStd.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, reader)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(bimw.ChannelHeader, audit.ChannelMCP)
	if d.authorization != "" {
		req.Header.Set("Authorization", d.authorization)
	}
	if d.apiKey != "" {
		req.Header.Set("X-API-Key", d.apiKey)
	}

	rec := &mcpResponseRecorder{header: make(http.Header), status: http.StatusOK}
	d.api.ServeHTTP(rec, req)

	text := rec.body.String()
	if rec.status < 200 || rec.status >= 300 {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("HTTP %d: %s", rec.status, text)}},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// mcpResponseRecorder is a minimal in-process http.ResponseWriter used to
// capture loopback dispatch responses without net/http/httptest.
type mcpResponseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *mcpResponseRecorder) Header() http.Header { return r.header }

func (r *mcpResponseRecorder) WriteHeader(status int) { r.status = status }

func (r *mcpResponseRecorder) Write(p []byte) (int, error) { return r.body.Write(p) }
