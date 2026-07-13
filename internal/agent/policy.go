package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/toolcontract"
)

// ToolName identifies one of the fixed set of tools a planner may call.
// Anything outside this set cannot be evaluated and is denied.
type ToolName string

const (
	ToolCatalog      ToolName = "catalog.resolve"
	ToolSemantic     ToolName = "semantic.resolve"
	ToolQueryCompile ToolName = "query.compile"
	ToolQueryExecute ToolName = "query.execute"
	ToolMemoryRecall ToolName = "memory.recall"

	// Web agent tools (MCP-parity governed set). These map 1:1 to the
	// toolcontract package's governed tools.
	ToolWebListDatasources     ToolName = ToolName(toolcontract.ToolListDatasources)
	ToolWebListModels          ToolName = ToolName(toolcontract.ToolListModels)
	ToolWebListPromptTemplates ToolName = ToolName(toolcontract.ToolListPromptTemplates)
	ToolWebRunQuestion         ToolName = ToolName(toolcontract.ToolRunQuestion)
	ToolWebRunLogicalQuery     ToolName = ToolName(toolcontract.ToolRunLogicalQuery)
	ToolWebListSkills          ToolName = ToolName(toolcontract.ToolListSkills)
	ToolWebRunSkill            ToolName = ToolName(toolcontract.ToolRunSkill)
	ToolWebDryPlan             ToolName = ToolName(toolcontract.ToolDryPlan)
	ToolWebDryRun              ToolName = ToolName(toolcontract.ToolDryRun)
	ToolWebMetricQuery         ToolName = ToolName(toolcontract.ToolMetricQuery)
	ToolWebListKnowledgeFiles  ToolName = ToolName(toolcontract.ToolListKnowledgeFiles)
	ToolWebReadKnowledgeFile   ToolName = ToolName(toolcontract.ToolReadKnowledgeFile)
)

// Proposal is a planner's request to invoke one tool with the given
// arguments, before policy has reviewed it.
type Proposal struct {
	Tool      ToolName
	Arguments json.RawMessage
}

// Decision is the policy engine's verdict. When Allowed is true, Arguments
// carries the (possibly narrowed) arguments the caller must use instead of
// the proposed ones — policy may tighten limits but never grants anything
// broader than what was proposed.
type Decision struct {
	Allowed    bool
	ReasonCode string
	Arguments  json.RawMessage
}

// Deny reason codes. Each names the specific rule that fired so callers can
// log, test, and alert on them without parsing free text.
const (
	ReasonToolNotAllowlisted    = "tool_not_allowlisted"
	ReasonRetryBudgetExhausted  = "retry_budget_exhausted"
	ReasonAirgappedEgressDenied = "airgapped_egress_denied"
	ReasonMalformedArguments    = "malformed_arguments"
	ReasonIdentityMismatch      = "identity_mismatch"
	ReasonPromptInjection       = "prompt_injection_suspected"
	ReasonMultiStatementSQL     = "multi_statement_sql_denied"
	ReasonWriteOrDDLDenied      = "write_or_ddl_denied"
	ReasonHiddenColumnDenied    = "hidden_column_denied"
	ReasonPIIMaskingRequired    = "pii_masking_required"
	ReasonInvalidJoinDenied     = "invalid_join_denied"
	ReasonRowFilterRequired     = "row_filter_required"
	ReasonContextCanceled       = "context_canceled"
)

// JoinEdge is an unordered pair of fully-qualified tables a query proposal
// joins. Equal regardless of Left/Right order.
type JoinEdge struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

func (e JoinEdge) key() string {
	l, r := strings.ToLower(e.Left), strings.ToLower(e.Right)
	if l > r {
		l, r = r, l
	}
	return l + "|" + r
}

// PriorTurn is a compact conversation-history item shown to the planner so
// follow-up questions can inherit earlier filters, date ranges and grouping.
type PriorTurn struct {
	User          string `json:"user,omitempty"`
	Assistant     string `json:"assistant,omitempty"`
	ResultSummary string `json:"result_summary,omitempty"`
}

