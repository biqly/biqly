## Critical - Security

- [x] `vite-env.d.ts:4` + `utils/env.ts:22` — `VITE_BI_ADMIN_API_KEY` compile-time env var bakes admin key into JS bundle. Production should ONLY use runtime injection (`window.__BIQLY_ENV__`). Remove `VITE_BI_ADMIN_API_KEY` from `ImportMetaEnv` or add build-time guard.
- [x] `types/metadata.ts:11,17` — `Datasource.config` ve `connection_params` sunucudan DSN/şifre sızdırabilir. Backend'in bu alanları strip ettiğini doğrula.

## Performance

- [x] `utils/resultCellFormat.ts:66-81` — `wantsFractionalDisplay` her çağrıda 11 regex derliyor. Regexleri modül seviyesinde bir kez derle.
- [x] `utils/resultCellFormat.ts:104-125` — Her hücre için yeni `Intl.NumberFormat` instance oluşturuluyor. Cache ile sakla (Map keyed by options).
- [x] `components/ResultTable.tsx:61-75` — `sortedRows` her render'da sıralama yapıyor (10K satıra kadar). `useMemo` ile memoize et.
- [x] `components/AIQuery.tsx:756` — `chartData` her render'da yeniden hesaplanıyor. `useMemo` ekle.
- [x] `components/QueryBuilder.tsx:459` — `chartData` her render'da hesaplanıyor. `useMemo` ekle.
- [x] `components/AIQuery.tsx:1197-1214` — `recentPriorTurns()` her render'da tüm mesajları geziyor. `useMemo` ile memoize et.
- [x] `hooks/useAIJobs.tsx:265` — Her job için ayrı `setInterval` (1200ms). Eşzamanlı job'lar çoğaldığında request flooding olabilir. Tek bir polling döngüsüne birleştir.
- [x] `index.css` — Route bazlı CSS splitting yapıldı: AIQuery/Evaluation/QueryBuilder/Modeling kendi CSS chunk'larına taşındı (`src/styles/`). Initial CSS ~127KB → ~68KB.

## Code Duplication / Refactoring

- [x] **Tip tanımları** — `SemanticModelSummary`, `SemanticDimension`, `SemanticMetric`, `SemanticJoin`, `SemanticModelDetail`, `ColumnRow`, `TableRow` Modeling.tsx, QueryBuilder.tsx, Metadata.tsx ve FewShotExamples.tsx'te tekrar tanımlı. `types/semantic.ts`'e taşı.
- [x] **Chart data mapping** — `rows.map(row => ({ name: String(row[0]), value: Number(row[1]) }))` AIQuery.tsx:756, QueryBuilder.tsx:459 ve Dashboard.tsx:61'de tekrarlı. `utils/chartData.ts`'teki `rowsToChartData`'yı kullan.
- [x] **HTML tag stripping** — `/<[^>]*>/g` regex'i `api/metadataDescribe.ts:34` ve `hooks/useApi.ts:28`'de tekrar. Ortak utility'e taşı.
- [x] **Question preview** — `hooks/useAIJobs.tsx` içinde 3 yerde benzer question preview kodu (238-241, 278-281, 354-359). Tek bir fonksiyona çıkar.
- [x] **SavedQuestions new/edit modal** — 128 satır neredeyse birebir tekrar (536-664 ve 667-795). Ortak form component'ine çıkar.
- [x] **DescribeResult tipi** — `api/metadataDescribe.ts:3-14` ve `types/metadata.ts:47-59`'da tekrar tanımlı. Tek yerden export et.
- [x] **useApi method wrapper'lar** — `useAdminApi.ts` içinde get/postData/putData/patchData/deleteData tekrar wrap edilmiş. Generic wrapper ile sadeleştir.
- [x] **Proxy handler'lar** — `http/ai_proxy.go`, `query_proxy.go`, `catalog_proxy.go` tek `proxy_routes.go`'ya birleşti (table-driven `upstreamProxySpec`); upstream factory zaten paylaşımlıydı.

## i18n / Hard-coded Strings

- [x] `ResultTable.tsx:115` — "Filtre:" hard-coded Türkçe
- [x] `ResultTable.tsx:125` — "Değeri kopyala" hard-coded Türkçe
- [x] `ResultTable.tsx:195,200` — "satır" ve "Sıralama:" hard-coded Türkçe
- [x] `LoadingOverlay.tsx:10` — "Yükleniyor..." hard-coded Türkçe
- [x] `Select.tsx:239` — "Seçenek yok" hard-coded Türkçe
- [x] `AIQuery.tsx:696,700` — "Failed to execute query" ve "Execution failed" hard-coded İngilizce
- [x] `AIQuery.tsx:1518` — "ABI Chat Workspace" hard-coded
- [x] `FewShotExamples.tsx:209` — "Datasource is required" hard-coded İngilizce

## Accessibility

- [x] `AIQuery.tsx:988-996` — Feedback thumbs-up/down butonlarında `aria-label` yok.
- [x] `ResultTable.tsx:108-127` — Context menu sadece right-click ile açılıyor, klavye erişimi yok.
- [x] `Modeling.tsx` — Canvas drag-and-drop sadece mouse ile çalışıyor, klavye navigasyonu yok.
- [x] `Modeling.tsx:1834-1908` — Modal'lar `Modal.tsx` component'ini kullanmıyor, focus trap eksik.
- [x] `QueryBuilder.tsx:567` — `<label>` elementi `htmlFor` ile bağlanmamış.
- [x] `window.confirm()` kullanımı — Datasources.tsx:212, AIQuery.tsx:539, SavedQuestions.tsx:319, PromptTemplates.tsx:285,298. UI thread'i blocklar ve erişilebilir değil. Özel confirm dialog'u ile değiştir.

