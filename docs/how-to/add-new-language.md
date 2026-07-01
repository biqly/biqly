# Yeni Dil Onboarding Runbook'u

Bu runbook, Biqly'ye yeni bir kullanıcı dili eklemek için operasyonel adımları anlatır. Hedef: yeni dili backend registry'sine, prompt template alanına, AI domain lexicon'una ve frontend i18n katmanına güvenli şekilde almak; prod'da enable etmeden önce smoke test ve coverage ile doğrulamak.

## 0. Varsayımlar ve çıkış kriteri

- Yeni dil için BCP-47 dil kodu bellidir.
- Dil aktif edilmeden önce staging/prod benzeri ortamda test edilir.
- Çıkış kriteri:
  - `GET /api/ai/admin/i18n/locales` yeni locale satırını döner.
  - `GET /api/ai/admin/i18n/bundles/{locale}` yeni locale için effective bundle'ı döner (`source=database` veya `embedded`).
  - `GET /api/ai/admin/i18n/coverage/{locale}` i18n coverage raporunu döner.
  - `GET /api/ai/admin/lexicon?locale={locale}` domain lexicon kayıtlarını döner.
  - `GET /api/ai/prompt-templates?locale={locale}` prompt template listesini döner.
  - Frontend `supported` listesinde yeni dil görünür ve kullanıcı arayüzü fallback olmadan yüklenir.
  - Smoke testler 30 dakika içinde tamamlanır.

## 1. Locale registry satırını ekle

Önce yeni locale'i sadece backend registry'sine ekleyin, ama `enabled=false` bırakın.

Örnek: Almanca için `de`.

```bash
curl -X PUT "$API_URL/api/ai/admin/i18n/locales" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "locales": [
      {
        "locale": "de",
        "label": "Deutsch",
        "short_label": "DE",
        "question_letters": "äöüßÄÖÜ",
        "question_signals": [
          " zeige ",
          " göster ",
          " kaç ",
          " ne kadar ",
          " geçen ",
          " son "
        ],
        "uses_metadata_translations": true,
        "enabled": false
      }
    ]
  }'
```

Doğrulama:

```bash
curl "$API_URL/api/ai/admin/i18n/locales" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.locales[] | select(.locale == "de")'
```

Beklenen: `locale=de`, `enabled=false`, `uses_metadata_translations=true`.

## 2. Frontend locale paketini ekle

Frontend'de yeni dili desteklemek için:

1. `frontend/src/i18n/locales/{locale}` dizinini açın.
2. `core`, `admin`, `auth` section'larını ekleyin.
3. `frontend/src/i18n/index.tsx` içinde:
   - `Locale` union tipine yeni dili ekleyin.
   - `SUPPORTED_LOCALES` listesine yeni dili ekleyin.
   - `LOCALE_OPTIONS` içine label, short, languageTag girin.
   - `sectionLoaders` içine yeni dilin lazy section loader'larını ekleyin.
4. `frontend/src/i18n/locale.ts` için aynı değişiklikleri uygulayın.

Örnek:

```ts
export type Locale = 'en' | 'tr' | 'de'
export const SUPPORTED_LOCALES: Locale[] = ['en', 'tr', 'de']
export const LOCALE_OPTIONS: Record<Locale, { label: string; short: string; languageTag: string }> = {
  en: { label: 'English', short: 'EN', languageTag: 'en-US' },
  tr: { label: 'Türkçe', short: 'TR', languageTag: 'tr-TR' },
  de: { label: 'Deutsch', short: 'DE', languageTag: 'de-DE' },
}
```

Örnek lazy loader:

```ts
const sectionLoaders: Record<Locale, Record<LocaleSectionName, () => Promise<PartialDict>>> = {
  en: {
    admin: () => import('./locales/en/admin').then((m) => ({ admin: m.admin })),
    auth: () => import('./locales/en/auth').then((m) => ({ auth: m.auth })),
  },
  tr: {
    admin: () => import('./locales/tr/admin').then((m) => ({ admin: m.admin })),
    auth: () => import('./locales/tr/auth').then((m) => ({ auth: m.auth })),
  },
  de: {
    admin: () => import('./locales/de/admin').then((m) => ({ admin: m.admin })),
    auth: () => import('./locales/de/auth').then((m) => ({ auth: m.auth })),
  },
}
```

