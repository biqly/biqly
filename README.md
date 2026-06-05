# Biqly

AI-powered Business Intelligence platform that translates natural language questions into safe, parameterized SQL queries. Built with Go, React, and a semantic-layer-first architecture where **AI generates LogicalQuery JSON — never raw SQL**.

## Architecture

```text
User Question (NL)
  → Table Router (keyword + synonym + embedding + FK graph)
  → Prompt Builder (semantic context, few-shot, glossary, sample data)
  → LLM Provider (OpenAI-compatible / Anthropic)
  → LogicalQuery JSON (AI output — never raw SQL)
  → Validator (semantic validation against model)
  → Compiler (LogicalQuery → parameterized SQL, dialect-specific)
  → Read-only Checker (security gate)
  → Executor (safe execution with timeout / row limits)
  → Result Enrichment (chart suggestions, semantic types)
  → Audit / History (full traceability)
```

### Microservice Architecture

Biqly can run as a **monolith** or as **independent microservices** behind a BFF (Backend-For-Frontend) proxy:

```text
                    ┌─────────────┐
                    │   Frontend  │  React 19 + TypeScript + Vite
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  API / BFF  │  :8888 (Chi router, CORS, proxy)
                    └──────┬──────┘
              ┌────────────┼────────────┐
       ┌──────▼──────┐ ┌──▼──────┐   ┌──▼──────┐
       │   Catalog   │ │  Query  │   │   AI    │
       │   :8880     │ │  :8881  │   │  :8882  │
       └──────┬──────┘ └────┬────┘   └────┬────┘
              │             │             │
       ┌──────▼──────┐      │      ┌──────▼──────┐
       │  Auth       │      │      │   Worker    │
       │  :8889      │      │      │  (NATS)     │
       └──────┬──────┘      │      └─────────────┘
              │             │
       ┌──────▼──────┐ ┌────▼──────────┐
       │   Mail      │ │  PostgreSQL   │
       │  :8890      │ │  (metadata)   │
       └─────────────┘ └───────────────┘
```

Unset a service URL to fall back to in-process monolith handler for that domain.

## Features

### AI & Natural Language

- NL-to-LogicalQuery with self-consistency voting (multi-candidate at stepped temperatures)
- Hybrid table routing: keyword + synonym + FK graph + embedding similarity
- Business glossary injection into prompts
- Localized prompt templates (English / Turkish) with version tracking
- Few-shot example management
- Tiered context (few-shot, prior turns, glossary caps expand on retry)
- Response caching (Redis-backed, high-confidence responses)
- Follow-up intent classification with filter state inheritance
- 3-tier evaluation framework: structural comparison, execution comparison, LLM judge
- SFT dataset export for fine-tuning (Gemma 4)
- Prompt A/B testing: deterministic traffic split, statistical significance, winner recommendation
- Expression AST: sealed interface with JSON discriminator, dialect-aware compilation, visual builder
- Schema drift detection: automatic on metadata sync, email alerts, scheduled re-checks

### Query Engine API

- LogicalQuery-first: AI generates JSON, backend compiles to SQL
- Parameterized queries — never concatenates values
- 4 database dialects: PostgreSQL, MySQL, SQL Server, ClickHouse
- Window functions, CTEs, CASE expressions, HAVING, subquery filters
- Time grain support: day, week, month, quarter, year
- Query planner with join resolution and fanout detection
- SHA-256 query fingerprinting for cache/audit

### Semantic Layer

- Visual semantic model editor (canvas-based)
- Dimensions, metrics, joins with synonyms
- Calculated dimensions (expression-based with controlled AST)
- Expression builder: text mode (backend compile) and visual mode (recursive AST editor)
- Draft / publish / rollback workflow with versioning
- Circular dependency detection and expression lineage graph
- Auto-generation from introspected metadata
- Business glossary with term definitions
- Enum mappings for dimension values
- Composite semantic models — merge multiple published models into one cross-domain model ([docs](docs/composite-semantic-models.md))

### Security

- Read-only SQL checker (22+ dangerous keywords blocked)
- Row-level security filter injection at compile time
- Fail-closed permissions (nil policy = deny all)
- AES-256-GCM encrypted DSN storage
- Full audit trail for every query
- Automatic PII detection on metadata sync (7 types: email, phone, IBAN, TCKN, address, IP, credit card) with confidence scoring and admin review
- Role-based PII masking compiled into SQL (admin: raw, analyst: masked, viewer: hidden) — see `docs/pii-detection-masking.md`
- Schema drift detection: compares semantic model references against synced datasource columns, alerts on dropped/changed columns

