# Composite Semantic Models — Implementation Plan

> Biqly'de birden fazla domain modelini (Sales, Customer, Campaign, Inventory, Finance vb.)
> tek bir "Composite Model" altında birleştirerek, kullanıcıların "Satışları, iade oranını,
> müşteri segmentini ve kampanya etkisini birlikte göster" gibi çapraz-domain sorular
> sorabilmesini sağlayan özellik planı.

---

## Mevcut Durum Analizi

- Her `SemanticModel` tek bir `datasource_id` + tek bir `base_table` bağlı (`pkg/semantic/types.go`).
- `LogicalQuery` tek bir `model_id` referans eder (`pkg/logicalquery/types.go:17`).
- Compiler, Validator, Planner tamamı tek model üzerinden çalışır (`internal/query/compiler.go`, `validator.go`, `planner.go`).
- Table Router auto-mode'da çoklu tablo seçebilir ama sonuç tek synthetic model üretir (`internal/ai/routing/router.go`).
- DB şemasında `semantic_models` tablosu tek datasource bağlar (`migrations/003a`).

---

## Faz 1: Tip Tanımları & Veri Modeli

### 1.1 Composite Model Canonical Types (`pkg/semantic/`)

- [x] `CompositeModel` tipi tanımla — birden fazla `SemanticModel` referans eder
  - `ID`, `Name`, `Label`, `Description`
  - `DatasourceID` — tüm sub-model'ler aynı datasource'da olmalı (ilk faz)
  - `ComponentModels []ComponentModelRef` — sub-model referansları
  - `CrossModelJoins []CrossModelJoin` — model'ler arası join tanımları
  - `CanonicalDateDimension *CanonicalDateRef` — ortak tarih boyutu
  - `DimensionConflictResolutions []DimensionConflictResolution`
  - `Status`, `Version` — draft/publish workflow
  - `CreatedAt`, `UpdatedAt`

- [x] `ComponentModelRef` tipi tanımla
  - `ModelID string` — referans edilen semantic model
  - `Alias string` — composite içinde benzersiz kısa ad (ör: "sales", "cust")
  - `Role string` — "primary" | "secondary" (primary = FROM clause base)

- [x] `CrossModelJoin` tipi tanımla
  - `Name string`
  - `FromModel string` — alias reference
  - `FromDimension string` — kaynak modeldeki dimension name
  - `ToModel string` — hedef model alias
  - `ToDimension string` — hedef modeldeki dimension name
  - `JoinType string` — LEFT / INNER / RIGHT
  - `Relationship string` — many_to_one, one_to_many vb.
  - `IsActive bool`

- [x] `CanonicalDateRef` tipi tanımla
  - `ModelAlias string` — hangi component model'den
  - `DimensionName string` — hangi dimension

- [x] `DimensionConflictResolution` tipi tanımla
  - `DimensionName string` — çakışan dimension adı
  - `Resolution string` — "use_primary" | "rename" | "merge"
  - `SourceModelAlias string` — "rename" için kaynak
  - `TargetAlias string` — "rename" sonrası ad

### 1.2 Database Migration

- [x] `037a_composite_semantic_models.up.sql` — `composite_models` tablosu oluştur
  - `id UUID PK`, `datasource_id UUID FK`, `name TEXT`, `label TEXT`, `description TEXT`
  - `canonical_date JSONB` — CanonicalDateRef
  - `status TEXT DEFAULT 'draft'`, `version INT DEFAULT 0`
  - `is_active BOOLEAN DEFAULT true`
  - `created_by TEXT`, `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`
  - `UNIQUE(datasource_id, name)`

- [x] `037a` (devam) — `composite_model_components` tablosu oluştur
  - `id UUID PK`, `composite_id UUID FK -> composite_models`
  - `model_id UUID FK -> semantic_models`, `alias TEXT NOT NULL`
  - `role TEXT NOT NULL DEFAULT 'secondary'`
  - `UNIQUE(composite_id, alias)`, `UNIQUE(composite_id, model_id)`

- [x] `037a` (devam) — `composite_cross_model_joins` tablosu oluştur
  - `id UUID PK`, `composite_id UUID FK`
  - `name TEXT`, `from_alias TEXT`, `from_dimension TEXT`
  - `to_alias TEXT`, `to_dimension TEXT`
  - `join_type TEXT DEFAULT 'LEFT'`, `relationship TEXT DEFAULT 'many_to_one'`
  - `is_active BOOLEAN DEFAULT true`
  - `UNIQUE(composite_id, name)`

