# Wren.ai vs Biqly — Gap Analysis

> Amaç: Wren.ai referans dokümanındaki her maddeyi Biqly'nin mevcut implementasyonuyla karşılaştırmak.
> Eksikler, fazladan olanlar, olmaması gerekenler ve Go backend için uygulanacak somut maddeler belirlenmiştir.
>
> Tarih: 2026-05-31 (v2 — derinlemesine kod analizi ile güncellendi)

---

## 1. Genel Değerlendirme

| Alan | Biqly Durumu | Wren Referansı | Kapatma |
| --- | --- | --- | --- |
| NL → SQL Pipeline | Tamamı mevcut | Tam | ✅ |
| Semantic Layer (MDL karşılığı) | Tamamı mevcut | Tam | ✅ |
| Retrieval (Hybrid) | Tamamı mevcut | Tam | ✅ |
| Prompt Engineering | Tamamı mevcut | Tam | ✅ |
| Validation / Security | Tamamı mevcut | Tam | ✅ |
| Business Glossary | Backend mevcut, Frontend browser yok | Tam | ⚠️ |
| Feedback / Memory | Tamamı mevcut | Tam | ✅ |
| Eval / Regression | Tamamı mevcut | Tam | ✅ |
| Async Job Queue | NATS worker mevcut | Tam | ✅ |
| Dashboard Builder | Yok | Var | ❌ |
| Raw SQL Editor | Yok (tasarım kararı) | Var | — |
| Row-Level Security UI | Backend var, Frontend yok | Var | ❌ |

---

## 2. Wren Bölüm Bazında Karşılaştırma

### 2.1 Wren UI (Bölüm 2.1) → Frontend

| Wren Özelliği | Biqly Durumu | Detay |
| --- | --- | --- |
| Datasource bağlama | ✅ Tamamı | `Datasources.tsx` — PostgreSQL, MySQL, SQL Server, ClickHouse |
| Model/ilişki tanımlama | ✅ Tamamı | `Modeling.tsx` — visual canvas, join lines, dimension/metric palette |
| NL soru sorma | ✅ Tamamı | `AIQuery.tsx` — chat UI, multi-turn conversation |
| Generated SQL preview | ✅ Tamamı | `AssistantMessageCard.tsx` — collapsible SQL + syntax highlighting |
| Result preview / chart | ✅ Tamamı | Recharts bar/line/pie, pivot table, anomaly highlighting |
| Saved question / favorite | ✅ Tamamı | `SavedQuestions.tsx` — CRUD, favorites, few-shot toggle |
| Feedback ekranları | ✅ Tamamı | `FeedbackSection.tsx` — thumbs up/down, categories, free text |
| Dashboard builder (drag-drop) | ❌ Eksik | Mevcut `/dashboard` AI usage analytics gösteriyor. Kullanıcıların kendi dashboard oluşturabileceği bir builder yok. |
| Public embed / sharing | ⚠️ Kısmen | `ShareButton` ile kullanıcı/workspace paylaşımı var ama public iframe embed yok |

### 2.2 Wren AI Service (Bölüm 2.2) → Backend AI Layer

| Wren Özelliği | Biqly Durumu | Detay |
| --- | --- | --- |
| Intent analysis | ✅ Tamamı | `ClassifyFollowUpIntent()` — follow-up vs yeni soru sınıflandırma |
| Semantic context retrieval | ✅ Tamamı | `table_router.go` — keyword + synonym + embedding + FK graph |
| Few-shot retrieval | ✅ Tamamı | `ListSuccessfulAIQueries()` + curated examples, prompt'a injection |
| Prompt building | ✅ Tamamı | `prompt.go` — budget-aware, locale-aware, denied field filtering |
| LLM call | ✅ Tamamı | OpenAI-compatible + Anthropic provider, multi-candidate voting |
| Logical query parsing | ✅ Tamamı | `parseLogicalQueryFromRaw()` — markdown-wrapped JSON extraction |
| SQL generation | ✅ Tamamı | `compiler.go` — LogicalQuery → parameterized SQL, 4 dialect |
| Validation | ✅ Tamamı | `validator.go` — field/metric/window/CTE/HAVING validation |
| Execution preview | ✅ Tamamı | EXPLAIN dry-run, SQL preview, read-only check |
| Feedback storage | ✅ Tamamı | `InsertAIFeedback()` — rating, categories, text |
| Async job processing | ✅ Tamamı | `cmd/worker/main.go` — NATS consumer, `internal/queue/nats.go` |

