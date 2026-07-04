#!/usr/bin/env bash
set -euo pipefail

VALUES_FILE="deploy/helm/biqly/values-prod.yaml"

COMMIT_SHA=$(git rev-parse HEAD)
TAG="sha-$COMMIT_SHA"

echo "Checking registry for tag: $TAG"

# Services and their yq update paths (parallel arrays: no declare -A,
# which is bash 4+ and not available on macOS 3.2).
SERVICES=("auth" "frontend" "catalog" "query" "ai" "mcp" "worker" "mail")
PATHS=(
  "auth.image.tag auth.migrate.image.tag"
  "frontend.image.tag"
  "catalog.image.tag"
  "query.image.tag"
  "ai.image.tag"
  "mcp.image.tag"
  "worker.image.tag"
  "mail.image.tag mail.migrate.image.tag"
)

UPDATED=0

for i in "${!SERVICES[@]}"; do
  svc="${SERVICES[$i]}"
  image="ghcr.io/biqly/$svc:$TAG"
  echo -n "Checking $image... "
  if docker manifest inspect "$image" >/dev/null 2>&1; then
    echo "FOUND!"
    for path in ${PATHS[$i]}; do
      /opt/homebrew/bin/yq eval ".${path} = \"$TAG\"" -i "$VALUES_FILE"
    done
    UPDATED=1
  else
    echo "not found (skipping)"
  fi
done

if [ $UPDATED -eq 1 ]; then
  echo "Success: values-prod.yaml updated with new image tags."
else
  echo "Info: No new images found for tag $TAG. values-prod.yaml remains unchanged."
fi
