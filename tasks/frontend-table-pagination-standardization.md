# Frontend Standardizasyon Planı — Table, Pagination, Filter, Sorting, State Rendering ve Genel UI/Data Pattern'leri

> Durum: **Plan — kod değişikliği yapılmadı.**
> Kapsam: `frontend/src` altındaki tüm tekrar eden UI/data-handling pattern'leri (table, pagination, sorting, filtering, loading/empty/error state, row/bulk action, cell formatter, form/button, modal, URL state, i18n, test).
> Kısıtlar: mevcut ekran davranışları bozulmayacak, backend API kontratı değişmeyecek, büyük/breaking refactor yerine küçük ve bağımsız uygulanabilir adımlar.
> Analiz tarihi: 2026-06-12. Bu dokümandaki tüm dosya yolları repo köküne göredir ve gerçek dosyalardır.

---

## 1. Mevcut Mimari Özeti

| Konu | Durum |
| --- | --- |
| Stack | React 19.2, TypeScript 6.0 (strict), Vite 8, react-router-dom 7.17, recharts 3, clsx |
| Data fetching | Üçüncü parti kütüphane **yok** (react-query/swr yok). `frontend/src/api/apiClient.ts` (`apiFetch`) + `frontend/src/hooks/useApi.ts` (`useApi`/`useAdminApi`) |
| Stil | Vanilla CSS + BEM. `frontend/src/styles/` altında 34 dosya + `frontend/src/index.css` (84 KB) |
| i18n | `frontend/src/i18n/` → `useT()` hook'u |
| Test | vitest 4, **sadece pure-function testleri** (26 dosya). `@testing-library/react` yok, component render testi yok |
| Lint | ESLint 9 (`--max-warnings 0`, `no-explicit-any`, jsx-a11y, security, react-hooks v7), Prettier, Knip |
| Shared UI | `frontend/src/components/ui/` — 36 dosya (Modal, Select, Pagination, EmptyState, ErrorAlert, Toast, vb.) |

Önemli mimari gerçek: `useApi().call` **hook-instance bazlı tek bir `loading`/`error` state** tutar (`frontend/src/hooks/useApi.ts:22-86`), hata fırlatmaz `T | null` döner. Bu yüzden her ekran kendi `data`/`loading`/`error`/`page` state üçlü-dörtlüsünü elle kurmak zorunda kalıyor — tekrarın kök nedeni budur.

---

## 2. Envanter ve Tespit Edilen Tekrarlar

### 2.1 Table render eden component'ler (38 dosya, `<table>` grep ile doğrulandı)

**Grup A — Sayfalı (paginated) admin/list tabloları** (asıl standardizasyon hedefi):