### 2.3 Wren AI Core (Bölüm 2.3) → Backend Query/Semantic Layer

| Wren Özelliği | Biqly Durumu | Detay |
| --- | --- | --- |
| MDL manifest okuma | ✅ Tamamı | `SemanticModel` + `GetPublishedFullModel()` |
| Model/metric/relationship temsili | ✅ Tamamı | Dimensions, Metrics, Joins struct'ları |
| Logical sorgu planlama | ✅ Tamamı | `planner.go` — join requirement analysis, fanout risk |
| ANSI SQL / dialect uyarlama | ✅ Tamamı | 4 dialect: PostgreSQL, MySQL, SQL Server, ClickHouse |
| Validation / dry-plan | ✅ Tamamı | `validator.go` + `ReadOnlyChecker` |
| Query execution | ✅ Tamamı | `executor.go` — timeout, row limit, read-only enforcement |
| Business glossary | ✅ Tamamı | `business_glossary_terms` table + `ai_glossary.go` CRUD handler |

---

## 3. Checklist Bazında Durum (Bölüm 10)

### 3.1 Architecture Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | AI query akışı ham SQL yerine LogicalQuery üretiyor mu? | ✅ | `service.go` → LLM generates LogicalQuery JSON |
| 2 | Semantic model DB schema'dan ayrılmış mı? | ✅ | `semantic_models` vs `tables` ayrı tablolar |
| 3 | Model, dimension, metric, relationship ayrı kavramlar mı? | ✅ | Ayrı struct'lar ve DB tabloları |
| 4 | LLM sadece retrieved context ile sınırlandırılmış mı? | ✅ | Prompt budget enforcement + denied field stripping |
| 5 | Prompt'ta tüm schema değil ilgili subset mi var? | ✅ | `table_router.go` → only relevant tables/columns |
| 6 | Query generator dialect bağımsız logical layer'dan mı çalışıyor? | ✅ | `compiler.go` + `Dialect` interface |
| 7 | SQL execution öncesi AST/parser validation var mı? | ✅ | `ReadOnlyChecker` + parameterized queries |
| 8 | Query yalnızca SELECT/read-only mi? | ✅ | Keyword blacklist + comment stripping |
| 9 | Row limit / timeout / cost guardrail var mı? | ✅ | `BI_QUERY_TIMEOUT_SECONDS`, `BI_QUERY_MAX_ROWS` |
| 10 | Generated SQL, context, confidence audit olarak saklanıyor mu? | ✅ | `query_history` + `ai_query_history` + `ai_query_telemetry` (migration 012) |

### 3.2 Semantic Layer Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | Her modelin business description'ı var mı? | ✅ | `description` field on `SemanticModel` |
| 2 | Her dimension/metric açıklamalı mı? | ✅ | Column descriptions + AI-generated descriptions |
| 3 | Metric definitions reusable ve tek kaynak mı? | ✅ | `semantic_metrics` tablosu, publish workflow ile locked |
| 4 | Join path'ler explicit relationship olarak tanımlı mı? | ✅ | `semantic_joins` — from_table, from_column, to_table, to_column, join_type, relationship |
| 5 | Ambiguous alanlar için synonym/glossary var mı? | ✅ | Synonyms on models, dimensions, metrics + `business_glossary_terms` table |
| 6 | Enum mapping var mı? | ⚠️ | Açık enum mapping yok. Description'larda dokümente edilebilir ama `status = 4 => refunded` gibi otomatik mapping mekanizması yok. **B1'e bakın.** |
| 7 | PII/sensitive kolonlar hide edilebiliyor mu? | ✅ | `WithDeniedFields` — column-level access control |
| 8 | Row-level / column-level access tasarlanmış mı? | ✅ | Backend tamamı. `CompileWithPermissions()` row-level, `PermissionManager` column-level |
| 9 | Versioning var mı? | ✅ | `publish.go` — draft/publish/rollback, version increment |
| 10 | Calculated fields var mı? | ✅ | `calculated_expression` on Dimension, bracket `[token]` resolution |

