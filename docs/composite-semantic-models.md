# Composite Semantic Models

Composite semantic models merge two or more published `SemanticModel`s into a
single cross-domain model. A user can then ask one natural-language question
that spans multiple business domains (e.g. orders + customers + support
tickets) and Biqly resolves the cross-domain joins automatically.

The core principle is unchanged: **AI generates `LogicalQuery` JSON — never raw
SQL.** A composite is resolved into an ordinary `SemanticModel` before
compilation, so the existing compiler, validator, read-only checker, and
executor apply without modification.

---

## Concepts

| Concept | Description |
|---|---|
| **Component** | A published `SemanticModel` referenced by a composite, addressed by a unique `alias`. Exactly one component has role `primary`; the rest are `secondary`. |
| **Cross-model join** | A join between two component aliases (`from_alias.from_dimension` → `to_alias.to_dimension`) with a `join_type` and a `relationship` cardinality. |
| **Canonical date** | One component dimension chosen as the composite's authoritative date field for time-grain queries. |
| **Dimension resolution** | Strategy for handling duplicate dimension names across components (alias-prefix by default, or `use_primary` to drop the secondary copy). |
| **Resolved model** | The flattened `SemanticModel` produced by merging components, deduplicating fields, and flattening cross-model joins into ordinary joins. |

---

## Pipeline

```
NL question
  → Composite Router (keyword overlap over component domains)
  → Prompt Builder (cross-domain CompositeContext narrative)
  → LLM → LogicalQuery JSON (carries composite_id)
  → GetPublishedResolvedComposite (cache → snapshot)
  → Validator (against resolved model)
  → Compiler (resolved model → parameterized SQL)
  → Read-only Checker → Executor
```

The resolved model is what every downstream stage sees; nothing below the
resolver is aware that a model is composite.

---

## Resolver

`internal/semantic/composite.go` — `CompositeResolver.Resolve(composite, components)`:

1. Start from the primary component's base schema/table.
2. Merge dimensions and metrics from every component.
3. Deduplicate by name:
   - Default: prefix the secondary copy with its alias (`region` → `customers_region`).
   - `use_primary`: drop the secondary copy entirely.
4. Flatten each active cross-model join into an ordinary `Join` on the resolved model.
5. Attach the canonical date dimension so time-grain queries have an authoritative date field.

The output is a normal `SemanticModel` with `Dimensions`, `Metrics`, and `Joins`.

---

## Metric Graph & Circular Dependencies

`internal/semantic/metric_graph.go` builds a dependency graph across component
metrics (`derives_from` references, including cross-model) and detects cycles
before publish so a composite cannot define mutually recursive metrics.

---

## Fanout Detection

When a cross-model join is `one_to_many` or `many_to_many`, aggregated metrics
can fan out and double-count. `ValidateComposite` emits warnings:

- `one_to_many` → "verify metric grain to avoid fanout".
- `many_to_many` → "aggregated metrics may fan out and double-count".

`many_to_one` joins are safe and produce no fanout warning.

---

## Publish / Rollback

`internal/semantic/composite_publish.go`:

- `ValidateComposite(ctx, composite, provider)` — checks name, ≥2 components,
  exactly 1 primary, cross-join alias/dimension references, canonical date, and
  applies fanout warnings and configured size limits.
- `PublishComposite` — validates, then writes a versioned snapshot
  (`composite_context_snapshots`) containing the resolved model, invalidates the
  cache, and reloads.
- `RollbackComposite(targetVersion)` — restores a prior snapshot as a new
  version.
- `GetPublishedResolvedComposite` — cache read-through, falling back to the
  latest snapshot.

Components are supplied through a `ComponentProvider`, which loads each
component's published full model.

---

## Caching

Resolved composites are cached in Redis (`NewRedisCompositeCache`). The cache is
built directly on `go-redis` inside `internal/semantic` to avoid an import cycle
(the generic Redis cache package imports `query`, which imports `semantic`). A
nil Redis client yields a nil cache, so caching degrades gracefully.

