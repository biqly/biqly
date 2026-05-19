# Biqly Backend Code Review

> Tarih: 2026-05-19
> Kapsam: `internal/` altindaki tum Go dosyalari
> Amac: Kod tekrari, anti-pattern ve refactoring firsatlarini belirlemek

---

## Ozet

Toplam **22 bulgu** tespit edildi. Kod tabani genel olarak iyi yapilandirilmis; `BaseDriver`, `BaseDialect`, `baseProvider` ve `decodeJSON[T]` gibi soyutlamalar mevcut. Ancak onemli tekrar alanlari var:

| Siddet | Sayi |
|---------|------|
| HIGH    | 2    |
| MEDIUM  | 8    |
| LOW     | 12   |

---

## HIGH Severity

### H1: `Create` vs `datasourceDraft` — ~80 satir tekrar

**Dosya:** `internal/http/handlers/datasources.go`

`Create` (satir 89-239, 151 satir) ve `datasourceDraft` (satir 241-394, 154 satir) metodlari ~%85 ayni mantigi tasiyor.

**Tekrarlanan bloklar:**

1. **Validasyon** (95-113 vs 242-260): `name/type` bosluk kontrolu, `NormalizeDriverType`, `resolveCreateDatasourceMode`
2. **hasConnPayload kontrolu** (119-130 vs 264-274): Ayni alan kontrolleri
3. **Structured DSN field mapping** (161-202 vs 317-356): host, port, username, database_name, ssl_mode, connection_params
4. **ConnectionFields struct** (204-212 vs 367-375): Ayni alan atamalari

**Subtle bug:** `Create` satir 121'de `req.Connection.Port != nil && *req.Connection.Port > 0` ve satir 124'te `strings.TrimSpace(req.Connection.Password) != ""` kontrolu yaparken, `datasourceDraft` satir 267'de sadece `req.Connection.Port != nil` ve satir 270'de `req.Connection.Password != ""` kontrolu yapiyor. Kosullar farkli — potansiyel bug.

**Oneri:** Ortak mantigi `resolveDatasourceFields()` fonksiyonuna cikartin. `Create` metodu ~20 satira iner.

```go
func (h *DatasourceHandler) resolveDatasourceFields(
    ctx context.Context,
    req createDatasourceRequest,
    isUpdate bool,
) (*metadata.Datasource, datasource.ConnectionFields, string, int, string, error) {
    // ortak validasyon + alan esleme mantigi
}
```

**Tahmini kazanc:** ~120 satir azalma, 1 bug onleme

---

### H2: 12 fonksiyon 80+ satir uzunlugunda

Uzun fonksiyonlar test edilebilirligi ve okunabilirligi dusurur.

| Dosya | Satir | Fonksiyon | Uzunluk |
|-------|-------|-----------|---------|
| `query/validator.go` | 27-225 | `Validate` | 199 |
| `http/handlers/datasources.go` | 586-750 | `SyncMetadata` | 165 |
| `http/handlers/datasources.go` | 241-394 | `datasourceDraft` | 154 |
| `http/handlers/datasources.go` | 89-239 | `Create` | 151 |
| `ai/service.go` | 144-276 | `ProcessQuestion` | 133 |
| `datasource/dsn.go` | 88-217 | `ComposeDSN` | 130 |
| `http/router.go` | 15-142 | `Router` | 128 |
| `ai/prompt.go` | 52-173 | `Build` | 122 |
| `app/dependencies.go` | 52-163 | `NewDependencies` | 112 |
| `http/handlers/semantic.go` | 52-161 | `GenerateModel` | 110 |
| `semanticgen/generator.go` | 30-138 | `GenerateModelFromMetadata` | 109 |
| `http/handlers/ai_eval.go` | 212-317 | `EvalRunStream` | 106 |

**Oneri:** Oncelikli hedefler:
1. `Validate` — alt-validator metodlara bolunmeli (select, filter, join, metric validation)
2. `Create` + `datasourceDraft` — H1 ile birlikte cozulur
3. `ProcessQuestion` — retry dongusunu ayri metoda cikartin
4. `ComposeDSN` — her driver icin ayri DSN builder fonksiyon

---

## MEDIUM Severity

### M1: `writeInternalError` tutarsiz kullanimi

**Dosya:** `internal/http/handlers/helpers.go:48`