Frontend doğrulama:

```bash
npm --prefix frontend run lint
npm --prefix frontend run build
```

### 2.1 Backend i18n bundle overlay'ini doldur

Locale registry satırı tek başına yeterli değildir; backend kullanıcı mesajları
(`clarification.*`, `admin.*` vb.) için `i18n_bundles` overlay'ini de ekleyin.

Örnek:

```bash
curl -X PUT "$API_URL/api/ai/admin/i18n/bundles/de" \
  -H "Authorization: ******" \
  -H 'Content-Type: application/json' \
  -d '{
    "clarification": {
      "ambiguity_reason": "Soru birden fazla şekilde yorumlanabiliyor.",
      "needs_clarification_warning": "Devam etmeden önce lütfen ne demek istediğinizi seçin."
    }
  }'
```

Doğrulama:

```bash
curl "$API_URL/api/ai/admin/i18n/bundles/de" \
  -H "Authorization: ******" | jq

curl "$API_URL/api/ai/admin/i18n/coverage/de" \
  -H "Authorization: ******" | jq
```

Beklenen: bundle endpoint'i `source=database` döner; coverage endpoint'i
eksik anahtarları `missing_keys` altında listeler. Locale'i prod'da
enable etmeden önce ideal durum `missing_keys=[]` olmasıdır.

## 3. Prompt template alanını hazırla

Yeni locale prompt template'leri için iki yol var:

- Staging/prod benzeri operasyon: locale registry'de `enabled=true` yapıp DB prompt template'lerini admin endpoint ile doldurun.
- Release zamanlı operasyon: `internal/ai/prompt/prompts/{locale}/*.tmpl` altına embedded fallback ekleyin, sonra backend deploy edin.

DB prompt template'leri için:

```bash
curl "$API_URL/api/ai/prompt-templates?locale=de" \
  -H "Authorization: ******" | jq '.[] | select(.locale == "de")'
```

Eğer locale henüz enabled değilse, `PUT /api/ai/prompt-templates/{name}/{locale}`
çağrısı `unsupported locale` döner. Bu durumda staging/prod benzeri ortamda
locale'i geçici olarak enable edin:

```bash
curl -X PUT "$API_URL/api/ai/admin/i18n/locales" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "locales": [
      {
        "locale": "de",
        "label": "Deutsch",
        "short_label": "DE",
        "question_letters": "äöüßÄÖÜ",
        "question_signals": [" zeige ", " kaç ", " son "],
        "uses_metadata_translations": true,
        "enabled": true
      }
    ]
  }'
```

Prompt template güncelleme:

```bash
curl -X PUT "$API_URL/api/ai/prompt-templates/system_rules/de" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "content": "Sen Biqly asistanısın. Kullanıcıya Almanca yanıt ver. Eksik veri varsa varsayma; netleştir."
  }'
```

Doğrulama:

```bash
curl "$API_URL/api/ai/prompt-templates?locale=de" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.[] | select(.locale == "de")'
```

## 4. AI domain lexicon satırlarını ekle

Yeni locale için en az bir `temporal_phrase` domain entry'si eklenmelidir. Çünkü prompt template'leri zaman ifadelerini yorumlamak için bu domain'i kullanır.

Örnek:

```bash
curl -X PUT "$API_URL/api/ai/admin/lexicon" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "entries": [
      {
        "locale": "de",
        "domain": "temporal_phrase",
        "key": "son 7 gün",
        "terms": ["son 7 gün", "son yedi gün", "letzte 7 tage"],
        "interpretation_keys": ["rolling_7d"]
      },
      {
        "locale": "de",
        "domain": "temporal_phrase",
        "key": "geçen ay",
        "terms": ["geçen ay", "letzter monat"],
        "interpretation_keys": ["prev_calendar_month", "rolling_30d"]
      }
    ]
  }'
```

Doğrulama:

```bash
curl "$API_URL/api/ai/admin/lexicon?locale=de" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.entries[] | select(.domain == "temporal_phrase")'
```

Beklenen: `locale=de` ve `domain=temporal_phrase` için en az bir satır.

