# Wren.ai vs Biqly — Gap Analysis

> Amaç: Wren.ai referans dokümanındaki her maddeyi Biqly'nin mevcut implementasyonuyla karşılaştırmak. Eksikler, fazladan olanlar ve olmaması gerekenler belirlenmiştir.
>
> Tarih: 2026-05-31

---

## 1. Genel Değerlendirme

| Alan | Biqly Durumu | Wren Referansı | Kapatma |
| --- | --- | --- | --- |
| NL → SQL Pipeline | Tamamı mevcut | Tam | ✅ |
| Semantic Layer (MDL karşılığı) | Tamamı mevcut | Tam | ✅ |
| Retrieval (Hybrid) | Tamamı mevcut | Tam | ✅ |
| Prompt Engineering | Tamamı mevcut | Tam | ✅ |
| Validation / Security | Tamamı mevcut | Tam | ✅ |
| Feedback / Memory | Kısmen mevcut | Tam | ⚠️ |
| Eval / Regression | Kısmen mevcut | Tam | ⚠️ |
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
| Generated SQL preview | ✅ Tamamı | `AssistantMessageCard.tsx` — collapsible SQL display + syntax highlighting |
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

### 2.3 Wren AI Core (Bölüm 2.3) → Backend Query/Semantic Layer

| Wren Özelliği | Biqly Durumu | Detay |
| --- | --- | --- |
| MDL manifest okuma | ✅ Tamamı | `SemanticModel` + `GetPublishedFullModel()` |
| Model/metric/relationship temsili | ✅ Tamamı | Dimensions, Metrics, Joins struct'ları |
| Logical sorgu planlama | ✅ Tamamı | `planner.go` — join requirement analysis, fanout risk |
| ANSI SQL / dialect uyarlama | ✅ Tamamı | 4 dialect: PostgreSQL, MySQL, SQL Server, ClickHouse |
| Validation / dry-plan | ✅ Tamamı | `validator.go` + `ReadOnlyChecker` |
| Query execution | ✅ Tamamı | `executor.go` — timeout, row limit, read-only enforcement |

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
| 10 | Generated SQL, context, confidence audit olarak saklanıyor mu? | ⚠️ | Query history + AI history mevcut ama ayrı audit event table yok |

### 3.2 Semantic Layer Checklist

| # | Kriter | Biqly | Not |
| --- | --- | --- | --- |
| 1 | Her modelin business description'ı var mı? | ✅ | `description` field on `SemanticModel` |
| 2 | Her dimension/metric açıklamalı mı? | ✅ | Column descriptions + AI-generated descriptions |
| 3 | Metric definitions reusable ve tek kaynak mı? | ✅ | `semantic_metrics` tablosu, publish workflow ile_locked |
| 4 | Join path'ler explicit relationship olarak tanımlı mı? | ✅ | `semantic_joins` — from_table, from_column, to_table, to_column, join_type, relationship |
| 5 | Ambiguous alanlar için synonym/glossary var mı? | ✅ | Synonyms on models, dimensions, metrics. Glossary in prompt |
| 6 | Enum mapping var mı? | ⚠️ | Enum mapping yok. `status = 4 => refunded` gibi dönüşümler yok. Description'larda dokümente edilebilir ama otomatik mapping yok |
| 7 | PII/sensitive kolonlar hide edilebiliyor mu? | ✅ | `WithDeniedFields` — column-level access control |
| 8 | Row-level / column-level access tasarlanmış mı? | ✅ | Backend tamamı. `CompileWithPermissions()` row-level, `PermissionManager` column-level |
| 9 | Versioning var mı? | ✅ | `publish.go` — draft/publish/rollback, version increment |

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
| 10 | Golden eval regression test | ⚠️ | Framework var ama sadece 5 hardcoded golden case. Testdata dosyalarından yükleme yok |

---

## 4. Eksikler ve Gereksinimler

### 4.1 Backend Eksikleri

