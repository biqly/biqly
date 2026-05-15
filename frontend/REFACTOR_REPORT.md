# Biqly Frontend — Refactoring & Best Practices Report

> Tarih: 2026-05-15
> Kapsam: `frontend/src/` altındaki tüm dosyalar (19 TSX + 15 TS + 1 CSS)
> Toplam satır: ~7.400 (`.tsx` sayfaları ~5.200, CSS ~3.050)

---

## 1. Özet Metrikler

| Metrik                             | Değer            |
|------------------------------------|------------------|
| En büyük dosya                    | `AIQuery.tsx` (1.147 satır) |
| Toplam inline `style={{}}`        | **122**          |
| Tekrarlanan CSS property-value     | 30 farklı değer × 3+ kez |
| Monolitik CSS dosyası             | `index.css` (3.048 satır) |
| Hardcoded Türkçe string (AIQuery) | ~90              |
| Birden fazla dosyada tekrar       | Chart rendering, API loading, card layout |
| Sub-component (aynı dosya)        | AIQuery'de 18 fonksiyon |
| Custom router (kütüphane yok)     | 1 (App.tsx)      |
| Test coverage                     | 1 dosya (`formatters.test.ts`) |

---

## 2. Kritik Sorunlar

### 2.1 Chart Rendering Kod Tekrarı (Yüksek Öncelik)

**3 dosyada aynı chart rendering yapısı tekrarlanıyor:**

| Dosya            | Satır | BarChart | LineChart | PieChart |
|------------------|-------|----------|-----------|----------|
| `AIQuery.tsx`    | ~40   | ✅       | ✅        | ✅        |
| `QueryBuilder.tsx`| ~30   | ✅       | ✅        | ✅        |
| `Dashboard.tsx`  | ~25   | ✅       | ✅        | ✅        |

Hepsi aynı kalıbı kullanıyor:
```tsx
<ResponsiveContainer>
  {chartType === 'bar' ? (
    <BarChart data={chartData}>
      <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
      <XAxis dataKey="name" stroke={chartAxisStroke} />
      <YAxis stroke={chartAxisStroke} />
      <Tooltip contentStyle={chartTooltipStyle} />
      <Bar dataKey="value" fill="#3b82f6" />
    </BarChart>
  ) : chartType === 'line' ? ... : ...}
</ResponsiveContainer>
```

**Çözüm:** `ChartContainer` component'i çıkarılmalı:

```tsx
// components/ui/ChartContainer.tsx
interface ChartContainerProps {
  data: { name: string; value?: number }[]
  type: 'bar' | 'line' | 'pie'
  height?: number
}

export function ChartContainer({ data, type, height = 300 }: ChartContainerProps) {
  // Tüm chart mantığı tek yerde
}
```

---

### 2.2 Chart Toggle UI Tekrarı (Yüksek Öncelik)

`AIQuery.tsx` ve `QueryBuilder.tsx`'te aynı toggle buton yapısı:

```tsx
<div className="chart-toggle">
  {(['bar', 'line', 'pie', 'table'] as const).map((t) => (
    <button key={t} className={chartType === t ? 'active' : ''}
            onClick={() => setChartType(t)}>
      {t === 'table' ? 'Tablo' : t === 'bar' ? 'Çubuk' : ...}
    </button>
  ))}
</div>
```

**Çözüm:** `ChartTypeSelector` component'i:

```tsx
// components/ui/ChartTypeSelector.tsx
const CHART_LABELS = { bar: 'Çubuk', line: 'Çizgi', pie: 'Pasta', table: 'Tablo' } as const
export function ChartTypeSelector({ value, onChange }: { value: ChartType; onChange: (v: ChartType) => void }) { ... }
```

---

### 2.3 AIQuery.tsx — God Component (Yüksek Öncelik)

1.147 satır, 28 `useState`, 10 `useEffect`, 18 sub-function/component.

**Sorunlar:**
- Tek dosyada 18 fonksiyon tanımı (ConfidenceBar, TableRoutingViz, ClarificationCard, vb.)
- Tüm state tek component'te toplanmış
- Sorumluluklar: datasource seçimi, tablo routing, soru gönderme, sonuç gösterme, chart, feedback, conversation, sample data, clarification, candidates, retry

