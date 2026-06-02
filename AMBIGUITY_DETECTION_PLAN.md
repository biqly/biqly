# Ambiguity Detection / Clarification Mode — Implementation Plan

> **Amaç:** Kullanıcının doğal dil sorusunda belirsiz terimler/niyetler varsa, sistem
> doğrudan LogicalQuery üretmek yerine kullanıcıya seçenek sunarak netleştirme yapmalı.

## Mevcut Durum (Already Exists)

- [x] Router-source clarification — tablo seçimi belirsiz olduğunda (`routing/router.go`)
- [x] AI/parse-failure clarification — LLM çıktısı parse edilemezse (`service.go:tryGenerateClarification`)
- [x] Validator-source clarification — semantik doğrulama başarısız olursa (`schema.go:Clarification`)
- [x] `ClarificationResponse` + `Clarification` tipleri (`schema.go`)
- [x] `clarification.tmpl` prompt şablonları (EN/TR)
- [x] Multi-candidate self-consistency voting (`service.go:tryMultiCandidate`)

## Eksik Olan — Semantic Ambiguity Detection

Mevcut sistem sadece **hata durumlarında** clarification üretir. Örneğin:
- "aktif müşteri" sorusu başarıyla LogicalQuery'ye çevrilebilir — AMA hangi "aktif" tanımı
  kullanıldığı belirsiz olabilir. Sistem bunu **proaktif olarak** yakalamaz.

---

## Phase 1 — Ambiguity Analyzer Core

### 1.1 Intent + Ambiguity Analyzer Modülü

- [x] `internal/ai/ambiguity/analyzer.go` — yeni paket oluştur
  - `type AmbiguityResult struct` tanımla:
    - `IsAmbiguous bool`
    - `Ambiguities []AmbiguityItem`
    - `ResolvedQuestion string` (kullanıcının seçiminden sonra yeniden yazılmış soru)
  - `type AmbiguityItem struct`:
    - `Term string` — belirsiz terim ("aktif müşteri", "büyük sipariş", "büyük" vb.)
    - `Type string` — `"semantic"` | `"temporal"` | `"scope"` | `"metric"`
    - `Interpretations []Interpretation` — olası anlamlar
  - `type Interpretation struct`:
    - `Label string` — "Son 30 günde sipariş veren müşteri"
    - `Description string` — detay
    - `SemanticMapping` — hangi dimension/metric/filter'a map olur
    - `Confidence float64`

### 1.2 Ambiguity Detection Stratejileri

- [x] **Glossary-based detection** — `internal/ai/ambiguity/glossary_detector.go`
  - Glossary'de birden fazla eşleşme varsa belirsizlik işaretle
  - Örn: "aktif" terimi hem `status = 'active'` hem `last_order_date >= now()-30d` ile eşleşiyor
  - Mevcut `prompt/glossary.go` → `SelectGlossaryForQuestion()` sonucunu kullan

- [x] **Synonym collision detection** — `internal/ai/ambiguity/synonym_detector.go`
  - Birden fazla dimension/metric aynı synonym'a sahipse belirsizlik işaretle
  - Semantic model'deki `Synonyms []string` alanlarını tara
  - Case-insensitive matching + fuzzy matching (Levenshtein)

- [x] **Temporal ambiguity detection** — `internal/ai/ambiguity/temporal_detector.go`
  - "geçen ay", "son zamanlarda", "yakın zamanda" gibi ifadeler net değilse
  - Model'de time-grain dimension yoksa uyarı
  - Locale-aware: TR/EN date expression parsing

- [x] **Metric scope ambiguity** — `internal/ai/ambiguity/scope_detector.go`
  - "büyük", "yüksek", "çok", "az" gibi niteleyiciler
  - Hangi metric'e uygulandığı belirsizse (threshold mu? top-N mi? percentage mi?)
  - Semantic model'deki metric sayısı > 1 ve soruda hangi metric olduğu belirli değilse

### 1.3 Analyzer Orchestrator

- [x] `internal/ai/ambiguity/analyzer.go` — tüm detector'ları koordine et
  - `func Analyze(ctx, question, model, glossary) AmbiguityResult`
  - Her detector sırayla/paralel çalışır
  - Sonuçlar birleşir, duplicate elimination yapılır
  - Confidence threshold'a göre belirsizlik kararı verilir

---

## Phase 2 — LLM-Enhanced Ambiguity Detection

### 2.1 LLM-based Ambiguity Analyzer