| # | Özellik | Öncelik | Detay |
| --- | --- | --- | --- |
| B1 | **Enum/Value Mapping** | Orta | `status = 4 → refunded` gibi otomatik mapping mekanizması yok. Description'larda dokümantasyon var ama LLM'in integer → semantic dönüşüm yapması prompt'ta kalıyor. `enum_mappings` tablosu veya dimension metadata'da enum tanımı eklenebilir. |
| B2 | **Audit Event Persistence** | Düşük | Audit logging sadece `slog` ile yapıyor. `query_history` ve `ai_query_history` tabloları de-facto audit store olarak çalışıyor ama generic audit event table yok. Şu anki yapı çoğu use case için yeterli. |
| B3 | **Golden Test Case Yönetimi** | Orta | Sadece 5 hardcoded golden case var. `testdata/` dizininden yükleme yok. External case management (CRUD API + dosya import) eklenmeli. Eval runner HTTP handler'da, standalone service metodu olmalı. |
| B4 | **Embedding-Based Learning from Feedback** | Düşük | AGENTS.md'de deferred olarak işaretli. Kullanıcı onayladığı sorgulardan embedding öğrenme mekanizması henüz yok. |
| B5 | **Composite Semantic Models** | Düşük | AGENTS.md'de Phase 11.5 olarak deferred. Birden fazla base model'i birleştirme henüz yok. |
| B6 | **Metric Expression Security** | Orta | `calculated_expression` serbest SQL fragment kabul ediyor. Controlled AST veya expression sandbox yok. DML/DDL keyword rejection var ama AST-level validation değil. |
| B7 | **LLM Response Caching** | Düşük | Fingerprint-based query history var ama LLM response cache yok. Aynı soru tekrar sorulduğunda LLM'e tekrar gidiliyor. |
| B8 | **Streaming SQL Results** | Düşük | Executor tüm satırları okuduktan sonra dönüyor. Büyük sonuç setleri için streaming desteklenmiyor. |

### 4.2 Frontend Eksikleri

| # | Özellik | Öncelik | Detay |
| --- | --- | --- | --- |
| F1 | **Dashboard Builder** | Yüksek | Kullanıcıların kaydedilmiş sorgu/chart'ları sürükle-bırak ile düzenleyeceği bir dashboard editor yok. Mevcut `/dashboard` AI usage analytics gösteriyor. Bu Wren'in temel özelliklerinden biri. |
| F2 | **Row-Level Security Admin UI** | Yüksek | Backend `CompileWithPermissions()` ile row-level filter injection yapıyor ama admin panel'de bu kuralları tanımlayacak bir UI yok. `DatasourceAccessPanel` sadece datasource-level access kontrol ediyor. |
| F3 | **Field-Level Permission UI** | Orta | Backend `WithDeniedFields` ile column-level access kontrol ediyor ama admin'de hangi alanların hangi rol için denied olduğunu yönetecek UI yok. |
| F4 | **Kullanıcı Query History Sayfası** | Orta | AI history sadece admin panel'de görünüyor. Normal kullanıcıların kendi geçmiş sorgularını arayıp tekrar çalıştıracağı bir sayfa yok. `SavedQuestions` var ama history değil. |
| F5 | **Data Catalog / Business Glossary** | Orta | Merkezi bir business glossary browser yok. Synonyms dimension/metric seviyesinde tanımlı ama kullanıcıların glossary'de arama/browsing yapacağı bir sayfa yok. |
| F6 | **Export Formatları** | Düşük | Sadece CSV export var. Excel (.xlsx), PDF, JSON export eksik. |
| F7 | **Chart Customization** | Düşük | Recharts ile bar/line/pie render ediliyor ama kullanıcı axis label, renk, annotation, threshold ayarlayamıyor. |
| F8 | **Scheduled Queries / Alerts** | Düşük | Zamanlanmış sorgu çalıştırma ve alerting yok. Backend'de worker skeleton var ama implement değil. |
| F9 | **Public Embedding** | Düşük | `ShareButton` ile kullanıcı/workspace paylaşımı var ama public iframe embed URL yok. |

