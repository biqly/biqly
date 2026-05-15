# Biqly Go Backend — Code Review & Best Practices Report

> Tarih: May 2026 | Kapsam: `internal/` (tüm Go paketleri)  
> **Uygulama durumu güncellendi:** May 2026 (refactor sprint 1–4)

---

## Uygulama Durumu (Özet)

| Durum | Açıklama |
|-------|----------|
| ✅ | Tamamlandı — kodda mevcut |
| 🔶 | Kısmen — hedefe yakın alternatif uygulandı |
| ⏳ | Bekliyor — henüz yapılmadı |

**Sprint 1:** tamamlandı (5/5)  
**Sprint 2:** tamamlandı (6/6)  
**Sprint 3:** tamamlandı (`baseProvider` / `baseEmbedder`, `processAndObserve` ✅)  
**Sprint 4:** büyük ölçüde tamamlandı (`ServiceError`, validation, batch upsert, errmsg, embedding upsert/list, `interface{}`→`any`, handler method check, dialect compile-time check ✅; model/struct örtüşmesi ⏳)

### Tamamlanan başlıca dosyalar / API’ler

| Alan | Uygulama |
|------|----------|
| Hata & HTTP | `internal/core/service_error.go` (`ToServiceError`, `*ServiceError` dönüşü), `writeInternalError`, `writeServiceError`, `MapQueryServiceError` |
| Repository | `platform/db.QuerySlice`, `metadata/batch_columns.go`, `metadata/batch_relations.go`, `batch_tx.go`, `scanTable`/`scanColumn`, `sql_columns.go` sabitleri |
| Driver / dialect | `datasource/base_driver.go`, `dialect/base.go` + `CalendarPartLookup` |
| HTTP | `decodeJSON`, `requireURLParam`, `ResolveDatasourceDB`, admin middleware |
| AI HTTP | `http_provider.go`, `base_provider.go` (`baseProvider` + `baseEmbedder` hooks), `http_transport.go`, `execHTTPPostRetry` / `execHTTPPostRetryBytes`, `readResponseBody`, `tailSlice`, `newService`, `FetchTableSample` |
| Semantic publish | `commitPublishedVersionTx` |
| Mesajlar | `internal/errmsg` (import cycle önleme) |

---

## İçindekiler