### 3.3 Retrieval Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | Hybrid retrieval (lexical + vector + graph)? | ✅ | `table_router.go` — keyword + embedding + FK graph expansion |
| 2 | Similar approved query examples retrieve ediliyor mu? | ✅ | `ListSuccessfulAIQueries()` + curated few-shot examples |
| 3 | Retrieved context skorlanıyor mu? | ✅ | `routeConfidence()` + hybrid scoring (keyword + embedding) |
| 4 | Context pack/rerank ediliyor mu? | ✅ | `filterTablesBySchemaCluster()` + budget enforcement |
| 5 | Irrelevant tablolar prompt'tan çıkarılıyor mu? | ✅ | Only top-scored tables make it to prompt |
| 6 | User permission retrieval'da uygulanıyor mu? | ✅ | `WithDeniedFields` strips fields before prompt + validation |

### 3.4 Prompt/Output Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | JSON schema uyumlu çıktı isteniyor mu? | ✅ | `LogicalQuerySchema` constant + structured parsing |
| 2 | Unknown field/metric invent etmesi yasak mı? | ✅ | System rules template + validation against semantic model |
| 3 | Join sadece relationships üzerinden mi seçiliyor? | ✅ | `semantic_joins` + BFS join resolution |
| 4 | Metric sadece semantic definition'dan mı geliyor? | ✅ | Whitelist validation against model metrics |
| 5 | Ambiguous durumda clarification dönebiliyor mu? | ✅ | `tryGenerateClarification()` + `NeedsClarification` flag |
| 6 | Explanation ve confidence üretiyor mu? | ✅ | `computeConfidence()` + token usage + prompt template trace |
| 7 | Prompt injection'a karşı user/system ayrımı net mi? | ✅ | System prompt + user question ayrı bölümler |

### 3.5 Validation Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | LogicalQuery schema validation | ✅ | `validator.go` — full structural validation |
| 2 | Model/field/metric whitelist validation | ✅ | Against published semantic model |
| 3 | Relationship validation | ✅ | Join path validation + fanout detection |
| 4 | SQL parser validation | ✅ | `ReadOnlyChecker` — dangerous keyword detection |
| 5 | Read-only enforcement | ✅ | Keyword blacklist + parameterized queries |
| 6 | LIMIT zorunlu mu? | ✅ | Default limit applied if missing |
| 7 | Timeout/cost guardrail | ✅ | Context timeout + `BI_QUERY_TIMEOUT_SECONDS` |
| 8 | Access control validation | ✅ | `PermissionManager` + `CompileWithPermissions` |
| 9 | Result sanity validation | ✅ | Anomaly detection (IQR-based) in `enrich.go` |
| 10 | Golden eval regression test | ✅ | `EvalRepository.GenerateRegressionReport()` + SSE streaming eval + benchmark suite |

---

## 4. Backend Go İçin Uygulanacak Maddeler

Bu bölüm Wren dokümanından Biqly Go backend'ine doğrudan uygulanacak somut teknik maddelerdir.

### 4.1 Enum / Value Mapping Mekanizması (B1 — Orta Öncelik)

**Wren referansı (Bölüm 6.1):** "value profiles / enum mappings"

**Mevcut durum:** Dimension'ların `description` alanında enum değerleri dokümente edilebiliyor ama LLM'in `status = 4` → `refunded` gibi dönüşümleri yapması tamamen prompt'a kalıyor. Yapısal bir mapping mekanizması yok.

**Uygulanacak:**

- [ ] `enum_mappings` tablosu oluştur (migration):

  ```sql
  CREATE TABLE enum_mappings (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      dimension_id UUID NOT NULL REFERENCES semantic_dimensions(id) ON DELETE CASCADE,
      raw_value TEXT NOT NULL,
      label TEXT NOT NULL,
      description TEXT,
      sort_order INT DEFAULT 0,
    UNIQUE(dimension_id, raw_value)
  );
  ```