### Authentication & Authorization

- JWT + DB session management
- RBAC with role inheritance and 4 scope types (global / workspace / datasource / model)
- MFA: TOTP + WebAuthn/Passkeys + backup codes
- OAuth2: GitHub, Google
- LDAP integration
- Magic link authentication
- Email verification, password reset, invitation flow
- Per-user AI model access control
- Multi-tenant workspaces with datasource sharing
- GDPR data export
- Password policy enforcement (length, complexity, history)
- Account state management (active / suspended / locked)
- Rate limiting via Redis
- CSRF protection, security headers

### Observability

- Structured logging (slog, JSON or text format)
- Prometheus metrics
- Distributed tracing support
- AI usage analytics (token counts, costs, per-model breakdown)
- Query history with full audit trail

### Frontend

- React 19 + TypeScript + Vite 6
- Visual semantic model editor (canvas with join lines)
- Notebook-style query builder (fields, filters, summarize, sort, having, CTE, window functions)
- AI chat panel with routing visualization
- Dashboard builder
- Admin panel: users, roles, permissions, datasource access, RLS, AI providers, AI model sharing, usage, audit log, LDAP, platform settings, workspaces, PII detection, schema drift
- i18n: English + Turkish
- Light / dark theme
- Command palette
- Keyboard shortcuts

### Deployment

- Docker Compose for local development (14 services)
- Helm umbrella chart with 7 sub-charts
- Argo CD GitOps with image updater
- Prometheus / Grafana monitoring
- Distributed tracing (Jaeger / OTEL)
- CloudNativePG support
- Dragonfly (Redis alternative)

## Quick Start

### Prerequisites

- Go 1.26.4+
- Docker & Docker Compose
- Node.js 22+ (frontend development)

### Local Development (Monolith)

```bash
# Start infrastructure (PostgreSQL, Redis, NATS)
make docker-up

# Run the API server
make dev
```

API available at `http://localhost:8888`.

### Local Development (Microservices)

```bash
# Terminal 1: Catalog Service
BI_HTTP_PORT=8880 make run-catalog

# Terminal 2: Query Engine
BI_HTTP_PORT=8881 \
BI_CATALOG_SERVICE_URL=http://localhost:8880 \
make run-query

# Terminal 3: AI Service
BI_HTTP_PORT=8882 \
BI_CATALOG_SERVICE_URL=http://localhost:8880 \
BI_QUERY_SERVICE_URL=http://localhost:8881 \
make run-ai

# Terminal 4: BFF / Public API
BI_HTTP_PORT=8888 \
BI_CATALOG_SERVICE_URL=http://localhost:8880 \
BI_QUERY_SERVICE_URL=http://localhost:8881 \
BI_AI_SERVICE_URL=http://localhost:8882 \
make run
```

### Frontend Development

```bash
cd frontend
npm install
npm run dev
```

Frontend available at `http://localhost:3333`.

### Environment Variables

```bash
cp .env.example .env
# Edit .env with your values
```

See `.env.example` for all available options. AI providers and models are configured at runtime via the admin UI.

## Supported Databases

| Database | Driver | Dialect |
| ---------- | -------- | --------- |
| PostgreSQL | `jackc/pgx/v5` | `$1, $2` placeholders, `date_trunc` |
| MySQL | `go-sql-driver/mysql` | `?` placeholders, `DATE_FORMAT` |
| SQL Server | `microsoft/go-mssqldb` | `@p1` placeholders, `DATETRUNC` |
| ClickHouse | `clickhouse-go/v2` | `toStartOf*` functions, special `LIMIT` |

## Project Structure

