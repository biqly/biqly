# 03 — Backend: Observability ve Performans

**Öncelik: P0** (uncommitted Tier-2 metrik işi devam ediyor; tüm diğer kararların ölçüm temeli)

## Amaç

AI sorgu hattındaki latency kaynaklarını uçtan uca görünür kılmak; "declared ama wired değil"
metrikleri bağlamak; gereksiz LLM/embedding çağrılarını ölçüp azaltmak.

## Mevcut Durum (doğrulandı, 2026-06-10)

Genel tablo: **~%76 tamam.** Core/AI/repair/compile/execute metrikleri ve 10 OTEL span'i
çalışıyor. Sorun, yeni eklenen katmanların kablolamasında.

| Katman | Durum |
| --- | --- |
| Core (query, AI request, step, repair, compile, execute, catalog, model publish) | ✅ Tam — `metrics.go`, Record* çağrıları bağlı |
| Tier 1: HTTP, LLM error/retry, routing, embedding API | ✅ Bağlı (`tier1_metrics.go`; `base_provider.go:66,84,89`, embed: `base_provider.go:110-163`) |
| Tier 1: `biqly_llm_tokens_prompt_total` / `_completion_total` | ❌ **Declared ama Record metodu Inc/Add çağırmıyor** — token split kayboluyor |
| Tier 2 (uncommitted): NATS, memory, ambiguity rounds, enrich, LLM cache | ⚠️ Metrikler + Record* metodları tanımlı; **uygulama kodundan çağrı yok** (keşif call-site bulamadı) |
| Tier 3: semanticgen 4 metrik | ❌ Declared, Record metodu bile yok |
| OTEL spans | ✅ 10 span: ProcessQuestion, TableRoute, RouteEmbedding, PromptBuild, ProviderGenerate, AmbiguityAnalyze, Embed, EmbedMetadata, LLMGenerate, MultiCandidate |
| Span eksikleri | ⚠️ glossary load, memory recall, repair, compile, execute için ayrı span yok (step-histogram var) |
| Sampling | ✅ Yapılandırılabilir: `OTEL_TRACES_SAMPLER(_ARG)`; dev %25 ratio — **prod `always_on` %100 (05'te düzeltilecek)** |
| Grafana | ✅ 5 dashboard helm'de (`biqly-ai`, `query`, `catalog`, `cardinality`, `infrastructure`) |

## Yapılacaklar

### 1. Token split düzeltmesi (en küçük, ilk iş)

1. `tier1_metrics.go` içindeki `RecordLLMProviderTokens(prompt, completion)` gövdesini düzelt:
   `llmTokensPromptTotal.Add(float64(prompt))` + `llmTokensCompletionTotal.Add(float64(completion))`.
   Çağrı zinciri zaten bağlı (`base_provider.go:89` → `llm_metrics.go`), yalnızca gövde boş.
2. Birim test: Record çağrısı sonrası `testutil.ToFloat64` ile her iki counter'ın arttığını doğrula.

### 2. Tier-2 metriklerini uygulama koduna kablola

Her Record* metodu için call-site ekle (gograph_plan ile her hedef sembolü önce planla):

| Record metodu | Bağlanacağı yer |
| --- | --- |
| `RecordNATSPublish/ConsumeErrors/DLQMove/ConsumerPending` | NATS publish/consume/DLQ kodu (`internal/platform/` veya job queue katmanı — `gograph_query` ile bul) |
| `RecordMemoryRecallLatency`, `RecordMemoryRecallMiss` | `internal/ai/memory/recall.go` — embed+sort süresi; boş sonuçta miss |
| `RecordMemoryStoreConfirmedEmbeddingError` | Confirmed query kaydında embedding hatası yolu |
| `RecordAmbiguityClarificationRound` | `ProcessContext` round ilerlemesi (`ai_context.go`) |
| `RecordLLMResponseCacheHit/Miss` | LLM response cache lookup noktası (`BI_AI_RESPONSE_CACHE_TTL` kullanan kod) |
| `RecordEnrichContextSuggestionsGenerated/SuggestLatency/ApplyErrors` | Enrich-context suggest/apply handler'ları |

Not: Keşif raporlarından biri Tier-2'yi "instrumented" diğeri "çağrılmıyor" olarak işaretledi —
uncommitted çalışma akış halinde olduğundan **her satır için call-site'ı tek tek doğrula**;
zaten bağlı olanı atla, eksikleri bağla. (`grep` değil `gograph_callers` kullan.)

### 3. LLM tier yield metriği

`biqly_ambiguity_llm_tier_yield_total{outcome}` — detay 01 §4'te. Bu dosyanın kapsamında
metrik tanımı + kablolama; karar süreci 01'de.

### 4. Eksik span'ler (düşük efor, yüksek teşhis değeri)

`bi_ai_step_duration_seconds` histogramı var ama trace'te kopuk aşamalar şunlar; her birine span:

1. `ai.GlossaryLoad` — glossary yükleme.
2. `ai.MemoryRecall` — recall toplam süresi (embedding alt-span'i zaten `ai.Embed`).
3. `ai.Repair` — repair döngüsü (attempt sayısı attribute olarak).
4. `query.Compile` ve `query.Execute` — compile/execute (metrikleri var, span'i yok;
   uçtan uca trace'te SQL aşaması görünmüyor).

### 5. Semanticgen metrikleri: bağla ya da sil

4 orphan metrik (`semanticgenModelsGenerated`, `Duration`, `Dimensions`, `Metrics`):
semantic model generation çağrı noktası varsa Record metodu yazıp bağla; akış artık
kullanılmıyorsa metrikleri sil (CLAUDE.md: speculative kod bırakma). Önce `gograph_query` ile
semanticgen çağrı yolunu doğrula, sonra karar ver.

### 6. Grafana doğrulaması

1. `biqly-ai.json` dashboard'ına yeni panel(ler): token split, ambiguity rounds histogram,
   recall miss oranı, embedding cache hit-rate (01'den), LLM tier yield.
2. Dev ortamında dashboard'ların gerçek veri çizdiğini doğrula
   (`kubectl -n biqly port-forward` + birkaç AI sorgusu çalıştırıp panelleri kontrol et).

## Kabul Kriterleri

- [x] Token split: `RecordLLMProviderTokens` her iki counter'a Add ediyor; çağrı zinciri
      `base_provider.go:89 → llm_metrics.go → tier1_metrics.go` bağlı; birim test mevcut
      (`TestTier1MetricsRecord`, `TestMetricsRecord` prompt=800/completion=434 doğruluyor).
- [x] Tier-2 metriklerinin tamamının call-site'ı `gograph_callers` ile tek tek kanıtlandı —
      declared-only metrik 0 (detay aşağıda).
- [x] Semanticgen metrikleri orphan değil: `GenerateModelFromMetadata`
      (`internal/semanticgen/generator.go:38`) kaydediyor; test var (`TestMetricsRecordTier3`).
- [x] Span zinciri kod düzeyinde tam: routing → ambiguity → **glossary (yeni)** →
      **recall (yeni)** → prompt → generate → **repair (yeni)** → compile (mevcut) →
      execute (mevcut). ⏳ Jaeger'da canlı doğrulama deploy sonrası yapılacak (aşağıda).
- [x] Grafana `biqly-ai` dashboard'ına 4 ekleme yapıldı; JSON render + parse doğrulandı
      (35 panel). ⏳ Canlı veri çizimi deploy sonrası doğrulanacak.
- [x] `make test-go` (-race, tam suite) + `make lint-go` + `make eval-regression` temiz;
      `gofmt` uygulandı (2026-06-10).

## Uygulama Notları (2026-06-10)

**Ana bulgu: keşif raporundaki "declared-but-not-wired" iddiaları eski graf indeksine
dayanıyormuş.** `gograph build` sonrası tek tek doğrulama, Tier-2'nin TAMAMININ zaten bağlı
olduğunu gösterdi:

| Metrik grubu | Call-site (kanıt) |
| --- | --- |
| NATS publish/consume/DLQ/pending | `internal/queue/nats.go:80,124,135,161` |
| Memory recall miss/latency | `internal/ai/memory/recall.go:35-38` |
| Memory confirmed embed error | `ai_memory.go:89` |
| Ambiguity clarification round / resolution | `ai_telemetry.go:145,154`, `ai.go:227` |
| LLM response cache hit/miss | `service.go:310-313` |
| Enrich suggest/latency/apply-errors | `enrichcontext/suggest.go:58,81`, `ai_enrich_context.go:68` |
| Token split (prompt/completion) | gövde Add'li, `base_provider.go:89` üzerinden bağlı |
| Semanticgen (4 metrik) | `semanticgen/generator.go:38` |

Bu görevden çıkan gerçek işler ve yapılanlar:

1. **3 yeni span**: `ai.GlossaryLoad` (`ai.go loadGlossaryEntries` — katalog/harici sayaç
   attribute'ları), `ai.MemoryRecall` (`ai_memory.go appendConfirmedFewShot` — candidates/hits;
   alt `ai.Embed` span'i ctx üzerinden iç içe geçer), `ai.Repair` (`service.go
   generateWithRetries` retry dalı — attempt attribute'u; `buildPrompt`/`buildNextAttemptPrompt`
   repair ctx'i altında). `query.Compile` ve `query.Execute` span'leri **zaten vardı**
   (`compiler.go:66`, `executor.go:53`) — plan bunu eski rapora dayanarak eksik sanıyordu.
2. **Grafana `biqly-ai`**: "Memory store & recall" paneline `recall misses` hedefi; yeni
   "Önbellek & LLM tier verimi" satırı — Embedding cache hit rate (stat), Ambiguity LLM tier
   yield (outcome bazlı, 01 §4 kararının veri kaynağı), Clarification sonucu
   (resolved/abandoned). Render + JSON parse doğrulandı.
3. Sampling zaten yapılandırılabilir (`OTEL_TRACES_SAMPLER(_ARG)`, dev %25 ratio);
   prod'un `always_on` → ratio'ya çekilmesi **05 §3 kapsamında**.

### ⏳ Deploy-sonrası doğrulama (bu değişiklikler prod/dev'e çıktıktan sonra)

1. Dev'de 20-30 AI sorgusu at (birkaçı bilinçli belirsiz, birkaçı aynı soru tekrarı —
   embedding cache hit'i için).
2. Jaeger: tek sorgunun trace'inde `ai.GlossaryLoad`, `ai.MemoryRecall`, `ai.Repair`
   span'lerinin zincirde göründüğünü kontrol et.
3. Grafana `biqly-ai`: yeni 4 panelin veri çizdiğini kontrol et.
   Not: prod'da serviceMonitor kapalı olduğundan (05 §3.2 çözülene dek) bu doğrulama
   yalnızca dev'de yapılabilir.

## İlgili Dosyalar

- `internal/platform/observability/metrics.go` (⚠️ uncommitted değişiklik var — üstüne çalış),
  `tier1_metrics.go`, `tier2_metrics.go`, `trace.go`, `sampler.go`
- `internal/ai/provider/base_provider.go`, `llm_metrics.go`
- `internal/ai/memory/recall.go`, `internal/http/handlers/ai_context.go`
- `deploy/helm/biqly/templates/grafana-dashboards.yaml`, `servicemonitors.yaml`
