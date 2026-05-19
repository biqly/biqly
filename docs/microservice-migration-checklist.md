# Biqly Microservice Migration Checklist

`docs/microservice-decomposition.md` plan'inin uygulama takip listesi. Her madde
tamamlandikca `[ ]` → `[x]` yapilir. Backend once, infra/deployment sonra.

Legend: `[ ]` pending · `[~]` in-progress · `[x]` done · `[-]` cancelled

---

## Phase 0 — Paylasilan Foundation (Backend)

- [ ] **B0.1** `pkg/logicalquery/types.go` — LogicalQuery, SelectItem, Filter, GroupBy, OrderBy, CTE, WindowSpec tipleri `internal/query`'den tasi
- [ ] **B0.2** `pkg/semantic/types.go` — SemanticModel, Dimension, Metric, Join tiplerini `internal/semantic`'ten tasi
- [ ] **B0.3** `pkg/metadata/types.go` — Datasource, Table, Column, Relation tiplerini `internal/metadata`'dan tasi
- [ ] **B0.4** `pkg/query/types.go` — CompiledQuery, RunResult, HistoryEntry tipleri
- [ ] **B0.5** `pkg/security/types.go` — PermissionPolicy, denied fields, row filter tipleri
- [ ] **B0.6** `pkg/common/errors.go` — ServiceError, ErrorCode enum, HTTP status mapping
- [ ] **B0.7** `pkg/catalogclient` — typed HTTP client (Get/List/Create + retry + timeout + circuit breaker)
- [ ] **B0.8** `pkg/queryclient` — typed HTTP client (Compile/Run/DryRun + retry + timeout)
- [ ] **B0.9** `pkg/aiclient` — typed HTTP client (Query/Preview/Run/Describe)
- [ ] **B0.10** `internal/config` refactor — her binary kendi env var'larini, ortak `BI_*` setini paylasir
- [ ] **B0.11** `internal/` paketleri yeni `pkg/` tiplerini import edecek sekilde guncelle (drop-in alias)
- [ ] **B0.12** `go test ./...` + `golangci-lint run` yesil, mevcut davranis bozulmadi

---

## Phase 1 — Internal API Layer (Backend, monolith hala tek binary)

### 1A. Catalog read endpoints

- [ ] **B1.1** `GET /internal/models/{id}/full` — published SemanticModel (dimensions, metrics, joins)
- [ ] **B1.2** `GET /internal/models?datasource_id=` — model listesi
- [ ] **B1.3** `GET /internal/datasources/{id}` — Datasource (DSN decrypted, internal only)
- [ ] **B1.4** `GET /internal/tables?datasource_id=` — synced tables
- [ ] **B1.5** `GET /internal/columns?datasource_id=` — synced columns
- [ ] **B1.6** `GET /internal/relations?datasource_id=` — FK relations
- [ ] **B1.7** `GET /internal/few-shot?datasource_id=&model_id=` — curated examples
- [ ] **B1.8** `GET /internal/glossary?datasource_id=&model_id=` — business glossary

### 1B. Catalog write endpoints

- [ ] **B1.9** `POST /internal/ai-history` — AI query history insert
- [ ] **B1.10** `POST /internal/query-history` — Query Engine history insert
- [ ] **B1.11** `POST /internal/eval-results` — Eval result persistence

### 1C. Query internal endpoints

- [ ] **B1.12** `POST /internal/query/compile-with-context` — LogicalQuery + context → SQL
- [ ] **B1.13** `POST /internal/query/run-with-context` — LogicalQuery + context → execute
- [ ] **B1.14** `POST /internal/query/dry-run` — EXPLAIN gate

### 1D. Security + tests

- [ ] **B1.15** Internal auth middleware — `X-Internal-Token` shared secret header (gelecekte mTLS)
- [ ] **B1.16** Internal endpoint'ler `/api/*` HTTPRoute'tan ayri router'da (Cilium L7 policy ile kilitlenecek)
- [ ] **B1.17** Internal endpoint'ler icin integration test — golden response + schema validation
- [ ] **B1.18** Internal endpoint'ler audit log'a yazsin (`source=service`, `caller=ai|query|catalog`)

---

## Phase 2 — Catalog Service Extraction (Backend)

