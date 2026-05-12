# Biqly Roadmap

Biqly is a Business Intelligence platform: a Go backend that generates safe
SQL from a structured logical query (and from natural language via an LLM),
plus a React frontend for managing datasources, browsing introspected
metadata, building queries, and asking AI questions.

This document tracks what is planned, in progress, and done across both the
backend and the frontend.

## Goal

Build a Business Intelligence query engine in Go that can generate safe SQL queries from structured logical queries and later from natural language prompts.

The system should work like a simplified Metabase-style query engine:

1. Connect to different databases.
2. Introspect schemas, tables, columns, indexes, relations, and data types.
3. Store metadata in an internal metadata database.
4. Define a semantic layer with datasets, dimensions, metrics, joins, aliases, and permissions.
5. Accept a database-independent `LogicalQuery` object.
6. Compile `LogicalQuery` into dialect-specific SQL.
7. Execute the query safely.
8. Return tabular results and chart-ready data.
9. Later, use AI to convert natural language questions into `LogicalQuery`, not directly into raw SQL.

---

## Recommended Go Stack

### Backend

- Language: Go 1.23+
- HTTP framework: `chi`, `gin`, or standard `net/http`
- Internal metadata DB: PostgreSQL
- PostgreSQL driver: `github.com/jackc/pgx/v5`
- SQL builder/parser:
  - Start with custom SQL compiler for control
  - Optional later: `github.com/doug-martin/goqu/v9`
  - Optional SQL parsing/validation: `github.com/xwb1989/sqlparser` or dialect-specific alternatives
- Migrations:
  - `github.com/golang-migrate/migrate`
  - or `github.com/pressly/goose`
- Config:
  - `github.com/spf13/viper`
  - or plain environment variables
- Logging:
  - `log/slog`
  - optional: `go.uber.org/zap`
- Validation:
  - `github.com/go-playground/validator/v10`
- Background jobs:
  - initially simple Go workers
  - optional later: NATS JetStream, Asynq, or Temporal
- Cache:
  - Redis or Dragonfly
- Auth:
  - JWT/session-based auth
  - OIDC later
- API format:
  - REST first
  - OpenAPI spec recommended

### AI Layer

- LLM should not generate raw SQL directly.
- LLM should generate a strict JSON object matching `LogicalQuery`.
- Backend validates the JSON.
- Backend compiles the validated logical query to SQL.
- Backend applies permissions, row limits, timeout, and read-only rules.

---

## Suggested Repository Structure

```text
biqly/
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── api.go
│   │   └── dependencies.go
│   ├── config/
│   │   └── config.go
│   ├── http/
│   │   ├── router.go
│   │   ├── middleware.go
│   │   └── handlers/
│   │       ├── datasources.go
│   │       ├── metadata.go
│   │       ├── semantic.go
│   │       ├── query.go
│   │       └── ai.go
│   ├── datasource/
│   │   ├── datasource.go
│   │   ├── registry.go
│   │   ├── postgres/
│   │   │   ├── driver.go
│   │   │   ├── introspect.go
│   │   │   └── dialect.go
│   │   ├── mysql/
│   │   │   ├── driver.go
│   │   │   ├── introspect.go
│   │   │   └── dialect.go
│   │   ├── sqlserver/
│   │   │   ├── driver.go
│   │   │   ├── introspect.go
│   │   │   └── dialect.go
│   │   └── clickhouse/
│   │       ├── driver.go
│   │       ├── introspect.go
│   │       └── dialect.go
│   ├── metadata/
│   │   ├── model.go
│   │   ├── repository.go
│   │   └── service.go
│   ├── semantic/
│   │   ├── model.go
│   │   ├── repository.go
│   │   ├── resolver.go
│   │   ├── validator.go
│   │   ├── composite_model.go
│   │   ├── composite_repository.go
│   │   ├── composite_resolver.go
│   │   └── composite_validator.go
│   ├── query/
│   │   ├── logical.go
│   │   ├── compiler.go
│   │   ├── executor.go
│   │   ├── planner.go
│   │   ├── validator.go
│   │   └── result.go
│   ├── dialect/
│   │   ├── dialect.go
│   │   ├── postgres.go
│   │   ├── mysql.go
│   │   ├── sqlserver.go
│   │   └── clickhouse.go
│   ├── security/
│   │   ├── permissions.go
│   │   ├── policy.go
│   │   └── readonly.go
│   ├── ai/
│   │   ├── prompt.go
│   │   ├── schema.go
│   │   ├── client.go
│   │   ├── service.go
│   │   └── validator.go
│   ├── audit/
│   │   ├── model.go
│   │   └── repository.go
│   └── platform/
│       ├── db/
│       ├── redis/
│       └── logger/
├── migrations/
├── docs/
│   ├── architecture.md
│   ├── logical-query.md
│   ├── semantic-layer.md
│   └── ai-text-to-query.md
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

---

## Phase 1 — Core Foundation

### 1. Initialize Go Project

- [x] Create repository: `biqly`
- [x] Initialize module:

```bash
go mod init github.com/YOUR_ORG/biqly
```

- [x] Add base folders:

```bash
mkdir -p cmd/api internal/{config,http,datasource,metadata,semantic,query,dialect,security,ai,audit,platform} migrations docs
```

- [x] Add `Makefile`
- [x] Add `.golangci.yml`
- [x] Add Dockerfile
- [x] Add `docker-compose.yml` with:
  - API
  - PostgreSQL metadata DB
  - Redis/Dragonfly
  - optional test PostgreSQL datasource

### 2. Define Application Config

Create `internal/config/config.go`.

Required config:

- [x] API host
- [x] API port
- [x] Metadata database DSN
- [x] Redis DSN
- [x] Query timeout
- [x] Max result rows
- [x] Max query runtime
- [x] AI provider config
- [x] Encryption key for datasource credentials

Example:

```go
type Config struct {
    HTTP      HTTPConfig
    Metadata  MetadataConfig
    Query     QueryConfig
    Security  SecurityConfig
    AI        AIConfig
}