- [ ] `Dimension` struct'ına `EnumValues []EnumMapping` field'ı ekle (`pkg/semantic/`)
- [ ] `semantic/repository.go`'da enum mapping CRUD metotları ekle
- [ ] `prompt.go`'da dimension render edilirken enum mapping'leri prompt'a inject et:

  ```text
  status (TEXT) — Order status. Values: 1=pending, 2=processing, 3=shipped, 4=refunded
  ```

- [ ] `table_router.go`'da enum label'ları synonym olarak kullan
- [ ] Frontend'de dimension edit modal'ında enum value editor UI ekle

**Dosyalar:** `migrations/`, `pkg/semantic/model.go`, `internal/semantic/repository.go`, `internal/ai/prompt.go`, `internal/ai/table_router.go`

### 4.2 Metric Expression Security — Controlled AST (B6 — Orta Öncelik)

**Wren referansı (Bölüm 8):** "SQL injection / unsafe SQL → LogicalQuery schema + SQL AST validation"

**Mevcut durum:** `calculated_expression` serbest string olarak kabul ediliyor. `ValidateContext()` içinde DML/DDL keyword rejection var ama AST-level parse yok. Zararlı expression teorik olarak geçebilir.

**Uygulanacak:**

- [ ] `internal/query/expression_parser.go` oluştur — basit expression AST parser:
  - İzin verilen token tipleri: identifier, number, string, arithmetic operator, parens, function call
  - Yasaklı: subquery, semicolon, comment, DML/DDL keyword
- [ ] `ValidateContext()` içinde `calculated_expression` alanlarını AST parser'dan geçir
- [ ] AST parser testleri: `expression_parser_test.go`
  - Geçerli: `[total_amount] - [discount]`, `COALESCE([amount], 0)`
  - Geçersiz: `1; DROP TABLE`, `(SELECT * FROM users)`, `exec xp_cmdshell`

**Dosyalar:** `internal/query/expression_parser.go` (yeni), `internal/semantic/publish.go` (güncelle)

### 4.3 Golden Test Case External Loader (B3 — Orta Öncelik)

**Wren referansı (Bölüm 9, Faz 5):** "Golden eval dataset oluştur"

**Mevcut durum:** `DefaultGoldenCases()` ile 5 hardcoded case + `BenchmarkCases()` mevcut. `testdata/` dizini var ama dosyalardan yükleme yok.

**Uygulanacak:**

- [ ] `internal/ai/golden_loader.go` oluştur:

  ```go
  func LoadGoldenCasesFromDir(dir string) ([]GoldenCase, error)
  ```

  - `testdata/golden/*.json` dosyalarından case'leri yükle
  - Her dosya: `{id, question, model, expected: LogicalQuery}`
- [ ] `evalModesFromRequest()`'da `suite=file:golden` parametresi destekle
- [ ] `testdata/golden/` dizini oluştur, örnek JSON case dosyaları ekle
- [ ] Golden case CRUD API ekle:
  - `GET /api/ai/eval/cases` — mevcut golden case'leri listele
  - `POST /api/ai/eval/cases` — yeni golden case ekle
  - `DELETE /api/ai/eval/cases/{id}` — golden case sil
- [ ] CI'da eval runner: `Makefile`'a `make eval` target ekle

**Dosyalar:** `internal/ai/golden_loader.go` (yeni), `internal/http/handlers/ai_eval.go` (güncelle), `testdata/golden/` (yeni dizin)

### 4.4 LLM Response Cache (B7 — Düşük Öncelik)

**Wren referansı (Bölüm 6.3):** "Context confidence" + Wren'in semantic caching yaklaşımı

**Mevcut durum:** Aynı soru tekrar sorulduğunda LLM'e tekrar gidiliyor. Fingerprint mekanizması `query_history`'de var ama cache lookup yok.

**Uygulanacak:**

- [ ] `internal/ai/response_cache.go` oluştur:

  ```go
  type ResponseCache interface {
      Get(ctx context.Context, fingerprint string) (*AIResponse, error)
      Put(ctx context.Context, fingerprint string, resp *AIResponse, ttl time.Duration) error
  }
  ```