- [ ] **B2.1** `services/catalog/cmd/main.go` — chi router, graceful shutdown, slog logger
- [ ] **B2.2** `services/catalog/internal/` altina `internal/metadata/`, `internal/semantic/`, `internal/semanticgen/` paketlerini tasi
- [ ] **B2.3** `services/catalog/internal/handlers/datasources.go` — CRUD + test + sync
- [ ] **B2.4** `services/catalog/internal/handlers/metadata.go` — table/column search + update
- [ ] **B2.5** `services/catalog/internal/handlers/semantic.go` — model CRUD + validate + publish + rollback
- [ ] **B2.6** `services/catalog/internal/handlers/internal.go` — `/internal/*` read + write endpoint'leri
- [ ] **B2.7** `services/catalog/Dockerfile` — multi-stage Go build → distroless runtime
- [ ] **B2.8** `cmd/api/main.go`'da Catalog handler'larini `pkg/catalogclient` proxy'sine cevir
- [ ] **B2.9** Monolith'ten `internal/metadata` + `internal/semantic` import'larini kaldir, sadece client kullanilsin
- [ ] **B2.10** Integration test — frontend route'larin Catalog Service uzerinden calistigini dogrula
- [ ] **B2.11** Catalog Service prom metrics — `catalog_db_query_duration_seconds`, `model_publish_duration_seconds`

---

## Phase 3 — Query Engine Extraction (Backend)

- [ ] **B3.1** `services/query/cmd/main.go` — chi router, graceful shutdown
- [ ] **B3.2** `services/query/internal/` altina `internal/query/`, `internal/dialect/`, `internal/security/`, `internal/datasource/` tasi
- [ ] **B3.3** `services/query/internal/handlers/query.go` — compile, run, explain, history
- [ ] **B3.4** `services/query/internal/handlers/internal.go` — compile-with-context, run-with-context, dry-run
- [ ] **B3.5** Query Engine icine `pkg/catalogclient` wire — datasource + model okuma
- [ ] **B3.6** `services/query/Dockerfile` — multi-stage build
- [ ] **B3.7** `cmd/api/main.go`'da query handler'larini `pkg/queryclient` proxy'sine cevir
- [ ] **B3.8** Monolith'ten query/dialect/security/datasource import'larini kaldir
- [ ] **B3.9** Integration test — query compile + run hala dogru sonuc veriyor
- [ ] **B3.10** Query Engine prom metrics — `query_compile_duration_seconds`, `query_execute_duration_seconds`, `query_rows_returned`
- [ ] **B3.11** ReadOnlyChecker + permission policy testleri yeni binary'de yesil

---

## Phase 4 — AI Service Extraction (Backend)

- [ ] **B4.1** `services/ai/cmd/main.go` — chi router, graceful shutdown
- [ ] **B4.2** `services/ai/internal/ai/` — internal/ai tum dosyalari (service, prompt, table_router, client, anthropic, provider, schema, validator, describe, embedder, sample, glossary, eval, retry_helpers)
- [ ] **B4.3** AI Service icine `pkg/catalogclient` wire — model/metadata/glossary read + ai-history write
- [ ] **B4.4** AI Service icine `pkg/queryclient` wire — compile/run/dry-run
- [ ] **B4.5** `services/ai/internal/handlers/query.go` — `/api/ai/query`, `/preview`, `/run`
- [ ] **B4.6** `services/ai/internal/handlers/metadata.go` — `/metadata/describe`, `/metadata/embed`
- [ ] **B4.7** `services/ai/internal/handlers/eval.go` — `/eval/run`, `/run/stream`, `/runs`, `/regression`
- [ ] **B4.8** `services/ai/internal/handlers/examples.go` — `/examples`, `/feedback`, `/glossary`, `/usage`, `/settings`, `/stats/models`
- [ ] **B4.9** `services/ai/Dockerfile` — multi-stage build
- [ ] **B4.10** `cmd/api/main.go`'da AI handler'larini `pkg/aiclient` proxy'sine cevir, ardindan kaldir
- [ ] **B4.11** Self-consistency + clarification flow yeni binary'de calisiyor
- [ ] **B4.12** AI Service prom metrics — `llm_request_duration_seconds`, `llm_tokens_used_total`, `prompt_build_duration_seconds`

---

## Phase 5 — Monolith Sonlandirma (Backend)

