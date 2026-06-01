# Wren.ai vs Biqly — Gap Analysis

> Amaç: Wren.ai referans dokümanındaki her maddeyi Biqly'nin mevcut implementasyonuyla karşılaştırmak.
> Eksikler, fazladan olanlar, olmaması gerekenler ve Go backend için uygulanacak somut maddeler belirlenmiştir.
>
> Tarih: 2026-05-31 (v3 — mevcut kod tabanı analizi ile güncellendi)

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
| Async Job Queue | Tamamı mevcut | Tam | ✅ |
| Auth / RBAC / Workspaces | Tamamı mevcut | Kısmen | ✅ |
| i18n / Locale Detection | Tamamı mevcut | Yok | ✅ |
| Prompt Template Management | Tamamı mevcut | Yok | ✅ |
| AI Job System | Tamamı mevcut | Yok | ✅ |
| Entity Translations | Tamamı mevcut | Yok | ✅ |
| SFT Export Pipeline | Tamamı mevcut | Yok | ✅ |
| Dashboard Builder | Yok | Var | ❌ |
| Raw SQL Editor | Yok (tasarım kararı) | Var | — |
| Row-Level Security UI | Backend var, Frontend yok | Var | ❌ |
| AI Provider DB Yönetimi | Yok (env-based) | Kısmen | ❌ |

---

## 2. Wren Bölüm Bazinda Karşılaştırma

### 2.1 Wren UI (Bölüm 2.1) → Frontend

| Wren Özelliği | Biqly Durumu | Detay |
| --- | --- | --- |
| Datasource bağlama | ✅ Tamamı | `Datasources.tsx` — PostgreSQL, MySQL, SQL Server, ClickHouse; structured connection fields (host/port/username/password separately) |
| Model/ilişki tanımlama | ✅ Tamamı | `Modeling.tsx` — visual canvas, join lines, dimension/metric palette, base swap, suggested joins API |
| NL soru sorma | ✅ Tamamı | `AIQuery.tsx` — chat UI, multi-turn conversation, filter session carry-forward |
| Generated SQL preview | ✅ Tamamı | `AssistantMessageCard.tsx` — collapsible SQL + syntax highlighting |
| Result preview / chart | ✅ Tamamı | Recharts bar/line/pie, pivot table, anomaly highlighting, context menu |
| Saved question / favorite | ✅ Tamamı | `SavedQuestions.tsx` — CRUD, favorites (`is_favorite`), few-shot toggle, detail pane |
| Feedback ekranları | ✅ Tamamı | `FeedbackSection.tsx` — thumbs up/down, categories, free text |
| Prompt template editor | ✅ Tamamı | `PromptTemplates.tsx` — locale-specific prompt section editor with versioning |
| Time grain editor | ✅ Tamamı | `TimeGrains.tsx` — customizable time grain synonyms (TR/EN) |
| AI job tracker | ✅ Tamamı | `AIJobTracker.tsx` — background AI job status tracking |
| Metadata AI descriptions | ✅ Tamamı | `MetadataDescribeModal.tsx`, `MetadataBulkDescribeModal.tsx` — single + batch AI-generated descriptions |
| Eval dashboard | ✅ Tamamı | `EvalRunTab.tsx`, `EvalHistoryTab.tsx`, `EvalRegressionTab.tsx` |
| Admin panel | ✅ Tamamı | `Admin.tsx` — user management, roles (`RolesPanel`), workspaces (`WorkspacesPanel`), datasource access (`DatasourceAccessPanel`), audit log (`AuditLogPanel`), queue status |
| Auth pages | ✅ Tamamı | Sign-in/up, OAuth, passkey, MFA, password reset, email verification, invitation claiming |
| Settings | ✅ Tamamı | MFA enrollment, passkey management, workspace selector |
| Dashboard builder (drag-drop) | ❌ Eksik | Mevcut `/dashboard` AI usage analytics gösteriyor. Kullanıcıların kendi dashboard oluşturabileceği bir builder yok. |
| Public embed / sharing | ⚠️ Kısmen | `ShareButton` + `SharedResourcesList` ile workspace paylaşımı var ama public iframe embed yok |

### 2.2 Wren AI Service (Bölüm 2.2) → Backend AI Layer