```text
cmd/
  api/                  API server + BFF proxy (:8888)
  auth/                 Auth microservice (:8889)
  mail/                 Mail worker (:8890)
  worker/               AI job worker (NATS consumer)
  migrate/              Metadata DB migration CLI
  auth-migrate/         Auth DB migration CLI
  mail-migrate/         Mail DB migration CLI
  export-sft/           SFT dataset exporter for fine-tuning

services/
  ai/                   Standalone AI service (:8882)
  catalog/              Standalone catalog service (:8880)
  query/                Standalone query engine (:8881)

internal/
  ai/                   AI/LLM text-to-LogicalQuery pipeline
    provider/           LLM provider implementations (OpenAI, Anthropic)
    prompt/             Prompt engineering, templates, budget, glossary
    routing/            Hybrid table router (keyword + embedding + FK)
    eval/               Golden eval framework (structural + execution + LLM judge)
    lingua/             Language/locale detection
    jsonextract/        JSON extraction from LLM output
  app/                  Dependency injection (monolith + per-service wiring)
  audit/                Audit event logging
  auth/                 Authentication & authorization
    handlers/           Auth HTTP handlers
    mfa/                MFA (TOTP + WebAuthn/Passkeys)
    rbac/               Role-based access control
    workspace/          Multi-tenant workspace management
    ldap/               LDAP client
    oauth/              OAuth2 providers (GitHub, Google)
  config/               Environment-based configuration
  core/                 High-level query service orchestration
  dashboard/            Dashboard persistence
  datasource/           Database driver abstraction
    postgres/           PostgreSQL driver + introspection
    mysql/              MySQL driver
    sqlserver/          SQL Server driver
    clickhouse/         ClickHouse driver
  dialect/              SQL dialect implementations (13-method interface)
  emailaddr/            Email validation utilities
  errmsg/               Structured error messages
  http/                 HTTP layer
    handlers/           47 handler files for all endpoints
    middleware/          JWT, RBAC, API key, security headers, locale
    response/           Standardized response helpers
  i18n/                 Internationalization (en + tr)
  mail/                 Transactional email (SMTP, templates, block-list)
  metadata/             Metadata repository (PostgreSQL CRUD)
  platform/             Infrastructure (db, redis, logger, observability)
  query/                Query compilation & execution
    logical.go          LogicalQuery type aliases
    compiler.go         LogicalQuery → SQL compiler
    compiler_filter.go  Filter compilation
    compiler_case.go    CASE expression compilation
    compiler_nested.go  Subquery/CTE compilation
    executor.go         Safe query executor
    validator.go        LogicalQuery validation
    planner.go          Query planner (joins, fanout)
    enrich.go           Result enrichment + chart suggestions
    fingerprint.go      SHA-256 fingerprinting
  queue/                Job queue (NATS JetStream + in-memory fallback)
  security/             Encryption, read-only checker, permissions, RLS
  semantic/             Semantic layer (models, CRUD, publish workflow)
  semanticgen/          Auto-generate semantic models from metadata

pkg/
  aiclient/             AI service HTTP client (for other services)
  catalogclient/        Catalog service HTTP client
  queryclient/          Query service HTTP client
  common/               Shared utilities (errors, httpclient, requestid, tracecontext)
  internalapi/          Inter-service API types
  logicalquery/         Canonical LogicalQuery type definitions
  metadata/             Canonical metadata type definitions
  query/                Canonical query result types
  security/             Canonical security type definitions
  semantic/             Canonical semantic model type definitions

frontend/               React 19 + TypeScript + Vite 6
  src/
    api/                API client modules
    components/
      admin/            Admin panel (22 components)
      aiQuery/          AI chat panel + routing visualization
      auth/             Sign in, sign up, MFA, OAuth, magic link
      datasources/      Datasource management
      evaluation/       Eval suite UI
      metadata/         Metadata browser + AI describe
      modeling/         Visual semantic model editor (canvas)
      queryBuilder/     Notebook-style query builder
      resultTable/      Result table with sorting + anomaly detection
      savedQuestions/   Saved question management
      settings/         Profile, AI preferences, MFA, avatar
      sharing/          Resource sharing
      ui/               Reusable UI component library
      workspaces/       Workspace selector + settings
    hooks/              18 custom React hooks
    i18n/               English + Turkish translations
    styles/             CSS modules
    theme/              Light/dark theme provider
    types/              TypeScript type definitions
    utils/              Utility functions

deploy/
  helm/biqly/           Helm umbrella chart (7 sub-charts)
    charts/             catalog, query, ai, auth, mail, frontend, postgresql
    templates/          28 templates (configmaps, secrets, monitoring, etc.)
  argocd/               Argo CD Application + Image Updater config

migrations/
  001-036               Metadata DB (36 migrations)
  auth/001-035          Auth DB (35 migrations)
  mail/001              Mail DB (1 migration)

scripts/                Operational scripts (seed data, keygen, env sync)
docs/                   Architecture docs, design specs, runbooks (28 files)
examples/               AdventureWorks sample queries, NL test sets
testdata/               Golden SQL test files
```

