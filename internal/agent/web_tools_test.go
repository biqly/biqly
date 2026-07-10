package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/toolcontract"
)

// webTestBackend is a fake http.Handler for web tool tests; it records the
// request and returns the configured status/body.
func webTestBackend(t *testing.T, status int, respBody string) (http.Handler, *webRecordedReq) {
	t.Helper()
	rec := &webRecordedReq{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		rec.headers = r.Header.Clone()
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			rec.body = string(b)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	})
	return handler, rec
}

type webRecordedReq struct {
	method  string
	path    string
	query   string
	body    string
	headers http.Header
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := sonic.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestWebTools_AllReturnsTwelve(t *testing.T) {
	wt := NewWebTools(&toolcontract.HTTPDispatcher{API: http.NewServeMux()}, toolcontract.Credential{})
	tools := wt.All()
	if len(tools) != 12 {
		t.Fatalf("expected 12 tools, got %d", len(tools))
	}
	want := map[ToolName]bool{
		ToolWebListDatasources:     true,
		ToolWebListModels:          true,
		ToolWebListPromptTemplates: true,
		ToolWebRunQuestion:         true,
		ToolWebRunLogicalQuery:     true,
		ToolWebListSkills:          true,
		ToolWebRunSkill:            true,
		ToolWebDryPlan:             true,
		ToolWebDryRun:              true,
		ToolWebMetricQuery:         true,
		ToolWebListKnowledgeFiles:  true,
		ToolWebReadKnowledgeFile:   true,
	}
	for _, tool := range tools {
		if !want[tool.Name()] {
			t.Errorf("unexpected tool %q", tool.Name())
		}
	}
}

func TestWebListDatasourcesTool_DispatchesAndTruncates(t *testing.T) {
	backend, rec := webTestBackend(t, http.StatusOK, `{"datasources":[]}`)
	disp := &toolcontract.HTTPDispatcher{API: backend}
	wt := NewWebTools(disp, toolcontract.Credential{Authorization: "Bearer jwt"})

	obs, err := wt.All()[0].Execute(context.Background(), RunContext{}, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/api/datasources" {
		t.Errorf("dispatch = %s %s, want GET /api/datasources", rec.method, rec.path)
	}
	if got := rec.headers.Get("Authorization"); got != "Bearer jwt" {
		t.Errorf("Authorization = %q", got)
	}
	if got := rec.headers.Get("X-Biqly-Channel"); got != "agent" {
		t.Errorf("channel = %q, want agent", got)
	}
	if obs.Tool != ToolWebListDatasources {
		t.Errorf("obs tool = %q", obs.Tool)
	}
}

func TestWebRunQuestionTool_BuildsArgsFromJSON(t *testing.T) {
	backend, rec := webTestBackend(t, http.StatusOK, `{"result":{"rows":[]}}`)
	disp := &toolcontract.HTTPDispatcher{API: backend}
	wt := NewWebTools(disp, toolcontract.Credential{Authorization: "Bearer tok"})

	args := mustMarshal(t, toolcontract.RunQuestionInput{
		DatasourceID: "ds-1",
		Question:     "revenue last month",
	})

	// Find the run_question tool from All().
	tools := wt.All()
	var runQuestionTool Tool
	for _, tool := range tools {
		if tool.Name() == ToolWebRunQuestion {
			runQuestionTool = tool
			break
		}
	}
	if runQuestionTool == nil {
		t.Fatal("run_question tool not found in All()")
	}
	obs, err := runQuestionTool.Execute(context.Background(), RunContext{}, args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/ai/query/run" {
		t.Errorf("dispatch = %s %s", rec.method, rec.path)
	}
	// Verify the observation payload is valid JSON.
	var payload map[string]any
	if err := sonic.Unmarshal(obs.Payload, &payload); err != nil {
		t.Fatalf("unmarshal obs: %v: %s", err, string(obs.Payload))
	}
}

// (removed w() helpers — tests use wt.All() to find tools by name)

func TestWebRunLogicalQueryTool(t *testing.T) {
	backend, rec := webTestBackend(t, http.StatusOK, `{"rows":[]}`)
	disp := &toolcontract.HTTPDispatcher{API: backend}
	wt := NewWebTools(disp, toolcontract.Credential{})

	args := mustMarshal(t, toolcontract.RunLogicalQueryInput{
		LogicalQuery: map[string]any{"datasource_id": "ds", "select": []any{}},
	})

	tools := wt.All()
	var runLogicalQueryTool Tool
	for _, tool := range tools {
		if tool.Name() == ToolWebRunLogicalQuery {
			runLogicalQueryTool = tool
			break
		}
	}
	if runLogicalQueryTool == nil {
		t.Fatal("run_logical_query tool not found")
	}
	_, err := runLogicalQueryTool.Execute(context.Background(), RunContext{}, args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/query/run" {
		t.Errorf("dispatch = %s %s", rec.method, rec.path)
	}
}

func TestWebRunLogicalQuery_InjectsDatasourceAndModelIDFromRunContext(t *testing.T) {
	backend, rec := webTestBackend(t, http.StatusOK, `{"rows":[]}`)
	disp := &toolcontract.HTTPDispatcher{API: backend}
	wt := NewWebTools(disp, toolcontract.Credential{})

	// Planner omitted both ids — only the select clause.
	args := mustMarshal(t, toolcontract.RunLogicalQueryInput{
		LogicalQuery: map[string]any{"select": []any{}},
	})

	var runLogicalQueryTool Tool
	for _, tool := range wt.All() {
		if tool.Name() == ToolWebRunLogicalQuery {
			runLogicalQueryTool = tool
			break
		}
	}
	if runLogicalQueryTool == nil {
		t.Fatal("run_logical_query tool not found")
	}
	_, err := runLogicalQueryTool.Execute(context.Background(), RunContext{
		DatasourceID: "ds-from-run",
		ModelID:      "model-from-run",
	}, args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	lq, ok := body["logical_query"].(map[string]any)
	if !ok {
		t.Fatalf("logical_query type = %T body=%s", body["logical_query"], rec.body)
	}
	if lq["datasource_id"] != "ds-from-run" {
		t.Errorf("datasource_id = %v, want ds-from-run", lq["datasource_id"])
	}
	if lq["model_id"] != "model-from-run" {
		t.Errorf("model_id = %v, want model-from-run", lq["model_id"])
	}
}

func TestWebTool_Non2xxReturnsError(t *testing.T) {
	backend, _ := webTestBackend(t, http.StatusForbidden, `{"error":"denied"}`)
	disp := &toolcontract.HTTPDispatcher{API: backend}
	wt := NewWebTools(disp, toolcontract.Credential{})

	_, err := wt.All()[0].Execute(context.Background(), RunContext{}, nil)
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

func TestTruncateForPlanner_CapsRows(t *testing.T) {
	// Build a response with 200 rows.
	rows := make([]map[string]any, 200)
	for i := range 200 {
		rows[i] = map[string]any{"id": i}
	}
	raw := mustMarshal(t, map[string]any{"rows": rows, "sql": "SELECT 1"})

	truncated := truncateForPlanner(raw)

	var result map[string]json.RawMessage
	_ = sonic.Unmarshal(truncated, &result)

	var cappedRows []json.RawMessage
	_ = sonic.Unmarshal(result["rows"], &cappedRows)
	if len(cappedRows) != maxPlannerRows {
		t.Errorf("expected %d rows after truncation, got %d", maxPlannerRows, len(cappedRows))
	}

	if _, ok := result["rows_truncated"]; !ok {
		t.Error("expected rows_truncated field")
	}
}

func TestTruncateForPlanner_NoTruncationNeeded(t *testing.T) {
	raw := json.RawMessage(`{"rows":[{"a":1},{"a":2}]}`)
	result := truncateForPlanner(raw)
	if string(result) != string(raw) {
		t.Errorf("expected no truncation, got different output")
	}
}

func TestTruncateForPlanner_NoRowsField(t *testing.T) {
	raw := json.RawMessage(`{"models":[{"id":"m1"}]}`)
	result := truncateForPlanner(raw)
	if string(result) != string(raw) {
		t.Errorf("expected no truncation for non-rows response")
	}
}

func TestPolicyEngine_AllowsWebToolsWithoutIdentity(t *testing.T) {
	// Web tools don't carry identity in arguments — identity is in
	// RunContext.Credential. PolicyEngine should allow them without
	// requiring identityArgs in the proposal.
	pe := &PolicyEngine{}
	run := RunContext{
		AllowedTools: []ToolName{
			ToolWebListDatasources, ToolWebListModels,
		},
	}

	// Empty JSON object args — no identity fields at all.
	decision := pe.Evaluate(context.Background(), run, Proposal{
		Tool:      ToolWebListDatasources,
		Arguments: json.RawMessage(`{}`),
	})
	if !decision.Allowed {
		t.Errorf("expected web tool allowed, denied: %s", decision.ReasonCode)
	}
}

func TestPolicyEngine_DeniesWebToolNotInAllowlist(t *testing.T) {
	pe := &PolicyEngine{}
	run := RunContext{
		AllowedTools: []ToolName{ToolWebListDatasources},
	}

	decision := pe.Evaluate(context.Background(), run, Proposal{
		Tool:      ToolWebRunQuestion, // not in allowlist
		Arguments: json.RawMessage(`{}`),
	})
	if decision.Allowed {
		t.Error("expected denial for tool not in allowlist")
	}
	if decision.ReasonCode != ReasonToolNotAllowlisted {
		t.Errorf("reason = %s, want %s", decision.ReasonCode, ReasonToolNotAllowlisted)
	}
}

func TestPolicyEngine_WebToolRespectsRetryBudget(t *testing.T) {
	pe := &PolicyEngine{}
	run := RunContext{
		AllowedTools: []ToolName{ToolWebListDatasources},
		RetryBudget:  map[ToolName]int{ToolWebListDatasources: 0}, // exhausted
	}

	decision := pe.Evaluate(context.Background(), run, Proposal{
		Tool:      ToolWebListDatasources,
		Arguments: json.RawMessage(`{}`),
	})
	if decision.Allowed {
		t.Error("expected denial for exhausted retry budget")
	}
	if decision.ReasonCode != ReasonRetryBudgetExhausted {
		t.Errorf("reason = %s, want %s", decision.ReasonCode, ReasonRetryBudgetExhausted)
	}
}

func TestPolicyEngine_LegacyToolStillRequiresIdentity(t *testing.T) {
	// Legacy tools (catalog.resolve etc.) still require identity in args.
	pe := &PolicyEngine{}
	run := RunContext{
		TenantID:     "tenant-1",
		UserID:       "user-1",
		DatasourceID: "ds-1",
		AllowedTools: []ToolName{ToolCatalog},
	}

	// Empty args → identity fields are all "" → mismatch with RunContext.
	decision := pe.Evaluate(context.Background(), run, Proposal{
		Tool:      ToolCatalog,
		Arguments: json.RawMessage(`{}`),
	})
	if decision.Allowed {
		t.Error("expected legacy tool to require identity, but it was allowed")
	}
	if decision.ReasonCode != ReasonIdentityMismatch {
		t.Errorf("reason = %s, want %s", decision.ReasonCode, ReasonIdentityMismatch)
	}
}

func TestDecodeArgs_EmptyInput(t *testing.T) {
	in, err := decodeArgs[toolcontract.RunQuestionInput](nil)
	if err != nil {
		t.Fatalf("decode nil: %v", err)
	}
	if in.Question != "" {
		t.Errorf("expected empty struct, got %+v", in)
	}
}

func TestDecodeArgs_ValidInput(t *testing.T) {
	raw := mustMarshal(t, toolcontract.RunQuestionInput{
		DatasourceID: "ds",
		Question:     "hello",
	})
	in, err := decodeArgs[toolcontract.RunQuestionInput](raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if in.DatasourceID != "ds" || in.Question != "hello" {
		t.Errorf("decoded = %+v", in)
	}
}

func TestDecodeArgs_InvalidJSON(t *testing.T) {
	_, err := decodeArgs[toolcontract.RunQuestionInput](json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
