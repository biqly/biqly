package parity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/toolcontract"
)

func lqPtr(lq query.LogicalQuery) *query.LogicalQuery { return &lq }

func TestCompareReportsParityWhenPathsAgree(t *testing.T) {
	lq := cancelledOrdersTotal()
	mcp := PathResult{DatasourceID: DatasourceOrders, ModelID: ModelOrders, LogicalQuery: lqPtr(lq)}
	agent := PathResult{DatasourceID: DatasourceOrders, ModelID: ModelOrders, LogicalQuery: lqPtr(lq)}

	result := Compare("case-1", "iptal edilen siparişlerin tutarı", mcp, agent)

	if result.Diverged {
		t.Fatalf("expected parity, got divergence: %v", result.Notes)
	}
	if result.MCPFingerprint == "" || result.MCPFingerprint != result.AgentFingerprint {
		t.Fatalf("expected matching non-empty fingerprints, got mcp=%q agent=%q", result.MCPFingerprint, result.AgentFingerprint)
	}
}

func TestCompareDetectsModelSelectionDivergence(t *testing.T) {
	lq := cancelledOrdersTotal()
	mcp := PathResult{DatasourceID: DatasourceOrders, ModelID: "orders", LogicalQuery: lqPtr(lq)}
	agent := PathResult{DatasourceID: DatasourceOrders, ModelID: "orders-v2", LogicalQuery: lqPtr(lq)}

	result := Compare("case-2", "q", mcp, agent)

	if !result.Diverged {
		t.Fatal("expected divergence for mismatched model_id")
	}
	if !containsSubstring(result.Notes, "model_id mismatch") {
		t.Fatalf("expected a model_id mismatch note, got %v", result.Notes)
	}
}

func TestCompareDetectsLogicalQueryDivergence(t *testing.T) {
	mcpLQ := cancelledOrdersTotal()
	agentLQ := cancelledOrdersTotal()
	agentLQ.Filters = []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}}

	mcp := PathResult{DatasourceID: DatasourceOrders, ModelID: ModelOrders, LogicalQuery: lqPtr(mcpLQ)}
	agent := PathResult{DatasourceID: DatasourceOrders, ModelID: ModelOrders, LogicalQuery: lqPtr(agentLQ)}

	result := Compare("case-3", "q", mcp, agent)

	if !result.Diverged {
		t.Fatal("expected divergence for different LogicalQuery filters")
	}
	if !containsSubstring(result.Notes, "fingerprint mismatch") {
		t.Fatalf("expected a fingerprint mismatch note, got %v", result.Notes)
	}
	if result.MCPFingerprint == result.AgentFingerprint {
		t.Fatal("fingerprints should differ for different LogicalQuery documents")
	}
}

func TestCompareIsOrderInsensitiveForFiltersAndGroupBy(t *testing.T) {
	base := cancelledOrdersTotal()
	base.Filters = []query.Filter{
		{Field: "status", Operator: "eq", Value: "cancelled"},
		{Field: "country", Operator: "eq", Value: "DE"},
	}
	reordered := base
	reordered.Filters = []query.Filter{
		{Field: "country", Operator: "eq", Value: "DE"},
		{Field: "status", Operator: "eq", Value: "cancelled"},
	}

	mcp := PathResult{DatasourceID: DatasourceOrders, ModelID: ModelOrders, LogicalQuery: lqPtr(base)}
	agent := PathResult{DatasourceID: DatasourceOrders, ModelID: ModelOrders, LogicalQuery: lqPtr(reordered)}

	result := Compare("case-4", "q", mcp, agent)

	if result.Diverged {
		t.Fatalf("expected filter order to be irrelevant to parity, got: %v", result.Notes)
	}
}

func TestCompareFallsBackToRawResponseEqualityWhenNoLogicalQuery(t *testing.T) {
	mcp := PathResult{RawResponse: json.RawMessage(`[{"id":"ds-1","name":"Orders"}]`)}

	identical := PathResult{RawResponse: json.RawMessage(`[{"id":"ds-1","name":"Orders"}]`)}
	if r := Compare("case-5a", "q", mcp, identical); r.Diverged {
		t.Fatalf("expected identical raw responses to match, got: %v", r.Notes)
	}

	different := PathResult{RawResponse: json.RawMessage(`[{"id":"ds-2","name":"Marketing"}]`)}
	if r := Compare("case-5b", "q", mcp, different); !r.Diverged {
		t.Fatal("expected different raw responses to diverge")
	}

	// Key-order-only differences within an object still count as a match,
	// via canonicalizeJSON's re-marshal-through-any normalization.
	reorderedKeys := PathResult{RawResponse: json.RawMessage(`{"b":2,"a":1}`)}
	original := PathResult{RawResponse: json.RawMessage(`{"a":1,"b":2}`)}
	if r := Compare("case-5c", "q", original, reorderedKeys); r.Diverged {
		t.Fatalf("expected key-order-only difference to match, got: %v", r.Notes)
	}
}

func TestCompareDetectsErrorMismatch(t *testing.T) {
	mcp := PathResult{Error: ""}
	agent := PathResult{Error: "tool_error: HTTP 500"}

	result := Compare("case-6", "q", mcp, agent)
	if !result.Diverged {
		t.Fatal("expected divergence when only one path errored")
	}
}