## API Endpoints

### Infrastructure

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/health` | Liveness probe |
| `GET` | `/ready` | Readiness probe (pings DB + upstream services) |
| `GET` | `/metrics` | Prometheus metrics |

### Datasources

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/datasources` | Create datasource |
| `GET` | `/api/datasources` | List datasources |
| `POST` | `/api/datasources/test-connection` | Test draft connection config |
| `GET` | `/api/datasources/{id}` | Get datasource |
| `PUT` | `/api/datasources/{id}` | Update datasource |
| `DELETE` | `/api/datasources/{id}` | Delete datasource |
| `POST` | `/api/datasources/{id}/test` | Test existing connection |
| `POST` | `/api/datasources/{id}/sync-metadata` | Introspect & store schema (auto PII scan, `?scan_pii=false` to skip) |
| `POST` | `/api/datasources/{id}/scan-pii` | Trigger PII detection scan |
| `GET` | `/api/datasources/{id}/pii-columns` | List PII-annotated columns |

### Metadata

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/datasources/{id}/tables` | List synced tables |
| `GET` | `/api/datasources/{id}/columns` | List synced columns |
| `GET` | `/api/metadata/columns/search` | Search columns |
| `GET` | `/api/metadata/tables/search` | Search tables |
| `PATCH` | `/api/metadata/tables/{id}` | Update table description |
| `PATCH` | `/api/metadata/columns/{id}` | Update column description |
| `GET` | `/api/metadata/tables/{id}/translations` | Get table locale translations |
| `PUT` | `/api/metadata/tables/{id}/translations` | Upsert table translations |
| `GET` | `/api/metadata/columns/{id}/translations` | Get column locale translations |
| `PUT` | `/api/metadata/columns/{id}/translations` | Upsert column translations |
| `PATCH` | `/api/metadata/columns/{id}/pii` | Manually set/override column PII annotation |
| `DELETE` | `/api/metadata/columns/{id}/pii` | Clear PII annotation (mark reviewed safe) |
| `GET` | `/api/compliance/pii-summary` | PII compliance summary (`?format=csv`) |

### Semantic Models

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/semantic/models` | Create semantic model |
| `POST` | `/api/semantic/models/generate` | Auto-generate model from metadata |
| `GET` | `/api/semantic/models` | List models |
| `GET` | `/api/semantic/models/{id}` | Get full model |
| `GET` | `/api/semantic/models/{id}/fields` | List model fields |
| `PUT` | `/api/semantic/models/{id}` | Update model |
| `DELETE` | `/api/semantic/models/{id}` | Delete model |
| `POST` | `/api/semantic/models/{id}/validate` | Validate model |
| `POST` | `/api/semantic/models/{id}/publish` | Publish model |
| `POST` | `/api/semantic/models/{id}/rollback` | Rollback model |
| `POST` | `/api/semantic/models/{id}/dimensions` | Add dimension |
| `PUT` | `/api/semantic/models/{id}/dimensions/{did}` | Update dimension |
| `DELETE` | `/api/semantic/models/{id}/dimensions/{did}` | Remove dimension |
| `GET` | `/api/semantic/models/{id}/dimensions/{did}/enums` | Get dimension enums |
| `PUT` | `/api/semantic/models/{id}/dimensions/{did}/enums` | Replace dimension enums |
| `POST` | `/api/semantic/models/{id}/metrics` | Add metric |
| `PUT` | `/api/semantic/models/{id}/metrics/{mid}` | Update metric |
| `DELETE` | `/api/semantic/models/{id}/metrics/{mid}` | Remove metric |
| `POST` | `/api/semantic/models/{id}/joins` | Add join |
| `PUT` | `/api/semantic/models/{id}/joins/{jid}` | Update join |
| `DELETE` | `/api/semantic/models/{id}/joins/{jid}` | Remove join |
| `GET` | `/api/semantic/models/{id}/suggested-joins` | AI-suggested joins |
| `GET` | `/api/semantic/models/{id}/lineage` | Expression lineage graph (nodes + edges) |
| `POST` | `/api/semantic/models/{id}/compile-expression` | Validate and compile expression AST/string to SQL |

