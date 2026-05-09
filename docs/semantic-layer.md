# Semantic Layer

The semantic layer maps business concepts to physical database structures.

## Model

A **SemanticModel** represents a business entity:

```json
{
  "name": "orders",
  "label": "Customer Orders",
  "base_schema": "public",
  "base_table": "orders",
  "synonyms": ["purchases", "sales"]
}
```

## Dimensions

Group-by-able fields:

```json
{
  "name": "country",
  "column_ref": "customers.country",
  "type": "text",
  "synonyms": ["region", "market"]
}
```

Types: `text`, `number`, `date`, `boolean`, `geo`

## Metrics

Aggregatable values:

```json
{
  "name": "revenue",
  "expression": "orders.total_amount",
  "aggregation": "sum",
  "format": "$#,##0"
}
```

Aggregations: `count`, `sum`, `avg`, `min`, `max`, `count_distinct`

## Joins

Table relationships:

```json
{
  "name": "orders_to_customers",
  "from_table": "orders",
  "from_column": "customer_id",
  "to_table": "customers",
  "to_column": "id",
  "join_type": "LEFT",
  "relationship": "many_to_one"
}
```

Relationship types affect fanout detection:

- `many_to_one` — safe for aggregations
- `one_to_many` — can cause fanout
- `many_to_many` — high fanout risk