- [ ] Redis-backed implementasyon: question hash → AIResponse cache
  - TTL: configurable (`BI_AI_RESPONSE_CACHE_TTL`, default 1h)
  - Cache key: SHA-256(question + model_id + denied_fields_hash)
  - Sadece high-confidence (>= 0.85) response'ları cache'le
- [ ] `ProcessQuestion()` içinde cache lookup ekle (LLM call öncesi)
- [ ] Cache invalidation: model publish edildiğinde ilgili cache'leri temizle

**Dosyalar:** `internal/ai/response_cache.go` (yeni), `internal/ai/service.go` (güncelle)

### 4.5 Audit Event DB Persistence (B2 — Düşük Öncelik)

**Wren referansı (Bölüm 5.5):** "generated SQL, used context ve confidence audit olarak saklanıyor mu?"

**Mevcut durum:** `internal/audit/audit.go` sadece `slog` ile logluyor. `query_history` ve `ai_query_history` tabloları de-facto audit store olarak çalışıyor ama generic audit event table yok.

**Uygulanacak:**

- [ ] `audit_events` tablosu oluştur (migration):

  ```sql
  CREATE TABLE audit_events (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      user_id UUID,
      event_type TEXT NOT NULL,
      datasource_id UUID,
      model_id UUID,
      details JSONB,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE INDEX idx_audit_events_type ON audit_events(event_type);
  CREATE INDEX idx_audit_events_user ON audit_events(user_id);
  CREATE INDEX idx_audit_events_created ON audit_events(created_at DESC);
  ```

- [ ] `audit.Logger`'a DB writer ekle (async channel-based batch write)
- [ ] `audit.go`'da `Log()` metodu hem slog hem DB'ye yazsın

**Dosyalar:** `migrations/` (yeni), `internal/audit/audit.go` (güncelle), `internal/audit/db_writer.go` (yeni)

### 4.6 Streaming Query Results (B8 — Düşük Öncelik)

**Wren referansı (Bölüm 5.6):** "bounded execution → result metadata → chart suggestion"

**Mevcut durum:** Executor tüm satırları okuduktan sonra dönüyor. Büyük sonuç setleri için memory baskısı var. Eval streaming SSE ile yapılabiliyor (`EvalRunStream`), query execution streaming yok.

**Uygulanacak:**

- [ ] `internal/query/executor.go`'da streaming variant ekle:

  ```go
  func (e *Executor) ExecuteStream(ctx context.Context, compiled CompiledQuery, fn func(rows []Row) bool) error
  ```

  - `sql.Rows` üzerinde batch iteration (100 rows per callback)
  - Callback `false` dönürse iteration'ı durdur
- [ ] HTTP handler'da SSE endpoint: `GET /api/query/run/stream`
  - Her batch'i SSE event olarak gönder
  - Client-side progressive rendering

**Dosyalar:** `internal/query/executor.go` (güncelle), `internal/http/handlers/query.go` (güncelle)

---

## 5. Frontend İçin Uygulanacak Maddeler

### 5.1 Dashboard Builder (F1 — Yüksek Öncelik)

**Wren referansı (Bölüm 2.1):** "result preview, chart, dashboard"

**Mevcut durum:** `/dashboard` route AI usage analytics gösteriyor. Kullanıcıların kaydedilmiş sorgu/chart'ları düzenleyeceği drag-drop editor yok.

**Uygulanacak:**

- [ ] Grid-based dashboard layout engine (react-grid-layout veya css grid)
- [ ] Widget tipleri: ChartWidget, TableWidget, KPIWidget, TextWidget
- [ ] Dashboard CRUD API (backend):
  - `POST /api/dashboards` — oluştur
  - `GET /api/dashboards` — listele
  - `GET /api/dashboards/{id}` — getir
  - `PUT /api/dashboards/{id}` — güncelle
  - `DELETE /api/dashboards/{id}` — sil
- [ ] Dashboard DB table (migration)
- [ ] Frontend: `DashboardBuilder.tsx` — drag-drop widget editor
- [ ] Frontend: Widget configuration panel (data source = saved query)

### 5.2 Row-Level Security Admin UI (F2 — Yüksek Öncelik)

