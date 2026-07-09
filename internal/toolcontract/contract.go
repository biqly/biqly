// Package toolcontract defines the governed BI tool contract shared by the MCP
// server and the web agent. Both consumers call the exact same tool definitions
// and dispatch through the same [Dispatcher] (in-process loopback HTTP) with the
// caller's own credentials, so every tool call traverses the standard /api/*
// middleware chain: authentication, RBAC, datasource access, RLS, PII masking,
// spend caps, and audit.
//
// The contract is the literal implementation of the "same tool contract"
// criterion: one definition, two consumers.
package toolcontract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/audit"
)

// Channel constants for the X-Biqly-Channel header are re-exported from
// internal/audit for consumer convenience (Dispatcher callers pass a channel
// string). Use audit.ChannelMCP, audit.ChannelAgent, etc.
var (
	ChannelMCP   = audit.ChannelMCP
	ChannelAgent = audit.ChannelAgent
)

// Credential carries the caller's authentication forwarded to loopback calls.
// Either Authorization (JWT Bearer) or X-API-Key (PAT) is set, matching what the
// inbound request carried.
type Credential struct {
	Authorization string // raw "Bearer ..." or ""
	APIKey        string // raw PAT or ""
}

// ToolName identifies one of the six governed tools.
type ToolName string

const (
	ToolListDatasources ToolName = "list_datasources"
	ToolListModels      ToolName = "list_models"
	ToolRunQuestion     ToolName = "run_question"
	ToolRunLogicalQuery ToolName = "run_logical_query"
	ToolListSkills      ToolName = "list_skills"
	ToolRunSkill        ToolName = "run_skill"
)

// ToolSpec is the static definition of one governed tool: its name,
// description, and the HTTP dispatch target.
type ToolSpec struct {
	Name        ToolName
	Description string
	Method      string
	Path        string
}

// AllTools lists the six governed tools in stable order.
var AllTools = []ToolSpec{
	{
		Name: ToolListDatasources,
		Description: "List the datasources the caller can access. Returns id, name and " +
			"type for each; use the id with the other tools.",
		Method: http.MethodGet,
		Path:   "/api/datasources",
	},
	{
		Name: ToolListModels,
		Description: "List published semantic models (dimensions, metrics, joins) the " +
			"caller can query, optionally filtered by datasource.",
		Method: http.MethodGet,
		Path:   "/api/semantic/models",
	},
	{
		Name: ToolRunQuestion,
		Description: "Answer a natural-language question against a datasource. The " +
			"backend generates a governed LogicalQuery, compiles it to SQL with " +
			"row-level security and PII masking applied, executes it read-only and " +
			"returns the result rows plus the generated query.",
		Method: http.MethodPost,
		Path:   "/api/ai/query/run",
	},
	{
		Name: ToolRunLogicalQuery,
		Description: "Compile and execute a Biqly LogicalQuery JSON document " +
			"(datasource_id, model_id, select, filters, group_by, order_by, limit). " +
			"The backend enforces the semantic model, permissions and read-only " +
			"execution; raw SQL is never accepted.",
		Method: http.MethodPost,
		Path:   "/api/query/run",
	},
	{
		Name: ToolListSkills,
		Description: "List saved skills: named, parameterized LogicalQuery templates " +
			"validated by users. Prefer running a matching skill over generating a " +
			"fresh query. Returns name, description, parameters and tags per skill.",
		Method: http.MethodGet,
		Path:   "/api/ai/skills",
	},
	{
		Name: ToolRunSkill,
		Description: "Execute a saved skill by id with parameter values. The stored " +
			"LogicalQuery template is compiled and executed through the governed " +
			"query path with all permissions applied.",
		Method: http.MethodPost,
		Path:   "/api/ai/skills",
	},
}

// Input argument structs with jsonschema tags (consumed by MCP SDK for schema
// generation and by the web agent for argument construction).

// ListModelsInput filters models by datasource.
type ListModelsInput struct {
	DatasourceID string `json:"datasource_id,omitempty" jsonschema:"optional datasource id to filter models by"`
}

// RunQuestionInput answers a natural-language question.
type RunQuestionInput struct {
	DatasourceID string `json:"datasource_id" jsonschema:"datasource id to answer the question against"`
	Question     string `json:"question" jsonschema:"the natural-language question to answer"`
	ModelID      string `json:"model_id,omitempty" jsonschema:"optional semantic model id; omitted = automatic routing"`
}

// RunLogicalQueryInput compiles and executes a LogicalQuery document.
type RunLogicalQueryInput struct {
	DatasourceID string         `json:"datasource_id,omitempty" jsonschema:"datasource id; auto-injected into the logical query when missing"`
	LogicalQuery map[string]any `json:"logical_query" jsonschema:"the LogicalQuery document to compile and execute"`
}

// ListSkillsInput filters skills by datasource.
type ListSkillsInput struct {
	DatasourceID string `json:"datasource_id,omitempty" jsonschema:"optional datasource id to filter skills by"`
}

// RunSkillInput executes a saved skill.
type RunSkillInput struct {
	SkillID    string         `json:"skill_id" jsonschema:"id of the skill to run"`
	Parameters map[string]any `json:"parameters,omitempty" jsonschema:"parameter values keyed by parameter name"`
}

// DispatchResult is the outcome of a single tool dispatch.
type DispatchResult struct {
	// StatusCode is the HTTP status from the loopback call.
	StatusCode int
	// Body is the raw JSON body returned by the /api/* endpoint.
	Body json.RawMessage
}

// IsError reports whether the loopback call returned a non-2xx status.
func (r DispatchResult) IsError() bool {
	return r.StatusCode < 200 || r.StatusCode >= 300
}

