# Biqly Microservice Migration Checklist

`docs/microservice-decomposition.md` plan'inin uygulama takip listesi. Her madde
tamamlandikca `[ ]` → `[x]` yapilir. Backend once, infra/deployment sonra.

Legend: `[ ]` pending · `[~]` in-progress · `[x]` done · `[-]` cancelled

---

## Phase 0 — Paylasilan Foundation (Backend)

- [x] **B0.1** `pkg/logicalquery/types.go` — LogicalQuery, SelectItem, Filter, GroupBy, OrderBy, CTE, WindowSpec tipleri `internal/query`'den tasi
      _Kanonik tip + sabitler + saf metotlar `pkg/logicalquery/`'e tasindi. `internal/query/logical.go` alias-shim'e ceviildi (`//revive:disable:exported`)._
- [x] **B0.2** `pkg/semantic/types.go` — SemanticModel, Dimension, Metric, Join tiplerini `internal/semantic`'ten tasi
      _Saf DTO + sabitler `pkg/semantic/`'e tasindi. Davranissal kod (MetricRegistry, repository, publisher) `internal/semantic/` icinde kaldi. `internal/semantic/model.go` alias-shim._
- [x] **B0.3** `pkg/metadata/types.go` — Datasource, Table, Column, Relation tiplerini `internal/metadata`'dan tasi
      _**Sapma notu:** `Datasource.RuntimeDSN` metodu, `security.Encryption` ve `datasource.Driver` bagimliliklari icerdiginden DTO paketinden cikarildi. `internal/metadata` icinde standalone `RuntimeDSN(ds, enc)` free function olarak yasiyor. 6 call site (ai/describe, http/handlers/datasources, app/datasource_resolve, core/query_service, datasource_runtime_test) guncellendi. Sebep: `pkg/` paketleri saf data + saf helper olmali; I/O ve crypto bagimliligi `internal/`'de kalmali._
- [x] **B0.4** `pkg/query/types.go` — CompiledQuery, RunResult, HistoryEntry tipleri
      _`Result`, `Stats`, `CompiledQuery`, `HistoryEntry`, `ValidationError`, `ValidationErrors` + format/chart/semantic sabitleri `pkg/query/`'e tasindi. `internal/query/result.go` alias-shim; `QueryResult/QueryStats/QueryHistoryEntry` deprecated alias'lar korundu._
- [x] **B0.5** `pkg/security/types.go` — PermissionPolicy, denied fields, row filter tipleri
      _`PermissionPolicy` + `RowFilter` `pkg/security/`'e tasindi. `PermissionManager` davranissal kod ve helper'lar `internal/security/` icinde kaldi._
- [x] **B0.6** `pkg/common/errors.go` — ServiceError, ErrorCode enum, HTTP status mapping
      _Kanonik `pkg/common/errors.ServiceError` eklendi: HTTP status + `pkg/internalapi.Error` envelope round-trip + `Unwrap()` + `HTTPStatusFor(code)` mapping. `internal/core.ServiceError` (query-layer specific) yerinde kaldi — onunla ayni shape degil; gelecekteki servislerin inter-service hata sozlesmesi icin yeni paket kullanilacak. Unit testler `pkg/common/errors/errors_test.go`'da._
- [~] **B0.7** `pkg/catalogclient` — typed HTTP client (Get/List/Create + retry + timeout + circuit breaker)
      _Phase 1: temel client + HTTPDoer seam + APIError + sentinel error'lar + caller-supplied timeout + `WithCaller` audit header var. Retry policy ve circuit breaker B6.8/B6.9'a ertelendi._
- [~] **B0.8** `pkg/queryclient` — typed HTTP client (Compile/Run/DryRun + retry + timeout)
      _Phase 1: temel client + query-spesifik sentinels (ErrCompile/Execution/Permission/ReadOnly) + `WithCaller` audit header var. Retry B6.8'e ertelendi._
- [x] **B0.9** `pkg/aiclient` — typed HTTP client (Query/Preview/Run/Describe)
      _`pkg/aiclient/` paketi: `Client`, functional options (`WithHTTPClient`, `WithTimeout`, `WithCaller`), `HTTPDoer` interface, sentinel error'lar (`ErrNotFound`/`ErrInvalidRequest`/`ErrUnauthorized`/`ErrUpstream`/`ErrNeedsClarification`), `APIError` envelope, `Query/Preview/Run/Describe/Embed/Settings` metotlari + `httptest.Server` ile tam unit-test kapsama (subagent'la paralel olarak yapildi)._
- [~] **B0.10** `internal/config` refactor — her binary kendi env var'larini, ortak `BI_*` setini paylasir
      _**Erteleme notu:** Per-binary config split, Phase 2-4'te `services/<x>/cmd/main.go` dosyalari ortaya cikinca yapilacak — su an kim hangi env var'a ihtiyac duyacak netlesmemis. Phase 1 monolith icinde tek `internal/config` tek sumda hizmet veriyor; bu tasidigimiz tiplerin servis sinirina degmeden split etmek erken-optimizasyon. Phase 2'de Catalog Service cikarken bu madde tekrar acilacak ve `pkg/common/config` shared base + per-service overlay seklinde tasarlanacak._
- [x] **B0.11** `internal/` paketleri yeni `pkg/` tiplerini import edecek sekilde guncelle (drop-in alias)
      _Tum shim dosyalarda `//revive:disable:exported` directive ile alias gruplarinin doc-comment gurultusu susturuldu. `go build ./...` + `go test ./...` yesil._
- [x] **B0.12** `go test ./...` + `golangci-lint run` yesil, mevcut davranis bozulmadi
      _Tum migrated paketler (`pkg/logicalquery`, `pkg/query`, `pkg/semantic`, `pkg/metadata`, `pkg/security`, `pkg/common/errors`, `pkg/aiclient`) + 5 alias shim lint-clean (`golangci-lint run` 0 issue). Full `go test ./...` exit 0. Pre-existing 75 lint issue'lari `internal/query/compiler.go`, `internal/semantic/repository.go` gibi dokunmadigimiz alanlarda, scope-out._

---

## Phase 1 — Internal API Layer (Backend, monolith hala tek binary)

### 1A. Catalog read endpoints

- [x] **B1.1** `GET /internal/models/{id}/full` — published SemanticModel (dimensions, metrics, joins)
      _Impl path: `GET /internal/models/{id}` (`/full` suffix kaldirildi — internal sadece published full model donuyor zaten)._
- [x] **B1.2** `GET /internal/models?datasource_id=` — model listesi
- [x] **B1.3** `GET /internal/datasources/{id}` — Datasource (DSN decrypted, internal only)
      _Tasarim sapmasi: DSN encrypted donuyor; peer service `BI_ENCRYPTION_KEY`'i paylasarak yerel decrypt eder. Kume ici dahi plaintext kimlik ucmamasi icin bilincli karar (decomposition.md Security Checklist)._
- [x] **B1.4** `GET /internal/tables?datasource_id=` — synced tables
      _Impl path: `GET /internal/datasources/{id}/tables` (RESTful nested; opsiyonel `?schema_name=` filtresi)._
- [x] **B1.5** `GET /internal/columns?datasource_id=` — synced columns
      _Impl path: `GET /internal/datasources/{id}/columns` (opsiyonel `?schema_name=&table_name=`)._
- [x] **B1.6** `GET /internal/relations?datasource_id=` — FK relations
      _Impl path: `GET /internal/datasources/{id}/relations`._
- [x] **B1.7** `GET /internal/few-shot?datasource_id=&model_id=` — curated examples
- [x] **B1.8** `GET /internal/glossary?datasource_id=&model_id=` — business glossary

### 1B. Catalog write endpoints

- [x] **B1.9** `POST /internal/ai-history` — AI query history insert
      _Impl path: `POST /internal/history/ai`. `internalapi.AIHistoryRequest{Entry}` body, kanonik envelope donusu, `entry.datasource_id` zorunlu validasyonu._
- [x] **B1.10** `POST /internal/query-history` — Query Engine history insert
      _Impl path: `POST /internal/history/query`._
- [x] **B1.11** `POST /internal/eval-results` — Eval result persistence
      _`internalapi.EvalResultsRequest` batch kontrati + `InternalHandler.CreateEvalResults` + `catalogclient.CreateEvalResults` eklendi. Catalog `EvalRepository.SaveRunResults` ile case result'lari ve run summary'yi persist eder._

