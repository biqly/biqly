package http

// T13 parity harness: runs a fixed question set through both governed
// surfaces and reports any drift between them.
//
// Architecture: in-process, go-test-driven (not a cmd/ binary). Both
// production code paths this test drives are real, unmodified production
// code:
//   - MCP path: the actual newMCPServer (internal/http/mcp_server.go),
//     driven the same way internal/http/mcp_server_test.go already does
//     (mcpTestSession/mcpTestBackend) — reused as-is, no new plumbing.
//   - Web agent path: the actual internal/agent.Runtime +
//     internal/agent.NewProviderPlanner + internal/agent.NewWebTools, the
//     same production types internal/agent/provider_planner_test.go's
//     TestProviderPlannerWebHappyPathListModelsRunQuestionFinal exercises,
//     just with the real WebTools (backed by toolcontract.HTTPDispatcher)
//     instead of that test's staticTool fakes.
//
// This lives in package http (rather than a standalone cmd/ or a fully
// self-contained package) for exactly one reason: newMCPServer is
// unexported, and exporting it purely for this dev tool would be a
// production-code change this task's scope explicitly avoids ("don't
// restructure the MCP server ... to accommodate it"). Living in the same
// package is the zero-touch way to reach the real constructor. Everything
// comparison-related (fixed case set, fake governed backend, drift
// comparison logic) lives in the transport-agnostic, independently unit
// tested internal/agent/parity package and is merely wired here.
//
// Both paths dispatch against internal/agent/parity.NewFakeBackend(), a
// deterministic in-memory double for the real /api/* router + DB +
// ai.Service — not a live server or a real LLM. See that package's doc
// comment for why (this harness cannot exercise real NL routing without a
// live provider, matching the documented internal/ai/eval stub-provider gap
// for the agent's planner loop; tasks/todo.md's "Agentic Query Runner" Phase
// 4 deferred items). What this harness DOES verify for real: that the
// six-tool contract (internal/toolcontract) dispatches identically
// regardless of which channel calls it, and that a web-agent plan that lands
// on the same tool+arguments as a direct MCP call produces an equivalent
// LogicalQuery (via internal/query.ComputeFingerprint).
//
// Run standalone: go test -run TestAgentMCPParity ./internal/http/... -v
//
// Not CI-gating in the sense of tasks/todo.md's eval-regression note (it is
// not, and cannot be, added to `make eval-regression` — see the parity
// package doc comment). It DOES run under plain `go test ./...`/`make
// test-go` like every other test in this file's package, which is
// intentional: it is fully deterministic and fast (no live LLM, no DB), so
// it is a cheap regression guard against a future change accidentally
// breaking channel parity — unlike eval-regression's real-LLM nightly gate,
// there is no reason to keep it out of the ordinary test run.
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/agent/parity"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/toolcontract"
)

// allWebAgentTools is the fixed six-tool allowlist every parity case's
// RunContext grants — mirrors webAgentAllowedTools' unrestricted case in
// production (internal/http/handlers/ai_agent_chat.go).
var allWebAgentTools = []agent.ToolName{
	agent.ToolWebListDatasources, agent.ToolWebListModels, agent.ToolWebRunQuestion,
	agent.ToolWebRunLogicalQuery, agent.ToolWebListSkills, agent.ToolWebRunSkill,
}

// TestAgentMCPParity runs internal/agent/parity.Cases() through the real MCP
// tool surface and the real web-agent planner/tool loop and asserts no
// divergence — this is T13's "Done when" criterion.
func TestAgentMCPParity(t *testing.T) {
	backend := parity.NewFakeBackend()
	cred := toolcontract.Credential{Authorization: "Bearer test-token", APIKey: "test-key"}

	cases, err := parity.Cases()
	if err != nil {
		t.Fatalf("build parity case set: %v", err)
	}

	report := parity.Report{}
	for _, c := range cases {
		mcpResult := runParityMCPCase(t, backend, c)
		agentResult := runParityAgentCase(t, backend, cred, c)
		report.Cases = append(report.Cases, parity.Compare(c.ID, c.Question, mcpResult, agentResult))
	}

	t.Log(report.String())
	for _, diverged := range report.Diverged() {
		t.Errorf("parity divergence in case %q (%q): %v", diverged.CaseID, diverged.Question, diverged.Notes)
	}
}

