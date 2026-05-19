# Biqly Frontend — Kod Tekrarı Analizi & Best Practices

> Tarih: 2026-05-19
> Kapsam: `frontend/src/` altındaki tüm dosyalar (22 TSX + 22 TS)
> Toplam kaynak dosya: 44 dosya, ~7.800 satır

---

## İçindekiler

1. [Kod Tekrarı — Kritik](#1-kod-tekrarı--kritik)
2. [Kod Tekrarı — Orta Öncelik](#2-kod-tekrarı--orta-öncelik)
3. [God Component Sorunları](#3-god-component-sorunları)
4. [Tip Duplikasyonu & Dead Code](#4-tip-duplikasyonu--dead-code)
5. [Hook Anti-Pattern'leri](#5-hook-anti-patternleri)
6. [CSS & Styling Sorunları](#6-css--styling-sorunları)
7. [Accessibility Eksiklikleri](#7-accessibility-eksiklikleri)
8. [i18n Tutarsızlıkları](#8-i18n-tutarsızlıkları)
9. [Tema & Renk Hardcoding](#9-tema--renk-hardcoding)
10. [Best Practice Kuralları](#10-best-practice-kuralları)
11. [Refactoring Yol Haritası](#11-refactoring-yol-haritası)

---

## 1. Kod Tekrarı — Kritik

### 1.1 Datasource Loading Pattern — 5 Dosyada Aynı

Aşağıdaki blok **5 farklı dosyada** neredeyse satır satır aynı:

```typescript
// AIQuery.tsx:~200, QueryBuilder.tsx:237-246, Modeling.tsx:~243,
// Metadata.tsx:131-139, Datasources.tsx (variant)
useEffect(() => {
  get<Datasource[]>('/api/datasources').then((data) => {
    if (!data) return
    setDatasources(data)
    setDatasourceId((prev) => {
      if (prev && data.some((d) => d.id === prev)) return prev
      return data[0]?.id ?? ''
    })
  })
}, [])
```

Artı URL param sync:
```typescript
useEffect(() => { setDsParam(datasourceId) }, [datasourceId, setDsParam])
```

**Çözüm:**
```typescript
// hooks/useDatasourceSelector.ts
export function useDatasourceSelector() {
  const { get } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useQueryParam('ds')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => {
        if (prev && data.some((d) => d.id === prev)) return prev
        return data[0]?.id ?? ''
      })
    }).finally(() => setLoading(false))
  }, [])

  return { datasources, datasourceId, setDatasourceId, loading }
}
```

**Etki:** 5 dosyadan ~60 satır kaldırılır.

---

### 1.2 Semantic Model Loading — 3 Dosyada Aynı

```typescript
// QueryBuilder.tsx:258-275, Modeling.tsx, AIQuery.tsx:~230
get<SemanticModelSummary[]>(
  `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`
).then((data) => {
  setModels(data || [])
  // auto-select published model...
})
```

**Çözüm:**
```typescript
// hooks/useSemanticModels.ts
export function useSemanticModels(datasourceId: string) {
  const { get } = useApi()
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!datasourceId) { setModels([]); return }
    setLoading(true)
    get<SemanticModelSummary[]>(
      `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`
    ).then((data) => {
      setModels(data || [])
    }).finally(() => setLoading(false))
  }, [datasourceId])

  return { models, loading }
}
```

**Etki:** 3 dosyadan ~40 satır kaldırılır.

---

### 1.3 Modal Pattern — 7 Yerde Elle Yazılmış

`Modal.tsx` UI component'i mevcut olmasına rağmen, aşağıdaki dosyalarda elle yazılmış modal backdrop pattern:

| Dosya | Modal Adedi |
|---|---|
| `FewShotExamples.tsx` | 1 |
| `Modeling.tsx` (rename, confirm, base-swap, add-metric) | 4 |
| `Metadata.tsx` (bulk, describe) | 2 |

Elle yazılan pattern:
```tsx
<div className="modal-backdrop" onClick={close}>
  <div className="modal-card" onClick={(e) => e.stopPropagation()}>
    {/* ... */}
  </div>
</div>
```

**Çözüm:** Mevcut `<Modal>` component'ini tüm dosyalarda kullan.

**Etki:** ~200 satır kaldırılır, tutarlı modal davranışı sağlanır.

---

### 1.4 Toggle Button Group — 3 Dosyada Aynı Pattern

| Dosya | İşlev |
|---|---|
| `ThemeToggle.tsx:34-52` | Light/Dark/System toggle |
| `LanguageSwitcher.tsx:11-27` | TR/EN toggle |
| `ChartTypeSelector.tsx:37-50` | Bar/Line/Pie/Table toggle |

Hepsi aynı kalıbı uyguluyor:
```tsx
{options.map((option) => (
  <button
    key={option}
    className={value === option ? 'active' : ''}
    onClick={() => onChange(option)}
    aria-pressed={value === option}
  >
    {label(option)}
  </button>
))}
```

**Çözüm:**
```tsx
// components/ui/ToggleButtonGroup.tsx
interface ToggleButtonGroupProps<T extends string> {
  options: T[]
  value: T
  onChange: (value: T) => void
  getLabel: (option: T) => string
  className?: string
  ariaLabel: string
}

export function ToggleButtonGroup<T extends string>({
  options, value, onChange, getLabel, className, ariaLabel
}: ToggleButtonGroupProps<T>) {
  return (
    <div role="group" aria-label={ariaLabel} className={className}>
      {options.map((option) => (
        <button
          key={option}
          className={value === option ? 'active' : ''}
          onClick={() => onChange(option)}
          aria-pressed={value === option}
        >
          {getLabel(option)}
        </button>
      ))}
    </div>
  )
}
```

**Etki:** 3 component'ten ~60 satır kaldırılır.

---

### 1.5 Chart Data Transformation — 2 Dosyada Aynı

```typescript
// QueryBuilder.tsx:459-463 ve AIQuery.tsx (aynı mantık)
const chartData = result?.rows?.map((row) => {
  const obj: { name: string; value?: number } = { name: String(row[0]) }
  if (row[1] !== undefined) obj.value = Number(row[1]) || 0
  return obj
}) || []
```

**Çözüm:**
```typescript
// utils/chartData.ts
export function rowsToChartData(
  rows: unknown[][] | undefined
): { name: string; value?: number }[] {
  return rows?.map((row) => {
    const obj: { name: string; value?: number } = { name: String(row[0]) }
    if (row[1] !== undefined) obj.value = Number(row[1]) || 0
    return obj
  }) || []
}
```

---

### 1.6 Results Table Pattern — 4 Dosyada Tekrarlanan Tablo

`ResultTable` component'i mevcut ama `QueryBuilder.tsx`, `Dashboard.tsx`, `Evaluation.tsx` hala ham `<table className="results-table">` kullanıyor.

**Çözüm:** Tüm dosyalar `<ResultTable>` component'ini kullanmalı.

---

### 1.7 Status Badge Duplikasyonu

- `TagBadge.tsx` — CSS class bazlı status badge
- `bulkProgress.tsx:41-55` — `BulkStatusBadge` inline style ile aynı işi yapıyor

**Çözüm:** `BulkStatusBadge` yerine `TagBadge` kullanılmalı.

---

### 1.8 `localeNumberTag()` — 2 Dosyada Farklı Implementasyon

```typescript
// AIQuery.tsx — fonksiyon olarak
function localeNumberTag(locale: Locale): string {
  return locale === 'en' ? 'en-US' : 'tr-TR'
}

// Evaluation.tsx:164 — inline
const localeTag = locale === 'tr' ? 'tr-TR' : 'en-US'
```

**Çözüm:** `utils/formatters.ts`'e ortak fonksiyon olarak ekle.

---

## 2. Kod Tekrarı — Orta Öncelik

### 2.1 `useAdminApi` — 5 Metodu Birebir Kopyalıyor

```typescript
// useApi.ts:162-192 — her HTTP metodu için aynı wrapper
const get = useCallback(
  <T = unknown>(url: string, options?: RequestOptions) =>
    api.get<T>(url, withHeaders(options, adminHeaders)),
  [api],
)
// postData, putData, patchData, deleteData — aynı kalıp 5 kez
```

**Çözüm:** Tek bir wrapper fonksiyon ile tüm metodları sar:
```typescript
function withAdmin<T extends object>(obj: T, headers: Record<string, string>): T {
  return new Proxy(obj, {
    get(target, prop) {
      const fn = target[prop as keyof T]
      if (typeof fn === 'function') {
        return (...args: unknown[]) => fn(...wrapArgs(args, headers))
      }
      return fn
    }
  })
}
```

---

### 2.2 Timeout/AbortController Pattern — 3 Yerde

- `useApi.ts:34-42`
- `useStreamingApi.ts:118-119`
- `useStreamingApi.ts:147-148`

**Çözüm:** `utils/createTimeoutSignal.ts`:
```typescript
export function createTimeoutSignal(
  timeoutMs: number,
  externalSignal?: AbortSignal
): { signal: AbortSignal; cleanup: () => void } {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  // merge external signal
  if (externalSignal) {
    externalSignal.addEventListener('abort', () => controller.abort())
  }
  return {
    signal: controller.signal,
    cleanup: () => clearTimeout(timer),
  }
}
```

---

### 2.3 Error Parsing — 2 Hook'ta Farklı Mantık

- `useApi.ts` — `responseError()` HTML temizleme + yapısal hata çıkarma
- `useStreamingApi.ts` — sadece `throw new Error(text || ...)`

**Çözüm:** `useStreamingApi`, `useApi`'deki `responseError()` fonksiyonunu kullanmalı.

---

### 2.4 Inline Grid Styles — Dashboard & Evaluation

```typescript
// Dashboard.tsx'te 4 kez:
style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '1.5rem' }}
// Evaluation.tsx'te 1 kez:
style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}
```

**Çözüm:** CSS utility classes:
```css
.grid-kpi { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; }
.grid-cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1.5rem; }
.grid-charts { display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 1.5rem; }
```

---

### 2.5 `className` Concatenation — 3 Farklı Yaklaşım

```typescript
// Yaklaşım 1: Template literal (ChartContainer, ErrorAlert, LoadingOverlay, InlineEdit, KPICard)
`chart-container${className ? ` ${className}` : ''}`

// Yaklaşım 2: Array + filter + join (Select, EmptyState)
['ui-select', className].filter(Boolean).join(' ')

// Yaklaşım 3: String concat (KPICard)
'kpi-card' + (className ? ` ${className}` : '')
```

Proje `clsx` dependency'sine sahip ama hiç kullanılmıyor.

**Çözüm:** Tüm dosyalarda `clsx` kullan:
```typescript
import clsx from 'clsx'
// <div className={clsx('chart-container', className)}>
```

---

## 3. God Component Sorunları

### 3.1 Modeling.tsx — 2.094 Satır (En Büyük Dosya)

| Metrik | Değer |
|---|---|
| Toplam satır | 2.094 |
| `useState` sayısı | ~30 |
| `useEffect` sayısı | ~10 |
| Sub-component (aynı dosya) | RenameModal, ConfirmModal, BaseSwapModal, AddMetricModal, + diğerleri |

**Önerilen yapı:**
```
components/modeling/
├── ModelingPage.tsx           (sayfa wrapper, routing)
├── ModelList.tsx              (sol panel, model listesi)
├── ModelEditor.tsx            (sağ panel, boyut/metrik/join editörü)
├── DimensionEditor.tsx        (boyut ekleme/düzenleme)
├── MetricEditor.tsx           (metrik ekleme/düzenleme)
├── JoinEditor.tsx             (join tanımlama)
├── ModelPublishBar.tsx        (draft/publish/rollback butonları)
├── BaseSwapModal.tsx          (base table değiştirme)
├── AddMetricModal.tsx         (metrik ekleme dialogu)
└── hooks/
    └── useModelingState.ts    (tüm state yönetimi)
```

### 3.2 AIQuery.tsx — 1.147 Satır

| Metrik | Değer |
|---|---|
| Toplam satır | 1.147 |
| `useState` sayısı | ~25-28 |
| `useEffect` sayısı | ~10 |
| Sub-function | 18 (ConfidenceBar, TableRoutingViz, ClarificationCard, vb.) |

(Mevcut REFACTOR_REPORT.md'deki yapı önerisi geçerli)

### 3.3 Metadata.tsx — 1.048 Satır

| Metrik | Değer |
|---|---|
| Toplam satır | 1.048 |
| `useState` sayısı | ~20 |
| Elle yazılmış modal | 2 |

### 3.4 QueryBuilder.tsx — 875 Satır

| Metrik | Değer |
|---|---|
| Toplam satır | 875 |
| `useState` sayısı | ~15 |
| Ham tablo rendering | 1 (ResultTable kullanmıyor) |

---

## 4. Tip Duplikasyonu & Dead Code

### 4.1 Duplicate Type Tanımları

| Tip | Dosyalar |
|---|---|
| `SemanticModelSummary` | `QueryBuilder.tsx:14-22`, `Modeling.tsx` |
| `SemanticDimension` | `QueryBuilder.tsx:24-29`, `Modeling.tsx` |
| `SemanticMetric` | `QueryBuilder.tsx:31-36`, `Modeling.tsx` |
| `SemanticJoin` | `QueryBuilder.tsx:39-50`, `Modeling.tsx` |
| `SemanticModelDetail` | `QueryBuilder.tsx:52-61`, `Modeling.tsx` |
| `GenerateSemanticModelResponse` | `QueryBuilder.tsx:63-73`, `Modeling.tsx` |
| `TableRow` / `ColumnRow` | `Modeling.tsx`, `Metadata.tsx:21-42` |

**Çözüm:** `types/semantic.ts` ve `types/metadata.ts` zaten mevcut — tüm dosyalar bunları kullanmalı, yerel tanımlar kaldırılmalı.

### 4.2 Chart Type Union — 5 Yerde Tanımlı

```typescript
// ai.ts:333
ChartSuggestion = 'bar' | 'line' | 'table' | 'number' | 'pie'
// ai.ts:149
VisualizationHint.chart_type: 'bar' | 'line' | 'pie' | 'table'
// ChartTypeSelector.tsx:1
ChartTypeOption = 'bar' | 'line' | 'pie' | 'table'
// AIQuery.tsx:514 — inline
'bar' | 'line' | 'pie' | 'table'
// QueryBuilder.tsx:310 — inline
'bar' | 'line' | 'pie'
```

`ChartSuggestion` `'number'` içeriyor ama `ChartTypeOption` içermiyor — tutarsız.

**Çözüm:** `types/chart.ts`'te tek bir union tanımla:
```typescript
export type ChartType = 'bar' | 'line' | 'pie' | 'table' | 'number'
export type SelectableChartType = Exclude<ChartType, 'number'>
```

### 4.3 Dead Code

| Export | Dosya | Durum |
|---|---|---|
| `WindowFunction` | `ai.ts:297` | Hiç import edilmiyor |
| `CompiledQuery` | `ai.ts:376` | Hiç import edilmiyor |
| `VisualizationHint` | `ai.ts` | Neredeyse hiç kullanılmıyor |
| `useStreamingApi` | `useStreamingApi.ts` | Hiçbir sayfa kullanmıyor (228 satır) |

**Çözüm:** Kullanılmayan export'lar kaldırılmalı. `useStreamingApi` aktifleştirilene kadar dead code olarak işaretlenmeli.

### 4.4 Frontend-Backend Tip İsim Uyumsuzluğu

| Frontend Tipi | Backend Karşılığı |
|---|---|
| `FilterClause` | `Filter` |
| `SelectField` | `SelectItem` |
| `GroupByField` | `GroupBy` |
| `OrderByField` | `OrderBy` |

**Çözüm:** Backend isimlerine uyum sağla, API dokümantasyonu ile tutarlılığı koru.

### 4.5 Eksik Tip: `Dimension.type` → `'geo'` Eksik

```typescript
// semantic.ts:18
type: 'string' | 'number' | 'date' | 'boolean'
// AGENTS.md backend spec
type: 'text' | 'number' | 'date' | 'boolean' | 'geo'
```

Frontend `'geo'` desteklemiyor. Backend gönderirse temsil edilemez.

---

## 5. Hook Anti-Pattern'leri

### 5.1 `useConversation` — Stale Closure Bug

```typescript
// useConversation.ts:55
const createConversation = useCallback(() => {
  const conv = { id: generateId(), ... }
  persist([conv, ...conversations])  // ← `conversations` stale olabilir!
}, [conversations, persist])
```

Aynı sorun `deleteConversation`, `renameConversation`, `clearConversation`'da da var.

**Çözüm:** Functional `setState` kullan:
```typescript
setConversations((prev) => {
  const next = [conv, ...prev]
  persist(next)
  return next
})
```

### 5.2 `useConversation` — localStorage 2x Parse

```typescript
const [conversations, setConversations] = useState<Conversation[]>(loadConversations)
const [activeConversationId, setActiveConversationId] = useState<string | null>(
  () => loadConversations()[0]?.id ?? null  // ← 2. kez parse!
)
```

**Çözüm:**
```typescript
const [conversations, setConversations] = useState<Conversation[]>(() => {
  const loaded = loadConversations()
  // activeConversationId'yi burada da hesapla
  return loaded
})
```

### 5.3 `useConversation` — Catch İçinde Tekrar Throw Riski

```typescript
function saveConversations(conversations: Conversation[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(conversations))
  } catch {
    const trimmed = conversations.slice(-20)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed))  // ← yine throw olabilir!
  }
}
```

**Çözüm:** İçeriye de try-catch ekle.

### 5.4 `useAdminApi` — `adminHeaders` Her Render'da Yeniden Hesaplanıyor

```typescript
const adminHeaders = adminAuthHeaders()  // her render'da yeni object
```

**Çözüm:** `useMemo` ile sarmala veya module-level constant yap.

### 5.5 `useApi()` Birden Fazla Kez Aynı Component'te

- `AIQuery.tsx`: `useApi()` **2 kez** — query ve embedding için ayrı instance
- `Dashboard.tsx`: Alt component'ler (`AIUsageSection`, `ModelSuccessRates`) kendi `useApi()`'lerini yaratıyor — 3 bağımsız loading/error state

**Çözüm:** Tek bir `useApi()` instance kullan veya her API çağrısını custom hook'a çıkar.

### 5.6 `Metadata.tsx` — Raw `fetch()` Kullanımı

```typescript
// Metadata.tsx:355-366 — useApi hook'u varken raw fetch kullanıyor
const res = await fetch(url, { method: 'POST', ... })
```

Bu, auth header'ları ve error handling'i bypass ediyor.

**Çözüm:** `useApi`'deki `postData()` kullanılmalı.

---

## 6. CSS & Styling Sorunları

### 6.1 Monolitik `index.css` — 3.048 Satır

En çok tekrarlanan property-value çiftleri:

| Property-Value | Tekrar | Önerilen Utility |
|---|---|---|
| `color: var(--text-primary)` | 40 | `.text-primary` |
| `display: flex` | 38 | `.flex` |
| `color: var(--text-secondary)` | 35 | `.text-secondary` |
| `color: var(--text-muted)` | 32 | `.text-muted` |
| `border: 1px solid var(--border)` | 26 | `.card-border` |
| `align-items: center` | 25 | `.items-center` |
| `font-size: 0.72rem` | 18 | `.text-xs` |
| `font-size: 0.8rem` | 15 | `.text-sm` |
| `border-radius: 0.5rem` | 12 | `.rounded-md` |
| `border-radius: 0.4rem` | 10 | `.rounded` |

**Çözüm:**
```
styles/
├── base.css          (reset, custom properties, typography)
├── utilities.css     (.flex, .items-center, .text-sm, vb.)
├── components.css    (.card, .btn, .form-group)
└── pages/
    ├── ai-query.css
    ├── modeling.css
    └── ...
```

### 6.2 Inline Style Sayısı — 122 Adet

| Dosya | Inline Style Sayısı |
|---|---|
| `Dashboard.tsx` | 36 |
| `Evaluation.tsx` | 30 |
| `AIQuery.tsx` | 20 |
| `Metadata.tsx` | 13 |
| `QueryBuilder.tsx` | 9 |
| `FewShotExamples.tsx` | 7 |
| `Datasources.tsx` | 7 |
| `bulkProgress.tsx` | ~30 property |
| `ResultTable.tsx` | 5 |
| `SavedQuestions.tsx` | 5 |

### 6.3 CSS-in-JS vs CSS Class Tutarsızlığı

| Yaklaşım | Dosyalar |
|---|---|
| Sadece CSS class | ErrorAlert, ErrorBoundary, EmptyState, ChartTypeSelector, TagBadge |
| CSS class + inline style | ChartContainer, InlineEdit, KPICard, Select |
| Tamamen CSS-in-JS | **ModelBadgeRow**, **bulkProgress** |
| Karışık | ResultTable |

`ModelBadgeRow` ve `bulkProgress` tüm layout'u inline style ile yapıyor — codebase'in geri kalanından tamamen farklı.

**Kural:** Tüm styling tek bir strateji kullanmalı: CSS class + CSS custom properties.

### 6.4 CSS Class İsimlendirme Tutarsızlığı

| Pattern | Örnek |
|---|---|
| BEM (doğru) | `bulk-segmented__btn--active` |
| Single class | `card`, `btn`, `tag-pill` |
| `--` modifier | `card--query-builder`, `card--elevated` |
| Tek `-` modifier | `error--top-gap` |
| Karma yaklaşım | `btn btn-sm btn-danger` vs `btn btn--destructive` vs `btn-danger-outline` |

"Tehlike butonu" 3 farklı isimle: `btn-danger`, `btn--destructive`, `btn-danger-outline`

**Kural:** BEM standardını tutarlı uygula:
```css
.btn--danger { ... }
.btn--danger-outline { ... }
```

### 6.5 Duplicate CSS Selector'lar

- `.add-btn` → satır 474 ve 587
- `.ai-embedding-error` → satır 1945 ve 1952

---

## 7. Accessibility Eksiklikleri

### 7.1 Kritik — Modal Focus Trap Eksik

`Modal.tsx` focus trap implement etmiyor. Tab tuşu ile arka plana geçiş yapılabilir. Escape tuşu çalışmıyor.

**Çözüm:** Focus trap ekle veya `aria-modal` + keyboard event listener ekle.

### 7.2 ResultTable — Sort Header Klavye Erişimi Yok

```tsx
<th onClick={() => toggleSort(i)}>  // ← klavye ile erişilemez
```

**Çözüm:**
```tsx
<th
  role="button"
  tabIndex={0}
  onClick={() => toggleSort(i)}
  onKeyDown={(e) => e.key === 'Enter' && toggleSort(i)}
>
```

### 7.3 InlineEdit — Klavye ile Başlatılamaz

`onDoubleClick` ile başlıyor, klavye mekanizması yok.

**Çözüm:** Enter veya Space tuşu ile de düzenlemeyi başlat.

### 7.4 ChartContainer — Hiçbir ARIA Yok

SVG chart'lar `role="img"`, `aria-label` olmadan render ediliyor. Ekran okuyucular için tamamen görünmez.

### 7.5 KPICard — Semantik Rol Yok

`<div>` olarak render ediliyor, `role` veya `aria-label` yok.

### 7.6 TagBadge — Sadece Renk ile Anlam İletiliyor

Status renkleri (yeşil/kırmızı/sarı) sadece görsel. `role="status"` ve `aria-label` eksik.

---

## 8. i18n Tutarsızlıkları

### Tamamen Hardcoded Türkçe String'ler (i18n Kullanılmıyor)

| Dosya | Hardcoded String | Satır |
|---|---|---|
| `Select.tsx` | `'— seçin —'` (placeholder default) | 37 |
| `Select.tsx` | `'Seçenek yok'` | 239 |
| `LoadingOverlay.tsx` | `'Yükleniyor…'` (label default) | 10 |
| `ResultTable.tsx` | `'Filtre:'`, `'Değeri kopyala'` | 105, 113 |
| `ResultTable.tsx` | `'Sıralamak için tıklayın...'` | 139 |
| `ResultTable.tsx` | `'satır'`, `'artan'`, `'azalan'` | 176, 181 |
| `ChartTypeSelector.tsx` | `FALLBACK_LABELS` (İngilizce) | 3-8 |
| `AIQuery.tsx` | ~90 hardcoded Türkçe string | - |
| `Modeling.tsx` | `'✎'`, `'★'`, `'×'`, `'+'`, `'‹'`, `'›'` | - |

**Kural:** Tüm kullanıcı-görünür string'ler `useT()` hook'u ile çevrilmeli. UI component'ler (Select, LoadingOverlay, ResultTable) i18n key almalı, hardcoded default kullanmamalı.

### UI Component'lerin i18n Yaklaşımı Tutarsız

| Component | i18n Yaklaşımı |
|---|---|
| `ErrorBoundary` | `useT()` kullanıyor |
| `Modal` | `useT()` kullanıyor |
| `InlineEdit` | `useT()` kullanıyor |
| `ThemeToggle` | `useT()` kullanıyor |
| `LanguageSwitcher` | `useT()` kullanıyor |
| `Select` | **Hardcoded Türkçe** |
| `LoadingOverlay` | **Hardcoded Türkçe** |
| `ResultTable` | **Hardcoded Türkçe** |
| `DriverTileGrid` | `t` prop olarak alıyor (tutarsız) |

**Kural:** Tüm component'ler doğrudan `useT()` hook'unu kullanmalı. `t` prop olarak geçirilmemeli.

---

## 9. Tema & Renk Hardcoding

### Hardcoded Renkler (CSS Variable Kullanmıyor)

| Dosya | Hardcoded Renk | Kullanım |
|---|---|---|
| `chartConfig.ts` | `#94a3b8`, `#475569`, `#1e293b` | Chart axis, grid, tooltip |
| `constants.ts` | 8 hardcoded hex (`#3b82f6`, vb.) | CHART_COLORS |
| `ChartContainer.tsx` | `#3b82f6` | Default fill |
| `bulkProgress.tsx` | `#60a5fa`, `#4ade80`, `#f87171` | Status renkleri |
| `ModelBadgeRow.tsx` | `rgba(255,255,255,0.05)` | Surface fallback |

**Sorun:** Chart'lar ve status göstergeleri light/dark temayı takip etmiyor. Light modda bile dark tema renkleri gösteriliyor.

**Çözüm:** CSS variable bazlı renk tanımları:
```css
:root {
  --chart-primary: #3b82f6;
  --chart-axis: var(--text-muted);
  --chart-grid: var(--border);
  --chart-tooltip-bg: var(--bg-card);
  --status-ok: #4ade80;
  --status-error: #f87171;
  --status-running: #60a5fa;
}
[data-theme="light"] {
  --chart-axis: #64748b;
  --chart-grid: #e2e8f0;
  --chart-tooltip-bg: #ffffff;
}
```

---

## 10. Best Practice Kuralları

### 10.1 Component Yapısı

| Kural | Açıklama |
|---|---|
| **Tek Sorumluluk** | Bir component 300 satırı geçmemeli. Geçiyorsa böl. |
| **Named Export** | Tüm component'ler `export function` kullanmalı. Default export yok. |
| **Prop Interface** | Her component `interface ComponentNameProps` tanımlamalı. |
| ** clsx Kullan** | `className` birleştirme için her zaman `clsx` kullan. |
| **Alt Component'ler Ayrı Dosya** | Aynı dosyada 5+ sub-component olmamalı. |

### 10.2 Hook Kuralları

| Kural | Açıklama |
|---|---|
| **Functional setState** | State callback'ler her zaman `(prev) =>` kullanmalı. Stale closure riski. |
| **Tek useApi Instance** | Bir sayfada birden fazla `useApi()` kullanmamalı. Her API çağrısı ayrı hook'a çıkılmalı. |
| **useEffect Cleanup** | Timeout/EventSource/interval her zaman cleanup ile kullanılmalı. |
| **Dependency Array** | Tüm dependency'ler eksiksiz olmalı. `void prev` hack'i kullanma. |

### 10.3 TypeScript Kuralları

| Kural | Açıklama |
|---|---|
| **Backend Tip Uyumu** | Frontend tipleri backend ile aynı isimleri kullanmalı (`Filter` → `Filter`, `SelectItem` → `SelectItem`). |
| **`unknown` Yerine Tip** | `rows: unknown[][]` yerine proper tipler tanımla. |
| **Dead Code Temizle** | Kullanılmayan export, tip, fonksiyon hemen kaldırılmalı. |
| **`any` Yasak** | `any` tip kullanımı yasak. |

### 10.4 CSS Kuralları

| Kural | Açıklama |
|---|---|
| **CSS Variable Kullan** | Tüm renkler CSS variable olmalı. Hardcoded hex yasak. |
| **Inline Style Yasak** | Layout ve renk hiçbir zaman inline style olmamalı. |
| **CSS Class BEM** | `.block__element--modifier` standardı tutarlı uygulanmalı. |
| **Utility Classes** | Tekrarlanan property'ler utility class'a çıkarılmalı. |

### 10.5 Accessibility Kuralları

| Kural | Açıklama |
|---|---|
| **Focus Trap** | Modal'lar focus trap kullanmalı. |
| **Keyboard Navigation** | Tüm interaktif elementler klavye ile erişilebilir olmalı. |
| **ARIA Labels** | Chart, KPI, badge gibi non-text elementler `aria-label` içermeli. |
| **Renk ile Anlam Yasak** | Sadece renk ile anlam iletmek yasak. Text veya aria-label ekle. |

### 10.6 i18n Kuralları

| Kural | Açıklama |
|---|---|
| **Hardcoded String Yasak** | Kullanıcı-görünür hiçbir string hardcoded olmamalı. |
| **useT() Hook** | Tüm component'ler doğrudan `useT()` kullanmalı. `t` prop geçişi yok. |
| **UI Component Defaults** | Select, LoadingOverlay gibi component'ler hardcoded default text kullanmamalı. |

---

## 11. Refactoring Yol Haritası

### Faz 1 — Hızlı Kazançlar (1-2 gün)

| # | Görev | Dosya/Etki | Tahmini Kazanç |
|---|---|---|---|
| 1 | `useDatasourceSelector` hook çıkar | 5 dosya | ~60 satır |
| 2 | `useSemanticModels` hook çıkar | 3 dosya | ~40 satır |
| 3 | Mevcut `<Modal>` component'ini kullan | 3 dosya, 7 modal | ~200 satır |
| 4 | `ToggleButtonGroup` component çıkar | 3 dosya | ~60 satır |
| 5 | `clsx` kullanıma al | Tüm UI component'ler | Tutarlılık |
| 6 | Duplicate CSS selector'ları temizle | `index.css` | ~20 satır |
| 7 | Dead code kaldır (`WindowFunction`, `CompiledQuery`, `useStreamingApi`) | `ai.ts`, `useStreamingApi.ts` | ~250 satır |
| 8 | `rowsToChartData` utility çıkar | 2 dosya | ~10 satır |
| 9 | `localeNumberTag` utility çıkar | 2 dosya | ~10 satır |
| 10 | `BulkStatusBadge` → `TagBadge` kullan | `bulkProgress.tsx` | ~15 satır |

**Toplam tahmini kaldırılan satır:** ~665 satır

### Faz 2 — Orta Vadeli (3-5 gün)

| # | Görev | Öncelik |
|---|---|---|
| 11 | `Modeling.tsx` böl → `components/modeling/` | Yüksek |
| 12 | `AIQuery.tsx` böl → `components/ai-query/` | Yüksek |
| 13 | `Metadata.tsx` böl → `components/metadata/` (kısmen yapılmış) | Yüksek |
| 14 | `QueryBuilder.tsx` ResultTable kullan, ham tabloyu kaldır | Yüksek |
| 15 | Duplicate tipleri `types/semantic.ts` ve `types/metadata.ts`'e taşı | Yüksek |
| 16 | Chart type union'ı tek yerde tanımla (`types/chart.ts`) | Orta |
| 17 | `index.css` böl → `styles/` yapısı | Orta |
| 18 | Inline style'ları CSS class'a çevir (öncelikle Dashboard, Evaluation) | Orta |
| 19 | Export tutarlılığı (tüm component → named export) | Düşük |
| 20 | Hardcoded string'leri `useT()` ile çevir (ResultTable, Select, LoadingOverlay) | Orta |

### Faz 3 — Mimari (1-2 hafta)

| # | Görev | Öncelik |
|---|---|---|
| 21 | CSS renk hardcoding'ini temizle → CSS variable bazlı | Yüksek |
| 22 | Modal focus trap ekle | Yüksek (a11y) |
| 23 | ResultTable klavye erişimi ekle | Yüksek (a11y) |
| 24 | InlineEdit klavye başlatma ekle | Orta (a11y) |
| 25 | Chart ARIA ekle (role, aria-label) | Orta (a11y) |
| 26 | `useConversation` stale closure bug'larını düzelt | Yüksek (bug) |
| 27 | `Metadata.tsx` raw fetch → useApi | Orta (bug) |
| 28 | `useAdminApi` wrapper'ı basitleştir | Düşük |
| 29 | `createTimeoutSignal` utility çıkar | Düşük |
| 30 | `useStreamingApi` aktifleştir veya kaldır | Düşük |
| 31 | Router kütüphanesi geçişi (wouter / tanstack-router) | Düşük |
| 32 | Test coverage artışı | Orta |

---

## Ek: Dosya Bazlı Metrikler

| Dosya | Satır | Inline Style | useState | useEffect | Sorun Seviyesi |
|---|---|---|---|---|---|
| `Modeling.tsx` | 2.094 | - | ~30 | ~10 | **Kritik** |
| `AIQuery.tsx` | 1.147 | 20 | ~28 | ~10 | **Kritik** |
| `Metadata.tsx` | 1.048 | 13 | ~20 | ~8 | **Yüksek** |
| `QueryBuilder.tsx` | 875 | 9 | ~15 | ~5 | **Yüksek** |
| `Evaluation.tsx` | 674 | 30 | ~12 | ~6 | Orta |
| `Dashboard.tsx` | 349 | 36 | ~4 | ~3 | Orta |
| `Select.tsx` | 284 | 0 | 4 | 5 | Düşük |
| `App.tsx` | 181 | 0 | 1 | 3 | Düşük |
| `ResultTable.tsx` | 153 | 5 | 3 | 0 | Düşük |
| `FewShotExamples.tsx` | 201 | 7 | ~8 | ~1 | Düşük |
| `SavedQuestions.tsx` | 172 | 5 | 3 | 0 | Düşük |
| `Datasources.tsx` | 166 | 7 | ~10 | ~1 | Düşük |
| `Settings.tsx` | 29 | 0 | 0 | 0 | Temiz |
| `index.css` | 3.048 | - | - | - | **Yüksek** |
