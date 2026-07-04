# Governed Context Layer — Gap Analysis & Roadmap

Comparison of a reference "governed context layer for BI + AI agents" against biqly's
current capabilities, with concrete build plans. Each item lists **what**, **current
state** (✅ have / 🟡 partial / ❌ missing) with file anchors, **where to build**,
**best practice**, and the **frontend UI/UX pattern** to use.

Legend for effort: **S** (days) · **M** (1–2 wks) · **L** (multi-week / architectural).

---

## 0. Where biqly already stands (so we don't rebuild)

Strong today: dialect-agnostic IR→SQL compiler across 8 dialects (`internal/query`,
`internal/dialect`); semantic layer with dimensions/metrics/joins/composites + publish
snapshots + rollback (`internal/semantic`, `pkg/semantic`); query-time governance —
row-level security (`internal/security/row_injection.go`), column/field permissions &
PII masking (`internal/security/pii`, `internal/query/pii_masking.go`), per-datasource
access (`internal/http/middleware/permission.go`, `internal/auth/rbac`); NL→SQL with
validation/retry/self-consistency/confidence/ambiguity (`internal/ai/service.go`),
versioned per-locale prompt templates + A/B + eval (`internal/ai/prompt`, `internal/ai/eval`);
audit log (`internal/audit`); async AI jobs over NATS (`internal/queue`); conversation
memory recall + embeddings + glossary + few-shot (`internal/ai`); drift detection
(`internal/semantic/drift`); self-hostable Helm/k8s + OTel→SigNoz (`deploy/`).

The gaps below are the delta.

---

## 1. Agentic multi-step reasoning — traceable & replayable runs

**What:** Surface the NL→answer flow as an explicit, ordered agent run (understand intent
→ plan → validate → execute → answer), where every step is inspectable and a past run can
be replayed.

**Current state:** 🟡 Partial. The pipeline already produces the signals — planning steps
(`internal/ai/prompt/prompts/*/prompt_layout.tmpl`), routing result, confidence,
candidates, retry count, validation/EXPLAIN result (`internal/ai/schema.go` `AIMetadata`),
and a routing visualization (`frontend/src/components/aiQuery/routingViz.tsx`). What's
missing is a **persisted, step-structured run trace** and a **replay** action. AI history
today stores request/result JSON, not a normalized step timeline.

**Where to build:**
- Backend: add an `ai_run_steps` table (or a `steps jsonb` column on the existing AI job /
  history rows in `internal/metadata/ai_jobs.go` / AI query history migration). Emit a step
  record from each phase in `internal/ai/service.go` (`generateWithRetries`,
  `tryMultiCandidate`, validator, `SQLValidator` dry-run) via a lightweight `RunRecorder`.
- API: `GET /api/ai/runs/{id}` returning ordered steps; `POST /api/ai/runs/{id}/replay`
  that re-executes with the same inputs (reuse `processAIQuestion`).
- Frontend: new `components/aiQuery/RunTrace.tsx`.

**Best practice:** Model steps as an append-only, typed event stream (kind, status,
input/output digest, duration, tokens). Never store raw PII in the trace — reference the
already-masked payloads. Make replay idempotent and clearly labeled (it spends tokens).

**UI/UX pattern:** Vertical **stepper/timeline** with per-step status icons (done/failed),
expandable to show the prompt slice, the produced LogicalQuery, and the validation verdict.
A "Replay" button on the run header. Collapse by default; progressive disclosure. Respect
`prefers-reduced-motion` for the reveal.

---

## 2. Skills library — reusable, saved agent capabilities

**What:** Let users save a validated question→query (and its chart/config) as a named,
reusable "skill" the assistant can recall and re-run, and that appears in a catalog.

**Current state:** 🟡 Partial. We have "Confirmed Queries" (`ai_confirmed_queries`
migration + `Confirmed Queries` admin surface) and few-shot examples — the raw materials.
There is no first-class, user-facing **Skills** catalog with invoke/param/versioning.

**Where to build:**
- Backend: promote confirmed queries into a `skills` concept — name, description, the
  LogicalQuery template, parameter slots, owner/workspace, version. New repo methods in
  `internal/metadata`, endpoints under `internal/http/ai_router.go` (`/ai/skills`), gated by
  `RequireDatasourceAccess` (skills are datasource-scoped — see the glossary/examples fix
  already applied this session).