- [ ] `internal/ai/ambiguity/llm_analyzer.go` — LLM ile belirsizlik tespiti
  - Soruyu + semantic model bilgisini LLM'e gönder
  - "Bu soruda hangi terimler/niyetler belirsiz?" sorusunu sor
  - Structured JSON output: `{ "ambiguous_terms": [...], "clarification_needed": bool }`
  - Mevcut provider altyapısını kullan (`provider.Provider` interface)

### 2.2 Ambiguity Prompt Template

- [ ] `internal/ai/prompt/prompts/en/ambiguity.tmpl` — İngilizce şablon
  ```
  Analyze this question for semantic ambiguity given the model:
  Question: {{.Question}}
  Model: {{.ModelName}}
  Dimensions: {{.Dimensions}}
  Metrics: {{.Metrics}}
  Glossary: {{.Glossary}}

  Return JSON:
  {
    "is_ambiguous": bool,
    "ambiguities": [{
      "term": "...",
      "possible_meanings": ["...", "..."],
      "recommended_clarification": "..."
    }]
  }
  ```

- [ ] `internal/ai/prompt/prompts/tr/ambiguity.tmpl` — Türkçe şablon

---

## Phase 3 — Pipeline Integration

### 3.1 Pipeline'a Ambiguity Analyzer Ekleme

Mevcut pipeline:
```
NL Question → Table Router → Prompt Builder → LLM → LogicalQuery → Validator → Compiler
```

Yeni pipeline:
```
NL Question → Table Router → [Ambiguity Analyzer] → Prompt Builder → LLM → LogicalQuery → Validator → Compiler
                                    ↓ (if ambiguous)
                              Clarification Response (options)
```

- [x] `internal/ai/service.go` — `ProcessQuestion` fonksiyonunu güncelle
  - Table routing sonrası, ambiguity analysis ekle
  - `AmbiguityResult.IsAmbiguous == true` ise clarification response dön
  - Source: `"ambiguity_analyzer"`

### 3.2 Service Layer Değişiklikleri

- [x] `internal/ai/service.go` — yeni `ProcessOption` ekle
  - `WithAmbiguityCheck(enabled bool) ProcessOption` — açma/kapama
  - Default: kapalı (opt-in), ileride opt-out yapılabilir

- [x] `internal/ai/schema.go` — `Clarification` struct'ını genişlet
  - `Source` alanına `"ambiguity_analyzer"` değerini ekle
  - `AmbiguityDetail *AmbiguityDetail` alanı ekle (optional)
  - `type AmbiguityDetail struct { Ambiguities []AmbiguityItem }`

### 3.3 Clarification Continuation (Kullanıcı Yanıt Sonrası)

- [x] `internal/ai/ambiguity/resolver.go` — kullanıcının seçimini işle
  - Kullanıcı bir interpretation seçtiğinde, orijinal soruyu yeniden yaz
  - Örn: "aktif müşterileri göster" → "son 30 günde sipariş veren müşterileri göster"
  - Yeniden yazılan soru normal pipeline'dan geçer

- [x] `internal/http/handlers/ai.go` — handler'da clarification continuation desteği
  - Request'e `ClarificationChoice string` alanı ekle
  - Eğer clarification choice varsa, resolver'ı çağır, soruyu yeniden yaz, pipeline'a devam et

---

## Phase 4 — Configuration & Admin UI

### 4.1 Runtime Configuration

- [x] `internal/config/config.go` — config struct'ına ambiguity ayarları ekle
  - `AmbiguityCheckEnabled bool` — açma/kapama
  - `AmbiguityConfidenceThreshold float64` — hangi confidence altında clarification gösterilsin
  - `AmbiguityMaxOptions int` — maksimum kaç seçenek gösterilsin
  - `AmbiguityLLMEnabled bool` — LLM-based detection açma/kapama (maliyet kontrolü)

### 4.2 DB Migration

- [ ] `migrations/` — yeni migration
  - `ai_settings` tablosuna ambiguity ile ilgili kolonlar
  - veya mevcut settings tablosuna JSON column ekle

### 4.3 Frontend — Clarification UI

- [x] `frontend/` — clarification bileşeni oluştur
  - Kullanıcıya soru + seçenekler göster
  - Seçim sonrası `clarification_choice` ile otomatik re-submit

- [x] Mevcut AI query sonuç bileşenini güncelle
  - `ClarificationResponse` handling ekle
  - `source === "ambiguity_analyzer"` için özel UI

---

## Phase 5 — Testing & Evaluation

### 5.1 Unit Tests

- [ ] `internal/ai/ambiguity/analyzer_test.go`
  - Belirsiz sorular: "aktif müşteriler", "büyük siparişler", "son zamanlarda"
  - Belirli sorular: "son 30 günde sipariş veren müşteriler" → ambiguous olmamalı
  - Edge cases: boş soru, çok kısa soru, tam eşleşme