### Composite Semantic Models

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/semantic/composites` | Create composite |
| `GET` | `/api/semantic/composites` | List composites |
| `GET` | `/api/semantic/composites/{id}` | Get full composite |
| `PUT` | `/api/semantic/composites/{id}` | Update composite |
| `DELETE` | `/api/semantic/composites/{id}` | Delete composite |
| `POST` | `/api/semantic/composites/{id}/components` | Add component |
| `DELETE` | `/api/semantic/composites/{id}/components/{model_id}` | Remove component |
| `POST` | `/api/semantic/composites/{id}/cross-joins` | Add cross-model join |
| `PUT` | `/api/semantic/composites/{id}/cross-joins/{join_id}` | Update cross-model join |
| `DELETE` | `/api/semantic/composites/{id}/cross-joins/{join_id}` | Remove cross-model join |
| `PUT` | `/api/semantic/composites/{id}/canonical-date` | Set canonical date |
| `PUT` | `/api/semantic/composites/{id}/dimension-resolutions` | Set dimension resolutions |
| `POST` | `/api/semantic/composites/{id}/validate` | Validate composite |
| `POST` | `/api/semantic/composites/{id}/publish` | Publish composite |
| `POST` | `/api/semantic/composites/{id}/rollback` | Rollback composite |
| `GET` | `/api/semantic/composites/{id}/suggested-joins` | Suggested cross-model joins |

### Query Engine

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/query/compile` | Compile LogicalQuery to SQL |
| `POST` | `/api/query/run` | Execute query |
| `POST` | `/api/query/explain` | EXPLAIN query |
| `GET` | `/api/query/history` | Query history |
| `GET` | `/api/query/history/{id}` | Single history entry |

### AI — Query

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/ai/query` | NL → LogicalQuery |
| `POST` | `/api/ai/query/preview` | NL → LogicalQuery + SQL preview |
| `POST` | `/api/ai/query/run` | NL → execute and return results |
| `POST` | `/api/ai/metadata/describe` | AI-generated descriptions |
| `POST` | `/api/ai/metadata/embed` | Refresh embeddings |

### AI — History & Usage

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/ai/history` | Paginated AI query history |
| `GET` | `/api/ai/history/detail` | Single AI history detail |
| `GET` | `/api/ai/query/history` | Current user's recent queries |
| `GET` | `/api/ai/usage` | AI usage analytics |
| `GET` | `/api/ai/usage/breakdown` | Detailed usage breakdown |

### AI — Settings & Preferences

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/ai/settings` | Runtime AI settings |
| `GET` | `/api/ai/user-models` | Available models (RBAC-filtered) |
| `PUT` | `/api/ai/user-preferences` | Set per-user AI model preference |
| `DELETE` | `/api/ai/user-preferences/{purpose}` | Remove model preference |

### AI — Examples & Feedback

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/ai/examples` | List few-shot examples |
| `POST` | `/api/ai/examples` | Create example |
| `PUT` | `/api/ai/examples/{id}` | Update example |
| `DELETE` | `/api/ai/examples/{id}` | Delete example |
| `POST` | `/api/ai/feedback` | Submit feedback |

### AI — Glossary

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/ai/glossary` | List glossary terms |
| `POST` | `/api/ai/glossary` | Create term |
| `PUT` | `/api/ai/glossary/{id}` | Update term |
| `DELETE` | `/api/ai/glossary/{id}` | Delete term |

### AI — Prompt Templates

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/ai/prompt-templates` | List templates |
| `PUT` | `/api/ai/prompt-templates/{name}/{locale}` | Upsert template |
| `POST` | `/api/ai/prompt-templates/restore` | Restore defaults |
| `POST` | `/api/ai/prompt-templates/reseed` | Re-seed all templates |

### AI — Time Grains

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/ai/settings/time-grains` | List time grain definitions |
| `PUT` | `/api/ai/settings/time-grains/{grain}` | Update time grain |

### AI — Jobs (async)

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/ai/jobs` | Enqueue AI job |
| `GET` | `/api/ai/jobs` | List jobs for session |
| `GET` | `/api/ai/jobs/{id}` | Get job status |
| `DELETE` | `/api/ai/jobs/{id}` | Cancel job |
| `POST` | `/api/ai/jobs/cancel-active` | Cancel active jobs |
| `POST` | `/api/ai/jobs/cancel-batch` | Cancel multiple jobs |
| `GET` | `/api/ai/jobs/queue/status` | Queue status |

