# Biqly — Domain Context

Biqly is a **natural-language-to-SQL BI platform**. Users ask questions in plain language (often Turkish); the system routes to the right tables, resolves ambiguity, produces a **LogicalQuery** against a **SemanticModel**, compiles parameterized SQL, and executes it read-only against customer **datasources**.

Agent skills: use the terms below consistently in issues, refactors, tests, and ADRs. Coding and workflow conventions live in `AGENTS.md`, not here.

## Product boundary

| Layer | Role |
| --- | --- |
| **Metadata DB** | PostgreSQL (`bi_metadata`) — datasources, synced schema, semantic models, AI history, permissions, eval runs |
| **Customer datasource** | User-connected DB (PostgreSQL, MySQL, SQL Server, ClickHouse) — queried read-only at runtime |
| **Semantic layer** | Business-facing dimensions, metrics, joins on top of synced metadata — not raw tables in prompts |
| **AI layer** | NL → LogicalQuery; table routing; ambiguity/clarification; few-shot and glossary context |
| **Query layer** | LogicalQuery validation, join planning, dialect-aware SQL compile, safe execution |
| **Agentic query runner** | `cmd/agent` — bounded, policy-gated planner/tool loop; standalone service, `agent.enabled=false` by default (prod runs it in shadow mode), no public route |
| **Web agent mode** | `POST /api/agent/chat` (monolith/`ai` chart, `BI_WEB_AGENT_ENABLED=false` by default) — the same `internal/agent/` policy/runtime loop as the agentic query runner, but streamed over SSE in-request instead of via NATS job envelope; longer per-run budget (≤120s vs ≤45s), no standalone service |
| **MCP gateway** | `services/mcp` — stateless MCP server (streamable HTTP at `/mcp`) exposing governed tools (`run_question`, `run_logical_query`, `list_datasources`, `list_models`, `list_skills`, `run_skill`); every tool call proxies through the API gateway with the caller's credentials, so the same auth/RLS/PII/audit chain applies (channel=`mcp`). Owns no DB, no query logic. The same six tool definitions and dispatch path are shared with web agent mode via `internal/toolcontract` (channel=`agent`) — "one contract, two consumers." |
| **Frontend** | React 19 + Vite — chat, modeling UI, charts (Recharts), i18n via `useT()` |

Production deploys `catalog`, `query`, `ai`, `worker`, `auth`, `frontend`, `mail`, `mcp`, and the internal `agent` (shadow mode) into Kubernetes namespace `biqly`; the biqly system databases now live on the shared `prag-postgresql` instance in namespace `postgresql`.

## Glossary

| Term | Meaning | Avoid calling it |
| --- | --- | --- |
| **LogicalQuery** | Structured query intent (select, filters, group_by, order_by, limit) — not raw SQL | "query JSON", "AI output" when you mean the struct |
| **SemanticModel** | Published (or draft) model: base table, dimensions, metrics, joins, synonyms | "schema", "table" when you mean the business model |
| **Dimension** | Groupable/filterable field (`column_ref` or calculated expression) | "column" without semantic context |
| **Metric** | Aggregated measure (`expression` + `aggregation`: sum, count, avg, …) | "measure" interchangeably in code/docs |
| **Join** | Declared relationship between tables in a model (`join_type`, `relationship` cardinality) | Ad-hoc JOIN in SQL without model join |
| **Datasource** | Registered connection to a customer database (encrypted DSN) | "database" when you mean the metadata record |
| **Table routing** | Pre-LLM step picking relevant tables/model (`TableRouter`, `NeedsClarification`) | "model selection" alone |
| **Clarification** | User picks among ambiguous interpretations (`clarification_choice` wire field) | "disambiguation" in user-facing copy unless technical |
| **Ambiguity check** | Pre-LLM deterministic (+ optional LLM) pass for synonym/homonym collisions | Re-running after user already resolved a choice |
| **ProcessContext** | Single sync/async path for clarification state (`ClarificationResolved`) | Setting flags on `aiQueryRequest` outside ProcessContext |
| **Glossary** | Business term → semantic field mappings fed to prompts and ambiguity | Free-text synonyms not in glossary tables |
| **Compile** | LogicalQuery + SemanticModel → parameterized SQL (`CompiledQuery`) | "generate SQL" when you mean the compiler path |
| **Publish** | Promote semantic model draft → published version | "save" for model lifecycle |
| **Composite model** | Multi-model semantic model exposed via `/api/semantic/composites/*`, resolved to a `SemanticModel` at query time — see `docs/composite-semantic-models.md` | Treating it as deferred or purely speculative |
| **Eval / golden** | Regression suite for NL→LogicalQuery (`make eval-regression`) | Live LLM eval in pre-commit (`make eval-live`) |
| **AI job** | Async NATS-backed query/preview/run/describe/embed job | Sync HTTP handler path |
| **Agentic query runner** | `cmd/agent`'s bounded planner/tool loop (`internal/agent/`) — a policy-gated alternative to the legacy single-shot NL-to-SQL pipeline, rolled out via shadow/beta/default modes | The legacy pipeline itself; "the AI service" when you mean this separate runner |
| **Web agent mode** | The in-request SSE variant of the agentic query runner (`POST /api/agent/chat`, `internal/http/handlers/ai_agent_chat.go`) — same policy/runtime engine, no NATS job envelope, no standalone service | The agentic query runner's standalone `cmd/agent` service; "agent chat" alone without disambiguating from the NATS-driven runner |
| **Tool contract** | `internal/toolcontract` — the six governed BI tools (`run_question`, `run_logical_query`, `list_datasources`, `list_models`, `list_skills`, `run_skill`) and their loopback `Dispatcher`, shared verbatim by the MCP gateway and web agent mode | A tool definition owned by either consumer individually |
| **Skill** | Saved, named, parameterized LogicalQuery template validated by users; run via `/api/ai/skills/{id}/run` or the MCP `run_skill` tool | "saved query" in new code/docs |
| **Personal access token (PAT)** | Long-lived `bqpat_`-prefixed bearer token for MCP/API integrations; self-service CRUD at `/api/auth/me/tokens`, verified by the auth service (`internal/auth/pat.go`) | Session JWT; "API key" (`X-API-Key` is a separate header) |
| **MCP** | Model Context Protocol server (`services/mcp`) exposing the governed tools above to AI clients (Claude, Cursor, …) at `/mcp` | A second query path — it only proxies to `/api/*` |

