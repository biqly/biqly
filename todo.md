# Biqly Backend Refactoring Todo List

> Analiz tarihi: 2026-05-31
> Kapsam: `internal/` altindaki tum Go paketleri
> Toplam kaynak dosya: ~160 (test haric)
> Tespit edilen sorun: 30+ bulgular, 4 kategori

---

## Icindekiler

1. [Kod Tekrari ve Karisiklik](#1-kod-tekrari-ve-karisiklik)
2. [Memory ve Performans](#2-memory-ve-performans)
3. [Mimari ve Tasarim](#3-mimari-ve-tasarim)
4. [Test Aciklari ve Guvenlik](#4-test-aciklari-ve-guvenlik)
5. [Uygulama Plani ve Onceliklendirme](#5-uygulama-plani-ve-onceliklendirme)

---

## 1. Kod Tekrari ve Karisiklik

### [x] 1.1 [HIGH] Auth Repository - User Scan Pattern 4x Tekrar Eden (~80 satir)

**Dosyalar:**

- `internal/auth/repository.go:136-173` (GetUserByEmail)
- `internal/auth/repository.go:175-212` (GetUserByID)
- `internal/auth/repository.go:214-258` (ListUsers)
- `internal/auth/repository.go:728-765` (GetUserByEmailOrUsername)

**Sorun:** Dort farkli metod ayni `sql.NullString`/`sql.NullTime` -> pointer donusumunu tekrar ediyor:

```go
if usernameNull.Valid { user.Username = &usernameNull.String }
if displayNameNull.Valid { user.DisplayName = &displayNameNull.String }
if avatarURLNull.Valid { user.AvatarURL = &avatarURLNull.String }
// ... her metodda ayni 8 satir
```

**Cozum:** Tek bir `scanUser(s platformdb.Scanner) (*User, error)` helper cikarilmali. `metadata/repository.go`'daki `scanTable`, `scanColumn`, `scanRelation` ornekleri zaten bu pattern'i kullaniyor.

---

### [x] 1.2 [HIGH] Auth Repository - Workspace+Role Bootstrap 2x Tekrar (~50 satir)

**Dosyalar:**

- `internal/auth/repository.go:79-133` (CreateUser)
- `internal/auth/repository.go:396-444` (CreateUserWithOAuth)

**Sorun:** Her iki metod ayni 5 adimi tekrar ediyor:

1. Personal workspace olustur (`INSERT INTO workspaces`)
2. Default `viewer` rolunu al (`SELECT id FROM roles WHERE name = 'viewer'`)
3. Global viewer rol ata (`INSERT INTO user_roles`)
4. `admin` rolunu al (`SELECT id FROM roles WHERE name = 'admin'`)
5. Workspace member ekle (`INSERT INTO workspace_members`)

**Cozum:** `bootstrapUserWorkspace(ctx, tx, userID, displayName, email)` helper'i cikarilmali.

---

### [x] 1.3 [HIGH] Token Scoring Mantigi 2 Yerde Farkli Implementasyon

**Dosyalar:**

- `internal/ai/table_router.go:1799-1839` (`weightedTokenScore`, `tokenSet`, `turkishLowerReplacer`)
- `internal/http/handlers/ai.go:806-847` (`weightedHandlerTokenScore`, `handlerTokenSet`, `handlerTurkishReplacer`)

**Sorun:** Handler versiyonu `expandToken` synonym lookup icermiyor, `strings.FieldsSeq` kullaniyor (`strings.Fields` yerine). Iki implementasyon zamanla sessizce birbirinden uzaklasacak. Bu bir BI uygulamasinda routing dogrulugunu dogrudan etkiler.

**Cozum:** Token scoring logic'i `internal/ai/` altinda tek bir yerde tanimlanmali, handler bunu import etmeli.

---

### [ ] 1.4 [MEDIUM] Uc Ayri JSON Response Helper Seti (~60 satir)

**Dosyalar:**

- `internal/http/handlers/helpers.go:26-48` (`writeJSON`, `writeError`)
- `internal/auth/handler.go:386-402` (`respondJSON`, `respondError`)
- `internal/auth/handler_rbac.go:768-793` (`writeJSON`, `writeError`)

**Sorun:** Uc farkli yerde ayni isi yapan fonksiyonlar farkli isimlerle. `respondError` 5xx hatalarda generic mesaj donuyor ama `writeError` bunu yapmiyor.

**Cozum:** `internal/http/response` paketi olusturulmali:

- `WriteJSON(w, status, data)` - nil-slice normalization
- `WriteError(w, status, message)` - basic error
- `WriteInternalError(ctx, w, status, message, err)` - log + sanitized response

---

### [ ] 1.5 [MEDIUM] Workspace Datasource Filter 2 Handler'da Ayni (~40 satir)

**Dosyalar:**

- `internal/http/handlers/datasources.go:287-328`
- `internal/http/handlers/semantic.go:227-268`

**Sorun:** Iki handler da ayni pattern'i tekrar ediyor:

1. Auth enabled mi kontrol
2. User ID context'ten al
3. Super admin mi kontrol
4. `authClient.ListUserDatasources` cagir
5. Allowed set olustur
6. Workspace datasource'lar ile intersect
7. Sonucu filtrele

**Cozum:** `resolveUserDatasourceSet(ctx, deps) (map[string]struct{}, error)` helper'i cikarilmali.

---

### [ ] 1.6 [MEDIUM] Transaction Begin/Commit/Rollback 9x Tekrar (~45 satir)

**Dosyalar:** `internal/auth/repository.go` - 9 farkli yer (lines 35, 356, 514, 768, 816, 840, 892, 920, 952)

**Sorun:** Her yerde ayni pattern:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil { return err }
defer tx.Rollback()
// ... islemler ...
return tx.Commit()
```

**Cozum:** `internal/platform/db/` altina `RunInTx(ctx, db, fn func(*sql.Tx) error) error` eklenecek. `metadata/batch_tx.go`'daki `execBatchInTx` ornek alinabilir.

---

### [ ] 1.7 [MEDIUM] Nullable SQL Helper'leri 3 Pakette Daginik (~30 satir)

**Dosyalar:**

- `internal/metadata/repository.go:888-900` (`nullableInt`, `nullableEncrypted`)
- `internal/semantic/repository.go:641-674` (`nullStringArray`, `parseStringArray`)
- `internal/auth/repository.go` - inline `sql.NullString`/`sql.NullTime` her yerde

**Sorun:** `internal/platform/db/` zaten `NullIfEmptyPtr`, `StringPtrFromNull` sunuyor ama diger paketler bunlari kullanmiyor.

**Cozum:** Tum nullable helper'ler `internal/platform/db/` altinda toplanmali. Tum repository'ler bunu kullanmali.

---

### [ ] 1.8 [MEDIUM] DI Constructor'larinda 16x Ayni Driver Registration

**Dosyalar:**

- `internal/app/dependencies.go`
- `internal/app/ai_dependencies.go`
- `internal/app/query_dependencies.go`
- `internal/app/catalog_dependencies.go`

**Sorun:** Her biri 4 driver registration iceriyor (`reg.Register(postgres.New())`, vb.). Toplam 16 ozdes satir. Ayrica 3 constructor `sql.Open` + manual pool config kullaniyor, sadece 1 tanesi `platformdb.NewPool` kullaniyor (`ConnMaxLifetime`, `ConnMaxIdleTime` farkliligi).

**Cozum:** `registerDefaultDrivers(reg *datasource.Registry)` helper'i ve `openMetadataDB(dsn string) (*sql.DB, error)` helper'i cikarilmali. Tum constructor'lar ayni pool config'ini kullanmali.

---

### [ ] 1.9 [LOW] Dialect Aggregate() ve QuoteIdent() Boilerplate

**Dosyalar:**

- `internal/dialect/postgres.go:62-64`, `mysql.go:66-68`, `sqlserver.go:66-68` (Aggregate delegation)
- `internal/dialect/postgres.go:26-28`, `mysql.go:25-27`, `sqlserver.go:26-28`, `clickhouse.go:25-27` (QuoteIdent)

**Sorun:** 4 dialect ayni `QuoteIdent()` implementasyonuna sahip. 3 dialect gereksiz `Aggregate()` override'i yapiyor.

**Cozum:** `BaseDialect` uzerinde varsayilan implementasyon birakilmali, concrete dialect'lardan gereksiz override'lar kaldirilmali.

---

### [ ] 1.10 [LOW] WebAuthn Cookie Pattern 6x Tekrar (~50 satir)

**Dosyalar:** `internal/auth/handler.go` lines 424, 459, 580, 634, 669, 709

**Sorun:** Her cookie ayni pattern: secure/non-secure toggle, HttpOnly, SameSite, Path, MaxAge.

**Cozum:**

```go
func (h *AuthHandler) setSessionCookie(w, r, name, value string, maxAge int)
func (h *AuthHandler) clearSessionCookie(w, r, name string)
```

---

### [ ] 1.11 [LOW] core/errors.go ve app/errors.go Fragmentasyonu

**Dosyalar:**

- `internal/core/errors.go` (20 satir, 2 const, 6 error var)
- `internal/app/errors.go` (3 satir, 1 const)

**Sorun:** `app/errors.go` sadece 1 sabit icin ayri bir dosya.

**Cozum:** `app/errors.go`'yu `core/errors.go` ile birlestir.

---

## 2. Memory ve Performans

### [ ] 2.1 [MEDIUM] LogicalQuery Deger Olarak Geciliyor (~200 byte, 3-5x kopya)

**Dosyalar:**

- `internal/query/compiler_nested.go:81` (`compileStatement`)
- `internal/query/compiler.go:189` (`determineJoins`)
- `internal/query/compiler.go:100` (`tablesReferencedInLogicalQuery`)

**Sorun:** `LogicalQuery` ~200+ byte (7 slice header + 1 map + string'ler). Derleme basina 3-5 kez kopyalaniyor. Slice/map icerikleri paylasiliyor ama header'lar kopyalaniyor.

**Cozum:** `lq LogicalQuery` -> `lq *LogicalQuery` olarak degistirilmeli. `EnsureGroupBySelected()` cagrisi derleme oncesinde yapildigi icin mutasyon riski yok.

---

### [ ] 2.2 [MEDIUM] Slice Pre-Allocation Eksiklikleri

| Dosya | Satir | Sorun | Cozum |
| --- | --- | --- | --- |
| `internal/datasource/query_rows.go` | 16 | `var out []T` - her query'de cagriliyor | `make([]T, 0, 64)` veya caller hint |
| `internal/ai/row_scan.go` | 13 | `var out []map[string]any` - `limit` biliniyor | `make([]map[string]any, 0, limit)` |
| `internal/ai/table_router.go` | 53 | `sortedBundleColumns` - butun kolonlar | column count ile pre-allocate |
| `internal/ai/table_router.go` | 1301 | `buildJoins` - `len(relations)` upper bound | `make([]semantic.Join, 0, len(relations))` |
| `internal/ai/table_router.go` | 1349-1359 | `connectSelectedTables` 3 slice | `len(selected)` ile pre-allocate |

**Oncelik:** `query_rows.go` ve `row_scan.go` en sicak yollar (her introspection ve sample-data cagrisi).

---

### [ ] 2.3 [MEDIUM] Map Size Hint Eksiklikleri

| Dosya | Satir | Sorun | Cozum |
| --- | --- | --- | --- |
| `internal/ai/table_router.go` | 1385 | `relationAdjacency` map | `make(map[string][]string, len(relations)*2)` |
| `internal/ai/table_router.go` | 1589 | `groupColumnsByTable` map | `make(map[string][]metadata.Column, tableCount)` |
| `internal/ai/table_router.go` | 1810 | `tokenSet` map | `len(strings.Fields(...))` ile hint |
| `internal/ai/table_router.go` | 1598 | `columnNameCounts` map | `len(columns)` ile hint |
| `internal/query/compiler.go` | 107 | `tablesReferenced` map | `len(lq.Select)+len(lq.Filters)+1` ile hint |

---

### [ ] 2.4 [MEDIUM] Dimension/Metric Map'lerde Value Copy

**Dosya:** `internal/query/compiler_nested.go:93-113`

**Sorun:**

```go
dimMap := make(map[string]semantic.Dimension)   // Dimension ~180 byte
metricMap := make(map[string]semantic.Metric)    // Metric ~150 byte
```

Her lookup ~150-180 byte struct kopyaliyor. 20-50 dimension, 10-30 metric ile ~10-20 KB gereksiz kopya.

**Cozum:** Uzun vadeli: `map[string]*semantic.Dimension` olarak degistir. Kisa vadeli: Etki sinirli, compiler basina tek seferlik.

---

### [ ] 2.5 [LOW] JSON Fingerprint icin Gereksiz Marshal

**Dosya:** `internal/ai/service.go:542-565`

**Sorun:** Multi-candidate modda her candidate icin `json.Marshal` ile fingerprint olusturuluyor. Ara byte slice allocation'i var.

**Cozum:** JSON ordering garantisi icin `json.Marshal` makul. Ama `strings.Builder` + deterministic field sirasi ile daha hafif olabilir. Dusuk oncelik.

---

### [ ] 2.6 [LOW] activePromptStore Senkronizasyon Eksikligi

**Dosya:** `internal/ai/prompt_store.go:58`

**Sorun:** Package-level `var activePromptStore` yazma/okuma arasinda mutex yok. Practicede startup'ta set ediliyor ama race condition riski var.

**Cozum:** `sync.Once` veya `atomic.Value` kullanilmali.

---

### [ ] 2.7 [LOW] Multi-Candidate Goroutine'ler Context Cancel'a Duyarsiz

**Dosya:** `internal/ai/service.go:451-484`

**Sorun:** Eger parent context cancel edilirse, N goroutine LLM API call'larina devam ediyor. Token/credit israfi.

**Cozum:** Her goroutine icinde `select { case <-ctx.Done(): return ... case result := <-ch: ... }` pattern'i eklenmeli.

---

### 2.8 [GOOD] Dogru Kullanilan Patternler

- `sync.Pool` kullanimi: `prompt.go:53-68` (bytes.Buffer pool), `prompt_render.go:18-20` (render buffer pool)
- `io.LimitReader` ile response body sinirlandirma: `http_transport.go:11-19` (10 MB limit)
- `defer rows.Close()` her yerde dogru kullanim
- Context timeout'lar executor ve retry helper'larda mevcut
- String concatenation yerine `bytes.Buffer`/`strings.Builder` prompt ve compiler'da

---

## 3. Mimari ve Tasarim

### [ ] 3.1 [HIGH] internal/ai/ Mega-Paket (51 dosya, ~14,000+ satir)

**Sorun:** Tek pakette 8 farkli sorumluluk:

| Sorumluluk | Dosya Sayisi | Satir (yaklasik) |
| --- | --- | --- |
| LLM Provider abstraction | 7 | ~500 |
| NL-to-query orchestration | 1 | ~688 |
| Prompt construction | 7 | ~1,900 |
| Table routing | 9 | ~3,200 |
| Embeddings | 3 | ~650 |
| Evaluation framework | 8 | ~1,400 |
| SFT/export & translation | 2 | ~680 |
| Metadata description | 3 | ~900 |

**Cozum (kademeli):**

1. **Faz 1:** `internal/ai/routing/` - table router + scoring + schema partition (~3,200 satir)
2. **Faz 2:** `internal/ai/eval/` - evaluation framework (~1,400 satir)
3. **Faz 3:** `internal/ai/prompt/` - prompt builder + templates + render (~1,900 satir)
4. **Faz 4:** `internal/ai/provider/` - LLM provider abstraction (~500 satir)

---

### [ ] 3.2 [HIGH] table_router.go God File (1,945 satir)

**Dosya:** `internal/ai/table_router.go`

**Sorun:** Tek dosyada 45+ fonksiyon, en buyukleri:

- `Route()`: 159 satir - metadata loading, translation, embedding, selection, model building
- `appendQuestionEntityTables()`: 87 satir
- `expandSelectedWithJoinBridges()`: 70 satir
- `buildDimensions()`: 60 satir

**Cozum:** Routing paketine ayirildiginda:

- `routing/router.go` - ana Route() orkestrasyonu
- `routing/scorer.go` - token scoring, weighted scoring
- `routing/selector.go` - tablo secim mantigi
- `routing/model_builder.go` - otomatik semantic model olusturma
- `routing/entity_resolver.go` - FK graph traversal, entity resolution
- `routing/join_builder.go` - join path discovery

---

### [ ] 3.3 [HIGH] PromptBuilder.Build() 11 Parametre

**Dosya:** `internal/ai/prompt.go:103`

**Sorun:**

```go
func (b *PromptBuilder) Build(ctx context.Context, question string, model *semantic.SemanticModel,
    maxPromptRunes int, locale i18n.Locale, targetDialect string,
    examples []FewShotExample, samples []TableSample, priorTurns []ConversationTurn,
    deniedFields []string, glossary []GlossaryEntry) string {
```

**Cozum:** Functional options veya `PromptConfig` struct:

```go
type PromptConfig struct {
    MaxRunes      int
    Locale        i18n.Locale
    Dialect       string
    Examples      []FewShotExample
    Samples       []TableSample
    PriorTurns    []ConversationTurn
    DeniedFields  []string
    Glossary      []GlossaryEntry
}

func (b *PromptBuilder) Build(ctx context.Context, question string, model *semantic.SemanticModel, cfg PromptConfig) string
```

---

### [ ] 3.4 [HIGH] buildFilterPart() 167 Satirlik Tek Method

**Dosya:** `internal/query/compiler.go:739`

**Sorun:** Tek switch-case ile 14 filter operator handle ediliyor. Her case parameterized SQL parcasi olusturuyor.

**Cozum:** Map-based dispatch veya `filterBuilder` interface:

```go
type filterHandler func(c *Compiler, lhsSQL string, f query.Filter, args *[]any) (string, error)

var filterHandlers = map[string]filterHandler{
    "eq":    (*Compiler).buildEqFilter,
    "neq":   (*Compiler).buildNeqFilter,
    "gt":    (*Compiler).buildGtFilter,
    // ...
}
```

---

### [ ] 3.5 [HIGH] Dependencies Struct - 23 Alanli God Container

**Dosya:** `internal/app/dependencies.go:34-68`

**Sorun:** Her handler butun `*Dependencies`'i aliyor ama cogu sadece 2-3 alan kullaniyor. Bu testing'i zorlastiriyor ve gereksiz coupling yaratyor.

**Cozum (kademeli):**

1. Kisa vadeli: Handler'larin ihtiyacina gore kucuk interface'ler tanimla (ISP)
2. Orta vadeli: `AIDeps`, `QueryDeps`, `AuthDeps`, `CatalogDeps` gibi narrow dependency struct'lari
3. Uzun vadeli: Wire/Dig gibi DI framework ile constructor injection

---

### [ ] 3.6 [HIGH] Response Struct - 24 Alan, 7 Farkli Kaygi

**Dosya:** `internal/ai/schema.go:19-55`

**Sorun:** Tek struct'ta karisan kaygilar:

- Query output (LogicalQuery, SQL, Args, Result)
- Routing metadata (TableRouting)
- Clarification UI (Clarification, ClarificationQuestion, ClarificationOptions)
- Multi-candidate (Candidates, CandidatesCount)
- Retry/validation (RetryCount, ValidationResult)
- Observability (ModelUsed, PromptStats, TokenUsage, CostUSD, LatencyMs)
- Template traceability (PromptTemplateLocale, PromptTemplateVersions)

**Cozum:**

```go
type AIResult struct {
    Query    *query.LogicalQuery
    SQL      string
    Args     []any
    Result   *query.QueryResult
}

type AIMetadata struct {
    ModelUsed       string
    TokenUsage      TokenUsage
    CostUSD         float64
    LatencyMs       int64
    RetryCount      int
    Routing         *TableRoutingInfo
}

type AIResponse struct {
    Result   *AIResult
    Metadata *AIMetadata
    Clarification *ClarificationResponse  // optional
}
```

---

### [ ] 3.7 [MEDIUM] PoolCache Mutex Altinda driver.Open() Cagrisi

**Dosya:** `internal/datasource/pool_cache.go:44-61`

**Sorun:** `driver.Open()` mutex altinda cagriliyor. `database/sql.Open` non-blocking ama herhangi bir driver `Ping` yaparsa tum cache islemleri bloklanir.

**Cozum:** Double-checked locking veya `singleflight.Group`:

```go
func (p *PoolCache) Get(ctx context.Context, driver Driver, datasourceID, dsn string) (*sql.DB, error) {
    key := poolKey(datasourceID, dsn)
    p.mu.RLock()
    db, ok := p.pools[key]
    p.mu.RUnlock()
    if ok {
        return db, nil
    }
    
    result, err, _ := p.sf.Do(key, func() (any, error) {
        db, err := driver.Open(ctx, dsn)
        if err != nil {
            return nil, err
        }
        p.mu.Lock()
        p.pools[key] = db
        p.mu.Unlock()
        return db, nil
    })
    return result.(*sql.DB), err
}
```

---

### [ ] 3.8 [MEDIUM] internal/auth/ Paket Genisligi (40 dosya, 12 domain)

**Sorun:** Tek pakette authentication, RBAC, OAuth, MFA, WebAuthn, Workspace, Invitation, GDPR, Audit, Sharing, Datasource Access, Password Policy.

**Cozum (uzun vadeli - kademeli):**

- `internal/auth/core/` - User, Session, JWT, password hashing
- `internal/auth/rbac/` - Roles, permissions, RBACService
- `internal/auth/oauth/` - Google, GitHub, exchange
- `internal/auth/mfa/` - TOTP, recovery, WebAuthn
- `internal/auth/handlers/` - Tum HTTP handler'lar
- `internal/auth/workspace/` - Workspace management

**Not:** Bu buyuk bir refactor. Incremental yapilmali. Ilk adim: `auth/service.go`'daki (988 satir, 11 field) orchestrator'u kucult.

---

### [ ] 3.9 [MEDIUM] Dialect Interface 13 Method

**Dosya:** `internal/dialect/dialect.go`

**Sorun:** Bazi methodlar sadece sample-data path'inde kullaniliyor (`SelectWithLimit`, `DefaultOrderBy`). Interface Segregation Principle ihlali.

**Cozum:** `CoreDialect` (compiler icin) + `SampleDialect` (sample data icin) olarak ayirmak. Veya `base.go`'daki default implementasyonlara guvenmek.

---

### [ ] 3.10 [MEDIUM] internal/metadata -> internal/query Import Coupling

**Dosya:** `internal/metadata/repository.go` imports `internal/query`

**Sorun:** Metadata storage layer, query compilation output'larini biliyor. Tangential coupling.

**Cozum:** Paylasilan tipleri (history entries, fingerprints) ayri bir `internal/types/` veya `internal/query/types.go`'ya tasinmali.

---

### [ ] 3.11 [MEDIUM] Inconsistent DB Pool Creation

**Dosyalar:**

- `internal/app/dependencies.go:156` - `platformdb.NewPool` (ConnMaxLifetime, ConnMaxIdleTime set)
- `internal/app/ai_dependencies.go:28` - raw `sql.Open` (bunlar yok)
- `internal/app/query_dependencies.go:27` - raw `sql.Open`
- `internal/app/catalog_dependencies.go:28` - raw `sql.Open`

**Sorun:** 3 servis farkli pool davranisina sahip (connection recycling yok).

**Cozum:** Tum constructor'lar `platformdb.NewPool` kullanmali.

---

## 4. Test Aciklari ve Guvenlik

### [ ] 4.1 [HIGH] Tum Datasource Driver'lar Test Eksik

| Paket | Kaynak | Test | Risk |
| --- | --- | --- | --- |
| `internal/datasource/clickhouse/` | 1 | 0 | Yüksek - introspection SQL test edilmemis |
| `internal/datasource/postgres/` | 3 | 0 | Yüksek - introspection SQL test edilmemis |
| `internal/datasource/mysql/` | 1 | 0 | Yüksek - introspection SQL test edilmemis |
| `internal/datasource/sqlserver/` | 1 | 0 | Yüksek - introspection SQL test edilmemis |

**Cozum:**

1. Docker-based integration test'ler (testcontainers-go ile)
2. Golden SQL test'leri `testdata/sql/<driver>/` altinda
3. Minimal: introspection SQL'lerinin syntax dogrulugu icin unit test

---

### [ ] 4.2 [MEDIUM] Dusuk Test-to-Source Orani Olan Paketler

| Paket | Kaynak | Test | Oran | En Kritik Dosya |
| --- | --- | --- | --- | --- |
| `internal/metadata/` | 17 | 2 | 0.12 | `repository.go` (900 satir), `ai_jobs.go` (452 satir) |
| `internal/app/` | 8 | 1 | 0.13 | 4 DI constructor + adapter |
| `internal/auth/` | 40 | 13 | 0.33 | `service.go` (988 satir) |
| `internal/queue/` | 3 | 0 | 0.00 | Job queue lifecycle |

**Oncelik:** `metadata/repository.go` en kritik gap. 900 satirlik repository sadece 2 test dosyasina sahip.

---

### [ ] 4.3 [LOW] Test Eksik Paketler

| Paket | Risk | Aciklama |
| --- | --- | --- |
| `internal/audit/` | Dusuk | Simple logger |
| `internal/emailaddr/` | Dusuk | Email validation |
| `internal/errmsg/` | Dusuk | Error message templates |
| `internal/platform/db/` | Orta | Pool, scanner, null helpers |
| `internal/i18n/` | Dusuk | Locale utilities |

---

## 5. Uygulama Plani ve Onceliklendirme

### Faz 1: Hizli Kazanimlar (1-2 gun)

| # | Gorev | Dosya | Etki | Efor |
| --- | --- | --- | --- | --- |
| [x] 1.1 | `scanUser` helper cikar | `auth/repository.go` | 75 satir azalma | 30 dk |
| [x] 1.2 | `bootstrapUserWorkspace` helper cikar | `auth/repository.go` | 50 satir azalma | 30 dk |
| [ ] 1.3 | `resolveUserDatasourceSet` helper cikar | `http/handlers/` | 40 satir azalma | 45 dk |
| [ ] 1.4 | `RunInTx` helper ekle | `platform/db/` | 45 satir azalma | 30 dk |
| [ ] 1.5 | Nullable helper'leri `platform/db/`'de birlestir | `platform/db/` | 30 satir azalma | 1 saat |
| [x] 1.6 | Token scoring duplication'i kaldir | `handlers/ai.go` | ~40 satir, dogruluk artisi | 1 saat |
| [ ] 1.7 | Slice pre-allocation (query_rows, row_scan) | `datasource/`, `ai/` | Memory allocation azalmasi | 30 dk |
| [ ] 1.8 | Map size hints ekle | `ai/table_router.go` | Rehash azalmasi | 30 dk |
| [ ] 1.9 | `registerDefaultDrivers` + `openMetadataDB` helper | `app/*_dependencies.go` | 60+ satir azalma | 45 dk |
| [ ] 1.10 | Pool config standardizasyonu | `app/*_dependencies.go` | Tutarli pool davranisi | 30 dk |
| [ ] 1.11 | `activePromptStore` senkronizasyonu | `ai/prompt_store.go` | Race condition onleme | 15 dk |

### Faz 2: Orta Vadeli Refactoring (1 hafta)

| # | Gorev | Dosya | Etki | Efor |
| --- | --- | --- | --- | --- |
| [ ] 2.1 | Shared HTTP response helpers | `http/response/` | 60 satir, tutarli API | 3 saat |
| [ ] 2.2 | `PromptBuilder.Build()` -> PromptConfig struct | `ai/prompt.go` | 11 param -> 4 | 2 saat |
| [ ] 2.3 | `LogicalQuery` -> pointer gecisi | `query/compiler*.go` | 3-5x copy azalmasi | 2 saat |
| [ ] 2.4 | `buildFilterPart()` map-based dispatch | `query/compiler.go` | 167 satirlik method parcalanmasi | 3 saat |
| [ ] 2.5 | `Response` struct parcalama | `ai/schema.go` | 7 kaygi ayrimi | 3 saat |
| [ ] 2.6 | PoolCache singleflight | `datasource/pool_cache.go` | Lock contention azalmasi | 2 saat |
| [ ] 2.7 | WebAuthn cookie helper | `auth/handler.go` | 35 satir azalma | 1 saat |
| [ ] 2.8 | Dialect boilerplate temizligi | `dialect/*.go` | 24 satir azalma | 1 saat |
| [ ] 2.9 | Multi-candidate context cancellation | `ai/service.go` | API credit tasarrufu | 1 saat |
| [ ] 2.10 | `metadata -> query` coupling azaltma | `metadata/repository.go` | Modularity | 2 saat |

### Faz 3: Paket Yeniden Yapilandirma (2-3 hafta)

| # | Gorev | Mevcut | Hedef | Efor |
| --- | --- | --- | --- | --- |
| [ ] 3.1 | Table routing'i ayri pakete tasi | `ai/table_router.go` (1,945 satir) | `ai/routing/` (5-6 dosya) | 2 gun |
| [ ] 3.2 | Evaluation framework'u ayri pakete tasi | `ai/eval_*.go` (8 dosya) | `ai/eval/` | 1 gun |
| [ ] 3.3 | Prompt subsystem'i ayri pakete tasi | `ai/prompt_*.go` (7 dosya) | `ai/prompt/` | 1 gun |
| [ ] 3.4 | Provider abstraction ayri pakete tasi | `ai/*provider*.go` (7 dosya) | `ai/provider/` | 1 gun |
| [ ] 3.5 | `Dependencies` struct'i narrow deps'e parcala | `app/dependencies.go` (23 alan) | Kucuk interface'ler | 3 gun |

### Faz 4: Uzun Vadeli (1 ay+)

| # | Gorev | Aciklama |
| --- | --- | --- |
| [ ] 4.1 | Auth paketini alt paketlere ayir | core, rbac, oauth, mfa, handlers, workspace |
| [ ] 4.2 | Datasource driver integration test'leri ekle | testcontainers-go ile Docker-based |
| [ ] 4.3 | Metadata repository test coverage artir | 900 satirlik repository icin table-driven tests |
| [ ] 4.4 | DI framework entegrasyonu | Wire/Dig ile constructor injection |
| [ ] 4.5 | Observability standartizasyonu | Structured logging, metrics, tracing tutarliligi |

---

## Ek: Biqly icin Go Best Practice Tasarim Onerileri

### 1. Query Engine Pattern'i

Bir BI query engine'de derleme ve execution pipeline'inin birbirinden bagimsiz olmasi gerekir:

- **Compiler** hicbir I/O yapmamali, sadece `LogicalQuery + SemanticModel -> SQL` donusumu
- **Executor** sadece SQL calistirmali, timeout ve row limit uygulama
- **Planner** join path ve fanout analizi yapmali, compiler'dan once cagrilmali
- Bu katmanlar arasi iletisim interface'ler ile olmali, concrete implementation ile degil

### 2. Provider Abstraction Pattern'i

LLM provider'lari icin `baseProvider` + `providerHooks` pattern'i zaten iyi tasarlanmis. Bu pattern devam etmeli:

- Her yeni provider (Gemini, DeepSeek, vb.) sadece marshaling + auth hook'larini implement etmeli
- Retry, timeout, rate limiting `baseProvider`'da merkezi olmali

### 3. Semantic Layer Immutability

`SemanticModel` publish edildikten sonra immutable olmali:

- Publish aninda snapshot alinmali (mevcut `semantic_context_snapshots` tablosu dogru yolda)
- Compiler publish edilmis model'in uzerinde calismali
- Draft degisiklikler query'leri etkilememeli

### 4. Connection Pool Yonetimi

Multi-datasource BI uygulamalarinda connection pool kritik:

- Her datasource icin ayrı pool (mevcut `PoolCache` dogru)
- Pool size limitasyonu per-datasource olmali (buyuk sirketlerde 100+ datasource olabilir)
- Idle connection cleanup periyodik olmali
- `singleflight` ile ayni datasource icin duplicate pool acilmasi onlenmeli

### 5. Error Handling Stratejisi

BI uygulamalarinda error hierarchy onemli:

- Domain errors: `ErrInvalidFilter`, `ErrFieldNotFound`, `ErrPermissionDenied`
- Infrastructure errors: `ErrDBConnection`, `ErrLLMTimeout`, `ErrPoolExhausted`
- Her katman kendi error'larini wrap etmeli: `fmt.Errorf("compile select: %w", err)`
- Handler katmaninda `ServiceError` ile HTTP status mapping yapilmali (mevcut pattern iyi)
- AI error'lari ozel: `ErrNeedsClarification` user-facing, `ErrInvalidLogicalQuery` internal

### 6. Cache Stratejisi

- **Prompt cache**: Ayni soru + ayni semantic model -> cached LogicalQuery (fingerprint ile)
- **SQL cache**: Ayni CompiledQuery -> cached result (kisa TTL, veri degistikge invalidate)
- **Embedding cache**: Table/column embedding'leri metadata sync sonrasi guncellenmeli
- **Model cache**: Published semantic model'ler memory'de tutulmali, DB round-trip onlenmeli

### 7. Observability

BI query engine'de her katmanin observability'si kritik:

- **Compile time**: Kac ms surdu, kac join, kac filter
- **Execution time**: DB round-trip, row count, bytes read
- **AI generation time**: LLM latency, token usage, retry count
- **Table routing**: Hangi tablolar secildi, score'lar, neden digerleri elendi
- Her metric `slog.With` ile context carrier olmali (request_id, datasource_id, model_id)

---

## Statistikler

| Metrik | Deger |
| --- | --- |
| Toplam kaynak dosya (test haric) | ~160 |
| Toplam tespit edilen bulgu | 30+ |
| HIGH severity | 12 |
| MEDIUM severity | 13 |
| LOW severity | 9 |
| Tahmini toplam gereksiz satir | ~600+ |
| Faz 1 ile cozulecek satir | ~400 |
| Faz 1 tahmini efor | 1-2 gun |
| Faz 2 tahmini efor | 1 hafta |
| Faz 3 tahmini efor | 2-3 hafta |