### AI — Admin (eval, providers, models)

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/ai/eval/run` | Run golden evaluation suite |
| `GET` | `/api/ai/eval/run/stream` | Stream eval progress (SSE) |
| `GET` | `/api/ai/eval/runs` | List eval runs |
| `GET` | `/api/ai/eval/runs/{id}` | Get eval run details |
| `GET` | `/api/ai/eval/regression` | Regression report |
| `GET` | `/api/ai/eval/cases` | List eval cases |
| `POST` | `/api/ai/eval/cases` | Create eval case |
| `DELETE` | `/api/ai/eval/cases/{id}` | Delete eval case |
| `GET` | `/api/ai/providers` | List AI providers |
| `POST` | `/api/ai/providers` | Create provider |
| `GET` | `/api/ai/providers/{id}` | Get provider |
| `PUT` | `/api/ai/providers/{id}` | Update provider |
| `DELETE` | `/api/ai/providers/{id}` | Delete provider |
| `POST` | `/api/ai/providers/{id}/test` | Test provider connection |
| `GET` | `/api/ai/providers/{id}/remote-models` | List remote models |
| `GET` | `/api/ai/models` | List configured models |
| `POST` | `/api/ai/models` | Create model config |
| `PUT` | `/api/ai/models/{id}` | Update model config |
| `DELETE` | `/api/ai/models/{id}` | Delete model config |
| `POST` | `/api/ai/models/{id}/default` | Set default model |

### AI — Prompt A/B Testing

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/ai/ab-experiments` | Create experiment (admin only) |
| `GET` | `/api/ai/ab-experiments` | List experiments |
| `GET` | `/api/ai/ab-experiments/{id}` | Get experiment details |
| `PUT` | `/api/ai/ab-experiments/{id}` | Update experiment details |
| `PUT` | `/api/ai/ab-experiments/{id}/status` | Transition experiment status |
| `POST` | `/api/ai/ab-experiments/{id}/variants` | Add variant |
| `PUT` | `/api/ai/ab-experiments/{id}/variants/{variantId}` | Update variant |
| `DELETE` | `/api/ai/ab-experiments/{id}/variants/{variantId}` | Delete variant |
| `GET` | `/api/ai/ab-experiments/{id}/metrics` | Compute and return experiment metrics |
| `GET` | `/api/ai/ab-experiments/{id}/timeseries` | Daily metrics timeseries breakdown |
| `GET` | `/api/ai/ab-experiments/{id}/recommendation` | Get winner recommendation analysis |

