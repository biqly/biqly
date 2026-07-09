// Package parity implements the MCP/web-agent parity harness (T13 of the Web
// Agent Mode plan): it runs a fixed natural-language question set through
// both governed surfaces — the MCP tool contract (internal/http/mcp_server.go)
// and the web agent's planner/tool loop (internal/agent.Runtime) — and reports
// any divergence in selected datasource/model or in the resulting
// LogicalQuery.
//
// This package holds the transport-agnostic pieces: the fixed case set, the
// deterministic fake governed backend both paths dispatch against, and the
// comparison logic. The actual wiring of the two real production code paths
// (internal/http.newMCPServer and internal/agent.Runtime) lives in
// internal/http's test suite — see internal/http/agent_mcp_parity_test.go —
// because the MCP server constructor is unexported and this harness is
// deliberately zero-touch on production code (see that file's doc comment
// for the full architecture rationale).
//
// Known gap: this harness drives the web-agent planner with a scripted
// (non-LLM) provider, exactly like the existing internal/ai/eval stub-provider
// harness drives the legacy single-shot pipeline. Neither harness exercises a
// real LLM's planning/tool-selection judgment — internal/ai/eval's stub
// harness cannot, because it drives the single-shot pipeline, not the agent's
// planner/tool loop (see tasks/todo.md, "Agentic Query Runner" Phase 4
// deferred items). This harness fills that gap for *structural* parity
// (shared dispatch contract, LogicalQuery equivalence) but not for real
// planner decision quality; that would need its own live-LLM harness, out of
// scope for T13.
package parity

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/toolcontract"
)

// PathResult is one governed path's (MCP or web agent) observable outcome for
// a single case: the raw tool response plus the structured fields callers
// care about for parity (selected datasource/model, resulting LogicalQuery).
type PathResult struct {
	// Tool is the governed tool whose response RawResponse carries (the last
	// one dispatched for this case — e.g. run_question, run_logical_query).
	Tool string
	// RawResponse is the raw JSON body returned by the shared /api/* backend
	// for Tool, exactly as the MCP or web-agent dispatch path saw it.
	RawResponse json.RawMessage
	// DatasourceID / ModelID are the datasource/model the shared backend
	// echoed back for this call, when applicable (empty for list_* tools).
	DatasourceID string
	ModelID      string
	// LogicalQuery is the compiled/executed query the shared backend echoed
	// back, when applicable (nil for list_* tools).
	LogicalQuery *query.LogicalQuery
	// Error is a non-empty dispatch/decode error description; when set the
	// other fields are not meaningful.
	Error string
}

// ExtractFromResponse decodes a governed tool's raw JSON response into a
// PathResult. Both the MCP and web-agent paths dispatch through the exact
// same toolcontract helpers against the exact same fake backend (see
// NewFakeBackend), so a single decode function used for both sides of a
// comparison is itself part of the parity guarantee: any divergence found by
// Compare is a genuine difference in what each path received, never an
// artifact of parsing them differently.
func ExtractFromResponse(tool toolcontract.ToolName, raw json.RawMessage) (PathResult, error) {
	res := PathResult{Tool: string(tool), RawResponse: raw}
	if !sonic.Valid(raw) {
		return res, fmt.Errorf("invalid JSON response for %s: %s", tool, raw)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		// list_* tools return a bare JSON array — no structured
		// datasource/model/logical_query fields to extract; RawResponse is
		// still compared by Compare's raw-equality fallback.
		return res, nil
	}

	var body struct {
		DatasourceID string          `json:"datasource_id,omitempty"`
		ModelID      string          `json:"model_id,omitempty"`
		LogicalQuery json.RawMessage `json:"logical_query,omitempty"`
	}
	if err := sonic.Unmarshal(raw, &body); err != nil {
		return res, fmt.Errorf("decode %s response: %w", tool, err)
	}
	res.DatasourceID = body.DatasourceID
	res.ModelID = body.ModelID
	if len(body.LogicalQuery) > 0 && string(body.LogicalQuery) != "null" {
		var lq query.LogicalQuery
		if err := sonic.Unmarshal(body.LogicalQuery, &lq); err != nil {
			return res, fmt.Errorf("decode %s logical_query: %w", tool, err)
		}
		res.LogicalQuery = &lq
	}
	return res, nil
}

// CaseResult is one case's parity verdict.
type CaseResult struct {
	CaseID   string
	Question string
	Diverged bool
	// Notes explains every divergence found, one entry per rule that fired.
	Notes []string

	MCP   PathResult
	Agent PathResult

	// MCPFingerprint / AgentFingerprint are the LogicalQuery audit
	// fingerprints (internal/query.ComputeFingerprint) for cases where both
	// paths produced a LogicalQuery; empty otherwise.
	MCPFingerprint   string
	AgentFingerprint string
}

