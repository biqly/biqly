# Biqly Microservice Decomposition Plan

Mevcut monolitik Go backend'inin Kubernetes'te microservice olarak calismasi icin
bolunme stratejisi. Her karar domain-driven design, scaling gereksinimleri ve
deployment independence prensiplerine dayanir.

---

## Mevcut Durum

```
cmd/api/main.go (single binary, port 8888)
  |
  v
internal/app/dependencies.go (15 dependency, manual DI)
  |
  +-- HTTP Handlers (chi router, 50+ endpoint)
  +-- PostgreSQL (single metadata DB, bi_metadata)
  +-- AI Provider (OpenAI/Anthropic HTTP client)
  +-- Datasource Drivers (postgres, mysql, sqlserver, clickhouse)
  +-- Encryption (AES, in-process)
  +-- No message queue, no event bus
  +-- Redis defined but NOT wired (dead code)
  +-- Worker binary exists but skeleton only
```

**Tek binary. Tek DB. Tek deployment birimi.** Herhangi bir degisiklikte
tum sistemi tekrar deploy etmeniz gerekiyor.

---

## Hedef Mimari

```
                    ┌───────────────────────┐
                    │  Cilium Gateway API   │  (lan-gw, GatewayClass: cilium)
                    │  L2 LB IP (LB-IPAM)   │  TLS terminate (cert-manager)
                    │  HTTPRoute per service│
                    └──────────┬────────────┘
                               │
          ┌────────────────────┼───────────────────────┐
          │                    │                       │
          v                    v                       v
  ┌───────────────┐    ┌──────────────┐      ┌─────────────────┐
  │  AI Service   │    │ Query Engine │      │ Catalog Service │
  │  (NL → LQ)    │    │  (Compile +  │      │ (Datasource +   │
  │               │    │   Execute)   │      │  Semantic +     │
  │  LLM calls    │    │              │      │  Metadata)      │
  │  Table Router │    │  SQL compile │      │                 │
  │  Prompt Build │    │  Execution   │      │  CRUD operations│
  │  Validation   │    │  Security    │      │  Introspection  │
  └───────┬───────┘    └──────┬───────┘      └────────┬────────┘
          │                   │                       │
          │            ┌──────┴──────┐                │
          │            │ User DBs    │                │
          │            │ (datasource │                │
          │            │  drivers)   │                │
          │            └─────────────┘                │
          │                                           │
          └──────────────┬────────────────────────────┘
                         │
                  ┌──────┴──────┐         ┌──────────────┐
                  │ PostgreSQL  │         │  Dragonfly   │
                  │ bi_metadata │         │  (Redis API) │
                  │ (Bitnami    │         │  cache       │
                  │  Helm)      │         └──────────────┘
                  └─────────────┘
```

**Cluster bilesenleri (zlitter ile ayni standartta):**