`writeInternalError(ctx, w, status, msg, err)` yardimcisi mevcut ve `datasources.go`, `ai.go`, `ai_eval.go`'da kullaniliyor. Ancak **30 farkli yerde** elle `slog.ErrorContext` + `writeError` cagriliyor.

**Etkilenen dosyalar:**

| Dosya | Manuel kullanim sayisi |
|-------|----------------------|
| `semantic.go` | 13 |
| `ai_examples.go` | 8 |
| `ai_glossary.go` | 4 |
| `datasources.go` | 6 |
| `ai.go` | 2 |

**Mevcut manual pattern (30 kez tekrar):**
```go
slog.ErrorContext(ctx, "failed to X", "error", err)
writeError(w, http.StatusInternalServerError, "failed to X")
return
```

**Oneri:** Tum handler'larda `writeInternalError` kullanima gecilmeli:

```go
// Once:
writeInternalError(ctx, w, http.StatusInternalServerError, "failed to X", err)
return
```

**Tahmini kazanc:** 30 ayri satir ciftini tek satira indirir.

---

### M2: Magic string literals — `"many_to_one"` ve `"LEFT"`

**`"many_to_one"` — 8 uretim kullanimi:**

| Dosya | Satir |
|-------|-------|
| `http/handlers/semantic.go` | 522, 806 |
| `semanticgen/generator.go` | 280 |
| `semantic/publish.go` | 147 |
| `query/planner.go` | 151 |
| `datasource/postgres/introspect.go` | 151 |
| `datasource/mysql/driver.go` | 71 |
| `datasource/sqlserver/driver.go` | 89 |

**`"LEFT"` (default join type) — 5 uretim kullanimi:**

| Dosya | Satir |
|-------|-------|
| `http/handlers/semantic.go` | 517, 802 |
| `query/compiler.go` | 575 |
| `semanticgen/generator.go` | 292 |
| `ai/table_router.go` | 1201 |

**Oneri:** `internal/semantic/model.go` veya ortak bir constants dosyasina:

```go
const (
    DefaultJoinType     = "LEFT"
    RelationshipManyToOne  = "many_to_one"
    RelationshipOneToMany  = "one_to_many"
    RelationshipOneToOne   = "one_to_one"
)
```

---

### M3: `nullableString` ayni isimde, farkli imzada, 2 pakette

| Dosya | Satir | Imza | Fark |
|-------|-------|------|------|
| `metadata/repository.go` | 751 | `func nullableString(p *string) any` | `*string` alir, nil + empty check |
| `semantic/repository.go` | 564 | `func nullableString(value string) any` | `string` alir, empty check |

Ayni amac (SQL nullable parametre donusumu), farkli imza.

**Oneri:** `internal/platform/db/helpers.go` altinda birlestirin:

```go
func NullIfEmptyPtr(p *string) any { ... }
func NullIfEmpty(s string) any      { ... }
```

---

### M4: Semantic CRUD handler boilerplate

**Dosya:** `internal/http/handlers/semantic.go`

`requireURLParam → decodeJSON → struct olustur → repo cagri → writeJSON` deseni 9 kez tekrarlaniyor:

- `CreateDimension` (409-442)
- `CreateMetric` (454-489)
- `CreateJoin` (504-547)
- `UpdateDimension` (567-598)
- `UpdateMetric` (618-651)
- `UpdateJoin` (787-827)
- `DeleteDimension` (550-564)
- `DeleteMetric` (601-615)
- `DeleteJoin` (654-668)

Join default degerleri `CreateJoin` ve `UpdateJoin`'de birebir ayni:

```go
// semantic.go:515-523 ve semantic.go:800-807
joinType := req.JoinType
if joinType == "" {
    joinType = "LEFT"
}
relationship := req.Relationship
if relationship == "" {
    relationship = "many_to_one"
}
```

**Oneri:** `resolveJoinDefaults()` ve `buildJoinFromRequest()` helper'lari cikartin.

---

### M5: `AIResponse` construction — 4 kez tekrar

**Dosya:** `internal/ai/service.go`

`&AIResponse{...}` literali 4 farkli yerde (satir 201-211, 232-243, 249-260, 263-275) ~10 alan ile olusturuluyor.

**Oneri:** Builder veya helper fonksiyon:

```go
func newAIResponse(lq *LogicalQuery, warnings []string, prompt, raw string, retries int, stats AIStats, gen GenerationResult, clarification *string) *AIResponse
```

---

### M6: Token usage mapping tekrari

**Dosyalar:** `internal/ai/client.go:121-128`, `internal/ai/anthropic.go:121-128`

