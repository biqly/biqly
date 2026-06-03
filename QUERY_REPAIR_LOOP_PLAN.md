Created At: 2026-06-03T08:44:04Z
Completed At: 2026-06-03T08:44:04Z
File Path: `file:///Users/baris.dogu/src/biqly/biqly/QUERY_REPAIR_LOOP_PLAN.md`

# Query Repair Loop — Implementation Plan

> **Amaç:** LLM LogicalQuery ürettiğinde validator hata verirse, mevcut basit retry mekanizmasını
> sistematik bir **repair loop**'a dönüştürmek. Validator hataları makine tarafından okunabilir
> structured error formatında olmalı; repair prompt bu yapılandırılmış hataları kullanarak LLM'e
> net, eyleme dönüştürülebilir düzeltme talimatları vermeli.

---

## Mevcut Durum Analizi

**Mevcut akış** (`internal/ai/service.go:338-397`):
- `for attempt := 0; attempt <= s.maxRetries` döngüsü var
- `parseAndValidate()` raw JSON → parse → validate yapıyor
- Başarısızlıkta `failureMessageFor()` ile string hata mesajı üretiliyor
- `BuildRetry()` ile orijinal prompt + son yanıt + hata mesajı birleştiriliyor
- Hata mesajı sadece free-form string — LLM'e yapılandırılmış düzeltme bilgisi verilmiyor

**Mevcut `ValidationError`** (`pkg/query/types.go:117-120`):
- Sadece `Field` ve `Message` alanları var
- `error_code`, `allowed_alternatives` gibi makine-tarafından-okunabilir alanlar yok

**Mevcut retry prompt** (`internal/ai/prompt/prompt.go:612-635`):
- "Why It Failed" bölümünde sadece string mesaj gösteriliyor
- "Required Action" bölümü generic — hangi alanı, neyle değiştirmesi gerektiği söylenmiyor

---

## [x] Aşama 1 — Structured ValidationError Sistemi

**Dosyalar:** `pkg/query/types.go`, `internal/errmsg/messages.go`

- [x] **1.1** `ValidationError` struct'ına `Code` alanı ekle:
  ```go
  type ValidationError struct {
      Field             string   `json:"field"`
      Code              string   `json:"code,omitempty"`
      Message           string   `json:"message"`
      Value             string   `json:"value,omitempty"`
      AllowedAlternatives []string `json:"allowed_alternatives,omitempty"`
  }
  ```
  - `Code`: `"UNKNOWN_DIMENSION"`, `"UNKNOWN_METRIC"`, `"UNKNOWN_FIELD"`, `"INVALID_OPERATOR"`,
    `"INVALID_TIME_GRAIN"`, `"TIME_GRAIN_ON_NON_DATE"`, `"MISSING_FIELD"`, `"INVALID_SELECT_TYPE"`,
    `"ROW_LIMIT_EXCEEDED"`, `"NEGATIVE_OFFSET"`, `"DATE_VALUE_TYPE_MISMATCH"`
  - `Value`: Hatalı değer (örn. `"customer_city"`)
  - `AllowedAlternatives`: Modeldeki geçerli alternatifler (örn. `["city", "customer_location", "billing_city"]`)

- [x] **1.2** `ValidationErrors`'a yardımcı metodlar ekle:
  ```go
  func (ve ValidationErrors) HasCode(code string) bool
  func (ve ValidationErrors) FilterByCode(code string) []*ValidationError
  func (ve ValidationErrors) ToRepairJSON() string  // Repair prompt için JSON export
  ```

- [x] **1.3** `errmsg` paketini güncelle — her hata tipi için `Code` sabiti tanımla:
  ```go
  const (
      CodeUnknownDimension    = "UNKNOWN_DIMENSION"
      CodeUnknownMetric       = "UNKNOWN_METRIC"
      CodeUnknownField        = "UNKNOWN_FIELD"
      CodeInvalidOperator     = "INVALID_OPERATOR"
      CodeInvalidTimeGrain    = "INVALID_TIME_GRAIN"
      // ...
  )
  ```

---

## [x] Aşama 2 — Validator'ı Structured Error Üretecek Şekilde Güncelle

**Dosyalar:** `internal/query/validator.go`, `internal/query/compiler.go`, `internal/query/calendar_grain_filter.go`

- [x] **2.1** `Validator.Validate()` metodunu güncelle — her `&ValidationError{}` oluşturulduğunda
  `Code`, `Value`, `AllowedAlternatives` alanlarını doldur:
  - **UNKNOWN_DIMENSION**: `Value = item.Name`, `AllowedAlternatives = model.Dimensions`'dan fuzzy match ile en yakın 3-5 isim
  - **UNKNOWN_METRIC**: `Value = item.Name`, `AllowedAlternatives = model.Metrics`'ten benzer isimler
  - **UNKNOWN_FIELD**: `Value = f.Field`, `AllowedAlternatives = dims + metrics` birleşiminden benzerler
  - **INVALID_OPERATOR**: `Value = f.Operator`, `AllowedAlternatives = validFilterOps`
  - **INVALID_TIME_GRAIN**: `Value = gb.TimeGrain`, `AllowedAlternatives = ["day","week","month","quarter","year"]`
  - **TIME_GRAIN_ON_NON_DATE**: `Value = gb.Field`