### Schema Drift

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/semantic/models/{id}/drift` | List unresolved drift reports for model |
| `GET` | `/api/datasources/{id}/drift` | List unresolved drifts across all models |
| `POST` | `/api/drift/{id}/resolve` | Mark drift report as resolved |

### Permissions

| Method | Path | Description |
| -------- | ------ | ------------- |
| `GET` | `/api/permissions/` | List policies |
| `GET` | `/api/permissions/keys` | Get by user + datasource |
| `PUT` | `/api/permissions/` | Upsert policy |
| `DELETE` | `/api/permissions/{id}` | Delete policy |

### Dashboards

| Method | Path | Description |
| -------- | ------ | ------------- |
| `POST` | `/api/dashboards/` | Create dashboard |
| `GET` | `/api/dashboards/` | List dashboards |
| `GET` | `/api/dashboards/{id}` | Get dashboard |
| `PUT` | `/api/dashboards/{id}` | Update dashboard |
| `DELETE` | `/api/dashboards/{id}` | Delete dashboard |

## Tech Stack

| Layer | Technology |
| ------- | ----------- |
| Backend | Go 1.26.4 |
| HTTP | `go-chi/chi/v5` |
| Metadata DB | PostgreSQL via `jackc/pgx/v5` |
| Cache | Redis (`redis/go-redis/v9`) |
| Queue | NATS JetStream (`nats-io/nats.go`) |
| Auth | JWT (`golang-jwt/jwt/v5`), bcrypt, WebAuthn |
| Migrations | Custom runner (idempotent, PostgreSQL error codes) |
| Logging | `log/slog` (JSON or text) |
| Metrics | Prometheus (`prometheus/client_golang`) |
| Linting | golangci-lint v2 |
| Testing | `stretchr/testify` + Go standard |
| Frontend | React 19, TypeScript 5, Vite 6, Recharts |
| AI Providers | OpenAI-compatible, Anthropic |
| Supported DBs | PostgreSQL, MySQL, SQL Server, ClickHouse |
| Deployment | Docker, Helm, Argo CD |

## Configuration

Key environment variables (prefix `BI_`):

| Variable | Default | Purpose |
| ---------- | --------- | --------- |
| `BI_HTTP_PORT` | `8888` | API server port |
| `BI_METADATA_DB_DSN` | — | PostgreSQL metadata DB |
| `BI_REDIS_DSN` | — | Redis cache |
| `BI_NATS_URL` | — | NATS JetStream for async jobs |
| `BI_QUERY_TIMEOUT_SECONDS` | `30` | Query execution timeout |
| `BI_QUERY_MAX_ROWS` | `10000` | Max result rows |
| `BI_ENCRYPTION_KEY` | — | 32-byte AES-256-GCM key for DSNs |
| `BI_AI_HTTP_TIMEOUT_SECONDS` | `300` | AI request timeout |
| `BI_AI_RATE_LIMIT_PER_MINUTE` | `20` | AI request rate limit |
| `BI_AI_MAX_PROMPT_RUNES` | `80000` | Prompt size cap |
| `BI_AI_MULTI_CANDIDATE_COUNT` | `1` | Self-consistency candidates |
| `BI_ADMIN_API_KEY` | — | Admin API key |
| `BI_LOG_LEVEL` | `info` | Log level |
| `BI_LOG_FORMAT` | `json` | Log format (`json` or `text`) |

AI providers, models, and API keys are configured at runtime via the admin UI, not environment variables.

## Turkish Language Support

If business users mostly ask questions in Turkish, keep table and column descriptions Turkish-first. The AI router embeds user questions together with metadata descriptions, so Turkish descriptions improve matching:

```text
Müşteri siparişlerinin başlık bilgileri. Sipariş tarihi, müşteri, bölge,
toplam tutar ve durum bilgisini içerir. Teknik tablo adı: SalesOrderHeader.
```

After metadata sync or description edits, refresh embeddings from the AI Query page.

Optional translation layer for AI-generated descriptions (e.g., Ollama TranslateGemma):

```env
BI_AI_TRANSLATION_MODEL=translategemma:4b
BI_AI_TRANSLATION_BASE_URL=http://localhost:11434/v1
BI_AI_TRANSLATION_TARGET_LANGUAGE=Turkish
BI_AI_TRANSLATION_TARGET_CODE=tr
```

## Development Commands

```bash
make build              # Build API binary
make build-catalog      # Build catalog service
make build-query        # Build query service
make build-ai           # Build AI service
make run                # Build and run API
make run-catalog        # Run catalog service
make run-query          # Run query service
make run-ai             # Run AI service
make dev                # Run API with go run
make test               # Run tests with race detection + coverage
make eval               # Run golden eval tests
make eval-regression    # Run eval regression gate
make lint               # Run golangci-lint
make semgrep-scan       # Run Semgrep security scan
make migrate-up         # Run metadata DB migrations
make migrate-down       # Rollback last migration
make docker-up          # Start all Docker Compose services
make docker-down        # Stop and remove all services + volumes
make seed-adventureworks # Load AdventureWorks test data
make export-sft         # Export SFT training data
make helm-deps          # Update Helm chart dependencies
make helm-lint          # Lint Helm chart
make helm-template      # Render Helm templates
make clean              # Remove bin/ and coverage.out
```

## Database Schema

Metadata DB: PostgreSQL `bi_metadata`, 42 migrations.

| Table | Purpose |
| ------- | --------- |
| `datasources` | Database connections (DSN AES-encrypted) |
| `schemas`, `tables`, `columns` | Synced schema metadata |
| `relations` | Foreign key relationships |
| `semantic_models` | Semantic models (draft/published, versioned) |
| `semantic_dimensions` | Dimensions with type, synonyms, calculated expressions (string + AST JSON) |
| `semantic_metrics` | Metrics with aggregation, expression, synonyms (string + AST JSON) |
| `semantic_joins` | Join definitions with cardinality |
| `query_history` | Compiled SQL, status, duration, fingerprint |
| `ai_query_history` | AI prompts, responses, confidence, token usage |
| `few_shot_examples` | Curated NL → LogicalQuery pairs |
| `ai_feedback` | User feedback (correct/wrong/unsafe/ambiguous) |
| `permissions` | Allowed models, denied fields, row filters |
| `business_glossary` | Business term definitions |
| `ai_jobs` | Async AI job tracking |
| `ai_prompt_templates` | Versioned prompt template storage |
| `time_grain_synonyms` | Customizable time grain definitions |
| `ai_providers` / `ai_models` | Runtime AI provider/model registry |
| `audit_events` | Audit trail |
| `dashboards` | Dashboard persistence |
| `ab_experiments` | Prompt A/B testing experiments config |
| `ab_variants` | A/B testing experiment variants and traffic splits |
| `drift_reports` | Schema drift detection reports (per model, per sync) |

Auth DB: PostgreSQL `bi_auth`, 35 migrations (users, sessions, OAuth, MFA, RBAC, workspaces, invitations, LDAP).

Mail DB: PostgreSQL `bi_mail`, 1 migration (block list).

## LogicalQuery Reference

```json
{
  "version": "v1",
  "datasource_id": "ds-123",
  "model_id": "model-456",
  "select": [
    { "type": "dimension", "name": "order_date", "alias": "Month" },
    { "type": "metric", "name": "revenue" },
    { "type": "window", "name": "revenue", "alias": "running_total",
      "window": { "aggregation": "sum", "partition_by": ["category"], "order_by": [{"field": "order_date", "direction": "asc"}] } }
  ],
  "filters": [
    { "field": "order_date", "operator": "between", "value": ["2024-01-01", "2024-12-31"] },
    { "field": "status", "operator": "in", "value": ["completed", "shipped"] }
  ],
  "group_by": [
    { "field": "order_date", "time_grain": "month" }
  ],
  "having": [
    { "field": "revenue", "operator": "gt", "value": 10000 }
  ],
  "order_by": [
    { "field": "order_date", "direction": "asc" }
  ],
  "limit": 100
}
```

**Filter operators:** `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `not_in`, `contains`, `starts_with`, `ends_with`, `between`, `is_null`, `is_not_null`

