# ADR-0001: DB Tabanlı NL Lexicon ve Dinamik i18n

- **Durum:** Kabul edildi (2026-06-10)
- **Karar vericiler:** Baris Dogu
- **Kapsam:** DİL-0 (tasks/todo.md → "Dil Varlıklarının Koddan Çıkarılması" yol haritası)

## Bağlam

"aylık", "günlük", "geçen ay", "silinen", "kaç/adet/toplam" gibi doğal-dil tanımlamaları
yalnızca EN+TR için kod içinde gömülü. Yeni bir dil eklemek bugün release gerektiriyor.
Envanter (2026-06-10, tasks/todo.md'deki tablo) iki varlık sınıfı gösteriyor:

1. **NL lexicon verisi** — eşleştirmede kullanılan kelime/kalıp listeleri:
   `vagueTemporalPhrases` (ambiguity), `softDeleteColumnSynonyms` (model_builder),
   intent token'ları (routing_budget), grain/row-count synonym'leri (time_grains +
   semanticgen'de duplike), routing lexicon (embedded JSON + dosya override).
2. **Mesaj katalogları** — kullanıcıya gösterilen metinler: `internal/i18n/locales/{en,tr}.json`
   (embed), locale registry (`SupportedLocales`, `localeProfiles`), prompt şablonları
   (DB-backed, embed yalnızca en/tr), frontend bundle'ları.

Repoda üç kez kurulmuş çalışan bir kalıp zaten var: **embedded seed + DB overlay + cache +
invalidate** (`ai_time_grains`/dbTimeGrainStore, `ai_prompt_templates`/dbPromptStore,
`ai_runtime_config`/effectiveAmbiguityConfig). Bu ADR aynı kalıbı genelleştirir.

## Karar

### K1 — Tek generic tablo: `ai_nl_lexicon`

Domain başına ayrı tablo yerine tek tablo; tek store/cache/CRUD/seed altyapısı.

```sql
CREATE TABLE IF NOT EXISTS ai_nl_lexicon (
    locale      TEXT        NOT NULL,            -- BCP-47 alt kümesi ("tr", "en", "de", …)
    domain      TEXT        NOT NULL,            -- bkz. K2
    key         TEXT        NOT NULL,            -- domain içi tanımlayıcı
    value       JSONB       NOT NULL,            -- domain'e özgü şekil (bkz. K2)
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (locale, domain, key)
);
CREATE INDEX IF NOT EXISTS idx_ai_nl_lexicon_domain ON ai_nl_lexicon (domain, locale);
```

- `id BIGSERIAL` yok; doğal anahtar (locale, domain, key) upsert/import-export'u basitleştirir.
- Versiyonlama yok (prompt şablonlarındaki gibi); `updated_at` + admin audit log yeterli.
  İhtiyaç doğarsa sonradan `version int` eklenebilir — şimdilik YAGNI.

### K2 — Domain'ler ve value şekilleri

| domain | key | value şekli (JSONB) | Bugünkü kaynak |
|---|---|---|---|
| `temporal_phrase` | kalıp ("geçen ay") | `{"interpretation_keys": ["prev_calendar_month","rolling_30d"]}` | `ambiguity/temporal_detector.go` |
| `grain_synonym` | grain adı ("month") | `{"terms": ["ay","aylık","aylik"]}` | `routing/time_grains.go` + `semanticgen/generator.go` (duplike) |
| `soft_delete` | kural adı ("ts_deleted") | `{"terms": ["silinen","silinmiş",…]}` | `routing/model_builder.go` |
| `intent_token` | intent ("quantity", "total") | `{"terms": ["kaç","adet","toplam"]}` | `routing/routing_budget.go` |
| `row_count` | "row_count" | `{"terms": ["adet","sayısı","kaç tane"]}` | routing lexicon + semanticgen |
| `token_synonym` | token ("müşteri") | `{"terms": ["customer","client"]}` | `routing_lexicon_default.json` |
| `metric_synonym` | metric anahtarı ("min_numeric") | `{"terms": [...]}` | `routing_lexicon_default.json` |

Not: `interpretation_keys` i18n katalog anahtarlarına (`clarification.temporal.*_label/_desc`)
işaret eder — *davranış* lexicon'da, *metin* i18n kataloğunda. İkisi ayrı kalır.

### K3 — `ai_time_grains` kalır; synonym'leri lexicon'a taşınır

Grain *yapısı* (grain, suffix, requires_time) `ai_time_grains`'te kalır — bu dil verisi değil.
Dil-taşıyan `synonyms TEXT[]` kolonu DİL-1'de `grain_synonym` domain'ine taşınır:

- Geçiş süresince `TimeGrainStore.List` iki kaynağı birleştirir (kolon + lexicon);
  kolon DİL-1 sonunda deprecate edilir (drop ayrı migration, acele yok).
- `synonyms_by_locale JSONB` alternatifi REDDEDİLDİ: grain synonym'lerini diğer
  domain'lerden farklı bir mekanizmada yönetmek tek-yüzey hedefini bozar.
- `semanticgen/generator.go`'daki duplike listeler aynı taşımada silinir; tek kaynak
  time-grain store + lexicon olur.

### K4 — Eşleştirme semantiği: etkin locale'lerin BİRLEŞİMİ

Eşleştirme (routing/ambiguity) **tüm etkin locale'lerin terimlerinin birleşimi** üzerinde
çalışır — bugünkü davranış budur (`DefaultTimeGrains.Synonyms` EN+TR karışık tek dizi).
Locale boyutu *yönetim* içindir (bir dili diğerlerine dokunmadan ekle/güncelle), eşleştirmeyi
kısıtlamaz. Soru-diline göre filtreleme olası bir optimizasyondur; davranış değiştirdiği için
bu ADR kapsamı dışında bırakıldı.

### K5 — Fallback zinciri ve boot bağımsızlığı

Kod, bugünkü hardcoded listeleri **embedded seed/fallback** olarak taşımaya devam eder:

- **Lexicon okuma:** DB satırları (is_active=true) → DB hatası VEYA domain tamamen boş →
  embedded default. Boot asla DB'ye bağımlı değildir (dbTimeGrainStore kalıbı birebir).
- **Mesaj katalogları (DİL-3):** istek locale DB bundle → istek locale embedded bundle →
  DefaultLocale (en) → anahtarın kendisi. EN ve TR embedded bundle'lar her zaman gemide kalır;
  EN terminal fallback olduğu için registry'den devre dışı bırakılamaz.

### K6 — Cache ve invalidation

`ai_runtime_config` deseni: **30s TTL + yazan replikada anında Invalidate**. PUT yapan pod
hemen taze okur; diğer replikalar TTL içinde yakınsar (kabul edilmiş tradeoff, aynı gerekçe).
`routing_lexicon.go`'daki `sync.Once` bu kalıba çevrilir (DİL-2). Hot-path kuralı: lexicon
erişimi her zaman süreç-içi cache'ten; istek başına DB sorgusu yasak.

### K7 — Seed ve kurtarma

- Startup'ta idempotent seed: ilgili domain boşsa embedded default'lardan doldur
  (`SeedTimeGrains` kalıbı).
- Admin kaçış kapısı: `RestorePromptTemplateFromEmbed` muadili — domain'i (veya tek key'i)
  embedded default'a sıfırlayan admin endpoint'i (DİL-1 admin CRUD'una dahil).

### K8 — Locale registry (DİL-3 ön kararı)

```sql
CREATE TABLE IF NOT EXISTS i18n_locales (
    locale                     TEXT PRIMARY KEY,
    label                      TEXT NOT NULL,
    short_label                TEXT NOT NULL,
    question_letters           TEXT NOT NULL DEFAULT '',
    question_signals           JSONB NOT NULL DEFAULT '[]',
    uses_metadata_translations BOOLEAN NOT NULL DEFAULT FALSE,
    enabled                    BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`SupportedLocales`/`localeProfiles`/`ParseLocale` registry'den beslenir; EN/TR embedded
profiller seed'dir ve EN devre dışı bırakılamaz (K5). Katalog overlay tablosu (`i18n_bundles`)
şeması DİL-3'te bu ADR'nin K5/K6 sözleşmesine göre detaylandırılır.

### K9 — Admin yüzeyi

`GET/PUT /api/ai/admin/lexicon?locale=&domain=` + JSON import/export, AdminKeyMiddleware
arkasında (`ai_admin_config.go` kalıbı). PUT validasyonu: bilinen domain, geçerli locale
formatı, domain'e uygun value şeması. PUT → Invalidate.

## Değerlendirilen alternatifler

1. **Domain başına ayrı tablo** — Reddedildi: N× migration/store/CRUD boilerplate'i;
   tek tablonun JSONB value'su domain çeşitliliğini zaten taşıyor.
2. **ConfigMap/dosya tabanlı lexicon'lar** (BI_AI_ROUTING_LEXICON_PATH'in genelleştirilmesi) —
   Reddedildi (birincil mekanizma olarak): admin UI yok, ops-only, locale boyutu yok,
   pod restart/reload karmaşası. Mevcut dosya override'ı geçiş süresince korunur (DİL-2).
3. **Her şeyi i18n bundle'larına koymak** — Reddedildi: lexicon görüntü metni değil,
   yapılandırılmış eşleştirme verisi (interpretation_keys gibi alanlar taşıyor); ayrıca
   bundle'lar istek-locale'e göre seçilirken eşleştirme birleşim ister (K4).
4. **Harici TMS / çeviri servisi entegrasyonu** — Kapsam dışı: katalog yönetimi olgunlaşınca
   üstüne eklenebilir; bu ADR'nin veri modeli buna engel değil.
5. **`ai_time_grains.synonyms_by_locale JSONB`** — Reddedildi (K3'te gerekçe).

## Sonuçlar

**Olumlu:** Yeni dil = DB satırları + registry kaydı, backend release'i yok. Tek yönetim
yüzeyi. `semanticgen` ↔ `time_grains` duplikasyonu kalkar. Mevcut kalıpların tekrarı olduğu
için öğrenme maliyeti düşük.

**Olumsuz / riskler:**
- Davranışı etkileyen veri DB'ye taşınıyor → yanlış admin düzenlemesi routing/ambiguity
  kalitesini bozabilir. Hafifletme: value şema validasyonu (K9), embedded'a sıfırlama (K7),
  eval golden case'leri (davranış çıpası), DİL-5 CI bekçisi.
- Çok-replika tutarlılığı TTL penceresi kadar gecikir (K6) — kabul edildi.
- Eşleştirme birleşim semantiği (K4) dil sayısı arttıkça çapraz-dil yanlış eşleşme riskini
  büyütür (örn. iki dilde aynı yazılan farklı anlamlı kelime). İzlenecek; gerekirse
  locale-filtreli eşleştirme ayrı ADR ile gelir.

## Uygulama sırası

DİL-1 (`ai_nl_lexicon` + taşımalar + admin CRUD) → DİL-2 (routing lexicon overlay) ‖
DİL-3 (registry + katalog overlay) → DİL-4 (prompt yeni-locale seeding) → DİL-5 (frontend
kararı + eval + CI bekçisi). Ayrıntılar: tasks/todo.md.