| Bilesen | Kullanim |
|---|---|
| **Cilium CNI + Gateway API** | L7 routing, GatewayClass `cilium`, HTTPRoute per servis |
| **Cilium LB-IPAM + L2 announce** | LoadBalancer Service VIP'leri (`io.cilium/lb-ipam-ips` annotation) |
| **CiliumNetworkPolicy** | DNS, gateway ingress, intra-namespace, db egress, world egress |
| **cert-manager** | Wildcard TLS sertifika (Gateway listener'a Secret reference) |
| **ArgoCD** | GitOps deployment, `argocd.argoproj.io/tracking-id` annotation |
| **Argo Rollouts** | Frontend / kritik servisler icin canary deploy |
| **Helm umbrella chart** | `deploy/helm/biqly` — subchart per servis |
| **Prometheus + Grafana** | `prometheus.io/scrape` annotation ile auto-discovery |

---

## Servislerin Tanimi

### 1. AI Service (`biqly-ai`)

**Sorumluluk:** Natural language question'dan LogicalQuery JSON uretimi.

**Neden ayri:** En farkli scaling karakteristigi. LLM API latency 2-30 saniye.
Token maliyeti yuksek. Farkli deploy sikligi (prompt degisikligi icin sadece
bu servisi deploy et).

**Kapsam:**
| Package | Dosyalar |
|---------|----------|
| `internal/ai/` | `service.go`, `prompt.go`, `table_router.go`, `client.go`, `anthropic.go`, `provider.go`, `schema.go`, `validator.go`, `describe.go`, `embedder.go`, `embed_metadata.go`, `sample.go`, `glossary.go`, `eval.go`, `eval_repository.go`, `golden_seed.go`, `retry_helpers.go`, `i18n/`, `prompt_templates.go` |

**Dependency listesi:**
| Dependency | Kaynak | Yontem |
|---|---|---|
| `semantic.SemanticModel` | Catalog Service | HTTP call (read-only) |
| `metadata.Column`, `metadata.Table` | Catalog Service | HTTP call (read-only) |
| `metadata.Relation` | Catalog Service | HTTP call (read-only) |
| `query.Validator` | Internal | Paket kalir AI Service'te |
| `query.LogicalQuery` types | Internal | Paylasilan Go module |
| `dialect.Dialect` interface | Internal | Paylasilan Go module |
| LLM API (OpenAI/Anthropic) | External | HTTP client |
| Embedding API | External | HTTP client |
| AI History persistence | Catalog Service | HTTP call (write) |
| Few-shot examples | Catalog Service | HTTP call (read) |
| Business Glossary | Catalog Service | HTTP call (read) |
| Eval results persistence | Catalog Service | HTTP call (write) |

**Endpoints (AI Service'e tasianlar):**
```
POST /ai/query
POST /ai/query/preview
POST /ai/query/run
POST /ai/metadata/describe
POST /ai/metadata/embed
GET  /ai/settings
POST /ai/eval/run           (admin)
GET  /ai/eval/run/stream    (admin)
GET  /ai/eval/runs          (admin)
GET  /ai/eval/runs/{id}     (admin)
GET  /ai/eval/regression    (admin)
GET  /ai/examples
POST /ai/examples
PUT  /ai/examples/{id}
DELETE /ai/examples/{id}
POST /ai/feedback
GET  /ai/usage
GET  /ai/example-ids
GET  /ai/stats/models
GET  /ai/glossary
POST /ai/glossary
PUT  /ai/glossary/{id}
DELETE /ai/glossary/{id}
```

**Resource gereksinimleri:**
```yaml
requests:
  cpu: 500m
  memory: 512Mi
limits:
  cpu: "2"
  memory: 1Gi
# LLM call'ler uzun surer, az replica ile cok is yapilir
replicas: 2-4
# HPA: custom metric (in-flight LLM requests) ile scale
```

**Internal communication:**
- Catalog Service'ten model/metadata okur: `GET /internal/models/{id}/full`
- Catalog Service'e AI history yazar: `POST /internal/ai-history`
- Query Engine'e compile/execute request'i: `POST /internal/query/compile`, `POST /internal/query/run`

---

### 2. Query Engine (`biqly-query`)

**Sorumluluk:** LogicalQuery -> SQL compilation ve safe execution.

**Neden ayri:** Stateless compilation yuksek throughput icin horizontal scale edebilir.
Execution user DB'lere dogrudan erisir - security boundary olarak izole edilmeli.

**Kapsam:**
| Package | Dosyalar |
|---------|----------|
| `internal/query/` | `compiler.go`, `compiler_nested.go`, `compiler_case.go`, `executor.go`, `validator.go`, `planner.go`, `logical.go`, `enrich.go`, `result.go`, `fingerprint.go`, `physical_ref.go` |
| `internal/dialect/` | Tum dosyalar |
| `internal/security/` | `readonly.go`, `row_injection.go`, `encryption.go`, `permissions.go` |
| `internal/datasource/` | Tum driver implementations |

**Dependency listesi:**
| Dependency | Kaynak | Yontem |
|---|---|---|
| `semantic.SemanticModel` | Catalog Service | HTTP call (read-only) |
| `metadata.Datasource` | Catalog Service | HTTP call (read-only) |
| User database connections | External | Datasource drivers |
| `security.Encryption` | Internal | Encryption key shared via K8s secret |

**Endpoints (Query Engine'e tasianlar):**
```
POST /query/compile
POST /query/run
POST /query/explain
GET  /query/history
GET  /query/history/{id}
```

**Internal endpoints (AI Service tarafindan cagrilan):**
```
POST /internal/query/compile-with-context
POST /internal/query/run-with-context
POST /internal/query/dry-run
```

**Resource gereksinimleri:**
```yaml
requests:
  cpu: 250m
  memory: 256Mi
limits:
  cpu: "1"
  memory: 512Mi
# Compile CPU-bound, execution IO-bound
replicas: 3-10
# HPA: CPU utilization > 70%
```

**Security notu:** User DB credential'lari sadece bu servis tarafindan kullanilir.
Network policy ile sadece bu pod'larin user DB subnet'lerine erismesine izin verilir.
Baska servisin bu credential'lara erismesi gerekmez.

---

### 3. Catalog Service (`biqly-catalog`)

**Sorumluluk:** Datasource, metadata (table/column/relation), semantic model CRUD.
Tum metadata'nn tek kaynak (single source of truth).

**Neden ayri:** En buyuk data ownership boundary. Read-heavy ama write'lar da onemli
(draft/publish workflow). Diger servislerin hepsi buradan okur.

**Kapsam:**
| Package | Dosyalar |
|---------|----------|
| `internal/metadata/` | Tum dosyalar (repository, curated_ai, business_glossary, translations, ai_metrics, sft_export) |
| `internal/semantic/` | Tum dosyalar (repository, model, publish, budget) |
| `internal/semanticgen/` | Tum dosyalar |
| `internal/platform/db/` | Generic query helper |

**Dependency listesi:**
| Dependency | Kaynak | Yontem |
|---|---|---|
| PostgreSQL `bi_metadata` | External | Direct connection |
| Datasource drivers (introspection) | Internal | Sadece `SyncMetadata` icin |

**Endpoints (Catalog Service'e tasianlar):**
```
# Datasource CRUD
POST   /api/datasources
GET    /api/datasources
GET    /api/datasources/{id}
PUT    /api/datasources/{id}
DELETE /api/datasources/{id}
POST   /api/datasources/{id}/test
POST   /api/datasources/test-draft
POST   /api/datasources/{id}/sync-metadata

# Metadata
GET    /api/datasources/{id}/tables
GET    /api/datasources/{id}/columns
GET    /api/metadata/columns/search
GET    /api/metadata/tables/search
PATCH  /api/metadata/tables/{id}
PATCH  /api/metadata/columns/{id}

# Semantic Models
POST   /api/semantic/models
GET    /api/semantic/models
GET    /api/semantic/models/{id}
PUT    /api/semantic/models/{id}
DELETE /api/semantic/models/{id}
POST   /api/semantic/models/{id}/validate
POST   /api/semantic/models/{id}/publish
POST   /api/semantic/models/{id}/rollback
POST   /api/semantic/models/{id}/dimensions
POST   /api/semantic/models/{id}/metrics
POST   /api/semantic/models/{id}/joins
# ... tum alt-resource endpoint'leri
```

**Internal endpoints (diger servisler tarafindan cagrilan):**
```
GET    /internal/datasources/{id}              (Query Engine icin)
GET    /internal/models/{id}/full              (AI Service icin)
GET    /internal/models?datasource_id=         (AI Service icin)
GET    /internal/tables?datasource_id=         (AI Service icin)
GET    /internal/columns?datasource_id=        (AI Service icin)
GET    /internal/relations?datasource_id=      (AI Service icin)
GET    /internal/few-shot?datasource_id=&model_id=   (AI Service icin)
GET    /internal/glossary?datasource_id=&model_id=   (AI Service icin)
POST   /internal/ai-history                    (AI Service icin)
POST   /internal/query-history                 (Query Engine icin)
```

**Resource gereksinimleri:**
```yaml
requests:
  cpu: 250m
  memory: 256Mi
limits:
  cpu: "1"
  memory: 512Mi
replicas: 2-3
# HPA: CPU > 60% veya request rate
```

---

## Paylasilan Go Module

Servisler arasi ortak tipler icin Go module yapisi:

```
biqly/
├── go.mod (root module: github.com/biqly/biqly)
├── pkg/                          # PAYLASILAN TIPLER
│   ├── logicalquery/
│   │   └── types.go              # LogicalQuery, SelectItem, Filter, GroupBy, OrderBy, CTE, WindowSpec
│   ├── semantic/
│   │   └── types.go              # SemanticModel, Dimension, Metric, Join
│   ├── metadata/
│   │   └── types.go              # Datasource, Table, Column, Relation
│   ├── query/
│   │   └── types.go              # CompiledQuery, RunResult, HistoryEntry
│   ├── security/
│   │   └── types.go              # PermissionPolicy
│   └── common/
│       └── errors.go             # ServiceError, common error codes
├── services/
│   ├── ai/                       # AI Service binary
│   │   ├── cmd/main.go
│   │   └── internal/
│   │       ├── ai/               # internal/ai paketi
│   │       ├── handlers/
│   │       └── config/
│   ├── query/                    # Query Engine binary
│   │   ├── cmd/main.go
│   │   └── internal/
│   │       ├── query/            # internal/query paketi
│   │       ├── dialect/          # internal/dialect paketi
│   │       ├── security/         # internal/security paketi
│   │       ├── datasource/       # internal/datasource paketi
│   │       └── handlers/
│   └── catalog/                  # Catalog Service binary
│       ├── cmd/main.go
│       └── internal/
│           ├── metadata/         # internal/metadata paketi
│           ├── semantic/         # internal/semantic paketi
│           ├── semanticgen/      # internal/semanticgen paketi
│           └── handlers/
├── deploy/
│   ├── helm/
│   │   └── biqly/                # Umbrella Helm chart
│   │       ├── Chart.yaml
│   │       ├── values.yaml
│   │       ├── templates/
│   │       └── charts/
│   │           ├── ai/           # AI Service subchart
│   │           ├── query/        # Query Engine subchart
│   │           └── catalog/      # Catalog Service subchart
│   └── kustomize/
│       ├── base/
│       └── overlays/
│           ├── dev/
│           ├── staging/
│           └── production/
└── frontend/
```

**Alternatif (mono-repo, single Go module):**

Eger mono-repo kalmak istenirse, `cmd/` altinda 3 binary olusturulur:

```
biqly/
├── cmd/
│   ├── api/main.go          # MEVCUT - backward compatibility (BFF)
│   ├── ai/main.go           # AI Service
│   ├── query/main.go        # Query Engine
│   └── catalog/main.go      # Catalog Service
├── internal/                 # MEVCUT - paylasilan kod
├── pkg/                      # YENI - cross-service interface'ler
│   ├── catalogclient/       # Catalog Service HTTP client
│   ├── queryclient/         # Query Engine HTTP client
│   └── internalapi/         # Internal API contract tipleri
└── deploy/
```

Bu yaklasimda `internal/` paketleri aynen kalir, her `cmd/` binary'si
sadece ihtiyaci olan paketleri import eder. Geçis döneminde `cmd/api`
BFF (Backend-for-Frontend) olarak kalir, yavaş yavaş proxy'lere dönüsür.

---

## Servisler Arasi Iletisim

### Communication Matrix

```
                    Catalog    AI Service    Query Engine
                    Service
Catalog Service       -          WRITE          WRITE
                                (history)      (history)

AI Service          READ          -            RPC/HTTP
                    (model,                    (compile,
                    metadata,                   execute,
                    glossary)                   dry-run)

Query Engine        READ        SERVES          -
                    (datasource  (compile,
                    model)       execute)

API Gateway        PROXY        PROXY         PROXY
(frontend)         all CRUD     AI routes     query routes
```

### Internal API Protocol

Servisler arasi communication icin iki secenek:

#### Option A: HTTP/JSON (Baslangic icin recommended)

```
GET /internal/models/{id}/full
  Response: SemanticModel JSON (dimensions, metrics, joins dahil)

GET /internal/datasources/{id}
  Response: Datasource JSON (DSN decrypted)

POST /internal/query/compile
  Request:  { logical_query, model_id }
  Response: { sql, args, fingerprint }

POST /internal/query/run
  Request:  { logical_query, model_id }
  Response: { columns, rows, duration_ms, row_count }
```

**Avantaj:** Basit, debug kolay, her dilde client var.
**Dezavantaj:** JSON serialization overhead, HTTP connection overhead.

#### Option B: gRPC (Production scale icin)

```protobuf
service CatalogService {
  rpc GetFullModel(GetFullModelRequest) returns (SemanticModel);
  rpc GetDatasource(GetDatasourceRequest) returns (Datasource);
  rpc CreateAIQueryHistory(AIQueryHistoryEntry) returns (Empty);
}

service QueryEngine {
  rpc Compile(CompileRequest) returns (CompileResponse);
  rpc Run(RunRequest) returns (RunResponse);
  rpc DryRun(DryRunRequest) returns (DryRunResponse);
}
```

**Avantaj:** Typed, streaming support, connection pooling built-in, ~3-5x faster.
**Dezavantaj:** Proto file maintenance, binary debugging harder.

**Tavsiye:** Baslangicta HTTP/JSON ile basla, bottleneck olunca gRPC'e gec.

---

## Database Stratejisi

### Mevcut Durum
Tek PostgreSQL DB: `bi_metadata`. 15+ table.

### Hedef
Her servis kendi schema'sina sahip olur, ayni PostgreSQL instance'ini paylasir:

```
bi_metadata (PostgreSQL)
├── catalog schema (default: public)
│   ├── datasources
│   ├── schemas
│   ├── tables
│   ├── columns
│   ├── relations
│   ├── semantic_models
│   ├── semantic_dimensions
│   ├── semantic_metrics
│   ├── semantic_joins
│   ├── semantic_context_snapshots
│   ├── business_glossary
│   ├── translations
│   ├── few_shot_curated
│   └── permissions
├── query schema
│   └── query_history
└── ai schema
    ├── ai_query_history
    ├── ai_feedback
    ├── eval_runs
    └── eval_results
```

**Neden ayni DB instance:** Operasyonel basitlik. Mikro-optimize etmek yerine
schema isolation ile basla, gerekirse fiziksel ayir.

**Connection pooling:** PgBouncer sidecar veya managed proxy (RDS Proxy, Cloud SQL Proxy).

---

## Kubernetes Deployment

Tum manifest'ler `zlitter` namespace'inde kullandigimiz standartla ayni: Cilium
Gateway API + HTTPRoute, CiliumNetworkPolicy, Helm + ArgoCD GitOps, immutable
`sha-<commit>` image tag'leri, `envFrom` ile ConfigMap/Secret bag, initContainer
ile downstream wait, `checksum/config` annotation, prometheus scrape annotation.

### Standart Labels & Annotations

Tum kaynaklar Helm chart `biqly-<version>` tarafindan uretildigi icin Bitnami /
Kubernetes recommended labels seti kullanilir:

```yaml
labels:
  app.kubernetes.io/name: biqly-<component>   # biqly-ai, biqly-query, biqly-catalog
  app.kubernetes.io/component: <component>    # ai, query, catalog
  app.kubernetes.io/instance: biqly
  app.kubernetes.io/part-of: biqly
  app.kubernetes.io/managed-by: Helm
  app.kubernetes.io/version: latest
  helm.sh/chart: biqly-<chart-version>
annotations:
  argocd.argoproj.io/tracking-id: biqly:apps/Deployment:biqly/biqly-<component>
```

`selector.matchLabels` SADECE su uc anahtardan olusur (immutable, Helm upgrade
safe): `component`, `instance`, `part-of`. Ayrintili sebep: `version` /
`managed-by` /`chart` Helm upgrade'de degisirse selector'i kirar.

### Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: biqly
  labels:
    name: biqly
    kubernetes.io/metadata.name: biqly
```

Tek namespace, izolasyon **CiliumNetworkPolicy** ile saglanir (zlitter'da oldugu
gibi default-deny ihtiyac duyulan flow'lar acilir).

### Image Pull Secret (GHCR)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ghcr-registry
  namespace: biqly
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <base64-encoded>
```

Reflector ile `cert-manager` /`gateway` namespace'lerinden mirror edilebilir
(zlitter'da kullandigimiz pattern ile ayni).

### ServiceAccount

Helm chart tek bir SA olusturur, tum component'ler paylasir:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: biqly
  namespace: biqly
  labels:
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
automountServiceAccountToken: false
```

### CiliumNetworkPolicy (zlitter ile ayni 5'li set)

#### 1) DNS egress (kube-dns'e izin)

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: biqly-allow-dns
  namespace: biqly
spec:
  endpointSelector:
    matchExpressions:
      - key: app.kubernetes.io/component
        operator: In
        values: [ai, query, catalog]
    matchLabels:
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  egress:
    - toEndpoints:
        - matchLabels:
            k8s-app: kube-dns
            k8s:io.kubernetes.pod.namespace: kube-system
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
            - port: "53"
              protocol: TCP
```

#### 2) Gateway ingress (Cilium Gateway -> servis)

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: biqly-allow-gateway
  namespace: biqly
spec:
  endpointSelector:
    matchExpressions:
      - key: app.kubernetes.io/component
        operator: In
        values: [ai, query, catalog]
    matchLabels:
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  ingress:
    - fromEntities: [ingress]
      toPorts:
        - ports:
            - port: "8080"   # catalog
              protocol: TCP
            - port: "8081"   # query
              protocol: TCP
            - port: "8082"   # ai
              protocol: TCP
    - fromEntities: [host, remote-node, health]
      toPorts:
        - ports:
            - port: "8080"
              protocol: TCP
            - port: "8081"
              protocol: TCP
            - port: "8082"
              protocol: TCP
    - fromEndpoints:
        - matchLabels:
            app.kubernetes.io/instance: biqly
            app.kubernetes.io/part-of: biqly
      toPorts:
        - ports:
            - port: "8080"
              protocol: TCP
            - port: "8081"
              protocol: TCP
            - port: "8082"
              protocol: TCP
```

#### 3) Metadata DB + cache + nats egress

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: biqly-egress-metadata
  namespace: biqly
spec:
  endpointSelector:
    matchExpressions:
      - key: app.kubernetes.io/component
        operator: In
        values: [ai, query, catalog]
    matchLabels:
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  egress:
    - toEntities: [cluster]
      toPorts:
        - ports:
            - port: "5432"   # postgres bi_metadata
              protocol: TCP
            - port: "6379"   # dragonfly cache
              protocol: TCP
```

#### 4) Query Engine - user DB egress (sadece query'e ozel)

User database'leri farkli subnet/CIDR'larda olabilir. `toCIDR` veya `toFQDN`
kullanin. zlitter `nats/postgres` icin `toEntities: cluster` kullaniyor, biqly
icin user DB'ler clusterin disinda olabilir:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: biqly-query-egress-user-dbs
  namespace: biqly
spec:
  endpointSelector:
    matchLabels:
      app.kubernetes.io/component: query
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  egress:
    - toCIDR:
        - 10.0.0.0/8           # internal DB subnet (ornek)
      toPorts:
        - ports:
            - port: "5432"     # postgres
              protocol: TCP
            - port: "3306"     # mysql
              protocol: TCP
            - port: "1433"     # sqlserver
              protocol: TCP
            - port: "8123"     # clickhouse http
              protocol: TCP
            - port: "9000"     # clickhouse native
              protocol: TCP
```

#### 5) AI Service - external LLM egress

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: biqly-ai-egress-external
  namespace: biqly
spec:
  endpointSelector:
    matchLabels:
      app.kubernetes.io/component: ai
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  egress:
    - toFQDNs:
        - matchName: api.openai.com
        - matchName: api.anthropic.com
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
```

> `toFQDNs` kullanildigi icin `enable-l7-proxy` Cilium'da aktif olmali ve DNS
> egress policy (#1) onceden uygulanmali. zlitter'da `egress-external`
> `toEntities: world` ile genis tutuluyor; biqly'de AI servisini api.openai.com
> ve api.anthropic.com'a kilitlemek tercih edilir.

### AI Service Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: biqly-ai
  namespace: biqly
  annotations:
    argocd.argoproj.io/tracking-id: biqly:apps/Deployment:biqly/biqly-ai
  labels:
    app.kubernetes.io/name: biqly-ai
    app.kubernetes.io/component: ai
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/version: latest
    helm.sh/chart: biqly-0.1.0
spec:
  replicas: 2
  revisionHistoryLimit: 5
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 100%
      maxUnavailable: 0
  selector:
    matchLabels:
      app.kubernetes.io/component: ai
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  template:
    metadata:
      annotations:
        checksum/config: <sha256 of rendered ConfigMap+Secret>
        prometheus.io/scrape: "true"
        prometheus.io/port: "8082"
        prometheus.io/path: /metrics
      labels:
        app.kubernetes.io/name: biqly-ai
        app.kubernetes.io/component: ai
        app.kubernetes.io/instance: biqly
        app.kubernetes.io/part-of: biqly
        app.kubernetes.io/managed-by: Helm
        app.kubernetes.io/version: latest
        helm.sh/chart: biqly-0.1.0
    spec:
      serviceAccountName: biqly
      imagePullSecrets:
        - name: ghcr-registry
      initContainers:
        - name: wait-for-catalog
          image: curlimages/curl:8.00.1
          imagePullPolicy: IfNotPresent
          command:
            - sh
            - -c
            - |
              echo "Waiting for Catalog Service..."
              until curl -sf "http://biqly-catalog:8080/health"; do
                echo "Catalog not ready, retrying in 2s..."
                sleep 2
              done
              echo "Catalog ready"
          resources:
            requests: { cpu: 25m, memory: 16Mi }
            limits:   { cpu: 50m, memory: 32Mi }
        - name: wait-for-query
          image: curlimages/curl:8.00.1
          imagePullPolicy: IfNotPresent
          command:
            - sh
            - -c
            - |
              echo "Waiting for Query Engine..."
              until curl -sf "http://biqly-query:8081/health"; do
                echo "Query not ready, retrying in 2s..."
                sleep 2
              done
              echo "Query ready"
          resources:
            requests: { cpu: 25m, memory: 16Mi }
            limits:   { cpu: 50m, memory: 32Mi }
      containers:
        - name: ai
          image: ghcr.io/biqly/ai:sha-<commit>
          imagePullPolicy: Always
          ports:
            - name: http
              containerPort: 8082
              protocol: TCP
          env:
            - name: PORT
              value: "8082"
            - name: APP_VERSION
              value: sha-<commit>
            - name: BI_CATALOG_SERVICE_URL
              value: http://biqly-catalog:8080
            - name: BI_QUERY_SERVICE_URL
              value: http://biqly-query:8081
            - name: BI_REDIS_DSN
              value: redis://biqly-dragonfly:6379
          envFrom:
            - configMapRef:
                name: biqly-ai-config
            - secretRef:
                name: biqly-ai-secrets       # api-key
            - secretRef:
                name: biqly-embedding-secrets
                optional: true
          livenessProbe:
            httpGet: { path: /health, port: http }
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet: { path: /ready, port: http }
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: "2"
              memory: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: biqly-ai
  namespace: biqly
  labels:
    app.kubernetes.io/name: biqly-ai
    app.kubernetes.io/component: ai
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/component: ai
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
  ports:
    - name: http
      port: 8082
      targetPort: http
      protocol: TCP
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: biqly-ai
  namespace: biqly
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: biqly-ai
  minReplicas: 2
  maxReplicas: 8
  metrics:
    - type: Resource
      resource:
        name: cpu
        target: { type: Utilization, averageUtilization: 70 }
```

### Query Engine Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: biqly-query
  namespace: biqly
  annotations:
    argocd.argoproj.io/tracking-id: biqly:apps/Deployment:biqly/biqly-query
  labels:
    app.kubernetes.io/name: biqly-query
    app.kubernetes.io/component: query
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/version: latest
    helm.sh/chart: biqly-0.1.0
spec:
  replicas: 3
  revisionHistoryLimit: 5
  strategy:
    type: RollingUpdate
    rollingUpdate: { maxSurge: 100%, maxUnavailable: 0 }
  selector:
    matchLabels:
      app.kubernetes.io/component: query
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  template:
    metadata:
      annotations:
        checksum/config: <sha256 of rendered ConfigMap+Secret>
        prometheus.io/scrape: "true"
        prometheus.io/port: "8081"
        prometheus.io/path: /metrics
      labels:
        app.kubernetes.io/name: biqly-query
        app.kubernetes.io/component: query
        app.kubernetes.io/instance: biqly
        app.kubernetes.io/part-of: biqly
        app.kubernetes.io/managed-by: Helm
        app.kubernetes.io/version: latest
        helm.sh/chart: biqly-0.1.0
    spec:
      serviceAccountName: biqly
      imagePullSecrets:
        - name: ghcr-registry
      initContainers:
        - name: wait-for-postgres
          image: postgres:17-alpine
          command:
            - sh
            - -c
            - |
              echo "Waiting for bi_metadata PostgreSQL..."
              until pg_isready -h biqly-postgresql -p 5432 -U biqly -d bi_metadata; do
                echo "PostgreSQL not ready, retrying in 2s..."
                sleep 2
              done
              echo "PostgreSQL ready"
          resources:
            requests: { cpu: 50m, memory: 32Mi }
            limits:   { cpu: 100m, memory: 64Mi }
        - name: wait-for-catalog
          image: curlimages/curl:8.00.1
          command:
            - sh
            - -c
            - |
              until curl -sf "http://biqly-catalog:8080/health"; do
                sleep 2
              done
          resources:
            requests: { cpu: 25m, memory: 16Mi }
            limits:   { cpu: 50m, memory: 32Mi }
      containers:
        - name: query
          image: ghcr.io/biqly/query:sha-<commit>
          imagePullPolicy: Always
          ports:
            - name: http
              containerPort: 8081
              protocol: TCP
          env:
            - name: PORT
              value: "8081"
            - name: APP_VERSION
              value: sha-<commit>
            - name: BI_QUERY_TIMEOUT_SECONDS
              value: "30"
            - name: BI_QUERY_MAX_ROWS
              value: "10000"
            - name: BI_CATALOG_SERVICE_URL
              value: http://biqly-catalog:8080
          envFrom:
            - configMapRef:
                name: biqly-config
            - secretRef:
                name: biqly-db              # metadata DSN
            - secretRef:
                name: biqly-security        # BI_ENCRYPTION_KEY
          livenessProbe:
            httpGet: { path: /health, port: http }
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet: { path: /ready, port: http }
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: "1"
              memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: biqly-query
  namespace: biqly
  labels:
    app.kubernetes.io/name: biqly-query
    app.kubernetes.io/component: query
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/component: query
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
  ports:
    - name: http
      port: 8081
      targetPort: http
      protocol: TCP
```

### Catalog Service Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: biqly-catalog
  namespace: biqly
  annotations:
    argocd.argoproj.io/tracking-id: biqly:apps/Deployment:biqly/biqly-catalog
  labels:
    app.kubernetes.io/name: biqly-catalog
    app.kubernetes.io/component: catalog
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/version: latest
    helm.sh/chart: biqly-0.1.0
spec:
  replicas: 2
  revisionHistoryLimit: 5
  strategy:
    type: RollingUpdate
    rollingUpdate: { maxSurge: 100%, maxUnavailable: 0 }
  selector:
    matchLabels:
      app.kubernetes.io/component: catalog
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  template:
    metadata:
      annotations:
        checksum/config: <sha256 of rendered ConfigMap+Secret>
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: /metrics
      labels:
        app.kubernetes.io/name: biqly-catalog
        app.kubernetes.io/component: catalog
        app.kubernetes.io/instance: biqly
        app.kubernetes.io/part-of: biqly
        app.kubernetes.io/managed-by: Helm
        app.kubernetes.io/version: latest
        helm.sh/chart: biqly-0.1.0
    spec:
      serviceAccountName: biqly
      imagePullSecrets:
        - name: ghcr-registry
      initContainers:
        - name: wait-for-postgres
          image: postgres:17-alpine
          command:
            - sh
            - -c
            - |
              until pg_isready -h biqly-postgresql -p 5432 -U biqly -d bi_metadata; do
                sleep 2
              done
          resources:
            requests: { cpu: 50m, memory: 32Mi }
            limits:   { cpu: 100m, memory: 64Mi }
      containers:
        - name: catalog
          image: ghcr.io/biqly/catalog:sha-<commit>
          imagePullPolicy: Always
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          env:
            - name: PORT
              value: "8080"
            - name: APP_VERSION
              value: sha-<commit>
          envFrom:
            - configMapRef:
                name: biqly-config
            - secretRef:
                name: biqly-db
            - secretRef:
                name: biqly-security
          livenessProbe:
            httpGet: { path: /health, port: http }
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet: { path: /ready, port: http }
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: "1"
              memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: biqly-catalog
  namespace: biqly
  labels:
    app.kubernetes.io/name: biqly-catalog
    app.kubernetes.io/component: catalog
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/component: catalog
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
  ports:
    - name: http
      port: 8080
      targetPort: http
      protocol: TCP
```

### Cilium Gateway API (HTTPRoute)

zlitter `lan-gw` Gateway'ini paylasiyoruz (namespace `gateway`,
`gatewayClassName: cilium`, listener `*.il1.nl` HTTPS + cert-manager wildcard
TLS). Her servis kendi HTTPRoute'unu olusturur ve `parentRefs` ile bu Gateway'e
attach olur.

#### Catalog HTTPRoute (root /api/*)

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: biqly-catalog
  namespace: biqly
  annotations:
    argocd.argoproj.io/tracking-id: biqly:gateway.networking.k8s.io/HTTPRoute:biqly/biqly-catalog
  labels:
    app.kubernetes.io/component: catalog
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
spec:
  hostnames:
    - biqly.il1.nl
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: lan-gw
      namespace: gateway
  rules:
    - matches:
        - path: { type: PathPrefix, value: /api/datasources }
        - path: { type: PathPrefix, value: /api/metadata }
        - path: { type: PathPrefix, value: /api/semantic }
        - path: { type: PathPrefix, value: /health }
      backendRefs:
        - group: ""
          kind: Service
          name: biqly-catalog
          port: 8080
          weight: 1
```

#### Query Engine HTTPRoute

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: biqly-query
  namespace: biqly
  labels:
    app.kubernetes.io/component: query
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
spec:
  hostnames:
    - biqly.il1.nl
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: lan-gw
      namespace: gateway
  rules:
    - matches:
        - path: { type: PathPrefix, value: /api/query }
      backendRefs:
        - name: biqly-query
          port: 8081
```

#### AI Service HTTPRoute

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: biqly-ai
  namespace: biqly
  labels:
    app.kubernetes.io/component: ai
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
spec:
  hostnames:
    - biqly.il1.nl
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: lan-gw
      namespace: gateway
  rules:
    - matches:
        - path: { type: PathPrefix, value: /api/ai }
      backendRefs:
        - name: biqly-ai
          port: 8082
```

> Gateway `gateway` namespace'inde, HTTPRoute biqly namespace'inde — Gateway
> listener'larinda `allowedRoutes.namespaces.from: All` zaten acik (zlitter
> ile ayni `lan-gw` kullanildigi icin ReferenceGrant gerekmez).

#### LoadBalancer VIP (opsiyonel)

Frontend / public API icin `Service type=LoadBalancer` Cilium LB-IPAM ile
sabit IP alir (zlitter `zlitter-api-vip` ornegindeki gibi):

```yaml
apiVersion: v1
kind: Service
metadata:
  name: biqly-api-vip
  namespace: biqly
  annotations:
    io.cilium/lb-ipam-ips: '["192.168.0.181"]'
  labels:
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/name: biqly-api-vip
    app.kubernetes.io/part-of: biqly
spec:
  type: LoadBalancer
  selector:
    app.kubernetes.io/component: catalog
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
```

VIP, internal cluster disindan (LAN icinden) erisim gerektirdiginde kullanilir;
production traffic her durumda Gateway HTTPRoute uzerinden gelir.

---

## ConfigMap ve Secret Organization

zlitter pattern'i: tum env var'lar `envFrom` ile yuklenir, Pod template'ine
`checksum/config` annotation eklenir (Helm `sha256sum` ile uretir) — bu sayede
ConfigMap/Secret degisikligi otomatik rolling restart tetikler.

```yaml
# Tum servislerin paylastigi ortak konfig
apiVersion: v1
kind: ConfigMap
metadata:
  name: biqly-config
  namespace: biqly
  labels:
    app.kubernetes.io/instance: biqly
    app.kubernetes.io/part-of: biqly
data:
  BI_HTTP_PORT: "8080"                # her servis kendi PORT env'i ile override eder
  BI_QUERY_TIMEOUT_SECONDS: "30"
  BI_QUERY_MAX_ROWS: "10000"
  BI_AI_MAX_PROMPT_RUNES: "80000"
  BI_AI_MULTI_CANDIDATE_COUNT: "1"
  BI_LOG_LEVEL: "info"
---
# AI Service'e ozel konfig
apiVersion: v1
kind: ConfigMap
metadata:
  name: biqly-ai-config
  namespace: biqly
data:
  BI_AI_PROVIDER: "openai"
  BI_AI_MODEL: "gpt-4o"
  BI_AI_TEMPERATURE: "0.0"
  BI_AI_EMBEDDING_MODEL: "text-embedding-3-small"
---
# Metadata DB DSN (Catalog + Query kullanir)
apiVersion: v1
kind: Secret
metadata:
  name: biqly-db
  namespace: biqly
type: Opaque
stringData:
  BI_METADATA_DB_DSN: "postgres://biqly:<password>@biqly-postgresql:5432/bi_metadata?sslmode=disable"
---
# AES encryption key (sadece Catalog + Query okur)
apiVersion: v1
kind: Secret
metadata:
  name: biqly-security
  namespace: biqly
type: Opaque
stringData:
  BI_ENCRYPTION_KEY: "<base64 32-byte AES key>"
---
# LLM API key (sadece AI Service okur)
apiVersion: v1
kind: Secret
metadata:
  name: biqly-ai-secrets
  namespace: biqly
type: Opaque
stringData:
  BI_AI_API_KEY: "<OPENAI/ANTHROPIC API KEY>"
---
# Embedding API key (opsiyonel, AI Service envFrom optional: true ile alir)
apiVersion: v1
kind: Secret
metadata:
  name: biqly-embedding-secrets
  namespace: biqly
type: Opaque
stringData:
  BI_AI_EMBEDDING_API_KEY: "<embedding provider key>"
```

Helm template'inde her Deployment'in `spec.template.metadata.annotations`
altinda:

```yaml
annotations:
  checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
```

---

## GitOps: ArgoCD + Argo Rollouts

zlitter ile ayni GitOps pattern'i (her Helm chart bir ArgoCD `Application`).

### ArgoCD Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: biqly
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/biqly/biqly
    path: deploy/helm/biqly
    targetRevision: main
    helm:
      valueFiles:
        - values.yaml
        - values-prod.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: biqly
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
```

Tum kaynaklarda `argocd.argoproj.io/tracking-id` annotation kullanilir
(zlitter ornegindeki gibi `biqly:apps/Deployment:biqly/biqly-ai`).

### Argo Rollouts (kritik servisler icin canary)

zlitter'da `zlitter-web` icin canary kullanildi (`rollout-phase=stable` label).
Biqly'de `biqly-ai` (LLM cost riski) ve `biqly-query` (data risk) icin ayni
pattern uygulanir:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: biqly-ai
  namespace: biqly
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/component: ai
      app.kubernetes.io/instance: biqly
      app.kubernetes.io/part-of: biqly
  strategy:
    canary:
      maxSurge: "25%"
      maxUnavailable: 0
      steps:
        - setWeight: 20
        - pause: { duration: 5m }
        - setWeight: 50
        - pause: { duration: 10m }
        - setWeight: 100
      analysis:
        templates:
          - templateName: ai-success-rate
        startingStep: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/component: ai
        app.kubernetes.io/instance: biqly
        app.kubernetes.io/part-of: biqly
    # ... AI Deployment ile ayni pod spec
```

AnalysisTemplate icinde Prometheus query'leri ile success rate / latency
threshold'lari kontrol edilir (zlitter web-rollout ile ayni model).

---

## Migration Stratejisi (Step-by-Step)

### Phase 1: Internal API Layer (0 → 2 hafta)

Monolith'i degistirmeden internal HTTP endpoint'ler eklenir.

```
cmd/api/main.go (hala tek binary)
  |
  +-- /api/* (mevcut endpoint'ler, degisiklik yok)
  +-- /internal/* (YENI - serviceler arasi communication)
       +-- GET /internal/models/{id}/full
       +-- GET /internal/datasources/{id}
       +-- POST /internal/ai-history
       +-- POST /internal/query-history
       +-- POST /internal/query/compile
       +-- POST /internal/query/run
```

**Amaç:** Frontend hicbir degisiklik gormez. Internal endpoint'ler test edilir.

### Phase 2: Extract Catalog Service (2 → 4. hafta)

1. `cmd/catalog/main.go` olustur
2. `metadata.Repository` + `semantic.Repository` bagla
3. Catalog Service'i deploy et (K8s service)
4. Monolith'teki handler'lari Catalog Service HTTP client'a cevir:
   - `MetaRepo.GetDatasource()` → `catalogClient.GetDatasource()`
   - `SemanticRepo.GetPublishedFullModel()` → `catalogClient.GetFullModel()`
5. `HTTPRoute biqly-catalog`'u Catalog Service'e yonlendir:
   - `/api/datasources/*`, `/api/semantic/*`, `/api/metadata/*`, `/health`
     → `biqly-catalog:8080`
6. Test et, monolith'ten bu route'lari kaldir

**Dogrulama:** Frontend hala calisiyor, Catalog Service bagimsiz deploy edilebiliyor.

### Phase 3: Extract Query Engine (4 → 6. hafta)

1. `cmd/query/main.go` olustur
2. `query.Compiler`, `query.Executor`, `dialect`, `security`, `datasource` bagla
3. Query Engine'i deploy et
4. Monolith'teki query handler'lari Query Engine HTTP client'a cevir
5. `HTTPRoute biqly-query` olustur: `/api/query/*` → `biqly-query:8081`
6. **CiliumNetworkPolicy** ekle: `biqly-query-egress-user-dbs` ile sadece
   `app.kubernetes.io/component=query` etiketli pod'lar user DB CIDR'a erissin

**Dogrulama:** Compile/run hala calisiyor, security boundary saglanmis.

### Phase 4: Extract AI Service (6 → 9. hafta)

1. `cmd/ai/main.go` olustur
2. `ai.Service`, `ai.TableRouter`, `ai.PromptBuilder` bagla
3. Catalog Service'e internal client ekle (model/metadata/glossary)
4. Query Engine'e internal client ekle (compile/run/dry-run)
5. AI Service'i deploy et
6. `HTTPRoute biqly-ai` olustur: `/api/ai/*` → `biqly-ai:8082`
7. `CiliumNetworkPolicy biqly-ai-egress-external` ile `toFQDNs` (api.openai.com,
   api.anthropic.com) kilidi ekle
8. Monolith'ten AI handler'lari kaldir

**Dogrulama:** NL-to-query hala calisiyor, AI Service bagimsiz deploy.

### Phase 5: Remove Monolith (9 → 10. hafta)

1. `cmd/api/main.go`'yi API Gateway proxy'sine cevir (veya tamamen kaldir)
2. Frontend'in tum route'larin yeni servislere gittigini dogrula
3. Monolith deployment'ini kaldir

---

## Observability

### Her Servis Icin Minimum

Pod template annotation'lari (Prometheus operator auto-discovery — zlitter ile
ayni pattern):

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"     # her servis kendi PORT'u
  prometheus.io/path: /metrics
```

Servisin standart endpoint'leri:

| Endpoint | Amaç |
|---|---|
| `/metrics` | Prometheus exposition (Go runtime + business metrics) |
| `/health` | Liveness probe — process up |
| `/ready` | Readiness probe — DB/upstream check (Catalog, Query Engine) |

`/ready` Catalog Service'te PostgreSQL ping, Query Engine'de Catalog HTTP ping,
AI Service'te ise Catalog + Query ping yapar.

Structured logging: `log/slog` JSON formati, `traceparent` header propagasyonu
(OpenTelemetry W3C trace context).

### Recommended Stack

| Tool | Purpose |
|------|---------|
| Prometheus | Metrics collection |
| Grafana | Dashboards |
| Loki veya ELK | Log aggregation |
| Jaeger veya Tempo | Distributed tracing |
| OpenTelemetry Collector | Trace/metric pipeline |

### Key Metrics Per Service

| Service | Metric | Alert Threshold |
|---------|--------|----------------|
| AI Service | `llm_request_duration_seconds` | p99 > 30s |
| AI Service | `llm_tokens_used_total` | cost anomaly |
| AI Service | `prompt_build_duration_seconds` | p99 > 500ms |
| Query Engine | `query_compile_duration_seconds` | p99 > 100ms |
| Query Engine | `query_execute_duration_seconds` | p99 > 5s |
| Query Engine | `query_rows_returned` | > maxRows |
| Catalog | `db_query_duration_seconds` | p99 > 200ms |
| Catalog | `model_publish_duration_seconds` | p99 > 1s |

---

## Trade-off Kararlari

| Karar | Secenek A | Secenek B | Tercih | Neden |
|-------|-----------|-----------|--------|-------|
| Communication | HTTP/JSON | gRPC | **HTTP/JSON baslangicta** | Debug kolay, tooling basit |
| DB isolation | Shared PostgreSQL | Separate instances | **Shared instance, separate schemas** | Cost, operasyonel basitlik |
| Code structure | Multi-repo | Mono-repo | **Mono-repo, multi-binary** | Atomic refactoring, shared types |
| Service mesh | Istio/Linkerd | CNI L7 (Cilium) | **Cilium CNI only** | zlitter standardi, eBPF yeterli |
| Ingress | Ingress (Traefik/Nginx) | Gateway API | **Cilium Gateway API** | zlitter `lan-gw` paylasimi, native L7, HTTPRoute granularity |
| Load balancer | MetalLB | Cilium LB-IPAM | **Cilium LB-IPAM + L2 announce** | zlitter ile ayni, ek bilesen yok |
| NetworkPolicy | k8s NP | CiliumNetworkPolicy | **CiliumNetworkPolicy** | L7/FQDN/Entity desteklemesi (toFQDNs, toEntities) |
| TLS | Per-service cert | Wildcard at Gateway | **Wildcard at Gateway** | cert-manager + tek Secret, servisler temiz |
| Deployment | k8s Deployment | Argo Rollouts | **Deployment + Rollouts hibrit** | Catalog/Query Deployment, AI Rollouts (canary) |
| GitOps | Flux | ArgoCD | **ArgoCD** | zlitter ile ayni cluster Application |
| Config | ConfigMap + Secret | External config service | **ConfigMap + Secret + envFrom** | K8s native, checksum/config ile auto restart |
| Image registry | Docker Hub | GHCR | **GHCR + ghcr-registry pullSecret** | zlitter ile ayni, reflector mirror |
| Migrations | Shared migration table | Per-service migrations | **Shared migration, labeled by service** | DB shared oldugu icin |

---

## Security Checklist

### Cluster / Namespace
- [ ] Tek `biqly` ServiceAccount, `automountServiceAccountToken: false`
- [ ] Pod Security Standards: `restricted` profile (namespace label)
- [ ] `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, drop `ALL` capabilities
- [ ] Tum container'larda resource requests + limits tanimli
- [ ] Image'lar GHCR'dan `sha-<commit>` immutable tag ile cekilir, `imagePullPolicy: Always`

### Cilium Network Policies
- [ ] `biqly-allow-dns` — kube-dns icin egress (toFQDNs onkosulu)
- [ ] `biqly-allow-gateway` — Gateway `ingress` + intra-namespace ingress
- [ ] `biqly-egress-metadata` — `cluster` entity, postgres + dragonfly portlari
- [ ] `biqly-query-egress-user-dbs` — sadece Query Engine, user DB CIDR/portlar
- [ ] `biqly-ai-egress-external` — sadece AI Service, `toFQDNs: api.openai.com / api.anthropic.com`
- [ ] Catalog ve digerlerinin world egress'i KAPALI (sadece intra-cluster)

### Secrets
- [ ] `biqly-db` — sadece Catalog + Query mount eder
- [ ] `biqly-security` (BI_ENCRYPTION_KEY) — sadece Catalog + Query mount eder
- [ ] `biqly-ai-secrets` (LLM API key) — sadece AI Service mount eder
- [ ] `ghcr-registry` — reflector ile namespace'e mirror (zlitter pattern)
- [ ] User DB credential'lari read-only role ile uretilir, encrypted DSN olarak saklanir

### Internal API
- [ ] `/internal/*` endpoint'leri sadece intra-cluster (Gateway HTTPRoute match etmez)
- [ ] Servis-to-servis JWT veya mTLS (Cilium ClusterMesh / Cilium Authentication)
- [ ] Cilium L7 policy: `/internal` path'i sadece `app.kubernetes.io/instance=biqly`
       endpoint'lerden gelebilir

### Gateway / TLS
- [ ] `lan-gw` listener `mode: Terminate`, cert-manager wildcard sertifikasi
- [ ] HTTP -> HTTPS redirect (HTTPRoute `RequestRedirect` filter)
- [ ] Rate limit (Cilium L7 policy veya Envoy Gateway annotation)

---

## Risks ve Mitigation

| Risk | Etki | Mitigation |
|------|------|------------|
| Network latency eklendikten sonra response time artar | AI + Query path'e 2 ek HTTP call | Connection pooling, keep-alive, internal API response cache |
| Distributed transaction gerekebilir | AI history + query history ayni anda yazilmali | Saga pattern veya eventual consistency (async write) |
| Catalog Service down olursa digerleri calisamaz | Single point of failure | 2+ replica, readiness probe, circuit breaker |
| Servisler arasi type mismatch | Runtime error | Paylasilan Go module + integration test |
| Migration complexity | Long-running schema changes | Backward-compatible migration, blue-green DB deploy |

---

*Document generated on 2026-05-19, updated to match the `zlitter` namespace
production standard: Cilium Gateway API + HTTPRoute, CiliumNetworkPolicy, Cilium
LB-IPAM, ArgoCD + Argo Rollouts GitOps, Helm umbrella chart, immutable
`sha-<commit>` GHCR image tags, `envFrom` + `checksum/config` rolling updates,
initContainer downstream waits, prometheus scrape annotations. Based on
analysis of all Go files under `internal/`, dependency mapping, API surface
analysis, and live cluster inspection (`zlitter` namespace).*
