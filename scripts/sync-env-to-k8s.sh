#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${1:-$ROOT/.env}"
NS="${BI_K8S_NAMESPACE:-biqly}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing env file: $ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

pg_user="${BI_PG_USER:-biqly}"
pg_pass="$(kubectl -n "$NS" get secret biqly-postgresql-auth -o "go-template={{index .data \"password\" | base64decode}}" 2>/dev/null || true)"
if [[ -z "$pg_pass" ]]; then
  echo "could not read biqly-postgresql-auth password in namespace $NS" >&2
  exit 1
fi

dsn="postgres://${pg_user}:${pg_pass}@biqly-postgresql:5432/bi_metadata?sslmode=disable"
redis_dsn="${BI_REDIS_DSN:-redis://biqly-dragonfly:6379}"
redis_dsn="${redis_dsn//localhost/biqly-dragonfly}"
redis_dsn="${redis_dsn//127.0.0.1/biqly-dragonfly}"

internal_token="${BI_INTERNAL_API_TOKEN:-${BI_ADMIN_API_KEY:-}}"
auth_internal_token="${BI_AUTH_INTERNAL_TOKEN:-$internal_token}"
auth_encryption_key="${BI_AUTH_ENCRYPTION_KEY:-${BI_ENCRYPTION_KEY:-}}"
auth_pg_user="${BI_AUTH_PG_USER:-${BI_PG_USER:-biqly}}"
auth_pg_db="${BI_AUTH_PG_DB:-bi_auth}"
auth_dsn="${BI_AUTH_DB_DSN:-postgres://${auth_pg_user}:${pg_pass}@biqly-postgresql:5432/${auth_pg_db}?sslmode=disable}"

if [[ -z "$auth_encryption_key" ]]; then
  echo "set BI_AUTH_ENCRYPTION_KEY or BI_ENCRYPTION_KEY in $ENV_FILE (see scripts/generate-auth-secrets.sh)" >&2
  exit 1
fi

kubectl -n "$NS" create secret generic biqly-db \
  --from-literal="BI_METADATA_DB_DSN=$dsn" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create secret generic biqly-security \
  --from-literal="BI_ENCRYPTION_KEY=${BI_ENCRYPTION_KEY:?}" \
  --from-literal="BI_ADMIN_API_KEY=${BI_ADMIN_API_KEY:-}" \
  --from-literal="BI_INTERNAL_API_TOKEN=$internal_token" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create secret generic biqly-ai-secrets \
  --from-literal="BI_AI_API_KEY=${BI_AI_API_KEY:-}" \
  --from-literal="BI_AI_QUERY_API_KEY=${BI_AI_QUERY_API_KEY:-}" \
  --from-literal="BI_AI_TRANSLATION_API_KEY=${BI_AI_TRANSLATION_API_KEY:-}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create secret generic biqly-embedding-secrets \
  --from-literal="BI_AI_EMBEDDING_API_KEY=${BI_AI_EMBEDDING_API_KEY:-}" \
  --dry-run=client -o yaml | kubectl apply -f -

auth_secret_args=(
  --from-literal="BI_AUTH_ENCRYPTION_KEY=$auth_encryption_key"
  --from-literal="BI_AUTH_INTERNAL_TOKEN=$auth_internal_token"
)
if [[ -n "${BI_AUTH_JWT_PRIVATE_KEY_PATH:-}" && -f "$BI_AUTH_JWT_PRIVATE_KEY_PATH" ]]; then
  auth_secret_args+=(--from-file="BI_AUTH_JWT_PRIVATE_KEY=$BI_AUTH_JWT_PRIVATE_KEY_PATH")
elif [[ -n "${BI_AUTH_JWT_PRIVATE_KEY:-}" ]]; then
  auth_secret_args+=(--from-literal="BI_AUTH_JWT_PRIVATE_KEY=$BI_AUTH_JWT_PRIVATE_KEY")
else
  echo "set BI_AUTH_JWT_PRIVATE_KEY (PEM) or BI_AUTH_JWT_PRIVATE_KEY_PATH in $ENV_FILE" >&2
  exit 1
fi
[[ -n "${BI_AUTH_GITHUB_CLIENT_ID:-}" ]] && auth_secret_args+=(--from-literal="BI_AUTH_GITHUB_CLIENT_ID=$BI_AUTH_GITHUB_CLIENT_ID")
[[ -n "${BI_AUTH_GITHUB_CLIENT_SECRET:-}" ]] && auth_secret_args+=(--from-literal="BI_AUTH_GITHUB_CLIENT_SECRET=$BI_AUTH_GITHUB_CLIENT_SECRET")
[[ -n "${BI_AUTH_GOOGLE_CLIENT_ID:-}" ]] && auth_secret_args+=(--from-literal="BI_AUTH_GOOGLE_CLIENT_ID=$BI_AUTH_GOOGLE_CLIENT_ID")
[[ -n "${BI_AUTH_GOOGLE_CLIENT_SECRET:-}" ]] && auth_secret_args+=(--from-literal="BI_AUTH_GOOGLE_CLIENT_SECRET=$BI_AUTH_GOOGLE_CLIENT_SECRET")
[[ -n "${BI_AUTH_SMTP_USER:-}" ]] && auth_secret_args+=(--from-literal="BI_AUTH_SMTP_USER=$BI_AUTH_SMTP_USER")
[[ -n "${BI_AUTH_SMTP_PASS:-}" ]] && auth_secret_args+=(--from-literal="BI_AUTH_SMTP_PASS=$BI_AUTH_SMTP_PASS")

kubectl -n "$NS" create secret generic biqly-auth-secrets \
  "${auth_secret_args[@]}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create secret generic biqly-auth-db \
  --from-literal="BI_AUTH_DB_DSN=$auth_dsn" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create configmap biqly-config \
  --from-literal="BI_LOG_LEVEL=${BI_LOG_LEVEL:-info}" \
  --from-literal="BI_LOG_FORMAT=${BI_LOG_FORMAT:-json}" \
  --from-literal="BI_QUERY_TIMEOUT_SECONDS=${BI_QUERY_TIMEOUT_SECONDS:-30}" \
  --from-literal="BI_QUERY_MAX_ROWS=${BI_QUERY_MAX_ROWS:-10000}" \
  --from-literal="BI_QUERY_MAX_RUNTIME_SECONDS=${BI_QUERY_MAX_RUNTIME_SECONDS:-60}" \
  --from-literal="BI_REDIS_DSN=$redis_dsn" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create configmap biqly-ai-config-provider \
  --from-literal="BI_AI_PROVIDER=${BI_AI_PROVIDER:-openai}" \
  --from-literal="BI_AI_MODEL=${BI_AI_MODEL:-gpt-4o}" \
  --from-literal="BI_AI_TEMPERATURE=${BI_AI_TEMPERATURE:-0}" \
  --from-literal="BI_AI_EMBEDDING_MODEL=${BI_AI_EMBEDDING_MODEL:-}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create configmap biqly-ai-config \
  --from-literal="BI_HTTP_PORT=8082" \
  --from-literal="BI_CATALOG_SERVICE_URL=http://biqly-catalog:8080" \
  --from-literal="BI_QUERY_SERVICE_URL=http://biqly-query:8081" \
  --from-literal="BI_AI_BASE_URL=${BI_AI_BASE_URL:-}" \
  --from-literal="BI_AI_QUERY_PROVIDER=${BI_AI_QUERY_PROVIDER:-}" \
  --from-literal="BI_AI_QUERY_BASE_URL=${BI_AI_QUERY_BASE_URL:-}" \
  --from-literal="BI_AI_QUERY_MODEL=${BI_AI_QUERY_MODEL:-}" \
  --from-literal="BI_AI_EMBEDDING_BASE_URL=${BI_AI_EMBEDDING_BASE_URL:-}" \
  --from-literal="BI_AI_TRANSLATION_MODEL=${BI_AI_TRANSLATION_MODEL:-}" \
  --from-literal="BI_AI_TRANSLATION_BASE_URL=${BI_AI_TRANSLATION_BASE_URL:-}" \
  --from-literal="BI_AI_TRANSLATION_TARGET_LANGUAGE=${BI_AI_TRANSLATION_TARGET_LANGUAGE:-}" \
  --from-literal="BI_AI_TRANSLATION_TARGET_CODE=${BI_AI_TRANSLATION_TARGET_CODE:-tr}" \
  --from-literal="BI_AI_TRANSLATION_HTTP_TIMEOUT_SECONDS=${BI_AI_TRANSLATION_HTTP_TIMEOUT_SECONDS:-120}" \
  --from-literal="BI_AI_MAX_TOKENS=${BI_AI_MAX_TOKENS:-4096}" \
  --from-literal="BI_AI_TOP_P=${BI_AI_TOP_P:-0}" \
  --from-literal="BI_AI_NUM_CTX=${BI_AI_NUM_CTX:-0}" \
  --from-literal="BI_AI_RATE_LIMIT_PER_MINUTE=${BI_AI_RATE_LIMIT_PER_MINUTE:-20}" \
  --from-literal="BI_AI_MAX_PROMPT_RUNES=${BI_AI_MAX_PROMPT_RUNES:-80000}" \
  --from-literal="BI_AI_DESCRIBE_MAX_CELL_RUNES=${BI_AI_DESCRIBE_MAX_CELL_RUNES:-500}" \
  --from-literal="BI_AI_DESCRIBE_MAX_SAMPLE_ROWS=${BI_AI_DESCRIBE_MAX_SAMPLE_ROWS:-12}" \
  --from-literal="BI_AI_HTTP_TIMEOUT_SECONDS=${BI_AI_HTTP_TIMEOUT_SECONDS:-300}" \
  --from-literal="BI_AI_EMBEDDING_HTTP_TIMEOUT_SECONDS=${BI_AI_EMBEDDING_HTTP_TIMEOUT_SECONDS:-600}" \
  --from-literal="BI_AI_ROUTE_MAX_DIMENSIONS=${BI_AI_ROUTE_MAX_DIMENSIONS:-0}" \
  --from-literal="BI_AI_ROUTE_MAX_METRICS=${BI_AI_ROUTE_MAX_METRICS:-0}" \
  --from-literal="BI_AI_ROUTE_MAX_COLUMNS_PER_TABLE=${BI_AI_ROUTE_MAX_COLUMNS_PER_TABLE:-0}" \
  --from-literal="BI_AI_ROUTE_MAX_DATE_GRAIN_EXTRAS=${BI_AI_ROUTE_MAX_DATE_GRAIN_EXTRAS:-0}" \
  --from-literal="BI_AI_ROUTE_SLIM_NUMERIC_METRICS=${BI_AI_ROUTE_SLIM_NUMERIC_METRICS:-true}" \
  --from-literal="BI_LOG_LEVEL=${BI_LOG_LEVEL:-info}" \
  --from-literal="BI_LOG_FORMAT=${BI_LOG_FORMAT:-json}" \
  --from-literal="BI_QUERY_TIMEOUT_SECONDS=${BI_QUERY_TIMEOUT_SECONDS:-30}" \
  --from-literal="BI_QUERY_MAX_ROWS=${BI_QUERY_MAX_ROWS:-10000}" \
  --from-literal="BI_QUERY_MAX_RUNTIME_SECONDS=${BI_QUERY_MAX_RUNTIME_SECONDS:-60}" \
  --from-literal="BI_REDIS_DSN=$redis_dsn" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create configmap biqly-catalog-config \
  --from-literal="BI_HTTP_PORT=8080" \
  --from-literal="BI_LOG_LEVEL=${BI_LOG_LEVEL:-info}" \
  --from-literal="BI_LOG_FORMAT=${BI_LOG_FORMAT:-json}" \
  --from-literal="BI_QUERY_TIMEOUT_SECONDS=${BI_QUERY_TIMEOUT_SECONDS:-30}" \
  --from-literal="BI_QUERY_MAX_ROWS=${BI_QUERY_MAX_ROWS:-10000}" \
  --from-literal="BI_QUERY_MAX_RUNTIME_SECONDS=${BI_QUERY_MAX_RUNTIME_SECONDS:-60}" \
  --from-literal="BI_REDIS_DSN=$redis_dsn" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NS" create configmap biqly-query-config \
  --from-literal="BI_HTTP_PORT=8081" \
  --from-literal="BI_CATALOG_SERVICE_URL=http://biqly-catalog:8080" \
  --from-literal="BI_LOG_LEVEL=${BI_LOG_LEVEL:-info}" \
  --from-literal="BI_LOG_FORMAT=${BI_LOG_FORMAT:-json}" \
  --from-literal="BI_QUERY_TIMEOUT_SECONDS=${BI_QUERY_TIMEOUT_SECONDS:-30}" \
  --from-literal="BI_QUERY_MAX_ROWS=${BI_QUERY_MAX_ROWS:-10000}" \
  --from-literal="BI_QUERY_MAX_RUNTIME_SECONDS=${BI_QUERY_MAX_RUNTIME_SECONDS:-60}" \
  --from-literal="BI_REDIS_DSN=$redis_dsn" \
  --dry-run=client -o yaml | kubectl apply -f -

if [[ "${BI_SYNC_SMTP_FROM_ZLITTER:-1}" == "1" ]] && [[ -x "$ROOT/scripts/sync-smtp-from-zlitter.sh" ]]; then
  ZLITTER_NAMESPACE="${ZLITTER_NAMESPACE:-zlitter}" \
  BI_K8S_NAMESPACE="$NS" \
  "$ROOT/scripts/sync-smtp-from-zlitter.sh"
else
  rollout_targets=(deployment/biqly-ai deployment/biqly-catalog deployment/biqly-query)
  if kubectl -n "$NS" get deployment biqly-auth >/dev/null 2>&1; then
    rollout_targets+=(deployment/biqly-auth)
  fi
  kubectl -n "$NS" rollout restart "${rollout_targets[@]}"
fi

echo "synced $ENV_FILE -> namespace/$NS (secrets + configmaps, workloads restarted)"