| Wren Özelliği | Biqly Durumu | Detay |
| --- | --- | --- |
| Intent analysis | ✅ Tamamı | `filter_session.go` — `ClassifyFollowUpIntent()` (NewQuery/Refine/ReplaceFilters) + `ApplyFilterSession()` conversational filter carry-forward |
| Semantic context retrieval | ✅ Tamamı | `internal/ai/routing/` — keyword + synonym + embedding + FK graph + entity resolver + schema partitioning |
| Few-shot retrieval | ✅ Tamamı | Locale-aware few-shot (`few_shot_examples.locale`), name/description/is_few_shot fields, curated + dynamic retrieval |
| Prompt building | ✅ Tamamı | `internal/ai/prompt/` — Go template-based rendering, budget-aware, locale-aware (TR/EN), denied field filtering, glossary injection, versioned templates |
| LLM call | ✅ Tamamı | `internal/ai/provider/` — OpenAI-compatible + Anthropic, multi-candidate voting, `Provider` interface with `GenerateAt()` |
| Logical query parsing | ✅ Tamamı | `internal/ai/jsonextract/` — robust JSON extraction with brace-depth tracking, markdown fence stripping, reasoning preamble handling |
| SQL generation | ✅ Tamamı | `compiler.go` (816 satır) + `compiler_case.go` + `compiler_filter.go` + `compiler_nested.go` — CASE WHEN, CTE, subquery, IN subquery, cross-schema |
| Validation | ✅ Tamamı | `validator.go` — field/metric/window/CTE/HAVING/CASE validation |
| Execution preview | ✅ Tamamı | EXPLAIN dry-run, SQL preview, read-only check |
| Feedback storage | ✅ Tamamı | `InsertAIFeedback()` — rating, categories, text |
| Async job processing | ✅ Tamamı | `ai_jobs` table — query/preview/run/describe/describe_batch/embed_metadata jobs, progress tracking, cancel, stale detection, admin endpoints |
| Locale detection | ✅ Tamamı | `internal/ai/lingua/` — Turkish character profile detection, embedding model locale tagging |
| SFT export | ✅ Tamamı | `sft_export.go` — Gemma/Unsloth fine-tuning dataset export (train/val/test split, chat template rendering) |
| Eval with memory execution | ✅ Tamamı | `internal/ai/eval/eval_memory.go` — in-memory dataset execution without real DB, `eval_resultset.go` — semantic result comparison |
| Prompt template management | ✅ Tamamı | `ai_prompt_templates` table — versioned, locale-specific, editable prompt sections; restore/reseed API |

### 2.3 Wren AI Core (Bölüm 2.3) → Backend Query/Semantic Layer

| Wren Özelliği | Biqly Durumu | Detay |
| --- | --- | --- |
| MDL manifest okuma | ✅ Tamamı | `SemanticModel` + `GetPublishedFullModel()` |
| Model/metric/relationship temsili | ✅ Tamamı | Dimensions, Metrics, Joins struct'ları; cross-schema joins (`from_schema`/`to_schema`); per-dimension time grain |
| Logical sorgu planlama | ✅ Tamamı | `planner.go` — join requirement analysis, fanout risk |
| ANSI SQL / dialect uyarlama | ✅ Tamamı | 4 dialect: PostgreSQL, MySQL, SQL Server, ClickHouse; case-sensitive collation support |
| CASE WHEN compilation | ✅ Tamamı | `compiler_case.go` — branch-based CASE expression with predicate compilation |
| CTE / nested query support | ✅ Tamamı | `compiler_nested.go` — WITH ... AS, FROM subquery, FROM CTE, IN subquery filters |
| Cross-schema compilation | ✅ Tamamı | `physical_ref.go` — 2-part/3-part column ref resolution, `TableSchemas` overrides |
| Calendar grain filter intelligence | ✅ Tamamı | `calendar_grain_filter.go` — year coverage validation, DATE_TRUNC vs EXTRACT selection, date anchor parsing |
| Grain name repair | ✅ Tamamı | `grain_name_repair.go` — fixes LLM mistakes with calendar grain dimension names across all LogicalQuery fields |
| Validation / dry-plan | ✅ Tamamı | `validator.go` + `ReadOnlyChecker` |
| Query execution | ✅ Tamamı | `executor.go` — timeout, row limit, read-only enforcement |
| Business glossary | ✅ Tamamı | `business_glossary_terms` table + CRUD API (`/api/ai/glossary/*`) + prompt injection |
| Entity translations | ✅ Tamamı | `entity_translations` table — per-entity, per-locale, per-field translation overlay with fallback chain (requested → en → raw) |
| Calculated expression DML guard | ✅ Tamamı | `calc_expr_dml_test.go` — false-positive safe DML detection (e.g., `users.delete_flag` ≠ `DELETE`) |

---

## 3. Checklist Bazinda Durum (Bölüm 10)

