#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${BIQLY_ENV_FILE:-$ROOT/.env.local}"
PAT_URL="https://github.com/settings/personal-access-tokens/new"

usage() {
  cat <<'EOF'
Usage:
  ./deploy/argocd/setup-github-pat.sh              # interactive: open GitHub UI, paste PAT
  BIQLY_GITHUB_TOKEN=github_pat_... ./deploy/argocd/setup-github-pat.sh

Creates a fine-grained PAT on GitHub (manual UI step — not available via API), stores it in
.env.local, validates repo write access, then runs install-image-updater.sh.

GitHub UI settings:
  Resource owner: biqly
  Repository access: Only select repositories → biqly
  Permissions: Contents (Read and write), Metadata (Read)
  Expiration: 90–365 days (your policy)

Do not use gh auth token (gho_ OAuth) in production.
EOF
}

token_from_env() {
  if [[ -n "${BIQLY_GITHUB_TOKEN:-}" ]]; then
    printf '%s' "$BIQLY_GITHUB_TOKEN"
    return 0
  fi
  if [[ -f "$ENV_FILE" ]]; then
    local line
    line="$(grep -E '^BIQLY_GITHUB_TOKEN=' "$ENV_FILE" 2>/dev/null | tail -1 || true)"
    if [[ -n "$line" ]]; then
      printf '%s' "${line#BIQLY_GITHUB_TOKEN=}"
      return 0
    fi
  fi
  return 1
}

validate_token() {
  local token="$1"

  case "$token" in
    gho_*) echo "error: OAuth token (gho_) — create a fine-grained PAT (github_pat_...) instead." >&2; return 1 ;;
    github_pat_*) echo "ok: fine-grained PAT (github_pat_)" ;;
    ghp_*) echo "warn: classic PAT (ghp_) works; fine-grained (github_pat_) is preferred for production." >&2 ;;
    *) echo "warn: unrecognized token prefix; expected github_pat_ or ghp_." >&2 ;;
  esac

  local perms
  perms="$(gh api repos/biqly/biqly \
    -H "Authorization: Bearer ${token}" \
    -q '.permissions.push' 2>/dev/null || echo false)"
  if [[ "$perms" != "true" ]]; then
    echo "error: token cannot push to biqly/biqly (Contents: write required)." >&2
    return 1
  fi
  echo "ok: token has push access to biqly/biqly"
}

save_token() {
  local token="$1"
  touch "$ENV_FILE"
  if grep -q '^BIQLY_GITHUB_TOKEN=' "$ENV_FILE" 2>/dev/null; then
    if [[ "$(uname -s)" == Darwin ]]; then
      sed -i '' "s|^BIQLY_GITHUB_TOKEN=.*|BIQLY_GITHUB_TOKEN=${token}|" "$ENV_FILE"
    else
      sed -i "s|^BIQLY_GITHUB_TOKEN=.*|BIQLY_GITHUB_TOKEN=${token}|" "$ENV_FILE"
    fi
  else
    printf '\n# Argo CD Image Updater — fine-grained PAT (biqly/biqly Contents: write)\nBIQLY_GITHUB_TOKEN=%s\n' "$token" >>"$ENV_FILE"
  fi
  echo "saved BIQLY_GITHUB_TOKEN in $ENV_FILE"
}

main() {
  if [[ "${1:-}" == -h || "${1:-}" == --help ]]; then
    usage
    exit 0
  fi

  local token=""
  if token="$(token_from_env)"; then
    echo "==> Using token from env or $ENV_FILE"
  else
    echo "==> Fine-grained PAT must be created in GitHub UI (REST API cannot create these tokens)."
    echo "    $PAT_URL"
    if [[ "$(uname -s)" == Darwin ]]; then
      open "$PAT_URL" 2>/dev/null || true
    fi
    echo ""
    echo "After generating the token, paste it here (input hidden):"
    read -r -s token
    echo ""
    if [[ -z "$token" ]]; then
      echo "error: empty token" >&2
      exit 1
    fi
  fi

  validate_token "$token"
  save_token "$token"

  export BIQLY_GITHUB_TOKEN="$token"
  export BIQLY_GITHUB_USER="${BIQLY_GITHUB_USER:-x-access-token}"
  echo "==> Installing / updating Argo CD Image Updater git credentials"
  "$ROOT/deploy/argocd/install-image-updater.sh"
}

main "$@"
