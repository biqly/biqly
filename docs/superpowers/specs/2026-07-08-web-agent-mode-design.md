# Web Agent Mode for Biqly AI Chat — Design

Date: 2026-07-08
Status: draft (pending user approval)
Related: `docs/superpowers/specs/2026-07-06-agentic-query-runner-service-design.md` (internal runner),
`tasks/wren-agentic-workflow-findings.md` (reference-agent behavior gaps).

## Goal

Bring MCP-quality agentic query behavior into the web UI by routing web chat messages
through the existing bounded agent runtime (`internal/agent.Runtime`) equipped with the
same governed tool contract MCP exposes. The web agent must produce, for the same
question, the same datasource/model selection, the same governed execution path, a
comparable LogicalQuery, and a richer final answer (business summary + table + chart
suggestion + trace) than the current single-shot `/api/ai/query/run` path.

### Success criteria (from the ticket)

1. Web agent never accepts or emits raw SQL as user-controlled execution input.
2. All tool calls pass through existing `/api/*` governed paths.
3. Agent can call `list_models`, `list_datasources`, `run_question`, `run_logical_query`, `list_skills`, `run_skill`.
4. Frontend streams agent progress with SSE.
5. Final answer includes business summary, result table/chart suggestion, and trace.
6. Existing auth, RLS, PII masking, spend caps, and audit apply unchanged.

## Locked decisions

1. **In-process + SSE (MVP).** The agent loop runs inside the AI service for the
   duration of the HTTP request. Tools call `/api/*` in loopback with the **caller's own
   credentials** (JWT/PAT forwarded, exactly the `mcpDispatcher` pattern in
   `internal/http/mcp_server.go`). This makes criterion 6 true by construction — the
   governance lives in the `/api/*` middleware chain and the query compiler, and the
   web agent traverses it as the end user. The NATS-async `cmd/agent` path stays as-is
   (shadow evaluation of the legacy pipeline) and is a later phase for the web agent.
2. **MCP-parity 6-tool set.** A new shared tool-contract package defines the six tools
   above once; the MCP server and the web agent's tool registry both consume it. The
   internal runner's existing 5 tools (`catalog.resolve` …) are untouched.
3. **Reuse `ai_conversations` / `ai_conversation_messages`.** No new conversation
   tables. The **client remains the single conversation writer** (existing snapshot +
   idempotency ledger from migration 063a). The server persists `agent_runs` +
   `agent_steps` only; the final SSE `result` payload carries everything the client
   needs to store the assistant message (including `run_id`, so the existing
   `RunTracePanel` linkage keeps working). This avoids a second writer racing the
   snapshot idempotency layer.
4. **Dedicated `agent` purpose in the provider registry (MVP).** `PurposeAgent` is
   added to `internal/ai/provider_store.go` and the admin model UI, so the planner and
   finalizer can use a stronger model than query generation.

## Architecture (MVP)

```
Frontend ChatPanel (Agent Mode toggle)
      | fetch POST /api/agent/chat   (Accept: text/event-stream, Authorization: Bearer …)
      v
AI service — WebAgentHandler
      |  creates agent_runs row (web channel), builds RunContext from auth context
      v
internal/agent.Runtime (existing bounded loop, resumable state)
      |  planner: ProviderPlanner on PurposeAgent
      v
Web tool registry (6 tools from internal/toolcontract)
      |  loopback HTTP with caller's Authorization header, X-Biqly-Channel: agent
      v
Existing /api/* governed paths (auth → RBAC → datasource access → RLS/PII in compiler
      → spend cap → audit)
      |
      v
SSE frames back to the browser: run_started / step / clarification_required / result / done
```

### Endpoint

`POST /api/agent/chat` — a single streaming request; the response is
`text/event-stream` (pattern: `newEvalSSESender`, `internal/http/handlers/ai_eval.go:275`).
One request = one agent run. No separate job/stream endpoint pair in the MVP: the
in-process loop means connection lifetime equals run lifetime, no cross-request state,
and the JWT is present on the one request that needs it. Client cancel = aborting the
fetch, which cancels the request context and the run.

Request body:

```json
{
  "message": "Geçen ay bölge bazlı ciroyu göster",
  "conversation_id": "…",            // optional, for agent_runs linkage
  "datasource_id": "…",              // optional preselection
  "prior_turns": [...],               // same shape the legacy path already sends
  "resume_run_id": "…",              // set when answering a clarification
  "clarification_answer": "…"        // free text or a choice id
}
```

