#!/usr/bin/env bash
set -euo pipefail

VALUES_FILE="deploy/helm/biqly/values-prod.yaml"

COMMIT_SHA=$(git rev-parse HEAD)
TAG="sha-$COMMIT_SHA"

echo "Checking registry for tag: $TAG"

# Services that are pinned in values-prod.yaml
SERVICES=("auth" "frontend" "catalog" "query" "ai")

# Mapping of service name to yq update paths
declare -A PATHS=(
  ["auth"]="auth.image.tag auth.migrate.image.tag"
  ["frontend"]="frontend.image.tag"
  ["catalog"]="catalog.image.tag"
  ["query"]="query.image.tag"
  ["ai"]="ai.image.tag"
)

UPDATED=0

for svc in "${SERVICES[@]}"; do
  image="ghcr.io/biqly/$svc:$TAG"
  echo -n "Checking $image... "
  if docker manifest inspect "$image" >/dev/null 2>&1; then
    echo "FOUND!"
    for path in ${PATHS[$svc]}; do
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
