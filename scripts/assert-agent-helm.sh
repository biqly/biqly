#!/usr/bin/env bash
# Asserts the agentic query runner's rendered NetworkPolicy/Helm output stays
# least-privilege. Run after `make helm-template`:
#
#   make helm-template
#   ./scripts/assert-agent-helm.sh /tmp/biqly-helm-template.yaml
#
# Checks the DEFAULT (non-prod) values render, where agent's Postgres/OTel
# egress comes from the embedded-topology bundle in cnp-metadata.yaml
# (global.networkPolicy.metadata.enabled=true by default). The prod-topology
# bundle (global.networkPolicy.sharedPostgresql, values-prod.yaml) is a
# separate values-driven components list, not exercised by this script —
# verify it manually with `helm template -f values-prod.yaml` when changing
# that list.
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

assert_not_contains() {
  local block="$1" needle="$2" desc="$3"
  if echo "$block" | grep -qF -- "$needle"; then
    fail "$desc"
  else
    pass "$desc"
  fi
}

# --- cnp-agent.yaml: dedicated ingress policy for agent's own port. ---
AGENT_CNP="$(extract_source "biqly/templates/cnp-agent.yaml")"
if [ -z "$AGENT_CNP" ]; then
  fail "cnp-agent.yaml did not render at all"
else
  assert_contains "$AGENT_CNP" "app.kubernetes.io/component: agent" "cnp-agent.yaml selects the agent component"
  assert_contains "$AGENT_CNP" 'port: "8084"' "cnp-agent.yaml permits ingress on port 8084"
  assert_contains "$AGENT_CNP" "- host" "cnp-agent.yaml permits host-network monitoring ingress"
  assert_contains "$AGENT_CNP" "- health" "cnp-agent.yaml permits Cilium health-probe ingress"
  assert_contains "$AGENT_CNP" "app.kubernetes.io/part-of: biqly" "cnp-agent.yaml permits in-cluster (part-of: biqly) ingress"
fi

# --- cnp-dns.yaml: agent must resolve in-cluster service DNS names. ---
DNS_CNP="$(extract_source "biqly/templates/cnp-dns.yaml")"
assert_contains "$DNS_CNP" $'          - agent' "cnp-dns.yaml grants agent DNS egress"

# --- cnp-gateway.yaml: agent's catalog/query/ai/NATS egress + host-network
#     ingress for its own metrics port. ---
GATEWAY_CNP="$(extract_source "biqly/templates/cnp-gateway.yaml")"
assert_contains "$GATEWAY_CNP" $'          - agent' "cnp-gateway.yaml grants agent catalog/query/ai/NATS egress"
assert_contains "$GATEWAY_CNP" 'port: "8084"' "cnp-gateway.yaml permits host-network monitoring on agent's port 8084"

# --- cnp-metadata.yaml: agent's default-topology Postgres + OTel egress. ---
METADATA_CNP="$(extract_source "biqly/templates/cnp-metadata.yaml")"
assert_contains "$METADATA_CNP" $'          - agent' "cnp-metadata.yaml grants agent Postgres/OTel egress (default topology)"

# --- Least-privilege: agent must NOT get external egress. ---
AI_EXTERNAL_CNP="$(extract_source "biqly/templates/cnp-ai-external.yaml")"
assert_not_contains "$AI_EXTERNAL_CNP" $'          - agent' "agent has no world-HTTPS/FQDN external egress (cnp-ai-external.yaml)"

# --- Least-privilege: agent must NEVER get direct customer-database access. ---
USER_DBS_CNP="$(extract_source "biqly/templates/cnp-query-user-dbs.yaml")"
assert_not_contains "$USER_DBS_CNP" "agent" "agent has no query-user-database CIDR rule (cnp-query-user-dbs.yaml)"

# --- No public route ever selects agent (no HTTPRoute in its subchart). ---
if grep -q "kind: HTTPRoute" "$FILE" 2>/dev/null && grep -B5 "kind: HTTPRoute" "$FILE" | grep -q "app.kubernetes.io/component: agent"; then
  fail "an HTTPRoute selects the agent component — agent must have no public route"
else
  pass "no HTTPRoute selects the agent component"
fi
if grep -rq "kind: HTTPRoute" deploy/helm/biqly/charts/agent 2>/dev/null; then
  fail "deploy/helm/biqly/charts/agent defines an HTTPRoute — it must not have one"
else
  pass "charts/agent defines no HTTPRoute template"
fi

echo
if [ "$FAILURES" -eq 0 ]; then
  echo "All agent NetworkPolicy assertions passed."
  exit 0
fi
echo "$FAILURES assertion(s) failed."
exit 1
