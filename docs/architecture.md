# Architecture

## Overview

BI Query Engine is a Go-based system that converts structured logical queries into safe, dialect-specific SQL, executes them against various databases, and returns results.

```text
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐     ┌──────────────┐     ┌──────────┐
│  User / AI  │────▶│  LogicalQuery    │────▶│  Validator  │────▶│  Compiler    │────▶│ Executor │
│             │     │  (JSON)          │     │             │     │  (dialect)   │     │          │
└─────────────┘     └──────────────────┘     └─────────────┘     └──────────────┘     └──────────┘
                                                                                        │
                                                                             ┌──────────┴──────────┐
                                                                             ▼                     │
                                                                       ┌──────────┐     ┌──────────┐
                                                                       │ Results  │     │ Cache    │
                                                                       └──────────┘     └──────────┘
```

## Key Design Decisions

### LogicalQuery-First

AI generates `LogicalQuery` JSON, never raw SQL. The backend owns SQL generation. This ensures:

- **Safety**: Parameterized queries, no string concatenation
- **Multi-database**: Same logical query compiles to PostgreSQL, MySQL, SQL Server, ClickHouse
- **Validation**: Semantic layer prevents invalid field references
- **Testability**: Golden SQL tests verify compiler output

### Semantic Layer

Business users interact with models/dimensions/metrics, not raw tables/columns. The semantic layer:

- Maps business terms to physical columns
- Defines joins with cardinality awareness
- Provides synonyms for AI understanding
- Stores Turkish-first table/column descriptions when users ask Turkish questions
- Optionally normalizes AI-generated descriptions through a dedicated translation model
- Enables field-level permissions

### Driver Registry

Each database type registers as a driver implementing:

- Connection management
- Schema introspection
- SQL dialect (quoting, placeholders, functions)

## Data Flow

1. **Datasource setup**: User connects → credentials encrypted → metadata synced
2. **Semantic modeling**: Admin defines models, dimensions, metrics, joins
3. **Query submission**: User sends LogicalQuery (or natural language)
4. **Validation**: Fields checked against semantic model
5. **Planning**: Required joins determined, fanout risks detected
6. **Compilation**: Dialect-specific SQL with parameterized placeholders
7. **Execution**: Context timeout, read-only check, result collection
8. **Caching**: Result stored by hash for future requests

## Security Layers

| Layer | Protection |
| --- | --- |
| Read-only DB creds | Database user has only SELECT permissions |
| SQL checker | Rejects non-SELECT, dangerous keywords, multi-statement |
| Parameterized queries | Values never concatenated into SQL |
| Identifier quoting | Table/column names safely quoted per dialect |
| Permissions | Field-level, model-level, row-level security |
| Context timeouts | Queries cancelled after configured timeout |