- [x] `037a` (devam) — `composite_dimension_resolutions` tablosu oluştur
  - `id UUID PK`, `composite_id UUID FK`
  - `dimension_name TEXT`, `resolution TEXT`
  - `source_alias TEXT`, `target_alias TEXT`

- [x] `037a` (devam) — `composite_context_snapshots` tablosu oluştur (publish workflow)
  - `composite_id UUID`, `version INT`, `context JSONB`, `validation_result JSONB`
  - `created_by TEXT`, `created_at TIMESTAMPTZ`

- [x] `037b_composite_semantic_models.down.sql` — rollback migration

---

## Faz 2: Model Merger & Flatten Engine

### 2.1 Composite Model Resolver (`internal/semantic/composite.go`)

- [ ] `CompositeResolver` struct oluştur — bileşen modelleri yükler, çakışmaları çözer, birleştirilmiş `SemanticModel` üretir
  - `Resolve(ctx, composite *CompositeModel) (*SemanticModel, error)` — birleştirilmiş model döndürür
  - Bu model compiler/validator/planner tarafından direkt kullanılabilir

- [ ] `mergeDimensions` — component modellerin dimension'larını birleştir
  - Duplicate isim tespiti (farklı modellerdeki aynı isimli dimension'lar)
  - `DimensionConflictResolution` uygula
  - ColumnRef'leri yeniden yaz (alias-prefixed table refs)
  - Synonyms'leri birleştir

- [ ] `mergeMetrics` — component modellerin metric'lerini birleştir
  - Duplicate isim tespiti ve çözümleme
  - Expression'ları yeniden yaz (qualified column refs)
  - `MetricDependencyGraph` oluştur — hangi metric hangi modelden geliyor

- [ ] `mergeJoins` — intra-model join'leri + cross-model join'leri birleştir
  - Her component model'in kendi join'lerini koru
  - Cross-model join'leri ekle (farklı modellerin tabloları arası)
  - BFS ile join graph'ın connected olduğunu doğrula

- [ ] `selectCanonicalDate` — ortak tarih boyutu seç
  - Eğer `CanonicalDateRef` tanımlıysa onu kullan
  - Değilse, en çok kullanılan date dimension'ı otomatik seç
  - Time grain dimension'larını canonical date'e bağla

- [ ] `rewriteColumnRefs` — tüm column_ref'leri birleştirilmiş modelin tablo namespace'ine çevir
  - `orders.total_amount` → `orders.total_amount` (aynı kalabilir)
  - Cross-model referansları düzelt

### 2.2 Metric Dependency Graph (`internal/semantic/metric_graph.go`)

- [ ] `MetricDependencyGraph` tipi tanımla
  - `Nodes map[string]MetricNode` — metric adı → node
  - `Edges []MetricEdge` — bağımlılık kenarları

- [ ] `MetricNode` tipi tanımla
  - `Name string`, `SourceModelAlias string`
  - `DependsOn []string` — diğer metric'ler (derived metric'ler için)

- [ ] `MetricEdge` tipi tanımla — metric'ler arası bağımlılık
  - `From string`, `To string`, `Type string` ("derives_from" | "shares_dimension")

- [ ] `BuildMetricGraph(composite *CompositeModel, resolved *SemanticModel) *MetricDependencyGraph`
  - Her metric'in expression'ını parse et, bağımlı olduğu dimension/metric'leri tespit et
  - Cross-model bağımlılıkları işaretle

- [ ] `DetectCircularDependencies(graph *MetricDependencyGraph) error`
  - Topological sort ile döngü tespiti

---

## Faz 3: Fanout Risk Hesaplama

### 3.1 Cross-Model Fanout Detector (`internal/query/composite_fanout.go`)

- [ ] `CompositeFanoutDetector` struct oluştur
  - Birleştirilmiş model üzerinde cross-model join path'leri analiz eder

- [ ] `AnalyzeFanoutRisk(resolved *SemanticModel, crossJoins []CrossModelJoin) FanoutReport`
  - Her cross-model join'in cardinality'sini hesapla
  - Birden fazla one-to-many join varsa "multiplication risk" uyarısı
  - Many-to-many cross-model join'leri için özel uyarı
  - Aggregation accuracy impact skoru (0-1)

- [ ] `FanoutReport` tipi tanımla
  - `RiskLevel string` — "low" | "medium" | "high" | "critical"
  - `RiskFactors []RiskFactor`
  - `SuggestedMitigations []string` — örn. "Use CTE to pre-aggregate"

- [ ] `RiskFactor` tipi tanımla
  - `JoinName string`, `Cardinality string`, `Impact string`
  - `AffectedMetrics []string`