// RunContext carries the run-scoped facts already authorized upstream (by
// auth, RBAC, and the resolved semantic model) that every proposal is
// checked against. It is built once per run from trusted sources — never
// from tool arguments, which are untrusted planner output.
type RunContext struct {
	TenantID     string
	UserID       string
	DatasourceID string
	// ModelID is the optional published semantic model selected in the UI.
	// Injected into run_logical_query when the planner omits it — same
	// pattern as DatasourceID. Empty means auto-routing / no pinned model.
	ModelID string
	// Question is the user's original natural-language question. Policy
	// does not use it; the Planner does (see internal/agent/provider_planner.go).
	Question string
	// PriorTurns are recent conversation turns from the web chat request.
	// Policy does not use them; the Planner does for follow-up inheritance.
	PriorTurns []PriorTurn
	// ClarificationHistory is set only when resuming a run that has paused on
	// one or more clarification_required events: it carries every round's
	// question and the user's answer so far, oldest first, so the planner's
	// next Decide call sees the FULL round-trip history (not just the most
	// recent round) instead of re-asking a question it — or an earlier round
	// — already has an answer for. Policy does not use it; the Planner does
	// (see internal/agent/provider_planner.go).
	ClarificationHistory []ClarificationExchange

	// AllowedTools is the fixed tool allowlist for this run.
	AllowedTools []ToolName
	// RetryBudget caps remaining attempts per tool; the caller decrements it
	// after each attempt. A tool absent from the map has no budget.
	RetryBudget map[ToolName]int

	// DeploymentMode gates external egress; see ExternalEgressTools.
	DeploymentMode string
	// ExternalEgressTools marks which tools' upstream call leaves the
	// cluster. In an airgapped deployment those tools are denied outright.
	ExternalEgressTools map[ToolName]bool

	// HiddenColumns and PIIColumns are fully-qualified "table.column"
	// entries, matched case-insensitively.
	HiddenColumns []string
	PIIColumns    []string
	// AllowedJoins is the join graph the published semantic model actually
	// declares; a proposal cannot reference a join outside this set.
	AllowedJoins []JoinEdge
	// RequiredRowFilter forces every query proposal to carry a non-empty
	// row-level-security predicate.
	RequiredRowFilter bool

	MaxRows int
	Timeout time.Duration

	// Credential is the caller's JWT/PAT forwarded to loopback HTTP calls
	// by the web agent tools. Set only on the web path (channel=agent).
	Credential toolcontract.Credential

	// MaxSteps and MaxClarificationRounds bound the runtime's planning loop
	// (internal/agent/runtime.go); PolicyEngine.Evaluate does not use them.
	MaxSteps               int
	MaxClarificationRounds int
}

// PolicyEngine evaluates tool-call proposals against a RunContext. It has no
// dependencies beyond its inputs — no network calls, no clock reads beyond
// what ctx/RunContext already carry — so the same (ctx, run, proposal)
// always yields the same Decision.
type PolicyEngine struct{}

// identityArgs is the tenant/user/datasource identity every tool's
// arguments must carry; the policy engine trusts none of it until it matches
// RunContext.
type identityArgs struct {
	TenantID     string `json:"tenant_id"`
	UserID       string `json:"user_id"`
	DatasourceID string `json:"datasource_id"`
}

// queryProposalArgs is the full shape of query.compile / query.execute
// arguments, shared by the policy engine and the query tool adapters
// (internal/agent/query_tools.go) so both sides decode the exact same
// wire shape. LogicalQuery/Fingerprint carry the real tool payload;
// SQL/Columns/Joins/RowFilterSQL are the flat, policy-inspectable summary
// the runtime derives from that LogicalQuery for query.compile proposals —
// they are typically empty on query.execute proposals, whose compile-time
// shape was already checked when they were query.compile proposals.
type queryProposalArgs struct {
	identityArgs
	LogicalQuery   json.RawMessage `json:"logical_query,omitempty"`
	Fingerprint    string          `json:"fingerprint,omitempty"`
	SQL            string          `json:"sql,omitempty"`
	Columns        []string        `json:"columns,omitempty"`
	MaskedColumns  []string        `json:"masked_columns,omitempty"`
	Joins          []JoinEdge      `json:"joins,omitempty"`
	RowFilterSQL   string          `json:"row_filter_sql,omitempty"`
	RowLimit       int             `json:"row_limit,omitempty"`
	TimeoutSeconds int             `json:"timeout_seconds,omitempty"`
}

func deny(reason string) Decision {
	return Decision{Allowed: false, ReasonCode: reason}
}