**Wren referansı (Bölüm 4.1):** "Access rules: row-level / column-level control"

**Mevcut durum:** Backend `CompileWithPermissions()` + `BuildRowFilterPredicates()` ile row-level filter injection yapıyor. `permissions` tablosunda row_filters JSON olarak saklanıyor. Admin'de datasource access paneli var ama row-level filter tanımlama UI yok.

**Uygulanacak:**

- [ ] `admin/RowLevelSecurityPanel.tsx` oluştur:
  - Datasource/model seçimi
  - Role bazlı row filter tanımlama
  - Filter builder: `field` + `operator` + `value` satırları
  - JSON preview
- [ ] `admin/FieldPermissionPanel.tsx` oluştur:
  - Model seçimi → dimension/metric listesi
  - Role bazlı denied fields toggle
- [ ] Backend'de mevcut `permissions` tablosunu kullan

### 5.3 Business Glossary Browser (F5 — Orta Öncelik)

**Wren referansı (Bölüm 4.1):** "Instructions / business glossary"

**Mevcut durum:** Backend'de `business_glossary_terms` tablosu + CRUD API (`ai_glossary.go`) tamamen implement edildi. Frontend'de glossary browser/management sayfası yok.

**Uygulanacak:**

- [ ] `Glossary.tsx` page oluştur:
  - Datasource/model filtresi
  - Term listesi (table view)
  - Create/Edit/Delete glossary term modal
  - `maps_to_type` → dimension/metric/model link
  - Alias yönetimi
  - Search/filter
- [ ] Sidebar'a "Glossary" link ekle (AI bölümü altına)
- [ ] Mevcut API endpoint'leri: `GET/POST/PUT/DELETE /api/ai/glossary/*`

### 5.4 Kullanıcı Query History Sayfası (F4 — Orta Öncelik)

**Wren referansı (Bölüm 9):** "Feedback Loop ve Memory" — geçmiş soruların tekrar kullanılabilirliği

**Mevcut durum:** Admin panel'de `AIHistoryPanel` var ama normal kullanıcılar kendi geçmiş sorgularını göremiyor.

**Uygulanacak:**

- [ ] `QueryHistory.tsx` page oluştur:
  - Kullanıcının kendi AI sorgu geçmişi
  - Search/filter (datasource, model, tarih, status)
  - Tekrar çalıştır butonu
  - Sonuç preview
- [ ] `GET /api/ai/history?user_id=me` endpoint'i (mevcut `ListAIQueryHistory`'yi user-scoped yap)
- [ ] Sidebar'da "History" link ekle (Query bölümü altına)

---

## 6. Olması Gerekmeyenler (Biqly'nin Tasarım Kararları)

Wren dokümanında önerilen ama Biqly'nin bilerek uygulamadığı şeyler:

| # | Wren Önerisi | Biqly'nin Yaklaşımı | Neden Uygulanmamalı |
| --- | --- | --- | --- |
| W1 | Raw SQL Editor | LogicalQuery-first mimari | Biqly'nin temel tasarım kararı: AI her zaman LogicalQuery üretir, backend SQL derler. Raw SQL editor güvenlik modelini zayıflatır. |
| W2 | MDL (Modeling Definition Language) DSL | JSON-based semantic model | Wren YAML/DSL tabanlı bir modeling dili kullanıyor. Biqly REST API + JSON ile çalışıyor. Visual canvas ile model tanımlanabiliyor. |
| W3 | Apache DataFusion / Rust Core | Go-native compiler | Wren Rust-based execution engine kullanıyor. Biqly Go ile derli toplu çalışıyor, ayrı engine'a gerek yok. |
| W4 | Connector Marketplace | 4 built-in driver | Biqly PostgreSQL, MySQL, SQL Server, ClickHouse destekliyor. Plugin marketplace karmaşıklık getirir. |
| W5 | Views / Cubes / Pre-aggregations | Semantic model + calculated expressions | Wren materialized views/cubes tanımlıyor. Biqly semantic model + calculated expressions ile aynı işi yapıyor. DB-level pre-aggregation ayrı bir feature olarak değerlendirilebilir. |

---