## 5. Coverage ve smoke test

### 5.1 i18n coverage

```bash
curl "$API_URL/api/ai/admin/i18n/coverage/de" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq
```

Beklenen: Coverage endpoint'i yeni locale'in effective bundle'ında eksik kalan
anahtarları `missing_keys` altında raporlar; prod enable öncesi hedef bu listenin boş olmasıdır.

### 5.2 Prompt template coverage

```bash
curl "$API_URL/api/ai/prompt-templates?locale=de" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.[] | select(.locale == "de")'
```

Beklenen: Yeni locale için prompt template listesi döner.

### 5.3 Frontend smoke test

1. Frontend'i açın.
2. Dil seçici üzerinden yeni dili seçin.
3. Admin, auth ve core alanlarında eksik çeviri olup olmadığını kontrol edin.
4. Eğer eksik çeviri varsa ilgili `frontend/src/i18n/locales/{locale}` dosyalarını güncelleyin.
5. `npm --prefix frontend run build` tekrar çalıştırın.

### 5.4 Backend smoke test

1. Yeni locale ile basit bir query çalıştırın.
2. Query'nin locale-aware prompt template kullandığını doğrulayın.
3. Eğer query İngilizce yanıt dönerse:
   - `GET /api/ai/admin/i18n/bundles/{locale}` ile locale bundle'ını kontrol edin.
   - `GET /api/ai/prompt-templates?locale={locale}` ile locale template'lerini kontrol edin.
   - `GET /api/ai/admin/lexicon?locale={locale}` ile domain lexicon'u kontrol edin.
   - `GET /api/ai/admin/i18n/locales` ile locale'in enabled olduğunu doğrulayın.

### 5.5 30 dakikalık acceptance

- İlk 10 dakika: locale registry ve frontend smoke test.
- İkinci 10 dakika: prompt template ve lexicon smoke test.
- Son 10 dakika: gerçek kullanıcı senaryosu ve fallback davranışı.

## 6. Prod'a alma

Prod'da yeni dili açmadan önce:

1. Staging/prod benzeri ortamda smoke testleri geçirin.
2. Frontend build'ini geçirin.
3. `GET /api/ai/admin/i18n/bundles/{locale}` ile DB bundle overlay'inin yüklü olduğunu doğrulayın.
4. Coverage endpoint'inde `missing_keys` listesinin boş olduğunu doğrulayın.
5. Prompt template'lerin locale-specific olduğunu doğrulayın.
6. Lexicon'da en az bir `temporal_phrase` entry'si olduğunu doğrulayın.

Sonrasında:

```bash
curl -X PUT "$API_URL/api/ai/admin/i18n/locales" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "locales": [
      {
        "locale": "de",
        "label": "Deutsch",
        "short_label": "DE",
        "question_letters": "äöüßÄÖÜ",
        "question_signals": [" zeige ", " kaç ", " son "],
        "uses_metadata_translations": true,
        "enabled": true
      }
    ]
  }'
```

## 7. Rollback

Sorun olursa locale'i kapatın:

```bash
curl -X PUT "$API_URL/api/ai/admin/i18n/locales" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "locales": [
      {
        "locale": "de",
        "label": "Deutsch",
        "short_label": "DE",
        "question_letters": "äöüßÄÖÜ",
        "question_signals": [" zeige ", " kaç ", " son "],
        "uses_metadata_translations": true,
        "enabled": false
      }
    ]
  }'
```

Frontend'de sorun varsa `SUPPORTED_LOCALES` listesinden çıkarıp build'i tekrar alın.

## 8. Notlar

- Locale registry'ye ekleme yaparken `enabled=false` ile başlayın.
- Backend kullanıcı mesajları için `PUT /api/ai/admin/i18n/bundles/{locale}` ile DB bundle overlay'ini doldurun; coverage raporu bu bundle'ı ölçer.
- Prompt template'leri DB üzerinden yönetilebilir; embedded fallback sadece release zamanlı alternatif.
- Lexicon'da en az bir `temporal_phrase` domain'i olmadan yeni locale'i açmayın.
- Frontend locale'leri lazy-loaded section'lar halinde gelir; `admin` ve `auth` section'larını unutmayın.