// Evaluate applies every deterministic rule in turn, denying on the first
// violation. A proposal that survives may come back with narrowed
// Arguments (tighter row limit / timeout) but never broader ones.
func (*PolicyEngine) Evaluate(ctx context.Context, run RunContext, proposal Proposal) Decision {
	if err := ctx.Err(); err != nil {
		return deny(ReasonContextCanceled)
	}
	if !containsTool(run.AllowedTools, proposal.Tool) {
		return deny(ReasonToolNotAllowlisted)
	}
	if budget, ok := run.RetryBudget[proposal.Tool]; ok && budget <= 0 {
		return deny(ReasonRetryBudgetExhausted)
	}
	if run.DeploymentMode == "airgapped" && run.ExternalEgressTools[proposal.Tool] {
		return deny(ReasonAirgappedEgressDenied)
	}

	// Lenient here: this only peeks at the identity fields every tool shares.
	// The full shape is validated strictly per-tool below (e.g. evaluateQuery).
	// Web tools (channel=agent) carry no identity in their arguments — identity
	// comes from RunContext.Credential, verified by the /api/* middleware chain.
	isWebTool := isWebAgentTool(proposal.Tool)
	if !isWebTool {
		var ident identityArgs
		if err := sonic.Unmarshal(proposal.Arguments, &ident); err != nil {
			return deny(ReasonMalformedArguments)
		}
		if ident.TenantID != run.TenantID || ident.UserID != run.UserID || ident.DatasourceID != run.DatasourceID {
			return deny(ReasonIdentityMismatch)
		}
		if containsPromptInjection(string(proposal.Arguments)) {
			return deny(ReasonPromptInjection)
		}
	}

	switch proposal.Tool {
	case ToolQueryCompile, ToolQueryExecute:
		return evaluateQuery(run, proposal)
	case ToolCatalog, ToolSemantic, ToolMemoryRecall:
		return Decision{Allowed: true, Arguments: proposal.Arguments}
	case ToolWebListDatasources, ToolWebListModels, ToolWebListPromptTemplates,
		ToolWebRunQuestion, ToolWebRunLogicalQuery, ToolWebListSkills, ToolWebRunSkill,
		ToolWebDryPlan, ToolWebDryRun, ToolWebMetricQuery,
		ToolWebListKnowledgeFiles, ToolWebReadKnowledgeFile:
		return Decision{Allowed: true, Arguments: proposal.Arguments}
	default:
		return deny(ReasonToolNotAllowlisted)
	}
}

// webAgentTools is the set of MCP-parity web agent tools, for fast membership
// testing without triggering the exhaustive linter on a switch statement.
var webAgentTools = []ToolName{
	ToolWebListDatasources, ToolWebListModels, ToolWebListPromptTemplates,
	ToolWebRunQuestion, ToolWebRunLogicalQuery, ToolWebListSkills, ToolWebRunSkill,
	ToolWebDryPlan, ToolWebDryRun, ToolWebMetricQuery,
	ToolWebListKnowledgeFiles, ToolWebReadKnowledgeFile,
}

// isWebAgentTool reports whether tool is one of the MCP-parity web agent tools.
func isWebAgentTool(tool ToolName) bool {
	return slices.Contains(webAgentTools, tool)
}

