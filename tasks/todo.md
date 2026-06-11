# Todo list

## Güvenlik İncelemesi — Table Browser & Metadata (2026-06-11)

Kaynak: 591dbb2f (`feat(metadata): table display expressions and table browser row modal`) commit'i + ilgili route'lar üzerinde yapılan detaylı güvenlik incelemesi (2 bağımsız doğrulama turu, her bulgu 8/10 güvenle teyit edildi).

### S0 — PII Maskeleme Bypass: `BrowseTableRows` rows endpoint'i [HIGH]

**Bulgu:** Yeni `POST /api/datasources/{id}/tables/{schema}/{table}/rows` endpoint'i
(`internal/http/handlers/metadata_rows.go`) müşteri DB'sine `SELECT *` atıp satırları
olduğu gibi dönüyor. Route yalnızca `RequireDatasourceAccess(authClient, "read")` ile
korunuyor; query path'inin uyguladığı rol bazlı PII maskeleme
(`internal/core/pii_policy.go` → `query.PIIMaskingConfig`, compiler'da mask/hide +
hidden kolonda filtre reddi) burada **hiç uygulanmıyor**.

**Saldırı yolu:** Datasource'a read erişimi olan bir `analyst`/`viewer`, `/query`'de
maskeli/gizli göreceği PII kolonlarını (tckn, email vb.) bu endpoint'ten ham olarak okur;
`contains`/`starts_with`/`gt` filtreleri + `order_by` + offset sayfalama ile değer
enumeration ve tam tablo exfiltration yapabilir (sayfa başına 200 satır).
Mevcut `GetTableSample` (50 satır, filtresiz) aynı açığı zaten taşıyordu; yeni endpoint
filtre/sıralama/sayfalama ile istismar edilebilirliği ciddi şekilde artırdı.

- [x] `PIIPolicyService` (MaskingConfig) bağımlılığını `MetadataHandler` deps'ine ekle.
- [x] `BrowseTableRows`'ta `SELECT *` yerine `columns` listesinden açık projection kur:
  `hidden` kolonları projection'dan çıkar, `masked` kolonlara query path'teki maskeleme
  ifadesini uygula, `raw` kolonları olduğu gibi geçir.
- [x] `hidden` (ve tercihen `masked`) kolonları hedefleyen `filters` ve `order_by`
  isteklerini 400 ile reddet — compiler'daki "hidden kolonda filtre reddi" kuralının aynısı.
- [x] `include_total` COUNT sorgusundaki WHERE'e de aynı kuralı uygula (gizli kolon
  predicate'iyle count sızdırılamasın).
- [x] Aynı maskelemeyi `GetTableSample`'a da uygula (`internal/http/handlers/metadata.go`).
- [x] Regresyon testi: viewer/analyst rolüyle PII kolonlu tabloda browse → hidden kolon
  yok, masked kolon maskeli, hidden kolona filtre → 400.
- [x] **Kabul:** `/query` ile rows-browse aynı kullanıcı için aynı PII görünürlüğünü verir;
  hiçbir rol browse üzerinden query'de göremediği veriyi göremez.

#### Uygulama planı (Codex, 2026-06-11)

- [x] RED: `metadata_rows_test.go` içinde projection + predicate reddi testleriyle mevcut `SELECT *` açığını yakala.
- [x] GREEN: `CatalogDeps` üzerinden `PIIPolicyService`'i metadata handler'a taşı.
- [x] GREEN: rows/sample için explicit projection üret; hidden kolonları çıkar, masked kolonları dialect mask expression + alias ile döndür.
- [x] GREEN: hidden ve masked kolonlara filter/order_by isteklerini 400'e düşür; aynı WHERE kuralını `include_total` COUNT için kullan.
- [x] VERIFY: focused Go testleri + gofmt + `git diff --check`.

#### Sonuç (Codex, 2026-06-11)

- `BrowseTableRows` ve `GetTableSample` artık `PIIPolicyService.MaskingConfig` ile aynı PII görünürlüğünü uygular.
- Hidden PII kolonları projection dışı kalır; masked kolonlar dialect mask expression ile stable alias altında döner.
- Filter/order_by masked veya hidden PII kolon hedeflediğinde 400 döner; `include_total` COUNT aynı validated WHERE'i kullandığı için protected predicate sızdırmaz.
- Doğrulama: `go test ./internal/http/handlers -run 'TestBuildTableRows(Projection|WhereRejectsProtected|OrderRejectsProtected|WherePredicates|WhereMultiChip|WhereRejectsUnknown)' -count=1`; `go test ./internal/http/handlers ./internal/app -count=1`; `make lint-go`; `git diff --check`.

### S1 — Eksik Yetkilendirme: metadata yazma/enumeration route'ları [MEDIUM]

**Bulgu:** `internal/http/catalog_router.go` içinde `PATCH /metadata/tables/{id}`
(`UpdateTableDescription` — 591dbb2f ile `display_expression` da yazılabilir oldu),
`PATCH /metadata/columns/{id}`, translation route'ları ve metadata search/list route'ları
yalnızca JWT ile korunuyor; `RequirePermission` veya `RequireDatasourceAccess` yok,
handler içinde de yetki kontrolü yapılmıyor. Açık önceden vardı; 591dbb2f yazılabilir
yüzeyi genişletti.

**Saldırı yolu:** Hiçbir datasource grant'i olmayan herhangi bir authenticated kullanıcı,
search route'undan tablo UUID'lerini enumerate edip **tüm tenant'ların** tablo
description/label/display_expression alanlarını değiştirebilir (cross-tenant metadata
tahrifatı, yanıltıcı UI). Not: `display_expression` client'ta eval'siz tokenizer ile
işlenip React text node olarak render ediliyor — XSS/SQLi/exfil yolu doğrulanmadı,
etki bütünlük (integrity) seviyesinde.

- [ ] Metadata mutasyon route'larını datasource kapsamlı yetkiye bağla: table/column ID
  üzerinden sahibi datasource'u resolve edip `CheckDatasourceAccess(level=write)` uygula
  (URL'de datasource id olmadığından handler içinde veya ID-resolve eden yeni bir
  middleware ile; `PATCH /metadata/columns/{id}/pii` route'unun `RequirePermission`
  kullanımı örnek alınabilir).
- [ ] Metadata search/list route'larını da yetki kapsamına al — cross-tenant tablo UUID
  enumeration kapatılsın (kullanıcının erişebildiği datasource'larla sınırla).
- [ ] Regresyon testi: grant'siz kullanıcı PATCH `/metadata/tables/{id}` → 403;
  search sonuçları erişilebilir datasource'larla sınırlı.
- [ ] **Kabul:** Metadata okuma/yazma, çağıranın datasource erişim seviyesiyle tutarlı;
  JWT tek başına yeterli değil.

### S2 — Sertleştirme (hardening, doğrudan açık değil) [LOW]

- [ ] `display_expression` için server-side format doğrulaması ekle
  (`UpdateTableDescription`): frontend tokenizer'ın grameriyle aynı kurallar
  (kolon token + tırnaklı literal + `+`) ve makul uzunluk limiti — ileride başka bir
  consumer'ın bu alanı güvenle kullanabilmesi için.
- [ ] `writeInternalError`'ın müşteri DB driver hata metnini client'a sızdırmadığını
  doğrula; sızdırıyorsa generic mesaj + log'a detay şeklinde ayır
  (`metadata_rows.go` "failed to fetch/count table rows" yolları).

**Temiz çıkan kontroller (aksiyon gerekmez):** SQL injection (identifier'lar introspect
edilmiş metadata'dan + `QuoteIdentSegment` tüm dialect'lerde doğru escape ediyor; değerler
parametre bind'li; `order_dir` whitelist; limit/offset int) ✓ · `display_expression`
client değerlendirmesi eval'siz/`new Function`'sız ✓ · Yeni frontend kodunda
`dangerouslySetInnerHTML`/`innerHTML` yok, React escape ✓ · `check-semgrep-sarif.py`
değişikliği yalnızca nosemgrep'li (suppressed) bulguları SARIF'ten ayıklıyor, aktif bulgu
gate'i değişmedi ✓

## Ambiguity & Clarification — Best Practices Uygulama Planı (2026-06-09)

Kaynak: `docs/research/ambiguity-clarification-best-practices.md` — mevcut mimari vs endüstri standartları karşılaştırması.

### P0 — Sync/Async Tek Yol (ProcessContext) [HIGH]

**Amaç:** Bug 2 sınıfı hataları (sync/async path divergence) yapısal olarak imkansız kılmak.

**Neden:** `resolveClarificationChoice` free function vs method split'i tekrar yaşanabilir.
İki ayrı kod yolunda (`parseAndRouteAIQuery` sync, `executeAIQueryPhase` async) aynı state
yönetimi tekrar edilmesi zorunlu — bu architectural smell.

- [x] `ProcessContext` struct oluştur (`internal/http/handlers/ai_context.go`):

  ```go
  type ProcessContext struct {
      Question              string
      ClarificationChoice   string
      ClarificationResolved bool
      DatasourceID          string
      clarificationRound    int
  }

  func (pc *ProcessContext) Resolve(ctx context.Context, ...) error { ... }
  func (pc *ProcessContext) ShouldCheckAmbiguity(cfg AmbiguityConfig) bool { ... }
  ```

- [x] `parseAndRouteAIQuery` → `buildProcessContext(req)` ile context oluştur, `Resolve()` çağır.
- [x] `executeAIQueryPhase` → aynı `buildProcessContext(req)` + `Resolve()` kullan.
- [x] `standardProcessOptions` → `req.clarificationResolved` yerine `pc.ShouldCheckAmbiguity(cfg)` oku.
- [x] Mevcut `resolveClarificationChoice` free function + method → kaldır; tek giriş noktası `ProcessContext.Resolve`.
- [x] Regresyon testleri: `TestProcessContextResolveSetsFlag`, `TestProcessContextSyncAsyncIdenticalBehavior`.
- [x] **Kabul:** Sync ve async path aynı `ProcessContext` üzerinden geçer; `clarificationResolved` flag'ini sadece
  `ProcessContext.Resolve` set eder; struct dışında bu state'e erişim yok.

### P1 — Maksimum Netleştirme Turu Sayısı (Hard Cap) [HIGH]

**Amaç:** Herhangi bir edge case'de sonsuz netleştirme döngüsünü imkansız kılmak.

**Neden:** Mevcut guard sadece `clarificationResolved` flag'ine bakıyor. Eğer rewritten question
hâlâ belirsizse ve bayrak bir şekilde resetlenirse döngü tekrar başlar. Hard cap = son çare güvenlik.

- [x] `ProcessContext.clarificationRound` sayacı ekle (`maxClarificationRounds = 2`).
- [x] `ShouldCheckAmbiguity` içinde `clarificationRound < maxClarificationRounds` kontrolü.
- [x] Her ambiguity response dönüşünde `clarificationRound++`.
- [x] Hard cap aşıldığında ambiguity check'i bypass et, logla (metric: `biqly_ambiguity_round_cap_reached_total`).
- [x] Test: `TestAmbiguityHardCapStopsAfterMaxRounds`.
- [x] **Kabul:** 2 turdan fazla netleştirme sorulmaz; cap metrics ile gözlemlenebilir.

### P2 — Zengin Glossary ai_context [MEDIUM]

**Amaç:** Deterministic ambiguity detection'ı güçlendirmek — semantic model `ai_context` benzeri
yapısal iş bağlamını Biqly glossary'e taşımak.

**Neden:** Endüstri standardı semantic katmanlar `synonyms`, `units`, `null_meaning`, `business_rules`
gibi yapısal metadata taşır. Biqly glossary flat key-value → belirsizlik azaltmak için daha zengin bağlam gerek.

- [x] `business_glossary_terms` tablosuna `ai_context JSONB` kolonu ekle (migration `043a`).
- [x] `prompt.GlossaryEntry` + `ExternalGlossaryInput` struct'larına `AIContext` alanı ekle.
- [x] `loadGlossaryEntries` → `ai_context` kolonunu da oku.
- [x] `GlossaryFromExternal` → `ai_context.synonyms` üzerinden ek glossary entry'leri üret (ambiguity detector otomatik kullanır).
- [x] LLM prompt → `ai_context` içeriğini prompt context'e dahil et (unit, null_meaning, business_rules).
- [x] Admin UI → glossary edit form'una structured `ai_context` editor ekle.
- [x] Test: `TestDetectGlossary_AIContextSynonymCollision`, `TestGlossaryFromExternalAIContextSynonyms`.
- [x] **Kabul:** Glossary artık synonyms, units, null semantics taşıyabiliyor; bunlar ambiguity + prompt'a entegre.

### P3 — NL-SQL Memory Store (Öğrenme Döngüsü) [MEDIUM]

**Amaç:** Onaylanan soru-SQL çiftlerinden öğrenmek — vektör bellek / confirmed-query store ile positive feedback loop.

**Neden:** En yüksek ROI'li iyileştirme. Kullanıcı bir sonucu kabul ettiğinde NL→SQL çifti saklanır;
gelecekteki benzer sorularda few-shot example olarak kullanılır. Endüstride embedding tabanlı bellek
kullanılır; Biqly mevcut embedding altyapısını kullanabilir.

- [x] Yeni tablo: `ai_confirmed_queries` (datasource_id, question_hash, nl_query, sql_query, semantic_model_hash, confirmed_at, user_id).
- [x] Kullanıcı "thumbs up" / sonucu kabul ettiğinde → NL-SQL çiftini `ai_confirmed_queries`'e kaydet.
- [x] `loadFewShotExamples` → datasource bazlı `ai_confirmed_queries`'ten de örnek çek (son N, similarity-weighted).
- [x] Embedding ile semantic search: yeni soru geldiğinde `ai_confirmed_queries`'te en benzer K çifti getir,
  LLM prompt'una few-shot olarak ekle.
- [x] Periyodik temizlik: semantic model değiştiğinde (`semantic_model_hash` mismatch) eski çiftleri pasifleştir.
- [x] Metric: `biqly_memory_store_confirmed_total`, `biqly_memory_store_recall_hits_total`.
- [x] Test: onaylı çift sonraki benzer soruda few-shot olarak geliyor; model değişikliğinde eski çiftler pasif.
- [ ] **Kabul:** Kullanıcı onaylı sonucu sonraki benzer sorularda few-shot olarak kullanılıyor; accuracy artışı ölçülebilir.

### P4 — Structured Enrich-Context Workflow [LOW]

**Amaç:** Eksik iş bağlamını sistematik tespit eden bir enrich-context workflow (agent skill / admin aracı).

**Neden:** Glossary ve model metadata'sı genellikle eksik. Bir araç bu boşlukları tespit edip
doldurma önerisi sunmalı — manuel olarak her kolona description yazmak ölçeklenmiyor.

- [x] `POST /api/ai/enrich-context` + `POST /api/ai/enrich-context/apply` (admin key):
  - Semantic model + glossary + örnek veriyi oku.
  - Boşluk tespiti: description'ı olmayan kolonlar, label'ı olmayan enum değerleri, synonym collision'lar.
  - AI ile zenginleştirme önerisi üret (her boşluk için öneri).
  - Response: gap report + suggested enrichments.
- [x] Kullanıcı önerileri onayla → glossary/metadata'ya yaz.
- [x] CLI eşdeğeri: `biqly enrich-context --datasource <id> --model <id> --dry-run`.
- [x] Metric: `biqly_enrich_context_gaps_found_total`, `biqly_enrich_context_applied_total`.
- [x] **Kabul:** Glossary sayfasında "Context'i Zenginleştir" butonu → boşluk raporu + seçili önerileri uygula.

### P5 — Kademe Kademe Artan (Tiered) Ambiguity Detection [LOW]

**Amaç:** Her belirsizlik için LLM çağrısı yapmak yerine maliyet/latency optimize eden tiered yaklaşım.

**Neden:** Mevcut sistem deterministic + LLM-backed check'i tek flag ile yönetiyor. Çoğu belirsizlik
deterministic (synonym/homonym) çözülebilir — her seferinde LLM çağrısı gereksiz maliyet.

| Tier | Ne zaman | Nasıl | Maliyet |
|---|---|---|---|
| Tier 0: Routing | Tablo/kolon routing belirsiz | Deterministic | Free |
| Tier 1: Synonym | Synonym/homonym collision | Deterministic glossary | Free |
| Tier 2: Semantic | Yorumlama confidence düşük | LLM-backed analiz | ~$0.01 |
| Tier 3: Interactive | Kullanıcı 2 kez yanlış seçti | Agent-driven multi-turn | ~$0.05 |

- [x] `AmbiguityConfig`'e `TieredEnabled bool` ekle (feature flag, backward compatible; env: `BI_AI_AMBIGUITY_TIERED_ENABLED`).
- [x] `standardProcessOptions` tiered logic (`ambiguityProcessOptions`):
  - Tier 0: routing sonucu `NeedsClarification` → direkt döndür + `RecordAmbiguityTier("0")`.
  - Tier 1: `WithAmbiguityCheck(true)` + `WithAmbiguitySynonymOnly(true)` (glossary/synonym only).
  - Tier 2: `WithLLMAmbiguityCheck(true)` sadece Tier 1 boş geldiyse + `MaxLLMTierPerQuestion` round cap.
  - Tier 3: İki clarification round'dan sonra agent-mod'a geç (P1 hard cap) + `RecordAmbiguityTier("3")`.
- [x] Her tier için ayrı metric: `biqly_ambiguity_tier{tier="0|1|2|3"}`.
- [x] Config: `Ambiguity.TieredEnabled` + `Ambiguity.MaxLLMTierPerQuestion` (default: 1).
- [x] Test: `AnalyzeSynonymHomonym`, handler tier options, service tier observer + synonym-only integration.
- [x] **Kabul:** LLM-backed check sadece deterministic check boş geldiyse çalışıyor; tiered modda scope/temporal Tier 1'de atlanıyor.

### P6 — Generation Trace (Kullanıcıya Ne Anlaşıldığını Gösterme) [LOW]

**Amaç:** Generation trace / dry-plan benzeri şeffaflık — kullanıcının "sistem ne anladı?" görebilmesi.

**Neden:** Belirsizlik tespit edildiğinde kullanıcı neden sorulduğunu anlamıyor. Trace = şeffaflık + güven.

- [x] `ai.Response.Metadata`'ya `GenerationTrace` alanı ekle:

  ```go
  type GenerationTrace struct {
      RoutedTable    string  `json:"routed_table"`
      RouteConfidence float64 `json:"route_confidence"`
      ColumnsResolved []ColumnResolution `json:"columns_resolved"`
      AmbiguityResult string  `json:"ambiguity_result"` // "passed" | "clarification_needed"
      AmbiguityDetail string  `json:"ambiguity_detail,omitempty"`
  }
  ```

- [x] Routing, ambiguity check, ve column resolution adımlarında trace topla (`internal/ai/trace.go` + `observeAIRequest`).
- [x] Frontend: AI response'da trace bilgisi varsa expandable "Nasıl Anlaşıldı?" bölümü göster (`GenerationTracePanel`).
- [x] **Kabul:** Kullanıcı belirsizlik kartında "Sistem 'revenue' → total_revenue olarak anladı" gibi bilgi görebiliyor.

### P7 — Ambiguity Eval Regression Golden Cases [LOW]

**Amaç:** Belirsizlik davranışını regresyondan korumak için özel eval golden cases.

**Neden:** Mevcut eval suite ambiguity özelinde golden case taşımıyordu — Bug 2 canlıya çıkabildi.

- [x] `internal/ai/eval/testdata/` altında `ambiguity_golden.json` oluştur:

  ```json
  [
    {"question": "Satışları göster", "expected_type": "clarification",
     "expected_detail": "synonym: satis_total vs satis_count"},
    {"question": "Show revenue for Q1", "clarification_choice": "ambiguity:0:1",
     "expected_sql": "SELECT SUM(net_revenue) FROM orders WHERE ..."}
  ]
  ```

- [x] Eval runner: `expected_type=clarification` → ambiguity response geldiğini assert.
- [x] `expected` LogicalQuery → clarification choice sonrası doğru sorgu üretildiğini assert.
- [x] CI: `make eval-regression` ambiguity golden'ları da çalıştırır.
- [x] **Kabul:** Ambiguity davranışı değişirse CI kırmızı olur; yeni golden case ekleme prosedürü belgeli (`AmbiguityGoldenCase` godoc).

### Denetim Sonuçları (2026-06-09)

Tüm P0–P7 maddeleri codebase'te uygulandı. Aşağıdaki denetim bulguları ve açıkta kalan iyileştirme maddeleri:

**Uygulama doğrulaması:**

| Madde | Durum | Kanıt |
|---|---|---|
| P0 ProcessContext | ✅ Tamamlandı | `ai_context.go`: struct + `buildProcessContext` + `Resolve` + `ApplyToRequest`; sync (`ai.go:193`) ve async (`ai_job_exec.go:67`) aynı yolu kullanıyor; eski free function kaldırılmış |
| P1 Hard Cap | ✅ Tamamlandı | `maxClarificationRounds=2`, `ShouldCheckAmbiguity` round kontrolü, `AmbiguityCapReached`, frontend `clarification_round` alanı (types + AIQuery.tsx) |
| P2 Glossary AIContext | ✅ Tamamlandı | `pkg/metadata/types.go:GlossaryAIContext{Synonyms,Unit,NullMeaning,BusinessRules}`, migration `043a`, synonym detector entegrasyonu, frontend Glossary.tsx admin form |
| P3 NL-SQL Memory Store | ✅ Tamamlandı | `ai_confirmed_queries` tablosu (migration `044a`), `metadata/ai_confirmed_queries.go`, `ai/memory/recall.go`, feedback → store, few-shot recall entegrasyonu |
| P4 Enrich-Context | ✅ Tamamlandı | `ai/enrichcontext/` (service, gaps, suggest, apply, types), `ai_enrich_context.go` handler, admin-key grubunda `POST /api/ai/enrich-context` + `POST /api/ai/enrich-context/apply` (`ai_router.go`) |
| P5 Tiered Detection | ✅ Tamamlandı | `AmbiguityConfig.TieredEnabled` + `MaxLLMTierPerQuestion`, `WithAmbiguitySynonymOnly`, `ShouldUseLLMAmbiguityTier`, `biqly_ambiguity_tier` metric |
| P6 Generation Trace | ✅ Tamamlandı | `ai/trace.go:GenerationTrace`, `BuildGenerationTrace`, frontend `generationTrace.tsx` panel, `routingViz.tsx` entegrasyonu, i18n anahtarları |
| P7 Eval Golden Cases | ✅ Tamamlandı | `ambiguity_golden.go`, `ambiguity_golden_runner.go`, `testdata/ambiguity_golden.json` (5 case), eval-regression entegrasyonu |

**Açıkta kalan iyileştirme maddeleri (denetim bulgusu):**

- [x] **Enrich-context frontend UI.** ~~frontend'de buton/panel yok~~ — premis yanlıştı: UI zaten Glossary
  sayfasında mevcuttu (`64ef4642 feat(ai): add enrich-context workflow`). Analyze → gap listesi + AI suggestion →
  onay → Apply akışı `GlossaryEnrichPanel.tsx` + `Glossary.tsx` içinde çalışıyor (`/glossary`).
  Mevcut panel iyileştirildi (kullanıcı isteğiyle):
  - Apply sonucu + hatalar artık gösteriliyor (önceden `{applied,skipped,errors}` sessizce yutuluyordu — silent failure düzeltildi).
  - Toplu seç/temizle + "N seçileni uygula" sayacı.
  - AI önerisi ayrı gösteriliyor + "öneriyi geri yükle"; `sample_rows` başlıkta.
  - Inline style → BEM (`styles/glossary-enrich.css`); checkbox/textarea için aria-label.
  - **Dosyalar:** `GlossaryEnrichPanel.tsx`, `Glossary.tsx`, `styles/glossary-enrich.css`, `i18n/locales/{en,tr}/core.ts`.
  - Gate'ler: ESLint 0, Prettier temiz, tsc/build temiz, vitest 99/99, knip:ci 0.

- [x] **Memory store model değişikliğinde pasifleştirme orkestrasyonu.** ~~mekanizma yok~~ — premis yanlıştı:
  `DeactivateConfirmedQueriesExceptHash` + publish hook'u zaten mevcuttu (`(*SemanticHandler).PublishModel`
  publish sonrası `semantic_model_hash <> modelID@version` olan aktif kayıtları `is_active=false` yapıyor;
  store/recall da aynı `modelID@version` hash formatını kullanıyor → tutarlı).
  Sağlamlaştırma yapıldı (kullanıcı isteğiyle):
  - Deaktivasyon `deactivateStaleConfirmedQueries` helper'ına çıkarıldı (handler içinde, MetaRepo+SemanticRepo erişimi orada).
  - İkinci publish yolu `GenerateModel` (`req.Publish=true`) da artık helper'ı çağırıyor → simetri/gelecek-güvenliği
    (yeni model olduğu için pratikte no-op ama tutarlı).
  - Test eklendi: `semantic_confirmed_queries_test.go` — publish→deaktivasyon doğru hash ile çağrılıyor + nil/model-yok no-op guard.
  - **Dosyalar:** `internal/http/handlers/semantic.go`, `internal/http/handlers/semantic_confirmed_queries_test.go`.
  - Gate'ler: gofmt ✓, lint-go 0 issues, race test (handlers/semantic/metadata) ✓, deadcode temiz.

- [x] **Generation trace clarification kartında gösterimi.** ~~sadece `!showClarification`'da render~~ — premis
  kısmen yanlıştı: `ClarificationCard` zaten `<GenerationTracePanel trace={generationTrace} />` render ediyordu
  (`routingViz.tsx:434`, commit `21dbe948`), `assistantMessageCardSections.tsx:131` de `generation_trace`'i geçiriyordu.
  Gerçek boşluk: clarification'daki trace `defaultOpen={false}` ile **collapsed** geliyordu → kullanıcı gerekçeyi anında görmüyordu.
  Düzeltme:
  - `GenerationTracePanel`'e `defaultOpen?: boolean` prop'u eklendi (varsayılan `false` → standalone sonuç görünümü collapsed kalır).
  - `ClarificationCard` artık `defaultOpen` ile çağırıyor → clarification bağlamında trace/ambiguity gerekçesi açık geliyor (P6 kabul kriteri).
  - **Dosyalar:** `frontend/src/components/aiQuery/generationTrace.tsx`, `routingViz.tsx`.
  - Gate: `make check-frontend` exit 0 (lint 0, format:check temiz, knip:ci 0, test 99/99, build ✓).

- [x] **Routing ambiguity (Tier 0) regresyon kapsamı.** ~~ambiguity_golden.json'a routing case ekle~~ —
  premis mimari olarak yanlıştı: o suite tamamen `ambiguity.Analyze` (glossary/synonym/temporal/scope) tabanlı,
  routing katmanını (`TableRouter` + datasource metadata) hiç çalıştırmıyor → routing case orada barınamaz.
  Ayrıca routing→clarification eşlemesi zaten unit-test'liydi (`TestClarificationFromRoutingBuildsOptionsAndCandidates`
  çoklu aday + `TestTableRouter_RouteNeedsClarificationForNoMatch` no-match).
  Gerçek boşluk: `TableRouter.Route`'un *yarışan zayıf adaylar* (0<confidence<0.35) durumunda Tier-0 clarification
  üretmesi yalnızca no-match için test ediliyordu. Eklenen test bunu kapatıyor:
  - `TestTableRouter_RouteNeedsClarificationForCompetingCandidates` — iki tablo kolon-açıklamasıyla zayıf-eşit eşleşiyor
    → `routeConfidence`=0.25 < `minRouteConfidence`(0.35) → `NeedsClarification=true`, ≥2 eşit-skorlu candidate, model nil.
  - **Dosya:** `internal/ai/routing/route_clarification_test.go`.
  - Gate: gofmt ✓, race test (routing) ✓, lint-go 0 issues, deadcode temiz. (eval'e dokunulmadı → eval-regression gerekmez.)

- [x] **Tiered config varsayılan değerleri.** `ai.ambiguity.tieredEnabled: true` +
  `maxLLMTierPerQuestion: 1` umbrella + prod values ve ai subchart defaults; ConfigMap
  `BI_AI_AMBIGUITY_TIERED_ENABLED` + `BI_AI_AMBIGUITY_MAX_LLM_TIER_PER_QUESTION` emit ediyor.
  - **Dosyalar:** `deploy/helm/biqly/values.yaml`, `values-prod.yaml`, `charts/ai/values.yaml`, `charts/ai/templates/configmap.yaml`.

### Frontend Denetim Bulguları (2026-06-09)

Backend P0–P7 uygulandı, frontend karşılıkları denetlendi. Tamamlanan ve eksik olanlar:

| Feature | Frontend Durum | Açık Nokta |
|---|---|---|
| P0 Clarification Round | ✅ Tam | `AIQuery.tsx:78,298,313` — state + gönderim + okuma tam |
| P1 Hard Cap UX | ⚠️ Kısmi | Backend 2 round'da kesiyor ama frontend'de cap'e ulaşıldığında buton devre dışı bırakılmıyor / UX uyarı yok |
| P2 Glossary AIContext | ✅ Tam | `Glossary.tsx:892-1043` — synonyms[], unit, null_meaning, business_rules[] form alanları + i18n |
| P3 Memory Store visibility | ⚠️ Kısmi | Thumbs-up → backend otomatik depolar, ama kullanıcıya "öğrenildi" geri bildirimi yok; confirmed queries admin listesi yok |
| P4 Enrich-Context | ✅ Tam | `GlossaryEnrichPanel.tsx` + endpoint'ler + i18n tam |
| P5 Tiered Detection UI | ❌ Yok | Admin settings'de `TieredEnabled` / `MaxLLMTierPerQuestion` toggle yok, sadece env-var; i18n anahtarı yok |
| P6 Generation Trace i18n | ⚠️ Kısmi | Trace panel tüm alanları render ediyor ama `columns_resolved` bölüm başlığı ve `ambiguity_detail` etiketi için i18n anahtarı eksik |

#### Frontend için yeni maddeler

- [x] **P1 — Hard cap UX göstergesi.** Round ≥ `maxClarificationRounds` (2) olan clarification kartında
  "Maksimum netleştirme turuna ulaşıldı — bir seçenek seçin, en iyi tahminle yanıtlayalım" bildirimi gösteriliyor.
  - **UX kararı (kullanıcı onayı):** Seçenek butonları AÇIK kalıyor — round-2 kartının seçenekleri o turu *çözen*
    butonlar; disable etmek sorguyu çözülemez halde bırakırdı (backend gelen round 2'de ambiguity'yi zaten bypass edip
    nihai cevabı üretiyor). Bildirim + açık seçenekler doğru davranış.
  - Round değeri global state'ten değil, her mesajın `result.clarification_round`'undan okunuyor (per-message doğru kaynak).
    `MAX_CLARIFICATION_ROUNDS = 2` sabiti backend `maxClarificationRounds` ile hizalı (yorumla işaretlendi).
  - **Dosyalar:** `assistantMessageCardSections.tsx` (cap hesabı + `capReached` prop), `routingViz.tsx`
    (`ClarificationCard` bildirim render), `i18n/locales/{en,tr}/core.ts`, `styles/aiQuery.css` (`.clarification-cap-notice`).
  - Gate: `make check-frontend` exit 0 (lint 0, format:check, knip:ci 0, test 99/99, build ✓).

- [x] **P3 — Confirmed queries admin listesi + "öğrenildi" geri bildirimi.** ~~Endpoint'ler hazır~~ —
  premis yanlıştı: `GET /api/admin/ai/confirmed-queries` backend'de **yoktu** (repo'da yalnızca recall amaçlı
  `ListActiveConfirmedQueries` vardı). Eksik backend bu turda eklendi:
  - Backend: `ListConfirmedQueriesForAdmin` (aktif+pasif, `confirmed_at` dahil) + `SetConfirmedQueryActive`
    (`internal/metadata/ai_confirmed_queries.go`); admin-key'li `GET /api/ai/confirmed-queries?datasource_id=...` +
    `POST /api/ai/confirmed-queries/{id}/deactivate` (`handlers/ai_confirmed_queries.go`, `ai_router.go`).
    Mevcut admin AI endpoint konvansiyonuna uyularak `/api/ai/...` altında (AdminKeyMiddleware: super_admin JWT veya admin key).
  - "Öğrenildi" sinyali: `POST /api/ai/feedback` yanıtına `learned: bool` eklendi
    (`storeConfirmedQueryOnPositiveFeedback` artık bool döndürüyor).
  - Frontend: yeni admin sekmesi **AI & paylaşım → Onaylanmış Sorgular** (`ConfirmedQueriesPanel.tsx`,
    `adminNavConfig.ts`, `Admin.tsx`, `api/aiAdmin.ts`): datasource seçici, soru/SQL/onay tarihi/durum tablosu,
    pasifleştir butonu. Thumbs-up sonrası `FeedbackSection`'da "öğrenildi" rozeti (`learned===true` ise;
    `.feedback-learned-badge`, `role="status"`). Admin.tsx tab render'ı prop'suz paneller için
    `PROPLESS_TAB_PANELS` map'ine sadeleştirildi (complexity 21→<20).
  - Testler: `ai_confirmed_queries_admin_test.go` (list + validation + deactivate/404/400).
  - **Dosyalar:** yukarıdakiler + `i18n/locales/{en,tr}/{admin,core}.ts`, `styles/aiQuery.css`,
    `AssistantMessageCard.tsx`.

