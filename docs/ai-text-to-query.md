# AI Text-to-Query

## Architecture

```text
Natural Language Question
        │
        ▼
┌─────────────────────┐
│  Semantic Context   │  (dimensions, metrics, joins, synonyms)
│  Prompt Builder     │
└─────────────────────┘
        │
        ▼
┌─────────────────────┐
│  LLM (OpenAI API)   │  Returns strict LogicalQuery JSON
└─────────────────────┘
        │
        ▼
┌─────────────────────┐
│  JSON Validator     │  Checks structure and semantic alignment
└─────────────────────┘
        │
        ▼
┌─────────────────────┐
│  SQL Compiler       │  Dialect-specific parameterized SQL
└─────────────────────┘
        │
        ▼
┌─────────────────────┐
│  Safe Executor      │  Read-only, timeout, row limits
└─────────────────────┘
```

## AI Guardrails

1. **Never generates raw SQL** — always outputs LogicalQuery JSON
2. **Semantic-only fields** — can only reference dimensions/metrics in the model
3. **JSON schema validation** — strict schema enforcement
4. **Empty select rejection** — ambiguous questions return warnings, not garbage
5. **Preview before execution** — `/ai/query/preview` shows SQL without running
6. **Confidence scoring** — low confidence questions flagged

## Prompt Context

The AI receives:

- Model name, base table, description
- All dimensions with types, column refs, and synonyms
- All metrics with expressions, aggregations, and synonyms
- All joins with cardinality information
- Supported filter operators
- Current date/time for relative date reasoning
- Hard instructions: output JSON only, never invent fields