### 3.1 Architecture Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | AI query akışı ham SQL yerine LogicalQuery üretiyor mu? | ✅ | `service.go` → LLM generates LogicalQuery JSON |
| 2 | Semantic model DB schema'dan ayrılmış mı? | ✅ | `semantic_models` vs `tables` ayrı tablolar |
| 3 | Model, dimension, metric, relationship ayrı kavramlar mı? | ✅ | Ayrı struct'lar ve DB tabloları |
| 4 | LLM sadece retrieved context ile sınırlandırılmış mı? | ✅ | Prompt budget enforcement + denied field stripping |
| 5 | Prompt'ta tüm schema değil ilgili subset mi var? | ✅ | `internal/ai/routing/` → only relevant tables/columns |
| 6 | Query generator dialect bağımsız logical layer'dan mı çalışıyor? | ✅ | `compiler.go` + `Dialect` interface |
| 7 | SQL execution öncesi AST/parser validation var mı? | ✅ | `ReadOnlyChecker` + parameterized queries |
| 8 | Query yalnızca SELECT/read-only mi? | ✅ | Keyword blacklist + comment stripping |
| 9 | Row limit / timeout / cost guardrail var mı? | ✅ | `BI_QUERY_TIMEOUT_SECONDS`, `BI_QUERY_MAX_ROWS` |
| 10 | Generated SQL, context, confidence audit olarak saklanıyor mu? | ✅ | `query_history` + `ai_query_history` + `ai_query_telemetry` (migration 012) + `v_ai_metrics_daily` materialized view |
| 11 | AI job lifecycle tracking var mı? | ✅ | `ai_jobs` table — status, phase, progress, cancel, stale detection |
| 12 | Locale-aware prompt rendering var mı? | ✅ | Go template-based (TR/EN), `ai_prompt_templates` versioned |
| 13 | Microservice-ready routing architecture var mı? | ✅ | Sub-routers (`catalog_router`, `query_router`, `ai_router`) + internal API routes |

### 3.2 Semantic Layer Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | Her modelin business description'ı var mı? | ✅ | `description` field on `SemanticModel` |
| 2 | Her dimension/metric açıklamalı mı? | ✅ | Column descriptions + AI-generated descriptions (single + batch) |
| 3 | Metric definitions reusable ve tek kaynak mı? | ✅ | `semantic_metrics` tablosu, publish workflow ile locked |
| 4 | Join path'ler explicit relationship olarak tanımlı mı? | ✅ | `semantic_joins` — from_table, from_column, to_table, to_column, join_type, relationship + cross-schema (`from_schema`/`to_schema`) |
| 5 | Ambiguous alanlar için synonym/glossary var mı? | ✅ | Synonyms on models, dimensions, metrics + `business_glossary_terms` table + entity translations |
| 6 | Enum mapping var mı? | ⚠️ | Açık enum mapping yok. Description'larda dokümente edilebilir ama `status = 4 => refunded` gibi otomatik mapping mekanizması yok. **B1'e bakın.** |
| 7 | PII/sensitive kolonlar hide edilebiliyor mu? | ✅ | `WithDeniedFields` — column-level access control |
| 8 | Row-level / column-level access tasarlanmış mı? | ✅ | Backend tamamı. `CompileWithPermissions()` row-level, `PermissionManager` column-level + RBAC with scope types |
| 9 | Versioning var mı? | ✅ | `publish.go` — draft/publish/rollback, version increment |
| 10 | Calculated fields var mı? | ✅ | `calculated_expression` on Dimension, bracket `[token]` resolution, DML guard with false-positive handling |
| 11 | Per-dimension time grain var mı? | ✅ | `semantic_dimensions.time_grain` (migration 018) + customizable grain synonyms (`ai_time_grains` table) |

### 3.3 Retrieval Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | Hybrid retrieval (lexical + vector + graph)? | ✅ | `internal/ai/routing/` — keyword + embedding + FK graph expansion + entity resolver |
| 2 | Similar approved query examples retrieve ediliyor mu? | ✅ | Locale-aware few-shot + curated examples + dynamic retrieval |
| 3 | Retrieved context skorlanıyor mu? | ✅ | Configurable weighted scoring (`routing_weights_default.json`) + route confidence |
| 4 | Context pack/rerank ediliyor mu? | ✅ | Schema partitioning + budget pruning (`routing_budget.go`) + count-question detection |
| 5 | Irrelevant tablolar prompt'tan çıkarılıyor mu? | ✅ | Only top-scored tables make it to prompt |
| 6 | User permission retrieval'da uygulanıyor mu? | ✅ | `WithDeniedFields` strips fields before prompt + validation |
| 7 | Locale-aware retrieval var mı? | ✅ | `lingua/` — Turkish character detection, embedding model locale tagging, locale-preferred few-shot selection |

