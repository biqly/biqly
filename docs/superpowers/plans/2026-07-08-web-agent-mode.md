# Web Agent Mode — Execution Plan

Date: 2026-07-08
Design: `docs/superpowers/specs/2026-07-08-web-agent-mode-design.md`
Tracking: `tasks/todo.md → Web Agent Mode (2026-07-08)`

Every task ends with its focused tests green plus `gofmt` + `make lint-go` (Go) or
`make check-frontend` (frontend) on the touched stack. Commits land on `dev`.

## Progress

- [x] T1 — `internal/toolcontract` package
- [x] T2 — MCP server consumes toolcontract
- [x] T3 — `PurposeAgent`
- [x] T4 — Web tool registry + policy
- [x] T5 — Planner prompt + web run bounds
- [x] T6 — `POST /api/agent/chat` SSE handler
- [x] T7 — Finalizer
- [x] T8 — Clarification round-trip
- [x] T9-T12 — Frontend Agent Mode
- [x] T13 — Parity harness
- [~] T14 — Docs + full local gate complete; dev-cluster deploy + SSE gateway smoke
      test deferred (see note below T14)
- [ ] T15 — Rollout

---

## Phase 0 — Shared tool contract (foundation)

### T1 — `internal/toolcontract` package
- Extract the six tool definitions (name, description, input struct with `jsonschema`
  tags, method+path) from `internal/http/mcp_server.go` into `internal/toolcontract`.
- `Dispatcher` with a loopback-HTTP implementation: forwards `Authorization` /
  `X-API-Key`, sets `X-Biqly-Channel` (parameterized: `mcp` | `agent`), returns raw
  JSON or a typed HTTP error.
- Table-driven unit tests: dispatch paths, credential forwarding, channel header,
  non-2xx pass-through.
- **Done when:** package tests green; no behavior change anywhere yet.

### T2 — MCP server consumes toolcontract
- Re-register the six MCP tools from `internal/toolcontract`; delete the inline
  bodies. Channel stays `mcp`; output stays raw-JSON `TextContent`.
- **Done when:** `internal/http/mcp_server_test.go` green with no test edits beyond
  imports; `gograph_review --uncommitted` shows no orphaned symbols.

## Phase 1 — Web agent runtime (backend)

### T3 — `PurposeAgent`
- Add `agent` to the purpose enum (`internal/ai/provider_store.go`: const,
  `AllPurposes`, `ValidPurpose`; decide user-selectable set), fall back to
  `PurposeQuery` when no agent model is configured.
- Admin UI: purpose dropdown in `ModelModal.tsx`/`ProviderModal.tsx`.
- **Done when:** an admin can assign a model to purpose `agent`; existing deployments
  keep working with zero config (fallback proven by a test).

### T4 — Web tool registry + policy
- Six `agent.Tool` adapters over `toolcontract.Dispatcher` (channel `agent`,
  caller credential from the request context).
- Policy: web allowlist + per-tool per-run call caps (`run_question` ≤3,
  `run_logical_query` ≤2, list tools ≤2, `run_skill` ≤2) via the existing
  `RetryBudget`-style mechanism in `PolicyEngine`; role-based allowlist
  (viewer vs analyst) from auth context.
- Tool-output truncation for planner visibility (≤100 rows / bounded runes).
- **Done when:** focused tests cover every cap and the truncation; a denied tool call
  surfaces the policy reason in the step history.

### T5 — Planner prompt + web run bounds
- New system prompt for the 6-tool set (no raw SQL, skill-first, clarify-discipline,
  prior-turn inheritance). Extend `ProviderPlanner`/prompt builder to render the web
  tool descriptors and prior turns.
- `BI_WEB_AGENT_*` config: enabled flag (default false), workspace allowlist reuse,
  timeout 120s, max steps 6, max clarification rounds 2; config validation + docs
  (`docs/configuration.md`).
- **Done when:** planner unit tests (stub provider) drive a full happy path:
  list_models → run_question → final; config validation tested.

### T6 — `POST /api/agent/chat` SSE handler
- New handler in the AI service; route registered behind the standard `/api` auth
  middleware chain; feature-flag + allowlist gate; per-user concurrency guard
  (Redis, max 2, fail closed).