**Çözüm:** Component ve hook'lara bölünmeli:

```
components/ai-query/
├── AIQueryPage.tsx          (sayfa wrapper, ~100 satır)
├── QueryInputCard.tsx       (datasource seçimi, soru girişi)
├── QueryResultCard.tsx      (sonuç gösterimi, chart, tablo)
├── TableRoutingPanel.tsx    (tablo yönlendirme viz.)
├── ClarificationCard.tsx    (netleştirme UI)
├── CandidatePanel.tsx       (çoklu aday karşılaştırma)
├── FeedbackPanel.tsx        (geri bildirim)
├── ConfidenceBar.tsx        (güven ölçer)
├── CostBadge.tsx            (maliyet/time badge)
└── hooks/
    ├── useAIQuery.ts        (sorgu gönderme, retry, state)
    ├── useDatasourceSelect.ts (datasource + tablo seçimi)
    └── useQueryFeedback.ts  (feedback state)
```

---

### 2.4 Metadata.tsx — 993 Satır, Çok Fazla Sorumluluk (Yüksek Öncelik)

Metadata sayfası tek dosyada: tablo listesi, kolon detayları, AI description, bulk describe, arama, filtreleme, inline edit.

**Çözüm:**
```
components/metadata/
├── MetadataPage.tsx
├── TableList.tsx
├── ColumnDetail.tsx
├── BulkDescribePanel.tsx
├── MetadataSearch.tsx
```

---

### 2.5 Monolitik CSS — 3.048 Satır `index.css` (Yüksek Öncelik)

**En çok tekrarlanan property-value çiftleri:**

| Property-Value                      | Tekrar | Öneri                        |
|-------------------------------------|--------|------------------------------|
| `color: var(--text-primary)`        | 40     | Utility class: `.text-primary` |
| `display: flex`                     | 38     | Utility class veya layout component |
| `color: var(--text-secondary)`      | 35     | Utility class: `.text-secondary` |
| `color: var(--text-muted)`          | 32     | Utility class: `.text-muted` |
| `border: 1px solid var(--border)`   | 26     | Utility class: `.card-border` |
| `align-items: center`               | 25     | Flexbox utility               |
| `font-size: 0.72rem`               | 18     | Text size scale               |
| `font-size: 0.8rem`                | 15     | Text size scale               |
| `border-radius: 0.5rem`            | 12     | Utility class                 |
| `border-radius: 0.4rem`            | 10     | Utility class                 |

**Duplicate selectors:**
- `.add-btn` → satır 474 ve 587
- `.ai-embedding-error` → satır 1945 ve 1952

**Çözüm seçenekleri:**
1. **CSS Modules** (Vite native): Her component'e `.module.css`
2. **Utility-first yaklaşım:** `.flex`, `.items-center`, `.text-sm`, `.gap-1` gibi CSS custom property tabanlı utility classes
3. **CSS Layers:** `@layer base, components, utilities` ile organizasyon

**Önerilen yapı:**
```
styles/
├── base.css          (reset, custom properties, typography)
├── utilities.css     (.flex, .items-center, .text-sm, vb.)
├── components.css    (.card, .btn, .form-group)
└── pages/
    ├── ai-query.css
    ├── query-builder.css
    ├── metadata.css
    └── ...
```

---

## 3. Orta Öncelikli Sorunlar

### 3.1 Inline Style Kullanımı (122 adet)

**Dosya bazında dağılım:**

| Dosya                | Inline Style Sayısı |
|----------------------|---------------------|
| `Dashboard.tsx`      | 36                  |
| `Evaluation.tsx`     | 30                  |
| `AIQuery.tsx`        | 20                  |
| `Metadata.tsx`       | 13                  |
| `QueryBuilder.tsx`   | 9                   |
| `FewShotExamples.tsx`| 7                   |
| `Datasources.tsx`    | 7                   |
| `SavedQuestions.tsx` | 5                   |

**Tipik örnek (SavedQuestions.tsx):**
```tsx
style={{
  background: 'var(--bg-card)',
  padding: '0.125rem 0.5rem',
  borderRadius: '0.25rem',
  fontSize: '0.75rem',
  color: 'var(--text-secondary)',
}}
```

**Çözüm:** Bu stiller CSS class olmalı. TagBadge veya benzeri bir component ile yeniden kullanılabilir.