### 3.4 Prompt/Output Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | JSON schema uyumlu çıktı isteniyor mu? | ✅ | `LogicalQuerySchema` constant + structured parsing via `jsonextract/` |
| 2 | Unknown field/metric invent etmesi yasak mı? | ✅ | System rules template + validation against semantic model |
| 3 | Join sadece relationships üzerinden mi seçiliyor? | ✅ | `semantic_joins` + BFS join resolution |
| 4 | Metric sadece semantic definition'dan mı geliyor? | ✅ | Whitelist validation against model metrics |
| 5 | Ambiguous durumda clarification dönebiliyor mu? | ✅ | `tryGenerateClarification()` + `NeedsClarification` flag |
| 6 | Explanation ve confidence üretiyor mu? | ✅ | `computeConfidence()` + token usage + prompt template trace |
| 7 | Prompt injection'a karşı user/system ayrımı net mi? | ✅ | System prompt + user question ayrı bölümler |
| 8 | Prompt template'ler runtime'da değiştirilebiliyor mu? | ✅ | `ai_prompt_templates` table — versioned, locale-specific, editable without redeploy |

### 3.5 Validation Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | LogicalQuery schema validation | ✅ | `validator.go` — full structural validation including CASE, CTE, subquery |
| 2 | Model/field/metric whitelist validation | ✅ | Against published semantic model |
| 3 | Relationship validation | ✅ | Join path validation + fanout detection |
| 4 | SQL parser validation | ✅ | `ReadOnlyChecker` — dangerous keyword detection |
| 5 | Read-only enforcement | ✅ | Keyword blacklist + parameterized queries |
| 6 | LIMIT zorunlu mu? | ✅ | Default limit applied if missing |
| 7 | Timeout/cost guardrail | ✅ | Context timeout + `BI_QUERY_TIMEOUT_SECONDS` |
| 8 | Access control validation | ✅ | `PermissionManager` + `CompileWithPermissions` + RBAC scope types |
| 9 | Result sanity validation | ✅ | Anomaly detection (IQR-based) in `enrich.go` + `enrich_viz.go` visualization hints |
| 10 | Golden eval regression test | ✅ | `eval/eval_repository.go` + SSE streaming eval + memory-based execution + benchmark suite + LLM-based judge |
| 11 | Calendar grain year coverage validation | ✅ | `calendar_grain_filter.go` — rejects ambiguous month/quarter without year |
| 12 | Calculated expression DML guard | ✅ | False-positive safe detection (`users.delete_flag` ≠ `DELETE`), string literal/comment aware |

---

## 4. Backend Go İçin Uygulanacak Maddeler

Bu bölüm Wren dokümanından Biqly Go backend'ine doğrudan uygulanacak somut teknik maddelerdir.

### 4.1 Enum / Value Mapping Mekanizması (B1 — Orta Öncelik)

**Wren referansı (Bölüm 6.1):** "value profiles / enum mappings"

**Mevcut durum:** Dimension'ların `description` alanında enum değerleri dokümente edilebiliyor ama LLM'in `status = 4` → `refunded` gibi dönüşümleri yapması tamamen prompt'a kalıyor. Yapısal bir mapping mekanizması yok.

**Uygulanacak:**

- [x] `enum_mappings` tablosu oluştur (migration):

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

- [x] `Dimension` struct'ına `EnumValues []EnumMapping` field'ı ekle (`internal/semantic/`)
- [x] `semantic/repository.go`'da enum mapping CRUD metotları ekle
- [x] `internal/ai/prompt/`'da dimension render edilirken enum mapping'leri prompt'a inject et:

  ```text
  status (TEXT) — Order status. Values: 1=pending, 2=processing, 3=shipped, 4=refunded
  ```

- [x] `internal/ai/routing/`'da enum label'ları synonym olarak kullan
- [x] Frontend'de dimension edit modal'ında enum value editor UI ekle

**Dosyalar:** `migrations/`, `internal/semantic/model.go`, `internal/semantic/repository.go`, `internal/ai/prompt/prompt.go`, `internal/ai/routing/router.go`

### 4.2 Metric Expression Security — Controlled AST (B6 — Orta Öncelik)

**Wren referansı (Bölüm 8):** "SQL injection / unsafe SQL → LogicalQuery schema + SQL AST validation"

**Mevcut durum:** `calculated_expression` serbest string olarak kabul ediliyor. DML guard mevcut (`calc_expr_dml_test.go`) ve false-positive handling var, ama AST-level parse yok. Zararlı expression teorik olarak geçebilir.

