# Cloudflared tunnel (prag cluster)

Cloudflared runs in `kube-system` on the **prag** node. Public hostnames terminate at
Cloudflare; the daemon forwards into the cluster.

## biqly (`abi.il1.nl`)

Traffic for `abi.il1.nl` must go through **Envoy Gateway** (`eg-gw`), not individual
biqly Service ClusterIPs. HTTPRoutes in the `biqly` namespace (catalog, query, ai,
auth, mcp, frontend) attach to `envoy-gateway-system/eg-gw` and match path prefixes
there.

Upstream (stable DNS):

```text
http://eg-gw-http.envoy-gateway-system.svc.cluster.local:80
```

Apply the stable Service once (see `deploy/infra/envoy-gateway/eg-gw-http-service.yaml`).

## Apply / update config

```bash
kubectl -n kube-system create configmap cloudflared-config \
  --from-file=config.yaml=deploy/infra/cloudflared/config.yaml \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n kube-system rollout restart deployment/cloudflared
kubectl -n kube-system rollout status deployment/cloudflared
```

## Verify

```bash
curl -fsS https://abi.il1.nl/health
curl -fsS -o /dev/null -w "%{http_code}\n" https://abi.il1.nl/
```

From inside the cluster (Host header required):

```bash
kubectl -n biqly exec deploy/biqly-frontend -- \
  wget -qO- --header="Host: abi.il1.nl" \
  http://eg-gw-http.envoy-gateway-system.svc.cluster.local/health
```

## Other hostnames

`config.yaml` also lists argocd, grafana, zlitter, traceo, etc. Those remain
direct-to-Service rules until migrated to a gateway the same way.