- [ ] Planner'a entegrasyon — `internal/query/planner.go`
  - `Plan` metodunu genişlet: composite model cross-join'leri dahil et
  - Mevcut `checkFanout` metodunu `CompositeFanoutDetector` ile güçlendir

---

## Faz 4: Repository & CRUD Operations

### 4.1 Composite Repository (`internal/semantic/composite_repository.go`)

- [x] `CompositeRepository` struct oluştur
  - `CreateComposite(ctx, composite *CompositeModel) error`
  - `GetComposite(ctx, id string) (*CompositeModel, error)`
  - `ListComposites(ctx, datasourceID string) ([]CompositeModel, error)`
  - `UpdateComposite(ctx, composite *CompositeModel) error`
  - `DeleteComposite(ctx, id string) error`
  - `GetFullComposite(ctx, id string) (*CompositeModel, error)` — component modeller dahil
  - `AddComponent(ctx, compositeID string, ref ComponentModelRef) error`
  - `RemoveComponent(ctx, compositeID, modelID string) error`
  - `AddCrossModelJoin(ctx, compositeID string, join CrossModelJoin) error`
  - `UpdateCrossModelJoin(ctx, compositeID string, join CrossModelJoin) error`
  - `RemoveCrossModelJoin(ctx, compositeID, joinID string) error`
  - `SetCanonicalDate(ctx, compositeID string, ref *CanonicalDateRef) error`
  - `SetDimensionResolution(ctx, compositeID string, resolution DimensionConflictResolution) error`

### 4.2 Composite Publish Workflow (`internal/semantic/composite_publish.go`)

- [x] `ValidateComposite(ctx, composite *CompositeModel, catalog CatalogReader) PublishValidationResult`
  - Tüm component modellerin "published" olduğunu kontrol et
  - Cross-model join'lerdeki dimension'ların varlığını doğrula
  - Duplicate dimension tespiti ve çözüm kontrolü
  - Canonical date dimension'ın varlığını kontrol et
  - Join graph connectedness kontrolü
  - Fanout risk değerlendirmesi

- [x] `PublishComposite(ctx, id, publishedBy string, catalog CatalogReader) (*PublishResult, error)`
  - `ValidateComposite` çağır
  - Component modellerin mevcut published snapshot'larını kilitle (opsiyonel)
  - Composite model snapshot'ını kaydet
  - Status → "published"

- [x] `RollbackComposite(ctx, id string, targetVersion int, publishedBy string) (*PublishResult, error)`

---

## Faz 5: LogicalQuery & Compiler Genişletmesi

### 5.1 LogicalQuery Composite Support

- [x] `LogicalQuery` tipine `composite_id` alanı ekle (`pkg/logicalquery/types.go`)
  - `CompositeID string json:"composite_id,omitempty"`
  - Mevcut `model_id` ile geriye uyumlu — `composite_id` set ise merge edilmiş model kullanılır

- [x] Query pipeline güncellemesi — composite model flux:
  ```
  LogicalQuery.composite_id → CompositeResolver.Resolve() → merged SemanticModel → Compiler
  ```

### 5.2 Compiler Updates (`internal/query/compiler.go`)

- [x] `Compile` metodunu güncelle — composite model desteği
  - Eğer LogicalQuery'de `composite_id` varsa, resolver'dan merged model al
  - Cross-model join'leri FROM clause'a ekle
  - Mevcut `resolveFromClause` ve `determineJoins` metodlarını genişlet

- [x] `resolveFromClause` güncellemesi
  - Composite model'de base table = primary component model'in base table'ı
  - Cross-model join'leri JOIN clause'lara ekle

- [x] `CompileWithPermissions` güncellemesi
  - Her component model'in permission policy'sini birleştir
  - Denied field'ları merged model'de uygula

---

## Faz 6: Validator Genişletmesi

### 6.1 Composite Validator (`internal/query/composite_validator.go`)

- [x] `ValidateCompositeQuery` fonksiyonu
  - Dimension ve metric isimlerinin merged model'de benzersiz olduğunu kontrol et
  - Cross-model join path'lerin geçerliliğini doğrula
  - Fanout risk uyarıları ekle
  - Canonical date dimension kullanımını kontrol et

- [x] Mevcut `Validator.Validate` metodunu güncelle (`internal/query/validator.go`)
  - `composite_id` set ise merged model ile validasyon yap
  - Cross-model referansları çözümle

---

## Faz 7: AI Routing & Prompt Builder

### 7.1 Table Router Composite Awareness (`internal/ai/routing/router.go`)

