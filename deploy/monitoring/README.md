# Cluster monitoring (Grafana + Prometheus)

Biqly Helm chart publishes Grafana dashboard ConfigMaps when
`global.observability.dashboards.enabled` is true. The cluster Grafana deployment
(in `monitoring`) runs a kiwigrid sidecar with `NAMESPACE=ALL`, so it picks up
labeled ConfigMaps from the `biqly` namespace automatically.

## Enable Grafana

```bash
make grafana-enable
# equivalent: kubectl scale deployment/grafana -n monitoring --replicas=1
```

Grafana is exposed at `https://grafana.il1.nl` (HTTPRoute `zlitter-grafana` →
`grafana.monitoring.svc:3000`). Credentials live in secret `grafana-admin`
(`monitoring` namespace).

## Dashboard provisioning

| Source | Namespace | Label |
| --- | --- | --- |
| Biqly AI / Query / Catalog / Cardinality | `biqly` | `grafana_dashboard=1` |

Dashboards appear under the **biqly** folder (ConfigMap annotation
`grafana_folder: biqly`). Prometheus datasource UID is `prometheus` (matches
`grafana-datasources` ConfigMap in `monitoring`).

After changing dashboard JSON in `deploy/helm/biqly/templates/grafana-dashboards.yaml`,
commit and let Argo CD sync, or apply immediately:

```bash
make grafana-dashboards-sync
```

Metrics are scraped from biqly pods via `prometheus.io/scrape` pod annotations
(Prometheus job `kubernetes-pods`).