**Uygulanacak:**

- [x] `internal/query/expression_parser.go` oluştur — basit expression AST parser:
  - İzin verilen token tipleri: identifier, number, string, arithmetic operator, parens, function call
  - Yasaklı: subquery, semicolon, comment, DML/DDL keyword
- [x] `ValidateContext()` içinde `calculated_expression` alanlarını AST parser'dan geçir
- [x] AST parser testleri: `expression_parser_test.go`
  - Geçerli: `[total_amount] - [discount]`, `COALESCE([amount], 0)`
  - Geçersiz: `1; DROP TABLE`, `(SELECT * FROM users)`, `exec xp_cmdshell`

**Dosyalar:** `internal/query/expression_parser.go` (yeni), `internal/semantic/publish.go` (güncelle)

### 4.3 Golden Test Case External Loader (B3 — Orta Öncelik)

**Wren referansı (Bölüm 9, Faz 5):** "Golden eval dataset oluştur"

**Mevcut durum:** `DefaultGoldenCases()` + `BenchmarkCases()` + `golden_test.go` (803 satır, comprehensive fixture suite) + `eval/eval_memory.go` (in-memory execution) mevcut. Dosyalardan yükleme yok. SFT export pipeline mevcut (`sft_export.go`).

**Uygulanacak:**

- [ ] `internal/ai/eval/golden_loader.go` oluştur:

  ```go
  func LoadGoldenCasesFromDir(dir string) ([]GoldenCase, error)
  ```

  - `testdata/golden/*.json` dosyalarından case'leri yükle
  - Her dosya: `{id, question, model, expected: LogicalQuery}`
- [ ] Eval runner'da `suite=file:golden` parametresi destekle
- [ ] `testdata/golden/` dizini oluştur, örnek JSON case dosyaları ekle
- [ ] Golden case CRUD API ekle:
  - `GET /api/ai/eval/cases` — mevcut golden case'leri listele
  - `POST /api/ai/eval/cases` — yeni golden case ekle
  - `DELETE /api/ai/eval/cases/{id}` — golden case sil
- [ ] CI'da eval runner: `Makefile`'a `make eval` target ekle

**Dosyalar:** `internal/ai/eval/golden_loader.go` (yeni), `internal/http/handlers/ai_eval.go` (güncelle), `testdata/golden/` (yeni dizin)

### 4.4 LLM Response Cache (B7 — Düşük Öncelik)

**Wren referansı (Bölüm 6.3):** "Context confidence" + Wren'in semantic caching yaklaşımı

**Mevcut durum:** Aynı soru tekrar sorulduğunda LLM'e tekrar gidiliyor. Fingerprint mekanizması `query_history`'de var ama cache lookup yok.

**Uygulanacak:**

- [x] `internal/ai/response_cache.go` oluştur:

  ```go
  type ResponseCache interface {
      Get(ctx context.Context, fingerprint string) (*AIResponse, error)
      Put(ctx context.Context, fingerprint string, resp *AIResponse, ttl time.Duration) error
  }
  ```

- [x] Redis-backed implementasyon: question hash → AIResponse cache
  - TTL: configurable (`BI_AI_RESPONSE_CACHE_TTL`, default 1h)
  - Cache key: SHA-256(question + model_id + denied_fields_hash)
  - Sadece high-confidence (>= 0.85) response'ları cache'le
- [x] `ProcessQuestion()` içinde cache lookup ekle (LLM call öncesi)
- [x] Cache invalidation: model publish edildiğinde ilgili cache'leri temizle

**Dosyalar:** `internal/ai/response_cache.go` (yeni), `internal/ai/service.go` (güncelle)

### 4.5 Audit Event DB Persistence (B2 — Düşük Öncelik)

**Wren referansı (Bölüm 5.5):** "generated SQL, used context ve confidence audit olarak saklanıyor mu?"

**Mevcut durum:** `internal/audit/audit.go` sadece `slog` ile logluyor. Ancak auth modülünde (`internal/auth/audit.go`) DB-persisted audit log mevcut — 40+ audit event type ile kapsamlı tracking. `query_history` ve `ai_query_history` tabloları de-facto query audit store olarak çalışıyor. BI-specific generic audit event table yok.

**Uygulanacak:**

- [x] `audit_events` tablosu oluştur (migration):

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

- [x] `audit.Logger`'a DB writer ekle (async channel-based batch write)
- [x] `audit.go`'da `Log()` metodu hem slog hem DB'ye yazsın

**Dosyalar:** `migrations/` (yeni), `internal/audit/audit.go` (güncelle), `internal/audit/db_writer.go` (yeni)