## 7. Faz Bazlı Uygulama Planı

### Faz 1 — Semantic Contract (✅ Tamamlandı)

- ✅ `semantic_models`, `semantic_dimensions`, `semantic_metrics`, `semantic_joins` tabloları
- ✅ Publish/rollback workflow + versioning
- ✅ Column-level access (`WithDeniedFields`) + row-level (`CompileWithPermissions`)
- ✅ Budget enforcement + context snapshots

**Kalan:**

- [ ] Enum mapping mekanizması (B1)
- [ ] Metric expression AST validation (B6)

### Faz 2 — Retrieval Layer (✅ Tamamlandı)

- ✅ Hybrid retrieval (keyword + synonym + embedding + FK graph)
- ✅ Few-shot curated + dynamic retrieval
- ✅ Budget-aware context packing + route confidence scoring
- ✅ Business glossary integration (`business_glossary_terms`)

**Kalan:**

- [ ] Embedding-based learning from user confirmations (deferred)

### Faz 3 — LogicalQuery Contract (✅ Tamamlandı)

- ✅ `LogicalQuerySchema` JSON schema + structured parsing
- ✅ Validator semantic model whitelist + CTE/Window/CASE/Subquery

### Faz 4 — SQL Generation & Validation (✅ Tamamlandı)

- ✅ `compiler.go` — 1002 satır, 4 dialect, parameterized
- ✅ `ReadOnlyChecker` + default limit + timeout + EXPLAIN dry-run

### Faz 5 — Feedback + Eval (✅ Tamamlandı)

- ✅ Feedback storage + successful queries → dynamic few-shot
- ✅ Curated few-shot CRUD + regression report generation
- ✅ SSE streaming eval + benchmark suite + telemetry
- ✅ NATS-based async job queue + worker process

**Kalan:**

- [ ] Golden case external file loader + CRUD API (B3)
- [ ] CI pipeline'a eval runner ekle

### Faz 6 — Frontend Gap Kapatma (Yeni Faz — Uygulanacak)

- [ ] **Dashboard Builder** (F1) — Yüksek öncelik
- [ ] **Row-Level Security Admin UI** (F2) — Yüksek öncelik
- [ ] **Business Glossary Browser** (F5) — Orta öncelik (backend hazır)
- [ ] **Kullanıcı Query History** (F4) — Orta öncelik
- [ ] **Field-Level Permission UI** (F3) — Orta öncelik

---

## 8. Öncelik Matrisi

| Öncelik | Backend (Go) | Frontend (React) |
| --- | --- | --- |
| 🔴 Yüksek | — | Dashboard Builder (F1), RLS Admin UI (F2) |
| 🟡 Orta | Enum Mapping (B1), Metric AST (B6), Golden Loader (B3) | Glossary Browser (F5), Query History (F4), Field Permission UI (F3) |
| 🟢 Düşük | Audit DB (B2), LLM Cache (B7), Streaming Results (B8) | Export Formats (F6), Chart Customization (F7), Scheduled Queries (F8) |

---

## 9. Sonuç

Biqly, Wren.ai'nin önerdiği Text-to-SQL mimarisinin **%95'ini** implement etmiş durumda.

**Güçlü yanlar (Wren referansına göre fazladan olanlar):**

- Multi-candidate self-consistency voting (Wren'de yok)
- Locale-aware prompt templates + i18n (Wren'de yok)
- NATS async job queue + worker process (Wren'de yok)
- Anomaly detection (IQR-based) result enrichment (Wren'de yok)
- SSE streaming eval runner (Wren'de yok)
- Full auth system (Passkeys + MFA + OAuth) (Wren'de yok)
- Business glossary with mapping to semantic entities (Wren'de var ama Biqly'de daha yapısal)

**Kalan işler:**

- **6 somut backend maddesi** (B1-B8) — çoğu orta/düşük öncelik
- **5 frontend maddesi** (F1-F5) — dashboard builder ve RLS UI yüksek öncelik
- **Enum mapping** ve **metric expression security** orta vadeli teknik borç
- **Gereksiz olanlar** net sınırlanmış: raw SQL editor, MDL DSL, Rust engine, connector marketplace