// runParityMCPCase drives the real MCP server (newMCPServer, same package)
// exactly the way mcp_server_test.go does: an in-memory client/server
// transport pair, one direct CallTool per case (no planner — an MCP client
// decides its own tool calls).
func runParityMCPCase(t *testing.T, backend http.Handler, c parity.Case) parity.PathResult {
	t.Helper()
	session := mcpTestSession(t, backend)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      string(c.MCPTool),
		Arguments: c.MCPArgs,
	})
	if err != nil {
		return parity.PathResult{Error: fmt.Sprintf("mcp call_tool: %v", err)}
	}
	text, ok := firstTextContent(res)
	if !ok {
		return parity.PathResult{Error: fmt.Sprintf("mcp tool %s returned no text content", c.MCPTool)}
	}
	if res.IsError {
		return parity.PathResult{Error: "mcp tool error: " + text}
	}
	out, err := parity.ExtractFromResponse(c.MCPTool, json.RawMessage(text))
	if err != nil {
		return parity.PathResult{Error: fmt.Sprintf("extract mcp response: %v", err)}
	}
	return out
}

func firstTextContent(res *mcp.CallToolResult) (string, bool) {
	if len(res.Content) == 0 {
		return "", false
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		return "", false
	}
	return tc.Text, true
}

// runParityAgentCase drives the real agent.Runtime + agent.ProviderPlanner +
// agent.WebTools against the same fake backend, with a scripted (non-LLM)
// provider replaying the case's fixed plan — see this file's doc comment for
// why a scripted provider stands in for a real LLM.
func runParityAgentCase(t *testing.T, backend http.Handler, cred toolcontract.Credential, c parity.Case) parity.PathResult {
	t.Helper()
	provider := &scriptedPlannerProvider{steps: c.AgentScript}
	planner := agent.NewProviderPlanner(provider)
	webTools := agent.NewWebTools(&toolcontract.HTTPDispatcher{API: backend}, cred)
	registry := agent.NewRegistry(&agent.PolicyEngine{}, webTools.All()...)

	run := agent.RunContext{
		TenantID:               "parity-tenant",
		UserID:                 "parity-user",
		DatasourceID:           c.DatasourceID,
		Question:               c.Question,
		AllowedTools:           allWebAgentTools,
		Credential:             cred,
		MaxRows:                1000,
		Timeout:                30 * time.Second,
		MaxSteps:               6,
		MaxClarificationRounds: 2,
	}

	state, err := agent.NewRuntime(planner, registry, newMemStateStore()).Run(context.Background(), run, "parity-"+c.ID)
	if err != nil {
		return parity.PathResult{Error: fmt.Sprintf("runtime.Run: %v", err)}
	}
	if state.Terminal == nil || state.Terminal.Kind != agent.DecisionFinal {
		return parity.PathResult{Error: fmt.Sprintf("run did not reach a final answer: %+v", state.Terminal)}
	}

	for _, step := range state.Steps {
		if step.Proposal.Tool != agent.ToolName(c.CompareTool) || step.Observation == nil {
			continue
		}
		out, err := parity.ExtractFromResponse(c.CompareTool, step.Observation.Payload)
		if err != nil {
			return parity.PathResult{Error: fmt.Sprintf("extract agent response: %v", err)}
		}
		return out
	}
	return parity.PathResult{Error: fmt.Sprintf("no step called compare tool %s", c.CompareTool)}
}

// scriptedPlannerProvider replays a fixed sequence of planner-decision JSON
// envelopes, one per Generate call — the deterministic stand-in for a real
// LLM (see this file's doc comment on the stub-provider gap).
type scriptedPlannerProvider struct {
	steps []string
	calls int
}

func (p *scriptedPlannerProvider) Generate(_ context.Context, _ string) (providerpkg.GenerationResult, error) {
	if p.calls >= len(p.steps) {
		return providerpkg.GenerationResult{}, fmt.Errorf("scripted planner provider exhausted after %d call(s)", p.calls)
	}
	content := p.steps[p.calls]
	p.calls++
	return providerpkg.GenerationResult{Content: content}, nil
}

func (p *scriptedPlannerProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return p.Generate(ctx, prompt)
}

// memStateStore is a trivial in-memory agent.StateStore — this harness never
// crashes or resumes mid-run, so it only needs to satisfy Runtime's Save/Load
// calls within a single Run.
type memStateStore struct {
	mu    sync.Mutex
	state map[string]agent.RuntimeState
}

func newMemStateStore() *memStateStore {
	return &memStateStore{state: make(map[string]agent.RuntimeState)}
}

func (s *memStateStore) Save(_ context.Context, runID string, state agent.RuntimeState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[runID] = state
	return nil
}

func (s *memStateStore) Load(_ context.Context, runID string) (agent.RuntimeState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.state[runID]
	return st, ok, nil
}