## Key flows

1. **Sync query**: HTTP → `parseAndRouteAIQuery` → `ProcessContext.Resolve` → `processAIQuestion` → compile/run.
2. **Async query**: `executeAIQueryPhase` must use the same `ProcessContext` path as sync (no duplicate clarification logic).
3. **MCP tool call**: MCP client → gateway `/mcp` → `services/mcp` → reverse proxy back through the gateway to `/api/*` with the caller's PAT — the request crosses the Envoy gateway twice, so both routes carry explicit HTTPRoute timeouts (mcp/ai `1800s`, query `120s`; Envoy's default is 15s).
4. **Web agent chat**: `POST /api/agent/chat` → `internal/http/handlers.AIHandler.WebAgentChat` → `internal/agent`'s policy-gated planner/tool loop (same engine as `cmd/agent`, tools dispatched via `internal/toolcontract`, channel=`agent`) → SSE stream of `step`/`clarification_required`/`result` events.
5. **Security**: Read-only SQL only; row-level filters; denied fields never enter AI prompt; audit on every query.

## Turkish / business language

- Prefer **Turkish-first** descriptions and synonyms in semantic models and glossary when the user base is Turkish.
- Common business terms: **ciro** (revenue), **satış** (sales) — map to explicit metrics in the model; ambiguity arises when multiple metrics share synonyms.

## Where to look in code

| Concern | Location |
| --- | --- |
| HTTP AI handlers | `internal/http/handlers/ai.go`, `ai_job_exec.go`, `ai_context.go` |
| AI orchestration | `internal/ai/service.go`, `internal/ai/prompt/`, `internal/ai/routing/` |
| Ambiguity | `internal/ai/ambiguity/` |
| Compiler | `internal/query/compiler.go` |
| Semantic CRUD | `internal/semantic/` |
| Agentic runner | `internal/agent/`, `cmd/agent/` |
| Web agent chat | `internal/http/handlers/ai_agent_chat.go`, `internal/agent/web_tools.go`, `internal/toolcontract/` |
| MCP server | `internal/http/mcp_server.go`, `internal/http/mcp_router.go`, `services/mcp/` |
| Personal access tokens | `internal/auth/pat.go`, `internal/auth/handlers/handler_tokens.go`, `internal/http/middleware/jwt.go` (bearer-prefix detection) |
| Config flags | `internal/config/config.go` (`AmbiguityConfig`, `BI_*` env) — full list in `docs/configuration.md` |

## Related docs

- `docs/research/ambiguity-clarification-best-practices.md` — clarification architecture and roadmap
- `docs/composite-semantic-models.md` — composite model architecture and APIs
- `docs/openapi.yaml` — HTTP API
- `docs/agents/agent-runbook.md` — agentic query runner operations (mode/allowlist controls, metrics, alerts, conversation-repair CLI)
- `tasks/todo.md` — active implementation plans
- `docs/adr/` — architectural decision records (create as decisions land)