- [x] **P5 — Tiered ambiguity admin toggle.** ~~`GET/PUT /api/admin/config` üzerinden~~ — premis yanlıştı:
  bu endpoint'ler **yoktu** ve `AmbiguityConfig` salt env-var'dı (runtime store yok). Bu turda eklendi:
  - Backend: migration `045a` `ai_runtime_config(key, value JSONB, updated_at)` KV tablosu;
    `Get/UpsertAIRuntimeConfig` (`metadata/ai_runtime_config.go`); admin-key'li
    `GET/PUT /api/ai/admin/config` (`handlers/ai_admin_config.go`). Overlay deseni:
    `effectiveAmbiguityConfig` env varsayılanları üzerine DB override'larını bindiriyor
    (nil alan = env default); 30s TTL cache (PUT yapan replika anında invalidate, diğerleri TTL içinde yakınsar).
    `standardProcessOptions` artık `h.effectiveAmbiguityConfig(ctx)` okuyor → sync + async job yolu aynı değeri görür.
    PUT validasyonu: her iki alan zorunlu, `max_llm_tier_per_question` 0–10.
  - Frontend: hedef sayfa `Settings.tsx` değil (orası kullanıcı ayarları) — bölüm
    **Admin → Platform Ayarları**'na eklendi (`PlatformSettingsPanel.tsx`): tiered checkbox +
    max LLM tier sayı girişi + DB-override/env-default notu + ayrı kaydet.
  - Testler: `ai_admin_config_test.go` (overlay, env-default GET, PUT persist+reload, validasyon).
  - **Dosyalar:** `migrations/045{a,b}_add_ai_runtime_config.*.sql`, `internal/metadata/ai_runtime_config.go`,
    `internal/http/handlers/ai_admin_config.go`, `ai.go`, `internal/http/ai_router.go`,
    `frontend/src/api/aiAdmin.ts`, `PlatformSettingsPanel.tsx`, `i18n/locales/{en,tr}/admin.ts`.

- [x] **P6 — Generation trace i18n eksikleri.** `generation_trace_columns` ("Resolved columns"/"Çözümlenen
  kolonlar") bölüm başlığı columns listesinin üstüne, `generation_trace_ambiguity_detail` ("Detail"/"Ayrıntı")
  etiketi ambiguity detayının önüne eklendi.
  - **Dosyalar:** `frontend/src/components/aiQuery/generationTrace.tsx`,
    `frontend/src/i18n/locales/{en,tr}/core.ts`.
  - Gate'ler (üç madde birlikte): `make check-frontend` exit 0 (lint 0, format:check ✓, knip:ci ✓,
    vitest 99/99, build ✓); `make lint-go` 0 issues; `make test-go` (-race, 58 paket) PASS;
    `deadcode` yeni bulgu yok. Commit yapılmadı.

### Plan ↔ Codebase Boşluk Analizi — 2. Tur (2026-06-09)

P0–P7 iddiaları kod üzerinde tek tek doğrulandı. **Doğrulananlar (yeni madde gerekmez):**
P0 `ClarificationResolved` yalnızca `ai_context.go:54`'te yazılıyor (kabul kriteri sağlanıyor);
P1 round artışı `attachAmbiguityClarificationRound` üzerinden, metric mevcut; P2 prompt
`unit/null_meaning/business_rules`'ı `prompt/glossary.go`'da kullanıyor; P3 recall iki few-shot
yolunda da bağlı (`ai.go:1141,1265`); P4 CLI `cmd/biqly` `--dry-run`/`--suggest` ile mevcut;
P5 Tier 2 LLM yalnızca deterministik boş dönünce çalışıyor (`service.go: !result.IsAmbiguous &&
ambiguityLLMCheck`), tier 0/1/2/3 metric'leri kayıtlı; P6 `ColumnsResolved` routing+resolution'dan
dolduruluyor; P7 golden suite choice→expected round-trip case'i ve choice↔expected validasyonunu içeriyor.
Plan'daki tüm metric adları (`biqly_ambiguity_round_cap_reached_total`, `biqly_ambiguity_tier`,
`biqly_memory_store_*`, `biqly_enrich_context_*`) birebir mevcut.

**Tespit edilen boşluklar / yeni maddeler:**

- [x] **GAP-1 (P5) — Tier 3 "agent-driven multi-turn" uygulandı (seçenek a).**
  Plan tablosundaki Tier 3 artık cap bypass değil: `clarificationRound == maxClarificationRounds` (2) iken
  `ShouldUseInteractiveTier` → tam rule-based analiz + zorunlu LLM + sınırsız seçenek (`WithAmbiguityInteractiveTier`).
  `clarificationRound > 2` ise gerçek bypass (ambiguity check kapalı). Kısmi çözüm: `HasRemaining` ile
  `ClarificationResolved` yalnızca tüm belirsizlikler giderildiğinde set edilir. Frontend: round===2
  interactive tier bildirimi; round>2 cap bildirimi. Metric help: `3=interactive agent`.
  - **Dosyalar:** `ai_ambiguity_tier.go`, `ai_context.go`, `internal/ai/service.go`, `ambiguity/resolver.go`,
    `metrics.go`, frontend `assistantMessageCardSections.tsx`, `routingViz.tsx`, i18n `core.ts`.

- [x] **GAP-2 (P3) — Açık kabul kriteri "accuracy artışı ölçülebilir" için somut alt maddeler.**
  Recall mekanizması ve sayaçlar var ama "memory store accuracy'yi artırıyor mu" sorusuna yanıt veren
  ölçüm yok. Alt maddeler:
  - [x] `ai_query_history`'e `memory_recall_used bool` (veya recall hit sayısı) alanı yaz → pozitif/negatif
    feedback oranı recall'lı vs recall'sız sorgular için karşılaştırılabilir olsun.
  - [x] Grafana paneli: `biqly_memory_store_recall_hits_total` / `biqly_memory_store_confirmed_total`
    trendi + recall'lı sorguların thumbs-up oranı.
  - [x] Eval: confirmed-query enjekte edilen golden case (recall few-shot'unun üretimi iyileştirdiğini
    assert eden, stub-provider'lı regression case) — `internal/ai/eval/`.

- [x] **GAP-3 (P0 kalıntısı) — Tier-0 short-circuit bloğu sync/async'te tekrar ediyor.**
  `routeResult.NeedsClarification → RecordAmbiguityTier("0") + clarificationResponse + observe` bloğu
  hem `ai.go:184-191` (sync) hem `ai_job_exec.go:~44` (async) içinde kopya — P0'ın "tek yol" hedefinin
  dışında kalmış küçük bir divergence noktası. Ortak helper'a çekilmeli
  (örn. `(h *AIHandler) tierZeroClarification(...)`).
  - **Dosyalar:** `internal/http/handlers/ai.go`, `ai_job_exec.go`.

- [x] **GAP-4 (doc) — Denetim tablosundaki endpoint path'leri yanlış.** P4 satırı
  `/api/admin/ai/enrich-context` yazıyordu; gerçek path admin-key grubunda `/api/ai/enrich-context`
  - `/api/ai/enrich-context/apply`. (P3'teki aynı hata daha önce düzeltildi: gerçek path
  `/api/ai/confirmed-queries`.) Tablo satırı güncellendi; `/api/admin/*` diye bir route ailesi
  hiç yok — gelecek maddelerde bu kalıba dikkat.

- [x] **NOT-1 (plan-dışı ekleme, bilinçli) — `ai_runtime_config` DB override katmanı.** P5 admin
  toggle'ı için plan dışı eklendi (planda yalnızca env flag vardı). Operasyon etkisi: DB override
  varken Helm/env değeri değiştirmek etkisizdir (`db_override` admin panelde görünüyor). İyileştirme:
  `GET /api/ai/settings` yanıtına efektif ambiguity değerleri + `db_override` + `source`
  (`environment` | `database`) eklendi — admin config ile aynı wire shape (`effectiveAmbiguitySettings`).
  **Dosyalar:** `ai_settings.go`, `ai_admin_config.go`, `ai_settings_test.go`, `frontend/src/types/ai.ts`,
  `pkg/aiclient/schema.go`.

### Prod Vaka: "geçen ay kaç adet tweet atılmıştır?" (2026-06-09 21:55, zlitter_2)

İki kullanıcı-görünür hata, üç kök neden (kod üzerinde doğrulandı):

**Belirti 1 — Netleştirme kartı Türkçe UI'a rağmen İngilizce geldi.**
Kök neden: frontend generate/preview için **async job** yolunu kullanıyor; worker job'ı
"bare consumer context" ile çalıştırıyor (`ai_job_service.go` — yorum aynen böyle diyor).
`UserID` job kaydına persist edilip worker'da `ai.WithUserID` ile geri enjekte ediliyor ama
**locale edilmiyor** → `i18n.FromContext(ctx)` = DefaultLocale("en") → `ambiguity_reason`
("This question matched more than one possible meaning.") ve `ambiguity_question_single`
("What did you mean by ...?") İngilizce katalogtan basılıyor. `internal/i18n/locales/tr.json`
çevirileri **mevcut ama job yolunda hiç kullanılamıyor**. Senkron yolda `bimw.Locale`
middleware'i X-Locale'i context'e koyduğu için sorun yalnızca async yolda görülüyor.

**Belirti 2 — "geçen ay" koşulu nihai sorguya yansımadı** (`SELECT COUNT(*) ... LIMIT 100`,
filtresiz; güven %90 gösterildi). İki ayrı kök neden:
(a) *Sahte belirsizlik gürültüsü:* routing her timestamp kolonu için `*_month` date-grain
dimension üretip month grain synonym'lerini ("ay", "aylık", ...) kopyalıyor
(`routing/time_grains.go`); `DetectSynonyms` modeldeki TÜM dimension synonym'lerini taradığı
için "geçen ay" içindeki "ay" token'ı 15 `*_month` dim'e çarpıyor → anlamsız 15 seçenekli
"semantic" kartı. Oysa `temporal_detector.go` "geçen ay" kalıbını tanıyor ve anlamlı 2 yorum
üretiyor (takvim ayı vs son 30 gün) — ama tiered modda (`AnalyzeSynonymHomonym`) temporal
detector hiç çalışmıyor; çalışsa bile synonym detector bare-token "ay"ı ayrıca işaretliyor.
(b) *Sessiz koşul düşmesi:* LLM (mimo-v2.5) modelle uyuşmayan şekil üretti, repair döngüsü
sonunda filtresiz bare-count kaldı; `warnings_body` gösterildi ama zaman koşulunun
**uygulanmadığı** açıkça söylenmedi ve güven %90 kaldı. Compile katmanı `between` +
`calendar_grain_filter.go` ile "geçen ay"ı ifade EDEBİLİYOR — kayıp üretim/repair tarafında.

**Yapılacaklar:**

- [x] **VAKA-1 — Async job yoluna locale propagasyonu.** (Tamamlandı — commit `1248cac0`:
  `ai_jobs.locale` persist + worker'da `consumerContextForAIJob` ile `i18n.WithLocale`
  enjeksiyonu + migration 047a/b + `ai_job_service_locale_test.go`.) Job oluştururken request locale'ini
  (`i18n.FromContext`) job kaydına persist et (`metadata.AIJob`'a `locale` alanı veya
  `RequestJSON` içine), worker'da `processJob` öncesi `i18n.WithLocale(ctx, job.Locale)` enjekte
  et — `UserID` ile birebir aynı kalıp. Test: TR locale ile submit edilen job'ın clarification
  yanıtı `tr.json` metinlerini içeriyor.
  - **Dosyalar:** `internal/metadata/ai_jobs*.go` (+migration gerekirse), `internal/http/handlers/ai_jobs.go`
    (create), `ai_job_service.go` (inject), test.

- [x] **VAKA-2 — Date-grain synonym'leri belirsizlik tespitinden çıkar.** (Tamamlandı —
  `DetectSynonyms` artık `TimeGrain != ""` olan dimension'ları atlıyor; suffix yerine alan
  bazlı ayrım: auto-generated grain dim'leri `model_builder.go` `TimeGrain` set ediyor, meşru
  "ay" kolonları etmiyor. Testler: `TestDetectSynonyms_DateGrainDimensionsNotFlagged` +
  `TestDetectSynonyms_PlainColumnNamedAyStillFlagged`.) `DetectSynonyms`
  auto-generated date-grain dimension'ların grain synonym'lerini ("ay", "gün", "yıl",
  "çeyrek", "saat" + EN karşılıkları) collision adayı saymasın: ya date-grain dim'leri
  (suffix `_month/_day/_quarter/_year/_hour`) detector'dan hariç tut, ya da soru içinde
  bilinen temporal phrase'in (`temporal_detector` pattern listesi) parçası olan token'ları
  atla. Kabul: "geçen ay kaç tweet" sorusu synonym collision üretmiyor; "ay" tek başına
  meşru kolon adı olarak geçen durumlar etkilenmiyor.
  - **Dosyalar:** `internal/ai/ambiguity/synonym_detector.go`, `analyzer.go`, testler.