1. [Yönetici Özeti](#1-yönetici-özeti)
2. [Kritik Bulgular (Hemen Düzeltilmeli)](#2-kritik-bulgular-hemen-düzeltilmeli)
3. [Datasource Driver Katmanı — Kod Tekrarı](#3-datasource-driver-katmanı--kod-tekrarı)
4. [Dialect Katmanı — Kod Tekrarı](#4-dialect-katmanı--kod-tekrarı)
5. [Repository / Data-Access Katmanı — Kod Tekrarı](#5-repository--data-access-katmanı--kod-tekrarı)
6. [HTTP Handler Katmanı — Kod Tekrarı](#6-http-handler-katmanı--kod-tekrarı)
7. [AI Service Katmanı — Kod Tekrarı](#7-ai-service-katmanı--kod-tekrarı)
8. [Error Handling Anti-Patternleri](#8-error-handling-anti-patternleri)
9. [Performans & Güvenlik Bulguları](#9-performans--güvenlik-bulguları)
10. [Refactoring Öncelik Sıralaması](#10-refactoring-öncelik-sıralaması)

---

## 1. Yönetici Özeti

| Metrik | Değer |
|--------|-------|
| Analiz edilen dosya sayısı | ~80+ Go dosyası |
| Toplam tespit edilen bulgu | 65+ ayrı bulgu |
| **CRITICAL** (hemen fix) | 4 |
| **HIGH** (bir sonraki sprint) | 18 |
| **MEDIUM** (planlı refactor) | 28 |
| **LOW** (zaman olduğunda) | 15+ |
| Tahmini silinebilecek tekrar satırı | **~1.400+ satır** (~%20-25 azalma) |

### En Etkili 5 Refactoring

| # | Refactoring | Tahmini Etki |
|---|------------|-------------|
| 1 | Generic `QuerySlice[T]` — repository katmanı | ~270 satır azalma, 27 tekrar noktası |
| 2 | `BaseDriver` struct — datasource driver katmanı | ~385 satır azalma, 4 driver |
| 3 | `decodeJSON[T]` helper — HTTP handler katmanı | ~42 satır azalma, 21 tekrar noktası |
| 4 | `BaseDialect` embedding — dialect katmanı | ~110 satır azalma, 4 dialect |
| 5 | HTTP provider base — AI service katmanı | ~120 satır azalma |

---

## 2. Kritik Bulgular (Hemen Düzeltilmeli)

> **Sprint 1:** Bu bölümdeki maddelerin tamamı ✅ uygulandı.

### 2.1 Double Error Wrapping — `errors.Is/As` Bozuk ✅

**Dosyalar:** `internal/ai/table_router.go:381`, `internal/ai/table_router.go:1429`

```go
// YANLIŞ: Go 1.20+ sadece bir %w destekler
return nil, result, fmt.Errorf("%w: %w", ErrTableScopeInvalid, err)
```

**Düzeltme:**
```go
return nil, result, fmt.Errorf("%w: %v", ErrTableScopeInvalid, err)
// veya
return nil, result, errors.Join(ErrTableScopeInvalid, err)
```

**Etki:** `errors.Is(err, ai.ErrTableScopeInvalid)` çalışır ama içteki `err` asla erişilemez.

---

### 2.2 `panic()` Kullanımı — Library Kodunda ✅

**Dosyalar:**
- `internal/ai/routing_weights.go:53` — `panic(routingWeightsErr)`
- `internal/ai/routing_lexicon.go:38` — `panic(routingLexiconErr)`

Kod tabanında hiçbir yerde `recover()` yok. Bu fonksiyonlar çalışma zamanında tüm süreci çökertir.

**Düzeltme:** Hata döndürün:
```go
func ActiveRoutingWeights() (*RoutingWeights, error) {
    routingWeightsOnce.Do(func() {
        routingWeights, routingWeightsErr = loadRoutingWeights("")
    })
    if routingWeightsErr != nil {
        return nil, routingWeightsErr
    }
    return routingWeights, nil
}
```

---

### 2.3 Internal Error Detayları HTTP Client'a Sızdırılıyor ✅

**Dosyalar:** `internal/http/handlers/ai.go`, `datasources.go`, `ai_eval.go` (30+ nokta)

```go
// YANLIŞ: SQL detayları, dosya yolları client'a gider
writeError(w, http.StatusInternalServerError, err.Error())
writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", err.Error()))
```

**Düzeltme:** Generic mesaj + internal log:
```go
slog.ErrorContext(ctx, "compilation failed", "error", err)
writeError(w, http.StatusInternalServerError, "compilation failed")
```

---

### 2.4 Regexp Hot Path'te Derleniyor ✅

**Dosyalar:**
- `internal/security/readonly.go` — ✅ `init()` ile ön-derleme uygulandı
- `internal/ai/filter_session.go` — ✅ `init()` ile ön-derleme (`compileFollowUpPatterns`)

**Düzeltme:** `init()` veya paket seviyesinde ön-derleme:
```go
var dangerousPatterns []*regexp.Regexp

func init() {
    for _, kw := range dangerous {
        dangerousPatterns = append(dangerousPatterns,
            regexp.MustCompile(`\b`+regexp.QuoteMeta(kw)+`\b`))
    }
}
```

---

## 3. Datasource Driver Katmanı — Kod Tekrarı ✅

> `BaseDriver` + postgres compile-time check uygulandı.

### 3.1 Genel Durum

4 driver (postgres, mysql, sqlserver, clickhouse) arasında **~%85 yapısal tekrar** var. Toplam ~453 satır kodun ~385 satırı boilerplate.

### 3.2 Yapısal Tekrarlar

| Bulgu | Tekrar Sayısı | Şiddet |
|-------|--------------|--------|
| `Driver` struct + constructor (sadece dialect tipi farklı) | 4 | HIGH |
| `Ping()` (sadece driver adı farklı) | 4 | HIGH |
| `Open()` (sadece driver adı + pool limits farklı) | 4 | HIGH |
| `Dialect()` getter (karakter karakter aynı) | 4 | HIGH |
| `Type()` (sadece string sabiti farklı) | 4 | MEDIUM |
| `Introspect()` orkestrasyon (28 satır × 4, 3'ü karakter karakter aynı) | 4 | HIGH |
| `introspectSchemas()` scan lambda (karakter karakter aynı) | 4 | MEDIUM |
| `introspectColumns()` nullable int pattern | 3 | MEDIUM |
| `introspectRelations()` scan lambda | 3 | MEDIUM |

### 3.3 Önerilen Çözüm: `BaseDriver` Struct

```go
// internal/datasource/base_driver.go
type BaseDriver struct {
    typeName   string
    sqlDriver  string
    dialectVal dialect.Dialect
    poolLimits PoolLimits
}

func NewBaseDriver(typeName, sqlDriver string, d dialect.Dialect, pl PoolLimits) *BaseDriver {
    return &BaseDriver{typeName: typeName, sqlDriver: sqlDriver, dialectVal: d, poolLimits: pl}
}

func (b *BaseDriver) Type() string                { return b.typeName }
func (b *BaseDriver) Dialect() dialect.Dialect    { return b.dialectVal }
func (b *BaseDriver) Ping(ctx context.Context, dsn string) error {
    if err := Ping(ctx, b.sqlDriver, dsn); err != nil {
        return fmt.Errorf("failed to open %s connection: %w", b.typeName, err)
    }
    return nil
}
func (b *BaseDriver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
    db, err := OpenPool(ctx, b.sqlDriver, dsn, b.poolLimits)
    if err != nil {
        return nil, fmt.Errorf("failed to open %s connection: %w", b.typeName, err)
    }
    return db, nil
}
```

Her concrete driver sadece introspection SQL'lerini sağlar:

```go
// internal/datasource/postgres/driver.go
type Driver struct{ *datasource.BaseDriver }

func NewDriver() *Driver {
    return &Driver{datasource.NewBaseDriver("postgres", "pgx",
        dialect.PostgresDialect{}, datasource.DefaultPoolLimits())}
}
```

### 3.4 Eksik Compile-Time Check ✅

| Driver | `var _ datasource.Driver = (*Driver)(nil)` |
|--------|---------------------------------------------|
| mysql | Var |
| sqlserver | Var |
| clickhouse | Var |
| postgres | ✅ Eklendi |

---

## 4. Dialect Katmanı — Kod Tekrarı ✅

> `BaseDialect` embedding + `CalendarPartLookup` uygulandı.

### 4.1 Genel Durum

4 dialect dosyası toplam 353 satır. ~110 satır (~%35) delegation boilerplate.

### 4.2 Method Bazında Tekrar

| Method | Aynı Olan | Farklı Olan | Şiddet |
|--------|-----------|-------------|--------|
| `QuoteIdent()` | 4/4 | 0 | HIGH |
| `CastType()` | 4/4 | 0 | HIGH |
| `LimitOffset()` | 3/4 | SQL Server | HIGH |
| `Aggregate()` | 3/4 | ClickHouse | HIGH |
| `ExplainSQL()` | 3/4 | SQL Server (boş döner) | MEDIUM |
| `CalendarPart()` | 4/4 yapısal | format stringleri farklı | MEDIUM |
| `QuoteIdentSegment()` | 2/4 birebir | 2/4 sadece quote char farklı | MEDIUM |
| `Placeholder()` | 2/4 birebir | 2/4 prefix farklı | MEDIUM |

### 4.3 Önerilen Çözüm: `BaseDialect` Embedding

```go
// internal/dialect/base.go
type BaseDialect struct {
    PlaceholderFmt  string
    ExplainDisabled bool
}

func (b BaseDialect) CastType(sqlType string) string    { return CastTypeUpper(sqlType) }
func (b BaseDialect) LimitOffset(limit, offset int) string { return StandardLimitOffset(limit, offset) }
func (b BaseDialect) Aggregate(d Dialect, fn, column string) string { return AggregateStandardSQL(d, fn, column) }
func (b BaseDialect) ExplainSQL(sql string) string {
    if b.ExplainDisabled { return "" }
    return "EXPLAIN " + sql
}
func (b BaseDialect) Placeholder(index int) string {
    if b.PlaceholderFmt == "" { return "?" }
    return fmt.Sprintf(b.PlaceholderFmt, index)
}
```

Concrete dialect sadece 5 method tanımlar: `Name`, `QuoteIdentSegment`, `DateTrunc`, `CalendarPart`, `ILike`.

```go
// postgres.go — refactoring sonrası (~40 satır, şu an 76)
type PostgresDialect struct{ BaseDialect }

func init() { PostgresDialect{}.register() }
func (d PostgresDialect) Name() string { return "postgres" }
// ... sadece farklı olan methodlar
```

### 4.4 CalendarPart Yapısal Tekrarı ✅

Tüm 4 dialect aynı switch yapısını tekrar eder (refactor öncesi):

```go
switch strings.ToLower(strings.TrimSpace(part)) {
case "year":    return fmt.Sprintf("<dialect-specific>", q)
case "quarter": return fmt.Sprintf("<dialect-specific>", q)
case "month":   return fmt.Sprintf("<dialect-specific>", q)
default:        return d.DateTrunc(part, column)
}
```

**Çözüm:** `base.go`'da format string parametreli helper:
```go
func CalendarPartLookup(d Dialect, part, column string, yearFmt, quarterFmt, monthFmt string) string {
    q := d.QuoteIdent(column)
    switch strings.ToLower(strings.TrimSpace(part)) {
    case "year":    return fmt.Sprintf(yearFmt, q)
    case "quarter": return fmt.Sprintf(quarterFmt, q)
    case "month":   return fmt.Sprintf(monthFmt, q)
    default:        return d.DateTrunc(part, column)
    }
}
```

---

## 5. Repository / Data-Access Katmanı — Kod Tekrarı 🔶

> `QuerySlice`, scan helpers, SELECT sabitleri, batch columns/relations, `SearchColumns` FK alanları, transactional batch upsert uygulandı. Embedding upsert/list birleştirmesi ⏳.

### 5.1 Query-Loop-Close Pattern (27 Tekrar) ✅

**Dosyalar:** `metadata/repository.go`, `semantic/repository.go`, `ai/eval_repository.go`

Her SELECT method aynı yapıyı tekrar eder (~20 satır × 27):

```go
rows, err := r.db.QueryContext(ctx, query, args...)
if err != nil {
    return nil, fmt.Errorf("<operation>: %w", err)
}
defer func() { _ = rows.Close() }()
var results []Type
for rows.Next() {
    var t Type
    if err := rows.Scan(&t.Field1, &t.Field2); err != nil {
        return nil, fmt.Errorf("<operation>: %w", err)
    }
    results = append(results, t)
}
if err := rows.Err(); err != nil {
    return nil, fmt.Errorf("<operation>: %w", err)
}
return results, nil
```

**Çözüm:** Generic `QuerySlice[T]`:
```go
// internal/platform/db/query.go
func QuerySlice[T any](ctx context.Context, db *sql.DB, query string, args []any,
    scan func(Scanner) (T, error)) ([]T, error) {
    rows, err := db.QueryContext(ctx, query, args...)
    if err != nil { return nil, err }
    defer func() { _ = rows.Close() }()
    var out []T
    for rows.Next() {
        v, err := scan(rows)
        if err != nil { return nil, err }
        out = append(out, v)
    }
    return out, rows.Err()
}
```

Kullanım:
```go
datasources, err := db.QuerySlice(ctx, r.db, query, nil, func(s db.Scanner) (Datasource, error) {
    return r.scanDatasource(s)
})
```

### 5.2 Embedding Upsert/List Tekrarı

| Method | Satır | Kardeş Method | Fark |
|--------|-------|---------------|------|
| `UpsertTableEmbedding` | 235-251 | `UpsertColumnEmbedding` | 446-462 | Tablo adı |
| `ListTableEmbeddings` | 256-293 | `ListColumnEmbeddings` | 466-501 | Tablo adı + 1 alan |

**Çözüm:** Private `upsertEmbedding` helper:
```go
func (r *Repository) upsertEmbedding(ctx context.Context, tableName, entityID, modelName string, embedding []float32, opName string) error
```

### 5.3 Table/Column Scan Tekrarı

Table scan 10 alan — **3 yerde** aynı sırada tekrar eder.
Column scan 20 alan — **3 yerde** aynı sırada tekrar eder.

**Not:** `SearchColumns` 17 alan tarıyordu (3 FK alanı eksik) — ✅ düzeltildi (`scanColumn` + `columnSelectColumns` ile hizalı).

**Çözüm:** `scanTable(scanner)` ve `scanColumn(scanner)` helper metodları (mevcut `scanDatasource` pattern'ini takip ederek).

### 5.4 SELECT Sütun Listesi Tekrarı

Table SELECT sütun listesi 3 yerde, Column SELECT sütun listesi 3 yerde tekrar eder.

**Çözüm:** Sabit tanımla (mevcut `modelSelectSQL()` pattern'ini takip ederek):
```go
const tableSelectColumns = `id, datasource_id, schema_id, schema_name, table_name, table_type, row_estimate, description, created_at, updated_at`
const columnSelectColumns = `id, datasource_id, table_id, ...` // 20 sütun
```

### 5.5 Transaction Publish/Rollback Aynı SQL ✅

`semantic/repository.go` — `PublishModel` ve `RollbackModel` ortak `commitPublishedVersionTx` kullanıyor (önceki tekrar):

1. `INSERT INTO semantic_context_snapshots ...` — karakter karakter aynı
2. `UPDATE semantic_models SET status = 'published', ...` — karakter karakter aynı

**Çözüm:** `publishSnapshot` private metodu çıkarın.

### 5.6 Model/Struct Örtüşmesi

| Tip A | Tip B | Fark |
|-------|-------|------|
| `PermissionPolicyRecord` (metadata) | `CatalogPolicy` (semantic) | Yapısal olarak aynı, mapping gerektiriyor |
| `TableEmbedding` | `ColumnEmbedding` | 1 alan farkı |
| `Relation` (metadata) | `Join` (semantic) | From*/To* alanları aynı |

---

## 6. HTTP Handler Katmanı — Kod Tekrarı 🔶

> `decodeJSON`, `ResolveDatasourceDB`, admin middleware, `requireURLParam`, `writeJSON` nil-slice, history birleştirme, `ServiceError`, `processAndObserve` uygulandı.

### 6.1 JSON Decode + Error Response (21 Tekrar) ✅

```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    writeError(w, http.StatusBadRequest, "invalid request body")
    return
}
```

**Çözüm:** Generic helper:
```go
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
    var v T
    if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return nil, false
    }
    return &v, true
}
```

Kullanım: `req, ok := decodeJSON[createDatasourceRequest](w, r)`

### 6.2 Datasource + Driver + DSN Resolution (5 Tekrar)

```go
ds, err := h.deps.MetaRepo.GetDatasource(ctx, id)
driver, err := h.deps.DriverReg.Get(ds.Type)
dsn, err := security.ConnectionDSN(h.deps.Encryptor, ds.DSNEncrypted)
```

**Dosyalar:** `ai.go:212-262`, `datasources.go:134-203`

**Çözüm:** `Dependencies` üzerinde `ResolveDatasourceDB` helper:
```go
func (d *Dependencies) ResolveDatasourceDB(ctx context.Context, id string) (*metadata.Datasource, datasource.Driver, *sql.DB, error)
```

### 6.3 `requireAdminKey` Middleware Olmalı (5 Çağrı Noktası)

```go
if !h.requireAdminKey(w, r) { return }
```

**Çözüm:** Chi middleware:
```go
func AdminKeyMiddleware(adminKey string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := adminKeyFromRequest(r)
            if adminKey == "" || token == "" || token != adminKey {
                writeError(w, http.StatusUnauthorized, "invalid or missing admin API key")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### 6.4 AI Query/Preview/Run Ortak Akış (3 Tekrar) ✅

`Query` / `Preview` / `Run` → `processAndObserve(w, r, phase)`; faz sonrası `finishAIPreview` / `finishAIRun`. Run fazında datasource pool tek `ResolveDatasourceDB` + `closeResolvedDatasource`.

### 6.5 Nil-to-Empty-Slice Normalizasyonu (6 Tekrar)

```go
if results == nil {
    results = []SomeType{}
}
writeJSON(w, http.StatusOK, results)
```

**Çözüm:** `writeJSON` içinde nil slice → `[]` normalizasyonu yapın.

### 6.6 URL Param Validation Eksik (18/23 Nokta)

18 `chi.URLParam(r, "id")` çağrısında boş kontrolü yok.

**Çözüm:**
```go
func requireURLParam(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
    v := chi.URLParam(r, key)
    if v == "" {
        writeError(w, http.StatusBadRequest, key+" is required")
        return "", false
    }
    return v, true
}
```

### 6.7 `recordQueryHistory` vs `recordAIQueryHistory` — Yapısal İkiz

`history.go` — İmza, akış ve hata mesajları neredeyse aynı. Sadece log string'leri farklı.

**Çözüm:** Paket-seviyesi standalone fonksiyon çıkarın.

### 6.8 Method Check Handler İçinde (Middleware Olmalı)

`ai_eval.go:192-195` ve `ai_settings.go:97-100` — Handler içinde `r.Method` kontrolü yapıyor, bunu chi router'da `r.MethodFunc` ile çözün.

### 6.9 String-Matching Error Dispatch — Kırılgan ✅

`query.go` — `strings.Contains` dispatch kaldırıldı; `QueryService` `*ServiceError` döndürüyor, handler `writeServiceError` kullanıyor.

**Uygulanan çözüm:** Service katmanından typed error:
```go
type ServiceError struct {
    Status  int
    Message string
}
```

---

## 7. AI Service Katmanı — Kod Tekrarı 🔶

> `http_provider.go`, `base_provider.go` (`providerHooks` / `embeddingHooks`), `readResponseBody`, `execHTTPPost`/`execHTTPPostRetry`, `newService`, `FetchTableSample`, validator delegasyonu uygulandı.

### 7.1 HTTP Provider Pattern — `client.go` vs `anthropic.go` (HIGH) ✅

`client.go` ve `anthropic.go` artık ince adapter: `baseProvider` + `providerHooks` (path, headers, marshal, parse). Ortak HTTP/retry `base_provider.generateAt` içinde. Embeddings: `baseEmbedder` + `embeddingHooks` (`embedder.go`). Eski `llm_http_client.go` / `embedding_http_client.go` kaldırıldı.

### 7.2 Row Sampling — `describe.go` vs `sample.go` (HIGH)

`describe.go:199-261` ve `sample.go:26-94` — aynı identifier validation + query build + row scan mantığı.

**Çözüm:** Ortak `sampleRowsFromDB` fonksiyonu çıkarın.

### 7.3 JSON Parse + Validate Tekrarı

`service.go:505-522` ve `validator.go:21-37` — `TrimToJSONObject` + `json.Unmarshal` + empty-select check her ikisinde de var.

**Çözüm:** `parseAndValidate` sadece `SchemaValidator.Validate` çağırsın, tekrar eden parse kodunu kaldırın.

### 7.4 Service Constructor Tekrarı

`NewService` (satır 31-59) ve `NewServiceWithProvider` (satır 63-84) — gövde karakter karakter aynı.

**Çözüm:** Private `newServiceFromConfig` helper çıkarın.

### 7.5 Response Body Read + Close (3 Tekrar)

`client.go:126-133`, `anthropic.go:111-118`, `embedder.go:86-89`

**Çözüm:**
```go
func readResponseBody(resp *http.Response) ([]byte, error) {
    body, readErr := io.ReadAll(resp.Body)
    closeErr := resp.Body.Close()
    if readErr != nil { return nil, fmt.Errorf("read response: %w", readErr) }
    if closeErr != nil { return nil, fmt.Errorf("close response: %w", closeErr) }
    return body, nil
}
```

### 7.6 Tail-Slice Fonksiyonları (3 Tekrar)

`prompt_context.go:79-107` — `tailFewShot`, `tailPriorTurns`, `tailGlossary` aynı mantık.

**Çözüm:** Generic `tailSlice[T]`:
```go
func tailSlice[T any](s []T, max int) []T {
    if len(s) == 0 || max <= 0 { return nil }
    if len(s) <= max { return s }
    return append([]T(nil), s[len(s)-max:]...)
}
```

### 7.7 Truncate Utility Tekrarı

`validator.go:112-117` (byte-based, UTF-8 bozabilir) ve `describe.go:298-307` (rune-safe).

**Çözüm:** Rune-safe versiyonu tek yerde tutun, Türkçe metin için önemli.

### 7.8 Embedder Retry Logic Eksik ✅

`embedder.go` artık `execHTTPPostRetryBytes` + `execRetry[T]` kullanıyor (chat completions ile aynı 429/502-504 politikası).

---

## 8. Error Handling Anti-Patternleri

### 8.1 Custom Error Types — İyi Ama Yetersiz

Mevcut custom error tipleri:
- `ValidationError` / `ValidationErrors` — `query/result.go`
- `ErrTableScopeInvalid`, `ErrTypeScopeEmpty` — `ai/table_router.go`

**Eksik:** "unsupported operator", "readonly check failed", "permission denied" gibi durumlar `fmt.Errorf` ile dönüyor ve `errors.Is` ile yakalanamıyor.

### 8.2 Error Mesaj String Tekrarları

| Mesaj | Tekrar | Dosyalar |
|-------|--------|----------|
| `"unsupported driver: %s"` | 4 | ai.go, datasources.go |
| `"unknown dimension"` | 10 | compiler.go, validator.go, ai/validator.go |
| `"unknown metric"` | 7 | compiler.go, validator.go, ai/validator.go |
| `"unknown field"` | 11 | compiler.go, validator.go, row_injection.go, publish.go |

**Çözüm:** Paket-seviyesi sabitler tanımlayın.

### 8.3 Validation Logic Çiftlenmesi ✅

`ai/validator.go` artık `query.Validator.Validate()` delegasyonu kullanıyor (parse katmanı ayrı).

### 8.4 `interface{}` vs `any` Tutarsızlığı ✅

`ai_eval.go` artık tamamen `any` kullanıyor; `interface{}` kullanımı kalmadı.

### 8.5 Logging Tutarsızlığı

| Pattern | Sayı | Not |
|---------|------|-----|
| `slog.ErrorContext(ctx, ...)` | ~20 | Doğru |
| `slog.Error("...", ...)` (ctx yok) | 4 | helpers.go, sql_pool.go |
| `slog.Warn("...", ...)` (ctx yok) | 2 | dependencies.go |

---

## 9. Performans & Güvenlik Bulguları

### 9.1 Batch Upsert Round-Trip Overhead 🔶

`metadata/repository.go` — `UpsertSchemas`/`UpsertTables`/`UpsertColumns`/`UpsertRelations` transaction + çoklu satır INSERT (`batch_columns.go`, `batch_relations.go`). `pq.CopyFrom` ⏳.

### 9.2 SearchColumns Eksik Alanlar (Olası Bug) ✅

`SearchColumns` artık tam `columnSelectColumns` + `scanColumn` kullanıyor (FK alanları dahil).

### 9.3 Compile-Time Interface Check Tutarsızlığı ✅

`PostgresDialect`, `MySQLDialect`, `SQLServerDialect`, `ClickHouseDialect` ve tüm driver'lar `var _ Dialect = ...{}` / `var _ Driver = ...{}` ile compile-time check'e sahip.

---

## 10. Refactoring Öncelik Sıralaması

### Sprint 1 — Kritik (1-2 gün) ✅

| # | İş | Etki | Durum |
|---|----|------|-------|
| 1 | Double `%w: %w` wrapping düzelt | errors.Is/As doğruluğu | ✅ |
| 2 | `panic()` → error return | Uygulama kararlılığı | ✅ |
| 3 | `err.Error()` HTTP response'dan kaldır | Güvenlik | ✅ |
| 4 | Regex ön-derleme (`readonly.go`) | Performans | ✅ |
| 5 | Eksik compile-time check (postgres driver) | Derleme zamanı güvenlik | ✅ |

### Sprint 2 — Yüksek Etkili Refactoring (3-5 gün) ✅

| # | İş | Tahmini Satır Azalma | Durum |
|---|----|--------------------|-------|
| 6 | Generic `QuerySlice[T]` repository helper | ~270 | ✅ |
| 7 | `BaseDriver` struct ile driver boilerplate kaldır | ~385 | ✅ |
| 8 | `decodeJSON[T]` handler helper | ~42 | ✅ |
| 9 | `BaseDialect` embedding | ~110 | ✅ |
| 10 | Datasource resolution helper (`ResolveDatasourceDB`) | ~60 | ✅ |
| 11 | `requireAdminKey` → middleware | ~15 | ✅ |

### Sprint 3 — Orta Etkili Refactoring (3-5 gün) 🔶

| # | İş | Tahmini Satır Azalma | Durum |
|---|----|--------------------|-------|
| 12 | HTTP provider base (`baseProvider`) | ~120 | ✅ `http_provider.go`, `base_provider.go`, hooks |
| 13 | Row sampling birleştirme | ~60 | ✅ `FetchTableSample` |
| 14 | `recordQueryHistory` birleştirme | ~20 | ✅ `persistQueryHistory` |
| 15 | scan helper extraction (Table/Column) | ~50 | ✅ |
| 16 | Service constructor helper | ~20 | ✅ `newService` |
| 17 | `readResponseBody` helper | ~15 | ✅ |
| 18 | `tailSlice[T]` generic helper | ~15 | ✅ |
| 19 | Nil-to-empty-slice normalizasyonu `writeJSON` içinde | ~18 | ✅ |
| 20 | `requireURLParam` helper | ~36 | ✅ |

### Sprint 4 — Düşük Öncelikli İyileştirmeler 🔶

| # | İş | Durum |
|---|----|-------|
| 21 | Error mesaj string sabitleri | ✅ `internal/errmsg` |
| 22 | `interface{}` → `any` dönüşümü | ✅ |
| 23 | Validation logic birleştirme | ✅ |
| 24 | SELECT sütun listesi sabitleri | ✅ `sql_columns.go` |
| 25 | Embedding upsert/list birleştirme | ✅ `upsertEntityEmbedding`, `listEmbeddingsSkippingCorrupt` |
| 26 | Model/struct örtüşmesi çözümü | ⏳ |
| 27 | Typed error `ServiceError` | ✅ `QueryService` → `*ServiceError` |
| 28 | String-matching error dispatch kaldır | ✅ |
| 29 | Handler içi method check → router | ✅ chi method routing kullanılıyor |
| 30 | `ctx := r.Context()` standardizasyonu | ✅ `slog.ErrorContext` (sql_pool, eval stream) |
| 31 | `filter_session.go` regex ön-derleme | ✅ |
| 32 | Embedder HTTP retry | ✅ |
| 33 | Batch upsert (columns + relations) | ✅ |
| 34 | Compiler `ValidationErrors` → HTTP 400 | ✅ |

---

## Ek: Genel Best Practice Kontrolleri

### Go Style Compliance

| Kural | Durum |
|-------|-------|
| `gofmt` formatlama | Mevcut kod tabanında tutarlı |
| Error wrapping `%w` | Çoğunlukla doğru, istisnalar yukarıda |
| Struct field names (MixedCaps) | Tutarsızlık yok |
| Import gruplama (std → blank → external) | Tutarsızlık yok |
| Receiver isimleri | Tutarsızlık yok |
| Context ilk parametre | Tutarsızlık yok |

### Olumlu Bulgular

- `slog` tutarlı kullanımı (çoğunlukla)
- `defer rows.Close()` pattern'i doğru
- `context.Context` tutarlı kullanımı
- Early return hata akışları (deep nesting yok)
- `errors.Is()` / `errors.As()` doğru kullanım
- Custom error tipleri (`ValidationError`) iyi tasarlanmış
- Parameterized queries (SQL injection koruması)
- AES-encrypted DSN storage
- Compile-time interface check'ler (çoğunlukla)