Her iki provider'da neredeyse ayni token usage mapping:

```go
gen.Usage = &TokenUsage{
    Prompt:     usage.PromptTokens,
    Completion: usage.CompletionTokens,
    Total:      usage.PromptTokens + usage.CompletionTokens,
}
```

**Oneri:** Helper fonksiyon:

```go
func newTokenUsage(prompt, completion int) *TokenUsage {
    return &TokenUsage{
        Prompt:     prompt,
        Completion: completion,
        Total:      prompt + completion,
    }
}
```

---

### M7: System directive string tekrari

**Dosyalar:**
- `internal/ai/client.go:98` — inline hardcoded
- `internal/ai/anthropic.go:14` — `const anthropicSystemDirective = "You are a Business Intelligence query assistant. Output only valid JSON."`

Ayni string iki yerde.

**Oneri:** `internal/ai/system_directive.go`:

```go
const SystemDirective = "You are a Business Intelligence query assistant. Output only valid JSON."
```

---

### M8: Error mesajlari tutarsiz

Benzer hatalar icin farkli HTTP status kodlari ve mesaj formatlari kullaniliyor:

- `"model not found"` vs `"datasource not found"` vs `"column not found"`
- Bazi handler'lar lookup basarisinda `404`, bazilari `400` donuyor
- `"failed to create model"` vs `"failed to create dimension"` — tutarli ama kod tekrari

**Oneri:** Standardize edilmis entity-not-found ve operation-failed mesajlari:

```go
var (
    ErrEntityNotFound = func(entity string) string { return entity + " not found" }
    ErrOperationFailed = func(op string) string { return "failed to " + op }
)
```

---

## LOW Severity

### L1: `nil` slice normalization gereksiz tekrar

**Dosyalar:** `ai_examples.go:64-66,207-209,225-227,248-250`, `ai_glossary.go:60-62`

```go
if results == nil {
    results = []SomeType{}
}
```

`writeJSON` (helpers.go:26-30) zaten nil slice'lari `[]`'ya donusturuyor. Bu kontroller gereksiz.

---

### L2: Handler struct boilerplate

Her handler ayni deseni takip ediyor:

```go
type XHandler struct {
    deps *app.Dependencies
}
func NewXHandler(deps *app.Dependencies) *XHandler {
    return &XHandler{deps: deps}
}
```

7 handler icin tekrarli. Go'da bu idiomatik kabul edilir, ancak istenirse `BaseHandler` embed edilebilir.

---

### L3: `nullableStringPtr` isim cakismasi

- `metadata/repository.go:720` — `func nullableStringPtr(value sql.NullString) *string` (sql.NullString -> *string)
- `http/handlers/datasources.go:78` — `func optionalStringPtr(s string) *string` (string -> *string)

Farkli amaclar ama benzer isimlendirme karisirlik yaratir.

**Oneri:** Handler'dakini `stringPtrIfNonEmpty` olarak yeniden adlandirin.

---

### L4: Connection params default tekrari

**Dosya:** `metadata/repository.go:36-39, 77-80`

```go
cp := ds.ConnectionParams
if len(cp) == 0 {
    cp = []byte("{}")
}
```

**Oneri:** `defaultConnectionParams(cp []byte) []byte` helper.

---

### L5: `ExcludedSchemas` nil default tekrari

**Dosya:** `semantic/repository.go:31-33, 81-83`

```go
if m.ExcludedSchemas == nil {
    m.ExcludedSchemas = []string{}
}
```

**Oneri:** `SemanticModel` constructor'ina veya `BeforeSave` hook'una tasiyin.

---

### L6: `MarkModelDraft` 9 kez cagriliyor

**Dosya:** `semantic/repository.go`

Her dimension/metric/join Create/Update/Delete'inden sonra `r.MarkModelDraft(ctx, id)` cagriliyor (9 yer).

**Oneri:** Transaction wrapper veya otomatik draft-marking patterni.

---

### L7: `Aggregate` delegate 3 dialect'ta tekrar

**Dosyalar:** `dialect/postgres.go:56`, `dialect/mysql.go:67`, `dialect/sqlserver.go:66`

```go
func (d PostgresDialect) Aggregate(fn, column string) string {
    return d.BaseDialect.Aggregate(d, fn, column)
}
```

Sadece ClickHouse override ediyor. Diger 3'u ayni delegation.

---