---

### 3.2 API Loading/Error Pattern Tekrarı

Her sayfa aynı kalıbı tekrarlıyor:

```tsx
const { get, postData, loading, error, abort } = useApi()

// ... component body ...

{error && <div className="error" style={{ marginTop: '1rem' }}>{error}</div>}
{loading && <p>Yükleniyor...</p>}
```

**Çözüm:** `LoadingOverlay` ve `ErrorAlert` component'leri:

```tsx
// components/ui/LoadingOverlay.tsx
export function LoadingOverlay({ loading, children }: { loading: boolean; children: ReactNode }) { ... }

// components/ui/ErrorAlert.tsx
export function ErrorAlert({ error }: { error: string | null }) { ... }
```

---

### 3.3 Hardcoded Türkçe String'ler (~90+, AIQuery.tsx)

```tsx
<button>Düşünülüyor…</button>
<span>Güven</span>
<p>Daha spesifik olun veya tabloları manuel seçin.</p>
```

**Çözüm:** `i18n` sistemi veya en azından sabitler dosyası:

```ts
// i18n/tr.ts
export const t = {
  aiQuery: {
    thinking: 'Düşünülüyor…',
    confidence: 'Güven',
    hint: 'Daha spesifik olun veya tabloları manuel seçin.',
  },
  ...
}
```

---

### 3.4 Recharts Import Tekrarı

3 dosyada aynı import seti:

```tsx
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer, LineChart, Line,
  PieChart, Pie, Cell,
} from 'recharts'
```

**Çözüm:** `ChartContainer` component çıkarıldığında bu import sadece bir yerde kalır.

---

### 3.5 `types/query.ts` — Gereksiz Re-export

```ts
// query.ts sadece ai.ts'ten re-export yapıyor
export type LogicalQueryPayload = LogicalQuery
export type { CTE, CompiledQuery, FilterClause, ... }
export type QueryResult = QueryResultPayload
```

**Çözüm:** Kullanıcılar doğrudan `types/ai.ts`'ten import etmeli, `query.ts` kaldırılmalı.

---

### 3.6 `SavedQuestions.tsx` — Demo Data ile Çalışıyor

```tsx
const demoQuestions: SavedQuestion[] = [ ... ] // hardcoded
const [questions] = useState<SavedQuestion[]>(demoQuestions)
```

API bağlantısı yok, butonlar hiçbir şey yapmıyor ("Sorguyu çalıştır", "Düzenle", "Sil").

**Çözüm:** API entegrasyonu eklenmeli veya sayfa "yakında" badge'i ile işaretlenmeli.

---

## 4. Düşük Öncelikli / Mimari Öneriler

### 4.1 Custom Router → React Router veya TanStack Router

`App.tsx`'te custom routing var (~60 satır). Bu routing kütüphanesi:
- Nested routes desteklemiyor
- Route guards yok
- URL params yok (sadece search params `useQueryParam` ile)
- Code splitting elle yapılmış (`React.lazy`)

**Çözüm:** `tanstack/router` veya `react-router` v7'ye geçiş. Wouter da minimal bir alternatif.

---

### 4.2 State Management — Yerel Hook'lar Yeterli ama Sınıra Yakın

`useApi`, `useConversation`, `useQueryParam`, `useArrayState`, `useStreamingApi` — hepsi iyi tasarlanmış.

Ancak `AIQuery.tsx`'teki 28 `useState` cross-component state paylaşımı gerektiriyor. Context veya Zustand/Jotai düşünülebilir.

---

### 4.3 Test Coverage — 1 Dosya

Sadece `formatters.test.ts` var. Kritik test eksiklikleri:

| Öncelik | Hedef                            |
|---------|----------------------------------|
| Yüksek  | `useApi` — timeout, abort, error |
| Yüksek  | `compiler` output (SQL assertions) |
| Orta    | `ResultTable` — sorting, context menu |
| Orta    | `buildPivotTable` — edge cases   |
| Düşük   | `useConversation` — localStorage |

---

### 4.4 `useStreamingApi.ts` — Karmaşık ve Kullanılmıyor

228 satır, EventSource + fetch streaming + typing effect. Ancak şu an hiçbir sayfa bu hook'u kullanmıyor (eval streaming henüz aktif değil).