### 1C. Query internal endpoints

- [x] **B1.12** `POST /internal/query/compile-with-context` — LogicalQuery + context → SQL
      _Impl path: `POST /internal/query/compile`. `internalapi.CompileRequest{LogicalQuery}` body. "with-context" overlay'i (denied fields, row filters) Phase 1'de monolith ici uretiliyor; ileride peer service'ler kendi context'lerini POST etmek istediginde request body buyutulur._
- [x] **B1.13** `POST /internal/query/run-with-context` — LogicalQuery + context → execute
      _Impl path: `POST /internal/query/run`. Per-request `max_rows`/`timeout_ms` advisory override gondermek mumkun (server global tavanda cap atar)._
- [x] **B1.14** `POST /internal/query/dry-run` — EXPLAIN gate
      _Impl path: `POST /internal/query/dry-run`. Su an compile + warnings donuyor; dialect.ExplainSQL ile gercek EXPLAIN gate'i Phase 3'te executor ayrildiginda devreye girecek._

### 1D. Security + tests

- [x] **B1.15** Internal auth middleware — `X-Internal-Token` shared secret header (gelecekte mTLS)
      _`BI_INTERNAL_API_TOKEN` ile fail-closed middleware eklendi. `X-Internal-Token` ve existing client davranisi icin `Authorization: Bearer` kabul ediliyor; unset token 403, hatali/missing token 401._
- [x] **B1.16** Internal endpoint'ler `/api/*` HTTPRoute'tan ayri router'da (Cilium L7 policy ile kilitlenecek)
      _`router.go` icinde `r.Route("/internal", ...)` alt-agaci ile ayrildi. Cilium HTTPRoute Phase 9'da `/api/*` host'unu match edecek, `/internal/*` disariya hic verilmeyecek._
- [x] **B1.17** Internal endpoint'ler icin integration test — golden response + schema validation
      _Uc katmanli kapsam: (1) `pkg/catalogclient/client_test.go` + `pkg/queryclient/client_test.go` + `pkg/aiclient/client_test.go` httptest round-trip'leri, (2) `internal/http/handlers/internal_test.go` per-handler birim test'leri, (3) **YENI** `internal/http/handlers/internal_integration_test.go` router-level golden-response suite — chi `/internal` agacinin tamami (auth + audit middleware + 15 endpoint round-trip + 3 golden JSON `testdata/internal_golden/`). `internal_ports.go` ile catalog/eval/query bagimliliklari arayuze cikarildi, fake'lerle testte enjekte ediliyor. **Sapmalar:** (a) `metadata.Datasource.DSNEncrypted` JSON-skip oldugu icin wire-test plaintext-leak negatifi yapiyor (B1.3 tasarimi peer'larin shared `BI_ENCRYPTION_KEY` ile lokal decrypt'i — wire'da ciphertext bile gozukmuyor). (b) `/internal/query/*` hatalari hala legacy `{"error":"..."}` zarfini kullaniyor; `internalapi.Error` zarfine unify Phase 3'te query handler extract olunca yapilacak. (c) Gercek-DB golden suite Phase 2'de Catalog Service extract olunca devreye girecek (now: stub repositories enjekte ediliyor)._
- [x] **B1.18** Internal endpoint'ler audit log'a yazsin (`source=service`, `caller=ai|query|catalog`)
      _`EventInternalRequest` + `/internal/*` audit middleware eklendi. `X-Internal-Caller` client option'i ile `caller`, `source=service`, method/path/status detaylari loglanir._

---

## Phase 2 — Catalog Service Extraction (Backend)

- [x] **B2.1** `services/catalog/cmd/main.go` — chi router, graceful shutdown, slog logger
      _Standalone Catalog Service entrypoint eklendi: `services/catalog/cmd/main.go`. `app.NewCatalogDependencies` sadece metadata DB, datasource registry, metadata/semantic repo, encryptor, eval repo ve audit logger kuruyor; AI provider/query service baslatmiyor. `internal/http.CatalogRouter` catalog-only route tree'i mount ediyor. `go build -o /tmp/biqly-catalog ./services/catalog/cmd` yesil._
- [~] **B2.2** `services/catalog/internal/` altina `internal/metadata/`, `internal/semantic/`, `internal/semanticgen/` paketlerini tasi
      _**Best-practice sapma:** fiziksel package move su an ertelendi. Yeni binary repo-root `internal/*` paketlerini import edebiliyor ve servis seam'i calisiyor. Paketleri bugun kopyalamak/bolmek duplicate ownership + import churn yaratirdi. Once process boundary (`CatalogRouter` + `NewCatalogDependencies`) kuruldu; B2.8 proxy gecisi ve B2.10 entegrasyon testinden sonra package move daraltma yapilacak._
- [~] **B2.3** `services/catalog/internal/handlers/datasources.go` — CRUD + test + sync
      _Route-level olarak Catalog Service icinde aktif: `/api/datasources*` mevcut `DatasourceHandler` ile mount edildi. Fiziksel handler tasima B2.2 ile birlikte yapilacak._
- [~] **B2.4** `services/catalog/internal/handlers/metadata.go` — table/column search + update
      _Route-level olarak Catalog Service icinde aktif: `/api/datasources/{id}/tables`, `/columns`, `/api/metadata/*` mevcut `MetadataHandler` ile mount edildi. Fiziksel handler tasima B2.2 ile birlikte yapilacak._
- [~] **B2.5** `services/catalog/internal/handlers/semantic.go` — model CRUD + validate + publish + rollback
      _Route-level olarak Catalog Service icinde aktif: `/api/semantic/models*` mevcut `SemanticHandler` ile mount edildi. `GenerateModel` AI provider kullanmadigi icin Catalog tarafinda tutuldu (semanticgen metadata tabanli). Fiziksel handler tasima B2.2 ile birlikte yapilacak._
- [~] **B2.6** `services/catalog/internal/handlers/internal.go` — `/internal/*` read + write endpoint'leri
      _Catalog-owned internal endpoints Catalog Service router'inda mount edildi: health, datasource/model/table/column/relation read, few-shot/glossary read, history/eval write. Query internal endpoints bilerek mount edilmedi; onlar Phase 3 Query Engine'e ait. `InternalHandler` health response icin service-name parametresi alacak sekilde genisletildi (`biqly-catalog`)._
- [x] **B2.7** `services/catalog/Dockerfile` — multi-stage Go build → distroless runtime
      _`services/catalog/Dockerfile` eklendi: Go 1.26.3 alpine builder -> scratch runtime, non-root user, `/biqly-catalog` entrypoint. Root `Makefile` icin `build-catalog` ve `run-catalog` target'lari da eklendi._
- [x] **B2.8** `cmd/api/main.go`'da Catalog handler'larini `pkg/catalogclient` proxy'sine cevir
      _BFF proxy mode eklendi: `BI_CATALOG_SERVICE_URL` set edilirse monolith `/api/datasources*`, `/api/metadata*`, `/api/semantic*` route'larini standalone Catalog Service'e reverse proxy eder; unset ise mevcut in-process handler'lar calismaya devam eder. **Sapma notu:** public CRUD yuzeyi icin typed `pkg/catalogclient` uygun degil (o paket `/internal/*` service-to-service kontrati). Public `/api/*` icin reverse proxy kullanildi; typed client internal read/write kullanimi icin korunuyor. Testler: `internal/http/catalog_proxy_test.go` path-preservation + upstream hata envelope dogruluyor._
- [~] **B2.9** Monolith'ten `internal/metadata` + `internal/semantic` import'larini kaldir, sadece client kullanilsin
      _**Erteleme notu:** Public Catalog route'lari artik `BI_CATALOG_SERVICE_URL` ile proxylenebiliyor, fakat monolith icindeki Query ve AI handler'lari Phase 3/4'e kadar metadata/semantic repo'lara dogrudan ihtiyac duyuyor. Bu maddeyi simdi zorlamak query/AI runtime'i kirar veya fake client shim'leriyle gereksiz karmasiklik yaratir. B3.5 (Query -> catalogclient) ve B4.3 (AI -> catalogclient) tamamlandiktan sonra monolith/BFF import temizligi dar ve guvenli sekilde yapilacak._
