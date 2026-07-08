# Biqly Configuration Parameters

This document catalogs the BI_* variables loaded through the shared `internal/config.Config` path (`internal/config/config.go`) used by the monolith/catalog/query/ai services. Auth-service-only, mail-service-only, and test/eval-only env vars live in their own packages and are out of scope here; this page focuses on the shared config loader, its Helm overrides, its runtime database override capability, and the primary load/override points.

## Parameter Reference Table

| Key | Default (Go Code) | Helm Value | Runtime Override? | Primary Load / Override Point | UI Editable? | Notes / Explanation |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **System & HTTP** | | | | | | |
| `BI_HTTP_HOST` | `"0.0.0.0"` | N/A (Pod-bound) | No | `cmd/api` (HTTP Server host) | No | Listen interface host. |
| `BI_HTTP_PORT` | `8888` | `8082` (AI), `8080` (Catalog/Frontend), `8081` (Query) | No | `cmd/api` (HTTP Server port) | No | Listen port. |
| `BI_CORS_ALLOWED_ORIGINS` | `""` | `""` (Inherits) | No | `internal/config/config.go`, `internal/http/router.go` | No | List of allowed CORS origins. |
| `BI_HSTS_ENABLED` | `false` | `false` | No | `internal/config/config.go`, `internal/http/router.go` | No | Enables Strict-Transport-Security. |
| `BI_LOG_LEVEL` | `"info"` | `"info"` | No | `internal/platform/logger` | No | `debug`, `info`, `warn`, `error`. |
| `BI_LOG_FORMAT` | `"json"` | `"json"` | No | `internal/platform/logger` | No | `json` or `text`. |
| `BI_DEPLOYMENT_MODE` | `"cloud"` | `"cloud"` | No | `internal/config/config.go`, `internal/ai/provider/egress.go` | No | `cloud`, `private`, or `airgapped`. Airgapped fails closed on external LLM/embedding egress: provider endpoints must be private/in-cluster hosts. |
| `BI_DRIFT_CHECK_INTERVAL` | `"6h"` | N/A | No | `internal/config/config.go`, `internal/app/dependencies.go` | No | Period for semantic metadata drift checks. |
| `BI_CATALOG_SERVICE_URL` | `""` | `http://biqly-catalog:8080` | No | `internal/config/config.go`, `internal/http/router.go` | No | URL of the catalog service. |
| `BI_QUERY_SERVICE_URL` | `""` | `http://biqly-query:8081` | No | `internal/config/config.go`, `internal/http/router.go` | No | URL of the query service. |
| `BI_AI_SERVICE_URL` | `""` | `http://biqly-ai:8082` | No | `internal/config/config.go`, `internal/http/router.go` | No | URL of the AI service. |
| `BI_API_SERVICE_URL` | `""` | `https://abi.il1.nl` | No | `internal/config/config.go`, `internal/http/mcp_router.go` | No | Base URL of the API gateway the standalone MCP service forwards governed tool calls to. Must carry the gateway hostname so HTTPRoutes match. Empty in the monolith. |
| **Agentic Query Runner** | | | | | | |
| `BI_AGENT_ENABLED` | `false` | `false` | No | `internal/config/config.go`, `internal/agent` | No | Toggles the agentic query-runner pipeline. Disabled by default; the legacy NL-to-SQL pipeline handles all traffic until enabled. |
| `BI_AGENT_MODE` | `"shadow"` | `"shadow"` | No | `internal/config/config.go`, `internal/agent` | No | `shadow` (compute for comparison, never surface to the user) or `active` (the agent's result reaches the user). |
| `BI_AGENT_MAX_STEPS` | `6` | `6` | No | `internal/config/config.go`, `internal/agent` | No | Caps planner tool-call iterations per run; must be between 1 and 6. |
| `BI_AGENT_MAX_CLARIFICATION_ROUNDS` | `2` | `2` | No | `internal/config/config.go`, `internal/agent` | No | Caps clarification round-trips per run; must be between 0 and 2. |
| `BI_AGENT_TIMEOUT` | `"45s"` | `"45s"` | No | `internal/config/config.go`, `internal/agent` | No | Bounds total run wall-clock time; must be between 1s and 45s. |
| `BI_AGENT_MAX_ROWS` | `1000` | `1000` | No | `internal/config/config.go`, `internal/agent` | No | Caps rows returned by any `query.execute` tool call; must be between 1 and 1000. |
| `BI_AGENT_JOB_SUBJECT` | `"biqly.agent.jobs"` | `"biqly.agent.jobs"` | No | `internal/config/config.go`, `internal/agent` | No | NATS subject the agent job queue publishes/consumes on. |
| `BI_AGENT_STEP_SUBJECT` | `"biqly.agent.steps"` | `"biqly.agent.steps"` | No | `internal/config/config.go`, `internal/agent` | No | NATS subject the planner publishes per-step trace events on. |
| `BI_AGENT_RESULT_SUBJECT` | `"biqly.agent.results"` | `"biqly.agent.results"` | No | `internal/config/config.go`, `internal/agent` | No | NATS subject completed run results are published on. |
| `BI_AGENT_ERROR_SUBJECT` | `"biqly.agent.errors"` | `"biqly.agent.errors"` | No | `internal/config/config.go`, `internal/agent` | No | NATS subject failed run errors are published on. |
| `BI_AGENT_WORKSPACE_ALLOWLIST` | `""` | `""` | No | `internal/config/config.go`, `internal/agent` | No | Comma-separated workspace IDs the agent pipeline is restricted to during rollout; empty means all workspaces. |
| `BI_AGENT_LEGACY_FALLBACK_ENABLED` | `true` | `true` | No | `internal/config/config.go`, `internal/agent` | No | Falls back to the legacy NL-to-SQL pipeline when a run fails, times out, or its workspace is outside the allowlist. |
| **Web Agent Mode** | | | | | | |
| `BI_WEB_AGENT_ENABLED` | `false` | N/A | No | `internal/config/config.go`, `/api/agent/chat` | No | Feature flag for the in-request SSE web agent path. Disabled by default. |
| `BI_WEB_AGENT_MAX_STEPS` | `6` | N/A | No | `internal/config/config.go`, `internal/agent.Runtime` | No | Caps web-agent planner tool-call iterations per run; must be between 1 and 6. |
| `BI_WEB_AGENT_MAX_CLARIFICATION_ROUNDS` | `2` | N/A | No | `internal/config/config.go`, `internal/agent.Runtime` | No | Caps web-agent clarification round-trips per run; must be between 0 and 2. |
| `BI_WEB_AGENT_TIMEOUT` | `"120s"` | N/A | No | `internal/config/config.go`, `/api/agent/chat` | No | Bounds total web-agent request time; must be between 1s and 120s. Gateway HTTPRoute timeout is handled separately. |
| `BI_WEB_AGENT_WORKSPACE_ALLOWLIST` | falls back to `BI_AGENT_WORKSPACE_ALLOWLIST` | N/A | No | `internal/config/config.go`, `/api/agent/chat` | No | Comma-separated workspace IDs allowed to use web agent mode. If unset, reuses `BI_AGENT_WORKSPACE_ALLOWLIST`; empty means all workspaces. |
| **Databases, Queue & Query Engine** | | | | | | |
| `BI_METADATA_DB_DSN` | `postgres://localhost:5432/...` | Required (Secret) | No | `internal/platform/db` | No | Connection string for Metadata Postgres DB. |
| `BI_REDIS_DSN` | `redis://localhost:6379` | `redis://biqly-dragonfly:6379` | No | `internal/config/config.go` | No | Connection string for Dragonfly (Redis). |
| `BI_NATS_URL` | `""` | `nats://biqly-nats:4222` | No | `internal/queue/nats.go` | No | Connection string for NATS JetStream. |
| `BI_NATS_STREAM` | `"BIQLY_AI_JOBS"` | `"BIQLY_AI_JOBS"` | No | `internal/queue/nats.go` | No | JetStream stream name. |
| `BI_NATS_SUBJECT` | `"biqly.ai.jobs"` | `"biqly.ai.jobs"` | No | `internal/queue/nats.go` | No | NATS subject routing key. |
| `BI_NATS_CONSUMER_GROUP` | `"biqly-ai-workers"` | `"biqly-ai-workers"` | No | `internal/queue/nats.go` | No | NATS consumer group. |
| `BI_AI_JOBS_ENABLED` | `true` | `true` | No | `internal/app` / workers | No | Toggle processing of async AI jobs on NATS queues. |
| `BI_AI_JOBS_CONSUMER_ENABLED` | `true` | `false` (AI API), `true` (worker) | No | `cmd/api`, `services/ai/cmd`, `cmd/worker` | No | Enables the in-process AI job consumer; disable on API pods when standalone workers are deployed. |
| `BI_AI_JOBS_CONCURRENCY` | `1` | `1` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | No | Max concurrent AI jobs processed per worker; overridable in Administration → Platform Settings → AI Jobs Queue. |
| `BI_QUERY_TIMEOUT_SECONDS` | `30` | `30` | No | `internal/query` | No | Maximum time limit for query execution on databases. |
| `BI_QUERY_MAX_ROWS` | `10000` | `10000` | No | `internal/query` | No | Maximum rows returned by query execution to avoid OOM. |
| `BI_QUERY_MAX_RUNTIME_SECONDS` | `60` | `60` | No | `internal/query` | No | Maximum runtime for a single query execution. |
| `BI_QUERY_HISTORY_LIST_LIMIT` | `100` | N/A | No | `internal/query` | No | Maximum query history records shown in the UI. |
| `BI_EVAL_RUNS_LIST_LIMIT` | `50` | N/A | No | `internal/query` | No | Maximum evaluation run records shown in the list UI. |
| `BI_COMPOSITE_MAX_COMPONENTS` | `8` | N/A | No | `internal/config/config.go`, `internal/semantic/composite_publish.go` | No | Max component tables allowed in a composite database view. |
| `BI_COMPOSITE_MAX_CROSS_JOINS` | `16` | N/A | No | `internal/config/config.go`, `internal/semantic/composite_publish.go` | No | Max cross joins allowed in composite query resolution. |
| `BI_COMPOSITE_MAX_MERGED_FIELDS` | `300` | N/A | No | `internal/config/config.go`, `internal/semantic/composite_publish.go` | No | Max columns/fields allowed in a merged composite schema. |
| **Authentication & Security** | | | | | | |
| `BI_ENCRYPTION_KEY` | (Placeholder) | Required (Secret) | No | `internal/security` | No | AES key for encrypting provider keys in DB. |
| `BI_ADMIN_API_KEY` | `""` | Required (Secret) | No | `internal/config/config.go`, `internal/http/auth_middleware.go` | No | Shared token for machine-to-machine admin endpoints (CLI/CI). |
| `BI_INTERNAL_API_TOKEN` | `""` | Required (Secret) | No | `internal/config/config.go`, `internal/http/auth_middleware.go` | No | Token used for secure BFF-to-microservice peer calls. |
| `BI_API_KEY` | `""` | `""` | No | `internal/config/config.go`, `internal/http/auth_middleware.go` | No | Shared API key when JWT auth is disabled. |
| `BI_METRICS_API_KEY` | `""` | `""` | No | `internal/config/config.go`, `internal/http/auth_middleware.go` | No | Token to restrict scraping of the `/metrics` endpoint. |
| `BI_AUTH_ENABLED` | `false` | `true` | No | `internal/config/config.go`, `internal/http/auth_middleware.go` | No | Enables JWT authentication via Auth service. |
| `BI_AUTH_SERVICE_URL` | `""` | `http://biqly-auth:8889` | No | `internal/config/config.go`, `internal/http/auth_middleware.go` | No | Auth service public key fetch location. |
| `BI_AUTH_INTERNAL_TOKEN` | `""` | Required (Secret) | No | `internal/config/config.go`, `internal/http/auth_middleware.go` | No | BFF communication token for Auth. |
| `BI_AUTH_MAIL_SERVICE_URL` | `"http://localhost:8890"` | `http://biqly-mail:8080` (or similar) | No | `internal/auth` | No | URL of the mail notification microservice. |
| `BI_AUTH_MAIL_INTERNAL_TOKEN` | `""` | Required (Secret) | No | `internal/auth` | No | Peer token for secure calls to the mail service. |
| `BI_AUTH_FRONTEND_BASE_URL` | `"http://localhost:3333"` | Required | No | `internal/auth` | No | Base URL of the web UI for redirection/mail link generation. |
| **AI Settings (Core LLM & Cache)** | | | | | | |
| `BI_AI_HTTP_TIMEOUT_SECONDS`| `300` | `300` | No | `internal/config/config.go` (HTTPTimeout) | No | HTTP timeout for chat/completion calls. |
| `BI_AI_RATE_LIMIT_PER_MINUTE`| `20` | `20` | No | `internal/config/config.go` | No | Saved to DB configuration defaults but not active-throttled. |
| `BI_AI_MAX_PROMPT_RUNES` | `80000` | `40000` | No | `internal/config/config.go`, `internal/http/handlers/ai_settings.go` | No | Env fallback for prompt-size budgeting; exposed read-only in `/api/ai/settings`. Helm override is smaller. |
| `BI_AI_MAX_RETRIES` | `2` | `1` | No | `internal/config/config.go`, `internal/ai/service.go` | No | LLM error recovery retry count. Helm override is smaller. |
| `BI_AI_MULTI_CANDIDATE_COUNT`| `1` | `1` | No | `internal/ai/service.go` | No | Chat completion alternatives count. |
| `BI_AI_WORKSPACE_DAILY_TOKEN_BUDGET` | `0` | `0` | No | `internal/config/config.go`, `internal/ai/spend_limit.go` | No | Per-workspace daily LLM token cap (prompt+completion). `0` disables. On exceed, AI query endpoints return HTTP 429. Counters are stored in Redis (fails open if Redis is down). |
| `BI_AI_ANSWER_ENABLED` | `true` | `true` | No | `internal/config/config.go`, `internal/ai/answer.go` | No | Enables post-execution natural-language answer synthesis: a separate lightweight LLM call that summarizes the query result in 1-2 sentences in the user's locale. Gated by the workspace spend limiter; best-effort (never fails the query). |
| `BI_AI_RESPONSE_CACHE_TTL` | `3600` | `3600` | No | `internal/ai/service.go` | No | TTL for cached LLM query responses. |
| `BI_AI_DESCRIBE_MAX_CELL_RUNES`| `500` | `500` | No | `internal/config/config.go`, `internal/app/ai_dependencies.go` | No | String cell truncation length for database schema description. |
| `BI_AI_DESCRIBE_MAX_SAMPLE_ROWS`| `12` | `12` | No | `internal/config/config.go`, `internal/app/ai_dependencies.go` | No | Max database rows fetched for schema sampling. |
| **AI Settings (Embedding & Translation)** | | | | | | |
| `BI_AI_EMBEDDING_HTTP_TIMEOUT_SECONDS`| `600` | `1200` | No | `internal/config/config.go`, `internal/app/ai_dependencies.go` | No | HTTP timeout for embedding API calls. |
| `BI_AI_EMBEDDING_WEIGHT` | `30.0` | `30.0` | No | `internal/ai/routing/router.go` | No | Weight of embeddings in hybrid routing calculation. |
| `BI_AI_EMBEDDING_DENY_SCHEMAS`| `""` | `""` | No | `internal/ai/routing/router.go` | No | Comma-separated list of database schemas excluded from embedding. |
| `BI_AI_EMBEDDING_DENY_TABLES` | `""` | `""` | No | `internal/ai/routing/router.go` | No | Comma-separated list of schema.table pairs excluded from embedding. |
| `BI_AI_TRANSLATION_TARGET_LANGUAGE`| `"Turkish"` | `"Turkish"` | No | `internal/config/config.go`, `internal/app/ai_dependencies.go` | No | Target language for autogenerated schema comments. |
| `BI_AI_TRANSLATION_TARGET_CODE`| `"tr"` | `"tr"` | No | `internal/config/config.go`, `internal/app/ai_dependencies.go` | No | BCP-47 locale tag for translation target. |
| `BI_AI_TRANSLATION_HTTP_TIMEOUT_SECONDS`| `120` | `120` | No | `internal/config/config.go`, `internal/app/ai_dependencies.go` | No | HTTP timeout for translation API calls. |
| **AI Settings (Table Routing)** | | | | | | |
| `BI_AI_ROUTING_LEXICON_PATH` | `""` | `""` | No | `internal/config/config.go`, `internal/ai/routing/router.go` | No | Path to synonym/vocabulary override JSON file. |
| `BI_AI_ROUTING_WEIGHTS_PATH` | `""` | `""` | No | `internal/config/config.go`, `internal/ai/routing/router.go` | No | Path to routing score overrides JSON file. |
| `BI_AI_ROUTE_MAX_DIMENSIONS` | `0` | `56` (Helm override)| No | `internal/ai/routing/router.go` | No | Max dimensions auto-generated per semantic model. |
| `BI_AI_ROUTE_MAX_METRICS` | `0` | `32` (Helm override)| No | `internal/ai/routing/router.go` | No | Max metrics auto-generated per semantic model. |
| `BI_AI_ROUTE_MAX_COLUMNS_PER_TABLE`| `0` | `14` (Helm override)| No | `internal/ai/routing/router.go` | No | Max columns scored per database table in hybrid routing. |
| `BI_AI_ROUTE_MAX_DATE_GRAIN_EXTRAS`| `0` | `20` (Helm override)| No | `internal/ai/routing/router.go` | No | Max date grain variations computed per date/time field. |
| `BI_AI_ROUTE_SLIM_NUMERIC_METRICS`| `true` | `true` | No | `internal/ai/routing/router.go` | No | Exclude avg_/min_ metrics for faster generation. |
| **AI Settings (Ambiguity & Memory)** | | | | | | |
| `BI_AI_AMBIGUITY_CHECK_ENABLED`| `true` | `true` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Toggle pre-LLM clarification workflow. |
| `BI_AI_AMBIGUITY_CONFIDENCE_THRESHOLD`| `0.70`| `0.70` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Scoring floor where ambiguity checks kick in. |
| `BI_AI_AMBIGUITY_MAX_OPTIONS` | `5` | `5` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Max options returned to user for clarification cards. |
| `BI_AI_AMBIGUITY_LLM_ENABLED` | `false` | `false` | No | `internal/config/config.go`, `internal/ai/service.go` | No | Toggles LLM-driven backup ambiguity scoring. This flag is env-only today. |
| `BI_AI_AMBIGUITY_TIERED_ENABLED`| `false` | `true` (Helm override) | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Runs synonym checks before scope checks. |
| `BI_AI_AMBIGUITY_MAX_LLM_TIER_PER_QUESTION`| `1` | `1` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Limits how many rounds can invoke LLM ambiguity checks. |
| `BI_AI_CLARIFY_POLICY_ENABLED`| `true` | `true` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_ambiguity_tier.go` | No | Clarify only for genuine toss-ups; otherwise proceed with the top interpretation plus a one-line caveat. Disable to restore always-clarify. |
| `BI_AI_MEMORY_RECALL_ENABLED` | `true` | `true` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Enables injection of confirmed-query examples. |
| `BI_AI_MEMORY_RECALL_LIMIT` | `5` | `5` | Yes | `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Max count of few-shot example insertions. |
| **PII Detection & Masking** | | | | | | |
| `BI_PII_ENABLED` | `true` | `true` | No | `internal/security/pii` | No | Master switch for PII scanning and masking. |
| `BI_PII_DETECTION_THRESHOLD` | `0.6` | `0.6` | Yes | `internal/security/pii/scanner.go`, `internal/http/handlers/ai_admin_config.go` | **Yes** | Minimum PII detection confidence score. |
| `BI_PII_SAMPLE_DATA_LIMIT` | `50` | `50` | No | `internal/security/pii/scanner.go` | No | Count of database records sampled during PII scans. |
| `BI_PII_AUTO_SCAN_ON_SYNC` | `true` | `true` | No | `internal/security/pii/scanner.go` | No | Trigger automatic PII scanning on metadata sync. |
| `BI_PII_DEFAULT_MASKING_STRATEGY`| `"partial"`| `"partial"` | No | `internal/config/config.go`, `internal/security/pii/masking.go` | No | Default masking strategy name applied to PII columns. This knob is env-only today. |

## Key Insights & Architecture

1. **Hierarchy of Overrides**:
   - Every variable has a **Hardcoded Default** in `internal/config/config.go`.
   - **Helm configuration** (defined in `values.yaml` and subcharts) overrides these at startup via env variables.
   - For variables marked **Runtime Override? = Yes**, the values are loaded from the database (`ai_runtime_config` table) by `internal/http/handlers/ai_admin_config.go`. Today those override domains are ambiguity, `pii.detection_threshold`, memory recall, and AI job queue concurrency; if a database setting is set, it overrides both code defaults and env values dynamically at runtime.
2. **Provider & Model Selection**:
   - Connection/model routing targets (like OpenAI API Keys, model names, and base URLs) are **not** managed via environment variables.
   - They are configured dynamically in the administration panel under `Administration -> AI Providers` and stored in the database (`ai_providers` / `ai_models` tables), allowing live rotation of provider keys.
3. **Agentic Query Runner operations**: the `BI_AGENT_*` variables above control the NATS-driven runner (no runtime DB override yet). The `BI_WEB_AGENT_*` variables control the separate in-request SSE web path and default off. For metrics, alerts, the fallback-boundary caveat, and the `cmd/conversation-repair` CLI, see `docs/agents/agent-runbook.md`.