// ErrorText formats a non-2xx result as a human-readable error string.
func (r DispatchResult) ErrorText() string {
	return fmt.Sprintf("HTTP %d: %s", r.StatusCode, string(r.Body))
}

// Dispatcher dispatches a governed tool call via in-process loopback HTTP,
// forwarding the caller's credentials and setting the channel header.
type Dispatcher interface {
	// Dispatch performs a loopback HTTP call for the given method+path, with an
	// optional JSON body (nil for GET). It forwards [Credential] and sets
	// X-Biqly-Channel to [channel]. Returns the raw response body and status code.
	Dispatch(ctx context.Context, method, path string, body any, cred Credential, channel string) (DispatchResult, error)
}

// HTTPDispatcher is the production [Dispatcher] implementation: it drives the
// monolith's own http.Handler (the /api router) via an in-process
// ResponseRecorder, exactly as the MCP dispatcher did before extraction.
type HTTPDispatcher struct {
	API http.Handler
}

// responseRecorder is a minimal in-process http.ResponseWriter used to capture
// loopback dispatch responses without net/http/httptest.
type responseRecorder struct {
	header http.Header
	status int
	body   []byte
}

func (r *responseRecorder) Header() http.Header    { return r.header }
func (r *responseRecorder) WriteHeader(status int) { r.status = status }
func (r *responseRecorder) Write(p []byte) (int, error) {
	r.body = append(r.body, p...)
	return len(p), nil
}

// Dispatch implements [Dispatcher].
func (d *HTTPDispatcher) Dispatch(ctx context.Context, method, path string, body any, cred Credential, channel string) (DispatchResult, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = sonic.Marshal(body)
		if err != nil {
			return DispatchResult{}, fmt.Errorf("encode request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(bodyBytes))
	if err != nil {
		return DispatchResult{}, fmt.Errorf("build loopback request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Biqly-Channel", channel)
	if cred.Authorization != "" {
		req.Header.Set("Authorization", cred.Authorization)
	}
	if cred.APIKey != "" {
		req.Header.Set("X-API-Key", cred.APIKey)
	}

	rec := &responseRecorder{header: make(http.Header), status: http.StatusOK}
	d.API.ServeHTTP(rec, req)

	return DispatchResult{
		StatusCode: rec.status,
		Body:       json.RawMessage(rec.body),
	}, nil
}

// --- Tool-specific dispatch helpers (shared by MCP and web agent) ---

// DispatchListDatasources calls GET /api/datasources.
func DispatchListDatasources(ctx context.Context, disp Dispatcher, cred Credential, channel string) (DispatchResult, error) {
	return disp.Dispatch(ctx, http.MethodGet, "/api/datasources", nil, cred, channel)
}

// DispatchListModels calls GET /api/semantic/models, optionally filtered by datasource.
func DispatchListModels(ctx context.Context, disp Dispatcher, in ListModelsInput, cred Credential, channel string) (DispatchResult, error) {
	path := "/api/semantic/models"
	if ds := strings.TrimSpace(in.DatasourceID); ds != "" {
		path += "?datasource_id=" + url.QueryEscape(ds)
	}
	return disp.Dispatch(ctx, http.MethodGet, path, nil, cred, channel)
}

// DispatchRunQuestion calls POST /api/ai/query/run.
func DispatchRunQuestion(ctx context.Context, disp Dispatcher, in RunQuestionInput, cred Credential, channel string) (DispatchResult, error) {
	body := map[string]any{
		"datasource_id": in.DatasourceID,
		"question":      in.Question,
	}
	if in.ModelID != "" {
		body["model_id"] = in.ModelID
	}
	return disp.Dispatch(ctx, http.MethodPost, "/api/ai/query/run", body, cred, channel)
}

// DispatchRunLogicalQuery calls POST /api/query/run.
func DispatchRunLogicalQuery(ctx context.Context, disp Dispatcher, in RunLogicalQueryInput, cred Credential, channel string) (DispatchResult, error) {
	lq := in.LogicalQuery
	if lq == nil {
		lq = make(map[string]any)
	}
	// Inject datasource_id from the top-level input when the logical_query
	// document doesn't have one set (the planner may omit it). The query
	// service requires datasource_id on the LogicalQuery struct itself,
	// not as a separate top-level field.
	if _, ok := lq["datasource_id"]; !ok && in.DatasourceID != "" {
		// Clone to avoid mutating the caller's map.
		cloned := make(map[string]any, len(lq)+1)
		for k, v := range lq {
			cloned[k] = v
		}
		cloned["datasource_id"] = in.DatasourceID
		lq = cloned
	}
	return disp.Dispatch(ctx, http.MethodPost, "/api/query/run", map[string]any{"logical_query": lq}, cred, channel)
}

// DispatchListSkills calls GET /api/ai/skills, optionally filtered by datasource.
func DispatchListSkills(ctx context.Context, disp Dispatcher, in ListSkillsInput, cred Credential, channel string) (DispatchResult, error) {
	path := "/api/ai/skills"
	if ds := strings.TrimSpace(in.DatasourceID); ds != "" {
		path += "?datasource_id=" + url.QueryEscape(ds)
	}
	return disp.Dispatch(ctx, http.MethodGet, path, nil, cred, channel)
}

// DispatchRunSkill calls POST /api/ai/skills/{id}/run.
func DispatchRunSkill(ctx context.Context, disp Dispatcher, in RunSkillInput, cred Credential, channel string) (DispatchResult, error) {
	body := map[string]any{}
	if len(in.Parameters) > 0 {
		body["parameters"] = in.Parameters
	}
	path := "/api/ai/skills/" + url.PathEscape(in.SkillID) + "/run"
	return disp.Dispatch(ctx, http.MethodPost, path, body, cred, channel)
}
