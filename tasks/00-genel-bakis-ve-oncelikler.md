# 00 — Genel Bakış ve Öncelik Sırası

> Hedef: "AI-powered BI platform — AI generates **LogicalQuery JSON, never raw SQL**" vizyonunu
> destekleyen ambiguity/clarification, runtime config, observability, admin UI, devops ve
> dokümantasyon işlerinin uzun vadeli, kod incelemesine dayalı yol haritası.
>
> Bu dosyalar 2026-06-10 tarihinde kodun gerçek durumu incelenerek yazıldı. Her dosyada
> "Mevcut Durum (doğrulandı)" bölümü vardır — plan, varsayım değil tespit üzerine kuruludur.

## Öncelik Sırası (uygulama sırası)

| Sıra | Dosya | Öncelik | Neden bu sıra |
| --- | --- | --- | --- |
| 1 | [03-backend-observability-performans.md](03-backend-observability-performans.md) | **P0** | Uncommitted Tier-2 metrik işi zaten devam ediyor; declared-but-not-wired metrikler ve token split düzeltmesi küçük ama tüm diğer işlerin ölçümünü mümkün kılıyor. Önce ölç, sonra optimize et. |
| 2 | [05-devops-helm-config.md](05-devops-helm-config.md) §1 (güvenlik) | **P0** | `frontend.runtimeAdminKey` admin anahtarını tarayıcıya sızdırıyor (`window.__BIQLY_ENV__.adminApiKey`). Güvenlik açığı — diğer admin-API işlerinden önce kapatılmalı. |
| 3 | [02-backend-runtime-config.md](02-backend-runtime-config.md) | **P1** | `ai_runtime_config` altyapısı var; genişletme (yeni anahtarlar, RBAC, audit) frontend panelinin ön koşulu. |
| 4 | [04-frontend-admin-ayar-paneli.md](04-frontend-admin-ayar-paneli.md) | **P1** | 02'ye bağımlı. Panel genişletme + i18n/a11y boşlukları. |
| 5 | [01-backend-ambiguity-clarification.md](01-backend-ambiguity-clarification.md) | **P2** | Mimari büyük ölçüde tamam; kalanlar iyileştirme (embedding cache, LLM-tier yield metriği, test boşlukları). |
| 6 | [05-devops-helm-config.md](05-devops-helm-config.md) §2-4 | **P2** | Prod tracing sampling, prod'da prometheusRules/serviceMonitor, seed stratejisi. |
| 7 | [06-konfigurasyon-tablosu-dokumantasyon.md](06-konfigurasyon-tablosu-dokumantasyon.md) | **P2** | 02 ve 05 netleşince tablo tek seferde doğru yazılır. |
| 8 | [07-entegrasyon-ve-test-plani.md](07-entegrasyon-ve-test-plani.md) | **Sürekli** | Her iş paketinin kabul kapısı; ayrıca bağımsız e2e/regresyon maddeleri içerir. |

## Kilit Kararlar (karar verici notları)

1. **"Deterministik boşsa LLM'i atla" uygulanmayacak (şimdilik).** Mevcut tasarım LLM tier'ı
   `tieredEnabled` + `maxLLMTierPerQuestion` ile zaten kapılıyor. Tier-1 boş dönünce LLM'i
   koşulsuz atlamak, yalnızca LLM'in yakalayabildiği belirsizlikleri kaçırır. Karar: önce
   `biqly_ambiguity_llm_tier_yield_total` metriği eklenip LLM tier'ının gerçek katkısı ölçülecek;
   veri "katkı yok" derse atlama davranışı config bayrağıyla eklenecek. (Bkz. 01 §4, 03 §3)
2. **`ai_runtime_config` şeması değişmeyecek.** Tablo `key TEXT PK, value JSONB, updated_at`
   olarak migration 045a'da zaten var ve `ambiguity` anahtarıyla kullanılıyor. Locale-boyutlu
   veri ihtiyacı `ai_nl_lexicon` (048) ile zaten karşılanıyor; runtime config'e locale/domain
   kolonu eklemek gereksiz churn. (Bkz. 02 §1)
3. **Admin API yetkilendirmesi RBAC'a taşınacak.** Bugün `AdminKeyMiddleware` = JWT super_admin
   VEYA paylaşılan `BI_ADMIN_API_KEY`. Hedef: `ai:settings` / `admin:settings` izinleriyle JWT
   tabanlı kontrol; paylaşılan anahtar yalnızca makine-makine (CI/operasyon) için kalacak ve
   frontend'den tamamen kaldırılacak. (Bkz. 02 §3, 05 §1)
4. **UI'dan yönetilmeyecek ayarlar:** kripto anahtarlar (`BI_ENCRYPTION_KEY`), API key'ler,
   rate limit (`BI_AI_RATE_LIMIT_PER_MINUTE`), prompt boyutu (`BI_AI_MAX_PROMPT_RUNES`),
   timeout'lar, DSN'ler. Bunlar env/secret olarak kalır. UI'dan yönetilecekler: ambiguity
   bayrakları, PII detection threshold + default masking strategy, memory recall toggle.
5. **Prod tracing %100 → oranlı örnekleme.** values-prod.yaml bugün `always_on` + `samplerArg: "1"`;
   `parentbased_traceidratio` 0.1'e düşürülecek. (Bkz. 05 §3)