### 4.6 AI Provider/Model DB Yönetimi (B9 — Yüksek Öncelik)

**Wren referansı (Bölüm 2.2):** "AI Service — retrieval, prompting, SQL generation, result validation"

**Mevcut durum:** Tüm AI konfigürasyonu `BI_AI_*` environment variable'larından okunuyor (`internal/config/config.go`). Provider seçimi, API key, base URL, model adı hepsi env'de. Yeni provider eklemek veya model değiştirmek için deploy/restart gerekiyor. Ollama + OpenAI aynı anda kullanılamıyor. `internal/ai/provider/` altinda provider interface var ama konfigürasyon hala env-based.

**Detaylı plan:** [`ai_provider_db_yonetimi_plan.md`](ai_provider_db_yonetimi_plan.md)

**Özet:** ✅ Tamamlandı (2026-05-31)

- [x] `ai_providers` + `ai_models` DB tabloları (migration `033a/033b`)
- [x] `ProviderStore` — in-memory cache + hot-reload (`internal/ai/provider_store.go` + `PurposeProvider`, restart-free)
- [x] Purpose bazlı model yönetimi: query, describe, embedding, translation, judge
- [x] API key AES encryption + masking (`security.Encryption`, `••••last4`)
- [x] Admin CRUD API: `/api/ai/providers/*`, `/api/ai/models/*` (admin-gated)
- [x] Bağlantı test endpoint'i: `POST /api/ai/providers/{id}/test`
- [x] Frontend admin panel: provider kartları + model yönetimi + purpose bazlı seçim (`AIProvidersPanel.tsx`)
- [x] Env fallback (backward compatible) + auto-seed (`BI_AI_DB_MANAGED`, varsayılan `true`)

### 4.7 Streaming Query Results (B8 — Düşük Öncelik)

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

**Mevcut durum:** `/dashboard` route AI usage analytics gösteriyor (`v_ai_metrics_daily` materialized view). Kullanıcıların kaydedilmiş sorgu/chart'ları düzenleyeceği drag-drop editor yok.

**Uygulanacak:**

- [x] Grid-based dashboard layout engine (react-grid-layout veya css grid)
- [x] Widget tipleri: ChartWidget, TableWidget, KPIWidget, TextWidget
- [x] Dashboard CRUD API (backend):
  - `POST /api/dashboards` — oluştur
  - `GET /api/dashboards` — listele
  - `GET /api/dashboards/{id}` — getir
  - `PUT /api/dashboards/{id}` — güncelle
  - `DELETE /api/dashboards/{id}` — sil
- [x] Dashboard DB table (migration)
- [x] Frontend: `DashboardBuilder.tsx` — drag-drop widget editor
- [x] Frontend: Widget configuration panel (data source = saved query)

### 5.2 Row-Level Security Admin UI (F2 — Yüksek Öncelik)

**Wren referansı (Bölüm 4.1):** "Access rules: row-level / column-level control"

**Mevcut durum:** Backend `CompileWithPermissions()` + row-level filter injection yapıyor. `permissions` tablosunda row_filters JSON olarak saklanıyor. Admin'de `RolesPanel`, `WorkspacesPanel`, `DatasourceAccessPanel` mevcut ama row-level filter tanımlama UI yok. RBAC altyapısı tam (scope types: global, workspace, datasource, model).

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

**Mevcut durum:** Backend'de `business_glossary_terms` tablosu + CRUD API (`/api/ai/glossary/*`) tamamen implement edildi. Prompt'a glossary injection mevcut (`internal/ai/prompt/glossary.go`). Frontend'de glossary browser/management sayfası yok.

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

**Mevcut durum:** Admin panel'de `AIHistoryPanel` + `AIJobTracker` mevcut. Backend'de `GET /api/ai/history` + `GET /api/ai/history/detail` endpoint'leri eklendi. Normal kullanıcılar kendi geçmiş sorgularını görebiliyor ama ayrı bir sayfa yok.

**Uygulanacak:**

- [ ] `QueryHistory.tsx` page oluştur:
  - Kullanıcının kendi AI sorgu geçmişi
  - Search/filter (datasource, model, tarih, status)
  - Tekrar çalıştır butonu
  - Sonuç preview
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

## 7. Proje İstatistikleri

### Kod Organizasyonu

