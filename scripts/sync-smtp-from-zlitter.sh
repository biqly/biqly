#!/usr/bin/env bash
set -euo pipefail

SOURCE_NS="${ZLITTER_NAMESPACE:-zlitter}"
SOURCE_SECRET="${ZLITTER_SMTP_SECRET:-zlitter-smtp}"
TARGET_NS="${BI_K8S_NAMESPACE:-biqly}"
TARGET_SECRET="${BI_AUTH_SECRET_NAME:-biqly-auth-secrets}"
TARGET_CONFIGMAP="${BI_AUTH_CONFIGMAP_NAME:-biqly-auth-config}"

decode_key() {
  local secret=$1 key=$2
  kubectl -n "$SOURCE_NS" get secret "$secret" -o "go-template={{index .data \"${key}\" | base64decode}}"
}

require_key() {
  local secret=$1 key=$2
  local val
  val="$(decode_key "$secret" "$key" 2>/dev/null || true)"
  if [[ -z "$val" ]]; then
    echo "missing key $key in secret $SOURCE_NS/$secret" >&2
    exit 1
  fi
  printf '%s' "$val"
}

smtp_host="$(require_key "$SOURCE_SECRET" SMTP_HOST)"
smtp_port="$(require_key "$SOURCE_SECRET" SMTP_PORT)"
smtp_from="$(require_key "$SOURCE_SECRET" SMTP_FROM)"
smtp_user="$(require_key "$SOURCE_SECRET" SMTP_USERNAME)"
smtp_pass="$(require_key "$SOURCE_SECRET" SMTP_PASSWORD)"

if ! kubectl -n "$TARGET_NS" get secret "$TARGET_SECRET" >/dev/null 2>&1; then
  echo "target secret $TARGET_NS/$TARGET_SECRET not found; run scripts/sync-env-to-k8s.sh first" >&2
  exit 1
fi

smtp_patch="$(jq -n \
  --arg u "$smtp_user" \
  --arg p "$smtp_pass" \
  '{stringData: {BI_AUTH_SMTP_USER: $u, BI_AUTH_SMTP_PASS: $p}}')"
kubectl -n "$TARGET_NS" patch secret "$TARGET_SECRET" --type merge -p "$smtp_patch"

if kubectl -n "$TARGET_NS" get configmap "$TARGET_CONFIGMAP" >/dev/null 2>&1; then
  cm_patch="$(jq -n \
    --arg h "$smtp_host" \
    --arg p "$smtp_port" \
    --arg f "$smtp_from" \
    '{data: {BI_AUTH_SMTP_HOST: $h, BI_AUTH_SMTP_PORT: $p, BI_AUTH_SMTP_FROM: $f}}')"
  kubectl -n "$TARGET_NS" patch configmap "$TARGET_CONFIGMAP" --type merge -p "$cm_patch"
else
  echo "configmap $TARGET_NS/$TARGET_CONFIGMAP not found; SMTP host/port/from will come from helm on next sync" >&2
fi

if kubectl -n "$TARGET_NS" get deployment biqly-auth >/dev/null 2>&1; then
  if ! kubectl -n "$TARGET_NS" get deployment biqly-auth -o yaml | rg -q 'name: BI_AUTH_SMTP_PASS'; then
    kubectl -n "$TARGET_NS" patch deployment biqly-auth --type=json -p="[
      {\"op\":\"add\",\"path\":\"/spec/template/spec/containers/0/env/-\",\"value\":{\"name\":\"BI_AUTH_SMTP_USER\",\"valueFrom\":{\"secretKeyRef\":{\"name\":\"$TARGET_SECRET\",\"key\":\"BI_AUTH_SMTP_USER\"}}}},
      {\"op\":\"add\",\"path\":\"/spec/template/spec/containers/0/env/-\",\"value\":{\"name\":\"BI_AUTH_SMTP_PASS\",\"valueFrom\":{\"secretKeyRef\":{\"name\":\"$TARGET_SECRET\",\"key\":\"BI_AUTH_SMTP_PASS\"}}}}
    ]"
  fi
  kubectl -n "$TARGET_NS" rollout restart deployment/biqly-auth
fi

echo "synced SMTP from $SOURCE_NS/$SOURCE_SECRET -> $TARGET_NS (secret + configmap, auth restarted if present)"
