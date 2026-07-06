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
| **Agentic query runner** | `cmd/agent` — bounded, policy-gated planner/tool loop; standalone service, `agent.enabled=false` by default, no public route |
| **Frontend** | React 19 + Vite — chat, modeling UI, charts (Recharts), i18n via `useT()` |

Production deploys `catalog`, `query`, `ai`, `worker`, `auth`, `frontend`, and `mail` into Kubernetes namespace `biqly`; the biqly system databases now live on the shared `prag-postgresql` instance in namespace `postgresql`.

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

## Key flows

1. **Sync query**: HTTP → `parseAndRouteAIQuery` → `ProcessContext.Resolve` → `processAIQuestion` → compile/run.
2. **Async query**: `executeAIQueryPhase` must use the same `ProcessContext` path as sync (no duplicate clarification logic).
3. **Security**: Read-only SQL only; row-level filters; denied fields never enter AI prompt; audit on every query.

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
| Config flags | `internal/config/config.go` (`AmbiguityConfig`, `BI_*` env) |

## Related docs

- `docs/research/ambiguity-clarification-best-practices.md` — clarification architecture and roadmap
- `docs/composite-semantic-models.md` — composite model architecture and APIs
- `docs/openapi.yaml` — HTTP API
- `docs/agents/agent-runbook.md` — agentic query runner operations (mode/allowlist controls, metrics, alerts, conversation-repair CLI)
- `tasks/todo.md` — active implementation plans
- `docs/adr/` — architectural decision records (create as decisions land)