- AI: at recall time, offer matching skills before generating fresh SQL
  (`internal/ai/service.go` routing/memory path).

**Best practice:** A skill is a saved LogicalQuery + parameter schema, not raw SQL — so RLS/
CLS/PII still apply at run time. Version skills; keep an author + "last verified" timestamp.

**UI/UX pattern:** A **card grid catalog** (reuse the pattern just built for AI Providers:
clickable cards, clear primary affordance, selected state) with search/tags; a "Save as
skill" action from a successful AI answer; parameter chips for slots (reuse the drag/click
parameter pattern from Prompt Templates).

---

## 3. First-class Memory

**What:** Durable, inspectable memory (facts, preferences, prior resolutions) the agent
reads before answering — visible and editable by users, not a black box.

**Current state:** 🟡 Partial. Conversation memory recall + embedding cache exist
(`internal/ai/memory`, `ai_query_history_memory_recall` migration) but memory is implicit
and not user-manageable.

**Where to build:** Add a workspace/user-scoped `memory_entries` store surfaced read/write;
inject into the prompt via the existing memory path. UI under AI settings.

**Best practice:** Scope memory per workspace + user; make every entry deletable (GDPR);
never persist secrets. Show provenance ("remembered from conversation on …").

**UI/UX pattern:** A simple **editable list** with inline add/remove, empty-state that
teaches ("Nothing remembered yet — the assistant will note durable facts here"). Match the
metadata/settings list styling.

---

## 4. Open, file-based, version-controlled semantic definition (git-native)

**What:** One portable definition for the semantic model (+ instructions/skills) that lives
as reviewable files — export/import, diff, branch, PR-review, roll back — instead of only a
database record.

**Current state:** 🟡 Partial. We have DB-side versioning + rollback (semantic publish
snapshots, `PublishModel`/`RollbackModel` in `internal/http/handlers/semantic.go`; prompt
template versions). We do **not** have a file/serialized open format, git integration, or a
PR-style review flow.

**Where to build:**
- Backend: a canonical serializer for the semantic model → stable YAML/JSON
  (`internal/semantic` — extend `model.go`), with import + validation (reuse
  `internal/semantic/publish_validation.go`). Endpoints: `GET /ai/semantic/models/{id}/export`,
  `POST /ai/semantic/models/import` (behind the model datasource-access guard added this
  session).
- Optional deeper: a git-sync worker that writes definitions to a configured repo on publish.
- Frontend: import/export buttons on the modeling page; a **diff view** between versions.

**Best practice:** Deterministic serialization (stable key order, no volatile timestamps in
the body) so diffs are clean. Validate on import before activating. Keep the DB as source of
truth; files are an export/interchange surface first, git-sync second.

**UI/UX pattern:** "Export / Import" split button on the model header; a **side-by-side diff**
(monospace, added/removed line highlighting) for version compare + rollback confirmation —
extend the existing version-history table (Prompt Templates) with a "Compare" action.

---

## 5. Unified data policy across UI / API / agent — prove it

**What:** The same RLS/CLS/PII/access policy is enforced regardless of caller (UI, API,
agent/MCP), with an audit trail provable to "who saw what, which query produced which value."

**Current state:** ✅ Mostly have, 🟡 prove-ability gap. Policy lives in the query
service/compiler (`internal/core/query_service.go`, `internal/security`) so it applies to
any caller that goes through it. Audit log exists (`internal/audit`). Gaps: (a) the audit
record doesn't tie a specific returned result-set/number to the exact compiled SQL + policy
decisions in one queryable place; (b) no per-request "policy decisions applied" summary.

**Where to build:** Enrich audit events at `internal/core/query_service.go` execution with:
caller identity + channel (ui/api/mcp), datasource, compiled SQL fingerprint, RLS predicates
applied, masked columns, row count. Add a `GET /api/audit/query/{id}` detail view.

**Best practice:** Record the *policy decision*, not just the action — which predicates/masks
were applied — so it's provable, not inferred. Keep the audit write on a non-cancelable
context (see the `context.WithoutCancel` fix applied to AI jobs this session).

