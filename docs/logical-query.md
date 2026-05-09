# LogicalQuery

The `LogicalQuery` is a database-independent query representation that serves as the core abstraction in the system.

## Structure

```json
{
  "datasource_id": "ds_123",
  "model_id": "orders",
  "select": [
    { "type": "dimension", "name": "country" },
    { "type": "metric", "name": "revenue" }
  ],
  "filters": [
    { "field": "created_at", "operator": "gte", "value": "2026-01-01" }
  ],
  "group_by": [
    { "field": "country" }
  ],
  "order_by": [
    { "field": "revenue", "direction": "desc" }
  ],
  "limit": 100,
  "offset": 0
}
```

## Fields

### `select`

Array of items with `type` (dimension|metric), `name`, and optional `alias`.

### `filters`

Array of conditions. Supported operators:

- Comparison: `eq`, `neq`, `gt`, `gte`, `lt`, `lte`
- Set: `in`, `not_in`
- Pattern: `contains`, `starts_with`, `ends_with`
- Range: `between`
- Null: `is_null`, `is_not_null`

### `group_by`

Dimensions to group results by.

### `order_by`

Sort by dimension or metric with `asc`/`desc` direction.

### `limit` / `offset`

Pagination controls.

## Why Not Raw SQL?

| Problem | Raw SQL | LogicalQuery |
| --- | --- | --- |
| SQL injection | Risk from AI hallucination | Impossible (parameterized) |
| Multi-dialect | Tied to one database | Compiles to any dialect |
| Validation | Must parse and analyze SQL | Structured, easy to validate |
| Permissions | Hard to inject row filters | Filter injection is clean |
| Testing | Compare SQL strings | Compare AST |