- [x] **2.2** Benzer isim bulmak için yardımcı fonksiyon ekle (`internal/query/validation_helpers.go`):
  ```go
  func suggestAlternatives(unknown string, candidates []string, maxSuggestions int) []string
  ```
  - Levenshtein veya prefix-based benzerlik kullan
  - `pkg/logicalquery` içindeki dimension/metric isimlerini source of truth olarak kullan

- [x] **2.3** `compiler.go`'daki `validationErr()` çağrılarını da aynı structured formatla güncelle

- [x] **2.4** `validateDateFilterValueType` fonksiyonunun döndüğü `ValidationError`'a
  `Code: "DATE_VALUE_TYPE_MISMATCH"` ve `AllowedAlternatives: [grain suggestions]` ekle

---

## [x] Aşama 3 — Repair Prompt Builder

**Dosyalar:** `internal/ai/prompt/prompt.go`, `internal/ai/prompt/prompt_templates.go`

- [x] **3.1** `BuildRetry`'ı `BuildRepairPrompt` olarak genişlet (backward-compat için `BuildRetry` korunsun):
  ```go
  func (b *PromptBuilder) BuildRepairPrompt(ctx context.Context, locale i18n.Locale,
      originalPrompt, lastResponse string, errors query.ValidationErrors) string
  ```
  - Structured error'ları repair talimatına dönüştür
  - Her hata için: "Replace `customer_city` → use one of: `city`, `customer_location`"
  - Bilinmeyen operatör: "Replace operator `like` → use one of: `contains`, `starts_with`"
  - Tarih tipi hatası: "Use grain dimension `created_at_month` instead of filtering raw `created_at`"

- [x] **3.2** Repair prompt template'ini tanımla (`internal/ai/prompt/prompt_templates.go` veya DB-backed template):
  ```
  ## Correction Required (attempt {attempt})

  The previous LogicalQuery JSON had {error_count} validation error(s):

  {structured_errors_json}

  ### Specific Fixes Needed:
  {per_error_replacement_instructions}

  ### Available Alternatives:
  {grouped_by_field_alternatives}
  ```

- [x] **3.3** `retry` template'ine `repair` template'ini ekle (versioned template sistemi üzerinden)

---

## [x] Aşama 4 — Service Retry Loop'unu Repair Loop'a Yükselt

**Dosyalar:** `internal/ai/service.go`

- [x] **4.1** `parseAndValidate` metodunu güncelle — `ValidationErrors`'ı structured olarak döndür:
  ```go
  func (s *Service) parseAndValidate(raw string, model *semantic.SemanticModel) (
      *query.LogicalQuery, []string, int, error, query.ValidationErrors)
  ```
  - Son return değeri: `query.ValidationErrors` (repair prompt için)

- [x] **4.2** Retry loop'ta repair prompt oluştur:
  ```go
  // Mevcut:
  failureMsg := failureMessageFor(parseErr, sqlErr, warnings)
  prompt = s.promptBuilder.BuildRetry(...)

  // Yeni:
  if validationErrors != nil {
      prompt = s.promptBuilder.BuildRepairPrompt(ctx, locale, expanded, gen.Content, validationErrors)
  } else {
      prompt = s.promptBuilder.BuildRetry(ctx, locale, expanded, gen.Content, failureMsg)
  }
  ```

- [x] **4.3** `failureMessageFor` fonksiyonunu güncelle — `ValidationErrors` parametresi alsın
  ve structured error'ları JSON formatında repair context'e eklesin

- [x] **4.4** Repair loop metriklerini ekle (`internal/platform/observability/metrics.go`):
  - `bi_ai_repair_success_total` — repair sonrası başarılı olanlar
  - `bi_ai_repair_by_error_code_total{code="UNKNOWN_DIMENSION"}` — hata kodu bazında
  - Mevcut `bi_ai_retries_total` korunsun

---

## [x] Aşama 5 — Context-Aware Repair Stratejisi

**Dosyalar:** `internal/ai/service.go`, `internal/ai/prompt/prompt.go`

- [x] **5.1** Deneme sayısına göre repair stratejisi belirle:
  - **Attempt 1**: Minimal repair — sadece hatalı alanları değiştir, çalışkanı koru
  - **Attempt 2**: Orta düzey — hatalı alanlar + join path'i yeniden değerlendir
  - **Attempt 3+**: Agresif — tüm LogicalQuery'yi yeniden üret, sadece tablo adını koru

- [x] **5.2** `BuildRepairPrompt`'a `attempt int` parametresi ekle, stratejiye göre prompt'u ayarla

