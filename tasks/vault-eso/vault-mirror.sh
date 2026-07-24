#!/usr/bin/env bash
# rpi-4'te sudo ile çalışır. stdin'den NDJSON ({name, data:{k:v}}) okur,
# her K8s secret'ını Vault biqly/ KV v2'ye AYNEN yazar. Değer basmaz; anahtar adı raporlar.
set -euo pipefail
TOKEN=$(jq -r .root_token /root/vault-init.json)
ADDR=https://127.0.0.1:8200
declare -A MAP=(
  [biqly-db]=db [biqly-security]=security
  [biqly-auth-secrets]=auth [biqly-auth-db]=auth-db
  [biqly-ai-secrets]=ai [biqly-embedding-secrets]=embedding
  [biqly-mail-secrets]=mail [biqly-mail-db]=mail-db
  [biqly-otel-secrets]=otel
)
while IFS= read -r line; do
  [ -z "$line" ] && continue
  name=$(jq -r .name <<<"$line")
  path=${MAP[$name]:-}
  if [ -z "$path" ]; then echo "SKIP (haritada yok): $name"; continue; fi
  keys=$(jq -r '.data|keys|join(",")' <<<"$line")
  body=$(jq -c '{data: .data}' <<<"$line")
  code=$(curl -sk -o /dev/null -w '%{http_code}' -H "X-Vault-Token: $TOKEN" -X POST -d "$body" "$ADDR/v1/biqly/data/$path")
  echo "PUT biqly/$path  <= $name  [$keys]  http=$code"
done
echo MIRROR_DONE
