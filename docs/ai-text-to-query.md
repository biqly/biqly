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

## Metadata Description Language

For deployments where users mostly ask questions in Turkish, table and column
`description` values should be Turkish-first. Vector routing embeds the user's
question together with metadata descriptions, so descriptions like "müşteri
siparişlerinin toplam tutarı" usually match Turkish questions better than a
purely English catalog note.

Keep physical schema names and common English technical terms in the text when
they are useful:

```text
Müşteri siparişlerinin başlık bilgileri. Sipariş tarihi, müşteri, bölge,
toplam tutar ve durum bilgisini içerir. Teknik tablo adı: SalesOrderHeader.
```

After changing descriptions, run `POST /api/ai/metadata/embed` or use the UI's
**Refresh metadata embeddings** action so table routing and column retrieval
use the updated Turkish descriptions.

### Optional TranslateGemma layer

AI Describe can run a second OpenAI-compatible model to translate/normalize the
generated metadata descriptions before they are applied. This is intended for
models such as Ollama TranslateGemma:

```env
BI_AI_TRANSLATION_MODEL=translategemma:4b
BI_AI_TRANSLATION_BASE_URL=http://localhost:11434/v1
BI_AI_TRANSLATION_TARGET_LANGUAGE=Turkish
BI_AI_TRANSLATION_TARGET_CODE=tr
```

The translation layer receives a single JSON payload for the whole table and
must return the same shape. Biqly validates that column names and counts are
unchanged. If the translation request fails or changes identifiers, the original
AI Describe output is kept and the response includes `translation_error`.

Do not use this layer for LogicalQuery output. Query planning must keep strict
JSON, semantic field names, table names, and column names unchanged.