- [x] **B2.10** Integration test — frontend route'larin Catalog Service uzerinden calistigini dogrula
      _`internal/http/catalog_proxy_test.go` genisletildi: frontend'in kullandigi representative Catalog public route'lari (`/api/datasources*`, `/api/metadata*`, `/api/semantic*`) `BI_CATALOG_SERVICE_URL` set iken upstream httptest Catalog Service'e ayni method/path/query ile proxyleniyor. Negatif kontrol: `/api/query/compile` proxylenmiyor ve monolith'te kaliyor. Upstream hata durumunda `internalapi.Error{code=upstream_unavailable}` envelope testi de var._
- [x] **B2.11** Catalog Service prom metrics — `catalog_db_query_duration_seconds`, `model_publish_duration_seconds`
      _Process-local Prometheus text metrics eklendi: `catalog_db_queries_total`, `catalog_db_query_errors_total`, `catalog_db_query_duration_seconds`, `model_publish_total`, `model_publish_errors_total`, `model_publish_duration_seconds`. `CatalogMetricsMiddleware` Catalog-owned route latency'sini kaydeder; `SemanticHandler.PublishModel` publish latency'sini ayrica kaydeder. **Sapma notu:** repo-level DB hook henuz yok, bu yuzden `catalog_db_query_duration_seconds` handler latency'si uzerinden DB-bound Catalog request proxy metriği olarak tutuluyor. `internal/http/metrics_test.go` metric output kontratini dogrular._

---

## Phase 3 — Query Engine Extraction (Backend)

- [x] **B3.1** `services/query/cmd/main.go` — chi router, graceful shutdown
      _Standalone Query Engine entrypoint eklendi: `services/query/cmd/main.go`. `app.NewQueryDependencies` AI provider baslatmadan metadata DB, datasource registry, validator, executor, query service, encryptor ve audit logger kuruyor. `internal/http.QueryRouter` query-only route tree'i mount ediyor. `make build-query`, `go build ./...`, `go test ./...` yesil._
- [~] **B3.2** `services/query/internal/` altina `internal/query/`, `internal/dialect/`, `internal/security/`, `internal/datasource/` tasi
      _**Best-practice sapma:** fiziksel package move simdilik ertelendi. Yeni Query binary repo-root `internal/*` paketlerini import edebiliyor; bu sayede duplicate query/dialect/security/datasource ownership yaratmadan process boundary test edildi. B3.7 proxy + B3.9 integration coverage tamamlandiktan sonra daraltma yapilacak._
- [x] **B3.3** `services/query/internal/handlers/query.go` — compile, run, explain, history
      _Route-level olarak Query Engine icinde aktif: `/api/query/compile`, `/api/query/run`, `/api/query/explain`, `/api/query/history`, `/api/query/history/{id}` mevcut `QueryHandler` ile mount edildi. `internal/http/query_router_test.go` QueryRouter'in sadece query public route'larini expose ettigini ve Catalog route'larini mount etmedigini dogrular._
- [x] **B3.4** `services/query/internal/handlers/internal.go` — compile-with-context, run-with-context, dry-run
      _Route-level olarak Query Engine icinde aktif: `/internal/query/compile`, `/internal/query/run`, `/internal/query/dry-run` mevcut `InternalQueryHandler` ile mount edildi. Internal auth/audit middleware QueryRouter'da etkin._
- [x] **B3.5** Query Engine icine `pkg/catalogclient` wire — datasource + model okuma
      _`internal/app/queryCatalogAdapter` eklendi: `pkg/catalogclient.Client` -> `core.ModelLoader`, `core.DatasourceLoader`, `core.HistoryRecorder`. Query Engine `BI_CATALOG_SERVICE_URL` set ise model/datasource okuma ve query-history write islemlerini Catalog Service `/internal/*` uzerinden yapar; unset ise local repo fallback korunur. `internal/app/catalog_client_adapter_test.go` adapter'in 3 portu da dogru HTTP endpoint'lere bagladigini dogrular._
- [x] **B3.6** `services/query/Dockerfile` — multi-stage build
      _`services/query/Dockerfile` eklendi: Go 1.26.3 alpine builder -> scratch runtime, non-root user, `/biqly-query` entrypoint. Root `Makefile` icin `build-query` ve `run-query` target'lari da eklendi._
- [x] **B3.7** `cmd/api/main.go`'da query handler'larini `pkg/queryclient` proxy'sine cevir
      _BFF proxy mode eklendi: `BI_QUERY_SERVICE_URL` set edilirse monolith `/api/query*` route'larini standalone Query Engine'e reverse proxy eder; unset ise mevcut in-process `QueryHandler` calisir. **Sapma notu:** public `/api/query/*` body/response shape'i `pkg/queryclient`'in `/internal/query/*` typed kontratindan farkli oldugu icin BFF tarafinda reverse proxy kullanildi; `pkg/queryclient` AI->Query service-to-service hattinda kalacak. Testler: `internal/http/query_proxy_test.go` path-preservation, non-query negatif kontrol ve upstream hata envelope dogruluyor._
- [ ] **B3.8** Monolith'ten query/dialect/security/datasource import'larini kaldir
- [x] **B3.9** Integration test — query compile + run hala dogru sonuc veriyor
      _`internal/http/handlers/query_integration_test.go` eklendi. Public `/api/query/compile` ve `/api/query/run` handler'lari chi router uzerinden round-trip edilir; fake runner gercek `core.QueryService` compile path'ini kullanir ve run response'u deterministik row doner. Test compile SQL'de base table, join ve args kontratini; run response'ta column/row/row_count kontratini dogrular. Bunun icin `QueryHandler` compile/run/explain cagrisini concrete deps yerine mevcut `internalQueryRunner` portu uzerinden yapacak sekilde dar refactor edildi._
- [x] **B3.10** Query Engine prom metrics — `query_compile_duration_seconds`, `query_execute_duration_seconds`, `query_rows_returned`
      _Process-local Query metrics eklendi: `query_compile_total`, `query_compile_errors_total`, `query_compile_duration_seconds`, `query_execute_total`, `query_execute_errors_total`, `query_execute_duration_seconds`, `query_rows_returned`. Public `/api/query/compile`, `/api/query/explain`, `/api/query/run` ve internal `/internal/query/compile`, `/internal/query/dry-run`, `/internal/query/run` ayni `QueryMetricsRecorder` portu ile kayit aliyor. `internal/http/metrics_test.go` metric output kontratini dogrular._
- [x] **B3.11** ReadOnlyChecker + permission policy testleri yeni binary'de yesil
      _Verification-only kapatildi: `go build -o /tmp/biqly-query ./services/query/cmd` standalone Query binary build'i yesil. `go test ./internal/security -run 'TestReadOnlyChecker' -v` read-only gate testlerini, `go test ./internal/query -run 'TestGolden_RowFilterInjection|TestCompiler_PermissionInjection|TestComputeFingerprintDistinguishesPermissionScope' -v` row-level filter injection + permission fingerprint testlerini dogruladi. Fiziksel package move ertelendigi icin testler halen repo-root `internal/*` paketlerinde kosuyor; B3.2 daraltmasi yapildiginda ayni kapsam `services/query/internal/*` altina tasinacak._

---

## Phase 4 — AI Service Extraction (Backend)

- [x] **B4.1** `services/ai/cmd/main.go` — chi router, graceful shutdown
      _Standalone AI Service entrypoint eklendi: `services/ai/cmd/main.go`. `app.NewAIDependencies` metadata DB, AI providers, describer, embedder, eval repo, audit logger ve gecici olarak lokal QueryService/Executor graph'ini kuruyor. `internal/http.AIRouter` AI-only public route tree'i + `/internal/health` mount ediyor. `make build-ai`/`run-ai` target'lari eklendi. `go build -o /tmp/biqly-ai ./services/ai/cmd`, `go build ./...`, `go test ./...` yesil._
- [~] **B4.2** `services/ai/internal/ai/` — internal/ai tum dosyalari (service, prompt, table_router, client, anthropic, provider, schema, validator, describe, embedder, sample, glossary, eval, retry_helpers)
      _**Best-practice sapma:** fiziksel package move simdilik ertelendi. AI binary repo-root `internal/ai` paketlerini import edebiliyor; once process boundary + Catalog/Query client wiring (B4.3/B4.4) sabitlenecek, sonra fiziksel tasima daha dar riskle yapilacak._