Gateway note: the route needs the same 1800s HTTPRoute timeout treatment as
`/api/ai` and `/api/mcp` (`route.timeouts.request` in Helm values).

### SSE event contract

```
{ "type": "run_started", "run_id": "…" }
{ "type": "step", "kind": "tool_call_started",   "tool": "list_models", "seq": 2 }
{ "type": "step", "kind": "tool_call_completed", "tool": "list_models", "seq": 2, "summary": "…", "duration_ms": 412 }
{ "type": "clarification_required", "question": "…", "choices": [{"id": "net_revenue", "label": "Net satış tutarı"}], "allow_free_text": true }
{ "type": "result", "payload": { …final answer, see Finalizer… } }
{ "type": "error", "code": "…", "message": "…" }
data: [DONE]
```

Step payloads sent to the browser are **summaries**, not raw tool I/O: tool outputs are
truncated and never include result rows beyond what the final payload carries (PII
masking already happened in the compiler, but we still don't stream intermediate row
data). Full fidelity persists in `agent_steps` as today.

## Shared tool contract — `internal/toolcontract`

New package extracted from the inline bodies in `internal/http/mcp_server.go`:

- For each of the six tools: name, description, input struct with `jsonschema` tags,
  target method+path on `/api/*`.
- `Dispatcher` interface: `Call(ctx, tool, args, credential) (json.RawMessage, error)` —
  one implementation synthesizes loopback HTTP requests (same as `mcpDispatcher`:
  forward `Authorization`/`X-API-Key`, set `X-Biqly-Channel`).
- `internal/http/mcp_server.go` re-registers its tools from this package (mechanical
  refactor; `mcp_server_test.go` must stay green, channel remains `mcp`).
- The web agent's tool adapters (`agent.Tool` implementations) call the same
  `Dispatcher` with channel `agent`.

This is the "same tool contract" criterion made literal: one definition, two consumers.

## Runtime configuration for the web path

The existing `Runtime` bounds are reused with web-specific values (new
`BI_WEB_AGENT_*` env keys, defaults in parentheses):

- `BI_WEB_AGENT_ENABLED` (false) — feature flag, plus workspace allowlist reuse.
- max steps (6), max clarification rounds (2) — same semantics as today.
- run timeout (120s) — set at run construction on the web path; the 45s cap in
  `contracts.go` applies only to NATS job envelope validation and is not changed.
- per-tool call caps enforced by the policy allowlist: `run_question` ≤ 3,
  `run_logical_query` ≤ 2, `list_models`/`list_datasources`/`list_skills` ≤ 2 each,
  `run_skill` ≤ 2.
- max result rows visible to the LLM: 100 (planner/finalizer context truncation);
  the full governed result (row-limited by the query path itself) goes to the client.

Role-based tool policy (viewer: list/run_question/run_skill; analyst: + run_logical_query)
derives from the auth context the same way `/api/*` RBAC already gates the underlying
endpoints — the policy allowlist is a cheap first gate, the HTTP middleware remains the
authority.

## Planner

`ProviderPlanner` is reused with a new system prompt for the web tool set:

- You are Biqly's governed BI agent. Never write raw SQL. Never ask the user to run SQL.
- Use tools to inspect models, run questions, run skills, or run LogicalQuery.
- Prefer skills and confirmed examples before generating from scratch (`list_skills` first
  when the question resembles a known task).
- When the metric/grain is ambiguous, emit a structured clarification (numbered choices +
  free text) instead of guessing — clarify-discipline per
  `tasks/wren-agentic-workflow-findings.md` P0-B.
- Prior turns are provided; inherit filters/date context on follow-ups.

Planner and finalizer LLM calls run on `PurposeAgent` and are wrapped with
`SpendLimiter.Check` before / `Record` after, per call (the multi-step loop burns more
tokens than one-shot, so recording is per-step).

## Clarification round-trip

