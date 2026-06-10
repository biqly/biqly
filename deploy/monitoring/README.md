# Cluster monitoring (Grafana + Prometheus)

## kube-prometheus-stack (Prometheus Operator)

Biqly `ServiceMonitor` / `PrometheusRule` resources require the Prometheus
Operator CRDs. Install the stack in `monitoring` (Grafana/Loki stay as-is):

```bash
make monitoring-operator-install
```

This installs `kube-prometheus-stack` with Grafana/Alertmanager/node-exporter
disabled, scales down the legacy `prometheus` Deployment, and points Grafana at
`kube-prometheus-stack-prometheus`.

Uninstall (restores legacy prometheus):

```bash
make monitoring-operator-uninstall
```

## Grafana dashboards

Biqly Helm chart publishes Grafana dashboard ConfigMaps when
`global.observability.dashboards.enabled` is true. The cluster Grafana deployment
(in `monitoring`) runs a kiwigrid sidecar with `NAMESPACE=ALL`, so it picks up
labeled ConfigMaps from the `biqly` namespace automatically.

### Enable Grafana

```bash
make grafana-enable
# equivalent: kubectl scale deployment/grafana -n monitoring --replicas=1
```

Grafana is exposed at `https://grafana.il1.nl` (HTTPRoute `zlitter-grafana` →
`grafana.monitoring.svc:3000`). Credentials live in secret `grafana-admin`
(`monitoring` namespace).

### Dashboard provisioning

| Source | Namespace | Label |
| --- | --- | --- |
| Biqly AI / Query / Catalog / Cardinality | `biqly` | `grafana_dashboard=1` |

Dashboards appear under the **biqly** folder (ConfigMap annotation
`grafana_folder: biqly`). Prometheus datasource UID is `prometheus` (see
`deploy/monitoring/grafana-datasources.yaml`).

After changing dashboard JSON in `deploy/helm/biqly/templates/grafana-dashboards.yaml`,
commit and let Argo CD sync, or apply immediately:

```bash
make grafana-dashboards-sync
```

With kube-prometheus-stack, metrics are scraped via `ServiceMonitor` resources
(`release=kube-prometheus-stack` label). Legacy pod-annotation scraping is
retired when the operator install Makefile target runs.