**UI/UX pattern:** In the Audit Log admin panel, a **row-detail drawer**: identity, channel
badge (UI/API/MCP), the governed SQL, applied policies as chips (RLS ✓, masked: ssn/email),
and the result shape. Read-only, copyable.

---

## 6. MCP server — governed programmatic access (biggest new surface)

**What:** Expose biqly's governed query capability as an MCP server so external agents/clients
get the *same* policy-enforced access as the UI — "one policy whether UI, API, or MCP."

**Current state:** ❌ Missing. biqly consumes MCP tools (dev harness) but exposes none. No MCP
server, no agent-facing tool surface.

**Where to build:** New service `services/mcp/` (or a subrouter in `internal/http`) exposing MCP
tools: `list_models`, `run_question` (NL→governed result), `run_logical_query`, `list_skills`.
Every tool call routes through the **existing** `core.QueryService` + auth middleware so RLS/
CLS/PII/access all apply unchanged. Authn via the existing token/JWT layer; per-caller identity
threaded so audit (item 5) attributes MCP calls.

**Best practice:** Do not build a second query path — reuse `QueryService` so policy can't
diverge. Rate-limit + spend-cap MCP callers (reuse the per-workspace `SpendLimiter` added this
session). Default-deny; explicit tool allow-list.

**UI/UX pattern:** An admin **"Integrations / MCP" page**: connection URL + token issuance,
per-token scope (which datasources/skills), and a live "recent MCP calls" list linking into the
audit detail. Copy-to-clipboard connection snippet.

---

## 7. Knowledge Center

**What:** A single home for business context — glossary, metric definitions, semantic model
docs, verified answers — that both humans browse and the agent reads.

**Current state:** 🟡 Partial. Pieces exist (Glossary, semantic model, confirmed queries) but
they're scattered across separate admin surfaces; no unified, browsable knowledge home.

**Where to build:** Frontend-led: a `components/knowledge/` hub aggregating glossary terms,
metric/dimension definitions (from `internal/semantic`), and verified answers, with search. No
new backend model needed initially — compose existing endpoints.

**Best practice:** Make the knowledge the *same* source the prompt uses (glossary already feeds
the prompt via `{{.Glossary}}`) so "what the user reads" == "what the agent reasons over." No
divergent copy.

**UI/UX pattern:** A **searchable directory** with left-nav categories (Terms, Metrics,
Dimensions, Verified Answers) and detail panes; term pages show synonyms + which
models/columns they map to. Reuse card/table tokens already established.

---

## 8. Delivery integrations — Slack / Teams / scheduled reports

**What:** Self-service answers in Slack/Teams; scheduled "executive report" digests; always-on
dashboards.

**Current state:** 🟡 Partial. Dashboards exist (`internal/dashboard`, `Dashboard.tsx`) and
email delivery exists (`internal/mail`). No chat-platform bot; no scheduled report generator.

**Where to build:**
- Slack/Teams: a new `services/` bot (or worker consumer) that accepts a question, calls
  `QueryService` (policy-enforced, per-user identity mapped from the chat platform), returns a
  governed answer. Reuse the AI job queue (`internal/queue`) for async.
- Scheduled reports: a cron/worker (`internal/queue` + a schedule table) that runs saved
  skills/dashboards and emails a digest (`internal/mail`).

**Best practice:** Map the chat user to a biqly identity before answering — never a shared
service identity — so RLS/audit are per-person. Governed result only; no raw DB access from the
bot.

**UI/UX pattern:** Admin **"Integrations" cards** to connect Slack/Teams (OAuth), and a
**schedule builder** (recipients, cadence, which report) with a preview. Reuse the confirm-dialog
+ form patterns already in admin.

---

## 9. Deployment modes — cloud / private / air-gapped

**What:** Same product in multi-tenant cloud, private cloud, and air-gapped on-prem.

**Current state:** ✅ Mostly have. Self-hostable Helm/k8s; DB-managed AI providers allow
on-prem/local models; no hard external dependency for core query paths. Gaps: (a) document/verify
a fully offline profile (no external LLM required — local model path); (b) the PII-to-LLM egress
control (fixed this session via `ExcludePIIColumns`) should be paired with a global "no external
egress" switch for air-gapped installs.