- [ ] **B5.1** `cmd/api/main.go` proxy modu — frontend BFF (CORS, auth, fan-out)
- [ ] **B5.2** Tum frontend route'larin uc servise gittigini dogrula (HTTP trace)
- [ ] **B5.3** `cmd/api` tamamen kaldir veya minimum BFF olarak birak (karar an'inda netlesir)
- [ ] **B5.4** `internal/app/dependencies.go` artik kullanilmiyor → kaldir
- [ ] **B5.5** README + dev docs guncelle (uc binary calistirma)

---

## Phase 6 — Cross-Cutting Backend Concerns

- [ ] **B6.1** Tum servislerde `/metrics` Prometheus endpoint (Go runtime + business metrics)
- [ ] **B6.2** Tum servislerde `/health` (liveness — process up)
- [ ] **B6.3** Tum servislerde `/ready` (readiness — DB ping + upstream HTTP ping)
- [ ] **B6.4** slog JSON logger — TR/EN level kontrolu, correlation ID propagation
- [ ] **B6.5** OpenTelemetry `traceparent` middleware — AI → Query → Catalog tum chain
- [ ] **B6.6** Graceful shutdown — SIGTERM → server.Shutdown(ctx) + DB pool close, 30s drain
- [ ] **B6.7** HTTP client'larda timeout + max idle conns + keep-alive ayari
- [ ] **B6.8** Internal endpoint'ler arasinda retry policy (exponential backoff, max 3)
- [ ] **B6.9** Circuit breaker (gobreaker veya sony/gobreaker) — Catalog down olursa AI/Query degrade modunda
- [ ] **B6.10** Audit log her servis icin — kim, ne zaman, hangi sorgu/model

---

## Phase 7 — Helm Chart (Infra)

- [ ] **I1.1** `deploy/helm/biqly/Chart.yaml` umbrella + dependency: ai, query, catalog subchart
- [ ] **I1.2** `deploy/helm/biqly/values.yaml` — global image registry, pullSecret, gateway, postgres, redis
- [ ] **I1.3** `deploy/helm/biqly/values-dev.yaml`, `values-staging.yaml`, `values-prod.yaml` overlay
- [ ] **I1.4** Subchart `charts/ai/` — Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret template
- [ ] **I1.5** Subchart `charts/query/` — Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret template
- [ ] **I1.6** Subchart `charts/catalog/` — Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret template
- [ ] **I1.7** `_helpers.tpl` — labels, selectorLabels, fullname, serviceAccountName helper
- [ ] **I1.8** `checksum/config` annotation — `sha256sum` Helm template
- [ ] **I1.9** `helm lint` + `helm template` snapshot test
- [ ] **I1.10** `helm install --dry-run` ile dev cluster'da validate

---

## Phase 8 — Cluster Foundation (Infra)

- [ ] **I2.1** `biqly` namespace + label `kubernetes.io/metadata.name=biqly`, `name=biqly`
- [ ] **I2.2** `biqly` ServiceAccount + `automountServiceAccountToken: false`
- [ ] **I2.3** `ghcr-registry` Secret + reflector ile `reflector.v1.k8s.emberstack.com/reflection-allowed=true`
- [ ] **I2.4** `wildcard-il1-nl-tls` Secret reflector ile `gateway` ns'den biqly ns'ye mirror (gerekirse)
- [ ] **I2.5** `biqly-config` ConfigMap (BI_HTTP_PORT, BI_QUERY_TIMEOUT_SECONDS, BI_QUERY_MAX_ROWS, BI_LOG_LEVEL)
- [ ] **I2.6** `biqly-ai-config` ConfigMap (BI_AI_PROVIDER, BI_AI_MODEL, BI_AI_TEMPERATURE, BI_AI_EMBEDDING_MODEL)
- [ ] **I2.7** `biqly-db` Secret (BI_METADATA_DB_DSN)
- [ ] **I2.8** `biqly-security` Secret (BI_ENCRYPTION_KEY, 32-byte AES)
- [ ] **I2.9** `biqly-ai-secrets` Secret (BI_AI_API_KEY)
- [ ] **I2.10** `biqly-embedding-secrets` Secret optional (BI_AI_EMBEDDING_API_KEY)
- [ ] **I2.11** External Secrets Operator entegrasyonu (opsiyonel, Vault/SOPS arkasinda)

---

## Phase 9 — Cilium Gateway + HTTPRoute (Infra)

- [ ] **I3.1** `gateway/lan-gw` Gateway'in `*.il1.nl` listener'inda `allowedRoutes.namespaces.from: All` oldugunu dogrula
- [ ] **I3.2** `HTTPRoute biqly-catalog` — hostname `biqly.il1.nl`, paths `/api/datasources`, `/api/metadata`, `/api/semantic`, `/health`, backend `biqly-catalog:8080`
- [ ] **I3.3** `HTTPRoute biqly-query` — path `/api/query`, backend `biqly-query:8081`
- [ ] **I3.4** `HTTPRoute biqly-ai` — path `/api/ai`, backend `biqly-ai:8082`
- [ ] **I3.5** HTTP → HTTPS redirect (RequestRedirect filter veya Gateway listener)
- [ ] **I3.6** (opsiyonel) `biqly-api-vip` LoadBalancer Service + `io.cilium/lb-ipam-ips` annotation
- [ ] **I3.7** `dig biqly.il1.nl` → 192.168.0.160 (lan-gw IP) dogrula
- [ ] **I3.8** `curl https://biqly.il1.nl/health` → 200 dogrula

---

## Phase 10 — CiliumNetworkPolicy (Infra)

- [ ] **I4.1** `biqly-allow-dns` — endpointSelector component IN (ai, query, catalog), egress kube-dns 53/UDP+TCP
- [ ] **I4.2** `biqly-allow-gateway` — fromEntities `ingress` + `host`/`remote-node`/`health` + intra-namespace, ports 8080/8081/8082
- [ ] **I4.3** `biqly-egress-metadata` — egress toEntities `cluster`, ports 5432 (postgres) + 6379 (dragonfly)
- [ ] **I4.4** `biqly-query-egress-user-dbs` — sadece component=query, toCIDR user DB subnet, ports 5432/3306/1433/8123/9000
- [ ] **I4.5** `biqly-ai-egress-external` — sadece component=ai, toFQDNs `api.openai.com` + `api.anthropic.com`, port 443/TCP
- [ ] **I4.6** Cilium `enable-l7-proxy: true` configmap ayari dogrula (toFQDNs icin gerekli)
- [ ] **I4.7** `hubble observe` ile policy'lerin trafigi dogru izin verdigini gozle
- [ ] **I4.8** Negative test — biqly-ai pod'undan `curl postgres:5432` BLOCKED olmali

---

## Phase 11 — Data Layer (Infra)

- [ ] **I5.1** `biqly-postgresql` Bitnami chart deploy — `bi_metadata` DB, primary-only (replication ileride)
- [ ] **I5.2** PostgreSQL StatefulSet PVC retain policy + backup CronJob (pg_dump → S3)
- [ ] **I5.3** Schema'lari olustur: `catalog`, `query`, `ai` (`migrations/` icindeki SQL'e schema label ekle)
- [ ] **I5.4** PgBouncer sidecar veya `bitnami/pgbouncer` — connection pooling
- [ ] **I5.5** PostgreSQL NetworkPolicy — ingress sadece `app.kubernetes.io/instance=biqly` etiketli pod'lardan
- [ ] **I5.6** `biqly-postgresql-vip` LoadBalancer (opsiyonel, dev erisimi icin)
- [ ] **I5.7** `biqly-dragonfly` Helm chart deploy (Redis-compatible cache, query result + AI rate limit)
- [ ] **I5.8** `cmd/migrate` Helm post-install/post-upgrade Job — `golang-migrate up`
- [ ] **I5.9** `pg_isready` initContainer her servis Deployment'inde calisiyor