- [x] `internal/ai/ambiguity/glossary_detector_test.go`
  - Glossary multi-match senaryoları
  - Glossary boşken behavior

- [x] `internal/ai/ambiguity/synonym_detector_test.go`
  - Synonym collision senaryoları

- [ ] `internal/ai/ambiguity/temporal_detector_test.go`
  - Locale-aware TR/EN date ambiguity

- [ ] `internal/ai/ambiguity/llm_analyzer_test.go`
  - Mock provider ile test

### 5.2 Integration Tests

- [x] `internal/ai/service_test.go` — ambiguity integration
  - Ambiguity açıkken belirsiz soru → clarification response
  - Ambiguity kapalıken belirsiz soru → normal LogicalQuery
  - Clarification choice sonrası başarılı LogicalQuery

### 5.3 Golden/Eval Cases

- [ ] `internal/ai/eval/` — ambiguity eval cases ekle
  - Belirsiz sorular için expected clarification output
  - Eval runner'da `EvalModeClarification` modu ekle

---

## Phase 6 — Performance & Optimization

### 6.1 Performance Guardrails

- [ ] Ambiguity analysis timeout (max 2s for rule-based, max 5s for LLM-based)
- [ ] Ambiguity analysis'i cache'leme (aynı soru + model → aynı ambiguity result)
- [ ] Parallel execution: rule-based detectors paralel çalışsın
- [ ] LLM call maliyeti: sadece rule-based ambiguous değilse ve LLM flag açıkssa çağır

### 6.2 Metrics & Observability

- [ ] Prometheus metrics ekle
  - `biqly_ambiguity_detected_total` — kaç soruda ambiguity tespit edildi
  - `biqly_ambiguity_clarified_total` — kaç clarification kullanıcı tarafından yanıtlandı
  - `biqly_ambiguity_latency_ms` — analysis süresi
  - `biqly_ambiguity_by_source` — rule-based vs LLM-based

---

## Implementation Priority (Suggested Order)

| Sıra | Task | Tahmini Süre | Bağımlılık |
|------|------|-------------|------------|
| 1    | Phase 1.1 — Analyzer types + struct | 2-3 saat | - |
| 2    | Phase 1.2 — Glossary detector | 3-4 saat | 1 |
| 3    | Phase 1.2 — Synonym detector | 2-3 saat | 1 |
| 4    | Phase 3.1 — Pipeline integration (rule-based only) | 3-4 saat | 1,2,3 |
| 5    | Phase 3.3 — Clarification continuation | 3-4 saat | 4 |
| 6    | Phase 5.1 — Unit tests | 3-4 saat | 4 |
| 7    | Phase 4.1 — Config | 1-2 saat | 4 |
| 8    | Phase 4.3 — Frontend UI | 4-6 saat | 5 |
| 9    | Phase 1.2 — Temporal detector | 2-3 saat | 1 |
| 10   | Phase 1.2 — Scope detector | 2-3 saat | 1 |
| 11   | Phase 2 — LLM-based analyzer | 4-6 saat | 4 |
| 12   | Phase 5.3 — Eval cases | 2-3 saat | 6 |
| 13   | Phase 6 — Performance + metrics | 3-4 saat | all |

**Toplam tahmini süre:** ~35-45 saat

---

## Dosya Değişiklik Özeti

### Yeni Dosyalar
```
internal/ai/ambiguity/
├── analyzer.go              # Ana orchestrator + tipler
├── analyzer_test.go
├── glossary_detector.go     # Glossary-based detection
├── glossary_detector_test.go
├── synonym_detector.go      # Synonym collision detection
├── synonym_detector_test.go
├── temporal_detector.go     # Temporal ambiguity
├── temporal_detector_test.go
├── scope_detector.go        # Metric scope ambiguity
├── scope_detector_test.go
├── llm_analyzer.go          # LLM-enhanced detection
├── llm_analyzer_test.go
└── resolver.go              # Clarification choice → rewritten question

internal/ai/prompt/prompts/en/ambiguity.tmpl
internal/ai/prompt/prompts/tr/ambiguity.tmpl
```

### Değişecek Dosyalar
```
internal/ai/service.go       # Ambiguity check ekleme
internal/ai/schema.go        # AmbiguityDetail tipi, source genişletme
internal/ai/prompt/prompt.go # BuildAmbiguity metodu
internal/config/ai.go        # Ambiguity config alanları
internal/http/handlers/ai.go # Clarification continuation handler
frontend/src/                # Clarification UI bileşenleri
```
