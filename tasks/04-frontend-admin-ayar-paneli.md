# 04 — Frontend: Admin Ayar Paneli, i18n ve a11y

**Öncelik: P1** (02'ye bağımlı: backend domain'leri genişlemeden panel alanları eklenemez)

## Amaç

Admin panelindeki PlatformSettingsPanel'i AI/PII/memory runtime ayarlarını kapsayacak şekilde
genişletmek; ClarificationCard / GenerationTrace / learned-badge bileşenlerindeki i18n ve
erişilebilirlik boşluklarını kapatmak; test güvencesi eklemek.

## Mevcut Durum (doğrulandı, 2026-06-10)

| Bileşen | Durum | Konum |
| --- | --- | --- |
| Admin nav (16 bölüm, 4 grup, lazy-load kalıbı) | ✅ Hazır altyapı | `frontend/src/components/admin/Admin.tsx`, `adminNavConfig.ts` |
| Tiered ambiguity config UI (`tieredEnabled` checkbox + `maxLLMTier` 0–10 input + DB-override notu) | ✅ **Zaten var** | `admin/PlatformSettingsPanel.tsx:24-234` |
| API client (`getAIAdminConfig` / `updateAIAdminConfig`) | ✅ Var | `frontend/src/api/aiAdmin.ts` |
| RBAC gate (`hasPermission('admin:settings')`) | ✅ Kalıp oturmuş | `auth/AuthProvider.tsx` |
| Toast sistemi (`useToast`: success/error) | ✅ Var | `hooks/useToast.tsx` |
| ClarificationCard | ⚠️ Çalışıyor; i18n boşlukları var | `aiQuery/routingViz.tsx:371-451` |
| GenerationTracePanel | ✅ ClarificationCard içinde `defaultOpen={true}` zaten veriliyor (`routingViz.tsx:448`); TR çevirileri tam | `aiQuery/generationTrace.tsx` |
| Learned badge (`role="status"` + i18n) | ✅ Var | `aiQuery/FeedbackSection.tsx` |
| ConfirmedQueriesPanel | ⚠️ Durum yalnızca düz metin ("Active"); görsel rozet yok; test yok | `admin/ConfirmedQueriesPanel.tsx` |
| Frontend testler | ⚠️ Vitest kurulu; `routingViz.tsx` (650+ satır) ve ConfirmedQueriesPanel testsiz; Playwright yok | `frontend/src/**/*.test.ts` |

> Not: Kullanıcı isteğindeki "useRuntimeConfig hook'u yaz" işi fiilen `aiAdmin.ts` + panel state
> ile karşılanmış durumda. Yeni domain'ler eklenince paneli sadeleştirmek için hook'a çıkarılacak.

## Yapılacaklar

### 1. PlatformSettingsPanel genişletmesi ("AI Ambiguity & Config")

Backend 02 tamamlandıktan sonra:

1. `aiAdmin.ts` tiplerini genişlet: `AIAdminConfig { ambiguity, pii, memory }`, her alan
   `source: 'db_override' | 'env' | 'default'` ile.
2. `useRuntimeConfig()` hook'u çıkar (`hooks/useRuntimeConfig.ts`):
   `{ config, loading, error, save, saving }` — GET/PUT'u, hata state'ini ve optimistic
   olmayan kaydetmeyi (save → yeniden GET) kapsüller. Mevcut panel state'i hook'a taşınır.
3. Panel bölümleri (mevcut ambiguity bölümünün yanına):
   - **Ambiguity**: mevcut 2 alan + `check_enabled` toggle, `confidence_threshold`
     (0.0–1.0 step 0.05), `max_options` (1–10).
   - **PII**: `detection_threshold` (0.0–1.0), `default_masking_strategy`
     (select: partial/full/none). `BI_PII_ENABLED` **gösterilir ama salt-okunur**
     (env-only — yanına "Helm üzerinden yönetilir" notu).
   - **Memory**: `recall_enabled` toggle, `recall_limit` (1–10).