---

## Phase 12 — Progressive Delivery + Resilience (Infra)

- [ ] **I6.1** Argo Rollouts CRD install (cluster-wide, `argo-rollouts` namespace zaten var)
- [ ] **I6.2** `biqly-ai` Rollout — canary steps 20% (5m pause) → 50% (10m) → 100%
- [ ] **I6.3** `AnalysisTemplate ai-success-rate` — Prometheus query, `success_rate >= 0.95` threshold
- [ ] **I6.4** `AnalysisTemplate ai-llm-latency` — `p99 < 30s`
- [ ] **I6.5** HPA `biqly-ai` — CPU 70%, min 2, max 8
- [ ] **I6.6** HPA `biqly-query` — CPU 70%, min 3, max 10
- [ ] **I6.7** HPA `biqly-catalog` — CPU 60%, min 2, max 4
- [ ] **I6.8** PodDisruptionBudget `biqly-ai` — minAvailable: 1
- [ ] **I6.9** PodDisruptionBudget `biqly-query` — minAvailable: 2
- [ ] **I6.10** PodDisruptionBudget `biqly-catalog` — minAvailable: 1
- [ ] **I6.11** Pod Security Standards — namespace label `pod-security.kubernetes.io/enforce=restricted`
- [ ] **I6.12** Container securityContext — `runAsNonRoot: true`, `runAsUser: 65532`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`, `allowPrivilegeEscalation: false`, `seccompProfile.type: RuntimeDefault`

---

## Phase 13 — GitOps + CI/CD (Infra)

- [ ] **I7.1** ArgoCD `Application` manifest — repo `github.com/biqly/biqly`, path `deploy/helm/biqly`, automated prune+selfHeal
- [ ] **I7.2** ArgoCD AppProject `biqly` — RBAC, allowed sources, allowed destinations
- [ ] **I7.3** GitHub Actions `.github/workflows/build-ai.yml` — multi-arch Docker build → `ghcr.io/biqly/ai:sha-<commit>`
- [ ] **I7.4** GitHub Actions `.github/workflows/build-query.yml` — `ghcr.io/biqly/query:sha-<commit>`
- [ ] **I7.5** GitHub Actions `.github/workflows/build-catalog.yml` — `ghcr.io/biqly/catalog:sha-<commit>`
- [ ] **I7.6** GitHub Actions test workflow — `go test ./...` + `golangci-lint run` + `helm lint`
- [ ] **I7.7** `argocd-image-updater` ile values.yaml `image.tag` otomatik bump (write-back commit)
- [ ] **I7.8** Branch protection — main'e direkt push yasak, PR + CI yesil + 1 review
- [ ] **I7.9** Renovate veya Dependabot — Go module + Helm chart dependency update

---

## Phase 14 — Observability (Infra)

- [ ] **I8.1** ServiceMonitor (prom-operator) veya `prometheus.io/scrape` annotation ile metric scrape
- [ ] **I8.2** Grafana dashboard `biqly-ai` — LLM request duration, tokens used, cost estimate, success rate
- [ ] **I8.3** Grafana dashboard `biqly-query` — compile/execute duration, rows returned, error rate
- [ ] **I8.4** Grafana dashboard `biqly-catalog` — DB query latency, publish duration, request rate
- [ ] **I8.5** Loki/Promtail veya Vector — slog JSON log ingestion, correlation ID label
- [ ] **I8.6** OpenTelemetry Collector deploy — OTLP receiver, Tempo/Jaeger exporter
- [ ] **I8.7** Tempo/Jaeger backend — distributed trace storage, retention 7d
- [ ] **I8.8** Alertmanager rule — AI p99 LLM latency > 30s for 5m
- [ ] **I8.9** Alertmanager rule — Query p99 compile > 100ms for 10m
- [ ] **I8.10** Alertmanager rule — Catalog p99 DB > 200ms for 10m
- [ ] **I8.11** Alertmanager rule — Error budget burn rate (SLO based)
- [ ] **I8.12** PagerDuty/Slack webhook entegrasyonu

---

## Phase 15 — Production Cutover

- [ ] **C1** Staging environment'ta full end-to-end test (frontend → 3 servis)
- [ ] **C2** Load test — k6 ile 100 RPS NL query, p99 latency < 35s
- [ ] **C3** Failure injection — Catalog Service'i kapat, AI/Query degrade modunda calisiyor mu
- [ ] **C4** Database migration provasi — staging'de tum migration'lar yesil
- [ ] **C5** Backup + restore drill — pg_dump al, yeni DB'ye restore, query calisiyor
- [ ] **C6** Production cutover plan dokumani (rollback steps dahil)
- [ ] **C7** Production deploy — blue-green veya canary 10% traffic
- [ ] **C8** Production smoke test — frontend route'lari, AI query, query compile
- [ ] **C9** Monolith deployment'ini production'dan kaldir
- [ ] **C10** Post-mortem + runbook update

---

*Bu liste `docs/microservice-decomposition.md` tasarim dokumanindan otomatik
turetilmistir. Yeni madde eklendiginde ana tasarim dokumani da guncellenmelidir.*