**Time grains:** `day`, `week`, `month`, `quarter`, `year`

**SelectItem types:** `dimension`, `metric`, `window`, `case`

## Security Model

### Always Reject

INSERT, UPDATE, DELETE, MERGE, DROP, ALTER, TRUNCATE, CREATE, GRANT, REVOKE, EXEC, EXECUTE, CALL, xp_cmdshell, OPENROWSET, BULK INSERT, pg_read_file, LOAD_FILE, dblink, lo_import

### Always Enforce

- Read-only connection credentials
- Query timeout and row limits
- Parameterized queries (never concatenate values)
- Identifier quoting per dialect
- Permission checks before generation AND execution
- Audit log for every query
- AES-256-GCM encrypted DSN storage
- SHA-256 query fingerprinting

### Permission Layers

1. **Semantic context builder** — denied fields never enter AI prompt
2. **AI prompt builder** — `WithDeniedFields` strips fields
3. **LogicalQuery validator** — validates against permitted fields
4. **SQL compiler** — row-level filter injection via `CompileWithPermissions`
5. **Read-only checker** — 22+ dangerous keywords blocked after comment/string stripping
6. **Audit log** — permission decision logged

## Roadmap

- [x] Phase 1: Core Foundation
- [x] Phase 2: Metadata Database
- [x] Phase 3: Datasource Driver System (PostgreSQL, MySQL, SQL Server, ClickHouse)
- [x] Phase 4: Semantic Layer
- [x] Phase 5: Logical Query Model
- [x] Phase 6: SQL Compiler
- [x] Phase 7: Safe Query Execution
- [x] Phase 8: Query API
- [x] Phase 9: Datasource API (CRUD, sync, encryption)
- [x] Phase 10: AI Text-to-Query
- [x] Phase 11: Query Planner
- [x] Phase 12: Permissions & Row-Level Security
- [x] Phase 13: Caching (Redis)
- [x] Phase 14: Observability
- [x] Phase 15: Frontend
- [x] Phase 16: Testing (unit, integration, golden SQL, eval framework)
- [x] Authentication & Authorization (JWT, RBAC, MFA, OAuth, LDAP)
- [x] Transactional Email Service
- [x] Microservice Decomposition (AI, Query, Catalog services)
- [x] Dashboard Builder
- [x] Business Glossary
- [x] Prompt Template Management
- [x] AI Provider Management (runtime, DB-backed)
- [x] SFT Dataset Export for Fine-Tuning
- [x] Helm Chart + Argo CD GitOps
- [x] Composite Semantic Models
- [x] Metric Expression Security (controlled AST)
- [x] Schema Drift Detection & Alerts
- [x] Prompt A/B Testing
- [ ] Embedding-based Learning from User Confirmations
