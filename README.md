# BI Query Engine

A Business Intelligence query engine built in Go that generates safe SQL queries from structured logical queries, with a semantic layer and AI-powered natural language to query support.

## Architecture

```
User Question → LogicalQuery JSON → Validate → Compile (dialect) → Execute → Results
                                      ↑
                              Semantic Layer (models, dimensions, metrics, joins)
```

The key design decision: **AI generates LogicalQuery JSON, never raw SQL**. The backend owns SQL generation.

## Quick Start

### Prerequisites

- Go 1.23+
- Docker & Docker Compose
- PostgreSQL (for metadata DB)

### Local Development

```bash
# Start infrastructure
make docker-up

# Run migrations
make migrate-up

# Run the API server
make dev
```

API will be available at `http://localhost:8888`.

### Environment Variables

```bash
cp .env.example .env
# Edit .env with your values
```

See `.env.example` for all available options.

### Metadata descriptions for Turkish questions

If business users mostly ask questions in Turkish, keep table and column
descriptions Turkish-first. The AI router embeds user questions together with
metadata descriptions, so Turkish descriptions usually improve table and column
matching. Keep physical schema names or common English technical terms in the
description when useful, for example:

```text
Müşteri siparişlerinin başlık bilgileri. Sipariş tarihi, müşteri, bölge,
toplam tutar ve durum bilgisini içerir. Teknik tablo adı: SalesOrderHeader.
```

After metadata sync or description edits, refresh metadata embeddings from the
AI Query page so routing uses the latest descriptions.

Optionally enable a dedicated OpenAI-compatible translation layer for AI Describe
output, for example Ollama TranslateGemma:

```env
BI_AI_TRANSLATION_MODEL=translategemma:4b
BI_AI_TRANSLATION_BASE_URL=http://localhost:11434/v1
BI_AI_TRANSLATION_TARGET_LANGUAGE=Turkish
BI_AI_TRANSLATION_TARGET_CODE=tr
```

This layer only translates/normalizes table and column `description` values. It
does not translate LogicalQuery JSON, table names, column names, or SQL
identifiers.

## API Endpoints

### Datasources
- `POST /api/datasources` - Create a datasource
- `GET /api/datasources` - List datasources
- `GET /api/datasources/{id}` - Get datasource details
- `POST /api/datasources/{id}/test` - Test connection
- `POST /api/datasources/{id}/sync-metadata` - Sync schema metadata

### Semantic Layer
- `POST /api/semantic/models` - Create semantic model
- `GET /api/semantic/models` - List models
- `GET /api/semantic/models/{id}` - Get model with dimensions, metrics, joins
- `POST /api/semantic/models/{id}/dimensions` - Add dimension
- `POST /api/semantic/models/{id}/metrics` - Add metric
- `POST /api/semantic/models/{id}/joins` - Add join

### Queries
- `POST /api/query/compile` - Compile LogicalQuery to SQL
- `POST /api/query/run` - Execute a query
- `POST /api/query/explain` - Get query explanation

### AI
- `POST /api/ai/query` - Natural language to LogicalQuery
- `POST /api/ai/query/preview` - Preview generated SQL
- `POST /api/ai/query/run` - Execute AI-generated query

### Health & Metrics
- `GET /health` - Health check
- `GET /metrics` - Prometheus-style metrics

## Project Structure

```
cmd/api/                    - API entry point
internal/
  config/                   - Application configuration
  app/                      - Dependency wiring
  http/                     - HTTP router and handlers
  datasource/               - Database driver system
    postgres/               - PostgreSQL driver
  metadata/                 - Metadata repository
  semantic/                 - Semantic layer
  query/                    - LogicalQuery, compiler, executor
  dialect/                  - SQL dialect implementations
  security/                 - Read-only protection, permissions
```

## Roadmap

- [x] Phase 1: Core Foundation
- [x] Phase 2: Metadata Database
- [x] Phase 3: PostgreSQL Driver
- [x] Phase 4: Semantic Layer
- [x] Phase 5: Logical Query Model
- [x] Phase 6: SQL Compiler
- [x] Phase 7: Safe Query Execution
- [x] Phase 8: Query API
- [x] Phase 9: MySQL, SQL Server, ClickHouse drivers
- [x] Phase 10: AI Text-to-Query
- [x] Phase 11: Query Planner
- [x] Phase 12: Permissions & RLS
- [x] Phase 13: Caching (Redis)
- [x] Phase 14: Observability