type QueryConfig struct {
    TimeoutSeconds int
    MaxRows        int
}
```

---

## Phase 2 — Metadata Database

The metadata database stores information about user-defined datasources, schemas, tables, columns, joins, metrics, permissions, and query history.

### 3. Create Metadata Tables

Create migrations for:

- [x] `datasources`
- [x] `schemas`
- [x] `tables`
- [x] `columns`
- [x] `relations`
- [x] `semantic_models`
- [x] `semantic_dimensions`
- [x] `semantic_metrics`
- [x] `semantic_joins`
- [x] `query_history`
- [x] `query_saved`
- [x] `ai_query_history`
- [x] `permissions`

Suggested `datasources` table:

```sql
CREATE TABLE datasources (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    dsn_encrypted TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Suggested `columns` table:

```sql
CREATE TABLE columns (
    id UUID PRIMARY KEY,
    datasource_id UUID NOT NULL REFERENCES datasources(id),
    schema_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    data_type TEXT NOT NULL,
    nullable BOOLEAN NOT NULL DEFAULT true,
    ordinal_position INT,
    is_primary_key BOOLEAN NOT NULL DEFAULT false,
    is_foreign_key BOOLEAN NOT NULL DEFAULT false,
    referenced_schema TEXT,
    referenced_table TEXT,
    referenced_column TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4. Implement Metadata Repository

Create:

```text
internal/metadata/model.go
internal/metadata/repository.go
internal/metadata/service.go
```

Repository methods:

- [x] CreateDatasource
- [x] GetDatasource
- [x] ListDatasources
- [x] DeleteDatasource
- [x] UpsertSchemas
- [x] UpsertTables
- [x] UpsertColumns
- [x] UpsertRelations
- [x] GetDatasourceMetadata
- [x] SearchColumns
- [x] SearchTables

---

## Phase 3 — Datasource Driver System

### 5. Define Driver Interfaces

Create `internal/datasource/datasource.go`.

```go
package datasource

import (
    "context"
    "database/sql"
)

type Driver interface {
    Type() string
    Open(ctx context.Context, dsn string) (*sql.DB, error)
    Ping(ctx context.Context, dsn string) error
    Introspect(ctx context.Context, db *sql.DB) (*IntrospectionResult, error)
    Dialect() Dialect
}

type Dialect interface {
    Name() string
    QuoteIdent(identifier string) string
    Placeholder(index int) string
    LimitOffset(limit, offset int) string
}
```

### 6. Create Driver Registry

Create `internal/datasource/registry.go`.

- [x] Register available drivers
- [x] Resolve driver by datasource type
- [x] Return error for unsupported driver

Supported first:

- [x] PostgreSQL
- [x] MySQL
- [x] SQL Server
- [x] ClickHouse

Implementation order:

1. PostgreSQL
2. MySQL
3. SQL Server
4. ClickHouse

### 7. Implement PostgreSQL Driver

Create:

```text
internal/datasource/postgres/driver.go
internal/datasource/postgres/introspect.go
internal/datasource/postgres/dialect.go
```

Use:

```go
import _ "github.com/jackc/pgx/v5/stdlib"
```

Introspection queries should read:

- [x] schemas
- [x] tables
- [x] columns
- [x] primary keys
- [x] foreign keys
- [x] indexes
- [x] views

PostgreSQL metadata sources:

- `information_schema.tables`
- `information_schema.columns`
- `information_schema.table_constraints`
- `information_schema.key_column_usage`
- `information_schema.constraint_column_usage`
- `pg_catalog.pg_indexes`

---

## Phase 4 — Semantic Layer

The semantic layer is the most important part of the project.

It prevents the AI from guessing wrong table names and allows business users to ask questions using business terms.

### 8. Define Semantic Model

Create `internal/semantic/model.go`.

Core entities:

```go
type SemanticModel struct {
    ID           string
    DatasourceID string
    Name         string
    BaseSchema   string
    BaseTable    string
    Dimensions   []Dimension
    Metrics      []Metric
    Joins        []Join
    Synonyms     []Synonym
}

type Dimension struct {
    Name       string
    Column     string
    Type       DimensionType
    Label      string
    Synonyms   []string
}

type Metric struct {
    Name        string
    Expression  string
    Aggregation AggregationType
    Label       string
    Synonyms    []string
}

type Join struct {
    Name          string
    FromTable     string
    FromColumn    string
    ToTable       string
    ToColumn      string
    Relationship  string
}
```

### 9. Add Semantic Layer CRUD API

Endpoints:

- [x] `POST /api/semantic/models`
- [x] `GET /api/semantic/models`
- [x] `GET /api/semantic/models/{id}`
- [x] `PUT /api/semantic/models/{id}`
- [x] `DELETE /api/semantic/models/{id}`
- [x] `POST /api/semantic/models/{id}/dimensions`
- [x] `POST /api/semantic/models/{id}/metrics`
- [x] `POST /api/semantic/models/{id}/joins`

### 10. Add Semantic Resolver

Create `internal/semantic/resolver.go`.

Responsibilities:

- [x] Resolve business names to physical columns
- [x] Resolve aliases and synonyms
- [x] Validate metric expressions
- [x] Validate dimension references
- [x] Determine required joins
- [x] Build available schema context for AI prompts

---

## Phase 5 — Logical Query Model

The system should not expose raw SQL as the main abstraction.

Create an internal database-independent query representation.

### 11. Define LogicalQuery

Create `internal/query/logical.go`.

```go
type LogicalQuery struct {
    DatasourceID string       `json:"datasource_id"`
    ModelID      string       `json:"model_id"`
    Select       []SelectItem `json:"select"`
    Filters      []Filter     `json:"filters"`
    GroupBy      []GroupBy    `json:"group_by"`
    OrderBy      []OrderBy    `json:"order_by"`
    Limit        int          `json:"limit"`
    Offset       int          `json:"offset"`
}

type SelectItem struct {
    Type  string `json:"type"` // dimension | metric
    Name  string `json:"name"`
    Alias string `json:"alias,omitempty"`
}

type Filter struct {
    Field    string      `json:"field"`
    Operator string      `json:"operator"`
    Value    any `json:"value"`
}

type GroupBy struct {
    Field string `json:"field"`
}

type OrderBy struct {
    Field     string `json:"field"`
    Direction string `json:"direction"` // asc | desc
}
```

Supported operators:

- [x] `eq`
- [x] `neq`
- [x] `gt`
- [x] `gte`
- [x] `lt`
- [x] `lte`
- [x] `in`
- [x] `not_in`
- [x] `contains`
- [x] `starts_with`
- [x] `ends_with`
- [x] `between`
- [x] `is_null`
- [x] `is_not_null`

### 12. Validate LogicalQuery

Create `internal/query/validator.go`.

Validation rules:

- [x] Datasource must exist
- [x] Semantic model must exist
- [x] Selected fields must exist in semantic layer
- [x] Filter fields must exist
- [x] Operators must be allowed by field type
- [x] Limit must not exceed configured max rows
- [x] Offset must be non-negative
- [x] Order fields must be selected or valid
- [x] User must have permission for datasource/model/fields
- [x] Query must be read-only

---

## Phase 6 — SQL Compiler

### 13. Define Compiler Interface

Create `internal/query/compiler.go`.

```go
type Compiler interface {
    Compile(ctx context.Context, q LogicalQuery, model semantic.SemanticModel) (*CompiledQuery, error)
}

type CompiledQuery struct {
    SQL  string
    Args []any
}
```

### 14. Implement SQL Builder

The compiler should:

- [x] Build SELECT clause
- [x] Build FROM clause
- [x] Build JOIN clauses
- [x] Build WHERE clause
- [x] Build GROUP BY clause
- [x] Build ORDER BY clause
- [x] Build LIMIT/OFFSET
- [x] Use placeholders, never string-concatenate values
- [x] Quote identifiers using dialect implementation
- [x] Prevent SQL injection by separating identifiers and values

Example compiled SQL:

```sql
SELECT
    "customers"."country" AS "country",
    COUNT("orders"."id") AS "order_count"
FROM "orders"
LEFT JOIN "customers" ON "orders"."customer_id" = "customers"."id"
WHERE "orders"."created_at" >= $1
GROUP BY "customers"."country"
ORDER BY "order_count" DESC
LIMIT 100
```

Arguments:

```text
["2026-01-01"]
```

### 15. Implement Dialect Layer

Create:

```text
internal/dialect/dialect.go
internal/dialect/postgres.go
internal/dialect/mysql.go
internal/dialect/sqlserver.go
internal/dialect/clickhouse.go
```

Dialect responsibilities:

- [x] Identifier quoting
- [x] Placeholder format
- [x] LIMIT/OFFSET syntax
- [x] Date truncation syntax
- [x] String comparison syntax
- [x] Case-insensitive search syntax
- [x] Aggregation differences
- [x] Type casting differences

Examples:

PostgreSQL placeholder:

```go
func (d PostgresDialect) Placeholder(i int) string {
    return fmt.Sprintf("$%d", i)
}
```

MySQL placeholder:

```go
func (d MySQLDialect) Placeholder(i int) string {
    return "?"
}
```

SQL Server placeholder:

```go
func (d SQLServerDialect) Placeholder(i int) string {
    return fmt.Sprintf("@p%d", i)
}
```

---

## Phase 7 — Safe Query Execution

### 16. Implement Query Executor

Create `internal/query/executor.go`.

Responsibilities:

- [x] Open datasource connection
- [x] Apply context timeout
- [x] Execute compiled SQL
- [x] Read columns dynamically
- [x] Scan rows into generic result format
- [x] Enforce max rows
- [x] Cancel long-running queries
- [x] Return structured query result

Result model:

```go
type QueryResult struct {
    Columns []ResultColumn `json:"columns"`
    Rows    [][]any        `json:"rows"`
    Stats   QueryStats     `json:"stats"`
}

type ResultColumn struct {
    Name string `json:"name"`
    Type string `json:"type"`
}

type QueryStats struct {
    DurationMs int64 `json:"duration_ms"`
    RowCount    int `json:"row_count"`
}
```

### 17. Add Read-Only Protection

Create `internal/security/readonly.go`.

Rules:

- [x] Only allow SELECT queries
- [x] Reject semicolons in compiled SQL unless explicitly controlled
- [x] Reject multiple statements
- [x] Reject unsafe keywords:
  - INSERT
  - UPDATE
  - DELETE
  - DROP
  - ALTER
  - TRUNCATE
  - CREATE
  - GRANT
  - REVOKE
  - MERGE
  - CALL
  - EXEC
- [x] Prefer connecting with a database user that only has read permissions
- [x] Add statement timeout at DB connection/session level where possible

Important: read-only DB credentials are not optional. They are the main safety layer.

---

## Phase 8 — Query API

### 18. Add Query Endpoints

Create `internal/http/handlers/query.go`.

Endpoints:

- [x] `POST /api/query/compile`
- [x] `POST /api/query/run`
- [x] `POST /api/query/explain`
- [x] `GET /api/query/history`
- [x] `GET /api/query/history/{id}`

Persistence:

- [x] Write successful `/api/query/run` executions to `query_history`
- [x] Write failed `/api/query/run` compile/execute attempts to `query_history`
- [x] Return recent query history from `/api/query/history`
- [x] Return a single query history row from `/api/query/history/{id}`

Example request:

```json
{
  "datasource_id": "ds_123",
  "model_id": "orders",
  "select": [
    {
      "type": "dimension",
      "name": "country"
    },
    {
      "type": "metric",
      "name": "order_count"
    }
  ],
  "filters": [
    {
      "field": "created_at",
      "operator": "gte",
      "value": "2026-01-01"
    }
  ],
  "group_by": [
    {
      "field": "country"
    }
  ],
  "order_by": [
    {
      "field": "order_count",
      "direction": "desc"
    }
  ],
  "limit": 100
}
```

---

## Phase 9 — Datasource API

### 19. Add Datasource Management

Create `internal/http/handlers/datasources.go`.

Endpoints:

- [x] `POST /api/datasources`
- [x] `GET /api/datasources`
- [x] `GET /api/datasources/{id}`
- [x] `DELETE /api/datasources/{id}`
- [x] `POST /api/datasources/{id}/test`
- [x] `POST /api/datasources/{id}/sync-metadata`
- [x] `GET /api/datasources/{id}/tables` (browse synced tables, optional `?schema=`)
- [x] `GET /api/datasources/{id}/columns` (optional `?schema=&table=`)

The `sync-metadata` endpoint introspects the source database and stores schemas,
tables, columns, and relations in the metadata DB. Native database comments
(`pg_description` for PostgreSQL via `obj_description` / `col_description`) are
imported into the `description` column on tables and columns. Re-syncing
preserves existing descriptions when the source has no native comment, so
manually edited or AI-generated descriptions are not erased.

### 20. Encrypt Datasource Credentials

Create `internal/security/credentials.go`.

- [x] Encrypt DSN before storing
- [x] Decrypt only at runtime
- [x] Never log DSNs
- [x] Mask credentials in API responses
- [x] Rotate encryption key strategy later

---

## Phase 10 — AI Text-to-Query Layer

### 21. Define AI Output Schema

Create `internal/ai/schema.go`.

The AI must output strict JSON matching `LogicalQuery`.

Never ask the AI to directly produce executable SQL in v1.

Example AI output:

```json
{
  "datasource_id": "ds_123",
  "model_id": "orders",
  "select": [
    {
      "type": "dimension",
      "name": "country"
    },
    {
      "type": "metric",
      "name": "total_revenue"
    }
  ],
  "filters": [
    {
      "field": "created_at",
      "operator": "between",
      "value": ["2026-01-01", "2026-01-31"]
    }
  ],
  "group_by": [
    {
      "field": "country"
    }
  ],
  "order_by": [
    {
      "field": "total_revenue",
      "direction": "desc"
    }
  ],
  "limit": 50
}
```

### 22. Build AI Prompt Context

Create `internal/ai/prompt.go`.

Prompt should include:

- [x] User question
- [x] Available semantic models
- [x] Available dimensions
- [x] Available metrics
- [x] Allowed filters
- [x] Synonyms
- [x] Current date/time
- [x] Hard instruction: output JSON only
- [x] Hard instruction: do not invent fields
- [x] Hard instruction: use only provided semantic layer fields

### 23. Add AI Query Endpoint

Create `internal/http/handlers/ai.go`.

Endpoints:

- [x] `POST /api/ai/query`
- [x] `POST /api/ai/query/preview`
- [x] `POST /api/ai/query/run`
- [x] `POST /api/ai/metadata/describe` (sample rows → AI-generated table/column descriptions)

Flow:

1. User asks natural language question.
2. Backend loads semantic context.
3. Backend sends prompt to LLM.
4. LLM returns `LogicalQuery` JSON.
5. Backend validates JSON.
6. Backend compiles SQL.
7. Backend optionally returns preview.
8. Backend executes only after validation.

Request:

```json
{
  "datasource_id": "ds_123",
  "model_id": "orders",
  "question": "Show total revenue by country for January 2026"
}
```

Response:

```json
{
  "logical_query": {},
  "sql": "SELECT ...",
  "args": [],
  "warnings": [],
  "result": {}
}
```

### 24. Add AI Guardrails

- [x] Reject unknown fields
- [x] Reject unknown metrics
- [x] Reject ambiguous questions
- [x] Ask clarification when required
- [x] Never execute AI query automatically without validation
- [x] Add confidence score
- [x] Store AI prompt and response in `ai_query_history`
- [x] Mask sensitive metadata
- [x] Add per-user rate limit

### 24.5 Add Automatic Table Routing When Semantic Model Is Empty

When the user does not select a semantic model or table scope, the backend should
decide which table or tables are relevant to the natural language question before
building the AI prompt context.

Required behavior:

- [x] Treat `model_id` / semantic model selection as optional in AI query requests.
- [x] Build a datasource metadata catalog from introspected schemas, tables,
  columns, relations, indexes, descriptions, and lightweight table statistics.
- [x] Generate a compact "table card" for each table, including:
  - `schema.table`
  - table description
  - column names and types
  - primary keys and foreign keys
  - known joins / relations
  - synonyms and business keywords
- [x] Add a metadata-driven table retrieval step that combines:
  - keyword matching over table cards
  - synonym matching over business terms
  - exact matches on table, column, metric, and entity names
  - foreign-key graph expansion for related tables
- [x] Score candidate tables and return top matches with confidence.
- [x] Automatically continue when confidence is high.
- [x] Ask for clarification when confidence is low or multiple unrelated table
  groups match the question.
- [x] Pass only selected tables, allowed columns, and join paths into the LLM
  prompt to reduce hallucination risk.
- [ ] Persist user confirmations/corrections so future routing can learn from
  repeated choices.

Completed implementation notes:

- [x] Added `internal/ai/table_router.go`
- [x] Added synthetic semantic model generation for routed tables
- [x] Added `COUNT(*)` support in dialect aggregation for auto-generated row count metrics
- [x] Added routing metadata to AI responses (`selected_tables`, `join_paths`, candidates, confidence)
- [x] Persist AI query, preview, run, and clarification attempts to `ai_query_history`

Suggested flow:

```text
User question
  ↓
Datasource metadata catalog
  ↓
Hybrid table retrieval
  ↓
Foreign-key join expansion
  ↓
Confidence check
  ↓
LogicalQuery generation
```

Example AI request without manual semantic model:

```json
{
  "datasource_id": "ds_123",
  "question": "Show last month's revenue by customer"
}
```

Example router result:

```json
{
  "selected_tables": ["public.orders", "public.customers"],
  "join_paths": [
    "public.orders.customer_id = public.customers.id"
  ],
  "confidence": 0.91
}
```

---

## Phase 11 — Query Planner

### 25. Add Basic Query Planner

Create `internal/query/planner.go`.

Responsibilities:

- [x] Determine needed base table
- [x] Determine required joins
- [x] Prevent fanout issues
- [x] Prevent invalid metric/dimension combinations
- [x] Validate relationship cardinality
- [x] Add warnings for risky joins
- [x] Add default date filters if needed

Important concepts:

- one-to-one
- one-to-many
- many-to-one
- many-to-many
- fanout
- aggregate awareness

---

## Phase 11.5 — Composite Semantic Models

> Composite model, semantic layer ile query planner arasina kurgulanir.
> Raw metadata tarafinda degil, AI prompt icinde rastgele degil, SQL compiler
> icinde de sonradan uydurularak degil.
>
> WrenAI'deki semantic context yaklasimindan ilham alir; ancak Biqly'de Go +
> LogicalQuery-first mimari korunur. Composite model bir SQL view veya ayri
> query engine degildir.

Composite model, birden fazla base semantic modelin kontrollu sekilde bir araya
getirilmesiyle olusan business-level virtual dataset'tir. AI'nin ham tablolar
veya rastgele join path'ler uzerinden calismasi yerine, oncedan tanimlanmis,
validate edilmis ve publish edilmis is baglamlari uzerinden LogicalQuery uretir.

### 34. Extend LogicalQuery for Composite Models

Update `internal/query/logical.go`.

```go
type LogicalQuery struct {
    DatasourceID     string       `json:"datasource_id"`
    ModelID          string       `json:"model_id,omitempty"`
    CompositeModelID string       `json:"composite_model_id,omitempty"`
    Select           []SelectItem `json:"select"`
    Filters          []Filter     `json:"filters"`
    GroupBy          []GroupBy    `json:"group_by"`
    OrderBy          []OrderBy    `json:"order_by"`
    Limit            int          `json:"limit"`
    Offset           int          `json:"offset"`
}
```

Validation rules:

- [ ] `model_id` or `composite_model_id` — only one may be set.
- [ ] Both set → reject.
- [ ] Neither set → TableRouter decides.
- [ ] `composite_model_id` set → select/filter/group/order fields must be
      exposed in the composite model.
- [ ] Composite model must be published; draft → reject.

### 35. Add Composite Model Database Tables

Create migrations for:

- [ ] `semantic_composite_models`
  - `id UUID PK`
  - `datasource_id UUID FK → datasources(id)`
  - `name TEXT NOT NULL`
  - `label TEXT`
  - `description TEXT`
  - `version INT NOT NULL DEFAULT 1`
  - `status TEXT NOT NULL DEFAULT 'draft'` (draft | published | archived)
  - `published_at TIMESTAMPTZ`
  - `published_by UUID`
  - `created_at TIMESTAMPTZ`
  - `updated_at TIMESTAMPTZ`

- [ ] `semantic_composite_model_members`
  - `id UUID PK`
  - `composite_model_id UUID FK → semantic_composite_models(id)`
  - `semantic_model_id UUID FK → semantic_models(id)`
  - `role TEXT NOT NULL DEFAULT 'related'` (base | related | optional)
  - `alias TEXT`
  - `required BOOLEAN NOT NULL DEFAULT false`

- [ ] `semantic_composite_fields`
  - `id UUID PK`
  - `composite_model_id UUID FK → semantic_composite_models(id)`
  - `source_model_id UUID FK → semantic_models(id)`
  - `source_field_name TEXT NOT NULL`
  - `field_type TEXT NOT NULL` (dimension | metric)
  - `exposed_name TEXT NOT NULL`
  - `label TEXT`
  - `description TEXT`
  - `is_default BOOLEAN NOT NULL DEFAULT false`

- [ ] `semantic_composite_joins`
  - `id UUID PK`
  - `composite_model_id UUID FK → semantic_composite_models(id)`
  - `from_model_id UUID FK → semantic_models(id)`
  - `to_model_id UUID FK → semantic_models(id)`
  - `join_name TEXT NOT NULL`
  - `join_type TEXT NOT NULL DEFAULT 'left'`
  - `relationship TEXT NOT NULL` (one_to_one | many_to_one | one_to_many | many_to_many)
  - `required BOOLEAN NOT NULL DEFAULT false`
  - `risk_level TEXT NOT NULL DEFAULT 'low'`

### 36. Add Composite Model CRUD and Publish API

Create:

```text
internal/semantic/composite_model.go
internal/semantic/composite_repository.go
internal/semantic/composite_resolver.go
internal/semantic/composite_validator.go
```

Endpoints:

- [ ] `POST /api/semantic/composite-models`
- [ ] `GET /api/semantic/composite-models`
- [ ] `GET /api/semantic/composite-models/{id}`
- [ ] `PUT /api/semantic/composite-models/{id}`
- [ ] `DELETE /api/semantic/composite-models/{id}`
- [ ] `POST /api/semantic/composite-models/{id}/members`
- [ ] `POST /api/semantic/composite-models/{id}/fields`
- [ ] `POST /api/semantic/composite-models/{id}/joins`
- [ ] `POST /api/semantic/composite-models/{id}/validate`
- [ ] `POST /api/semantic/composite-models/{id}/publish`
- [ ] `POST /api/semantic/composite-models/{id}/rollback`

### 37. Add Composite Resolver

Create `internal/semantic/composite_resolver.go`.

Responsibilities:

- [ ] Load published active version of composite model.
- [ ] Resolve member semantic models.
- [ ] Produce exposed dimension and metric list.
- [ ] Resolve source field → exposed field mapping:

```text
orders.total_revenue    → total_revenue
customers.country       → customer_country
products.category       → product_category
sales_reps.name         → sales_rep_name
orders.created_at       → order_date
```

- [ ] Provide approved join paths to query planner.
- [ ] Apply field-level permissions before producing context.
- [ ] Send only permitted fields to AI prompt.

### 38. Extend Query Planner for Composite Models

Update `internal/query/planner.go`.

- [ ] Determine required base models from selected composite fields.
- [ ] Use only composite-approved join paths.
- [ ] Reject if join path is missing.
- [ ] Validate relationship cardinality.
- [ ] Warn or reject on aggregation fanout risk.
- [ ] Reject many-to-many metric aggregation if unsafe.
- [ ] Auto-include required joins.
- [ ] Conditionally include optional joins.

### 39. Extend AI Context for Composite Models

Update `internal/ai/prompt.go`.

- [ ] Include composite models in prompt context.
- [ ] Format composite model as structured context:

```json
{
  "type": "composite_model",
  "name": "sales_performance",
  "description": "Sales, customer, product and revenue analysis context",
  "dimensions": ["customer_name", "customer_country", "product_category",
                  "sales_rep_name", "order_date"],
  "metrics": ["total_revenue", "order_count", "average_order_value"],
  "allowed_filters": ["order_date", "customer_country", "product_category",
                       "sales_rep_name"]
}
```

- [ ] AI cannot see fields outside the composite model context.
- [ ] AI cannot use table/column names outside the composite model.
- [ ] Return clarification question if composite context confidence is low.
- [ ] Include composite model in debug trace response.

Example AI output using composite model:

```json
{
  "datasource_id": "ds_123",
  "composite_model_id": "sales_performance",
  "select": [
    {"type": "dimension", "name": "customer_country"},
    {"type": "metric", "name": "total_revenue"}
  ],
  "group_by": [{"field": "customer_country"}],
  "order_by": [{"field": "total_revenue", "direction": "desc"}],
  "limit": 50
}
```

### 40. Extend TableRouter for Composite Context

Update `internal/ai/table_router.go`.

- [ ] Add composite semantic model as candidate type alongside base semantic
      model and raw table group.
- [ ] Priority: published composite → published base → auto-generated raw.
- [ ] Return routing metadata:

```json
{
  "selected_context_type": "composite_model",
  "selected_context_name": "sales_performance",
  "selected_models": ["orders", "customers", "products"],
  "selected_fields": ["customer_country", "total_revenue"],
  "join_paths": [
    "orders.customer_id = customers.id",
    "orders.product_id = products.id"
  ],
  "confidence": 0.91,
  "ranking_method": "keyword+embedding+semantic_model"
}
```

### 41. Composite Model Validation and Publish Flow

Pre-publish checks:

- [ ] Referenced base semantic models exist.
- [ ] Base semantic models are published.
- [ ] Exposed fields exist in source models.
- [ ] No duplicate exposed field names.
- [ ] Metric expression references are valid.
- [ ] Join paths are compatible with semantic join metadata.
- [ ] Relationship cardinality is safe for aggregation.
- [ ] Many-to-many metric aggregation is safe.
- [ ] Permission policy references are valid.
- [ ] Prompt/context size is reasonable.
- [ ] At least one dimension or metric is exposed.

Runtime rules:

- [ ] Draft composite model does not affect query runtime.
- [ ] Only published composite model is usable.
- [ ] Published version recorded in query history and audit log.
- [ ] Erroneous composite join/metric caught at publish stage.
- [ ] Previous published version retained for rollback.

### 42. Composite Model Permission Integration

- [ ] Composite model visibility governed by user permissions.
- [ ] If user cannot access a base model, composite model is unavailable or
      restricted fields are removed.
- [ ] Field-level permission applied at composite exposed field level.
- [ ] Disallowed fields never enter AI prompt.
- [ ] Row-level filters safely injected by compiler.
- [ ] Audit log: composite_model_id, composite_model_version,
      selected_base_models, selected_fields, selected_join_paths,
      permission_decision, query_fingerprint.

### 43. Composite Model Tests

Unit tests:

- [ ] Composite model resolver tests.
- [ ] Composite field mapping tests.
- [ ] Composite join validation tests.
- [ ] Duplicate exposed field validation tests.
- [ ] model_id / composite_model_id conflict validation tests.
- [ ] Disallowed field excluded from prompt context test.
- [ ] Fanout risk validator tests.

Golden SQL tests:

- [ ] Composite model simple select.
- [ ] Composite model dimension + metric group by.
- [ ] Composite model filter.
- [ ] Composite model order by metric.
- [ ] Composite model multi-join query.
- [ ] Risky many-to-many aggregation rejection.

Eval tests:

- [ ] "monthly revenue by customer country" → sales_performance composite model.
- [ ] "top products by revenue" → products + orders path.
- [ ] "customer payment status" → customer_360 or finance context.
- [ ] Ambiguous context → clarification question.

---

## Phase 12 — Permissions

### 26. Add Permission Model

Create `internal/security/permissions.go`.

Permission levels:

- [x] Datasource-level
- [x] Schema-level
- [x] Table-level
- [x] Column-level
- [x] Metric-level
- [x] Row-level security

Example:

```go
type PermissionPolicy struct {
    UserID       string
    DatasourceID string
    AllowedModels []string
    DeniedFields  []string
    RowFilters    []Filter
}
```

### 27. Apply Permissions Before Compile

- [x] Load user permission policy
- [x] Remove inaccessible fields from semantic context
- [x] Reject queries with denied fields
- [x] Inject mandatory row filters
- [x] Store permission decision in audit logs

---

## Phase 13 — Caching

### 28. Add Query Cache

- [x] Generate cache key from:
  - datasource ID
  - model ID
  - logical query JSON
  - user permission scope
- [x] Store query result in Redis/Dragonfly
- [x] Add TTL per datasource/model
- [x] Add cache bypass option
- [x] Invalidate cache after metadata sync

---

## Phase 14 — Observability

### 29. Add Logging

Use `slog`.

Log:

- [x] request ID
- [x] user ID
- [x] datasource ID
- [x] model ID
- [x] query duration
- [x] row count
- [x] errors
- [x] AI provider latency

Never log:

- [x] raw credentials
- [x] full DSN
- [x] sensitive query values
- [x] tokens
- [x] AI API keys

### 30. Add Metrics

Expose `/metrics`.

Metrics:

- [x] total queries
- [x] query duration histogram
- [x] query errors
- [x] datasource connection errors
- [x] AI requests
- [x] AI latency
- [x] validation failures
- [x] cache hit/miss

---

## Phase 14.5 — Metadata Description Workflow

The system gives the user three layers for filling in `tables.description` and
`columns.description`. They compose: native comments seed the field, the user
can edit at any time, and AI can fill the gaps.

### M1. Auto-import from source DB on sync

- [x] PostgreSQL: read `pg_class` for `obj_description` (table comments)
- [x] PostgreSQL: read `col_description(...)` per column ordinal
- [ ] MySQL: read `information_schema.tables.table_comment` and `columns.column_comment`
- [ ] SQL Server: read `sys.extended_properties` (`MS_Description`)
- [ ] ClickHouse: read `system.tables.comment` / `system.columns.comment`
- [x] On `POST /api/datasources/{id}/sync-metadata`, store comments into `tables.description` / `columns.description`
- [x] Repository upserts must preserve existing description when the source comment is empty (`COALESCE(NULLIF(EXCLUDED.description,''), tables.description)`)

### M2. Manual editing endpoints

- [x] `GET /api/datasources/{id}/tables?schema=`
- [x] `GET /api/datasources/{id}/columns?schema=&table=`
- [x] `PATCH /api/metadata/tables/{id}` body `{"description": "..."}`
- [x] `PATCH /api/metadata/columns/{id}` body `{"description": "..."}`

The PATCH handlers accept `null` to clear the field. Updates set only the
description column; introspected metadata (data type, nullability, FK refs) is
never mutated by these endpoints — that path is reserved for `sync-metadata`.

### M3. AI-suggested descriptions from sample data

Endpoint: `POST /api/ai/metadata/describe`.

Request:

```json
{
  "datasource_id": "ds_123",
  "schema": "public",
  "table": "orders",
  "sample_size": 10,
  "auto_apply": false
}
```

Flow:

1. Load column metadata for the requested table (must already be synced).
2. Open the user's source database and `SELECT` up to `sample_size` rows
   (defaults to 10) using identifiers that pass an allowlist regex
   `^[A-Za-z_][A-Za-z0-9_$]*$`. Names that fail the allowlist are rejected
   before any SQL is built, since SQL placeholders cannot bind identifiers.
3. Build a prompt that contains the column list (with PK/FK hints + any
   existing description) plus the sample rows as JSON.
4. The LLM responds with `{"table_description": "...", "columns": [...]}`.
5. If `auto_apply` is true the suggestions are written back via the same
   repository methods used by manual editing; otherwise the response is
   returned for the user to review and PATCH selectively.

Guardrails:

- [x] Identifier allowlist before any sample query is built
- [x] Dialect quoting escapes embedded double quotes (defense in depth)
- [x] Sample query routes through the regular driver pool with the configured
      query timeout
- [x] AI response is parsed defensively (markdown fences stripped, missing
      keys tolerated)
- [x] `auto_apply` defaults to false so AI never silently overwrites human edits

Response shape:

```json
{
  "schema": "public",
  "table": "orders",
  "description": "Customer order header. One row per checkout.",
  "columns": [
    {"name": "id", "description": "Order primary key (UUID)."},
    {"name": "customer_id", "description": "FK to customers.id."}
  ],
  "applied": false,
  "sample_rows": 10
}
```

---

## Phase 15 — Frontend Later

Frontend is not required for v1, but the backend should support it.

Suggested future frontend:

- React + Vite
- HTMX if keeping it simple
- Table preview
- Chart builder
- Query builder UI
- Semantic model editor
- Datasource setup wizard
- AI question box
- Datasource-aware table selector for AI question scope

### 30.5 Add Datasource Table Selector UI

The AI question form should not require users to type semantic model or table
names manually. After a datasource is selected, the UI should list available
tables from that datasource and allow users to optionally constrain the AI query
scope.

Required behavior:

- [x] Add a backend endpoint for listing queryable tables for a datasource, for
  example `GET /api/datasources/{id}/tables`.
- [x] Return tables in `schema.table` format with enough metadata for display
  and search.
- [x] Replace the free-text Semantic Model input with a searchable select or
  multi-select combobox.
- [x] Allow no manual table selection; in that case the backend uses automatic
  table routing.
- [x] Allow one or more manual table selections; in that case the backend limits
  AI routing and prompt context to those tables.
- [x] Display labels as `schema.table`, and optionally show descriptions and
  key columns in the dropdown.
- [x] Disable the table selector until a datasource is selected.
- [x] Refresh the table list after datasource metadata sync.

Example table listing response:

```json
[
  {
    "schema": "public",
    "name": "orders",
    "label": "public.orders",
    "description": "Customer order records",
    "columns": ["id", "customer_id", "created_at", "total_amount"]
  }
]
```

Initial backend-first endpoints are enough.

---

## Phase 16 — Testing Strategy

### 31. Unit Tests

- [x] LogicalQuery validation
- [x] SQL compiler
- [x] Dialect quoting
- [x] Placeholder generation
- [x] Filter compiler
- [x] Permission injection
- [x] AI JSON schema validation
- [x] Automatic table routing confidence scoring
- [x] Table selector request/response mapping

### 32. Integration Tests

Use Docker Compose test databases:

- [x] PostgreSQL
- [x] MySQL
- [x] SQL Server
- [x] ClickHouse

Test:

- [x] datasource connection
- [x] introspection
- [x] compile query
- [x] execute query
- [x] compare result
- [x] list datasource tables for frontend selection
- [x] AI query without `model_id` routes to expected table set

### 33. Golden SQL Tests

Create golden files:

```text
testdata/sql/postgres/simple_select.sql
testdata/sql/postgres/group_by_metric.sql
testdata/sql/mysql/simple_select.sql
testdata/sql/sqlserver/simple_select.sql
```

Test that compiler output matches expected SQL.

---

## MVP Scope

Do not build everything at once.

### MVP 1

- [x] PostgreSQL datasource only
- [x] Metadata sync
- [x] Semantic model manually created via API
- [x] LogicalQuery JSON API
- [x] SQL compiler for PostgreSQL
- [x] Query execution
- [x] Query history

### MVP 2

- [x] AI natural language to LogicalQuery
- [x] Prompt context from semantic layer
- [x] AI preview endpoint
- [x] AI validation and guardrails
- [x] Automatic table routing when semantic model/table scope is empty

### MVP 3

- [x] MySQL driver
- [x] SQL Server driver
- [x] ClickHouse driver
- [x] Query cache
- [x] Permissions
- [x] Observability

### MVP 4

- [x] Frontend query builder
- [x] Dashboard
- [x] Charts
- [x] Saved questions
- [x] Scheduled reports
- [x] Searchable datasource table selector for AI question scope

---

## Phase 11.6 — Reviewer Roadmap Additions

External review (kasım 2026) raporu sonrası eklenen kalemler. Roadmap'in
çekirdek omurgası (`LogicalQuery → Validate → Compile → Execute` + semantic
context + AI text-to-LogicalQuery) doğru kabul edildi; aşağıdaki maddeler hem
"eksik gördüğüm noktalar" hem de "ileride patlamayı önleyen küçük eklentiler"
olarak işaretlendi.

### R1. LogicalQuery Versioning

`LogicalQuery` ileride büyüyecek. Eval, replay ve history için şimdiden version
alanı taşımak gerekiyor.

- [ ] `LogicalQuery` struct'ına `Version string` alanı ekle (`omitempty`).
- [ ] Mevcut sorgular için varsayılan `v1`.
- [ ] Query history ve audit log içine LogicalQuery version alanını yaz.
- [ ] Eval/replay testleri version'a göre filtrelenebilsin.

```go
type LogicalQuery struct {
    Version string `json:"version,omitempty"`
    // ...
}
```

### R2. Time Grain Support

BI sorgularında çok gerekli. Şu anda `GroupBy` sadece field içeriyor.

- [ ] `GroupBy` struct'ını `TimeGrain` alanı ile genişlet
  (`day | week | month | quarter | year`).
- [ ] Dialect bazlı date truncation (PostgreSQL `date_trunc`, MySQL `DATE_FORMAT`,
  SQL Server `DATETRUNC`, ClickHouse `toStartOf*`).
- [ ] AI prompt mapping: "daily", "weekly", "monthly", "yearly".
- [ ] Golden SQL test her dialect için ayrı.

```go
type GroupBy struct {
    Field     string `json:"field"`
    TimeGrain string `json:"time_grain,omitempty"`
}
```

### R3. HAVING Support

Şu an WHERE filtreleri var ama "100'den fazla siparişi olan müşteriler" gibi
metric-level filter HAVING gerektirir.

- [ ] LogicalQuery'ye `Having []Filter` alanı ekle.
- [ ] Validator HAVING field'larının metric olmasını zorunlu kılsın.
- [ ] Compiler GROUP BY'dan sonra HAVING üretsin.
- [ ] Golden SQL testleri.

### R4. Clarification Response Model

AI veya router belirsiz kaldığında düz error dönmek yerine yapılandırılmış
response dönmeli.

- [ ] AI response için `status: needs_clarification` modeli.
- [ ] Options listesi (`gross_revenue` vs `net_revenue` gibi).
- [ ] Reason ve aday context listesi.
- [ ] AI history'ye clarification denemesi olarak yazılsın.
- [ ] UI bu structured response'u soru olarak gösterebilsin.

```json
{
  "status": "needs_clarification",
  "question": "Revenue derken gross mu net mi?",
  "options": [
    {"key": "gross_revenue", "label": "Gross revenue"},
    {"key": "net_revenue",  "label": "Net revenue"}
  ],
  "reason": "Multiple revenue metrics matched the question."
}
```

### R5. Query Fingerprint

Audit ve cache için çok değerli. Aynı sorguları gruplayabilmek lazım.

- [ ] Fingerprint = normalize(LogicalQuery JSON) + `datasource_id` +
  `context_version` + `permission_scope` hash.
- [ ] Query history ve audit log'a yazılsın.
- [ ] Cache key olarak da kullanılsın.

### R6. Semantic Context Size Budget

Prompt büyümesini erken kontrol altına almak için her context'e bütçe.

- [ ] Composite/base modelde `max_models`, `max_dimensions`, `max_metrics`,
  `max_joins`, `max_prompt_chars` limitleri.
- [ ] Validate aşamasında budget aşılırsa uyar.
- [ ] Prompt builder budget'ı zorla.

```text
max_models: 3
max_dimensions: 80
max_metrics: 40
max_joins: 30
max_prompt_chars: 20000
```

### R7. Chart-Ready Result Metadata

Result modeli sade; frontend için biraz daha bilgi şart.

- [ ] `ResultColumn`'a `semantic_type` (dimension | metric), `format`
  (currency | percent | date | number) ekle.
- [ ] Result envelope'una `chart_suggestions []string` (örn. `["bar", "table"]`).
- [ ] Frontend'in tip seçimini kolaylaştırır.

```json
{
  "columns": [
    {"name": "country", "type": "string", "semantic_type": "dimension"},
    {"name": "total_revenue", "type": "number",
     "semantic_type": "metric", "format": "currency"}
  ],
  "rows": [],
  "chart_suggestions": ["bar", "table"]
}
```

---

## Phase 11.7 — Design Risks (Reviewer)

Bu maddeler kod yazılırken kasten korunması gereken sınırlar. MVP üretimini
yavaşlatan değil, hızlı büyürken yanlış soyutlama açılmasını engelleyen kurallar.

### Risk 1 — Metric expression güvenliği

Metric `expression` alanı serbest SQL olursa `SUM(CASE WHEN ...)` veya kötü
durumda `DROP TABLE ...` yazılabilir. İlk sürümde serbest SQL yok.

- [ ] Metric tanımı `source_column` + `aggregation` ikilisinden ibaret olsun:

```json
{
  "name": "total_revenue",
  "source_column": "orders.total_amount",
  "aggregation": "sum"
}
```

- [ ] Calculated metric gerekirse kontrollü AST üzerinden tanımlansın
  (ör: `divide(total_revenue, order_count)`); serbest SQL en geç gelmeli.
- [ ] Validator: aggregation enum (`sum|avg|min|max|count|count_distinct`),
  source_column semantic layer'da tanımlı olmalı.

### Risk 2 — Join fanout

BI'da en çok yanlış sonuç üreten problem. Relationship metadata sadece bilgi
değil, query planner input'u.

- [ ] Cardinality kuralları:
  - `many_to_one`: güvenli
  - `one_to_many`: metric aggregation için warn veya reject
  - `many_to_many` + metric: reject
- [ ] Planner sessizce yanlış JOIN üretmesin; risk seviyesi response'a yazılsın.

### Risk 3 — Permission tüm katmanlarda uygulanmalı

Permission sadece endpoint seviyesinde değil; aşağıdaki tüm katmanlar permission
filtresinden geçmiş context kullanmalı:

- [ ] Semantic context builder
- [ ] AI prompt builder (permission dışı field prompt'a hiç girmemeli)
- [ ] LogicalQuery validator
- [ ] SQL compiler (row-level filter inject)
- [ ] Audit log (permission decision dahil)

### Risk 4 — TableRouter erken karmaşıklaşıyor

İlk router için embedding/few-shot/feedback loop zorunlu değil. Şu sıra yeter:

```text
1. Exact match
2. Synonym match
3. Column/table description keyword match
4. Semantic model name/label match
5. FK neighbor expansion
```

Embedding sonra eklenir; erken eklemek silent regression riski yaratır.

### Risk 5 — Çok erken multi-database

PostgreSQL compiler tam olgunlaşmadan diğer dialect'leri eklemek silent SQL
hatası riski getirir.

```text
PostgreSQL compiler tam olsun
  → Golden SQL otursun
  → Dialect interface stabilize olsun
  → MySQL ekle
  → SQL Server ekle
  → ClickHouse ekle
```

ClickHouse özellikle OLAP fonksiyonları, date truncation, LIMIT sözdizimi ve
aggregation davranışları farklı; en son.

---

## Reviewer Recommended Sequence

Composite model dahil tüm geniş kapsam doğru kabul edildi; fakat sıra şu:

```text
Core              : LogicalQuery, Semantic model, Compiler, Validator,
                    Executor, Read-only safety, Golden SQL tests
        ↓
AI + Routing      : Text → LogicalQuery, TableRouter (basit),
                    Feedback, Eval, Publish/version
        ↓
Composite model   : Composite semantic model, Advanced governance,
                    Dashboard, Scheduled reports, Complex frontend
```

Composite model fikri **doğru**, ama erken eklemek; LogicalQuery, validator,
planner, prompt builder, TableRouter, permission, audit, query history, UI ve
tests'i aynı anda etkilediği için sistemi gereğinden fazla karmaşıklaştırır.
Önce base semantic model + query compiler + AI flow taş gibi çalışmalı, sonra
composite eklenmeli.

---

## AI Agent Implementation Instructions

Use this section as the instruction block for an AI coding agent.

### Rules

- Implement in Go.
- Keep code readable and modular.
- Prefer interfaces for datasource drivers and dialects.
- Do not let AI-generated SQL execute directly.
- The main query abstraction is `LogicalQuery`.
- Always validate `LogicalQuery` before compilation.
- Always compile using parameterized queries.
- Never concatenate user values into SQL.
- Quote identifiers using the selected dialect.
- Use context timeouts for all DB operations.
- Do not log secrets.
- Add tests for every compiler feature.
- Add one feature at a time.

### First Coding Task

Create the initial repository structure and implement:

1. Config loader
2. Metadata PostgreSQL connection
3. Datasource driver interface
4. PostgreSQL datasource driver
5. PostgreSQL schema introspection
6. LogicalQuery structs
7. PostgreSQL SQL compiler for:
   - select dimensions
   - select metrics
   - filters
   - group by
   - order by
   - limit
8. Query execution endpoint

### Second Coding Task

Add semantic layer:

1. Semantic model structs
2. CRUD repository
3. Semantic resolver
4. LogicalQuery validation against semantic model
5. Required join resolution

### Third Coding Task

Add AI:

1. AI provider interface
2. Prompt builder
3. JSON schema for LogicalQuery
4. AI endpoint
5. AI output validator
6. Preview query endpoint

---

## Example Minimal LogicalQuery to PostgreSQL Compiler Flow

Input:

```json
{
  "datasource_id": "ds1",
  "model_id": "orders",
  "select": [
    {
      "type": "dimension",
      "name": "country"
    },
    {
      "type": "metric",
      "name": "order_count"
    }
  ],
  "filters": [
    {
      "field": "created_at",
      "operator": "gte",
      "value": "2026-01-01"
    }
  ],
  "group_by": [
    {
      "field": "country"
    }
  ],
  "order_by": [
    {
      "field": "order_count",
      "direction": "desc"
    }
  ],
  "limit": 100
}
```

Semantic model:

```json
{
  "name": "orders",
  "base_schema": "public",
  "base_table": "orders",
  "dimensions": [
    {
      "name": "country",
      "column": "customers.country"
    },
    {
      "name": "created_at",
      "column": "orders.created_at"
    }
  ],
  "metrics": [
    {
      "name": "order_count",
      "expression": "orders.id",
      "aggregation": "count"
    }
  ],
  "joins": [
    {
      "name": "orders_customers",
      "from_table": "orders",
      "from_column": "customer_id",
      "to_table": "customers",
      "to_column": "id",
      "relationship": "many_to_one"
    }
  ]
}
```

Compiled SQL:

```sql
SELECT
  "customers"."country" AS "country",
  COUNT("orders"."id") AS "order_count"
FROM "public"."orders"
LEFT JOIN "public"."customers"
  ON "orders"."customer_id" = "customers"."id"
WHERE "orders"."created_at" >= $1
GROUP BY "customers"."country"
ORDER BY "order_count" DESC
LIMIT 100
```

Args:

```json
["2026-01-01"]
```

---

## Important Design Decision

The AI should generate this:

```json
{
  "select": [],
  "filters": [],
  "group_by": [],
  "order_by": []
}
```

Not this:

```sql
SELECT * FROM orders;
```

The backend owns SQL generation.

This is the key design that makes the project safer, testable, multi-database, and maintainable.