---

## Configuration

Size limits are enforced at validate/publish time. Zero disables a limit.

| Env var | Default | Limit |
|---|---|---|
| `BI_COMPOSITE_MAX_COMPONENTS` | 8 | Component models per composite |
| `BI_COMPOSITE_MAX_CROSS_JOINS` | 16 | Active cross-model joins |
| `BI_COMPOSITE_MAX_MERGED_FIELDS` | 300 | Combined dimensions + metrics of the resolved model |

Limits are wired onto the repository via `CompositeRepository.WithLimits` and
enforced in `ValidateComposite` / `PublishComposite`.

---

## HTTP API

All routes are served by the catalog service (and the monolith) under
`/api/semantic/composites`:

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/semantic/composites` | Create composite |
| `GET` | `/semantic/composites` | List composites |
| `GET` | `/semantic/composites/{id}` | Get full composite |
| `PUT` | `/semantic/composites/{id}` | Update composite |
| `DELETE` | `/semantic/composites/{id}` | Delete composite |
| `POST` | `/semantic/composites/{id}/components` | Add component |
| `DELETE` | `/semantic/composites/{id}/components/{model_id}` | Remove component |
| `POST` | `/semantic/composites/{id}/cross-joins` | Add cross-model join |
| `PUT` | `/semantic/composites/{id}/cross-joins/{join_id}` | Update cross-model join |
| `DELETE` | `/semantic/composites/{id}/cross-joins/{join_id}` | Remove cross-model join |
| `PUT` | `/semantic/composites/{id}/canonical-date` | Set canonical date |
| `PUT` | `/semantic/composites/{id}/dimension-resolutions` | Set dimension resolutions |
| `POST` | `/semantic/composites/{id}/validate` | Validate composite |
| `POST` | `/semantic/composites/{id}/publish` | Publish composite |
| `POST` | `/semantic/composites/{id}/rollback` | Rollback composite |
| `GET` | `/semantic/composites/{id}/suggested-joins` | Suggested cross-model joins |

---

## Database Schema

Migration `037a_composite_semantic_models.up.sql`:

| Table | Purpose |
|---|---|
| `composite_models` | Composite header (datasource, name, canonical_date JSONB, status, version) |
| `composite_model_components` | Component refs (model_id, alias, role) |
| `composite_cross_model_joins` | Cross-model joins (from/to alias+dimension, join_type, relationship) |
| `composite_dimension_resolutions` | Duplicate-dimension resolution strategy |
| `composite_context_snapshots` | Versioned resolved-model snapshots for publish/rollback |

---

## Where to change things

- Resolver / merger: `internal/semantic/composite.go`
- Metric graph: `internal/semantic/metric_graph.go`
- Publish / rollback / validation / limits: `internal/semantic/composite_publish.go`
- Persistence: `internal/semantic/composite_repository.go`
- Caching: `internal/semantic/composite_cache.go`
- Routing: `internal/ai/routing/composite_router.go`
- Prompt context: `internal/ai/prompt/prompt.go` (`writeCompositeContext`)
- HTTP handlers: `internal/http/handlers/composite.go`
- Compiler integration: composite is resolved to a `SemanticModel` before
  `internal/query/compiler.go` runs — no composite-specific compiler code.

---

## Tests

- `internal/semantic/composite_test.go` — resolver merge / dedup / flatten.
- `internal/semantic/metric_graph_test.go` — dependency graph + cycle detection.
- `internal/semantic/composite_fanout_test.go` — fanout warnings + size limits.
- `internal/semantic/composite_cache_test.go` — cache behavior.
- `internal/semantic/composite_integration_test.go` — DB-gated CRUD + publish/rollback (skips without a migrated metadata DB).
- `internal/query/golden_test.go` (`TestGolden_PostgresComposite`) + `testdata/sql/postgres/composite_cross_model.sql` — golden SQL for a cross-model query.