## Architecture / Design

- [x] `AIQuery.tsx` — 1592 satır, 25+ useState. God component. Sorun sorumluluklarını alt component'lere böl: chat panel, routing panel, candidate viewer, feedback section.
- [x] `Modeling.tsx` — 2738 satır, ~30 useState. Canvas engine, form management ve API logic aynı component'te. En azından canvas renderer ve form modallarını ayır.
- [x] `Metadata.tsx` — 1089 satır. Inline editing, AI describe, bulk describe ayrı component'lerde olmalı.
- [x] `hooks/useAIJobs.tsx:321-333` — Promise-based callback pattern. `dismissJob` çağrılırsa promise hiç resolve olmaz (memory leak). Cleanup ekle.
- [x] `hooks/useAIJobs.tsx:95-111` — `fetchJSON` JSON parse hatasında sessizce `null` dönüyor. API bug'larını maskeler.
- [x] `components/AIQuery.tsx:1119-1155` — useEffect içinde API çağrıları cancellation token olmadan yapılıyor. QueryBuilder.tsx'deki `let cancelled = false` pattern'ini uygula.
- [x] `components/Modeling.tsx:380-388` — useEffect dependency `tables.length` ve `columns.length` içeriyor. Veri yüklenirken gereksiz re-fetch tetikleniyor.
- [x] `components/QueryBuilder.tsx:567-592` — List rendering'de index key (`key={i}`) kullanılıyor. Eleman silindiğinde React reconciliation sorunları çıkar. Unique key kullan.

## Navigation / Router

- [x] `Settings.tsx:30,40` — `window.location.assign()` ile navigate ediliyor, full page reload oluyor. App.tsx'teki `navigate()` fonksiyonunu kullan.
- [x] `TimeGrains.tsx:97` — Aynı sorun: `window.location.assign('/settings')` full page reload.

## Theming

- [x] `utils/chartConfig.ts:3-8` — Hard-coded dark tema renkleri (`#94a3b8`, `#475569`, `#1e293b`). CSS custom properties kullanarak tema değişimine uyumlu hale getir.
- [x] `index.css` — Route bazlı splitting yapıldı; bkz. Performance bölümü.

## Testing

- [x] `AIQuery.tsx` — Ana feature, sıfır test. En azından: send query flow, conversation management, candidate selection testleri ekle.
- [x] `Modeling.tsx` — Canvas drag-and-drop, join creation, model publish flow testleri eksik.
- [x] `QueryBuilder.tsx` — LogicalQuery building logic, filter/having/groupBy ekleme çıkarma testleri eksik.
- [x] `hooks/useAIJobs.tsx` — Job lifecycle, polling, cancellation, conflict detection testleri eksik.
- [x] `hooks/useApi.ts` — Error parsing, timeout, abort handling testleri eksik.
- [x] `utils/resultCellFormat.ts` — Identifier detection, calendar detection, fractional display testleri eksik.
- [x] `hooks/useConversation.ts` — localStorage persistence, quota fallback testleri eksik.
- [x] `components/ResultTable.tsx` — Sorting, context menu, cell formatting testleri eksik.
- [x] Mevcut test coverage: sadece 2 dosya (`chartData.test.ts`, `formatters.test.ts`). Vitest konfigürasyonu mevcut ama kullanılmıyor.

## Quick Wins (< 1 Saat)

- [x] `utils/resultCellFormat.ts` — Regex'leri modül seviyesine taşı (11 regex → module-level `const`)
- [x] `utils/resultCellFormat.ts` — `Intl.NumberFormat` cache ekle (Map ile)
- [x] `ResultTable.tsx:61` — `sortedRows`'u `useMemo` ile sar
- [x] `AIQuery.tsx:756` + `QueryBuilder.tsx:459` — chartData'ya `useMemo` ekle
- [x] `Settings.tsx` + `TimeGrains.tsx` — `window.location.assign` → `navigate()` değiştir
- [x] `api/metadataDescribe.ts:34` + `hooks/useApi.ts:28` — HTML stripping regex'ini ortak utility'e taşı
- [x] `types/semantic.ts` — Duplike tip tanımlarını buraya taşı, diğer dosyalardan import et

## Bigger Refactors (Planlama Gerekli)

- [x] `AIQuery.tsx` (1592 satır) — Alt component'lere böl: ChatPanel, RoutingPanel, CandidateViewer, FeedbackSection, AssistantMessageCard
- [x] `Modeling.tsx` (2738 satır) — Canvas engine, form management ve API logic ayır. AddMetricModal (700 satır) ayrı dosya olmalı.
- [x] `Metadata.tsx` (1089 satır) — Inline editing, AI describe, bulk describe ayrı component'ler olmalı
- [x] `hooks/useAIJobs.tsx` (679 satır) — Promise-based callback pattern'ini iyileştir. Dangling promise leak'ini düzelt
- [x] `index.css` — Route bazlı CSS splitting tamamlandı (`src/styles/`). 4 ana route kendi CSS chunk'ında.
- [x] `window.confirm()` — Tüm kullanımları özel ConfirmDialog component'i ile değiştir
- [x] Test altyapısı — Vitest konfigürasyonu mevcut. En azından hooks ve utils için unit test'ler yazılmalı
