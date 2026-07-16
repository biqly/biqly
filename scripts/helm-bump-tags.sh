#!/usr/bin/env bash
set -euo pipefail

VALUES_FILE="deploy/helm/biqly/values-prod.yaml"

# yq binary: honor $YQ, else fall back to PATH (CI/Linux) then the Homebrew
# path (local macOS). Keeps the script runnable both locally and in CI.
YQ="${YQ:-$(command -v yq || echo /opt/homebrew/bin/yq)}"

# LATEST_PER_SERVICE=1 pins each service to the newest commit (walking back from
# HEAD, up to HISTORY_DEPTH) whose image is published — instead of requiring the
# image at HEAD. This keeps partial changes deployable: a commit that only
# rebuilds some services (e.g. a frontend-only change, or a docs/CI commit on
# top of a code commit) still advances the services that were built and leaves
# the rest on their latest real image, rather than stranding a release whose tip
# didn't rebuild every service. CI auto-bump sets this; local/manual runs
# default to HEAD-only (simpler, and manual deploys always rebuild the set).
LATEST_PER_SERVICE="${LATEST_PER_SERVICE:-0}"
HISTORY_DEPTH="${HISTORY_DEPTH:-40}"

HEAD_SHA=$(git rev-parse HEAD)
echo "Resolving image tags (HEAD=$HEAD_SHA, latest_per_service=$LATEST_PER_SERVICE)"

# Services and their yq update paths (parallel arrays: no declare -A,
# which is bash 4+ and not available on macOS 3.2).
SERVICES=("auth" "frontend" "catalog" "query" "ai" "mcp" "worker" "mail" "agent")
PATHS=(
  "auth.image.tag auth.migrate.image.tag"
  "frontend.image.tag"
  "catalog.image.tag"
  "query.image.tag"
  "ai.image.tag"
  "mcp.image.tag"
  "worker.image.tag"
  "mail.image.tag mail.migrate.image.tag"
  "agent.image.tag"
)

# Candidate commits, newest first: just HEAD unless walking history.
if [ "$LATEST_PER_SERVICE" = "1" ]; then
  mapfile -t CANDIDATES < <(git rev-list -n "$HISTORY_DEPTH" HEAD)
else
  CANDIDATES=("$HEAD_SHA")
fi

# resolve_tag <service> → prints the newest "sha-<commit>" with a published
# image among the candidates, or nothing if none are found.
resolve_tag() {
  local svc="$1" c image
  for c in "${CANDIDATES[@]}"; do
    image="ghcr.io/biqly/$svc:sha-$c"
    if docker manifest inspect "$image" >/dev/null 2>&1; then
      echo "sha-$c"
      return 0
    fi
  done
  return 1
}

UPDATED=0
for i in "${!SERVICES[@]}"; do
  svc="${SERVICES[$i]}"
  echo -n "Resolving $svc... "
  if tag=$(resolve_tag "$svc"); then
    echo "$tag"
    for path in ${PATHS[$i]}; do
      "$YQ" eval ".${path} = \"$tag\"" -i "$VALUES_FILE"
    done
    UPDATED=1
  else
    echo "no published image in the last ${#CANDIDATES[@]} commit(s) (leaving current tag)"
  fi
done

if [ $UPDATED -eq 1 ]; then
  echo "Success: values-prod.yaml image tags resolved."
else
  echo "Info: no published images found; values-prod.yaml unchanged."
fi
