# Biqly Configuration Parameters

This document catalogs all environment variables used by the Biqly platform. It details their code-level defaults, Helm-level defaults, runtime database override capability, consumption points, and UI editability.

## Parameter Reference Table

| Key | Default (Go Code) | Helm Value | Runtime Override? | Used In (File/Function) | UI Editable? | Notes / Explanation |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **System & HTTP** | | | | | | |
| `BI_HTTP_HOST` | `"0.0.0.0"` | N/A (Pod-bound) | No | `cmd/api` (HTTP Server host) | No | Listen interface host. |
| `BI_HTTP_PORT` | `8888` | `8082` (AI), `8080` (Catalog/Frontend), `8081` (Query) | No | `cmd/api` (HTTP Server port) | No | Listen port. |
| `BI_CORS_ALLOWED_ORIGINS` | `""` | `""` (Inherits) | No | `internal/http/middleware/cors.go` | No | List of allowed CORS origins. |
| `BI_HSTS_ENABLED` | `false` | `false` | No | `internal/http/middleware/hsts.go` | No | Enables Strict-Transport-Security. |
| `BI_LOG_LEVEL` | `"info"` | `"info"` | No | `internal/platform/logger` | No | `debug`, `info`, `warn`, `error`. |
| `BI_LOG_FORMAT` | `"json"` | `"json"` | No | `internal/platform/logger` | No | `json` or `text`. |
| `BI_DRIFT_CHECK_INTERVAL` | `"6h"` | N/A | No | `internal/app/drift` | No | Period for semantic metadata drift checks. |
| `BI_CATALOG_SERVICE_URL` | `""` | `http://biqly-catalog:8080` | No | `internal/auth/client` | No | URL of the catalog service. |
| `BI_QUERY_SERVICE_URL` | `""` | `http://biqly-query:8081` | No | `internal/auth/client` | No | URL of the query service. |
| `BI_AI_SERVICE_URL` | `""` | `http://biqly-ai:8082` | No | `internal/auth/client` | No | URL of the AI service. |
| **Databases, Queue & Query Engine** | | | | | | |
| `BI_METADATA_DB_DSN` | `postgres://localhost:5432/...` | Required (Secret) | No | `internal/platform/db` | No | Connection string for Metadata Postgres DB. |
| `BI_REDIS_DSN` | `redis://localhost:6379` | `redis://biqly-dragonfly:6379` | No | `internal/platform/db/redis.go` | No | Connection string for Dragonfly (Redis). |
| `BI_NATS_URL` | `""` | `nats://biqly-nats:4222` | No | `internal/queue/nats.go` | No | Connection string for NATS JetStream. |
| `BI_NATS_STREAM` | `"BIQLY_AI_JOBS"` | `"BIQLY_AI_JOBS"` | No | `internal/queue/nats.go` | No | JetStream stream name. |
| `BI_NATS_SUBJECT` | `"biqly.ai.jobs"` | `"biqly.ai.jobs"` | No | `internal/queue/nats.go` | No | NATS subject routing key. |
| `BI_NATS_CONSUMER_GROUP` | `"biqly-ai-workers"` | `"biqly-ai-workers"` | No | `internal/queue/nats.go` | No | NATS consumer group. |
| `BI_AI_JOBS_ENABLED` | `true` | `true` | No | `internal/app` / workers | No | Toggle processing of async AI jobs on NATS queues. |
| `BI_AI_JOBS_CONSUMER_ENABLED` | `true` | `false` (AI API), `true` (worker) | No | `cmd/api`, `services/ai/cmd`, `cmd/worker` | No | Enables the in-process AI job consumer; disable on API pods when standalone workers are deployed. |
| `BI_QUERY_TIMEOUT_SECONDS` | `30` | `30` | No | `internal/query` | No | Maximum time limit for query execution on databases. |
| `BI_QUERY_MAX_ROWS` | `10000` | `10000` | No | `internal/query` | No | Maximum rows returned by query execution to avoid OOM. |
| `BI_QUERY_MAX_RUNTIME_SECONDS` | `60` | `60` | No | `internal/query` | No | Maximum runtime for a single query execution. |
| `BI_QUERY_HISTORY_LIST_LIMIT` | `100` | N/A | No | `internal/query` | No | Maximum query history records shown in the UI. |
| `BI_EVAL_RUNS_LIST_LIMIT` | `50` | N/A | No | `internal/query` | No | Maximum evaluation run records shown in the list UI. |
| `BI_COMPOSITE_MAX_COMPONENTS` | `8` | N/A | No | `internal/query/composite` | No | Max component tables allowed in a composite database view. |
| `BI_COMPOSITE_MAX_CROSS_JOINS` | `16` | N/A | No | `internal/query/composite` | No | Max cross joins allowed in composite query resolution. |
| `BI_COMPOSITE_MAX_MERGED_FIELDS` | `300` | N/A | No | `internal/query/composite` | No | Max columns/fields allowed in a merged composite schema. |
| **Authentication & Security** | | | | | | |
| `BI_ENCRYPTION_KEY` | (Placeholder) | Required (Secret) | No | `internal/security` | No | AES key for encrypting provider keys in DB. |
| `BI_ADMIN_API_KEY` | `""` | Required (Secret) | No | `internal/http/middleware/auth.go` | No | Shared token for machine-to-machine admin endpoints (CLI/CI). |
| `BI_INTERNAL_API_TOKEN` | `""` | Required (Secret) | No | `internal/http/middleware/auth.go` | No | Token used for secure BFF-to-microservice peer calls. |
| `BI_API_KEY` | `""` | `""` | No | `internal/http/middleware/auth.go` | No | Shared API key when JWT auth is disabled. |
| `BI_METRICS_API_KEY` | `""` | `""` | No | `internal/http/middleware/auth.go` | No | Token to restrict scraping of the `/metrics` endpoint. |
| `BI_AUTH_ENABLED` | `false` | `true` | No | `internal/http/middleware/auth.go` | No | Enables JWT authentication via Auth service. |
| `BI_AUTH_SERVICE_URL` | `""` | `http://biqly-auth:8889` | No | `internal/auth/client` | No | Auth service public key fetch location. |
| `BI_AUTH_INTERNAL_TOKEN` | `""` | Required (Secret) | No | `internal/auth/client` | No | BFF communication token for Auth. |
| `BI_AUTH_MAIL_SERVICE_URL` | `"http://localhost:8890"` | `http://biqly-mail:8080` (or similar) | No | `internal/auth` | No | URL of the mail notification microservice. |
| `BI_AUTH_MAIL_INTERNAL_TOKEN` | `""` | Required (Secret) | No | `internal/auth` | No | Peer token for secure calls to the mail service. |
| `BI_AUTH_FRONTEND_BASE_URL` | `"http://localhost:3333"` | Required | No | `internal/auth` | No | Base URL of the web UI for redirection/mail link generation. |
| **AI Settings (Core LLM & Cache)** | | | | | | |
| `BI_AI_HTTP_TIMEOUT_SECONDS`| `300` | `300` | No | `internal/config/config.go` (HTTPTimeout) | No | HTTP timeout for chat/completion calls. |
| `BI_AI_RATE_LIMIT_PER_MINUTE`| `20` | `20` | No | `internal/config/config.go` | No | Saved to DB configuration defaults but not active-throttled. |
| `BI_AI_MAX_PROMPT_RUNES` | `80000` | `40000` | Yes | `internal/ai/prompt/prompt_builder.go` | **Yes** | Limit context window sizing. Helm override is smaller. |
| `BI_AI_MAX_RETRIES` | `2` | `1` | Yes | `internal/ai/service.go` (retry loop) | **Yes** | LLM error recovery retry count. Helm override is smaller. |
| `BI_AI_MULTI_CANDIDATE_COUNT`| `1` | `1` | No | `internal/ai/service.go` | No | Chat completion alternatives count. |
| `BI_AI_RESPONSE_CACHE_TTL` | `3600` | `3600` | No | `internal/ai/service.go` | No | TTL for cached LLM query responses. |
| `BI_AI_DESCRIBE_MAX_CELL_RUNES`| `500` | `500` | No | `internal/ai/describe` | No | String cell truncation length for database schema description. |
| `BI_AI_DESCRIBE_MAX_SAMPLE_ROWS`| `12` | `12` | No | `internal/ai/describe` | No | Max database rows fetched for schema sampling. |
| **AI Settings (Embedding & Translation)** | | | | | | |
| `BI_AI_EMBEDDING_HTTP_TIMEOUT_SECONDS`| `600` | `1200` | No | `internal/ai/memory/recall.go` | No | HTTP timeout for embedding API calls. |
| `BI_AI_EMBEDDING_WEIGHT` | `30.0` | `30.0` | No | `internal/ai/routing/router.go` | No | Weight of embeddings in hybrid routing calculation. |
| `BI_AI_EMBEDDING_DENY_SCHEMAS`| `""` | `""` | No | `internal/ai/routing/router.go` | No | Comma-separated list of database schemas excluded from embedding. |
| `BI_AI_EMBEDDING_DENY_TABLES` | `""` | `""` | No | `internal/ai/routing/router.go` | No | Comma-separated list of schema.table pairs excluded from embedding. |
| `BI_AI_TRANSLATION_TARGET_LANGUAGE`| `"Turkish"` | `"Turkish"` | No | `internal/ai/translation` | No | Target language for autogenerated schema comments. |
| `BI_AI_TRANSLATION_TARGET_CODE`| `"tr"` | `"tr"` | No | `internal/ai/translation` | No | BCP-47 locale tag for translation target. |
| `BI_AI_TRANSLATION_HTTP_TIMEOUT_SECONDS`| `120` | `120` | No | `internal/ai/translation` | No | HTTP timeout for translation API calls. |
| **AI Settings (Table Routing)** | | | | | | |
| `BI_AI_ROUTING_LEXICON_PATH` | `""` | `""` | No | `internal/ai/routing/lexicon` | No | Path to synonym/vocabulary override JSON file. |
| `BI_AI_ROUTING_WEIGHTS_PATH` | `""` | `""` | No | `internal/ai/routing/weights` | No | Path to routing score overrides JSON file. |
| `BI_AI_ROUTE_MAX_DIMENSIONS` | `0` | `56` (Helm override)| No | `internal/ai/routing/router.go` | No | Max dimensions auto-generated per semantic model. |
| `BI_AI_ROUTE_MAX_METRICS` | `0` | `32` (Helm override)| No | `internal/ai/routing/router.go` | No | Max metrics auto-generated per semantic model. |
| `BI_AI_ROUTE_MAX_COLUMNS_PER_TABLE`| `0` | `14` (Helm override)| No | `internal/ai/routing/router.go` | No | Max columns scored per database table in hybrid routing. |
| `BI_AI_ROUTE_MAX_DATE_GRAIN_EXTRAS`| `0` | `20` (Helm override)| No | `internal/ai/routing/router.go` | No | Max date grain variations computed per date/time field. |
| `BI_AI_ROUTE_SLIM_NUMERIC_METRICS`| `true` | `true` | No | `internal/ai/routing/router.go` | No | Exclude avg_/min_ metrics for faster generation. |
| **AI Settings (Ambiguity & Memory)** | | | | | | |
| `BI_AI_AMBIGUITY_CHECK_ENABLED`| `true` | `true` | Yes | `internal/http/handlers/ai.go` | **Yes** | Toggle pre-LLM clarification workflow. |
| `BI_AI_AMBIGUITY_CONFIDENCE_THRESHOLD`| `0.70`| `0.70` | Yes | `internal/http/handlers/ai.go` | **Yes** | Scoring floor where ambiguity checks kick in. |
| `BI_AI_AMBIGUITY_MAX_OPTIONS` | `5` | `5` | Yes | `internal/http/handlers/ai.go` | **Yes** | Max options returned to user for clarification cards. |
| `BI_AI_AMBIGUITY_LLM_ENABLED` | `false` | `false` | Yes | `internal/http/handlers/ai.go` | **Yes** | Toggles LLM-driven backup ambiguity scoring. |
| `BI_AI_AMBIGUITY_TIERED_ENABLED`| `false` | `true` (Helm override) | Yes | `internal/http/handlers/ai.go` | **Yes** | Runs synonym checks before scope checks. |
| `BI_AI_AMBIGUITY_MAX_LLM_TIER_PER_QUESTION`| `1` | `1` | Yes | `internal/http/handlers/ai.go` | **Yes** | Limits how many rounds can invoke LLM ambiguity checks. |
| `BI_AI_MEMORY_RECALL_ENABLED` | `true` | `true` | Yes | `internal/ai/memory/recall.go` | **Yes** | Enables injection of confirmed-query examples. |
| `BI_AI_MEMORY_RECALL_LIMIT` | `5` | `5` | Yes | `internal/ai/memory/recall.go` | **Yes** | Max count of few-shot example insertions. |
| **PII Detection & Masking** | | | | | | |
| `BI_PII_ENABLED` | `true` | `true` | No | `internal/security/pii` | No | Master switch for PII scanning and masking. |
| `BI_PII_DETECTION_THRESHOLD` | `0.6` | `0.6` | Yes | `internal/security/pii/scan.go` | **Yes** | Minimum PII detection confidence score. |
| `BI_PII_SAMPLE_DATA_LIMIT` | `50` | `50` | No | `internal/security/pii/scan.go` | No | Count of database records sampled during PII scans. |
| `BI_PII_AUTO_SCAN_ON_SYNC` | `true` | `true` | No | `internal/security/pii/scan.go` | No | Trigger automatic PII scanning on metadata sync. |
| `BI_PII_DEFAULT_MASKING_STRATEGY`| `"partial"`| `"partial"` | Yes | `internal/security/pii/mask.go` | **Yes** | Default masking strategy name applied to PII columns. |

## Key Insights & Architecture

1. **Hierarchy of Overrides**:
   - Every variable has a **Hardcoded Default** in `internal/config/config.go`.
   - **Helm configuration** (defined in `values.yaml` and subcharts) overrides these at startup via env variables.
   - For variables marked **Runtime Override? = Yes**, the values are loaded from the database (`ai_runtime_config` table). If a database setting is set, it overrides both code defaults and env values dynamically at runtime.
2. **Provider & Model Selection**:
   - Connection/model routing targets (like OpenAI API Keys, model names, and base URLs) are **not** managed via environment variables.
   - They are configured dynamically in the administration panel under `Administration -> AI Providers` and stored in the database (`ai_providers` / `ai_models` tables), allowing live rotation of provider keys.