func (c *CaseResult) diverge(format string, args ...any) {
	c.Diverged = true
	c.Notes = append(c.Notes, fmt.Sprintf(format, args...))
}

// Compare diffs one case's MCP and web-agent outcomes. It is pure and
// side-effect free so it can be exercised directly with synthetic
// PathResults (see parity_test.go) independent of the real MCP/agent wiring.
func Compare(caseID, question string, mcp, agentRes PathResult) CaseResult {
	result := CaseResult{CaseID: caseID, Question: question, MCP: mcp, Agent: agentRes}

	if mcp.Error != "" || agentRes.Error != "" {
		if mcp.Error != agentRes.Error {
			result.diverge("error mismatch: mcp=%q agent=%q", mcp.Error, agentRes.Error)
		}
		return result
	}

	if mcp.DatasourceID != "" && agentRes.DatasourceID != "" && mcp.DatasourceID != agentRes.DatasourceID {
		result.diverge("datasource_id mismatch: mcp=%q agent=%q", mcp.DatasourceID, agentRes.DatasourceID)
	}
	if mcp.ModelID != "" && agentRes.ModelID != "" && mcp.ModelID != agentRes.ModelID {
		result.diverge("model_id mismatch: mcp=%q agent=%q", mcp.ModelID, agentRes.ModelID)
	}

	switch {
	case mcp.LogicalQuery != nil && agentRes.LogicalQuery != nil:
		compareLogicalQueries(&result, mcp, agentRes)
	case mcp.LogicalQuery == nil && agentRes.LogicalQuery == nil:
		if !bytes.Equal(canonicalizeJSON(mcp.RawResponse), canonicalizeJSON(agentRes.RawResponse)) {
			result.diverge("raw response mismatch (no logical_query on either side)")
		}
	default:
		result.diverge("logical query presence mismatch: mcp_has=%v agent_has=%v",
			mcp.LogicalQuery != nil, agentRes.LogicalQuery != nil)
	}

	return result
}

func compareLogicalQueries(result *CaseResult, mcp, agentRes PathResult) {
	mcpFP, err := query.ComputeFingerprint(query.FingerprintInputs{
		LogicalQuery: mcp.LogicalQuery, DatasourceID: mcp.DatasourceID,
	})
	if err != nil {
		result.diverge("mcp logical_query fingerprint error: %v", err)
		return
	}
	agentFP, err := query.ComputeFingerprint(query.FingerprintInputs{
		LogicalQuery: agentRes.LogicalQuery, DatasourceID: agentRes.DatasourceID,
	})
	if err != nil {
		result.diverge("agent logical_query fingerprint error: %v", err)
		return
	}
	result.MCPFingerprint, result.AgentFingerprint = mcpFP, agentFP
	if mcpFP != agentFP {
		result.diverge("logical query fingerprint mismatch: mcp=%s agent=%s", mcpFP, agentFP)
	}
}

// canonicalizeJSON re-marshals raw through a generic any round-trip so two
// JSON documents that differ only in key order/whitespace compare equal.
// sonic.ConfigStd sorts map keys (SortMapKeys: true, matching
// encoding/json's behavior), which is what makes this stable.
func canonicalizeJSON(raw json.RawMessage) []byte {
	var v any
	if err := sonic.ConfigStd.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := sonic.ConfigStd.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

// Report is the full parity run's outcome across every case.
type Report struct {
	Cases []CaseResult
}

// Diverged returns every case whose comparison found a divergence.
func (r Report) Diverged() []CaseResult {
	var out []CaseResult
	for _, c := range r.Cases {
		if c.Diverged {
			out = append(out, c)
		}
	}
	return out
}

// HasDivergence reports whether any case diverged.
func (r Report) HasDivergence() bool {
	return len(r.Diverged()) > 0
}

// String renders a human-readable summary suitable for test logs or a saved
// report file.
func (r Report) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "parity report: %d case(s), %d diverged\n", len(r.Cases), len(r.Diverged()))
	for _, c := range r.Cases {
		status := "PARITY"
		if c.Diverged {
			status = "DIVERGED"
		}
		fmt.Fprintf(&b, "  [%s] %s (%q)\n", status, c.CaseID, c.Question)
		for _, note := range c.Notes {
			fmt.Fprintf(&b, "      - %s\n", note)
		}
	}
	return b.String()
}
