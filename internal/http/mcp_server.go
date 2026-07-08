package http

import (
	"context"
	"net/http"

	"github.com/biqly/biqly/internal/toolcontract"
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
	d := &mcpToolDispatcher{
		disp: &toolcontract.HTTPDispatcher{API: dispatch},
		cred: toolcontract.Credential{Authorization: authorization, APIKey: apiKey},
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "biqly", Title: "Biqly governed BI", Version: "1.0.0"}, nil)

	// Register the six governed tools from the shared toolcontract package.
	// Tool names/descriptions come from toolcontract.AllTools; the MCP-specific
	// handler functions below call the shared dispatch helpers.
	for _, spec := range toolcontract.AllTools {
		switch spec.Name {
		case toolcontract.ToolListDatasources:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description}, d.listDatasources)
		case toolcontract.ToolListModels:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description}, d.listModels)
		case toolcontract.ToolRunQuestion:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description}, d.runQuestion)
		case toolcontract.ToolRunLogicalQuery:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description}, d.runLogicalQuery)
		case toolcontract.ToolListSkills:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description}, d.listSkills)
		case toolcontract.ToolRunSkill:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description}, d.runSkill)
		}
	}

	return s
}

// mcpToolDispatcher wraps the shared toolcontract.Dispatcher, fixing the
// credential (from the inbound request) and channel (mcp) so each MCP tool
// handler is a thin adapter over the shared dispatch path.
type mcpToolDispatcher struct {
	disp toolcontract.Dispatcher
	cred toolcontract.Credential
}

func (d *mcpToolDispatcher) listDatasources(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchListDatasources(ctx, d.disp, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

func (d *mcpToolDispatcher) listModels(ctx context.Context, _ *mcp.CallToolRequest, in toolcontract.ListModelsInput) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchListModels(ctx, d.disp, in, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

func (d *mcpToolDispatcher) runQuestion(ctx context.Context, _ *mcp.CallToolRequest, in toolcontract.RunQuestionInput) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchRunQuestion(ctx, d.disp, in, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

func (d *mcpToolDispatcher) runLogicalQuery(ctx context.Context, _ *mcp.CallToolRequest, in toolcontract.RunLogicalQueryInput) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchRunLogicalQuery(ctx, d.disp, in, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

func (d *mcpToolDispatcher) listSkills(ctx context.Context, _ *mcp.CallToolRequest, in toolcontract.ListSkillsInput) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchListSkills(ctx, d.disp, in, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

func (d *mcpToolDispatcher) runSkill(ctx context.Context, _ *mcp.CallToolRequest, in toolcontract.RunSkillInput) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchRunSkill(ctx, d.disp, in, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

// toMCPResult converts a toolcontract.DispatchResult into the MCP SDK's
// CallToolResult shape: raw-JSON TextContent, IsError on non-2xx.
func toMCPResult(res toolcontract.DispatchResult) *mcp.CallToolResult {
	if res.IsError() {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: res.ErrorText()}},
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(res.Body)}},
	}
}