- [x] Composite model'leri routing candidate'lerine ekle
  - `CompositeMatcher` (`internal/ai/routing/composite_router.go`) — published composite'leri soruya karşı skorla, en iyi eşleşmeyi döndür
  - Merged model çözümü service/handler katmanında yapılır (router ham metadata üzerinde çalışır)

- [x] `TableRoutingResult` güncellemesi
  - `CompositeID string` alanı ekle
  - `ComponentModels []string` — seçilen bileşen modeller

### 7.2 Prompt Builder Updates (`internal/ai/prompt/`)

- [x] Composite model prompt template'i
  - `CompositeContext` (`internal/ai/prompt/prompt.go`) + `writeCompositeContext` — component domain'leri listele
  - Cross-model join'leri açıkla
  - Canonical date dimension'ı belirt
  - Duplicate dimension resolution (renamed dimensions) bilgisini ekle

- [x] Prompt context budget güncellemesi
  - Composite context bloğu sınırlı/küçük; mevcut dinamik bütçeleme (model context window + static reserve) yeterli

- [x] `prompt_render.go` güncellemesi
  - Layout'a `{{.CompositeContext}}` placeholder eklendi; composite bağlamı Semantic Model başlığından sonra render edilir

---

## Faz 8: HTTP API Endpoints

### 8.1 Composite Model CRUD Handlers (`internal/http/handlers/`)

- [x] `POST /api/semantic/composites` — composite model oluştur
- [x] `GET /api/semantic/composites` — listele
- [x] `GET /api/semantic/composites/{id}` — detay (component modeller dahil)
- [x] `PUT /api/semantic/composites/{id}` — güncelle
- [x] `DELETE /api/semantic/composites/{id}` — sil

### 8.2 Component Management Endpoints

- [x] `POST /api/semantic/composites/{id}/components` — component model ekle
- [x] `DELETE /api/semantic/composites/{id}/components/{modelId}` — component kaldır

### 8.3 Cross-Model Join Endpoints

- [x] `POST /api/semantic/composites/{id}/cross-joins` — cross-model join ekle
- [x] `PUT /api/semantic/composites/{id}/cross-joins/{joinId}` — güncelle
- [x] `DELETE /api/semantic/composites/{id}/cross-joins/{joinId}` — sil

### 8.4 Configuration Endpoints

- [x] `PUT /api/semantic/composites/{id}/canonical-date` — canonical date dimension ayarla
- [x] `PUT /api/semantic/composites/{id}/dimension-resolutions` — dimension çakışma çözümleri

### 8.5 Lifecycle Endpoints

- [x] `POST /api/semantic/composites/{id}/validate` — composite model doğrula
- [x] `POST /api/semantic/composites/{id}/publish` — yayınla
- [x] `POST /api/semantic/composites/{id}/rollback` — geri al
- [x] `GET /api/semantic/composites/{id}/suggested-joins` — AI destekli cross-model join önerileri

---

## Faz 9: Frontend — Composite Model Editor

### 9.1 Composite Model Canvas (`frontend/src/components/modeling/`)

- [x] `CompositeModelingCanvas.tsx` — composite model görsel editörü
  - Component modelleri canvas üzerinde göster (her biri ayrı node)
  - Cross-model join'leri çizgi ile göster
  - Farklı renk paletleri ile model domain'lerini ayır

- [x] `CompositePalette.tsx` — component model seçim paneli
  - Mevcut semantic modelleri listele
  - Drag & drop ile component ekle
  - Duplicate dimension uyarıları

- [x] `CrossJoinEditor.tsx` — cross-model join editörü
  - İki component model arası join tanımla
  - Dimension picker (her iki modelden)
  - Cardinality seçimi

### 9.2 Composite Model Management Pages

- [x] Composite model listesi sayfası
- [x] Composite model detay sayfası (component'ler, join'ler, resolutions)
- [x] Canonical date dimension selector
- [x] Dimension conflict resolver UI
- [x] Publish/validation sonuçları görüntüleme

### 9.3 AI Query Panel Updates

- [x] Composite model seçimi — AI query panel'de composite model seçilebilir
- [x] Cross-model soru desteği — "satış + kampanya etkisi" gibi sorular
- [x] Routing visualization — composite model routing detaylarını göster

---

## Faz 10: Permissions & Security

### 10.1 Composite Model Permissions (`internal/security/`)

- [x] Composite model permission policy
  - Kullanıcının tüm component modellere erişimi olmalı
  - Herhangi bir component model'de denied field → composite'de de denied
  - Row-level filter'ları her component model için ayrı ayrı uygula