| Modül | Dosya Sayısı | Açıklama |
| --- | --- | --- |
| `internal/ai/routing/` | 26 | Hybrid table routing, scoring, entity resolution, budget pruning |
| `internal/ai/prompt/` | 27 | Template-based prompt building, versioning, locale support |
| `internal/ai/provider/` | 13 | OpenAI/Anthropic provider abstraction |
| `internal/ai/eval/` | 9 | Golden eval, memory execution, LLM judge, result comparison |
| `internal/ai/lingua/` | 4 | Locale detection, embedding model tagging |
| `internal/ai/jsonextract/` | 2 | Robust JSON extraction from LLM responses |
| `internal/ai/` (top-level) | 20 | Service, validator, describe, filter session, SFT export, translation |
| `internal/query/` | 20 | Compiler (816 satır + case/filter/nested), executor, planner, enrich, golden tests (803 satır) |
| `internal/semantic/` | 7 | Model, repository, publish, budget, calc expr DML guard |
| `internal/auth/` | 67 | Full auth: JWT, OAuth, MFA, passkeys, RBAC, workspaces, audit, GDPR export |
| `internal/auth/rbac/` | 5 | Role-based access control with scope types |
| `internal/auth/mfa/` | 5 | TOTP + WebAuthn MFA |
| `internal/auth/oauth/` | 4 | GitHub + Google OAuth |
| `internal/auth/workspace/` | 4 | Workspace CRUD, membership, sharing |
| `internal/auth/handlers/` | 8 | HTTP handlers for all auth routes |
| `internal/mail/` | 13 | SMTP client, template rendering, block list |

### Veritabanı Migrasyonları

| Kategori | Sayı | Aralık |
| --- | --- | --- |
| Ana migrasyonlar | 31 | 001–031 |
| Auth migrasyonları | 32 | 001–032 |
| Mail migrasyonları | 1 | 001 |

### API Endpoint'leri

| Kategori | Sayı |
| --- | --- |
| Datasources | 7 |
| Metadata | 10 |
| Semantic Layer | 19 |
| Queries | 5 |
| AI NL-to-SQL | 6 |
| AI Jobs | 10+ |
| AI Evaluation | 5 |
| AI Examples/Feedback | 8 |
| AI Glossary | 4 |
| AI Prompt Templates | 4 |
| AI Time Grains | 2 |
| Auth | 35+ |
| Internal API | 12+ |

---

## 8. Faz Bazlı Uygulama Planı

### Faz 1 — Semantic Contract (✅ Tamamlandı)

- ✅ `semantic_models`, `semantic_dimensions`, `semantic_metrics`, `semantic_joins` tabloları
- ✅ Publish/rollback workflow + versioning
- ✅ Column-level access (`WithDeniedFields`) + row-level (`CompileWithPermissions`)
- ✅ Budget enforcement + context snapshots
- ✅ Cross-schema join support (`from_schema`/`to_schema`)
- ✅ Per-dimension time grain (`semantic_dimensions.time_grain`)
- ✅ Dimension/metric/join CRUD + delete + update endpoints

**Kalan:**

- [ ] Enum mapping mekanizması (B1)
- [x] Metric expression AST validation (B6) — DML guard mevcut ama AST parser yok

### Faz 2 — Retrieval Layer (✅ Tamamlandı)

- ✅ Hybrid retrieval (keyword + synonym + embedding + FK graph)
- ✅ Few-shot curated + dynamic retrieval + locale-aware selection
- ✅ Budget-aware context packing + route confidence scoring
- ✅ Business glossary integration (`business_glossary_terms`)
- ✅ Configurable routing weights + lexicon (`routing_weights_default.json`, `routing_lexicon_default.json`)
- ✅ Entity resolver for name-based questions
- ✅ Schema partitioning for multi-schema datasources
- ✅ Count-question detection + budget auto-adjustment

**Kalan:**

- [ ] Embedding-based learning from user confirmations (deferred)

### Faz 3 — LogicalQuery Contract (✅ Tamamlandı)

- ✅ `LogicalQuerySchema` JSON schema + structured parsing (`jsonextract/`)
- ✅ Validator semantic model whitelist + CTE/Window/CASE/Subquery
- ✅ Conversational filter carry-forward (`filter_session.go`)
- ✅ Grain name repair for LLM mistakes (`grain_name_repair.go`)

### Faz 4 — SQL Generation & Validation (✅ Tamamlandı)

- ✅ `compiler.go` (816 satır) + `compiler_case.go` + `compiler_filter.go` + `compiler_nested.go`
- ✅ 4 dialect: PostgreSQL, MySQL, SQL Server, ClickHouse
- ✅ Parameterized queries + case-sensitive collation support
- ✅ `ReadOnlyChecker` + default limit + timeout + EXPLAIN dry-run
- ✅ Cross-schema compilation (`physical_ref.go`)
- ✅ Calendar grain filter intelligence (`calendar_grain_filter.go`)
- ✅ Comprehensive golden tests (803 satır)