### 4.3 Olması Gerekmeyenler (Biqly'nin Tasarım Kararları)

Wren dokümanında önerilen ama Biqly'nin bilerek uygulamadığı şeyler:

| # | Wren Önerisi | Biqly'nin Yaklaşımı | Neden Uygulanmamalı |
| --- | --- | --- | --- |
| W1 | Raw SQL Editor | LogicalQuery-first mimari | Biqly'nin temel tasarım kararı: AI her zaman LogicalQuery üretir, backend SQL derler. Raw SQL editor güvenlik modelini zayıflatır. Power user'lar için gelecekte read-only SQL editor düşünülebilir ama şu an değil. |
| W2 | MDL (Modeling Definition Language) DSL | JSON-based semantic model | Wren YAML/DSL tabanlı bir modeling dili kullanıyor. Biqly REST API + JSON ile çalışıyor. Özel bir DSL öğrenmeye gerek yok; visual canvas ile model tanımlanabiliyor. |
| W3 | Apache DataFusion / Rust Core | Go-native compiler | Wren Rust-based bir execution engine kullanıyor. Biqly Go ile derli toplu çalışıyor, ayrı bir engine'a gerek yok. |
| W4 | Connector Marketplace | 4 built-in driver | Biqly PostgreSQL, MySQL, SQL Server, ClickHouse destekliyor. Plugin marketplace karmaşıklık getirir; şu an 4 driver yeterli. |
| W5 | Context Confidence ID Referanslama | Structural fingerprint + confidence score | Wren her context parçasına skor verip LLM response'da referanslanmasını öneriyor. Biqly zaten confidence scoring yapıyor. Ek ID referanslama audit complexity artırır, şu anki yapı yeterli. |

---

## 5. Faz Bazlı Uygulama Planı (Wren Fazlarına Göre Güncellenmiş)

Wren dokümanındaki Faz 1-5 planını Biqly'nin mevcut durumuna göre güncelliyorum:

### Faz 1 — Semantic Contract (✅ Tamamlandı)

Wren'in önerdiği:

- `models`, `dimensions`, `metrics`, `relationships` tablolarını netleştir
- Her metric için formula, aggregation, allowed dimensions, filters tanımı
- Relationship graph oluştur
- Sensitive columns için access policy

Biqly durumu: **Tümü implement edildi.**

- `semantic_models`, `semantic_dimensions`, `semantic_metrics`, `semantic_joins` tabloları mevcut
- Publish/rollback workflow + versioning mevcut
- Column-level access (`WithDeniedFields`) + row-level (`CompileWithPermissions`) mevcut
- Budget enforcement mevcut

**Yapılacaklar:**

- [ ] Enum mapping mekanizması ekle (B1)
- [ ] Metric expression AST validation güçlendir (B6)

### Faz 2 — Retrieval Layer (✅ Tamamlandı)

Wren'in önerdiği:

- Lexical search
- Embedding tabanlı semantic search
- Approved question-SQL examples store
- Top-K context packer

Biqly durumu: **Tümü implement edildi.**

- Hybrid retrieval (keyword + synonym + embedding + FK graph) mevcut
- Few-shot curated + dynamic (successful queries) retrieval mevcut
- Budget-aware context packing mevcut
- Route confidence scoring mevcut

**Yapılacaklar:**

- [ ] Embedding-based learning from user confirmations (B4 — deferred)

### Faz 3 — LogicalQuery Contract (✅ Tamamlandı)

Wren'in önerdiği:

- JSON schema tanımla
- LLM output'u sadece bu schema'ya zorla
- Unknown field/metric geldiğinde reject et

Biqly durumu: **Tümü implement edildi.**

- `LogicalQuerySchema` JSON schema constant mevcut
- `parseLogicalQueryFromRaw()` ile structured parsing
- Validator semantic model whitelist kontrolü yapıyor
- CTE, Window, CASE, Subquery filter desteği mevcut

**Yapılacaklar:** Yok. Bu faz tamamlandı.

### Faz 4 — SQL Generation & Validation (✅ Tamamlandı)