- [x] `CompileWithPermissions` güncellemesi
  - Her component model'in RLS filter'larını birleştir
  - Cross-model join'lerde RLS filter'ları doğru tabloya uygula

---

## Faz 11: Caching & Performance

### 11.1 Composite Model Cache

- [x] Merged model cache (Redis) — `composite:{id}:v{version}` key
  - Composite model resolve edilmiş hali cache'lenir
  - Component model publish edildiğinde cache invalidate

- [x] Composite model prefetch — sık kullanılan composite modelleri önceden resolve et

### 11.2 Query Fingerprinting Update

- [x] `internal/query/fingerprint.go` güncellemesi
  - `composite_id`'yi fingerprint'e dahil et
  - Component model version'larını fingerprint'e ekle

---

## Faz 12: Testing

### 12.1 Unit Tests

- [x] `composite_test.go` — CompositeResolver testleri
  - İki modeli birleştirme
  - Duplicate dimension resolution
  - Cross-model join resolution
  - Canonical date selection

- [x] `metric_graph_test.go` — MetricDependencyGraph testleri
  - Bağımlılık tespiti
  - Döngü tespiti
  - Cross-model bağımlılıklar

- [x] `composite_fanout_test.go` — fanout risk hesaplama testleri
  - Low risk senaryoları
  - High risk senaryoları (multiple one-to-many)
  - Critical risk (many-to-many cross-model)

### 12.2 Integration Tests

- [x] Composite model CRUD integration test (`composite_integration_test.go`, DB-gated)
- [x] Composite model → Compiler → SQL test (`golden_test.go` + `compiler_integration_test.go`)
- [x] Composite model permission test (Faz 10)
- [x] Composite model publish/rollback test (`composite_integration_test.go`, DB-gated)

### 12.3 Golden SQL Tests

- [x] Composite model golden test cases
  - Cross-model join ile basit sorgu (`composite_cross_model.sql`)
  - Birden fazla component model'den metric seçme
  - Canonical date ile time grain sorgusu
  - Fanout riskli senaryo (`composite_fanout_test.go`)

---

## Faz 13: Documentation & Configuration

### 13.1 Documentation

- [x] `docs/composite-semantic-models.md` — mimari doküman
- [x] README.md güncelle — Composite Semantic Models bölümü ekle
- [x] AGENTS.md güncelle — composite model değişiklik rehberi

### 13.2 Configuration

- [x] Composite model limit'leri (max components, max cross-joins, max merged fields)
- [x] Composite model prompt template'leri (tr + en)
- [x] Composite model routing ağırlıkları

---

## Bağımlılık Haritası (Sıralı Uygulama Önerisi)

```
Faz 1 (Types + Migration)
  ↓
Faz 2 (Resolver + Merger Engine)
  ↓
Faz 3 (Fanout Detection) ← Faz 2'ye bağımlı
  ↓
Faz 4 (Repository) ← Faz 1 + 2'ye bağımlı
  ↓
Faz 5 (LogicalQuery + Compiler) ← Faz 2 + 4'e bağımlı
  ↓
Faz 6 (Validator) ← Faz 5'e bağımlı
  ↓
Faz 7 (AI Routing + Prompt) ← Faz 5 + 6'ya bağımlı
  ↓
Faz 8 (HTTP API) ← Faz 4 + 5'e bağımlı
  ↓
Faz 9 (Frontend) ← Faz 8'e bağımlı
  ↓
Faz 10 (Permissions) ← Faz 5'e bağımlı
  ↓
Faz 11 (Caching) ← Faz 5 + 10'a bağımlı
  ↓
Faz 12 (Testing) ← tüm fazlara paralel
  ↓
Faz 13 (Documentation) ← tüm fazlara paralel
```

---

## Teknik Notlar

- **Aynı datasource kısıtlaması**: İlk fazda tüm component modeller aynı `datasource_id`'de olmalı. Farklı datasource'lar (cross-database joins) Faz 2'de değerlendirilecek.
- **Geriye uyumluluk**: Mevcut `model_id` ile çalışan tüm flowlar değişmeden kalır. `composite_id` opsiyonel bir alandır.
- **Prompt token budget**: Composite modeller daha büyük context gerektirir. Mevcut tiered budget sistemi (`prompt_context.go`) genişletilecek.
- **Fanout risk**: Cross-model join'lerde özellikle one-to-many + one-to-many kombinasyonları metric inflation'a yol açar. Bu durum kullanıcıya uyarı olarak gösterilecek.
- **Security**: Fail-closed yaklaşım korunur. Herhangi bir component model'e erişim yoksa composite sorgu reddedilir.
