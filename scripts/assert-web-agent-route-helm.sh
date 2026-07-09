#!/usr/bin/env bash
# Asserts the ai chart's rendered HTTPRoute exposes POST /api/agent/chat
# (T6, web agent mode) with the same 1800s request timeout as /api/ai, so a
# long-running agent run does not hit Envoy's 15s default and 504. Run after
# `make helm-template` (which supplies the required global secrets so the
# chart actually renders instead of failing on missing production values):
#
#   make helm-template
#   ./scripts/assert-web-agent-route-helm.sh /tmp/biqly-helm-template.yaml
#
# or simply: make helm-assert-web-agent-route
set -euo pipefail

FILE="${1:-/tmp/biqly-helm-template.yaml}"
if [ ! -f "$FILE" ]; then
  echo "FAIL: rendered template file not found: $FILE (run \`make helm-template\` first)"
  exit 1
fi

FAILURES=0

fail() {
  echo "FAIL: $1"
  FAILURES=$((FAILURES + 1))
}

pass() {
  echo "PASS: $1"
}

# extract_source SOURCE_PATH prints the rendered YAML block for one
# `# Source: biqly/...` document, up to (but not including) the next `---`
# document separator or the next `# Source:` line.
extract_source() {
  local source="$1"
  awk -v src="# Source: ${source}" '
    $0 == src { found=1; print; next }
    found && /^---$/ { exit }
    found && /^# Source:/ { exit }
    found { print }
  ' "$FILE"
}

assert_contains() {
  local block="$1" needle="$2" desc="$3"
  if echo "$block" | grep -qF -- "$needle"; then
    pass "$desc"
  else
    fail "$desc"
  fi
}

# --- ai chart HTTPRoute: /api/ai + /api/agent path prefixes, both with the
#     1800s request timeout (long-running AI/agent calls would otherwise hit
#     Envoy's 15s default and 504). ---
AI_ROUTE="$(extract_source "biqly/charts/ai/templates/httproute.yaml")"
if [ -z "$AI_ROUTE" ]; then
  fail "charts/ai/templates/httproute.yaml did not render at all"
else
  assert_contains "$AI_ROUTE" "app.kubernetes.io/component: ai" "ai HTTPRoute selects the ai component"
  assert_contains "$AI_ROUTE" 'value: "/api/ai"' "ai HTTPRoute matches /api/ai"
  assert_contains "$AI_ROUTE" 'value: "/api/agent"' "ai HTTPRoute matches /api/agent (web agent chat, T6)"

  # Each rule's timeout block trails its own `- matches:` entry, so split on
  # the rule boundary and check each one carries the 1800s timeout rather
  # than just grep'ing for the string once across the whole block.
  AI_RULE_COUNT="$(echo "$AI_ROUTE" | grep -c '^    - matches:' || true)"
  AI_TIMEOUT_COUNT="$(echo "$AI_ROUTE" | grep -c 'request: 1800s' || true)"
  if [ "$AI_RULE_COUNT" -ge 2 ] && [ "$AI_TIMEOUT_COUNT" -eq "$AI_RULE_COUNT" ]; then
    pass "every ai HTTPRoute rule (found $AI_RULE_COUNT) carries the 1800s request timeout"
  else
    fail "expected every ai HTTPRoute rule to carry a 1800s request timeout (rules=$AI_RULE_COUNT, timeouts=$AI_TIMEOUT_COUNT)"
  fi
fi

echo
if [ "$FAILURES" -eq 0 ]; then
  echo "All web agent route assertions passed."
  exit 0
fi
echo "$FAILURES assertion(s) failed."
exit 1
