# Biqly Backend Go Code Review — Kod Tekrarı & Best Practice Analizi

> Tarih: 2026-05-13  
> Kapsam: `internal/` altındaki tüm Go paketleri (71 üretim dosyası, ~97 toplam .go dosyası)

---

## İçindekiler

1. [Kritik Kod Tekrarı (High Priority)](#1-kritik-kod-tekrarı-high-priority)
2. [Orta Öncelikli Tekrarlar (Medium Priority)](#2-orta-öncelikli-tekrarlar-medium-priority)
3. [Go Best Practice İhlalleri](#3-go-best-practice-ihlalleri)
4. [Mimari Öneriler](#4-mimari-öneriler)
5. [Özet Metrikler](#5-özet-metrikler)

---

## 1. Kritik Kod Tekrarı (High Priority)

### 1.1 Dialect `Aggregate()` — 4 dosyada neredeyse aynı implementasyon

**Dosyalar:**

- `internal/dialect/postgres.go:107-124`
- `internal/dialect/mysql.go:103-120`
- `internal/dialect/sqlserver.go:97-114`
- `internal/dialect/clickhouse.go:99-116`

**Sorun:** `Aggregate()` fonksiyonu 4 dialect'te ~%90 aynı. Sadece `count_distinct` → `uniq()` (ClickHouse) ve `count(*)` → `count()` (ClickHouse) farklı. Geri kalan tüm case'ler birebir tekrar.

**Öneri:** `baseDialect` embed edilebilir struct veya `AggregateDefault()` yardımcı fonksiyonu:

```go
// dialect/base.go
func AggregateDefault(d Dialect, fn, column string) string {
    if strings.ToLower(fn) == "count" && column == "*" {
        return "COUNT(*)"
    }
    quotedCol := d.QuoteIdent(column)
    switch strings.ToLower(fn) {
    case "count":
        return fmt.Sprintf("COUNT(%s)", quotedCol)
    case "count_distinct":
        return fmt.Sprintf("COUNT(DISTINCT %s)", quotedCol)
    // ... sum, avg, min, max
    default:
        return fmt.Sprintf("COUNT(%s)", quotedCol)
    }
}

// ClickHouse override
func (d ClickHouseDialect) Aggregate(fn, column string) string {
    if strings.ToLower(fn) == "count_distinct" {
        return fmt.Sprintf("uniq(%s)", d.QuoteIdent(column))
    }
    return AggregateDefault(d, fn, column)
}
```

**Etki:** ~80 satır tekrarın önüne geçer, yeni dialect eklemeyi kolaylaştırır.

---

### 1.2 Dialect `QuoteIdent()` — 4 dosyada birebir aynı

**Dosyalar:**

- `internal/dialect/postgres.go:19-25`
- `internal/dialect/mysql.go:20-26`
- `internal/dialect/clickhouse.go:20-26`
- `internal/dialect/sqlserver.go:20-26`

**Sorun:** `QuoteIdent()` her dialect'te aynı algoritma: split on `.`, her parçayı `QuoteIdentSegment()` ile quote et, join with `.`.

**Öneri:** Ortak implementasyon `BaseDialect` struct'ına taşınabilir:

```go
type BaseDialect struct{}

func (BaseDialect) QuoteIdent(identifier string) string {
    parts := strings.Split(identifier, ".")
    quoted := make([]string, len(parts))
    for i, part := range parts {
        quoted[i] = QuoteSegmentDefault(part) // alt sınıf override eder
    }
    return strings.Join(quoted, ".")
}
```

Sadece `QuoteIdentSegment()` kalır dialect'e özel ( `"`, `` ` ``, `[` ).

---

### 1.3 Dialect `LimitOffset()` — 3 dosyada birebir aynı

**Dosyalar:**

- `internal/dialect/postgres.go:33-41`
- `internal/dialect/mysql.go:35-43`
- `internal/dialect/clickhouse.go:35-43`

**Sorun:** PostgreSQL, MySQL, ClickHouse'ın `LimitOffset()` implementasyonları birebir aynı. Sadece SQL Server farklı (`OFFSET ... ROWS FETCH NEXT ... ROWS ONLY`).

**Öneri:** `BaseDialect.LimitOffset()` olarak paylaşılabilir.

---

### 1.4 Dialect `CastType()` — 4 dosyada birebir aynı

**Dosyalar:**

- `internal/dialect/postgres.go:62`
- `internal/dialect/mysql.go:73`
- `internal/dialect/clickhouse.go:68`
- `internal/dialect/sqlserver.go:68`

**Sorun:** Tümü `strings.ToUpper(sqlType)` döndürüyor. Tek satırlık olsa da 4 kez yazılmış.

---

### 1.5 HTTP Handler İstek Parse/Validate Kalıbı — `ai.go` içinde 3 kez

**Dosya:** `internal/http/handlers/ai.go`

**Sorun:** `Query()`, `Preview()`, `Run()` handler'larının her üçü de şu kalıbı tekrarlıyor:

```go
var req aiQueryRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    writeError(w, http.StatusBadRequest, "invalid request body")
    return
}
if req.Question == "" {
    writeError(w, http.StatusBadRequest, "question is required")
    return
}
if req.DatasourceID == "" {
    writeError(w, http.StatusBadRequest, "datasource_id is required")
    return
}
ctx := r.Context()
model, routing, err := h.loadQueryModel(ctx, req)
if err != nil {
    h.writeModelLoadError(w, req, err)
    return
}
if routing != nil && routing.NeedsClarification {
    resp := clarificationResponse(routing)
    h.recordAIHistory(ctx, req, model, routing, resp)
    writeJSON(w, http.StatusOK, resp)
    return
}
```

Bu blok **satır satır aynı** `Query()`, `Preview()` ve `Run()` içinde.

**Öneri:**

```go
func (h *AIHandler) parseAndRouteAIQuery(w http.ResponseWriter, r *http.Request) (aiQueryRequest, *semantic.SemanticModel, *ai.TableRoutingResult, bool) {
    var req aiQueryRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return req, nil, nil, false
    }
    if req.Question == "" || req.DatasourceID == "" {
        writeError(w, http.StatusBadRequest, "question and datasource_id are required")
        return req, nil, nil, false
    }
    model, routing, err := h.loadQueryModel(r.Context(), req)
    if err != nil {
        h.writeModelLoadError(w, req, err)
        return req, nil, nil, false
    }
    if routing != nil && routing.NeedsClarification {
        resp := clarificationResponse(routing)
        h.recordAIHistory(r.Context(), req, model, routing, resp)
        writeJSON(w, http.StatusOK, resp)
        return req, nil, nil, false
    }
    return req, model, routing, true
}
```

**Etki:** ~60 satır tekrar giderilir, `Query()`, `Preview()`, `Run()` sadece kendi özgün mantığına odaklanır.

---

### 1.6 AI Provider HTTP Retry Loop — `client.go` ve `anthropic.go` birebir aynı

**Dosyalar:**

- `internal/ai/client.go:103-160` (`GenerateAt` fonksiyonu)
- `internal/ai/anthropic.go:79-136` (`GenerateAt` fonksiyonu)

**Sorun:** İki dosyadaki HTTP retry mekanizması (exponential backoff, 4 attempt, 429/502-504 retry) satır satır aynı:

```go
const maxAttempts = 4
var lastErr error
for attempt := 0; attempt < maxAttempts; attempt++ {
    if attempt > 0 {
        delay := time.Duration(250*(1<<uint(attempt-1))) * time.Millisecond
        if err := sleepCtx(ctx, delay); err != nil {
            return "", err
        }
    }
    // HTTP call, read body, check status...
}
```

Fark sadece: endpoint (`/chat/completions` vs `/messages`), header'lar ve response struct.

**Öneri:** Generic retry-wrapper fonksiyon:

```go
func retryHTTP(ctx context.Context, maxAttempts int, fn func() (string, error)) (string, error) {
    var lastErr error
    for attempt := 0; attempt < maxAttempts; attempt++ {
        if attempt > 0 {
            delay := time.Duration(250*(1<<uint(attempt-1))) * time.Millisecond
            if err := sleepCtx(ctx, delay); err != nil {
                return "", err
            }
        }
        result, err := fn()
        if err == nil {
            return result, nil
        }
        lastErr = err
    }
    return "", lastErr
}
```

---

### 1.7 Datasource Driver `Ping()` ve `Open()` Kalıbı — 4 dosyada çok benzer

**Dosyalar:**

- `internal/datasource/postgres/driver.go:36-55`
- `internal/datasource/mysql/driver.go:33-48`
- `internal/datasource/sqlserver/driver.go:33-48`
- `internal/datasource/clickhouse/driver.go:36-55`

**Sorun:** Her driver'da `Ping()` ve `Open()` aynı yapıyı takip ediyor:

```go
func (d *Driver) Ping(ctx context.Context, dsn string) error {
    db, err := sql.Open("<driver>", dsn)
    // ...
    defer db.Close()
    return db.PingContext(ctx)
}

func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
    db, err := sql.Open("<driver>", dsn)
    // ...
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    return db, nil
}
```

Sadece driver adı ve `MaxOpenConns` farklı (ClickHouse: 10).

**Öneri:** `BaseDriver` struct:

```go
type BaseDriverConfig struct {
    DriverName    string
    MaxOpenConns  int
    MaxIdleConns  int
}

func PingDB(ctx context.Context, driverName, dsn string) error { ... }
func OpenDB(ctx context.Context, driverName, dsn string, cfg BaseDriverConfig) (*sql.DB, error) { ... }
```

---

### 1.8 `historyModelID()` ve `marshalSQLArgs()` — 2 farklı pakette tekrar

**Dosyalar:**

- `internal/http/handlers/history.go:101-110` (`historyModelID`)
- `internal/core/query_service.go:174-178` (`historyModelID`)
- `internal/http/handlers/history.go:112-119` (`marshalSQLArgs`)
- `internal/core/query_service.go:180-188` (`marshalSQLArgs`)

**Sorun:** `historyModelID()` fonksiyonu iki yerde aynı şekilde tanımlı. `marshalSQLArgs()` da birebir tekrar.

**Öneri:** `query` paketine utility fonksiyon olarak taşınabilir:

```go
// internal/query/history_helpers.go
func ModelID(model *semantic.SemanticModel) *string { ... }
func MarshalSQLArgs(args []any) (*string, error) { ... }
```

---

## 2. Orta Öncelikli Tekrarlar (Medium Priority)

### 2.1 `scanner` Interface — 2 pakette tekrar tanımlı

**Dosyalar:**

- `internal/metadata/repository.go:91-93`
- `internal/semantic/repository.go:282-284`

**Sorun:** Her iki repository kendi `scanner` interface'ini tanımlıyor. Birebir aynı:

```go
type scanner interface {
    Scan(dest ...any) error
}
```

**Öneri:** `internal/platform/db` paketine taşınabilir.

---

### 2.2 Row-Level Security Filter Injection — 2 yerde benzer mantık

**Dosyalar:**

- `internal/query/compiler.go:99-172` (`CompileWithPermissions`)
- `internal/security/row_injection.go:24-68` (`InjectRowFilters`)

**Sorun:** İki yerde benzer row-filter injection mantığı var. Farklı yaklaşımlar kullanıyor (biri compiler-level, diğeri post-compile SQL string manipulation). `Compiler.CompileWithPermissions` aslında `PermissionInjector.InjectRowFilters`'ı kullanmıyor, kendi SQL string parsing'ini yapıyor.

**Öneri:** Tek bir implementasyon seçilmeli. Compiler'ın `CompileWithPermissions`'ı daha doğru yaklaşım (compile-time placeholder yönetimi). `row_injection.go`'daki `InjectRowFilters` kaldırılabilir veya refactor edilebilir.

---

### 2.3 `BuildQueryHistoryEntry` — 2 pakette aynı ama farklı

**Dosyalar:**

- `internal/http/handlers/history.go:55-82`
- `internal/core/query_service.go:141-173`

**Sorun:** Her iki yerde `BuildQueryHistoryEntry`/`buildQueryHistoryEntry` fonksiyonu var. Temel mantık aynı (`LogicalQuery` → `HistoryEntry`), ama `core` versiyonu `Fingerprint` ekliyor.

**Öneri:** Tek bir canonical `BuildQueryHistoryEntry` fonksiyonu `query` paketinde, opsiyonel `Fingerprint` parametresi ile.

---

### 2.4 Introspection Row-Scan Kalıbı — 4 dosyada tekrarlanan yapı

**Dosyalar:**

- `internal/datasource/postgres/introspect.go`
- `internal/datasource/mysql/driver.go` (inline)
- `internal/datasource/sqlserver/driver.go` (inline)
- `internal/datasource/clickhouse/driver.go` (inline)

**Sorun:** Her `introspectSchemas`, `introspectTables`, `introspectColumns`, `introspectRelations` fonksiyonu şu kalıbı tekrarlıyor:

```go
rows, err := db.QueryContext(ctx, query)
if err != nil { return nil, err }
defer func() { _ = rows.Close() }()
var result []T
for rows.Next() {
    var item T
    if err := rows.Scan(&item.Field1, &item.Field2, ...); err != nil {
        return nil, err
    }
    result = append(result, item)
}
return result, rows.Err()
```

**Öneri:** Generic helper fonksiyon (Go 1.18+ generics):

```go
func queryRows[T any](ctx context.Context, db *sql.DB, query string, scan func(*sql.Rows) (T, error)) ([]T, error) {
    rows, err := db.QueryContext(ctx, query)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []T
    for rows.Next() {
        item, err := scan(rows)
        if err != nil { return nil, err }
        out = append(out, item)
    }
    return out, rows.Err()
}
```

---

### 2.5 `PermissionManager.CheckFieldAccess` vs `PermissionInjector.CheckFieldAccess` vs `PermissionInjector.isFieldAllowed`

**Dosyalar:**

- `internal/security/permissions.go:56-61` (`HasFieldAccess`)
- `internal/security/row_injection.go:70-85` (`CheckFieldAccess` + `isFieldAllowed`)

**Sorun:** Aynı "field denied mi?" kontrolü 3 farklı yerde yapılıyor:

```go
// permissions.go
slices.Contains(policy.DeniedFields, qualifiedField) || slices.Contains(policy.DeniedFields, field)

// row_injection.go
for _, denied := range policy.DeniedFields {
    if denied == qualified || denied == unqualified { return false }
}
```

**Öneri:** Tek bir `IsFieldDenied(policy, modelName, field) bool` fonksiyonu.

---

## 3. Go Best Practice İhlalleri

### 3.1 Error Wrapping Tutarlılığı

**Sorun:** Bazı yerler `%w` ile wrap ediyor, bazıları etmiyor:

```go
// İyi — %w ile wrapping (errors.Is/As ile match edilebilir)
return nil, fmt.Errorf("failed to open postgres connection: %w", err)

// Kötü — sadece string, orijinal error kayıp
slog.ErrorContext(ctx, "list few-shot examples failed", "error", err)
return nil  // error dönmüyor, sadece log

// Kötü — error kayıp
writeError(w, http.StatusInternalServerError, "failed to list tables")
// orijinal SQL error log'a bile yazılmıyor
```

**Öneri:** Handler'larda internal error'ları log + client'a generic mesaj:

```go
slog.ErrorContext(ctx, "list tables failed", "error", err)
writeError(w, http.StatusInternalServerError, "failed to list tables")
```

Ama repository katmanında her zaman `%w` ile wrap etmek.

---

### 3.2 `defer rows.Close()` Sonrası `rows.Err()` Kontrolü

**Sorun:** Bazı yerlerde `rows.Err()` kontrolü eksik. Örnek:

```go
// metadata/repository.go — çoğu yerde var, iyi
return datasources, rows.Err()

// Ama bazı introspect fonksiyonlarında rows.Err() kontrolü eksik:
// postgres/introspect.go — schemas için rows.Err() çağrılmıyor (sadece tables/columns'da var)
```

**Öneri:** Tüm `for rows.Next()` döngülerinden sonra `return result, rows.Err()` eklenebilir. Linter rule: `rowserrcheck`.

---

### 3.3 Handler'larda `sql.DB` Doğrudan Kullanımı

**Dosya:** `internal/http/handlers/ai_examples.go`

**Sorun:** `AIExamplesHandler` doğrudan `h.deps.MetadataDB` ile SQL çalıştırıyor. Diğer tüm handler'lar `MetaRepo` üzerinden gidiyor. Bu hem tutarsız hem test edilebilirliği düşürüyor.

```go
// ai_examples.go — doğrudan SQL
rows, err := h.deps.MetadataDB.QueryContext(ctx, q, args...)

// datasource.go — repository üzerinden
datasources, err := h.deps.MetaRepo.ListDatasources(ctx)
```

**Öneri:** `ai_examples.go`'daki tüm SQL sorguları `metadata.Repository`'ye taşınmalı. Handler sadece repository metodunu çağırmalı.

---

### 3.4 Magic Numbers / Hardcoded Değerler

**Dosyalar:** Birden fazla dosya

**Sorunlar:**

| Değer | Yer | Sorun |
| ------- | ----- | ------- |
| `25` | 4 driver'ın `SetMaxOpenConns` | Config'e taşınmalı |
| `5` | 4 driver'ın `SetMaxIdleConns` | Config'e taşınmalı |
| `10` | ClickHouse `MaxOpenConns` | Neden farklı? Açıklama yok |
| `"many_to_one"` | mysql, sqlserver, clickhouse introspect | Hardcoded relationship type |
| `100` | `ListQueryHistory` LIMIT | Config'e taşınmalı |
| `50` | `EvalListRuns` LIMIT | Config'e taşınmalı |

**Öneri:** Pool ayarları `platform/db.Config`'dan, SQL limit'ler repository parametresi olarak geçirilmeli.

---

### 3.5 `db` Alan Adı Çakışması — `Datasource.DSNEncrypted`

**Dosya:** `internal/metadata/model.go:11`

**Sorun:** Alan adı `DSNEncrypted` ama JSON'da `"-"` (hiçbir zaman serialize edilmiyor). `SyncMetadata` handler'ı'nda (`datasources.go:181`) DSN decrypt edildikten sonra `ds.DSNEncrypted`'a decrypt edilmiş düz metin atanıyor:

```go
dsn := ds.DSNEncrypted  // encrypted
if h.deps.Encryptor != nil && h.deps.Encryptor.IsEncrypted(dsn) {
    dsn, err = h.deps.Encryptor.Decrypt(dsn)  // dsn = plaintext
}
// daha sonra:
db, err := driver.Open(ctx, dsn)  // plaintext DSN kullanılıyor — doğru
```

Bu çalışıyor ama kafa karıştırıcı. `DSNEncrypted` alanında bazen encrypted, bazen plaintext değer var.

**Öneri:** Repository'den dönen objelerde `DSNEncrypted`'ı değiştirmeden, yerel değişkende decrypted DSN tutulmalı:

```go
dsn := ds.DSNEncrypted
decryptedDSN, err := h.decryptDSN(ctx, dsn)
// decryptedDSN kullan, ds.DSNEncrypted'a dokunma
```

---

### 3.6 `semantic/model.go` — Deprecated Alias

**Dosya:** `internal/semantic/model.go:9-10`

```go
// Deprecated: Use SemanticModel instead
type Model = SemanticModel
```

**Sorun:** Deprecated alias tanımlı ama hiçbir yerde `Model` kullanılmıyor. Tüm kod tabanı `SemanticModel` kullanıyor.

**Öneri:** Kaldırılabilir.

---

### 3.7 `var _ strings.Contains` — Dead Code

**Dosya:** `internal/datasource/clickhouse/driver.go:94`

```go
var _ = strings.Contains
```

**Sorun:** Kullanılmayan import'u tutmak için hack. Direkt kaldırılmalı.

---

### 3.8 Handler Başına `*app.Dependencies` Field — Dependency Bloat

**Sorun:** Tüm handler'lar (`AIHandler`, `AIExamplesHandler`, `DatasourceHandler`, `QueryHandler`, `MetadataHandler`, `SemanticHandler`) `*app.Dependencies` tutuyor. Bu her handler'a tüm dependency'leri (AI client, Redis, encryptor, eval repo, embedder...)暴露 ediyor.çoğu sadece 1-2 dependency kullanıyor.

**Öneri (ileriye dönük):** Handler başına sadece ihtiyaç duyulan interface'leri kabul etmek daha temiz olur. Mevcut hali çalışıyor ama dependency graph büyüdükçe sorun yaratabilir.

---

## 4. Mimari Öneriler

### 4.1 Dialect Paketi — `BaseDialect` Embedding Pattern

Tüm dialect'lerin paylaştığı metotları (`QuoteIdent`, `LimitOffset`, `CastType`, `Aggregate` default, `ExplainSQL` default) içeren bir `BaseDialect` struct oluşturulabilir. Her dialect sadece farklı olan kısımları override eder:

```text
BaseDialect
├── PostgresDialect   (QuoteIdentSegment: ", Placeholder: $N)
├── MySQLDialect      (QuoteIdentSegment: `, Placeholder: ?)
├── ClickHouseDialect (QuoteIdentSegment: `, Aggregate: uniq())
└── SQLServerDialect  (QuoteIdentSegment: [], Placeholder: @pN, LimitOffset: OFFSET/FETCH)
```

**Tahmini satır tasarrufu:** ~100-150 satır.

---

### 4.2 Datasource Driver — `BaseDriver` Composition

Benzer şekilde driver'ların `Ping`, `Open`, `Introspect` iskeletini paylaşan bir base:

```text
BaseDriver(dialect, driverName, poolConfig)
├── PostgresDriver   (custom introspect queries)
├── MySQLDriver      (information_schema queries)
├── SQLServerDriver  (sys.* queries)
└── ClickHouseDriver (system.* queries, no relations)
```

---

### 4.3 AI Provider — Retry + Request/Response Decoupling

`client.go` ve `anthropic.go` retry logic'i ayrılmış olsaydı, yeni bir provider eklemek sadece request/response mapping yazmak olurdu:

```text
Provider interface
├── doRequest(ctx, prompt, temp) → raw response string
├── OpenAIProvider    (chat/completions endpoint)
├── AnthropicProvider (messages endpoint)
└── [gelecekte] GeminiProvider, OllamaProvider...
```

Retry logic `Provider` implementasyonuna değil, wrapper'a ait olmalı.

---

### 4.4 Repository — Generic Helpers

`queryRows[T]` generic fonksiyonu ile introspection ve repository sorguları daha kısa ve tutarlı hale getirilebilir. Mevcut Go 1.26 ile generics tam destekleniyor.

---

### 4.5 `scanner` Interface — Platform Paketine Taşıma

`scanner` interface'i `internal/platform/db/scanner.go` altına taşınabilir ve tüm repository'ler oradan import edebilir.

---

## 5. Özet Metrikler

| Metrik | Değer |
| -------- | ------- |
| Toplam analiz edilen dosya | 71 üretim .go dosyası |
| Tespit edilen kritik tekrar | 8 konum |
| Tahmini kaldırılabilir tekrar satırı | ~250-350 satır |
| Best practice ihlali | 8 konum |
| Önerilen yeni yardımcı dosya | 2-3 (base_dialect.go, base_driver.go, db/scanner.go) |
| Mevcut test coverage | 26 test dosyası (iyi) |

### Öncelik Sırası (Implementasyon İçin)

1. **Dialect BaseDialect** — En yüksek tekrar yoğunluğu, en kolay refactor
2. **AI Handler parseAndRouteAIQuery** — ~60 satır 3x tekrar, basit helper
3. **AI Provider retry wrapper** — 2 provider, gelecekte daha fazla eklenecek
4. **historyModelID / marshalSQLArgs** — Basit utility taşıma
5. **ai_examples.go → repository** — Test edilebilirlik + tutarlılık
6. **Datasource BaseDriver** — 4 dosya, orta karmaşıklık
7. **Permission field access consolidation** — 3 yerde aynı mantık
8. **Magic number → config** — Düşük çaba, yüksek readability

---

## 6. Refaktör takibi (nice-to-have)

Sonraki iterasyonlarda aşağıdakiler uygulandı (bu bölüm raporun orijinal bulgularını değiştirmez; sadece ilerlemeyi not eder):

- **`internal/metadata/repository.go`**: Sorgu/tarama/`rows.Err()` ve JSON yardımcıları için tutarlı `%w` hata sarımı; `ListPermissionPolicies` içinde `row_filters` JSON’u bozuksa artık hata dönüyor (sessiz `json.Unmarshal` yok).
- **`internal/semantic/repository.go`**: Model boyutları, boyut/metric/join listeleri, publish/rollback işlemleri, snapshot okuma ve `decodeModelSnapshot` için `%w` sarımı; `GetPublishedFullModel` içinde `sql.ErrNoRows` için `errors.Is` kullanımı.
- **Lint**: `rowserrcheck` golangci yapılandırmasında etkin (repo genelinde başka uyarılar ayrı iş konusu).

---

*Bu rapor otomatik kod analizi sonucu oluşturulmuştur. Her öneri mevcut test suite'i ile doğrulanarak uygulanmalıdır.*
