#!/usr/bin/env bash
# Generate BI_AUTH_* secret values for local dev and Kubernetes.
# BI_AUTH_ENCRYPTION_KEY must be base64 encoding of exactly 32 random bytes (AES-256-GCM).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-}"

gen_b64_32() {
  openssl rand -base64 32 | tr -d '\n'
}

gen_token() {
  openssl rand -hex 32
}

quote_env() {
  printf '%s' "$1" | sed "s/'/'\\\\''/g"
}

BI_AUTH_ENCRYPTION_KEY="$(gen_b64_32)"
BI_AUTH_INTERNAL_TOKEN="$(gen_token)"

emit() {
  local enc int
  enc="$(quote_env "$BI_AUTH_ENCRYPTION_KEY")"
  int="$(quote_env "$BI_AUTH_INTERNAL_TOKEN")"
  cat <<EOF
# --- Auth service secrets (generated $(date -u +%Y-%m-%dT%H:%M:%SZ)) ---
# AES-256-GCM: base64-encoded 32 random bytes. Do not rotate without a data migration plan.
BI_AUTH_ENCRYPTION_KEY='${enc}'

# Shared secret for catalog/query/ai -> auth internal API
BI_AUTH_INTERNAL_TOKEN='${int}'

# Main API / datasource DSN encryption — keep identical to BI_AUTH_ENCRYPTION_KEY when using one key
BI_ENCRYPTION_KEY='${enc}'
EOF
}

upsert_env_file() {
  local file="$1"
  local tmp
  tmp="$(mktemp)"
  awk -v enc="$BI_AUTH_ENCRYPTION_KEY" -v int="$BI_AUTH_INTERNAL_TOKEN" '
    BEGIN { skip = 0 }
    /^# --- Auth service secrets / { skip = 1; next }
    skip && /^BI_(AUTH_|ENCRYPTION_KEY=)/ { next }
    skip && /^$/ { skip = 0; next }
    skip && /^[^#]/ { skip = 0 }
    /^BI_AUTH_ENCRYPTION_KEY=/ { next }
    /^BI_AUTH_INTERNAL_TOKEN=/ { next }
    { print }
  ' "$file" >"$tmp"
  mv "$tmp" "$file"
  printf '\n' >>"$file"
  emit >>"$file"
}

if [[ -n "$OUT" ]]; then
  touch "$OUT"
  upsert_env_file "$OUT"
  chmod 600 "$OUT"
  echo "Updated $OUT (mode 600). Replaced any previous auth-secrets block."
else
  emit
  echo ""
  echo "Where to put these values:"
  echo "  1) Local: ./scripts/generate-auth-secrets.sh $ROOT/.env"
  echo "  2) Helm (dev only): biqly-gitops deploy/helm/biqly/values.yaml -> global.secrets.*"
  echo "  3) Prod K8s: ./scripts/sync-env-to-k8s.sh $ROOT/.env"
  echo ""
  echo "Also required (not generated here): BI_AUTH_JWT_PRIVATE_KEY (PEM), BI_AUTH_DB_DSN"
fi