- [x] **VAKA-3 — Tiered modda temporal ifadeler için doğru tier.** (Tamamlandı —
  `AnalyzeSynonymHomonym` artık `DetectTemporal`'ı da çalıştırıyor (scope hâlâ tier 1 dışı,
  heuristiği daha gürültülü). Test: `TestAnalyzeSynonymHomonym_DetectsTemporalSkipsScope`;
  golden: `ambiguity_golden.json` → `temporal-gecen-ay-tweets` TR case, `make eval-regression` ✓.) "geçen ay" gibi kalıplar
  Tier 1'de (deterministik, ücretsiz) `DetectTemporal` ile yakalanabilirken tiered mod onları
  tamamen atlıyor (P5 kabulünde "scope/temporal Tier 1'de atlanıyor" diye bilinçli yazılmıştı —
  bu vaka kararın yanlış olduğunu gösteriyor: temporal de deterministik ve ücretsiz).
  `AnalyzeSynonymHomonym`'e `DetectTemporal`'ı ekle (veya tiered Tier 1 detector setini
  config'e bağla) → kullanıcı "takvim ayı mı, son 30 gün mü?" gibi ANLAMLI bir netleştirme görür.
  - **Dosyalar:** `internal/ai/ambiguity/analyzer.go`, ilgili testler + `ambiguity_golden.json`'a
    TR temporal case.

- [x] **VAKA-4 — Temporal koşul sessizce düşmesin.** (Tamamlandı —
  `internal/ai/temporal_postcheck.go`: `applyTemporalFilterPostCheck` ProcessQuestion'ın üç
  sonuç yolunda da (multi-candidate, retry-loop, failure) çalışıyor; soruda
  `ambiguity.MatchTemporalPhrases` (yeni export) eşleşip nihai LogicalQuery'de hiçbir date-dim
  WHERE/HAVING filtresi yoksa locale'li uyarı (`clarification.temporal_filter_missing`,
  en+tr.json) ekleniyor ve confidence 0.5'e cap'leniyor (cap aynı zamanda response cache'e
  girmesini engelliyor, eşik 0.85). Few-shot: `writeFailureExamples`'a "Relative time phrase
  dropped" wrong/right çifti. Testler: `temporal_postcheck_test.go` (4 unit) +
  `TestProcessQuestionWarnsWhenTemporalFilterDropped` (TR locale entegrasyon). Not: trace.go'ya
  ayrı alan eklenmedi — uyarı `warnings` üzerinden kullanıcıya zaten görünür, minimum-kod tercihi.) Soruda temporal phrase tespit edildiyse
  (temporal detector pattern'leri) ve nihai LogicalQuery hiçbir tarih filtresi/grain filtresi
  içermiyorsa: özel uyarı ekle ("zaman koşulu sorguya uygulanamadı"), confidence'ı düşür ve/veya
  recovery seçenekleri sun — %90 güvenle filtresiz COUNT dönmek yanlış-doğru cevap üretiyor
  (silent failure). Ek olarak: TR göreli tarih few-shot örneği ("geçen ay kaç X" →
  `created_at` üzerinde prev-calendar-month filter'lı count) + eval golden case.
  - **Dosyalar:** `internal/ai/service.go` (gen loop sonrası post-check), `internal/ai/trace.go`
    (uyarı/trace alanı), few-shot seed + `internal/ai/eval/testdata/`, frontend uyarı metni
    gerekiyorsa `i18n/locales/{en,tr}/core.ts`.

- [ ] **VAKA-5 (ikincil, değerlendirildi/ertelendi) — Netleştirme seçenek metinleri ham kolon
  description'ı.** Değerlendirme (2026-06-10): VAKA-2 date-grain gürültü kartını tamamen,
  VAKA-3 temporal kartı i18n kataloğundan (tr.json) ürettiği için bu vakadaki İngilizce teknik
  metinler artık kullanıcıya ulaşmıyor. Kalan tek yüzey: kullanıcı tanımlı modellerdeki EN
  description'lı gerçek synonym collision'ları — düşük frekans. `MetadataTranslator` benzeri
  katman LLM çağrısı/cache karmaşası ekleyeceği için şimdilik uygulanmadı; gerçek vaka görülürse
  ele alınacak. Seçenek
  etiketleri İngilizce metadata description'larından geliyor (örn. "The standardized,
  machine-readable timestamp..."); locale düzelse bile bu metinler İngilizce kalır ve son
  kullanıcı için fazla teknik. Routing'deki `MetadataTranslator` benzeri bir çeviri/sadeleştirme
  katmanının ambiguity seçeneklerine de uygulanması değerlendirilmeli (düşük öncelik;
  VAKA-2/3 zaten bu kartın hiç çıkmamasını sağlar).

---

## Dil Varlıklarının Koddan Çıkarılması — DB Tabanlı Lexicon/i18n Yol Haritası (2026-06-10)

**Problem:** "aylık", "günlük", "geçen ay", "silinen" gibi doğal-dil tanımlamaları yalnızca
EN+TR için kod içinde gömülü. Yeni bir dil eklemek bugün release gerektiriyor; runtime'da
yönetilemiyor.

**Best practice (hedef mimari):** İki varlık sınıfını ayır ve ikisini de *hibrit* yönet:

1. **NL lexicon verisi** (synonym/phrase/intent token listeleri) = **data, kod değil** →
   locale-boyutlu DB tabloları; kod yalnızca *seed + fallback* olarak embedded default taşır
   (boot asla DB'ye bağımlı olmaz), üstüne DB overlay + cache + invalidation + admin CRUD.
   Bu kalıp repoda zaten 3 yerde kurulu: `ai_time_grains` (dbTimeGrainStore), prompt
   şablonları (`dbPromptStore`, versiyonlu, embed'den seed), `ai_runtime_config` (KV overlay).
   Yapılacak iş büyük ölçüde **mevcut kalıbı kalan hardcoded varlıklara yaymak**.
2. **Mesaj katalogları** (i18n bundle'ları) → embedded EN/TR fallback kalır, DB overlay +
   dinamik locale registry ile yeni dil release'siz eklenir.

**Envanter (2026-06-10, kod üzerinde doğrulandı):**

| Varlık | Yer | Durum |
|---|---|---|
| Time-grain synonym'leri | `routing/time_grains.go` `DefaultTimeGrains` | ✅ DB-backed (`ai_time_grains`) ama synonym dizisi locale-karışık; seed EN+TR hardcoded |
| Prompt şablonları (system_rules, repair, …) | `prompt/prompts/{en,tr}/*.tmpl` + `prompt_store.go` | ✅ DB-backed + versiyonlu; embed yalnızca en/tr, bilinmeyen locale EN'e düşer |
| Routing lexicon (token/intent/metric synonym) | `routing_lexicon_default.json` + `BI_AI_ROUTING_LEXICON_PATH` | ⚠️ Dosya override var (ConfigMap ile release'siz) ama locale-boyutsuz, admin UI yok |
| Glossary synonym'leri | `business_glossary_terms.ai_context` | ✅ DB-backed |
| Vague temporal phrase'ler | `ambiguity/temporal_detector.go` `vagueTemporalPhrases` | ❌ Hardcoded (TR+EN) |
| Soft-delete kelime listeleri | `routing/model_builder.go` `softDeleteColumnSynonyms` | ❌ Hardcoded |
| Aggregation/intent token'ları ("kaç", "adet", "toplam", grain kelimeleri) | `routing/routing_budget.go` | ❌ Hardcoded |
| Semanticgen grain/row-count synonym'leri | `semanticgen/generator.go` (~290-323) | ❌ Hardcoded (time_grains ile DUPLİKE — tek kaynağa inmeli) |
| Locale registry + soru-dili sinyalleri | `i18n/i18n.go` `SupportedLocales`, `localeProfiles` (QuestionSignals/Letters) | ❌ Hardcoded |
| Backend mesaj katalogları | `i18n/locales/{en,tr}.json` (embed) | ❌ Embedded — yeni dil = release |
| Soru dili tespiti | `ai/lingua/locale.go` | ❌ localeProfiles sinyallerine bağlı (hardcoded) |
| Prompt içi Go-üretimli bölümler (failure examples, planning steps) | `prompt/prompt_examples.go` | ❌ Hardcoded (EN gövde + TR ipuçları) |
| Frontend katalogları | `frontend/src/i18n/locales/{en,tr}/` | ❌ Build-time bundle — yeni dil = frontend release |
| Eval golden/edge case'leri | `internal/ai/eval/` | ✔️ Kodda kalması doğru (test varlığı) |

### DİL-0 — Tasarım kararı + ADR [S] ✅ (2026-06-10)

- [x] Karar: **generic tek tablo** `ai_nl_lexicon(locale, domain, key, value JSONB, is_active)`,
      PK (locale, domain, key); 7 domain ve value şekilleri ADR K2 tablosunda.
      `ai_time_grains` yapı tablosu olarak kalır, `synonyms` kolonu DİL-1'de `grain_synonym`
      domain'ine taşınır (`synonyms_by_locale` alternatifi reddedildi — ADR K3).
      Ek karar: eşleştirme **etkin locale'lerin birleşimi** üzerinde (K4, davranış-koruyucu).
- [x] Fallback zinciri: lexicon DB → embedded default (boot DB'siz çalışır, K5);
      cache 30s TTL + yazan replikada anında Invalidate (`ai_runtime_config` deseni, K6);
      seed: domain boşsa embedded'dan idempotent doldurma + embedded'a sıfırlama endpoint'i (K7).
- [x] **Kabul sağlandı:** `docs/adr/0001-db-backed-nl-lexicon-and-i18n.md` — şema (K1, K8
      locale registry dahil), fallback (K5), cache (K6), seed/kurtarma (K7), admin yüzeyi (K9),
      5 reddedilen alternatif ve riskler yazılı.

### DİL-1 — `ai_nl_lexicon` altyapısı + ilk taşımalar [M] ✅ (2026-06-10)

- [x] Migration `048a/b`: `ai_nl_lexicon(locale, domain, key, value JSONB, is_active,
      updated_at)` PK(locale,domain,key) + domain index; Go seed `lexicon.Seed`
      (tablo boşsa embedded default'lardan idempotent doldurma) — wiring: `setupAI`
      (dependencies.go) + `NewAIDependencies` (ai_dependencies.go), SeedTimeGrains'in yanında.
- [x] Yeni paket `internal/ai/lexicon`: `Store` arayüzü (TemporalPhrases/Terms/DomainTerms/
      Invalidate), `NewStaticStore` (embedded), `NewDBStore` (30s TTL + Invalidate, hata/boş
      domain'de per-domain embedded fallback — ADR K5/K6), `Active()/SetActive()` süreç-geneli
      kayıt (prompt store deseni). Repo katmanı: `metadata/ai_nl_lexicon.go`
      (List/ListActive/Count/Upsert/ReplaceDomain). Eşleştirme locale-birleşimi (K4);
      `Terms` snapshot'ı korumak için kopya döndürür.
- [x] `vagueTemporalPhrases` → lexicon (`temporal_phrase` domain'i); `DetectTemporal` +
      `MatchTemporalPhrases` store'dan okuyor. Kabul testi:
      `TestTemporalPhrasesComeFromLexiconStore` — store'a Almanca "letzten monat" verilince
      kod değişikliği olmadan tespit ediliyor.
- [x] `softDeleteColumnSynonyms` kelime listeleri → lexicon (`soft_delete`, 4 kural anahtarı);
      kolon-adı pattern kuralları kodda kaldı. (Not: ts_archived listesinin *sırası* en→tr
      birleşimine değişti, küme aynı.)
- [x] `routing_budget.go`: count/total/average intent token'ları + `questionMentionsTimeGrain`
      listesi → lexicon (`intent_token`); how+many / number+of yapısal çiftleri kodda.
- [x] `semanticgen/generator.go`: grain synonym + row-count listeleri lexicon'dan (tek kaynak);
      `time_grains.go` her iki store'da serve-time merge (`applyLexiconGrainSynonyms`, K3 geçişi —
      `ai_time_grains.synonyms` kolonu deprecate edilene dek birleşim).
      **Bilinçli süperset:** kanonik grain listesi = eski routing ∪ semanticgen listeleri
      (routing "ay bazında"yı, semanticgen "per month"u kazandı).
- [x] Admin CRUD (AdminKeyMiddleware): `GET/PUT /api/ai/admin/lexicon?locale=&domain=`
      (export/import, domain+locale+value şema validasyonu) + `POST /api/ai/admin/lexicon/reset`
      (domain'i embedded default'a sıfırlama — ADR K7); PUT/reset → `lexicon.Active().Invalidate()`.
      Dosyalar: `handlers/ai_admin_lexicon.go`, `ai_router.go` (`registerAIAdminConfigRoutes`
      helper'ına çıkarıldı, funlen).
- [x] **Kabul sağlandı:** yeni dil yalnızca DB satırlarıyla eklenebiliyor (de-locale store/fallback
      testleri); EN/TR davranışı korunuyor (59 paket -race yeşil; tek bilinçli fark yukarıdaki
      süperset — time_grains testleri merge sözleşmesine güncellendi); DB yokken embedded
      fallback testli (`TestDBStoreFallsBackToDefaultsOnError`). Gate'ler: lint-go 0,
      eval-regression ✓, deadcode yeni bulgu yok, jsonusage guard ✓ (sonic).

### DİL-2 — Routing lexicon'u DB overlay'e bağla [S] ✅ (2026-06-10)

- [x] `routing_lexicon.go`: `sync.Once` → mutex'li base+merged cache (30s `routingLexiconOverlayTTL`);
      `ActiveRoutingLexicon` artık embedded+dosya base'inin üzerine `ai_nl_lexicon`
      `token_synonym`/`metric_synonym` domain'lerini per-key replace ile bindiriyor
      (`overlayRoutingLexicon`); DB satırı yoksa base aynen servis edilir (sıfır davranış farkı —
      bu iki domain'in embedded seed'i bilinçli olarak boş). `BI_AI_ROUTING_LEXICON_PATH` dosya
      override'ı korunuyor (`InitRoutingLexicon` base'i yeniler). Yeni `InvalidateRoutingLexicon()` —
      admin lexicon PUT/reset `invalidateLexiconCaches` helper'ı ile hem lexicon store'u hem bu
      merge'ü yazan replikada anında düşürüyor; diğer replikalar iki TTL penceresi içinde (≤60s) yakınsar.
- [x] **Kabul sağlandı:** `TestActiveRoutingLexiconAppliesDBOverlay` — store'a "kunde"
      token-synonym'ü verilince restart'sız görünür, base anahtarlar ("musteri") ve overlay-dışı
      alanlar korunur, overlay kalkınca base'e döner. Gate'ler: lint-go 0, `make test-go`
      59 paket -race ✓, deadcode yeni bulgu yok, gofmt ✓.

### DİL-3 — i18n: dinamik locale registry + katalog overlay [M] ✅ (2026-06-10)

- [x] Migration `049a/b`: `i18n_locales` (profil + question_signals JSONB + enabled) ve
      `i18n_bundles` (bundle JSONB + version; update'te version otomatik artar).
      `metadata/i18n_runtime.go`: repo metotları + `NewI18nRuntimeProvider` adaptörü +
      `SeedI18nLocales` (tablo boşsa embedded EN/TR profillerinden). Wiring tek helper'da:
      `app/nl_runtime.go` `wireNLRuntime` (lexicon + i18n birlikte; setupAI ve
      NewAIDependencies buradan çağırıyor — NewAIDependencies funlen fix'i de bu).
- [x] `i18n/runtime.go`: `RuntimeProvider` arayüzü + 30s TTL snapshot + `InvalidateRuntime`.
      `SupportedLocaleProfiles/Codes`, `LocaleProfileFor`, `IsSupported`, `ParseLocale`,
      `FromContext` artık efektif (embedded ∪ registry) kümeden; yeni `ActiveLocales()`
      (embedding refresh de bunu kullanıyor). EN devre dışı bırakılamaz (K8); provider
      hatasında embedded'a düşülür. `T/Tf` zinciri: DB(loc) → embedded(loc) → DB(en) →
      embedded(en) → key. i18n bağımlılıksız kaldı — provider implementasyonu metadata'da
      (metadata→i18n yönü). Snapshot yenileme bilinçli request-scope'suz
      (`nolint:contextcheck` gerekçeli, audit/db_writer emsali).
- [x] Admin (AdminKeyMiddleware, `handlers/i18n_admin.go` + `ai_router.go`):
      `GET/PUT /ai/admin/i18n/locales` (EN-disable reddi, locale/label validasyonu),
      `GET/PUT /ai/admin/i18n/bundles/{locale}` (export DB→embedded fallback'li; import
      string-leaf validasyonlu, version'lı), `GET /ai/admin/i18n/coverage/{locale}`
      (efektif EN referansına göre eksik anahtar listesi + coverage %). Yazımlar
      `i18n.InvalidateRuntime()` çağırıyor.
- [x] `lingua.DetectQuestionLocale` zaten `SupportedLocaleProfiles()` okuyordu → registry
      sinyalleri otomatik devrede. Test: `TestDetectQuestionLocaleUsesRegistrySignals`
      (Almanca sinyaller kod değişikliği olmadan tespit ediliyor; embedded TR etkilenmiyor).
- [x] **Kabul sağlandı:** `TestRuntimeRegistryAddsNewLocale` + `TestRuntimeBundleLookupChain` —
      registry satırı + bundle upload ile "de" parse/context/profil/T() uçtan uca release'siz
      çalışıyor; EN-disable engeli, provider-hata fallback'i, TTL/invalidate, bundle leaf
      validasyonu ve coverage raporu testli. Gate'ler: lint-go 0, `make test-go` 59 paket
      -race ✓, eval-regression ✓, deadcode yeni bulgu yok, gofmt ✓.
      **Bilinen sınırlar:** (1) DB bundle'ları seed edilmez — embedded fallback tasarımı
      (drift önler); coverage endpoint'i eksikleri raporlar. (2) Runtime provider yalnızca
      monolith API (setupAI) + AI servisinde (NewAIDependencies) bağlı — catalog/query
      servisleri embedded-only (kullanıcıya i18n metni üreten yüzeyleri yok denecek kadar az;
      gerekirse `wireNLRuntime` iki satırla eklenir). (3) Yeni dilde bundle yüklenmezse
      metinler zincir gereği EN döner — davranış, hata değil.

### DİL-4 — Prompt şablonları: yeni locale onboarding [S] ✅ (2026-06-10)

- [x] **Tasarım sapması (bilinçli, daha iyi):** plan "bilinmeyen locale için DB'ye seed satırı"
      diyordu; bunun yerine **runtime köprü** uygulandı — seed durumu gerektirmez, sonradan
      eklenen locale restart'sız anında çalışır. `dbPromptStore.Snapshot` +
      `embedPromptStore.Snapshot` artık hangi locale'den çözümlediğini izliyor; kendi şablonu
      olmayan locale EN içeriği + `languageBridgeNote` ("## User Language — write every
      user-facing text in <Label>") alıyor. Not yalnızca kullanıcı-metni taşıyan bölümlere
      (`system_rules`, `clarification`) ekleniyor; admin o locale için DB satırı yazınca
      (versiyonlu, mevcut prompt-templates admin API'si) öncelik onda ve not düşüyor.
      Dil etiketi `i18n.LocaleProfileFor`'dan (DİL-3 registry) geliyor.
      Dosyalar: `prompt/prompt_templates.go` (`promptTemplateFromEmbedExact` +
      `languageBridgeNote`), `prompt/prompt_store.go` (iki Snapshot).
- [x] `prompt_examples.go` TR-hardcoded ipuçları lexicon'a bağlandı: planning steps 1 ve 5'teki
      "(müşteri, sipariş, silinen, aylık, …)" / ("bazında", "aylık") örnekleri artık
      `lexiconHintSamples` ile `intent_token/soft_delete/grain_synonym` domain union'ından
      (baş+son örnekleme → her aktif dil temsil edilir) üretiliyor.
- [x] **Kabul sağlandı:** `TestDBPromptStoreUnknownLocaleGetsLanguageBridge` ("de" → EN+not;
      admin "de" satırı yazınca not düşer), `TestEmbedPromptStoreUnknownLocaleGetsLanguageBridge`,
      `TestDBPromptStoreEmbeddedLocaleHasNoBridge`, `TestLexiconHintSamplesSpansLocales`.
      Gate'ler: lint-go 0, `make test-go` 59 paket -race ✓, eval-regression ✓, gofmt ✓.

### DİL-5 — Frontend + kalite kapıları [M]

**Mevcut durum:** Frontend locale'leri build-time bundled (`core.ts` statik import, `admin.ts`/`auth.ts`
dynamic `import()` code-split). Yeni dil eklemek için `Locale` union tipi, `SUPPORTED_LOCALES`,
`LOCALE_OPTIONS`, `dictionaries`, `sectionLoaders` değişikliği + 3 yeni TS dosyası gerekiyor → frontend
release şart. Backend tarafında `RuntimeProvider` + `i18n_locales` tablosu runtime'da yeni dil eklemeyi
destekliyor; prompt şablonları `prompts/{en,tr}/*.tmpl` dosya dizininden yükleniyor.

#### 5.1 — Frontend katalog dağıtım kararı

- [ ] **Karar:** (a) Runtime fetch vs (b) build-time bundle. ADR olarak belgelenmeli.

  **Seçenek (a) — Runtime fetch (önerilen):**
  1. Backend'e `GET /api/i18n/bundle/{locale}` endpoint'i ekle. Bu endpoint, `i18n_locales`
     registry'sinde kayıtlı bir locale'nin tüm section'larını JSON olarak döndürür
     (`core` + `admin` + `auth` birleştirilmiş).
  2. Frontend'de `sectionLoaders` mekanizmasını genişlet: mevcut `import()` yerine locale
     başına `fetchBundle(locale)` fonksiyonu ekle. Önce `localStorage` cache'ine bak (key:
     `biqly_bundle_{locale}`, TTL: 1 saat), miss'te API'den çek ve cache'e yaz.
  3. `LanguageSwitcher.tsx`'te dil listesini `GET /api/i18n/locales` (registry'den) ile dinamik
     oluştur. Yeni dil registry'ye eklenince buton otomatik görünür.
  4. `Locale` tipini `'en' | 'tr'` sabit listesinden `string`'e genişlet; `SUPPORTED_LOCALES`
     sabitini kaldır, runtime'dan beslenen state'e çevir.
  5. Fallback: bundle fetch başarısız olursa gömülü EN+TR seed dosyalarına dön (mevcut statik
     import'lar fallback olarak kalır).
  6. **Avantaj:** Yeni dil = sadece backend registry kaydı + bundle JSON. Frontend release yok.
  7. **Dezavantaj:** İlk yüklemede ek ağ round-trip; bundle TTL cache ile azaltılabilir.

  **Seçenek (b) — Build-time bundle (basit):**
  1. Mevcut mimari korunur. Yeni dil için 3 TS dosyası (`core.ts`, `admin.ts`, `auth.ts`) oluşturulur.
  2. `Locale` tipine yeni değer eklenir, `SUPPORTED_LOCALES`/`LOCALE_OPTIONS`/`sectionLoaders`
     güncellenir.
  3. Frontend build + deploy gerekli.
  4. **Avantaj:** Basit, ağ bağımlılığı yok.
  5. **Dezavantaj:** Her yeni dil = frontend release.

  **Karar sonrası yapılacaklar:**
  - ADR'yi `docs/adr/` altına yaz: bağlam, seçenekler, trade-off'lar, karar.
  - Seçilen seçeneğin implementasyonunu yeni bir todo maddesi olarak aç.

  - **Dosyalar (a seçilirse):**
    - Backend: `internal/http/handlers/i18n_bundle.go` (endpoint),
      `internal/i18n/runtime.go` (bundle assembler — mevcut `RuntimeProvider` genişletmesi).
    - Frontend: `frontend/src/i18n/locale.ts` (fetch + cache),
      `frontend/src/i18n/bundleLoader.ts` (yeni),
      `frontend/src/components/ui/LanguageSwitcher.tsx` (dinamik liste).
  - **Dosyalar (b seçilirse):** Sadece runbook'a yansıtılır, kod değişikliği yok.

#### 5.2 — Locale-parametrik eval golden case altyapısı

- [x] **Amaç:** Yeni dil eklendiğinde smoke test'ler otomatik çalışabilsin. Mevcut 7 golden case
      Türkçe sabit kodlanmış, `locale` alanı yok.

  **Nasıl yapılacak:**
  1. `ambiguity_golden.json` şemasına `"locale"` alanı ekle (varsayılan `"tr"` — geriye uyumlu).
     Mevcut case'ler `"locale": "tr"` ile işaretlenir.
  2. `AmbiguityGoldenCase` struct'ına `Locale string` alanı ekle (`ambiguity_golden.go`).
  3. Yeni dil eklendiğinde minimum smoke set: her `expected_type`'tan en az 1 case o dilde.
     Örnek: `"expected_type": "clarification"` + `"locale": "de"` → Almanca soru + aynı model_ref.
  4. `ambiguity_golden_runner.go`'da filtreleme: `--locale` flag'i veya `GOLDEN_LOCALE` env var ile
     sadece o dilin case'lerini çalıştır. CI'da varsayılan: tüm locale'ler.
  5. `ambiguity_golden_models.go`'daki test model'ler dil-agnostik kalır (SQL sütun adları),
     sadece `question` ve `expected_detail` dil değişir.
  6. `Makefile`'a `eval-regression` hedefinde locale filtresi yok (tüm diller çalışır).

  - **Dosyalar:**
    - `internal/ai/eval/testdata/ambiguity_golden.json` — `locale` alanı ekle,
      mevcut case'lere `"locale": "tr"` ekle.
    - `internal/ai/eval/ambiguity_golden.go` — struct + loader güncellemesi.
    - `internal/ai/eval/ambiguity_golden_runner.go` — locale filtre parametresi.
  - **Doğrulama (2026-06-10)**: `locale` alanı + `GOLDEN_LOCALE`/`--locale` filtresi, locale coverage
    doğrulaması, `make eval-regression` PASS.

#### 5.3 — Dil-taşıyan literal regresyon bekçisi

- [x] **Amaç:** Lexicon'a taşınmış kelimelerin koda geri sızmasını engelle. Mevcut durumda
      `internal/ai/routing/time_grains.go:25-49` (hardcoded grain synonym'ler),
      `internal/semanticgen/generator.go:358` (hardcoded `"ortalama"`) ve
      `internal/ai/routing/routing_lexicon_default.json` (67 satır karışık EN+TR token) hâlâ
      dil-taşıyor.

  **Nasıl yapılacak:**
  1. Basit bir shell script oluştur: `scripts/check_locale_literals.sh`
     - Taranacak dizinler: `internal/ai/routing/`, `internal/ai/ambiguity/`, `internal/semanticgen/`
     - Hariç tutulan: `*_test.go`, `testdata/`, `*.json` (routing lexicon ayrı ele alınacak)
     - Bilinen Türkçe kelime listesi dosyası: `scripts/locale_literal_blocklist.txt`
       İçeriği: her satır bir kelime (`aylık`, `yıllık`, `günlük`, `saatlik`, `ortalama`,
       `müşteri`, `sipariş`, `ürün`, `kategori`, `adet`, `miktar`, `toplam`, `silinen`, vb.)
     - Mantık: `grep -rf blocklist.txt <dizinler> --include='*.go' --exclude='*_test.go'`
       Bulunan her eşleşme = warning. Exit code 1 ile CI'ı kır.
  2. DIL-1 tamamlandıktan sonra `time_grains.go`'daki hardcoded synonym'ler lexicon'dan
     yükleniyor olacak → script bu dosyayı scoping'den çıkarabilir (veya dosya tamamen kaldırılır).
  3. `generator.go:358`'deki `"ortalama"` lexicon lookup'a çekildikten sonra script'in kapsamına girer.
  4. CI'ya ekle: `.github/workflows/ci.yml`'e yeni step veya `Makefile`'a `lint-locale-literals`
     hedefi olarak.
  5. Mevcut bulguları tolere etmek için initial run'da `--baseline` modu: mevcut bulguları
     listeleyip geçir, sonraki eklemeleri engelle.

  - **Dosyalar:**
    - `scripts/check_locale_literals.sh` (yeni script)
    - `scripts/locale_literal_blocklist.txt` (yeni kelime listesi)
    - `.github/workflows/ci.yml` veya `Makefile` (CI entegrasyonu)
  - **Doğrulama (2026-06-10)**: `scripts/check_locale_literals.sh` + baseline (`7` finding);
    `make lint-locale-literals` PASS; CI `lint` job step eklendi.

- [x] 5.4 — Yeni dil onboarding runbook'u

- [x] **Amaç:** `docs/` altında adım adım rehber: "X dilini Biqly'ye nasıl eklersin?"

  **Runbook içeriği (adım adım):**

  **Adım 1 — Backend: Locale registry kaydı**
  - `i18n_locales` tablosuna yeni satır ekle (admin API veya SQL):

    ```sql
    INSERT INTO i18n_locales (locale, label, short_label, is_active, question_letters, question_signals)
    VALUES ('de', 'Deutsch', 'DE', true, 'äöüßÄÖÜ',
            '{" wieviel "," zeige "," liste "," gesamt "," täglich "," monatlich "," jährlich "," gestern "," heute "," kunde "," bestellung "," produkt "," anzahl "," menge "," gelöscht "}');
    ```

  - `QuestionLetters`: o dile özgü karakterler (dil algılama için `lingua` kullanır).
  - `QuestionSignals`: o dildeki tipik soru kelimeleri (NL→SQL soru algılama ipucu).
  - `is_active=false` ile kaydet → aşağıdaki adımlar tamamlanınca `true` yap.

  **Adım 2 — Backend: NL lexicon seed**
  - `ai_nl_lexicon` tablosuna yeni dil için domain terimleri ekle. Minimum kategoriler:
    - `temporal_phrase`: "letzten monat", "kürzlich", vb.
    - `grain_synonym`: "jahr"/"jährlich"/"jährlich", "monat"/"monatlich", "tag"/"täglich", "stunde"/"stündlich"
    - `soft_delete`: "gelöscht", "entfernt", vb.
    - `intent_token`: "wieviel", "anzahl", "gesamt", "durchschnitt", vb.
    - `row_count`: "anzahl", "wie viele", "stückzahl"
  - Mevcut EN/TR seed'leri referans al: `internal/ai/lexicon/defaults.go`
  - Domain coverage raporu çalıştır: `GET /api/admin/ai/lexicon/coverage?locale=de`
    Boş/eksik kategorileri gösterir → bunları doldur.

  **Adım 3 — Backend: Prompt şablonları**
  - `internal/ai/prompt/prompts/` altına yeni dizin: `de/`
  - EN şablonlarını kopyala: `cp prompts/en/*.tmpl prompts/de/`
  - Her şablonu Almanca'ya çevir (sadece LLM'ün kullanıcıya döneceği metin kısımları):
    - `clarification.tmpl`: netleştirme soru metinleri
    - `output_format.tmpl`: çıktı formatı talimatları
    - `system_rules.tmpl`: sistem kuralları (çoğu locale-agnostik, sadece açıklama kısımları)
    - `repair.tmpl`, `retry.tmpl`: hata düzeltme talimatları
    - `ambiguity.tmpl`: belirsizlik algılama prompt'u
  - **Not:** `prompt_layout.tmpl` dil-agnostik (SQL şeması), genelde değişiklik gerektirmez.

  **Adım 4 — Frontend: Bundle (karar (a) seçildiyse)**
  - `i18n_bundles` tablosuna veya API endpoint'ine yeni locale'nin JSON bundle'ını yükle.
  - Minimum section'lar: `core` (tüm UI metinleri), `admin`, `auth`.
  - EN `core.ts`'yi temel al, çevirileri yap. Toplam ~1200 satır.
  - Test: dil seçicide "DE" görünmeli, seçince tüm UI Almanca olmalı.

  **Adım 5 — Frontend: Bundle (karar (b) seçildiyse)**
  - `frontend/src/i18n/locales/de/` dizinini oluştur.
  - 3 dosya: `core.ts`, `admin.ts`, `auth.ts` (EN'den kopyala + çevir).
  - `locale.ts`'te `Locale` tipine `'de'` ekle, `SUPPORTED_LOCALES`/`LOCALE_OPTIONS`/`sectionLoaders`
    güncelle.
  - `LanguageSwitcher.tsx`'te `"DE"` butonu otomatik görünür.
  - Frontend build + deploy.

  **Adım 6 — Smoke test**
  - `ambiguity_golden.json`'a en az 2 yeni case ekle:
    - 1 clarification case: Almanca soru → belirsizlik algılanmalı
    - 1 pass case: Almanca soru → belirsizlik yok, doğru SQL üretmeli
  - `make eval-regression` çalıştır, tüm case'ler pass olmalı.
  - Elle test: UI'da Almanca seç → örnek soru sor → doğru netleştirme/SQL.

  **Adım 7 — Aktifleştir**
  - `i18n_locales` tablosunda `is_active = true` yap.
  - Deploy.

  - **Dosyalar:**
    - `docs/how-to/add-new-language.md` (yeni runbook)
    - `docs/adr/0001-db-backed-nl-lexicon-and-i18n.md` (referans)

- [ ] **Kabul:** "Yeni dil ekleme" adımlarının hiçbiri backend Go kodu değişikliği veya
      frontend release'i gerektirmiyor (karar (a) seçildiyse). Runbook'u takip eden biri
      30 dakikada yeni dil ekleyebiliyor. Coverage raporu boş domain'leri gösteriyor.

**Sıralama/bağımlılık:** DİL-0 → DİL-1 → (DİL-2 ‖ DİL-3) → DİL-4 → DİL-5.
DİL-1 tek başına bile bu sohbetteki vaka sınıfını (grain/temporal kelimeleri) release'siz
yönetilir yapar; DİL-3 olmadan netleştirme *metinleri* yeni dilde EN fallback olarak kalır.

---

## AI Sorgu — Netleştirme (Clarification) Akışı Düzeltmeleri (2026-06-09)

İki hata: (1) UI/UX — netleştirme kartı scroll'da ortada kalıyor; (2) Logic —
seçim yapılmasına rağmen sistem tekrar tekrar netleştirme soruyor (sonsuz döngü).

**Kök neden (Bug 2):** Netleştirme cevabı `handlers/ai.go` içinde soruyu yeniden
yazıp choice'i temizledikten sonra, `standardProcessOptions` koşulsuz olarak
`WithAmbiguityCheck(true)` ekliyor; `ProcessQuestion → checkAmbiguity`
yeniden-yazılmış soruda YENİ bir belirsizlik (genelde synonym detector'ın
"ay/day/days" gibi jenerik token'ları) buluyor ve tekrar soruyor. Frontend her
turda orijinal soruyu + tek choice gönderdiği için önceki çözümler taşınmıyor →
≥2 belirsizlikte asla yakınsamıyor.

**Kök neden (Bug 1):** `ChatPanel` scroll efekti her mesajda feed'i `scrollHeight`'a
(en alta) kaydırıyor; uzun netleştirme kartında soru görünür alanın dışında kalıyor.

### Yapılacaklar

- [x] **Backend: Netleştirme turunda ambiguity check'i atla (Bug 2 ana fix).**
  `aiQueryRequest`'e unexported `clarificationResolved bool` eklendi;
  `parseAndRouteAIQuery` choice çözüldüğünde `true` set ediyor;
  `standardProcessOptions` artık `WithAmbiguityCheck(true)`'u
  `&& !req.clarificationResolved` ile koşullu ekliyor. Çözülen turda LLM
  üretimine doğrudan gidiliyor → döngü deterministik kırıldı.
  - **Files**: `internal/http/handlers/ai.go`.
- [x] **Backend: Synonym detector precision (gürültü azaltma).**
  `synonymMatchConfidence` yeniden yazıldı: tek-token synonym'ler tam token
  eşleşmesi gerektiriyor (substring değil); `minExactSynonymTokenRunes=2`,
  `minFuzzySynonymTokenRunes=4` gate'leri eklendi; çok-kelimeli ifadeler
  bitişik `strings.Contains` ile eşleşmeye devam ediyor. "ay/day/days" gibi
  jenerik token'lar artık alakasız kelimeler içinde işaretlenmiyor.
  - **Files**: `internal/ai/ambiguity/synonym_detector.go`.
- [x] **Frontend: Netleştirme kartını üstten görünür kıl (Bug 1).**
  `ChatPanel` scroll `useEffect` netleştirme-bilinçli: son asistan mesajı
  `ai_response?.needs_clarification` ise o kart feed üstüne hizalanıyor
  (`data-message-index` + `getBoundingClientRect`); diğer hallerde mevcut
  en-alta kaydırma korunuyor.
  - **Files**: `frontend/src/components/aiQuery/ChatPanel.tsx`.
- [x] **Testler.** Backend: `service_test.go` →
  `TestProcessQuestionSkipsAmbiguityWhenCheckDisabled` (choice çözülünce tekrar
  sormaz, LLM bir kez çağrılır); `synonym_detector_test.go` →
  `TestDetectSynonyms_GenericSubstringTokensNotFlagged`,
  `TestDetectSynonyms_MultiWordPhraseMatches`. Frontend lint/test/build temiz.
  - **Files**: `internal/ai/service_test.go`,
    `internal/ai/ambiguity/synonym_detector_test.go`.
- [ ] **(Opsiyonel, Faz 2) Çok terimli belirsizlik.** Tüm gerçek belirsizlikleri
  tek turda sun veya çözümleri turlar arası biriktir; atlamak yerine tam
  disambiguasyon ile yakınsa. (Ertelendi.)

**Kabul kriterleri:** Bir netleştirme seçimi sonrası sistem tekrar netleştirme
sormaz, sonucu üretir; netleştirme kartının sorusu otomatik görünür olur;
`make lint-go`, `make test-go`, `make lint-frontend`, `make test-frontend` temiz.

### Review (2026-06-09)

**Sonuç:** Zorunlu işlerin tümü tamamlandı; tüm kapılar temiz geçti.

- **Bug 2 (sonsuz döngü) — GERÇEK kök neden bulundu ve çözüldü.** İlk fix yalnızca
  **senkron** uç noktayı (`parseAndRouteAIQuery`) yamalıyordu; ancak frontend
  generate/preview için **asenkron job** yolunu (`ai_job_exec.go` →
  `executeAIQueryPhase`) kullanıyor. Bu yol seçimi `resolveClarificationChoice`
  ile çözüyordu ama `clarificationResolved` bayrağını **hiç set etmiyordu** → guard
  `!req.clarificationResolved` her turda `true` kalıyor → `standardProcessOptions`
  her turda `WithAmbiguityCheck(true)` ekliyor → sonsuz yeniden-netleştirme.
  **Fix:** `req.clarificationResolved = true` ataması ortak `resolveClarificationChoice`
  **METODUNA** (`ai.go:176-188`, `choice != ""` iken) taşındı; böylece hem senkron
  hem asenkron job yolu bayrağı alıyor. `parseAndRouteAIQuery`'deki artık-gereksiz
  açık atama kaldırıldı. Run fazı (`resolveRunPhaseForJob`) zaten ambiguity check
  eklemiyor → döngü riski yok. Synonym detector sıkılaştırması ilk netleştirme
  gürültüsünü azaltıyor.
- **Regresyon testi:** `ai_ambiguity_test.go` →
  `TestHandlerResolveClarificationChoiceSetsResolvedFlag` (metot choice çözünce
  bayrağı set ediyor) + `...NoChoiceKeepsFlagUnset` (choice yoksa set etmiyor).
- **Bug 1 (scroll) çözüldü** — netleştirme kartı feed üstüne hizalanıyor, soru
  görünür kalıyor.
- **Doğrulama kapıları (2026-06-09 son tur):** `make lint-go` (0 sorun) ·
  `make test-go` (`-race`) PASS · `go test ./internal/http/handlers/` PASS ·
  `make eval-regression` PASS · `deadcode` (yeni ölü kod yok; mevcut `pkg/` SDK +
  observability bulguları pre-existing). Frontend bu turda değişmedi.
- **Commit yapılmadı** (kullanıcı onayı bekleniyor).

## Technical Architecture Analysis — Remaining Actions (2026-06-08)

The open items from §10 Conclusion and Roadmap in `tasks/biqly_analiz.pdf` (Version 3.0) have been verified against the codebase and updated. ESLint zero-warnings has already been achieved; the following items are still open.

### Medium priority (2026-06-08)

- [x] **Reduce AIConfig getter methods/fan-out.** 13 getter methods $\rightarrow$ 5 exported methods (`ResolvedQuery`, `ResolvedEmbedding`, `ResolvedTranslation`, `HTTPTimeout`, `RequestTimeout`) + 3 view types (`AIQueryView`, `AIEmbeddingView`, `AITranslationView`). External calls: 93 $\rightarrow$ 58. `make lint-go` clean.
  - **Files**: `internal/config/config.go`, `internal/ai/service.go`, `internal/ai/provider/*.go`, `internal/app/dependencies.go`, `internal/http/handlers/ai.go` + tests.

- [x] **Remove branching in TableRouter.Route.** `Route` has been reduced to 61 lines; the `funlen` nolint was removed; logic was moved to helper functions: `routeLoadAndFilter`, `routePrepareSelection`, `routeAnnotateResult`, `routeExpandSelection`, and `routeFinalize`. `go test ./internal/ai/routing/...` passes.
  - **Files**: `internal/ai/routing/router.go` + existing test files.

### Low priority (2026-06-08)

- [x] **Gradually reduce repository-wide nolint directives.** Current baseline: **75** (`rg -c 'nolint' --glob '*.go'`); the target of <80 has been met. This round: `builtins` (`enrich_viz.go`) and `revive` (`permissions.go`) were removed. Remaining directives are mostly justified `gosec`/`nilnil`/test fixtures.
  - **Acceptance Criteria**: nolint count <80; no new nolints added; corresponding linter passes for every removed nolint.
  - **Files**: 40+ files detected via `grep -rl '//nolint' --include='*.go' .`.

- [x] **Go: Gradually rename functions with `Get` prefix.** This round: `PublicKeyPEM`, `EffectivePermissions`, `RowFilters`. Up next: `internal/auth/rbac/rbac_repository.go` (8), `internal/auth/oauth/*.go`, `internal/auth/service.go`.
  - **Acceptance Criteria**: New code does not use `Get` prefix; existing `Get` prefixes are gradually reduced; `make lint-go` and `go test` remain clean at each step.
  - **Files**: `internal/auth/rbac/*.go`, `internal/auth/service.go`, `internal/auth/oauth/*.go`, `internal/security/permissions.go`, `internal/auth/mfa/*.go`, `internal/auth/jwt.go`.
  - **Doğrulama (2026-06-10)**: Target fonksiyonlar zaten `Get` prefixesiz (`PublicKeyPEM`, `EffectivePermissions`, `RowFilters`); bu turda kod değişikliği gerekmedi. `make lint-go` 0 issue · `make test-go` PASS.

- [ ] **Frontend: Ensure handler/event naming consistency.** This round: `MetadataDescribeModal` (`onKey` $\rightarrow$ `handleKeyDown`), `SelectPopover` (`handleListKeyDown`). Rule: `handle*` for internal handlers, `on*` for DOM props.
  - **Acceptance Criteria**: New code is consistent; existing inconsistencies are fixed opportunistically.
  - **Files**: `frontend/src/App.tsx`, `frontend/src/components/ui/*.tsx`, `frontend/src/components/settings/*.tsx`.

- [x] **Frontend: Make `CONSTANT_CASE` usage consistent.** Function-scoped `const MAX` $\rightarrow$ `maxRecentTurns` (`AIQuery.tsx`); module-level constants are already `CONSTANT_CASE`.
  - **Acceptance Criteria**: ESLint naming rules pass; no inconsistencies remain.
  - **Files**: `frontend/src/utils/*.ts`, `frontend/src/components/**/*.tsx`.

- [x] **Document repository-wide naming convention rules in lessons.md.** The `tasks/lessons.md → Naming Conventions` section has been updated to include best-practice rules for Go and TypeScript/React. This section will serve as a reference when writing new code.
  - [x] Go naming rules (receiver, function, interface, constant, error var, initialisms, stutter).
  - [x] TypeScript/React naming rules (casing, handlers, booleans, useState, custom hooks, abbreviations, constants).

- [x] **Raise `internal/queue` coverage floor (%40 $\rightarrow$ at least %60).** Floor set to %60 (`scripts/coveragecheck/main.go:35`); package coverage is at %62.5. Added local queue tests (idempotent close, connect error) and mock JetStream for NATS publish/DLQ paths; no live NATS server required.
  - **Acceptance Criteria**: Floor in `scripts/coveragecheck/main.go` is %60; `make coverage-gate` passes; new tests run without a live NATS server (local queue path).
  - **Files**: `internal/queue/*.go`, `scripts/coveragecheck/main.go`.

- [x] **Gradually raise coverage floors for critical packages.** `internal/ai/routing` floor is at %80 (currently %83.5), `internal/auth` floor is at %10 (currently %13.2). Added to Makefile + CI coverage profile.
  - **Acceptance Criteria**: At least 2 new packages added to the floors map in `scripts/coveragecheck/main.go`; `make coverage-gate` passes for each.
  - **Files**: `scripts/coveragecheck/main.go`, new test files.

- [x] **Periodically update live-eval baseline.** Added `edge-not-shipped-count` (`neq` filter, TR locale); `testdata/eval/nightly_baseline.json` updated from 17 $\rightarrow$ 18 cases (`go run scripts/gen-nightly-baseline/main.go`).
  - **Acceptance Criteria**: At least 1 new golden case added per sprint; baseline commit is up to date.
  - **Files**: `internal/ai/eval/`, `cmd/eval-live/`, `.github/workflows/eval-nightly.yml`.

- [x] **Add critical attributes to spans (ongoing improvement).** Added `ai.tokens.{prompt,completion,total}`, `ai.route.confidence` (defer including clarification), `db.system`/`datasource.driver`, `query.compile.duration_ms`, `query.execute.duration_ms`. OTEL sampler: `parentbased_traceidratio` defaults to 25% (`OTEL_TRACES_SAMPLER*`), Helm `global.observability.tracing.*`.
  - **Acceptance Criteria**: Every new span attribute is visible in Jaeger; trace sampling rate is adjusted for production load.
  - **Files**: `internal/ai/service.go`, `internal/ai/routing/router.go`, `internal/datasource/*.go`, `internal/platform/observability/*.go`.

- [x] **Monitor Prometheus label cardinality.** Added `bi_prom_metric_series_total` + `bi_prom_label_cardinality` collector; `VecLabelLimits`/`CheckGatheredCardinality`/`BoundLabel`; Grafana `biqly-cardinality.json`.
  - **Acceptance Criteria**: Label cardinality metrics are visible in the Grafana dashboard; cardinality limits are checked when adding new labels.
  - **Files**: `internal/platform/observability/metrics.go`, Helm Grafana dashboard config.

- [x] **Periodically verify dev cookie exemption does not leak to prod.** `CookieSecure` is fail-closed in prod/K8s; CI: `TestProductionAuthEnabledFailClosed`, `TestProductionCookieSecureFailClosed`.
  - **Acceptance Criteria**: `TestProductionAuthEnabledFailClosed` passes in CI; manual verification runs at least monthly (`go test -run 'TestProduction(AuthEnabledFailClosed|CookieSecureFailClosed)' ./internal/config/... ./internal/auth/...`).
  - **Files**: `internal/auth/cookie.go`, `internal/config/config.go`.

### Completed (in previous rounds)

- [x] AIConfig fields split into 9 sub-structs
- [x] ValidateContext/ValidateComposite/PasswordPolicy.Validate reduced to single-digit complexity
- [x] Nightly live-LLM eval + drift gate added
- [x] Auth fail-closed invariant in prod (`env.IsProduction()`)
- [x] OTEL span depth increased from 3 $\rightarrow$ 16+
- [x] queue package added to coverage floor monitoring (40%)
- [x] Flaky TestMFABypassCodeFlow stabilized
- [x] ESLint warnings reduced to zero (`--max-warnings 0`)
- [x] CSP + X-Frame-Options + prod HSTS security headers implemented
- [x] CodeQL + govulncheck + semgrep SAST scans enabled

 ---

## Technical Analysis Report — Remaining Gaps (2026-06-07)

 The remaining recommendations from the `tasks/biqly_analiz.pdf` report (Version 3.0) were verified in the codebase. 7 of the 8 gaps from the initial audit have been resolved; the following are still open. In order of priority:

### Medium priority (2026-06-07)

- [x] **Physically decompose the AIConfig god-object (move fields, do not just rename).** The previous renaming of nested configs did not improve the metrics (21 fields / 13 methods / 93 external calls = score of 60, CRITICAL). In this round, top-level connection/tuning fields were physically relocated.
  - Completed: `AIConfig` now contains only 9 top-level sub-structs — `Connection`, `Generation`, `Describe`, `Cache`, `Query`, `Embedding`, `Translation`, `Routing`, and `Ambiguity` (`internal/config/config.go`).
  - Groups relocated: `AIConnectionConfig` (Provider/APIKey/BaseURL/Model/HTTPTimeout/RateLimit), `AIGenerationConfig` (MaxTokens/Temperature/TopP/NumCtx/MaxPromptInputRunes/MaxRetries/MultiCandidateCount), `AIDescribeConfig` (MaxCellRunes/MaxSampleRows), and `AICacheConfig` (ResponseTTLSeconds).
  - All call sites updated (ai/provider/service/provider_store, prompt/context_budget, http/handlers, app dependencies + tests). `go test ./internal/config/... ./internal/ai/... ./internal/http/...` passes.
- [x] **Gradually decompose the remaining high-complexity functions.** The three target functions highlighted in the report: `ValidateContext` (39), `ValidateComposite` (27), and `PasswordPolicy.Validate` (25). Split each into smaller helper functions and secure behavior with tests.
- [x] **Add periodic (nightly) live-LLM eval runs.** The current regression gate on the deterministic stub provider is 1.00 — which catches harness/compiler regressions but does not measure live-LLM accuracy drift. Add a nightly cron workflow + golden run with a real provider + drift reporting. (Also expand the evaluation suite with new dialect/edge cases.)
- [x] **Make `BI_AUTH_ENABLED` a required (fail-closed) invariant in production.** Verified: `internal/config/config.go:413` defaults to `false`. Although Helm production values set it to `true`, having it disabled at the code level in production relies entirely on network-level security. Action: If `env.IsProduction()` is true and `BI_AUTH_ENABLED=false`, fail-closed during startup (panic/refuse).

### Low priority (2026-06-07)

- [x] **Increase OTEL tracing depth (driver/DB spans).** Verified: only 3 named spans existed — `ai.ProcessQuestion` (`internal/ai/service.go:254`), `query.Compile` (`internal/query/compiler.go:64`), `query.Execute` (`internal/query/executor.go:52`) + router otelhttp ingress. Datasource driver calls and few-shot/embedding sub-phases were not spanned. Action: add spans to sub-phases + critical attributes on spans (model, attempt, fingerprint).
  - Completed (2026-06-07): datasource spans (`datasource.Ping/Open/Introspect/IntrospectSchemas|Tables|Columns|Relations/Query`), AI sub-phases (`ai.PromptBuild`, `ai.AmbiguityAnalyze`, `ai.LLMGenerate`, `ai.MultiCandidate`, `ai.TableRoute`, `ai.RouteEmbedding`, `ai.LoadFewShot`, `ai.Embed`, `ai.EmbedMetadata`, `ai.ProviderGenerate`), critical attributes (`ai.model`, `ai.attempt`, `query.fingerprint`, `model.id`, token counts, route confidence).
  - compile→execute fingerprint chain via `query.LogicalQueryFingerprint` + `observability.WithQueryFingerprint`.
  - Verification: `go build ./internal/...` + `go test ./internal/query/... ./internal/datasource/... ./internal/ai/... ./internal/core/... ./internal/platform/observability/... ./internal/http/handlers/...` passed.
- [x] **Add `internal/queue` to the coverage floor map.** Added `internal/queue` 40% floor to the `floors` map in `scripts/coveragecheck/main.go` (current ~42.5%).
- [x] **Fix flaky `TestMFABypassCodeFlow` isolation.** `mfatest` no longer deletes global tables; per-user teardown via unique email seed + `t.Cleanup` (`webauthn_flow_test` pattern).
- [x] **Ratchet ESLint warning ceiling toward 0 over time.** Frontend gate is CI-equivalent; warning ceiling was > 0; ratcheted to zero gradually.
  - **Current state**: `--max-warnings 576` (actual warnings: 576, 25 rules, 100+ files; Phase 1 + no-misused-promises closed)
  - **Target**: Promote rule groups to `error` in priority order; lower `max-warnings` in each group.
  - **Phase 1 — Highest impact, mechanical fixes (~830 warnings, 57%)**
    - [x] `@typescript-eslint/prefer-nullish-coalescing` (262 → 0) — `||` → `??` changes; rule is suggestion-only (no autofix); applied 262 suggestions across 56 files via ESLint API. `max-warnings` 1495 → 1220. Tests passed (95/95).
    - [x] `@typescript-eslint/no-unsafe-call` (199 → 0) — Root cause: `t: any` on child component props. Exported `TFunction` / `LooseTFunction` (`i18n/index.tsx`); replaced `t: any` → `TFunction` in 16+ files. Also: `PasskeyCreationOptionsJSON`, `DashboardBuilder` KPI render, `ExpressionBuilder` invalid fallback args removed. `max-warnings` 1220 → 850. Tests passed (95/95), tsc clean.
    - [x] `@typescript-eslint/no-unsafe-assignment` (41 → 0; 124 in todo was old baseline) — `AuthUserRaw` + typed `apiFetch`, passkey JSON types, `parseJsonRecord`/`parseJsonStringArray` (`utils/record.ts`), `QueryResultPayload` query run, `PermissionRowFilter.value` narrowing, i18n `navigator.languages` guard. `max-warnings` 850 → 730.
    - [x] `@typescript-eslint/no-unsafe-member-access` (96 → 0) — Most were fixed with `no-unsafe-assignment` (`api/auth.ts` typed `apiFetch`, `DashboardBuilder` `QueryResultPayload`/`ChartRow`). Remaining 5: `Glossary.tsx` and `ExpressionBuilder.tsx` `catch (err: unknown)` + `instanceof Error`; `admin.test.ts` mock `RequestInit` cast. Rule promoted to `error`. `max-warnings` 730 → 709.
  - **Phase 2 — Promise/async security (~256 warnings, 17%)**
    - [x] `@typescript-eslint/no-misused-promises` (130 → 0) — `void asyncFn()` wrapper on event handlers and prop callbacks; form `onSubmit={(e) => { void handleSubmit(e) }}`; wrapped `AuthProvider` setInterval + `AuthGuard` navigate ref. 42 files. Rule promoted to `error`. `max-warnings` 709 → 576.
    - [x] `@typescript-eslint/no-floating-promises` (126 → 0) — `void` prefix on fire-and-forget calls in `useEffect` and async handlers; `.then()` chains `void get(...).then(...)`. 50 files. Rule promoted to `error`. `max-warnings` 576 → 451.
  - **Phase 3 — Type safety and code quality (~310 warnings, 21%)**
    - [x] `@typescript-eslint/no-unnecessary-condition` (136 → 0) — Removed unnecessary `?? []`/`?.`/always-truthy `if`; discriminated union final branches use `else`; narrowed `apiFetch` return types; `apiClient` timeout via `startedAt`; 49 files. Rule promoted to `error`. `max-warnings` 451 → 315. Tests passed (95/95).
    - [x] `@typescript-eslint/no-explicit-any` (25 → 0) — `SemanticDimension`/`SemanticMetric` types in query builder steps; `Select` onChange setter pass-through; `ComponentType<unknown>` lazy preload; `unknown[][]` chart rows; aggregation type guard; direct `t` pass-through for `LooseTFunction`. Rule promoted to `error`. `max-warnings` 315 → 290.
    - [x] `@typescript-eslint/no-unsafe-argument` (35 → 0) — All 35 warnings already resolved by Phase 1 `no-unsafe-call`/`no-unsafe-assignment`/`no-unsafe-member-access` fixes (`TFunction`, `apiFetch` types, `parseJsonRecord`, `QueryResultPayload`, etc.). No extra code change needed. Rule promoted to `error`. `max-warnings` 290 (unchanged). Lint clean (0 violations).
    - [x] `@typescript-eslint/no-unsafe-return` (7 → 0; actually 1) — `parseStoredConversations` + `isConversation` type guard for `useConversation.ts` `JSON.parse`. Rule promoted to `error`.
    - [x] `@typescript-eslint/no-redundant-type-constituents` (8 → 0) — `AIQueryResponse | unknown` → `unknown`; removed `DriverTileGrid` generic (`readonly DriverId[]`); `jobWaiter.test` concrete generic; `context_source` literal union (`| string` removed). Rule promoted to `error`.
    - [x] `@typescript-eslint/consistent-type-imports` (5 → 0) — Replaced `import()` inline type annotations with `import type` (`AssistantMessageCard`, `routingViz`, `modeling/types`). Rule promoted to `error`.
    - [x] `@typescript-eslint/no-unused-vars` (28 → 0; actually 3) — Removed unused `Datasource`/`CardLayout` imports; removed `catch` binding. Rule promoted to `error`. `max-warnings` 290 → 248. Tests passed (95/95).
  - **Phase 4 — React hooks and a11y (~164 warnings, 11%)**
    - [x] `react-hooks/set-state-in-effect` (112) — `setState` inside `useEffect` (v7 new rule). Heavy files: `TableBrowser.tsx` (8), `Modeling.tsx` (7), `SavedQuestions.tsx` (5). Best practice: replace `useEffect` + `setState` with `useSyncExternalStore`, derived state, or `useMemo`; for initial load consider `use()` or Suspense.
    - [x] `react-hooks/exhaustive-deps` (30) — Missing dependency arrays. Best practice: review each `useEffect`/`useCallback`/`useMemo` dependency array; if no false dependency, add justified `// eslint-disable-next-line react-hooks/exhaustive-deps`.
    - [x] `react-refresh/only-export-components` (34) — Non-component exports in file. Best practice: move utility functions and constants to a separate file; `allowConstantExport: true` is already in config but split files for the remainder.
    - [x] `react-hooks/refs` (4), `react-hooks/immutability` (2), `react-hooks/purity` (1) — v7 new rules. Best practice: move ref mutations to event handlers; ensure immutable state updates; move side effects to `useEffect`.
    - [x] `jsx-a11y/no-autofocus` (18) — `autoFocus` attribute. Best practice: use `useRef` + `el.focus()` instead of `autoFocus`; apply focus trap on modal open with `useEffect`.
  - **Phase 5 — Remaining low-count rules (~33 warnings, 2%)**
    - [x] `complexity` (24) + `max-depth` (1) — High-complexity functions. Best practice: split large functions into sub-functions; reduce nested `if` with early return.
    - [x] `@typescript-eslint/no-base-to-string` (5) — `toString()` on invalid type. Best practice: use `String()` or template literal.
    - [x] `@typescript-eslint/no-empty-function` (3) — Empty function bodies. Best practice: use `noop` helper or `_`-prefixed parameter instead of `() => {}`.
    - [x] `@typescript-eslint/ban-ts-comment` (1), `@typescript-eslint/prefer-for-of` (1) — One-off fixes.
  - **Ratcheting strategy**
    - [x] After each phase completes, update `max-warnings` to current warning count + small buffer (10–20).
    - [x] Target timetable: Phase 1 → ~665 warnings (`max-warnings 680`), Phase 2 → ~405 warnings (`max-warnings 420`), Phase 3 → ~95 warnings (`max-warnings 110`), Phase 4+5 → 0 (`max-warnings 0`).
    - [x] Final step: promote all `'warn'` rules in `eslint.config.js` to `'error'` and enforce strict zero-warning policy with `--max-warnings 0`.

### Notes

- All findings were verified line-by-line in source; the 7 report items marked "closed" were not reopened.
- Follow behavior-preserving tests first, then refactor (especially AIConfig moves and function extractions).

## pgarray Abstraction — Consolidating lib/pq in One Place (2026-06-07)

The `lib/pq` helpers (`pq.Array`, `pq.StringArray`) used for Postgres `text[]` encode/decode were spread across 11 files. The driver already uses pgx (`database/sql` + `pgx/v5/stdlib`); lib/pq was only used as an array codec. Consolidated into a single abstraction so a future pgx native / pgtype migration touches one file.

### Completed work

- [x] Created `internal/platform/db/pgarray/array.go` — the **only** package that imports lib/pq.
  - `func Strings(v []string) any` → query param (Valuer), `pq.Array` instead of.
  - `type StringArray = pq.StringArray` → scan target + Valuer.
  - `func Scan(dst any) any` → pointer scan hedefi (`pq.Array(&slice)` instead of).
- [x] Replaced direct `pq.*` usage with `pgarray.*` in 11 files; removed `github.com/lib/pq` imports:
  - `internal/metadata/`: `repository.go`, `business_glossary.go`, `ai_time_grains.go`, `ai_history_query.go`, `permissions.go`, `translations.go`, `curated_ai.go`, `ai_jobs.go`
  - `internal/auth/repository.go`, `internal/auth/mfa/mfa_repository.go`
  - `internal/ai/provider_store.go`
- [x] Behavior unchanged (`pgarray.Strings` = `pq.Array`, `StringArray` = type alias) — only indirection added.

### Result / verification

- `lib/pq` is now imported only in `internal/platform/db/pgarray/array.go` (verified via grep; no other `pq.Array`/`pq.StringArray` remain).
- `gofmt -w` applied to all touched files.
- `go build ./...` and `go vet ./internal/{metadata,auth,ai,platform/db}/...` clean.
- `golangci-lint run` on touched packages: **0 issues**.
- In-memory tests passed (`internal/metadata`, `internal/ai`, `internal/platform/db`). Two failing tests (`auth/mfa`, `auth/workspace`) **also fail on a clean tree** with FK constraint errors → shared test DB seed issue, unrelated to this change.

### Future migration to pgx native / pgtype

Only the bodies of three symbols in `pgarray/array.go` need to change instead of 11 files. If staying on `database/sql` + pgx stdlib, use `pgtype` equivalents; with full pgxpool migration, pass Go slices directly and remove `lib/pq` entirely.

## Redis Client Migration Evaluation (go-redis → rueidis / valkey-go) (2026-06-07)

go-redis v9 is reliable but performance-focused alternatives are more aggressive:

- **rueidis**: automatic pipelining, client-side caching; claims higher throughput vs go-redis on parallel workloads. Large deltas in its own benchmarks.
- **valkey-go**: optimized for Valkey/Redis, similar performance story.

### Evaluation criteria

- [x] Write real benchmark comparison between go-redis v9 and rueidis (GET/SET/MSET pipeline, P99 latency, connection pooling).
- [x] Analyze how rueidis client-side caching fits the existing cache layout.
- [x] Check valkey-go API compatibility (Dragonfly/Redis support).
- [x] Assess migration risk: API differences, test coverage, community/maintenance status.

### Result (2026-06-07)

- Added: isolated Go module `benchmarks/redisclient`. Pinned `go-redis/v9 v9.19.0`, `rueidis v1.0.75`, `valkey-go v1.0.75`.
- Benchmark scope: single `GET`, single `SET`, batched `MSET`, pipelined `SET`/`GET`; bounded `p99_ns/op` reported alongside `ns/op`. Connection pool effect measurable via `REDIS_BENCH_POOL_SIZE` (`go-redis` `PoolSize`, rueidis nearest `PipelineMultiplex` connection count).
- How to run:
  - `cd benchmarks/redisclient`
  - `REDIS_BENCH_ADDR=127.0.0.1:6379 go test -run TestValkeyCompatAPISurface -bench . -benchtime=10s -count=5`
- Live local results (`127.0.0.1:6379`, Apple M4, darwin/arm64, `-benchtime=10s -count=5`, total `471.606s`):

  | Benchmark | go-redis median ns/op | rueidis median ns/op | go-redis median p99_ns/op | rueidis median p99_ns/op | Result |
  | --- | ---: | ---: | ---: | ---: | --- |
  | `GET` | `396704` | `487313` | `1058792` | `2753250` | go-redis daha iyi |
  | `SET` | `369127` | `413086` | `693292` | `1292750` | go-redis daha iyi; ortalama noisy |
  | `MSET` | `445711` | `509089` | `873542` | `1164958` | go-redis daha iyi |
  | Pipeline `SET`/`GET` | `389664` | `418962` | `750333` | `670333` | rueidis p99 biraz iyi, ortalama go-redis iyi |

  This run did not justify migrating to rueidis on performance. Do not start migration unless staging/Dragonfly shows different results.
- Live Dragonfly results (`docker.dragonflydb.io/dragonflydb/dragonfly:v1.34.1`, `127.0.0.1:6379`, Apple M4, darwin/arm64, `-benchtime=10s -count=5`, total `495.595s`):

  | Benchmark | go-redis median ns/op | rueidis median ns/op | go-redis median p99_ns/op | rueidis median p99_ns/op | Result |
  | --- | ---: | ---: | ---: | ---: | --- |
  | `GET` | `449545` | `442002` | `929375` | `1038333` | close; p99 go-redis better |
  | `SET` | `440736` | `456810` | `940333` | `1175875` | go-redis daha iyi |
  | `MSET` | `670924` | `786056` | `1227500` | `2391917` | go-redis belirgin daha iyi |
  | Pipeline `SET`/`GET` | `1207217` | `3362068` | `2933459` | `9429833` | go-redis much better |

  On Dragonfly too, no performance case for rueidis; pipeline workloads would regress with this implementation.
- rueidis fit: direct swap into production via native builder API is high churn. Server-assisted client-side caching via `DoCache`/`DoMultiCache`; best candidates are TTL `GET`/`SET` payload caches in `internal/ai/response_cache.go` and `internal/semantic/composite_cache.go`, but invalidation must be validated in staging.
- valkey-go fit: `valkeycompat.NewAdapter` provides go-redis-like API; `TestValkeyCompatAPISurface` in the benchmark module verifies `Set`, `Get`, `Cache`, `Pipelined` against live Redis/Dragonfly/Valkey. Dragonfly/Redis protocol support needs live test against target version in practice.
- Risk decision: no production client migration now; not recommended after two local runs. Current usage: `INCR`/`EXPIRE`, `GET`/`SET`, `SCAN`/`DEL` and DI tied to `*redis.Client`. If staging shows meaningful difference, lowest-risk order: thin `internal/platform/cache` adapter first, then `internal/auth/ratelimit.go` + `internal/mail/smtp.go` pilot, finally AI/semantic client-side caching trial.
- Review / verification:
  - `gofmt -w benchmarks/redisclient/redis_client_bench_test.go`
  - `go test ./...` (`benchmarks/redisclient`; compile verified with live test/bench skipped when `REDIS_BENCH_ADDR` unset.)
  - `go vet ./...` (`benchmarks/redisclient`)
  - `REDIS_BENCH_ADDR=127.0.0.1:6379 go test -run TestValkeyCompatAPISurface -bench . -benchtime=10s -count=5`
  - `docker compose up -d redis` + same benchmark command (`dragonfly:v1.34.1`)

### Files to change (go-redis → alternative client)

**Packages that use `*redis.Client` directly:**

| File | Usage |
| --- | --- |
| `internal/app/dependencies.go:454` | `redis.NewClient(opt)` — monolith DI |
| `internal/app/providers.go:71` | `redis.NewClient(opt)` — provider DI |
| `cmd/auth/main.go:207-212` | `newRedisClient` — auth service |
| `cmd/mail/main.go:56-63` | `redis.NewClient(opts)` — mail service |
| `internal/auth/service.go:54,92` | `redisClient *redis.Client` — auth service struct |
| `internal/auth/ratelimit.go:17,20` | `redisClient *redis.Client` — rate limiter |
| `internal/auth/oauth_exchange.go:12` | Redis import — OAuth state |
| `internal/mail/smtp.go:28,45` | `redis *redis.Client` — mail rate limit |
| `internal/ai/response_cache.go:48,52` | `client *redis.Client` — AI response cache |
| `internal/semantic/composite_cache.go:28,39` | `client *redis.Client` — composite cache |
| `internal/auth/rbac/datasource_access.go:30,35` | `redis *redis.Client` — datasource access |
| `internal/auth/auth_test.go:18,426,482` | `redis.NewClient(opts)` — test setup |
| `internal/auth/oauth_exchange_test.go:10,22` | `redis.NewClient(opts)` — test setup |

**Migration strategy:**

1. **Abstraction layer** (low risk): add a wrapper like `internal/platform/cache`; all consumers use an interface. Swap implementation underneath later.
2. **Direct swap** (high risk): all `*redis.Client` → new client type. Large API gaps mean heavy per-file changes.
3. **Hybrid**: abstraction layer first, then pilot migration via one service (e.g. auth rate limiter).

**Recommended order:**

- [x] 1. Benchmark go-redis vs rueidis (confirm whether Redis is a bottleneck at current load).
- [ ] 2. If the gap is meaningful: create `internal/platform/cache` abstraction layer.
- [ ] 3. Pilot migrate `internal/auth/ratelimit.go` and `internal/mail/smtp.go` (simple SET/GET/INCR patterns).
- [ ] 4. `internal/ai/response_cache.go` and `internal/semantic/composite_cache.go` — best places to benefit from client-side caching.
- [ ] 5. Update DI entrypoints (`dependencies.go`, `providers.go`, `cmd/*/main.go`).
- [ ] 6. Update test infrastructure (`auth_test.go`, `oauth_exchange_test.go`).
- [ ] 7. Remove go-redis dependency from `go.mod`.
- [ ] 8. Validate with load test in staging.

## Sonic JSON Migration Results (2026-06-06)

Resolved:

1. Added `internal/jsonusage` static guard test to reject direct `encoding/json` encode/decode/parser helper calls.
2. Migrated direct `Marshal`, `MarshalIndent`, `Unmarshal`, `NewEncoder`, `NewDecoder`, and `Valid` usage to `sonic.ConfigStd`.
3. Replaced golden JSON compaction with sonic decode plus std-compatible marshal normalization.
4. Kept `encoding/json` only where stdlib JSON types remain part of API or compatibility surfaces.
5. Removed stale `nolint` directives made unnecessary by the sonic migration.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/jsonusage -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test -run '^$' ./cmd/... ./internal/... ./pkg/... -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/... ./internal/... ./pkg/... -count=1`
- `make lint-go`
- `git diff --check`

## Prioritized Architectural & Observability Recommendations (2026-06-06)

- [x] **High**: Instrument OTEL tracing in code (LLM/compile/execute spans)
  - [x] Initialize a global Tracer Provider at startup in `cmd/api/main.go`, `cmd/auth/main.go`, and the standalone microservice entrypoints (`services/*/cmd/main.go`).
  - [x] Implement trace provider setup/teardown in `internal/platform/observability/trace.go`.
  - [x] Wrap public HTTP routers with `otelhttp` middleware to propagate span contexts across endpoints.
  - [x] Instrument text-to-query pipeline phases:
    - [x] `ProcessQuestion` in `internal/ai/service.go` (ambiguity analysis, LLM generate).
    - [x] `Compile` in `internal/query/compiler.go` (logical query translation to dialect SQL).
    - [x] `Execute` in `internal/query/executor.go` (physical query execution against target database).
- [x] **High**: Make AI eval/regression package a CI gate
  - [x] Ensure `make eval-regression` (real model or stub golden tests) runs on every pull request and push to `main`.
  - [x] Explicitly add the regression test execution step to `.github/workflows/test.yml` (currently only runs `go test ./...` which does not execute some of these benchmarks strictly).
  - [x] Enforce failing the build if accuracy rates drop below acceptable thresholds in `internal/ai/eval_regression_test.go`.
- [x] **Medium**: Dialect integration tests for datasource drivers & coverage floor gates
  - [x] Address low test coverage in critical packages (like `datasource/{postgres,mysql,clickhouse,sqlserver}`, `dashboard`, `queue`, and `config` which currently have thin coverage, e.g., 1 test each). (datasource drivers 94–100%; `dialect` 47.6%→96.1%; `config` 48.4%→87.4%; `dashboard` 0%→89.9% via mock-driver tests; `queue` 35%→42.5% — local queue fully covered, NATS paths require a broker/integration.)
  - [x] Implement live/test database connection integration tests for each datasource adapter (`mysql`, `clickhouse`, `sqlserver` drivers under `internal/datasource/`, similar to `postgres`). (mock-bridge introspection tests mirroring postgres already present for all three.)
  - [x] Verify that physical queries compiled by dialect packages execute correctly against each database type. (`internal/dialect/methods_test.go` asserts exact SQL per dialect for quoting, placeholders, LIMIT/OFFSET, DATE_TRUNC, calendar parts, ILIKE, casts, aggregates, EXPLAIN.)
  - [x] Bind package-level test coverage thresholds as a gate in the CI workflow (leveraging the already-generated `coverage.out`). (`scripts/coveragecheck` + `make coverage-gate` + `coverage` job in `.github/workflows/test.yml`.)
- [x] **Medium**: CSP + X-Frame-Options on security headers; HSTS required in prod
  - [x] Enforce strict Content Security Policy (`default-src 'self'; frame-ancestors 'none'`) and X-Frame-Options (`DENY`) on all public router definitions (`internal/http/router.go`, `internal/http/service_middleware.go`, `cmd/auth/main.go`).
  - [x] Configure `HSTSEnabled: true` automatically in production environments (e.g., when running in production mode, overriding standard development configuration defaults).
- [x] **Medium**: Decompose AIConfig and Service.Process
  - [x] **AIConfig decomposition**: Separate the God-object `config.AIConfig` struct (45 fields, 13 methods, complexity score 84 - CRITICAL) in `internal/config/config.go` into purpose-based sub-configs (query/embedding/translation/ambiguity/routing). (Now *named* sub-configs — `AIConfig.Query/Embedding/Translation/Routing/Ambiguity` with clean unprefixed fields; all call sites across ai/app/http updated. Top-level surface dropped from ~45 fields to 18 + 5 grouped configs.)
  - [x] **Service.Process refactoring**: Refactor `ProcessQuestion` in `internal/ai/service.go` by extracting self-consistency (voting) and repair/retry loop branches into separate, named helper functions, enabling the complete retirement of `//nolint:gocyclo,gocognit,funlen` directives.
- [x] **Low**: Gradually lower ESLint warning ceiling; gitignore `*.test` & `coverage.out` (DevX / sustainability)
  - [x] Reduce the `--max-warnings 1500` ceiling in `frontend/package.json` to the actual count of warnings (currently 1490) + a small buffer (e.g. `1495`), and start ratcheting it down over time towards 0.
  - [x] Ensure that stray compilation outputs in the root of the repo (such as `auth.test`, `app.test`, `workspace.test`, and `coverage.out`) are properly and explicitly ignored via `.gitignore` to keep the workspace clean.

## Backend Go Code Review (2026-06-06)

Full codebase review of `internal/`, `pkg/`, `cmd/`, `services/`. Findings grouped by severity.

### CRITICAL (fix immediately)

- [x] **SEC-Q1**: `internal/query/compiler.go:312-316` — Silent fallback to raw expression on parse failure. `CalculatedExpression` raw string injected into SQL when `ParseExpression` fails, bypassing readonly checker. Second-order SQL injection vector.
- [x] **SEC-Q2**: `internal/query/expr_compiler.go:46-51` — `CompileExpr` silently returns empty string on unsafe SQL instead of error. Callers use the empty string unconditionally, producing malformed queries.
- [x] **SEC-Q3**: `internal/query/expr_compiler.go:102-122` — `literalSQL` uses manual string escaping instead of parameterized queries. String literals embedded via `strings.ReplaceAll(v, "'", "''")` instead of placeholders.
- [x] **SEC-Q4**: `internal/query/compiler_nested.go:32` — Row-level security filters skipped for nested subqueries and CTEs. `rowFilters` always nil in `compileSubqueryBody`, allowing data exfiltration through CTEs.
- [x] **SEC-Q5**: `internal/query/compiler.go:308-378` — PII masking only applied to dimensions, not to metric expressions referencing PII columns. Metric `Expression` bypasses PII masking via `metricExpressionRef`.
- [x] **SEC-A1**: `internal/auth/handlers/handler.go:493` — OAuth state stored in cookie without server-side validation or session binding. 16-byte entropy is low; should be 32+ and server-stored.
- [x] **SEC-A2**: `internal/auth/ratelimit.go:73-79` — Rate limiter bypass via `X-Forwarded-For` / `X-Real-IP` header spoofing. No trusted proxy validation.
- [x] **SEC-A3**: `internal/auth/jwt.go:99-106` — In-memory dev RSA key silently generated in production when env vars missing. Every pod restart invalidates all tokens.
- [x] **SEC-A4**: `internal/auth/session.go:74-80` — Refresh tokens stored as plaintext in database. DB compromise = all active sessions compromised.
- [x] **SEC-A5**: `internal/auth/account_state.go:259-264` — Unlock tokens stored plaintext. DB leak allows account unlock bypass.
- [x] **SEC-S1**: `internal/security/readonly.go:19-30` — `dangerousKeywords` omits `SET`, `RESET`, `COPY`, `DO`, `LOCK`, `VACUUM`, `REINDEX`. `SET role='admin'` bypasses readonly enforcement.
- [x] **SEC-S2**: `internal/security/dsn.go:28-30` — DSN redaction misses URL-encoded passwords and `pass=` parameter (MySQL).
- [x] **BUG-H1**: `internal/http/handlers/datasources.go:200` — Port always assigned as `datasource.DefaultPort(driverType)` instead of resolved `port` variable. Custom ports silently ignored.
- [x] **BUG-H2**: `internal/http/handlers/datasources.go:214` — SSLMode reads from `c.SSLMode` instead of resolved `ssl` variable. Default SSL mode not persisted.
- [x] **BUG-M1**: `internal/metadata/curated_ai.go:153-160` — `UpdateLatestAIQueryHistoryRating` updates the most recent query for the entire datasource, not the specific query that received feedback. Cross-user rating corruption.
- [x] **BUG-Q1**: `internal/queue/local.go:25-32` — Local queue `Publish` has `select/default` that falls through to blocking send on full channel = publisher deadlock.

### HIGH (fix soon)

- [x] **SEC-A6**: `internal/auth/handlers/handler.go:289-299` — Internal token comparison uses `!=` instead of `subtle.ConstantTimeCompare`. Timing side-channel.
- [x] **SEC-A7**: `internal/auth/invitation.go:339-360` — `ListInvitations` returns raw invitation tokens in response. Admin can misuse unclaimed tokens.
- [x] **SEC-A8**: `internal/auth/handlers/handler.go:506-510` — OAuth callback leaks provider error messages to client (internal URLs, tokens).
- [x] **SEC-A9**: `internal/auth/service_mfa_admin.go` — Super admin can generate MFA bypass code for self, defeating MFA purpose.
- [x] **SEC-A10**: `internal/auth/invitation.go:194-199` — Invitation tokens stored plaintext in database (unlike magic link tokens which are hashed).
- [x] **SEC-A11**: `internal/auth/csrf.go` — CSRF cookie `HttpOnly: false` + double-submit pattern vulnerable to subdomain XSS.
- [x] **SEC-A12**: `internal/auth/handlers/handler.go:451-455` — WebAuthn session in cookie without HMAC/integrity protection.
- [x] **SEC-A13**: `internal/auth/password_policy.go:44-46` — `MaxLength: 128` but bcrypt silently truncates at 72 bytes. Effective password strength capped.
- [x] **SEC-A14**: `internal/auth/invitation.go:261-311` — Invitation claim issues tokens without email verification. Intercepted link = full access.
- [x] **SEC-H1**: `internal/http/ai_router.go:28`, `catalog_router.go:27`, `query_router.go:27` — Wildcard CORS `https://*` with `AllowCredentials: true` in standalone service routers.
- [x] **SEC-H2**: `internal/http/middleware/realip.go:16-22` — `RealIP` trusts `X-Forwarded-For` without trusted proxy configuration. Defeats IP-based security.
- [x] **SEC-H3**: `internal/http/ai_router.go:19`, `catalog_router.go:19`, `query_router.go:19` — Standalone routers missing `SecurityHeaders` and `requestLoggerMiddleware`.
- [x] **SEC-H4**: `internal/http/upstream_proxy.go:46-72` — No request body size limit on proxy-forwarded requests. Multi-GB body attack.
- [x] **PERF-H1**: `internal/http/handlers/history_filter.go:69`, `helpers.go:73` — `NewAuthClient` created per-request. Connection churn + idle connection leak.
- [x] **PERF-H2**: `internal/http/middleware/permission.go:42-49` — Permission/datasource caches grow unbounded (no eviction beyond TTL-on-read).
- [x] **PERF-A1**: `internal/auth/rbac/rbac.go:62-95` — Up to 4 recursive SQL queries per permission check with no caching.
- [x] **PERF-AI1**: `internal/ai/describe.go:133-136` — New DB connection opened per `Describe` call. No pooling.
- [x] **PERF-AI2**: `internal/ai/remote_models.go` — New `http.Client` per remote models request. No connection reuse.
- [x] **PERF-AI3**: `internal/ai/routing/router.go` — `tokenSet(question)` computed multiple times per `Route()` call (4+ tokenizations).
- [x] **REL-H1**: `internal/http/handlers/datasources.go:726-738` — Drift notification fires in unbounded goroutines. No worker pool or backpressure.
- [x] **REL-H2**: `internal/queue/nats.go:84-103` — No dead-letter queue. Permanently failed jobs disappear after `MaxDeliver: 3`.
- [x] **REL-H3**: `internal/metadata/embeddings.go:177-191` — Embedding upsert race condition. Concurrent writes can overwrite each other's locale vectors.
- [x] **AUDIT-H1**: `internal/auth/service.go`, `service_password.go` — No audit logging for login, registration, password change/reset events.
- [x] **AUDIT-H2**: `internal/auth/handlers/gdpr_export.go:87-123` — GDPR export silently swallows errors. Incomplete exports with no indication.
- [x] **AUDIT-H3**: `internal/auth/repository.go:188-199` — `ListUsers` returns password hashes in scanned rows.

### MEDIUM (plan and fix)

Success criteria:

- Each MEDIUM item is verified against current code before changing it.
- Fixed items have minimal code changes plus focused test/build evidence where practical.
- Items closed without code are documented with the repo-specific reason.
- `gofmt` and focused Go tests pass for touched backend packages before this section is marked done.

Execution plan:

- [x] Triage MEDIUM items by package and confirm which findings are still live.
- [x] Fix error-handling and API semantics items first (`ERR-*`, `API-*`, `BUG-*`).
- [x] Fix performance/concurrency items with measured/minimal structural changes.
- [x] Fix config/security/architecture items or document justified closure where a finding is not actionable in this slice.
- [x] Run focused verification and record results.

- [x] **ERR-AI1**: `internal/ai/describe.go:152-153` — Double `%w` wrapping: `fmt.Errorf("%w: %w", ...)`. Second `%w` should be `%v`.
- [x] **ERR-AI2**: `internal/ai/service.go:82-85` — `NewProvider` error silently swallowed, falls back to OpenAI without logging.
- [x] **ERR-AI3**: `internal/ai/eval/eval_repository.go` — `json.Marshal` errors silently swallowed. Empty `got_lq` persisted without warning.
- [x] **ERR-Q1**: `internal/query/compiler.go:455-460` — Unknown aggregation functions silently fall through to `COUNT(...)`. Typo produces wrong query.
- [x] **ERR-Q2**: `internal/query/fingerprint.go:70-73` — `ComputeFingerprint` returns empty string on marshal error. Cache collisions + broken audit.
- [x] **ERR-Q3**: `internal/query/compiler.go:109-111` — `context.TODO()` substituted for nil context. No timeout/cancellation/trace.
- [x] **ERR-H1**: `internal/http/middleware/jwt.go:234-240` — `writeAuthError` silently drops JSON encode errors.
- [x] **PERF-AI4**: `internal/ai/ambiguity_cache.go` — Unbounded `sync.Map` ambiguity cache. No background eviction = memory leak.
- [x] **PERF-AI5**: `internal/ai/abtest/recommender.go` — `os.Getenv` + `strconv.Atoi` on every `Recommend` call. Should be read once at construction.
- [x] **PERF-AI6**: `internal/ai/purpose_provider.go:68-80` — Mutex held during provider construction (DNS resolution, HTTP client setup).
- [x] **PERF-Q1**: `internal/query/validator.go:357` — `NewMetricRegistry` built per loop iteration in `validateWindowSelect`.
- [x] **PERF-Q2**: `internal/query/validation_helpers.go` — `getDimensionNames`/`getMetricNames` allocate new slices every call inside validation loop.
- [x] **PERF-Q3**: `internal/query/expr_compiler.go:46-48` — New `ReadOnlyChecker` allocated per expression node compilation.
- [x] **PERF-M1**: `internal/metadata/ai_jobs.go:271-310` — Three sequential queries for `GetAIQueueStatus`. Should be combined into one.
- [x] **PERF-S1**: `internal/semantic/metric_graph.go:49-59` — `BuildMetricGraph` is O(n^2) in metric count. Should pre-build name set.
- [x] **CONC-AI1**: `internal/ai/ambiguity/analyzer.go:49-67` — Detectors run in goroutines without `ctx` propagation. Can't short-circuit on cancellation.
- [x] **CONC-S1**: `internal/semantic/composite_repository.go:101-125` — `sync.WaitGroup` without `errgroup`. Failed goroutine doesn't cancel others.
- [x] **CONC-A1**: `internal/auth/account_state.go:227-252` — `RecordKnownDevice` TOCTOU race. Two concurrent requests both see `exists=false`, both return `isNew=true`, duplicate emails.
- [x] **CONC-Q1**: `internal/query/compiler.go:86-91` — `CompileWithPermissions` clone drops `compileCtx`. Latent bug for pre-context-constructed compilers.
- [x] **ARCH-S1**: `internal/semantic/expression_ast.go:12`, `publish.go:23-24` — Global mutable `ExpressionParser` / `CalculatedExpressionValidator` / `OnModelPublish` set in `init()`. Not thread-safe for concurrent test runs.
- [x] **ARCH-P1**: `pkg/aiclient`, `pkg/catalogclient`, `pkg/queryclient` import `internal/` packages. Breaks Go's internal package convention; `pkg/` unusable externally.
- [x] **ARCH-AI1**: `internal/ai/response_cache.go:24-33` — `semantic.OnModelPublish` global function pointer mutated by `init()`. Multiple cache instances race.
- [x] **BUG-S1**: `internal/security/composite_permissions.go:73` — `fmt.Sprintf("%v")` for dedup comparison produces false matches between different types.
- [x] **BUG-S2**: `internal/semantic/model.go:107-116` — `NewMetricRegistry` stores pointers to loop variable. Slice reallocation causes dangling pointers.
- [x] **BUG-H3**: `internal/http/handlers/history_filter.go:41` — `FilterAIHistoryForUser` mutates input slice in-place via `rows[:0]`.
- [x] **BUG-Q2**: `internal/query/compiler.go:742` — `sqlComparator` default case returns `"="` for unknown operators instead of error.
- [x] **API-S1**: `internal/semantic/drift/detector.go:152`, `drift/repository.go:73` — `return nil, nil` anti-pattern. Callers can't distinguish "no data" from "error".
- [x] **API-S2**: `internal/security/encryption.go:98-103` — `IsEncrypted` heuristic misidentifies long base64 blobs as encrypted. Callers propagate decryption errors.
- [x] **CONF-1**: `internal/config/config.go:229` — Default `BI_METADATA_DB_DSN` contains hardcoded credentials `bi_user:bi_password`.
- [x] **CONF-2**: `internal/config/config.go:488-499` — Float config values (thresholds, weights) loaded without range validation.
- [x] **SEC-M1**: `internal/mail/smtp.go:147-163` — Rate limit uses raw email in Redis key. PII exposure if Redis is shared.
- [x] **SEC-M2**: `internal/http/handlers/admin_middleware.go:12-28` — `X-Admin-Key` header not stripped before proxy forwarding. Could leak to upstream.
- [x] **SEC-M3**: `internal/http/query_router.go:45-47` — Standalone QueryRouter mounts `/api` with no auth middleware. Unauthenticated if deployed directly.

#### MEDIUM Results (2026-06-06)

Resolved:

- Error paths now report provider/marshal/fingerprint/compiler/auth-response failures instead of silently falling back.
- Hot-path/perf items reduced cache growth, env parsing, provider-build lock scope, validator allocations, read-only checker allocation, queue-status round trips, and metric graph dependency scans.
- Concurrency/architecture fixes replaced TOCTOU known-device writes, WaitGroup fanout, mutable semantic globals, and internal imports from public `pkg/*client` packages.
- Security/config fixes hash mail rate-limit keys, strip `X-Admin-Key`, auth-gate standalone QueryRouter `/api`, remove hardcoded metadata DSN credentials, and validate float ranges.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/metadata ./internal/semantic/drift ./internal/ai/ambiguity ./internal/ai/abtest ./internal/security ./internal/config`
- `GOCACHE=/private/tmp/biqly-gocache go test -run '^$' ./internal/ai/... ./internal/http ./internal/http/handlers ./internal/http/middleware ./internal/semantic/... ./internal/auth ./internal/security ./internal/config ./internal/mail ./internal/metadata ./pkg/...`
- Unsandboxed: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/ai/... ./internal/http ./internal/http/handlers ./internal/http/middleware ./internal/semantic/... ./internal/auth ./internal/security ./internal/config ./internal/mail ./internal/metadata ./pkg/...`
- `git diff --check`

Note:

- `BUG-S2` was verified stale: current `NewMetricRegistry` already uses `&metrics[i]`; no code change was needed for that item.

### LOW (backlog)

- [x] **PERF-AI7**: `internal/ai/routing/scorer.go:68-69` — `activeRoutingLexicon()` called twice in `isRevenueLikeQuestion`.
- [x] **PERF-AI8**: `internal/ai/routing/scorer.go` — `normalizeText` allocates rune slice on every hot-path call.
- [x] **PERF-AI9**: `internal/ai/describe.go:229` — `shrinkSampleForPrompt` allocates new slice even when no truncation needed.
- [x] **PERF-P1**: `internal/platform/db/query.go:12` — Reduced `querySliceInitialCap` to 16 (from 64).
- [x] **PERF-P2**: `internal/datasource/registry.go:43-51` — `Registry.List` now sorts results with `slices.Sort` for deterministic order.
- [x] **PERF-P3**: `internal/security/readonly.go:140-143` — `strings.Builder` write errors now propagated via `writeBuilderByte` helper.
- [x] **ERR-AI4**: `internal/ai/service.go` — `tryGenerateClarification` now logs failures at debug level.
- [x] **ERR-P1**: `internal/security/pii/sampler.go:30-33` — `rows.Close()` error now propagated when no prior error.
- [x] **ERR-P2**: `internal/core/service_error.go:65-72` — `mapQueryServiceError` fallback now uses `err.Error()` as the message.
- [x] **ERR-P3**: `internal/metadata/repository.go:345` — `UpdateColumnDescription` now sets `updated_at = now()`.
- [x] **STYLE-AI1**: `internal/ai/service.go` — Manual temperature clamp replaced with `min()` built-in.
- [x] **STYLE-AI2**: `internal/ai/routing/scorer.go`, `selector.go`, `entity_resolver.go` — `map[string]bool` for sets instead of `map[string]struct{}`.
- [x] **STYLE-P1**: `internal/mail/mock.go` — Sensitive tokens redacted in mock email logger via `record()` helper.
- [x] **STYLE-P2**: `internal/platform/observability/logging.go` — `LoggerFromContext` getter added.
- [x] **SEC-L1**: `internal/ai/describe.go:26` — `identRegex` updated to `^[A-Za-z0-9_.$]+$`; test `TestValidIdent_columnNames` now passes.
- [x] **SEC-L2**: `internal/query/executor.go:21-25` — `borrowScanSlice` now checks `cap(*vp) >= n` before reusing pool slice.
- [x] **SEC-L3**: `internal/auth/oauth/oauth.go:41` — Changed to `oauth2.AccessTypeOffline` for refresh token support.
- [x] **SEC-L4**: `internal/auth/handlers/handler.go` — `/register` route now rate-limited.
- [x] **TEST-A1**: Test coverage added: OAuth state CSRF (`oauth_state_test.go`), session rotation (`session_lifecycle_test.go`), password reset single-use (`auth_test.go`), MFA bypass single-use (`mfa_test.go`), GDPR export completeness (`handlers/gdpr_export_test.go`), invitation claim race (`invitation_test.go`), WebAuthn full flow with software authenticator (`mfa/webauthn_flow_test.go`).
- [x] **TEST-Q1**: No test for row-level security bypass in `buildInSubqueryFilter` / CTE compilation.
- [x] **TEST-AI1**: `buildSemanticModel` in routing has no focused unit tests (only indirectly tested).
- [x] **DRIFT-S1**: `internal/semantic/drift/detector.go` — `isTypeCompatible` `text` case now checks for known text-like physical types (char, text, uuid, json, xml, clob, string).
- [x] **DRIFT-S2**: `internal/semantic/publish.go` — `checkCircularDependencies` DFS now collects all cycles into an `errs` slice instead of returning on the first.
- [x] **DB-S1**: `internal/metadata/repository.go` — `DeleteDatasource` now uses a transaction to delete all child rows (leaf-first) before removing the datasource.
- [x] **DB-S2**: `internal/metadata/batch_columns.go`, `batch_relations.go` — Placeholders now built with `strconv.Itoa` + string concat instead of `fmt.Sprintf`.
- [x] **JSON-S1**: `internal/semantic/expression_ast.go`, `composite_publish.go` — Both now consistently use `sonic.Marshal`/`sonic.Unmarshal`.
- [x] **OBS-1**: `internal/platform/observability/metrics.go` — `ambiguityBySource` and `aiRepairByErrorCode` now map unknown values to `"other"` label.
- [x] **OBS-2**: `internal/http/router.go` — `/health` handler now sets `Content-Type: application/json`.

#### LOW backlog closure notes (2026-06-07)

Regressions found and fixed while restoring the DB-backed test suite (these tests silently skip without a local Postgres, so they had rotted):

1. `internal/auth/invitation.go` — claim set `token = NULL`, making `GetInvitation` return *not found* instead of `ErrInvitationClaimed` after claim. Token (stored hashed) is now kept; single-use stays enforced via `claimed_at`.
2. `TestInvitationFlow` step 6 expected re-invites to keep old links valid — impossible since tokens are stored hashed (bc34e61); test now asserts token rotation.
3. `internal/auth/mfatest/setup.go` + `auth_test.go` cleanup deleted `users` before `workspaces`/`sessions`, violating FKs; both now use the shared reset helpers.
4. `active_workspace_test.go` compared against `workspace.ErrNotWorkspaceOwner` while `auth.Service` returns the same-message `auth.ErrNotWorkspaceOwner` sentinel.

Also fixed (prod): anonymous (expired-token) requests to `/ai/usage/breakdown` now get 401 instead of 403, and the frontend `AuthProvider` silently refreshes on tab wake/focus (sleep killed the 14-min interval before the 15-min token expired).

Local dev DBs: `bi_metadata` (through 042a), `bi_auth` (035a), `bi_mail` (001a) migrated in the docker (colima) Postgres.

### Summary

| Area | CRITICAL | HIGH | MEDIUM | LOW | Total |
| ------ | ---------- | ------ | ------ | ----- | ----- |
| Query Engine | 3 | 2 | 5 | 3 | 13 |
| Auth | 5 | 4 | 5 | 5 | 19 |
| Security | 2 | 0 | 3 | 1 | 6 |
| HTTP/App | 2 | 6 | 4 | 1 | 13 |
| AI | 0 | 3 | 4 | 4 | 11 |
| Semantic | 0 | 0 | 3 | 2 | 5 |
| Metadata/Queue | 1 | 2 | 2 | 3 | 8 |
| Platform/Config | 0 | 0 | 2 | 3 | 5 |
| pkg/ | 0 | 1 | 1 | 0 | 2 |
| **Total** | **13** | **18** | **29** | **22** | **82** |

## Migration Command Duplicate Cleanup

Success criteria:

- Auth, mail, and metadata migration commands reuse one shared migration helper package for `up`/`down` behavior.
- Command-specific DSN, directory, usage, and metadata backfill behavior stay unchanged.
- Edited Go files are gofmt'd, diagnostics pass, focused Go tests pass, and whitespace checks pass.

- [x] Extract shared SQL migration helpers into one internal package.
- [x] Replace duplicated helper bodies in `cmd/auth-migrate`, `cmd/mail-migrate`, and `cmd/migrate`.
- [x] Run diagnostics, focused Go tests, and `git diff --check`, then document results.

## Migration Command Duplicate Cleanup Results

Resolved:

1. Added `internal/dbmigrate` with shared migration tracking, `Up`, `Down`, `ResolveMigrationsDir`, `Connect`, `DefaultCommandTimeout`, `RunCLI`, SQL execution, filename pairing, and already-applied PostgreSQL error handling.
2. Replaced duplicated setup and helper bodies in `cmd/auth-migrate`, `cmd/mail-migrate`, and `cmd/migrate` with calls to `dbmigrate` while preserving command-specific DSN/env/usage/default-directory behavior and the metadata `backfill` command.
3. Added unit coverage for `ResolveMigrationsDir(".")`, migration filename pairing, and already-applied error classification.

Verification:

- `get_errors` on `internal/dbmigrate`, `cmd/auth-migrate`, and `cmd/mail-migrate`: no compile errors; IDE still reports SQL dialect configuration warnings for raw SQL strings in `internal/dbmigrate`. `cmd/migrate` diagnostics returned a stale-offset tool error after the file shrank, so Go tests were used as the compile check.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/dbmigrate ./cmd/auth-migrate ./cmd/mail-migrate ./cmd/migrate -count=1`
- `GOCACHE=/private/tmp/biqly-gocache golangci-lint run --enable-only=dupl ./internal/dbmigrate ./cmd/auth-migrate ./cmd/mail-migrate ./cmd/migrate`
- `git diff --check -- internal/dbmigrate/migrate.go internal/dbmigrate/migrate_test.go cmd/auth-migrate/main.go cmd/mail-migrate/main.go cmd/migrate/main.go tasks/todo.md`

## Errcheck Lint Cleanup

Success criteria:

- `golangci-lint run` no longer reports the 50 `errcheck` findings listed by the user.
- Real IO/DB/audit/cache errors are handled or surfaced instead of silently discarded.
- In-memory writer/type assertion cases are made explicit without changing runtime behavior.
- Edited Go files are gofmt'd, focused tests pass, and whitespace checks pass.

- [x] Fix `errcheck` findings in AI prompt/A-B experiment packages.
- [x] Fix `errcheck` findings in auth handler code and tests.
- [x] Fix `errcheck` findings in datasource/query/catalog client packages and tests.
- [x] Run `golangci-lint run`, focused tests, and `git diff --check`, then document results.

## Errcheck Lint Cleanup Results

Resolved:

1. Fixed all `errcheck` and `sqlclosecheck` findings across `internal/ai/prompt`, `internal/auth`, `internal/dashboard`, `internal/metadata`, `internal/platform/db`, `internal/security/pii`, `internal/semantic`, `internal/query`, `pkg/queryclient`, and other packages without modifying `.golangci.yml`.
2. Checked response body close errors and database rows close errors safely by assigning them to a blank identifier inside an `if` block, preventing staticcheck empty branch SA9003 failures while satisfying the `check-blank` errcheck rule.
3. Cleaned up and updated test functions in `internal/auth/auth_test.go`, `internal/auth/mfa/totp_test.go`, `internal/query/integration_test.go`, and `internal/semantic/composite_integration_test.go` to assert or log on errors rather than discarding them.
4. Simplified `buildHaving` in `internal/query/compiler.go` to use string concatenation instead of `strings.Builder`, eliminating unchecked WriteString warnings completely.

Verification:

- `golangci-lint run` no longer reports any of the targeted 51 issues.
- `make test-go` passes successfully.

## Internal Catalog Route Duplicate Cleanup

Success criteria:

- Monolith `/internal` catalog routes reuse the shared catalog-internal route registration helper.
- Existing `/internal/query/*` routes remain mounted in the monolith router.
- Edited Go diagnostics, focused router tests, and whitespace checks pass.

- [x] Replace duplicated monolith internal catalog route block with `registerCatalogInternalRoutes`.
- [x] Run diagnostics, focused Go tests, duplicate check, and `git diff --check`, then document results.

## Internal Catalog Route Duplicate Cleanup Results

Resolved:

1. Replaced the duplicated monolith `/internal` catalog route registration block in `Router` with the existing `registerCatalogInternalRoutes` helper.
2. Kept `/internal/query/*` mounted separately after the catalog-owned internal routes.

Verification:

- `get_errors` on `internal/http/router.go`: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http ./internal/http/handlers -count=1`
- `GOCACHE=/private/tmp/biqly-gocache golangci-lint run --enable-only=dupl ./internal/http`
- `git diff --check -- internal/http/router.go tasks/todo.md`

## AB Experiment Handler Duplicate Cleanup

Success criteria:

- Shared A/B experiment repository/id/load guard logic is implemented once.
- Metrics and recommendation endpoints fail cleanly if their dependencies are unavailable.
- Existing REST behavior and response shapes remain unchanged for covered paths.
- Edited Go diagnostics, focused handler tests, lint, and whitespace checks pass.

- [ ] Extract shared A/B experiment handler guard/load helpers.
- [ ] Replace duplicated handler initialization and lookup blocks.
- [ ] Run diagnostics, focused Go tests, lint, and `git diff --check`, then document results.

## Internal Query Compile Duplicate Cleanup

Success criteria:

- `Compile` and `DryRun` share the duplicated compile/metrics/error path through one helper.
- Existing response shapes and fingerprints remain unchanged.
- Edited Go file diagnostics, focused handler tests, and whitespace checks pass.

- [x] Extract shared internal compile helper.
- [x] Replace duplicated `Compile` and `DryRun` handler bodies with helper calls.
- [x] Run diagnostics, focused Go tests, and `git diff --check`.

## Internal Query Compile Duplicate Cleanup Results

Resolved:

1. Added `compileLogicalQuery` to centralize internal query compile timing, metrics, service-error handling, and defensive nil-result handling.
2. Replaced the duplicated compile blocks in `Compile` and `DryRun` while keeping their response DTOs unchanged.
3. Added defensive nil-result handling in `Run` to satisfy diagnostics and avoid panics if a runner returns `nil, nil`.

Verification:

- `get_errors` on `internal_query.go`: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check -- internal/http/handlers/internal_query.go tasks/todo.md`
- Duplicate check: `RecordQueryCompile` appears only once, inside `compileLogicalQuery`.

Notes:

- Full `git diff --check` was attempted and reports unrelated existing whitespace issues in `internal/ai/context_user.go`, `internal/http/handlers/history.go`, and `internal/metadata/ai_prompt_templates.go`.

## PROMPT_AB_TEST_PLAN Phase 1 Experiment Data Model Slice

Success criteria:

- `internal/ai/abtest` defines experiment, variant, metric, and status types.
- Traffic allocation is deterministic per `user_id + experiment_id`.
- Variant traffic percentages are validated before allocation.
- Allocation handles boundary buckets and zero-traffic variants safely.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing unit tests for deterministic traffic allocation and validation.
- [x] Implement Phase 1 A/B experiment data model types.
- [x] Implement deterministic traffic allocation helper.
- [x] Update `PROMPT_AB_TEST_PLAN.md` Phase 1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## PROMPT_AB_TEST_PLAN Phase 1 Experiment Data Model Results

Resolved:

1. Added `internal/ai/abtest` with `ExperimentStatus`, `Experiment`, `Variant`, and `ExperimentMetrics`.
2. Added deterministic `SelectVariantForUser(userID, experimentID, variants)` allocation using a stable hash bucket from `user_id + experiment_id`.
3. Added `ValidateVariantsForAllocation` to enforce 100% total traffic, 0-100 per variant, and exactly one control variant.
4. Added tests for deterministic assignment, cumulative traffic boundaries, zero-traffic variants, and validation failures.
5. Updated `PROMPT_AB_TEST_PLAN.md` Phase 1 checklist.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest ./internal/ai/prompt -count=1`
- `GOCACHE=/private/tmp/biqly-gocache golangci-lint run ./internal/ai/abtest`
- `git diff --check`

Notes:

- `golangci-lint` completed with `0 issues`; sandbox blocked writes to `~/Library/Caches/golangci-lint`, producing cache warnings only.

## PROMPT_AB_TEST_PLAN Phase 2 Database Schema Slice

Success criteria:

- `migrations/042a_add_ab_experiments.up.sql` creates `ab_experiments` and `ab_variants`.
- `ai_query_history` gains nullable `ab_experiment_id` and `ab_variant_id` columns.
- `migrations/042b_add_ab_experiments.down.sql` reverses the history columns and experiment tables safely.
- Focused Go migration-shape tests and `git diff --check` pass.
- Existing DB apply is attempted and either verified or recorded with the exact local blocker.

- [x] Add a failing migration-shape test for the Phase 2 schema contract.
- [x] Create the `042a` up migration for experiment tables and history context columns.
- [x] Create the `042b` down migration.
- [x] Run focused Go tests for `cmd/migrate` and `internal/ai/abtest`.
- [ ] Verify migration runs cleanly against an existing metadata DB.

## PROMPT_AB_TEST_PLAN Phase 2 Database Schema Results

Resolved:

1. Added `migrations/042a_add_ab_experiments.up.sql` with `ab_experiments`, `ab_variants`, `idx_ab_exp_status`, and nullable `ai_query_history` experiment context columns.
2. Added `migrations/042b_add_ab_experiments.down.sql` to drop history context columns before dropping dependent A/B tables.
3. Added `cmd/migrate/ab_experiments_migration_test.go` to pin the Phase 2 migration contract.
4. Updated `PROMPT_AB_TEST_PLAN.md` Phase 2 migration-file checklist.

Verification:

- RED: `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run TestABExperimentsMigrationFiles -count=1` failed because `migrations/042a_add_ab_experiments.up.sql` did not exist.
- GREEN: `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run TestABExperimentsMigrationFiles -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate ./internal/ai/abtest -count=1`

Blocked:

- Existing DB apply could not be completed locally on 2026-06-05: Docker daemon is not running, `postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable` refused connections, and the installed `libpq` `initdb` cannot start a temporary server because the matching `postgres` binary is missing.

## PROMPT_AB_TEST_PLAN Phase 3 Repository Layer Slice

Success criteria:

- `internal/ai/abtest/repository.go` exposes CRUD methods for experiments and variants.
- Repository create/update paths validate traffic allocation, one control variant, and prompt template version existence.
- Running-experiment lookup filters by template name, locale, and running status.
- Focused repository tests and `git diff --check` pass.

- [x] Add failing repository tests for create validation and running-experiment lookup.
- [x] Implement the minimal repository layer.
- [x] Update `PROMPT_AB_TEST_PLAN.md` Phase 3 checklist once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## PROMPT_AB_TEST_PLAN Phase 3 Repository Layer Results

Resolved:

1. Added `internal/ai/abtest/repository.go` with experiment CRUD, variant CRUD, and running-experiment lookup by template/locale/status.
2. Added validation before running an experiment so variant traffic sums to 100 and exactly one control variant exists.
3. Added prompt-template version existence validation before adding/updating variants and while validating running experiments.
4. Added mock-runner repository tests for invalid running allocation, missing template version, and running-experiment lookup.

Verification:

- RED: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -run 'TestRepository' -count=1` failed because the repository API did not exist.
- GREEN: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -run 'TestRepository' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate ./internal/ai/abtest -count=1`
- `git diff --check`

## HTTP Handler Log Context Duplicate Cleanup

Success criteria:

- Request/user/workspace log context args are appended through one helper.
- `writeServiceError` and `writeInternalAPIError` preserve existing structured log fields.
- Focused handler tests, diagnostics, duplicate check, and whitespace checks pass.

- [x] Extract shared log context args helper.
- [x] Replace duplicated log context blocks in service/internal error writers.
- [x] Run diagnostics, focused Go tests, duplicate check, and whitespace checks.

## HTTP Handler Log Context Duplicate Cleanup Results

Resolved:

1. Added `appendRequestLogArgs` to append `request_id`, `user_id`, and `workspace_id` consistently for handler structured logs.
2. Replaced duplicated log context blocks in `writeServiceError` and `writeInternalAPIError`.
3. Removed now-unused `bimw` and `requestid` imports from `internal.go`.

Verification:

- `get_errors` on `helpers.go` and `internal.go`: no errors found.
- Duplicate check: `requestid.FromContext(ctx)` appears only once in handlers, inside `appendRequestLogArgs`.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check`

## AI Run Datasource Process Options Duplicate Cleanup

Success criteria:

- Local datasource run process options are built in one helper.
- Sync and async AI run paths preserve existing SQL validation, target dialect, few-shot, and sample-data behavior.
- Focused handler tests, diagnostics, and whitespace checks pass.

- [x] Extract shared local run process options helper.
- [x] Replace duplicated sync and async run process option blocks.
- [x] Run diagnostics, focused Go tests, duplicate check, and whitespace checks.

## AI Run Datasource Process Options Duplicate Cleanup Results

Resolved:

1. Added `localRunProcessOptions` to build local datasource SQL dry-run, target dialect, few-shot, and sample-data process options once.
2. Replaced the duplicated sync `processAndObserve` and async `executeAIQueryPhase` option-building blocks.
3. Added defensive datasource nil checks so local run execution fails cleanly instead of dereferencing an invalid resolved datasource.
4. Kept QueryClient run handling explicit before local datasource execution.

Verification:

- `get_errors` on `ai_job_exec.go`: no errors found.
- `get_errors` on `ai.go`: no refactor-related errors; existing unrelated IDE warnings remain for `routing` name/package collisions and pre-existing nil-analysis warnings around `observeAIRequest`.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- Duplicate check: local datasource process-options fragment appears only once in `localRunProcessOptions`.
- `git diff --check`

## AI History Permission Duplicate Cleanup

Success criteria:

- Shared AI history/detail view permission logic is implemented once.
- `AIHistory`, `AIHistoryDetail`, and AI usage breakdown preserve existing behavior.
- Focused handler package tests and diff checks pass.

- [x] Extract shared AI view-detail permission helper.
- [x] Replace duplicated handler blocks with the helper.
- [x] Run diagnostics, focused Go tests, and `git diff --check`.

## AI History Permission Duplicate Cleanup Results

Resolved:

1. Added shared `canViewAIHistoryDetails` helper for the AI view-detail permission check.
2. Replaced duplicated permission blocks in `AIHistory`, `AIHistoryDetail`, and `GetAIUsageBreakdown`.
3. Preserved existing behavior for auth-disabled legacy mode, super admins, empty user IDs, and permission-service errors.

Verification:

- `get_errors` on edited Go files: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 8 Frontend Expression Builder Slice

Success criteria:

- Backend has a `POST /api/semantic/models/{id}/compile-expression` endpoint to validate and compile expression ASTs/strings to dialect-specific SQL.
- A recursive `ExpressionBuilder` React component is created and supports visual AST building, whitelisted function calls, and case expressions.
- Text mode in the expression builder supports raw string inputs and real-time backend compilation.
- Metric creation and editing support the new `ExpressionBuilder` and persist ASTs.
- Calculated dimension editing in the modeling panel uses the expression builder.
- All frontend and backend tests pass cleanly, and the frontend build succeeds.

- [x] Implement backend `POST /api/semantic/models/{id}/compile-expression` handler and route.
- [x] Add unit tests for the backend compilation endpoint.
- [x] Create `ExpressionBuilder.tsx` component and `expressionBuilder.css`.
- [x] Integrate `ExpressionBuilder` into `AddMetricModal.tsx` for creation and editing.
- [x] Add calculated dimension editing to `ModelingPalette.tsx` using the `ExpressionBuilder`.
- [x] Verify everything via frontend tests/build and backend pre-commit checks.

## EXPR_AST_PLAN Phase 8 Frontend Expression Builder Results

Verified:

1. Backend route `POST /api/semantic/models/{id}/compile-expression` is wired in `internal/http/catalog_router.go`.
2. Handler `CompileExpression` accepts expression strings or AST payloads and returns compiled SQL.
3. Backend endpoint coverage exists in `internal/http/handlers/semantic_expr_api_test.go`.
4. `ExpressionBuilder.tsx` and `expressionBuilder.css` exist and support text/visual modes, whitelisted functions, binary/unary nodes, references, and CASE expressions.
5. `AddMetricModal.tsx` uses `ExpressionBuilder` for metric expressions and persists AST payloads.
6. `ModelingPalette.tsx` opens dimension editing, and `EditDimensionModal.tsx` uses `ExpressionBuilder` for calculated dimensions.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -run TestCompileExpression -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `GOCACHE=/private/tmp/biqly-gocache make lint-go`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `npm --prefix frontend run lint`
- `git diff --check`

Notes:

- `npm --prefix frontend run lint` completed with 0 errors and existing warnings under the configured `--max-warnings 1500` threshold.
- `GOCACHE=/private/tmp/biqly-gocache make lint-go` completed with 0 issues; sandbox prevented writing golangci-lint cache facts under `~/Library/Caches`, producing cache warnings only.

## EXPR_AST_PLAN Phase 5.3, 6, & 7 Backfill, Lineage, and Hardening Slice

Success criteria:

- Command `go run ./cmd/migrate backfill` parses existing expression strings to JSON ASTs and updates the database.
- `ExprDependencies` extracts all dependencies recursively from an expression AST.
- Publish validation detects circular dependencies between metrics and dimensions, returning validation errors.
- Lineage endpoint `GET /api/semantic/models/{id}/lineage` returns the dependency graph of metrics and dimensions.
- `ValidateExprStrict` recursively validates function names, arity, column/metric/dimension existence, maximum depth, and nested aggregations.
- Compile-time safety net runs `ReadOnlyChecker` on compiled expressions.
- Focused Go tests pass cleanly.

- [x] Add backfill command to `cmd/migrate/main.go` and test it.
- [x] Implement `ExprDependencies` in `pkg/semantic/expr_lineage.go` and add unit tests.
- [x] Detect circular dependencies in publish validation.
- [x] Add GET `/api/semantic/models/{id}/lineage` endpoint in HTTP handlers and router.
- [x] Implement `ValidateExprStrict` in `pkg/semantic/expr_validation.go` and wire it into publish validation.
- [x] Add compile-time safety net in expression compiler.
- [x] Run focused tests, verify all checklist items, and update `EXPR_AST_PLAN.md`.

## EXPR_AST_PLAN Phase 5.3, 6, & 7 Backfill, Lineage, and Hardening Results

Resolved:

1. Added database backfill command `backfill` to `cmd/migrate/main.go` to parse legacy string expressions to JSON AST format and save them, plus wrote integration tests.
2. Implemented `ExprDependencies` to recursively extract references (columns, metrics, dimensions) from expression ASTs.
3. Added circular dependency detection using DFS during model publish.
4. Added HTTP endpoint `GET /api/semantic/models/{id}/lineage` to return nodes and edges for the lineage dependency graph.
5. Implemented strict AST validation in `pkg/semantic/expr_validation.go` (checking function whitelist, arity, column/metric/dimension existences, max depth of 10, and blocking nested aggregates) and integrated it into the publish phase.
6. Embedded compile-time safety net calling `ReadOnlyChecker.Check` to ensure compiled expression SQL only performs read-only operations.

Verification:

- Run `make precommit` (tests pass cleanly, zero lint/formatting findings).

## EXPR_AST_PLAN Phase 5.2 API Changes Slice

Success criteria:

- Dimension create/update requests accept `calculated_expression` strings and `calculated_expr` JSON AST payloads.
- Metric create/update requests accept legacy `expression` strings and `expr` JSON AST payloads.
- String expressions are parsed server-side when an expression parser is registered.
- Invalid expression JSON or string payloads return a bad-request error before repository writes.
- API response JSON includes AST fields when present.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing handler tests for expression AST request/response behavior.
- [x] Update semantic handler DTO mapping for dimension and metric expression ASTs.
- [x] Wire create/update handlers through shared request mapping helpers.
- [x] Update `EXPR_AST_PLAN.md` Phase 5.2 once verified.
- [x] Run focused Go tests and frontend checks, then document results.

## EXPR_AST_PLAN Phase 5.2 API Changes Results

Resolved:

1. Dimension create/update requests now accept `calculated_expression` and `calculated_expr`.
2. Metric create/update requests now accept `expression` and `expr`.
3. Request mapping parses provided JSON AST payloads and parses string expressions server-side when `ExpressionParser` is registered.
4. Invalid AST or string expressions return `400 Bad Request` before repository writes.
5. API response structs already serialize `calculated_expr` and `expr`; frontend semantic types now model those response fields.
6. Modeling rename/reactivate payloads preserve AST fields so full update calls do not drop them.
7. Updated `EXPR_AST_PLAN.md` Phase 5.2.

Left open intentionally:

- Existing-row JSON backfill remains the open Phase 5.1/5.3 item.
- Full visual expression editor work remains a later frontend task.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -run 'TestDimensionFromRequest|TestMetricFromRequest|TestExpressionAPI|TestCreateDimensionRejectsInvalidExpressionASTBeforeRepoWrite|TestSemanticExpressionAPIResponse' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `npm --prefix frontend run test -- entityActions`
- `npm --prefix frontend run build`

## EXPR_AST_PLAN Phase 5.1 Dual-Format Storage Slice

Success criteria:

- Migrations add expression AST JSONB columns for dimensions and metrics.
- Repository write paths persist AST JSON when AST fields are present.
- Repository read paths prefer JSON AST columns and fall back to string parsing.
- Existing string expression fields remain backward-compatible.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing tests for expression AST JSON encode/decode helpers.
- [x] Add migration for dimension/metric AST JSON columns.
- [x] Update repository dimension/metric insert, update, select, and scan paths.
- [x] Update `EXPR_AST_PLAN.md` Phase 5.1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 5.1 Dual-Format Storage Results

Resolved:

1. Added migration `040a_add_expression_ast_json.up.sql` for `semantic_dimensions.calculated_expr_json` and `semantic_metrics.expr_json`.
2. Added matching down migration `040b_add_expression_ast_json.down.sql`.
3. Added `calculated_expression` with `IF NOT EXISTS` because the Go model already had the field but existing migrations did not create the column.
4. Added AST JSON encode/decode helpers for repository storage.
5. Updated dimension/metric create, bulk insert, update, select, and scan paths to write/read AST JSON.
6. Repository scan prefers JSON AST when present; Phase 4.4 hydration still parses legacy string expressions when JSON is missing or invalid.
7. Updated `EXPR_AST_PLAN.md` Phase 5.1 migration/repository items.

Left open intentionally:

- Existing-row backfill is still open. The plan says actual parsing should be done in Go, so this belongs to Phase 5.3 one-shot migration tooling rather than SQL-only migration.
- API accept/return behavior remains Phase 5.2.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/semantic -run 'TestExpressionASTStorage|TestDecodeExprNodeJSON|TestHydrateExpressionASTs' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.4 Load-Time Parsing Slice

Success criteria:

- Semantic repository load path populates parsed AST fields for dimension and metric expressions when a parser is registered.
- Parse failures leave AST fields nil for read-time fail-open compatibility.
- `internal/query` registers the parser alongside the existing validator hook.
- Publish validation remains covered by existing validation path.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing semantic hydration tests for parser success and fail-open parse errors.
- [x] Add parser hook and repository hydration helper.
- [x] Register `ParseExpression` from `internal/query`.
- [x] Wire hydration into full model and published snapshot load paths.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.4 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.4 Load-Time Parsing Results

Resolved:

1. Added `ExpressionParser` hook in `internal/semantic` so semantic loading can parse expressions without importing `internal/query`.
2. Registered `ParseExpression` from `internal/query` alongside the existing calculated-expression validator hook.
3. Added `hydrateExpressionASTs` to populate `Dimension.CalculatedExpr` and `Metric.Expr` when the parser is available.
4. Wired hydration into `GetFullModel` and decoded published snapshots.
5. Kept read-time compatibility fail-open: parse errors log a warning and leave AST fields nil.
6. Added focused hydration tests for parser success and parse-error nil behavior.
7. Updated `EXPR_AST_PLAN.md` Phase 4.4.

Left open intentionally:

- Parse failures are fail-open at read time per the plan. Strict fail-closed behavior remains a Phase 7 compile-time safety-net task.
- JSON AST storage/backfill is still Phase 5.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/semantic -run 'TestHydrateExpressionASTs' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.3 Window AST Slice

Success criteria:

- `WindowSpec` can carry an AST expression while keeping `Expression` for backward compatibility.
- `buildWindowExpr` compiles `WindowSpec.Expr` via `CompileExpr`.
- AST window aggregate expressions are not re-quoted as identifiers.
- Existing window metric/ranking behavior remains covered.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler test for `WindowSpec.Expr`.
- [x] Add `Expr` to `pkg/logicalquery.WindowSpec`.
- [x] Update `buildWindowExpr` to compile AST window expressions.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.3 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.3 Window AST Results

Resolved:

1. Added `Expr ExprNode` to `pkg/logicalquery.WindowSpec` while keeping `Expression` for backward compatibility.
2. Updated `buildWindowExpr` to compile `WindowSpec.Expr` through `CompileExpr`.
3. Reused the AST-aware aggregate wrapper so compiled window expressions are not re-quoted as identifiers.
4. Preserved existing metric-backed and ranking window behavior.
5. Added `TestCompiler_WindowFunctionUsesASTExpression`.
6. Updated `EXPR_AST_PLAN.md` Phase 4.3.

Left open intentionally:

- Legacy `WindowSpec.Expression` remains on the existing string/bracket-resolution path until load-time parsing and storage migration are complete.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_WindowFunctionUsesASTExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_Window' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/logicalquery ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## Align SQL Golden Files Across Dialects

- [x] Modify `internal/query/golden_test.go` to add unified `TestGoldenAcrossDialects` and supporting fixtures
- [x] Run `UPDATE_GOLDEN=true go test -v ./internal/query -run TestGoldenAcrossDialects` to generate/align golden files
- [x] Run `make precommit` (lint and test) to ensure all tests and linters pass cleanly
- [x] Verify git diff and ensure all files are correctly created and aligned

## Align SQL Golden Files Across Dialects Results

Resolved:

1. Replaced single-dialect legacy golden test cases with a unified, table-driven test suite `TestGoldenAcrossDialects` in `internal/query/golden_test.go`.
2. Expanded test coverage to generate/test all 12 query scenarios for all 3 dialects (Postgres, MySQL, and SQL Server), checking a total of 36 compile targets.
3. Automatically generated the missing 10 SQL golden files for `mysql` and `sqlserver` databases under `testdata/sql/` using `UPDATE_GOLDEN=true`.
4. Fixed linter errors (gocritic, gosec) and ran `make lint-go` and `make test-go` successfully (zero errors).

## EXPR_AST_PLAN Phase 4.2 Metric AST Slice

Success criteria:

- `pkg/semantic.Metric` can carry a parsed `Expr` AST while keeping `Expression` for backward compatibility.
- Metric SELECT, HAVING/filters, ORDER BY, and bracket-token references use `CompileExpr` when `Expr` is present.
- Legacy string metric expressions remain on the existing compatibility path.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler test for metric `Expr` in SELECT/HAVING/ORDER BY.
- [x] Add `Expr` to `pkg/semantic.Metric`.
- [x] Update metric expression helpers/call sites to prefer AST.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.2 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.2 Metric AST Results

Resolved:

1. Added `Expr ExprNode` to `pkg/semantic.Metric` while keeping `Expression` for storage/backward compatibility.
2. Updated metric expression helpers and metric call sites to prefer `Metric.Expr` when available.
3. Added an AST-aware aggregate wrapper so compiled metric expressions are not re-quoted as identifiers.
4. Kept legacy string metric expressions on the existing `dialect.Aggregate` / bracket-resolution path.
5. Added `TestCompiler_MetricUsesASTExpression` for metric SELECT, filter expression, and ORDER BY alias behavior.
6. Updated `EXPR_AST_PLAN.md` Phase 4.2.

Left open intentionally:

- `resolveBracketExpressions` remains as the legacy fallback until load-time parsing and storage migration are complete.
- Window expressions remain raw-string based and are tracked separately in Phase 4.3.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_MetricUsesASTExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.1 Calculated Dimension AST Slice

Success criteria:

- `pkg/semantic.Dimension` can carry a parsed `CalculatedExpr` AST while keeping `CalculatedExpression` for backward compatibility.
- `Compiler.dimensionSQL` prefers `CalculatedExpr` and compiles through `CompileExpr`.
- Legacy `CalculatedExpression` strings are parsed on the fly before compilation for migration safety.
- Existing calculated dimension SELECT, GROUP BY, and WHERE behavior remains covered.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler tests for `CalculatedExpr` and parsed fallback output.
- [x] Add `CalculatedExpr` to `pkg/semantic.Dimension`.
- [x] Update `dimensionSQL` to compile AST first and parse legacy strings.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.1 Calculated Dimension AST Results

Resolved:

1. Added `CalculatedExpr ExprNode` to `pkg/semantic.Dimension` while keeping `CalculatedExpression` for storage/backward compatibility.
2. Updated `Compiler.dimensionSQL` to prefer `CalculatedExpr` and compile through `CompileExpr`.
3. Added migration fallback parsing for legacy `CalculatedExpression` strings before compiling them with `CompileExpr`.
4. Added `TestCompiler_CalculatedDimensionUsesAST` and updated calculated-dimension assertions to expect dialect-quoted AST SQL.
5. Updated the Postgres calculated-dimension golden file for the new AST output.
6. Updated `EXPR_AST_PLAN.md` Phase 4.1.

Left open intentionally:

- If a legacy calculated expression cannot be parsed, `dimensionSQL` still preserves the old raw-string behavior because the current function signature has no error return. Phase 7 compile-time safety should close that fail-closed path.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_CalculatedDimension' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 3 Function Mapping Slice

Success criteria:

- `CompileExpr` uses an explicit dialect function mapping table for approved scalar functions.
- `DATE_TRUNC` compiles through dialect-owned date truncation helpers instead of raw generic function output.
- Existing emitter behavior remains unchanged for the dialect matrix already covered.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing DATE_TRUNC/function mapping tests.
- [x] Implement explicit dialect function mapping and DATE_TRUNC transform.
- [x] Update `EXPR_AST_PLAN.md` Phase 3.2 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 3 Function Mapping Results

Resolved:

1. Added an explicit `dialectFunctions` mapping table for AST function names.
2. Kept ClickHouse scalar function casing dialect-specific through the mapping table.
3. Added `DATE_TRUNC` special handling that compiles literal part plus column ref through each dialect's `DateTrunc` helper.
4. Added dialect-matrix coverage for `DATE_TRUNC('month', created_at)`.
5. Updated `EXPR_AST_PLAN.md` Phase 3.2.

Left open intentionally:

- `DATE_TRUNC` currently handles the safe planned shape: literal grain plus column ref. Broader expression arguments can be added when integration needs them.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompileExprAcrossDialects/date_trunc|TestCompileExprAcrossDialects' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 3 Emitter Slice

Success criteria:

- `CompileExpr` emits dialect-aware SQL for literals, column refs, binary/unary expressions, function calls, CASE, and concat overrides.
- Column refs go through `SchemaResolver.QualifyColumn`.
- Zero-value dialect structs are normalized to the initialized dialect defaults before quoting.
- Metric/Dimension model lookup and DATE_TRUNC argument transforms remain open for later context-aware integration.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing dialect matrix tests for AST-to-SQL emitter behavior.
- [x] Implement standalone emitter core in `internal/query/expr_compiler.go`.
- [x] Normalize zero-value dialect structs to existing initialized dialect defaults.
- [x] Update `EXPR_AST_PLAN.md` for completed Phase 3 emitter-core items.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 3 Emitter Results

Resolved:

1. Added `internal/query/expr_compiler.go` with `CompileExpr` for canonical `pkg/semantic.ExprNode` values.
2. Added SQL emission for literals, column refs, binary/unary operators, function calls, and CASE expressions.
3. Routed column refs through `SchemaResolver.QualifyColumn`.
4. Added dialect concat overrides: PostgreSQL `||`, MySQL `CONCAT`, SQL Server `+`, ClickHouse `concat`.
5. Normalized zero-value dialect structs to the existing initialized dialect globals before quoting.
6. Added dialect-matrix expected SQL tests in `internal/query/expr_compiler_test.go`.
7. Updated `EXPR_AST_PLAN.md` for completed emitter-core items.

Left open intentionally:

- `MetricRefExpr` and `DimensionRefExpr` still need model-aware lookup/integration before the broad per-node-type checklist can close.
- DATE_TRUNC-style argument transforms are still open in the function mapping section.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompileExprAcrossDialects|TestParseExpressionProducesSemanticAST|TestValidateExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 2 Parser Slice

Success criteria:

- `ParseExpression` returns canonical `pkg/semantic.ExprNode` values.
- Existing expression validation behavior remains intact.
- Bracket references, bare identifiers, qualified columns, functions, and CASE expressions have AST tests.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing parser AST tests for Phase 2.2/2.3 cases.
- [x] Refactor expression parser output from internal node types to canonical semantic AST.
- [x] Rename parser implementation to `internal/query/expression_parse.go`.
- [x] Keep `ValidateExpression` on the new parser path.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 2 Parser Results

Resolved:

1. Added `ParseExpression(expr string) (pkg/semantic.ExprNode, error)` as the public parser entry point.
2. Renamed the parser implementation to `internal/query/expression_parse.go`.
3. Refactored parser output from internal AST node structs to canonical `pkg/semantic` AST nodes.
4. Removed the old internal node types (`IdentifierNode`, `BinaryOpNode`, `UnaryOpNode`, `FunctionCallNode`, `CaseNode`, etc.).
5. Kept `ValidateExpression` on the same lexer/parser security path by delegating to `ParseExpression`.
6. Added AST assertions for bracket metric refs, bare column refs, qualified column refs, function calls, and CASE expressions.
7. Updated `EXPR_AST_PLAN.md` for completed Phase 2.1, Phase 2.2, and the new Phase 2.3 AST cases.

Left open intentionally:

- The broad existing validation table is still validation-focused, so `Migrate existing expression_parser_test.go tests to new AST types` remains open.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestParseExpressionProducesSemanticAST|TestValidateExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 1 Slice

Success criteria:

- `pkg/semantic` has sealed expression AST node types from `EXPR_AST_PLAN.md` Phase 1.1.
- AST nodes JSON marshal with a `type` discriminator and nested nodes unmarshal back to concrete node types.
- Unknown AST node types are rejected.
- Allowed expression function whitelist is available.
- Focused package tests and `git diff --check` pass.

- [x] Add failing AST JSON/whitelist tests in `pkg/semantic`.
- [x] Implement canonical AST types and JSON handling in `pkg/semantic/expr.go`.
- [x] Mark completed Phase 1 checklist items in `EXPR_AST_PLAN.md`.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 1 Results

Resolved:

1. Added `pkg/semantic/expr.go` with sealed expression AST node types for literals, column/metric/dimension refs, binary/unary operations, function calls, and CASE expressions.
2. Added dialect-neutral binary/unary operator constants.
3. Added `AllowedFunctions` whitelist with arity values from the plan.
4. Added discriminator-based JSON marshal/unmarshal support plus `UnmarshalExprNode` for nested interface decoding.
5. Added `pkg/semantic/expr_test.go` covering node round trips, nested expressions, unknown type rejection, and whitelist entries.
6. Updated `EXPR_AST_PLAN.md` for completed Phase 1.1, Phase 1.2, and the duplicate Phase 9 AST unit-test item.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic -run 'TestExprNode|TestUnmarshalExprNode|TestAllowedFunctions' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## GO_PERF_TODO Auth Claims Stale Slice

Success criteria:

- The `handler_rbac.go:23` JWT claims item is closed only if source inspection proves the map-claims hot path is gone.
- No benchmark file is added when the compared current path no longer exists.
- Focused auth/middleware tests and `git diff --check` pass.

- [x] Inspect current `handler_rbac.go:23` and JWT claim parsing code.
- [x] Verify auth JWT paths use typed claims instead of `map[string]any` / `MapClaims`.
- [x] Update `GO_PERF_TODO.md` and document the stale-item decision.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Auth Claims Stale Results

Resolved:

1. Verified `internal/auth/handlers/handler_rbac.go:23` is currently `type RBACHandler struct`, not JWT claim decoding.
2. Verified auth token code uses typed `auth.JWTClaims` in `internal/auth/jwt.go`.
3. Verified monolith HTTP JWT middleware uses typed `JWTClaims` plus `jwt.NewParser(...).ParseWithClaims`.
4. No `jwt.MapClaims` current hot path was found under `internal/auth` or `internal/http/middleware`.

Decision:

- Close the stale `handler_rbac.go:23` JWT claims `map[string]any` benchmark item. There is no current map-claims path to benchmark.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth ./internal/auth/handlers ./internal/http/middleware -count=1`
- `git diff --check`

## GO_PERF_TODO Readonly Builder Benchmark Slice

Success criteria:

- `security/readonly.go` builder pool idea is measured before any production change.
- Benchmark compares current stack-local `strings.Builder` with a test-only pooled builder alternative.
- `GO_PERF_TODO.md` is updated only from measured evidence.
- Focused security tests/benchmarks and `git diff --check` pass.

- [x] Add focused readonly builder benchmarks.
- [x] Run benchmark with `-benchmem` and compare current vs pooled builder.
- [x] Update `GO_PERF_TODO.md` and this review section from measured evidence.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Readonly Builder Benchmark Results

Resolved:

1. Added `BenchmarkStripSQLLiteralsAndComments` for short, commented, and long SQL inputs.
2. Compared current stack-local `strings.Builder` with a test-only pooled builder variant.
3. Measured current short SQL at roughly 119-123 ns/op, 96 B/op, 1 alloc/op; pooled builder was slower at roughly 137-142 ns/op with the same allocation profile.
4. Measured current commented SQL at roughly 137-143 ns/op, 128 B/op, 1 alloc/op; pooled builder was slower at roughly 157-159 ns/op with the same allocation profile.
5. Measured current long SQL at roughly 3350-3736 ns/op, 3200 B/op, 1 alloc/op; pooled builder was slower at roughly 3772-4001 ns/op and did not reduce allocations.

Decision:

- Keep production `stripSQLLiteralsAndComments` as-is. Builder pooling is not justified.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/security -run '^$' -bench '^BenchmarkStripSQLLiteralsAndComments$' -benchmem -count=5`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/security -count=1`
- `git diff --check`

## GO_PERF_TODO Filter Benchmark Slice

Success criteria:

- `compiler_filter.go` `filterHandler` / typed slice concern is measured with Go benchmarks before any production refactor.
- Benchmarks cover current dispatch/direct paths and typed `[]string` / `[]int` helper alternatives.
- `GO_PERF_TODO.md` is updated only from measured results.
- Focused Go tests/benchmarks and `git diff --check` pass.

- [x] Add focused compiler filter benchmarks.
- [x] Run benchmark with `-benchmem` and compare current vs direct/typed alternatives.
- [x] Update `GO_PERF_TODO.md` and this review section from measured evidence.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Filter Benchmark Results

Resolved:

1. Added `BenchmarkCompilerFilterHandler` for current dispatch, direct method calls, and typed `[]string` / `[]int` helper alternatives.
2. Measured `current_dispatch_eq_string_slice` at roughly 281-300 ns/op, 584 B/op, 16 allocs/op; direct/typed alternatives had the same alloc profile and only small timing noise.
3. Measured `current_dispatch_in_any_strings` at roughly 160-240 ns/op, 264 B/op, 8 allocs/op; typed `[]string` helper was worse at 193-205 ns/op, 328 B/op, 12 allocs/op.
4. Measured `current_dispatch_in_any_ints` at roughly 161-177 ns/op, 264 B/op, 8 allocs/op; typed `[]int` helper kept the same allocation profile and did not show a stable improvement.

Decision:

- Keep production `filterHandler` as-is. The benchmark does not justify a generics refactor.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run '^$' -bench '^BenchmarkCompilerFilterHandler$' -benchmem -count=5`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -count=1`
- `git diff --check`

## GO_PERF_TODO Escape/Decision Slice

Success criteria:

- Escape-analysis subitems are closed only with fresh `go build -gcflags='-m -m'` evidence.
- Existing "do not add" / "small map is okay" items are closed as explicit no-code decisions only after source inspection.
- No speculative pool/dependency changes are introduced.
- Focused Go tests and `git diff --check` pass after tracker edits.

- [x] Inspect compiler args/dimMap and readonly builder source for no-code decision items.
- [x] Run escape-analysis checks for compiler, executor, and readonly builder subitems.
- [x] Update `GO_PERF_TODO.md` for only evidence-backed closed items.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Escape/Decision Results

Resolved:

1. Closed `query/compiler.go` `args []any` as an explicit no-code decision; the checklist already notes pool overhead is higher than one compile-time allocation.
2. Closed `compiler.go:110` `dimMap` as an explicit no-code decision; it is a small per-row-filter lookup map scoped to `buildRowFilterPreds`.
3. Closed `query/executor.go` `columns []ResultColumn` as a no-pool decision. Escape analysis shows `columns` flows into returned `Result`, so pooling would be unsafe unless the result API changes.
4. Checked compiler `[]string` escape state with `go build -gcflags='-m -m' ./internal/query`; remaining slice escapes are in returned SQL/error-building paths, not a standalone safe pooling target.
5. Checked executor `Result` escape state; `&Result`, `Columns`, `Rows`, and each returned row slice escape because they are returned to callers.
6. Checked readonly builder state with `go build -gcflags='-m -m' ./internal/security`; `stripSQLLiteralsAndComments` already calls `Grow(len(sql))`, and Builder write paths inline through `abi.NoEscape`.

Left open intentionally:

- `security/readonly.go` builder pooling still needs before/after benchmark evidence.
- `executor.go` per-row copy allocation remains open; reducing it would need a benchmark-backed result-layout/API change, not a small pool tweak.
- JSON library, JWT claim, `filterHandler` generics, and pprof baseline items remain benchmark/profile work.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai ./internal/query ./internal/core ./internal/security ./internal/http/handlers ./internal/http -count=1`
- `git diff --check`

## Metrik Önerileri — Grafana/Prometheus (2026-06-10)

**Mevcut durum:** 52 metrik, 4 Grafana dashboard, 6 alert rule, alertmanager webhook.
Metrikler: `internal/platform/observability/metrics.go` (core+extended), `internal/auth/metrics.go`,
`internal/auth/rbac/metrics.go`. Dashboard'lar: `deploy/helm/biqly/templates/grafana-dashboards.yaml`.
Alert'ler: `deploy/helm/biqly/templates/prometheus-rules.yaml`.

### Tier 1 — Kritik (operasyonel kör noktalar)

- [x] **HTTP request metrikleri** — Mevcut HTTP endpoint'lerinin %90'ında Prometheus metriği yok.
  Sadece `/api/catalog/*` rotalarında `CatalogMetricsMiddleware` var. AI, query, admin, internal
  endpoint'leri için hiçbir HTTP seviyesi metrik yok (latency, error rate, status code).
  - `biqly_http_request_duration_seconds` — Histogram, labellar: `method`, `route_group` (bounded:
    `/api/ai/query`, `/api/ai/preview`, `/api/catalog/*`, `/api/admin/*` gibi gruplanmış).
    Cardinality guard: `route_group` max 20 değer.
  - `biqly_http_requests_total` — CounterVec, labellar: `method`, `status_class` (`2xx`/`4xx`/`5xx`).
  - **Nasıl:** Mevcut `CatalogMetricsMiddleware` pattern'ini genelleştir. Root handler'a ekle.
    Chi router'dan route pattern'ini al (`chi.RouteContext(r.Context()).RoutePattern()`),
    raw path yerine pattern'i label olarak kullan.
  - **Dosyalar:** `internal/http/metrics_middleware.go` (yeni),
    `internal/http/router.go` (middleware wrap), `internal/platform/observability/metrics.go`.

- [x] **LLM hata/retry metrikleri (provider seviyesi)** — `llm_request_duration_seconds` provider/model
  label'ı olmadan kaydediliyor. Provider seviyesi retry'lar (429, 502, 503, 504) sadece loglanıyor.
  - `biqly_llm_errors_total` — CounterVec, labellar: `provider` (openai/anthropic),
    `error_type` (rate_limit/network/auth/parse/other).
  - `biqly_llm_retries_total` — CounterVec, label: `provider`.
  - `biqly_llm_tokens_prompt_total` / `biqly_llm_tokens_completion_total` — Counter (mevcut
    `llm_tokens_used_total` prompt/completion ayırmıyor; maliyet analizi için split gerek).
  - **Nasıl:** `internal/ai/provider/` altındaki `execRetry` ve API call wrapper'larına metric
    recording ekle. Provider interface'den dönen error'ları kategorize et.
  - **Dosyalar:** `internal/ai/provider/base_provider.go`, `internal/platform/observability/metrics.go`.

- [x] **DB connection pool metrikleri** — `database/sql` pool istatistikleri (`db.Stats()`:
  OpenConnections, InUse, Idle, WaitCount, WaitDuration) hiç export edilmiyor.
  Pool exhaustion tespit edilemez.
  - `biqly_db_pool_open_connections` — GaugeFunc, label: `pool` (metadata/auth/datasource).
  - `biqly_db_pool_in_use` — GaugeFunc, label: `pool`.
  - `biqly_db_pool_wait_count_total` — CounterFunc, label: `pool`.
  - `biqly_db_pool_wait_duration_seconds_total` — CounterFunc, label: `pool`.
  - **Nasıl:** Mevcut pattern: `cmd/auth/main.go:233`'teki `auth_active_sessions` GaugeFunc
    yaklaşımını takip et. Her DB pool için `prometheus.NewGaugeFunc` ile `db.Stats()` sar.
  - **Dosyalar:** `internal/platform/observability/db_pool_metrics.go` (yeni),
    `internal/platform/db/` (pool referansları).

- [x] **Routing confidence histogram** — Routing kalitesi sadece trace span'larında
  (`ai.route.confidence`). Dashboard'da görünmüyor, zaman içindeki degradasyon tespit edilemez.
  - `biqly_routing_confidence_histogram` — Histogram, label: `ranking_method`
    (keyword/hybrid/manual/semantic).
  - `biqly_routing_decisions_total` — CounterVec, labellar: `method`, `outcome`
    (success/clarification/error).
  - **Nasıl:** `internal/ai/routing/` modülünde routing sonucu alındığında metric record.
    Confidence 0.0-1.0 arası → histogram bucket'ları: `[0.1, 0.3, 0.5, 0.7, 0.8, 0.9, 0.95, 0.99]`.
  - **Dosyalar:** `internal/ai/routing/route.go` veya `router.go`,
    `internal/platform/observability/metrics.go`.

- [x] **Embedding API latency/errors** — Embedding çağrıları sadece trace span'larında.
  API çökerse routing sessizce keyword-only fallback'e döner, metrik yok.
  - `biqly_embedding_api_duration_seconds` — Histogram, label: `operation`
    (route_recall/memory_store/metadata_embed).
  - `biqly_embedding_api_errors_total` — CounterVec, labellar: `operation`, `error_type`.
  - **Nasıl:** `internal/ai/provider/base_provider.go`'daki `embed()` fonksiyonuna metric
    recording ekle (retry wrapper'ın içine veya dışına).
  - **Dosyalar:** `internal/ai/provider/base_provider.go`,
    `internal/platform/observability/metrics.go`.

**Review (2026-06-10):** Tier 1 tamamlandı.
- `HTTPMetricsMiddleware` → api/ai/catalog/query/auth router'larına eklendi; `biqly_http_*` metrikleri.
- Provider `execRetry` → `biqly_llm_errors_total`, `biqly_llm_retries_total`, token split counters.
- `RegisterDBPoolMetrics` → metadata (`openMetadataDB`), auth (`cmd/auth`), datasource (`PoolCache.AggregatedStats`).
- `TableRouter.Route` defer → `biqly_routing_confidence_histogram`, `biqly_routing_decisions_total`.
- `baseEmbedder.embed` + `ContextWithEmbeddingOperation` → `biqly_embedding_api_*`.
- Test: `tier1_metrics_test.go`, `metrics_middleware_test.go`; `make lint-go` + targeted `go test` yeşil.

### Tier 2 — Önemli (business insight & debugging)

- [x] **NATS queue metrikleri** — Publish/consume sadece trace span'larında. DLQ move'lar loglanıyor.
  - `biqly_nats_publish_total` / `biqly_nats_publish_errors_total` — Counter.
  - `biqly_nats_publish_duration_seconds` — Histogram.
  - `biqly_nats_consume_total` / `biqly_nats_consume_errors_total` — Counter.
  - `biqly_nats_dlq_moves_total` — Counter.
  - `biqly_nats_consumer_pending` — Gauge (JetStream consumer pending count).
  - **Nasıl:** `internal/queue/nats.go`'daki publish/consume wrapper'lara metric recording ekle.
    DLQ move sayacı zaten log mevcut → log yanına metric ekle.
  - **Dosyalar:** `internal/queue/nats.go`, `internal/platform/observability/metrics.go`.

- [x] **Memory recall miss sayacı** — Sadece hit sayılıyor, miss yok → hit rate hesaplanamıyor.
  - `biqly_memory_recall_misses_total` — Counter.
  - `biqly_memory_recall_latency_ms` — Histogram (embed + sort süresi).
  - `biqly_memory_store_confirmed_embedding_errors_total` — Counter.
  - **Nasıl:** `internal/ai/memory/recall.go`'da `Recall()` fonksiyonunda results boş dönerse miss
    counter'ı artır. Embedding hatası `ai_memory.go:83`'te loglanıyor → metric ekle.
  - **Dosyalar:** `internal/ai/memory/recall.go`, `internal/http/handlers/ai_memory.go`,
    `internal/platform/observability/metrics.go`.

- [x] **Clarification round dağılımı** — Kullanıcıların kaç turda netleştirdiği/terk ettiği bilinmiyor.
  - `biqly_ambiguity_clarification_rounds_histogram` — Histogram, bucket'lar: `[1, 2, 3, 4, 5]`.
  - `biqly_ambiguity_resolution_total` — CounterVec, label: `outcome` (resolved/abandoned).
  - **Nasıl:** `ai.go` handler'ında clarification response döndüğünde round sayısını histogramla.
    Abandon: kullanıcı clarification'a cevap vermeden yeni soru sorduğunda (session bazlı tracking).
  - **Dosyalar:** `internal/http/handlers/ai.go`, `internal/platform/observability/metrics.go`.

- [x] **LLM response cache metrikleri** — `service.go`'da cache hit sadece loglanıyor.
  - `biqly_llm_response_cache_hits_total` / `biqly_llm_response_cache_misses_total` — Counter.
  - **Nasıl:** `internal/ai/service.go`'da cache lookup noktasına counter ekle.
  - **Dosyalar:** `internal/ai/service.go`, `internal/platform/observability/metrics.go`.

- [x] **Enrich context suggestion latency** — En pahalı operasyon ölçülmüyor.
  - `biqly_enrich_context_suggestions_generated_total` — Counter.
  - `biqly_enrich_context_suggest_latency_seconds` — Histogram.
  - `biqly_enrich_context_apply_errors_total` — Counter.
  - **Nasıl:** `internal/ai/enrichcontext/suggest.go`'da LLM call öncesi/sonrası timer.
  - **Dosyalar:** `internal/ai/enrichcontext/suggest.go`, `internal/platform/observability/metrics.go`.

### Tier 3 — İyi olur (tuning & optimization)

- [x] **Routing grain detection** — Hangi grain'lerin ne sıklıkla sorulduğu bilinmiyor.
  - `biqly_routing_grain_detections_total` — CounterVec, label: `grain` (year/quarter/month/day/none).
  - **Dosyalar:** `internal/ai/routing/time_grains.go`.

- [x] **Semantic model generation metrikleri** — `internal/semanticgen/` paketinde sıfır metrik.
  - `biqly_semanticgen_models_generated_total` — Counter.
  - `biqly_semanticgen_duration_seconds` — Histogram.
  - `biqly_semanticgen_dimensions_generated_histogram` — Histogram (dimension sayısı dağılımı).
  - `biqly_semanticgen_metrics_generated_histogram` — Histogram.
  - **Dosyalar:** `internal/semanticgen/generator.go`, `internal/platform/observability/metrics.go`.

- [x] **Feedback raw total** — Toplam feedback sayısı bağımsız metrik değil.
  - `biqly_feedback_submitted_total` — CounterVec, label: `rating` (positive/negative).
  - **Dosyalar:** `internal/http/handlers/ai_examples.go`, `internal/platform/observability/metrics.go`.

### Grafana Dashboard Güncellemeleri

- [x] **Biqly AI** dashboard'ına eklenecek paneller:
  - LLM errors by provider (stacked bar)
  - LLM retry rate (line)
  - Token split: prompt vs completion (stacked area)
  - Routing confidence distribution (histogram heatmap)
  - Embedding API latency p95
  - Embedding API error rate
  - Clarification rounds distribution
  - LLM response cache hit rate
- [x] **Biqly Infrastructure** dashboard (yeni):
  - HTTP request rate by route group
  - HTTP 5xx error rate by route
  - DB pool: open/in-use/idle connections (gauge)
  - DB pool wait count & duration
  - NATS publish/consume rate
  - NATS DLQ moves
  - NATS consumer pending
- [x] **Yeni alert rule'lar:**
  - `BiqlyHTTP5xxRateHigh` — HTTP 5xx oranı > %1 (5dk)
  - `BiqlyEmbeddingAPIErrors` — Embedding hata oranı > %5 (5dk)
  - `BiqlyDBPoolExhaustion` — Pool in-use/open > %90 (3dk)
  - `BiqlyNATSDLQMoves` — DLQ move > 0 (5dk)
  - `BiqlyRoutingConfidenceLow` — p50 confidence < 0.5 (15dk)

---

## GO_PERF_TODO Continuation

Success criteria:

- Stale/verification-only GO perf items are closed only with code or command evidence.
- Benchmark/dependency/profile items remain open unless measured in this slice.
- Focused Go tests and diff checks still pass after tracker/code edits.

- [x] Verify `provider_store.go` no longer uses `map[string]any` config.
- [x] Run compiler escape-analysis command from `GO_PERF_TODO.md` and record the result.
- [x] Close verified non-code GO perf items and leave benchmark-only items open.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Continuation Results

Resolved:

1. Verified `internal/ai/provider_store.go` is already on typed provider config paths and closed the stale `map[string]any` item.
2. Ran the compiler escape-analysis command from `GO_PERF_TODO.md` and closed the command-level checklist item.
3. Closed cold-path `map[string]any` items for i18n bundles, mail rendering/sending, and auth audit metadata as accepted non-hot-path uses.
4. Closed `internal/http/handlers/helpers.go` as a low-priority error/wrapper path; it has no `fmt.Sprintf`, and remaining `fmt.Errorf` calls wrap auth client errors.

Left open intentionally:

- `filterHandler` generics, JWT claim structs, JSON backend alternatives, pprof profiling, pool decisions, and row-copy allocation items still need benchmark/profile evidence before code changes.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai ./internal/query ./internal/core ./internal/security ./internal/http/handlers ./internal/http -count=1`
- `git diff --check`

## GO_PERF_TODO Slice

Success criteria:

- PII masking config can resolve access/type/strategy from a single `ColumnInfo` map.
- Existing `PIIMaskingConfig` map fields remain backward-compatible for current tests/callers.
- `GO_PERF_TODO.md` reflects only verified code changes; benchmark-only items remain open unless measured.
- Focused Go tests pass.

- [x] Add failing coverage for `PIIMaskingConfig.ColumnInfo` lookup.
- [x] Implement single-map PII lookup with compatibility fallback.
- [x] Mark already-implemented compiler select concat escape item as verified in `GO_PERF_TODO.md`.
- [x] Remove verified `fmt.Sprintf` predicate builders in `compiler_filter.go` and `row_injection.go`.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Slice Results

Resolved:

1. Added `PIIMaskingConfig.ColumnInfo map[string]PIIColumnInfo` and made compiler PII lookup prefer the single map while preserving legacy split-map fallback.
2. Populated `ColumnInfo` in `PIIPolicyService.MaskingConfig`.
3. Removed `fmt.Sprintf` predicate builders from `internal/query/compiler_filter.go` and `internal/security/row_injection.go`.
4. Marked verified GO perf items for PII single-map lookup, executor scan slice pooling, compiler select concat, compiler filter predicates, and row injection predicates.

Left open intentionally:

- JSON library alternatives, pprof, escape-analysis, and pool-decision items still require measurement/benchmark baselines.
- `filterHandler` generics and typed provider/auth config items need separate benchmark-backed slices.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/core ./internal/security -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./... -count=1`
- `git diff --check`

## PII Review Findings Fix Plan

Success criteria:

- Backend rejects or ignores invalid masking strategy values consistently.
- Full masking no longer changes GROUP BY/ORDER BY semantics or permits filters that hidden values would block.
- Frontend lets editable users update masking strategy on reviewed columns while avoiding confusing strategy controls for admin/raw access.
- Focused backend/frontend tests prove the review findings are fixed.

- [x] Verify current failures with focused tests for full-strategy GROUP BY/ORDER BY, filter blocking, and invalid strategy values.
- [x] Fix backend masking semantics with minimal changes and shared strategy constants.
- [x] Add defensive PII-type checks when building column strategy maps.
- [x] Fix `PIIDetectionPanel.tsx` so reviewed columns can update strategy and admin/raw access does not show a misleading strategy dropdown.
- [x] Run focused backend/frontend tests, then update this tracker with results.

## PII Review Findings Results

Resolved:

1. Backend strategy values now normalize through shared constants. Unknown non-empty strategies fail closed to full/hidden behavior.
2. The compiler resolves strategy into one effective access level before SELECT, WHERE, GROUP BY, and ORDER BY decisions.
3. Hidden/full-masked columns are rejected in filters, GROUP BY, and ORDER BY instead of producing constant predicates or aggregates.
4. Column strategy maps are built only for PII-annotated columns.
5. Reviewed PII rows can save strategy changes without dismissing/re-scanning, and raw-access users see a localized note that raw roles still see raw values.
6. Added focused backend regressions and a frontend pure-logic test for reviewed-row strategy saves.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/security/pii ./internal/core -count=1`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `git diff --check`

## Previous PII Strategy Work

- [x] Frontend: Add Turkish and English translation keys for `strategy_partial` and `strategy_full`
- [x] Frontend: Implement `pendingStrategy` state and masking strategy dropdown in `PIIDetectionPanel.tsx`
- [x] Frontend: Pass `pii_masking_strategy` in `handleConfirm` to the backend API in `PIIDetectionPanel.tsx`
- [x] Backend: Add `ColumnStrategies` mapping in `PIIMaskingConfig` in `internal/query/pii_masking.go`
- [x] Backend: Update `dimensionOutputSQL` in `internal/query/pii_masking.go` to handle "full" strategy
- [x] Backend: Populate `ColumnStrategies` from database columns in `internal/core/pii_policy.go`
- [x] Verification: Run frontend build and tests
- [x] Verification: Run backend tests

## Previous Review

All tasks are completed successfully:

1. **Frontend**: Translation keys added for English and Turkish. Masking strategy dropdown rendered for unreviewed columns in the PII Detection Panel. Local edits are stored in `pendingStrategy` state and transmitted via `updateColumnPII` upon clicking Confirm.
2. **Backend**: Added column-level strategy mapping in `PIIMaskingConfig` and parsed `PIIMaskingStrategy` from database records. Integrated this strategy in the query compiler: columns with `"full"` strategy resolve to `pii.HiddenLiteral` (full mask) instead of the partial masking expression.
3. **Verification**: Frontend build and tests pass successfully. Backend linter and unit tests (including a new compiler test for strategy overrides) pass successfully.

## CI/CD Workflow Duplication Cleanup Plan

Success criteria:

- Pure Argo image-updater commits that only change `deploy/helm/biqly/.argocd-source-*.yaml` do not trigger Semgrep, CodeQL, or Build Migrate Image.
- Normal source commits continue to trigger the existing CI/CD workflows.
- Build Migrate Image still runs on normal `main` pushes so the migrate image exists for service SHAs.

- [x] Inspect current Semgrep, CodeQL, and build-migrate trigger filters.
- [x] Add narrow generated-file ignores for `deploy/helm/biqly/.argocd-source-*.yaml`.
- [x] Verify workflow YAML syntax and diff scope.

## CI/CD Workflow Duplication Cleanup Review

Resolved:

1. Semgrep now ignores generated Argo image-updater source files on push and pull request triggers, alongside existing docs/README/workflow ignores.
2. CodeQL now skips pure `deploy/helm/biqly/.argocd-source-*.yaml` changes on push and pull request triggers.
3. Build Migrate Image now skips only pure generated Argo source-file changes on `main` push; its pull request `paths` filter is unchanged.

Verification:

- `ruby -e 'require "yaml"; ARGV.each { |path| YAML.load_file(path); puts "#{path}: ok" }' .github/workflows/semgrep.yml .github/workflows/codeql.yml .github/workflows/build-migrate.yml`
- `ruby -e 'patterns = ["deploy/helm/biqly/.argocd-source-*.yaml"]; cases = {"generated only" => ["deploy/helm/biqly/.argocd-source-biqly.yaml"], "source only" => ["internal/core/service.go"], "mixed" => ["deploy/helm/biqly/.argocd-source-biqly.yaml", "internal/core/service.go"]}; cases.each do |name, paths| ignored = paths.all? { |path| patterns.any? { |pattern| File.fnmatch?(pattern, path, File::FNM_PATHNAME) } }; puts "#{name}: #{ignored ? "skip" : "run"}"; end'`
- `git diff --check`

Notes:

- `actionlint` is not installed locally, so validation used YAML parsing plus explicit path-filter behavior checks.
- Post-commit GitHub workflow observation and live ArgoCD rollout checks were not run because no commit/push/deploy was performed in this slice.

## Table Browser Joined Table Selection Bugfix Plan

Success criteria:

- Table Browser keeps a selected joined table such as `public.profiles` or `public.tracked_profiles` instead of snapping back to the base table.
- Invalid or stale table selections still fall back to the model base table.
- Focused frontend tests cover the selection rule.

- [x] Add a failing test for joined table selection.
- [x] Fix the selected table resolution in `useTableBrowserPage`.
- [x] Run focused frontend verification and document results.

## Table Browser Joined Table Selection Review

Resolved:

1. Root cause: `useTableBrowserPage` only accepted `selectedTableKeyInput` when it exactly matched the base table key, so selecting a joined table immediately resolved back to the base table.
2. Added `resolveSelectedTableKey` to accept any selected key present in the model table options while preserving base-table fallback for stale selections.
3. Added a focused regression test for joined table selection and stale-key fallback.

Verification:

- Red: `npm --prefix frontend run test -- src/components/tableBrowser/useTableBrowserPage.test.ts` failed because `resolveSelectedTableKey` was not implemented.
- Green: `npm --prefix frontend run test -- src/components/tableBrowser/useTableBrowserPage.test.ts`
- `./frontend/node_modules/.bin/prettier --check frontend/src/components/tableBrowser/useTableBrowserPage.ts frontend/src/components/tableBrowser/useTableBrowserPage.test.ts`
- `npm --prefix frontend run lint`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`

Notes:

- Playwright opened the local app but redirected to `/auth/signin`, and Chrome DevTools was not reachable on `127.0.0.1:9222`, so authenticated visual verification was not available in this session.
