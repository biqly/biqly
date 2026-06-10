# 06 — Konfigürasyon Tablosu ve Dokümantasyon

**Öncelik: P2** (02 ve 05 netleştikten sonra tek seferde doğru yazılır)

## Amaç

Tüm AI/PII konfigürasyonlarını kaynak, default, kullanım yeri ve UI-yönetilebilirlik
bilgisiyle tek tabloda belgelemek: `docs/configuration.md`.

## Mevcut Durum (doğrulandı)

- `docs/pii-detection-masking.md` var; merkezi config referansı **yok**.
- Tüm env okumaları tek yerde: `internal/config/config.go` (PII: 444-450, AI: 454-511) —
  tablo buradan üretilebilir.
- Bilinen drift örneği: `ambiguity.tieredEnabled` helm'de `true`, kod default'u `false` —
  tablo tam da bu tip farkları görünür kılacak.

## Yapılacaklar

### 1. `docs/configuration.md` oluştur

Sütunlar: **Key | Default (kod) | Helm Source | Runtime Override? | Used In | UI Editable? | Notes**

İçerik blokları:

1. **AI — bağlantı/limit** (env-only, UI No): `BI_AI_HTTP_TIMEOUT_SECONDS` (300),
   `BI_AI_RATE_LIMIT_PER_MINUTE` (20), `BI_AI_MAX_PROMPT_RUNES` (80000; helm 40000!),
   `BI_AI_MAX_RETRIES` (2; helm 1!), `BI_AI_MULTI_CANDIDATE_COUNT`, `BI_AI_RESPONSE_CACHE_TTL`,
   `BI_AI_JOBS_ENABLED`, describe/translation/embedding timeout ve weight'leri,
   `BI_AI_ROUTE_*` (6 anahtar), `BI_AI_QUERY_*` (provider override).
2. **AI — ambiguity** (kısmen runtime-override, kısmen UI Yes):
   `BI_AI_AMBIGUITY_CHECK_ENABLED` (true), `_CONFIDENCE_THRESHOLD` (0.70), `_MAX_OPTIONS` (5),
   `_LLM_ENABLED` (false), `_TIERED_ENABLED` (false; helm true!),
   `_MAX_LLM_TIER_PER_QUESTION` (1). Runtime Override sütununda `ai_runtime_config.ambiguity`
   alanlarını işaretle.
3. **PII**: `BI_PII_ENABLED` (true; UI **No** — master switch), `BI_PII_DETECTION_THRESHOLD`
   (0.6; UI Yes), `BI_PII_SAMPLE_DATA_LIMIT` (50; No), `BI_PII_AUTO_SCAN_ON_SYNC` (true; No),
   `BI_PII_DEFAULT_MASKING_STRATEGY` (partial; UI Yes).
4. **Memory** (02 ile eklenecek): `BI_AI_MEMORY_RECALL_ENABLED` vb.
5. **Not satırları**: AI provider/model/API key seçiminin env'de DEĞİL DB'de
   (`ai_providers`/`ai_models`, Administration → AI Providers) olduğu; `ai_nl_lexicon` ve
   i18n runtime tablolarının ayrı admin API'lerle yönetildiği.

"Used In" sütunu için her anahtarı `gograph_query` ile tüketen fonksiyona kadar izle
(ör. `RateLimitPerMinute` → middleware adı; `MaxPromptInputRunes` → prompt builder fonksiyonu).
Sadece `config.go` satırı yazmak yetmez — gerçek tüketim noktası yazılacak.

### 2. Kod-doc senkronu

1. Drift'leri tabloda "Notes" ile açıkla (helm bilinçli override mi, unutulmuş fark mı —
   her birini tek tek kararlaştır; `BI_AI_MAX_RETRIES` 2 vs 1 gibi farklar bilinçliyse gerekçesini yaz).
2. `values.yaml` içine ve `CLAUDE.md` deployment bölümüne `docs/configuration.md` linki ekle.
3. PlatformSettingsPanel'e (04) "Tüm anahtarların referansı" linki — panel başlığı altına.

### 3. Güncel tutma mekanizması

1. `tasks/lessons.md`'ye kural: "yeni BI_* env anahtarı ekleyen PR, docs/configuration.md
   satırı da eklemek zorunda".
2. (Opsiyonel, ucuz) CI guard: `config.go`'daki `getEnv*("BI_` çağrılarından anahtar listesi
   çıkarıp `docs/configuration.md`'de geçmeyenleri raporlayan küçük bir test
   (`internal/config/config_doc_test.go`) — drift'i derlemede yakalar.

## Kabul Kriterleri

- [ ] `docs/configuration.md` tüm `BI_AI_*` + `BI_PII_*` + memory anahtarlarını kapsıyor
      (config.go ile birebir; doc-test yeşil).
- [ ] Her anahtarın gerçek tüketim noktası (dosya/fonksiyon) "Used In"de.
- [ ] UI Editable = Yes işaretli anahtarlar 04'teki panelle birebir örtüşüyor.
- [ ] Helm/kod default farkları Notes'ta açıklanmış.

## İlgili Dosyalar

- `internal/config/config.go`, `internal/http/handlers/ai_admin_config.go`
- `deploy/helm/biqly/values*.yaml`, `charts/ai/templates/configmap.yaml`
- `docs/configuration.md` (yeni), `docs/pii-detection-masking.md` (referans)