- [x] **B4.3** AI Service icine `pkg/catalogclient` wire — model/metadata/glossary read + ai-history write
      _`app.NewAIDependencies` `BI_CATALOG_SERVICE_URL` set ise `pkg/catalogclient` kurar (`caller=ai`, internal token). `AIHandler` CatalogClient varsa semantic model list/get, datasource dialect hint, table-router metadata (`ListTables/ListColumns/ListRelations`), glossary, curated few-shot ve AI history write islemlerini `/internal/*` uzerinden yapar; unset ise lokal repo fallback korunur. **Sapma notu:** embedding reader CatalogClient tarafinda henuz yok, bu yuzden external Catalog mode'da table routing keyword/metadata tabanli kalir; embedding-backed routing B4 sonrasi catalog embedding endpoint'i ile genisletilecek. Test: `internal/http/handlers/ai_catalogclient_test.go`._
- [x] **B4.4** AI Service icine `pkg/queryclient` wire — compile/run/dry-run
      _`app.NewAIDependencies` `BI_QUERY_SERVICE_URL` set ise `pkg/queryclient` kurar (`caller=ai`, internal token). `AIHandler` Run fazinda SQL validator icin `/internal/query/dry-run`, Preview icin `/internal/query/dry-run`, Run icin `/internal/query/run` kullanir; unset ise mevcut lokal `QueryService` + executor path'i korunur. **Sapma notu:** QueryClient mode'da AI prompt sample-data icin user DB'ye lokal baglanmiyoruz; bu, process isolation icin bilincli tradeoff. Test: `internal/http/handlers/ai_queryclient_test.go`._
- [~] **B4.5** `services/ai/internal/handlers/query.go` — `/api/ai/query`, `/preview`, `/run`
      _Route-level olarak AI Service icinde aktif: `/api/ai/query`, `/api/ai/query/preview`, `/api/ai/query/run` mevcut `AIHandler` ile mount edildi. Fiziksel handler tasima B4.2 ile birlikte yapilacak._
- [~] **B4.6** `services/ai/internal/handlers/metadata.go` — `/metadata/describe`, `/metadata/embed`
      _Route-level olarak AI Service icinde aktif: `/api/ai/metadata/describe`, `/api/ai/metadata/embed` mevcut `AIHandler` ile mount edildi. Fiziksel handler tasima B4.2 ile birlikte yapilacak._
- [~] **B4.7** `services/ai/internal/handlers/eval.go` — `/eval/run`, `/run/stream`, `/runs`, `/regression`
      _Route-level olarak AI Service icinde aktif: admin-gated `/api/ai/eval/run`, `/run/stream`, `/runs`, `/runs/{id}`, `/regression` mevcut `AIHandler` ile mount edildi. Fiziksel handler tasima B4.2 ile birlikte yapilacak._
- [~] **B4.8** `services/ai/internal/handlers/examples.go` — `/examples`, `/feedback`, `/glossary`, `/usage`, `/settings`, `/stats/models`
      _Route-level olarak AI Service icinde aktif: examples, feedback, usage, example-ids, model stats, glossary ve settings route'lari `AIRouter` altinda mount edildi. `internal/http/ai_router_test.go` AI route izolasyonunu ve internal health auth'unu dogrular. Fiziksel handler tasima B4.2 ile birlikte yapilacak._
- [x] **B4.9** `services/ai/Dockerfile` — multi-stage build
      _`services/ai/Dockerfile` eklendi: Go 1.26.3 alpine builder -> scratch runtime, non-root user, `/biqly-ai` entrypoint. Root `Makefile` icin `build-ai` ve `run-ai` target'lari da eklendi._
- [x] **B4.10** `cmd/api/main.go`'da AI handler'larini `pkg/aiclient` proxy'sine cevir, ardindan kaldir
      _BFF proxy mode eklendi: `BI_AI_SERVICE_URL` set edilirse monolith `/api/ai*` route'larini standalone AI Service'e reverse proxy eder; unset ise mevcut in-process AI handler'lar calismaya devam eder. **Sapma notu:** public `/api/ai/*` yuzeyi typed `pkg/aiclient` internal kontratindan genis oldugu icin Catalog/Query gibi reverse proxy kullanildi; `pkg/aiclient` service-to-service/internal cagrilar icin korunuyor. Testler: `internal/http/ai_proxy_test.go` path-preservation, non-AI negatif kontrol ve upstream hata envelope dogruluyor._
- [x] **B4.11** Self-consistency + clarification flow yeni binary'de calisiyor
      _Verification-only kapatildi: `go build -o /tmp/biqly-ai ./services/ai/cmd` standalone AI binary build'i yesil. `go test ./internal/ai -run 'TestProcessQuestionMultiCandidate|TestProcessQuestion.*Clarification|TestTableRouter_RouteNeedsClarification|TestClarificationFromRouting|TestGoldenSeedSelfConsistent|TestBenchmarkSuiteSelfConsistent' -v` self-consistency voting, no-majority fallback, retry-sonrasi clarification, routing clarification ve golden/eval consistency kapsamlarini dogruladi. `go test ./internal/http -run 'TestAIRouter|TestRouter_ProxiesAIOwnedPublicRoutes' -v` yeni AI router/BFF proxy yuzeyini dogruladi._
- [x] **B4.12** AI Service prom metrics — `llm_request_duration_seconds`, `llm_tokens_used_total`, `prompt_build_duration_seconds`
      _Process-local AI/LLM metrics eklendi: `llm_request_duration_seconds`, `llm_tokens_used_total`, `prompt_build_duration_seconds`. `PromptStats` artik `prompt_build_duration_ms` tasiyor; `AIHandler.observeAIRequest` token usage ve prompt build latency'yi metrics recorder'a iletir. `internal/http/metrics_test.go` Prometheus text output kontratini dogrular._

---

## Phase 5 — Monolith Sonlandirma (Backend)

- [x] **B5.1** `cmd/api/main.go` proxy modu — frontend BFF (CORS, auth, fan-out)
      _`cmd/api`/`internal/http.Router` artik `BI_CATALOG_SERVICE_URL`, `BI_QUERY_SERVICE_URL`, `BI_AI_SERVICE_URL` set edildiginde public `/api/*` yuzeyini frontend BFF olarak uc servise reverse proxy eder; unset domain'ler lokal in-process handler'a fallback eder. CORS/middleware public router'da kalir, servis route ownership'i `catalog_proxy`, `query_proxy`, `ai_proxy` ile ayrildi._
- [x] **B5.2** Tum frontend route'larin uc servise gittigini dogrula (HTTP trace)
      _`internal/http/bff_proxy_test.go` eklendi: tek Router icinde representative frontend route'lari Catalog (`/api/datasources`, `/api/metadata/*`, `/api/semantic/*`), Query (`/api/query/*`) ve AI (`/api/ai/*`) upstream httptest server'larina path/query korunarak dagitiliyor. `X-Forwarded-Host` trace header'i da dogrulaniyor._
