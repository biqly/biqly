#!/usr/bin/env bash
# rpi-4 üzerinde sudo ile çalışır. Vault (docker container "vault") üzerinde:
#  - KV v2 mount: biqly/
#  - JWT auth: k8s-prag/  (cluster SA public-key ile ÇEVRİMDIŞI doğrulama)
#  - policy: biqly-eso-ro (read biqly/*)
#  - role:   biqly-eso    (sub=system:serviceaccount:biqly:biqly-eso, aud=vault)
# Token /root/vault-init.json'dan okunur, ekrana basılmaz. İdempotent.
set -euo pipefail
INIT=/root/vault-init.json
PEM=${1:-/tmp/k8s-sa-pubkey.pem}
ISSUER="https://kubernetes.default.svc.cluster.local"
TOKEN=$(jq -r .root_token "$INIT")
PUB=$(cat "$PEM")
run(){ docker exec -i -e VAULT_ADDR=https://127.0.0.1:8200 -e VAULT_SKIP_VERIFY=1 -e VAULT_TOKEN="$TOKEN" vault vault "$@"; }

echo "== KV v2 mount: biqly/ =="
run secrets list -format=json | grep -q '"biqly/"' && echo "  zaten var" || run secrets enable -path=biqly kv-v2

echo "== JWT auth: k8s-prag/ =="
run auth list -format=json | grep -q '"k8s-prag/"' && echo "  zaten var" || run auth enable -path=k8s-prag jwt

echo "== JWT config (statik pubkey + issuer) =="
run write auth/k8s-prag/config jwt_validation_pubkeys="$PUB" bound_issuer="$ISSUER" >/dev/null && echo "  ok"

echo "== policy: biqly-eso-ro =="
printf 'path "biqly/data/*" {\n  capabilities = ["read"]\n}\npath "biqly/metadata/*" {\n  capabilities = ["read","list"]\n}\n' | run policy write biqly-eso-ro - >/dev/null && echo "  ok"

echo "== role: biqly-eso =="
run write auth/k8s-prag/role/biqly-eso \
  role_type="jwt" \
  bound_audiences="vault" \
  user_claim="sub" \
  bound_subject="system:serviceaccount:biqly:biqly-eso" \
  token_policies="biqly-eso-ro" \
  token_ttl="20m" token_max_ttl="40m" >/dev/null && echo "  ok"

echo "== ÖZET =="
run auth list 2>/dev/null | grep -E "k8s-prag|Path"
run secrets list 2>/dev/null | grep -E "biqly/|Path"
echo "VAULT_CONFIG_DONE"
