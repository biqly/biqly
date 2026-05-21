#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NS="${ARGOCD_NAMESPACE:-argocd}"
SECRET_NAME="${ARGOCD_IMAGE_UPDATER_GIT_SECRET:-argocd-image-updater-git}"
TOKEN="${BIQLY_GITHUB_TOKEN:-}"
GITHUB_USER="${BIQLY_GITHUB_USER:-x-access-token}"
USE_PR="${ARGOCD_IMAGE_UPDATER_USE_PR:-false}"

if [[ -z "$TOKEN" ]]; then
  echo "error: set BIQLY_GITHUB_TOKEN to a GitHub PAT with write access to biqly/biqly (do not commit it)." >&2
  exit 1
fi

echo "==> Git write secret ($SECRET_NAME in $NS)"
kubectl create secret generic "$SECRET_NAME" -n "$NS" \
  --from-literal=username="$GITHUB_USER" \
  --from-literal=password="$TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Helm: argocd-image-updater"
helm upgrade --install argocd-image-updater argo/argocd-image-updater \
  -n "$NS" \
  -f "$ROOT/deploy/argocd/image-updater-helm-values.yaml" \
  --wait --timeout 5m

echo "==> ImageUpdater CR"
kubectl apply -f "$ROOT/deploy/argocd/image-updater.yaml"

if [[ "$USE_PR" == "true" || "$USE_PR" == "1" ]]; then
  echo "==> Enabling GitHub PR write-back (branch protection mode)"
  kubectl patch imageupdater biqly -n "$NS" --type merge -p '{
    "spec": {
      "writeBackConfig": {
        "method": "git:secret:argocd/'"$SECRET_NAME"'",
        "gitConfig": {
          "branch": "main",
          "pullRequest": {
            "github": {}
          }
        }
      }
    }
  }'
fi

echo "==> Status"
kubectl -n "$NS" get pods -l app.kubernetes.io/name=argocd-image-updater
kubectl -n "$NS" get imageupdater biqly -o wide 2>/dev/null || true
echo "done."