- [ ] **B5.3** `cmd/api` tamamen kaldir veya minimum BFF olarak birak (karar an'inda netlesir)
- [ ] **B5.4** `internal/app/dependencies.go` artik kullanilmiyor → kaldir
- [x] **B5.5** README + dev docs guncelle (uc binary calistirma)
      _README local microservice mode ile guncellendi: `make run-catalog`, `make run-query`, `make run-ai`, BFF `make run` ve `BI_*_SERVICE_URL` env var'lariyla dort terminal lokal calisma akisi belgelendi._

---

## Phase 6 — Cross-Cutting Backend Concerns

- [x] **B6.1** Tum servislerde `/metrics` Prometheus endpoint (Go runtime + business metrics)
      _`/metrics` endpoint'i `cmd/api`, Catalog, Query ve AI router'larinda mevcut. Business metriklerine ek olarak `go_goroutines`, `go_memstats_alloc_bytes`, `process_uptime_seconds` runtime/process metrikleri eklendi ve `internal/http/metrics_test.go` ile dogrulandi._
- [x] **B6.2** Tum servislerde `/health` (liveness — process up)
      _Tum public service router'lari `/health` liveness endpoint'i expose ediyor: monolith/BFF, Catalog, Query, AI. Internal `/internal/health` servis endpoint'leri de token korumali kaldi._
- [x] **B6.3** Tum servislerde `/ready` (readiness — DB ping + upstream HTTP ping)
      _`internal/http/readiness.go` eklendi. Readiness Metadata DB varsa `PingContext`, konfigure upstream varsa `{baseURL}/health` kontrolu yapar. Catalog sadece DB; Query Catalog; AI Catalog+Query; BFF Catalog+Query+AI upstream'lerini kontrol eder. Testler: upstream OK/degraded ve tum router'larda `/ready` expose kontrolu._
- [x] **B6.4** slog JSON logger — TR/EN level kontrolu, correlation ID propagation
      _Dort backend binary artik `internal/platform/logger` kullaniyor. `BI_LOG_LEVEL` (`debug|info|warn|error`) ve `BI_LOG_FORMAT` (`json` default, `text` opsiyonel) config'e eklendi. `X-Request-ID` chi request ID'den context'e aliniyor; `catalogclient`, `queryclient`, `aiclient` ve BFF reverse proxy upstream isteklerine ayni ID'yi propagate ediyor._
- [x] **B6.5** OpenTelemetry `traceparent` middleware — AI → Query → Catalog tum chain
      _Tam OTEL SDK instrumentation yerine simdilik W3C `traceparent` pass-through middleware eklendi. Incoming `traceparent` context'e tasiniyor; typed internal client'lar ve BFF proxy upstream request'lerine header'i koruyarak iletiyor. Bu, AI → Query → Catalog zincirinde gateway/collector tarafindan verilen trace ID'nin kaybolmamasini saglar; span üretimi Helm/OTEL collector fazina birakildi._
- [x] **B6.6** Graceful shutdown — SIGTERM → server.Shutdown(ctx) + DB pool close, 30s drain
      _`cmd/api`, Catalog, Query ve AI binary'lerinde mevcut SIGINT/SIGTERM shutdown akisi 30s drain olacak sekilde guncellendi. `server.Shutdown(ctx)` tamamlandiktan sonra `deps.Close()` defer'i DB pool/resource kapatisini yapmaya devam eder._
- [x] **B6.7** HTTP client'larda timeout + max idle conns + keep-alive ayari
      _`pkg/common/httpclient` eklendi. `catalogclient`, `queryclient`, `aiclient` artik varsayilan olarak internal service transport'u kullaniyor: dial timeout, TLS handshake timeout, response header timeout, keep-alive ve idle connection pool ayarlari ortak. Custom `WithHTTPClient` test/ozel transport destegi aynen korundu._
- [x] **B6.8** Internal endpoint'ler arasinda retry policy (exponential backoff, max 3)
      _`pkg/common/httpclient.DoWithRetry` eklendi. `catalogclient`, `queryclient`, `aiclient` default olarak max 3 attempt + exponential backoff ile transport error ve 502/503/504 transient response'larini retry eder. Request body her attempt'te yeniden olusturulur; context cancel/deadline retry edilmez._
- [x] **B6.9** Circuit breaker (gobreaker veya sony/gobreaker) — Catalog down olursa AI/Query degrade modunda
      _Ek dependency almadan `pkg/common/httpclient.CircuitBreaker` eklendi. Typed service client'lar consecutive transient failure threshold sonrasinda kisa sure `ErrCircuitOpen` doner; basarili response breaker'i resetler. Bu sayede Catalog/Query/AI upstream down durumunda caller thread'leri gereksiz retry firtinasina girmez. `WithCircuitBreaker(nil)` ile test/ozel durumlarda kapatilabilir._
- [x] **B6.10** Audit log her servis icin — kim, ne zaman, hangi sorgu/model
      _Catalog, Query, AI ve BFF internal router'lari `InternalAuditMiddleware` kullaniyor; internal audit event'i caller/method/path/status yaninda `request_id` ve `traceparent` detaylarini da yazar. Query compile/run history ve AI history persistence onceki fazlarda Catalog/local repo fallback ile korunmustu. Not: public query/AI ayrintilari `query_history`/`ai_query_history`; service-to-service erisimler structured audit log uzerinden izlenir._

---

## Phase 7 — Helm Chart (Infra)

- [x] **I1.1** `deploy/helm/biqly/Chart.yaml` umbrella + dependency: ai, query, catalog subchart
      _Umbrella chart `deploy/helm/biqly` eklendi. `catalog`, `query`, `ai` subchart'lari file dependency olarak baglandi; `helm dependency build` Chart.lock uretip basariyla calisti._
- [x] **I1.2** `deploy/helm/biqly/values.yaml` — global image registry, pullSecret, gateway, postgres, redis
      _Global image registry, imagePullSecrets, serviceAccount, Gateway parentRef/hostnames, ortak config, metadata/security/AI secret placeholder'lari ve servis bazli port/resource/HPA/route config'leri values'a eklendi._
- [x] **I1.3** `deploy/helm/biqly/values-dev.yaml`, `values-staging.yaml`, `values-prod.yaml` overlay
      _Dev/staging/prod overlay dosyalari eklendi. Dev tek replica ve text/debug log; staging/prod daha yuksek replica/HPA limitleriyle ayrildi._
- [x] **I1.4** Subchart `charts/ai/` — Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret template
      _AI subchart eklendi: Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret. `/health`, `/ready`, `/metrics`, securityContext, Prometheus annotations ve config/secret checksum rollout annotation'lari var._
- [x] **I1.5** Subchart `charts/query/` — Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret template
      _Query subchart eklendi: Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret. Catalog upstream URL, query runtime config, probes, metrics ve restricted container security ayarlari values'tan geliyor._
- [x] **I1.6** Subchart `charts/catalog/` — Deployment, Service, HPA, HTTPRoute, ConfigMap, Secret template
      _Catalog subchart eklendi: Datasource/metadata/semantic route prefix'leri Gateway HTTPRoute uzerinden Catalog Service'e gidiyor; metadata DB/encryption/internal token secret template'i var._
- [x] **I1.7** `_helpers.tpl` — labels, selectorLabels, fullname, serviceAccountName helper
      _Umbrella ve her subchart icin helper template'leri eklendi: fullname, chart label, common labels, selectorLabels, serviceAccountName ve image helper'lari._
- [x] **I1.8** `checksum/config` annotation — `sha256sum` Helm template
      _Subchart Deployment pod template'lerinde `checksum/config` ve `checksum/secret` annotation'lari rendered ConfigMap/Secret template'lerinden `sha256sum` ile uretiliyor._
- [x] **I1.9** `helm lint` + `helm template` snapshot test
      _`make helm-lint` ve `make helm-template` hedefleri eklendi. `helm lint deploy/helm/biqly` ve `helm template biqly deploy/helm/biqly` dummy validation secret override'lariyla basarili calisti; template output `/tmp/biqly-helm-template.yaml` uzerinden snapshot/debug icin uretiliyor._
- [~] **I1.10** `helm install --dry-run` ile dev cluster'da validate
      _Local/client-side `helm install biqly deploy/helm/biqly --dry-run --debug` basarili calisti. Dev cluster kube-context dogrulamasi henuz yapilmadi; cluster erisimi/net ortam hazir oldugunda ayni komut `--dry-run=server` veya gercek dev namespace uzerinden tekrar calistirilmeli._

---

## Phase 8 — Cluster Foundation (Infra)

- [x] **I2.1** `biqly` namespace + label `kubernetes.io/metadata.name=biqly`, `name=biqly`
      _Umbrella chart `templates/namespace.yaml` eklendi. Namespace create/name/labels values ile yonetiliyor; `kubernetes.io/metadata.name`, `name=biqly` ve restricted Pod Security labels default._
- [x] **I2.2** `biqly` ServiceAccount + `automountServiceAccountToken: false`
      _Umbrella `templates/serviceaccount.yaml` mevcut ve `automountServiceAccountToken: false` default. Subchart Deployment'lari global serviceAccountName'i kullanir._
- [x] **I2.3** `ghcr-registry` Secret + reflector ile `reflector.v1.k8s.emberstack.com/reflection-allowed=true`
      _`global.registrySecret.create` ile `ghcr-registry` dockerconfigjson Secret template'i eklendi; reflector annotations default geliyor. `create=true` oldugunda Catalog/Query/AI pod'lari bu secret'i `imagePullSecrets` olarak otomatik referanslar._
- [~] **I2.4** `wildcard-il1-nl-tls` Secret reflector ile `gateway` ns'den biqly ns'ye mirror (gerekirse)
      _Opsiyonel `global.reflectedTLSSecret.create` template'i eklendi. Varsayilan kapali; gateway namespace'den Reflector otomatik mirror ediyorsa chart tarafinda yaratilmamali. Manuel hedef secret gerekiyorsa values ile tls.crt/tls.key override edilerek yaratilir._
- [x] **I2.5** `biqly-config` ConfigMap (BI_HTTP_PORT, BI_QUERY_TIMEOUT_SECONDS, BI_QUERY_MAX_ROWS, BI_LOG_LEVEL)
      _Umbrella `templates/configmaps.yaml` icinde `biqly-config` eklendi. Ortak log/query/redis config values'tan gelir ve tum servis deployment'lari envFrom ile kullanir._
- [x] **I2.6** `biqly-ai-config` ConfigMap (BI_AI_PROVIDER, BI_AI_MODEL, BI_AI_TEMPERATURE, BI_AI_EMBEDDING_MODEL)
      _`biqly-ai-config` ConfigMap eklendi; AI provider/model/temperature ve opsiyonel embedding model values'tan gelir. AI Deployment envFrom ile baglandi._
- [x] **I2.7** `biqly-db` Secret (BI_METADATA_DB_DSN)
      _Umbrella `biqly-db` Secret template'i eklendi; `BI_METADATA_DB_DSN` required value olarak tutuluyor ve tum servis pod'larina envFrom ile baglandi._
- [x] **I2.8** `biqly-security` Secret (BI_ENCRYPTION_KEY, 32-byte AES)
      _Umbrella `biqly-security` Secret template'i eklendi; `BI_ENCRYPTION_KEY` required, `BI_INTERNAL_API_TOKEN` opsiyonel. Tum servis pod'lari envFrom ile kullanir._
- [x] **I2.9** `biqly-ai-secrets` Secret (BI_AI_API_KEY)
      _Umbrella `biqly-ai-secrets` Secret template'i eklendi ve AI Deployment'a baglandi. Deger values/secret manager override ile gelecek; repo'da gercek secret yok._
- [x] **I2.10** `biqly-embedding-secrets` Secret optional (BI_AI_EMBEDDING_API_KEY)
      _Umbrella `biqly-embedding-secrets` Secret template'i eklendi; AI Deployment'da optional secretRef olarak baglandi._
- [ ] **I2.11** External Secrets Operator entegrasyonu (opsiyonel, Vault/SOPS arkasinda)

---

## Phase 9 — Cilium Gateway + HTTPRoute (Infra)

- [x] **I3.1** `gateway/lan-gw` Gateway'in `*.il1.nl` listener'inda `allowedRoutes.namespaces.from: All` oldugunu dogrula
      _Read-only cluster kontrolu yapildi: `gateway.networking.k8s.io/v1 Gateway gateway/lan-gw` mevcut, `http` ve `https` listener'lari `*.il1.nl` icin `allowedRoutes.namespaces.from: All`, `Accepted=True`, `Programmed=True`; Gateway status address `192.168.0.160`._
- [x] **I3.2** `HTTPRoute biqly-catalog` — hostname `biqly.il1.nl`, paths `/api/datasources`, `/api/metadata`, `/api/semantic`, `/health`, backend `biqly-catalog:8080`
      _Catalog subchart HTTPRoute template'i `global.gateway.parentRef` ve `global.gateway.hostnames` ile render ediliyor. Default prefix'ler `/api/datasources`, `/api/metadata`, `/api/semantic`, `/health`, `/ready`, `/metrics`; backend Catalog service port `8080`._
- [x] **I3.3** `HTTPRoute biqly-query` — path `/api/query`, backend `biqly-query:8081`
      _Query subchart HTTPRoute template'i `/api/query` PathPrefix'i ile Query service port `8081` backendRef'e render ediliyor._
- [x] **I3.4** `HTTPRoute biqly-ai` — path `/api/ai`, backend `biqly-ai:8082`
      _AI subchart HTTPRoute template'i `/api/ai` PathPrefix'i ile AI service port `8082` backendRef'e render ediliyor._
- [x] **I3.5** HTTP → HTTPS redirect (RequestRedirect filter veya Gateway listener)
      _Umbrella `templates/httproute-redirect.yaml` eklendi. `global.gateway.redirect.enabled=true` olunca HTTP listener (`sectionName: http`) uzerinden `RequestRedirect` filter ile HTTPS 301 redirect render ediliyor. Default kapali; cluster Gateway listener politikasina gore acilmali._
- [~] **I3.6** (opsiyonel) `biqly-api-vip` LoadBalancer Service + `io.cilium/lb-ipam-ips` annotation
      _Opsiyonel `templates/api-vip-service.yaml` eklendi ve `global.apiVIP.enabled=false` default birakildi. Cilium Gateway zaten `io.cilium/lb-ipam-ips=["192.168.0.160"]` annotation'i ile VIP almis durumda; ayrica uygulama LoadBalancer Service'i gerekip gerekmedigi cluster ingress standardina bagli._
- [~] **I3.7** `dig biqly.il1.nl` → 192.168.0.160 (lan-gw IP) dogrula
      _`dig +short biqly.il1.nl` su anda `172.67.221.102` donuyor; beklenen LAN Gateway IP `192.168.0.160` degil. DNS/Cloudflare split-horizon veya internal resolver kaydi ayrica duzeltilmeli/dogrulanmali._
- [x] **I3.8** `curl https://biqly.il1.nl/health` → 200 dogrula
      _LAN gateway (`192.168.0.160`) uzerinden `curl -k --resolve biqly.il1.nl:443:192.168.0.160 https://biqly.il1.nl/health` → `{"status":"ok"}`. Public DNS hala Cloudflare donuyor; internal erisim icin `/etc/hosts` veya internal resolver gerekir._

---

## Phase 10 — CiliumNetworkPolicy (Infra)

- [x] **I4.1** `biqly-allow-dns` — endpointSelector component IN (ai, query, catalog), egress kube-dns 53/UDP+TCP
      _`templates/cnp-dns.yaml` eklendi. `app.kubernetes.io/component IN (ai, query, catalog)` endpoint selector ile kube-dns 53/UDP+TCP egress izni values kontrollu render ediliyor._
- [x] **I4.2** `biqly-allow-gateway` — fromEntities `ingress` + `host`/`remote-node`/`health` + intra-namespace, ports 8080/8081/8082
      _`templates/cnp-gateway.yaml` eklendi. Gateway/host/remote-node/health entities ve `app.kubernetes.io/part-of=biqly` intra-namespace endpoints icin 8080/8081/8082 ingress + egress izni render ediliyor (Query→Catalog readiness icin egress gerekli)._
- [x] **I4.3** `biqly-egress-metadata` — egress toEntities `cluster`, ports 5432 (postgres) + 6379 (dragonfly)
      _`templates/cnp-metadata.yaml` eklendi. AI/Query/Catalog pod'larindan cluster entity'lerine sadece metadata/cache portlari (5432, 6379 default) icin egress izni render ediliyor._
- [~] **I4.4** `biqly-query-egress-user-dbs` — sadece component=query, toCIDR user DB subnet, ports 5432/3306/1433/8123/9000
      _`templates/cnp-query-user-dbs.yaml` eklendi fakat default kapali. `global.networkPolicy.queryUserDbs.enabled=true` ve en az bir CIDR verilirse sadece Query component icin user DB portlari render ediliyor. CIDR verilmeden acilirsa Helm `fail` ile duruyor._
- [x] **I4.5** `biqly-ai-egress-external` — sadece component=ai, toFQDNs `api.openai.com` + `api.anthropic.com`, port 443/TCP
      _`templates/cnp-ai-external.yaml` eklendi. AI component icin values'taki FQDN listesine (`api.openai.com`, `api.anthropic.com` default) 443/TCP toFQDNs egress izni render ediliyor._
- [x] **I4.6** Cilium `enable-l7-proxy: true` configmap ayari dogrula (toFQDNs icin gerekli)
      _Read-only cluster kontrolu yapildi: `kube-system/cilium-config` icinde `enable-l7-proxy: "true"`, `enable-gateway-api: "true"`, `enable-lb-ipam: "true"` goruldu._
- [ ] **I4.7** `hubble observe` ile policy'lerin trafigi dogru izin verdigini gozle
      _Uygulama chart'i cluster'a deploy edilmedigi icin runtime traffic/policy verdict gozlemi henuz yapilmadi._
- [ ] **I4.8** Negative test — biqly-ai pod'undan `curl postgres:5432` BLOCKED olmali
      _Uygulama pod'lari deploy edilmedigi icin negatif test henuz calistirilmadi. Policy template'i AI icin metadata DB portuna explicit izin vermiyor; sonuc deploy sonrasi Hubble/kubectl exec ile dogrulanmali._

---

## Phase 11 — Data Layer (Infra)

- [x] **I5.1** `biqly-postgresql` Bitnami chart deploy — `bi_metadata` DB, primary-only (replication ileride)
      _Umbrella chart'a `bitnami/postgresql` dependency eklendi (`18.6.7`). `postgresql.enabled=true`, `architecture=standalone`, `fullnameOverride=biqly-postgresql`, `auth.database=bi_metadata`, `auth.username=biqly`, `auth.existingSecret=biqly-postgresql-auth` ile primary-only metadata DB render ediliyor. Not: Biqly image registry ayari Bitnami `global.imageRegistry` ile cakismasin diye `global.biqlyImageRegistry` olarak adlandirildi._
- [~] **I5.2** PostgreSQL StatefulSet PVC retain policy + backup CronJob (pg_dump → S3)
      _PVC retain icin Bitnami `primary.persistence.resourcePolicy=keep` ayarlandi. `templates/backup-cronjob.yaml` eklendi fakat default kapali; `global.backup.enabled=true` ve `global.backup.s3.bucket` verilirse `pg_dump` Job render ediyor. S3 upload adapter'i ortam-spesifik oldugu icin simdilik bilincli olarak placeholder komutta birakildi._
- [ ] **I5.3** Schema'lari olustur: `catalog`, `query`, `ai` (`migrations/` icindeki SQL'e schema label ekle)
      _Ertelendi: mevcut uygulama sorgulari public schema varsayiyor. Bunu dogru yapmak icin sadece SQL'e schema eklemek yetmez; repository query'leri/search_path/migration strategy birlikte ele alinmali._