### Faz 5 — Feedback + Eval (✅ Tamamlandı)

- ✅ Feedback storage + successful queries → dynamic few-shot
- ✅ Curated few-shot CRUD + regression report generation
- ✅ SSE streaming eval + memory-based execution + LLM judge
- ✅ NATS-based async job queue + worker process
- ✅ AI telemetry + daily metrics materialized view
- ✅ SFT export pipeline (Gemma/Unsloth fine-tuning dataset)
- ✅ Prompt template versioning + locale support

**Kalan:**

- [ ] Golden case external file loader + CRUD API (B3)
- [ ] CI pipeline'a eval runner ekle

### Faz 6 — Auth & Admin (✅ Tamamlandı)

- ✅ Full auth system: JWT, OAuth (GitHub + Google), passkeys, MFA (TOTP + WebAuthn)
- ✅ RBAC with scope types (global, workspace, datasource, model)
- ✅ Workspaces + membership + resource sharing
- ✅ Audit logging (40+ event types, DB-persisted in auth module)
- ✅ User invitations + email verification + password policy
- ✅ Account lifecycle (freeze/delete/purge/restore)
- ✅ Admin panel: users, roles, workspaces, datasource access, audit log

### Faz 7 — Frontend Gap Kapatma (Uygulanacak)

- [ ] **Dashboard Builder** (F1) — Yüksek öncelik
- [ ] **Row-Level Security Admin UI** (F2) — Yüksek öncelik
- [ ] **AI Provider Admin UI** — Yüksek öncelik (B9 frontend)
- [ ] **Business Glossary Browser** (F5) — Orta öncelik (backend hazır)
- [ ] **Kullanıcı Query History** (F4) — Orta öncelik (backend hazır)
- [ ] **Field-Level Permission UI** (F3) — Orta öncelik

---

## 9. Öncelik Matrisi

| Öncelik | Backend (Go) | Frontend (React) |
| --- | --- | --- |
| 🔴 Yüksek | AI Provider DB (B9) | Dashboard Builder (F1), RLS Admin UI (F2), AI Provider Admin UI |
| 🟡 Orta | Enum Mapping (B1), Metric AST (B6), Golden Loader (B3) | Glossary Browser (F5), Query History (F4), Field Permission UI (F3) |
| 🟢 Düşük | Audit DB (B2), LLM Cache (B7), Streaming Results (B8) | Export Formats, Chart Customization, Scheduled Queries |

---

## 10. Sonuç

Biqly, Wren.ai'nin önerdiği Text-to-SQL mimarisinin **%95+'ini** implement etmiş durumda. Wren referansının ötesinde önemli eklemeler yapılmıştır.

**Güçlü yanlar (Wren referansına göre fazladan olanlar):**

- Multi-candidate self-consistency voting (Wren'de yok)
- Locale-aware prompt templates + i18n (TR/EN) + entity translations (Wren'de yok)
- NATS async job queue + AI job lifecycle management (Wren'de yok)
- Anomaly detection (IQR-based) result enrichment + visualization hints (Wren'de yok)
- SSE streaming eval runner + memory-based execution + LLM judge (Wren'de yok)
- Full auth system: Passkeys + MFA + OAuth + RBAC + Workspaces + GDPR export (Wren'de yok)
- Business glossary with semantic entity mapping + prompt injection (Wren'de var ama Biqly'de daha yapısal)
- Prompt template versioning + runtime editing without redeploy (Wren'de yok)
- Conversational filter carry-forward across multi-turn chat (Wren'de yok)
- SFT export pipeline for model fine-tuning (Wren'de yok)
- Cross-schema join support + calendar grain filter intelligence (Wren'de yok)
- Configurable routing weights/lexicon + count-question detection (Wren'de yok)
- Comprehensive golden test suite (803 satır) with fanout/RLS fixture coverage (Wren'de yok)
- Microservice-ready sub-router architecture (Wren'de yok)
- Entity translations with locale fallback chain (Wren'de yok)
- Customizable time grain synonyms per locale (Wren'de yok)

**Kalan işler:**

- **6 somut backend maddesi** (B1-B9) — çoğu orta/düşük öncelik, sadece B9 (AI Provider DB) yüksek
- **6 frontend maddesi** (F1-F5 + AI Provider UI) — dashboard builder ve RLS UI yüksek öncelik
- **Enum mapping** ve **metric expression security** orta vadeli teknik borç
- **Gereksiz olanlar** net sınırlanmış: raw SQL editor, MDL DSL, Rust engine, connector marketplace
