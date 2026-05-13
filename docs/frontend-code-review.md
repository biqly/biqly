# Frontend Code Review — Biqly

> Tarih: 2026-05-13  
> Kapsam: `frontend/src/` altındaki tum kaynak dosyalar  
> Stack: React 19 + Vite 6 + TypeScript 5.7 + Recharts 2.15

---

## Icerik

1. [Proje Ozeti](#1-proje-ozeti)
2. [Kritik Sorunlar](#2-kritik-sorunlar)
3. [Tip Tekrarlari (Type Duplication)](#3-tip-tekrarlari-type-duplication)
4. [Kod Tekrari (DRY Ihlalleri)](#4-kod-tekrari-dry-ihlalleri)
5. [Bilesen Boyutu ve Karmasiklik](#5-bilesen-boyutu-ve-karmasiklik)
6. [Inline Style Kullanimi](#6-inline-style-kullanimi)
7. [Anti-Patternler](#7-anti-patternler)
8. [Eksik En Iyi Uygulamalar (Missing Best Practices)](#8-eksik-en-iyi-uygulamalar)
9. [Oncelikli Aksiyon Plani](#9-oncelikli-aksiyon-plani)
10. [Dosya Bazli Ozet](#10-dosya-bazli-ozet)

---

## 1. Proje Ozeti

| Metrik | Deger |
|---|---|
| Bilesen dosyalari | 11 |
| Hook dosyalari | 4 |
| Tip dosyalari | 4 |
| Yardimci dosyalar | 1 |
| Toplam kaynak satiri | ~3,800+ |
| CSS (tek dosya) | ~2,940 satir (~65KB) |
| Bagimliliklar | `react`, `react-dom`, `recharts` (3 adet — cok minimal) |
| Routing kutuphanesi | Yok (ozel pushState implementasyonu) |
| State yonetimi | Yok (useState + localStorage) |
| UI kutuphanesi | Yok (tum UI sifirdan yazilmis) |

**Genel Degerlendirme:** Proje cok az bagimlilikla calisan fonksiyonel bir uygulama. Ancak hizli gelistirme surecinde teknik borc birikmis: tip tekrarlari, dev bilesenler, inline style daginikligi ve tutarsiz veri cekme paternleri onemli sorunlar arasinda.

---

## 2. Kritik Sorunlar

### 2.1 Metadata.tsx — 1,124 Satirlik Tek Dosya Monoliti

**Dosya:** `components/Metadata.tsx`

20+ state degiskeni, 2 modal, inline duzenleme, bulk islem ve tablo listesi yonetimi tek bir bilesende. Bu dosya en az 5 alt bilesene bolunmeli:

```
Metadata.tsx → MetadataBrowser/
              ├── MetadataTableList.tsx
              ├── ColumnEditor.tsx
              ├── InlineDescriptionEditor.tsx
              ├── DescribeModal.tsx
              └── BulkDescribeModal.tsx
```

### 2.2 Tip Tekrari — `types/ai.ts` ve `types/query.ts`

Iki dosya arasinda tamamen ayni veya neredeyse ayni olan **9 interface** tekrar edilmis:

| Interface | `types/ai.ts` | `types/query.ts` | Fark |
|---|---|---|---|
| `SelectField` | Var | Var | Ayni |
| `FilterClause` | Var | Var | Ayni |
| `GroupByField` | Var | Var | query.ts'de `time_grain` yok |
| `OrderByField` | Var | Var | Ayni |
| `WindowSpec` | Var | Var | Ayni |
| `CTE` | Var | Var | query.ts'de query tipi `Record<string, unknown>` |
| `WindowFunction` | Var | Var | query.ts'de daha az union |
| `QueryColumn` | Var | Var | ai.ts'de ek `semantic_type`, `format` alanlari |
| `CompiledQuery` | Var | Var | ai.ts'de yok, query.ts'de `execution_plan` var |

**Cozum:** `types/query.ts` temel tipleri icermeli, `types/ai.ts` onlari `import` edip genisletmeli.

### 2.3 `Datasource` Interface 3 Dosyada Tekrar

| Dosya | Satir |
|---|---|
| `components/Datasources.tsx:5-12` | `interface Datasource { id, name, type, created_at?, updated_at? }` |
| `components/Metadata.tsx:110-114` | `interface Datasource { id, name, type }` (alt kume) |
| `components/QueryBuilder.tsx:9-13` | `interface Datasource { id, name, type }` (alt kume) |

`types/metadata.ts` dosyasinda zaten bir `Datasource` interface'i var. Tum bilesenler bu dosyadan import yapmali.

---

## 3. Tip Tekrarlari (Type Duplication)

### Sorun 3.1: Ayni Isimde Farkli Dosyalarda Tanimlanan Tipler

```typescript
// types/ai.ts — SelectField (16 satir)
export interface SelectField {
  type: 'dimension' | 'metric' | 'window'
  name: string
  alias?: string
  window?: WindowSpec
}

// types/query.ts — SelectField (ayni icerik)
export interface SelectField {
  type: 'dimension' | 'metric' | 'window'
  name: string
  alias?: string
  window?: WindowSpec
}
```

Bu, `QueryBuilder.tsx` ve `AIQuery.tsx` gibi bilesenlerde yanlis import yapilmasina neden olabilir.

### Oneri

```typescript
// types/query.ts — temel tipler
export interface SelectField { ... }
export interface FilterClause { ... }
export interface OrderByField { ... }
// ...

// types/ai.ts — AI'ye ozel genisletmeler
import type { SelectField, FilterClause } from './query'
export type { SelectField, FilterClause }
export interface AIQueryResponse { ... }  // AI'ye ozel
```

### Sorun 3.2: `LogicalQuery` ile `LogicalQueryPayload` Ayri Tipler

`types/ai.ts`'deki `LogicalQuery` ile `types/query.ts`'deki `LogicalQueryPayload` neredeyse ayni. Tek bir temel tip tanimlanmali.

---

## 4. Kod Tekrari (DRY Ihlalleri)

### 4.1 `COLORS` Sabiti — 3 Dosyada Tekrar

| Dosya | Satir | Uzunluk |
|---|---|---|
| `components/Dashboard.tsx:6` | 7 renk |
| `components/Evaluation.tsx:104` | 4 renk |
| `components/QueryBuilder.tsx:142` | 8 renk |
| `components/AIQuery.tsx:17` | 8 renk |

**Cozum:**

```typescript
// utils/constants.ts
export const CHART_COLORS = [
  '#3b82f6', '#22c55e', '#f59e0b', '#ef4444',
  '#8b5cf6', '#ec4899', '#14b8a6', '#f97316',
] as const
```

### 4.2 Chart Yapilandirma Tekrari — 10+ Kez

Her chart bileseni ayni 4 satiri iceriyor:

```tsx
<CartesianGrid strokeDasharray="3 3" stroke="#475569" />
<XAxis dataKey="name" stroke="#94a3b8" />
<YAxis stroke="#94a3b8" />
<Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
```

**Dosyalar:** Dashboard.tsx (6x), Evaluation.tsx (3x), QueryBuilder.tsx (3x), AIQuery.tsx (3x)

**Cozum:**

```typescript
// utils/chartConfig.ts
export const DARK_CHART_STYLE = {
  cartesianGrid: { strokeDasharray: '3 3', stroke: '#475569' },
  axis: { stroke: '#94a3b8' },
  tooltip: { background: '#1e293b', border: '1px solid #475569' },
} as const

// Kullanim:
<CartesianGrid {...DARK_CHART_STYLE.cartesianGrid} />
<XAxis dataKey="name" {...DARK_CHART_STYLE.axis} />
<YAxis {...DARK_CHART_STYLE.axis} />
<Tooltip contentStyle={DARK_CHART_STYLE.tooltip} />
```

### 4.3 Modal Yapisi — 3 Kez Tekrar

| Dosya | Satir Araligi |
|---|---|
| `FewShotExamples.tsx` | 158-197 |
| `Metadata.tsx` (Describe) | 709-937 |
| `Metadata.tsx` (Bulk) | 939-1121 |

Her biri ayni yapida:

```tsx
<div className="modal-backdrop" onClick={close}>
  <div className="modal-card" onClick={e => e.stopPropagation()}>
    <div className="modal-header">
      <h2>Başlık</h2>
      <button className="modal-close">×</button>
    </div>
    <div className="modal-body">...</div>
    <div className="modal-actions">...</div>
  </div>
</div>
```

**Cozum:** Yeniden kullanilabilir `<Modal>` bileseni:

```typescript
// components/ui/Modal.tsx
interface ModalProps {
  open: boolean
  onClose: () => void
  title: string
  children: React.ReactNode
  footer?: React.ReactNode
}
export function Modal({ open, onClose, title, children, footer }: ModalProps) { ... }
```

### 4.4 Inline Duzenleme Alani — 2 Kez Tekrar (Metadata.tsx)

Tablo aciklamasi ve kolon aciklamasi icin inline textarea duzenleme (her biri ~20 satir JSX) ayni `onBlur` / `onKeyDown` handler'larini iceriyor.

**Cozum:** `<InlineEdit>` bileseni:

```typescript
interface InlineEditProps {
  value: string
  onSave: (value: string) => void
  placeholder?: string
  rows?: number
}
```

### 4.5 KPI / Metrik Karti — 4 Farkli Implementasyon

| Dosya | Satir | Yontem |
|---|---|---|
| Dashboard.tsx:61-72 | Inline style grid |
| Dashboard.tsx:213-229 | Inline style KPI |
| Evaluation.tsx:108-115 | CSS siniflari (`kpi-card`) |
| Evaluation.tsx:491-507 | Inline `kpi-card` (KPICard kullanmiyor) |

**Cozum:** `<KPICard>` bilesenini `components/ui/` altina tasi ve her yerde kullan.

### 4.6 Rate/Renk Mantigi — 2 Dosyada Ayni

```typescript
// Dashboard.tsx:319 ve Evaluation.tsx:468
color: rate >= 80 ? 'var(--success)' : rate >= 50 ? 'var(--warning)' : 'var(--error)'
```

**Cozum:** `utils/formatters.ts` altinda:

```typescript
export function getRateColor(rate: number): string {
  if (rate >= 80) return 'var(--success)'
  if (rate >= 50) return 'var(--warning)'
  return 'var(--error)'
}
```

### 4.7 Tag Badge — 2 Dosyada Benzer

FewShotExamples.tsx ve SavedQuestions.tsx ayni tag badge rendering'ini farkli inline stillerle yapiyor.

**Cozum:** `<TagBadge>` bileseni veya `.tag-badge` CSS sinifi.

### 4.8 Dinamik Satir Yonetimi — QueryBuilder.tsx (6x Tekrar)

`addX` / `updateX` / `removeX` paterni 6 farkli veri yapisi icin tekrarlanmis:

- `addSelectItem` / `updateSelectItem` / `removeSelectItem`
- `addFilter` / `updateFilter` / `removeFilter`
- `addGroupByRow` / `updateGroupByRow` / `removeGroupByRow`
- `addHaving` / `updateHaving` / `removeHaving`
- `addWindowFunc` / `updateWindowFunc` / `removeWindowFunc`
- `addCTE` / `updateCTE` / `removeCTE`

**Cozum:** Generic hook:

```typescript
// hooks/useArrayState.ts
export function useArrayState<T>(initial: T[]) {
  const [items, setItems] = useState<T[]>(initial)
  const add = (item: T) => setItems(prev => [...prev, item])
  const update = (idx: number, item: T) => setItems(prev => prev.map((x, i) => i === idx ? item : x))
  const remove = (idx: number) => setItems(prev => prev.filter((_, i) => i !== idx))
  return { items, setItems, add, update, remove }
}
```

### 4.9 `requestBody()` Fonksiyonu Gereksiz — AIQuery.tsx

`requestBody()` fonksiyonu tanimlanmis ama hic kullanilmiyor. Ayni icerik `sendQuery` icinde `body` olarak manuel olarak olusturulmus.

```typescript
// Satir ~195: tanimlanmis ama kullanilmamis
const requestBody = () => ({ datasource_id: datasourceId, question, ... })

// Satir ~205: sendQuery icinde ayni body manuel olusturulmus
const body = { datasource_id: datasourceId, question: q, ... }
```

**Cozum:** `requestBody()` fonksiyonunu kullan veya tamamen kaldir.

---

## 5. Bilesen Boyutu ve Karmasiklik

| Dosya | Satir | State Sayisi | Degerlendirme |
|---|---|---|---|
| Metadata.tsx | 1,124 | 20+ | **Kritik** — BOLUNMELI |
| Evaluation.tsx | 662 | 12 | **Yuksek** — Alt bilesenlere ayrilmali |
| QueryBuilder.tsx | 711 | 19 | **Yuksek** — Form alanlari alt bilesen olmali |
| AIQuery.tsx | ~500+ | ~15 | **Yuksek** — Sub-component'ler zaten var ama cok iceride |
| Dashboard.tsx | 356 | 6 | **Orta** — Sub-component'ler ayni dosyada |
| SavedQuestions.tsx | 207 | 4 | Kabul edilebilir |
| FewShotExamples.tsx | 200 | 9 | Kabul edilebilir |
| Datasources.tsx | 173 | 5 | Iyi |
| ResultTable.tsx | 179 | 3 | **Iyi** — En temiz bilesen |
| Select.tsx | ~170 | 5 | Iyi |
| ModelBadgeRow.tsx | ~80 | 0 | Iyi |

**En Iyi Uygulama (Best Practice):** Bir bilesen dosyasi **200-300 satiri** gecmemeli. Bir bilesende **5'ten fazla** state degiskeni varsa alt bilesenlere bolunmeli.

---

## 6. Inline Style Kullanimi

### Ozet

Toplamda **100+ inline style** kullanimi tespit edildi. Bunlarin buyuk cogunlugu CSS siniflarina donusturulebilir.

### En Sik Tekrarlanan Inline Style'lar

| Pattern | Tekrar Sayisi | Dosyalar | Oneri |
|---|---|---|---|
| `fontSize: '0.8rem', color: 'var(--text-secondary)'` | 15+ | Dashboard, Evaluation, AIQuery | `.text-secondary-sm` CSS sinifi |
| `display: 'grid', gridTemplateColumns: '...'` | 8+ | Dashboard, Evaluation, SavedQuestions | `.grid-2`, `.grid-3` CSS siniflari |
| `textOverflow: 'ellipsis', whiteSpace: 'nowrap', overflow: 'hidden'` | 4+ | FewShotExamples, Evaluation, SavedQuestions | `.truncate` CSS sinifi |
| `fontWeight: 700, fontSize: '2rem'` | 5+ | Dashboard, Evaluation | `.kpi-value` CSS sinifi |
| Tooltip `contentStyle` objesi | 10+ | Dashboard, Evaluation, QueryBuilder, AIQuery | Sabit objeye cik |
| Modal `position: 'fixed', zIndex: ...` | 3+ | FewShotExamples, Metadata | Modal bileseni |

### Inline Style Problemleri

1. **Surudulemez (Not draggable):** CSS siniflari ile tema degisikligi tek yerden yapilabilir; inline style'lar her dosyada guncellenmeli.
2. **Performans:** Her render'da yeni obje olusturulur, React'in diff algoritmasini etkiler.
3. **Bakim zorlugu:** Ayni stil 10 farkli dosyada tekrar edildiginde degisiklik yapmak cok zor.

---

## 7. Anti-Patternler

### 7.1 `useApi` Hook'unun `loading`/`error` Degerlerinin Tutarsiz Kullanimi

Bazi bilesenler `useApi`'nin `loading` ve `error` degerlerini kullanirken, bazilari manuel `useState(true)` ile kendi loading state'lerini yonetiyor:

| Yontem | Dosyalar |
|---|---|
| useApi'nin loading/error'unu kullanma | Datasources, QueryBuilder, Metadata |
| Manuel useState(true) + sadece `get` kullanimi | Dashboard (AIUsageSection, ModelSuccessRates) |
| Tamamen farkli loading yonetimi | Evaluation (useAdminApi + useStreamingApi) |

**Oneri:** Standart bir `useFetch` veya `useResource` paterni benimsenmeli.

### 7.2 Raw `fetch` Kullanimi — Metadata.tsx

```typescript
// Metadata.tsx:423-448
const res = await fetch(url, { method: 'POST', ... })
```

`useApi` hook'u varken raw `fetch` kullaniliyor. Bu da:
- Timeout yonetimi yok
- Hata formati farkli
- Loading state yok

**Cozum:** `useApi`'nin `postData` metodunu `timeout` opsiyonu ile kullan.

### 7.3 `eslint-disable-line react-hooks/exhaustive-deps` — 9 Kez

Hemen tum `useEffect`'lerde `get` fonksiyonu dependency array'e eklenmiyor ve eslint baskaldiriliyor:

```typescript
useEffect(() => {
  get<Datasource[]>('/api/datasources').then(...)
}, []) // eslint-disable-line react-hooks/exhaustive-deps
```

**Neden sorun:** `useApi`'nin `get` fonksiyonu `useCallback` ile sarmalanmis ama dependency olarak `[call]` iceriyor, `call` ise `[]` dependency'sine sahip. Yani `get` stabil bir referans olmali.

**Cozum:** Ya `get`'i dependency array'e ekleyin (güvenli cunku memoized), ya da `useApi`'den gelen degerleri stabilize etmek icin `useRef` paterni kullanin.

### 7.4 `any` Tipi Kullanimi — 4 Dosyada

| Dosya | Satir | Kullanim |
|---|---|---|
| Dashboard.tsx | 97 | `(_: any, i: number)` PieChart |
| QueryBuilder.tsx | 218, 351 | `result: any`, chart data |
| AIQuery.tsx |多处 | `sample.columns`, chart data |
| SavedQuestions.tsx | 9 | `logical_query: any` |

**Oneri:** Tum `any` kullanimlari uygun tiplerle degistirilmeli.

### 7.5 `useAdminApi` Gereksiz Tekrar — useApi.ts

`useAdminApi`, `useApi`'yi sarmalayip her metodda `adminHeaders`'i tekrar ekliyor. Her metodda ayni `headers` birlestirme mantigi tekrarlanmis:

```typescript
const get = useCallback(
  <T = any>(url: string, options?: RequestOptions) =>
    api.get<T>(url, { ...options, headers: { ...adminHeaders, ...options?.headers } }),
  [api],
)
// postData, putData, patchData, deleteData icin ayni tekrar...
```

**Cozum:** Tek bir wrapper fonksiyon:

```typescript
export function useAdminApi() {
  const api = useApi()
  const withAuth = useCallback(
    <T>(fn: (url: string, body?: unknown, opts?: RequestOptions) => Promise<T | null>) =>
      (url: string, bodyOrOpts?: unknown, opts?: RequestOptions) => {
        // headers inject logic
      },
    [api]
  )
  return { ...api, /* overridden methods */ }
}
```

Ya da `useApi`'ye bir `defaultHeaders` opsiyonu ekleyerek bu tekrari onleyin.

### 7.6 `requestBody()` Fonksiyonu Olusturulmus Ama Kullanilmamis — AIQuery.tsx

```typescript
// Olusturulmus ama hicbir yerde cagirilmamis
const requestBody = () => ({
  datasource_id: datasourceId,
  question,
  tables: autoTableRouting ? undefined : selectedTables,
  ...
})

// sendQuery icinde ayni body manuel tekrar olusturulmus
const sendQuery = async (q: string, execute: boolean) => {
  const body = {
    datasource_id: datasourceId,
    question: q,
    tables: autoTableRouting ? undefined : selectedTables,
    ...
  }
}
```

---

## 8. Eksik En Iyi Uygulamalar (Missing Best Practices)

### 8.1 Error Boundary Yok

Uygulamanin hicbir yerinde React Error Boundary yok. Bir bilesen hatasinda tum uygulama coker.

**Oneri:**

```typescript
// components/ui/ErrorBoundary.tsx
export class ErrorBoundary extends React.Component<...> { ... }
```

### 8.2 Loading / Error State'leri Tutarsiz

Her bilesen loading ve error state'lerini farkli sekillerde gosteriyor. Bazi bilesenler hic error gostermiyor.

**Oneri:** `<LoadingSpinner>`, `<ErrorMessage>`, `<EmptyState>` gibi ortak UI bilesenleri olusturun.

### 8.3 Custom Router — Robustluk Eksikligi

`App.tsx`'te ozel bir routing implementasyonu var (pushState / replaceState). Bu:
- Nested route'lari desteklemiyor
- Route guard'lari yok
- 404 handling minimal
- Deep linking sorunlari olabilir

**Oneri:** `react-router` veya `wouter` gibi hafif bir routing kutuphanesi ekleyin. (Proje 3 bagimlilikla basladigi icin `wouter` — ~1.5KB — daha uygun.)

### 8.4 State Yonetimi Eksikligi

Conversation state `localStorage` + `useState` ile yonetiliyor. Birden fazla bilesen ayni veri kaynagini (`datasources`) bagimsiz olarak fetch ediyor.

**Oneri:** Ya `React Context` + `useReducer` ile global state yonetimi kurun, ya da `zustand` (~1KB) gibi minimal bir kutuphane ekleyin.

### 8.5 CSS Dosyasi Cok Buyuk (~65KB, ~2,940 Satir)

Tek bir `index.css` dosyasi tum uygulamayi kapsiyor. Bilesen bazli CSS (CSS Modules, veya en azindan bilesen bazli CSS dosyalari) tercih edilmeli.

**Oneri:**
- Kisa vadede: `index.css`'i mantiksal bolumlere ayirin (`base.css`, `components.css`, `utilities.css`)
- Orta vadede: CSS Modules veya Tailwind CSS gecisi degerlendirin

### 8.6 Test Yok

`frontend/` dizininde hicbir test dosyasi yok. `package.json`'da test script'i tanimlanmamis.

**Oneri:** Vitest + React Testing Library ile en azindan hook'lar (`useApi`, `useConversation`) icin birim testler yazin.

### 8.7 Accessibility (Erisilebilirlik) Iyilestirmeleri

- Bazı bilesenlerde `aria-*` attribute'leri eksik
- Modal'lar icin focus trap yok
- `Select` bileseni iyi erisilebilirlik uygulamalarina sahip (ileriye ornek)
- Keyboard navigation bazi yerlerde eksik

### 8.8 Environment Variable Dogrulama Yok

`VITE_BI_ADMIN_API_KEY` gibi env variable'lar dogrulanmadan kullariliyor. Baslangicta bir kontrol mekanizmasi olmali.

---

## 9. Oncelikli Aksiyon Plani

### Faz 1 — Hizli Duzeltmeler (1-2 Gun)

| # | Gorev | Dosya | Etki |
|---|---|---|---|
| 1 | `Datasource` interface'ini `types/metadata.ts`'den import et | 3 dosya | Tip guvenligi |
| 2 | `COLORS` sabitini `utils/constants.ts`'e cikar | 4 dosya | DRY |
| 3 | `requestBody()` olusturulmus ama kullanilmamis — kullan veya sil | AIQuery.tsx | Dead code |
| 4 | Chart tooltip/axis sabitlerini `utils/chartConfig.ts`'e cikar | 4 dosya, 10+ yer | DRY |
| 5 | Rate-renk fonksiyonunu `utils/formatters.ts`'e cikar | 2 dosya | DRY |

### Faz 2 — Yeniden Kullanilabilir Bilesenler (2-3 Gun)

| # | Gorev | Dosya | Etki |
|---|---|---|---|
| 6 | `<Modal>` bileseni olustur | `ui/Modal.tsx` | ~200 satir tasarruf |
| 7 | `<KPICard>` bilesenini standardlastir | `ui/KPICard.tsx` | Tutarlilik |
| 8 | `<InlineEdit>` bileseni olustur | `ui/InlineEdit.tsx` | Metadata.tsx'de ~40 satir tasarruf |
| 9 | `<TagBadge>` bileseni olustur | `ui/TagBadge.tsx` | 2 dosya |
| 10 | `useArrayState<T>()` hook'u olustur | `hooks/useArrayState.ts` | QueryBuilder.tsx'de ~60 satir tasarruf |

### Faz 3 — Bilesen Bolme (3-5 Gun)

| # | Gorev | Dosya | Etki |
|---|---|---|---|
| 11 | Metadata.tsx'i 5 alt bilesene bol | Metadata.tsx | Okunabilirlik, test edilebilirlik |
| 12 | Evaluation.tsx'i 3 alt bilesene bol | Evaluation.tsx | Okunabilirlik |
| 13 | QueryBuilder.tsx'te form alanlarini alt bilesen yap | QueryBuilder.tsx | Okunabilirlik |
| 14 | AIQuery.tsx'teki sub-component'leri ayri dosyalara tasi | AIQuery.tsx | Modulerlik |

### Faz 4 — Altyapi Iyilestirmeleri (3-5 Gun)

| # | Gorev | Etki |
|---|---|---|
| 15 | `types/query.ts` ve `types/ai.ts` tip tekrarlarini gider | Tip guvenligi |
| 16 | `useAdminApi` tekrarini gider | DRY |
| 17 | `eslint-disable-line` yorumlarini gider | Kod kalitesi |
| 18 | `any` tip kullanimlarini gider | Tip guvenligi |
| 19 | Error Boundary ekle | Hata toleransi |
| 20 | Vitest + test alt yapisi kur | Guvenilirlik |

---

## 10. Dosya Bazli Ozet

### Bilesenler

| Dosya | Satir | State | Sorunlar |
|---|---|---|---|
| `AIQuery.tsx` | ~500+ | ~15 | Buyuk dosya, inline chart'lar, `requestBody()` kullanilmamis |
| `Dashboard.tsx` | 356 | 6 | Inline style agirlikli, `COLORS` tekrari, `any` tip |
| `Datasources.tsx` | 173 | 5 | En temiz CRUD bileseni, `Datasource` interface tekrari |
| `Evaluation.tsx` | 662 | 12 | Cok karmasik, demo data inline, `COLORS` tekrari |
| `FewShotExamples.tsx` | 200 | 9 | Fazla state, modal tekrari, tag badge inline |
| `Metadata.tsx` | 1,124 | 20+ | **En sorunlu** — bolunmeli, raw fetch, 2 modal |
| `QueryBuilder.tsx` | 711 | 19 | Cok state, 6x add/update/remove tekrari, `any` tip |
| `ResultTable.tsx` | 179 | 3 | **En temiz** bilesen |
| `SavedQuestions.tsx` | 207 | 4 | Demo-only, tag badge inline, `any` tip |
| `ui/ModelBadgeRow.tsx` | ~80 | 0 | Temiz, inline style icin iyilestirilebilir |
| `ui/Select.tsx` | ~170 | 5 | Iyi erisilebilirlik, temiz implementasyon |

### Hooklar

| Dosya | Satir | Degerlendirme |
|---|---|---|
| `useApi.ts` | ~150 | Iyi — ama `useAdminApi` tekrar iceriyor |
| `useConversation.ts` | ~110 | Iyi — localStorage yonetimi temiz |
| `useQueryParam.ts` | ~55 | Iyi — `useSyncExternalStore` dogru kullanim |
| `useStreamingApi.ts` | ~240 | Karmaşik ama islevsel — SSE/fetch/fallback 3 katmanli |

### Tipler

| Dosya | Satir | Sorunlar |
|---|---|---|
| `types/ai.ts` | ~270 | `types/query.ts` ile 9 interface tekrari |
| `types/metadata.ts` | ~55 | Temiz |
| `types/query.ts` | ~100 | `types/ai.ts` ile tekrar |
| `types/semantic.ts` | ~50 | Temiz |

### Yardimcilar

| Dosya | Satir | Degerlendirme |
|---|---|---|
| `utils/resultCellFormat.ts` | ~140 | Iyi — yeterince moduler ve iyi test edilebilir |

---

## Ek Notlar

- **Select.tsx** erisilebilirlik acisindan en iyi implementasyona sahip bilesen. Diger bilesenler icin ornek alinmali.
- **ResultTable.tsx** en temiz bilesen yapisi. Bilesen bolme calismalarinda referans alinmali.
- **useQueryParam.ts** dogru bir sekilde `useSyncExternalStore` kullaniyor — React 19 best practice.
- Projenin bagimlilik listesi cok minimal (3 paket). Bu bir avantaj ama routing ve state yonetimi icin minimum bir kutuphane eklenmeli.