- Creates the `agent_runs` row (web channel, no `job_id`), builds `RunContext` from
  auth context, runs `Runtime.Run` in-request with an event sink bridged to SSE
  (`newEvalSSESender` pattern + 15s heartbeat), persists steps as today.
- Spend cap: `SpendLimiter.Check`/`Record` around every planner/finalizer LLM call.
- Client abort → context cancel → run abandoned cleanly (existing runtime semantics).
- Helm: `/api/agent` HTTPRoute with 1800s request timeout (values + assertion script).
- **Done when:** httptest SSE tests cover event order, cancel, spend-cap rejection,
  concurrency guard, flag-off 404/403; `helm template` shows the route + timeout.

Progress:
- Implemented `POST /api/agent/chat` registration, feature flag + workspace allowlist,
  SSE error/result framing, `agent_runs` creation, governed web-tool dispatch with caller
  credentials, in-request runtime execution, metadata-backed state persistence, planner LLM
  spend limiting, and Redis-backed fail-closed per-user concurrency (max 2).
- Added `/api/agent` to AI route values with `1800s` timeout and rebuilt Helm
  dependencies.
- T6 complete: live per-step SSE event sink + heartbeat, explicit cancel test,
  spend-cap rejection test, role-based web-tool narrowing wired into the handler's
  tool set construction, and a `make helm-assert-web-agent-route` assertion — all
  landed and reviewed clean (commit `49d02da1`).

### T7 — Finalizer
- Compose the `result` payload: `SynthesizeAnswer` summary, chart suggestion
  heuristic, follow-up chips, LogicalQuery (+ SQL preview per existing gating),
  ordered step trace (`run_steps`/`run_id` shape the frontend already knows).
- **Done when:** result payload golden test; a run's final SSE frame renders in the
  existing `AssistantMessageCard` shape without frontend changes (verified in T11).

