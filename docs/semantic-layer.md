# Semantic Layer

The semantic layer maps business concepts to physical database structures.

## Model

A **SemanticModel** represents a business entity:

```json
{
  "name": "orders",
  "label": "Müşteri Siparişleri",
  "description": "Müşteri siparişlerinin başlık bilgileri. Teknik tablo adı: orders.",
  "base_schema": "public",
  "base_table": "orders",
  "synonyms": ["siparişler", "satışlar", "purchases", "sales"]
}
```

If users usually ask questions in Turkish, prefer Turkish-first model, table,
and column descriptions. Keep English physical names or common technical terms
inside the description when they help bridge the Turkish business language to
the database schema.

AI Describe can optionally post-process generated descriptions through a
translation model such as Ollama TranslateGemma. Configure
`BI_AI_TRANSLATION_MODEL` and `BI_AI_TRANSLATION_BASE_URL` to translate or
normalize only description text while preserving semantic names, table names,
column names, and SQL identifiers.

## Dimensions

Group-by-able fields:

```json
{
  "name": "country",
  "column_ref": "customers.country",
  "type": "text",
  "description": "Müşterinin ülkesi veya pazarı.",
  "synonyms": ["ülke", "bölge", "pazar", "region", "market"]
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
