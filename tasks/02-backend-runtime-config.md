# 02 — Backend: Konfigürasyon ve DB-backed Runtime Ayarlar

**Öncelik: P1** (frontend ayar panelinin ön koşulu; altyapı büyük ölçüde hazır)

## Amaç

Helm/env üzerinden gelen AI/PII ayarlarının düşük-riskli alt kümesini DB üzerinden runtime'da
yönetilebilir hale getirmek; admin API'yi RBAC'a bağlamak; değişiklikleri audit'lemek.

## Mevcut Durum (doğrulandı, 2026-06-10)

| Bileşen | Durum | Konum |
| --- | --- | --- |
| `ai_runtime_config` tablosu (`key TEXT PK, value JSONB, updated_at`) | ✅ **Zaten var** — migration 045a | `migrations/045a_add_ai_runtime_config.up.sql` |
| `GET/PUT /api/ai/admin/config` | ✅ Var — ambiguity override'ları okuyor/yazıyor | `internal/http/handlers/ai_admin_config.go` |
| `effectiveAmbiguityConfig(ctx)` (DB overlay → env fallback) | ✅ Var — 30s TTL cache'li | `ai_admin_config.go:42-88` |
| DB'den yönetilebilen anahtarlar | ⚠️ Yalnızca 2: `tiered_enabled`, `max_llm_tier_per_question` (0–10 validasyonlu) | `ai_admin_config.go:146` |
| `AdminKeyMiddleware` | ⚠️ JWT super_admin VEYA paylaşılan `BI_ADMIN_API_KEY`; RBAC izni (`ai:settings`) **yok** | `internal/http/handlers/admin_middleware.go:16-39` |
| Config değişikliği audit log | ❌ Yok | — |
| Yeniden kullanılabilir DB-store kalıbı (TTL cache + invalidation + admin API + embedded fallback) | ✅ `ai_nl_lexicon` ile kanıtlanmış | `internal/ai/lexicon/store.go`, `ai_admin_lexicon.go` |

> **Karar (00 §Karar-2):** Tablo şeması değişmeyecek. Kullanıcı isteğindeki
> `locale, domain, key, value` şeması locale-boyutlu veri içindi; o ihtiyacı `ai_nl_lexicon`
> zaten karşılıyor. Runtime config dil-bağımsız platform ayarı tutar; `key/value JSONB` yeterli.

## Yapılacaklar

### 1. Runtime config kapsamını genişlet (yeni anahtarlar)

`ai_runtime_config` tablosuna yeni key'ler (her biri JSONB value, mevcut `ambiguity` kalıbı gibi):

| Key | Alanlar | Validasyon | Env fallback |
| --- | --- | --- | --- |
| `ambiguity` (mevcut) | `tiered_enabled`, `max_llm_tier_per_question` | bool; 0–10 | `BI_AI_AMBIGUITY_*` |
| `ambiguity` (ek alanlar) | `check_enabled`, `confidence_threshold`, `max_options` | bool; 0.0–1.0; 1–10 | `BI_AI_AMBIGUITY_*` |
| `pii` (yeni) | `detection_threshold`, `default_masking_strategy` | 0.0–1.0; enum partial/full/none | `BI_PII_*` |
| `memory` (yeni) | `recall_enabled`, `recall_limit` | bool; 1–10 | (yeni env: `BI_AI_MEMORY_RECALL_ENABLED`, default true) |

Adımlar:

1. `ai_admin_config.go` içindeki `ambiguityOverrides` kalıbını genelle: her config domain'i için
   `overrides struct + load + effective + validate` üçlüsü. Tek generic `runtimeOverrides[T]`
   helper'ı yaz (Go generics) — kopyala-yapıştır üç struct yerine.
2. `effectivePIIConfig(ctx)` ve `effectiveMemoryConfig(ctx)` ekle; çağıran yerleri bul
   (`gograph_query` ile `config.PII`, memory recall çağrı noktaları) ve effective-fonksiyona geçir.
   **Dikkat:** `BI_PII_ENABLED` master kill-switch'i env-only kalır (UI'dan PII tamamen
   kapatılamaz — güvenlik ayarı); UI yalnızca threshold ve masking stratejisini yönetir.
3. GET `/api/ai/admin/config` yanıtını genişlet: tüm domain'leri, her alanın `source`
   bilgisiyle (`db_override` | `env` | `default`) döndür — UI "bu değer nereden geliyor"
   gösterebilsin (PlatformSettingsPanel'de bu kalıp zaten başladı).
4. PUT validasyonları: bilinmeyen alan → 400; aralık dışı → 400 + alan-bazlı hata mesajı.
5. Cache invalidation: PUT sonrası ilgili domain cache'ini düşür (lexicon'daki
   `invalidateLexiconCaches` kalıbı, 30s TTL ile diğer replikalar yakınsar).

### 2. Audit logging

1. Mevcut audit altyapısını tespit et (Denetim Günlüğü paneli var → backend audit store mevcut).
2. PUT `/api/ai/admin/config` başarılı olduğunda audit kaydı: actor (JWT sub/email veya
   "admin-key"), domain, eski değer → yeni değer (JSON diff), timestamp.
3. Test: PUT sonrası audit satırının düştüğünü doğrula.

### 3. RBAC enforcement (00 §Karar-3)

1. `AdminKeyMiddleware`'i genişlet: JWT'de `ai:settings` (veya `admin:settings`) izni varsa da
   geçir. Sıralama: super_admin → RBAC izni → paylaşılan anahtar (yalnızca makine-makine).
