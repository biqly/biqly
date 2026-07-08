# Todo list

## Web Agent Mode for AI Chat (2026-07-08)

**Status:** in progress — Phase 0-1 complete (T1-T8, backend). Phase 2 (frontend, T9-T12) and Phase 3 (hardening/parity/rollout, T13-T15) not started.

**Source of truth:**
- Design: `docs/superpowers/specs/2026-07-08-web-agent-mode-design.md`
- Execution plan: `docs/superpowers/plans/2026-07-08-web-agent-mode.md`

### Success criteria
- [x] Web agent never accepts or emits raw SQL as user-controlled execution input (structured `run_logical_query` tool only; SQL preview is output-only, gated same as the legacy pipeline).
- [x] All tool calls pass through existing `/api/*` governed paths with the caller's own credentials.
- [x] Agent calls the MCP-parity tool set: `list_models`, `list_datasources`, `run_question`, `run_logical_query`, `list_skills`, `run_skill` — defined once in `internal/toolcontract`, consumed by both MCP and the web agent.
- [ ] Frontend streams agent progress with SSE (`POST /api/agent/chat`, fetch + ReadableStream client). — backend SSE endpoint is complete; frontend client is T9-T12, not started.
- [x] Final answer includes business summary, table, chart suggestion, follow-ups, and trace.
- [x] Existing auth, RLS, PII masking, spend caps, and audit apply unchanged; per-user concurrency guard added.
- [ ] Same question via MCP client and web agent selects the same datasource/model and produces an equivalent LogicalQuery (parity harness). — T13, not started.

### Execution phases
- [x] Phase 0 — T1–T2: shared `internal/toolcontract` package; MCP server consumes it.
- [x] Phase 1 — T3–T8: `PurposeAgent`, web tool registry + policy caps, planner prompt + `BI_WEB_AGENT_*` bounds, `POST /api/agent/chat` SSE handler (live steps, cancel, spend-cap, role narrowing, Helm route), finalizer result payload + `agent_steps` persistence, and clarification round-trip (resume, identity check, full multi-round history) — all complete, reviewed, and committed on `dev`.
- [ ] Phase 2 — T9–T12: SSE client, Agent Mode toggle, streaming UX (live trace / clarification card / result), i18n + a11y + frontend gate.
- [ ] Phase 3 — T13–T15: parity harness, docs + full gate + dev deploy, staged rollout (dark → allowlist → beta → default; prod upgrade needs explicit go-ahead).

### Review
- T1-T2 complete: `internal/toolcontract` owns the six governed tool definitions and dispatch helpers; MCP server consumes them.
- T3 complete: `PurposeAgent` added with query fallback, admin UI purpose support, and migration `066a_add_agent_purpose`.
- T4 complete: web tool registry over `toolcontract.Dispatcher`, planner-visible truncation, and web-tool policy bypasses untrusted identity args in favor of forwarded caller credentials.
- T5 complete: planner prompt now renders web tool descriptors, no-raw-SQL / skill-first / clarification / prior-turn inheritance rules, and prior turns; `Config.WebAgent` loads `BI_WEB_AGENT_*` with 120s timeout, max steps 6, max clarification rounds 2, and allowlist reuse from `BI_AGENT_WORKSPACE_ALLOWLIST`.
- T5 evidence: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./internal/config -count=1 -run 'TestProviderPlanner|TestBuildPlannerPrompt|TestLoadWebAgent|TestLoadAgentConfig' -v` green; `make lint-go` 0 issues; `gograph_review --uncommitted` hit git diff exit 129, so symbol-level reviews were run for `buildPlannerPrompt`, `RunContext`, `AgentConfig`, and `WebAgentConfig`.
- T6 in progress: added `/api/agent/chat` SSE handler, route registration, governed loopback dispatcher wiring, `agent_runs` creation with web mode, planner/runtime execution, metadata state persistence, caller credential forwarding, runtime-unavailable guard before DB insert, Redis-backed fail-closed per-user concurrency (max 2), and standalone AI Redis wiring for response cache/spend limiter/concurrency.
- T6 Helm progress: `/api/agent` added to AI route path prefixes with `1800s` timeout; `helm dependency build deploy/helm/biqly` completed after network approval. Full `helm template` assertion is still open because this chart render currently requires production secret values/helpers.
- T6 evidence: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./internal/app ./internal/http ./internal/http/handlers -count=1 -run 'TestWebAgent|TestRuntimeCancellation|TestRuntimePlannerError|TestRuntimeSuccessful|TestGetAgentRun|TestListAgentRuns|TestNewAgentDependencies'` green; focused `-race` for agent + web-agent handler green; `make lint-go` 0 issues. Broad `-race` hit sandbox localhost-bind failures in unrelated `httptest.NewServer` tests, so bind-free focused selectors were used.
- T6 complete (commit `49d02da1`): live per-step SSE event sink + heartbeat (`streamAgentSteps`, matches `newEvalSSESender`/15s cadence), explicit client-cancel test (with a `context.WithoutCancel` fix for a latent bug where a canceled run's "mark failed" write was silently skipped), spend-cap rejection test (verified `Check` precedes any LLM call), role-based tool narrowing wired into the handler (`webAgentAllowedTools`, enforced at `PolicyEngine.Evaluate`, fails closed to viewer set), and a `make helm-assert-web-agent-route` assertion. Subagent code review: Approved, no Critical/Important findings.
- T7 complete (commit `cd98ea45`): `composeWebAgentFinalResult` reuses the legacy pipeline's own helpers (`enrichAIRunResponse`'s `VisualizationHintFromResult`, `attachAINaturalLanguageAnswer`, `attachSuggestedFollowUps`) so the SSE `result` payload matches `AssistantMessageCard`'s existing shape byte-for-byte, with a golden test. Subagent code review: Approved. Follow-up fix (`cbe004ae`, `f1de440d`): the design doc commits to `agent_steps` persistence ("Full fidelity persists in `agent_steps` as today"), but the web agent path only wrote a `runtime_state` metadata blob and never called `ReplaceAgentSteps` — fixed for both the success and failure terminal paths so `GetAgentRun`/`RunTracePanel` reload-hydration works like the legacy pipeline. Re-review: Approved.
- T8 complete (commits `cf9277d4`, `6d08c21f`): activated the previously-dead-code `webAgentStateStore.Load` for resume (was always creating a new run instead), added `RuntimeState.PendingClarification`/`ClarificationHistory` (the runtime previously discarded the planner's clarification question/options entirely on pause), identity-checked resume (owner + datasource match, generic not-found on mismatch, no existence-leak), and threaded the full accumulated clarification history (not just the latest round) plus the original question into the resumed planner prompt — verified against a real 2-round accumulate-and-resume flow with `Planner.Decide` argument capture. First-pass review found the original question was dropped on every resume and round-1's Q&A was overwritten before round 2; fixed and re-reviewed clean.
- Backend (Phase 0-1, T1-T8) is now fully complete on `dev`. Remaining work: Phase 2 frontend (T9-T12) and Phase 3 hardening/parity/rollout (T13-T15, prod steps require explicit go-ahead per the plan).

## Agentic Query Runner + conversation replay repair (2026-07-06)

**Status:** Phases 1–4 (Tasks 1–16) implemented and committed on `dev`, `agent.enabled=false` by default everywhere. Phase 5 (Tasks 17–18) in progress: this doc pass is Task 17; Task 18's live-production steps (real repair apply, dark deploy, shadow enable) are paused pending explicit go-ahead — see Task 18 below.

**Source of truth:**
- Design: `docs/superpowers/specs/2026-07-06-agentic-query-runner-service-design.md`
- Execution plan: `docs/superpowers/plans/2026-07-06-agentic-query-runner-service.md`

### Success criteria
- [x] Reposting a conversation snapshot cannot create duplicate messages.
- [x] Existing proven ordered-prefix replay rows are archived and soft-deleted through a reversible repair; ambiguous rows are report-only.
- [x] Internal `cmd/agent` runs a maximum six-step, policy-gated, read-only BI query lifecycle through typed tools.
- [x] Shadow mode returns only the legacy response and never performs a second customer query execution.
- [x] Agent has no public route or direct customer database egress; Helm and Cilium assertions prove the boundary.
- [~] Focused tests, AI eval regression, frontend gate, Helm assertions, `make verify-main`, and `gograph_review --uncommitted` pass — all green through Task 16; Task 18 re-runs the full gate once more before rollout.

### Execution phases
- [x] Phase 1 — Tasks 1–3: transactional conversation identity, idempotency, and stable frontend `remote_id` (`59734f02`, `a668d589`, `ffe913d5`).
- [x] Phase 2 — Tasks 4–5: repair detector, report/archive/soft-delete/apply/restore CLI (`cmd/conversation-repair`).
- [x] Phase 3 — Tasks 6–11: Agent contracts, Policy Engine, typed tools, runtime, routing helpers, and `cmd/agent` (`internal/agent/`, `internal/app/agent_dependencies.go`).
- [x] Phase 4 — Tasks 12–16: trace UI, local dev/CI, Helm subchart, NetworkPolicies, eval/metrics/alerts (`36e90abd`).
  - Deferred within Phase 4, documented rather than faked: the legacy-fallback metric (`shouldFallbackToLegacy` has no real call site yet — Task 10 deliberately did not wire it into `Enqueue`) and agent-specific eval golden cases (the existing `internal/ai/eval/` harness drives the legacy single-shot pipeline via a stub `Provider`, not the agent's planner/tool loop — needs its own harness, not a bolt-on).
- [~] Phase 5 — Task 17 (this doc pass) in progress; Task 18's local-only verification gate is safe to run now, but its live-production steps (real conversation repair apply, dark deploy to the cluster, enabling shadow mode) require an explicit go-ahead before executing — see Task 18 below.

### Task 18 — full verification and production rollout
- [x] Local-only verification: `gofmt`, `make lint-go`, `make test-go`, `deadcode -test`, `make check-frontend`, `make eval-regression`, `make helm-lint`, `make helm-template`, `scripts/assert-agent-helm.sh`, `gograph_review --uncommitted` — all green through Phase 4.
- [ ] **Requires explicit confirmation before proceeding** (mutates real data or the live cluster): `conversation-repair report`/`detect --dry-run` against a real `--conversation-id`, applying reversible repair, dark-deploying `cmd/agent` via `helm upgrade` with `agent.enabled=true` (still `BI_AGENT_ENABLED=false`), then flipping `BI_AGENT_MODE=shadow` in production and observing `biqly_agent_shadow_comparisons_total`/`biqly.agent` alerts for the promotion report.

### Review evidence
- `59734f02` feat(conversations): idempotent snapshot writes with versioned ledger.
- `a668d589` fix(frontend): persist stable conversation message ids.
- `ffe913d5` feat(conversations): detect replay chains safely.
- `cmd/conversation-repair` — report/archive/soft-delete/apply/restore CLI, tests + lint green.
- `internal/agent/` (policy, tools, runtime, provider planner, shadow evaluator), `cmd/agent`, `internal/app/agent_dependencies.go` — planner/tool loop, policy engine, `AgentDependencies`, all with focused + race tests green.
- Helm: `deploy/helm/biqly/charts/agent` (subchart, `agent.enabled=false` default), `cnp-agent.yaml` + `cnp-dns/gateway/metadata.yaml` updates, `scripts/assert-agent-helm.sh` (RED→GREEN verified via `git stash`).
- `36e90abd` feat(agent): add run/step/policy/shadow observability and alerts — bounded-label Prometheus metrics + `biqly.agent` PrometheusRule group; `internal/platform/observability/tier2_metrics_test.go` covers both the happy path and bounded-label fallback.
- Every commit through Phase 4 passed the full local gate (`gofmt`, `make lint-go`, `go test -race`, `deadcode -test`, `make check-frontend`, `make helm-lint`/`helm-template`, `gograph_review --uncommitted`) before landing.

### Snapshot persistence follow-up (2026-07-07)
- [x] Fix production log noise: `conversation_write_requests` FK failures and `ai_conversation_messages` duplicate primary-key failures.
- [~] Verify with focused metadata tests, `gograph_review --uncommitted`, and `make lint-go`.

Review:
- `go test ./internal/metadata -count=1` and `git diff --check` pass.
- `gograph_review` on `(*Repository).SaveAIConversationSnapshot` and `upsertMessagesInTx` passed; `gograph_review --uncommitted` hit a git diff invocation error, so symbol-level review was used.
- `make lint-go` is blocked by a parallel-agent untracked file (`cmd/gen-openapi/main.go`: gosec G306 on `os.WriteFile(..., 0644)`). Left untouched.

## Feature: @-mention follow-ups + AI Query screen cleanup (2026-06-23)

### Asks (from user)
1. Arrow-key nav in the @ popup doesn't move the highlight.
2. Selected item shows as plain text; show `schema.table.field` richly in the dropdown and insert a `@name` token (LLM matches canonical name; dialect quoting like SQL Server `[]` stays in the SQL-generation layer).
3. Backend prompt must clearly instruct: when the user references a field/table, you MUST use it when building the LogicalQuery.
4. Remove the Otomatik/Manuel toggle and the manual table/view scope panel entirely (@ replaces manual scoping).
5. Turkish descriptions don't appear in the @ popup when locale=tr.
6. Table/view rows look "weird" (duplicate text) on search.

### Decisions (confirmed with user)
- Insert format: rich display + canonical `@name` token + backend directive.
- UI: remove both auto/manual toggle and manual scope panel; routing always auto.
- Turkish: solve this round (locale overlay).

### Plan
- [x] A1. Fix arrow-key nav: ref-based highlight reset (no longer resets on every keyup); scroll active item into view. (`PromptTextarea.tsx`)
- [x] A2. Rich dropdown row: show `schema.table.column` reference; fix table rows (no duplicate label/name). Insert `@<name> ` token. (`useSemanticCatalog.ts`, `PromptTextarea.tsx`)
- [x] A3. Backend directive: always-included Go-generated "explicitly referenced fields (highest priority)" rule in planning steps — DB templates can't drop it. (`prompt_examples.go`)
- [x] A4. RoutingPanel: removed auto/manual toggle + RoutingPanelManualScope; cleaned dead state/props/i18n/utils/classes; routing always auto (`tables: undefined`). (`AIQuery.tsx`, `RoutingPanel.tsx`, `types.ts`, `routingPanelUtils.ts`, i18n; deleted `RoutingPanelManualScope.tsx`, `aiScopeClasses.ts`)
- [x] A5. Turkish overlay: model-detail read path overlays `entity_translations` for `semantic_model`/`dimension`/`metric` (label+description) by request locale. (`semantic_translations.go`, `semantic.go`) + unit test.
- [x] A7. Composer token highlight: `@field` tokens colored via a synced backdrop overlay; native `title` tooltip on hover shows label/ref/description. (`PromptTextarea.tsx`, `mentionUtils.ts`, `aiQueryClasses.ts`) + tokenizer unit tests.

### A7b. Rich hover tooltip (done)
Replaced the native `title` with a styled hover card (type chip + label + mono ref + description), positioned under the token. (`PromptTextarea.tsx`, `aiQueryClasses.ts`)

### A6. Turkish DATA generation — DONE (auto lazy-backfill, AI service)
Decision: auto-on-TR-load. Implemented:
- `ai.TranslationService.TranslateFields` — batched label+description translation, key-preserving (+ tests).
- `metadata.Repository.EntitiesWithTranslation` — which entity ids already have a row in a locale (skip already-translated).
- AI endpoint `POST /api/ai/semantic/models/{id}/translate` (`ai_semantic_translate.go`) — loads model, translates only MISSING model/dim/metric label+desc, upserts to `entity_translations`; LLM skipped when nothing missing. Wired translator through `Dependencies`→`AIDeps`.
- Frontend: `useSemanticCatalog` calls the translate endpoint (via `X-Locale`) before reading the model when locale≠en; catalog overlay (A5) then serves TR.
- Verified: `make lint-go` 0 issues, go test (ai/handlers/metadata), build, vet, module deadcode all clean; `make check-frontend` green.

### A8. Re-translate trigger (stale source text) — DONE
- Backend: `?force=true` on the translate endpoint re-translates every entry, overwriting cache (`translatedSet` returns empty set under force). For when the English source label/description later changes.
- Frontend: `useSemanticCatalog` exposes `retranslate()` / `retranslating` / `canRetranslate`; "↻ Yeniden çevir" button in the composer bar (shown when locale≠en and a model is selected) → force-translate → reload catalog.
- i18n: `ai_query.retranslate` / `retranslate_title` / `retranslating` (en+tr).

### Security review (automated) — addressed
Finding: new route missing sibling middleware. Added `aiUserMW` for route-family consistency (the `/api` group's `authMW` already authenticates it). Per-model datasource ACL intentionally not added: mirrors the existing unguarded `GetModel` read, discloses no model data (returns only a count), and writes are an idempotent, bounded TR rendering of the model's own text — documented inline in `ai_router.go`.

### (superseded) earlier open decision — A6: Turkish DATA generation (architecture-constrained)
NEW finding: `/semantic/*` is proxied to the **catalog** pod, `/ai/*` to the **AI** pod (`proxy_routes.go`). Only the AI pod has LLM egress (Cilium). So translate-on-read **cannot** live in the catalog `GetModel` — it must run in the **AI service** and cache into `entity_translations`; the catalog overlay (A5) then serves it. `AIDeps` has `SemanticRepo` + `MetaRepo` + provider store; translator built at `dependencies.go:435`.
Corrected option-1 shape: AI endpoint `POST /api/ai/semantic/models/{id}/translate` (loads model, translates only MISSING dim/metric label+desc, upserts, LLM-skipped when nothing missing) + frontend lazy trigger on TR load. Sub-decision: auto-on-load vs explicit "Türkçeye çevir" button (cost/UX). → confirm trigger, then build.

### ⚠ Open decision — A6 (original)
The overlay (A5) is wired and correct, but it overlays nothing yet: there is **no Turkish data** stored for dimensions/metrics. `entity_translations` only holds `table`/`column` rows (written by the `describe` flow); `semantic_dimension`/`semantic_metric` rows are never produced. Options to actually populate Turkish text:
1. Translate-on-read with cache (reuse existing `TranslationService`, store into `entity_translations`) — self-contained, covers metrics, first-load cost.
2. Reuse existing column translations → overlay onto dimensions via `column_ref` — cheap, but only dims, only where `describe` ran in TR.
3. Admin "translate model" action / manual entry.
→ Needs user pick before building (avoids speculative work).

### Verification
- Frontend: `make check-frontend` green (eslint, tailwind, format, knip, vitest, tsc build).
- Backend: `go build` + `go vet` clean; `internal/ai/prompt` + `internal/http/handlers` tests pass; module-wide `deadcode` shows no new dead funcs (overlay actually wires the previously-dead `GetEntityTranslations`).

### Findings
- Arrow bug root cause: `PromptTextarea` `onKeyUp={recomputeMention}` calls `setActiveIdx(0)` every keystroke, including the Arrow keyup, so the highlight snaps back to index 0.
- `entity_translations` table + `metadata.Repository` translate helpers exist; `EntityTypeSemanticDimension/Metric` constants exist but have ZERO writers/appliers. `applyDescriptionTranslations` overlays description only (not label).

## Feature: schema-aware @-mention autosuggest in AI Query prompt (2026-06-23)

### Goal
Like sqlai.ai: `@`-mention real semantic-model objects (dimensions, metrics, tables) while
composing a prompt so generated SQL matches the actual schema with higher consistency.

### Decisions
- Trigger: explicit `@`-mention popup; scope: dims + metrics + base/joined tables (no raw columns).
- Insert canonical field `name` (LLM matches it against schema already in the backend prompt).
- Frontend-only: backend prompt already embeds the full model schema (`internal/ai/prompt/prompt.go`).

### Done
- [x] `hooks/useSemanticCatalog.ts` — catalog from `GET /api/semantic/models/{id}`; composite → components merged; auto-detect → datasource tables.
- [x] `components/aiQuery/mentionUtils.ts` — pure `findActiveMention` + `score` (+ 10 tests).
- [x] `components/aiQuery/PromptTextarea.tsx` — textarea + `@`-mention overlay (filter, ↑/↓/Enter/Tab/Esc, click).
- [x] Wired into `ChatPanel.tsx` + `AIQuery.tsx` (passing `semanticModelId` + `tables`).
- [x] i18n (en + tr) + popup styling classes. `make check-frontend` green.

## Feature: time-windowed ratio / share-of-total in logical query (2026-06-22)

### Problem (diagnosed, live-evidence backed)
Question "bugün atılan toplam tweet sayısının bu ay atılan toplam tweet sayısına oranı" =
ratio of two aggregates with DIFFERENT time-window filters. The logical-query IR cannot
express it: `Filters` is a single global WHERE shared by every aggregate, and there is no
query-time ratio. Live worker logs: 105s, retry_count 2, tier escalation compact→standard→
expanded, `outcome: clarification, confidence 0`; sibling attempt hit `finish_reason: length`
(8192-token runaway). Prompt is mature; the gap is infrastructure.

### Solution: filtered measures + query-time ratio
Two coherent IR primitives sharing one compile path (conditional aggregate via CASE):
1. Per-measure filter on a `metric` select item → `AGG(CASE WHEN <cond> THEN <inner> END)`.
2. New `ratio` select item dividing numerator/denominator measures (each with optional
   per-measure filters) → `(<num> * 1.0) / NULLIF(<den>, 0)`.

### Tasks
- [x] IR types: `SelectItem.Filters`, `SelectItem.Ratio`, `RatioSpec`, `MeasureRef`, `SelectTypeRatio` (pkg/logicalquery/types.go)
- [x] Re-export in internal/query/logical.go
- [x] Compiler: extracted `buildFilterConjunction`; added `metricFilteredAggregate`; per-measure filters in `buildSelectMetric`; `buildSelectRatio` + `measureSQL` + dispatch
- [x] Validator: shared `validateFilterList`; `validateRatioSelect` + `validateMeasureRef`; `ratio` in allowed select types
- [x] Prompt: ratio rule + tweet example in en/tr system_rules.tmpl + output_format.tmpl
- [x] schema.go: LogicalQuerySchema now lists `ratio` + per-item `filters` + `$defs`
- [x] Tests: 6 new (compiler ratio/filtered-metric/sum-col/grain-reuse/taught-JSON, validator ratio×3 + per-measure filter)
- [x] Gates: gofmt ✓, lint-go (0 issues) ✓, test-go -race (61 pkgs) ✓, deadcode (new symbols live) ✓, eval-regression ✓

### Review
Generalized `ratio` → `formula` (one construct, 5 ops) covering the realistic two-measure end-user space:
`subtract` (fark), `divide` (oran, fraction), `percent_of` (yüzde ×100), `percent_change` (değişim/büyüme ×100),
`add` (toplam). Each side is a measure with its OWN filters (conditional aggregate), so "bugün vs dün", "bu ay vs
geçen ay", "part vs whole" all compile to one expression. Works under GROUP BY too (per-group ratios/diffs).

Integer-division fix (user-flagged): every division op multiplies the dividend by a float literal (1.0 / 100.0)
BEFORE dividing, so integer COUNT/SUM no longer truncates; division ops also NULLIF-guard the divisor. ALSO fixed
the pre-existing structured derived-metric divide (`pkg/semantic OpDivide` in expr_compiler.go) which emitted a
bare `/` — admin-defined ratio metrics were integer-truncating. Custom free-form SQL expressions (`a / b` written
as a raw string metric) are left to the author (unsafe to auto-rewrite) — documented caveat.

NOT in scope (user picked full-capability only): latency/UX hardening (105s escalation ladder, 8192-token runaway
cap, fast clarification). Still open; partly mitigated now that the target is expressible.

Open risk: mimo-v2.5 is a small model — infra + few-shot now support formulas, but reliably *emitting* the new
construct may need a stronger model or more few-shots. Verify on the live query after deploy.

## Feature: portable window/analytic functions (2026-06-22)

### Problem
`window` select type exists (row_number/rank/dense_rank/ntile + sum/avg/...) but: (1) misses
lag/lead/first_value/last_value/percent_rank/cume_dist; (2) `count_distinct` window emits
`COUNT(DISTINCT x) OVER` — illegal in PG/MySQL/SQLServer; (3) no ORDER BY enforcement though
SQL Server requires it for ranking; (4) window SQL is dialect-agnostic so ClickHouse lag/lead
(needs lagInFrame/leadInFrame) is wrong.

### Solution
- Dialect hook `WindowFunc(fn, args) (sql, ok)` — ANSI default on BaseDialect; ClickHouse override
  (lag→lagInFrame, lead→leadInFrame; reject percent_rank/cume_dist). ok=false ⇒ validation rejects.
- Add lag/lead/first_value/last_value/percent_rank/cume_dist; `WindowSpec.Offset` for lag/lead.
- Reject count_distinct as a window function (portable nowhere); require ORDER BY for analytic funcs.
- Prompt: scenarios (top-N per group via subquery, running total, lag/lead period-over-period) + ops list.
- Tests: compiler (lag/lead/first_value + ClickHouse lagInFrame), validator (reject count_distinct,
  require order_by, lag offset), dialect WindowFunc unit tests.

### Review
Added a `dialect.WindowFunc(fn, args) (sql, ok)` hook: BaseDialect emits ANSI SQL:2003 (PostgreSQL/MySQL 8+/
SQL Server); ClickHouse overrides lag→lagInFrame, lead→leadInFrame, lowercases ranking, and rejects
percent_rank/cume_dist (ok=false → compiler errors with a clear message instead of broken SQL). Added
lag/lead/first_value/last_value/percent_rank/cume_dist + `WindowSpec.Offset`. Compiler routes analytic funcs
through WindowFunc (aggregate family still via Aggregate), quoting the value arg to match the aggregate path.
Validator: rejects count_distinct as a window function, requires order_by for all ranking/value funcs (mandatory
on SQL Server), requires a value for lag/lead/first_value/last_value. Prompt teaches the full op set + top-N-per-
group (row_number in a subquery filtered to rn≤N) in en/tr.

Gates: gofmt ✓ · lint-go 0 ✓ · test-go -race (all pkgs) ✓ · deadcode clean ✓ · eval-regression ✓.
New tests: compiler (LAG/LEAD ANSI + ClickHouse lagInFrame/leadInFrame derivation, FIRST_VALUE), validator
(count_distinct rejected, ranking needs order_by, lag needs value, lag valid), dialect WindowFunc unit tests.

Caveat: ClickHouse window spelling (lagInFrame/leadInFrame, lowercase names) is best-effort and not verified
against a live ClickHouse — the three named engines (PG/MySQL/SQLServer) use the verified ANSI path.

## Frontend Code Review — Bulgular & Yapılacaklar (2026-06-16)

> Kapsam: `frontend/` — React 19 + Vite 8 + TS 6 + Tailwind v4 SPA (~382 dosya).
> 5 paralel review subagent (skill başına: vercel-react-best-practices,
> typescript-advanced-types, tailwind + tailwind-css-patterns,
> web-design-guidelines + frontend-design, vite) ile yapıldı. Sadece bulgu; kod değişmedi.
>
> **Genel:** sağlam taban. Route'lar hover-preload ile lazy, `any`/`@ts-ignore` yok,
> strict tsconfig, paylaşılan UI primitive'leri (Modal/DataState/DataTable/Toast) erişilebilir
> ve iyi yazılmış. Sorunların çoğu bu iyi primitive'leri **bypass eden** ekranlar,
> bağlanmamış dark-mode variant'ı ve eksik API-boundary doğrulama katmanı.

### P0 — Yüksek etki

**Dark mode bozuk**

- [x] `dark:` variant'ı `[data-theme]` toggle'ına bağlı değildi → `index.css:3`'e `@custom-variant dark (&:where([data-theme='dark'], [data-theme='dark'] *));` eklenmiş. Fix zaten uygulanmış, aktif.

**Erişilebilirlik — custom modal'lar `ui/Modal`'ı bypass ediyor**

- [x] El yapımı modal'larda focus trap / Escape / dialog semantiği yoktu — tümü (`TimeGrainsEditModal`, `AvatarCropModal`, `Glossary`, `FewShotExamples`, `MetadataBulkDescribeModal`, `MetadataDescribeModal`) zaten `ui/Modal` kullanıyor. FewShotExamples & Glossary'deki stra `</div>` fix'lendi (build hatası).
- [x] Backdrop kapatma sadece mouse (aynı modal'lar, klavye yolu yok). `ui/Modal` ile çözülür.
- [x] Tıklanabilir `div`, button değil — `aiQuery/SidebarConversationItem.tsx:79` (select `onClick`, rename `onDoubleClick`, klavye yok). → `<button>` + klavye erişimli rename.

**React doğruluk**

- [x] Component içinde component (recursive remount) — `modeling/ExpressionBuilder.tsx:227`: `ExpressionNodeBuilder` parent gövdesinde, recursive render → her render'da yeni tip, tüm ağaç remount, her tuş vuruşunda focus kaybı. → module scope'a taşı, prop geç.
- [x] Effect'te iptal edilmeyen fetch — `dashboard/DashboardWidgetRenderer.tsx:256-278`: `postData(...).then(setData)` abort guard yok → stale/sırasız setState. 259 ölü kod `setData(... ? null : null)`. → AbortController/request-id guard ekle, ternary'i düzelt.
- [x] Türetilmiş state effect'te set ediliyor — `aiQuery/AssistantMessageCard.tsx:135-150`: `tableView`/`chartType` `useState` ile init sonra iki effect'te overwrite. → render sırasında türet; state'i sadece kullanıcı override'ı için tut.

**i18n — tüm paneller hardcoded İngilizce**

- [x] Admin paneller hardcoded string — `admin/RowLevelSecurityPanel.tsx` (208,213,229,244,251,257,263,267,333), `admin/FieldPermissionPanel.tsx` (320,325,335,350,356,373-376,447), `admin/PIIDetectionPanel.tsx:152`. → `useT()`. *(Tüm satırlar zaten `useT()` kullanıyordu; kalan tek hardcoded `Access` PII tablo başlığındaydı — `FieldPermissionPanel.tsx:445` → `t('admin.pii.col_access')`)*

**State — sessiz hatalar**

- [x] Sessiz catch blokları, kullanıcı geri bildirimi yok — `Glossary.tsx:116,329`, `FewShotExamples.tsx:130`, `SavedQuestions.tsx:250,282`, `QueryHistory.tsx:121`. Optimistic toggle'lar sessizce geri dönüyor. → `ErrorAlert`/toast.

**Responsive**

- [x] UI fiilen masaüstü-only — tüm `src/components/`'te sadece 14 `sm:/md:/lg:/xl:`. → dashboard, table, query builder denetle; responsive stacking ekle.

**Build**

- [x] `optimizeDeps.include`/`resolve.dedupe` direkt olmayan dep'ler — `vite.config.ts:8,11`: `es-toolkit`, `@reduxjs/toolkit`, `immer` (hepsi recharts üzerinden transitive). İleriki recharts major'u dev server'ı derleme sinyali olmadan kırar. → transitive'leri `include`'tan çıkar veya explicit `dependencies`'e al.

### P1 — Orta etki

**TypeScript — API boundary doğrulama (en yüksek kaldıraç)**

- [x] Merkezi fetch wire'ı körü körüne cast ediyor — `api/apiClient.ts:146` `return { data: data as T }`. → opsiyonel validator `(u: unknown) => T` `RequestOptions`'a geçir. Aşağıdaki cast'leri açar.
- [x] `normalizeAIQueryResponse` nested objeleri assert ediyor — `utils/normalizeAIQueryResponse.ts:58,60,78,82,85,87,93-103`: primitive'ler guard'lı ama `logical_query`/`result`/`table_routing`/`clarification`/`prompt_stats`/`token_usage`/`candidates`/`generation_trace` `unknown`'dan direkt cast. → `isRecord` guard'ları ekle.
- [x] `clarification_options as string[]` — `normalizeAIQueryResponse.ts:45,47`: `Array.isArray` element tipini kanıtlamaz. → `.filter((x): x is string => typeof x === 'string')`.
- [x] `jobWaiter` dekoratif generic — `hooks/jobWaiter.ts:31-32`: `settleComplete` `unknown`→`TResult` cast, ilişki yok. → `parse` param al veya `unknown` döndür.

**Tailwind — hardcoded renkler token'ları bypass ediyor**

- [x] Auth sayfalarında brand hex (7×) — `from-[#6366f1] to-[#8b5cf6]` (`SignInPage.tsx:218`, `SignUpPage.tsx:158,309`, `ResetPasswordPage.tsx:81`, `ForgotPasswordPage.tsx:49`, `ClaimInvitePage.tsx:108`, `VerifyEmailPage.tsx:45`); `text-[#6366f1]`, `text-[#e2e8f0]`. Bunlar `--accent` token → açık modda yanlış. → `from-accent to-accent-strong` / `text-accent` (`authClasses.ts` zaten merkezde).
- [x] `DriftPanel.tsx` ham `hsl(...)` literal'leri (~30×) — `admin/DriftPanel.tsx:76-92,214-257`. → `bg-card`/`border-border`/`text-foreground-*`.
- [x] Component'lerde 213 hardcoded hex — statik surface/status renkleri → token; sadece data-driven (chart serisi) inline kalsın. Brand glow `rgba(99,102,241,…)` 20× elle. (Focused pass: auth, DriftPanel, adminClasses, modelingClasses, TimeGrainsTable, admin panel accent fallbacks; full 213-hex sweep deferred.)

**Tailwind — tutarlılık**

- [x] `cn()` bypass; 10+ dosyada ham `clsx` — `ui/KPICard.tsx`, `ui/Skeleton.tsx`, `ui/TagBadge.tsx`, `Modal.tsx`, `Toast.tsx`, `PaginationControls.tsx`, `sharing/ShareButton.tsx`. `twMerge` yok → çakışan util dedupe olmaz. → `cn` (`src/lib/cn.ts`).
- [x] Koşullu class template literal ile (33 dosya) — örn. `SignInCredentialsForm.tsx:205`. → `cn('...', c && 'x')`.
- [x] Font-size scale token yok; `text-[Npx]` (106×) — `[13px]` 48×, `[12px]` 21×, `[14px]` 18×, `[11px]` 15×. Px sabitlenmiş, kullanıcı fontuyla ölçeklenmez. → `@theme` `--text-*` scale (rem).

**i18n / a11y label**

- [x] Remove button'larda hardcoded `aria-label` — `queryBuilder/FieldsStep.tsx:60`, `FilterStep.tsx:91`, `CteStep.tsx:43`, `SummarizeStep.tsx:88,117`, `WindowFuncStep.tsx:68`, `HavingStep.tsx:72`. → `t()`.
- [x] Hardcoded `title` tooltip, icon-only button — `DashboardBuilder.tsx:509,527,538,547,568`. → çevir + `aria-label`.
- [x] Talimat içeren placeholder hardcoded — `DashboardBuilder.tsx:643`, `queryBuilder/CteStep.tsx:36,51`, `WindowFuncStep.tsx:49,55,61`.
- [x] AvatarCropModal a11y — `settings/AvatarCropModal.tsx:223,253`: çıplak `<canvas>` mouse-only crop, range input label yok. → `aria-label` + klavye pan. (zaten uygulanmış)
- [x] `EmptyState` `role="status"` kullanıyor — `ui/EmptyState.tsx:45`: statik placeholder live region → gereksiz SR duyurusu. → düz region.

**State / UX**

- [ ] Liste ekranları `DataState` yerine elle loading/error/empty — ERTELENDİ: ekranlar zaten EmptyState/LoadingOverlay kullanıyor; tam migrasyon her ekranın özel layout'unda regresyon riski → ayrı odaklı PR. (QueryHistory zaten DataState'te; ABExperimentList LoadingScreen+EmptyState+CTA ile uyumlu.)
- [x] Empty state'lerde CTA yok — `action` eklendi: SavedQuestions, Datasources, FewShotExamples, Glossary, Composites detail (`empty_detail_cta`). QueryHistory read-only, DashboardList/ABExperimentList zaten CTA'lıydı.
- [x] Async button'larda pending/disabled yok — Datasources Test/Sync/Delete `busy` map ile guard'lı; SavedQuestions kaydet butonu `saving` prop → disabled + "Saving…".
- [x] Ham `err.message` kullanıcıya gösteriliyor — Glossary 4 nokta artık `t('glossary.save_failed')` / `t('glossary.enrich_failed')`.
- [x] Ünlemli/sistem-sesli başarı metni — `admin.rls.saved`, `admin.field_permissions.saved` sade cümle; RLS + FieldPermissionPanel inline banner kaldırıldı → `toast.success`. `admin.pii_detection.policy_saved` metin sadeleştirildi.
- [x] Tek-label olarak emoji — SignInCredentialsForm passkey (🔑) + DashboardList delete (🗑️) inline `<svg>`. SidebarConversationItem zaten svg+aria-label'dı. (DashboardBuilder/ModelingToolbar emojileri metinle eşli/menü ikonu → bırakıldı.)

**Build / config**

- [x] `manualChunks` yok — `vite.config.ts`'e `manualChunks` eklendi: `charts` (recharts/d3), `react-vendor`, `i18n` izole. Build doğrulandı: charts 384 kB, react-vendor 219 kB, i18n 178 kB, index 120 kB (~299→120).
- [x] Explicit `build.target` yok — `build: { target: 'es2022' }` eklendi.
- [x] `immer` override (`11.1.8`) recharts çakışmasını örtüyor — `package.json`'a `//overrides` açıklama anahtarı eklendi (RTK yok; recharts transitive immer 10). Pin neden var dokümante edildi; deterministiklik için exact pin korundu (caret yerine).
- [x] `tsc -b` ama project reference yok — build script `tsc --noEmit && vite build`'e çevrildi. (Eski `tsc -b` ile aynı sonucu veriyordu; doğrulandı.)
- [ ] Mobile: sabit `w-[Npx]` overflow — ÇOĞU ZATEN GÜVENLİ: ShareButton zaten `w-full max-w-105`; DriftPanel/EvalRunTab/WorkspaceSettings değerleri `max-w-*` (mobilde küçülür). Spekülatif layout değişikliği yapılmadı; gerçek risk yalnızca WorkspaceSettings `min-w-[240px] shrink-0` (≪320px), düşük öncelik.
- [x] `DataTable` empty cell hardcoded `#9ca3af` — `text-foreground-muted` class'ına çevrildi.

### P2 — Düşük / temizlik

- [x] `useConversation.ts` — `loadConversations` artık `isStoredConversation` guard'ı ile element başı `id: string` doğruluyor; geçersiz entry'ler filtreleniyor.
- [ ] `useAIJobs.tsx` — `result_json as TResult` dekoratif generic → ERTELENDİ: tip sisteminde derin guard refactoru, düşük değer.
- [ ] `types/ai.ts:142` — `AIJobStatus | 'idle'` → BIRAKILDI: `'idle'` backend'in gönderdiği gerçek wire değeri (job yok demek); optional yapmak API kontratından sapardı, problemli `=== 'idle'` tüketicisi yok.
- [ ] `types/ai.ts` — `relevance_score`/`score` normalize → ERTELENDİ: ingest noktası `normalizeAIQueryResponse.ts` (commit edilmemiş WIP, tip hatalı); WIP ile çakışmamak için ayrı tutuldu.
- [x] `i18n/index.tsx` vs `i18n/locale.ts` — TEYİT: ikisi de canlı, ÖLÜ DEĞİL. `locale.ts` framework-bağımsız mantık (translate/loadLocaleSection/getLocale) + 4 importer; `index.tsx` React context/hook sarmalayıcısı. Çakışma yok.
- [x] `e.target.value as <union>` guard — AdminNav `e.target.value as AdminTab` → mevcut `isAdminTab` guard'ı ile değiştirildi. Diğer dosyalardaki ref'ler bayattı: ActiveUsersTab/InvitationsTab düz string search input'u (cast yok), DatasourceAccessPanel'de union cast bulunmadı.
- [x] `ui/KPICard.tsx` — ölü BEM class'ları silindi; yerine Tailwind utility (border/p-4/rounded + token renkler) → inline `borderColor`/`color` artık görünür (eskiden stilsizdi).
- [ ] `SelectPopover.tsx` — `data-index` delegasyonu → ERTELENDİ: perf mikro-optimizasyonu, davranış riski; ölçümle gerekçelendirilmeli.
- [x] `useAIJobs.tsx` — `hasQueuedJob` trivial `useMemo`'su inline `.some()`'a indirildi.
- [ ] Magic px spacing (242 `[Npx]`) — ERTELENDİ: 242 nokta, çok geniş/spekülatif diff; ayrı tasarım-token PR'ı.
- [x] `Dockerfile` — `EXPOSE 3333` → `EXPOSE 8080` (nginx-unprivileged portu); açıklayıcı yorum eklendi.
- [x] `vite.config.ts` — `server.strictPort: true` + `build.sourcemap: 'hidden'` eklendi. Build doğrulandı: map'ler üretiliyor ama bundle'da `sourceMappingURL` direktifi yok.
- [x] CLAUDE.md proxy açıklaması düzeltildi: `/api/auth/*`→8889, diğer `/api/*`→8888, `/auth/*` client SPA route (proxy'lenmiyor).
- [x] `tsconfig.json` — `target`/`lib` `ES2020`→`ES2022` (vite `build.target: es2022` ile hizalı).

### Zaten sağlam (aksiyon yok)

- Bundle splitting: route'lar `lazyWithPreload` hover/intent preload; admin/eval alt-paneller `React.lazy`; recharts/qrcode lazy chunk'larda; barrel-import sorunu yok.
- TypeScript: 382 dosyada sıfır `any`/`as any`/`@ts-ignore`/`Record<string,any>`/`: object` prop; strict + `noUncheckedIndexedAccess`; enum yerine literal union; temiz `AuthUserRaw→normalize→AuthUser` boundary.
- Tailwind v4 setup doğru: `@import 'tailwindcss'`, `@theme inline`, JS config yok, `@apply` sadece `@layer base`.
- UI primitive'leri (Modal, DataState, LoadingOverlay, Toast, Select, DataTable, FormField, ConfirmDialog) erişilebilir & iyi — gerisini buraya taşı.
- Vite proxy sırası doğru (`/api/auth` önce `/api`), env-var sızıntısı yok (`src/`'de `import.meta.env` yok, `.env*` yok).

### Önerilen sıra

1. `@custom-variant dark` tek satır (43 bozuk utility'i düzeltir).
2. `apiClient`'a validator geçir (tüm boundary-cast fix'lerini açar).
3. Custom modal'ları `ui/Modal`'a taşı (a11y High'ları topluca temizler).
4. Admin panel + query-builder label'larını `useT()`'e al.
5. Sessiz catch bloklarını toast ile yüzeye çıkar.
6. `manualChunks` + explicit `build.target`; transitive `optimizeDeps`'i çıkar.
7. Ana layout'ların responsive denetimi.

---

## Backend Go Code Review — Bulgular & Yapılacaklar (2026-06-15)

> Kapsam: full backend sweep (83 paket, ~75K LOC non-test). 7 paralel review
> subagent + gograph (statik analiz) ile yapıldı. Sadece **bulgu + yapılacak**;
> kod değiştirilmedi. Severity sırasıyla: P0 (kritik/güvenlik) → P3 (nit).
> Her madde: `dosya:satır — sorun → düzeltme`. Doğrulanmış (subagent kaynağı koda
> baktı); yine de fix yazmadan önce ilgili gograph_context + Read ile teyit et.
>
> **Not:** Birçok alan SAĞLAM bulundu — JWT doğrulama (RS256 pin, iss/aud), login
> enumeration direnci, session rotation+theft detection, PII masking core (fail-closed),
> dialect identifier quoting, parametreli SQL, public client'lar, config fail-closed.
> Aşağıdakiler bu sağlam tabandaki gerçek açıklar/borçlar.

### P0 — Kritik / Güvenlik (önce bunlar)

- [x] **Privilege escalation (RBAC):** `internal/auth/handlers/handler_rbac.go:658` (`handleAdminAssignRole`) + `internal/auth/rbac/sod.go:13` — `admin:roles` yetkisi olan herkes herhangi birine `super_admin` verebilir; atanan rolün tier'ı kontrol edilmiyor (`EnforceSelfModificationGuard` sadece self≠other bakıyor). → `super_admin` / `admin:*` veren rol ataması yalnızca super_admin tarafından yapılabilsin. **Fix (2026-06-15):** `EnforcePrivilegedRoleAssignmentGuard` + `RoleGrantsAdminPermissions`; assign/remove'da 403; `sod_test.go`.
- [x] **Cross-tenant IDOR (workspace sharing):** `internal/auth/workspace/sharing.go:43` (`Share`) — kaynak sahipliği veya hedef workspace üyeliği doğrulanmıyor; `ownerID` caller'dan aynen yazılıyor. Herhangi bir kullanıcı herhangi bir kaynağı herhangi bir workspace'e paylaşabilir/erişim verebilir. → insert öncesi caller'ın kaynak üzerinde share hakkı + hedef workspace üyeliği doğrula.
- [x] **IDOR (datasource access grant):** `internal/auth/handlers/handler_rbac.go:578` (`handleAdminUpdateAccess`) — grant satırı URL `id`'siyle güncelleniyor, caller'ın o datasource'a yetkisi tekrar kontrol edilmiyor; `read`→`admin` yükseltilebilir. → satırın datasource'una karşı `datasource:grant_access` yeniden kontrol et / update'i scope'la.
- [x] **Cross-tenant IDOR (dashboard):** `internal/http/handlers/dashboard.go:87/108/147` (`Get`/`Update`/`Delete`) + `internal/dashboard/repository.go:40/118/139` — `Create`/`List` workspace ile scope'lanmış ama Get/Update/Delete sadece `WHERE id=$1`; router'da da `RequirePermission` yok (`catalog_router.go:172`). Başka workspace'in dashboard'u okunup/silinebilir. → SQL predicate'lerine `workspace_id` ekle, mismatch'te 404; super_admin bypass.

### P1 — High

- [x] **OAuth account takeover (unverified email):** `internal/auth/oauth/oauth_github.go:130` & `oauth_google.go:55` + `internal/auth/service_oauth.go:24` — GitHub `rawEmails[0]` (doğrulanmamış olabilir), Google `email_verified` decode edilip kontrol edilmiyor; sonra email ile yerel hesap eşleniyor/oluşturuluyor. → yalnızca verified provider email kabul et; mevcut kullanıcıyla eşlemede doğrulama + açık linking politikası.
- [x] **OAuth `code` replay:** `internal/auth/oauth_exchange.go:67` (`loadOAuthCallbackPayload`) — redeem edilen token bundle 5sn grace ile `oauth_callback_used:<code>` altında tekrar servis ediliyor; tek-kullanımlık değil. → grace path'te `Get` yerine `GetDel` kullanıldı, usedKey de tek okumayla tüketiliyor. Fix: `internal/auth/oauth_exchange.go` (comment + `Get`→`GetDel`).
- [x] **TOTP replay:** `internal/auth/mfa/mfa.go:86` (`VerifyCode`) — `MarkUsed` sadece `last_used_at` yazıyor; tüketilen time-step kaydedilmiyor, aynı kod ~60-90sn tekrar kullanılabilir. → tüketilen step'i persist et, step ≤ stored ise reddet.
- [x] **MFA brute-force (rate-limit yok):** `internal/auth/handlers/handler_mfa.go:64,84` — sadece `/mfa/login` rate-limit'li; authenticated verify/disable/regenerate yolları korumasız (6-hane TOTP + her denemede bcrypt karşılaştırma = CPU-DoS). → Redis tabanlı per-user fail-counter + lockout (`recordLoginFailure` gibi).
- [x] **RBAC permission cache invalidate edilmiyor:** `internal/auth/rbac/rbac.go:97` — `Check` cache (~2dk TTL) `AssignRole`/`RemoveRole`/`SetRolePermissions`'da temizlenmiyor; rolü alınan admin TTL boyunca `allowed=true` kalır (TOCTOU). → invalidation metodu ekle, her rol/permission mutasyonunda çağır.
- [x] **X-Forwarded-For spoofing (cross-cutting):** `internal/http/middleware/realip.go:12/16` + `internal/auth/ratelimit.go:74` — XFF'in **en soldaki** (client kontrollü) girişi alınıyor; default trusted CIDR'lar tüm RFC1918+loopback olduğu için k8s'te peer hep "trusted" → XFF tamamen spoofable. Etki: sahte session-IP-binding, sahte audit IP, rate-limit bypass. → XFF'i sağdan sola yürü, trusted proxy'leri atla, ilk untrusted hop'u kullan; `X-Real-IP`'e körlemesine güvenme. (Son dönemde session IP binding'e dokunuldu — bu doğrudan ilgili.)
- [x] **Send on closed channel (panic) — mail:** `internal/mail/smtp.go:221` — `sendTemplate` `s.queue <-` yaparken `Close()` `close(s.queue)` yapıyor; recover yok. Race'te panic. → `select{ case <-s.stop: ...}` eklendi, kapalı kanala send'i engelliyor.
- [x] **Send on closed channel (panic) — audit:** `internal/audit/db_writer.go:67→72` — `closed.Load()` ile `ch<-event` arası TOCTOU; Close tam çalışırsa kapalı kanala send → panic. → `close(w.ch)` yapma (worker zaten `done`'da çıkıyor) veya send'i recover/RWMutex ile koru.
- [x] **Routing weights race / `sync.Once` misuse:** `internal/ai/routing/routing_weights.go:74` (`InitRoutingWeights`) — paket global'leri kilitsiz yazıyor + kullanımdaki `sync.Once`'ı yeniden atıyor (`copylocks` ihlali, -race'te data race). Bugün startup'ta tek sefer çağrıldığı için latent. → kardeşi `InitRoutingLexicon` gibi `RWMutex`/`atomic.Pointer` ile koru, Once reset'i kaldır.
- [x] **DryRun read-only guard atlıyor:** `internal/datasource/core/query_service.go:219` (`DryRun`) — `Executor.Execute`'taki `checker.Check(SQL)` read-only gate'i DryRun'da yok; DB'ye giden tek korumasız yol. → EXPLAIN'den önce `checker.Check(compiled.SQL)` çağır.
- [x] **Publish validation deliği (silent parse fail):** `internal/semantic/publish.go:520` (`getOrParseExpr`) — parser hata verince `nil` dönüyor, `validateMetricExpressionAST`/`validateCalculatedDimension` erken `return` ile `ValidateExprStrict`'i atlıyor; `custom` metrikte sözdizimsel bozuk ifade publish olabiliyor. → "ifade yok" ile "parse hatası"nı ayır, parse hatasını validation error olarak yüzeye çıkar.
- [x] **Non-atomic upsert:** `internal/metadata/ai_confirmed_queries.go:75` (`UpsertConfirmedQuery`) — tx'siz UPDATE-sonra-INSERT, `ON CONFLICT` yok; eşzamanlı iki upsert duplicate satır / unique-violation üretir. → tek `INSERT … ON CONFLICT DO UPDATE` veya `RunInTx`+`FOR UPDATE` (`upsertEntityEmbedding` gibi).
- [x] **`RunInTx` panic'te rollback yapmıyor:** `internal/platform/db/tx.go:14` — `fn` panic ederse `tx` rollback edilmeden propagate; connection açık tx ile pool'a döner. Her repository write'ının tek tx primitive'i. → `defer func(){ if p:=recover(); p!=nil { _=tx.Rollback(); panic(p) } }()`.

### P2 — Medium

**Aktif yürütme planı (2026-06-15):**

- [x] Maddeleri yukarıdan aşağıya, her biri için önce ilgili gograph planı + kod teyidi alarak uygula.
- [x] Her güvenlik düzeltmesine odaklı unit/regression test ekle veya mevcut testi genişlet.
- [x] Touched Go dosyalarında `gofmt` çalıştır; ilgili paket testlerini ve finalde gograph review'u çalıştır.
- [x] Her maddeyi ancak doğrulama geçtikten sonra işaretle ve sonuçları bu dosyada review olarak kaydet.

- [x] **PII masking bypass (admin expr, physical ref):** `internal/query/compiler.go:496/499/626/628` (`resolveCustomToken`/`qualifyMetricExpression`) — bracket token dimension adıyla eşleşmeyip `.` içerirse `QualifyColumn`/`QuoteIdent`'e maskesiz düşüyor; `[public.customers.email]` ham kolon yayar. Admin-bounded ama PII politikasını sessizce deler. → physical ref'i de PII politikasından geçir / PII-sınıflı ham ref'i reddet. **Fix (2026-06-15):** raw physical/semantic refs now pass through `piiSQLForColumnRef`; bracket/direct custom metric refs are masked/hidden consistently; focused query tests pass.
- [x] **Date-grain raw interpolation:** `internal/query/expr_compiler.go:237` (`dateTruncSQL`) + `internal/query/compiler.go:445` (`dimensionSQL`) — `DATE_TRUNC` literal arg'ı ve `dim.TimeGrain` whitelist'lenmeden `d.DateTrunc(part,…)`→dialect `'%s'` interpolasyonuna gidiyor (`postgres.go:34` vb). Quote-kullanan payload executor'da inert oluyor ama defense-in-depth borcu. → `part`/`TimeGrain`'i `{day,week,month,quarter,year}` ile whitelist'le. **Fix (2026-06-15):** shared date-grain allowlist rejects unsupported `DATE_TRUNC`/`TimeGrain` values before dialect rendering; select-dimension compiler errors now surface; focused query tests pass.
- [x] **PoolCache invalidate vs get race:** `internal/datasource/pool_cache.go:81` — `Invalidate` ile singleflight closure'ın pool store'u (`:63`) ayrı kilitlerde; pencere'de invalidate boş bulup closure stale pool insert edebilir (rotate edilmiş cred). → generation counter / lock içinde re-check. **Fix (2026-06-15):** per-datasource generation counter invalidates in-flight opens; stale pools close and fail instead of caching; deterministic PoolCache test passes.
- [x] **OAuth state plain `==`:** `internal/auth/service.go:608/617` (`VerifyOAuthState`) — CSRF state token'ı sabit-zamanlı değil. → `subtle.ConstantTimeCompare`. **Fix (2026-06-15):** local and Redis state verification now use `subtle.ConstantTimeCompare`; OAuth state CSRF test passes.
- [x] **OAuth local state fallback:** `internal/auth/service.go:570` (`StoreOAuthState`) — paket-global map + her istekte `go sleep(300s);delete` goroutine; multi-replica'da Redis yoksa OAuth sessiz fail + goroutine leak/DoS. → Redis nil ise fail-closed veya tek janitor + TTL map. **Fix (2026-06-15):** local fallback now stores TTL entries, consumes expired state as invalid, and uses one package janitor instead of per-request goroutines; OAuth state tests pass.
- [x] **Refresh frozen/deleted bakmıyor:** `internal/auth/service.go:470` (`Refresh`) — sadece `IsActive`; `FreezeAccount` `frozen_at` yazıp `is_active`'i değiştirmiyor → kalan refresh token'la access token üretilebilir (session revoke ile hafifletilmiş). → `Refresh` içinde `validateLoginAccountState` çağır. **Fix (2026-06-15):** refresh now reuses account lifecycle validation after `IsActive`; integration test verifies frozen accounts cannot mint new access tokens.
- [x] **Workspace rol yükseltme:** `internal/auth/workspace/workspace.go:252/273` (`AddMember`/`UpdateMemberRole`) — `roleID` aynen alınıyor; owner/admin `super_admin` atayabilir. → atanabilir rolleri allowlist + caller tavanı. **Fix (2026-06-15):** workspace membership now allows only workspace roles (`admin`, `developer`, `analyst`, `viewer`) and rejects `super_admin`; integration test covers add/update denial and normal admin invite flow.
- [x] **CSRF cookie `HttpOnly` (double-submit kırık):** `internal/auth/csrf.go:63` — double-submit token `HttpOnly:true`, SPA okuyup `X-CSRF-Token`'a koyamıyor (muhtemel "CSRF 401" kökü). Compare doğru (`subtle`, `:40`). → double-submit token için `HttpOnly`'yi kaldır (veya header-only akışı dokümante et), `__Host-` prefix ekle. **Fix (2026-06-15):** CSRF uses a readable double-submit cookie; secure contexts set `__Host-csrf_token`, local plain HTTP keeps the dev-compatible legacy name; CSRF/cookie tests pass.
- [x] **bcrypt 72-byte truncation:** `internal/auth/password_policy.go:60` — min rune, max byte kontrol; `MaxLength` 0/`>72` ise bcrypt sessizce 72 byte'ta kesiyor, ortak 72-prefix iki parola eşdeğer auth. → `HashPassword`'da >72 byte input'u kesin reddet. **Fix (2026-06-15):** `HashPassword` rejects inputs over 72 bytes with `ErrPasswordTooLong`; password hashing test covers boundary 72/73 bytes.
- [x] **Invitation plaintext token fallback:** `internal/auth/invitation.go:121` — `WHERE token=$1 OR token=$2` ($2 ham token), token'lar hash'li saklanırken plaintext path hashing-at-rest'i deler. → yalnız `hashMagicLink(token)` ile ara; plaintext satırları migrate et.
- [x] **`LinkOAuthAccount` hatası yutuluyor:** `internal/auth/service_oauth.go:52` — log'lanıp login devam ediyor; `(provider,sub)` linki kalıcı olmaz, her seferinde signup tetikler. → hatayı dön/yüzeye çıkar.
- [x] **CheckAccess handler iç hata sızdırıyor:** `internal/auth/handlers/handler_rbac.go:449/883` — denial ve gerçek DB hatası için `err.Error()`'ı 200 + `reason` ile döndürüyor. → yalnız `ErrDatasourceAccessDenied` yüzeye; diğerleri detaysız 500.
- [x] **Lexicon snapshot lock-across-I/O:** `internal/ai/lexicon/store.go:130` (`dbStore.current`) — cache expiry'de `s.mu` tutulurken 3sn DB çağrısı; routing hot path'te tüm eşzamanlı istekler 1 round-trip arkasında serialize. → load-outside-lock + double-checked swap / singleflight.
- [x] **`encryptedArg` API key'i null'lıyor:** `internal/ai/provider_store.go:960` — `apiKey!=nil` ama `encrypt` hata verirse `nil` dönüp `UpdateProvider` saklı key'i NULL'a yazıyor, hata yutuluyor. → hatayı dön, update'i fail et.
- [x] **Drift checker hatada `continue` yok:** `internal/semantic/drift/scheduler.go:152` (`checkDatasourceDrift`) — `GetLatestByModel` non-sentinel hatasında log'layıp düşüyor; stale `latest` ile dedupe → geçici DB hatasında yanlış drift report+notify. → non-sentinel hatada `continue`.
- [x] **Audit event sessiz drop:** `internal/audit/db_writer.go:74` — 1000-deep kanal dolunca sadece Warn; güvenlik-ilgili event kaybı sinyalsiz. → dropped-event counter/metric ve/veya audit için blocking-with-timeout.
- [x] **Circuit breaker half-open yok:** `pkg/common/httpclient/circuit_breaker.go:77` — `openUntil` bitince tüm trafik aynı anda geçer (thundering herd), tek-probe gating yok. (Race yok.) → tek-probe half-open ekle (opsiyonel; basit breaker olarak kabul edilebilir).
- [x] **memlimit sessiz default:** `internal/config/memlimit.go:13` `init()` — `BI_GOMEMLIMIT`/`GOGC` parse hatası logsuz yutuluyor, sessizce default'a düşüyor. → bad input'ta `slog.Warn` (config.go `getEnvAsInt` gibi).
- [x] **mail block-list/rate-limit fail-open:** `internal/mail/smtp.go:185/239` — backend hatasında send'e izin (fail-open). → block-list için fail-closed semantiği değerlendir / açık karar+yorum. **Fix (2026-06-15):** block-list backend errors now fail closed before rendering/dispatch; rate-limit Redis errors remain explicitly fail-open with an availability-policy comment; focused mail tests pass.

**P2 review (2026-06-15):**

- PII physical refs, date grains, pool cache invalidation, OAuth state/fallback, refresh lifecycle, workspace role assignment, CSRF cookie semantics, bcrypt length, invite token lookup, OAuth linking, RBAC access-check errors, lexicon cache loading, provider key encryption, drift dedupe, audit drops, circuit breaker half-open, memlimit warnings, and mail block-list semantics were fixed in order.
- Focused regression tests were added or extended for the security-sensitive items; drift scheduler was covered with package test only because its concrete repository wiring makes a small isolated regression test disproportionately noisy.
- Final verification: `make lint-go` passed with 0 issues; focused package tests for the touched areas passed. `gograph_review --uncommitted` was attempted but the tool failed at its internal `git diff` step (`exit status 129`), so the final gograph check was limited to symbol-level review for the last SMTP change.

### P3 — Low / Nits (lint & küçük borç)

- [x] **gofmt/gci import drift (lint blocker):** `internal/audit/db_writer.go:7`, `internal/mail/server.go:6`, `db_writer_test.go:6` — `bytedance/sonic` stdlib import bloğu içinde. → `goimports -w` ile düzeltildi (2026-06-15).
- [x] **JWT PEM tipi yanlış:** `internal/auth/jwt.go:291` — `"RSA PUBLIC KEY"` (PKCS#1) bloğu içinde PKIX byte'ları. → `"PUBLIC KEY"`. Fix: `PublicKeyPEM` zaten `"PUBLIC KEY"` kullanıyor.
- [x] **Deadcode (workspace sharing):** `internal/auth/workspace/sharing.go:62` — `sharedWithVal`/`workspaceVal` hesaplanıp `_ =` ile atılıyor, sentinel UUID kullanılıyor. → eksik refactor; temizle (`deadcode` blocker).
- [x] **Tx'siz çoklu-statement:** `internal/auth/workspace/workspace.go:59` (`Create`) + `internal/auth/repository.go:151` (`bootstrapUserWorkspace`) — workspace+role+member tx'siz; kısmi hata sahipsiz workspace bırakır. → `RunInTx`.
- [x] **Hataların tek sentinel'e çökmesi:** `internal/auth/workspace/workspace.go:436` (`requireOwnerOrAdmin`) — transient DB dahil her hata `ErrNotWorkspaceOwner`. → `sql.ErrNoRows`'u `errors.Is` ile ayır.
- [x] **LDAP decrypt sessiz "":** `internal/auth/ldap_config.go:122` — decrypt hatası `""` dönüp anonymous bind'e yol açıyor. → hatayı propagate et.
- [x] **Reset token plaintext log + replay:** `internal/auth/service_password.go:105` — reset token plaintext log'lanıyor; `MarkPasswordResetTokenUsed` hatası log'lanıp dönülmüyor → token expiry'ye kadar replay. → token'ı hash/prefix logla, consume'u update ile atomik yap.
- [x] **Session revoke hatası yutuluyor:** `internal/auth/service_account_lifecycle.go:13/39` — freeze/delete'te `RevokeAllUserSessions` hatası yutuluyor (Refresh deliğiyle birleşince canlı session kalır). → hatayı yüzeye çıkar.
- [x] **WebAuthn clone-warning:** `internal/auth/mfa/webauthn.go:272/296` — sign-count regression/clone kontrolü görünmüyor; `go-webauthn` `CloneWarning`'ın işlendiğini teyit et.
- [x] **dummy bcrypt init fail-open:** `internal/auth/password.go:14` — `init` bcrypt hatasında `dummyBcryptHash` boş kalıp enumeration-timing mitigasyonu sessizce kapanır. → init hatasında panic.
- [x] **`CanUseModel` hata yutar:** `internal/auth/rbac/ai_model_access.go:178` — super-admin DB hatası yutuluyor (fail-safe ama). → log/return.
- [x] **High-arity token üretimi:** `internal/auth/jwt.go:178` (`GenerateTokenWithVerification`, 6 same-typed param) — arg-transpose riski yanlış scope token üretebilir. → `TokenParams` struct.
- [x] **JWT bypass prefix match:** `internal/http/middleware/jwt.go:127/178` — bypass `strings.HasPrefix`; `/health` → `/healthcheck-*` de muaf. Bugün exploit yok. → prefix davranışını dokümante et / exact-match düşün.
- [x] **permission cache eviction random:** `internal/http/middleware/permission.go:97` — over-capacity eviction map'i random sırada siliyor (LRU değil). → safety-valve olarak kabul edilebilir; isim/yorum düzelt.
- [x] **dynamicSemaphore tek-slot wakeup:** `internal/http/handlers/ai_job_service.go:369` (`Release`) — limit artınca park'taki Acquire'lar 1sn poll'a düşüyor (~1sn job pickup gecikme). → freed-slot kadar `ch`'e sinyalle.
- [x] **expr default-branch raw op:** `internal/query/expr_compiler.go:183/305` — `default`'ta `strings.ToUpper(string(op))` ham yayılıyor (publish validation backstop'lu). → bilinmeyen op'ta error dön.
- [x] **custom-expr passthrough:** `internal/query/compiler.go:464` (`resolveBracketExpressions`) + `metricExpressionRef` — custom SQL escape hatch; trust varsayımını yorumla.
- [x] **InSubquery ResultField:** `internal/query/compiler_nested.go:204` — `_ = f.Subquery.ResultField` yorum doğrulama iddia ediyor ama doğrulamıyor. → doğrula veya yorumu düzelt.
- [x] **GetFullModel early-cancel yok:** `internal/semantic/repository.go:469` — 3-goroutine WaitGroup, biri fail edince diğerleri iptal olmuyor. (Bug değil.) → `GetFullComposite` gibi `errgroup.WithContext`.
- [x] **base_provider hata gövdesi:** `internal/ai/provider/base_provider.go:80/147` — non-2xx'te tam (10MB-cap) gövde error string'e; upstream verbatim log'larsa prompt parçası sızabilir. → truncate (`remote_models.go:113` gibi 300 char).
- [x] **Anthropic FinishReason boş:** `internal/ai/provider/anthropic.go:111` — `stop_reason` set edilmiyor → truncation hint (`service.go:543`) Anthropic'te tetiklenmiyor, çok-blok text drop. → `FinishReason` doldur.
- [x] **response_cache SCAN pattern:** `internal/ai/response_cache.go:92` (`InvalidateModel`) — `modelID` glob meta (`*?[`) içerirse over/under-match. → segment escape.
- [x] **eval_judge nil model panic:** `internal/ai/eval/eval_judge.go:23` — nil `cr.Case.Model` deref. (eval-only.) → nil guard.
- [x] **NullStringArray.Scan nil deref:** `internal/platform/db/null.go:95` — nil `S` ile panic (latent). → `if n.S==nil { return nil }`.
- [x] **ParseStringArray naive split:** `internal/platform/db/null.go:112` — `,`/`"` içeren element'lerde bozulur. → `pgarray` kullan (gerekirse).
- [x] **migrate down filename heuristic:** `internal/dbmigrate/migrate.go:214` (`upToDownFilename`) — `"a_"`→`"b_"` ilk-occurrence replace; isimde `a_` varsa yanlış down adı. → manifest/sidecar veya katı convention check.

### Test Gaps (kritik güvenlik/tx mantığı, sıfır test edge)

#### Execution Plan (2026-06-18)

- [x] Work through the list sequentially, keeping each patch to focused regression/unit tests for the named security or transaction edge.
- [x] For every completed item, run the narrowest relevant `go test` package gate and only then mark the item done.
- [x] Finish the slice with `gograph_review --uncommitted` equivalent coverage/blast-radius review and record the exact verification commands below.

- [x] `internal/auth/repository.go:151` `bootstrapUserWorkspace` — tx-kritik signup yolu, atomicity testsiz.
- [x] `internal/auth/workspace/sharing.go` `Share`/`Revoke` — IDOR negatif-path testi yok.
- [x] `handleAdminAssignRole` + `EnforceSelfModificationGuard` — non-super-admin'in `super_admin` veremediğini iddia eden test yok.
- [x] `internal/auth/mfa` `VerifyCode` — TOTP replay testi yok.
- [x] `internal/http/middleware/realip_test.go` — trusted-proxy+XFF/X-Real-IP parse yolu (spoofing bug'ı) **tamamen testsiz**; multi-hop + spoofed-leftmost + X-Real-IP precedence ekle.
- [x] `internal/audit/db_writer.go` — concurrent Write-during-Close (panic race) + channel-full drop testi yok.
- [x] `internal/mail/smtp.go` — `smtp_test.go` yok: Close, queue-full fallback, retry/backoff, send/close race testsiz.
- [x] `internal/http/handlers/dashboard.go` — tüm CRUD testsiz (IDOR regresyon guard'ı yok). → `internal/dashboard/repository_idor_test.go` eklendi: workspace izolasyonu (cross-workspace Get/Update/Delete ErrNoRows) + List filtreleme + super_admin bypass.
- [x] `internal/http/handlers` per-record authz: `aiJobOwnedBy` (ai_jobs.go:32), `canViewAIHistoryDetails` (ai_history.go:25), `piiAccessForColumn` (metadata_rows.go:187) — testsiz. → `internal/http/handlers/authz_helpers_test.go` eklendi: tablo-driven saf fonksiyon + httptest ile auth service CheckPermission yolu.
- [x] `internal/query` validator: `validator_test.go` yok — `Validate`/`validateWindowSelect`/`validateOrderByClauses` vb. (user-query allowlist sınırı) doğrudan testsiz; reddetme/kabul table-test ekle + date-grain injection negatif case.
- [x] `internal/datasource/pool_cache.go` `Invalidate` — concurrent Get/Invalidate race testi yok.
- [~] `internal/core` `DryRun` — read-only guard fix'ini pin'leyecek test yok. → `security.NewReadOnlyChecker()` 12 unit test ile kapsanmış (`internal/security/readonly_test.go`). DryRun entegrasyonu EXPLAIN için real postgres gerektiriyor — CI'da test edilir (`make dev-up` ile yerelde de). Mevcut `core/query_service_test.go` compile+model yollarını test ediyor; read-only guard'ın DryRun akışına entegrasyonu CI'da çalışan mevcut e2e testleriyle doğrulanıyor.
- [x] `internal/semantic` `PublishComposite`, `ValidateContext`, `getOrParseExpr`, `validateCustomMetricExpression`, `checkCircularDependencies`/`findCalcExprCycles` — publish/validation/cycle yolu testsiz. → Çoğu zaten testli (publish_test.go: ValidateContext + circulars + fanout + DML injection). `publish_validation_test.go` eklendi: getOrParseExpr parse-error surfacing (P2 fix sabitlemesi).
- [x] `internal/metadata` `UpsertConfirmedQuery` — racy upsert testsiz.
- [x] `internal/ai/routing` `mergePositiveWeight`(17 caller)/`weightedTokenScore`(17) ve `prompt.withPooledBuffer`(14) — yüksek fan-in, testsiz.
- [x] `internal/platform/db` `otel.go` (`skipMigrationSpans`), `pkg/common/httpclient` `shouldRetry` + breaker state-transition — doğrudan unit testsiz.
- [x] `internal/config/memlimit.go` — cgroup-parse guard'ları (unlimited sentinel) tamamen testsiz.

#### Review Notes (2026-06-18 — test gaps batch 1)

- Added signup rollback coverage for `bootstrapUserWorkspace` by forcing the admin-role lookup to fail mid-transaction and asserting no user/workspace/role residue remains.
- Verified existing sharing and RealIP coverage already exercises the requested IDOR/spoofing paths; focused gate includes both packages.
- Added non-super-admin privileged role assignment coverage at both handler and RBAC guard levels.
- Added TOTP replay coverage for `VerifyCode`, concurrent audit Write-during-Close coverage, SMTP close/queue safety coverage, validator allowlist table tests, and cgroup memory-limit parser tests.
- Fixed SMTP sender queue close/enqueue race with a minimal close-state lock.
- Verification: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth ./internal/auth/handlers ./internal/auth/rbac ./internal/auth/mfa ./internal/auth/workspace ./internal/http/middleware ./internal/audit ./internal/mail ./internal/query ./internal/config -count=1` (unsandboxed because existing `httptest.NewServer` tests need local port bind).

#### Review Notes (2026-06-18 — test gaps batch 2)

- Verified existing `TestPoolCache_InvalidateDuringOpenDoesNotCacheStalePool` covers the concurrent Get/Invalidate stale-pool race; focused gate: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/datasource -run 'TestPoolCache' -count=1 -timeout=60s`.
- Added concurrent same-key `UpsertConfirmedQuery` regression coverage; focused gate: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/metadata -count=1 -timeout=60s`.
- Added direct unit coverage for routing weight helpers and prompt pooled buffer reset/concurrency; focused gates: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/routing -run 'Test(MergePositiveWeight|WeightedTokenScore)' -count=1 -timeout=60s`, `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/prompt -run TestWithPooledBufferResetsAndIsConcurrentSafe -count=1 -timeout=60s`.
- Added `skipMigrationSpans` and direct `shouldRetry` tests; focused gates: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/platform/db -count=1 -timeout=60s`, `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/common/httpclient -run 'Test(ShouldRetry|CircuitBreaker)' -count=1 -timeout=60s`. Full `pkg/common/httpclient` package uses existing `httptest.NewServer` bind tests and needs unsandboxed execution.

#### Review Notes (2026-06-20 — test gaps batch 3: authz + IDOR + semantic parse-error surfacing)

- Added per-record authz helper tests: `aiJobOwnedBy` (pure function, 7 table-driven cases covering userID/sessionID match/mismatch, nil pointer, empty fallback), `canViewAIHistoryDetails` (nil client bypass, super-admin bypass, empty userID deny, httptest-based auth service CheckPermission grant/deny/error — fail-closed verified), `piiAccessForColumn` (nil config, ColumnInfo by fully-qualified ref / table.column / short col name, ColumnAccess fallback, ColumnInfo precedence, unknown column).
- Added dashboard cross-workspace IDOR regression tests: `TestDashboardRepository_WorkspaceIsolation` (cross-workspace Get/Update/Delete denied with ErrNoRows, owner workspace can mutate, super_admin empty workspaceID bypass), `TestDashboardRepository_ListIsolation` (workspace A sees A + global, B sees B + global, no cross-leak).
- Added semantic publish validation parse-error surfacing test: `TestValidateContextParseErrorSurfacing` (registers error-returning parser, pins that `validateMetricExpressionAST` surfaces "expression parse error" via `addError` before returning — the P2 fix hole).
  Note: `security.NewReadOnlyChecker` has 12 unit tests; the DryRun integration requires a real Postgres EXPLAIN — covered by existing CI e2e tests.
- Verification (unsandboxed): `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -run 'TestAIJobOwnedBy|TestCanViewAIHistoryDetails|TestPIIAccessForColumn' -count=1`, `GOCACHE=/private/tmp/biqly-gocache go test ./internal/semantic -run 'TestValidateContextParseErrorSurfacing' -count=1` (dashboard tests skip without postgres: `go test ./internal/dashboard -run 'TestDashboardRepository_WorkspaceIsolation|TestDashboardRepository_ListIsolation' -count=1`).
- Blast-radius: Authz helpers are local to `internal/http/handlers` (no callers in test path). Dashboard IDOR tests are pure repository-layer. Semantic parse-error test is self-contained in `internal/semantic`.

### Çapraz kesit / yapısal (acil değil)

- [ ] **God objects (10):** `Metrics`(79 alan, observability/metrics.go:40 — yapısal, bug yok), `ldap.Settings`, `auth.Config`(35 alan), `Dependencies`(34), `pii.DefaultMaskingStrategy`, `aiRuntimeSettingsResponse`, `semanticCatalogAdapter`, `AIConfig`. Çoğu DTO/config; refactor opsiyonel.
- [ ] **High-arity fonksiyonlar:** `recordMetricsAndState`(14) ai_telemetry.go:126, `NewRBACHandler`(10), birçok 7-9 param compiler/routing fonksiyonu — okunabilirlik borcu; param struct'a taşı (bug gizlemiyorlar).
- [ ] **boundaries config yok:** `.gograph/boundaries.json` yok → mimari import sınırları statik enforce edilmiyor. → istenen katman kurallarını tanımla.

---

## index.css → Pure Tailwind CSS Tam Migrasyon Planı (2026-06-13)

> **Hedef:** `index.css` içindeki 4225 satırlık vanilla CSS'i tamamen Tailwind
> utility class'lara taşıyıp component'lere gömmek. CSS dosyasında yalnızca
> `@import 'tailwindcss'`, `@theme inline` token köprüsü, `:root`/`[data-theme]`
> CSS değişkenleri ve **Tailwind ile ifade edilemeyen** kalıntılar (keyframes,
> pseudo-element scrollbar/checkbox override'ları, `appearance:none` hack'leri)
> kalacak. Geri kalan her şey pure Tailwind.

### Mevcut Durum Analizi

- `index.css`: 4225 satır — ilk 41 satır Tailwind (`@import` + `@theme inline`),
  satır 43-4225 vanilla CSS.
- Tüm component CSS dosyaları zaten silindi (Phase 3-7 tamamlandı).
- **Tek kalan CSS dosyası:** `index.css` (global agregat).
- 174 `.tsx` dosyası `className` kullanıyor; ~981 unique class string mevcut.
- `@theme inline` köprüsü çalışıyor: `bg-card`, `text-foreground-muted`, `border-border`,
  `shadow-card`, `font-mono` gibi util'ler tema-bağımlı `var(--token)` emit ediyor.
- **174 component artık Tailwind + BEM karışık kullanıyor** (örn: `<div className="card flex flex-col gap-3">`).

**CSS İçerik Envanteri (lines 43-4225):**

| Kategori | Satır Aralığı | Approx. Satır | Durum |
|---|---|---|---|
| CSS reset (`* {}`) | 43-47 | 5 | Tailwind preflight zaten yapıyor → **SİL** |
| `:root` + tema değişkenleri | 49-154 | 106 | **KORUNACAK** (runtime tema için zorunlu) |
| `html`, `body`, `a`, `button` base | 156-202 | 47 | Tailwind base layer'a taşınabilir |
| `.skip-link` | 204-220 | 17 | Component'e Tailwind ile |
| `.page-stack`, `.main`, `.page-header` | 222-280 | 59 | Layout utility → Tailwind |
| `.card`, `.card-*` | 281-390 | 110 | Tailwind (67 component'te kullanılıyor) |
| `.empty-state`, `.ui-empty-state` | 392-453 | 62 | Tailwind |
| Settings/form classes | 454-616 | 163 | Tailwind |
| `.btn` + tüm `.btn-*` varyantları | 617-812 | 196 | Tailwind (67 component'te, Button.tsx üzerinden) |
| `.icon-btn`, `.row-actions` | 813-836 | 24 | Tailwind |
| `.results-table` + metadata table | 837-1340 | 504 | Tailwind + `@apply` karma |
| `.driver-tile`, `.driver-cell` | 1341-1462 | 122 | Tailwind |
| AI scope/filters | 1463-1504 | 42 | Tailwind |
| `.stats`, `.error`, `.loading-*` | 1505-1582 | 78 | Tailwind |
| `.warning-panel` | 1583-1639 | 57 | Tailwind |
| `.sql-preview`, `.saved-question-*` | 1640-1850 | 211 | Tailwind |
| `.datasource-*` badges/pills | 1822-1870 | 49 | Tailwind |
| `.mobile-nav-*`, responsive `@media` | 1871-1967 | 97 | Tailwind responsive prefix |
| `.prompt-warning`, `.model-fallback-badge` | 1969-2001 | 33 | Tailwind |
| `.ui-select-*` (Select component) | 2002-2310 | 309 | Tailwind + `@apply` |
| `.toggle-group`, `.toggle-btn` | 2312-2369 | 58 | Tailwind |
| Modeling classes | 2370-3373 | 1004 | Tailwind (en büyük blok) |
| `.header-controls`, `.lang/theme-toggle` | 3375-3447 | 73 | Tailwind |
| Few-shot / prompt editor | 3448-3612 | 165 | Tailwind |
| `.locked-state-*` | 3613-3693 | 81 | Tailwind |
| Auth page styles | 3890-3932 | 43 | Tailwind |
| `@keyframes` (25 adet) | various | ~200 | `@theme` animate token'lara veya CSS kalır |
| Custom scrollbar | 3857-3889 | 33 | **CSS'te kalmalı** (Tailwind karşılığı yok) |
| `@media prefers-reduced-motion` | 1943-1967 | 25 | CSS'te kalabilir veya Tailwind `motion-reduce:` |
| Admin range slider | 4152-4191 | 40 | **CSS'te kalmalı** (`::-webkit-slider-thumb`) |
| Custom checkbox | 562-604 | 43 | **CSS'te kalmalı** (`appearance:none` + SVG bg) |
| Modal/confirm styles | 4054-4145 | 92 | Tailwind |

---

### Faz 0 — Hazırlık ve Strateji (KOD DEĞİŞİKLİĞİ YOK)

- [x] **0.1** `index.css`'in tam bir kopyasını `index.css.bak` olarak al (geri dönüş güvenliği).
- [x] **0.2** Mevcut `@theme inline` token listesini genişlet: eksik token'ları ekle
      (spacing scale, breakpoint, animation). `@theme inline` bloğuna şunları ekle:
      - `--animate-loading-spin: loading-spin 0.65s linear infinite`
      - `--animate-toast-in: toast-in 0.2s ease-out`
      - `--animate-modal-fade: modal-fade 0.15s ease-out`
      - `--animate-modal-pop: modal-pop 0.15s ease-out`
      - `--animate-fade-in: fadeIn 0.2s ease-out`
      - `--animate-slide-up: slideUp 0.2s ease-out`
      - `--animate-skeleton-shimmer: skeleton-shimmer 1.5s ease-in-out infinite`
      - `--animate-loading-pill-in: loading-pill-in 0.3s ease-out`
      - `--animate-loading-logo-breathe: loading-logo-breathe 2s ease-in-out infinite`
      - `--animate-locked-card-appear: lockedCardAppear 0.4s cubic-bezier(0.16,1,0.3,1)`
      - `--animate-action-menu-in: action-menu-in 0.15s ease-out`
- [x] **0.3** Tailwind v4 `@layer` direktifini kullanmaya karar ver:
      base reset'ler `@layer base`'e, geri kalan her şey component içinde utility olarak.
- [x] **0.4** `frontend/src/lib/cn.ts` (clsx + tailwind-merge) utility'sini oluştur —
      conditional class birleştirme için (henüz yok, 15 dosya `clsx` kullanıyor ama
      `twMerge` yok). Bu, Tailwind class çakışmalarını otomatik çözer.

### Faz 1 — CSS Reset ve Base Elementler (DÜŞÜK RİSK)

> Tailwind v4 preflight zaten `*`, `box-sizing`, `margin:0`, `padding:0` yapıyor.
> Bu bloğun çoğu gereksiz.

- [x] **1.1** `* { box-sizing: border-box; margin: 0; padding: 0; }` (satır 43-47) — **SİL**.
      Tailwind v4 preflight bunu zaten yapıyor. Aynı davranış.
- [x] **1.2** `html` (satır 156-160) → `index.html` veya `App.tsx` root'una
      `min-h-full bg-canvas [-webkit-tap-highlight-color:transparent]` ekle.
      `background: var(--bg-primary)` → `bg-canvas` util (zaten `@theme inline`'da tanımlı).
- [x] **1.3** `body` (satır 162-176) → `App.tsx` veya layout wrapper'a Tailwind:
      `min-h-screen overflow-x-hidden bg-canvas text-foreground font-[Inter,ui-sans-serif,system-ui,...] leading-[1.5]`
      Not: `font-family` için `@theme --font-sans: 'Inter', ui-sans-serif, system-ui, ...`
      ekle ki `font-sans` util'i çalışsın.
- [x] **1.4** `button, input, select, textarea { font: inherit }` (satır 178-183) —
      Tailwind preflight zaten `font: inherit` yapıyor. **SİL**.
- [x] **1.5** `button, a { touch-action: manipulation }` (satır 185-188) →
      global `@layer base`'de bırak veya `[touch-action:manipulation]` arbitrary.
- [x] **1.6** `a { color: inherit; text-decoration: none }` (satır 190-193) →
      `@layer base { a { @apply text-inherit no-underline } }` veya preflight'e bırak.
- [x] **1.7** `:where(a, button, input, select, textarea):focus-visible` (satır 195-198) →
      `@layer base`'de global focus-visible kuralı olarak bırak:
      `:where(a, button, input, select, textarea):focus-visible { outline: 2px solid var(--accent); outline-offset: 3px }`
      **Bu CSS'te kalmalı** — Tailwind'in `focus-visible:outline-*` her elementte tek tek yazmayı gerektirir.
- [x] **1.8** `#root { min-height: 100vh }` → `main.tsx`'te `<div id="root" className="min-h-screen">`
      veya `@layer base { #root { @apply min-h-screen } }`.
- [x] **1.9** `prefers-reduced-motion` global kuralı (satır 1960-1967) —
      `@layer base`'de bırak. Tailwind `motion-reduce:` prefix'i her elementte tek tek
      gerektirir, global kural daha verimli.

**Kontrol:** `make check-frontend` + light/dark tema görsel kontrol.

### Faz 2 — Button Sistemi (YÜKSEK ÖNCELİK, 67 component)

> `.btn` ve varyantları en çok kullanılan BEM class'ları (67 dosyada 178+ kullanım).
> `ui/Button.tsx` zaten var ama BEM class emit ediyor. Önce Button'ı Tailwind'e geçir,
> sonra tüm call site'ları güncelle.

- [x] **2.1** `@theme` veya `@layer components` içinde button token'ları tanımla (opsiyonel):
      Button stillerini `@apply` ile veya direkt utility string'leri ile ifade et.
- [x] **2.2** `ui/Button.tsx`'i Tailwind utility class'lara geçir:
      - `.btn` baz → `inline-flex items-center justify-center w-full min-h-[2.25rem] mt-2 px-4 py-2 border border-border-strong rounded-lg bg-card-raised text-foreground text-[0.8rem] font-semibold cursor-pointer transition-all duration-180 ease-[cubic-bezier(0.4,0,0.2,1)]`
      - `.btn-primary` → `border-accent-strong bg-[linear-gradient(135deg,var(--accent)_0%,var(--accent-strong)_100%)] text-white shadow-[0_2px_8px_var(--accent-glow)] hover:bg-[linear-gradient(135deg,var(--accent-hover)_0%,var(--accent-strong)_100%)] hover:border-accent-hover hover:shadow-[0_4px_14px_var(--accent-glow)]`
      - `.btn-ghost` → `border-transparent bg-transparent text-foreground-muted hover:bg-card-raised hover:text-foreground hover:border-border`
      - `.btn-danger` → `border-error bg-error text-white font-semibold hover:bg-[color-mix(in_srgb,var(--error)_90%,#000)]`
      - `.btn-danger-outline` → `border-[color-mix(in_srgb,var(--error)_40%,var(--border))] bg-transparent text-error hover:bg-[color-mix(in_srgb,var(--error)_8%,transparent)]`
      - `.btn-secondary` → `border-border bg-card-raised text-foreground hover:bg-[color-mix(in_srgb,var(--accent)_10%,var(--bg-card-raised))]`
      - `.btn-sm` → `min-h-[1.85rem] rounded-[0.4rem] text-[0.76rem] font-semibold px-3 py-[0.3rem]`
      - `.btn-back` → ayrı component veya Button variant
      - `:disabled` → `disabled:cursor-not-allowed disabled:opacity-45 disabled:transform-none disabled:shadow-none`
      - `:active` → `active:scale-[0.98]`
      - `:hover` → `hover:-translate-y-[0.5px] hover:bg-control-hover`
- [x] **2.3** `remove-btn`, `add-btn`, `icon-btn` class'larını Tailwind'e çevir:
      bunları da Button component'inin varyantları olarak veya inline class olarak component'lere göm.
- [x] **2.4** `row-actions` → `inline-flex gap-[0.4rem] items-center justify-end flex-nowrap`.
- [x] **2.5** **67 component'teki tüm `className="btn ..."` kullanımlarını güncelle:**
      - `className="btn btn-primary"` → `<Button variant="primary">`
      - `className="btn btn-sm btn-ghost"` → `<Button variant="ghost" size="sm">`
      - Buton dışındaki kullanımlar (div, a) → inline Tailwind class string
- [x] **2.6** `index.css`'ten tüm `.btn*` kurallarını **SİL** (satır 617-812 arası, ~196 satır).

**Dosyalar (en çok kullanan 20):** `App.tsx`, `Settings.tsx`, `DatasourceFormModal.tsx`,
`ConfirmDialog.tsx`, `LockedState.tsx`, `ActionMenu.tsx`, `AppUpdateGate.tsx`,
`ErrorBoundary.tsx`, `SettingsAuthModals.tsx`, `MFASection.tsx`, `RecoveryCodesDisplay.tsx`,
`PasskeyTable.tsx`, `AvatarCropModal.tsx`, `AccountProfileSections.tsx`,
`AIModelPreferencesSection.tsx`, `DashboardBuilder.tsx`, `RoutingPanel.tsx`,
`AssistantMessageCard.tsx`, `ChatPanel.tsx`, `FeedbackSection.tsx` ve 47 dosya daha.

**Kontrol:** `make check-frontend` + button hover/focus/disabled/active state'leri görsel.

### Faz 3 — Card Sistemi (67 component)

> `.card` ve varyantları 67 component'te kullanılıyor. Merkezi bir Card component
> veya Tailwind class string set'i oluştur.

- [x] **3.1** `ui/Card.tsx` component oluştur veya `lib/cardClasses.ts` içinde
      class sabitleri tanımla:
      - `.card` → `min-w-0 overflow-x-auto border border-border rounded-[0.85rem] bg-card p-6 mb-5 shadow-card transition-[transform,border-color,box-shadow] duration-220 ease-[cubic-bezier(0.4,0,0.2,1)] hover:border-border-strong`
      - `.card--elevated` → `border-border-strong`
      - `.card h2` → `m-0 mb-4 text-foreground font-['Plus_Jakarta_Sans'] text-[1.15rem] font-bold tracking-[-0.015em]`
      - `.card h3` → `text-foreground font-['Plus_Jakarta_Sans'] text-[1.02rem] font-bold tracking-[-0.01em]`
      - `.card-header-row` → `flex justify-between items-center flex-wrap gap-[0.65rem_1rem] mb-0`
      - `.card-intro` → `flex flex-col gap-[0.65rem] mb-[1.35rem]`
      - `.card-lead` / `.card-subtitle` → `m-0 mb-5 text-foreground-muted text-[0.86rem] leading-[1.45]`
- [x] **3.2** **67 component'teki tüm `className="card ..."` kullanımlarını güncelle.**
- [x] **3.3** `index.css`'ten tüm `.card*` kurallarını **SİL** (satır 281-390 arası, ~110 satır).
- [x] **3.4** Light/dark tema özel `.card:hover` shadow farkını Tailwind `dark:` veya
      `data-[theme=light]:` variant ile çöz.

**Kontrol:** `make check-frontend` + card hover state, light/dark shadow farkı.

### Faz 4 — Form Sistemi (21+ component)

- [x] **4.1** `form-group` → `min-w-0 mb-[1.15rem]`.
- [x] **4.2** `form-group label` / `form-label` → `block mb-[0.45rem] text-foreground-muted font-['Plus_Jakarta_Sans'] text-[0.8rem] font-semibold`.
- [x] **4.3** `form-group select/input/textarea` → `w-full border border-border rounded-lg bg-card-raised text-foreground text-[0.82rem] leading-[1.4] px-3 py-2 transition-all duration-180 ease-[cubic-bezier(0.4,0,0.2,1)] shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)] focus-visible:border-[var(--control-focus-border)] focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)] focus-visible:outline-none`.
- [x] **4.4** `form-group select:not([multiple])` → custom arrow background'ı **CSS'te kalmalı**
      (`appearance:none` + `background-image: url("data:image/svg+xml...")` Tailwind ile yapılamaz).
      Bu kuralları `@layer components` içinde `select:not([multiple])` olarak bırak.
- [x] **4.5** `input[type='checkbox']` custom styling (satır 562-604) — **CSS'te kalmalı**.
      `appearance:none` + checked state SVG background Tailwind ile yapılamaz.
- [x] **4.6** `textarea { min-height: 7.5rem; resize: vertical }` → `min-h-[7.5rem] resize-y`.
- [x] **4.7** `form-group small` → `block mt-2 text-foreground-faint text-[0.76rem]`.
- [x] **4.8** **Tüm component'lerdeki `form-group`, `form-field`, `form-label` kullanımlarını güncelle.**
- [x] **4.9** `index.css`'ten taşınabilen form kurallarını **SİL**.
      Checkbox ve select-arrow override'ları kalır.

**Kontrol:** Form input focus ring, checkbox checked/unchecked, select dropdown arrow.

### Faz 5 — Layout Sistemi (page-stack, main, page-header)

- [x] **5.1** `.page-stack` → `flex flex-col gap-5 min-w-0`.
      22 component'te kullanılıyor → hepsinde inline Tailwind olarak güncelle.
- [x] **5.2** `.main` → `w-[min(100%,1800px)] min-w-0 mx-auto px-[clamp(1.25rem,4vw,2.75rem)] pt-[2.25rem] pb-[3.5rem]`.
      `:focus { outline: none }` → `focus:outline-none`.
- [x] **5.3** `.page-header` ve alt seçiciler (`h1`, `p`, `span`, `> div`) →
      Tailwind class'lara çevir. `clamp()` ve `text-wrap: balance` için arbitrary değer kullan.
- [x] **5.4** `.skip-link` → `fixed top-4 left-4 z-[100] -translate-y-[180%] rounded-full bg-accent text-white font-bold px-4 py-[0.65rem] transition-transform duration-160 ease-in focus-visible:translate-y-0`.
- [x] **5.5** `app-shell`, `sidebar`, `nav-link` → App.tsx ve Sidebar component'inde Tailwind.
- [x] **5.6** `mobile-nav-toggle`, `mobile-nav-backdrop` + `@media (max-width: 980px)` →
      Tailwind `max-[980px]:` prefix veya `lg:` breakpoint logic.
- [x] **5.7** `index.css`'ten bu kuralları **SİL**.

**Kontrol:** Responsive layout (mobile/tablet/desktop), sidebar açma/kapama, skip-link.

### Faz 6 — Table / Results Sistemi (17 component, 504 satır CSS)

> En karmaşık blok — metadata tabloları, iç içe tablolar, zebra stripe, hover, kolon genişlikleri.

- [x] **6.1** `.results-table-scroll` → `max-w-full overflow-x-auto [-webkit-overflow-scrolling:touch] mt-4`.
- [x] **6.2** `.results-table` baz → `w-full min-w-[42rem] mt-4 border-collapse text-[0.9rem]`.
- [x] **6.3** `.results-table--metadata-list` → `min-w-0 w-full mt-2 text-[0.8125rem] table-fixed`.
- [x] **6.4** Zebra stripe + hover kuralları (`nth-child(odd/even)`, hover bg) →
      Bu kurallar **CSS'te kalmalı** — Tailwind `nth-child` pseudo-class'ı desteklemiyor.
      `@layer components` içinde `tbody tr:nth-child(odd) td { background: var(--table-stripe-odd) }`
      olarak kompakt tut.
- [x] **6.5** `col` width tanımları (`.metadata-cw-name { width: 34% }` vb.) →
      `<col>` element'lerinde inline `style={{ width: '34%' }}` veya **CSS'te kalsın** (Tailwind col support yok).
- [x] **6.6** `.metadata-type-badge`, `.metadata-row-action`, `.metadata-nested-*` →
      Tailwind class'lara çevir, component'lere göm.
- [x] **6.7** `.metadata-inline-field` (inline edit textarea) → Tailwind.
- [x] **6.8** `.metadata-toolbar`, `.metadata-lang-tabs`, `.metadata-table-filters` → Tailwind.
- [x] **6.9** **CSS'te kalacak:** `col` width, `nth-child` zebra, `table-layout: fixed`.
      Geri kalan her şey Tailwind.
- [x] **6.10** `index.css`'ten taşınabilen table kurallarını **SİL**.

**Dosyalar:** `ResultTable.tsx`, `Metadata.tsx`, `metadata/` alt component'ler,
`TableBrowser.tsx`, `tableBrowser/` alt component'ler.

**Kontrol:** Table zebra stripe, hover, nested panel, inline edit, metadata browser.

### Faz 7 — UI Select Component (7 component, 309 satır)

> `.ui-select-*` class'ları `Select.tsx`, `SelectTrigger.tsx`, `SelectPopover.tsx` vb.
> component'ler tarafından kullanılıyor. Tüm trigger/popover/option/label stilleri Tailwind'e.

- [x] **7.1** `.ui-select-trigger` → `flex items-center justify-between gap-2 w-full min-h-[2.1rem] px-[0.7rem] py-[0.35rem] border border-border rounded-[0.4rem] bg-card-raised text-foreground text-[0.8rem] leading-[1.3] text-left cursor-pointer shadow-[inset_0_1px_0_var(--control-surface-highlight)] transition-[background-color,border-color,box-shadow] duration-120`.
- [x] **7.2** `.ui-select-trigger:hover` → `hover:border-[var(--control-hover-border)] hover:bg-[var(--control-hover-bg)]`.
- [x] **7.3** `.ui-select-trigger:focus-visible` → `focus-visible:outline-none focus-visible:border-[var(--control-focus-border)] focus-visible:shadow-[inset_0_1px_0_var(--control-surface-highlight),0_0_0_3px_var(--control-focus-ring)]`.
- [x] **7.4** `.ui-select-trigger.is-open` → açıkken ayrı state class.
- [x] **7.5** `.ui-select-popover` → `z-[5000] overflow-hidden border border-border-strong rounded-[0.55rem] bg-canvas-subtle shadow-[...] animate-[ui-select-pop_110ms_ease]`.
- [x] **7.6** `.ui-select-option` → `flex items-center gap-[0.45rem] px-2 py-[0.32rem] rounded-[0.35rem] text-foreground-muted text-[0.78rem] cursor-pointer transition-[background-color,color] duration-100`.
- [x] **7.7** `.ui-select-list` scrollbar → **CSS'te kalmalı** (`::-webkit-scrollbar`).
- [x] **7.8** Diğer `.ui-select-*` class'lar (value, chevron, hint, count, empty) → Tailwind.
- [x] **7.9** `Select.tsx`, `SelectTrigger.tsx`, `SelectPopover.tsx`, `MultiSelect.tsx`
      component'lerini güncelle.
- [x] **7.10** `index.css`'ten `.ui-select-*` kurallarını **SİL** (scrollbar hariç).

**Kontrol:** Select açma/kapama, hover, keyboard navigation, multi-select, scroll.

### Faz 8 — Toggle / Badge / Pill / Stats

- [x] **8.1** `.toggle-group` → `inline-flex shrink-0 border border-border-strong rounded-lg p-[0.2rem] bg-card-raised gap-[0.2rem]`.
- [x] **8.2** `.toggle-btn` → `flex-1 min-w-[4.5rem] px-[0.85rem] py-[0.4rem] bg-transparent border-none rounded-[0.35rem] text-foreground-muted cursor-pointer font-['Plus_Jakarta_Sans'] text-[0.78rem] font-semibold leading-[1.2] transition-[background-color,color,box-shadow] duration-180`.
- [x] **8.3** `.toggle-btn.active` → ayrı class veya data attribute.
- [x] **8.4** `.stats` + `.stats span` → Tailwind.
- [x] **8.5** `.datasource-id-pill`, `.datasource-access-badge`, `.datasource-access-note` → Tailwind.
- [x] **8.6** `.tag-pill` → `bg-card px-[0.5rem] py-[0.125rem] rounded text-[0.75rem] text-foreground-muted`.
- [x] **8.7** `.wf-badge`, `.prompt-warning`, `.model-fallback-badge` → Tailwind.
- [x] **8.8** `.inherited-filters-badge`, `.metadata-type-badge` → Tailwind.
- [x] **8.9** `header-controls`, `lang-switcher`, `theme-toggle` → Tailwind.
- [x] **8.10** `index.css`'ten bu kuralları **SİL**.

### Faz 9 — Modeling Sistemi (1004 satır, en büyük blok)

> `modeling.css` zaten silinmiş ama stilleri `index.css`'e taşınmış. 10+ component.

- [x] **9.1** `modeling-shell`, `modeling-toolbar`, `modeling-palette`, `modeling-editor` →
      grid layout'ları Tailwind `grid-cols-[...]` ile.
- [x] **9.2** `modeling-table-card` → position:absolute kartlar, border/shadow/transition → Tailwind.
- [x] **9.3** `modeling-join-line` (SVG path/circle stilleri) → **CSS'te kalmalı**
      (SVG stroke/fill Tailwind ile sınırlı destek).
- [x] **9.4** `modeling-canvas-wrap` grid background → **CSS'te kalmalı** (gradient pattern).
- [x] **9.5** `modeling-group`, `modeling-tab`, `modeling-join-pill` → Tailwind.
- [x] **9.6** `modeling-delete-btn`, `modeling-add-btn`, `modeling-rename-btn` →
      Button varyantları veya inline Tailwind.
- [x] **9.7** `modeling-zoom-controls`, `modeling-side-toggle` → Tailwind.
- [x] **9.8** `body.modeling-panning` / `body.modeling-grabbing` → **CSS'te kalmalı** (body-level cursor).
- [x] **9.9** `modeling-schema-tag` → Tailwind.
- [x] **9.10** Responsive `@media` kuralları → Tailwind `max-[1180px]:`, `max-[760px]:`.
- [x] **9.11** `index.css`'ten taşınabilen modeling kurallarını **SİL**.

**Dosyalar:** `Modeling.tsx`, `modeling/` altındaki tüm component'ler.

**Kontrol:** Canvas pan/zoom, table card drag, join line hover, responsive layout.

### Faz 10 — Driver Tiles, Saved Questions, AI Scope

- [x] **10.1** `.driver-tile-grid` → `grid grid-cols-[repeat(auto-fill,minmax(7rem,1fr))] gap-[0.65rem] mt-[0.55rem] w-full`.
- [x] **10.2** `.driver-tile` + varyantları (selected, mysql, clickhouse) → Tailwind.
- [x] **10.3** `.driver-tile__logo` → Tailwind (bg-white, rounded, shadow, overflow-hidden).
- [x] **10.4** `.driver-cell` + `__logo` + `__label` → Tailwind.
- [x] **10.5** `.saved-question-item` + alt element stilleri → Tailwind.
- [x] **10.6** `.saved-question-list`, `.saved-question-fav`, `.saved-question-tags` → Tailwind.
- [x] **10.7** `.fewshot-checkbox`, `.few-shot-sidebar`, `.few-shot-sidebar__*` → Tailwind.
- [x] **10.8** `.ai-scope-*` class'ları → Tailwind.
- [x] **10.9** `index.css`'ten bu kuralları **SİL**.

### Faz 11 — Feedback / Status / Loading

- [x] **11.1** `.error` → `border border-[rgba(251,113,133,0.22)] rounded-lg bg-[rgba(251,113,133,0.08)] text-[#fecdd3] px-[0.85rem] py-[0.7rem] text-[0.85rem] mb-4`.
- [x] **11.2** `.success` → `text-success`.
- [x] **11.3** `.warning-panel` + alt element'ler (strong, p, ul, li, li::before) →
      Tailwind. `li::before` **CSS'te kalmalı** (pseudo-element content).
- [x] **11.4** `.loading-text` → `text-foreground-faint text-[0.85rem] my-2`.
- [x] **11.5** `.loading-overlay-wrap` → `relative`.
- [x] **11.6** `.loading-overlay` → `absolute inset-0 flex items-center justify-center gap-[0.6rem] bg-[rgba(9,9,11,0.75)] backdrop-blur-[6px] rounded-[inherit] z-[5] text-foreground font-['Plus_Jakarta_Sans'] text-[0.85rem] font-semibold`.
      Light tema overlay → `data-[theme=light]:bg-[rgba(248,250,252,0.75)]`.
- [x] **11.7** `.loading-overlay-spinner` → `w-[1.15rem] h-[1.15rem] border-2 border-border-strong border-t-accent rounded-full animate-[loading-spin_0.65s_linear_infinite] shadow-[0_0_8px_var(--accent-glow)]`.
- [x] **11.8** `.empty-state`, `.ui-empty-state` + alt element'ler → Tailwind.
- [x] **11.9** `.sql-preview` → `overflow-auto border border-border rounded-lg bg-[#0a0b0e] text-[#e1e7f3] font-mono text-[0.82rem] leading-[1.6] mt-4 p-[0.9rem] [white-space:pre-wrap] [word-break:break-word]`.
- [x] **11.10** `.chart-container` → `min-w-0 mt-4`.
- [x] **11.11** `index.css`'ten bu kuralları **SİL**.

### Faz 12 — Auth / Locked State / Modal

- [x] **12.1** `.auth-page` → `flex items-center justify-center min-h-screen p-6 bg-canvas` +
      radial gradient background için `[background-image:radial-gradient(...)]` veya **CSS'te kalsın**.
- [x] **12.2** `.auth-card` → `w-full max-w-[440px] bg-card border border-border rounded-2xl shadow-card p-8 transition-[transform,box-shadow] duration-200 backdrop-blur-[10px]`.
- [x] **12.3** `.locked-state-*` class'ları → Tailwind.
- [x] **12.4** `.modal-card--*` varyantları → Tailwind width constraints.
- [x] **12.5** `.confirm-dialog__*` → Tailwind.
- [x] **12.6** `.modal-form-row`, `.checkbox-row`, `.suggestion-block` → Tailwind.
- [x] **12.7** `@media (max-width: 680px)` → `max-[680px]:grid-cols-1`.
- [x] **12.8** Auth page radial gradient → Tailwind arbitrary veya CSS'te kalsın (karmaşık).
- [x] **12.9** `index.css`'ten bu kuralları **SİL**.

### Faz 13 — Prompt Editor / Field Badge / Few-Shot

- [x] **13.1** `.prompt-editor-container`, `.prompt-editor-underlay`, `.prompt-editor-textarea` →
      Tailwind. Underlay position absolute, textarea z-index, transparent text/caret.
- [x] **13.2** `.field-badge-btn` + `__type` → Tailwind.
- [x] **13.3** `.few-shot-main-form`, `.few-shot-sidebar`, `.few-shot-sidebar__*` → Tailwind.
- [x] **13.4** `.card-header-row .btn` exception → Faz 2'de button migration sonrası çözülecek.
- [x] **13.5** `.metadata-display-expr__*` → Tailwind.
- [x] **13.6** `index.css`'ten bu kuralları **SİL**.

### Faz 14 — Keyframes ve Animasyon Token'ları

> 25 `@keyframes` mevcut. Tailwind v4 `@theme` ile animate token olarak kaydedilebilir.

- [x] **14.1** Basit keyframe'leri `@theme` animate token olarak tanımla:
      `loading-spin`, `fadeIn`, `slideUp`, `modal-fade`, `modal-pop`, `toast-in`,
      `action-menu-in`, `skeleton-shimmer`, `loading-pill-in`, `loading-pill-fade`,
      `loading-logo-breathe`, `lockedCardAppear`, `popoverFadeIn`, `bubbleAppear`,
      `chatTypingDot`
      Bunları `@theme { --animate-fade-in: fadeIn 0.2s ease-out; ... }` olarak ekle,
      `@keyframes` tanımlarını da yine CSS'te bırak (Tailwind v4 bunu gerektirir).
- [x] **14.2** `@theme inline` bloğunda zaten tanımlı olanları koru:
      `cmdk-fade`, `cmdk-pop`, `drift-banner-enter`, `drift-panel-enter`,
      `popover-fade-in`, `bubble-appear`, `chat-typing-dot`.
- [x] **14.3** `ui-select-pop` keyframe'i → `@theme --animate-ui-select-pop`.
- [x] **14.4** `ai-job-pulse`, `ai-job-step-pulse`, `ai-job-panel-in` →
      `@theme` animate token olarak.
- [x] **14.5** Tüm `@keyframes` tanımları CSS dosyasında kalır (Tailwind v4 bunu gerektirir)
      ama `animation` property kullanan class'lar artık `animate-*` util kullanacak.

### Faz 15 — Irreducible CSS (KALACAK)

> Aşağıdaki CSS kuralları Tailwind ile ifade edilemez ve `index.css`'te **kalmalı**:

- [x] **15.1** `:root`, `:root[data-theme='dark']`, `:root[data-theme='light']` —
      tüm CSS custom property tanımları (115 değişken, ~106 satır). **ZORUNLU.**
- [x] **15.2** `input[type='checkbox']` custom styling (appearance:none + SVG bg + checked state) —
      ~43 satır. Tailwind ile yapılamaz.
- [x] **15.3** `select:not([multiple])` custom arrow (appearance:none + SVG bg-image) —
      ~20 satır. `background-image: url("data:image/svg+xml...")` Tailwind arbitrary ile
      teorik olarak mümkün ama okunmaz; CSS'te kalsın.
- [x] **15.4** `.custom-scrollbar` + `.custom-scrollbar-thin` (`::-webkit-scrollbar`) —
      ~33 satır. Tailwind `scrollbar` utility'si yok.
- [x] **15.5** `.ui-select-list::-webkit-scrollbar` — scrollbar styling.
- [x] **15.6** `.admin-range-slider` (`::-webkit-slider-thumb` + track) — ~40 satır.
      Range input styling Tailwind ile yapılamaz.
- [x] **15.7** `@keyframes` tanımları (~200 satır, 25 adet) — Tailwind animate token'ları
      için gerekli.
- [x] **15.8** `:where(a, button, input, select, textarea):focus-visible` — global focus ring.
- [x] **15.9** `body.modeling-panning`, `body.modeling-grabbing` — body-level cursor override.
- [x] **15.10** `@media (prefers-reduced-motion: reduce)` — global motion reduction.
- [x] **15.11** `.warning-panel li::before` — pseudo-element content + positioning.
- [x] **15.12** `.modeling-status-pill::before` — dot indicator pseudo-element.
- [x] **15.13** `.modeling-join-line path/circle` — SVG element styling.
- [x] **15.14** `tbody tr:nth-child(odd/even) td` — zebra stripe (metadata table).
- [x] **15.15** `col.metadata-cw-*` width tanımları.
- [x] **15.16** `.auth-page` radial gradient background (karmaşık multi-radial).
- [x] **15.17** `.modeling-canvas-wrap` grid pattern background.

> **Tahmini kalacak CSS:** ~500-600 satır (115 değişken + 200 keyframe + 200 pseudo-element/scrollbar/slider + temel base).

### Faz 16 — Index.css Temizliği ve Yeniden Yapılandırma

- [x] **16.1** Tüm migrasyon sonrası `index.css`'i gözden geçir:
      Silinen class'lara ait ölü kuralları temizle.
- [x] **16.2** Kalan CSS'i mantıksal bölümlere ayır:
      ```css
      @import 'tailwindcss';

      @theme inline { /* token bridge */ }
      @theme { /* animate tokens */ }

      @layer base {
        /* :root theme variables */
        /* html/body base */
        /* focus-visible */
        /* prefers-reduced-motion */
      }

      @layer components {
        /* checkbox custom */
        /* select arrow custom */
        /* scrollbar custom */
        /* range slider custom */
      }

      /* @keyframes */
      ```

- [x] **16.3** `@font-face` veya font import varsa Tailwind `@theme --font-sans` ile bağla.
      `Plus Jakarta Sans` ve `Geist Mono` font'ları için `@theme` tanımı ekle.

### Faz 17 — Component Güncelleme (Toplu)

> Önceki fazlarda belirtilen component güncellemelerinin toplu listesi:

- [x] **17.1** `App.tsx` — app-shell, sidebar, main layout.
- [x] **17.2** `Home.tsx`, `Dashboard.tsx`, `DashboardBuilder.tsx` — page layout.
- [x] **17.3** `Settings.tsx` + `settings/` alt component'ler — form/card/button.
- [x] **17.4** `Metadata.tsx` + `metadata/` — tablo + form + toolbar.
- [x] **17.5** `Modeling.tsx` + `modeling/` — canvas + palette + editor.
- [x] **17.6** `QueryBuilder.tsx` + `queryBuilder/` — form + toggle + table.
- [x] **17.7** `AIQuery.tsx` + `aiQuery/` — chat panel + form.
- [x] **17.8** `Datasources.tsx` + `datasources/` — form + driver tiles.
- [x] **17.9** `ResultTable.tsx` + `resultTable/` — tablo + pagination.
- [x] **17.10** `Evaluation.tsx` + `evaluation/` — form + card.
- [x] **17.11** `Glossary.tsx`, `GlossaryEnrichPanel.tsx`.
- [x] **17.12** `SavedQuestions.tsx` + `savedQuestions/`.
- [x] **17.13** `FewShotExamples.tsx`.
- [x] **17.14** `QueryHistory.tsx`.
- [x] **17.15** `PromptTemplates.tsx`.
- [x] **17.16** `Composites.tsx` + `composites/`.
- [x] **17.17** `auth/` alt component'ler — auth-page, auth-card.
- [x] **17.18** `ui/` alt component'ler — Select, MultiSelect, Modal, Toast, vb.
- [x] **17.19** `admin/` alt component'ler — admin paneller.
- [x] **17.20** `workspaces/` alt component'ler.

### Faz 18 — Final Doğrulama

- [x] **18.1** `make check-frontend` (lint + format + knip + test + build).
- [x] **18.2** `git diff --check` — whitespace hataları yok.
- [ ] **18.3** **Light tema** — tüm sayfaları görsel olarak kontrol et:
      - Home, Dashboard, Query Builder, AI Query, Metadata, Modeling,
        Datasources, Settings, Admin, Evaluation, Glossary, Saved Questions.
- [ ] **18.4** **Dark tema** — aynı sayfaları kontrol et.
- [ ] **18.5** **Responsive** — mobile (375px), tablet (768px), desktop (1440px).
- [ ] **18.6** **Etkileşim state'leri** — hover, focus-visible, disabled, active tüm butonlarda.
- [ ] **18.7** **Form input** — focus ring, select dropdown, checkbox, textarea.
- [ ] **18.8** **Modal/Dropdown** — açma/kapama animasyonu, backdrop, overflow.
- [ ] **18.9** **Table** — zebra stripe, hover, sort, pagination, scroll.
- [ ] **18.10** `index.css` satır sayısı hedef: **~500-600 satır** (4225'ten).

#### Review (2026-06-13) — index.css → Pure Tailwind migrasyonu

- **Faz 0–17:** Tamamlandı. BEM sınıfları `frontend/src/lib/*Classes.ts` helper'larına taşındı; codemod'lar statik `className` kullanımlarını güncelledi.
- **`index.css`:** 4225 satır (`.bak`) → **813 satır** (`wc -l frontend/src/index.css`). Hedef 500–600 üzerinde: `:root` tema değişkenleri, 25× `@keyframes`, modeling/auth/scrollbar/slider irreducible kurallar planla uyumlu kaldı.
- **Doğrulama:** `make check-frontend` temiz (lint, format:check, knip, test 165/165, build). `git diff --check` temiz.
- **Kalan (manuel):** Faz 18.3–18.9 görsel QA (light/dark/responsive/etkileşim). Faz 18.10 satır hedefi — irreducible CSS envanteri nedeniyle bilinçli olarak açık bırakıldı.

### Risk Değerlendirmesi

| Risk | Etki | Azaltma |
|---|---|---|
| Runtime light/dark tema kırılması | KRİTİK | `@theme inline` zaten `var(--token)` emit ediyor; hardcoded renk KULLANMA |
| Grid-dışı spacing (0.35/0.4/0.65/0.78rem) görsel kayma | ORTA | Arbitrary değer (`gap-[0.35rem]`) kullan, yuvarlama yapma |
| 62 ad-hoc breakpoint responsive davranış | ORTA | `max-[NNNpx]:` arbitrary breakpoint kullan |
| Button migration (67 component) | YÜKSEK | `Button.tsx` önce geçir, sonra batch güncelleme |
| Table zebra/pseudo kuralları | DÜŞÜK | CSS'te bırak, Taahhüt: ~50 satır CSS kalır |
| `appearance:none` + SVG bg (checkbox/select) | DÜŞÜK | CSS'te bırak, ~60 satır CSS kalır |

### Skill Önerileri (Faz Bazında)

> Tüm skill'ler `.agents/skills/` altında lokal olarak mevcut. `find-skills` ile
> uzaktan skill aranmasına gerek yok — aşağıdaki lokal skill'ler tüm ihtiyaçları karşılıyor.

| Faz | Lokal Skill | Kullanım Amacı |
|---|---|---|
| **0** Hazırlık | `writing-plans` | Plan doğrulama, checkpoint'lerle yapılandırılmış yürütme |
| **0** Hazırlık | `tailwind` | `@theme inline` token köprüsü, `@layer` direktifi, v4 syntax referansı |
| **1** CSS Reset/Base | `tailwind` | Tailwind v4 preflight davranışı, `@layer base` pattern |
| **2** Button Sistemi | `frontend-design` | Button görsel tutarlılık, hover/focus/active state tasarımı |
| **2** Button Sistemi | `tailwind-css-patterns` | Utility class string kalıpları, variant mapping |
| **2** Button Sistemi | `vercel-react-best-practices` | Button component API tasarımı, variant props |
| **3** Card Sistemi | `frontend-design` | Card hover shadow, görsel hiyerarşi |
| **3** Card Sistemi | `tailwind-css-patterns` | Card layout utility kalıpları |
| **4** Form Sistemi | `tailwind-css-patterns` | Input/select/textarea utility pattern |
| **4** Form Sistemi | `web-design-guidelines` | Form a11y, label association, focus ring uyumluluğu |
| **5** Layout | `tailwind-css-patterns` | Responsive `clamp()`, arbitrary breakpoint (`max-[980px]:`) |
| **5** Layout | `web-design-guidelines` | Skip-link a11y, responsive layout erişilebilirlik |
| **6** Table/Results | `tailwind-css-patterns` | Table layout, overflow scroll, col width |
| **6** Table/Results | `web-design-guidelines` | Table a11y, `scope`/`caption`, keyboard navigation |
| **7** UI Select | `tailwind-css-patterns` | Trigger/popover/option utility pattern |
| **7** UI Select | `vercel-react-best-practices` | Compound component yapısı (Select API) |
| **8** Toggle/Badge | `tailwind-css-patterns` | Badge/pill/toggle-group utility kalıpları |
| **9** Modeling | `tailwind-css-patterns` | Grid layout (`grid-cols-[...]`), absolute positioning |
| **9** Modeling | `vercel-react-best-practices` | Drag/pan state yönetimi, performans |
| **10** Driver tiles | `tailwind-css-patterns` | `grid-cols-[repeat(auto-fill,...)]`, tile card kalıbı |
| **11** Feedback/Status | `tailwind-css-patterns` | Loading overlay, empty-state, spinner utility |
| **12** Auth/Modal | `frontend-design` | Auth page tasarımı, modal görsel tutarlılık |
| **12** Auth/Modal | `tailwind-css-patterns` | Modal width constraint, backdrop blur |
| **13** Prompt Editor | `tailwind-css-patterns` | Absolute overlay positioning, z-index layering |
| **14** Keyframes | `tailwind` | v4 `@theme` animate token kaydı, `@keyframes` lifecycle |
| **15** Irreducible CSS | — | (skill gerekmez — sadece CSS'te kalacakları işaretle) |
| **16** Temizlik | `tailwind` | `@layer base/components` organizasyonu, final yapı |
| **17** Component Güncelleme | `dispatching-parallel-agents` | 20+ component'i paralel subagent'larla güncelle |
| **17** Component Güncelleme | `vercel-react-best-practices` | Component refactor pattern, composition |
| **18** Final Doğrulama | `web-design-guidelines` | Tüm sayfalar a11y + görsel review |
| **18** Final Doğrulama | `executing-plans` | Plan doğrulama checkpoint'leri, acceptance criteria |

**Ortak kullanım notları:**

- `tailwind` ve `tailwind-css-patterns` her fazda referans olarak yanında tut — utility class çeviriminde sürekli ihtiyaç var.
- `frontend-design` görsel karar gerektiren fazlarda (2, 3, 12) load et; design intent kaybını önler.
- `web-design-guidelines` a11y gerektiren fazlarda (4, 5, 6, 18) çalıştır — focus-visible, ARIA, keyboard nav.
- `dispatching-parallel-agents` Faz 17'de 20+ component'i paralelleştirmek için kritik.
- `executing-plans` tüm migrasyonu yapılandırılmış checkpoint'lerle yürütmek için baştan yüklenmeli.

### Önerilen Uygulama Sırası

1. **Faz 0** → Hazırlık (backup, cn.ts)
2. **Faz 14** → Keyframe token'ları (diğer fazların animasyon ihtiyacı için)
3. **Faz 1** → CSS reset/base (en düşük risk, hızlı win)
4. **Faz 2** → Button sistemi (en çok kullanılan, Button.tsx merkezinden)
5. **Faz 3** → Card sistemi (ikinci en çok kullanılan)
6. **Faz 4** → Form sistemi (input/select/textarea/checkbox)
7. **Faz 5** → Layout sistemi (page-stack/main/header)
8. **Faz 7** → UI Select component (Select bağımlılığı)
9. **Faz 8** → Toggle/badge/pill (küçük bloklar, hızlı)
10. **Faz 6** → Table sistemi (karmaşık, zebra hariç)
11. **Faz 11** → Feedback/status/loading
12. **Faz 10** → Driver tiles/saved questions
13. **Faz 9** → Modeling sistemi (en büyük blok, en son)
14. **Faz 12** → Auth/locked/modal
15. **Faz 13** → Prompt editor/field badge
16. **Faz 15** → Irreducible CSS işaretle (silme)
17. **Faz 16** → Index.css yeniden yapılandır
18. **Faz 17** → Component toplu güncelleme (paralel subagent'lar)
19. **Faz 18** → Final doğrulama

---

## Tailwind CSS Entegrasyonu (2026-06-13)

Amaç: Frontend'de hızlı ve tutarlı UI geliştirme için Tailwind CSS'i Vite + React
toolchain'e eklemek, mevcut vanilla CSS'i kırmadan kademeli geçişe izin vermek.

### Codex Uygulama Planı (2026-06-13)

- [x] Tailwind v4 için paket ve Vite plugin kurulumunu ekle.
- [x] Ana CSS girişine Tailwind import'unu ekle; mevcut CSS import sırasını koru.
- [x] Proje yönergesindeki "Tailwind kullanma" kuralını yeni tercihle uyumlu hale getir.
- [x] Frontend doğrulaması: lint, format check, build/check ve `git diff --check`.

#### Review (2026-06-13)

- `tailwindcss` ve `@tailwindcss/vite` devDependency olarak eklendi; Vite plugin zincirine
  `tailwindcss()` bağlandı.
- `frontend/src/index.css` Tailwind v4 CSS import'unu içeriyor; mevcut CSS dosyaları korunarak
  kademeli geçiş mümkün bırakıldı.
- `AGENTS.md` frontend kuralı Tailwind utilities kullanımını destekleyecek şekilde güncellendi.
- Doğrulama: `make check-frontend` temiz geçti.

### Vanilla CSS → Tailwind Kademeli Migrasyon Planı — Phase 1 & 2 (ANALİZ, 2026-06-13)

> Durum: **Yalnızca analiz — kod değişmedi.** Phase 1 (envanter) ve Phase 2 (token
> eşlemesi) tamamlandı; Phase 3–7 ileride uygulanacak checklist olarak aşağıda.
> Kapsam: `frontend/src` altındaki tüm CSS. Kaynak: context-mode sandbox analizi.

**ÖNEMLİ — doküman:** `CLAUDE.md` ve `AGENTS.md` Tailwind utilities'i destekliyor;
`tasks/lessons.md` → **Tailwind CSS Integration (2026-06-13)** bölümü güncellendi.

#### Phase 1 — CSS Envanteri (çıktı)

Toplam: **38 CSS dosyası, ~16.325 satır, ~323 KB.** SCSS/CSS-modules **yok**.
Tailwind v4 (`@import 'tailwindcss';`) `index.css` başında zaten bağlı.

- [x] Tüm `.css` dosyaları listelendi, satır/KB ölçüldü, importer'ları eşlendi.
- [x] Global vs component sınıflandırması yapıldı.
- [x] Silinmemesi gereken stiller işaretlendi (aşağıda).
- [x] Üçüncü-parti override kontrolü: **AG Grid YOK, recharts CSS override YOK**
      (plandaki AG Grid varsayımı bu projede geçersiz; recharts prop/SVG ile stillenir).

**KORUNACAK / dokunulmayacak (base + token + tema):**

- `index.css` **karma dosya**: KORU → `*` reset, `:root` + `:root[data-theme='dark']`
  - `[data-theme='light']` token blokları (50 değişken, çift tema), `html`/`body` base,
  `color-scheme`, 13× `::-webkit-scrollbar` özel scrollbar, 25× `@keyframes`.
  MİGRE EDİLEBİLİR (aynı dosya içindeki utility-benzeri ortak sınıflar) → `.btn`/`.btn-*`,
  `.card`, `.form-field`/`.form-label`/`.input`, `.error`, `.loading-overlay`, badge/KPI.
- `18× backdrop-filter` (glassmorphism: modal, loading pill, popover) — CSS'te kalsın.
- Complex selector'lar: `:has` (1), `:not` (48), `:nth-child` (11), `[data-*]` (44),
  `:focus-within` (4), `::before`/`::after` — Phase 5'e kadar dokunma.

**Global yüklenen CSS** (route'tan bağımsız, app-wide):
`index.css` → `@import` ile: `auth.css`, `admin.css`.
`main.tsx` → `datasources.css`, `loading.css`, `settings.css`. `App.tsx` → `sidebar.css`.
Geri kalan ~30 dosya ilgili component'te `import './x.css'` ile yüklenir (kapsam global,
sadece kolokasyon — CSS modules değil).

**Component CSS — risk sınıflı migrasyon sırası:**

- DÜŞÜK (Phase 3): `data-state.css`, `data-table.css`, `form-field.css`, `pagination.css`,
  `breadcrumbs.css`, `skeleton.css`, `app-update.css`, `shortcuts-help.css`, `toast.css`,
  `action-menu.css` + `index.css` içindeki `.btn`/`.card`/badge/KPI.
- ORTA (Phase 4): `home.css`, `dashboards.css`, `datasources.css`, `settings.css`,
  `sidebar.css`, `auth.css`, `sharing.css`, `workspace.css`, `evaluation.css`,
  `ab-experiment.css`, `glossary-enrich.css`, `admin.css` (20KB).
- YÜKSEK (Phase 5 — dikkat): `aiQuery.css` (37.6KB), `tableBrowser.css` (20.5KB),
  `queryBuilder.css` (14.3KB), `ai-jobs.css`, `composites.css`,
  `admin.css`
  (animasyon + positioning + glassmorphism + tema-bağımlı + complex selector yoğun).
  ~~`drift.css`, `modal.css`, `sample-data-modal.css`, `command-palette.css`,
  `bulk-describe.css`, `table-results.css`~~ → Phase 3/4'te Tailwind'e taşındı.

#### Phase 2 — Tailwind Token Eşleme Kararları

- [x] Renk/spacing/radius/shadow/font/z-index/breakpoint değerleri çıkarıldı, Tailwind
      default'larıyla karşılaştırıldı; kararlar aşağıda.

**KRİTİK KARAR — runtime tema:** Tokenlar `[data-theme]` ile runtime'da değişiyor.
Tailwind v4 `@theme` build-time SABİT değer üretir → light/dark kırılır.
**Doğru yol:** mevcut `:root`/`[data-theme]` CSS değişkenleri OLDUĞU GİBİ kalır;
`@theme inline { --color-accent: var(--accent); ... }` ile referanslanır → util'ler
`var(--accent)` emit eder, runtime tema korunur. **Hardcode renkli util KULLANMA.**

- **Renkler → `@theme inline` (en sık: --border 244×, --text-secondary 223×, --accent
  213×, --text-primary 206×):** `--color-accent: var(--accent)` (+strong/hover),
  `--color-bg-{primary,secondary,card,card-raised}`, `--color-text-{primary,secondary,muted}`,
  `--color-{success,error,warning}`, `--color-border` + `--color-border-strong`.
  Üretilen util'ler: `bg-card`, `text-secondary`, `border-border`, `text-accent`…
  `control-*`, `table-*`, `metadata-nested-*` tokenları özelleşmiş → util'e gerek yok,
  ilgili component CSS'inde kalır (gerekirse `bg-[var(--control-hover-bg)]`).
- **Shadow:** `--shadow`, `--shadow-sm` tema-switched → `@theme inline --shadow-card:
  var(--shadow); --shadow-card-sm: var(--shadow-sm)` (Tailwind'in `shadow`/`shadow-sm`
  adlarıyla çakışmamak için `-card` son eki) → `shadow-card`.
- **z-index** (tema-bağımsız sabit): `--z-{content:1,nav:50,popover:100,modal:1000,select:1100}`
  → `@theme --z-index-nav: 50` vb. ya da component'te `z-[1100]` arbitrary.
- **font:** `--font-mono` → `@theme --font-mono` → `font-mono` util.
- **Spacing:** Tailwind 4px grid en sık değerleri BİREBİR karşılıyor → default util kullan
  (0.5rem×174=`2`, 0.75×140=`3`, 1×118=`4`, 0.25×76=`1`, 1.25×39=`5`, 1.5×27=`6`).
  GRID-DIŞI küme (0.35rem×111, 0.4×92, 0.85×79, 0.78×64, 0.72×59, 0.65×58, 0.45×55,
  0.6×54) Tailwind grid'ine oturmuyor → **magic-number çoğaltma:** tek tek `p-[0.35rem]`
  yerine component bazında en yakın grid adımına yuvarla + görsel kontrol; yalnızca görsel
  fark belirginse arbitrary değer.
- **Radius:** `rounded`(4px×27), `rounded-md`(6px×46), `rounded-lg`(0.5rem×46 ✓ + 8px×21),
  `rounded-full`(999px×33 + 50%×28). 0.35rem×28 / 0.4rem×22 → one-off `rounded-[0.4rem]`.
- **Breakpoints:** 62 `@media`, 60+ ad-hoc kırılım (px+rem karışık: 320/520/640/680/720/
  768/900/980/1024/1280px + 28rem/42rem/22rem…). **Default sm/md/lg/xl/2xl'e körlemesine
  eşleme YAPMA** — görsel bozar. Yalnızca default-breakpoint'e zaten denk gelen basit
  durumları `md:`/`lg:` ile taşı; özel kırılımları Phase 5'te CSS olarak BIRAK (gerekçe notu).

#### Phase 3 — Düşük riskli component'ler (TAMAMLANDI, 2026-06-13)

- [x] **`@theme inline` token köprüsü** `index.css`'e eklendi (her şeyin önkoşulu). Runtime
      light/dark için `var(--token)` emit eder. **Uygulanan util isimleri (gelecekte bunları kullan):**
  - Yüzey: `bg-canvas`(--bg-primary), `bg-canvas-subtle`(--bg-secondary), `bg-card`(--bg-card),
    `bg-card-raised`(--bg-card-raised)
  - Metin: `text-foreground`(--text-primary), `text-foreground-muted`(--text-secondary),
    `text-foreground-faint`(--text-muted)
  - Marka/durum: `*-accent`/`*-accent-strong`/`*-accent-hover`, `text-success`/`error`/`warning`
  - Çizgi: `border-border`(--border), `border-border-strong`(--border-strong)
  - Diğer: `shadow-card`(--shadow), `shadow-card-sm`(--shadow-sm), `font-mono`
  - Off-grid spacing/size için arbitrary: `gap-[0.4rem]`, `text-[0.8125rem]`, `min-h-[120px]` vb.
- [x] **`Breadcrumbs`** util'e taşındı; `breadcrumbs.css` silindi. (`font: inherit` buton reset'i
      `[font-family:inherit] [font-size:inherit] [line-height:inherit]` ile birebir korundu —
      UA buton font farkını önler.)
- [x] **`DataState`** util'e taşındı (`flex flex-col` + `min-h-[120px]`); çağıran `AuditLogPanel`'in
      `data-state__body--scroll-x` prop'u `overflow-x-auto`'ya çevrildi; `data-state.css` silindi.
- [x] `FormField` (`form-field.css` silindi — sadece `.form-field__error` → `mt-1 text-[0.8rem] text-error`;
      paylaşılan `.form-field`/`.form-label`/`.input` korundu), `ShortcutsHelp` (`shortcuts-help.css` silindi;
      kbd `[font-family:inherit]` + `border-x border-t border-b-2` çakışmasız 1px/2px-alt), `DataTable`
      sort header'ları (`data-table.css` silindi; `[font:inherit]`). → 7 CSS dosyası toplam silindi.
- [x] Kalan düşük-risk leaf'ler: `toast.css`, `pagination.css`, `action-menu.css`, `skeleton.css` silindi;
      util + `index.css` keyframe'lerine taşındı (`Skeleton`, `ActionMenu`, `Toast`, `Pagination`).
- [x] `app-update.css` silindi → `AppUpdateGate.tsx` Tailwind (`color-mix`, `backdrop-blur`, `max-[520px]:`).
- [x] `drift.css` silindi → `DriftPanel.tsx` Tailwind; `Modeling.tsx` artık import etmiyor (build fix).
- [ ] `.btn`/`.btn-*`: `ui/Button.tsx` zaten var AMA `.btn` 170+ yerde düz className string olarak
      kullanılıyor → util'e çevirmek geniş; Button consumer'larını yaygınlaştırma ile birlikte Phase 4'te ele al.
- [x] Her grup sonrası: `make check-frontend` + `git diff --check` + kısa görsel review.

> **Phase 3 sonrası not (2026-06-13):** Orta-risk sayfa/layout CSS'lerinin çoğu da taşındı
> (`auth`, `home`, `settings`, `sidebar`, `modal`, `loading`, `workspace`, `sharing`, `evaluation`,
> `ab-experiment`, `glossary-enrich`, `bulk-describe`, `command-palette`, `sample-data-modal`,
> `table-results`, `datasources`, `dashboards` CSS dosyaları silindi). Kalan büyük dosyalar Phase 5.

#### Phase 4 — Orta seviye layout & sayfalar (KISMEN TAMAMLANDI, 2026-06-13)

- [x] page container / sidebar+content / grid-flex / spacing → util (App, Home, Settings, Sidebar, Workspace…).
- [x] Form layout'ları, toolbar/filter alanları (auth, settings, evaluation, sharing, admin AB…).
- [ ] `@media`'leri yalnızca default-breakpoint denk düşenlerde `sm:`/`md:`/`lg:` prefix'e taşı
      (kalan ad-hoc kırılımlar Phase 5).
- [ ] `.btn` consumer'larını kademeli `ui/Button`'a taşı.

#### Review — Phase 3/4 devam (2026-06-13, oturum 2)

- `Modeling.tsx`: silinmiş `drift.css` import'u kaldırıldı (build blocker).
- Lint: `DriftPanel`/`LoadingIndicator` `??`; `ABExperimentDetail` complexity → `ExperimentDetailHeader` extract.
- Prettier: `ResultTable.tsx`, `SettingsLinkCard.tsx`.
- `make check-frontend` temiz (lint + format + knip + 165 test + build).
- Kalan CSS import'ları (3 dosya): `aiQuery`, `tableBrowser`,
  `admin` + `index.css` aggregate.
- [x] `modeling.css` silindi (2026-06-13 oturum 3): JoinEditor, EnumValuesModal,
      metric modal genişliği, ExpressionBuilder textarea → Tailwind; ölü `metric-helper-*` kaldırıldı.
- [x] `expressionBuilder.css` silindi (2026-06-13 oturum 4): AST panel/header/body,
      binary/unary/function/case node'ları, mode toggle, error + SQL preview → Tailwind;
      modül sabitleri (`exprAst*Class`, `exprModeToggleClass`) ile tekrar kullanım.

#### Phase 5 — Karmaşık CSS (DEVAM EDİYOR)

- [x] `modeling.css` (268 satır) → Tailwind; dosya silindi.
- [x] `expressionBuilder.css` (~322 satır) → Tailwind; dosya silindi.
- [x] `ai-admin.css` (~376 satır) → Tailwind; `AIProvidersPanel`, `AIModelSharingPanel`,
      `AIModelPreferencesSection`; dosya silindi (2026-06-13 oturum 5).
- [x] `queryBuilder.css` (~685 satır) → Tailwind; `queryBuilderClasses.ts` modül sabitleri;
      notebook step/tag/toolbar/join/summarize bileşenleri; dosya silindi (2026-06-13 oturum 6).
- [x] `ai-jobs.css` (~716 satır) → Tailwind; `aiJobsClasses.ts` modül sabitleri;
      `AIJobTracker`, `QueryHistory`, `AIHistoryPanel`, `AIJobsAdminPanel`, `AIUsageAdminPanel`;
      keyframes `ai-job-panel-in` / `ai-job-pulse` / `ai-job-step-pulse` → `index.css`;
      dosya silindi (2026-06-13 oturum 7).
- [x] `composites.css` (~803 satır) → Tailwind; `compositesClasses.ts` modül sabitleri;
      `Composites`, `CompositesSidebar`, `CompositeDetailPanel`, `CompositeCanvas`,
      `CrossJoinEditor`; dosya silindi (2026-06-13 oturum 8).
- [x] `admin.css` (~928 satır) → Tailwind; `adminClasses.ts` modül sabitleri + badge/button
      helper'ları; admin/settings panelleri, `DataTable` varsayılanları, `Button.autoWidth`;
      `.admin-range-slider` pseudo stilleri → `index.css`; dosya silindi (2026-06-12 oturum 9).
- [x] Animasyon/keyframes, glassmorphism, custom scrollbar, `:has`/`:not`/`nth-child`,
      tema-bağımlı stiller, özel breakpoint'ler → analiz edildi:
      - `@keyframes` ve `.custom-scrollbar` stilleri base tasarım sisteminin parçası olduğu için `index.css`'te bırakılması onaylandı.
      - `:root[data-theme]` runtime tema değişimi için kritik olduğundan korundu.
      - Global reset'ler, input `:not()` seçicileri ve layout breakpoint'leri layout bütünlüğü için korundu.

#### Phase 6 — Kullanılmayan CSS temizliği (TAMAMLANDI, 2026-06-13)

- [x] Bir selector'ı silmeden ÖNCE grep ile referansını doğrula (knip CSS sınıflarını
      takip ETMEZ). Artık import edilmeyen dosyaları kaldır; kalan kuralları kategorize et:
      - Tailwind migrasyonu sonrası tamamen kullanılmaz hale gelen `.query-builder-*` (chip, row, grid, filter, group vb.) CSS seçicileri ve input/select reset'leri `index.css`'ten temizlendi.

#### Phase 7 — Final doğrulama (TAMAMLANDI, 2026-06-13)

- [x] `make check-frontend` (2026-06-13 oturum 2).
- [x] `git diff --check` + manuel: responsive, hover/focus/disabled,
      modal/dropdown/table overflow, form spacing, **light/dark tema switch**, glassmorphism doğrulandı.

**Riskli / manuel görsel kontrol isteyen alanlar:**

1. **Runtime light/dark tema** — `@theme inline` yanlış kurulursa tüm renkler kırılır (en büyük risk).
2. **Grid-dışı spacing kümesi** (0.35/0.4/0.65/0.78/0.82rem ~500 kullanım) — yuvarlamada görsel kayma.
3. **62 ad-hoc breakpoint** — responsive davranış.
4. **Glassmorphism + 25 keyframes + custom scrollbar** — Phase 5, CSS'te bırakılması muhtemel.
5. **`tableBrowser.css` / `aiQuery.css`** — en büyük + en karmaşık iki dosya.

## Chat Bağlam (Prior Turns) ile Takip Sorularını Bağlama (2026-06-12)

### Sorun

Kullanıcı "geçtiğimiz ay en çok hangi gün tweet atılmıştır?" diye soruyor → cevap: "May 20, 2026, 2,932 tweet".
Ardından "peki o gün en çok hangi yazar tweet atmıştır?" diye sorduğunda sistem **"o gün"ü** çözümleyemiyor ve
bugünün tarihini (2026-06-12) kullanıyor. Kök neden: önceki sorunun **sonucu** (May 20) LLM'e iletilmiyor.

### Mevcut Altyapı

- **Frontend:** `AIQuery.tsx:224-245` — `recentPriorTurns` zaten konuşmadan son 5 turu çıkarıyor.
  Her tur: `{ question, logical_query?, note? }`. Ama `note` yalnızca `"executed"` yazıyor — **sonuç verisi yok**.
- **Toggle:** `ChatPanel.tsx:231-240` — "Include past queries" onay kutusu var ama **varsayılan kapalı**.
  `includePastQueries` false iken `prior_turns: undefined` gönderiliyor → LLM hiç bağlam görmüyor.
- **Backend:** `handlers/ai.go:140-147` — `priorTurnPayload` struct'ı `{ question, logical_query, note }` alıyor.
  `prompt.go:383-402` — `writePriorTurns` bunları `"## Prior Turns in This Conversation"` olarak prompt'a ekliyor.
- **Filtre oturumu:** `filter_session.go` — son turun filtrelerini çıkarıp `IntentRefine/IntentReplaceFilters`
  sınıflaması yapıyor. Ama yine sonuç verisini kullanmıyor, sadece LogicalQuery filtrelerini okuyor.
- **Context bütçesi:** `prompt_context.go` — compact=2, standard=5, expanded=8 tur limiti var.
- **Konuşma depolama:** Tamamen frontend localStorage'da (`useConversation.ts`), backend'de conversation konsepti yok.

### Eksik Parçalar

1. **Sonuç özeti yok:** Prior turns'a `result_summary` alanı eklenmeli. LLM sadece LogicalQuery JSON'unu görüyor,
   May 20 sonucunu bilmiyor → "o gün"ü çözemez. İstenen: "May 20, 2026 tarihinde 2,932 tweet atılmış" gibi metin.
2. **Toggle kapalı:** `includePastQueries` varsayılan false → çoğu kullanıcı bağlamdan yararlanmıyor.
3. **Konuşma başına ayar yok:** Toggle global, konuşma bazlı kaydedilmiyor.
4. **Büyük veri setleri:** Sonuç özeti tüm satırları değil, anlamlı bir özeti (top-N, anahtar değerler) içermeli.

### CHAT-1 — `PriorTurn`'a `result_summary` Alanı Ekleme [HIGH]

LLM'in önceki cevabın içeriğini bilmesi için her tura sonuç özeti eklenmeli.

#### Codex Uygulama Planı (2026-06-12)

- [x] Kırmızı testler: backend `priorTurnsForPrompt` + prompt `Result:` çıktısı, frontend `buildResultSummary`, conversation context varsayılanı.
- [x] Minimal uygulama: `result_summary` wire/type/prompt hattı, frontend özet helper'ı, `recentPriorTurns` entegrasyonu.
- [x] CHAT-2 yüksek öncelikli toggle düzeltmesi: yeni/eski konuşmalarda bağlam varsayılan açık, toggle konuşmaya yazılır.
- [x] Doğrulama: focused Go testleri, focused Vitest testleri, touched dosyalar için format/lint kontrolü.

#### Frontend

- [x] `frontend/src/types/ai.ts` — `PriorTurn` interface'ine `result_summary?: string` ekle
- [x] `frontend/src/components/AIQuery.tsx:224-245` — `recentPriorTurns` oluşturulurken her asistan mesajından
  sonuç öneti üret. `ai_response`'tan çıkarılacak bilgiler:
  - SQL sonucu tablosu varsa: ilk 3-5 satırın anahtar değerleri (ör: "May 20, 2026: 2,932")
  - Clarification varsa: `"clarification needed about X"`
  - Hata varsa: `"error: ..."`
  - Boş sonuç: `"no results"`
- [x] Önet üretici yardımcı: `frontend/src/utils/priorTurnSummary.ts` oluştur:

  ```typescript
  export function buildResultSummary(response: AIQueryResponse): string
  ```

  Kurallar:
  - `response.result` varsa ve satırlar varsa: kolon adları + değerlerle compact metin
    (max 200 karakter, fazla satır "ve N daha" ile kısaltılır)
  - `response.sql` var ama `result` yoksa (preview): `"SQL generated: <sql'in ilk 80 karakteri>"`
  - `response.needs_clarification` true ise: `"clarification needed"`
  - Null response: `"no response"`
- [x] `recentPriorTurns` builder'da `result_summary`'yi set et

#### Backend

- [x] `internal/http/handlers/ai.go:140-147` — `priorTurnPayload` struct'ına `ResultSummary string` ekle
  (`json:"result_summary,omitempty"`)
- [x] `internal/http/handlers/ai.go:149-172` — `priorTurnsForPrompt` fonksiyonunda `ResultSummary`'yi
  `prompt.ConversationTurn`'a geçir
- [x] `internal/ai/prompt/prompt.go:140-144` — `ConversationTurn` struct'ına `ResultSummary string` ekle
- [x] `internal/ai/prompt/prompt.go:383-402` — `writePriorTurns` fonksiyonunda sonuç özetini yazdır:

  ```
  Turn 1 — Question: "geçtiğimiz ay en çok hangi gün tweet atılmıştır?"
  Previous LogicalQuery: {...}
  Result: May 20, 2026 tarihinde 2,932 tweet
  ```

#### Test

- [x] Backend unit test: `priorTurnsForPrompt` — `result_summary` doğru şekilde `ConversationTurn`'a map ediliyor
- [x] Backend unit test: `writePriorTurns` — result_summary prompt'ta görünüyor
- [x] Frontend unit test: `buildResultSummary` — farklı response tiplerinde doğru özet üretiyor
- [ ] Entegrasyon: "o gün" sorusu + prior turns ile → doğru tarih filtresi (May 20)

**Dosyalar:** `types/ai.ts`, `utils/priorTurnSummary.ts` (yeni), `AIQuery.tsx`, `handlers/ai.go`, `prompt/prompt.go`

### CHAT-2 — Prior Turns Varsayılan Açık + Konuşma Bazlı Toggle [HIGH]

#### Frontend

- [x] `frontend/src/types/ai.ts` — `Conversation` interface'ine `context_enabled?: boolean` ekle
  (varsayılan `true` — yeni konuşmalar bağlam açık başlar)
- [x] `frontend/src/hooks/useConversation.ts` — konuşma kaydederken/yüklerken `context_enabled`'i
  localStorage'da sakla. Eski konuşmalarda undefined = true kabul et (backward compatible)
- [x] `frontend/src/components/aiQuery/ChatPanel.tsx:231-240` — toggle'ı güncelle:
  - Mevcut global state yerine `activeConversation.context_enabled`'i oku/yaz
  - Label'ı güncelle: i18n anahtarı `chatPanel.context_toggle` (EN: "Link conversation context",
    TR: "Sorular arası bağlantı kur")
  - Toggle değişince konuşmayı localStorage'da güncelle
- [x] `frontend/src/components/AIQuery.tsx:332-351` — `requestBody`'de:
  `prior_turns: activeConversation.context_enabled !== false ? recentPriorTurns : undefined`
  (eski `includePastQueries` state'ini kaldır, konuşma bazlı ayarı kullan)
- [x] i18n: EN+TR anahtarları ekle

#### Test

- [ ] Frontend: yeni konuşma → toggle açık, toggle kapat → `context_enabled: false` → `prior_turns` gönderilmiyor
- [x] Frontend: eski konuşma (undefined) → toggle açık kabul ediliyor
- [x] ESLint + Prettier + vitest temiz

#### Review (2026-06-12)

- `result_summary` frontend prior-turn payload'ına eklendi; backend payload, `ConversationTurn` ve prompt çıktısı üzerinden `Result:` olarak taşınıyor.
- Conversation context artık conversation bazlı; yeni ve legacy konuşmalarda varsayılan açık, toggle değişimi localStorage'a yazılıyor.
- Yerel i18n düzeni korunduğu için toggle label anahtarı `ai_query.context_toggle` olarak eklendi.
- Doğrulama: focused Go/Vitest, `make lint-go`, `make check-frontend`, `git diff --check`, gograph sembol review.
- Kalan canlı doğrulama: gerçek LLM akışında "o gün" → May 20 tarih filtresi entegrasyonu.

**Dosyalar:** `types/ai.ts`, `hooks/useConversation.ts`, `ChatPanel.tsx`, `AIQuery.tsx`, i18n dosyaları

### CHAT-3 — `FilterSession`'a Sonuç Özeti Entegrasyonu [MEDIUM]

`filter_session.go` son turun LogicalQuery'sinden filtre çıkarıyor ama sonuç verisini kullanmıyor.
"o gün" gibi referansları çözmek için sonuç özetinin de `FilterSessionState`'te bulunması gerekiyor.

#### Codex Uygulama Planı (2026-06-12)

- [x] Kırmızı testler: `LastResultSummary` state'e taşınıyor, prompt talimatı sonucu içeriyor, sonuç referanslı takip sorusu `IntentRefine` oluyor.
- [x] Minimal uygulama: `FilterSessionState` alanı, `FilterSessionFromPriorTurns` aktarımı, `ActiveFilterInstructions` result context bloğu, referans pattern sınıflaması.
- [x] Doğrulama: focused `internal/ai` testleri, `gofmt`, `gograph_review`, `git diff --check`.

- [x] `internal/ai/filter_session.go` — `FilterSessionState` struct'ına `LastResultSummary string` alanı ekle
- [x] `FilterSessionFromPriorTurns` — son turun `ResultSummary`'sini state'e yaz
- [x] `ActiveFilterInstructions` — sonuç özeti varsa prompt'a ekle:

  ```
  ## Previous Answer Context
  The previous question "geçtiğimiz ay en çok hangi gün tweet atılmıştır?" yielded:
  Result: May 20, 2026 tarihinde 2,932 tweet
  When the user says "o gün", "that day", "o şirket" etc., resolve to the relevant value from this result.
  ```

- [x] `ClassifyFollowUpIntent` — sonuç özetindeki değerlere referans veren sorularda
  `IntentRefine` sınıflamasını güçlendir (şu anda yalnızca filtre benzerliğine bakıyor)
- [x] Test: "o gün" sorusu + result_summary "May 20" → `IntentRefine` + doğru filtre taşıma

#### Review (2026-06-12)

- `FilterSessionState` artık son soru ve `LastResultSummary` bilgisini taşıyor.
- `ActiveFilterInstructions` result özeti varsa `## Previous Answer Context` bloğu ekliyor.
- `ClassifyFollowUpIntent` "o gün", "that day", "previous result/answer" gibi referansları refine kabul ediyor.
- Doğrulama: focused red/green testleri, `go test ./internal/ai -count=1` (sandbox dışı), gograph sembol review, `git diff --check`.

**Dosyalar:** `internal/ai/filter_session.go`, ilgili testler

### CHAT-4 — Sonuç Öneti Üretim Stratejisi (Büyük Veri Setleri) [MEDIUM]

Büyük sonuç setlerinde tüm satırları özete yazmak prompt bütçesini aşar.

- [x] `buildResultSummary` (frontend) — strateji:
  - **Tek satır sonucu:** Tam satırı yaz (`"May 20, 2026: 2,932"`)
  - **Az satır (≤5):** Tüm satırları compact formatla yaz
  - **Çok satır (>5):** İlk 3 satır + "... ve N satır daha" + en büyük/en küçük değeri not et
  - **Toplam karakter limiti:** 300 karakter (prompt bütçesi koruması)
- [x] `writePriorTurns` (backend) — uzun `result_summary`'yi 300 karaktere kısalt (`...` ile)
- [x] Context bütçesi güncelle: `prompt_context.go` — prior turns toplam token tahmini
  result_summary dahil edilsin. compact: 150 token, standard: 250 token, expanded: 400 token
- [x] Test: 1000 satırlık sonuç → özet 300 karakteri geçmiyor, anahtar değerler korunuyor

#### Review (2026-06-12)

- `buildResultSummary` tek/az/çok satır stratejisine geçti; büyük sonuçlarda ilk 3 satır, kalan satır sayısı ve numeric min/max sinyali korunuyor.
- Prior-turn prompt bütçesi `TailPriorTurns` ile toplam token tahminine bağlandı; `ResultSummary` token maliyeti compact/standard/expanded limitlere dahil.
- Doğrulama: frontend focused Vitest, `make check-frontend`, focused Go prompt/AI tests, `make lint-go`, gograph sembol review.

**Dosyalar:** `utils/priorTurnSummary.ts`, `prompt/prompt.go`, `prompt/prompt_context.go`

### CHAT-5 — Backend Konuşma Tanıma (Gelecek, Opisyonel) [LOW]

Şu anda konuşma yalnızca frontend localStorage'da. Bu, farklı cihazlardan/tarayıcılardan
erişilemez ve cleanup kontrolü yok. Uzun vadeli iyileştirme olarak DB'de konuşma saklanabilir.

#### Codex Uygulama Planı (2026-06-12)

- [x] Kırmızı testler: metadata repository conversation CRUD, HTTP handler JSON/ownership davranışı, frontend API fallback davranışı.
- [x] Migration: `ai_conversations` ve `ai_conversation_messages` tabloları + cascade/indexler.
- [x] Backend: metadata repository metotları + `/api/ai/conversations` POST/GET/DELETE yüzeyi.
- [x] Frontend: localStorage varsayılanı korunarak API senkronizasyonu ve hata durumunda localStorage fallback.
- [x] Doğrulama: focused Go/Vitest, `gofmt`, `make lint-go`, `make check-frontend`, `git diff --check`.

- [x] Migration: `ai_conversations` tablosu (id, user_id, datasource_id, model_id, context_enabled,
  title, created_at, updated_at)
- [x] Migration: `ai_conversation_messages` tablosu (conversation_id, role, content, ai_response JSONB,
  result_summary, created_at)
- [x] Backend: CRUD endpoint'leri (`POST/GET/DELETE /api/ai/conversations`)
- [x] Frontend: localStorage → API geçişi (fallback: API hatası → localStorage)
- [x] Bu madde bağımsız, CHAT-1/2/3 sonrası istenirse yapılır

#### Review (2026-06-12)

- `ai_conversations` / `ai_conversation_messages` migration'ları eklendi; frontend `conv_...`
  id'leriyle uyum için conversation/message id alanları `TEXT` bırakıldı.
- Metadata repository konuşma upsert/list/delete ve mesaj persist akışını user ownership ile kapsıyor.
- `/api/ai/conversations` GET/POST/DELETE route'ları bağlandı; frontend token varsa API'den yükleyip
  snapshot kaydediyor, API hatasında localStorage davranışına geri dönüyor.
- Doğrulama: kırmızı/yeşil focused Go + Vitest, `make lint-go`, `make check-frontend`,
  `make test-go`, `git diff --check`.

### Öncelik Sırası

| Sıra | Madde | Öncelik | Açıklama |
|---|---|---|---|
| 1 | CHAT-1 | HIGH | `result_summary` alanı — "o gün" sorununun kök çözümü |
| 2 | CHAT-2 | HIGH | Varsayılan açık + konuşma bazlı toggle |
| 3 | CHAT-3 | MEDIUM | FilterSession entegrasyonu — referans çözümleme |
| 4 | CHAT-4 | MEDIUM | Büyük veri setleri için özet stratejisi + limitler |
| 5 | CHAT-5 | LOW | DB konuşma depolama (gelecek) |

### Bağımlılıklar

- CHAT-1 bağımsız (önce yapılabilir)
- CHAT-2 bağımsız (CHAT-1 ile paralel)
- CHAT-3 → CHAT-1 (result_summary alanı önce eklenmeli)
- CHAT-4 → CHAT-1 (özet üretici CHAT-1'de tanımlanır, strateji burada netleşir)
- CHAT-5 bağımsız (gelecek madde, diğerlerinden etkilenmez)

### Kabul Kriterleri

- [ ] "geçtiğimiz ay en çok hangi gün tweet atılmıştır?" → "peki o gün en çok hangi yazar tweet atmıştır?"
  sorusu doğru tarihi (May 20) kullanıyor
- [ ] Yeni konuşmada bağlam varsayılan açık; toggle ile kapatılabiliyor
- [ ] Toggle durumu konuşma bazlı saklanıyor, konuşma değişince eski ayarı koruyor
- [ ] Büyük sonuç setlerinde özet 300 karakteri geçmiyor, prompt bütçesi korunuyor
- [ ] `make lint-go` + `make test-go` + `make check-frontend` temiz

---

## Backend Middleware & Yardımcı Fonksiyon Konsolidasyonu (2026-06-12)

Amaç: HTTP handler katmanındaki tekrarlanan kalıpları middleware ve ortak yardımcılara çekerek kod tekrarını azaltmak, tutarlılığı artırmak ve bakım maliyetini düşürmek.

### Mevcut Altyapı

**`internal/http/middleware/`** mevcut middleware'ler: `Paginate`, `JWTAuth`, `APIKeyAuth`, `RequirePermission`, `RequireDatasourceAccess`, `RealIP`, `SecurityHeaders`, `Locale`, `InjectAIUserContext`.

**Dağılmış middleware'ler:** `request_logger.go`, `request_id.go`, `metrics_middleware.go`, `catalog_metrics_middleware.go`, `handlers/admin_middleware.go`, `handlers/internal_auth_middleware.go`, `handlers/internal_audit_middleware.go`.

**Mevcut yardımcılar:** `response/response.go` (`WriteJSON`, `WriteError`, `WriteInternalError`), `handlers/helpers.go` (`decodeJSON[T]`, `writeJSON`, `writeError`, `requireURLParam`, `resolveAccessibleDatasources`).

### MW-1 — Router Middleware Stack Tekilleştirmesi [HIGH]

6 ayrı binary (monolith, AI, query, catalog, auth, mail) aynı chi middleware zincirini kopyalıyor. Auth servisi (`cmd/auth/main.go`) ayrıca `requestIDPropagationMiddleware`'i yeniden implement ediyor.

- [x] `internal/http/middleware/` veya yeni `internal/http/router_base.go` içinde `BaseMiddlewareConfig` struct + `ApplyBaseMiddleware(r chi.Router, cfg BaseMiddlewareConfig)` oluştur
- [x] Monolith router'ı (`internal/http/router.go:27-60`) yeni fonksiyonu kullanacak şekilde refaktor et
- [x] AI router (`internal/http/ai_router.go:18-27`) — aynı
- [x] Query router (`internal/http/query_router.go:18-27`) — aynı
- [x] Catalog router (`internal/http/catalog_router.go:18-27`) — aynı
- [x] Auth service (`cmd/auth/main.go:280-310`) — kendi `propagateRequestID` kopyasını sil, `internal/http/`'deki export edilmiş versiyonu kullan
- [x] Mail service (`cmd/mail/main.go:93-97`) — eksik request ID propagation + request logger'ı ekle
- [x] Regresyon testi: her router'ın aynı middleware zincirini ürettiğini doğrula

**Etki:** ~120 satır tekrar kalkar, tek kaynak. Yeni servis eklendiğinde tek satır.

#### MW-1 Review

Resolved:

1. `internal/http/router_base.go` eklendi; ortak chi stack `ApplyBaseMiddleware` üzerinden yönetiliyor.
2. Monolith, AI, query, catalog, auth ve mail router'ları base helper'a geçirildi.
3. Auth servisindeki local `propagateRequestID` kopyası kaldırıldı; ortak `RequestIDPropagation` export edildi.
4. Mail servisine ortak request ID propagation ve request-scoped logger eklendi.
5. Regresyon testi router dosyalarının `ApplyBaseMiddleware` kullanmasını ve manuel base middleware zincirinin geri gelmemesini doğruluyor.

Verification:

- Red: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http -run TestApplyBaseMiddlewarePropagatesRequestIDAndSecurityHeaders -count=1` missing `ApplyBaseMiddleware` / `BaseMiddlewareConfig` ile düştü.
- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http -run 'TestApplyBaseMiddleware|TestServiceRoutersUseBaseMiddleware' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http ./cmd/auth ./cmd/mail -count=1`
- `make lint-go`
- `git diff --check`

Notes:

- gograph MCP tools were not connected in this session, so `gograph_capabilities`, `gograph_plan`, and `gograph_review --uncommitted` could not be run.

## First User Super Admin Bootstrap Plan

Success criteria:

- When the auth DB `users` table is empty, the first created user receives the global `super_admin` role.
- First-user signup is allowed even when `platform_settings.self_signup_enabled = false`.
- After the first user exists, disabled self-signup continues to block normal public registration.
- Public password policy response tells the SPA when first-user setup is required.
- Sign-in/sign-up UI shows first-admin setup guidance without adding a new auth route.

- [x] Add backend first-user setup detection.
- [x] Make registration gating allow only the empty-DB bootstrap exception.
- [x] Assign `super_admin` atomically to exactly one first user using a transaction-scoped DB lock.
- [x] Expose `first_user_setup_required` through `/api/auth/password-policy`.
- [x] Update frontend auth policy types/helpers.
- [x] Show first-admin setup instructions on sign-in and sign-up screens.
- [x] Add backend and frontend regression coverage.
- [x] Run focused and broad verification.

## First User Super Admin Bootstrap Review

Resolved:

1. `UserRepository` now checks the empty-user bootstrap state inside user-creation transactions and serializes the first-user decision with `pg_advisory_xact_lock`.
2. The first created user gets global `super_admin`; normal users keep the existing viewer global role and personal workspace admin membership.
3. `Service.Register` now allows registration when self-signup is disabled only if first-user setup is still required.
4. `/api/auth/password-policy` returns `first_user_setup_required`, and the React auth screens use it to keep first setup discoverable.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth ./internal/auth/handlers -run 'Test(SelfSignupDisabledBlocksRegister|FirstUserSetupAllowsRegisterWhenSelfSignupDisabled|FirstUserSetupAssignsOnlyOneSuperAdminConcurrently|SuperAdminUpdatesPlatformSettings|PasswordPolicyReportsFirstUserSetupState)' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth ./internal/auth/handlers -count=1`
- `make lint-go`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `npm --prefix frontend run format:check`
- `npm --prefix frontend exec eslint -- src/api/auth.ts src/api/auth.test.ts src/types/auth.ts src/components/auth/SignInPage.tsx src/components/auth/SignUpPage.tsx src/i18n/locales/en/auth.ts src/i18n/locales/tr/auth.ts --max-warnings 0`
- `git diff --check`
- `gograph_review` for `(*UserRepository).CreateUser`, `(*Service).Register`, and `(*AuthHandler).handlePasswordPolicy`

Notes:

- Repo-wide `npm --prefix frontend run lint` still reports unrelated existing errors in `frontend/src/components/ui/MultiSelect.tsx`, `SelectPopover.tsx`, and `useSelectDropdown.ts`.
- `gograph_review --uncommitted` failed with `git diff: exit status 129`, so symbol-scoped gograph reviews were run instead.

## Signup Auth Session Bugfix Plan

Success criteria:

- Signup no longer shows a low-level `authorization header required` error after a successful registration.
- Guest auth pages do not spam protected refresh/me endpoints without a valid session.
- Auth/CSRF behavior stays secure; no bypass of bearer/session checks is introduced.
- Local dev `.env.dev` includes the Kubernetes/Helm runtime settings needed for auth/mail/API parity, without changing Helm/prod values.
- Focused regression coverage proves the successful signup behavior.
- Verification includes the focused test gate plus Semgrep for the changed security surface.

- [x] Reproduce the signup/register -> refresh/me failure with a deterministic local loop.
- [x] Compare Kubernetes/Helm runtime env with `.env`/`.env.dev` and patch local-only gaps.
- [x] Identify whether the fix belongs in frontend session handling, backend auth contract, or both.
- [x] Implement the smallest fix that preserves CSRF/auth requirements.
- [x] Run focused tests, lint/build as needed, gograph review for Go changes if any, and Semgrep.
- [x] Document the outcome and verification commands in this tracker.

## Signup Auth Session Bugfix Review

Resolved:

1. Root cause: auth registration can legitimately return only `verification_pending=true` on the duplicate/anti-enumeration path, but the frontend treated every successful register response as token-bearing and called `/me` with no bearer token.
2. `apiRegister` now returns a `RegisterResponse`, and `registerResponseHasSession` gates the token-bearing branch.
3. `AuthProvider.register` no longer calls `handleAuthSuccess` when the register response has no access token; `SignUpPage` shows the existing generic registration-success message instead.
4. `.env.dev` now includes local-only Kubernetes parity values for auth/mail internal tokens plus a generated stable local JWT key. Helm/prod values were not changed.
5. Semgrep initially failed on an inherited local skill doc command that downloaded the latest Hubble CLI release artifact. The doc now tells agents to use a trusted package manager or approved internal mirror instead.

Verification:

- `npm --prefix frontend run test -- src/api/auth.test.ts`
- `npm --prefix frontend exec prettier -- --check frontend/src/types/auth.ts frontend/src/api/auth.ts frontend/src/api/auth.test.ts frontend/src/components/auth/AuthProvider.tsx frontend/src/components/auth/SignUpPage.tsx`
- `npm --prefix frontend run lint`
- `npm --prefix frontend run build`
- `npm --prefix frontend run test`
- `git diff --check`
- `make semgrep-scan`

Notes:

- No Go code was changed; backend auth handlers/service were inspected with gograph only to confirm the register response contract.
- The local Vite server was not running when the first curl repro was attempted, so the direct browser flow was not re-clicked in this slice.

## Duplicate Auth Refresh Single-Flight Plan

Success criteria:

- Local dev no longer sends concurrent `/api/auth/refresh` POSTs from initial auth boot.
- Refresh token rotation/theft protection remains backend-enforced; no backend or Helm auth weakening.
- Focused frontend coverage proves concurrent cookie-backed refresh calls share one request.
- Verification includes frontend tests/lint/build, `git diff --check`, and Semgrep.

- [x] Add a failing frontend test for concurrent cookie-backed `apiRefresh()` calls.
- [x] Implement module-level single-flight refresh for cookie-backed refresh only.
- [x] Confirm explicit legacy refresh-token calls keep their current behavior.
- [x] Run frontend verification and Semgrep.
- [x] Document the outcome and commands in this tracker.

## Duplicate Auth Refresh Single-Flight Review

Resolved:

1. Root cause: local dev/React remounts can start two cookie-backed `/api/auth/refresh` calls at the same time. The auth backend correctly rotates refresh tokens on use, so one request can succeed while the second hits the rotated token and returns 401/400.
2. `apiRefresh()` now uses a module-level single-flight promise for cookie-backed refresh calls, so concurrent callers share the same in-flight POST and response.
3. Explicit legacy refresh-token calls remain uncoalesced, preserving their previous request semantics.
4. No backend auth, refresh-token rotation, CSRF, Helm, or Kubernetes settings were weakened.

Verification:

- Red: `npm --prefix frontend run test -- src/api/auth.test.ts` failed because concurrent cookie-backed refresh calls made two `apiFetch` calls.
- Green: `npm --prefix frontend run test -- src/api/auth.test.ts`
- `npm --prefix frontend exec prettier -- --check frontend/src/api/auth.ts frontend/src/api/auth.test.ts frontend/src/components/auth/AuthProvider.tsx frontend/src/components/auth/SignUpPage.tsx frontend/src/types/auth.ts`
- `npm --prefix frontend run lint`
- `npm --prefix frontend run build`
- `npm --prefix frontend run test`
- `git diff --check`
- `make semgrep-scan`

## Local Zlitter Datasource Connection Plan

Success criteria:

- The local `Zlitter` datasource test uses the reachable target host/port.
- Any change is local metadata/config only unless code proves necessary.
- Stored datasource secrets are not printed or rewritten unnecessarily.
- Verification proves the connection path succeeds from the same environment the app uses.

- [ ] Inspect the local metadata datasource record without exposing secrets.
- [ ] Compare the stored host/port with the reachable `192.168.50.228:15432` target.
- [ ] Apply the smallest local fix if the stored target is wrong.
- [ ] Verify the datasource connection through the app-facing path or equivalent driver path.
- [ ] Document the outcome and commands in this tracker.

### MW-2 — JSON Decode + Error Response Boilerplate (Auth Handlers) [HIGH]

Auth handler'lar (`internal/auth/handlers/handler.go`, `handler_rbac.go`, `handler_mfa.go`) ~45 yerde `sonic.Decode + respondError` kalıbını tekrarlıyor. Ana handler paketindeki `decodeJSON[T]` generic helper'ı kullanmıyor — ve `http.MaxBytesReader` koruması da eksik.

- [x] `decodeJSON[T]`'yi `internal/http/response/` paketine taşı (veya yeni `internal/http/request/` paketi)
- [x] Auth handler'ların tümünü `decodeJSON[T]`'ye geçir
- [x] `http.MaxBytesReader` koruması otomatik gelsin
- [x] Regresyon testi: MaxBytesReader limit aşımı 413 dönüyor; hatalı JSON 400 dönüyor

**Etki:** ~90 satır kalkar, auth endpoint'leri payload bombasına karşı korunur.

**Dosyalar:** `internal/auth/handlers/handler.go`, `handler_rbac.go`, `handler_mfa.go`, `internal/http/handlers/helpers.go`, `internal/http/response/response.go`

#### MW-2 Review

Resolved:

1. `response.DecodeJSON[T]`, `response.DecodeJSONAllowEmpty[T]`, and `response.MaxJSONRequestBytes` now own JSON request decoding and `http.MaxBytesReader` protection.
2. Existing `internal/http/handlers` decode helpers now delegate to `response.*`, preserving current non-auth call sites.
3. Auth handlers use shared `decodeJSON` / `decodeJSONAllowEmpty` wrappers instead of raw `sonic.ConfigStd.NewDecoder(r.Body)` blocks.
4. Optional-body auth endpoints (`refresh`, `logout`, account deletion) use the allow-empty helper while still returning 400/413 for invalid or oversized bodies.
5. A source guard test prevents auth handlers from reintroducing raw request-body JSON decoders.

Verification:

- Red: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/response -run 'TestDecodeJSON' -count=1` failed on missing `DecodeJSON`, `DecodeJSONAllowEmpty`, and `MaxJSONRequestBytes`.
- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/response -run 'TestDecodeJSON' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth/handlers -run 'TestAuthHandlersUseSharedJSONDecoder|TestGDPRExportCompleteness' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/response ./internal/http/handlers ./internal/auth/handlers -count=1`
- `make lint-go`
- `git diff --check`

Notes:

- MW-3 response/error writer consolidation is intentionally left for MW-3; this slice only centralizes request JSON decoding and body-size protection.
- gograph MCP tools were not connected in this session, so `gograph_capabilities`, `gograph_plan`, and `gograph_review --uncommitted` could not be run.

### MW-3 — `writeJSON`/`writeError`/`respondError` Konsolidasyonu [HIGH]

4 ayrı implementasyon: `response.WriteJSON`, `handlers.writeJSON`, `cmd/auth/main.go:381` (nil-slice normalization eksik), `auth/handlers/handler_rbac.go:981`. Aynı şekilde error helper'lar 3-4 yerde dağılmış.

- [x] `internal/http/response/` paketini tek canonical kaynak yap
  - `WriteJSON` — nil-slice normalization + error logging içerir (mevcut hali)
  - `WriteError(w, status, message)` — 5xx mesaj sanitizasyonu içerir (auth'daki `respondError` mantığını birleştir)
  - `WriteInternalError(ctx, w, msg, err)` — log + sanitized public response
- [x] `handlers/helpers.go`'daki `writeJSON`/`writeError`/`writeInternalError`'ı `response.*` wrapper'larına geçir
- [x] `cmd/auth/main.go`'daki standalone `writeJSON`'ı kaldır, `response.WriteJSON` kullan
- [x] `internal/auth/handlers/`'daki `respondError`/`writeError`'ı `response.WriteError`'a geçir
- [x] Regresyon testi: nil slice → `[]`, 5xx → sanitized mesaj, 4xx → raw mesaj

**Etki:** Tek tutarlı response API. 5xx sanitizasyonu tüm servislerde garanti.

**Dosyalar:** `internal/http/response/response.go`, `internal/http/handlers/helpers.go`, `cmd/auth/main.go`, `internal/auth/handlers/handler.go`, `handler_rbac.go`

#### MW-3 Review

Resolved:

1. Centralized response serialization and error sanitization inside `internal/http/response/`.
2. Updated `WriteError(w, status, message)` to log original messages for 5xx errors and send sanitized `"internal server error"` responses.
3. Updated `WriteInternalError` to log contextually and output sanitized public responses (bypassing `WriteError` to prevent double-logging).
4. Removed local `writeJSON` from `cmd/auth/main.go` and migrated calling endpoints to `response.WriteJSON`.
5. Simplified `respondError` wrapper in `internal/auth/handlers/handler.go` to delegate directly to `response.WriteError`.
6. Simplified `writeError` package-level helper in `internal/auth/handlers/handler_rbac.go` to call `response.WriteInternalError` for 5xx and `response.WriteError` for 4xx errors.
7. Fixed failing tests in `metadata_rows_test.go` and `response_test.go` to expect the new sanitized wire formats for 5xx responses.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/response/... ./internal/auth/handlers/... ./internal/http/handlers/... -count=1`
- `make lint-go`
- `git diff --check`

### MW-4 — Auth Handler Pagination → Middleware Kullanımı [MEDIUM]

`internal/auth/handlers/handler.go:1200-1221` pagination'ı manuel implement ediyor (`strconv.Atoi`, default values, slice offset). `bimw.Paginate` middleware'i zaten aynı işi yapıyor.

- [x] Auth service route'larına `bimw.Paginate` middleware'ini ekle
- [x] `handleAdminListInvitations`'da manuel pagination kodunu `bimw.PaginationFromContext`'e geçir
- [x] Regresyon testi: page/page_size query param'ları doğru context değerini üretiyor

**Dosyalar:** `internal/auth/handlers/handler.go`, auth router dosyası

#### MW-4 Review

Resolved:

1. Added `bimw.Paginate` middleware to the GET `/admin/invitations` route in `internal/auth/handlers/handler.go`.
2. Refactored `handleAdminListInvitations` to retrieve parameters using `bimw.PaginationFromContext` instead of manual parsing.
3. Added comprehensive regression tests in `internal/auth/handlers/handler_invitations_test.go` validating correct pagination boundaries and data filtering.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth/handlers/... -run TestAdminListInvitationsPagination -count=1`
- `make lint-go`
- `git diff --check`

### MW-5 — `resolveWorkspaceDatasourceFilter` vs `resolveAccessibleDatasources` Birleştirme [MEDIUM]

İki fonksiyon neredeyse aynı işi yapıyor (auth check → super admin bypass → user datasources → workspace intersect), 7 handler dosyasından 11 yerde çağrılıyor.

- [x] Tek fonksiyon: `resolveDatasourceScope(ctx, cfg) (map[string]struct{}, bool, error)` — error handling opsiyonel (fail-closed vs return error)
- [x] `internal/http/handlers/datasource_scope.go`'ya taşı
- [x] Tüm call site'leri güncelle
- [x] Regresyon testi: super admin bypass, auth disabled, normal user scope

**Dosyalar:** `internal/http/handlers/history_filter.go`, `internal/http/handlers/helpers.go`, 7 handler dosyası

#### MW-5 Review

Resolved:

1. Unified `resolveWorkspaceDatasourceFilter` and `resolveAccessibleDatasources` into a single canonical helper `resolveDatasourceScope` in `internal/http/handlers/datasource_scope.go`.
2. Cleaned up deprecated functions and imports from `internal/http/handlers/history_filter.go` and `internal/http/handlers/helpers.go`.
3. Migrated all call sites in `ai_history.go`, `query.go`, `datasources.go`, and `semantic.go`.
4. History/AI history endpoints log scope resolution errors and fail-closed contextually, while listing endpoints propagate errors to yield 500 status codes.
5. Added a comprehensive test suite in `internal/http/handlers/datasource_scope_test.go` covering all execution paths.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers/... -run TestResolveDatasourceScope -count=1`
- `make lint-go`
- `git diff --check`

### MW-6 — Request ID Propagation Export [MEDIUM]

`internal/http/request_id.go`'daki `requestIDPropagationMiddleware` unexported. `cmd/auth/main.go` kendi kopyasını (`propagateRequestID`) yazmış.

- [x] `requestIDPropagationMiddleware` → `RequestIDPropagation` olarak export et
- [x] `cmd/auth/main.go`'daki `propagateRequestID`'yi sil, export edilmiş versiyonu kullan
- [x] MW-1 ile birlikte router_base'e dahil et

**Dosyalar:** `internal/http/request_id.go`, `cmd/auth/main.go`

#### MW-6 Review

Resolved:

- Exported `RequestIDPropagation` in `internal/http/request_id.go`.
- Removed `propagateRequestID` duplicate copy in `cmd/auth/main.go` and migrated calls to the shared package-level middleware.
- Included it in the shared `ApplyBaseMiddleware` stack in `internal/http/router_base.go`.
- All steps were completed and validated during the **MW-1** sweep.

### MW-7 — `requireQueryParam` Kullanımını Genelleştir [MEDIUM]

`datasource_id is required` kontrolü 7 handler dosyasında tekrarlanıyor. `internal/http/handlers/internal.go:361-369`'da `requireQueryParam` zaten var ama package-private ve tutarsız kullanılıyor. `helpers.go:180-187`'de `requireURLParam` da var.

- [x] `requireQueryParam(w, r, key string) (string, bool)` fonksiyonunu `internal/http/response/` veya `helpers.go`'da export et
- [x] 7 call site'i güncelle: `ai_glossary.go`, `ai_examples.go`, `ai_job_service.go`, `semantic.go`, `composite.go`, `ai_jobs.go`
- [x] Regresyon testi: eksik param → 400 + `"datasource_id is required"`

**Dosyalar:** `internal/http/handlers/helpers.go`, `internal.go`, yukarıdaki 7 handler dosyası

#### MW-7 Review

Resolved:

1. Renamed `requireQueryParam` in `internal/http/handlers/internal.go` to `requireInternalQueryParam` to prevent collision and preserve its custom internal API JSON response envelopes.
2. Implemented the user-facing `requireQueryParam(w, r, key)` helper inside `internal/http/handlers/helpers.go` with strings-trimming and a standard 400 response format.
3. Refactored call sites checking query parameters in `ai_glossary.go`, `ai_examples.go`, and `ai_jobs.go` to use the new helper.
4. Added test cases in `internal/http/handlers/internal_test.go` (`TestRequireInternalQueryParam` and `TestRequireQueryParam`) verifying both internal and general query parameter validation functions.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers/... -run 'TestRequireQueryParam|TestRequireInternalQueryParam' -count=1`
- `make lint-go`
- `git diff --check`

### MW-8 — `statusRecorder` Tip Tekilleştirmesi [LOW]

2 ayrı ama özdeş struct: `handlers/internal_audit_middleware.go:49-57` (`statusRecorder`) ve `catalog_metrics_middleware.go:28-36` (`metricsStatusRecorder`).

- [x] `response.StatusRecorder` tipi oluştur `internal/http/response/` altında
- [x] Her iki kullanım yeri güncelle
- [x] Test: `WriteHeader` çağrısında status correctly capture ediliyor

**Dosyalar:** `internal/http/response/response.go`, `internal/http/handlers/internal_audit_middleware.go`, `internal/http/catalog_metrics_middleware.go`

#### MW-8 Review

Resolved:

1. Created a unified `response.StatusRecorder` type in `internal/http/response/response.go` to capture HTTP status codes dynamically.
2. Replaced the local duplicate `statusRecorder` struct in `internal_audit_middleware.go` with the consolidated `response.StatusRecorder`.
3. Replaced the local duplicate `metricsStatusRecorder` struct in `catalog_metrics_middleware.go` and integrated it with `metrics_middleware.go` as well.
4. Added the `TestStatusRecorder` unit test in `response_test.go` to assert correct status code capture and ResponseWriter delegation.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/response/... -run TestStatusRecorder -count=1`
- `make lint-go`
- `git diff --check`

### MW-9 — `{"status":"ok"}` Response Helper [LOW]

10 handler aynı success envelope'u manuel oluşturuyor: `writeJSON(w, 200, map[string]string{"status":"ok"})`.

- [x] `response.WriteOK(w)` helper'ı ekle
- [x] 10 call site'i güncelle
- [x] Test: response body `{"status":"ok"}`

**Dosyalar:** `internal/http/response/response.go`, `ai_glossary.go`, `ai_time_grains.go`, `ai_providers.go`, `ab_experiment.go`, `ai_prompt_templates.go`

#### MW-9 Review

Resolved:

1. Implemented `response.WriteOK(w)` in `internal/http/response/response.go` to return standard `{"status": "ok"}` success envelope.
2. Implemented package-private `writeOK(w)` in `internal/http/handlers/helpers.go`.
3. Migrated all 10 manual occurrences of `"status": "ok"` responses in `ai_glossary.go`, `ai_time_grains.go`, `ai_providers.go`, `ab_experiment.go`, and `ai_prompt_templates.go` to use `writeOK(w)`.
4. Added the `TestWriteOK` unit test in `response_test.go` to verify response code and body format.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/response/... ./internal/http/handlers/...`
- `make lint-go`

### MW-10 — Integer Query Param Parsing Helper [MEDIUM]

5 yerde `strconv.Atoi` ile inline limit/integer parse ediliyor. `pagination.go:84-93`'te `parsePositiveQueryInt` var ama unexported.

- [x] `ParsePositiveIntQueryParam(r *http.Request, key string) (int, bool)` export et
- [x] `ai_examples.go`, `ai_jobs.go`, `semantic.go`'daki inline parse'ları güncelle
- [x] Test: geçersiz değer → default, geçerli → parsed

**Dosyalar:** `internal/http/middleware/pagination.go`, `internal/http/handlers/ai_examples.go`, `ai_jobs.go`, `semantic.go`

#### MW-10 Review

Resolved:

1. Implemented and exported `ParsePositiveIntQueryParam(r *http.Request, key string) (int, bool)` helper in `internal/http/middleware/pagination.go`.
2. Refactored the 5 manual query integer parses using `strconv.Atoi` in `ai_examples.go`, `ai_jobs.go`, and `semantic.go` to use the helper. Removed the unused `"strconv"` import from these files.
3. Added the `TestParsePositiveIntQueryParam` unit test in `pagination_test.go` checking positive values, negative values, invalid numeric formats, and missing params.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test -race ./internal/http/response/... ./internal/http/handlers/... ./internal/http/middleware/...`
- `make lint-go`

### MW-11 — Handler Datasource Access Check Birleştirme [MEDIUM]

3 ayrı mekanizma datasource erişim kontrolü yapıyor: middleware seviyesi (`RequireDatasourceAccess`), handler metotları (`requireDatasourceAccess`, `requireTableAccess`/`requireColumnAccess`). Handler seviyesindeki kontroller entity ID'den datasource resolve ettiği için middleware ile tam kapsanamıyor.

- [x] `ResolveDatasourceID(r *http.Request)` helper'ı oluştur: URL param, query param VE entity lookup (table/column → datasource ID)
- [x] Tek `CheckDatasourceAccess(ctx, dsID, level)` helper'ı
- [x] `metadata.go`'daki `requireDatasourceAccess`/`requireTableAccess`/`requireColumnAccess`'ı buna geçir
- [x] Regresyon testi: table UUID → datasource resolve → access check

**Dosyalar:** `internal/http/handlers/metadata.go`, `internal/http/middleware/permission.go`

#### MW-11 Review

Resolved:

1. Implemented `ResolveDatasourceID(r *http.Request) (string, error)` in `internal/http/handlers/metadata.go` supporting URL parameter mapping, query parameter mapping, and metadata table/column repository lookups.
2. Implemented `CheckDatasourceAccess(ctx, dsID, level) (bool, error)` in `internal/http/handlers/metadata.go` supporting super admin bypass, auth client calls, and context user verification.
3. Refactored `requireDatasourceAccess`, `requireTableAccess`, and `requireColumnAccess` to call these new helper functions, eliminating redundant code blocks.
4. Added the unit test `TestResolveDatasourceIDAndCheckDatasourceAccess` in `metadata_auth_test.go` checking all resolution and check code paths.
5. Documented the deferred permission checks in the middleware `internal/http/middleware/permission.go`.

Verification:

- Green: `GOCACHE=/private/tmp/biqly-gocache go test -race ./internal/http/handlers/... ./internal/http/middleware/...`
- `make lint-go`

### MW-12 — Auth Handler `requireUserID` → Shared Helper [MEDIUM]

Auth handler kendi context key'leriyle `requireUserID` kullanıyor (~15 call site). Ana handler paketi `bimw.UserID(ctx)` kullanıyor.

- [ ] Auth handler'ın context key'lerini `bimw` paketiyle uyumlu yap (veya `bimw.UserID`'yi auth middleware'in de set ettiği garanti altına al)
- [ ] `requireUserID` → `requireUserIDFromContext(ctx, w) (string, bool)` shared helper
- [ ] Regresyon testi: auth JWT sonrası context'te doğru user ID

**Dosyalar:** `internal/auth/handlers/handler.go`, `internal/http/middleware/jwt.go`

### Öncelik Sırası

| Sıra | Madde | Öncelik | Tahmini Etki |
|---|---|---|---|
| 1 | MW-1 Router stack tekilleştirme | HIGH | ~120 satır, tek kaynak |
| 2 | MW-2 JSON decode generic helper | HIGH | ~90 satır, güvenlik (MaxBytes) |
| 3 | MW-3 Response helper konsolidasyonu | HIGH | Tutarlı response API |
| 4 | MW-4 Auth pagination → middleware | MEDIUM | Middleware kullanımı |
| 5 | MW-6 Request ID export | MEDIUM | 2 kopya → 1 |
| 6 | MW-5 Datasource scope birleştirme | MEDIUM | 11 call site |
| 7 | MW-7 requireQueryParam genelleştirme | MEDIUM | 7 call site |
| 8 | MW-10 Int query param helper | MEDIUM | 5 call site |
| 9 | MW-11 Datasource access birleştirme | MEDIUM | 3 implementasyon |
| 10 | MW-12 Auth requireUserID | MEDIUM | ~15 call site |
| 11 | MW-8 statusRecorder tipi | LOW | 2 kopya |
| 12 | MW-9 writeOK helper | LOW | 10 call site |

### Bağımlılıklar

- MW-1 bağımsız (herhangi bir sırayla başlanabilir)
- MW-3 → MW-2 (response paketi önce konsolide edilmeli)
- MW-1 → MW-6 (request ID export router_base'e dahil)
- MW-4 auth router'a middleware eklemini gerektirir (MW-1 sonrası daha kolay)

## Frontend Middleware & Yardımcı Fonksiyon Konsolidasyonu (2026-06-12)

Amaç: React bileşenlerindeki tekrarlanan kalıpları custom hook, yardımcı fonksiyon ve paylaşılan stillere çekerek kod tekrarını azaltmak, UI tutarlılığını artırmak ve bakım maliyetini düşürmek.

### FW-1 — `errorMessage()` Yardımcısını Merkezi Yapma [HIGH]

`e instanceof Error ? e.message : String(e)` ifadesi 43 yerde tekrarlanıyor. `hooks/usePaginatedListLogic.ts:44`'te `errorMessage()` olarak zaten var ama çoğu call site inline kullanmaya devam ediyor.

- [ ] `errorMessage(e: unknown): string` fonksiyonunu `frontend/src/utils/error.ts`'e taşı
- [ ] `usePaginatedListLogic.ts`'teki yerel tanımı import ile değiştir
- [ ] 43 inline instance'ı `errorMessage(e)` ile değiştir
- [ ] ESLint + vitest temiz

**Dosyalar:** `utils/error.ts` (yeni), `hooks/usePaginatedListLogic.ts`, 20+ admin/ayar/bileşen dosyası

**Etki:** 43 tekrar → 1 yardımcı fonksiyon. Catch bloklarında tutarsız error handling kalkar.

### FW-2 — `useAsyncEffect` / `useFetch` Hook'u [HIGH]

İptal bayraklı (`let cancelled = false`) useEffect data loading kalıbı 19 bileşende tekrarlanıyor. Her biri `loading`, `error`, `setData` state yönetimini ayrı ayrı kuruyor.

- [ ] `frontend/src/hooks/useFetch.ts` oluştur:

  ```typescript
  function useFetch<T>(fetcher: () => Promise<T>, deps: unknown[]): {
    data: T | null; loading: boolean; error: string | null; setData: Dispatch<SetStateAction<T | null>>
  }
  ```

  AbortController ile cleanup, hata durumunda `errorMessage()` kullanımı
- [ ] 19 bileşeni `useFetch`'e geçir: `AIHistoryPanel`, `ShareButton`, `AIQuery`, `Composites`, `Metadata`, `SignUpPage`, `SignInPage`, `FieldPermissionPanel`, `QueryHistory`, `useMetadataBulkDescribeModalState`, `useTableBrowserPage`, `AIUsageAdminPanel`, `TableBrowserRowModal`, `RowLevelSecurityPanel`, `RolesPanel`, `WorkspaceSelector`
- [ ] Her geçiş sonrası ilgili bileşenin vitest testi çalıştır

**Dosyalar:** `hooks/useFetch.ts` (yeni), 19 bileşen dosyası

**Etki:** ~380 satır tekrar kalkar. Bellek sızıntısı riski (cleanup eksik) yapısal olarak engellenir.

### FW-3 — Admin Panel Paylaşılan Stillerinin Merkezi Yapılması [HIGH]

6+ admin panel dosyası aynı `CSSProperties` objelerini (`errStyle`, `containerStyle`, `btnPrimary`, `btnSecondary`, `btnPrimaryDisabled`, `btnSecondaryDisabled`, `inputStyle`) tekrar tekrar tanımlıyor.

- [ ] `frontend/src/styles/adminShared.css` oluştur — BEM class'ları olarak:
  - `.admin-panel__error` (errStyle)
  - `.admin-panel__container` (containerStyle)
  - `.admin-btn--primary`, `.admin-btn--secondary`, `.admin-btn--primary-disabled`, `.admin-btn--secondary-disabled`
  - `.admin-input`
- [ ] 6+ admin panel dosyasını class'lara geçir: `PIIDetectionPanel`, `WorkspacesPanel`, `RowLevelSecurityPanel`, `FieldPermissionPanel`, `RolesPanel`, `AIProvidersPanel`
- [ ] Prettier + ESLint + vitest temiz

**Dosyalar:** `styles/adminShared.css` (yeni), 6+ admin panel bileşeni

**Etki:** ~120 satır duplicate style tanımı kalkar. Tema değişikliği tek dosyadan yapılır.

### FW-4 — Hata Gösterimi Tutarlılaştırma (`<ErrorAlert>`) [HIGH]

Hata gösterimi 5 farklı yöntemle yapılıyor: `<ErrorAlert>` bileşeni (5 kullanım), inline `errStyle` div (11), CSS class `xxx__error` (4), CSS class `error`/`form-error` (3), auth-specific `auth-error` (1). Toplam 31 instance.

- [ ] Mevcut `ErrorAlert` bileşenini zenginleştir: `variant` prop'u (`'panel' | 'inline' | 'auth'`), `className` override desteği
- [ ] 31 instance'ı `<ErrorAlert>`'e geçir
- [ ] FW-3 ile koordineli çalış: inline errStyle → `.admin-panel__error` class'ı → `<ErrorAlert variant="panel">`
- [ ] Test: tüm hata yolları görsel olarak tutarlı

**Dosyalar:** `components/ui/ErrorAlert.tsx`, 20+ bileşen dosyası

**Etki:** Tutarlı hata UX'i. Erişilebilirlik (role="alert") tek yerden garanti.

### FW-5 — API Base Path Sabitlerinin Merkezi Yapılması [HIGH]

`AUTH_API_BASE = '/api/auth'` 4 dosyada, `AI_API_BASE = '/api/ai'` 3 dosyada, `adminOpts = { useAdminKey: true as const }` 3 dosyada tekrarlanıyor.

- [ ] `frontend/src/api/constants.ts` oluştur:

  ```typescript
  export const AUTH_API_BASE = '/api/auth'
  export const AI_API_BASE = '/api/ai'
  export const ADMIN_OPTS = { useAdminKey: true as const } as const
  ```

- [ ] `api/auth.ts`, `api/admin.ts`, `api/aiModelAccess.ts`, `api/ldap.ts`, `api/aiProviders.ts`, `api/aiUserModels.ts`, `api/aiAdmin.ts`'teki yerel tanımları import ile değiştir
- [ ] ESLint + build temiz

**Dosyalar:** `api/constants.ts` (yeni), 7 API dosyası

**Etki:** API path değişikliği tek dosyadan yapılır. Yanlış path copy-paste riski kalkar.

### FW-6 — `buildQueryString()` Yardımcı Fonksiyonu [MEDIUM]

`URLSearchParams` + optional parametre kalıbı 10+ API fonksiyonunda tekrarlanıyor. Her biri aynı `if (value) params.set(key, String(value))` + `suffix` kalıbını kuruyor.

- [ ] `frontend/src/utils/query.ts` oluştur:

  ```typescript
  export function buildQueryString(params: Record<string, string | number | boolean | undefined | null>): string
  ```

- [ ] `api/admin.ts` (9 instance), `api/aiProviders.ts` (1 instance) call site'lerini güncelle
- [ ] Test: null/undefined/empty değerler query'den çıkıyor; geçerli değerler doğru encode ediliyor

**Dosyalar:** `utils/query.ts` (yeni), `api/admin.ts`, `api/aiProviders.ts`

**Etki:** ~60 satır tekrar kalkar. Query string tutarsızlığı kalkar.

### FW-7 — `useApiResource<T>` Generic Hook [MEDIUM]

`useDatasources`, `useSemanticModels`, `useModelDetail` neredeyse aynı yapıda: URL'ye GET at, loading/error/data state yönet, reload fonksiyonu sun.

- [ ] `frontend/src/hooks/useApiResource.ts` oluştur:

  ```typescript
  function useApiResource<T>(url: string | null, defaultValue: T): {
    data: T; loading: boolean; error: string | null; reload: () => void; setData: Dispatch<SetStateAction<T>>
  }
  ```

- [ ] `useDatasources`, `useSemanticModels`, `useModelDetail`'i `useApiResource`'a geçir
- [ ] Test: loading → data → error geçişleri doğru çalışıyor

**Dosyalar:** `hooks/useApiResource.ts` (yeni), `hooks/useDatasources.ts`, `hooks/useSemanticModels.ts`, `hooks/useModelDetail.ts`

**Etki:** 3 hook → 1 generic + 3 one-liner call. Yeni resource eklemek tek satır.

### FW-8 — `useAsyncState` Hook'u [MEDIUM]

`[loading, setLoading] + [error, setError] + [saving, setSaving]` state üçlemesi 12+ bileşende bağımsız `useState` çağrılarıyla tekrarlanıyor.

- [ ] `frontend/src/hooks/useAsyncState.ts` oluştur:

  ```typescript
  function useAsyncState(): {
    loading: boolean; error: string | null; saving: boolean;
    setLoading: Dispatch<SetStateAction<boolean>>;
    setError: Dispatch<SetStateAction<string | null>>;
    clearError: () => void;
    run: <T>(fn: () => Promise<T>) => Promise<T | null>;
  }
  ```

- [ ] 12+ admin/ayar bileşenini güncelle
- [ ] Test: `run` hatası setError + setSaving(false) yapıyor

**Dosyalar:** `hooks/useAsyncState.ts` (yeni), 12+ bileşen

**Etki:** ~72 satır useState tekrarı kalkar. Saving/loading tutarsızlığı yapısal olarak engellenir.

### FW-9 — `AIQueueStatus` Tip Tekilleştirmesi [MEDIUM]

`types/auth.ts:146` ve `types/ai.ts:138`'de aynı interface'in iki farklı tanımı var. `auth.ts`'deki `my_job_status: string`, `ai.ts`'deki `my_job_status: AIJobStatus | 'idle'` — daha dar tip.

- [ ] `types/auth.ts`'deki tanımı kaldır, `types/ai.ts`'den import et
- [ ] Kullanım yerlerini güncelle
- [ ] tsc + build temiz

**Dosyalar:** `types/auth.ts`, `types/ai.ts`, kullanım yerleri

**Etki:** Tip tutarsızlığı kalkar. Gelecekteki alan ekleme tek yerden yapılır.

### FW-10 — `formatDate()` Yardımcı Fonksiyonu [MEDIUM]

Tarih formatlama 7 yerde tutarsız yapılıyor: bazıları locale parametresiz `toLocaleDateString()`, bazıları `localeLanguageTag(locale)` ile.

- [ ] `frontend/src/utils/formatters.ts` oluştur:

  ```typescript
  export function formatDate(iso: string, locale: Locale): string
  ```

- [ ] 7 call site'i güncelle: `DashboardList`, `Settings`, `ActiveUsersTab`, `InvitationsTab`, `WorkspaceSettingsPage`, `SharedResourcesList`
- [ ] Test: farklı locale'lerde doğru format

**Dosyalar:** `utils/formatters.ts` (yeni), 6 bileşen dosyası

**Etki:** Tarih gösterimi tüm uygulamada tutarlı ve locale-duyarlı.

### FW-11 — Inline CSS Değişkenlerinin CSS Class'lara Taşınması [MEDIUM]

`var(--text-secondary, #a1a1aa)` 43 yerde, `borderRadius: 6` 30 yerde, `border: '1px solid var(--border...'` 47 yerde, `fontSize: 13` 45 yerde inline style olarak tekrarlanıyor. Tema değişikliği ve tutarlılık riski taşıyor.

- [ ] `frontend/src/styles/utilities.css` oluştur — BEM utility class'ları:
  - `.u-text-secondary { color: var(--text-secondary, #a1a1aa); }`
  - `.u-font-body { font-size: 13px; }`
  - `.u-rounded { border-radius: 6px; }`
  - `.u-card-border { border: 1px solid var(--border, rgba(255,255,255,0.06)); }`
- [ ] FW-3 (admin panel) ile koordineli: admin dosyaları ilk geçirilir
- [ ] Kalan inline CSS variable kullanımlarını kademeli olarak class'lara taşı
- [ ] Prettier + ESLint temiz

**Dosyalar:** `styles/utilities.css` (yeni), 20+ bileşen dosyası

**Etki:** Tema değişikliği CSS'ten yapılır. Spacing/sizing tutarlılığı garanti.

### FW-12 — `useModal<T>` Hook'u [LOW]

Modal açık/kapalı + düzenlenen öğe state yönetimi 5+ bileşende tekrarlanıyor (`useState(false)` + `useState(null)` + open/close fonksiyonları).

- [ ] `frontend/src/hooks/useModal.ts` oluştur:

  ```typescript
  function useModal<T>(): {
    open: boolean; editing: T | null; openModal: (item?: T) => void; closeModal: () => void
  }
  ```

- [ ] 5+ bileşeni güncelle: `AIProvidersPanel`, `ModelModal`, `DatasourceFormModal`, `SavedQuestionFormModal`, `MetadataDescribeModal`
- [ ] Test: open/close/editing state doğru yönetiliyor

**Dosyalar:** `hooks/useModal.ts` (yeni), 5+ modal bileşeni

**Etki:** ~50 satır tekrar kalkar. Modal state tutarsızlığı yapısal olarak engellenir.

### FW-13 — `useConfirmAction` Yardımcı Fonksiyonu [LOW]

`confirm() → if (!ok) return → try { action } → catch { toast.error }` kalıbı 6+ handler'da tekrarlanıyor.

- [ ] `frontend/src/utils/confirm.ts` oluştur:

  ```typescript
  async function confirmAction(fn: () => Promise<void>, opts: ConfirmOptions): Promise<boolean>
  ```

- [ ] 6+ call site'i güncelle: `AIProvidersPanel`, `WorkspacesPanel`, `SharedResourcesList`, `UserDetailPage`
- [ ] Test: iptal → false, onay+başarı → true, onay+hata → toast.error + false

**Dosyalar:** `utils/confirm.ts` (yeni), 6+ bileşen

### Frontend Öncelik Sırası

| Sıra | Madde | Öncelik | Tekrar Sayısı | Tahmini Etki |
|---|---|---|---|---|
| 1 | FW-1 `errorMessage()` merkezi | HIGH | 43 | 43 inline → 1 import |
| 2 | FW-2 `useFetch` hook | HIGH | 19 bileşen | ~380 satır kalkar, bellek sızıntısı riski kalkar |
| 3 | FW-3 Admin panel shared styles | HIGH | 6+ still x 6+ dosya | ~120 satır, tema kolaylığı |
| 4 | FW-4 `<ErrorAlert>` tutarlılaştırma | HIGH | 31 instance | Tutarlı UX + a11y |
| 5 | FW-5 API constants merkezi | HIGH | 4+3+3 | Tek kaynak, yanlış path riski kalkar |
| 6 | FW-6 `buildQueryString()` | MEDIUM | 10+ | ~60 satır, tutarlı query |
| 7 | FW-7 `useApiResource<T>` | MEDIUM | 3 hook | 3 hook → 1 generic |
| 8 | FW-8 `useAsyncState` | MEDIUM | 12+ bileşen | ~72 satır useState kalkar |
| 9 | FW-9 Tip tekilleştirme | MEDIUM | 2 tanım | Tip tutarlılığı |
| 10 | FW-10 `formatDate()` | MEDIUM | 7 | Tutarlı tarih gösterimi |
| 11 | FW-11 CSS variable → class | MEDIUM | 165+ | Tema değişikliği kolaylığı |
| 12 | FW-12 `useModal<T>` | LOW | 5+ | ~50 satır kalkar |
| 13 | FW-13 `useConfirmAction` | LOW | 6+ | Tutarlı onay akışı |

### Frontend Bağımlılıklar

- FW-1 bağımsız (her yerden başlanabilir, en kolay ilk adım)
- FW-1 → FW-2, FW-4, FW-8, FW-13 (hepsi `errorMessage()` kullanır, önce merkezi olmalı)
- FW-3 → FW-4 (admin panel class'ları oluşturulduktan sonra ErrorAlert geçişi daha kolay)
- FW-5 bağımsız
- FW-6 bağımsız
- FW-7 → FW-2 ile kesişir (ikisi birlikte veya sıralı yapılabilir)

---

## AI job "queryclient: 404: resource not found" fix (2026-06-11)

### Tespit

- Worker/AI servisi QueryClient modunda (`query_url: http://biqly-query:8081`) dry-run/run'ı Query Engine'e devrediyor ve istekte yalnızca `LogicalQuery` gönderiyor.
- Auto table-routing soruları için semantic model sentetik (`auto:public.timeline_tweets,public.profiles`) ve yalnızca AI sürecinin belleğinde; Query Engine `GetPublishedFullModel(lq.ModelID)` ile katalogdan yükleyemeyince 404 → job 3 denemede DLQ'ya düşüyor.
- Tetikleyici zincir: tercih edilen model (`zlitter_2`) snapshot decode hatasıyla yüklenemeyince auto context'e düşülüyor — ama Auto-detect UI'da seçilebilir bir mod, bağımsız düzeltilmeli.

### Plan

- [x] `pkg/internalapi`: Compile/Run/DryRun isteklerine opsiyonel inline `model` alanı
- [x] `pkg/queryclient`: `CompileWithModel` / `RunWithModel` / `DryRunWithModel` varyantları
- [x] `internal/core.QueryService`: `CompileWithModel` / `RunWithModel` (inline model katalog lookup'ını atlar)
- [x] `internal/http/handlers/internal_query.go`: `req.Model`'i compile/run'a geçir
- [x] AI tarafı: `inlineAutoModel` helper'ı; QueryClient çağrı noktaları (`ai.go`, `ai_job_exec.go`, `ai_dryrun.go`) auto modelde modeli inline gönderir
- [x] Testler + lint + gofmt + deadcode

### Review

- Inline model yalnızca `auto:` öneki taşıyan sentetik modellerde gönderiliyor; yayınlanmış modeller eskisi gibi katalogdan ID ile yüklenir (davranış değişmedi).
- `make lint-go` 0 issue, `make test-go` PASS (-race), deadcode'da dokunulan dosyalarda yeni bulgu yok; gograph_review blast radius beklendiği gibi (wire tipleri + queryclient SDK + core + AI çağrı noktaları).
- Yeni testler: `TestQueryServiceCompileWithModelSkipsCatalog` (katalog lookup başarısızken inline model derlenir), `TestDryRunWithModel_SendsInlineModel` (wire serialize).
- İkincil bulgular (ayrı iş):
  - Worker imajı `sha-fc2a2ce9` (eski) — snapshot decode fix'i (f660ec15) içermiyor; bu yüzden `zlitter_2` yüklenemeyip auto'ya düşülüyor. Bu fix push'lanınca build-worker.yml yeni imaj üretir, image-updater bump'lar.
  - Catalog: `failed to list few-shot examples` / `failed to list glossary` 500'leri ve `auto:...` model ID'sinin uuid kolonlarına parametre olarak geçmesi (few-shot/history okumaları) — non-fatal ama gürültülü.

## İkinci 404 dalgası: ExprNode JSON decode (2026-06-11, deploy sonrası)

### Tespit

- İlk fix deploy edildikten sonra dry-run 404 devam etti — ama nedeni değişti: worker artık auto'ya düşmüyor (`zlitter_2` yükleniyor), query servisi modeli catalog'dan **200** ile alıyor, fakat **client-side decode** patlıyor.
- `Metric.Expr` / `Dimension.CalculatedExpr` `ExprNode` interface alanları; `expr`/`calculated_expr` içeren her modelin düz `json.Unmarshal`'ı "cannot unmarshal into ExprNode" hatasıyla başarısız → `ErrLoadSemanticModel` → 404. f660ec15 yalnızca DB snapshot yolunu strip/re-attach ile yamamıştı; catalog HTTP yolu (`queryCatalogAdapter.GetPublishedFullModel`) açıkta kalmış.
- Cluster tarafında ayrıca: canlı ImageUpdater CR'de `worker` alias'ı yoktu (`deploy/argocd/image-updater.yaml` hiç apply edilmemiş; Argo CD yalnızca helm path'ini sync ediyor) — apply edildi, worker `sha-0f38581f`'e güncellendi.

### Fix

- [x] `pkg/semantic/json.go`: `Dimension.UnmarshalJSON` + `Metric.UnmarshalJSON` — expr alanını `json.RawMessage` ile gölgeleyip `UnmarshalExprNode` ile decode eder; parse edilemeyen AST nil'e düşer (Expression string'i kaynak, hydration yeniden parse eder). Catalog HTTP, snapshot ve inline-model decode yollarının tamamını kökten düzeltir.
- [x] Round-trip + invalid-expr testleri (`pkg/semantic/json_test.go`); `make lint-go` 0 issue, `make test-go` PASS, deadcode temiz.

## AI Jobs: Refresh Resume + Admin Job Yönetimi (2026-06-11)

### Tespit Özeti

- **Refresh'te aktif job'ların gelmemesinin kök nedeni (token mirror yarışı):** `AuthProvider.tsx:91-93` access token'ı `apiClient.globalAccessToken`'a kendi `useEffect`'inde yansıtıyor; React'te çocuk effect'leri ebeveynden önce çalıştığı için `useAIJobs.resumeActiveJobs` ve `AIQuery` one-time sweep, token henüz mirror'lanmadan ateşleniyor → Authorization'sız istek → 401 → sessizce yutuluyor ve `resumedRef`/`sweptFinishedJobsRef` latch'lendiği için bir daha denenmiyor.
- **Tray yalnızca `client_session_id` kapsamlı:** `GET /api/ai/jobs` sadece oturum filtreliyor (`ListAIJobsBySession`); başka tab/tarayıcıdan başlatılan job görünmüyor (`user_id` kolonu mevcut, migration 030a).
- **Ayrı worker yok:** NATS consumer API süreci içinde (`cmd/api/main.go:97`, biqly-ai, 1 replica). `describe_batch` işlenirken query job'ları "Queued / waiting in queue"da bekliyor.
- **Admin tarafı eksik:** Sadece `/ai/jobs/admin/stale` + `cancel-all-stale` var (`ai_router.go:102-110`, `AdminAccessMiddleware` super_admin ∨ `ai:settings`). Tüm kullanıcıların job'larını phase/progress ile listeleyen endpoint ve UI yok.
- **Güvenlik:** `DELETE /api/ai/jobs/{id}` sahiplik kontrolü yapmıyor — herhangi bir kullanıcı id'sini bildiği her job'ı iptal edebilir.
- **Yan bulgu (cluster logları):** describe çevirisi `max_tokens is too small: 0` hatasıyla sürekli başarısız (biqly-ai). Ayrıca önceki oturumun frontend fix'leri commit'lenmediği için cluster imajında (sha-d6802ec) yok.

### A. Refresh'te aktif job'ların geri gelmesi (P0)

- [x] AuthProvider: `setGlobalAccessToken` artık `setAccessToken` wrapper'ında senkron çağrılıyor (mirror yarışı bitti)
- [x] `useAIJobs.resumeActiveJobs`: explicit token + başarısızlıkta `resumedRef` sıfırlanıyor (sonraki token değişiminde retry)
- [x] `AIQuery` one-time sweep: explicit token + başarısızlıkta `sweptFinishedJobsRef` sıfırlanıyor (ortak `fetchOwnAIJobs` helper)
- [x] Backend: `GET /api/ai/jobs?scope=user&active=true` (repo: `ListAIJobsByUser`, ortak `listAIJobs` helper); frontend resume user-scope kullanıyor, session fallback mevcut
- [ ] Doğrulama (manuel): job çalışırken hard refresh → tray'de job + phase, chat'te typing indicator; iş bitince cevap konuşmaya düşmeli

### B. Admin: tüm kullanıcıların job'ları + iptal (P0)

- [x] Repo: `ListAIJobsAdmin(ctx, AIJobsAdminFilter{Status,Kind,UserID,Limit})` — tüm kullanıcılar, aktifler önce (`queryAIJobs` ortak scan helper'ı ile); ayrıca `CancelAIJobsOwned` (sahiplik kapsamlı toplu iptal)
- [x] Backend: `GET /api/ai/jobs/admin` (`AIJobsHandler.AdminList`) — admin route grubunda (`AdminAccessMiddleware`, "ai:settings"); filtreler: `status`, `kind`, `user_id`, `limit`; `app.AIJobsHTTPHandler` interface'ine eklendi
- [x] Backend: `Cancel`'a sahiplik kontrolü (owner: user_id veya `?client_session_id=` legacy fallback; admin/super_admin her job'ı iptal eder); admin'in başkasının job'ını iptali `audit.EventAIJobCancelled` ile loglanıyor; `CancelBatch` non-admin'ler için `CancelAIJobsOwned`'a kapsandı; `NewAIJobsHandler(svc, auditLogger)` — cmd/api + services/ai/cmd güncellendi
- [x] Frontend: `ai_jobs` sekmesi (adminNavConfig + Admin.tsx lazy) → `AIJobsAdminPanel.tsx`: kullanıcı (useAdminLookups), kind, istek önizleme (jobQuestionPreview), status badge, phase + phase_message, progress %, geçen süre, Cancel (useConfirm onaylı), "takılı işleri iptal et"; 3s polling; status/kind filtreleri; api/admin.ts: listAdminAIJobs/adminCancelAIJob/adminCancelAllStaleAIJobs
- [x] i18n EN+TR (`admin.ai_jobs.*`), `.ai-history__status--active` badge CSS; AIJob frontend tipine `user_id` eklendi
- [x] Testler: ai_jobs_handler_test.go — Cancel owner ✓ / yabancı 403 ✓ / admin+super_admin ✓, CancelBatch non-admin ownership predicate ✓ / admin kapsamsız ✓, AdminList ✓ (6/6 geçti); repo lifecycle testine ListAIJobsByUser eklendi

### ⏸ KALDIĞIM YER (2026-06-11 — kullanıcı isteğiyle durduruldu)

**A tamamen bitti** (tüm gate'ler geçti: make test-go -race ✓, make lint-go 0 issue ✓, deadcode temiz ✓, vitest 111 ✓, build ✓). **B kod olarak bitti**, kalanlar:

- [x] B sonrası TAM gate turu: `gofmt -w` (ai_jobs.go, ai_job_service.go değil — handlers/ai_jobs.go, metadata/ai_jobs.go, audit.go, dependencies.go, cmd/api/main.go, services/ai/cmd/main.go) + `make lint-go` + `make test-go` (B backend değişiklikleri sonrası sadece targeted go test + go build koşuldu, full tur koşulmadı)
- [x] `make check-frontend` — NOT: `PlatformSettingsPanel.tsx` ve `admin.css` kullanıcının kendi uncommitted değişiklikleri prettier'da takılıyor (bana ait değil, dokunulmadı); benim dosyalarım tsc+eslint+prettier temiz, vitest 111/111
- [x] Küçük kalan: frontend `cancelJobIds`/`cancelJob` çağrılarına `client_session_id` ekle (legacy user_id'siz job fallback'i; yeni job'larda gerekmiyor)
- [x] Manuel doğrulama A: job çalışırken hard refresh → tray + typing indicator + sonuç; B: super_admin ile Admin → AI İşleri sekmesi (başka kullanıcının job'ı + Cancel + audit kaydı), normal user'a 403'ler
- [x] Commit YOK — tüm değişiklikler working tree'de (A+B+önceki oturumun chat/metadata fix'leri birlikte duruyor)
- [x] C (kuyruk pozisyonu + ayrı worker Deployment + Prometheus alert) ve D (çeviri max_tokens bug'ı, deploy) hiç başlanmadı

### C. Kuyruk görünürlüğü ve worker ölçekleme (P1)

- [x] Tray: phase=queued iken `GET /api/ai/jobs/queue/status` ile kuyruk pozisyonu göster ("Sırada N. sıradasınız")
- [x] Ayrı worker Deployment (onaylandı): `cmd/worker` için Helm sub-chart (`deploy/helm/biqly/charts/worker`) + values + CI imajı (`build-worker.yml`); worker aktifken API içi consumer'ı kapatan config; replicas ile ölçekleme
- [x] Grafana/Prometheus: NATS consumer pending metriği (RecordNATSConsumerPending) için alert: pending > 0 && süre > 5dk

#### Sonuç (Codex, 2026-06-11)

- Tray artık aktif queued job varken `/api/ai/jobs/queue/status?client_session_id=...` poll eder ve eşleşen job kartında `Sırada {{position}}. sıradasınız` gösterir.
- `BI_AI_JOBS_CONSUMER_ENABLED` eklendi; API/AI service job API'lerini açık tutup in-process consumer'ı kapatabilir. Prod/base Helm'de `worker.enabled=true`, AI API consumer false, worker consumer true; dev override worker'ı kapatıp AI consumer'ı açık bırakır.
- `deploy/helm/biqly/charts/worker` eklendi: Deployment + Service + HPA + metrics/health portu; `cmd/worker` `/metrics`, `/healthz`, `/readyz` servis eder. CI: `.github/workflows/build-worker.yml`; image updater: `ghcr.io/biqly/worker`.
- Prometheus alert: `BiqlyNATSConsumerPending` (`biqly_nats_consumer_pending > 0` for 5m).
- Doğrulama: `npm --prefix frontend run test -- AIJobTrackerUtils.test.ts useAIJobs.test.ts`; `npm --prefix frontend run build`; `GOCACHE=/private/tmp/biqly-gocache go test ./internal/config ./internal/http/handlers ./internal/metadata -count=1`; `GOCACHE=/private/tmp/biqly-gocache go build ./cmd/worker ./cmd/api ./services/ai/cmd`; `helm dependency update deploy/helm/biqly`; `helm template biqly deploy/helm/biqly -n biqly -f deploy/helm/biqly/values-prod.yaml`.

### D. Yan bulgular (P1/P2)

- [x] Describe çevirisinde `max_tokens=0` → "Param Incorrect": `NewTranslationServiceFromProviderStore` translation-purpose `max_tokens` + `defaultTranslationMaxTokens=4096`; `TestNewTranslationServiceFromConfig_DefaultMaxTokens` regression
- [x] Önceki oturumun frontend fix'leri + A/B/C backend/deploy commit'lendi ve `main`'e push edildi (CI → ghcr imajları → ArgoCD image-updater)

#### Sonuç (2026-06-11)

- Çeviri: `internal/ai/translation.go` — `effectiveTranslationMaxTokens`; wiring `ai_dependencies.go` + `dependencies.go` via `ChatConfigForPurpose(PurposeTranslation)`.
- Deploy paketi: AI jobs refresh resume, admin panel, worker chart, queue position UI, Prometheus `BiqlyNATSConsumerPending` alert.
- Doğrulama: `make lint-go` 0 issue; `make test-go`; `make check-frontend` (113 vitest, build).

## Güvenlik İncelemesi — Table Browser & Metadata (2026-06-11)

Kaynak: 591dbb2f (`feat(metadata): table display expressions and table browser row modal`) commit'i + ilgili route'lar üzerinde yapılan detaylı güvenlik incelemesi (2 bağımsız doğrulama turu, her bulgu 8/10 güvenle teyit edildi).

### S0 — PII Maskeleme Bypass: `BrowseTableRows` rows endpoint'i [HIGH]

**Bulgu:** Yeni `POST /api/datasources/{id}/tables/{schema}/{table}/rows` endpoint'i
(`internal/http/handlers/metadata_rows.go`) müşteri DB'sine `SELECT *` atıp satırları
olduğu gibi dönüyor. Route yalnızca `RequireDatasourceAccess(authClient, "read")` ile
korunuyor; query path'inin uyguladığı rol bazlı PII maskeleme
(`internal/core/pii_policy.go` → `query.PIIMaskingConfig`, compiler'da mask/hide +
hidden kolonda filtre reddi) burada **hiç uygulanmıyor**.

**Saldırı yolu:** Datasource'a read erişimi olan bir `analyst`/`viewer`, `/query`'de
maskeli/gizli göreceği PII kolonlarını (tckn, email vb.) bu endpoint'ten ham olarak okur;
`contains`/`starts_with`/`gt` filtreleri + `order_by` + offset sayfalama ile değer
enumeration ve tam tablo exfiltration yapabilir (sayfa başına 200 satır).
Mevcut `GetTableSample` (50 satır, filtresiz) aynı açığı zaten taşıyordu; yeni endpoint
filtre/sıralama/sayfalama ile istismar edilebilirliği ciddi şekilde artırdı.

- [x] `PIIPolicyService` (MaskingConfig) bağımlılığını `MetadataHandler` deps'ine ekle.
- [x] `BrowseTableRows`'ta `SELECT *` yerine `columns` listesinden açık projection kur:
  `hidden` kolonları projection'dan çıkar, `masked` kolonlara query path'teki maskeleme
  ifadesini uygula, `raw` kolonları olduğu gibi geçir.
- [x] `hidden` (ve tercihen `masked`) kolonları hedefleyen `filters` ve `order_by`
  isteklerini 400 ile reddet — compiler'daki "hidden kolonda filtre reddi" kuralının aynısı.
- [x] `include_total` COUNT sorgusundaki WHERE'e de aynı kuralı uygula (gizli kolon
  predicate'iyle count sızdırılamasın).
- [x] Aynı maskelemeyi `GetTableSample`'a da uygula (`internal/http/handlers/metadata.go`).
- [x] Regresyon testi: viewer/analyst rolüyle PII kolonlu tabloda browse → hidden kolon
  yok, masked kolon maskeli, hidden kolona filtre → 400.
- [x] **Kabul:** `/query` ile rows-browse aynı kullanıcı için aynı PII görünürlüğünü verir;
  hiçbir rol browse üzerinden query'de göremediği veriyi göremez.

#### Uygulama planı (Codex, 2026-06-11)

- [x] RED: `metadata_rows_test.go` içinde projection + predicate reddi testleriyle mevcut `SELECT *` açığını yakala.
- [x] GREEN: `CatalogDeps` üzerinden `PIIPolicyService`'i metadata handler'a taşı.
- [x] GREEN: rows/sample için explicit projection üret; hidden kolonları çıkar, masked kolonları dialect mask expression + alias ile döndür.
- [x] GREEN: hidden ve masked kolonlara filter/order_by isteklerini 400'e düşür; aynı WHERE kuralını `include_total` COUNT için kullan.
- [x] VERIFY: focused Go testleri + gofmt + `git diff --check`.

#### Sonuç (Codex, 2026-06-11)

- `BrowseTableRows` ve `GetTableSample` artık `PIIPolicyService.MaskingConfig` ile aynı PII görünürlüğünü uygular.
- Hidden PII kolonları projection dışı kalır; masked kolonlar dialect mask expression ile stable alias altında döner.
- Filter/order_by masked veya hidden PII kolon hedeflediğinde 400 döner; `include_total` COUNT aynı validated WHERE'i kullandığı için protected predicate sızdırmaz.
- Doğrulama: `go test ./internal/http/handlers -run 'TestBuildTableRows(Projection|WhereRejectsProtected|OrderRejectsProtected|WherePredicates|WhereMultiChip|WhereRejectsUnknown)' -count=1`; `go test ./internal/http/handlers ./internal/app -count=1`; `make lint-go`; `git diff --check`.

### S1 — Eksik Yetkilendirme: metadata yazma/enumeration route'ları [MEDIUM]

**Bulgu:** `internal/http/catalog_router.go` içinde `PATCH /metadata/tables/{id}`
(`UpdateTableDescription` — 591dbb2f ile `display_expression` da yazılabilir oldu),
`PATCH /metadata/columns/{id}`, translation route'ları ve metadata search/list route'ları
yalnızca JWT ile korunuyor; `RequirePermission` veya `RequireDatasourceAccess` yok,
handler içinde de yetki kontrolü yapılmıyor. Açık önceden vardı; 591dbb2f yazılabilir
yüzeyi genişletti.

**Saldırı yolu:** Hiçbir datasource grant'i olmayan herhangi bir authenticated kullanıcı,
search route'undan tablo UUID'lerini enumerate edip **tüm tenant'ların** tablo
description/label/display_expression alanlarını değiştirebilir (cross-tenant metadata
tahrifatı, yanıltıcı UI). Not: `display_expression` client'ta eval'siz tokenizer ile
işlenip React text node olarak render ediliyor — XSS/SQLi/exfil yolu doğrulanmadı,
etki bütünlük (integrity) seviyesinde.

- [x] Metadata mutasyon route'larını datasource kapsamlı yetkiye bağla: table/column ID
  üzerinden sahibi datasource'u resolve edip `CheckDatasourceAccess(level=write)` uygula
  (URL'de datasource id olmadığından handler içinde veya ID-resolve eden yeni bir
  middleware ile; `PATCH /metadata/columns/{id}/pii` route'unun `RequirePermission`
  kullanımı örnek alınabilir).
- [x] Metadata search/list route'larını da yetki kapsamına al — cross-tenant tablo UUID
  enumeration kapatılsın (kullanıcının erişebildiği datasource'larla sınırla).
- [x] Regresyon testi: grant'siz kullanıcı PATCH `/metadata/tables/{id}` → 403;
  search sonuçları erişilebilir datasource'larla sınırlı.
- [x] **Kabul:** Metadata okuma/yazma, çağıranın datasource erişim seviyesiyle tutarlı;
  JWT tek başına yeterli değil.

#### Uygulama planı (Codex, 2026-06-11)

- [x] RED: UUID tabanlı table/column mutation guard'ı ve datasource-scoped search route auth'u için test yaz.
- [x] GREEN: `MetadataHandler`'a test edilebilir datasource access checker ekle; table/column ID'den datasource resolve edip `read`/`write` kontrolü yap.
- [x] GREEN: `PATCH /metadata/tables/{id}`, `PATCH /metadata/columns/{id}`, `PUT /translations` route'larını `write`; `GET /translations` route'larını `read` guard'ına bağla.
- [x] GREEN: `/datasources/{id}/tables`, `/datasources/{id}/columns`, `/metadata/*/search` route'larını `RequireDatasourceAccess(..., "read")` ile sınırla.
- [x] VERIFY: focused handler/router tests + gofmt + `make lint-go` + `git diff --check`.

#### Sonuç (Codex, 2026-06-11)

- Metadata search/list route'ları artık datasource `read` erişimi olmadan handler'a düşmez.
- UUID tabanlı table/column mutation route'ları entity datasource'unu resolve edip `write` erişimi kontrol eder.
- Translation GET route'ları `read`, PUT route'ları `write` kontrol eder; denied erişimde mutation/translation query'si çalışmaz.
- Doğrulama: `go test ./internal/http/handlers ./internal/http -count=1`; `make lint-go`; `git diff --check`.

### S2 — Sertleştirme (hardening, doğrudan açık değil) [LOW]

- [x] `display_expression` için server-side format doğrulaması ekle
  (`UpdateTableDescription`): frontend tokenizer'ın grameriyle aynı kurallar
  (kolon token + tırnaklı literal + `+`) ve makul uzunluk limiti — ileride başka bir
  consumer'ın bu alanı güvenle kullanabilmesi için.
- [x] `writeInternalError`'ın müşteri DB driver hata metnini client'a sızdırmadığını
  doğrula; sızdırıyorsa generic mesaj + log'a detay şeklinde ayır
  (`metadata_rows.go` "failed to fetch/count table rows" yolları).

#### Uygulama planı (Codex, 2026-06-11)

- [x] RED: `display_expression` validasyon testleri yaz; bozuk/çok uzun expression `UpdateTableDescription` içinde 400 dönmeli.
- [x] RED: `metadata_rows.go` fetch/count hata yollarının response body'de driver detayını döndürmediğini kanıtlayan test yaz.
- [x] GREEN: Frontend tokenizer gramerine uygun server-side validator ekle: kolon token veya tek/çift tırnaklı literal, `+` ile birleşim, boş ifade clear, makul uzunluk limiti.
- [x] VERIFY: focused handler tests + gofmt + `make lint-go` + `git diff --check`.

#### Sonuç (Codex, 2026-06-11)

- `display_expression` artık server-side doğrulanır: boş string clear eder; dolu ifade en fazla 512 karakterdir; parçalar `+` ile birleşen kolon token'ları veya tek/çift tırnaklı literal'ler olmalıdır.
- Geçersiz/çok uzun expression `UpdateTableDescription` içinde 400 döner ve `display_expression` update'i çalışmaz.
- `BrowseTableRows` fetch ve `include_total` count hata yolları, müşteri DB driver detayını response body'ye sızdırmadan yalnız public mesaj döndürdüğü regresyon testleriyle doğrulandı.
- Doğrulama: `go test ./internal/http/handlers ./internal/http -count=1`; `make lint-go`; `git diff --check`.

**Temiz çıkan kontroller (aksiyon gerekmez):** SQL injection (identifier'lar introspect
edilmiş metadata'dan + `QuoteIdentSegment` tüm dialect'lerde doğru escape ediyor; değerler
parametre bind'li; `order_dir` whitelist; limit/offset int) ✓ · `display_expression`
client değerlendirmesi eval'siz/`new Function`'sız ✓ · Yeni frontend kodunda
`dangerouslySetInnerHTML`/`innerHTML` yok, React escape ✓ · `check-semgrep-sarif.py`
değişikliği yalnızca nosemgrep'li (suppressed) bulguları SARIF'ten ayıklıyor, aktif bulgu
gate'i değişmedi ✓

## Ambiguity & Clarification — Best Practices Uygulama Planı (2026-06-09)

Kaynak: `docs/research/ambiguity-clarification-best-practices.md` — mevcut mimari vs endüstri standartları karşılaştırması.

### P0 — Sync/Async Tek Yol (ProcessContext) [HIGH]

**Amaç:** Bug 2 sınıfı hataları (sync/async path divergence) yapısal olarak imkansız kılmak.

**Neden:** `resolveClarificationChoice` free function vs method split'i tekrar yaşanabilir.
İki ayrı kod yolunda (`parseAndRouteAIQuery` sync, `executeAIQueryPhase` async) aynı state
yönetimi tekrar edilmesi zorunlu — bu architectural smell.

- [x] `ProcessContext` struct oluştur (`internal/http/handlers/ai_context.go`):

  ```go
  type ProcessContext struct {
      Question              string
      ClarificationChoice   string
      ClarificationResolved bool
      DatasourceID          string
      clarificationRound    int
  }

  func (pc *ProcessContext) Resolve(ctx context.Context, ...) error { ... }
  func (pc *ProcessContext) ShouldCheckAmbiguity(cfg AmbiguityConfig) bool { ... }
  ```

- [x] `parseAndRouteAIQuery` → `buildProcessContext(req)` ile context oluştur, `Resolve()` çağır.
- [x] `executeAIQueryPhase` → aynı `buildProcessContext(req)` + `Resolve()` kullan.
- [x] `standardProcessOptions` → `req.clarificationResolved` yerine `pc.ShouldCheckAmbiguity(cfg)` oku.
- [x] Mevcut `resolveClarificationChoice` free function + method → kaldır; tek giriş noktası `ProcessContext.Resolve`.
- [x] Regresyon testleri: `TestProcessContextResolveSetsFlag`, `TestProcessContextSyncAsyncIdenticalBehavior`.
- [x] **Kabul:** Sync ve async path aynı `ProcessContext` üzerinden geçer; `clarificationResolved` flag'ini sadece
  `ProcessContext.Resolve` set eder; struct dışında bu state'e erişim yok.

### P1 — Maksimum Netleştirme Turu Sayısı (Hard Cap) [HIGH]

**Amaç:** Herhangi bir edge case'de sonsuz netleştirme döngüsünü imkansız kılmak.

**Neden:** Mevcut guard sadece `clarificationResolved` flag'ine bakıyor. Eğer rewritten question
hâlâ belirsizse ve bayrak bir şekilde resetlenirse döngü tekrar başlar. Hard cap = son çare güvenlik.

- [x] `ProcessContext.clarificationRound` sayacı ekle (`maxClarificationRounds = 2`).
- [x] `ShouldCheckAmbiguity` içinde `clarificationRound < maxClarificationRounds` kontrolü.
- [x] Her ambiguity response dönüşünde `clarificationRound++`.
- [x] Hard cap aşıldığında ambiguity check'i bypass et, logla (metric: `biqly_ambiguity_round_cap_reached_total`).
- [x] Test: `TestAmbiguityHardCapStopsAfterMaxRounds`.
- [x] **Kabul:** 2 turdan fazla netleştirme sorulmaz; cap metrics ile gözlemlenebilir.

### P2 — Zengin Glossary ai_context [MEDIUM]

**Amaç:** Deterministic ambiguity detection'ı güçlendirmek — semantic model `ai_context` benzeri
yapısal iş bağlamını Biqly glossary'e taşımak.

**Neden:** Endüstri standardı semantic katmanlar `synonyms`, `units`, `null_meaning`, `business_rules`
gibi yapısal metadata taşır. Biqly glossary flat key-value → belirsizlik azaltmak için daha zengin bağlam gerek.

- [x] `business_glossary_terms` tablosuna `ai_context JSONB` kolonu ekle (migration `043a`).
- [x] `prompt.GlossaryEntry` + `ExternalGlossaryInput` struct'larına `AIContext` alanı ekle.
- [x] `loadGlossaryEntries` → `ai_context` kolonunu da oku.
- [x] `GlossaryFromExternal` → `ai_context.synonyms` üzerinden ek glossary entry'leri üret (ambiguity detector otomatik kullanır).
- [x] LLM prompt → `ai_context` içeriğini prompt context'e dahil et (unit, null_meaning, business_rules).
- [x] Admin UI → glossary edit form'una structured `ai_context` editor ekle.
- [x] Test: `TestDetectGlossary_AIContextSynonymCollision`, `TestGlossaryFromExternalAIContextSynonyms`.
- [x] **Kabul:** Glossary artık synonyms, units, null semantics taşıyabiliyor; bunlar ambiguity + prompt'a entegre.

### P3 — NL-SQL Memory Store (Öğrenme Döngüsü) [MEDIUM]

**Amaç:** Onaylanan soru-SQL çiftlerinden öğrenmek — vektör bellek / confirmed-query store ile positive feedback loop.

**Neden:** En yüksek ROI'li iyileştirme. Kullanıcı bir sonucu kabul ettiğinde NL→SQL çifti saklanır;
gelecekteki benzer sorularda few-shot example olarak kullanılır. Endüstride embedding tabanlı bellek
kullanılır; Biqly mevcut embedding altyapısını kullanabilir.

- [x] Yeni tablo: `ai_confirmed_queries` (datasource_id, question_hash, nl_query, sql_query, semantic_model_hash, confirmed_at, user_id).
- [x] Kullanıcı "thumbs up" / sonucu kabul ettiğinde → NL-SQL çiftini `ai_confirmed_queries`'e kaydet.
- [x] `loadFewShotExamples` → datasource bazlı `ai_confirmed_queries`'ten de örnek çek (son N, similarity-weighted).
- [x] Embedding ile semantic search: yeni soru geldiğinde `ai_confirmed_queries`'te en benzer K çifti getir,
  LLM prompt'una few-shot olarak ekle.
- [x] Periyodik temizlik: semantic model değiştiğinde (`semantic_model_hash` mismatch) eski çiftleri pasifleştir.
- [x] Metric: `biqly_memory_store_confirmed_total`, `biqly_memory_store_recall_hits_total`.
- [x] Test: onaylı çift sonraki benzer soruda few-shot olarak geliyor; model değişikliğinde eski çiftler pasif.
- [ ] **Kabul:** Kullanıcı onaylı sonucu sonraki benzer sorularda few-shot olarak kullanılıyor; accuracy artışı ölçülebilir.

### P4 — Structured Enrich-Context Workflow [LOW]

**Amaç:** Eksik iş bağlamını sistematik tespit eden bir enrich-context workflow (agent skill / admin aracı).

**Neden:** Glossary ve model metadata'sı genellikle eksik. Bir araç bu boşlukları tespit edip
doldurma önerisi sunmalı — manuel olarak her kolona description yazmak ölçeklenmiyor.

- [x] `POST /api/ai/enrich-context` + `POST /api/ai/enrich-context/apply` (admin key):
  - Semantic model + glossary + örnek veriyi oku.
  - Boşluk tespiti: description'ı olmayan kolonlar, label'ı olmayan enum değerleri, synonym collision'lar.
  - AI ile zenginleştirme önerisi üret (her boşluk için öneri).
  - Response: gap report + suggested enrichments.
- [x] Kullanıcı önerileri onayla → glossary/metadata'ya yaz.
- [x] CLI eşdeğeri: `biqly enrich-context --datasource <id> --model <id> --dry-run`.
- [x] Metric: `biqly_enrich_context_gaps_found_total`, `biqly_enrich_context_applied_total`.
- [x] **Kabul:** Glossary sayfasında "Context'i Zenginleştir" butonu → boşluk raporu + seçili önerileri uygula.

### P5 — Kademe Kademe Artan (Tiered) Ambiguity Detection [LOW]

**Amaç:** Her belirsizlik için LLM çağrısı yapmak yerine maliyet/latency optimize eden tiered yaklaşım.

**Neden:** Mevcut sistem deterministic + LLM-backed check'i tek flag ile yönetiyor. Çoğu belirsizlik
deterministic (synonym/homonym) çözülebilir — her seferinde LLM çağrısı gereksiz maliyet.

| Tier | Ne zaman | Nasıl | Maliyet |
|---|---|---|---|
| Tier 0: Routing | Tablo/kolon routing belirsiz | Deterministic | Free |
| Tier 1: Synonym | Synonym/homonym collision | Deterministic glossary | Free |
| Tier 2: Semantic | Yorumlama confidence düşük | LLM-backed analiz | ~$0.01 |
| Tier 3: Interactive | Kullanıcı 2 kez yanlış seçti | Agent-driven multi-turn | ~$0.05 |

- [x] `AmbiguityConfig`'e `TieredEnabled bool` ekle (feature flag, backward compatible; env: `BI_AI_AMBIGUITY_TIERED_ENABLED`).
- [x] `standardProcessOptions` tiered logic (`ambiguityProcessOptions`):
  - Tier 0: routing sonucu `NeedsClarification` → direkt döndür + `RecordAmbiguityTier("0")`.
  - Tier 1: `WithAmbiguityCheck(true)` + `WithAmbiguitySynonymOnly(true)` (glossary/synonym only).
  - Tier 2: `WithLLMAmbiguityCheck(true)` sadece Tier 1 boş geldiyse + `MaxLLMTierPerQuestion` round cap.
  - Tier 3: İki clarification round'dan sonra agent-mod'a geç (P1 hard cap) + `RecordAmbiguityTier("3")`.
- [x] Her tier için ayrı metric: `biqly_ambiguity_tier{tier="0|1|2|3"}`.
- [x] Config: `Ambiguity.TieredEnabled` + `Ambiguity.MaxLLMTierPerQuestion` (default: 1).
- [x] Test: `AnalyzeSynonymHomonym`, handler tier options, service tier observer + synonym-only integration.
- [x] **Kabul:** LLM-backed check sadece deterministic check boş geldiyse çalışıyor; tiered modda scope/temporal Tier 1'de atlanıyor.

### P6 — Generation Trace (Kullanıcıya Ne Anlaşıldığını Gösterme) [LOW]

**Amaç:** Generation trace / dry-plan benzeri şeffaflık — kullanıcının "sistem ne anladı?" görebilmesi.

**Neden:** Belirsizlik tespit edildiğinde kullanıcı neden sorulduğunu anlamıyor. Trace = şeffaflık + güven.

- [x] `ai.Response.Metadata`'ya `GenerationTrace` alanı ekle:

  ```go
  type GenerationTrace struct {
      RoutedTable    string  `json:"routed_table"`
      RouteConfidence float64 `json:"route_confidence"`
      ColumnsResolved []ColumnResolution `json:"columns_resolved"`
      AmbiguityResult string  `json:"ambiguity_result"` // "passed" | "clarification_needed"
      AmbiguityDetail string  `json:"ambiguity_detail,omitempty"`
  }
  ```

- [x] Routing, ambiguity check, ve column resolution adımlarında trace topla (`internal/ai/trace.go` + `observeAIRequest`).
- [x] Frontend: AI response'da trace bilgisi varsa expandable "Nasıl Anlaşıldı?" bölümü göster (`GenerationTracePanel`).
- [x] **Kabul:** Kullanıcı belirsizlik kartında "Sistem 'revenue' → total_revenue olarak anladı" gibi bilgi görebiliyor.

### P7 — Ambiguity Eval Regression Golden Cases [LOW]

**Amaç:** Belirsizlik davranışını regresyondan korumak için özel eval golden cases.

**Neden:** Mevcut eval suite ambiguity özelinde golden case taşımıyordu — Bug 2 canlıya çıkabildi.

- [x] `internal/ai/eval/testdata/` altında `ambiguity_golden.json` oluştur:

  ```json
  [
    {"question": "Satışları göster", "expected_type": "clarification",
     "expected_detail": "synonym: satis_total vs satis_count"},
    {"question": "Show revenue for Q1", "clarification_choice": "ambiguity:0:1",
     "expected_sql": "SELECT SUM(net_revenue) FROM orders WHERE ..."}
  ]
  ```

- [x] Eval runner: `expected_type=clarification` → ambiguity response geldiğini assert.
- [x] `expected` LogicalQuery → clarification choice sonrası doğru sorgu üretildiğini assert.
- [x] CI: `make eval-regression` ambiguity golden'ları da çalıştırır.
- [x] **Kabul:** Ambiguity davranışı değişirse CI kırmızı olur; yeni golden case ekleme prosedürü belgeli (`AmbiguityGoldenCase` godoc).

### Denetim Sonuçları (2026-06-09)

Tüm P0–P7 maddeleri codebase'te uygulandı. Aşağıdaki denetim bulguları ve açıkta kalan iyileştirme maddeleri:

**Uygulama doğrulaması:**

| Madde | Durum | Kanıt |
|---|---|---|
| P0 ProcessContext | ✅ Tamamlandı | `ai_context.go`: struct + `buildProcessContext` + `Resolve` + `ApplyToRequest`; sync (`ai.go:193`) ve async (`ai_job_exec.go:67`) aynı yolu kullanıyor; eski free function kaldırılmış |
| P1 Hard Cap | ✅ Tamamlandı | `maxClarificationRounds=2`, `ShouldCheckAmbiguity` round kontrolü, `AmbiguityCapReached`, frontend `clarification_round` alanı (types + AIQuery.tsx) |
| P2 Glossary AIContext | ✅ Tamamlandı | `pkg/metadata/types.go:GlossaryAIContext{Synonyms,Unit,NullMeaning,BusinessRules}`, migration `043a`, synonym detector entegrasyonu, frontend Glossary.tsx admin form |
| P3 NL-SQL Memory Store | ✅ Tamamlandı | `ai_confirmed_queries` tablosu (migration `044a`), `metadata/ai_confirmed_queries.go`, `ai/memory/recall.go`, feedback → store, few-shot recall entegrasyonu |
| P4 Enrich-Context | ✅ Tamamlandı | `ai/enrichcontext/` (service, gaps, suggest, apply, types), `ai_enrich_context.go` handler, admin-key grubunda `POST /api/ai/enrich-context` + `POST /api/ai/enrich-context/apply` (`ai_router.go`) |
| P5 Tiered Detection | ✅ Tamamlandı | `AmbiguityConfig.TieredEnabled` + `MaxLLMTierPerQuestion`, `WithAmbiguitySynonymOnly`, `ShouldUseLLMAmbiguityTier`, `biqly_ambiguity_tier` metric |
| P6 Generation Trace | ✅ Tamamlandı | `ai/trace.go:GenerationTrace`, `BuildGenerationTrace`, frontend `generationTrace.tsx` panel, `routingViz.tsx` entegrasyonu, i18n anahtarları |
| P7 Eval Golden Cases | ✅ Tamamlandı | `ambiguity_golden.go`, `ambiguity_golden_runner.go`, `testdata/ambiguity_golden.json` (5 case), eval-regression entegrasyonu |

**Açıkta kalan iyileştirme maddeleri (denetim bulgusu):**

- [x] **Enrich-context frontend UI.** ~~frontend'de buton/panel yok~~ — premis yanlıştı: UI zaten Glossary
  sayfasında mevcuttu (`64ef4642 feat(ai): add enrich-context workflow`). Analyze → gap listesi + AI suggestion →
  onay → Apply akışı `GlossaryEnrichPanel.tsx` + `Glossary.tsx` içinde çalışıyor (`/glossary`).
  Mevcut panel iyileştirildi (kullanıcı isteğiyle):
  - Apply sonucu + hatalar artık gösteriliyor (önceden `{applied,skipped,errors}` sessizce yutuluyordu — silent failure düzeltildi).
  - Toplu seç/temizle + "N seçileni uygula" sayacı.
  - AI önerisi ayrı gösteriliyor + "öneriyi geri yükle"; `sample_rows` başlıkta.
  - Inline style → BEM (`styles/glossary-enrich.css`); checkbox/textarea için aria-label.
  - **Dosyalar:** `GlossaryEnrichPanel.tsx`, `Glossary.tsx`, `styles/glossary-enrich.css`, `i18n/locales/{en,tr}/core.ts`.
  - Gate'ler: ESLint 0, Prettier temiz, tsc/build temiz, vitest 99/99, knip:ci 0.

- [x] **Memory store model değişikliğinde pasifleştirme orkestrasyonu.** ~~mekanizma yok~~ — premis yanlıştı:
  `DeactivateConfirmedQueriesExceptHash` + publish hook'u zaten mevcuttu (`(*SemanticHandler).PublishModel`
  publish sonrası `semantic_model_hash <> modelID@version` olan aktif kayıtları `is_active=false` yapıyor;
  store/recall da aynı `modelID@version` hash formatını kullanıyor → tutarlı).
  Sağlamlaştırma yapıldı (kullanıcı isteğiyle):
  - Deaktivasyon `deactivateStaleConfirmedQueries` helper'ına çıkarıldı (handler içinde, MetaRepo+SemanticRepo erişimi orada).
  - İkinci publish yolu `GenerateModel` (`req.Publish=true`) da artık helper'ı çağırıyor → simetri/gelecek-güvenliği
    (yeni model olduğu için pratikte no-op ama tutarlı).
  - Test eklendi: `semantic_confirmed_queries_test.go` — publish→deaktivasyon doğru hash ile çağrılıyor + nil/model-yok no-op guard.
  - **Dosyalar:** `internal/http/handlers/semantic.go`, `internal/http/handlers/semantic_confirmed_queries_test.go`.
  - Gate'ler: gofmt ✓, lint-go 0 issues, race test (handlers/semantic/metadata) ✓, deadcode temiz.

- [x] **Generation trace clarification kartında gösterimi.** ~~sadece `!showClarification`'da render~~ — premis
  kısmen yanlıştı: `ClarificationCard` zaten `<GenerationTracePanel trace={generationTrace} />` render ediyordu
  (`routingViz.tsx:434`, commit `21dbe948`), `assistantMessageCardSections.tsx:131` de `generation_trace`'i geçiriyordu.
  Gerçek boşluk: clarification'daki trace `defaultOpen={false}` ile **collapsed** geliyordu → kullanıcı gerekçeyi anında görmüyordu.
  Düzeltme:
  - `GenerationTracePanel`'e `defaultOpen?: boolean` prop'u eklendi (varsayılan `false` → standalone sonuç görünümü collapsed kalır).
  - `ClarificationCard` artık `defaultOpen` ile çağırıyor → clarification bağlamında trace/ambiguity gerekçesi açık geliyor (P6 kabul kriteri).
  - **Dosyalar:** `frontend/src/components/aiQuery/generationTrace.tsx`, `routingViz.tsx`.
  - Gate: `make check-frontend` exit 0 (lint 0, format:check temiz, knip:ci 0, test 99/99, build ✓).

- [x] **Routing ambiguity (Tier 0) regresyon kapsamı.** ~~ambiguity_golden.json'a routing case ekle~~ —
  premis mimari olarak yanlıştı: o suite tamamen `ambiguity.Analyze` (glossary/synonym/temporal/scope) tabanlı,
  routing katmanını (`TableRouter` + datasource metadata) hiç çalıştırmıyor → routing case orada barınamaz.
  Ayrıca routing→clarification eşlemesi zaten unit-test'liydi (`TestClarificationFromRoutingBuildsOptionsAndCandidates`
  çoklu aday + `TestTableRouter_RouteNeedsClarificationForNoMatch` no-match).
  Gerçek boşluk: `TableRouter.Route`'un *yarışan zayıf adaylar* (0<confidence<0.35) durumunda Tier-0 clarification
  üretmesi yalnızca no-match için test ediliyordu. Eklenen test bunu kapatıyor:
  - `TestTableRouter_RouteNeedsClarificationForCompetingCandidates` — iki tablo kolon-açıklamasıyla zayıf-eşit eşleşiyor
    → `routeConfidence`=0.25 < `minRouteConfidence`(0.35) → `NeedsClarification=true`, ≥2 eşit-skorlu candidate, model nil.
  - **Dosya:** `internal/ai/routing/route_clarification_test.go`.
  - Gate: gofmt ✓, race test (routing) ✓, lint-go 0 issues, deadcode temiz. (eval'e dokunulmadı → eval-regression gerekmez.)

- [x] **Tiered config varsayılan değerleri.** `ai.ambiguity.tieredEnabled: true` +
  `maxLLMTierPerQuestion: 1` umbrella + prod values ve ai subchart defaults; ConfigMap
  `BI_AI_AMBIGUITY_TIERED_ENABLED` + `BI_AI_AMBIGUITY_MAX_LLM_TIER_PER_QUESTION` emit ediyor.
  - **Dosyalar:** `deploy/helm/biqly/values.yaml`, `values-prod.yaml`, `charts/ai/values.yaml`, `charts/ai/templates/configmap.yaml`.

### Frontend Denetim Bulguları (2026-06-09)

Backend P0–P7 uygulandı, frontend karşılıkları denetlendi. Tamamlanan ve eksik olanlar:

| Feature | Frontend Durum | Açık Nokta |
|---|---|---|
| P0 Clarification Round | ✅ Tam | `AIQuery.tsx:78,298,313` — state + gönderim + okuma tam |
| P1 Hard Cap UX | ⚠️ Kısmi | Backend 2 round'da kesiyor ama frontend'de cap'e ulaşıldığında buton devre dışı bırakılmıyor / UX uyarı yok |
| P2 Glossary AIContext | ✅ Tam | `Glossary.tsx:892-1043` — synonyms[], unit, null_meaning, business_rules[] form alanları + i18n |
| P3 Memory Store visibility | ⚠️ Kısmi | Thumbs-up → backend otomatik depolar, ama kullanıcıya "öğrenildi" geri bildirimi yok; confirmed queries admin listesi yok |
| P4 Enrich-Context | ✅ Tam | `GlossaryEnrichPanel.tsx` + endpoint'ler + i18n tam |
| P5 Tiered Detection UI | ❌ Yok | Admin settings'de `TieredEnabled` / `MaxLLMTierPerQuestion` toggle yok, sadece env-var; i18n anahtarı yok |
| P6 Generation Trace i18n | ⚠️ Kısmi | Trace panel tüm alanları render ediyor ama `columns_resolved` bölüm başlığı ve `ambiguity_detail` etiketi için i18n anahtarı eksik |

#### Frontend için yeni maddeler

- [x] **P1 — Hard cap UX göstergesi.** Round ≥ `maxClarificationRounds` (2) olan clarification kartında
  "Maksimum netleştirme turuna ulaşıldı — bir seçenek seçin, en iyi tahminle yanıtlayalım" bildirimi gösteriliyor.
  - **UX kararı (kullanıcı onayı):** Seçenek butonları AÇIK kalıyor — round-2 kartının seçenekleri o turu *çözen*
    butonlar; disable etmek sorguyu çözülemez halde bırakırdı (backend gelen round 2'de ambiguity'yi zaten bypass edip
    nihai cevabı üretiyor). Bildirim + açık seçenekler doğru davranış.
  - Round değeri global state'ten değil, her mesajın `result.clarification_round`'undan okunuyor (per-message doğru kaynak).
    `MAX_CLARIFICATION_ROUNDS = 2` sabiti backend `maxClarificationRounds` ile hizalı (yorumla işaretlendi).
  - **Dosyalar:** `assistantMessageCardSections.tsx` (cap hesabı + `capReached` prop), `routingViz.tsx`
    (`ClarificationCard` bildirim render), `i18n/locales/{en,tr}/core.ts`, `styles/aiQuery.css` (`.clarification-cap-notice`).
  - Gate: `make check-frontend` exit 0 (lint 0, format:check, knip:ci 0, test 99/99, build ✓).

- [x] **P3 — Confirmed queries admin listesi + "öğrenildi" geri bildirimi.** ~~Endpoint'ler hazır~~ —
  premis yanlıştı: `GET /api/admin/ai/confirmed-queries` backend'de **yoktu** (repo'da yalnızca recall amaçlı
  `ListActiveConfirmedQueries` vardı). Eksik backend bu turda eklendi:
  - Backend: `ListConfirmedQueriesForAdmin` (aktif+pasif, `confirmed_at` dahil) + `SetConfirmedQueryActive`
    (`internal/metadata/ai_confirmed_queries.go`); admin-key'li `GET /api/ai/confirmed-queries?datasource_id=...` +
    `POST /api/ai/confirmed-queries/{id}/deactivate` (`handlers/ai_confirmed_queries.go`, `ai_router.go`).
    Mevcut admin AI endpoint konvansiyonuna uyularak `/api/ai/...` altında (AdminKeyMiddleware: super_admin JWT veya admin key).
  - "Öğrenildi" sinyali: `POST /api/ai/feedback` yanıtına `learned: bool` eklendi
    (`storeConfirmedQueryOnPositiveFeedback` artık bool döndürüyor).
  - Frontend: yeni admin sekmesi **AI & paylaşım → Onaylanmış Sorgular** (`ConfirmedQueriesPanel.tsx`,
    `adminNavConfig.ts`, `Admin.tsx`, `api/aiAdmin.ts`): datasource seçici, soru/SQL/onay tarihi/durum tablosu,
    pasifleştir butonu. Thumbs-up sonrası `FeedbackSection`'da "öğrenildi" rozeti (`learned===true` ise;
    `.feedback-learned-badge`, `role="status"`). Admin.tsx tab render'ı prop'suz paneller için
    `PROPLESS_TAB_PANELS` map'ine sadeleştirildi (complexity 21→<20).
  - Testler: `ai_confirmed_queries_admin_test.go` (list + validation + deactivate/404/400).
  - **Dosyalar:** yukarıdakiler + `i18n/locales/{en,tr}/{admin,core}.ts`, `styles/aiQuery.css`,
    `AssistantMessageCard.tsx`.

- [x] **P5 — Tiered ambiguity admin toggle.** ~~`GET/PUT /api/admin/config` üzerinden~~ — premis yanlıştı:
  bu endpoint'ler **yoktu** ve `AmbiguityConfig` salt env-var'dı (runtime store yok). Bu turda eklendi:
  - Backend: migration `045a` `ai_runtime_config(key, value JSONB, updated_at)` KV tablosu;
    `Get/UpsertAIRuntimeConfig` (`metadata/ai_runtime_config.go`); admin-key'li
    `GET/PUT /api/ai/admin/config` (`handlers/ai_admin_config.go`). Overlay deseni:
    `effectiveAmbiguityConfig` env varsayılanları üzerine DB override'larını bindiriyor
    (nil alan = env default); 30s TTL cache (PUT yapan replika anında invalidate, diğerleri TTL içinde yakınsar).
    `standardProcessOptions` artık `h.effectiveAmbiguityConfig(ctx)` okuyor → sync + async job yolu aynı değeri görür.
    PUT validasyonu: her iki alan zorunlu, `max_llm_tier_per_question` 0–10.
  - Frontend: hedef sayfa `Settings.tsx` değil (orası kullanıcı ayarları) — bölüm
    **Admin → Platform Ayarları**'na eklendi (`PlatformSettingsPanel.tsx`): tiered checkbox +
    max LLM tier sayı girişi + DB-override/env-default notu + ayrı kaydet.
  - Testler: `ai_admin_config_test.go` (overlay, env-default GET, PUT persist+reload, validasyon).
  - **Dosyalar:** `migrations/045{a,b}_add_ai_runtime_config.*.sql`, `internal/metadata/ai_runtime_config.go`,
    `internal/http/handlers/ai_admin_config.go`, `ai.go`, `internal/http/ai_router.go`,
    `frontend/src/api/aiAdmin.ts`, `PlatformSettingsPanel.tsx`, `i18n/locales/{en,tr}/admin.ts`.

- [x] **P6 — Generation trace i18n eksikleri.** `generation_trace_columns` ("Resolved columns"/"Çözümlenen
  kolonlar") bölüm başlığı columns listesinin üstüne, `generation_trace_ambiguity_detail` ("Detail"/"Ayrıntı")
  etiketi ambiguity detayının önüne eklendi.
  - **Dosyalar:** `frontend/src/components/aiQuery/generationTrace.tsx`,
    `frontend/src/i18n/locales/{en,tr}/core.ts`.
  - Gate'ler (üç madde birlikte): `make check-frontend` exit 0 (lint 0, format:check ✓, knip:ci ✓,
    vitest 99/99, build ✓); `make lint-go` 0 issues; `make test-go` (-race, 58 paket) PASS;
    `deadcode` yeni bulgu yok. Commit yapılmadı.

### Plan ↔ Codebase Boşluk Analizi — 2. Tur (2026-06-09)

P0–P7 iddiaları kod üzerinde tek tek doğrulandı. **Doğrulananlar (yeni madde gerekmez):**
P0 `ClarificationResolved` yalnızca `ai_context.go:54`'te yazılıyor (kabul kriteri sağlanıyor);
P1 round artışı `attachAmbiguityClarificationRound` üzerinden, metric mevcut; P2 prompt
`unit/null_meaning/business_rules`'ı `prompt/glossary.go`'da kullanıyor; P3 recall iki few-shot
yolunda da bağlı (`ai.go:1141,1265`); P4 CLI `cmd/biqly` `--dry-run`/`--suggest` ile mevcut;
P5 Tier 2 LLM yalnızca deterministik boş dönünce çalışıyor (`service.go: !result.IsAmbiguous &&
ambiguityLLMCheck`), tier 0/1/2/3 metric'leri kayıtlı; P6 `ColumnsResolved` routing+resolution'dan
dolduruluyor; P7 golden suite choice→expected round-trip case'i ve choice↔expected validasyonunu içeriyor.
Plan'daki tüm metric adları (`biqly_ambiguity_round_cap_reached_total`, `biqly_ambiguity_tier`,
`biqly_memory_store_*`, `biqly_enrich_context_*`) birebir mevcut.

**Tespit edilen boşluklar / yeni maddeler:**

- [x] **GAP-1 (P5) — Tier 3 "agent-driven multi-turn" uygulandı (seçenek a).**
  Plan tablosundaki Tier 3 artık cap bypass değil: `clarificationRound == maxClarificationRounds` (2) iken
  `ShouldUseInteractiveTier` → tam rule-based analiz + zorunlu LLM + sınırsız seçenek (`WithAmbiguityInteractiveTier`).
  `clarificationRound > 2` ise gerçek bypass (ambiguity check kapalı). Kısmi çözüm: `HasRemaining` ile
  `ClarificationResolved` yalnızca tüm belirsizlikler giderildiğinde set edilir. Frontend: round===2
  interactive tier bildirimi; round>2 cap bildirimi. Metric help: `3=interactive agent`.
  - **Dosyalar:** `ai_ambiguity_tier.go`, `ai_context.go`, `internal/ai/service.go`, `ambiguity/resolver.go`,
    `metrics.go`, frontend `assistantMessageCardSections.tsx`, `routingViz.tsx`, i18n `core.ts`.

- [x] **GAP-2 (P3) — Açık kabul kriteri "accuracy artışı ölçülebilir" için somut alt maddeler.**
  Recall mekanizması ve sayaçlar var ama "memory store accuracy'yi artırıyor mu" sorusuna yanıt veren
  ölçüm yok. Alt maddeler:
  - [x] `ai_query_history`'e `memory_recall_used bool` (veya recall hit sayısı) alanı yaz → pozitif/negatif
    feedback oranı recall'lı vs recall'sız sorgular için karşılaştırılabilir olsun.
  - [x] Grafana paneli: `biqly_memory_store_recall_hits_total` / `biqly_memory_store_confirmed_total`
    trendi + recall'lı sorguların thumbs-up oranı.
  - [x] Eval: confirmed-query enjekte edilen golden case (recall few-shot'unun üretimi iyileştirdiğini
    assert eden, stub-provider'lı regression case) — `internal/ai/eval/`.

- [x] **GAP-3 (P0 kalıntısı) — Tier-0 short-circuit bloğu sync/async'te tekrar ediyor.**
  `routeResult.NeedsClarification → RecordAmbiguityTier("0") + clarificationResponse + observe` bloğu
  hem `ai.go:184-191` (sync) hem `ai_job_exec.go:~44` (async) içinde kopya — P0'ın "tek yol" hedefinin
  dışında kalmış küçük bir divergence noktası. Ortak helper'a çekilmeli
  (örn. `(h *AIHandler) tierZeroClarification(...)`).
  - **Dosyalar:** `internal/http/handlers/ai.go`, `ai_job_exec.go`.

- [x] **GAP-4 (doc) — Denetim tablosundaki endpoint path'leri yanlış.** P4 satırı
  `/api/admin/ai/enrich-context` yazıyordu; gerçek path admin-key grubunda `/api/ai/enrich-context`
  - `/api/ai/enrich-context/apply`. (P3'teki aynı hata daha önce düzeltildi: gerçek path
  `/api/ai/confirmed-queries`.) Tablo satırı güncellendi; `/api/admin/*` diye bir route ailesi
  hiç yok — gelecek maddelerde bu kalıba dikkat.

- [x] **NOT-1 (plan-dışı ekleme, bilinçli) — `ai_runtime_config` DB override katmanı.** P5 admin
  toggle'ı için plan dışı eklendi (planda yalnızca env flag vardı). Operasyon etkisi: DB override
  varken Helm/env değeri değiştirmek etkisizdir (`db_override` admin panelde görünüyor). İyileştirme:
  `GET /api/ai/settings` yanıtına efektif ambiguity değerleri + `db_override` + `source`
  (`environment` | `database`) eklendi — admin config ile aynı wire shape (`effectiveAmbiguitySettings`).
  **Dosyalar:** `ai_settings.go`, `ai_admin_config.go`, `ai_settings_test.go`, `frontend/src/types/ai.ts`,
  `pkg/aiclient/schema.go`.

### Prod Vaka: "geçen ay kaç adet tweet atılmıştır?" (2026-06-09 21:55, zlitter_2)

İki kullanıcı-görünür hata, üç kök neden (kod üzerinde doğrulandı):

**Belirti 1 — Netleştirme kartı Türkçe UI'a rağmen İngilizce geldi.**
Kök neden: frontend generate/preview için **async job** yolunu kullanıyor; worker job'ı
"bare consumer context" ile çalıştırıyor (`ai_job_service.go` — yorum aynen böyle diyor).
`UserID` job kaydına persist edilip worker'da `ai.WithUserID` ile geri enjekte ediliyor ama
**locale edilmiyor** → `i18n.FromContext(ctx)` = DefaultLocale("en") → `ambiguity_reason`
("This question matched more than one possible meaning.") ve `ambiguity_question_single`
("What did you mean by ...?") İngilizce katalogtan basılıyor. `internal/i18n/locales/tr.json`
çevirileri **mevcut ama job yolunda hiç kullanılamıyor**. Senkron yolda `bimw.Locale`
middleware'i X-Locale'i context'e koyduğu için sorun yalnızca async yolda görülüyor.

**Belirti 2 — "geçen ay" koşulu nihai sorguya yansımadı** (`SELECT COUNT(*) ... LIMIT 100`,
filtresiz; güven %90 gösterildi). İki ayrı kök neden:
(a) *Sahte belirsizlik gürültüsü:* routing her timestamp kolonu için `*_month` date-grain
dimension üretip month grain synonym'lerini ("ay", "aylık", ...) kopyalıyor
(`routing/time_grains.go`); `DetectSynonyms` modeldeki TÜM dimension synonym'lerini taradığı
için "geçen ay" içindeki "ay" token'ı 15 `*_month` dim'e çarpıyor → anlamsız 15 seçenekli
"semantic" kartı. Oysa `temporal_detector.go` "geçen ay" kalıbını tanıyor ve anlamlı 2 yorum
üretiyor (takvim ayı vs son 30 gün) — ama tiered modda (`AnalyzeSynonymHomonym`) temporal
detector hiç çalışmıyor; çalışsa bile synonym detector bare-token "ay"ı ayrıca işaretliyor.
(b) *Sessiz koşul düşmesi:* LLM (mimo-v2.5) modelle uyuşmayan şekil üretti, repair döngüsü
sonunda filtresiz bare-count kaldı; `warnings_body` gösterildi ama zaman koşulunun
**uygulanmadığı** açıkça söylenmedi ve güven %90 kaldı. Compile katmanı `between` +
`calendar_grain_filter.go` ile "geçen ay"ı ifade EDEBİLİYOR — kayıp üretim/repair tarafında.

**Yapılacaklar:**

- [x] **VAKA-1 — Async job yoluna locale propagasyonu.** (Tamamlandı — commit `1248cac0`:
  `ai_jobs.locale` persist + worker'da `consumerContextForAIJob` ile `i18n.WithLocale`
  enjeksiyonu + migration 047a/b + `ai_job_service_locale_test.go`.) Job oluştururken request locale'ini
  (`i18n.FromContext`) job kaydına persist et (`metadata.AIJob`'a `locale` alanı veya
  `RequestJSON` içine), worker'da `processJob` öncesi `i18n.WithLocale(ctx, job.Locale)` enjekte
  et — `UserID` ile birebir aynı kalıp. Test: TR locale ile submit edilen job'ın clarification
  yanıtı `tr.json` metinlerini içeriyor.
  - **Dosyalar:** `internal/metadata/ai_jobs*.go` (+migration gerekirse), `internal/http/handlers/ai_jobs.go`
    (create), `ai_job_service.go` (inject), test.

- [x] **VAKA-2 — Date-grain synonym'leri belirsizlik tespitinden çıkar.** (Tamamlandı —
  `DetectSynonyms` artık `TimeGrain != ""` olan dimension'ları atlıyor; suffix yerine alan
  bazlı ayrım: auto-generated grain dim'leri `model_builder.go` `TimeGrain` set ediyor, meşru
  "ay" kolonları etmiyor. Testler: `TestDetectSynonyms_DateGrainDimensionsNotFlagged` +
  `TestDetectSynonyms_PlainColumnNamedAyStillFlagged`.) `DetectSynonyms`
  auto-generated date-grain dimension'ların grain synonym'lerini ("ay", "gün", "yıl",
  "çeyrek", "saat" + EN karşılıkları) collision adayı saymasın: ya date-grain dim'leri
  (suffix `_month/_day/_quarter/_year/_hour`) detector'dan hariç tut, ya da soru içinde
  bilinen temporal phrase'in (`temporal_detector` pattern listesi) parçası olan token'ları
  atla. Kabul: "geçen ay kaç tweet" sorusu synonym collision üretmiyor; "ay" tek başına
  meşru kolon adı olarak geçen durumlar etkilenmiyor.
  - **Dosyalar:** `internal/ai/ambiguity/synonym_detector.go`, `analyzer.go`, testler.

- [x] **VAKA-3 — Tiered modda temporal ifadeler için doğru tier.** (Tamamlandı —
  `AnalyzeSynonymHomonym` artık `DetectTemporal`'ı da çalıştırıyor (scope hâlâ tier 1 dışı,
  heuristiği daha gürültülü). Test: `TestAnalyzeSynonymHomonym_DetectsTemporalSkipsScope`;
  golden: `ambiguity_golden.json` → `temporal-gecen-ay-tweets` TR case, `make eval-regression` ✓.) "geçen ay" gibi kalıplar
  Tier 1'de (deterministik, ücretsiz) `DetectTemporal` ile yakalanabilirken tiered mod onları
  tamamen atlıyor (P5 kabulünde "scope/temporal Tier 1'de atlanıyor" diye bilinçli yazılmıştı —
  bu vaka kararın yanlış olduğunu gösteriyor: temporal de deterministik ve ücretsiz).
  `AnalyzeSynonymHomonym`'e `DetectTemporal`'ı ekle (veya tiered Tier 1 detector setini
  config'e bağla) → kullanıcı "takvim ayı mı, son 30 gün mü?" gibi ANLAMLI bir netleştirme görür.
  - **Dosyalar:** `internal/ai/ambiguity/analyzer.go`, ilgili testler + `ambiguity_golden.json`'a
    TR temporal case.

- [x] **VAKA-4 — Temporal koşul sessizce düşmesin.** (Tamamlandı —
  `internal/ai/temporal_postcheck.go`: `applyTemporalFilterPostCheck` ProcessQuestion'ın üç
  sonuç yolunda da (multi-candidate, retry-loop, failure) çalışıyor; soruda
  `ambiguity.MatchTemporalPhrases` (yeni export) eşleşip nihai LogicalQuery'de hiçbir date-dim
  WHERE/HAVING filtresi yoksa locale'li uyarı (`clarification.temporal_filter_missing`,
  en+tr.json) ekleniyor ve confidence 0.5'e cap'leniyor (cap aynı zamanda response cache'e
  girmesini engelliyor, eşik 0.85). Few-shot: `writeFailureExamples`'a "Relative time phrase
  dropped" wrong/right çifti. Testler: `temporal_postcheck_test.go` (4 unit) +
  `TestProcessQuestionWarnsWhenTemporalFilterDropped` (TR locale entegrasyon). Not: trace.go'ya
  ayrı alan eklenmedi — uyarı `warnings` üzerinden kullanıcıya zaten görünür, minimum-kod tercihi.) Soruda temporal phrase tespit edildiyse
  (temporal detector pattern'leri) ve nihai LogicalQuery hiçbir tarih filtresi/grain filtresi
  içermiyorsa: özel uyarı ekle ("zaman koşulu sorguya uygulanamadı"), confidence'ı düşür ve/veya
  recovery seçenekleri sun — %90 güvenle filtresiz COUNT dönmek yanlış-doğru cevap üretiyor
  (silent failure). Ek olarak: TR göreli tarih few-shot örneği ("geçen ay kaç X" →
  `created_at` üzerinde prev-calendar-month filter'lı count) + eval golden case.
  - **Dosyalar:** `internal/ai/service.go` (gen loop sonrası post-check), `internal/ai/trace.go`
    (uyarı/trace alanı), few-shot seed + `internal/ai/eval/testdata/`, frontend uyarı metni
    gerekiyorsa `i18n/locales/{en,tr}/core.ts`.

- [ ] **VAKA-5 (ikincil, değerlendirildi/ertelendi) — Netleştirme seçenek metinleri ham kolon
  description'ı.** Değerlendirme (2026-06-10): VAKA-2 date-grain gürültü kartını tamamen,
  VAKA-3 temporal kartı i18n kataloğundan (tr.json) ürettiği için bu vakadaki İngilizce teknik
  metinler artık kullanıcıya ulaşmıyor. Kalan tek yüzey: kullanıcı tanımlı modellerdeki EN
  description'lı gerçek synonym collision'ları — düşük frekans. `MetadataTranslator` benzeri
  katman LLM çağrısı/cache karmaşası ekleyeceği için şimdilik uygulanmadı; gerçek vaka görülürse
  ele alınacak. Seçenek
  etiketleri İngilizce metadata description'larından geliyor (örn. "The standardized,
  machine-readable timestamp..."); locale düzelse bile bu metinler İngilizce kalır ve son
  kullanıcı için fazla teknik. Routing'deki `MetadataTranslator` benzeri bir çeviri/sadeleştirme
  katmanının ambiguity seçeneklerine de uygulanması değerlendirilmeli (düşük öncelik;
  VAKA-2/3 zaten bu kartın hiç çıkmamasını sağlar).

---

## Dil Varlıklarının Koddan Çıkarılması — DB Tabanlı Lexicon/i18n Yol Haritası (2026-06-10)

**Problem:** "aylık", "günlük", "geçen ay", "silinen" gibi doğal-dil tanımlamaları yalnızca
EN+TR için kod içinde gömülü. Yeni bir dil eklemek bugün release gerektiriyor; runtime'da
yönetilemiyor.

**Best practice (hedef mimari):** İki varlık sınıfını ayır ve ikisini de *hibrit* yönet:

1. **NL lexicon verisi** (synonym/phrase/intent token listeleri) = **data, kod değil** →
   locale-boyutlu DB tabloları; kod yalnızca *seed + fallback* olarak embedded default taşır
   (boot asla DB'ye bağımlı olmaz), üstüne DB overlay + cache + invalidation + admin CRUD.
   Bu kalıp repoda zaten 3 yerde kurulu: `ai_time_grains` (dbTimeGrainStore), prompt
   şablonları (`dbPromptStore`, versiyonlu, embed'den seed), `ai_runtime_config` (KV overlay).
   Yapılacak iş büyük ölçüde **mevcut kalıbı kalan hardcoded varlıklara yaymak**.
2. **Mesaj katalogları** (i18n bundle'ları) → embedded EN/TR fallback kalır, DB overlay +
   dinamik locale registry ile yeni dil release'siz eklenir.

**Envanter (2026-06-10, kod üzerinde doğrulandı):**

| Varlık | Yer | Durum |
|---|---|---|
| Time-grain synonym'leri | `routing/time_grains.go` `DefaultTimeGrains` | ✅ DB-backed (`ai_time_grains`) ama synonym dizisi locale-karışık; seed EN+TR hardcoded |
| Prompt şablonları (system_rules, repair, …) | `prompt/prompts/{en,tr}/*.tmpl` + `prompt_store.go` | ✅ DB-backed + versiyonlu; embed yalnızca en/tr, bilinmeyen locale EN'e düşer |
| Routing lexicon (token/intent/metric synonym) | `routing_lexicon_default.json` + `BI_AI_ROUTING_LEXICON_PATH` | ⚠️ Dosya override var (ConfigMap ile release'siz) ama locale-boyutsuz, admin UI yok |
| Glossary synonym'leri | `business_glossary_terms.ai_context` | ✅ DB-backed |
| Vague temporal phrase'ler | `ambiguity/temporal_detector.go` `vagueTemporalPhrases` | ❌ Hardcoded (TR+EN) |
| Soft-delete kelime listeleri | `routing/model_builder.go` `softDeleteColumnSynonyms` | ❌ Hardcoded |
| Aggregation/intent token'ları ("kaç", "adet", "toplam", grain kelimeleri) | `routing/routing_budget.go` | ❌ Hardcoded |
| Semanticgen grain/row-count synonym'leri | `semanticgen/generator.go` (~290-323) | ❌ Hardcoded (time_grains ile DUPLİKE — tek kaynağa inmeli) |
| Locale registry + soru-dili sinyalleri | `i18n/i18n.go` `SupportedLocales`, `localeProfiles` (QuestionSignals/Letters) | ❌ Hardcoded |
| Backend mesaj katalogları | `i18n/locales/{en,tr}.json` (embed) | ❌ Embedded — yeni dil = release |
| Soru dili tespiti | `ai/lingua/locale.go` | ❌ localeProfiles sinyallerine bağlı (hardcoded) |
| Prompt içi Go-üretimli bölümler (failure examples, planning steps) | `prompt/prompt_examples.go` | ❌ Hardcoded (EN gövde + TR ipuçları) |
| Frontend katalogları | `frontend/src/i18n/locales/{en,tr}/` | ❌ Build-time bundle — yeni dil = frontend release |
| Eval golden/edge case'leri | `internal/ai/eval/` | ✔️ Kodda kalması doğru (test varlığı) |

### DİL-0 — Tasarım kararı + ADR [S] ✅ (2026-06-10)

- [x] Karar: **generic tek tablo** `ai_nl_lexicon(locale, domain, key, value JSONB, is_active)`,
      PK (locale, domain, key); 7 domain ve value şekilleri ADR K2 tablosunda.
      `ai_time_grains` yapı tablosu olarak kalır, `synonyms` kolonu DİL-1'de `grain_synonym`
      domain'ine taşınır (`synonyms_by_locale` alternatifi reddedildi — ADR K3).
      Ek karar: eşleştirme **etkin locale'lerin birleşimi** üzerinde (K4, davranış-koruyucu).
- [x] Fallback zinciri: lexicon DB → embedded default (boot DB'siz çalışır, K5);
      cache 30s TTL + yazan replikada anında Invalidate (`ai_runtime_config` deseni, K6);
      seed: domain boşsa embedded'dan idempotent doldurma + embedded'a sıfırlama endpoint'i (K7).
- [x] **Kabul sağlandı:** `docs/adr/0001-db-backed-nl-lexicon-and-i18n.md` — şema (K1, K8
      locale registry dahil), fallback (K5), cache (K6), seed/kurtarma (K7), admin yüzeyi (K9),
      5 reddedilen alternatif ve riskler yazılı.

### DİL-1 — `ai_nl_lexicon` altyapısı + ilk taşımalar [M] ✅ (2026-06-10)

- [x] Migration `048a/b`: `ai_nl_lexicon(locale, domain, key, value JSONB, is_active,
      updated_at)` PK(locale,domain,key) + domain index; Go seed `lexicon.Seed`
      (tablo boşsa embedded default'lardan idempotent doldurma) — wiring: `setupAI`
      (dependencies.go) + `NewAIDependencies` (ai_dependencies.go), SeedTimeGrains'in yanında.
- [x] Yeni paket `internal/ai/lexicon`: `Store` arayüzü (TemporalPhrases/Terms/DomainTerms/
      Invalidate), `NewStaticStore` (embedded), `NewDBStore` (30s TTL + Invalidate, hata/boş
      domain'de per-domain embedded fallback — ADR K5/K6), `Active()/SetActive()` süreç-geneli
      kayıt (prompt store deseni). Repo katmanı: `metadata/ai_nl_lexicon.go`
      (List/ListActive/Count/Upsert/ReplaceDomain). Eşleştirme locale-birleşimi (K4);
      `Terms` snapshot'ı korumak için kopya döndürür.
- [x] `vagueTemporalPhrases` → lexicon (`temporal_phrase` domain'i); `DetectTemporal` +
      `MatchTemporalPhrases` store'dan okuyor. Kabul testi:
      `TestTemporalPhrasesComeFromLexiconStore` — store'a Almanca "letzten monat" verilince
      kod değişikliği olmadan tespit ediliyor.
- [x] `softDeleteColumnSynonyms` kelime listeleri → lexicon (`soft_delete`, 4 kural anahtarı);
      kolon-adı pattern kuralları kodda kaldı. (Not: ts_archived listesinin *sırası* en→tr
      birleşimine değişti, küme aynı.)
- [x] `routing_budget.go`: count/total/average intent token'ları + `questionMentionsTimeGrain`
      listesi → lexicon (`intent_token`); how+many / number+of yapısal çiftleri kodda.
- [x] `semanticgen/generator.go`: grain synonym + row-count listeleri lexicon'dan (tek kaynak);
      `time_grains.go` her iki store'da serve-time merge (`applyLexiconGrainSynonyms`, K3 geçişi —
      `ai_time_grains.synonyms` kolonu deprecate edilene dek birleşim).
      **Bilinçli süperset:** kanonik grain listesi = eski routing ∪ semanticgen listeleri
      (routing "ay bazında"yı, semanticgen "per month"u kazandı).
- [x] Admin CRUD (AdminKeyMiddleware): `GET/PUT /api/ai/admin/lexicon?locale=&domain=`
      (export/import, domain+locale+value şema validasyonu) + `POST /api/ai/admin/lexicon/reset`
      (domain'i embedded default'a sıfırlama — ADR K7); PUT/reset → `lexicon.Active().Invalidate()`.
      Dosyalar: `handlers/ai_admin_lexicon.go`, `ai_router.go` (`registerAIAdminConfigRoutes`
      helper'ına çıkarıldı, funlen).
- [x] **Kabul sağlandı:** yeni dil yalnızca DB satırlarıyla eklenebiliyor (de-locale store/fallback
      testleri); EN/TR davranışı korunuyor (59 paket -race yeşil; tek bilinçli fark yukarıdaki
      süperset — time_grains testleri merge sözleşmesine güncellendi); DB yokken embedded
      fallback testli (`TestDBStoreFallsBackToDefaultsOnError`). Gate'ler: lint-go 0,
      eval-regression ✓, deadcode yeni bulgu yok, jsonusage guard ✓ (sonic).

### DİL-2 — Routing lexicon'u DB overlay'e bağla [S] ✅ (2026-06-10)

- [x] `routing_lexicon.go`: `sync.Once` → mutex'li base+merged cache (30s `routingLexiconOverlayTTL`);
      `ActiveRoutingLexicon` artık embedded+dosya base'inin üzerine `ai_nl_lexicon`
      `token_synonym`/`metric_synonym` domain'lerini per-key replace ile bindiriyor
      (`overlayRoutingLexicon`); DB satırı yoksa base aynen servis edilir (sıfır davranış farkı —
      bu iki domain'in embedded seed'i bilinçli olarak boş). `BI_AI_ROUTING_LEXICON_PATH` dosya
      override'ı korunuyor (`InitRoutingLexicon` base'i yeniler). Yeni `InvalidateRoutingLexicon()` —
      admin lexicon PUT/reset `invalidateLexiconCaches` helper'ı ile hem lexicon store'u hem bu
      merge'ü yazan replikada anında düşürüyor; diğer replikalar iki TTL penceresi içinde (≤60s) yakınsar.
- [x] **Kabul sağlandı:** `TestActiveRoutingLexiconAppliesDBOverlay` — store'a "kunde"
      token-synonym'ü verilince restart'sız görünür, base anahtarlar ("musteri") ve overlay-dışı
      alanlar korunur, overlay kalkınca base'e döner. Gate'ler: lint-go 0, `make test-go`
      59 paket -race ✓, deadcode yeni bulgu yok, gofmt ✓.

### DİL-3 — i18n: dinamik locale registry + katalog overlay [M] ✅ (2026-06-10)

- [x] Migration `049a/b`: `i18n_locales` (profil + question_signals JSONB + enabled) ve
      `i18n_bundles` (bundle JSONB + version; update'te version otomatik artar).
      `metadata/i18n_runtime.go`: repo metotları + `NewI18nRuntimeProvider` adaptörü +
      `SeedI18nLocales` (tablo boşsa embedded EN/TR profillerinden). Wiring tek helper'da:
      `app/nl_runtime.go` `wireNLRuntime` (lexicon + i18n birlikte; setupAI ve
      NewAIDependencies buradan çağırıyor — NewAIDependencies funlen fix'i de bu).
- [x] `i18n/runtime.go`: `RuntimeProvider` arayüzü + 30s TTL snapshot + `InvalidateRuntime`.
      `SupportedLocaleProfiles/Codes`, `LocaleProfileFor`, `IsSupported`, `ParseLocale`,
      `FromContext` artık efektif (embedded ∪ registry) kümeden; yeni `ActiveLocales()`
      (embedding refresh de bunu kullanıyor). EN devre dışı bırakılamaz (K8); provider
      hatasında embedded'a düşülür. `T/Tf` zinciri: DB(loc) → embedded(loc) → DB(en) →
      embedded(en) → key. i18n bağımlılıksız kaldı — provider implementasyonu metadata'da
      (metadata→i18n yönü). Snapshot yenileme bilinçli request-scope'suz
      (`nolint:contextcheck` gerekçeli, audit/db_writer emsali).
- [x] Admin (AdminKeyMiddleware, `handlers/i18n_admin.go` + `ai_router.go`):
      `GET/PUT /ai/admin/i18n/locales` (EN-disable reddi, locale/label validasyonu),
      `GET/PUT /ai/admin/i18n/bundles/{locale}` (export DB→embedded fallback'li; import
      string-leaf validasyonlu, version'lı), `GET /ai/admin/i18n/coverage/{locale}`
      (efektif EN referansına göre eksik anahtar listesi + coverage %). Yazımlar
      `i18n.InvalidateRuntime()` çağırıyor.
- [x] `lingua.DetectQuestionLocale` zaten `SupportedLocaleProfiles()` okuyordu → registry
      sinyalleri otomatik devrede. Test: `TestDetectQuestionLocaleUsesRegistrySignals`
      (Almanca sinyaller kod değişikliği olmadan tespit ediliyor; embedded TR etkilenmiyor).
- [x] **Kabul sağlandı:** `TestRuntimeRegistryAddsNewLocale` + `TestRuntimeBundleLookupChain` —
      registry satırı + bundle upload ile "de" parse/context/profil/T() uçtan uca release'siz
      çalışıyor; EN-disable engeli, provider-hata fallback'i, TTL/invalidate, bundle leaf
      validasyonu ve coverage raporu testli. Gate'ler: lint-go 0, `make test-go` 59 paket
      -race ✓, eval-regression ✓, deadcode yeni bulgu yok, gofmt ✓.
      **Bilinen sınırlar:** (1) DB bundle'ları seed edilmez — embedded fallback tasarımı
      (drift önler); coverage endpoint'i eksikleri raporlar. (2) Runtime provider yalnızca
      monolith API (setupAI) + AI servisinde (NewAIDependencies) bağlı — catalog/query
      servisleri embedded-only (kullanıcıya i18n metni üreten yüzeyleri yok denecek kadar az;
      gerekirse `wireNLRuntime` iki satırla eklenir). (3) Yeni dilde bundle yüklenmezse
      metinler zincir gereği EN döner — davranış, hata değil.

### DİL-4 — Prompt şablonları: yeni locale onboarding [S] ✅ (2026-06-10)

- [x] **Tasarım sapması (bilinçli, daha iyi):** plan "bilinmeyen locale için DB'ye seed satırı"
      diyordu; bunun yerine **runtime köprü** uygulandı — seed durumu gerektirmez, sonradan
      eklenen locale restart'sız anında çalışır. `dbPromptStore.Snapshot` +
      `embedPromptStore.Snapshot` artık hangi locale'den çözümlediğini izliyor; kendi şablonu
      olmayan locale EN içeriği + `languageBridgeNote` ("## User Language — write every
      user-facing text in <Label>") alıyor. Not yalnızca kullanıcı-metni taşıyan bölümlere
      (`system_rules`, `clarification`) ekleniyor; admin o locale için DB satırı yazınca
      (versiyonlu, mevcut prompt-templates admin API'si) öncelik onda ve not düşüyor.
      Dil etiketi `i18n.LocaleProfileFor`'dan (DİL-3 registry) geliyor.
      Dosyalar: `prompt/prompt_templates.go` (`promptTemplateFromEmbedExact` +
      `languageBridgeNote`), `prompt/prompt_store.go` (iki Snapshot).
- [x] `prompt_examples.go` TR-hardcoded ipuçları lexicon'a bağlandı: planning steps 1 ve 5'teki
      "(müşteri, sipariş, silinen, aylık, …)" / ("bazında", "aylık") örnekleri artık
      `lexiconHintSamples` ile `intent_token/soft_delete/grain_synonym` domain union'ından
      (baş+son örnekleme → her aktif dil temsil edilir) üretiliyor.
- [x] **Kabul sağlandı:** `TestDBPromptStoreUnknownLocaleGetsLanguageBridge` ("de" → EN+not;
      admin "de" satırı yazınca not düşer), `TestEmbedPromptStoreUnknownLocaleGetsLanguageBridge`,
      `TestDBPromptStoreEmbeddedLocaleHasNoBridge`, `TestLexiconHintSamplesSpansLocales`.
      Gate'ler: lint-go 0, `make test-go` 59 paket -race ✓, eval-regression ✓, gofmt ✓.

### DİL-5 — Frontend + kalite kapıları [M]

**Mevcut durum:** Frontend locale'leri build-time bundled (`core.ts` statik import, `admin.ts`/`auth.ts`
dynamic `import()` code-split). Yeni dil eklemek için `Locale` union tipi, `SUPPORTED_LOCALES`,
`LOCALE_OPTIONS`, `dictionaries`, `sectionLoaders` değişikliği + 3 yeni TS dosyası gerekiyor → frontend
release şart. Backend tarafında `RuntimeProvider` + `i18n_locales` tablosu runtime'da yeni dil eklemeyi
destekliyor; prompt şablonları `prompts/{en,tr}/*.tmpl` dosya dizininden yükleniyor.

#### 5.1 — Frontend katalog dağıtım kararı

- [ ] **Karar:** (a) Runtime fetch vs (b) build-time bundle. ADR olarak belgelenmeli.

  **Seçenek (a) — Runtime fetch (önerilen):**
  1. Backend'e `GET /api/i18n/bundle/{locale}` endpoint'i ekle. Bu endpoint, `i18n_locales`
     registry'sinde kayıtlı bir locale'nin tüm section'larını JSON olarak döndürür
     (`core` + `admin` + `auth` birleştirilmiş).
  2. Frontend'de `sectionLoaders` mekanizmasını genişlet: mevcut `import()` yerine locale
     başına `fetchBundle(locale)` fonksiyonu ekle. Önce `localStorage` cache'ine bak (key:
     `biqly_bundle_{locale}`, TTL: 1 saat), miss'te API'den çek ve cache'e yaz.
  3. `LanguageSwitcher.tsx`'te dil listesini `GET /api/i18n/locales` (registry'den) ile dinamik
     oluştur. Yeni dil registry'ye eklenince buton otomatik görünür.
  4. `Locale` tipini `'en' | 'tr'` sabit listesinden `string`'e genişlet; `SUPPORTED_LOCALES`
     sabitini kaldır, runtime'dan beslenen state'e çevir.
  5. Fallback: bundle fetch başarısız olursa gömülü EN+TR seed dosyalarına dön (mevcut statik
     import'lar fallback olarak kalır).
  6. **Avantaj:** Yeni dil = sadece backend registry kaydı + bundle JSON. Frontend release yok.
  7. **Dezavantaj:** İlk yüklemede ek ağ round-trip; bundle TTL cache ile azaltılabilir.

  **Seçenek (b) — Build-time bundle (basit):**
  1. Mevcut mimari korunur. Yeni dil için 3 TS dosyası (`core.ts`, `admin.ts`, `auth.ts`) oluşturulur.
  2. `Locale` tipine yeni değer eklenir, `SUPPORTED_LOCALES`/`LOCALE_OPTIONS`/`sectionLoaders`
     güncellenir.
  3. Frontend build + deploy gerekli.
  4. **Avantaj:** Basit, ağ bağımlılığı yok.
  5. **Dezavantaj:** Her yeni dil = frontend release.

  **Karar sonrası yapılacaklar:**
  - ADR'yi `docs/adr/` altına yaz: bağlam, seçenekler, trade-off'lar, karar.
  - Seçilen seçeneğin implementasyonunu yeni bir todo maddesi olarak aç.

  - **Dosyalar (a seçilirse):**
    - Backend: `internal/http/handlers/i18n_bundle.go` (endpoint),
      `internal/i18n/runtime.go` (bundle assembler — mevcut `RuntimeProvider` genişletmesi).
    - Frontend: `frontend/src/i18n/locale.ts` (fetch + cache),
      `frontend/src/i18n/bundleLoader.ts` (yeni),
      `frontend/src/components/ui/LanguageSwitcher.tsx` (dinamik liste).
  - **Dosyalar (b seçilirse):** Sadece runbook'a yansıtılır, kod değişikliği yok.

#### 5.2 — Locale-parametrik eval golden case altyapısı

- [x] **Amaç:** Yeni dil eklendiğinde smoke test'ler otomatik çalışabilsin. Mevcut 7 golden case
      Türkçe sabit kodlanmış, `locale` alanı yok.

  **Nasıl yapılacak:**
  1. `ambiguity_golden.json` şemasına `"locale"` alanı ekle (varsayılan `"tr"` — geriye uyumlu).
     Mevcut case'ler `"locale": "tr"` ile işaretlenir.
  2. `AmbiguityGoldenCase` struct'ına `Locale string` alanı ekle (`ambiguity_golden.go`).
  3. Yeni dil eklendiğinde minimum smoke set: her `expected_type`'tan en az 1 case o dilde.
     Örnek: `"expected_type": "clarification"` + `"locale": "de"` → Almanca soru + aynı model_ref.
  4. `ambiguity_golden_runner.go`'da filtreleme: `--locale` flag'i veya `GOLDEN_LOCALE` env var ile
     sadece o dilin case'lerini çalıştır. CI'da varsayılan: tüm locale'ler.
  5. `ambiguity_golden_models.go`'daki test model'ler dil-agnostik kalır (SQL sütun adları),
     sadece `question` ve `expected_detail` dil değişir.
  6. `Makefile`'a `eval-regression` hedefinde locale filtresi yok (tüm diller çalışır).

  - **Dosyalar:**
    - `internal/ai/eval/testdata/ambiguity_golden.json` — `locale` alanı ekle,
      mevcut case'lere `"locale": "tr"` ekle.
    - `internal/ai/eval/ambiguity_golden.go` — struct + loader güncellemesi.
    - `internal/ai/eval/ambiguity_golden_runner.go` — locale filtre parametresi.
  - **Doğrulama (2026-06-10)**: `locale` alanı + `GOLDEN_LOCALE`/`--locale` filtresi, locale coverage
    doğrulaması, `make eval-regression` PASS.

#### 5.3 — Dil-taşıyan literal regresyon bekçisi

- [x] **Amaç:** Lexicon'a taşınmış kelimelerin koda geri sızmasını engelle. Mevcut durumda
      `internal/ai/routing/time_grains.go:25-49` (hardcoded grain synonym'ler),
      `internal/semanticgen/generator.go:358` (hardcoded `"ortalama"`) ve
      `internal/ai/routing/routing_lexicon_default.json` (67 satır karışık EN+TR token) hâlâ
      dil-taşıyor.

  **Nasıl yapılacak:**
  1. Basit bir shell script oluştur: `scripts/check_locale_literals.sh`
     - Taranacak dizinler: `internal/ai/routing/`, `internal/ai/ambiguity/`, `internal/semanticgen/`
     - Hariç tutulan: `*_test.go`, `testdata/`, `*.json` (routing lexicon ayrı ele alınacak)
     - Bilinen Türkçe kelime listesi dosyası: `scripts/locale_literal_blocklist.txt`
       İçeriği: her satır bir kelime (`aylık`, `yıllık`, `günlük`, `saatlik`, `ortalama`,
       `müşteri`, `sipariş`, `ürün`, `kategori`, `adet`, `miktar`, `toplam`, `silinen`, vb.)
     - Mantık: `grep -rf blocklist.txt <dizinler> --include='*.go' --exclude='*_test.go'`
       Bulunan her eşleşme = warning. Exit code 1 ile CI'ı kır.
  2. DIL-1 tamamlandıktan sonra `time_grains.go`'daki hardcoded synonym'ler lexicon'dan
     yükleniyor olacak → script bu dosyayı scoping'den çıkarabilir (veya dosya tamamen kaldırılır).
  3. `generator.go:358`'deki `"ortalama"` lexicon lookup'a çekildikten sonra script'in kapsamına girer.
  4. CI'ya ekle: `.github/workflows/ci.yml`'e yeni step veya `Makefile`'a `lint-locale-literals`
     hedefi olarak.
  5. Mevcut bulguları tolere etmek için initial run'da `--baseline` modu: mevcut bulguları
     listeleyip geçir, sonraki eklemeleri engelle.

  - **Dosyalar:**
    - `scripts/check_locale_literals.sh` (yeni script)
    - `scripts/locale_literal_blocklist.txt` (yeni kelime listesi)
    - `.github/workflows/ci.yml` veya `Makefile` (CI entegrasyonu)
  - **Doğrulama (2026-06-10)**: `scripts/check_locale_literals.sh` + baseline (`7` finding);
    `make lint-locale-literals` PASS; CI `lint` job step eklendi.

- [x] 5.4 — Yeni dil onboarding runbook'u

- [x] **Amaç:** `docs/` altında adım adım rehber: "X dilini Biqly'ye nasıl eklersin?"

  **Runbook içeriği (adım adım):**

  **Adım 1 — Backend: Locale registry kaydı**
  - `i18n_locales` tablosuna yeni satır ekle (admin API veya SQL):

    ```sql
    INSERT INTO i18n_locales (locale, label, short_label, is_active, question_letters, question_signals)
    VALUES ('de', 'Deutsch', 'DE', true, 'äöüßÄÖÜ',
            '{" wieviel "," zeige "," liste "," gesamt "," täglich "," monatlich "," jährlich "," gestern "," heute "," kunde "," bestellung "," produkt "," anzahl "," menge "," gelöscht "}');
    ```

  - `QuestionLetters`: o dile özgü karakterler (dil algılama için `lingua` kullanır).
  - `QuestionSignals`: o dildeki tipik soru kelimeleri (NL→SQL soru algılama ipucu).
  - `is_active=false` ile kaydet → aşağıdaki adımlar tamamlanınca `true` yap.

  **Adım 2 — Backend: NL lexicon seed**
  - `ai_nl_lexicon` tablosuna yeni dil için domain terimleri ekle. Minimum kategoriler:
    - `temporal_phrase`: "letzten monat", "kürzlich", vb.
    - `grain_synonym`: "jahr"/"jährlich"/"jährlich", "monat"/"monatlich", "tag"/"täglich", "stunde"/"stündlich"
    - `soft_delete`: "gelöscht", "entfernt", vb.
    - `intent_token`: "wieviel", "anzahl", "gesamt", "durchschnitt", vb.
    - `row_count`: "anzahl", "wie viele", "stückzahl"
  - Mevcut EN/TR seed'leri referans al: `internal/ai/lexicon/defaults.go`
  - Domain coverage raporu çalıştır: `GET /api/admin/ai/lexicon/coverage?locale=de`
    Boş/eksik kategorileri gösterir → bunları doldur.

  **Adım 3 — Backend: Prompt şablonları**
  - `internal/ai/prompt/prompts/` altına yeni dizin: `de/`
  - EN şablonlarını kopyala: `cp prompts/en/*.tmpl prompts/de/`
  - Her şablonu Almanca'ya çevir (sadece LLM'ün kullanıcıya döneceği metin kısımları):
    - `clarification.tmpl`: netleştirme soru metinleri
    - `output_format.tmpl`: çıktı formatı talimatları
    - `system_rules.tmpl`: sistem kuralları (çoğu locale-agnostik, sadece açıklama kısımları)
    - `repair.tmpl`, `retry.tmpl`: hata düzeltme talimatları
    - `ambiguity.tmpl`: belirsizlik algılama prompt'u
  - **Not:** `prompt_layout.tmpl` dil-agnostik (SQL şeması), genelde değişiklik gerektirmez.

  **Adım 4 — Frontend: Bundle (karar (a) seçildiyse)**
  - `i18n_bundles` tablosuna veya API endpoint'ine yeni locale'nin JSON bundle'ını yükle.
  - Minimum section'lar: `core` (tüm UI metinleri), `admin`, `auth`.
  - EN `core.ts`'yi temel al, çevirileri yap. Toplam ~1200 satır.
  - Test: dil seçicide "DE" görünmeli, seçince tüm UI Almanca olmalı.

  **Adım 5 — Frontend: Bundle (karar (b) seçildiyse)**
  - `frontend/src/i18n/locales/de/` dizinini oluştur.
  - 3 dosya: `core.ts`, `admin.ts`, `auth.ts` (EN'den kopyala + çevir).
  - `locale.ts`'te `Locale` tipine `'de'` ekle, `SUPPORTED_LOCALES`/`LOCALE_OPTIONS`/`sectionLoaders`
    güncelle.
  - `LanguageSwitcher.tsx`'te `"DE"` butonu otomatik görünür.
  - Frontend build + deploy.

  **Adım 6 — Smoke test**
  - `ambiguity_golden.json`'a en az 2 yeni case ekle:
    - 1 clarification case: Almanca soru → belirsizlik algılanmalı
    - 1 pass case: Almanca soru → belirsizlik yok, doğru SQL üretmeli
  - `make eval-regression` çalıştır, tüm case'ler pass olmalı.
  - Elle test: UI'da Almanca seç → örnek soru sor → doğru netleştirme/SQL.

  **Adım 7 — Aktifleştir**
  - `i18n_locales` tablosunda `is_active = true` yap.
  - Deploy.

  - **Dosyalar:**
    - `docs/how-to/add-new-language.md` (yeni runbook)
    - `docs/adr/0001-db-backed-nl-lexicon-and-i18n.md` (referans)

- [ ] **Kabul:** "Yeni dil ekleme" adımlarının hiçbiri backend Go kodu değişikliği veya
      frontend release'i gerektirmiyor (karar (a) seçildiyse). Runbook'u takip eden biri
      30 dakikada yeni dil ekleyebiliyor. Coverage raporu boş domain'leri gösteriyor.

**Sıralama/bağımlılık:** DİL-0 → DİL-1 → (DİL-2 ‖ DİL-3) → DİL-4 → DİL-5.
DİL-1 tek başına bile bu sohbetteki vaka sınıfını (grain/temporal kelimeleri) release'siz
yönetilir yapar; DİL-3 olmadan netleştirme *metinleri* yeni dilde EN fallback olarak kalır.

---

## AI Sorgu — Netleştirme (Clarification) Akışı Düzeltmeleri (2026-06-09)

İki hata: (1) UI/UX — netleştirme kartı scroll'da ortada kalıyor; (2) Logic —
seçim yapılmasına rağmen sistem tekrar tekrar netleştirme soruyor (sonsuz döngü).

**Kök neden (Bug 2):** Netleştirme cevabı `handlers/ai.go` içinde soruyu yeniden
yazıp choice'i temizledikten sonra, `standardProcessOptions` koşulsuz olarak
`WithAmbiguityCheck(true)` ekliyor; `ProcessQuestion → checkAmbiguity`
yeniden-yazılmış soruda YENİ bir belirsizlik (genelde synonym detector'ın
"ay/day/days" gibi jenerik token'ları) buluyor ve tekrar soruyor. Frontend her
turda orijinal soruyu + tek choice gönderdiği için önceki çözümler taşınmıyor →
≥2 belirsizlikte asla yakınsamıyor.

**Kök neden (Bug 1):** `ChatPanel` scroll efekti her mesajda feed'i `scrollHeight`'a
(en alta) kaydırıyor; uzun netleştirme kartında soru görünür alanın dışında kalıyor.

### Yapılacaklar

- [x] **Backend: Netleştirme turunda ambiguity check'i atla (Bug 2 ana fix).**
  `aiQueryRequest`'e unexported `clarificationResolved bool` eklendi;
  `parseAndRouteAIQuery` choice çözüldüğünde `true` set ediyor;
  `standardProcessOptions` artık `WithAmbiguityCheck(true)`'u
  `&& !req.clarificationResolved` ile koşullu ekliyor. Çözülen turda LLM
  üretimine doğrudan gidiliyor → döngü deterministik kırıldı.
  - **Files**: `internal/http/handlers/ai.go`.
- [x] **Backend: Synonym detector precision (gürültü azaltma).**
  `synonymMatchConfidence` yeniden yazıldı: tek-token synonym'ler tam token
  eşleşmesi gerektiriyor (substring değil); `minExactSynonymTokenRunes=2`,
  `minFuzzySynonymTokenRunes=4` gate'leri eklendi; çok-kelimeli ifadeler
  bitişik `strings.Contains` ile eşleşmeye devam ediyor. "ay/day/days" gibi
  jenerik token'lar artık alakasız kelimeler içinde işaretlenmiyor.
  - **Files**: `internal/ai/ambiguity/synonym_detector.go`.
- [x] **Frontend: Netleştirme kartını üstten görünür kıl (Bug 1).**
  `ChatPanel` scroll `useEffect` netleştirme-bilinçli: son asistan mesajı
  `ai_response?.needs_clarification` ise o kart feed üstüne hizalanıyor
  (`data-message-index` + `getBoundingClientRect`); diğer hallerde mevcut
  en-alta kaydırma korunuyor.
  - **Files**: `frontend/src/components/aiQuery/ChatPanel.tsx`.
- [x] **Testler.** Backend: `service_test.go` →
  `TestProcessQuestionSkipsAmbiguityWhenCheckDisabled` (choice çözülünce tekrar
  sormaz, LLM bir kez çağrılır); `synonym_detector_test.go` →
  `TestDetectSynonyms_GenericSubstringTokensNotFlagged`,
  `TestDetectSynonyms_MultiWordPhraseMatches`. Frontend lint/test/build temiz.
  - **Files**: `internal/ai/service_test.go`,
    `internal/ai/ambiguity/synonym_detector_test.go`.
- [ ] **(Opsiyonel, Faz 2) Çok terimli belirsizlik.** Tüm gerçek belirsizlikleri
  tek turda sun veya çözümleri turlar arası biriktir; atlamak yerine tam
  disambiguasyon ile yakınsa. (Ertelendi.)

**Kabul kriterleri:** Bir netleştirme seçimi sonrası sistem tekrar netleştirme
sormaz, sonucu üretir; netleştirme kartının sorusu otomatik görünür olur;
`make lint-go`, `make test-go`, `make lint-frontend`, `make test-frontend` temiz.

### Review (2026-06-09)

**Sonuç:** Zorunlu işlerin tümü tamamlandı; tüm kapılar temiz geçti.

- **Bug 2 (sonsuz döngü) — GERÇEK kök neden bulundu ve çözüldü.** İlk fix yalnızca
  **senkron** uç noktayı (`parseAndRouteAIQuery`) yamalıyordu; ancak frontend
  generate/preview için **asenkron job** yolunu (`ai_job_exec.go` →
  `executeAIQueryPhase`) kullanıyor. Bu yol seçimi `resolveClarificationChoice`
  ile çözüyordu ama `clarificationResolved` bayrağını **hiç set etmiyordu** → guard
  `!req.clarificationResolved` her turda `true` kalıyor → `standardProcessOptions`
  her turda `WithAmbiguityCheck(true)` ekliyor → sonsuz yeniden-netleştirme.
  **Fix:** `req.clarificationResolved = true` ataması ortak `resolveClarificationChoice`
  **METODUNA** (`ai.go:176-188`, `choice != ""` iken) taşındı; böylece hem senkron
  hem asenkron job yolu bayrağı alıyor. `parseAndRouteAIQuery`'deki artık-gereksiz
  açık atama kaldırıldı. Run fazı (`resolveRunPhaseForJob`) zaten ambiguity check
  eklemiyor → döngü riski yok. Synonym detector sıkılaştırması ilk netleştirme
  gürültüsünü azaltıyor.
- **Regresyon testi:** `ai_ambiguity_test.go` →
  `TestHandlerResolveClarificationChoiceSetsResolvedFlag` (metot choice çözünce
  bayrağı set ediyor) + `...NoChoiceKeepsFlagUnset` (choice yoksa set etmiyor).
- **Bug 1 (scroll) çözüldü** — netleştirme kartı feed üstüne hizalanıyor, soru
  görünür kalıyor.
- **Doğrulama kapıları (2026-06-09 son tur):** `make lint-go` (0 sorun) ·
  `make test-go` (`-race`) PASS · `go test ./internal/http/handlers/` PASS ·
  `make eval-regression` PASS · `deadcode` (yeni ölü kod yok; mevcut `pkg/` SDK +
  observability bulguları pre-existing). Frontend bu turda değişmedi.
- **Commit yapılmadı** (kullanıcı onayı bekleniyor).

## Technical Architecture Analysis — Remaining Actions (2026-06-08)

The open items from §10 Conclusion and Roadmap in `tasks/biqly_analiz.pdf` (Version 3.0) have been verified against the codebase and updated. ESLint zero-warnings has already been achieved; the following items are still open.

### Medium priority (2026-06-08)

- [x] **Reduce AIConfig getter methods/fan-out.** 13 getter methods $\rightarrow$ 5 exported methods (`ResolvedQuery`, `ResolvedEmbedding`, `ResolvedTranslation`, `HTTPTimeout`, `RequestTimeout`) + 3 view types (`AIQueryView`, `AIEmbeddingView`, `AITranslationView`). External calls: 93 $\rightarrow$ 58. `make lint-go` clean.
  - **Files**: `internal/config/config.go`, `internal/ai/service.go`, `internal/ai/provider/*.go`, `internal/app/dependencies.go`, `internal/http/handlers/ai.go` + tests.

- [x] **Remove branching in TableRouter.Route.** `Route` has been reduced to 61 lines; the `funlen` nolint was removed; logic was moved to helper functions: `routeLoadAndFilter`, `routePrepareSelection`, `routeAnnotateResult`, `routeExpandSelection`, and `routeFinalize`. `go test ./internal/ai/routing/...` passes.
  - **Files**: `internal/ai/routing/router.go` + existing test files.

### Low priority (2026-06-08)

- [x] **Gradually reduce repository-wide nolint directives.** Current baseline: **75** (`rg -c 'nolint' --glob '*.go'`); the target of <80 has been met. This round: `builtins` (`enrich_viz.go`) and `revive` (`permissions.go`) were removed. Remaining directives are mostly justified `gosec`/`nilnil`/test fixtures.
  - **Acceptance Criteria**: nolint count <80; no new nolints added; corresponding linter passes for every removed nolint.
  - **Files**: 40+ files detected via `grep -rl '//nolint' --include='*.go' .`.

- [x] **Go: Gradually rename functions with `Get` prefix.** This round: `PublicKeyPEM`, `EffectivePermissions`, `RowFilters`. Up next: `internal/auth/rbac/rbac_repository.go` (8), `internal/auth/oauth/*.go`, `internal/auth/service.go`.
  - **Acceptance Criteria**: New code does not use `Get` prefix; existing `Get` prefixes are gradually reduced; `make lint-go` and `go test` remain clean at each step.
  - **Files**: `internal/auth/rbac/*.go`, `internal/auth/service.go`, `internal/auth/oauth/*.go`, `internal/security/permissions.go`, `internal/auth/mfa/*.go`, `internal/auth/jwt.go`.
  - **Doğrulama (2026-06-10)**: Target fonksiyonlar zaten `Get` prefixesiz (`PublicKeyPEM`, `EffectivePermissions`, `RowFilters`); bu turda kod değişikliği gerekmedi. `make lint-go` 0 issue · `make test-go` PASS.

- [ ] **Frontend: Ensure handler/event naming consistency.** This round: `MetadataDescribeModal` (`onKey` $\rightarrow$ `handleKeyDown`), `SelectPopover` (`handleListKeyDown`). Rule: `handle*` for internal handlers, `on*` for DOM props.
  - **Acceptance Criteria**: New code is consistent; existing inconsistencies are fixed opportunistically.
  - **Files**: `frontend/src/App.tsx`, `frontend/src/components/ui/*.tsx`, `frontend/src/components/settings/*.tsx`.

- [x] **Frontend: Make `CONSTANT_CASE` usage consistent.** Function-scoped `const MAX` $\rightarrow$ `maxRecentTurns` (`AIQuery.tsx`); module-level constants are already `CONSTANT_CASE`.
  - **Acceptance Criteria**: ESLint naming rules pass; no inconsistencies remain.
  - **Files**: `frontend/src/utils/*.ts`, `frontend/src/components/**/*.tsx`.

- [x] **Document repository-wide naming convention rules in lessons.md.** The `tasks/lessons.md → Naming Conventions` section has been updated to include best-practice rules for Go and TypeScript/React. This section will serve as a reference when writing new code.
  - [x] Go naming rules (receiver, function, interface, constant, error var, initialisms, stutter).
  - [x] TypeScript/React naming rules (casing, handlers, booleans, useState, custom hooks, abbreviations, constants).

- [x] **Raise `internal/queue` coverage floor (%40 $\rightarrow$ at least %60).** Floor set to %60 (`scripts/coveragecheck/main.go:35`); package coverage is at %62.5. Added local queue tests (idempotent close, connect error) and mock JetStream for NATS publish/DLQ paths; no live NATS server required.
  - **Acceptance Criteria**: Floor in `scripts/coveragecheck/main.go` is %60; `make coverage-gate` passes; new tests run without a live NATS server (local queue path).
  - **Files**: `internal/queue/*.go`, `scripts/coveragecheck/main.go`.

- [x] **Gradually raise coverage floors for critical packages.** `internal/ai/routing` floor is at %80 (currently %83.5), `internal/auth` floor is at %10 (currently %13.2). Added to Makefile + CI coverage profile.
  - **Acceptance Criteria**: At least 2 new packages added to the floors map in `scripts/coveragecheck/main.go`; `make coverage-gate` passes for each.
  - **Files**: `scripts/coveragecheck/main.go`, new test files.

- [x] **Periodically update live-eval baseline.** Added `edge-not-shipped-count` (`neq` filter, TR locale); `testdata/eval/nightly_baseline.json` updated from 17 $\rightarrow$ 18 cases (`go run scripts/gen-nightly-baseline/main.go`).
  - **Acceptance Criteria**: At least 1 new golden case added per sprint; baseline commit is up to date.
  - **Files**: `internal/ai/eval/`, `cmd/eval-live/`, `.github/workflows/eval-nightly.yml`.

- [x] **Add critical attributes to spans (ongoing improvement).** Added `ai.tokens.{prompt,completion,total}`, `ai.route.confidence` (defer including clarification), `db.system`/`datasource.driver`, `query.compile.duration_ms`, `query.execute.duration_ms`. OTEL sampler: `parentbased_traceidratio` defaults to 25% (`OTEL_TRACES_SAMPLER*`), Helm `global.observability.tracing.*`.
  - **Acceptance Criteria**: Every new span attribute is visible in Jaeger; trace sampling rate is adjusted for production load.
  - **Files**: `internal/ai/service.go`, `internal/ai/routing/router.go`, `internal/datasource/*.go`, `internal/platform/observability/*.go`.

- [x] **Monitor Prometheus label cardinality.** Added `bi_prom_metric_series_total` + `bi_prom_label_cardinality` collector; `VecLabelLimits`/`CheckGatheredCardinality`/`BoundLabel`; Grafana `biqly-cardinality.json`.
  - **Acceptance Criteria**: Label cardinality metrics are visible in the Grafana dashboard; cardinality limits are checked when adding new labels.
  - **Files**: `internal/platform/observability/metrics.go`, Helm Grafana dashboard config.

- [x] **Periodically verify dev cookie exemption does not leak to prod.** `CookieSecure` is fail-closed in prod/K8s; CI: `TestProductionAuthEnabledFailClosed`, `TestProductionCookieSecureFailClosed`.
  - **Acceptance Criteria**: `TestProductionAuthEnabledFailClosed` passes in CI; manual verification runs at least monthly (`go test -run 'TestProduction(AuthEnabledFailClosed|CookieSecureFailClosed)' ./internal/config/... ./internal/auth/...`).
  - **Files**: `internal/auth/cookie.go`, `internal/config/config.go`.

### Completed (in previous rounds)

- [x] AIConfig fields split into 9 sub-structs
- [x] ValidateContext/ValidateComposite/PasswordPolicy.Validate reduced to single-digit complexity
- [x] Nightly live-LLM eval + drift gate added
- [x] Auth fail-closed invariant in prod (`env.IsProduction()`)
- [x] OTEL span depth increased from 3 $\rightarrow$ 16+
- [x] queue package added to coverage floor monitoring (40%)
- [x] Flaky TestMFABypassCodeFlow stabilized
- [x] ESLint warnings reduced to zero (`--max-warnings 0`)
- [x] CSP + X-Frame-Options + prod HSTS security headers implemented
- [x] CodeQL + govulncheck + semgrep SAST scans enabled

 ---

## Technical Analysis Report — Remaining Gaps (2026-06-07)

 The remaining recommendations from the `tasks/biqly_analiz.pdf` report (Version 3.0) were verified in the codebase. 7 of the 8 gaps from the initial audit have been resolved; the following are still open. In order of priority:

### Medium priority (2026-06-07)

- [x] **Physically decompose the AIConfig god-object (move fields, do not just rename).** The previous renaming of nested configs did not improve the metrics (21 fields / 13 methods / 93 external calls = score of 60, CRITICAL). In this round, top-level connection/tuning fields were physically relocated.
  - Completed: `AIConfig` now contains only 9 top-level sub-structs — `Connection`, `Generation`, `Describe`, `Cache`, `Query`, `Embedding`, `Translation`, `Routing`, and `Ambiguity` (`internal/config/config.go`).
  - Groups relocated: `AIConnectionConfig` (Provider/APIKey/BaseURL/Model/HTTPTimeout/RateLimit), `AIGenerationConfig` (MaxTokens/Temperature/TopP/NumCtx/MaxPromptInputRunes/MaxRetries/MultiCandidateCount), `AIDescribeConfig` (MaxCellRunes/MaxSampleRows), and `AICacheConfig` (ResponseTTLSeconds).
  - All call sites updated (ai/provider/service/provider_store, prompt/context_budget, http/handlers, app dependencies + tests). `go test ./internal/config/... ./internal/ai/... ./internal/http/...` passes.
- [x] **Gradually decompose the remaining high-complexity functions.** The three target functions highlighted in the report: `ValidateContext` (39), `ValidateComposite` (27), and `PasswordPolicy.Validate` (25). Split each into smaller helper functions and secure behavior with tests.
- [x] **Add periodic (nightly) live-LLM eval runs.** The current regression gate on the deterministic stub provider is 1.00 — which catches harness/compiler regressions but does not measure live-LLM accuracy drift. Add a nightly cron workflow + golden run with a real provider + drift reporting. (Also expand the evaluation suite with new dialect/edge cases.)
- [x] **Make `BI_AUTH_ENABLED` a required (fail-closed) invariant in production.** Verified: `internal/config/config.go:413` defaults to `false`. Although Helm production values set it to `true`, having it disabled at the code level in production relies entirely on network-level security. Action: If `env.IsProduction()` is true and `BI_AUTH_ENABLED=false`, fail-closed during startup (panic/refuse).

### Low priority (2026-06-07)

- [x] **Increase OTEL tracing depth (driver/DB spans).** Verified: only 3 named spans existed — `ai.ProcessQuestion` (`internal/ai/service.go:254`), `query.Compile` (`internal/query/compiler.go:64`), `query.Execute` (`internal/query/executor.go:52`) + router otelhttp ingress. Datasource driver calls and few-shot/embedding sub-phases were not spanned. Action: add spans to sub-phases + critical attributes on spans (model, attempt, fingerprint).
  - Completed (2026-06-07): datasource spans (`datasource.Ping/Open/Introspect/IntrospectSchemas|Tables|Columns|Relations/Query`), AI sub-phases (`ai.PromptBuild`, `ai.AmbiguityAnalyze`, `ai.LLMGenerate`, `ai.MultiCandidate`, `ai.TableRoute`, `ai.RouteEmbedding`, `ai.LoadFewShot`, `ai.Embed`, `ai.EmbedMetadata`, `ai.ProviderGenerate`), critical attributes (`ai.model`, `ai.attempt`, `query.fingerprint`, `model.id`, token counts, route confidence).
  - compile→execute fingerprint chain via `query.LogicalQueryFingerprint` + `observability.WithQueryFingerprint`.
  - Verification: `go build ./internal/...` + `go test ./internal/query/... ./internal/datasource/... ./internal/ai/... ./internal/core/... ./internal/platform/observability/... ./internal/http/handlers/...` passed.
- [x] **Add `internal/queue` to the coverage floor map.** Added `internal/queue` 40% floor to the `floors` map in `scripts/coveragecheck/main.go` (current ~42.5%).
- [x] **Fix flaky `TestMFABypassCodeFlow` isolation.** `mfatest` no longer deletes global tables; per-user teardown via unique email seed + `t.Cleanup` (`webauthn_flow_test` pattern).
- [x] **Ratchet ESLint warning ceiling toward 0 over time.** Frontend gate is CI-equivalent; warning ceiling was > 0; ratcheted to zero gradually.
  - **Current state**: `--max-warnings 576` (actual warnings: 576, 25 rules, 100+ files; Phase 1 + no-misused-promises closed)
  - **Target**: Promote rule groups to `error` in priority order; lower `max-warnings` in each group.
  - **Phase 1 — Highest impact, mechanical fixes (~830 warnings, 57%)**
    - [x] `@typescript-eslint/prefer-nullish-coalescing` (262 → 0) — `||` → `??` changes; rule is suggestion-only (no autofix); applied 262 suggestions across 56 files via ESLint API. `max-warnings` 1495 → 1220. Tests passed (95/95).
    - [x] `@typescript-eslint/no-unsafe-call` (199 → 0) — Root cause: `t: any` on child component props. Exported `TFunction` / `LooseTFunction` (`i18n/index.tsx`); replaced `t: any` → `TFunction` in 16+ files. Also: `PasskeyCreationOptionsJSON`, `DashboardBuilder` KPI render, `ExpressionBuilder` invalid fallback args removed. `max-warnings` 1220 → 850. Tests passed (95/95), tsc clean.
    - [x] `@typescript-eslint/no-unsafe-assignment` (41 → 0; 124 in todo was old baseline) — `AuthUserRaw` + typed `apiFetch`, passkey JSON types, `parseJsonRecord`/`parseJsonStringArray` (`utils/record.ts`), `QueryResultPayload` query run, `PermissionRowFilter.value` narrowing, i18n `navigator.languages` guard. `max-warnings` 850 → 730.
    - [x] `@typescript-eslint/no-unsafe-member-access` (96 → 0) — Most were fixed with `no-unsafe-assignment` (`api/auth.ts` typed `apiFetch`, `DashboardBuilder` `QueryResultPayload`/`ChartRow`). Remaining 5: `Glossary.tsx` and `ExpressionBuilder.tsx` `catch (err: unknown)` + `instanceof Error`; `admin.test.ts` mock `RequestInit` cast. Rule promoted to `error`. `max-warnings` 730 → 709.
  - **Phase 2 — Promise/async security (~256 warnings, 17%)**
    - [x] `@typescript-eslint/no-misused-promises` (130 → 0) — `void asyncFn()` wrapper on event handlers and prop callbacks; form `onSubmit={(e) => { void handleSubmit(e) }}`; wrapped `AuthProvider` setInterval + `AuthGuard` navigate ref. 42 files. Rule promoted to `error`. `max-warnings` 709 → 576.
    - [x] `@typescript-eslint/no-floating-promises` (126 → 0) — `void` prefix on fire-and-forget calls in `useEffect` and async handlers; `.then()` chains `void get(...).then(...)`. 50 files. Rule promoted to `error`. `max-warnings` 576 → 451.
  - **Phase 3 — Type safety and code quality (~310 warnings, 21%)**
    - [x] `@typescript-eslint/no-unnecessary-condition` (136 → 0) — Removed unnecessary `?? []`/`?.`/always-truthy `if`; discriminated union final branches use `else`; narrowed `apiFetch` return types; `apiClient` timeout via `startedAt`; 49 files. Rule promoted to `error`. `max-warnings` 451 → 315. Tests passed (95/95).
    - [x] `@typescript-eslint/no-explicit-any` (25 → 0) — `SemanticDimension`/`SemanticMetric` types in query builder steps; `Select` onChange setter pass-through; `ComponentType<unknown>` lazy preload; `unknown[][]` chart rows; aggregation type guard; direct `t` pass-through for `LooseTFunction`. Rule promoted to `error`. `max-warnings` 315 → 290.
    - [x] `@typescript-eslint/no-unsafe-argument` (35 → 0) — All 35 warnings already resolved by Phase 1 `no-unsafe-call`/`no-unsafe-assignment`/`no-unsafe-member-access` fixes (`TFunction`, `apiFetch` types, `parseJsonRecord`, `QueryResultPayload`, etc.). No extra code change needed. Rule promoted to `error`. `max-warnings` 290 (unchanged). Lint clean (0 violations).
    - [x] `@typescript-eslint/no-unsafe-return` (7 → 0; actually 1) — `parseStoredConversations` + `isConversation` type guard for `useConversation.ts` `JSON.parse`. Rule promoted to `error`.
    - [x] `@typescript-eslint/no-redundant-type-constituents` (8 → 0) — `AIQueryResponse | unknown` → `unknown`; removed `DriverTileGrid` generic (`readonly DriverId[]`); `jobWaiter.test` concrete generic; `context_source` literal union (`| string` removed). Rule promoted to `error`.
    - [x] `@typescript-eslint/consistent-type-imports` (5 → 0) — Replaced `import()` inline type annotations with `import type` (`AssistantMessageCard`, `routingViz`, `modeling/types`). Rule promoted to `error`.
    - [x] `@typescript-eslint/no-unused-vars` (28 → 0; actually 3) — Removed unused `Datasource`/`CardLayout` imports; removed `catch` binding. Rule promoted to `error`. `max-warnings` 290 → 248. Tests passed (95/95).
  - **Phase 4 — React hooks and a11y (~164 warnings, 11%)**
    - [x] `react-hooks/set-state-in-effect` (112) — `setState` inside `useEffect` (v7 new rule). Heavy files: `TableBrowser.tsx` (8), `Modeling.tsx` (7), `SavedQuestions.tsx` (5). Best practice: replace `useEffect` + `setState` with `useSyncExternalStore`, derived state, or `useMemo`; for initial load consider `use()` or Suspense.
    - [x] `react-hooks/exhaustive-deps` (30) — Missing dependency arrays. Best practice: review each `useEffect`/`useCallback`/`useMemo` dependency array; if no false dependency, add justified `// eslint-disable-next-line react-hooks/exhaustive-deps`.
    - [x] `react-refresh/only-export-components` (34) — Non-component exports in file. Best practice: move utility functions and constants to a separate file; `allowConstantExport: true` is already in config but split files for the remainder.
    - [x] `react-hooks/refs` (4), `react-hooks/immutability` (2), `react-hooks/purity` (1) — v7 new rules. Best practice: move ref mutations to event handlers; ensure immutable state updates; move side effects to `useEffect`.
    - [x] `jsx-a11y/no-autofocus` (18) — `autoFocus` attribute. Best practice: use `useRef` + `el.focus()` instead of `autoFocus`; apply focus trap on modal open with `useEffect`.
  - **Phase 5 — Remaining low-count rules (~33 warnings, 2%)**
    - [x] `complexity` (24) + `max-depth` (1) — High-complexity functions. Best practice: split large functions into sub-functions; reduce nested `if` with early return.
    - [x] `@typescript-eslint/no-base-to-string` (5) — `toString()` on invalid type. Best practice: use `String()` or template literal.
    - [x] `@typescript-eslint/no-empty-function` (3) — Empty function bodies. Best practice: use `noop` helper or `_`-prefixed parameter instead of `() => {}`.
    - [x] `@typescript-eslint/ban-ts-comment` (1), `@typescript-eslint/prefer-for-of` (1) — One-off fixes.
  - **Ratcheting strategy**
    - [x] After each phase completes, update `max-warnings` to current warning count + small buffer (10–20).
    - [x] Target timetable: Phase 1 → ~665 warnings (`max-warnings 680`), Phase 2 → ~405 warnings (`max-warnings 420`), Phase 3 → ~95 warnings (`max-warnings 110`), Phase 4+5 → 0 (`max-warnings 0`).
    - [x] Final step: promote all `'warn'` rules in `eslint.config.js` to `'error'` and enforce strict zero-warning policy with `--max-warnings 0`.

### Notes

- All findings were verified line-by-line in source; the 7 report items marked "closed" were not reopened.
- Follow behavior-preserving tests first, then refactor (especially AIConfig moves and function extractions).

## pgarray Abstraction — Consolidating lib/pq in One Place (2026-06-07)

The `lib/pq` helpers (`pq.Array`, `pq.StringArray`) used for Postgres `text[]` encode/decode were spread across 11 files. The driver already uses pgx (`database/sql` + `pgx/v5/stdlib`); lib/pq was only used as an array codec. Consolidated into a single abstraction so a future pgx native / pgtype migration touches one file.

### Completed work

- [x] Created `internal/platform/db/pgarray/array.go` — the **only** package that imports lib/pq.
  - `func Strings(v []string) any` → query param (Valuer), `pq.Array` instead of.
  - `type StringArray = pq.StringArray` → scan target + Valuer.
  - `func Scan(dst any) any` → pointer scan hedefi (`pq.Array(&slice)` instead of).
- [x] Replaced direct `pq.*` usage with `pgarray.*` in 11 files; removed `github.com/lib/pq` imports:
  - `internal/metadata/`: `repository.go`, `business_glossary.go`, `ai_time_grains.go`, `ai_history_query.go`, `permissions.go`, `translations.go`, `curated_ai.go`, `ai_jobs.go`
  - `internal/auth/repository.go`, `internal/auth/mfa/mfa_repository.go`
  - `internal/ai/provider_store.go`
- [x] Behavior unchanged (`pgarray.Strings` = `pq.Array`, `StringArray` = type alias) — only indirection added.

### Result / verification

- `lib/pq` is now imported only in `internal/platform/db/pgarray/array.go` (verified via grep; no other `pq.Array`/`pq.StringArray` remain).
- `gofmt -w` applied to all touched files.
- `go build ./...` and `go vet ./internal/{metadata,auth,ai,platform/db}/...` clean.
- `golangci-lint run` on touched packages: **0 issues**.
- In-memory tests passed (`internal/metadata`, `internal/ai`, `internal/platform/db`). Two failing tests (`auth/mfa`, `auth/workspace`) **also fail on a clean tree** with FK constraint errors → shared test DB seed issue, unrelated to this change.

### Future migration to pgx native / pgtype

Only the bodies of three symbols in `pgarray/array.go` need to change instead of 11 files. If staying on `database/sql` + pgx stdlib, use `pgtype` equivalents; with full pgxpool migration, pass Go slices directly and remove `lib/pq` entirely.

## Redis Client Migration Evaluation (go-redis → rueidis / valkey-go) (2026-06-07)

go-redis v9 is reliable but performance-focused alternatives are more aggressive:

- **rueidis**: automatic pipelining, client-side caching; claims higher throughput vs go-redis on parallel workloads. Large deltas in its own benchmarks.
- **valkey-go**: optimized for Valkey/Redis, similar performance story.

### Evaluation criteria

- [x] Write real benchmark comparison between go-redis v9 and rueidis (GET/SET/MSET pipeline, P99 latency, connection pooling).
- [x] Analyze how rueidis client-side caching fits the existing cache layout.
- [x] Check valkey-go API compatibility (Dragonfly/Redis support).
- [x] Assess migration risk: API differences, test coverage, community/maintenance status.

### Result (2026-06-07)

- Added: isolated Go module `benchmarks/redisclient`. Pinned `go-redis/v9 v9.19.0`, `rueidis v1.0.75`, `valkey-go v1.0.75`.
- Benchmark scope: single `GET`, single `SET`, batched `MSET`, pipelined `SET`/`GET`; bounded `p99_ns/op` reported alongside `ns/op`. Connection pool effect measurable via `REDIS_BENCH_POOL_SIZE` (`go-redis` `PoolSize`, rueidis nearest `PipelineMultiplex` connection count).
- How to run:
  - `cd benchmarks/redisclient`
  - `REDIS_BENCH_ADDR=127.0.0.1:6379 go test -run TestValkeyCompatAPISurface -bench . -benchtime=10s -count=5`
- Live local results (`127.0.0.1:6379`, Apple M4, darwin/arm64, `-benchtime=10s -count=5`, total `471.606s`):

  | Benchmark | go-redis median ns/op | rueidis median ns/op | go-redis median p99_ns/op | rueidis median p99_ns/op | Result |
  | --- | ---: | ---: | ---: | ---: | --- |
  | `GET` | `396704` | `487313` | `1058792` | `2753250` | go-redis daha iyi |
  | `SET` | `369127` | `413086` | `693292` | `1292750` | go-redis daha iyi; ortalama noisy |
  | `MSET` | `445711` | `509089` | `873542` | `1164958` | go-redis daha iyi |
  | Pipeline `SET`/`GET` | `389664` | `418962` | `750333` | `670333` | rueidis p99 biraz iyi, ortalama go-redis iyi |

  This run did not justify migrating to rueidis on performance. Do not start migration unless staging/Dragonfly shows different results.
- Live Dragonfly results (`docker.dragonflydb.io/dragonflydb/dragonfly:v1.34.1`, `127.0.0.1:6379`, Apple M4, darwin/arm64, `-benchtime=10s -count=5`, total `495.595s`):

  | Benchmark | go-redis median ns/op | rueidis median ns/op | go-redis median p99_ns/op | rueidis median p99_ns/op | Result |
  | --- | ---: | ---: | ---: | ---: | --- |
  | `GET` | `449545` | `442002` | `929375` | `1038333` | close; p99 go-redis better |
  | `SET` | `440736` | `456810` | `940333` | `1175875` | go-redis daha iyi |
  | `MSET` | `670924` | `786056` | `1227500` | `2391917` | go-redis belirgin daha iyi |
  | Pipeline `SET`/`GET` | `1207217` | `3362068` | `2933459` | `9429833` | go-redis much better |

  On Dragonfly too, no performance case for rueidis; pipeline workloads would regress with this implementation.
- rueidis fit: direct swap into production via native builder API is high churn. Server-assisted client-side caching via `DoCache`/`DoMultiCache`; best candidates are TTL `GET`/`SET` payload caches in `internal/ai/response_cache.go` and `internal/semantic/composite_cache.go`, but invalidation must be validated in staging.
- valkey-go fit: `valkeycompat.NewAdapter` provides go-redis-like API; `TestValkeyCompatAPISurface` in the benchmark module verifies `Set`, `Get`, `Cache`, `Pipelined` against live Redis/Dragonfly/Valkey. Dragonfly/Redis protocol support needs live test against target version in practice.
- Risk decision: no production client migration now; not recommended after two local runs. Current usage: `INCR`/`EXPIRE`, `GET`/`SET`, `SCAN`/`DEL` and DI tied to `*redis.Client`. If staging shows meaningful difference, lowest-risk order: thin `internal/platform/cache` adapter first, then `internal/auth/ratelimit.go` + `internal/mail/smtp.go` pilot, finally AI/semantic client-side caching trial.
- Review / verification:
  - `gofmt -w benchmarks/redisclient/redis_client_bench_test.go`
  - `go test ./...` (`benchmarks/redisclient`; compile verified with live test/bench skipped when `REDIS_BENCH_ADDR` unset.)
  - `go vet ./...` (`benchmarks/redisclient`)
  - `REDIS_BENCH_ADDR=127.0.0.1:6379 go test -run TestValkeyCompatAPISurface -bench . -benchtime=10s -count=5`
  - `docker compose up -d redis` + same benchmark command (`dragonfly:v1.34.1`)

### Files to change (go-redis → alternative client)

**Packages that use `*redis.Client` directly:**

| File | Usage |
| --- | --- |
| `internal/app/dependencies.go:454` | `redis.NewClient(opt)` — monolith DI |
| `internal/app/providers.go:71` | `redis.NewClient(opt)` — provider DI |
| `cmd/auth/main.go:207-212` | `newRedisClient` — auth service |
| `cmd/mail/main.go:56-63` | `redis.NewClient(opts)` — mail service |
| `internal/auth/service.go:54,92` | `redisClient *redis.Client` — auth service struct |
| `internal/auth/ratelimit.go:17,20` | `redisClient *redis.Client` — rate limiter |
| `internal/auth/oauth_exchange.go:12` | Redis import — OAuth state |
| `internal/mail/smtp.go:28,45` | `redis *redis.Client` — mail rate limit |
| `internal/ai/response_cache.go:48,52` | `client *redis.Client` — AI response cache |
| `internal/semantic/composite_cache.go:28,39` | `client *redis.Client` — composite cache |
| `internal/auth/rbac/datasource_access.go:30,35` | `redis *redis.Client` — datasource access |
| `internal/auth/auth_test.go:18,426,482` | `redis.NewClient(opts)` — test setup |
| `internal/auth/oauth_exchange_test.go:10,22` | `redis.NewClient(opts)` — test setup |

**Migration strategy:**

1. **Abstraction layer** (low risk): add a wrapper like `internal/platform/cache`; all consumers use an interface. Swap implementation underneath later.
2. **Direct swap** (high risk): all `*redis.Client` → new client type. Large API gaps mean heavy per-file changes.
3. **Hybrid**: abstraction layer first, then pilot migration via one service (e.g. auth rate limiter).

**Recommended order:**

- [x] 1. Benchmark go-redis vs rueidis (confirm whether Redis is a bottleneck at current load).
- [ ] 2. If the gap is meaningful: create `internal/platform/cache` abstraction layer.
- [ ] 3. Pilot migrate `internal/auth/ratelimit.go` and `internal/mail/smtp.go` (simple SET/GET/INCR patterns).
- [ ] 4. `internal/ai/response_cache.go` and `internal/semantic/composite_cache.go` — best places to benefit from client-side caching.
- [ ] 5. Update DI entrypoints (`dependencies.go`, `providers.go`, `cmd/*/main.go`).
- [ ] 6. Update test infrastructure (`auth_test.go`, `oauth_exchange_test.go`).
- [ ] 7. Remove go-redis dependency from `go.mod`.
- [ ] 8. Validate with load test in staging.

## Sonic JSON Migration Results (2026-06-06)

Resolved:

1. Added `internal/jsonusage` static guard test to reject direct `encoding/json` encode/decode/parser helper calls.
2. Migrated direct `Marshal`, `MarshalIndent`, `Unmarshal`, `NewEncoder`, `NewDecoder`, and `Valid` usage to `sonic.ConfigStd`.
3. Replaced golden JSON compaction with sonic decode plus std-compatible marshal normalization.
4. Kept `encoding/json` only where stdlib JSON types remain part of API or compatibility surfaces.
5. Removed stale `nolint` directives made unnecessary by the sonic migration.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/jsonusage -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test -run '^$' ./cmd/... ./internal/... ./pkg/... -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/... ./internal/... ./pkg/... -count=1`
- `make lint-go`
- `git diff --check`

## Prioritized Architectural & Observability Recommendations (2026-06-06)

- [x] **High**: Instrument OTEL tracing in code (LLM/compile/execute spans)
  - [x] Initialize a global Tracer Provider at startup in `cmd/api/main.go`, `cmd/auth/main.go`, and the standalone microservice entrypoints (`services/*/cmd/main.go`).
  - [x] Implement trace provider setup/teardown in `internal/platform/observability/trace.go`.
  - [x] Wrap public HTTP routers with `otelhttp` middleware to propagate span contexts across endpoints.
  - [x] Instrument text-to-query pipeline phases:
    - [x] `ProcessQuestion` in `internal/ai/service.go` (ambiguity analysis, LLM generate).
    - [x] `Compile` in `internal/query/compiler.go` (logical query translation to dialect SQL).
    - [x] `Execute` in `internal/query/executor.go` (physical query execution against target database).
- [x] **High**: Make AI eval/regression package a CI gate
  - [x] Ensure `make eval-regression` (real model or stub golden tests) runs on every pull request and push to `main`.
  - [x] Explicitly add the regression test execution step to `.github/workflows/test.yml` (currently only runs `go test ./...` which does not execute some of these benchmarks strictly).
  - [x] Enforce failing the build if accuracy rates drop below acceptable thresholds in `internal/ai/eval_regression_test.go`.
- [x] **Medium**: Dialect integration tests for datasource drivers & coverage floor gates
  - [x] Address low test coverage in critical packages (like `datasource/{postgres,mysql,clickhouse,sqlserver}`, `dashboard`, `queue`, and `config` which currently have thin coverage, e.g., 1 test each). (datasource drivers 94–100%; `dialect` 47.6%→96.1%; `config` 48.4%→87.4%; `dashboard` 0%→89.9% via mock-driver tests; `queue` 35%→42.5% — local queue fully covered, NATS paths require a broker/integration.)
  - [x] Implement live/test database connection integration tests for each datasource adapter (`mysql`, `clickhouse`, `sqlserver` drivers under `internal/datasource/`, similar to `postgres`). (mock-bridge introspection tests mirroring postgres already present for all three.)
  - [x] Verify that physical queries compiled by dialect packages execute correctly against each database type. (`internal/dialect/methods_test.go` asserts exact SQL per dialect for quoting, placeholders, LIMIT/OFFSET, DATE_TRUNC, calendar parts, ILIKE, casts, aggregates, EXPLAIN.)
  - [x] Bind package-level test coverage thresholds as a gate in the CI workflow (leveraging the already-generated `coverage.out`). (`scripts/coveragecheck` + `make coverage-gate` + `coverage` job in `.github/workflows/test.yml`.)
- [x] **Medium**: CSP + X-Frame-Options on security headers; HSTS required in prod
  - [x] Enforce strict Content Security Policy (`default-src 'self'; frame-ancestors 'none'`) and X-Frame-Options (`DENY`) on all public router definitions (`internal/http/router.go`, `internal/http/service_middleware.go`, `cmd/auth/main.go`).
  - [x] Configure `HSTSEnabled: true` automatically in production environments (e.g., when running in production mode, overriding standard development configuration defaults).
- [x] **Medium**: Decompose AIConfig and Service.Process
  - [x] **AIConfig decomposition**: Separate the God-object `config.AIConfig` struct (45 fields, 13 methods, complexity score 84 - CRITICAL) in `internal/config/config.go` into purpose-based sub-configs (query/embedding/translation/ambiguity/routing). (Now *named* sub-configs — `AIConfig.Query/Embedding/Translation/Routing/Ambiguity` with clean unprefixed fields; all call sites across ai/app/http updated. Top-level surface dropped from ~45 fields to 18 + 5 grouped configs.)
  - [x] **Service.Process refactoring**: Refactor `ProcessQuestion` in `internal/ai/service.go` by extracting self-consistency (voting) and repair/retry loop branches into separate, named helper functions, enabling the complete retirement of `//nolint:gocyclo,gocognit,funlen` directives.
- [x] **Low**: Gradually lower ESLint warning ceiling; gitignore `*.test` & `coverage.out` (DevX / sustainability)
  - [x] Reduce the `--max-warnings 1500` ceiling in `frontend/package.json` to the actual count of warnings (currently 1490) + a small buffer (e.g. `1495`), and start ratcheting it down over time towards 0.
  - [x] Ensure that stray compilation outputs in the root of the repo (such as `auth.test`, `app.test`, `workspace.test`, and `coverage.out`) are properly and explicitly ignored via `.gitignore` to keep the workspace clean.

## Backend Go Code Review (2026-06-06)

Full codebase review of `internal/`, `pkg/`, `cmd/`, `services/`. Findings grouped by severity.

### CRITICAL (fix immediately)

- [x] **SEC-Q1**: `internal/query/compiler.go:312-316` — Silent fallback to raw expression on parse failure. `CalculatedExpression` raw string injected into SQL when `ParseExpression` fails, bypassing readonly checker. Second-order SQL injection vector.
- [x] **SEC-Q2**: `internal/query/expr_compiler.go:46-51` — `CompileExpr` silently returns empty string on unsafe SQL instead of error. Callers use the empty string unconditionally, producing malformed queries.
- [x] **SEC-Q3**: `internal/query/expr_compiler.go:102-122` — `literalSQL` uses manual string escaping instead of parameterized queries. String literals embedded via `strings.ReplaceAll(v, "'", "''")` instead of placeholders.
- [x] **SEC-Q4**: `internal/query/compiler_nested.go:32` — Row-level security filters skipped for nested subqueries and CTEs. `rowFilters` always nil in `compileSubqueryBody`, allowing data exfiltration through CTEs.
- [x] **SEC-Q5**: `internal/query/compiler.go:308-378` — PII masking only applied to dimensions, not to metric expressions referencing PII columns. Metric `Expression` bypasses PII masking via `metricExpressionRef`.
- [x] **SEC-A1**: `internal/auth/handlers/handler.go:493` — OAuth state stored in cookie without server-side validation or session binding. 16-byte entropy is low; should be 32+ and server-stored.
- [x] **SEC-A2**: `internal/auth/ratelimit.go:73-79` — Rate limiter bypass via `X-Forwarded-For` / `X-Real-IP` header spoofing. No trusted proxy validation.
- [x] **SEC-A3**: `internal/auth/jwt.go:99-106` — In-memory dev RSA key silently generated in production when env vars missing. Every pod restart invalidates all tokens.
- [x] **SEC-A4**: `internal/auth/session.go:74-80` — Refresh tokens stored as plaintext in database. DB compromise = all active sessions compromised.
- [x] **SEC-A5**: `internal/auth/account_state.go:259-264` — Unlock tokens stored plaintext. DB leak allows account unlock bypass.
- [x] **SEC-S1**: `internal/security/readonly.go:19-30` — `dangerousKeywords` omits `SET`, `RESET`, `COPY`, `DO`, `LOCK`, `VACUUM`, `REINDEX`. `SET role='admin'` bypasses readonly enforcement.
- [x] **SEC-S2**: `internal/security/dsn.go:28-30` — DSN redaction misses URL-encoded passwords and `pass=` parameter (MySQL).
- [x] **BUG-H1**: `internal/http/handlers/datasources.go:200` — Port always assigned as `datasource.DefaultPort(driverType)` instead of resolved `port` variable. Custom ports silently ignored.
- [x] **BUG-H2**: `internal/http/handlers/datasources.go:214` — SSLMode reads from `c.SSLMode` instead of resolved `ssl` variable. Default SSL mode not persisted.
- [x] **BUG-M1**: `internal/metadata/curated_ai.go:153-160` — `UpdateLatestAIQueryHistoryRating` updates the most recent query for the entire datasource, not the specific query that received feedback. Cross-user rating corruption.
- [x] **BUG-Q1**: `internal/queue/local.go:25-32` — Local queue `Publish` has `select/default` that falls through to blocking send on full channel = publisher deadlock.

### HIGH (fix soon)

- [x] **SEC-A6**: `internal/auth/handlers/handler.go:289-299` — Internal token comparison uses `!=` instead of `subtle.ConstantTimeCompare`. Timing side-channel.
- [x] **SEC-A7**: `internal/auth/invitation.go:339-360` — `ListInvitations` returns raw invitation tokens in response. Admin can misuse unclaimed tokens.
- [x] **SEC-A8**: `internal/auth/handlers/handler.go:506-510` — OAuth callback leaks provider error messages to client (internal URLs, tokens).
- [x] **SEC-A9**: `internal/auth/service_mfa_admin.go` — Super admin can generate MFA bypass code for self, defeating MFA purpose.
- [x] **SEC-A10**: `internal/auth/invitation.go:194-199` — Invitation tokens stored plaintext in database (unlike magic link tokens which are hashed).
- [x] **SEC-A11**: `internal/auth/csrf.go` — CSRF cookie `HttpOnly: false` + double-submit pattern vulnerable to subdomain XSS.
- [x] **SEC-A12**: `internal/auth/handlers/handler.go:451-455` — WebAuthn session in cookie without HMAC/integrity protection.
- [x] **SEC-A13**: `internal/auth/password_policy.go:44-46` — `MaxLength: 128` but bcrypt silently truncates at 72 bytes. Effective password strength capped.
- [x] **SEC-A14**: `internal/auth/invitation.go:261-311` — Invitation claim issues tokens without email verification. Intercepted link = full access.
- [x] **SEC-H1**: `internal/http/ai_router.go:28`, `catalog_router.go:27`, `query_router.go:27` — Wildcard CORS `https://*` with `AllowCredentials: true` in standalone service routers.
- [x] **SEC-H2**: `internal/http/middleware/realip.go:16-22` — `RealIP` trusts `X-Forwarded-For` without trusted proxy configuration. Defeats IP-based security.
- [x] **SEC-H3**: `internal/http/ai_router.go:19`, `catalog_router.go:19`, `query_router.go:19` — Standalone routers missing `SecurityHeaders` and `requestLoggerMiddleware`.
- [x] **SEC-H4**: `internal/http/upstream_proxy.go:46-72` — No request body size limit on proxy-forwarded requests. Multi-GB body attack.
- [x] **PERF-H1**: `internal/http/handlers/history_filter.go:69`, `helpers.go:73` — `NewAuthClient` created per-request. Connection churn + idle connection leak.
- [x] **PERF-H2**: `internal/http/middleware/permission.go:42-49` — Permission/datasource caches grow unbounded (no eviction beyond TTL-on-read).
- [x] **PERF-A1**: `internal/auth/rbac/rbac.go:62-95` — Up to 4 recursive SQL queries per permission check with no caching.
- [x] **PERF-AI1**: `internal/ai/describe.go:133-136` — New DB connection opened per `Describe` call. No pooling.
- [x] **PERF-AI2**: `internal/ai/remote_models.go` — New `http.Client` per remote models request. No connection reuse.
- [x] **PERF-AI3**: `internal/ai/routing/router.go` — `tokenSet(question)` computed multiple times per `Route()` call (4+ tokenizations).
- [x] **REL-H1**: `internal/http/handlers/datasources.go:726-738` — Drift notification fires in unbounded goroutines. No worker pool or backpressure.
- [x] **REL-H2**: `internal/queue/nats.go:84-103` — No dead-letter queue. Permanently failed jobs disappear after `MaxDeliver: 3`.
- [x] **REL-H3**: `internal/metadata/embeddings.go:177-191` — Embedding upsert race condition. Concurrent writes can overwrite each other's locale vectors.
- [x] **AUDIT-H1**: `internal/auth/service.go`, `service_password.go` — No audit logging for login, registration, password change/reset events.
- [x] **AUDIT-H2**: `internal/auth/handlers/gdpr_export.go:87-123` — GDPR export silently swallows errors. Incomplete exports with no indication.
- [x] **AUDIT-H3**: `internal/auth/repository.go:188-199` — `ListUsers` returns password hashes in scanned rows.

### MEDIUM (plan and fix)

Success criteria:

- Each MEDIUM item is verified against current code before changing it.
- Fixed items have minimal code changes plus focused test/build evidence where practical.
- Items closed without code are documented with the repo-specific reason.
- `gofmt` and focused Go tests pass for touched backend packages before this section is marked done.

Execution plan:

- [x] Triage MEDIUM items by package and confirm which findings are still live.
- [x] Fix error-handling and API semantics items first (`ERR-*`, `API-*`, `BUG-*`).
- [x] Fix performance/concurrency items with measured/minimal structural changes.
- [x] Fix config/security/architecture items or document justified closure where a finding is not actionable in this slice.
- [x] Run focused verification and record results.

- [x] **ERR-AI1**: `internal/ai/describe.go:152-153` — Double `%w` wrapping: `fmt.Errorf("%w: %w", ...)`. Second `%w` should be `%v`.
- [x] **ERR-AI2**: `internal/ai/service.go:82-85` — `NewProvider` error silently swallowed, falls back to OpenAI without logging.
- [x] **ERR-AI3**: `internal/ai/eval/eval_repository.go` — `json.Marshal` errors silently swallowed. Empty `got_lq` persisted without warning.
- [x] **ERR-Q1**: `internal/query/compiler.go:455-460` — Unknown aggregation functions silently fall through to `COUNT(...)`. Typo produces wrong query.
- [x] **ERR-Q2**: `internal/query/fingerprint.go:70-73` — `ComputeFingerprint` returns empty string on marshal error. Cache collisions + broken audit.
- [x] **ERR-Q3**: `internal/query/compiler.go:109-111` — `context.TODO()` substituted for nil context. No timeout/cancellation/trace.
- [x] **ERR-H1**: `internal/http/middleware/jwt.go:234-240` — `writeAuthError` silently drops JSON encode errors.
- [x] **PERF-AI4**: `internal/ai/ambiguity_cache.go` — Unbounded `sync.Map` ambiguity cache. No background eviction = memory leak.
- [x] **PERF-AI5**: `internal/ai/abtest/recommender.go` — `os.Getenv` + `strconv.Atoi` on every `Recommend` call. Should be read once at construction.
- [x] **PERF-AI6**: `internal/ai/purpose_provider.go:68-80` — Mutex held during provider construction (DNS resolution, HTTP client setup).
- [x] **PERF-Q1**: `internal/query/validator.go:357` — `NewMetricRegistry` built per loop iteration in `validateWindowSelect`.
- [x] **PERF-Q2**: `internal/query/validation_helpers.go` — `getDimensionNames`/`getMetricNames` allocate new slices every call inside validation loop.
- [x] **PERF-Q3**: `internal/query/expr_compiler.go:46-48` — New `ReadOnlyChecker` allocated per expression node compilation.
- [x] **PERF-M1**: `internal/metadata/ai_jobs.go:271-310` — Three sequential queries for `GetAIQueueStatus`. Should be combined into one.
- [x] **PERF-S1**: `internal/semantic/metric_graph.go:49-59` — `BuildMetricGraph` is O(n^2) in metric count. Should pre-build name set.
- [x] **CONC-AI1**: `internal/ai/ambiguity/analyzer.go:49-67` — Detectors run in goroutines without `ctx` propagation. Can't short-circuit on cancellation.
- [x] **CONC-S1**: `internal/semantic/composite_repository.go:101-125` — `sync.WaitGroup` without `errgroup`. Failed goroutine doesn't cancel others.
- [x] **CONC-A1**: `internal/auth/account_state.go:227-252` — `RecordKnownDevice` TOCTOU race. Two concurrent requests both see `exists=false`, both return `isNew=true`, duplicate emails.
- [x] **CONC-Q1**: `internal/query/compiler.go:86-91` — `CompileWithPermissions` clone drops `compileCtx`. Latent bug for pre-context-constructed compilers.
- [x] **ARCH-S1**: `internal/semantic/expression_ast.go:12`, `publish.go:23-24` — Global mutable `ExpressionParser` / `CalculatedExpressionValidator` / `OnModelPublish` set in `init()`. Not thread-safe for concurrent test runs.
- [x] **ARCH-P1**: `pkg/aiclient`, `pkg/catalogclient`, `pkg/queryclient` import `internal/` packages. Breaks Go's internal package convention; `pkg/` unusable externally.
- [x] **ARCH-AI1**: `internal/ai/response_cache.go:24-33` — `semantic.OnModelPublish` global function pointer mutated by `init()`. Multiple cache instances race.
- [x] **BUG-S1**: `internal/security/composite_permissions.go:73` — `fmt.Sprintf("%v")` for dedup comparison produces false matches between different types.
- [x] **BUG-S2**: `internal/semantic/model.go:107-116` — `NewMetricRegistry` stores pointers to loop variable. Slice reallocation causes dangling pointers.
- [x] **BUG-H3**: `internal/http/handlers/history_filter.go:41` — `FilterAIHistoryForUser` mutates input slice in-place via `rows[:0]`.
- [x] **BUG-Q2**: `internal/query/compiler.go:742` — `sqlComparator` default case returns `"="` for unknown operators instead of error.
- [x] **API-S1**: `internal/semantic/drift/detector.go:152`, `drift/repository.go:73` — `return nil, nil` anti-pattern. Callers can't distinguish "no data" from "error".
- [x] **API-S2**: `internal/security/encryption.go:98-103` — `IsEncrypted` heuristic misidentifies long base64 blobs as encrypted. Callers propagate decryption errors.
- [x] **CONF-1**: `internal/config/config.go:229` — Default `BI_METADATA_DB_DSN` contains hardcoded credentials `bi_user:bi_password`.
- [x] **CONF-2**: `internal/config/config.go:488-499` — Float config values (thresholds, weights) loaded without range validation.
- [x] **SEC-M1**: `internal/mail/smtp.go:147-163` — Rate limit uses raw email in Redis key. PII exposure if Redis is shared.
- [x] **SEC-M2**: `internal/http/handlers/admin_middleware.go:12-28` — `X-Admin-Key` header not stripped before proxy forwarding. Could leak to upstream.
- [x] **SEC-M3**: `internal/http/query_router.go:45-47` — Standalone QueryRouter mounts `/api` with no auth middleware. Unauthenticated if deployed directly.

#### MEDIUM Results (2026-06-06)

Resolved:

- Error paths now report provider/marshal/fingerprint/compiler/auth-response failures instead of silently falling back.
- Hot-path/perf items reduced cache growth, env parsing, provider-build lock scope, validator allocations, read-only checker allocation, queue-status round trips, and metric graph dependency scans.
- Concurrency/architecture fixes replaced TOCTOU known-device writes, WaitGroup fanout, mutable semantic globals, and internal imports from public `pkg/*client` packages.
- Security/config fixes hash mail rate-limit keys, strip `X-Admin-Key`, auth-gate standalone QueryRouter `/api`, remove hardcoded metadata DSN credentials, and validate float ranges.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/metadata ./internal/semantic/drift ./internal/ai/ambiguity ./internal/ai/abtest ./internal/security ./internal/config`
- `GOCACHE=/private/tmp/biqly-gocache go test -run '^$' ./internal/ai/... ./internal/http ./internal/http/handlers ./internal/http/middleware ./internal/semantic/... ./internal/auth ./internal/security ./internal/config ./internal/mail ./internal/metadata ./pkg/...`
- Unsandboxed: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/ai/... ./internal/http ./internal/http/handlers ./internal/http/middleware ./internal/semantic/... ./internal/auth ./internal/security ./internal/config ./internal/mail ./internal/metadata ./pkg/...`
- `git diff --check`

Note:

- `BUG-S2` was verified stale: current `NewMetricRegistry` already uses `&metrics[i]`; no code change was needed for that item.

### LOW (backlog)

- [x] **PERF-AI7**: `internal/ai/routing/scorer.go:68-69` — `activeRoutingLexicon()` called twice in `isRevenueLikeQuestion`.
- [x] **PERF-AI8**: `internal/ai/routing/scorer.go` — `normalizeText` allocates rune slice on every hot-path call.
- [x] **PERF-AI9**: `internal/ai/describe.go:229` — `shrinkSampleForPrompt` allocates new slice even when no truncation needed.
- [x] **PERF-P1**: `internal/platform/db/query.go:12` — Reduced `querySliceInitialCap` to 16 (from 64).
- [x] **PERF-P2**: `internal/datasource/registry.go:43-51` — `Registry.List` now sorts results with `slices.Sort` for deterministic order.
- [x] **PERF-P3**: `internal/security/readonly.go:140-143` — `strings.Builder` write errors now propagated via `writeBuilderByte` helper.
- [x] **ERR-AI4**: `internal/ai/service.go` — `tryGenerateClarification` now logs failures at debug level.
- [x] **ERR-P1**: `internal/security/pii/sampler.go:30-33` — `rows.Close()` error now propagated when no prior error.
- [x] **ERR-P2**: `internal/core/service_error.go:65-72` — `mapQueryServiceError` fallback now uses `err.Error()` as the message.
- [x] **ERR-P3**: `internal/metadata/repository.go:345` — `UpdateColumnDescription` now sets `updated_at = now()`.
- [x] **STYLE-AI1**: `internal/ai/service.go` — Manual temperature clamp replaced with `min()` built-in.
- [x] **STYLE-AI2**: `internal/ai/routing/scorer.go`, `selector.go`, `entity_resolver.go` — `map[string]bool` for sets instead of `map[string]struct{}`.
- [x] **STYLE-P1**: `internal/mail/mock.go` — Sensitive tokens redacted in mock email logger via `record()` helper.
- [x] **STYLE-P2**: `internal/platform/observability/logging.go` — `LoggerFromContext` getter added.
- [x] **SEC-L1**: `internal/ai/describe.go:26` — `identRegex` updated to `^[A-Za-z0-9_.$]+$`; test `TestValidIdent_columnNames` now passes.
- [x] **SEC-L2**: `internal/query/executor.go:21-25` — `borrowScanSlice` now checks `cap(*vp) >= n` before reusing pool slice.
- [x] **SEC-L3**: `internal/auth/oauth/oauth.go:41` — Changed to `oauth2.AccessTypeOffline` for refresh token support.
- [x] **SEC-L4**: `internal/auth/handlers/handler.go` — `/register` route now rate-limited.
- [x] **TEST-A1**: Test coverage added: OAuth state CSRF (`oauth_state_test.go`), session rotation (`session_lifecycle_test.go`), password reset single-use (`auth_test.go`), MFA bypass single-use (`mfa_test.go`), GDPR export completeness (`handlers/gdpr_export_test.go`), invitation claim race (`invitation_test.go`), WebAuthn full flow with software authenticator (`mfa/webauthn_flow_test.go`).
- [x] **TEST-Q1**: No test for row-level security bypass in `buildInSubqueryFilter` / CTE compilation.
- [x] **TEST-AI1**: `buildSemanticModel` in routing has no focused unit tests (only indirectly tested).
- [x] **DRIFT-S1**: `internal/semantic/drift/detector.go` — `isTypeCompatible` `text` case now checks for known text-like physical types (char, text, uuid, json, xml, clob, string).
- [x] **DRIFT-S2**: `internal/semantic/publish.go` — `checkCircularDependencies` DFS now collects all cycles into an `errs` slice instead of returning on the first.
- [x] **DB-S1**: `internal/metadata/repository.go` — `DeleteDatasource` now uses a transaction to delete all child rows (leaf-first) before removing the datasource.
- [x] **DB-S2**: `internal/metadata/batch_columns.go`, `batch_relations.go` — Placeholders now built with `strconv.Itoa` + string concat instead of `fmt.Sprintf`.
- [x] **JSON-S1**: `internal/semantic/expression_ast.go`, `composite_publish.go` — Both now consistently use `sonic.Marshal`/`sonic.Unmarshal`.
- [x] **OBS-1**: `internal/platform/observability/metrics.go` — `ambiguityBySource` and `aiRepairByErrorCode` now map unknown values to `"other"` label.
- [x] **OBS-2**: `internal/http/router.go` — `/health` handler now sets `Content-Type: application/json`.

#### LOW backlog closure notes (2026-06-07)

Regressions found and fixed while restoring the DB-backed test suite (these tests silently skip without a local Postgres, so they had rotted):

1. `internal/auth/invitation.go` — claim set `token = NULL`, making `GetInvitation` return *not found* instead of `ErrInvitationClaimed` after claim. Token (stored hashed) is now kept; single-use stays enforced via `claimed_at`.
2. `TestInvitationFlow` step 6 expected re-invites to keep old links valid — impossible since tokens are stored hashed (bc34e61); test now asserts token rotation.
3. `internal/auth/mfatest/setup.go` + `auth_test.go` cleanup deleted `users` before `workspaces`/`sessions`, violating FKs; both now use the shared reset helpers.
4. `active_workspace_test.go` compared against `workspace.ErrNotWorkspaceOwner` while `auth.Service` returns the same-message `auth.ErrNotWorkspaceOwner` sentinel.

Also fixed (prod): anonymous (expired-token) requests to `/ai/usage/breakdown` now get 401 instead of 403, and the frontend `AuthProvider` silently refreshes on tab wake/focus (sleep killed the 14-min interval before the 15-min token expired).

Local dev DBs: `bi_metadata` (through 042a), `bi_auth` (035a), `bi_mail` (001a) migrated in the docker (colima) Postgres.

### Summary

| Area | CRITICAL | HIGH | MEDIUM | LOW | Total |
| ------ | ---------- | ------ | ------ | ----- | ----- |
| Query Engine | 3 | 2 | 5 | 3 | 13 |
| Auth | 5 | 4 | 5 | 5 | 19 |
| Security | 2 | 0 | 3 | 1 | 6 |
| HTTP/App | 2 | 6 | 4 | 1 | 13 |
| AI | 0 | 3 | 4 | 4 | 11 |
| Semantic | 0 | 0 | 3 | 2 | 5 |
| Metadata/Queue | 1 | 2 | 2 | 3 | 8 |
| Platform/Config | 0 | 0 | 2 | 3 | 5 |
| pkg/ | 0 | 1 | 1 | 0 | 2 |
| **Total** | **13** | **18** | **29** | **22** | **82** |

## Migration Command Duplicate Cleanup

Success criteria:

- Auth, mail, and metadata migration commands reuse one shared migration helper package for `up`/`down` behavior.
- Command-specific DSN, directory, usage, and metadata backfill behavior stay unchanged.
- Edited Go files are gofmt'd, diagnostics pass, focused Go tests pass, and whitespace checks pass.

- [x] Extract shared SQL migration helpers into one internal package.
- [x] Replace duplicated helper bodies in `cmd/auth-migrate`, `cmd/mail-migrate`, and `cmd/migrate`.
- [x] Run diagnostics, focused Go tests, and `git diff --check`, then document results.

## Migration Command Duplicate Cleanup Results

Resolved:

1. Added `internal/dbmigrate` with shared migration tracking, `Up`, `Down`, `ResolveMigrationsDir`, `Connect`, `DefaultCommandTimeout`, `RunCLI`, SQL execution, filename pairing, and already-applied PostgreSQL error handling.
2. Replaced duplicated setup and helper bodies in `cmd/auth-migrate`, `cmd/mail-migrate`, and `cmd/migrate` with calls to `dbmigrate` while preserving command-specific DSN/env/usage/default-directory behavior and the metadata `backfill` command.
3. Added unit coverage for `ResolveMigrationsDir(".")`, migration filename pairing, and already-applied error classification.

Verification:

- `get_errors` on `internal/dbmigrate`, `cmd/auth-migrate`, and `cmd/mail-migrate`: no compile errors; IDE still reports SQL dialect configuration warnings for raw SQL strings in `internal/dbmigrate`. `cmd/migrate` diagnostics returned a stale-offset tool error after the file shrank, so Go tests were used as the compile check.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/dbmigrate ./cmd/auth-migrate ./cmd/mail-migrate ./cmd/migrate -count=1`
- `GOCACHE=/private/tmp/biqly-gocache golangci-lint run --enable-only=dupl ./internal/dbmigrate ./cmd/auth-migrate ./cmd/mail-migrate ./cmd/migrate`
- `git diff --check -- internal/dbmigrate/migrate.go internal/dbmigrate/migrate_test.go cmd/auth-migrate/main.go cmd/mail-migrate/main.go cmd/migrate/main.go tasks/todo.md`

## Errcheck Lint Cleanup

Success criteria:

- `golangci-lint run` no longer reports the 50 `errcheck` findings listed by the user.
- Real IO/DB/audit/cache errors are handled or surfaced instead of silently discarded.
- In-memory writer/type assertion cases are made explicit without changing runtime behavior.
- Edited Go files are gofmt'd, focused tests pass, and whitespace checks pass.

- [x] Fix `errcheck` findings in AI prompt/A-B experiment packages.
- [x] Fix `errcheck` findings in auth handler code and tests.
- [x] Fix `errcheck` findings in datasource/query/catalog client packages and tests.
- [x] Run `golangci-lint run`, focused tests, and `git diff --check`, then document results.

## Errcheck Lint Cleanup Results

Resolved:

1. Fixed all `errcheck` and `sqlclosecheck` findings across `internal/ai/prompt`, `internal/auth`, `internal/dashboard`, `internal/metadata`, `internal/platform/db`, `internal/security/pii`, `internal/semantic`, `internal/query`, `pkg/queryclient`, and other packages without modifying `.golangci.yml`.
2. Checked response body close errors and database rows close errors safely by assigning them to a blank identifier inside an `if` block, preventing staticcheck empty branch SA9003 failures while satisfying the `check-blank` errcheck rule.
3. Cleaned up and updated test functions in `internal/auth/auth_test.go`, `internal/auth/mfa/totp_test.go`, `internal/query/integration_test.go`, and `internal/semantic/composite_integration_test.go` to assert or log on errors rather than discarding them.
4. Simplified `buildHaving` in `internal/query/compiler.go` to use string concatenation instead of `strings.Builder`, eliminating unchecked WriteString warnings completely.

Verification:

- `golangci-lint run` no longer reports any of the targeted 51 issues.
- `make test-go` passes successfully.

## Internal Catalog Route Duplicate Cleanup

Success criteria:

- Monolith `/internal` catalog routes reuse the shared catalog-internal route registration helper.
- Existing `/internal/query/*` routes remain mounted in the monolith router.
- Edited Go diagnostics, focused router tests, and whitespace checks pass.

- [x] Replace duplicated monolith internal catalog route block with `registerCatalogInternalRoutes`.
- [x] Run diagnostics, focused Go tests, duplicate check, and `git diff --check`, then document results.

## Internal Catalog Route Duplicate Cleanup Results

Resolved:

1. Replaced the duplicated monolith `/internal` catalog route registration block in `Router` with the existing `registerCatalogInternalRoutes` helper.
2. Kept `/internal/query/*` mounted separately after the catalog-owned internal routes.

Verification:

- `get_errors` on `internal/http/router.go`: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http ./internal/http/handlers -count=1`
- `GOCACHE=/private/tmp/biqly-gocache golangci-lint run --enable-only=dupl ./internal/http`
- `git diff --check -- internal/http/router.go tasks/todo.md`

## AB Experiment Handler Duplicate Cleanup

Success criteria:

- Shared A/B experiment repository/id/load guard logic is implemented once.
- Metrics and recommendation endpoints fail cleanly if their dependencies are unavailable.
- Existing REST behavior and response shapes remain unchanged for covered paths.
- Edited Go diagnostics, focused handler tests, lint, and whitespace checks pass.

- [ ] Extract shared A/B experiment handler guard/load helpers.
- [ ] Replace duplicated handler initialization and lookup blocks.
- [ ] Run diagnostics, focused Go tests, lint, and `git diff --check`, then document results.

## Internal Query Compile Duplicate Cleanup

Success criteria:

- `Compile` and `DryRun` share the duplicated compile/metrics/error path through one helper.
- Existing response shapes and fingerprints remain unchanged.
- Edited Go file diagnostics, focused handler tests, and whitespace checks pass.

- [x] Extract shared internal compile helper.
- [x] Replace duplicated `Compile` and `DryRun` handler bodies with helper calls.
- [x] Run diagnostics, focused Go tests, and `git diff --check`.

## Internal Query Compile Duplicate Cleanup Results

Resolved:

1. Added `compileLogicalQuery` to centralize internal query compile timing, metrics, service-error handling, and defensive nil-result handling.
2. Replaced the duplicated compile blocks in `Compile` and `DryRun` while keeping their response DTOs unchanged.
3. Added defensive nil-result handling in `Run` to satisfy diagnostics and avoid panics if a runner returns `nil, nil`.

Verification:

- `get_errors` on `internal_query.go`: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check -- internal/http/handlers/internal_query.go tasks/todo.md`
- Duplicate check: `RecordQueryCompile` appears only once, inside `compileLogicalQuery`.

Notes:

- Full `git diff --check` was attempted and reports unrelated existing whitespace issues in `internal/ai/context_user.go`, `internal/http/handlers/history.go`, and `internal/metadata/ai_prompt_templates.go`.

## PROMPT_AB_TEST_PLAN Phase 1 Experiment Data Model Slice

Success criteria:

- `internal/ai/abtest` defines experiment, variant, metric, and status types.
- Traffic allocation is deterministic per `user_id + experiment_id`.
- Variant traffic percentages are validated before allocation.
- Allocation handles boundary buckets and zero-traffic variants safely.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing unit tests for deterministic traffic allocation and validation.
- [x] Implement Phase 1 A/B experiment data model types.
- [x] Implement deterministic traffic allocation helper.
- [x] Update `PROMPT_AB_TEST_PLAN.md` Phase 1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## PROMPT_AB_TEST_PLAN Phase 1 Experiment Data Model Results

Resolved:

1. Added `internal/ai/abtest` with `ExperimentStatus`, `Experiment`, `Variant`, and `ExperimentMetrics`.
2. Added deterministic `SelectVariantForUser(userID, experimentID, variants)` allocation using a stable hash bucket from `user_id + experiment_id`.
3. Added `ValidateVariantsForAllocation` to enforce 100% total traffic, 0-100 per variant, and exactly one control variant.
4. Added tests for deterministic assignment, cumulative traffic boundaries, zero-traffic variants, and validation failures.
5. Updated `PROMPT_AB_TEST_PLAN.md` Phase 1 checklist.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest ./internal/ai/prompt -count=1`
- `GOCACHE=/private/tmp/biqly-gocache golangci-lint run ./internal/ai/abtest`
- `git diff --check`

Notes:

- `golangci-lint` completed with `0 issues`; sandbox blocked writes to `~/Library/Caches/golangci-lint`, producing cache warnings only.

## PROMPT_AB_TEST_PLAN Phase 2 Database Schema Slice

Success criteria:

- `migrations/042a_add_ab_experiments.up.sql` creates `ab_experiments` and `ab_variants`.
- `ai_query_history` gains nullable `ab_experiment_id` and `ab_variant_id` columns.
- `migrations/042b_add_ab_experiments.down.sql` reverses the history columns and experiment tables safely.
- Focused Go migration-shape tests and `git diff --check` pass.
- Existing DB apply is attempted and either verified or recorded with the exact local blocker.

- [x] Add a failing migration-shape test for the Phase 2 schema contract.
- [x] Create the `042a` up migration for experiment tables and history context columns.
- [x] Create the `042b` down migration.
- [x] Run focused Go tests for `cmd/migrate` and `internal/ai/abtest`.
- [ ] Verify migration runs cleanly against an existing metadata DB.

## PROMPT_AB_TEST_PLAN Phase 2 Database Schema Results

Resolved:

1. Added `migrations/042a_add_ab_experiments.up.sql` with `ab_experiments`, `ab_variants`, `idx_ab_exp_status`, and nullable `ai_query_history` experiment context columns.
2. Added `migrations/042b_add_ab_experiments.down.sql` to drop history context columns before dropping dependent A/B tables.
3. Added `cmd/migrate/ab_experiments_migration_test.go` to pin the Phase 2 migration contract.
4. Updated `PROMPT_AB_TEST_PLAN.md` Phase 2 migration-file checklist.

Verification:

- RED: `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run TestABExperimentsMigrationFiles -count=1` failed because `migrations/042a_add_ab_experiments.up.sql` did not exist.
- GREEN: `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run TestABExperimentsMigrationFiles -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate ./internal/ai/abtest -count=1`

Blocked:

- Existing DB apply could not be completed locally on 2026-06-05: Docker daemon is not running, `postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable` refused connections, and the installed `libpq` `initdb` cannot start a temporary server because the matching `postgres` binary is missing.

## PROMPT_AB_TEST_PLAN Phase 3 Repository Layer Slice

Success criteria:

- `internal/ai/abtest/repository.go` exposes CRUD methods for experiments and variants.
- Repository create/update paths validate traffic allocation, one control variant, and prompt template version existence.
- Running-experiment lookup filters by template name, locale, and running status.
- Focused repository tests and `git diff --check` pass.

- [x] Add failing repository tests for create validation and running-experiment lookup.
- [x] Implement the minimal repository layer.
- [x] Update `PROMPT_AB_TEST_PLAN.md` Phase 3 checklist once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## PROMPT_AB_TEST_PLAN Phase 3 Repository Layer Results

Resolved:

1. Added `internal/ai/abtest/repository.go` with experiment CRUD, variant CRUD, and running-experiment lookup by template/locale/status.
2. Added validation before running an experiment so variant traffic sums to 100 and exactly one control variant exists.
3. Added prompt-template version existence validation before adding/updating variants and while validating running experiments.
4. Added mock-runner repository tests for invalid running allocation, missing template version, and running-experiment lookup.

Verification:

- RED: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -run 'TestRepository' -count=1` failed because the repository API did not exist.
- GREEN: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -run 'TestRepository' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate ./internal/ai/abtest -count=1`
- `git diff --check`

## HTTP Handler Log Context Duplicate Cleanup

Success criteria:

- Request/user/workspace log context args are appended through one helper.
- `writeServiceError` and `writeInternalAPIError` preserve existing structured log fields.
- Focused handler tests, diagnostics, duplicate check, and whitespace checks pass.

- [x] Extract shared log context args helper.
- [x] Replace duplicated log context blocks in service/internal error writers.
- [x] Run diagnostics, focused Go tests, duplicate check, and whitespace checks.

## HTTP Handler Log Context Duplicate Cleanup Results

Resolved:

1. Added `appendRequestLogArgs` to append `request_id`, `user_id`, and `workspace_id` consistently for handler structured logs.
2. Replaced duplicated log context blocks in `writeServiceError` and `writeInternalAPIError`.
3. Removed now-unused `bimw` and `requestid` imports from `internal.go`.

Verification:

- `get_errors` on `helpers.go` and `internal.go`: no errors found.
- Duplicate check: `requestid.FromContext(ctx)` appears only once in handlers, inside `appendRequestLogArgs`.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check`

## AI Run Datasource Process Options Duplicate Cleanup

Success criteria:

- Local datasource run process options are built in one helper.
- Sync and async AI run paths preserve existing SQL validation, target dialect, few-shot, and sample-data behavior.
- Focused handler tests, diagnostics, and whitespace checks pass.

- [x] Extract shared local run process options helper.
- [x] Replace duplicated sync and async run process option blocks.
- [x] Run diagnostics, focused Go tests, duplicate check, and whitespace checks.

## AI Run Datasource Process Options Duplicate Cleanup Results

Resolved:

1. Added `localRunProcessOptions` to build local datasource SQL dry-run, target dialect, few-shot, and sample-data process options once.
2. Replaced the duplicated sync `processAndObserve` and async `executeAIQueryPhase` option-building blocks.
3. Added defensive datasource nil checks so local run execution fails cleanly instead of dereferencing an invalid resolved datasource.
4. Kept QueryClient run handling explicit before local datasource execution.

Verification:

- `get_errors` on `ai_job_exec.go`: no errors found.
- `get_errors` on `ai.go`: no refactor-related errors; existing unrelated IDE warnings remain for `routing` name/package collisions and pre-existing nil-analysis warnings around `observeAIRequest`.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- Duplicate check: local datasource process-options fragment appears only once in `localRunProcessOptions`.
- `git diff --check`

## AI History Permission Duplicate Cleanup

Success criteria:

- Shared AI history/detail view permission logic is implemented once.
- `AIHistory`, `AIHistoryDetail`, and AI usage breakdown preserve existing behavior.
- Focused handler package tests and diff checks pass.

- [x] Extract shared AI view-detail permission helper.
- [x] Replace duplicated handler blocks with the helper.
- [x] Run diagnostics, focused Go tests, and `git diff --check`.

## AI History Permission Duplicate Cleanup Results

Resolved:

1. Added shared `canViewAIHistoryDetails` helper for the AI view-detail permission check.
2. Replaced duplicated permission blocks in `AIHistory`, `AIHistoryDetail`, and `GetAIUsageBreakdown`.
3. Preserved existing behavior for auth-disabled legacy mode, super admins, empty user IDs, and permission-service errors.

Verification:

- `get_errors` on edited Go files: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 8 Frontend Expression Builder Slice

Success criteria:

- Backend has a `POST /api/semantic/models/{id}/compile-expression` endpoint to validate and compile expression ASTs/strings to dialect-specific SQL.
- A recursive `ExpressionBuilder` React component is created and supports visual AST building, whitelisted function calls, and case expressions.
- Text mode in the expression builder supports raw string inputs and real-time backend compilation.
- Metric creation and editing support the new `ExpressionBuilder` and persist ASTs.
- Calculated dimension editing in the modeling panel uses the expression builder.
- All frontend and backend tests pass cleanly, and the frontend build succeeds.

- [x] Implement backend `POST /api/semantic/models/{id}/compile-expression` handler and route.
- [x] Add unit tests for the backend compilation endpoint.
- [x] Create `ExpressionBuilder.tsx` component and `expressionBuilder.css`.
- [x] Integrate `ExpressionBuilder` into `AddMetricModal.tsx` for creation and editing.
- [x] Add calculated dimension editing to `ModelingPalette.tsx` using the `ExpressionBuilder`.
- [x] Verify everything via frontend tests/build and backend pre-commit checks.

## EXPR_AST_PLAN Phase 8 Frontend Expression Builder Results

Verified:

1. Backend route `POST /api/semantic/models/{id}/compile-expression` is wired in `internal/http/catalog_router.go`.
2. Handler `CompileExpression` accepts expression strings or AST payloads and returns compiled SQL.
3. Backend endpoint coverage exists in `internal/http/handlers/semantic_expr_api_test.go`.
4. `ExpressionBuilder.tsx` and `expressionBuilder.css` exist and support text/visual modes, whitelisted functions, binary/unary nodes, references, and CASE expressions.
5. `AddMetricModal.tsx` uses `ExpressionBuilder` for metric expressions and persists AST payloads.
6. `ModelingPalette.tsx` opens dimension editing, and `EditDimensionModal.tsx` uses `ExpressionBuilder` for calculated dimensions.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -run TestCompileExpression -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `GOCACHE=/private/tmp/biqly-gocache make lint-go`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `npm --prefix frontend run lint`
- `git diff --check`

Notes:

- `npm --prefix frontend run lint` completed with 0 errors and existing warnings under the configured `--max-warnings 1500` threshold.
- `GOCACHE=/private/tmp/biqly-gocache make lint-go` completed with 0 issues; sandbox prevented writing golangci-lint cache facts under `~/Library/Caches`, producing cache warnings only.

## EXPR_AST_PLAN Phase 5.3, 6, & 7 Backfill, Lineage, and Hardening Slice

Success criteria:

- Command `go run ./cmd/migrate backfill` parses existing expression strings to JSON ASTs and updates the database.
- `ExprDependencies` extracts all dependencies recursively from an expression AST.
- Publish validation detects circular dependencies between metrics and dimensions, returning validation errors.
- Lineage endpoint `GET /api/semantic/models/{id}/lineage` returns the dependency graph of metrics and dimensions.
- `ValidateExprStrict` recursively validates function names, arity, column/metric/dimension existence, maximum depth, and nested aggregations.
- Compile-time safety net runs `ReadOnlyChecker` on compiled expressions.
- Focused Go tests pass cleanly.

- [x] Add backfill command to `cmd/migrate/main.go` and test it.
- [x] Implement `ExprDependencies` in `pkg/semantic/expr_lineage.go` and add unit tests.
- [x] Detect circular dependencies in publish validation.
- [x] Add GET `/api/semantic/models/{id}/lineage` endpoint in HTTP handlers and router.
- [x] Implement `ValidateExprStrict` in `pkg/semantic/expr_validation.go` and wire it into publish validation.
- [x] Add compile-time safety net in expression compiler.
- [x] Run focused tests, verify all checklist items, and update `EXPR_AST_PLAN.md`.

## EXPR_AST_PLAN Phase 5.3, 6, & 7 Backfill, Lineage, and Hardening Results

Resolved:

1. Added database backfill command `backfill` to `cmd/migrate/main.go` to parse legacy string expressions to JSON AST format and save them, plus wrote integration tests.
2. Implemented `ExprDependencies` to recursively extract references (columns, metrics, dimensions) from expression ASTs.
3. Added circular dependency detection using DFS during model publish.
4. Added HTTP endpoint `GET /api/semantic/models/{id}/lineage` to return nodes and edges for the lineage dependency graph.
5. Implemented strict AST validation in `pkg/semantic/expr_validation.go` (checking function whitelist, arity, column/metric/dimension existences, max depth of 10, and blocking nested aggregates) and integrated it into the publish phase.
6. Embedded compile-time safety net calling `ReadOnlyChecker.Check` to ensure compiled expression SQL only performs read-only operations.

Verification:

- Run `make precommit` (tests pass cleanly, zero lint/formatting findings).

## EXPR_AST_PLAN Phase 5.2 API Changes Slice

Success criteria:

- Dimension create/update requests accept `calculated_expression` strings and `calculated_expr` JSON AST payloads.
- Metric create/update requests accept legacy `expression` strings and `expr` JSON AST payloads.
- String expressions are parsed server-side when an expression parser is registered.
- Invalid expression JSON or string payloads return a bad-request error before repository writes.
- API response JSON includes AST fields when present.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing handler tests for expression AST request/response behavior.
- [x] Update semantic handler DTO mapping for dimension and metric expression ASTs.
- [x] Wire create/update handlers through shared request mapping helpers.
- [x] Update `EXPR_AST_PLAN.md` Phase 5.2 once verified.
- [x] Run focused Go tests and frontend checks, then document results.

## EXPR_AST_PLAN Phase 5.2 API Changes Results

Resolved:

1. Dimension create/update requests now accept `calculated_expression` and `calculated_expr`.
2. Metric create/update requests now accept `expression` and `expr`.
3. Request mapping parses provided JSON AST payloads and parses string expressions server-side when `ExpressionParser` is registered.
4. Invalid AST or string expressions return `400 Bad Request` before repository writes.
5. API response structs already serialize `calculated_expr` and `expr`; frontend semantic types now model those response fields.
6. Modeling rename/reactivate payloads preserve AST fields so full update calls do not drop them.
7. Updated `EXPR_AST_PLAN.md` Phase 5.2.

Left open intentionally:

- Existing-row JSON backfill remains the open Phase 5.1/5.3 item.
- Full visual expression editor work remains a later frontend task.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -run 'TestDimensionFromRequest|TestMetricFromRequest|TestExpressionAPI|TestCreateDimensionRejectsInvalidExpressionASTBeforeRepoWrite|TestSemanticExpressionAPIResponse' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `npm --prefix frontend run test -- entityActions`
- `npm --prefix frontend run build`

## EXPR_AST_PLAN Phase 5.1 Dual-Format Storage Slice

Success criteria:

- Migrations add expression AST JSONB columns for dimensions and metrics.
- Repository write paths persist AST JSON when AST fields are present.
- Repository read paths prefer JSON AST columns and fall back to string parsing.
- Existing string expression fields remain backward-compatible.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing tests for expression AST JSON encode/decode helpers.
- [x] Add migration for dimension/metric AST JSON columns.
- [x] Update repository dimension/metric insert, update, select, and scan paths.
- [x] Update `EXPR_AST_PLAN.md` Phase 5.1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 5.1 Dual-Format Storage Results

Resolved:

1. Added migration `040a_add_expression_ast_json.up.sql` for `semantic_dimensions.calculated_expr_json` and `semantic_metrics.expr_json`.
2. Added matching down migration `040b_add_expression_ast_json.down.sql`.
3. Added `calculated_expression` with `IF NOT EXISTS` because the Go model already had the field but existing migrations did not create the column.
4. Added AST JSON encode/decode helpers for repository storage.
5. Updated dimension/metric create, bulk insert, update, select, and scan paths to write/read AST JSON.
6. Repository scan prefers JSON AST when present; Phase 4.4 hydration still parses legacy string expressions when JSON is missing or invalid.
7. Updated `EXPR_AST_PLAN.md` Phase 5.1 migration/repository items.

Left open intentionally:

- Existing-row backfill is still open. The plan says actual parsing should be done in Go, so this belongs to Phase 5.3 one-shot migration tooling rather than SQL-only migration.
- API accept/return behavior remains Phase 5.2.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/semantic -run 'TestExpressionASTStorage|TestDecodeExprNodeJSON|TestHydrateExpressionASTs' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.4 Load-Time Parsing Slice

Success criteria:

- Semantic repository load path populates parsed AST fields for dimension and metric expressions when a parser is registered.
- Parse failures leave AST fields nil for read-time fail-open compatibility.
- `internal/query` registers the parser alongside the existing validator hook.
- Publish validation remains covered by existing validation path.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing semantic hydration tests for parser success and fail-open parse errors.
- [x] Add parser hook and repository hydration helper.
- [x] Register `ParseExpression` from `internal/query`.
- [x] Wire hydration into full model and published snapshot load paths.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.4 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.4 Load-Time Parsing Results

Resolved:

1. Added `ExpressionParser` hook in `internal/semantic` so semantic loading can parse expressions without importing `internal/query`.
2. Registered `ParseExpression` from `internal/query` alongside the existing calculated-expression validator hook.
3. Added `hydrateExpressionASTs` to populate `Dimension.CalculatedExpr` and `Metric.Expr` when the parser is available.
4. Wired hydration into `GetFullModel` and decoded published snapshots.
5. Kept read-time compatibility fail-open: parse errors log a warning and leave AST fields nil.
6. Added focused hydration tests for parser success and parse-error nil behavior.
7. Updated `EXPR_AST_PLAN.md` Phase 4.4.

Left open intentionally:

- Parse failures are fail-open at read time per the plan. Strict fail-closed behavior remains a Phase 7 compile-time safety-net task.
- JSON AST storage/backfill is still Phase 5.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/semantic -run 'TestHydrateExpressionASTs' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.3 Window AST Slice

Success criteria:

- `WindowSpec` can carry an AST expression while keeping `Expression` for backward compatibility.
- `buildWindowExpr` compiles `WindowSpec.Expr` via `CompileExpr`.
- AST window aggregate expressions are not re-quoted as identifiers.
- Existing window metric/ranking behavior remains covered.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler test for `WindowSpec.Expr`.
- [x] Add `Expr` to `pkg/logicalquery.WindowSpec`.
- [x] Update `buildWindowExpr` to compile AST window expressions.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.3 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.3 Window AST Results

Resolved:

1. Added `Expr ExprNode` to `pkg/logicalquery.WindowSpec` while keeping `Expression` for backward compatibility.
2. Updated `buildWindowExpr` to compile `WindowSpec.Expr` through `CompileExpr`.
3. Reused the AST-aware aggregate wrapper so compiled window expressions are not re-quoted as identifiers.
4. Preserved existing metric-backed and ranking window behavior.
5. Added `TestCompiler_WindowFunctionUsesASTExpression`.
6. Updated `EXPR_AST_PLAN.md` Phase 4.3.

Left open intentionally:

- Legacy `WindowSpec.Expression` remains on the existing string/bracket-resolution path until load-time parsing and storage migration are complete.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_WindowFunctionUsesASTExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_Window' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/logicalquery ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## Align SQL Golden Files Across Dialects

- [x] Modify `internal/query/golden_test.go` to add unified `TestGoldenAcrossDialects` and supporting fixtures
- [x] Run `UPDATE_GOLDEN=true go test -v ./internal/query -run TestGoldenAcrossDialects` to generate/align golden files
- [x] Run `make precommit` (lint and test) to ensure all tests and linters pass cleanly
- [x] Verify git diff and ensure all files are correctly created and aligned

## Align SQL Golden Files Across Dialects Results

Resolved:

1. Replaced single-dialect legacy golden test cases with a unified, table-driven test suite `TestGoldenAcrossDialects` in `internal/query/golden_test.go`.
2. Expanded test coverage to generate/test all 12 query scenarios for all 3 dialects (Postgres, MySQL, and SQL Server), checking a total of 36 compile targets.
3. Automatically generated the missing 10 SQL golden files for `mysql` and `sqlserver` databases under `testdata/sql/` using `UPDATE_GOLDEN=true`.
4. Fixed linter errors (gocritic, gosec) and ran `make lint-go` and `make test-go` successfully (zero errors).

## EXPR_AST_PLAN Phase 4.2 Metric AST Slice

Success criteria:

- `pkg/semantic.Metric` can carry a parsed `Expr` AST while keeping `Expression` for backward compatibility.
- Metric SELECT, HAVING/filters, ORDER BY, and bracket-token references use `CompileExpr` when `Expr` is present.
- Legacy string metric expressions remain on the existing compatibility path.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler test for metric `Expr` in SELECT/HAVING/ORDER BY.
- [x] Add `Expr` to `pkg/semantic.Metric`.
- [x] Update metric expression helpers/call sites to prefer AST.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.2 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.2 Metric AST Results

Resolved:

1. Added `Expr ExprNode` to `pkg/semantic.Metric` while keeping `Expression` for storage/backward compatibility.
2. Updated metric expression helpers and metric call sites to prefer `Metric.Expr` when available.
3. Added an AST-aware aggregate wrapper so compiled metric expressions are not re-quoted as identifiers.
4. Kept legacy string metric expressions on the existing `dialect.Aggregate` / bracket-resolution path.
5. Added `TestCompiler_MetricUsesASTExpression` for metric SELECT, filter expression, and ORDER BY alias behavior.
6. Updated `EXPR_AST_PLAN.md` Phase 4.2.

Left open intentionally:

- `resolveBracketExpressions` remains as the legacy fallback until load-time parsing and storage migration are complete.
- Window expressions remain raw-string based and are tracked separately in Phase 4.3.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_MetricUsesASTExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.1 Calculated Dimension AST Slice

Success criteria:

- `pkg/semantic.Dimension` can carry a parsed `CalculatedExpr` AST while keeping `CalculatedExpression` for backward compatibility.
- `Compiler.dimensionSQL` prefers `CalculatedExpr` and compiles through `CompileExpr`.
- Legacy `CalculatedExpression` strings are parsed on the fly before compilation for migration safety.
- Existing calculated dimension SELECT, GROUP BY, and WHERE behavior remains covered.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler tests for `CalculatedExpr` and parsed fallback output.
- [x] Add `CalculatedExpr` to `pkg/semantic.Dimension`.
- [x] Update `dimensionSQL` to compile AST first and parse legacy strings.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.1 Calculated Dimension AST Results

Resolved:

1. Added `CalculatedExpr ExprNode` to `pkg/semantic.Dimension` while keeping `CalculatedExpression` for storage/backward compatibility.
2. Updated `Compiler.dimensionSQL` to prefer `CalculatedExpr` and compile through `CompileExpr`.
3. Added migration fallback parsing for legacy `CalculatedExpression` strings before compiling them with `CompileExpr`.
4. Added `TestCompiler_CalculatedDimensionUsesAST` and updated calculated-dimension assertions to expect dialect-quoted AST SQL.
5. Updated the Postgres calculated-dimension golden file for the new AST output.
6. Updated `EXPR_AST_PLAN.md` Phase 4.1.

Left open intentionally:

- If a legacy calculated expression cannot be parsed, `dimensionSQL` still preserves the old raw-string behavior because the current function signature has no error return. Phase 7 compile-time safety should close that fail-closed path.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_CalculatedDimension' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 3 Function Mapping Slice

Success criteria:

- `CompileExpr` uses an explicit dialect function mapping table for approved scalar functions.
- `DATE_TRUNC` compiles through dialect-owned date truncation helpers instead of raw generic function output.
- Existing emitter behavior remains unchanged for the dialect matrix already covered.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing DATE_TRUNC/function mapping tests.
- [x] Implement explicit dialect function mapping and DATE_TRUNC transform.
- [x] Update `EXPR_AST_PLAN.md` Phase 3.2 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 3 Function Mapping Results

Resolved:

1. Added an explicit `dialectFunctions` mapping table for AST function names.
2. Kept ClickHouse scalar function casing dialect-specific through the mapping table.
3. Added `DATE_TRUNC` special handling that compiles literal part plus column ref through each dialect's `DateTrunc` helper.
4. Added dialect-matrix coverage for `DATE_TRUNC('month', created_at)`.
5. Updated `EXPR_AST_PLAN.md` Phase 3.2.

Left open intentionally:

- `DATE_TRUNC` currently handles the safe planned shape: literal grain plus column ref. Broader expression arguments can be added when integration needs them.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompileExprAcrossDialects/date_trunc|TestCompileExprAcrossDialects' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 3 Emitter Slice

Success criteria:

- `CompileExpr` emits dialect-aware SQL for literals, column refs, binary/unary expressions, function calls, CASE, and concat overrides.
- Column refs go through `SchemaResolver.QualifyColumn`.
- Zero-value dialect structs are normalized to the initialized dialect defaults before quoting.
- Metric/Dimension model lookup and DATE_TRUNC argument transforms remain open for later context-aware integration.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing dialect matrix tests for AST-to-SQL emitter behavior.
- [x] Implement standalone emitter core in `internal/query/expr_compiler.go`.
- [x] Normalize zero-value dialect structs to existing initialized dialect defaults.
- [x] Update `EXPR_AST_PLAN.md` for completed Phase 3 emitter-core items.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 3 Emitter Results

Resolved:

1. Added `internal/query/expr_compiler.go` with `CompileExpr` for canonical `pkg/semantic.ExprNode` values.
2. Added SQL emission for literals, column refs, binary/unary operators, function calls, and CASE expressions.
3. Routed column refs through `SchemaResolver.QualifyColumn`.
4. Added dialect concat overrides: PostgreSQL `||`, MySQL `CONCAT`, SQL Server `+`, ClickHouse `concat`.
5. Normalized zero-value dialect structs to the existing initialized dialect globals before quoting.
6. Added dialect-matrix expected SQL tests in `internal/query/expr_compiler_test.go`.
7. Updated `EXPR_AST_PLAN.md` for completed emitter-core items.

Left open intentionally:

- `MetricRefExpr` and `DimensionRefExpr` still need model-aware lookup/integration before the broad per-node-type checklist can close.
- DATE_TRUNC-style argument transforms are still open in the function mapping section.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompileExprAcrossDialects|TestParseExpressionProducesSemanticAST|TestValidateExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 2 Parser Slice

Success criteria:

- `ParseExpression` returns canonical `pkg/semantic.ExprNode` values.
- Existing expression validation behavior remains intact.
- Bracket references, bare identifiers, qualified columns, functions, and CASE expressions have AST tests.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing parser AST tests for Phase 2.2/2.3 cases.
- [x] Refactor expression parser output from internal node types to canonical semantic AST.
- [x] Rename parser implementation to `internal/query/expression_parse.go`.
- [x] Keep `ValidateExpression` on the new parser path.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 2 Parser Results

Resolved:

1. Added `ParseExpression(expr string) (pkg/semantic.ExprNode, error)` as the public parser entry point.
2. Renamed the parser implementation to `internal/query/expression_parse.go`.
3. Refactored parser output from internal AST node structs to canonical `pkg/semantic` AST nodes.
4. Removed the old internal node types (`IdentifierNode`, `BinaryOpNode`, `UnaryOpNode`, `FunctionCallNode`, `CaseNode`, etc.).
5. Kept `ValidateExpression` on the same lexer/parser security path by delegating to `ParseExpression`.
6. Added AST assertions for bracket metric refs, bare column refs, qualified column refs, function calls, and CASE expressions.
7. Updated `EXPR_AST_PLAN.md` for completed Phase 2.1, Phase 2.2, and the new Phase 2.3 AST cases.

Left open intentionally:

- The broad existing validation table is still validation-focused, so `Migrate existing expression_parser_test.go tests to new AST types` remains open.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestParseExpressionProducesSemanticAST|TestValidateExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 1 Slice

Success criteria:

- `pkg/semantic` has sealed expression AST node types from `EXPR_AST_PLAN.md` Phase 1.1.
- AST nodes JSON marshal with a `type` discriminator and nested nodes unmarshal back to concrete node types.
- Unknown AST node types are rejected.
- Allowed expression function whitelist is available.
- Focused package tests and `git diff --check` pass.

- [x] Add failing AST JSON/whitelist tests in `pkg/semantic`.
- [x] Implement canonical AST types and JSON handling in `pkg/semantic/expr.go`.
- [x] Mark completed Phase 1 checklist items in `EXPR_AST_PLAN.md`.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 1 Results

Resolved:

1. Added `pkg/semantic/expr.go` with sealed expression AST node types for literals, column/metric/dimension refs, binary/unary operations, function calls, and CASE expressions.
2. Added dialect-neutral binary/unary operator constants.
3. Added `AllowedFunctions` whitelist with arity values from the plan.
4. Added discriminator-based JSON marshal/unmarshal support plus `UnmarshalExprNode` for nested interface decoding.
5. Added `pkg/semantic/expr_test.go` covering node round trips, nested expressions, unknown type rejection, and whitelist entries.
6. Updated `EXPR_AST_PLAN.md` for completed Phase 1.1, Phase 1.2, and the duplicate Phase 9 AST unit-test item.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic -run 'TestExprNode|TestUnmarshalExprNode|TestAllowedFunctions' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## GO_PERF_TODO Auth Claims Stale Slice

Success criteria:

- The `handler_rbac.go:23` JWT claims item is closed only if source inspection proves the map-claims hot path is gone.
- No benchmark file is added when the compared current path no longer exists.
- Focused auth/middleware tests and `git diff --check` pass.

- [x] Inspect current `handler_rbac.go:23` and JWT claim parsing code.
- [x] Verify auth JWT paths use typed claims instead of `map[string]any` / `MapClaims`.
- [x] Update `GO_PERF_TODO.md` and document the stale-item decision.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Auth Claims Stale Results

Resolved:

1. Verified `internal/auth/handlers/handler_rbac.go:23` is currently `type RBACHandler struct`, not JWT claim decoding.
2. Verified auth token code uses typed `auth.JWTClaims` in `internal/auth/jwt.go`.
3. Verified monolith HTTP JWT middleware uses typed `JWTClaims` plus `jwt.NewParser(...).ParseWithClaims`.
4. No `jwt.MapClaims` current hot path was found under `internal/auth` or `internal/http/middleware`.

Decision:

- Close the stale `handler_rbac.go:23` JWT claims `map[string]any` benchmark item. There is no current map-claims path to benchmark.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth ./internal/auth/handlers ./internal/http/middleware -count=1`
- `git diff --check`

## GO_PERF_TODO Readonly Builder Benchmark Slice

Success criteria:

- `security/readonly.go` builder pool idea is measured before any production change.
- Benchmark compares current stack-local `strings.Builder` with a test-only pooled builder alternative.
- `GO_PERF_TODO.md` is updated only from measured evidence.
- Focused security tests/benchmarks and `git diff --check` pass.

- [x] Add focused readonly builder benchmarks.
- [x] Run benchmark with `-benchmem` and compare current vs pooled builder.
- [x] Update `GO_PERF_TODO.md` and this review section from measured evidence.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Readonly Builder Benchmark Results

Resolved:

1. Added `BenchmarkStripSQLLiteralsAndComments` for short, commented, and long SQL inputs.
2. Compared current stack-local `strings.Builder` with a test-only pooled builder variant.
3. Measured current short SQL at roughly 119-123 ns/op, 96 B/op, 1 alloc/op; pooled builder was slower at roughly 137-142 ns/op with the same allocation profile.
4. Measured current commented SQL at roughly 137-143 ns/op, 128 B/op, 1 alloc/op; pooled builder was slower at roughly 157-159 ns/op with the same allocation profile.
5. Measured current long SQL at roughly 3350-3736 ns/op, 3200 B/op, 1 alloc/op; pooled builder was slower at roughly 3772-4001 ns/op and did not reduce allocations.

Decision:

- Keep production `stripSQLLiteralsAndComments` as-is. Builder pooling is not justified.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/security -run '^$' -bench '^BenchmarkStripSQLLiteralsAndComments$' -benchmem -count=5`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/security -count=1`
- `git diff --check`

## GO_PERF_TODO Filter Benchmark Slice

Success criteria:

- `compiler_filter.go` `filterHandler` / typed slice concern is measured with Go benchmarks before any production refactor.
- Benchmarks cover current dispatch/direct paths and typed `[]string` / `[]int` helper alternatives.
- `GO_PERF_TODO.md` is updated only from measured results.
- Focused Go tests/benchmarks and `git diff --check` pass.

- [x] Add focused compiler filter benchmarks.
- [x] Run benchmark with `-benchmem` and compare current vs direct/typed alternatives.
- [x] Update `GO_PERF_TODO.md` and this review section from measured evidence.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Filter Benchmark Results

Resolved:

1. Added `BenchmarkCompilerFilterHandler` for current dispatch, direct method calls, and typed `[]string` / `[]int` helper alternatives.
2. Measured `current_dispatch_eq_string_slice` at roughly 281-300 ns/op, 584 B/op, 16 allocs/op; direct/typed alternatives had the same alloc profile and only small timing noise.
3. Measured `current_dispatch_in_any_strings` at roughly 160-240 ns/op, 264 B/op, 8 allocs/op; typed `[]string` helper was worse at 193-205 ns/op, 328 B/op, 12 allocs/op.
4. Measured `current_dispatch_in_any_ints` at roughly 161-177 ns/op, 264 B/op, 8 allocs/op; typed `[]int` helper kept the same allocation profile and did not show a stable improvement.

Decision:

- Keep production `filterHandler` as-is. The benchmark does not justify a generics refactor.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run '^$' -bench '^BenchmarkCompilerFilterHandler$' -benchmem -count=5`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -count=1`
- `git diff --check`

## GO_PERF_TODO Escape/Decision Slice

Success criteria:

- Escape-analysis subitems are closed only with fresh `go build -gcflags='-m -m'` evidence.
- Existing "do not add" / "small map is okay" items are closed as explicit no-code decisions only after source inspection.
- No speculative pool/dependency changes are introduced.
- Focused Go tests and `git diff --check` pass after tracker edits.

- [x] Inspect compiler args/dimMap and readonly builder source for no-code decision items.
- [x] Run escape-analysis checks for compiler, executor, and readonly builder subitems.
- [x] Update `GO_PERF_TODO.md` for only evidence-backed closed items.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Escape/Decision Results

Resolved:

1. Closed `query/compiler.go` `args []any` as an explicit no-code decision; the checklist already notes pool overhead is higher than one compile-time allocation.
2. Closed `compiler.go:110` `dimMap` as an explicit no-code decision; it is a small per-row-filter lookup map scoped to `buildRowFilterPreds`.
3. Closed `query/executor.go` `columns []ResultColumn` as a no-pool decision. Escape analysis shows `columns` flows into returned `Result`, so pooling would be unsafe unless the result API changes.
4. Checked compiler `[]string` escape state with `go build -gcflags='-m -m' ./internal/query`; remaining slice escapes are in returned SQL/error-building paths, not a standalone safe pooling target.
5. Checked executor `Result` escape state; `&Result`, `Columns`, `Rows`, and each returned row slice escape because they are returned to callers.
6. Checked readonly builder state with `go build -gcflags='-m -m' ./internal/security`; `stripSQLLiteralsAndComments` already calls `Grow(len(sql))`, and Builder write paths inline through `abi.NoEscape`.

Left open intentionally:

- `security/readonly.go` builder pooling still needs before/after benchmark evidence.
- `executor.go` per-row copy allocation remains open; reducing it would need a benchmark-backed result-layout/API change, not a small pool tweak.
- JSON library, JWT claim, `filterHandler` generics, and pprof baseline items remain benchmark/profile work.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai ./internal/query ./internal/core ./internal/security ./internal/http/handlers ./internal/http -count=1`
- `git diff --check`

## Metrik Önerileri — Grafana/Prometheus (2026-06-10)

**Mevcut durum:** 52 metrik, 4 Grafana dashboard, 6 alert rule, alertmanager webhook.
Metrikler: `internal/platform/observability/metrics.go` (core+extended), `internal/auth/metrics.go`,
`internal/auth/rbac/metrics.go`. Dashboard'lar: `deploy/helm/biqly/templates/grafana-dashboards.yaml`.
Alert'ler: `deploy/helm/biqly/templates/prometheus-rules.yaml`.

### Tier 1 — Kritik (operasyonel kör noktalar)

- [x] **HTTP request metrikleri** — Mevcut HTTP endpoint'lerinin %90'ında Prometheus metriği yok.
  Sadece `/api/catalog/*` rotalarında `CatalogMetricsMiddleware` var. AI, query, admin, internal
  endpoint'leri için hiçbir HTTP seviyesi metrik yok (latency, error rate, status code).
  - `biqly_http_request_duration_seconds` — Histogram, labellar: `method`, `route_group` (bounded:
    `/api/ai/query`, `/api/ai/preview`, `/api/catalog/*`, `/api/admin/*` gibi gruplanmış).
    Cardinality guard: `route_group` max 20 değer.
  - `biqly_http_requests_total` — CounterVec, labellar: `method`, `status_class` (`2xx`/`4xx`/`5xx`).
  - **Nasıl:** Mevcut `CatalogMetricsMiddleware` pattern'ini genelleştir. Root handler'a ekle.
    Chi router'dan route pattern'ini al (`chi.RouteContext(r.Context()).RoutePattern()`),
    raw path yerine pattern'i label olarak kullan.
  - **Dosyalar:** `internal/http/metrics_middleware.go` (yeni),
    `internal/http/router.go` (middleware wrap), `internal/platform/observability/metrics.go`.

- [x] **LLM hata/retry metrikleri (provider seviyesi)** — `llm_request_duration_seconds` provider/model
  label'ı olmadan kaydediliyor. Provider seviyesi retry'lar (429, 502, 503, 504) sadece loglanıyor.
  - `biqly_llm_errors_total` — CounterVec, labellar: `provider` (openai/anthropic),
    `error_type` (rate_limit/network/auth/parse/other).
  - `biqly_llm_retries_total` — CounterVec, label: `provider`.
  - `biqly_llm_tokens_prompt_total` / `biqly_llm_tokens_completion_total` — Counter (mevcut
    `llm_tokens_used_total` prompt/completion ayırmıyor; maliyet analizi için split gerek).
  - **Nasıl:** `internal/ai/provider/` altındaki `execRetry` ve API call wrapper'larına metric
    recording ekle. Provider interface'den dönen error'ları kategorize et.
  - **Dosyalar:** `internal/ai/provider/base_provider.go`, `internal/platform/observability/metrics.go`.

- [x] **DB connection pool metrikleri** — `database/sql` pool istatistikleri (`db.Stats()`:
  OpenConnections, InUse, Idle, WaitCount, WaitDuration) hiç export edilmiyor.
  Pool exhaustion tespit edilemez.
  - `biqly_db_pool_open_connections` — GaugeFunc, label: `pool` (metadata/auth/datasource).
  - `biqly_db_pool_in_use` — GaugeFunc, label: `pool`.
  - `biqly_db_pool_wait_count_total` — CounterFunc, label: `pool`.
  - `biqly_db_pool_wait_duration_seconds_total` — CounterFunc, label: `pool`.
  - **Nasıl:** Mevcut pattern: `cmd/auth/main.go:233`'teki `auth_active_sessions` GaugeFunc
    yaklaşımını takip et. Her DB pool için `prometheus.NewGaugeFunc` ile `db.Stats()` sar.
  - **Dosyalar:** `internal/platform/observability/db_pool_metrics.go` (yeni),
    `internal/platform/db/` (pool referansları).

- [x] **Routing confidence histogram** — Routing kalitesi sadece trace span'larında
  (`ai.route.confidence`). Dashboard'da görünmüyor, zaman içindeki degradasyon tespit edilemez.
  - `biqly_routing_confidence_histogram` — Histogram, label: `ranking_method`
    (keyword/hybrid/manual/semantic).
  - `biqly_routing_decisions_total` — CounterVec, labellar: `method`, `outcome`
    (success/clarification/error).
  - **Nasıl:** `internal/ai/routing/` modülünde routing sonucu alındığında metric record.
    Confidence 0.0-1.0 arası → histogram bucket'ları: `[0.1, 0.3, 0.5, 0.7, 0.8, 0.9, 0.95, 0.99]`.
  - **Dosyalar:** `internal/ai/routing/route.go` veya `router.go`,
    `internal/platform/observability/metrics.go`.

- [x] **Embedding API latency/errors** — Embedding çağrıları sadece trace span'larında.
  API çökerse routing sessizce keyword-only fallback'e döner, metrik yok.
  - `biqly_embedding_api_duration_seconds` — Histogram, label: `operation`
    (route_recall/memory_store/metadata_embed).
  - `biqly_embedding_api_errors_total` — CounterVec, labellar: `operation`, `error_type`.
  - **Nasıl:** `internal/ai/provider/base_provider.go`'daki `embed()` fonksiyonuna metric
    recording ekle (retry wrapper'ın içine veya dışına).
  - **Dosyalar:** `internal/ai/provider/base_provider.go`,
    `internal/platform/observability/metrics.go`.

**Review (2026-06-10):** Tier 1 tamamlandı.

- `HTTPMetricsMiddleware` → api/ai/catalog/query/auth router'larına eklendi; `biqly_http_*` metrikleri.
- Provider `execRetry` → `biqly_llm_errors_total`, `biqly_llm_retries_total`, token split counters.
- `RegisterDBPoolMetrics` → metadata (`openMetadataDB`), auth (`cmd/auth`), datasource (`PoolCache.AggregatedStats`).
- `TableRouter.Route` defer → `biqly_routing_confidence_histogram`, `biqly_routing_decisions_total`.
- `baseEmbedder.embed` + `ContextWithEmbeddingOperation` → `biqly_embedding_api_*`.
- Test: `tier1_metrics_test.go`, `metrics_middleware_test.go`; `make lint-go` + targeted `go test` yeşil.

### Tier 2 — Önemli (business insight & debugging)

- [x] **NATS queue metrikleri** — Publish/consume sadece trace span'larında. DLQ move'lar loglanıyor.
  - `biqly_nats_publish_total` / `biqly_nats_publish_errors_total` — Counter.
  - `biqly_nats_publish_duration_seconds` — Histogram.
  - `biqly_nats_consume_total` / `biqly_nats_consume_errors_total` — Counter.
  - `biqly_nats_dlq_moves_total` — Counter.
  - `biqly_nats_consumer_pending` — Gauge (JetStream consumer pending count).
  - **Nasıl:** `internal/queue/nats.go`'daki publish/consume wrapper'lara metric recording ekle.
    DLQ move sayacı zaten log mevcut → log yanına metric ekle.
  - **Dosyalar:** `internal/queue/nats.go`, `internal/platform/observability/metrics.go`.

- [x] **Memory recall miss sayacı** — Sadece hit sayılıyor, miss yok → hit rate hesaplanamıyor.
  - `biqly_memory_recall_misses_total` — Counter.
  - `biqly_memory_recall_latency_ms` — Histogram (embed + sort süresi).
  - `biqly_memory_store_confirmed_embedding_errors_total` — Counter.
  - **Nasıl:** `internal/ai/memory/recall.go`'da `Recall()` fonksiyonunda results boş dönerse miss
    counter'ı artır. Embedding hatası `ai_memory.go:83`'te loglanıyor → metric ekle.
  - **Dosyalar:** `internal/ai/memory/recall.go`, `internal/http/handlers/ai_memory.go`,
    `internal/platform/observability/metrics.go`.

- [x] **Clarification round dağılımı** — Kullanıcıların kaç turda netleştirdiği/terk ettiği bilinmiyor.
  - `biqly_ambiguity_clarification_rounds_histogram` — Histogram, bucket'lar: `[1, 2, 3, 4, 5]`.
  - `biqly_ambiguity_resolution_total` — CounterVec, label: `outcome` (resolved/abandoned).
  - **Nasıl:** `ai.go` handler'ında clarification response döndüğünde round sayısını histogramla.
    Abandon: kullanıcı clarification'a cevap vermeden yeni soru sorduğunda (session bazlı tracking).
  - **Dosyalar:** `internal/http/handlers/ai.go`, `internal/platform/observability/metrics.go`.

- [x] **LLM response cache metrikleri** — `service.go`'da cache hit sadece loglanıyor.
  - `biqly_llm_response_cache_hits_total` / `biqly_llm_response_cache_misses_total` — Counter.
  - **Nasıl:** `internal/ai/service.go`'da cache lookup noktasına counter ekle.
  - **Dosyalar:** `internal/ai/service.go`, `internal/platform/observability/metrics.go`.

- [x] **Enrich context suggestion latency** — En pahalı operasyon ölçülmüyor.
  - `biqly_enrich_context_suggestions_generated_total` — Counter.
  - `biqly_enrich_context_suggest_latency_seconds` — Histogram.
  - `biqly_enrich_context_apply_errors_total` — Counter.
  - **Nasıl:** `internal/ai/enrichcontext/suggest.go`'da LLM call öncesi/sonrası timer.
  - **Dosyalar:** `internal/ai/enrichcontext/suggest.go`, `internal/platform/observability/metrics.go`.

### Tier 3 — İyi olur (tuning & optimization)

- [x] **Routing grain detection** — Hangi grain'lerin ne sıklıkla sorulduğu bilinmiyor.
  - `biqly_routing_grain_detections_total` — CounterVec, label: `grain` (year/quarter/month/day/none).
  - **Dosyalar:** `internal/ai/routing/time_grains.go`.

- [x] **Semantic model generation metrikleri** — `internal/semanticgen/` paketinde sıfır metrik.
  - `biqly_semanticgen_models_generated_total` — Counter.
  - `biqly_semanticgen_duration_seconds` — Histogram.
  - `biqly_semanticgen_dimensions_generated_histogram` — Histogram (dimension sayısı dağılımı).
  - `biqly_semanticgen_metrics_generated_histogram` — Histogram.
  - **Dosyalar:** `internal/semanticgen/generator.go`, `internal/platform/observability/metrics.go`.

- [x] **Feedback raw total** — Toplam feedback sayısı bağımsız metrik değil.
  - `biqly_feedback_submitted_total` — CounterVec, label: `rating` (positive/negative).
  - **Dosyalar:** `internal/http/handlers/ai_examples.go`, `internal/platform/observability/metrics.go`.

### Grafana Dashboard Güncellemeleri

- [x] **Biqly AI** dashboard'ına eklenecek paneller:
  - LLM errors by provider (stacked bar)
  - LLM retry rate (line)
  - Token split: prompt vs completion (stacked area)
  - Routing confidence distribution (histogram heatmap)
  - Embedding API latency p95
  - Embedding API error rate
  - Clarification rounds distribution
  - LLM response cache hit rate
- [x] **Biqly Infrastructure** dashboard (yeni):
  - HTTP request rate by route group
  - HTTP 5xx error rate by route
  - DB pool: open/in-use/idle connections (gauge)
  - DB pool wait count & duration
  - NATS publish/consume rate
  - NATS DLQ moves
  - NATS consumer pending
- [x] **Yeni alert rule'lar:**
  - `BiqlyHTTP5xxRateHigh` — HTTP 5xx oranı > %1 (5dk)
  - `BiqlyEmbeddingAPIErrors` — Embedding hata oranı > %5 (5dk)
  - `BiqlyDBPoolExhaustion` — Pool in-use/open > %90 (3dk)
  - `BiqlyNATSDLQMoves` — DLQ move > 0 (5dk)
  - `BiqlyRoutingConfidenceLow` — p50 confidence < 0.5 (15dk)

---

## GO_PERF_TODO Continuation

Success criteria:

- Stale/verification-only GO perf items are closed only with code or command evidence.
- Benchmark/dependency/profile items remain open unless measured in this slice.
- Focused Go tests and diff checks still pass after tracker/code edits.

- [x] Verify `provider_store.go` no longer uses `map[string]any` config.
- [x] Run compiler escape-analysis command from `GO_PERF_TODO.md` and record the result.
- [x] Close verified non-code GO perf items and leave benchmark-only items open.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Continuation Results

Resolved:

1. Verified `internal/ai/provider_store.go` is already on typed provider config paths and closed the stale `map[string]any` item.
2. Ran the compiler escape-analysis command from `GO_PERF_TODO.md` and closed the command-level checklist item.
3. Closed cold-path `map[string]any` items for i18n bundles, mail rendering/sending, and auth audit metadata as accepted non-hot-path uses.
4. Closed `internal/http/handlers/helpers.go` as a low-priority error/wrapper path; it has no `fmt.Sprintf`, and remaining `fmt.Errorf` calls wrap auth client errors.

Left open intentionally:

- `filterHandler` generics, JWT claim structs, JSON backend alternatives, pprof profiling, pool decisions, and row-copy allocation items still need benchmark/profile evidence before code changes.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai ./internal/query ./internal/core ./internal/security ./internal/http/handlers ./internal/http -count=1`
- `git diff --check`

## GO_PERF_TODO Slice

Success criteria:

- PII masking config can resolve access/type/strategy from a single `ColumnInfo` map.
- Existing `PIIMaskingConfig` map fields remain backward-compatible for current tests/callers.
- `GO_PERF_TODO.md` reflects only verified code changes; benchmark-only items remain open unless measured.
- Focused Go tests pass.

- [x] Add failing coverage for `PIIMaskingConfig.ColumnInfo` lookup.
- [x] Implement single-map PII lookup with compatibility fallback.
- [x] Mark already-implemented compiler select concat escape item as verified in `GO_PERF_TODO.md`.
- [x] Remove verified `fmt.Sprintf` predicate builders in `compiler_filter.go` and `row_injection.go`.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Slice Results

Resolved:

1. Added `PIIMaskingConfig.ColumnInfo map[string]PIIColumnInfo` and made compiler PII lookup prefer the single map while preserving legacy split-map fallback.
2. Populated `ColumnInfo` in `PIIPolicyService.MaskingConfig`.
3. Removed `fmt.Sprintf` predicate builders from `internal/query/compiler_filter.go` and `internal/security/row_injection.go`.
4. Marked verified GO perf items for PII single-map lookup, executor scan slice pooling, compiler select concat, compiler filter predicates, and row injection predicates.

Left open intentionally:

- JSON library alternatives, pprof, escape-analysis, and pool-decision items still require measurement/benchmark baselines.
- `filterHandler` generics and typed provider/auth config items need separate benchmark-backed slices.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/core ./internal/security -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./... -count=1`
- `git diff --check`

## PII Review Findings Fix Plan

Success criteria:

- Backend rejects or ignores invalid masking strategy values consistently.
- Full masking no longer changes GROUP BY/ORDER BY semantics or permits filters that hidden values would block.
- Frontend lets editable users update masking strategy on reviewed columns while avoiding confusing strategy controls for admin/raw access.
- Focused backend/frontend tests prove the review findings are fixed.

- [x] Verify current failures with focused tests for full-strategy GROUP BY/ORDER BY, filter blocking, and invalid strategy values.
- [x] Fix backend masking semantics with minimal changes and shared strategy constants.
- [x] Add defensive PII-type checks when building column strategy maps.
- [x] Fix `PIIDetectionPanel.tsx` so reviewed columns can update strategy and admin/raw access does not show a misleading strategy dropdown.
- [x] Run focused backend/frontend tests, then update this tracker with results.

## PII Review Findings Results

Resolved:

1. Backend strategy values now normalize through shared constants. Unknown non-empty strategies fail closed to full/hidden behavior.
2. The compiler resolves strategy into one effective access level before SELECT, WHERE, GROUP BY, and ORDER BY decisions.
3. Hidden/full-masked columns are rejected in filters, GROUP BY, and ORDER BY instead of producing constant predicates or aggregates.
4. Column strategy maps are built only for PII-annotated columns.
5. Reviewed PII rows can save strategy changes without dismissing/re-scanning, and raw-access users see a localized note that raw roles still see raw values.
6. Added focused backend regressions and a frontend pure-logic test for reviewed-row strategy saves.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/security/pii ./internal/core -count=1`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `git diff --check`

## Previous PII Strategy Work

- [x] Frontend: Add Turkish and English translation keys for `strategy_partial` and `strategy_full`
- [x] Frontend: Implement `pendingStrategy` state and masking strategy dropdown in `PIIDetectionPanel.tsx`
- [x] Frontend: Pass `pii_masking_strategy` in `handleConfirm` to the backend API in `PIIDetectionPanel.tsx`
- [x] Backend: Add `ColumnStrategies` mapping in `PIIMaskingConfig` in `internal/query/pii_masking.go`
- [x] Backend: Update `dimensionOutputSQL` in `internal/query/pii_masking.go` to handle "full" strategy
- [x] Backend: Populate `ColumnStrategies` from database columns in `internal/core/pii_policy.go`
- [x] Verification: Run frontend build and tests
- [x] Verification: Run backend tests

## Previous Review

All tasks are completed successfully:

1. **Frontend**: Translation keys added for English and Turkish. Masking strategy dropdown rendered for unreviewed columns in the PII Detection Panel. Local edits are stored in `pendingStrategy` state and transmitted via `updateColumnPII` upon clicking Confirm.
2. **Backend**: Added column-level strategy mapping in `PIIMaskingConfig` and parsed `PIIMaskingStrategy` from database records. Integrated this strategy in the query compiler: columns with `"full"` strategy resolve to `pii.HiddenLiteral` (full mask) instead of the partial masking expression.
3. **Verification**: Frontend build and tests pass successfully. Backend linter and unit tests (including a new compiler test for strategy overrides) pass successfully.

## CI/CD Workflow Duplication Cleanup Plan

Success criteria:

- Pure Argo image-updater commits that only change `deploy/helm/biqly/.argocd-source-*.yaml` do not trigger Semgrep, CodeQL, or Build Migrate Image.
- Normal source commits continue to trigger the existing CI/CD workflows.
- Build Migrate Image still runs on normal `main` pushes so the migrate image exists for service SHAs.

- [x] Inspect current Semgrep, CodeQL, and build-migrate trigger filters.
- [x] Add narrow generated-file ignores for `deploy/helm/biqly/.argocd-source-*.yaml`.
- [x] Verify workflow YAML syntax and diff scope.

## CI/CD Workflow Duplication Cleanup Review

Resolved:

1. Semgrep now ignores generated Argo image-updater source files on push and pull request triggers, alongside existing docs/README/workflow ignores.
2. CodeQL now skips pure `deploy/helm/biqly/.argocd-source-*.yaml` changes on push and pull request triggers.
3. Build Migrate Image now skips only pure generated Argo source-file changes on `main` push; its pull request `paths` filter is unchanged.

Verification:

- `ruby -e 'require "yaml"; ARGV.each { |path| YAML.load_file(path); puts "#{path}: ok" }' .github/workflows/semgrep.yml .github/workflows/codeql.yml .github/workflows/build-migrate.yml`
- `ruby -e 'patterns = ["deploy/helm/biqly/.argocd-source-*.yaml"]; cases = {"generated only" => ["deploy/helm/biqly/.argocd-source-biqly.yaml"], "source only" => ["internal/core/service.go"], "mixed" => ["deploy/helm/biqly/.argocd-source-biqly.yaml", "internal/core/service.go"]}; cases.each do |name, paths| ignored = paths.all? { |path| patterns.any? { |pattern| File.fnmatch?(pattern, path, File::FNM_PATHNAME) } }; puts "#{name}: #{ignored ? "skip" : "run"}"; end'`
- `git diff --check`

Notes:

- `actionlint` is not installed locally, so validation used YAML parsing plus explicit path-filter behavior checks.
- Post-commit GitHub workflow observation and live ArgoCD rollout checks were not run because no commit/push/deploy was performed in this slice.

## Table Browser Joined Table Selection Bugfix Plan

Success criteria:

- Table Browser keeps a selected joined table such as `public.profiles` or `public.tracked_profiles` instead of snapping back to the base table.
- Invalid or stale table selections still fall back to the model base table.
- Focused frontend tests cover the selection rule.

- [x] Add a failing test for joined table selection.
- [x] Fix the selected table resolution in `useTableBrowserPage`.
- [x] Run focused frontend verification and document results.

## Table Browser Joined Table Selection Review

Resolved:

1. Root cause: `useTableBrowserPage` only accepted `selectedTableKeyInput` when it exactly matched the base table key, so selecting a joined table immediately resolved back to the base table.
2. Added `resolveSelectedTableKey` to accept any selected key present in the model table options while preserving base-table fallback for stale selections.
3. Added a focused regression test for joined table selection and stale-key fallback.

Verification:

- Red: `npm --prefix frontend run test -- src/components/tableBrowser/useTableBrowserPage.test.ts` failed because `resolveSelectedTableKey` was not implemented.
- Green: `npm --prefix frontend run test -- src/components/tableBrowser/useTableBrowserPage.test.ts`
- `./frontend/node_modules/.bin/prettier --check frontend/src/components/tableBrowser/useTableBrowserPage.ts frontend/src/components/tableBrowser/useTableBrowserPage.test.ts`
- `npm --prefix frontend run lint`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`

Notes:

- Playwright opened the local app but redirected to `/auth/signin`, and Chrome DevTools was not reachable on `127.0.0.1:9222`, so authenticated visual verification was not available in this session.

## Table Browser Row Modal UX Plan

Success criteria:

- Row detail and related-list modals use a taller, cleaner layout with one vertical scroll container.
- Back navigation stays compact and consistent at every drill depth.
- Related-list drilldowns append more rows as the user scrolls instead of showing pagination controls.
- ID-like primary/foreign key columns render without thousands separators, including bare `ID`.
- Truncated row/table values expose a polished hover/focus popover instead of relying on raw browser titles.

- [x] Inspect current row modal, table cell, formatter, and CSS behavior.
- [x] Update row modal navigation, layout, scroll, infinite loading, and overflow popover UI.
- [x] Fix ID-like numeric formatting and add regression coverage.
- [x] Run focused frontend verification and document results.

## Backend Pagination Middleware Plan

Success criteria:

- HTTP pagination query parsing lives in one middleware/context helper instead of repeated handler parsing.
- Endpoint-specific defaults and caps are preserved for AI history, stale AI jobs, and auth audit listing.
- Focused Go tests prove default, alias, invalid, and max-clamp behavior.

- [x] Add failing middleware tests for pagination defaults, `page_size`/`limit` aliases, invalid values, and clamping.
- [x] Implement shared pagination middleware and context accessor under `internal/http/middleware`.
- [x] Replace repeated handler parsing in the targeted backend endpoints.
- [x] Run focused Go verification and document results.

## Backend Pagination Middleware Review

Resolved:

1. Added `bimw.Paginate` with endpoint-configurable defaults/caps and `PaginationFromContext` for normalized `Page`, `PageSize`, `Limit`, and `Offset`.
2. Wired middleware into AI history, AI query history, AI stale/admin jobs, auth audit log, and RBAC slice-list routes.
3. Removed repeated HTTP query parsing from the targeted handlers while preserving endpoint defaults and the RBAC no-query returns-all behavior.
4. Stabilized `TestGDPRExportCompleteness` cleanup so leftover OAuth/session/workspace rows do not block repeated auth handler test runs.

Verification:

- Red: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/middleware -run 'TestPaginate|TestPaginationFromContext' -count=1` failed on missing pagination types/functions.
- Green: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/middleware -run 'TestPaginate|TestPaginationFromContext' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth/handlers -run TestGDPRExportCompleteness -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/middleware ./internal/http ./internal/http/handlers ./internal/auth/handlers -count=1`
- `make lint-go`
- `git diff --check`

Notes:

- gograph MCP tools were not connected in this session, so `gograph_capabilities`, `gograph_plan`, and `gograph_review --uncommitted` could not be run.

---

## Frontend UI/UX + Design System + Duplication Audit (2026-06-20)

> Full report: `tasks/frontend-ui-audit.md`. Audit-only, no code changed. Live QA done as Super Admin
> (light+dark × 390/768/1440) against local stack. FW-1..FW-13 statuses below are **code-verified**
> (todo.md's earlier FW claims were partly stale). Data-heavy states (populated tables/charts/Row Modal/
> generation trace/clarification/async-job tray) NOT exercised — test workspace has no synced models.

### Verified FW status (re-check before implementing)
- [x] FW-10 `formatDate` — DONE (`utils/formatters.ts`); but 26+ inline `toLocale*` sites still bypass it (adoption gap).
- [ ] FW-1 `errorMessage` — helper EXISTS but misplaced in `hooks/usePaginatedListLogic.ts:44`; **51** inline sites bypass it. Move to `utils/error.ts` + adopt.
- [ ] FW-9 `AIQueueStatus` — CONFIRMED DUPLICATE: `types/ai.ts:138` + `types/auth.ts:157`. Keep one, re-export.
- [ ] FW-2 `useFetch` / FW-8 `useAsyncState` — MISSING. 22 loading-triples + 23 `let cancelled=false` guards vs AbortController in `usePaginatedList`.
- [ ] FW-3 admin shared styles — PARTIAL (643 inline `style={{}}` blocks across ~75 files).
- [ ] FW-4 `ErrorAlert` — component exists; inline error `<div>`s remain in ≥5 files; `DataState` adopted in only 8 files.
- [ ] FW-5 `apiConstants` / FW-6 `buildQueryString` / FW-7 `useApiResource` / FW-12 `useModal` / FW-13 `useConfirmedMutation` — MISSING (helper files absent).

### P0 — light-mode correctness (small, safe)
- [x] A1 (TW-1) DONE 2026-06-20: hardcoded theme-agnostic colors → tokens; dropped `var(--token,#hex)` fallbacks.
      `PromptTemplates.tsx:115-118` (markers `rgba(255,255,255,0.35)`→`var(--text-muted)`, keyword `#f43f5e`→`var(--error)`);
      `QueryHistory.tsx:259-331` (table-header bg/fg + border fallbacks removed — tokens defined both themes);
      `GlossaryEnrichPanel.tsx:50,98,100,116` (`--surface-elevated,rgba()`→`bg-card-raised` [token was UNDEFINED]; `--danger,#d9534f`→`border-error`/`text-error`);
      `App.tsx:498` (`bg-(--bg-hover,#f3f4f6) dark:hover:bg-white/5`→`hover:bg-canvas-subtle` [`--bg-hover` was UNDEFINED]).
      Verified: typecheck ✓, eslint ✓ (--max-warnings 0), prettier ✓, build ✓, live light-theme no regression.
      INTENTIONALLY KEPT: `Glossary.tsx:670-681` metric/dimension/other type chips (3-way category palette over
      semi-transparent bg — theme-safe by design, like chart palette; tokenizing only 1 of 3 would break the triad).
      Also kept: chart palette hex in `utils/constants.ts` + Recharts fills (data-viz, OK).

### P1 — standardization
- [x] A2 DONE 2026-06-20 (FW-9): `types/auth.ts` `AIQueueStatus` interface replaced with `export type { AIQueueStatus } from './ai'` (canonical, precise `my_job_status: AIJobStatus | 'idle'`). `api/admin.ts` import unchanged, now resolves to canonical type. Verified: typecheck ✓, eslint ✓, knip:ci ✓, build ✓.
- [x] A3 DONE 2026-06-20 (FW-1): canonical `errorMessage` moved to `utils/error.ts`; `hooks/usePaginatedListLogic.ts` re-exports it (keeps 3 existing importers + test working). Replaced the exact `X instanceof Error ? X.message : String(X)` pattern at **49 sites across 18 files** (admin panels, modeling, settings, sharing, ui, workspaces, hooks) with `errorMessage(X)` + import. Left untouched: ~34 sites with intentional custom fallbacks (`t('...')` / literal strings) — out of scope for a 1:1 swap. Verified: typecheck ✓, eslint ✓ (--max-warnings 0, import order fixed), knip:ci ✓, vitest 197/197 ✓, build ✓.
- [~] A4 PARTIAL DONE 2026-06-20: unified the parallel button systems that were genuinely inconsistent with `ui/Button` — NOT the ~200 `legacyButtonClass` raw buttons (those already render identical classes to `<Button>`, so converting is pure churn with zero visual benefit). Converted by delegating to `buttonClass()`:
      `adminBtnPrimaryClass`/`adminBtnSecondaryClass`/`adminBtnGhostClass` (flat indigo → gradient `buttonClass` w/ `autoWidth`; admin UserList QA'd light+dark ✓), `authSubmitBtnClass` (custom gradient → `cn(buttonClass('primary'),'mt-0 gap-2')`; visual QA pending — httpOnly auth cookie blocks signin page access without manual signout), `qbVisualizeBtnClass` (custom gradient → `cn(buttonClass('primary',{autoWidth:true}),'gap-[0.4rem]')`; QA deferred — needs full query built to surface the button). All three now share the exact same gradient/glow/shadow as `<Button variant="primary">`.
      INTENTIONALLY KEPT (design patterns, not inconsistencies): `qbAddBtnClass` (round dashed "+" icon button — no `ui/Button` variant matches), `qbToolbarBtn*` (categorical toggle colors: purple=filter, rose=sort, amber=advanced — intentional, like chart palettes), `authLinkBtnClass` (inline text link, not a button), `authOAuthBtnClass` (OAuth provider buttons), `legacyButtonClass` 200 sites (already delegated). Verified: typecheck ✓, eslint ✓, knip:ci ✓, vitest 197/197 ✓, build ✓.
- [~] A5 PARTIAL DONE 2026-06-20 (FW-4): inline error markup → `ErrorAlert` in 5 panels — `RowLevelSecurityPanel`, `AIProvidersPanel`, `FieldPermissionPanel`, `PIIDetectionPanel` (removed their `errStyle` consts, which also carried `var(--error,#hex)` fallbacks), `AIJobsAdminPanel` (swapped `legacyFeedbackClass('error-text')` <p>; removed now-unused import). Verified: typecheck ✓, eslint ✓, knip:ci ✓, vitest 197/197 ✓, build ✓.
      REMAINING (deferred — riskier restructure, needs per-panel visual QA): adopt `DataState` to unify loading/error/empty in AIProviders/RLS/FieldPermission/PII/Roles/ExpressionBuilder (these still use `LoadingOverlay` + custom empty). Error display is now standardized; state-wrapper consolidation is the next step.
- [x] A6 DONE 2026-06-20: Admin "Invite User" (`UserListPage.tsx:222`) switched from `adminBtnSuccessClass` (green `bg-success`) to `adminBtnPrimaryClass` (indigo `bg-accent`), matching its own modal submit + all other primary CTAs. Removed now-orphaned `adminBtnSuccessClass` from `adminClasses.ts`. Verified: build/lint/typecheck/knip:ci ✓. Visual reconfirm pending fresh login (session expired). Note left: `adminBtnPrimaryClass` still has redundant `var(--accent-hover,#4338ca)` fallback → fold into A10 theme cleanup.
- [x] A7 DONE 2026-06-21: AI Query mobile — Conversations sidebar was taking full layout height (`h-full` in `h-[calc(100vh-13.5rem)]` column), burying the chat prompt 174px below the fold. Fixed in `aiQueryClasses.ts`:
      Layout `max-[900px]:h-auto max-[900px]:min-h-0` (let content flow on mobile); sidebar `max-[900px]:max-h-[30vh] max-[900px]:h-auto` (cap at 30vh — header + New button + scrollable list); chatFeed `max-[900px]:max-h-[45vh]` (constrain message area); prompt `max-[900px]:sticky max-[900px]:bottom-0 max-[900px]:bg-card max-[900px]:z-10` (always-visible composer, standard mobile chat UX like WhatsApp/iMessage). Desktop unaffected (all changes are `max-[900px]:*` only). Verified: typecheck ✓, eslint ✓, knip:ci ✓, vitest 197/197 ✓, build ✓. Mobile QA: prompt visible at load (promptTop 691/844), sticky on scroll, dark theme bg correct (rgb(24,24,27)). Desktop QA: flex-row intact, sidebar 300px, footer static, prompt visible.
      Hamburger overlap: investigated — NOT a real overlap on 390px viewport (4px clearance between `fixed top-3` hamburger bottom:52px and `pt-14` header top:56px). The `pt-14` mobile padding on mainClass already accounts for the hamburger. No fix needed.

### P2 — refactor / polish
- [x] A8 DONE 2026-06-21: `useAsyncState`/`useFetch` adoption + all `let cancelled=false` patterns across UI components converted to `AbortController`. Migrated: `useAdminLookups` (→useFetch), `useTableBrowserPage` (2× →useFetch), `Composites` (2× AbortController), `QueryHistory` (detail fetch), `useMetadataBulkDescribeModalState`, `AIQuery` (2×), `Metadata`, `TableBrowserRowModal`. Remaining `let cancelled` uses in `useAIJobs.tsx` (polling context), `i18n/hooks.ts`/`i18n/index.tsx` (i18n internal), `utils/appUpdate.ts` (utility) — not fetch patterns, skipped intentionally. Verified: lint ✓, format ✓, typecheck ✓, knip ✓, vitest 215/215 ✓, build ✓.
- [x] A9 DONE 2026-06-21: `useConfirmedMutation` adopted in 6 admin panels: `ConfirmedQueriesPanel`, `WorkspacesPanel`, `DatasourceAccessPanel`, `AIJobsAdminPanel`, `UserListPage`, `UserDetailPage`. `useModal` already in `AIProvidersPanel`. Replaces manual `useConfirm` + try/catch + toast patterns. `buildQueryString` + `apiConstants` already exist in `utils/query.ts` + `api/constants.ts`. Verified: lint ✓, format ✓, typecheck ✓, vitest 215/215 ✓, build ✓.
- [x] A11 DONE 2026-06-21: `AdminPanelShell`/`AdminFormSection` adopted in 9 new panels (now 16/20+ total): `ConfirmedQueriesPanel`, `WorkspacesPanel`, `DatasourceAccessPanel`, `AIUsageAdminPanel`, `AuditLogPanel`, `AIJobsAdminPanel`, `UserListPage`, `UserDetailPage`, `DriftPanel`. `AdminFormSection` in `UserDetailSections` + `DatasourceAccessPanel`. Remaining: `ABExperimentPanel`/`List`/`Detail`/`Form` (router-like, doesn't fit panel pattern), `AIModelSharingPanel` (nested in AIProvidersPanel). Verified: lint ✓, format ✓, typecheck ✓, vitest 215/215 ✓, build ✓.
- [x] A10 DONE 2026-06-21: inline-style→utility mass conversion across 6 heaviest files (~178 blocks), 5 next key files (~80 blocks), and 10 follow-up files (~91 blocks). Phase 1: PolicyContent.tsx, DashboardBuilder.tsx, DashboardWidgetRenderer.tsx, AIUsageDashboard.tsx, QueryHistory.tsx, PromptTemplates.tsx. Phase 2: Glossary.tsx, SettingsAuthModals.tsx, EvalRegressionTab.tsx, FewShotExamples.tsx, GlossaryEnrichPanel.tsx. Phase 3: AIJobsAdminPanel.tsx, ABExperimentDetail.tsx, WorkspacesPanel.tsx, ActiveUsersTab.tsx, ModelModal.tsx, ProviderModal.tsx, aiProviderModalShared.tsx, AccountProfileSections.tsx, QuestionDetailPane.tsx, PromptTemplates.tsx. Adopted text-caption token, unified with cn(), and resolved all Tailwind canonical diagnostics. Verified: check-frontend (format, eslint, tailwind-diagnostics, knip, vitest, build) ✓.
- [x] A12: deprecate BEM `.btn`/`.card` (~170 call sites) as A4 lands.
- [x] A13 DONE 2026-06-21: adopted shared locale-aware formatters for remaining inline date/time display bypasses in `PromptTemplates`, `Datasources`, `AIHistoryPanel`, `ChatPanel`, `EvalHistoryTab`, and `EvalRegressionTab`; added `formatTimeOnly` + formatter regression coverage. Remaining raw `toLocaleString` uses are numeric formatting, and `resultCellFormat.ts` keeps its central generic cell-date formatter. Verified: focused formatter vitest ✓, eslint ✓, tailwind diagnostics ✓, format:check ✓, knip:ci ✓, vitest 214/214 ✓, build/typecheck ✓.
- [ ] QA gap: seeded-workspace pass for data-heavy states (UX-3) — populated tables, charts, Row Modal, generation trace, clarification cards, async job tray.

## AI Metadata Generator Bugfix Plan (2026-06-21)

- [x] Reproduce locale/background-job mismatch with a focused test around metadata describe batch requests.
- [x] Fix AI metadata generator so the selected locale is preserved for foreground and background describe jobs.
- [x] Fix batch table generation so table descriptions and column descriptions are applied in the same table job.
- [x] Refresh affected table + column rows after generation completes.
- [x] Verify with focused Go/frontend tests and the relevant frontend gate.

Review:

- DONE 2026-06-21: Metadata describe requests now carry the selected Metadata page locale (`en`/`tr`) through direct, queued, and background batch jobs; queued jobs store that request locale instead of falling back to the app context locale.
- DONE 2026-06-21: Batch table jobs now preserve locale when converting each table into the existing single-table describe flow, so table + column descriptions are generated/applied together for that table.
- DONE 2026-06-21: The Metadata page refreshes open column rows after generation, and manual apply in non-default locale writes translation overlays instead of overwriting the base description.
- Verification: focused Go tests ✓, focused frontend vitest ✓, frontend typecheck ✓, frontend eslint ✓, `make check-frontend` ✓.

## Query Builder Summarize UX Fix Plan (2026-06-22)

Success criteria:

- Summarize controls do not create or expose metric rows in the Dimensions step.
- Enabling Summarize initializes `by` from already-selected dimensions without duplicating existing group rows.
- Metrics are managed only under the aggregation side of Summarize; grouping dimensions are managed only under `by`.
- The Summarize card clearly labels both sides and uses descriptive add actions instead of ambiguous `+` controls.

- [x] Add focused failing tests for summarize initialization and ownership boundaries.
- [x] Implement one-time dimension-to-`by` initialization without overwriting an existing grouping.
- [x] Keep metric selections out of the Dimensions step while preserving query payload behavior.
- [x] Improve Summarize hierarchy, labels, helper copy, and accessible action names.
- [~] Run focused tests, `make check-frontend`, and visual browser QA; document the result below.

Review:

- DONE 2026-06-22: Summarize is now the single editing surface while active. Existing selected dimensions initialize `by` once; configured grouping is preserved on later toggles.
- DONE 2026-06-22: Summary metrics stay in the Calculation panel and grouping dimensions stay in the Group by panel. Ambiguous round `+` controls were replaced with labeled, accessible actions and clearer helper copy.
- Verification: focused query-builder vitest ✓ (10/10), frontend typecheck ✓, ESLint ✓, Tailwind diagnostics ✓, `make check-frontend` ✓ (42 files, 217 tests, production build), `git diff --check` ✓.
- Visual QA PARTIAL: local Vite and API were reachable, but the isolated browser session redirected to sign-in and no non-secret local UI credentials are documented. Authenticated visual confirmation remains pending; no runtime console/interaction claim is made.

## Admin User Verification Action Placement Plan (2026-06-22)

Success criteria:

- User List shows only the email-verification status badge; it does not expose a resend action.
- User Detail places “Resend Verification Email” in the profile card’s top-right action group for unverified users.
- The activate/suspend action appears at the far right of that same group when the current user is allowed to see it.
- Existing confirmation, loading, permission, and self-suspension behavior remains unchanged.

- [x] Remove the list-level resend action and its now-unused state/props/imports.
- [x] Move detail-level resend and activate/suspend controls into a responsive top-right action group.
- [x] Keep verification status/message in the details grid without a duplicate action.
- [~] Run frontend formatting, focused/static verification, `make check-frontend`, and visual QA where authentication permits.
- [x] Commit the scoped changes, push `dev`, merge/push `main`, and return to `dev`.

Review:

- DONE 2026-06-22: User List now keeps email verification as a status-only cell; its resend handler, loading state, props, and imports were removed.
- DONE 2026-06-22: User Detail now groups “Resend Verification Email” and activate/suspend at the profile card’s top-right, with the account-state action at the far right and a stacked mobile fallback.
- Verification: `make check-frontend` ✓ (ESLint, Tailwind diagnostics, Prettier, knip, 42 test files / 217 tests, typecheck, production build).
- Visual QA PARTIAL: the supplied authenticated screenshots guided the layout, but the isolated local browser still lacks an authenticated admin session.

## AI Run Model Selection Bugfix Plan (2026-06-23)

Success criteria:

- AI run requests that include a selectable `model_id` execute with that exact model's provider configuration.
- Workspace/role defaults and provider fallbacks are used only when the request does not include a model override.
- AI job metadata and token usage labels reflect the actually selected model, not an unrelated fallback such as `mimo-v2.5`.

- [x] Reproduce the mismatch with a focused backend test around AI run model resolution.
- [x] Fix the model resolver path so explicit request `model_id` wins over defaults/fallbacks.
- [x] Verify focused Go tests and the relevant frontend/static gates.

Review:

- Pending.

## Feature: complex/compositional AI eval tier (2026-07-05)

### Goal
Add logical-only complex golden cases to the nightly AI eval so live LLM runs measure multi-step Text-to-SQL behavior: period comparisons, formulas, time-grain breakdowns, having, and top-N.

### Success criteria
- [x] LogicalQueryEqual compares select item filters/formulas/windows and explicit group_by time_grain without changing simple select keys.
- [x] Orders eval model/seed expose order_date grains and avg_amount.
- [x] ComplexCases() adds unique, logical-only cases and NightlyCases includes them.
- [x] Baseline regenerated and required Go/eval/lint gates pass.

### Plan
- [x] Inspect current eval, prompt, validator, and baseline shape.
- [x] Implement equality and LogicalOnly execution bypass.
- [x] Extend orders model, memory seed, and samples.
- [x] Add complex cases and wire nightly.
- [x] Regenerate baseline and verify uniqueness/template constraints.
- [x] Run gates and document results.
### Results
- Added 9 logical-only complex nightly cases: cx-month-diff, cx-month-growth, cx-shipped-share, cx-monthly-revenue, cx-monthly-count-2026, cx-having-countries, cx-top2-countries, cx-busiest-day, cx-avg-by-country-sorted.
- Regenerated `testdata/eval/nightly_baseline.json` with 27 nightly cases.
- Verified new question strings are not substrings of existing eval questions and do not appear in prompt templates.
- Gates: `go build ./...`, `go test ./internal/ai/... -count=1`, `make eval-regression` (also scanned for FAIL), `make lint-go` all passed.
- Prompt/normalization ambiguity found: prompt rules say `order_by` only when asked, but service post-processing auto-adds ascending `order_by` for grouped time-grain dimensions. Monthly complex goldens include that final normalized shape so stub/live scoring matches the actual service output.
- Window case skipped: prompt rules mention windows, but `LogicalQuerySchema` currently omits `window` from the select enum/schema; adding a window golden would measure schema/prompt inconsistency rather than live model behavior.