- [x] **5.3** Partial repair desteği: `ValidationErrors`'daki sadece yüksek öncelikli (UNKNOWN_FIELD,
  UNKNOWN_DIMENSION) hataları ilk denemede göster, düşük öncelikli (row_limit, offset)
  hataları sonraki denemelere bırak

---

## [x] Aşama 6 — Test Coverage

**Dosyalar:** `internal/query/validator_test.go`, `internal/ai/service_test.go`, yeni dosyalar

- [x] **6.1** Structured `ValidationError` unit test'leri:
  - Her error code için doğru `Code`, `Value`, `AllowedAlternatives` üretildiğini doğrula
  - `suggestAlternatives` fonksiyonunun doğru öneriler yaptığını test et

- [x] **6.2** Repair loop entegrasyon test'leri:
  - UNKNOWN_FIELD → repair → başarılı LogicalQuery akışı
  - Çoklu hata → sıralı repair → başarılı
  - Max retry aşıldığında structured error'ların response'a eklenmesi

- [x] **6.3** `BuildRepairPrompt` snapshot test'leri:
  - Farklı error code kombinasyonları için prompt çıktılarını kaydet
  - Regression koruması

- [x] **6.4** Eval golden set'ine repair senaryoları ekle (`internal/ai/eval/`)

---

## [x] Aşama 7 — Observability & Monitoring

**Dosyalar:** `internal/platform/observability/metrics.go`, `internal/http/handlers/ai_telemetry.go`

- [x] **7.1** `AIResponse.Metadata`'ya repair detaylarını ekle:
  ```go
  type RepairDetail struct {
      Attempt        int                    `json:"attempt"`
      ErrorCodes     []string               `json:"error_codes"`
      ErrorsJSON     string                 `json:"errors_json,omitempty"`
      Strategy       string                 `json:"strategy"`
  }
  ```

- [x] **7.2** Telemetry handler'da repair detaylarını logla/kaydet

- [x] **7.3** Prometheus metrikleri:
  - `bi_ai_repair_success_total{error_code}`
  - `bi_ai_repair_attempts_histogram` (attempt sayısı dağılımı)

---

## [x] Aşama 8 — Backward Compatibility & Migration

- [x] **8.1** `ValidationError.Code` alanı `omitempty` — mevcut JSON serialization'ı bozmaz
- [x] **8.2** `BuildRetry` metodu korunur, yeni `BuildRepairPrompt` eklenir — mevcut call site'lar kırılmaz
- [x] **8.3** DB'deki `prompt_templates` tablosuna `repair` template'i eklenebilir (migrasyon gerekmez, runtime insert)
- [x] **8.4** API response formatı değişmez — repair detayları sadece `Metadata` içinde, optional

---

## Dosya Değişiklik Özeti

| Dosya | Değişiklik Türü |
|-------|----------------|
| `pkg/query/types.go` | `ValidationError` struct genişletme |
| `internal/errmsg/messages.go` | Error code sabitleri |
| `internal/query/validator.go` | Structured error üretimi |
| `internal/query/compiler.go` | Structured error üretimi |
| `internal/query/validation_helpers.go` | `suggestAlternatives` fonksiyonu |
| `internal/query/calendar_grain_filter.go` | Error code ekleme |
| `internal/ai/service.go` | Repair loop, `parseAndValidate` genişletme |
| `internal/ai/prompt/prompt.go` | `BuildRepairPrompt` metodu |
| `internal/ai/prompt/prompt_templates.go` | Repair template |
| `internal/platform/observability/metrics.go` | Repair metrikleri |
| `internal/http/handlers/ai_telemetry.go` | Repair telemetri |
| Yeni: `internal/query/repair_suggest_test.go` | Test |
| Yeni: `internal/ai/repair_test.go` | Entegrasyon test |
| Yeni: `internal/ai/eval/repair_cases.go` | Eval golden repair senaryoları (Aşama 6.4) |
| Yeni: `internal/ai/repair_eval_test.go` | Eval repair regresyon kapısı (bad→good recovery) |

> Not: `RepairStrategy(locale, attempt)` helper'ı `internal/ai/prompt/prompt.go`'a eklendi;
> strateji metni hem repair prompt'unda hem de service telemetrisinde tek kaynaktan üretiliyor
> (önceki kopyalanmış switch blokları kaldırıldı).

---

## Öncelik Sırası

1. **Aşama 1-2** (Structured error sistemi) — Temel, her şey buna bağlı
2. **Aşama 3-4** (Repair prompt + loop) — Core feature
3. **Aşama 6** (Testler) — Paralel yapılabilir
4. **Aşama 5** (Context-aware strateji) — Nice-to-have, Aşama 3-4'ten sonra
5. **Aşama 7** (Observability) — Production readiness
6. **Aşama 8** (Compat) — Sürekli dikkat, ayrı aşama değil