4. Her alanın yanında kaynak rozeti (DB / Env / Default) — mevcut `db_override` notu kalıbı
   genelleştirilir.
5. Kaydet: tek "Save" butonu, başarıda `toast.success(t('admin.platform_settings.settings_saved'))`,
   hatada alan-bazlı mesaj (backend 400 yanıtındaki alan hatalarını input altında göster).
6. Validasyon client-side da aynalansın (number input min/max + step), ama otorite backend.
7. BEM CSS (`frontend/src/styles/`), Tailwind yok; tüm metinler `useT()` ile; EN + TR
   anahtarları `i18n/locales/{en,tr}/admin.ts`.

### 2. ClarificationCard i18n + a11y boşlukları

1. Eksik i18n anahtarları (`routingViz.tsx:371-451`):
   - Resolved-columns listesi "Term" / "Resolved" sütun başlıkları için
     `ai_query.clarification_term` / `ai_query.clarification_resolved` ekle (EN+TR).
   - Maks tur göstergesi: `MAX_CLARIFICATION_ROUNDS = 2` sabiti kullanıcıya görünür değil;
     `ai_query.clarification_round_indicator` ("Tur {{current}}/{{max}}") ekle ve cap'e
     yaklaşırken göster (backend yanıtı round bilgisini taşıyorsa kullan; taşımıyorsa
     backend yanıtına `clarification_round` alanı eklenmesi 01 kapsamında istenir).