- [ ] **I5.4** PgBouncer sidecar veya `bitnami/pgbouncer` — connection pooling
      _Ertelendi: mevcut Helm repo'larinda `bitnami/pgbouncer` chart bulunamadi. Connection pooling icin ya ayrik PgBouncer chart kaynagi secilecek ya da servis bazli DB pool limitleri once olculecek._
- [x] **I5.5** PostgreSQL NetworkPolicy — ingress sadece `app.kubernetes.io/instance=biqly` etiketli pod'lardan
      _`templates/cnp-postgresql.yaml` eklendi. `app.kubernetes.io/name=postgresql`, `component=primary` endpoint'ine sadece ayni release instance ve `app.kubernetes.io/part-of=biqly` pod'larindan 5432/TCP ingress izni veriliyor. Bitnami `primary.networkPolicy.allowExternal=false` de acik._
- [~] **I5.6** `biqly-postgresql-vip` LoadBalancer (opsiyonel, dev erisimi icin)
      _`templates/postgresql-vip.yaml` eklendi, default kapali. `global.postgresqlVIP.enabled=true` ile dev/ops erisimi icin LoadBalancer Service render ediliyor._
- [~] **I5.7** `biqly-dragonfly` Helm chart deploy (Redis-compatible cache, query result + AI rate limit)
      _Mevcut Helm repo'larda Dragonfly chart bulunamadigi icin umbrella template olarak `templates/dragonfly.yaml` eklendi. `biqly-dragonfly` Deployment + Service Redis-compatible 6379 portuyla render ediliyor ve `BI_REDIS_DSN=redis://biqly-dragonfly:6379` global config'e baglandi._