func evaluateQuery(run RunContext, proposal Proposal) Decision {
	var args queryProposalArgs
	if err := strictDecode(proposal.Arguments, &args); err != nil {
		return deny(ReasonMalformedArguments)
	}

	if isMultiStatementSQL(args.SQL) {
		return deny(ReasonMultiStatementSQL)
	}
	if isWriteOrDDL(args.SQL) {
		return deny(ReasonWriteOrDDLDenied)
	}
	// The hidden-column / PII / allowed-join / row-filter checks below are
	// defense-in-depth that only fire when the corresponding RunContext fields
	// are populated (they are empty on the NATS agent path today). They are NOT
	// the authoritative control: the Query service re-validates columns, joins,
	// PII masking, and row-level security at compile time. Each check is inert
	// (allows) when its RunContext slice is empty, deferring to that enforcement.
	if col := firstHiddenColumn(args.Columns, run.HiddenColumns); col != "" {
		return deny(ReasonHiddenColumnDenied)
	}
	if col := firstUnmaskedPIIColumn(args.Columns, args.MaskedColumns, run.PIIColumns); col != "" {
		return deny(ReasonPIIMaskingRequired)
	}
	if join := firstInvalidJoin(args.Joins, run.AllowedJoins); join != nil {
		return deny(ReasonInvalidJoinDenied)
	}
	if run.RequiredRowFilter && strings.TrimSpace(args.RowFilterSQL) == "" {
		return deny(ReasonRowFilterRequired)
	}

	// Narrow, never expand: clamp row limit and timeout to the run's ceiling.
	if run.MaxRows > 0 && (args.RowLimit <= 0 || args.RowLimit > run.MaxRows) {
		args.RowLimit = run.MaxRows
	}
	if maxSeconds := int(run.Timeout.Seconds()); maxSeconds > 0 && (args.TimeoutSeconds <= 0 || args.TimeoutSeconds > maxSeconds) {
		args.TimeoutSeconds = maxSeconds
	}

	narrowed, err := sonic.Marshal(args)
	if err != nil {
		return deny(ReasonMalformedArguments)
	}
	return Decision{Allowed: true, Arguments: narrowed}
}

func containsTool(allowlist []ToolName, tool ToolName) bool {
	return slices.Contains(allowlist, tool)
}

// strictDecode rejects unknown fields so a proposal cannot smuggle
// unrecognized keys past the policy engine's typed view of its arguments.
func strictDecode(raw json.RawMessage, v any) error {
	dec := sonic.ConfigStd.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode arguments: %w", err)
	}
	return nil
}

// promptInjectionMarkers are deterministic substring signals of an attempt to
// override the run's system instructions via tool arguments. This is a coarse,
// NON-AUTHORITATIVE signal (trivially bypassed by rewording/translation), not a
// security control: it runs only for non-web tools and the authoritative
// protection is the strict tool contract plus the governed /api/* enforcement
// that every web tool's arguments pass through. Treat a match as telemetry.
var promptInjectionMarkers = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"disregard the system prompt",
	"you are now",
	"new instructions:",
}

func containsPromptInjection(raw string) bool {
	lower := strings.ToLower(raw)
	for _, marker := range promptInjectionMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// isMultiStatementSQL reports whether sql contains more than one statement:
// a semicolon followed by further non-whitespace content.
func isMultiStatementSQL(sql string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(sql), ";")
	idx := strings.Index(trimmed, ";")
	return idx >= 0 && strings.TrimSpace(trimmed[idx+1:]) != ""
}

// writeOrDDLKeywords are leading SQL keywords the agent's read-only tools
// must never execute.
var writeOrDDLKeywords = []string{
	"insert", "update", "delete", "drop", "alter", "create", "truncate", "grant", "revoke",
}

func isWriteOrDDL(sql string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(sql)))
	if len(fields) == 0 {
		return false
	}
	leading := strings.TrimPrefix(fields[0], "(")
	return slices.Contains(writeOrDDLKeywords, leading)
}

func firstHiddenColumn(requested, hidden []string) string {
	for _, col := range requested {
		if containsFold(hidden, col) {
			return col
		}
	}
	return ""
}

func firstUnmaskedPIIColumn(requested, masked, pii []string) string {
	for _, col := range requested {
		if containsFold(pii, col) && !containsFold(masked, col) {
			return col
		}
	}
	return ""
}

func containsFold(list []string, target string) bool {
	for _, v := range list {
		if strings.EqualFold(v, target) {
			return true
		}
	}
	return false
}

func firstInvalidJoin(requested, allowed []JoinEdge) *JoinEdge {
	// An empty allowlist means the join restriction is not configured for this
	// run (e.g. RunContext.AllowedJoins not populated on the NATS agent path),
	// NOT "deny every join". Treating it as deny-all would reject any join a
	// planner proposes; join validity is enforced authoritatively downstream by
	// the Query service at compile time. Only enforce when an allowlist exists.
	if len(allowed) == 0 {
		return nil
	}
	allowedKeys := make(map[string]struct{}, len(allowed))
	for _, j := range allowed {
		allowedKeys[j.key()] = struct{}{}
	}
	for i, j := range requested {
		if _, ok := allowedKeys[j.key()]; !ok {
			return &requested[i]
		}
	}
	return nil
}