### L8: `Generate`/`GenerateAt` pass-through boilerplate

**Dosyalar:** `ai/client.go:83-91`, `ai/anthropic.go:80-88`

Her iki provider da ayni sekilde delegate ediyor:

```go
func (c *Client) Generate(ctx context.Context, prompt string) (GenerationResult, error) {
    return c.base.generate(ctx, prompt)
}
```

Explicit type safety icin kabul edilebilir boilerplate.

---

### L9: `closeResolvedDatasource` patterni

**Dosya:** `http/handlers/ai.go:234-241`

3 yerde kullanilan `defer func() { _ = resolved.DB.Close() }()` patterni.

---

### L10: `"datasource_id is required"` mesaji 7 dosyada

**Dosyalar:** `core/errors.go`, `core/service_error.go`, `http/handlers/ai.go`, `http/handlers/ai_examples.go`, `http/handlers/ai_glossary.go`

Ayni validation mesaji tekrar tekrar yazilmis.

**Oneri:** `const ErrDatasourceIDRequired = "datasource_id is required"`

---

### L11: `"unsupported datasource type"` mesaji 3 dosyada

**Dosyalar:** `app/errors.go`, `core/service_error.go`, `http/handlers/datasources.go`

---

### L12: `"many_to_one"` default deger driver'larda hardcoded

**Dosyalar:** `datasource/mysql/driver.go:71`, `datasource/sqlserver/driver.go:89`, `datasource/postgres/introspect.go:151`

**Oneri:** `const DefaultRelationshipType = "many_to_one"` in `datasource` package.

---

## Iyi Yapilandirilmis Alanlar

Asagidaki katmanlar temiz soyutlamalar kullaniliyor, refactoring gerektirmiyor:

| Katman | Soyutlama | Durum |
|--------|-----------|-------|
| Dialect | `BaseDialect`, `QuoteIdentQualified`, `CalendarPartLookup`, `AggregateStandardSQL` | Temiz |
| Datasource Driver | `BaseDriver`, `ComposeIntrospection`, `QueryAll[T]` | Temiz |
| AI Provider | `baseProvider`, `providerHooks` (marshal/parse/headers/path) | Temiz |
| HTTP Helpers | `decodeJSON[T]`, `writeJSON`, `writeError`, `writeInternalError` | Temiz (kullanim arttirilmali) |
| Query | `LogicalQuery` struct, parameterized compilation | Temiz |

---

## Onceliklendirilmis Refactoring Plani

### Faz 1 — Yuksek Etki, Dusuk Risk

| # | Islem | Etkilenen Dosyalar | Tahmini Kazanc |
|---|--------|-------------------|----------------|
| 1 | `Create` + `datasourceDraft` birlestir | `datasources.go` | ~120 satir + bug fix |
| 2 | Magic string constant'lari cikar | 15+ dosya | 0 satir ama guvenlik |
| 3 | `writeInternalError` standardize et | 5 handler dosyasi | 30 satir cifti -> 30 tek satir |

### Faz 2 — Orta Etki

| # | Islem | Etkilenen Dosyalar | Tahmini Kazanc |
|---|--------|-------------------|----------------|
| 4 | `nullableString` helper'larini birlestir | 2 repository | Bakim kolayligi |
| 5 | Join default + struct construction helper | `semantic.go`, `generator.go` | ~30 satir |
| 6 | `AIResponse` builder cikar | `ai/service.go` | ~40 satir |
| 7 | System directive + token usage helper | `ai/client.go`, `ai/anthropic.go` | ~10 satir |

### Faz 3 — Dusuk Oncelik

| # | Islem | Etkilenen Dosyalar |
|---|--------|-------------------|
| 8 | Uzun fonksiyonlari bol (Validate, ProcessQuestion, ComposeDSN) | 6 dosya |
| 9 | Gereksiz nil slice kontrollerini kaldir | 2 handler |
| 10 | Error mesaj sabitlerini standardize et | 5+ dosya |

---

## Ek Notlar

- Test dosyalarinda da `"many_to_one"` ve `"LEFT"` ~12+ kez hardcoded. Constant cikarildiginda testler de guncellenmeli.
- `datasourceDraft`'daki subtle bug (farkli Port/Password kosullari) H1 cozulurken duzeltilmeli.
- `writeInternalError` kullanima gecirildiginde mevcut log mesajlarinin kaybolmamasi icin `publicMsg` parametresinin log message ile ayni olmasina dikkat edilmeli.