- [x] **I5.8** `cmd/migrate` Helm post-install/post-upgrade Job — `golang-migrate up`
      _`templates/migrate-job.yaml` eklendi. `Dockerfile.migrate` image'i `global.migrate.image` values'i ile post-install/post-upgrade hook olarak `/migrate -dir /migrations up` calistiriyor._
- [x] **I5.9** `pg_isready` initContainer her servis Deployment'inde calisiyor
      _AI/Query/Catalog Deployment template'lerine `postgres:18-alpine` tabanli `wait-for-postgres` initContainer eklendi. `BI_METADATA_DB_DSN` ile `pg_isready -d` dongusu metadata DB hazir olana kadar uygulama container'ini bekletiyor._

---

## Phase 12 — Progressive Delivery + Resilience (Infra)

- [x] **I6.1** Argo Rollouts CRD install (cluster-wide, `argo-rollouts` namespace zaten var)
      _Read-only cluster kontrolu yapildi: `rollouts.argoproj.io` ve `analysistemplates.argoproj.io` CRD'leri mevcut, `argo-rollouts` namespace mevcut._
- [x] **I6.2** `biqly-ai` Rollout — canary steps 20% (5m pause) → 50% (10m) → 100%
      _AI subchart'a `templates/rollout.yaml` eklendi. `ai.rollout.enabled=true` ile Argo Rollout render ediliyor; workloadRef mevcut Deployment'i progressively scaleDown ediyor ve canary steps 20%/5m -> 50%/10m -> 100% values'tan geliyor. Default kapali, deployment davranisi degismiyor._
- [x] **I6.3** `AnalysisTemplate ai-success-rate` — Prometheus query, `success_rate >= 0.95` threshold
      _`templates/analysis_templates.yaml` icinde `biqly-ai-success-rate` AnalysisTemplate eklendi. Prometheus query `1 - rate(bi_ai_errors) / rate(bi_ai_requests_total)` hesabi yapip threshold'u `ai.rollout.analysis.successRateThreshold` (`0.95`) ile kontrol ediyor._
- [~] **I6.4** `AnalysisTemplate ai-llm-latency` — `p99 < 30s`
      _`biqly-ai-llm-latency` AnalysisTemplate eklendi fakat mevcut metrikler histogram degil counter (`llm_request_duration_seconds`) oldugu icin gercek p99 yerine 5 dakikalik ortalama LLM latency kontrolu render ediliyor. Gercek p99 icin Phase 14'te histogram bucket metrikleri eklenmeli._
- [x] **I6.5** HPA `biqly-ai` — CPU 70%, min 2, max 8
      _AI HPA zaten render ediliyordu; Rollout acikken `scaleTargetRef` otomatik `argoproj.io/v1alpha1 Rollout` oluyor. Values: min 2, max 8, CPU 70%._
- [x] **I6.6** HPA `biqly-query` — CPU 70%, min 3, max 10
      _Query HPA values ile render ediliyor: min 3, max 10, CPU 70%._
- [x] **I6.7** HPA `biqly-catalog` — CPU 60%, min 2, max 4
      _Catalog HPA values ile render ediliyor: min 2, max 4, CPU 60%._
- [x] **I6.8** PodDisruptionBudget `biqly-ai` — minAvailable: 1
      _AI subchart'a `templates/pdb.yaml` eklendi._
- [x] **I6.9** PodDisruptionBudget `biqly-query` — minAvailable: 2
      _Query subchart'a `templates/pdb.yaml` eklendi._
- [x] **I6.10** PodDisruptionBudget `biqly-catalog` — minAvailable: 1
      _Catalog subchart'a `templates/pdb.yaml` eklendi._
- [x] **I6.11** Pod Security Standards — namespace label `pod-security.kubernetes.io/enforce=restricted`
      _Umbrella chart namespace template'i `pod-security.kubernetes.io/enforce/audit/warn=restricted` label'larini render ediyor._
- [x] **I6.12** Container securityContext — `runAsNonRoot: true`, `runAsUser: 65532`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`, `allowPrivilegeEscalation: false`, `seccompProfile.type: RuntimeDefault`
      _AI/Query/Catalog app container'lari ve Postgres wait initContainer'lari restricted securityContext ile tamamlandi. Migrate Job, Dragonfly ve backup CronJob container securityContext'leri de ayni profile cekildi (backup container writable `/tmp` ihtiyaci nedeniyle `readOnlyRootFilesystem=false`)._

---

## Phase 13 — GitOps + CI/CD (Infra)

- [x] **I7.1** ArgoCD `Application` manifest — repo `github.com/biqly/biqly`, path `deploy/helm/biqly`, automated prune+selfHeal
      _`deploy/argocd/application.yaml` eklendi. `repoURL=https://github.com/biqly/biqly.git`, `path=deploy/helm/biqly`, `valueFiles=[values-prod.yaml]`, automated `prune/selfHeal` ve `CreateNamespace/ServerSideApply` sync options tanimli. `kubectl apply --dry-run=client` basarili._