| Ekran | Dosya | Pagination | Veri kaynağı |
| --- | --- | --- | --- |
| Audit Log | `frontend/src/components/admin/AuditLogPanel.tsx` | server-side, `Pagination` | `listAuditLog` (`api/admin.ts:131`) |
| Kullanıcılar | `frontend/src/components/admin/userList/ActiveUsersTab.tsx` (+ `UserListPage.tsx` orchestrator) | server-side, `Pagination` | `listUsers` (`api/admin.ts:326`) |
| Davetler | `frontend/src/components/admin/userList/InvitationsTab.tsx` | server-side, `Pagination` | `listInvitations` (`api/auth.ts:324`) |
| Datasource Access | `frontend/src/components/admin/DatasourceAccessPanel.tsx` | server-side, `Pagination` | `listDatasourceAccess` (`api/admin.ts:164`) |
| Workspaces | `frontend/src/components/admin/WorkspacesPanel.tsx` | server-side, `Pagination` | `listWorkspaces` (`api/admin.ts:246`) |
| Field Permissions | `frontend/src/components/admin/FieldPermissionPanel.tsx` | server-side, `Pagination` (`page_size` query param'ı elle string kuruyor, satır 163) | admin API |
| Confirmed Queries | `frontend/src/components/admin/ConfirmedQueriesPanel.tsx` | **client-side slice** (API tüm listeyi döndürür; Faz 1'de düzeltilen sınıflandırma) | `listConfirmedQueries` (`api/aiAdmin.ts`) |
| AI Usage (admin) | `frontend/src/components/admin/AIUsageAdminPanel.tsx` | **client-side slice** (`rows.slice(start, start+pageSize)`, satır 83-85), `Pagination` | admin API |
| AI History | `frontend/src/components/ai/AIHistoryPanel.tsx` | server-side, `Pagination` + `useQueryParam` | `listAIHistory` (`api/admin.ts:523`) |
| Query History | `frontend/src/components/QueryHistory.tsx` | server-side, `Pagination`, sabit `pageSize = 10` (satır 39) | query API |
| Shared Resources | `frontend/src/components/sharing/SharedResourcesList.tsx` | server-side, `Pagination`, sabit `pageSize = 10` (satır 28) | `api/admin.ts:613` |
| Table Browser | `frontend/src/components/tableBrowser/TableBrowserModelContent.tsx` | server-side **offset/limit + hasNext**, doğrudan `PaginationControls` | `useTableBrowserQueryState.ts` |

**Grup B — Sayfasız tablolar** (sıralama/boş-durum standardizasyonundan faydalanır): `ABExperimentList.tsx`, `ABExperimentDetail.tsx`, `AIJobsAdminPanel.tsx`, `AIProvidersPanel.tsx`, `DriftPanel.tsx`, `PIIDetectionPanel.tsx`, `UserDetailSections.tsx` (admin/); `EvalHistoryTab.tsx`, `EvalRegressionTab.tsx`, `EvalRunTab.tsx` (evaluation/); `Datasources.tsx`, `FewShotExamples.tsx`, `Glossary.tsx`, `GlossaryEnrichPanel.tsx`, `PromptTemplates.tsx`, `TimeGrainsTable.tsx`, `AIUsageDashboard.tsx`, `MetadataTablesPanel.tsx`, `MetadataColumnPanel.tsx`, `MetadataDescribeResults.tsx`, `MetadataBulkDescribeProgress.tsx` (metadata/), `WorkspaceSettingsPage.tsx`, `SharedResourcesList.tsx`.

**Grup C — Özelleşmiş tablolar** (bu planın **dışında** tutulacak, sadece formatter/empty-state ortaklığından faydalanır): `frontend/src/components/ResultTable.tsx` (sorgu sonuç grid'i, kendi sort/filter/pivot mantığı `components/resultTable/` altında), `queryBuilder/QueryBuilderResults.tsx`, `dashboard/DashboardWidgetRenderer.tsx`, `aiQuery/SampleDataModal.tsx`, `tableBrowser/TableBrowserRowModal.tsx`.

### 2.2 Pagination — bileşenler iyi, state yönetimi 12 yerde kopyalanmış

Shared bileşenler sağlam ve davranışları test edilmeye değer:

- `frontend/src/components/ui/PaginationControls.tsx` — 7-slot stabil sayfa penceresi, 1-based, a11y'li (`aria-current`, `aria-label`), `size: 'sm'|'md'`, `formatNumber` prop'u. CSS: `frontend/src/styles/pagination.css`.
- `frontend/src/components/ui/Pagination.tsx` — `PaginationControls`'u sarar + "x–y / toplam" aralık etiketi. Props: `currentPage, totalPages, onPageChange, totalItems?, itemsPerPage?, alwaysShow?`. **Not:** içinde inline `style={{...}}` blokları var (satır 44-60) — BEM kuralına aykırı, refactor'da `pagination.css`'e taşınmalı.

Asıl tekrar **state tarafında**. Her tüketici şu bloğu kopyalıyor (örnek: `AuditLogPanel.tsx:69-114`, `DatasourceAccessPanel.tsx:45-71`, `SharedResourcesList.tsx:28-52`, `AIUsageAdminPanel.tsx:66-85`, `QueryHistory.tsx:39-95`):

```tsx
const [currentPage, setCurrentPage] = useState(1)
const [pageSize, setPageSize] = useState(DEFAULT_X_PAGE_SIZE)   // veya sabit 10
const [totalItems, setTotalItems] = useState(0)
const totalPages = Math.ceil(totalItems / pageSize)             // bazısı Math.max(1, ...) ile, bazısı değil!
const [loading, setLoading] = useState(true)
const [error, setError] = useState<string | null>(null)
useEffect(() => { /* fetch + setEntries + setTotalItems + try/catch */ }, [token, currentPage])
```

Tutarsızlıklar (davranış farkı yaratan):

- `totalPages` hesabı: `Math.max(1, Math.ceil(...))` (`QueryHistory.tsx:40`, `AIUsageAdminPanel.tsx:79`) vs çıplak `Math.ceil(...)` (`AuditLogPanel.tsx:72`, `DatasourceAccessPanel.tsx:47`, `SharedResourcesList.tsx:30`) → total=0'da `totalPages=0`.
- Filtre değişince sayfa 1'e dönme: `AuditLogPanel.tsx:118-125` submit-on-filter + `eslint-disable exhaustive-deps`; `ActiveUsersTab` ise filtreyi parent'ta (`UserListPage.tsx`) tutup 18 prop drilling yapıyor (`t: TFunction` ve `locale` dahil — anti-pattern).
- 1-based `page/pageSize` (admin ekranları) vs 0-based `offset/limit + include_total` (`tableBrowser/useTableBrowserQueryState.ts:28-47`, `hasNext` türetimi `derivePaging`, satır 50-61).
- `FieldPermissionPanel.tsx:163` query string'i elle kuruyor (`page_size: String(fieldPageSize)`), diğerleri `api/admin.ts` fonksiyonlarına devrediyor.

### 2.3 API response shape'leri (backend kontratı — DEĞİŞMEYECEK, adapter ile soğurulacak)

Backend zaten tutarlı bir aileye sahip: `{ <kaynakAdı>: T[]; total: number }` + `page`/`pageSize` (1-based) query parametreleri. Frontend'de tespit edilen tüm şekiller:

| Shape | Tanım yeri |
| --- | --- |
| `{ roles: Role[]; total }` | `frontend/src/api/admin.ts:29` |
| `{ permissions: Permission[]; total }` | `frontend/src/api/admin.ts:50` |
| `{ entries: AuditLogEntry[]; total }` | `frontend/src/api/admin.ts:131` |
| `{ access: DatasourceAccess[]; total }` | `frontend/src/api/admin.ts:164` |
| `{ workspaces: Workspace[]; total }` | `frontend/src/api/admin.ts:246` |
| `{ users: AuthUserRaw[]; total }` | `frontend/src/api/admin.ts:326` |
| `{ entries: AIHistoryEntry[]; total }` | `frontend/src/api/admin.ts:523` |
| `{ shares: ResourceShare[] \| null; total }` | `frontend/src/api/admin.ts:613` (null-guard'lı!) |
| `{ invitations: Invitation[]; total }` | `frontend/src/api/auth.ts:324` |
| `{ columns, rows, total? }` + `limit/offset/order_by/order_dir` body | `frontend/src/components/tableBrowser/useTableBrowserQueryState.ts:5-47` |
| `{ total, page_size, ... }` (semantic) | `frontend/src/types/semantic.ts:166-168` |

`frontend/src/types/` altında ortak bir `Paginated<T>` / list-response wrapper tipi **yok**; her api fonksiyonu kendi inline tipini yazıyor.

### 2.4 Sorting

- **Client-side:** `frontend/src/components/resultTable/sort.ts` + `ResultTable.tsx` (özelleşmiş, kapsamda değil). Grup B tablolarının çoğunda kolon sıralama **hiç yok**.
- **Server-side:** sadece Table Browser — `order_by` / `order_dir` body alanları (`useTableBrowserQueryState.ts:36-44`).
- `queryBuilder/SortStep.tsx` sorgu semantiği (SQL ORDER BY), UI tablosu sıralaması değil — kapsam dışı.
- Ortak bir `useSortState` hook'u veya sıralanabilir `<th>` bileşeni yok; sıralama eklenmek istense her ekran sıfırdan yazacak.

### 2.5 Filtering / arama

Dört farklı pattern yan yana yaşıyor:

1. **Parent-state + prop drilling:** `UserListPage.tsx` → `ActiveUsersTab.tsx` (`search`, `statusFilter`, `setSearch`... 18 prop).
2. **Filter-on-submit:** `AuditLogPanel.tsx:118-125` (form submit, sayfa-1 reset, `exhaustive-deps` disable).
3. **Yapısal filtre state'i:** `tableBrowser/useTableBrowserFilterState.ts` (8.3 KB, operator/case-sensitive destekli) — repodaki en gelişmiş örnek.
4. **Context-menu filtreleri:** `ResultTable` (kapsam dışı).

Debounce'lu ortak arama hook'u yok; her ekran kendi `<input className="admin-input">` + state ikilisini kuruyor.

### 2.6 Loading / Error / Empty state

Shared bileşenler **var** ama benimseme yarım:

| Bileşen | Tüketici sayısı (grep ile) | Boşluk |
| --- | --- | --- |
| `ui/LoadingOverlay` / `ui/LoadingIndicator` / `ui/Skeleton` | 28 dosya | Geri kalan ekranlar inline spinner/`{loading ? '' : ...}` (örn. `TimeGrainsTable.tsx:110`) |
| `ui/ErrorAlert.tsx` | 21 dosya | Diğerlerinde inline `{error && <div>...}`, bazılarında `useToast().error` — tutarsız |
| `ui/EmptyState.tsx` | 13 dosya | Grup A admin tablolarının **hiçbiri** kullanmıyor; `colSpan`'li inline `<td>` metinleri yaygın |
| `ui/ErrorBoundary.tsx` | App seviyesi | — |

Her sayfalı ekran loading→error→empty→data karar ağacını JSX'te elle kuruyor; sıra ve öncelik ekranlar arasında farklı (kimi loading'de tabloyu gizliyor, kimi `LoadingOverlay` ile üstüne biniyor).

### 2.7 Row action / bulk action

- `frontend/src/components/ui/ActionMenu.tsx` mevcut ama **tek tüketicisi var**: `components/modeling/ModelingToolbar.tsx`. Tüm admin tabloları satır aksiyonlarını inline `<button className="btn btn-...">` dizisiyle render ediyor (örn. `ActiveUsersTab` "resend verification", `Datasources.tsx`, `PromptTemplates.tsx`).
- Checkbox tabanlı çoklu seçim en az 4 yerde elle kurulmuş: `admin/DriftPanel.tsx`, `admin/RolesPanel.tsx`, `modeling/ModelingPalette.tsx`, `composites/CompositeDetailPanel.tsx` (+ `GlossaryEnrichPanel.tsx` bulk-select). Ortak `useRowSelection` hook'u yok; select-all/indeterminate davranışı her yerde yeniden yazılıyor.
- Silme onayı: `ui/ConfirmDialog.tsx` + `hooks/useConfirm.tsx` mevcut ve iyi — yeni standardın parçası olmalı.

### 2.8 Cell formatter tekrarları

- `frontend/src/utils/formatters.ts` (genel tarih/sayı formatlama) ile `frontend/src/utils/resultCellFormat.ts` (kolon tipi sınıflandırma + Intl cache) **çakışan sorumluluklara** sahip; ikisinin de testi var (`formatters.test.ts`, `resultCellFormat.test.ts`).
- Tarih formatlama ayrıca ekranlarda inline `toLocaleString`/`localeLanguageTag` çağrılarıyla tekrarlanıyor (örn. `ActiveUsersTab` `locale` prop'u alıp kendi formatlıyor).
- `frontend/src/utils/exportCsv.ts` ve `utils/pivotTable.ts` ResultTable'a özgü, dokunulmayacak.

### 2.9 Form / input / button (frontend geneli)

- **Shared `Button` bileşeni yok** — `className="btn btn-*"` kalıbı 171 yerde geçiyor; varyant adlarının envanteri çıkarılmalı (Assumption: `btn-primary`, `btn-secondary`, `btn-danger`, `btn-ghost` ailesi, `index.css`'te tanımlı).
- **Shared `Input`/`FormField` bileşeni yok** — `admin-input`, `auth-input` vb. sınıflarla ham `<input>` (137 civarı; Explore ajanı sayımı, ASSUMPTION). Alan-bazlı hata gösterimi her formda farklı.
- `ui/Select.tsx` güçlü ve yaygın (49 dosya), `MultiSelect`, `SelectPopover`, `useSelectDropdown` ekosistemi düzgün.

### 2.10 Modal

- `frontend/src/components/ui/Modal.tsx` kanonik (focus-trap/aria'lı, `styles/modal.css`).
- Elle yazılmış modal/overlay'ler mevcut: `tableBrowser/TableBrowserRowModal.tsx` (16 KB, kendi overlay'i), `aiQuery/SampleDataModal.tsx` (kendi CSS dosyası `styles/sample-data-modal.css`). Explore ajanı toplam ~12 hand-rolled overlay raporladı (ASSUMPTION — Faz 7'de dosya dosya doğrulanacak).

### 2.11 URL state

- `frontend/src/hooks/useQueryParam.ts` (`useSyncExternalStore` + popstate) — 10 tüketici: `App.tsx`, `Evaluation.tsx`, `AIQuery.tsx`, `Metadata.tsx`, `QueryBuilder.tsx`, `admin/UserListPage.tsx`, `admin/Admin.tsx`, `admin/WorkspacesPanel.tsx`, `ai/AIHistoryPanel.tsx`, `modeling/useModelingPageState.ts`.
- **Hiçbir ekran sayfa numarasını/filtreyi URL'e yazmıyor** (AIHistoryPanel kısmen yazıyor olabilir — Open Question). Sayfa yenilenince liste state'i kayboluyor.

### 2.12 i18n

- Standart: `useT()`. İhlal: 10+ dosya `t: TFunction`'ı **prop olarak** alıyor (`TimeGrainsTable.tsx`, `TimeGrainsEditModal.tsx`, `ActiveUsersTab.tsx`, `queryBuilder/*Step.tsx`, `auth/SignInMfaForm.tsx`...). Ayrıca `TimeGrainsTable.tsx:110`'da `t('common.no_data') || 'No data found'` gibi fallback-string kalıntıları var.
- `ui/Pagination.tsx` aralık etiketi için `table_browser.*` anahtarlarını kullanıyor — genel bileşen, feature-scoped anahtar (taşınmalı: `common.pagination.*`).

### 2.13 Test altyapısı

- 26 test dosyası, tamamı pure-function (örn. `components/tableBrowser/useTableBrowserPage.test.ts`, `components/admin/AuditLogPanel.test.ts` — logic extraction yöntemiyle). Component render testi sıfır; `@testing-library/react`/`jsdom` **kurulu değil**.
- Bu repoda kanıtlanmış test pattern'i: **mantığı pure fonksiyona çıkar, onu test et** (`abExperimentPanelLogic.test.ts`, `piiDetectionPanelLogic.test.ts`). Plan bu pattern'i temel alır; render testi eklemek Open Question.

---

## 3. Hedef Mimari

Yeni paylaşılan katman (hepsi mevcut klasör düzenine uyar):

```text
frontend/src/types/pagination.ts          # PageQuery, Paginated<T> yardımcı tipleri
frontend/src/utils/paging.ts              # getTotalPages, clampPage, pageRange, sliceClientPage (pure)
frontend/src/hooks/usePaginatedList.ts    # server-side liste: data/loading/error/page/pageSize/total + reload
frontend/src/hooks/useClientPagination.ts # client-side dilimleme (AIUsageAdminPanel pattern'i)
frontend/src/hooks/useSortState.ts        # { key, dir, toggle } — client & server kullanımına nötr
frontend/src/hooks/useDebouncedValue.ts   # arama inputları için
frontend/src/hooks/useRowSelection.ts     # Set tabanlı seçim + selectAll/indeterminate
frontend/src/components/ui/DataState.tsx  # loading→error→empty→children karar ağacı (tek yerde)
frontend/src/components/ui/DataTable.tsx  # column-config'li tablo + sortable th + row actions slot
frontend/src/components/ui/SortableTh.tsx # aria-sort'lu başlık hücresi
frontend/src/components/ui/RowActions.tsx # satır aksiyon butonları/menüsü (ActionMenu'yu sarar)
frontend/src/utils/format/                # formatters.ts + resultCellFormat.ts konsolidasyonu (Faz 6)
```

### 3.1 Çekirdek sözleşmeler (tasarım taslağı)

```ts
// types/pagination.ts
export interface PageQuery { page: number; pageSize: number }          // 1-based — admin API'lerle aynı
export interface PagedResult<T> { items: T[]; total: number }

// hooks/usePaginatedList.ts — backend kontratını DEĞİŞTİRMEZ; her ekran kendi
// fetcher'ıyla mevcut api/ fonksiyonunu sarar ve {key: T[], total} -> PagedResult'a map eder.
export function usePaginatedList<T>(opts: {
  fetcher: (q: PageQuery, signal?: AbortSignal) => Promise<PagedResult<T>>
  initialPageSize?: number          // default 10 — mevcut ekran default'larıyla birebir
  deps?: unknown[]                  // filtre değerleri; değişince sayfa 1'e döner (opt-in)
  resetPageOnDepsChange?: boolean
}): {
  items: T[]; total: number; totalPages: number
  page: number; setPage: (p: number) => void
  pageSize: number; setPageSize: (s: number) => void
  loading: boolean; error: string | null
  reload: () => Promise<void>
}
```

```tsx
// components/ui/DataTable.tsx — kolonlar config, hücre render'ı type-safe
export interface ColumnDef<T> {
  key: string
  header: ReactNode
  cell: (row: T) => ReactNode
  sortable?: boolean                // useSortState ile entegre
  width?: string
  align?: 'left' | 'right' | 'center'
}
export function DataTable<T>(props: {
  columns: ColumnDef<T>[]
  rows: T[]
  rowKey: (row: T) => string
  sort?: SortState; onSortChange?: (s: SortState) => void
  selection?: RowSelection<T>       // useRowSelection çıktısı, opsiyonel
  rowActions?: (row: T) => ReactNode
  emptyState?: ReactNode            // verilmezse <EmptyState/> default'u
  loading?: boolean                 // LoadingOverlay entegrasyonu
  caption?: string                  // a11y
})
```

```tsx
// components/ui/DataState.tsx — karar ağacını tek yerde sabitler
<DataState loading={loading} error={error} empty={items.length === 0}
           emptyState={<EmptyState description={t('...')} />}>
  {children}
</DataState>
// Sıralama sabit: error > loading(ilk yükleme) > empty > children.
// Yeniden yüklemede (data varken) LoadingOverlay ile üst bindirme — mevcut admin davranışı korunur.
```

Tasarım ilkeleri:

1. **Adapter, dayatma değil:** `usePaginatedList` mevcut `api/admin.ts` fonksiyonlarını sarar; query param adları (`page`, `pageSize`, `page_size`, `limit/offset`) api katmanında kalır. Table Browser'ın offset/limit + `hasNext` dünyası **bu hook'a zorlanmaz** (kendi `useTableBrowserPage` hook'u zaten test edilmiş ve iyi durumda — olduğu gibi kalır).
2. **`ResultTable` dokunulmaz:** sorgu sonuç grid'i kendi özel dünyasında kalır; sadece Faz 6 formatter konsolidasyonundan faydalanır.
3. **Opt-in migrasyon:** her ekran ayrı PR'da geçer; geçmeyen ekran eski haliyle çalışmaya devam eder.
4. **Knip uyumu:** yeni paylaşılan modüller, en az bir tüketiciyle **aynı PR'da** eklenir (knip unused-export hatası vermesin diye).

---

## 4. Adım Adım Uygulama Planı

Her faz bağımsız merge edilebilir; her adımın sonunda `make lint-frontend && npm --prefix frontend run format:check && npm --prefix frontend run knip:ci && make test-frontend && npm --prefix frontend run build` yeşil olmalı (CI `check` gate'iyle aynı).

### Faz 0 — Güvenlik ağı (davranış kilitleme) ✅ TAMAMLANDI (2026-06-12)

- [x] **0.1** `frontend/src/utils/paging.ts` oluşturuldu: `getTotalPages` (**OQ-1 kararı: `Math.max(1, ceil)`** — ekranlardaki çıplak `ceil` ile UI farkı yok çünkü `Pagination` bileşeni zaten `Math.max(1, totalPages)` ile savunuyor; test yorumlarında belgelendi), `clampPage`, `pageRange` (`ui/Pagination.tsx` inline matematiğiyle birebir), `sliceClientPage` (AIUsageAdminPanel clamp'li slice pattern'i). Testler: `frontend/src/utils/paging.test.ts`.
- [x] **0.2** `buildStablePageTokens` ve `PageToken` tipi yeni `frontend/src/components/ui/paginationTokens.ts` modülüne taşındı (ESLint `react-refresh/only-export-components` kuralı component dosyasından fonksiyon export'una izin vermiyor; repo'daki `ui/selectKeyboard.ts` pattern'i izlendi — kod birebir, davranış değişikliği yok). Karakterizasyon testleri: `frontend/src/components/ui/paginationControls.test.ts` (pad/leading/middle/trailing pencereleri + her zaman 7 token invaryantı).
- [x] **0.3** Pilot ekranların mevcut davranış notları §8.1'e eklendi.

Kabul: yeni testler yeşil, prod kodda davranış değişikliği sıfır (sadece test + yeni pure util + iki `export` keyword'ü).
Not: knip CI gate'i (`knip:ci` → `files,dependencies,unlisted,unresolved`) unused-export kontrolü yapmıyor ve vitest test dosyaları entry sayılıyor → test'li `paging.ts` knip'e takılmıyor (doğrulandı).

### Faz 1 — Pagination state standardizasyonu (`usePaginatedList`) ✅ TAMAMLANDI (2026-06-12)

- [x] **1.1** `frontend/src/types/pagination.ts` (`PageQuery`, `PagedResult<T>`) + `frontend/src/hooks/usePaginatedList.ts`. Tasarım kararları:
  - Fetcher **latest-ref** ile tutuluyor: filtre değerleri fetcher closure'ında olsa bile yazarken fetch tetiklenmez — AuditLogPanel'in filter-on-submit semantiği (sayfa değişiminde *o anda yazılı* filtrelerle fetch dahil) birebir korunur.
  - Tetikleyiciler primitive: `fetchKey` (refetch, sayfa korunur), `resetPageKey` (sayfa-1 + refetch). Bileşik değerler template string ile kompoze edilir (eslint `exhaustive-deps`'in spread-deps kısıtı nedeniyle dizi yerine primitive seçildi).
  - `enabled: false` → fetch yok, `loading` başlangıç değeri `true`'da kalır (SharedResourcesList'in token-yokken-overlay davranışı korunur).
  - Race guard: her fetch'e `AbortController`; cleanup'ta abort; abort edilen isteğin sonucu/hatası state'e yazılmaz (eski ekranların çoğunda bu guard yoktu — geç gelen yanıtın state ezmesi düzeldi, görünür davranış değişmez).
  - `setError` dışa açık: mutation (grant/revoke/delete) hataları eski `setError` gibi aynı kanala yazılır, sonraki başarılı load temizler.
  - Tek `eslint-disable react-hooks/set-state-in-effect` hook içinde (ekran başına 1'er disable'ın yerine).
- [x] **1.2** Pure mantık `frontend/src/hooks/usePaginatedListLogic.ts`'te (`paginatedListReducer`, `initialPaginatedListState`, `errorMessage`); testler `usePaginatedListLogic.test.ts` (stale-rows-on-error ve overlay-on-reload karakterizasyonu dahil).
- [x] **1.3** Pilot 1: `admin/DatasourceAccessPanel.tsx` ✓ (`{access,total}` adapter; grant/revoke'taki `setCurrentPage(1)+reload` çifti artık tek fetch — final state aynı).
- [x] **1.4** Pilot 2: `sharing/SharedResourcesList.tsx` ✓ (`resetPageKey: resourceType`, `fetchKey: accessToken|refreshKey`, `enabled: Boolean(accessToken)`).
- [x] **1.5** `AuditLogPanel` ✓ (iki `eslint-disable` silindi; submit/reset/pageSize semantiği korundu), `WorkspacesPanel` ✓, `QueryHistory` ✓ (filterKey → `resetPageKey`; elle kurulan key'li pageState kalktı), `AIHistoryPanel` ✓ (`resetPageKey: showAll`), `UserListPage` ✓ (users + invitations iki ayrı hook instance'ı; `enabled` sub-tab'a bağlı; `loadInvitations` → `reload()`). **Not:** `ActiveUsersTab`/`InvitationsTab` prop arayüzlerine dokunulmadı; `t`/`locale` prop-drilling temizliği Faz 7.4'e bırakıldı.
  - **İstisna — `FieldPermissionPanel`:** yalnızca planlanan API extraction yapıldı (`frontend/src/api/semantic.ts` → `listSemanticModelFields`, URL/param birebir aynı). State migrasyonu YAPILMADI çünkü ekran üç noktada hook sözleşmesinden sapıyor: `loadingFields` false başlıyor, hata durumunda satırları/`modelName`'i temizliyor (hook stale bırakır), `modelName` aynı response'tan türüyor. Faz 3 DataTable migrasyonunda yeniden değerlendirilecek.
  - **Düzeltme — `ConfirmedQueriesPanel`:** §2.1'de server-side sınıflandırılmıştı; gerçekte **client-side** (API tüm listeyi döndürüyor). `useClientPagination` ile geçirildi ✓.
- [x] **1.6** `frontend/src/hooks/useClientPagination.ts` (sayfa clamp'li slice; satır sahipliği component'te kalır) + `AIUsageAdminPanel.tsx` ✓ ve `ConfirmedQueriesPanel.tsx` ✓.

Kabul doğrulaması: API çağrıları aynı fonksiyonlar üzerinden aynı parametrelerle (`page`/`pageSize` değerleri hook'tan, kaynağı aynı); `make check-frontend` zinciri yeşil (29 dosya / 133 test); net −173 satır (594 silindi, 421 eklendi, hook/test dosyaları hariç ekran kodunda).
Bilinen minör fark (kabul edildi, davranışsal iyileştirme): eski ekranlardaki bazı çift-fetch'ler (örn. filtre+sayfa reset'inin iki ayrı effect tetiklemesi) React batching sayesinde tek fetch'e indi; final ekran durumu birebir aynı. Edge: super_admin olmayan kullanıcı URL'e `?subTab=invitations` yazarsa eskiden boş tablo, şimdi loading overlay görür (fetch yine yapılmaz).

### Faz 2 — Loading / Error / Empty standardizasyonu (`DataState`) ✅ TAMAMLANDI (2026-06-12)

- [x] **2.1** `frontend/src/components/ui/DataState.tsx` + `frontend/src/styles/data-state.css`. API: `loading`, `error` (+`errorPrefix`), `empty`, `emptyState?` (ReactNode), `className?` (`data-state__body--scroll-x` gibi). Karar ağacı tek yerde: `ErrorAlert` banner üstte (içerik/stale satırlar görünür kalır) → `LoadingOverlay` → boş+yüklemede 120px rezerve kutu → `emptyState` ya da children. `emptyState` **opsiyonel**: tbody-içi `'—'` placeholder satırı olan tablolar (DatasourceAccessPanel, WorkspacesPanel listesi) children'da kalır — bunlar Faz 3'te `DataTable.emptyState`'e taşınacak.
- [x] **2.2** Geçirilen ekranlar: `DatasourceAccessPanel`, `SharedResourcesList`, `AuditLogPanel`, `WorkspacesPanel`, `QueryHistory`, `AIHistoryPanel` (DataState); `UserListPage` ve `AIUsageAdminPanel` (yalnızca `ErrorAlert` — overlay sahipliği tab'larda/özel yapıda). `TimeGrainsTable.tsx`'teki `|| 'fallback'` ölü i18n fallback'lerinin tamamı temizlendi (anahtarların EN+TR'de varlığı doğrulandı).
- [x] **2.3** Grup B yüksek trafikli ekranlar (Datasources, Glossary, PromptTemplates, FewShotExamples) tarandı: **zaten** `ErrorAlert`/`EmptyState`/`LoadingOverlay` kullanıyorlar; kalan `loading ?` kalıpları buton etiketi (save/saving), liste-durumu değil. Churn yapılmadı; DataState'e geçişleri Faz 3 dokunuşlarında fırsatçı yapılacak.

Kabul doğrulaması: `make check-frontend` zinciri yeşil (29 dosya / 133 test). Grup A'da inline error-banner ve minHeight hack'i kalmadı.
**Bilinçli görsel normalizasyon (kabul edildi):** (1) hata mesajları ekran-başına farklı kırmızı stillerden (`admin-err-text` düz metin, `shared-list__error`, `ai-history__error`, inline style) standart `ErrorAlert` banner'ına geçti ve artık tablo container'ının içinde, tablonun hemen üstünde görünüyor; (2) tam-tablo boş durumları (`AuditLog`, `SharedResources`, `AIHistory` padded metinleri) standart `EmptyState`'e geçti; (3) `QueryHistory` ilk yüklemede başlık-satırlı boş tablo yerine diğer ekranlarla aynı boş overlay kutusunu gösteriyor. Davranış (ne zaman ne görünür) aynı; yalnızca stil birleşti.

### Faz 3 — `DataTable` bileşeni ✅ TAMAMLANDI (2026-06-12)

- [x] **3.1** `frontend/src/components/ui/DataTable.tsx` + `ColumnDef<T>` (`key`, `header`, `cell`, `className?`, `align?`). **Yeni CSS dosyası gerekmedi:** bileşen default olarak mevcut `admin-table`/`admin-thead-row`/`admin-th`/`admin-tr`/`admin-td` sınıflarını üretir (markup-nötr migrasyon); başka tablo aileleri için `tableClassName`/`headRowClassName`/`headerCellClassName`/`rowClassName`/`cellClassName`/`tableStyle` override'ları var. tbody-içi boş-satır placeholder'ı (başlıklar boşken görünür, yüklenirken boş string) `emptyCell` prop'uyla bileşene taşındı.
- [x] **3.2** Pilot: `admin/userList/InvitationsTab.tsx` ✓ — `DataState` + `DataTable` + `Pagination` üçlüsü ilk kez bir arada. Tab'a `inviteTotalPages` prop'u eklendi (içerideki çıplak `Math.ceil(inviteTotalItems / pageSize)` silindi; değer `UserListPage`'deki hook'tan geliyor). **Normalizasyon:** hata artık tabloyu *değiştirmiyor*; standart kalıpla banner + stale içerik gösteriliyor ve Pagination hatada gizlenmiyor.
- [x] **3.3** Geçirilenler (6 tüketici): `InvitationsTab`, `ActiveUsersTab`, `DatasourceAccessPanel`, `AuditLogPanel`, `ConfirmedQueriesPanel` (`rowClassName=""` ile satır sınıfsız hali korundu), `SharedResourcesList` (`shared-list__*` sınıf override'ları). Kolon hücre JSX'leri `ColumnDef.cell`'e birebir taşındı.
  - **Kapsam dışı bırakılanlar (gerekçeli):** `WorkspacesPanel` — tablo değil `<ul>` listesi, DataTable zorlanmadı; `QueryHistory` ve `AIHistoryPanel` — genişleyebilir detay satırları (`Fragment` + colSpan detail row) DataTable v1'in desteklemediği bir yapı, sorting/selection ile birlikte v2'de değerlendirilecek; `AIUsageAdminPanel` — th/td tamamen inline-style'lı kendine özgü görünüm; `FieldPermissionPanel` — Faz 1 istisnası (checkbox-grid).
- [ ] **3.4** Grup B tabloları: yalnızca **yeni özellik dokunuşu gerektiğinde** `DataTable`'a geçirilir (fırsatçı migrasyon) — zorunlu sprint işi değil. (Devam eden kural.)

Kabul doğrulaması: DOM yapısı ve sınıflar birebir (admin-* default'ları); `make check-frontend` zinciri yeşil (29 dosya / 133 test). Başarı kriteri 4 (≥5 admin tablosu `DataTable`) karşılandı: 6 tüketici.

### Faz 4 — Sorting & filtering ortak pattern'leri ✅ TAMAMLANDI (2026-06-12)

- [x] **4.1** `frontend/src/utils/sorting.ts` (pure: `SortState`, `toggleSort` asc→desc→none döngüsü, `compareValues` null-last + sayısal/locale karşılaştırma — `components/resultTable/sort.ts` semantiğiyle birebir hizalı, `sortRows`, `ariaSortFor`, `sortArrowFor`) + `sorting.test.ts`. `frontend/src/hooks/useSortState.ts` ince hook. **Ayrı `SortableTh.tsx` yerine** sıralanabilir başlık `DataTable` içine gömüldü (`ColumnDef.sortable` + tablo-seviyesi `sort`/`onSortToggle` props; `aria-sort`'lu th, buton + ok ikonu; CSS: `frontend/src/styles/data-table.css`). **DataTable satırları kendisi SIRALAMAZ** — çağıran `sortRows` ile sıralar; böylece ileride server-side sort aynı başlık UI'ını kullanabilir.
- [x] **4.2** **OQ-3 kararı (teknik gerekçeli):** kolon sıralaması yalnızca **client-side** listelerde açıldı — server-side sayfalı tablolarda yalnızca görünen sayfayı sıralamak yanıltıcı olur ve backend'e sort parametresi eklemek API kontratını değiştirir (yasak). Aktivasyon: `ConfirmedQueriesPanel` (question/confirmed_at/status kolonları; sıralama dilimden ÖNCE tüm liste üzerinde, sıralama değişince sayfa 1'e döner). Diğer client-side aday `AIUsageAdminPanel` DataTable'a geçmediği için (Faz 3 istisnası) şimdilik sıralamasız; DataTable'a geçtiğinde `sortable: true` tek satırlık iş.
- [x] **4.3** `frontend/src/hooks/useDebouncedValue.ts`; mevcut elle yazılmış 3 debounce ortak hook'a taşındı: `UserListPage` (users + invitations search, `value.trim()` semantiği korundu) ve `QueryHistory` (trimsiz). Süre 300ms — mevcut davranışla birebir. Debounce'u olmayan inputlara YENİ debounce eklenmedi.
- [x] **4.4** "Filtre değişti → sayfa 1" taraması: Grup A ekranlarının tamamı Faz 1'de `resetPageKey`/handler'larla tekleşti; `ConfirmedQueriesPanel` (DS değişimi + yeni sort değişimi) ve `AIUsageAdminPanel` (period/pageSize değişimi) elle `setPage(1)` çağırıyor — istisna kalmadı.

Kabul doğrulaması: `make check-frontend` zinciri yeşil (30 dosya / 141 test — +8 sorting testi). OQ-3 kapatıldı (yukarıdaki kararla).

### Faz 5 — Row action / bulk action ✅ TAMAMLANDI (2026-06-12, kapsam gerçeğe göre düzeltildi)

- [x] **5.1** `RowActions.tsx` **YAZILMADI — bilinçli karar:** kod taramasında satır başına 3+ aksiyonlu hiçbir ekran yok (maksimum 2: `InvitationsTab` resend/revoke); inline aksiyon butonları Faz 3'te `ColumnDef.cell` içinde zaten standartlaştı; silme onayı `useConfirm`/`ConfirmDialog` ile zaten ortak. ">2 aksiyonda menü" yolunun tüketicisi olmadığından spekülatif bileşen eklenmedi — 3+ aksiyonlu bir ekran ortaya çıktığında mevcut `ui/ActionMenu.tsx` doğrudan `cell` içinde kullanılır.
- [x] **5.2** `frontend/src/utils/selection.ts` (pure: `toggleId`, `setIds`, `sameIdSet` — dirty karşılaştırma, `selectionStateFor` — none/some/all ile indeterminate) + `selection.test.ts`; `frontend/src/hooks/useRowSelection.ts` (`selected/isSelected/toggle/setMany/replace/clear`). **OQ-4 kararı:** hook seçim setini sayfa/filtre değişiminde KENDİSİ sıfırlamaz — politika ekran kararıdır, sayfa state'inin yaşadığı yerde verilir (hook dokümantasyonuna yazıldı).
- [x] **5.3** **Kapsam düzeltmesi:** plandaki hedef listesi Explore taramasının yanlış sınıflandırmasıymış — `DriftPanel.resolving` in-flight işlem takibi, `ModelingPalette.excludedSchemas` şema filtresi, `CompositeDetailPanel.usedAliases` alias takibi; hiçbiri bulk-selection değil (dokunulmadı). Gerçek bulk-selection ekranları:
  - `admin/RolesPanel.tsx` ✓ geçirildi: elle `Set` yönetimi (`togglePermission`, `toggleResourceGroup`, dirty döngüsü, indeterminate hesabı) → `useRowSelection` + `sameIdSet`/`selectionStateFor`. Bonus: inline hata div'i Faz 2 standardı `ErrorAlert`'e geçti.
  - `GlossaryEnrichPanel.tsx` **istisna:** seçim kaydı `{selected, value}` birleşik (checkbox + input değeri tek kayıtta, parent-state'te) — Set tabanlı modele zorlamak davranış riski; mevcut yapısıyla bırakıldı.

Kabul doğrulaması: `make check-frontend` zinciri yeşil (31 dosya / 148 test — +7 selection testi). OQ-4 kapatıldı.

### Faz 6 — Formatter konsolidasyonu

- [ ] **6.1** `frontend/src/utils/format/` altında birleştir: `formatters.ts` (genel) + `resultCellFormat.ts` (tablo hücresi) → tek giriş noktası, Intl instance cache ortaklaşır. Eski dosyalar re-export ile korunur (import kırılmaz), bir sonraki adımda import'lar güncellenip eski dosyalar silinir (knip yakalar).
- [ ] **6.2** Ekranlardaki inline `toLocaleString`/tarih formatlama çağrıları taranır (`grep -rn 'toLocaleString\|toLocaleDateString' frontend/src/components`) ve `format/` fonksiyonlarına bağlanır; `locale` prop drilling kalkar.
- [ ] **6.3** Mevcut `formatters.test.ts` + `resultCellFormat.test.ts` taşınır ve genişletilir (TR/EN locale snapshot'ları).

### Faz 7 — Frontend geneli tamamlayıcı standardizasyon (table dışı)

Bu faz bağımsız iş paketleri halindedir; herhangi bir sırayla yapılabilir:

- [ ] **7.1 Button:** `frontend/src/components/ui/Button.tsx` (`variant`, `size`, `loading` prop'ları; mevcut `btn btn-*` sınıflarını **aynen** üretir — CSS değişmez). 171 kullanım fırsatçı migrasyonla geçer; yeni kod için zorunlu kural `tasks/lessons.md`'ye yazılır.
- [ ] **7.2 FormField:** `frontend/src/components/ui/FormField.tsx` (label + input + hata + `aria-describedby`/unique id). Pilot: `admin/` formlarından biri.
- [ ] **7.3 Modal konsolidasyonu:** elle yazılmış overlay'ler (`TableBrowserRowModal.tsx`, `SampleDataModal.tsx` + Faz başında grep ile doğrulanacak tam liste) `ui/Modal.tsx`'e taşınır — focus-trap/Esc/aria davranışı bedavaya gelir.
- [ ] **7.4 i18n temizliği:** `t: TFunction` prop'u alan 10+ dosya `useT()`'ye geçer (`grep -rln 't: TFunction' frontend/src/components`); `ui/Pagination.tsx`'in `table_browser.*` anahtarları `common.pagination.*`'a taşınır (eski anahtarlar TR/EN locale dosyalarında alias olarak tutulur ya da tek PR'da hepsi güncellenir).
- [ ] **7.5 Stil temizliği:** `ui/Pagination.tsx` ve `ActiveUsersTab.tsx`'teki inline `style={{...}}` blokları ilgili CSS dosyalarına (BEM) taşınır. `index.css` (84 KB) bölme işi ayrı bir plan konusudur — bu planda kapsam dışı.
- [ ] **7.6 URL state:** sayfalı admin ekranlarında `page`/filtre değerlerinin `useQueryParam` ile URL'e yazılması (davranış EKLEMEdir → OQ-2 ürün kararı; teknik altyapı `usePaginatedList`'e `syncToUrl?: string` opsiyonu olarak eklenir).

### Faz sıralaması ve bağımlılıklar

```text
Faz 0 ──> Faz 1 ──> Faz 2 ──> Faz 3 ──> Faz 4, Faz 5
Faz 6 (bağımsız)        Faz 7 (bağımsız, alt maddeleri de bağımsız)
```

---

## 5. Korunacak Davranışlar / Risk Listesi

| Risk | Önlem |
| --- | --- |
| `totalPages=0` vs `=1` farkı ekranlar arasında değişiyor (§2.2) | Faz 0.1'de tek politika seç (OQ-1), karakterizasyon testleriyle kilitle; `Pagination.tsx` zaten `Math.max(1, totalPages)` ile savunmalı |
| `AuditLogPanel` filter-on-submit semantiği (yazarken fetch ETMEZ) | `usePaginatedList.deps`'e filtreler verilmez; submit → `reload()` çağrılır — semantik korunur |
| `shares: null` dönebilen endpoint (`api/admin.ts:625`) | Adapter map'inde `?? []` — mevcut guard taşınır |
| Table Browser'ın offset/limit + `hasNext` modeli | **Kapsam dışı bırakıldı** — `useTableBrowserPage`/`useTableBrowserQueryState` testli ve özel; zorla ortaklaştırılmaz |
| `ResultTable` performansı (büyük sonuç setleri) | Kapsam dışı; `DataTable` admin/list ölçeği içindir (≤ ~100 satır/sayfa) |
| Knip unused-export hatası | Yeni shared modül + ilk tüketici aynı PR'da |
| ESLint `--max-warnings 0`, `react-hooks` v7 (`set-state-in-effect`) | `usePaginatedList` tasarımı effect-içi-setState'i tek noktaya indirir; mevcut `eslint-disable`'lar migrasyonla **silinir**, yenisi eklenmez |
| i18n anahtar taşıma (7.4) | TR ve EN locale dosyaları aynı PR'da güncellenir; eksik anahtar runtime'da görünür — manuel TR/EN smoke test şart |
| Görsel regresyon | CSS sınıfları korunur; her ekran PR'ında önce/sonra ekran görüntüsü PR açıklamasına eklenir |

---

## 6. Assumptions & Open Questions

**Assumptions**

- A-1: Backend admin list endpoint'leri 1-based `page` + `pageSize`/`page_size` query parametresi alır ve `{key: T[], total}` döner (frontend kullanımından çıkarıldı, §2.3; Go handler'ları ayrıca doğrulanmadı).
- A-2: "137 ham input / ~12 hand-rolled modal / 26 buton sınıfı varyantı" sayıları Explore ajanı grep'lerinden gelir; ilgili fazların başında yeniden sayılacak.
- A-3: `AIUsageAdminPanel` dışında client-side dilimleme yapan başka sayfalı ekran yok.
- A-4: `QueryHistory` pagination'ı server-side'dır (fetch deps'inde `pageSize` var; satır 78-95).

**Open Questions**

- OQ-1: `total=0` iken `totalPages` 0 mı 1 mi olmalı? (Öneri: `Math.max(1, ...)` — `Pagination` bileşeni zaten böyle savunuyor; ürün açısından fark yok ama tek politika seçilmeli.)
- OQ-2: Sayfa/filtre state'i URL'e yazılsın mı (deep-link + refresh dayanıklılığı)? Davranış eklemesidir; ürün onayı gerekir. (Faz 7.6)
- ~~OQ-3~~ **KAPATILDI (Faz 4.2):** Sıralama yalnızca client-side listelerde açıldı (`ConfirmedQueriesPanel`); server-side sayfalı tablolara backend desteği olmadan sıralama EKLENMEYECEK (sayfa-içi sıralama yanıltıcı). Debounce'suz inputlara yeni debounce eklenmedi; mevcut 3 debounce ortak hook'a taşındı.
- ~~OQ-4~~ **KAPATILDI (Faz 5.2):** `useRowSelection` seçim setini sayfa/filtre değişiminde kendisi sıfırlamaz; koruma/sıfırlama politikası ekran-bazlı karardır ve sayfa state'inin yaşadığı yerde verilir. (Bugün sayfalı + bulk-selection'lı ekran yok; ilk ortaya çıktığında bu kural uygulanır.)
- OQ-5: Component render testi için `@testing-library/react` + `jsdom` devDependency olarak eklensin mi, yoksa repo'nun mevcut "logic-extraction + pure test" pattern'i yeterli mi? (Plan, mevcut pattern'le ilerleyecek şekilde yazıldı; render testi eklenirse Faz 0 genişler.)

---

## 7. Başarı Kriterleri

1. Grup A'daki 12 sayfalı ekranın tamamı `usePaginatedList` (veya `useClientPagination`) + `DataState` + `Pagination` üçlüsünü kullanıyor; elle kurulan `currentPage/totalItems/loading/error` state bloğu **sıfır**.
2. `getTotalPages` tek implementasyon; `Math.ceil` kopyası kalmadı (`grep -rn 'Math.ceil(total' frontend/src/components` boş döner).
3. Admin tablolarında inline empty/error/loading JSX'i yok; `EmptyState`/`ErrorAlert`/`LoadingOverlay` kompozisyonu `DataState` üzerinden.
4. En az 5 admin tablosu `DataTable<T>` ile render ediliyor; kolonlar `ColumnDef` config'i.
5. Backend'e giden istekler (URL, method, query/body parametreleri) migrasyon öncesi/sonrası birebir aynı — her ekran PR'ında Network kaydıyla doğrulandı.
6. `make check-frontend` (lint + format + knip + test + build) her fazda yeşil; test sayısı artmış durumda (paging, sort, selection, pagination-tokens testleri).
7. `eslint-disable` sayısı azaldı (özellikle `AuditLogPanel.tsx`'teki `exhaustive-deps`/`set-state-in-effect`).

## 8. Manuel Doğrulama Listesi (her ekran migrasyonunda)

### 8.1 Faz 0.3 — Pilot ekranların mevcut davranış notları (migrasyon sonrası birebir korunacak)

**Pilot 1: `frontend/src/components/admin/DatasourceAccessPanel.tsx`** (kaynak okuması: 2026-06-12)

- `pageSize` sabit **10**; `totalPages = Math.ceil(totalItems / 10)` (çıplak ceil, satır 47) → `total=0`'da `totalPages=0`, ama `Pagination` `alwaysShow` prop'uyla çağrılıyor (satır 246) ve içerideki `Math.max(1, totalPages)` sayesinde "1" gösteriyor.
- Fetch: `reload` callback'i `useEffect`'ten (`token`/`currentPage` değişiminde) çağrılıyor; `eslint-disable react-hooks/set-state-in-effect` mevcut (satır 74).
- Boş durum: tbody içinde `colSpan={5}`, loading'de boş string, değilse `'—'` (satır 192-197) — `EmptyState` kullanılmıyor.
- Loading: `LoadingOverlay` sarmalı + boş listede `minHeight: 120` hack'i (satır 176).
- Hata: form altında `admin-err-text` div'i, `t('common.error')` prefix'li (satır 166-170).
- Aksiyon davranışları: **grant** → form reset + `setCurrentPage(1)` + `reload()`; **revoke** → `useConfirm` (danger) sonrası `setCurrentPage(1)` + `reload()`; **level değişimi** → sadece `reload()` (sayfa korunur). Zaten 1. sayfadayken `setCurrentPage(1)` no-op olduğundan `reload()` closure'daki sayfayla çalışır — migrasyonda bu sıra korunmalı.
- Tarih: `new Date(r.granted_at).toLocaleString(localeLanguageTag(locale))` (satır 219) — locale'e duyarlı.
- RBAC: `datasource:grant_access` yoksa `ReadOnlyNote` + tüm girdiler `disabled`.
- Lookup'lar: `useAdminLookups(token)` → user/datasource id→isim eşlemesi `Array.find` ile satır başına (satır 200-201).

**Pilot 2: `frontend/src/components/sharing/SharedResourcesList.tsx`** (kaynak okuması: 2026-06-12)

- `pageSize` sabit **10**; çıplak `Math.ceil` (satır 30); `Pagination` **yalnızca** `displayedItems.length > 0` iken render ediliyor (satır 173, `alwaysShow` yok) → boş listede pagination hiç görünmez.
- `resourceType` prop'u değişince ayrı bir effect ile `setCurrentPage(1)` (satır 59-62); `refreshKey` prop'u parent'tan reload tetikler (satır 57).
- Boş durum: `shared-list__empty` paragrafı, loading'de boş string (satır 109-115).
- Hata: üstte `shared-list__error` div'i (satır 98) — `t('common.error')` prefix'i YOK (DatasourceAccessPanel'den farklı).
- Tarih: `new Date(share.created_at).toLocaleDateString()` — **locale parametresiz**, tarayıcı locale'i (satır 155); DatasourceAccessPanel'den farklı. Migrasyonda aynen korunacak (locale düzeltmesi Faz 6.2 işi).
- `accessToken` yokken `load` erken döner (satır 34-36) ve başlangıç `loading=true` olduğu için overlay açık kalır — mevcut davranış böyle; migrasyonda değiştirilmeyecek (düzeltme istenirse ayrı karar).
- Revoke: `useConfirm` (danger) sonrası `deleteShare` + `reload()` — sayfa **korunur** (`setCurrentPage(1)` yok; DatasourceAccessPanel'den farklı).

- [ ] İlk yükleme: loading göstergesi → veri.
- [ ] Sayfa ileri/geri/ilk/son; sayfa numarası penceresi (7-slot) doğru.
- [ ] Sayfa boyutu değişimi (varsa) → sayfa 1'e döner.
- [ ] Filtre/arama değişimi → sayfa 1'e döner (veya submit semantiği korunur — AuditLog).
- [ ] Boş sonuç: EmptyState; hata: ErrorAlert + retry yolu.
- [ ] Satır aksiyonları (ve varsa bulk seçim) çalışıyor; silmede ConfirmDialog.
- [ ] TR ve EN locale'de metinler tam (anahtar kaçağı yok).
- [ ] Network: istek URL/parametreleri eskiyle birebir aynı.