T7 complete (commit `cd98ea45`, reviewed clean). Follow-up fix (`cbe004ae`,
`f1de440d`): the design doc commits to `agent_steps` persistence ("Full fidelity
persists in `agent_steps` as today") but the web agent path only wrote a
`runtime_state` metadata blob — never called `ReplaceAgentSteps`. Fixed for both
the success (`Terminal.Final`) and failure (`Terminal.Failure`) terminal paths, so
`GetAgentRun`/`RunTracePanel` reload-hydration works for web agent runs exactly
like the legacy pipeline.

### T8 — Clarification round-trip
- `clarification_required` SSE event with structured choices + free text; stream ends.
- Resume: `resume_run_id` + `clarification_answer` loads persisted `RuntimeState`,
  identity-checked (tenant/user), appends answer to history, continues the loop on a
  new stream.
- **Done when:** pause/resume integration test (stub planner emitting a clarification,
  then a final) passes; resuming as a different user is rejected.

T8 complete (commits `cf9277d4`, `6d08c21f`). `agent.RuntimeState` gained
`PendingClarification`/`ClarificationHistory` fields (the runtime previously
discarded the planner's clarification question entirely); resume loads the
existing run via the previously-dead-code `webAgentStateStore.Load` instead of
creating a new run, identity-checks it (owner + datasource match, generic
not-found error on mismatch), and threads the original question plus the full
accumulated clarification history (not just the latest round) into the resumed
planner prompt — verified against a real 2-round accumulate-and-resume flow.

## Phase 2 — Frontend Agent Mode

### T9 — SSE client + types
- `frontend/src/api/agentStream.ts`: fetch + `ReadableStream.getReader()` SSE parser
  (handles multi-line frames, `[DONE]`, abort); `frontend/src/types/agent.ts`.
- **Done when:** vitest parser tests (chunk splits, heartbeats, error frames) green.

T9 complete (commit `0dc41ce0`, reviewed clean). Types were derived from the actual
backend handler shapes (not just the design doc's illustrative JSON) via gograph, and
`AssistantMessageCard`-reused types (`LogicalQuery`, `QueryResultPayload`,
`VisualizationHint`, `SuggestedFollowUp`) so the result-payload type needs no
duplication.

### T10 — Agent Mode toggle + send path
- Toggle in `ChatPanel.tsx` beside context/auto-find (localStorage, threaded from
  `AIQuery.tsx`); when on, `sendQuery` calls the agent stream instead of jobs/polling;
  prior turns + datasource preselection included.
- **Done when:** toggle persists; agent-off behavior byte-identical to today.

T10 complete (commits `aeb0cb7b`, `ee9abf0b`, `d15b5125`). Two review rounds found real
races: the SSE stream had no `AbortController`/unmount cleanup (fixed), and the fix's
own "abort the previous turn on a new send" safety net had an unguarded
`setQueryAction(null)` that let a just-superseded turn's settlement clobber the new
turn's loading state, re-enabling the composer mid-stream (fixed, with a genuine
double-send regression test).

### T11 — Streaming UX
- Live trace: `step` events feed `RunTracePanel` incrementally; clarification card
  (numbered choices + free text + skip → resume POST); `result` renders through
  existing sections (table, chart, caption, follow-ups, SQL preview, trace);
  cancel button aborts the fetch; error frames use the existing error section.
- Conversation persistence unchanged: the client snapshot flow stores the assistant
  message from the `result` payload (with `run_id`).
- **Done when:** component tests for trace/clarification/result; manual dev-loop pass
  (`make watch` + `make dev-frontend`) of the full flow incl. cancel + clarification.

T11 complete (commits `292d8c62`, `380d57c4`). Reused `ClarificationCard`/`RunTracePanel`
unmodified; found and fixed a real pre-existing bug in `normalizeAIQueryResponse` that
silently dropped `run_id` on a second normalization pass (broke trace reload-hydration
for every agent-mode result, and for some legacy-pipeline messages too). One fix round:
switching conversations while one had a pending clarification/in-flight stream clobbered
it via unguarded global state and a single global abort ref — now scoped per-conversation
(`agentTurnsByConversation`, per-conversation `AbortController` map). No skip sentinel
exists server-side for the web agent path yet, so Skip sends a natural-language
instruction as a stopgap (disclosed, not blocking). Manual dev-loop/browser QA was
explicitly not performed by the implementer (honest per-task disclosure) — recommend a
manual pass before shipping the toggle broadly.
Known follow-up (not fixed, filed): `queryAction` busy-flag is still global, not
per-conversation — while one conversation streams, an unrelated idle conversation's
composer is disabled and shows a dead Cancel button. Pre-existing shape, exposed (not
introduced) by this task's per-conversation scoping.

### T12 — i18n, a11y, frontend gate
- All strings via `useT()` (ai_query.agent_*); keyboard/ARIA on the clarification
  card and toggle; `make check-frontend` (tsc, eslint, knip, vitest, build) green.

T12 complete (commit `b102bd37`). Audit came back genuinely clean (i18n, toggle a11y,
clarification-card a11y on the live path all already correct from T9-T11) except one
real gap: the live trace card had no `aria-live` announcement for screen readers — fixed
with `role="status"` (matches the existing `TypingIndicator` convention). Full frontend
gate green.

**Phase 2 (T9-T12) is fully complete.**

## Phase 3 — Hardening, parity, rollout

### T13 — Parity harness
- Script (dev tooling, not CI-gating): run a fixed question set through the MCP
  contract and through `/api/agent/chat`; assert same datasource/model selection and
  equivalent LogicalQuery; report drift. Add agent-path cases to `make
  eval-regression` only if the stub-provider harness supports the planner loop —
  otherwise document as the known eval gap (matches the Task-16 note in todo.md).
- **Done when:** parity report for the golden question set shows no governed-path or
  model-selection divergence.

T13 complete (commits `081d0ffa`, `cb747e57`). `internal/agent/parity` holds a
transport-agnostic 8-case bilingual question set, a shared deterministic fake
backend, and comparison logic (independently unit-tested); `internal/http/
agent_mcp_parity_test.go` wires it to the real, unmodified production
`newMCPServer` and `agent.Runtime` — living in package `http` only because
`newMCPServer` is unexported, so no production code changed to make the harness
possible. Result: zero governed-path or datasource-selection divergence across all
8 cases. Disclosed limitation: the fixture's `handleListModels` returns exactly one
model per datasource, so the 0-divergence result for *model* selection specifically
is a structural fact of the fixture, not empirical proof the two paths agree when a
real choice between models exists — the divergence-comparison logic itself is real
and separately unit-tested with synthetic multi-model data. Not wired into `make
eval-regression` (dev tooling only, per plan; the stub-provider eval harness drives
the legacy single-shot pipeline, not the agent planner loop — same gap already
documented for the NATS agent runner).

### T14 — Docs + full gate + dev deploy
- `docs/configuration.md` (BI_WEB_AGENT_*), `CONTEXT.md`/agents docs touchpoints;
  update `tasks/todo.md` statuses.
- Full local gate: `make precommit`, `make eval-regression`, `deadcode -test`,
  `gograph_review --uncommitted`, `make verify-main`, helm lint/template +
  route assertions.
- Deploy to dev (`helm dependency build` first — stale-tgz gotcha), verify SSE
  through the gateway (no buffering, heartbeats, 401 propagation).
- **Done when:** all gates green; dev-cluster smoke of the full flow passes.

Docs + full local gate portion complete (commit `0cee180d`, reviewed clean).
`docs/configuration.md`'s Web Agent Mode row corrected (two "Primary Load / Override
Point" cells pointed at a nonexistent `internal/agent.Runtime`; fixed to the real
`internal/http/handlers/ai_agent_chat.go` call sites). `CONTEXT.md` got a Web agent
mode / Tool contract touchpoint (Product boundary, glossary, Key flows, Where-to-
look). Full local gate green except `make verify-main`'s `govulncheck` sub-step,
which fails only on two Go **stdlib** CVEs fixed in go1.26.5 (repo pinned to
go1.26.4 in `go.mod`) — pre-existing and unrelated to Web Agent Mode, independently
reproduced by the reviewer; every other sub-step (vet, lint-go, test-go,
coverage-gate, eval-regression, check-frontend, helm-lint/template, semgrep-scan 0
findings) passed.

**Dev-cluster deploy + gateway SSE smoke test: deferred, not done.** Before touching
anything, checked the actual cluster state: the `prag` cluster has exactly one
namespace/Helm release (`biqly`, revision 82), currently running with
`values-prod.yaml`'s pinned image tags. `values-dev.yaml` has no tag overrides at
all — it inherits the base chart's `tag: latest` for every service. Running `helm
upgrade -f values-dev.yaml` against this cluster would revert the live release's
images to `latest` in place, on the only environment that exists — there is no
isolated dev sandbox to deploy into. Raised this with the user; they chose to skip
the cluster deploy rather than accept that tradeoff right now. Attempted a local
smoke test instead (`make watch SVC="api auth mail"` + `make dev-frontend`, with
`BI_WEB_AGENT_ENABLED=true` set locally in the gitignored `.env.dev`) — got the
stack running, but had no local dev login credentials to actually drive a request
through the browser, and no PAT/bearer token to drive it via curl either; the user
chose to skip that too rather than provide/mint one. Local dev processes were
stopped and the temporary `.env.dev` line was reverted — no lasting changes from
this attempt. **This means the full live-request flow (real SSE through the
gateway, or even through a local server) has not been exercised end-to-end by a
human or an agent in this session** — only in-process Go tests (httptest,
`internal/agent/parity`) and frontend component tests have verified the wire
protocol and UI behavior. Recommend a manual pass (with real credentials) before
any rollout decision.

### T15 — Rollout
- Ship dark: `BI_WEB_AGENT_ENABLED=false` in prod values.
- Stage 1: enable for the internal workspace (allowlist); watch spend metrics,
  error rates, run durations.
- Stage 2: beta workspaces; Stage 3: default-on with the toggle as opt-out.
- **Requires explicit go-ahead before any prod `helm upgrade`** (values-prod +
  `--force-conflicts` HTTPRoute caveat per `tasks/lessons.md` conventions).

## Later phases (tracked, not in this plan)

- NATS-async web runs (wire `routeAIJob`, publish step envelopes, SSE bridge) for
  horizontal scale-out and reconnectable streams.
- Admin controls dashboard (per-workspace mode, max steps/cost overrides, run audit UI).
- Skill-first learning loop + knowledge base (wren P0-A/P1-C) and embedding-based
  confirmed-query learning.