- [~] **I7.1b** Ilk cluster deploy (2026-05-19)
      _Private repo: `argocd/repo-biqly-biqly` secret zlitter PAT'ten turetildi. Private GHCR: `zlitter/ghcr-registry` reflector ile `biqly` ns'ine yansitildi; `values-prod` `imagePullSecrets: [ghcr-registry]`. App secret'lari (`biqly-postgresql-auth`, `biqly-db`, `biqly-security`, `biqly-ai-secrets`) cluster'da elle olusturuldu; chart `global.secrets.createSecrets=false`. TLS: `gateway/wildcard-il1-nl-tls` reflector ile `biqly` ns'ine yansitildi. Observability CRD template'leri prod'da kapali (Prometheus Operator yok)._
- [x] **I7.2** ArgoCD AppProject `biqly` — RBAC, allowed sources, allowed destinations
      _`deploy/argocd/project.yaml` eklendi. Source repo ve `biqly` namespace destination kisitlandi; readonly role eklendi. `kubectl apply --dry-run=client` basarili._
- [x] **I7.3** GitHub Actions `.github/workflows/build-ai.yml` — multi-arch Docker build → `ghcr.io/biqly/ai:sha-<commit>`
      _AI image workflow eklendi. PR'da build-only, main push'ta GHCR push; `linux/amd64,linux/arm64`, `sha-<commit>` ve default branch `latest` tag render ediyor._
- [x] **I7.4** GitHub Actions `.github/workflows/build-query.yml` — `ghcr.io/biqly/query:sha-<commit>`
      _Query image workflow eklendi. PR'da build-only, main push'ta GHCR push; multi-arch ve sha tag kullaniyor._
- [x] **I7.5** GitHub Actions `.github/workflows/build-catalog.yml` — `ghcr.io/biqly/catalog:sha-<commit>`
      _Catalog image workflow eklendi. PR'da build-only, main push'ta GHCR push; multi-arch ve sha tag kullaniyor._
- [x] **I7.6** GitHub Actions test workflow — `go test ./...` + `golangci-lint run` + `helm lint`
      _`.github/workflows/test.yml` eklendi. Go test, golangci-lint ve `make helm-lint`/`make helm-template` job'lari ayri calisiyor. Workflow YAML parse dogrulamasi basarili._
- [x] **I7.7** `argocd-image-updater` ile image tag otomatik bump (git write-back)
      _Cluster: `helm upgrade --install argocd-image-updater argo/argocd-image-updater -f deploy/argocd/image-updater-helm-values.yaml`. CR: `deploy/argocd/image-updater.yaml` (`namePattern: biqly`, `newest-build`, `sha-*` tags). Git: `deploy/helm/biqly/.argocd-source-biqly.yaml` + `argocd-image-updater-git` secret (see `image-updater-git-secret.example.yaml`)._
- [~] **I7.8** Branch protection — main'e direkt push yasak, PR + CI yesil + 1 review
      _GitHub branch protection repo state oldugu icin dogrudan degistirilmedi. `docs/github-branch-protection.md` eklendi; main icin PR zorunlu, 1 approval, required checks ve up-to-date branch politikasini dokumante ediyor._
- [x] **I7.9** Renovate veya Dependabot — Go module + Helm chart dependency update
      _`.github/dependabot.yml` eklendi. Go modules, frontend npm, GitHub Actions ve Dockerfile dependency update PR'lari weekly acilacak._

---

## Phase 14 — Observability (Infra)

- [~] **I8.1** ServiceMonitor (prom-operator) veya `prometheus.io/scrape` annotation ile metric scrape
      _AI/Query/Catalog Deployment'larinda `prometheus.io/scrape=true` annotation'i var. Ilk prod deploy'da `values-prod.yaml` icinde `global.observability.serviceMonitor.enabled=false` (cluster'da Prometheus Operator CRD yok; Alloy + vanilla Prometheus + Loki kullaniliyor). Sonraki adim: Alloy scrape config ile `/metrics` endpoint'lerini baglamak._
- [x] **I8.2** Grafana dashboard `biqly-ai` — LLM request duration, tokens used, cost estimate, success rate
      _`templates/grafana-dashboards.yaml` eklendi. `biqly-ai.json` request rate, success rate, average LLM latency, tokens used ve clarification metriklerini render ediyor._
- [x] **I8.3** Grafana dashboard `biqly-query` — compile/execute duration, rows returned, error rate
      _Ayni dashboard ConfigMap icinde `biqly-query.json` compile/execute duration, error rate ve rows returned panellerini render ediyor._
- [x] **I8.4** Grafana dashboard `biqly-catalog` — DB query latency, publish duration, request rate
      _Ayni dashboard ConfigMap icinde `biqly-catalog.json` Catalog DB query latency/rate/error ve model publish panellerini render ediyor._
- [~] **I8.5** Loki/Promtail veya Vector — slog JSON log ingestion, correlation ID label
      _`templates/vector-config.yaml` eklendi. Biqly pod log'larini `app.kubernetes.io/part-of=biqly` selector ile okuyan, slog JSON parse eden ve `request_id`/`traceparent` alanlarini `correlation_id` olarak Loki label'ina tasiyan Vector config render ediliyor. Runtime icin cluster genel Vector/Agent deployment'ina bu ConfigMap mount edilmeli._
- [x] **I8.6** OpenTelemetry Collector deploy — OTLP receiver, Tempo/Jaeger exporter
      _`templates/otel-collector.yaml` eklendi. OTLP gRPC/HTTP receiver, memory limiter, batch processor ve Jaeger OTLP exporter ile `biqly-otel-collector` Deployment/Service render ediliyor._
- [x] **I8.7** Tempo/Jaeger backend — distributed trace storage, retention 7d
      _`templates/jaeger.yaml` eklendi. Default olarak dev/staging icin `jaegertracing/all-in-one` OTLP-enabled backend render ediliyor. Retention all-in-one backend'in in-memory/dev sinirlariyla kisitli; production icin Tempo veya Jaeger persistent backend'e tasinmali._
- [~] **I8.8** Alertmanager rule — AI p99 LLM latency > 30s for 5m
      _`templates/prometheus-rules.yaml` icinde `BiqlyAILLMLatencyHigh` eklendi. Mevcut metrik histogram olmadigi icin p99 degil, 5 dakikalik average LLM latency > 30s olarak render ediliyor. Gercek p99 icin histogram bucket metriği eklenmeli._
- [~] **I8.9** Alertmanager rule — Query p99 compile > 100ms for 10m
      _`BiqlyQueryCompileLatencyHigh` eklendi. Mevcut `query_compile_duration_seconds` counter oldugu icin p99 yerine 10 dakikalik average compile latency > 100ms kontrolu yapiliyor._
- [~] **I8.10** Alertmanager rule — Catalog p99 DB > 200ms for 10m
      _`BiqlyCatalogDBLatencyHigh` eklendi. Mevcut `catalog_db_query_duration_seconds` counter oldugu icin p99 yerine 10 dakikalik average DB latency > 200ms kontrolu yapiliyor._
- [~] **I8.11** Alertmanager rule — Error budget burn rate (SLO based)
      _AI/Query/Catalog icin 95% success SLO'ya gore error budget burn alert'leri eklendi. Read-only cluster kontrolunde `prometheusrules.monitoring.coreos.com` CRD'si bulunamadi; Prometheus Operator CRD'leri kurulunca uygulanabilir._
- [~] **I8.12** PagerDuty/Slack webhook entegrasyonu
      _`templates/alertmanager-config.yaml` eklendi ve default kapali. `global.observability.alertmanager.enabled=true` + webhook URL Secret ile AlertmanagerConfig render ediyor. Cluster'da `alertmanagerconfigs.monitoring.coreos.com` CRD'si henuz yok; ayrica gercek PagerDuty/Slack webhook secret'i ops tarafindan saglanmali._

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