When the planner emits a clarification, the run pauses exactly as today
(`runtime_state` persisted, terminal not reached), the handler sends
`clarification_required` + `[DONE]` and ends the stream. The user's answer arrives as a
new `POST /api/agent/chat` with `resume_run_id` + `clarification_answer`; the handler
loads the persisted `RuntimeState`, appends the answer to history, and resumes the loop
on a fresh stream. Identity check: the resuming caller must match the run's
tenant/user. Runs left unresumed simply expire (no reaper in MVP; `agent_runs` status
already records `waiting_clarification`).

## Finalizer

After the loop reaches `final`, a finalizer composes the result payload — reusing
existing services rather than a new pipeline:

- **Business summary**: `Service.SynthesizeAnswer` (`internal/ai/answer.go`) over the
  (truncated) result — localized, business language, numbers formatted, caveats when
  the row limit or missing data applies.
- **Chart suggestion**: the existing result-shape heuristic used by the NL-caption work
  (categorical → bar desc, time → line).
- **Follow-ups**: `attachSuggestedFollowUps` chips.
- **Trace**: ordered `agent_steps` summaries (tool names + durations) — feeds the
  existing `RunTracePanel` via `run_steps`/`run_id`.
- **LogicalQuery + SQL preview**: included per existing permission gating (SQL preview
  only where the current UI already shows it — the agent never treats SQL as input).

The `result` payload lands in the same message shape the legacy path produces
(`ai_response` JSONB), so `AssistantMessageCard` renders it with minimal changes.

## Frontend

- **Agent Mode toggle** in `ChatPanel.tsx` beside the context/auto-find toggles,
  localStorage-persisted, threaded from `AIQuery.tsx` like `autoFindEnabled`.
- **SSE client** (first in the codebase): `fetch` + `ReadableStream.getReader()` parser
  (native `EventSource` can't POST or carry the Authorization header). Lives in
  `frontend/src/api/agentStream.ts` with types in `frontend/src/types/agent.ts`.
- **Live trace**: streamed `step` events feed `RunTracePanel` incrementally instead of
  post-hoc `GET /api/ai/runs/{id}`.
- **Clarification card**: numbered clickable choices + free-text + skip, reusing the
  existing clarification surface (`clarificationStage.ts`); a choice triggers the
  resume POST.
- **Result rendering**: existing `AssistantMessageCard` sections (table, chart, caption,
  follow-ups, SQL preview, trace).
- Cancel button aborts the fetch. Errors render through the existing error section.
- All new strings via `useT()`.

## Error handling and failure behavior

- Loop failures map to the SSE `error` event with the runtime's deny/fail reason codes;
  the run row is completed terminal as today.
- Mid-run token expiry: loopback calls return 401 → surfaced as a policy-style failure;
  the client's normal token-refresh + retry flow applies to the next message. No token
  refresh mid-run in MVP.
- Concurrency guard: at most 2 in-flight agent runs per user (Redis counter, fail
  closed) — the MCP/AI path has no rate limiter today; this is the minimum protection
  for a loop that can fan out LLM calls.
- SSE through the gateway: `X-Accel-Buffering: no` + Helm HTTPRoute timeout 1800s;
  heartbeat comment frames every 15s to keep intermediaries from idling out.

## Testing

- `internal/toolcontract`: table-driven unit tests per tool (dispatch path, credential
  forwarding, channel header, error pass-through); MCP server tests stay green.
- Web tool adapters + policy caps: focused tests incl. per-tool budget exhaustion.
- Handler: httptest SSE tests (event order, clarification pause/resume, cancel,
  spend-cap rejection, concurrency guard).
- Frontend: vitest for the SSE parser and the toggle/send path; component tests for the
  live trace and clarification card.
- Parity check: scripted comparison (same question → MCP client vs web agent) asserting
  same datasource/model selection and equivalent LogicalQuery — the ticket's
  Phase 1 success criterion.
- Full gates: `make precommit`, `make eval-regression` (AI-eval-adjacent code),
  `gograph_review --uncommitted`, Helm lint/template + route assertions.

## Out of scope (later phases)

- NATS-async web runs (`routeAIJob` wiring, step-envelope publishing, SSE bridge).
- Admin dashboard for agent runs; per-workspace mode matrix beyond the enable flag +
  allowlist.
- Skill-first learning loop / embedding-based confirmed-query learning (pairs with the
  knowledge-base work in `tasks/wren-agentic-workflow-findings.md` P0-A).
- Server-side conversation message writes (client snapshot flow remains the writer).
- Mid-run token refresh; multi-datasource cross-joins.
