# 01 — Backend: Ambiguity ve Clarification İyileştirmeleri

**Öncelik: P2** (mimari büyük ölçüde tamamlanmış; kalanlar iyileştirme + test boşlukları)

## Amaç

Sync ve async iş akışlarının tek bir `ProcessContext` üzerinden çalıştığını garanti altına almak,
gereksiz netleştirme döngülerini ve gereksiz embedding/LLM çağrılarını ortadan kaldırmak,
temporal post-check'lerin doğru tetiklendiğini test güvencesine bağlamak.

## Mevcut Durum (doğrulandı, 2026-06-10)

| Bileşen | Durum | Konum |
| --- | --- | --- |
| `ProcessContext` + `Resolve()` tek giriş noktası | ✅ Tamam — sync/async parity testli | `internal/http/handlers/ai_context.go:16-75` |
| Clarification dışında kalmış serbest fonksiyon | ✅ Yok — yalnızca analiz fonksiyonları (`ambiguity.Analyze` vb.) bağımsız | — |
| `maxClarificationRounds = 2` hard cap | ✅ Sabit, 3 kontrol noktasında enforce ediliyor | `ai_context.go:14, 77-130` |
| Tiered ambiguity (Tier 0 routing → Tier 1 deterministik → Tier 2 LLM → Tier 3 interactive) | ✅ Tamam, config kapılı | `ai_ambiguity_tier.go:30-72`, `internal/ai/service.go:376-434` |
| `DetectTemporal` (deterministik, lexicon-driven, confidence=1.0) | ✅ Tamam | `internal/ai/ambiguity/temporal_detector.go:29-50` |
| Temporal post-check (uyarı + confidence cap 0.5) | ✅ Tüm yanıt yollarında çağrılıyor | `internal/ai/temporal_postcheck.go:22-38`, `service.go:324,334,339` |
| Memory recall (cosine similarity, stored embedding'ler ön-hesaplı) | ✅ Tamam | `internal/ai/memory/recall.go:22-69` |
| Gelen soru için transient embedding cache | ❌ **Yok** — her recall'da soru yeniden embed ediliyor | — |
| `recall_miss` metriği | ✅ Tanımlı (`biqly_memory_recall_misses_total`) — call-site bağlantısı 03'te doğrulanacak | `tier2_metrics.go:47-49` |

## Yapılacaklar

### 1. Transient embedding cache (asıl iş)

Aynı soru çok-turlu clarification oturumunda her turda yeniden embed ediliyor. Kısa TTL'li
hash-cache ile tekrar embedding çağrısı engellenecek.

1. `internal/ai/memory/` altına `embed_cache.go` ekle:
   - Key: `sha256(question + datasourceID + embeddingModel)` — model değişince cache geçersiz olmalı.
   - Store: önce **in-memory LRU + TTL** (5 dk, maks ~512 entry). Redis (Dragonfly,
     `BI_REDIS_DSN`) ancak çok-replika AI pod'unda hit-rate düşük kalırsa ikinci faz olarak
     değerlendirilecek — ölçmeden dağıtık cache ekleme (CLAUDE.md: sync.Pool kuralıyla aynı ilke).
2. `RecallFewShot` (`recall.go:22-69`) içinde `embedder.Embed(...)` çağrısını cache-aware sarmala.
3. Metrik ekle: `biqly_embedding_cache_hits_total` / `biqly_embedding_cache_misses_total`
   (03 numaralı görevdeki metrik dosya düzenine uygun şekilde `tier2_metrics.go`'ya).
4. Birim test: aynı (soru, datasource, model) ikinci çağrıda `embedder.Embed`'in **çağrılmadığını**
   mock embedder sayaç ile kanıtla; TTL dolunca yeniden çağrıldığını kanıtla; farklı model →
   cache miss kanıtla.

### 2. Test boşluklarını kapat

Keşifte tespit edilen eksik testler (kod var, test yok):

1. **Temporal post-check confidence cap testi**: `temporal_postcheck_test.go` içine —
   temporal phrase var + tarih koşulu yok → uyarı eklenir VE `Confidence ≤ 0.5`;
   temporal phrase var + tarih koşulu var → uyarı yok, confidence dokunulmaz.
2. **Çok-turlu oturum e2e testi**: round 0 → clarification → round 1 → clarification →
   round 2'de Tier 3 interactive'e geçiş → round 3'te tüm kontroller bypass. Hem sync handler
   hem async job yolunda aynı senaryo (mevcut `TestProcessContextSyncAsyncIdenticalBehavior`
   kalıbını genişlet: `ai_ambiguity_test.go:75-121`).
3. **`RecallFewShot` hash-dedup testi**: aday listesinde sorunun birebir aynısı varsa
   örnek olarak dönmediğini doğrula (`recall.go:41-49` mantığı test edilmemiş).
4. **Tier 3 interactive tam HTTP akış testi**: `ai_ambiguity_tier_test.go` opsiyon kurulumunu
   test ediyor; cap'e ulaşan gerçek bir HTTP isteğinin interactive clarification response
   döndürdüğü uçtan uca test eksik.

### 3. Sync/async parity'yi kalıcı koru (regresyon kalkanı)

1. `ai_context.go` başına paket yorumu: "clarification çözümü yalnızca `ProcessContext.Resolve`
   üzerinden yapılır; yeni giriş noktası eklemeyin" — mimari sözleşmeyi koda yaz.
2. Eval golden suite'e (`internal/ai/eval/ambiguity_*.go`) round-cap senaryosu ekle ki
   `make eval-regression` parity bozulmasını yakalasın.

### 4. LLM tier yield metriği (karar verisi — 00 §Karar-1)

"Deterministik boş dönerse LLM'i atla" kararını veriye bağlamak için:

1. `analyzeAmbiguity` (`service.go:376-434`) içinde Tier 2 LLM çağrısı sonrası kaydet:
   `biqly_ambiguity_llm_tier_yield_total{outcome="found|empty|error|timeout"}`.
2. 2–4 hafta prod verisi sonrası: `empty` oranı > %95 ise
   `BI_AI_AMBIGUITY_LLM_SKIP_WHEN_TIER1_EMPTY` bayrağı ekle (default false) ve runtime
   config'e taşı (02 numaralı görev).

## Kabul Kriterleri

- [x] Aynı soru için ikinci recall'da embedding API çağrısı yapılmıyor (mock sayaç testi yeşil — `TestRecallFewShotUsesEmbedCache`).
- [x] Embedding cache hit/miss metrikleri kayıtlı (`biqly_embedding_cache_{hits,misses}_total`, metrik testi yeşil).
- [x] Temporal post-check confidence-cap birim testi yeşil (zaten mevcuttu: `TestApplyTemporalFilterPostCheck_*`).
- [x] Çok-turlu (0→1→2→Tier3→bypass) ilerleme testi yeşil (`TestAmbiguityMultiRoundProgression`; sync/async aynı `buildProcessContext`+predikatları paylaşır, parity ayrıca testli).
- [x] `biqly_ambiguity_llm_tier_yield_total{outcome=found|empty|timeout|error}` `analyzeAmbiguity` LLM dalında kaydediliyor.
- [x] `make test-go` + `make eval-regression` + `make lint-go` temiz (2026-06-10).
- [x] `gograph_review --uncommitted` raporu temiz (risk: public_api=Metrics record metodları — beklenen).

## Uygulama Notları (2026-06-10)

- Cache anahtarı **model + soru** (datasourceID dahil edilmedi — embedding çıktısı yalnızca
  metin+modele bağlı; datasource eklemek hit-rate'i sebepsiz düşürürdü).
- Cache: in-memory TTL'li LRU (`container/list`), 5 dk / 512 entry, `internal/ai/memory/embed_cache.go`.
- §2.3 hash-dedup testi ve §2.1 temporal cap testleri zaten mevcuttu — yeni iş çıkmadı.
- §3.2 eval golden round-cap senaryosu **eklenmedi**: golden harness soru-bazlı detection
  modeller, tur kavramı yok; round-cap güvencesi handler birim testlerinde
  (`TestAmbiguityHardCapStopsAfterMaxRounds` + `TestAmbiguityMultiRoundProgression`).
- §2.4 Tier-3 tam HTTP e2e **ertelendi**: `parseAndRouteAIQuery` için full-stack test harness'i
  yok; kurmak bu işin kapsamını aşıyor. Mevcut katmanlı kapsam: predikatlar +
  `ambiguityProcessOptions` + tier-0 handler testi + service `analyzeAmbiguity`.

## İlgili Dosyalar

- `internal/http/handlers/ai_context.go`, `ai_ambiguity_tier.go`, `ai_ambiguity_test.go`
- `internal/ai/service.go`, `internal/ai/temporal_postcheck.go`
- `internal/ai/ambiguity/` (analyzer, temporal_detector, synonym_detector, llm_analyzer)
- `internal/ai/memory/recall.go` (+ yeni `embed_cache.go`)
- `internal/platform/observability/tier2_metrics.go`
