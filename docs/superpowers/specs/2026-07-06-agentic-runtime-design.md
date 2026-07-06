# Agentic Runtime — Design

Date: 2026-07-06
Status: Approved (design). Implement incrementally; each sub-project its own spec→plan→build.
Prod baseline: rev 59.

## Goal

Evolve biqly's text-to-SQL pipeline into an observable, governed **agentic runtime** (per the wren.ai "Agentic GenBI" reference), reusing what already exists rather than rebuilding.

## Locked decision

**Evolve the existing pipeline behind a thin Tool seam** (NOT a from-scratch agent runtime). Wrap `Service.ProcessQuestion` in an `AgentService` that persists the run + steps; reuse the existing generate/validate/compile/execute unchanged. Keep the door open for future non-SQL tools (chart/PDF/dashboard) via the orchestration seam, but do not build a tool VM now.

## What biqly ALREADY has (reuse, do not rebuild)

- Text-to-SQL pipeline: `internal/ai` `Service.ProcessQuestion` (+ retry/repair, multi-candidate).
- Step trace: `internal/ai/run_trace.go` `RunRecorder`/`RunStep` (in-memory) + `frontend/src/components/aiQuery/RunTrace.tsx`.
- Clarification + ambiguity analyzer: `internal/ai/ambiguity`, `internal/ai/schema.go` clarification types, `ClarificationCard`.
- Knowledge / instructions: `ai_instructions` + Knowledge Center (SP1); glossary; few-shot.
- Skills / saved queries: `ai_saved_queries` (SP1-SP3: AI authoring + "/" selection + auto-find).
- Memory: `memory_entries` (+ prompt injection).
- Evaluation: `internal/ai/eval` + `make eval-regression` (golden cases).
- NL answer synthesis, empty-response retry, localized month/quarter labels (recent).

So doc phases 3 (knowledge), 5 (memory), 6 (skills), 7 (eval) are largely DONE. The NEW work is phases 1, 2, 4.

## Sub-projects (build order)

### A1 — Persistent Agent Run + Step Trace (foundation; DO FIRST)
Persist each run and its steps so a thread's reasoning is durable, reviewable, and debuggable.
- **Migration (additive)**:
  - `agent_runs`: id, conversation_id (FK `ai_conversations`, nullable for ad-hoc), datasource_id, model_id (nullable), user_id, question, mode TEXT default 'interactive', status TEXT (`running`|`waiting_clarification`|`completed`|`failed`), confidence, answer TEXT, created_at, updated_at.
  - `agent_steps`: id, run_id (FK `agent_runs` ON DELETE CASCADE), seq INT, kind TEXT, status TEXT (`ok`|`failed`|`skipped`), attempt INT, duration_ms INT, detail TEXT, created_at.
  - **No `agent_tool_calls` table (YAGNI)** — tool invocations are steps for now; add a table only when non-SQL tools land.
- **Backend**: a thin `AgentService` (new file in `internal/ai` or `internal/agent`) wrapping `Service.ProcessQuestion`: create a run (`running`) → run the existing pipeline → collect steps from the existing `RunRecorder` → if clarification needed, set `waiting_clarification` (persist; resume on the clarification answer, same run) → on finish persist status + answer + confidence + steps. Store via a new `internal/metadata` store (`agent_runs.go`). The existing generate/validate/compile/execute are UNCHANGED; the Tool seam is only the orchestration wrapper.
  - Endpoints: `GET /api/ai/runs/{id}` (run + steps), `GET /api/ai/runs?conversation_id=…` (list). Auth + per-datasource access like other AI routes.
- **Run scope**: one run per user question, spanning its clarification rounds (status `waiting_clarification` between rounds; resumed on the answer). Keyed by conversation + question.
- **Frontend**: feed the existing `RunTrace.tsx` timeline from the persisted run (thread-scoped, survives reload) instead of only the in-request `run_steps`. No new component; make the trace DB-sourced.
- **Success**: asking a question writes a run + steps; the thread timeline shows them after reload; `GET /runs/{id}` returns them; existing query behavior unchanged; `make lint-go`/`test-go`/`check-frontend` green.

### A2 — Clarification Policy
An explicit policy governing WHEN to clarify vs. proceed with a default+caveat, evolving the current ambiguity/clarification path.
- Rules — clarify ONLY when: (1) metric not found; (2) ≥2 metrics tie at the same confidence; (3) segment definition ambiguous; (4) the answer would change dramatically by interpretation; (5) no safe default exists. Otherwise: use the default (e.g. default metric formula), proceed, and surface a one-line `caveat` in the answer + offer "save a different definition".
- **Skip is first-class**: skipping a clarification proceeds with the default (never dead-ends).
- Wire into `internal/ai/service.go` clarification gate; add a `caveat` field to the response/answer surfaced in the UI.

### A3 — Metric Definition Layer (RateBehavior)
Give metrics explicit semantics so rate/ratio metrics don't trigger clarification.
- Extend the semantic `Metric` with `Aggregation`, `Formula`, `Grain`, and `RateBehavior` (`ratio_of_sums` | `average_of_customer_rates` | `weighted_average` | `latest_value`).
- The generator/compiler + prompt use `RateBehavior` to pick the correct formula deterministically (e.g. rate = SUM(x)/SUM(y) for `ratio_of_sums`), removing the "which rate formula?" clarification the reference flags.
- Migration to add the columns; UI in the metric editor to set them; prompt/compiler consume them.
- Pairs with A2: when a metric lacks an explicit RateBehavior, A2's policy applies the safe default + caveat.

### Existing-phase polish (as needed, small)
Knowledge/Memory/Skills/Eval already exist; touch only if the agentic thread surfaces a concrete gap.

## Cross-cutting
- Additive, reversible migrations; no destructive drops (old `ai_confirmed_queries`/`ai_skills` still pending SP5 after prod verification — unrelated).
- Every step: `make lint-go` + `make test-go` (race) + `make check-frontend` green; `gograph_review` after Go edits; `TestConfigDocSync` for new env vars; `make eval-regression` when touching eval/generation prompts (A3).
- i18n (EN+TR) for new UI.

## Out of scope
- From-scratch tool VM / sandboxed non-SQL tool execution (chart/PDF as tools) — the seam allows it later; not built now.
- Separate `agent_tool_calls` / `agent_eval_*` tables — reuse steps + existing eval suite.