**Where to build:** A config profile (`internal/config`) `BI_DEPLOYMENT_MODE=airgapped` that
fails closed on any external-egress path (LLM provider must be in-cluster); document in `deploy/`.

**Best practice:** Air-gapped = default-deny egress, enforced, not documented-only. Tie into the
NetworkPolicy work already identified (see `tasks/hardening-recommendations.md` #2).

**UI/UX pattern:** Not user-facing; a Platform Settings read-only "Deployment mode" indicator so
admins can confirm the posture.

---

## 10. Chat / Agent experience — concrete UI/UX spec

Detailed front-end direction for the ask→reason→answer flow, benchmarked against a
best-in-class agentic BI chat. biqly already has a capable chat
(`frontend/src/components/aiQuery/`: `ChatPanel.tsx`, `AssistantMessageCard.tsx`,
`generationTrace.tsx`, `RoutingPanel`/`routingViz.tsx`, `clarificationStage.ts`,
`SampleDataModal.tsx`, `PromptTextarea.tsx`) — the items below are the polish/delta.

### 10.1 Message layout — user right, assistant left
- **Now:** 🟡 `ChatPanel.tsx:238` branches on `message.role === 'user'` but user turns are not
  clearly right-aligned with an avatar.
- **Build:** Right-align the user bubble (`ml-auto`, max-width ~72ch, rounded, subtle surface)
  with the user avatar on the right; assistant turns stay left with the app/agent glyph.
- **Best practice:** One column, alternating alignment; timestamps on hover; never full-bleed
  user text — a contained bubble reads as "my input."
- **UI/UX pattern:** Classic chat transcript. Avatar + name label above the first bubble in a
  run; wrap long content, don't truncate.

### 10.2 Thinking steps — collapsible agentic timeline
- **Now:** 🟡 `generationTrace.tsx` + `routingViz.tsx` `Collapsible` render an NL→SQL trace
  (route confidence, chosen columns). There is no **multi-step tool timeline** (understand →
  locate refs → read files → run command → chart → done).
- **Build:** A "Show thinking steps" collapsible above the answer, rendering the persisted run
  steps (see item 1). Each step: status dot (done/failed), a tool/label badge ("Executed tool
  …", "Read N files", "Ran a command"), and an expander. Expanded substeps:
  - **Read files:** a list of small file-cards (name + chevron) that open the file/definition.
  - **Ran a command:** the compiled SQL in a monospace block + a **"View data"** action that
    opens a right-side drawer with **Data Preview / SQL Query** tabs (reuse/extend
    `SampleDataModal.tsx` as a side panel).
  - **Tool step:** show `input`/`result` payloads in collapsed code blocks.
- **Best practice:** Collapsed by default once the answer is ready; auto-expand only while the
  run is streaming. Stream steps in as they complete. Every step maps to a real recorded event
  (no cosmetic fake steps). Respect `prefers-reduced-motion`.
- **UI/UX pattern:** Vertical connected timeline (dotted rail + node icons); nested cards for
  substeps; a persistent "View data" side drawer with tabbed preview/SQL.

### 10.3 Answer as rich narrative, not just a card
- **Now:** 🟡 `AssistantMessageCard.tsx` / `assistantMessageCardSections.tsx` render SQL +
  result table + chart + feedback. Less **narrative prose** (headline number, assumptions,
  follow-up suggestions).
- **Build:** Render the assistant answer as **markdown**: a bolded headline number/sentence, a
  short "Note:" line for assumptions (timezone, excluded statuses — e.g. `canceled`/`unavailable`),
  the data **table** and/or **chart with a one-line caption + interpretation**, then a bulleted
  **"You can go further:"** list of suggested follow-ups (each clickable to ask it).
- **Best practice:** Always state assumptions the query made (timezone, filters applied,
  definition chosen) — these come from the governed compile, so surface them from
  `internal/query`/`internal/core`. Follow-ups should be real, runnable questions.
- **UI/UX pattern:** Markdown renderer with a constrained prose width; number/units emphasized;
  chart caption in muted text below the figure; follow-ups as chips or a bullet list with an
  "ask this" affordance.

### 10.4 Interactive clarification — "Ask a question back"
- **Now:** 🟡 `clarificationStage.ts` + backend interactive Tier-3 (`ai_context.go`) exist, but
  the UI isn't the rich picker the reference shows.
- **Build:** When the agent needs a decision, render an inline **"Let's clarify some details"**
  card: a short rationale, a **recommended default** (labeled), **numbered options** (1/2/3) as
  selectable rows, a free-text **"Add additional details"** field, and **Skip / Send** buttons.
  "Skip" proceeds with the recommended default and the answer states which definition it used.
- **Best practice:** Always offer a recommended default so the user is never blocked; echo the
  chosen definition back in the final answer ("Used definition: group-level ratios, full history").
  Keep it one decision at a time.
- **UI/UX pattern:** Inline card (not a blocking modal) within the transcript; numbered
  radio-like rows; primary "Send" disabled until a choice or free-text; "Skip" secondary.

### 10.5 Home empty state — data-aware suggested questions
- **Now:** 🟡 `ChatPanel.tsx` shows a "Try asking…" empty state but from **static** i18n strings
  (`ai_query.suggestion_1..4`).
- **Build:** **Auto-generate** starter questions from the active semantic model — pick a few
  dimensions/metrics/joins and compose typed prompts (segmentation, comparison, trend). Backend:
  a `GET /api/ai/suggested-questions?datasource_id=…` that inspects `internal/semantic` (published
  model dimensions/metrics) and returns categorized suggestions; cache per model version.
- **Best practice:** Derive from the *published* model so suggestions never reference fields the
  user can't query; tag each by type (Segmentation / Comparative / Trend); refresh when the model
  changes. Keep them short with a category eyebrow.
- **UI/UX pattern:** Centered empty state with an icon + "Know more about your data", then 3–4
  **suggestion cards** (category chip + question text, clickable to run). Reuse the card grid +
  clickable-card affordance established for AI Providers.

### 10.6 Artifacts library — saved charts / answers
- **Now:** ❌ No artifacts library. Saved outputs live only inside a thread.
- **Build:** Let a chart/answer be saved as an **artifact** (name, thread origin, created_at),
  browsable in an **Artifacts** view with date filter + search. Ties to Skills (item 2) and
  Dashboards (`internal/dashboard`).
- **Best practice:** An artifact stores the LogicalQuery + view config, not a static snapshot, so
  re-opening re-runs under current policy (RLS/CLS/PII still apply).
- **UI/UX pattern:** Grid/list with thumbnails, date-range filter, search; empty state "No
  artifacts yet — create one inside a thread."

### 10.7 Settings surfaces — MCP / Skills / Memories
- **Now:** ❌ Missing dedicated surfaces (see items 2, 3, 6).
- **Build:** Add left-nav settings sections: **MCP connection** (connect URL + token, per-token
  scope), **Skills** (catalog + editor), **Memories** (editable list). Group under an "Agentic"
  settings heading.
- **UI/UX pattern:** Standard settings list-nav + detail pane (match the existing admin panel
  shell). "New" badges on newly shipped sections.

> Cross-cutting for §10: all answers, previews, and "View data" must go through
> `core.QueryService` so RLS/CLS/PII/access and the per-workspace spend cap apply — the chat is
> just another governed caller, never a second query path.

## Suggested sequencing

1. **Run trace + replay (item 1)** — M. Highest UX value, builds on data we already emit.
2. **Audit prove-ability (item 5)** — M. Governance credibility; small, high-trust.
3. **MCP server (item 6)** — L. Strategic surface; reuses QueryService so policy is free.
4. **Skills library (item 2)** — M. Promotes existing confirmed-queries.
5. **File export + diff/rollback (item 4)** — M, then git-sync as L follow-up.
6. **Knowledge Center (item 7)** — M, mostly frontend composition.
7. **Slack/Teams + scheduled reports (item 8)** — L.
8. **Memory management (item 3)** and **air-gapped profile (item 9)** — S–M, opportunistic.

Cross-cutting: everything routes through `core.QueryService` so RLS/CLS/PII/access and the
per-workspace spend cap apply to every new channel by construction — do not add parallel query
paths.