2. Frontend zaten `hasPermission('admin:settings')` ile gate ediyor — backend ile simetri kur.
3. Test: izinli JWT geçer, izinsiz JWT 403, geçerli admin-key geçer.

### 4. Helm defaults hizalaması

1. `deploy/helm/biqly/charts/ai/templates/configmap.yaml` zaten `ambiguity.*` değerlerini env'e
   çeviriyor — yeni `BI_AI_MEMORY_RECALL_ENABLED` için aynı kalıbı ekle.
2. values.yaml yorumlarına "bu anahtar DB'den override edilebilir; env yalnızca fallback" notu
   düş (06 numaralı görevdeki tabloya link).
3. **Drift uyarısı:** values.yaml'da `ambiguity.tieredEnabled: true`, kodda env default `false`
   (`config.go:259`). Bu bilinçli (helm açar) ama 06'daki tabloda açıkça belgelenecek.

## Kabul Kriterleri

- [x] GET `/api/ai/admin/config` üç domain'i (`ambiguity`, `pii`, `memory`) alan-bazlı `sources`
      haritasıyla dönüyor (`database` | `environment`).
- [x] PUT ile yazılan değer pod restart olmadan etkili: yazan replika anında invalidate,
      diğerleri ≤30 sn TTL (`TestRuntimeOverridesRefreshAfterExpiryAndInvalidate` +
      `TestUpdateAdminRuntimeConfigPersistsAndReloads`).
- [x] Aralık dışı / bilinmeyen alan → 400 + alan-bazlı mesaj (strict decode,
      `TestUpdateAdminRuntimeConfigValidation` 9 senaryo).
- [x] Config değişiklikleri audit'leniyor: `ai.config_updated` event'i, actor + domain başına
      old/new JSON diff (`audit.EventAIConfigUpdated`).
- [x] `ai:settings` RBAC izinli JWT admin endpoint'lerine erişiyor; izinsiz 403, anonim 401
      (`AdminAccessMiddleware`, auth-service `CheckPermission` cache'li).
- [x] `make test-go` + `make lint-go` + `make eval-regression` temiz; `gograph_review` temiz;
      `helm lint` temiz (2026-06-10).

## Uygulama Notları (2026-06-10)

- **Generic store**: `runtimeOverrides[T]` (`internal/http/handlers/runtime_overrides.go`) —
  TTL cache + `invalidate` + cache'siz `fetchRuntimeOverrides` (read-through). Ambiguity ve
  memory AIHandler üzerinde cache'li (sıcak yol); **PII read-through** — scan nadir, ayrıca
  catalog servisi AI servisindeki PUT invalidation'ını göremez (cross-service), TTL bile
  gerekmeden her scan taze okur.
- **PII kapsam sapması**: yalnızca `detection_threshold` taşındı.
  `default_masking_strategy` **bilinçli olarak eklenmedi** — `cfg.PII.DefaultMaskingStrategy`
  kodda hiçbir yerde tüketilmiyor (maskeleme stratejileri kolon-bazlı DB'de; boş değer
  `partial`'a normalize oluyor). Ölü knob eklemek yerine 06'daki tabloda "unused" olarak
  belgelenecek. `BI_PII_ENABLED` plandaki gibi env-only kill switch (UI'da salt-okunur döner).
- **Source ayrımı**: `env` vs `default` çalışma zamanında ayırt edilemez (loader yüklendikten
  sonra bilgi kaybolur) → wire'da `database` | `environment` (environment = env-veya-default).
- **PUT semantiği**: gönderilen domain satırı **tamamen değiştirir**; domain içinde gönderilmeyen
  alan env'e düşer; boş obje (`{"ambiguity":{}}`) tüm override'ları temizler (reset mekanizması).
  Bilinmeyen alan strict decode ile 400. Eski frontend gövdesi (`tiered_enabled` +
  `max_llm_tier_per_question`) değişiklik gerektirmeden çalışır.
- **Ambiguity ek alanları**: `check_enabled`, `confidence_threshold` (0–1), `max_options` (1–10)
  artık DB-override'lı; `effectiveAmbiguityConfig` beşini de overlay'liyor.
- **Memory**: yeni `BI_AI_MEMORY_RECALL_ENABLED` (true) + `BI_AI_MEMORY_RECALL_LIMIT` (5) env
  default'ları (`config.AIMemoryConfig`); `appendConfirmedFewShot` effective config'e bağlı —
  `recall_enabled=false` recall'u tamamen kapatır, `recall_limit` few-shot tavanıyla `min`lenir.
- **RBAC**: `AdminAccessMiddleware(adminKey, authClient, "ai:settings")` tüm AI admin grubunu
  (providers, eval, AB, enrich, lexicon, config) koruyor. Sıra: super_admin → paylaşılan anahtar
  (M2M, auth-service çağrısı yapılmadan) → RBAC izni. `AdminKeyMiddleware` ince sarmalayıcı
  olarak korundu. Frontend'in admin-key fallback'inin kaldırılması 05 §1'de.
- **Helm**: `charts/ai/values.yaml`'a `memory.recallEnabled/recallLimit` + "DB override eder"
  yorumu; configmap'e `BI_AI_MEMORY_RECALL_*` render bloğu eklendi (`helm template` ile doğrulandı).

## İlgili Dosyalar

- `internal/http/handlers/ai_admin_config.go`, `admin_middleware.go`
- `internal/config/config.go` (PII: 444-450, AI: 454-511)
- `internal/ai/lexicon/store.go` (kalıp referansı)
- `migrations/045a_add_ai_runtime_config.up.sql` (mevcut — değişmeyecek)
- `deploy/helm/biqly/charts/ai/templates/configmap.yaml`, `values*.yaml`