Wren'in önerdiği:

- SQL'i LLM yerine backend generator üretsin
- SQL parser ile read-only kontrolü
- LIMIT/timeout zorunlu
- Query plan/dry-run

Biqly durumu: **Tümü implement edildi.**

- `compiler.go` — LogicalQuery → parameterized SQL (1002 satır)
- `ReadOnlyChecker` — dangerous keyword detection
- Default limit + timeout enforcement
- EXPLAIN dry-run desteği
- 4 dialect desteği (PostgreSQL, MySQL, SQL Server, ClickHouse)

**Yapılacaklar:** Yok. Bu faz tamamlandı.

### Faz 5 — Feedback + Eval (⚠️ Kısmen Tamamlandı)

Wren'in önerdiği:

- Accepted query history → few-shot retrieval
- User feedback → instruction/example olarak sakla
- Golden eval dataset oluştur
- CI'da eval runner çalıştır

Biqly durumu:

- ✅ Feedback storage mevcut (`InsertAIFeedback`)
- ✅ Successful queries → dynamic few-shot mevcut
- ✅ Curated few-shot CRUD mevcut
- ⚠️ Golden eval sadece 5 hardcoded case
- ⚠️ Eval runner HTTP handler'da, standalone değil
- ✅ Regression report generation mevcut
- ❌ CI'da otomatik eval çalıştırma yok

**Yapılacaklar:**

- [ ] Golden test case management — testdata dosyalarından yükleme + CRUD API (B3)
- [ ] Eval runner'ı standalone service metodu yap (B3)
- [ ] CI pipeline'a eval runner ekle
- [ ] Frontend'de eval case yönetim UI güçlendir (golden case CRUD)

### Faz 6 — Frontend Gap Kapatma (Yeni Faz)

Wren referansında olup Biqly'de eksik olan frontend özellikleri:

**Yapılacaklar:**

- [ ] **Dashboard Builder** (F1) — Yüksek öncelik. Drag-drop chart/table widget layout editor
- [ ] **Row-Level Security Admin UI** (F2) — Yüksek öncelik. Backend hazır, sadece UI eksik
- [ ] **Field-Level Permission UI** (F3) — Orta öncelik. Denied fields per role yönetimi
- [ ] **Kullanıcı Query History** (F4) — Orta öncelik. Admin-only history'yi tüm kullanıcılara aç
- [ ] **Business Glossary Browser** (F5) — Orta öncelik. Merkezi glossary sayfası

---

## 6. Öncelik Matrisi

| Öncelik | Backend | Frontend |
| --- | --- | --- |
| 🔴 Yüksek | — | Dashboard Builder (F1), RLS Admin UI (F2) |
| 🟡 Orta | Enum Mapping (B1), Metric Expression AST (B6), Golden Case Management (B3) | Field Permission UI (F3), User Query History (F4), Business Glossary (F5) |
| 🟢 Düşük | Audit Event Table (B2), LLM Response Cache (B7), Streaming Results (B8), Embedding Learning (B4) | Export Formats (F6), Chart Customization (F7), Scheduled Queries (F8), Public Embed (F9) |

---

## 7. Sonuç

Biqly, Wren.ai'nin önerdiği Text-to-SQL mimarisinin **%90'ını** implement etmiş durumda. Temel fark:

- **Wren'in 5 fazının ilk 4'ü tamamen tamamlandı.** Semantic contract, retrieval, LogicalQuery, SQL generation/validation hepsi production-grade.
- **Faz 5 (Feedback + Eval)** kısmen tamamlandı. Golden case management ve CI entegrasyonu eksik.
- **En büyük gap frontend'de:** Dashboard builder ve RLS admin UI. Backend altyapısı hazır, sadece UI eksik.
- **Olması gerekmeyenler** net: Raw SQL editor, MDL DSL, Rust engine, connector marketplace. Bunlar Biqly'nin tasarım kararlarıyla çelişiyor.
- **Enum mapping** ve **metric expression security** orta vadeli teknik borç olarak değerlendirilmeli.