**Çözüm:** Kullanıma alınana kadar dead code olarak işaretlenebilir veya basitleştirilebilir.

---

### 4.5 Accessibility Eksiklikleri

- `Dashboard.tsx`'te chart'lar için `aria-label` yok
- `SavedQuestions.tsx`'te butonların `aria-label` eksik
- Inline edit'de focus yönetimi iyileştirilmeli
- Tab navigation bazı modal'lerde eksik (trap focus)

---

### 4.6 Component API Tutarsızlıkları

| Sorun                              | Dosya                    |
|------------------------------------|--------------------------|
| `Select` default + named export    | `Select.tsx`             |
| `Modal` sadece named export        | `Modal.tsx`              |
| `ResultTable` default export       | `ResultTable.tsx`        |
| `KPICard` named export             | `KPICard.tsx`            |

**Çözüm:** Tüm UI component'leri named export kullanmalı:

```tsx
export function ComponentName() { ... }
```

---

## 5. Önerilen Refactoring Yol Haritası

### Faz 1 — Hızlı Kazançlar (1-2 gün)
1. `ChartContainer` component'i çıkarılması → 3 dosyadan ~100 satır kaldırılır
2. `ChartTypeSelector` component'i → tekrarlanan toggle UI
3. `ErrorAlert` + `LoadingOverlay` UI component'leri
4. `index.css`'teki duplicate selector'lar temizlenmeli

### Faz 2 — Orta Vadeli (3-5 gün)
5. `AIQuery.tsx` parçalanması → `components/ai-query/` alt yapısı
6. `Metadata.tsx` parçalanması → `components/metadata/` alt yapısı
7. `index.css` bölünmesi → `styles/` klasör yapısı
8. Inline style'ların CSS class'a çevrilmesi
9. Export tutarlılığı (tüm UI → named export)

### Faz 3 — Mimari (1-2 hafta)
10. Router kütüphanesi geçişi (wouter / tanstack-router)
11. i18n altyapısı (en az sabitler dosyası)
12. Test coverage artışı (hooks + utils + component'ler)
13. `SavedQuestions` API entegrasyonu
14. `useStreamingApi` aktifleştirme veya kaldırma

---

## 6. Dosya Bazlı Satır Sayıları ve Inline Style Dağılımı

| Dosya                   | Satır | Inline Style | useState | useEffect |
|-------------------------|-------|-------------|----------|-----------|
| `AIQuery.tsx`           | 1.147 | 20          | 28       | 10        |
| `Metadata.tsx`          | 993   | 13          | -        | -         |
| `QueryBuilder.tsx`      | 782   | 9           | 7        | 4         |
| `Evaluation.tsx`        | 643   | 30          | -        | -         |
| `ResultTable.tsx`       | 153   | 1           | 3        | 0         |
| `Dashboard.tsx`         | 349   | 36          | 6        | 2         |
| `SavedQuestions.tsx`    | 172   | 5           | 3        | 0         |
| `FewShotExamples.tsx`   | 201   | 7           | -        | -         |
| `Datasources.tsx`       | 166   | 7           | -        | -         |
| `Select.tsx` (UI)       | 284   | 0           | 4        | 5         |
| `App.tsx`               | 181   | 0           | 1        | 3         |
| `index.css`             | 3.048 | -           | -        | -         |

---

## 7. İyi Tasarlanmış Alanlar (Değiştirilecek Değil)

- **`useApi` hook:** Generic, abort support, timeout — iyi tasarlanmış
- **`useQueryParam` hook:** `useSyncExternalStore` ile doğru implementasyon
- **`useConversation` hook:** localStorage persistence, quota handling
- **`Select` component:** Tam erişilebilir, keyboard navigation, popover positioning
- **`ErrorBoundary` component:** Temiz retry mekanizması
- **`Modal` component:** Backdrop click-to-close, aria modal
- **`buildPivotTable` utility:** Saf fonksiyon, iyi test edilebilir
- **`resultCellFormat` utility:** Column-type-aware formatlama, identifier/calendar detection
- **CSS custom properties:** Dark theme değişkenleri tutarlı
- **Lazy loading:** Tüm sayfalar `React.lazy` ile code-split edilmiş