2. a11y: seçenek butonlarına `aria-label` (seçenek metni + "netleştirme seçeneği"),
   kart köküne `role="group"` + `aria-labelledby` (başlık id'si).

### 3. ConfirmedQueriesPanel iyileştirmesi

1. Durum sütununu görsel rozete çevir (BEM: `confirmed-query__status--active|inactive`),
   `aria-label` ile.
2. Deactivate akışına onay + toast; hata durumunda satır-bazlı hata mesajı.

### 4. Testler

1. **`useRuntimeConfig.test.ts`**: GET başarı/hata, PUT başarı (yeniden fetch), PUT 400
   (alan hatası yüzeye çıkar) — mevcut `useApi.test.ts` kalıbıyla.
2. **`PlatformSettingsPanel` testi**: toggle/inputs render + değer değişimi + save çağrısı +
   toast tetiklenmesi; `admin:settings` izni yokken salt-okunur not.
3. **ClarificationCard testi** (`routingViz` içinden): seçenekler render, skip butonu,
   cap-reached notu, tur göstergesi.
4. **ConfirmedQueriesPanel testi**: liste yükleme, deactivate akışı.
5. e2e: Playwright bu repoda frontend için kurulu değil — **karar:** bu fazda Playwright
   eklenmeyecek; kritik akışlar Vitest + Testing Library ile kapsanacak. Playwright yatırımı
   ayrı bir karar olarak 07'de değerlendirilecek.

### 5. Kalite kapısı

Her dokunulan dosya için: `make lint-frontend` + `npx prettier --check` + `npm --prefix
frontend run test`; commit öncesi `make check-frontend` (lint + format + knip + test + build).

## Kabul Kriterleri

- [x] Admin → Platform Ayarları'nda üç bölüm (Ambiguity 5 knob / PII / Memory) alan-bazlı
      DB/Env kaynak rozetleriyle görünüyor (`SourceBadge`, `sources` haritasından).
- [x] Save → tek PUT (üç domain) → başarıda `runtime_saved` toast → yanıt state'e yazılıp
      draft yeniden senkronize ediliyor; backend'in alan-bazlı 400 mesajı toast'ta gösteriliyor.
      Client-side clamp (`buildRuntimeConfigUpdate`) aralık dışı değerin 400'e gitmesini önlüyor.
- [x] Düzenleme yetkisi `hasPermission('ai:settings')` (super_admin otomatik geçer) — backend
      `AdminAccessMiddleware` ile simetrik. İzinsiz kullanıcıya alanlar disabled.
- [x] ClarificationCard: "Tur {{current}}/{{max}}" göstergesi (`clarification_round_indicator`,
      EN+TR), terim listesine `aria-label` (`clarification_terms_label`), kart köküne
      `role="group"` + `aria-labelledby`. Tur bilgisi backend'in `clarification_round` alanından
      `deriveClarificationStage` ile türetiliyor (cap'te "2/2", cap sonrası gösterge sabit).
- [x] Yeni testler yeşil (8 test: draft/clamp/round-stage); `make check-frontend` tam kapı temiz
      (lint + format:check + knip + test + build, 2026-06-10).
- [x] Form alanları `useId` ile benzersiz id, label-htmlFor eşleşmesi, rozetlerde aria-label,
      hata mesajında `role="alert"`.

## Uygulama Notları (2026-06-10)

- **`useRuntimeConfig` hook'u** (`hooks/useRuntimeConfig.ts`): GET/PUT yaşam döngüsü +
  test edilebilir saf yardımcılar (`draftFromConfig`, `buildRuntimeConfigUpdate`, `clampNumber`).
  `save` throw eder ki panel backend'in alan-bazlı mesajını toast'layabilsin.
- **Panel mimarisi**: ESLint `complexity ≤ 20` kuralı yüzünden runtime bölümü
  `AIRuntimeSection` (yaşam döngüsü) + `AIRuntimeForm` (non-null props, salt sunum) olarak
  ayrıldı; `ToggleField`/`NumberField`/`SourceBadge`/`SectionHeader` yeniden kullanılabilir
  alt bileşenler. Eski `saveAmbiguity`/`ambiguity_saved` kaldırıldı (EN+TR).
- **PII bölümü**: yalnızca `detection_threshold` düzenlenebilir; `BI_PII_ENABLED` durumu
  salt-okunur metin + "Helm üzerinden yönetilir" notu (02'deki backend kararıyla uyumlu;
  masking stratejisi kodda tüketilmediği için forma hiç konmadı).
- **Test stratejisi**: repo'da Testing Library/jsdom yok — mevcut kalıba uyularak
  (abExperimentPanelLogic.test.ts gibi) mantık saf fonksiyonlara çıkarılıp vitest ile test
  edildi: `useRuntimeConfig.test.ts` (5) + `clarificationStage.test.ts` (4 senaryo).
  Bileşen-render testleri için Testing Library yatırımı 07'deki Playwright kararıyla birlikte
  ayrıca değerlendirilecek.
- **ConfirmedQueriesPanel**: durum artık `admin-badge-active/inactive` rozeti (aria-label'lı);
  pasifleştirme `useConfirm` dialoguyla onaylı (`deactivate_confirm_*` anahtarları EN+TR).
- **Tur göstergesi mimarisi**: `clarificationStage.ts` — `MAX_CLARIFICATION_ROUNDS` sabiti ve
  `deriveClarificationStage` tek kaynağa toplandı; assistantMessageCardSections içindeki kopya
  mantık silindi. CSS: `.clarification-round-indicator` (aiQuery.css).
- GenerationTracePanel `defaultOpen` clarification bağlamında zaten true idi — dokunulmadı.

## İlgili Dosyalar

- `frontend/src/components/admin/PlatformSettingsPanel.tsx`, `ConfirmedQueriesPanel.tsx`,
  `Admin.tsx`, `adminNavConfig.ts`
- `frontend/src/components/aiQuery/routingViz.tsx`, `generationTrace.tsx`, `FeedbackSection.tsx`
- `frontend/src/api/aiAdmin.ts`, `src/hooks/useApi.ts`, `useToast.tsx` (+ yeni `useRuntimeConfig.ts`)
- `frontend/src/i18n/locales/{en,tr}/{admin,core}.ts`