func TestCompareDetectsLogicalQueryPresenceMismatch(t *testing.T) {
	mcp := PathResult{LogicalQuery: lqPtr(cancelledOrdersTotal())}
	agent := PathResult{}

	result := Compare("case-7", "q", mcp, agent)
	if !result.Diverged {
		t.Fatal("expected divergence when only one path produced a LogicalQuery")
	}
	if !containsSubstring(result.Notes, "presence mismatch") {
		t.Fatalf("expected a presence mismatch note, got %v", result.Notes)
	}
}

func TestExtractFromResponseParsesObjectResponse(t *testing.T) {
	raw := json.RawMessage(`{"datasource_id":"ds-1","model_id":"orders","logical_query":{"select":[{"type":"metric","name":"row_count"}],"limit":100}}`)

	res, err := ExtractFromResponse(toolcontract.ToolRunQuestion, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DatasourceID != "ds-1" || res.ModelID != "orders" {
		t.Fatalf("unexpected extracted fields: %+v", res)
	}
	if res.LogicalQuery == nil || len(res.LogicalQuery.Select) != 1 {
		t.Fatalf("expected a decoded LogicalQuery, got %+v", res.LogicalQuery)
	}
}

func TestExtractFromResponseToleratesArrayResponse(t *testing.T) {
	raw := json.RawMessage(`[{"id":"ds-1","name":"Orders"}]`)

	res, err := ExtractFromResponse(toolcontract.ToolListDatasources, raw)
	if err != nil {
		t.Fatalf("unexpected error for a list_* array response: %v", err)
	}
	if res.LogicalQuery != nil || res.DatasourceID != "" {
		t.Fatalf("expected no structured fields for an array response, got %+v", res)
	}
	if string(res.RawResponse) != string(raw) {
		t.Fatal("expected RawResponse to be preserved")
	}
}

func TestExtractFromResponseRejectsInvalidJSON(t *testing.T) {
	if _, err := ExtractFromResponse(toolcontract.ToolRunQuestion, json.RawMessage(`not json`)); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestReportSummarizesDivergence(t *testing.T) {
	report := Report{Cases: []CaseResult{
		{CaseID: "a", Question: "q1", Diverged: false},
		{CaseID: "b", Question: "q2", Diverged: true, Notes: []string{"model_id mismatch: mcp=\"x\" agent=\"y\""}},
	}}

	if !report.HasDivergence() {
		t.Fatal("expected HasDivergence to be true")
	}
	if len(report.Diverged()) != 1 || report.Diverged()[0].CaseID != "b" {
		t.Fatalf("expected exactly case b to be diverged, got %v", report.Diverged())
	}
	if !strings.Contains(report.String(), "DIVERGED") || !strings.Contains(report.String(), "PARITY") {
		t.Fatalf("expected report string to mention both statuses, got: %s", report.String())
	}
}

// TestNewFakeBackendIsDeterministic proves the fake governed backend used by
// both harness paths returns byte-identical responses for byte-identical
// requests — the precondition that makes any observed MCP-vs-agent
// divergence meaningful rather than backend noise.
func TestNewFakeBackendIsDeterministic(t *testing.T) {
	backend := NewFakeBackend()

	call := func() string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
			"/api/semantic/models?datasource_id="+DatasourceOrders, http.NoBody)
		backend.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	first, second := call(), call()
	if first != second {
		t.Fatalf("expected deterministic backend responses, got %q vs %q", first, second)
	}
}

// TestCasesBuildsUniqueIDsCoveringAllSixTools proves the fixed case set
// (consumed by internal/http's TestAgentMCPParity) builds without error, has
// unique case IDs, and exercises every one of the six governed tools at
// least once as either the direct MCP call or a step in the agent script.
func TestCasesBuildsUniqueIDsCoveringAllSixTools(t *testing.T) {
	cases, err := Cases()
	if err != nil {
		t.Fatalf("Cases() returned an error: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("expected a non-empty case set")
	}

	seenIDs := make(map[string]bool, len(cases))
	seenTools := make(map[toolcontract.ToolName]bool)
	for _, c := range cases {
		if seenIDs[c.ID] {
			t.Errorf("duplicate case ID %q", c.ID)
		}
		seenIDs[c.ID] = true
		seenTools[c.MCPTool] = true
		if len(c.AgentScript) == 0 {
			t.Errorf("case %q has an empty agent script", c.ID)
		}
	}

	for _, want := range []toolcontract.ToolName{
		toolcontract.ToolListDatasources, toolcontract.ToolListModels, toolcontract.ToolListPromptTemplates,
		toolcontract.ToolRunQuestion, toolcontract.ToolRunLogicalQuery, toolcontract.ToolListSkills,
		toolcontract.ToolRunSkill,
	} {
		if !seenTools[want] && !scriptMentionsTool(cases, want) {
			t.Errorf("no case exercises tool %q", want)
		}
	}
}

func scriptMentionsTool(cases []Case, tool toolcontract.ToolName) bool {
	for _, c := range cases {
		for _, step := range c.AgentScript {
			if strings.Contains(step, `"name":"`+string(tool)+`"`) {
				return true
			}
		}
	}
	return false
}

func containsSubstring(notes []string, substr string) bool {
	for _, n := range notes {
		if strings.Contains(n, substr) {
			return true
		}
	}
	return false
}
