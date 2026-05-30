# Biqly Frontend Refactoring & UX Improvement Plan

> **Olusturulma:** 2026-05-30  
> **Kapsam:** Kod tekrarı, mimari sorunlar, client-side memory, UI/UX best practices  
> **Toplam kaynak dosya:** ~95 TSX/TS/CSS dosya, ~7,500 satir komponent kodu, ~183KB CSS  

---

## Icerik

1. [Kritik Kod Sorunlari](#1-kritik-kod-sorunlari)
2. [API Katmani Problemleri](#2-api-katmani-problemleri)
3. [CSS / Styling Sorunlari](#3-css--styling-sorunlari)
4. [Tip Guvenligi Sorunlari](#4-tip-guvenligi-sorunlari)
5. [i18n Sorunlari](#5-i18n-sorunlari)
6. [Client-Side Memory & Performans](#6-client-side-memory--performans)
7. [UI/UX Best Practices (BI Dashboard)](#7-uiux-best-practices-bi-dashboard)
8. [Oncelikli Aksiyon Plani](#8-oncelikli-aksiyon-plani)

---

## 1. Kritik Kod Sorunlari

### 1.1 God Component Anti-Pattern (6 dosya, toplam ~5,000 satir)

| Dosya | Satir | useState | useEffect | Sorun |
| ------- | ------- | ---------- | ----------- | ------- |
| `Modeling.tsx` | 1,332 | 27 | 8 | Model CRUD, join, dimension, metric, canvas, rename hepsi bir arada |
| `TableBrowser.tsx` | 1,111 | 25 | 8 | Filtre, pagination, drag-drop, data fetch hepsi tek dosyada |
| `QueryBuilder.tsx` | 950 | 18 | 5 | 8 notebook step, filter, having, window func, CTE tek komponent |
| `SavedQuestions.tsx` | 816 | 24 | 6 | Soru CRUD + form + calistirma + few-shot toggle |
| `Settings.tsx` | 955 | 27 | 2 | Passkey, MFA, TOTP, QR code, recovery codes hepsi bir arada |
| `UserListPage.tsx` | 845 | 24 | 5 | Active users + invitations + invite modal, tamamen inline style |

**Modeling.tsx detay:** 30 farkli `/api/` endpoint cagrisi, 3 tekrarlanan sil/reactive paterni, 3 tekrarlanan rename paterni.

**Settings.tsx detay:** 83 inline style, OTP input 3 kez tekrarlanan, recovery codes grid 2 kez kopyalanmis, 9 adet `catch (err: any)`.

### 1.2 Datasource Yukleme Paterni 4 Dosyada Tekrarlanmis

`Modeling.tsx`, `TableBrowser.tsx`, `QueryBuilder.tsx`, `SavedQuestions.tsx` hepsi ayni `useEffect` ile datasource listesini yukluyor:

```typescript
// Her 4 dosyada ayni kod:
useEffect(() => {
  get<Datasource[]>('/api/datasources').then(...)
}, [])
```

**Cozum:** `useDatasources()` hook'u olusturulmali.

### 1.3 Semantic Model Yukleme Paterni 4 Dosyada Tekrarlanmis

Ayni sekilde `get<SemanticModelSummary[]>('/api/semantic/models?datasource_id=...')` 4 dosyada tekrarlanmis.

**Cozum:** `useSemanticModels(datasourceId)` hook'u olusturulmali.

### 1.4 `columnRefMatchesTable` Fonksiyonu 2 Dosyada Kopyalanmis

- `Modeling.tsx:222` (useCallback icinde)
- `TableBrowser.tsx:202` (standalone fonksiyon)

**Cozum:** `modeling/utils.ts` veya `queryBuilder/utils.ts`'a tasinmali.

### 1.5 Custom Router - `react-router` Yok

`App.tsx` (628 satir) kendi routing sistemini `window.history.pushState` ile yonetiyor. Neden sorunlu:

- Route parametreleri yok (`/model/:id`)
- Query string parsing yok
- Nested routes yok
- `navigate` prop'u tum sayfalara prop drilling ile aktariliyor
- `globalNavigate` module-level escape hatch kullaniliyor

**Auth route'lar 3 farkli yerde tanimli:** `if/else if` renderer, document title setter, ve route detection.

**Cozum:** `react-router-dom` ile degistirilmeli. Lazy loading zaten dogru yapilmis (`React.lazy`), sadece routing katmani degistirilmeli.

### 1.6 `window.confirm()` vs `useConfirm()` Tutarsizligi

- `Modeling.tsx`, `SavedQuestions.tsx` → `useConfirm` hook kullanir
- `UserListPage.tsx`, `UserDetailPage.tsx`, `DatasourceAccessPanel.tsx` → `window.confirm()` kullanir

---

## 2. API Katmani Problemleri

### 2.1 4 Farkli Fetch Paterni Birlikte Yasiyor

| Patern | Nerde | Sorun |
| -------- | ------- | ------- |
| `useApi()` hook | 13 komponent | CSRF yok |
| `fetchJSON()` | `useAIJobs.tsx` | CSRF yok, farkli error handling |
| `csrfFetch()` | `admin.ts`, `auth.ts` | CSRF var ama tekrarli kod |
| Raw `fetch()` | `DatasourceAccessPanel`, `AuditLogPanel`, `metadataDescribe` | Ne CSRF ne error handling |

### 2.2 5 Farkli Response Parser

- `useApi.ts:15-22` → `parseResponseBody(text)`
- `useAIJobs.tsx:95-111` → `fetchJSON<T>(url, init)`
- `admin.ts:20-34` → `handle<T>(res)`
- `auth.ts:41-58` → `handleResponse<T>(res)` (admin.ts ile ayni)
- `metadataDescribe.ts:9-30` → `readDescribeResponse(res)`

### 2.3 4 Farkli Error Handling Kontrati

| Modul | Return Tipi | Yaklasim |
| ------- | ------------- | ---------- |
| `useApi.request()` | `{ data, error }` | Non-throwing |
| `fetchJSON()` | `{ data, status, error }` | Non-throwing + status |
| `admin.ts` `handle()` | Throw `Error` | Exception |
| `auth.ts` `handleResponse()` | Throw `Error` | Exception |

Ayni dosyada bile tutarsiz: `admin.ts` bazen `handle()` kullanir, bazen `if (!res.ok) throw` kullanir (11 yerde).

### 2.4 Auth Header 3 Farkli Sekilde Olusturuluyor

- `useApi.ts:160-163` → `adminAuthHeaders()` (admin API key)
- `admin.ts:36-38` → `authHeaders(token)` (parametre olarak)
- `auth.ts` → Her fonksiyonda inline `Authorization: Bearer ${accessToken}` (~20 yerde)

`Authorization: Bearer ${token}` paterni toplamda **~25 kez** tekrarlanmis.

### 2.5 CSRF Acigi (KRITIK)

`useApi` hook'u (13 komponent tarafindan kullanilan) **CSRF token gondermiyor**. Bu su anlama gelir:

- Datasource CRUD islemleri CSRF'siz
- Query execution CSRF'siz
- Semantic model degisiklikleri CSRF'siz

Ayrica `admin.ts` dosyasinda 7 satir raw `fetch` kullanirken geri kalani `csrfFetch` kullanir. `updateUserActiveStatus` (PUT) raw `fetch` ile CSRF'siz gidiyor.

### 2.6 Race Conditions

1. `useApi` tek `AbortController` kullanir → pes pese 2 istekte birinciyi abort edemez
2. `useAIJobs` polling'de stale closure riski
3. `admin.ts` ve `auth.ts` hicbir AbortController kullanmiyor
4. `startBulkDescribe` fire-and-forget IIFE → iki concurrent bulk operation ayni state'e yazar

---

## 3. CSS / Styling Sorunlari

### 3.1 `index.css` 107KB Monolit

5,092 satirlik tek dosyada sidebar, nav, cards, tables, modals, modeling canvas, AI jobs panel, workspace settings, sharing UI hepsi karisik.

**Toplam CSS:** 183KB (7 dosya)

### 3.2 79 Adet `!important` Kullanimi

| Dosya | `!important` Sayisi | Ciddiyet |
| ------- | --------------------- | ---------- |
| `tableBrowser.css` | **59** | Kritik |
| `index.css` | 12 | Orta |
| `aiQuery.css` | 6 | Dusuk |

`tableBrowser.css` tek basina 59 `!important` kullaniyor. Bu, `index.css`'teki `.results-table` kurallari ile specificity savasindan kaynaklaniyor.

### 3.3 500+ Inline Style Kullanimi (Komponentlerde)

| Komponent | Inline `style=` Sayisi |
| ----------- | ------------------------ |
| `UserListPage.tsx` | 90 |
| `Settings.tsx` | 83 |
| `UserDetailPage.tsx` | 67 |
| `PolicyContent.tsx` | 44 |
| `AuditLogPanel.tsx` | 37 |
| `DatasourceAccessPanel.tsx` | 35 |

`UserListPage.tsx`'te 262 satirlik `const xxxStyle: React.CSSProperties = {...}` sabitleri var. Hic CSS class kullanmiyor (sadece 3 `className` referansi).

### 3.4 Duplicate CSS Kurallari

- `.card-header-row` → `index.css` ve `aiQuery.css`'te tanimli
- `.feedback-*` siniflari → `index.css` ve `aiQuery.css`'te tamamen kopyalanmis
- `.form-group` → `index.css` ve `auth.css`'te farkli degerlerle
- `.past-queries-toggle` → her iki dosyada farkli layout

### 3.5 Z-Index Yonetimi Yok

21 farkli z-index degeri, formal bir scale yok:

- 1-5: Content layer
- 40-60: Navigation
- 100: Popovers
- 1000: Modal
- **5000:** Select popover (kirmizi bayrak - modalin ustune cikiyor)

### 3.6 Tema Disi Hardcoded Renkler

`queryBuilder.css`'te notebook tag'ler: `#a5b4fc`, `#6ee7b7`, `#d8b4fe`, `#38bdf8` → light mode'da yanlis gorunur.

Admin komponentlerinde onlarca hardcoded hex renk: `#4b5563`, `#9ca3af`, `#10b981`, `#ef4444`, vs.

### 3.7 CSS Naming Tutarsizligi

- %60 flat class name (`.card`, `.btn-primary`)
- %20 BEM-ish yeni kod (`.ws-settings__section`)
- %20 legacy ad-hoc

---

## 4. Tip Guvenligi Sorunlari

### 4.1 44 Adet `any` Kullanimi

| Kategori | Sayi | Ornek |
| ---------- | ------ | ------- |
| `catch (err: any)` | 22 | Settings.tsx (9), SignInPage.tsx (3), auth pages (5) |
| `aiQuery/types.ts` prop'lari | 8 | `t: any`, `postData: <T>(url, body: any, options?: any)` |
| API response | 4 | `let data: any` (auth.ts, admin.ts) |
| WebAuthn credential | 2 | `credential: any` |
| Utility parametreleri | 3 | `rowsToChartData(rows: any[][])` |

### 4.2 Backend ile Tip Uyumsuzlugu

| Frontend Tipi | Backend Format | Sorun |
| --------------- | ---------------- | ------- |
| `Relation.relation_type` | `'one-to-many'` (hypenated) | Backend `'one_to_many'` (underscored) kullanir |
| `SemanticJoin.relationship` | `string` (tip guvenligi yok) | Olmali: `'many_to_one' \| 'one_to_many' \| ...` |
| `FilterClause.operator` | `string` | Olmali: 14 operatorun union type'i |
| `SemanticJoin.join_type` | `string` | Olmali: `'LEFT' \| 'INNER' \| 'RIGHT'` |

### 4.3 Table/TableRow/TableOption Ucuzlu Tip Duplikasyonu

Ayni entity icin 3 farkli tip:

- `types/metadata.ts` → `Table`
- `types/semantic.ts` → `TableRow` (id eklenmis)
- `components/aiQuery/types.ts` → `TableOption` (id cikarilmis)

---

## 5. i18n Sorunlari

### 5.1 Genel Durum

- **Toplam:** 1,322 key, 21 section, EN+TR
- **Custom implementasyon:** Kutuphanesiz, React Context tabanli
- **Tip guvenligi:** `TranslationKey` recursive mapped type ile tam compile-time safety
- **Eksik ceviri:** Sadece 2 key (1 EN eksik, 1 TR eksik) - cok iyi

### 5.2 Hardcoded String'ler (52 instance)

| Dosya | Sayi | Ornek |
| ------- | ------ | ------- |
| `TimeGrains.tsx` | ~20 | `t('key') \|\| 'English fallback'` anti-paterni |
| `QueryBuilder.tsx` | 7+ | `"Filter"`, `"Fields"`, `"Summarize"`, `"Row limit"` |
| `UserDetailPage.tsx` | ~15 | UUID label, loading text, hex renkler |
| `Evaluation.tsx` | ~10 | DEMO_DATA icerigi tamamen Ingilizce |
| `Dashboard.tsx` | 5 | `'👍'`, `'👎'` emoji kolon basliklari |

### 5.3 `t('key') || 'fallback'` Anti-Paterni

`TimeGrains.tsx`'te ~20 yerde bu patern var. Development'ta fallback her zaman truthy oldugu icin eksik ceviriler gizleniyor.

### 5.4 Locale Dosyalari Boyutu

- `en.ts`: 64KB (1,458 satir)
- `tr.ts`: 68KB (1,450 satir)
- Her ikisi de senkron import ediliyor (lazy loading yok)

---

## 6. Client-Side Memory & Performans

### 6.1 Bundle Boyutu Sorunlari

**Dist analizi:**

| Chunk | Boyut | Sorun |
| ------- | ------ | ------- |
| `chartConfig-*.js` | **410KB** | Recharts tum kutuphane, tree-shaking yetersiz |
| `index-*.js` | **366KB** | Ana bundle |
| `index-*.css` | **94KB** | CSS monolit |
| `Admin-*.js` | 65KB | Admin panel |
| `Modeling-*.js` | 53KB | Modeling |

**chartConfig.js 410KB** → Recharts'in tam import edilmesi muhtemel. Sadece kullanilan bilesenler import edilmeli (`ComposedChart`, `BarChart`, vs.).

### 6.2 Memory Tuketim Noktalari

1. **`useAIJobs.tsx` polling sistemi:** `setInterval` ile surekli polling. Component unmount olsa bile `settleDismiss()` ile promise resolve ediyor → state update unmounted component'te.

2. **`startBulkDescribe` fire-and-forget IIFE:** Hata durumunda `setBulkRunning(false)` unmounted component'te calisir.

3. **Locale dosyalari her ikisi de yuklu:** Sadece bir dil kullanilsa bile EN+TR ikisi de bundle'da.

4. **DEMO_DATA (Evaluation.tsx):** 60 satirlik inline JSON module scope'ta kalici olarak tutuluyor.

5. **`useApi` single AbortController:** Pes pese isteklerde onceki istekler iptal edilemiyor, response'lar memory'de birikir.

6. **Admin panel inline style objeleri:** Her render'da yeniden olusturuluyor (CSS sabit degil). `UserListPage.tsx`'te 262 satirlik style sabitleri bile komponent disina cikarilmamis.

### 6.3 Recharts Tree-Shaking

`utils/chartConfig.ts` su an sadece 333 byte ama `dist/chartConfig-*.js` 410KB. Bu, Recharts'in tam import edildigini gosteriyor.

**Cozum:**

```typescript
// YANLIS:
import { BarChart, ... } from 'recharts'

// DOGRU:
import BarChart from 'recharts/es6/charts/BarChart'
import { XAxis, YAxis } from 'recharts/es6/components'
```

### 6.4 CSS Performans

183KB CSS (sikistirilmamis) → gzip sonrasi ~25-30KB tahmini. Bu kabul edilebilir ama `index.css` monolit her sayfada yuklenir. CSS code-splitting yok.

---

## 7. UI/UX Best Practices (BI Dashboard)

### 7.1 Mevcut Durum Degerlendirmesi

**Iyi yapilanlar:**

- Dark/light theme destegi (46 CSS variable)
- Lazy loading tum sayfa komponentleri
- `prefers-reduced-motion` destegi
- Skip link ve `aria-current` accessibility
- ErrorBoundary her route icin ayri
- Responsive breakpoints (640px - 1340px)

**Eksik olanlar:**

- Keyboard shortcut destegi yok (BI araclarinda kritik)
- Undo/redo yok (modeling, query builder'da gerekli)
- Drag-drop siralama sadece TableBrowser'da var
- Toast/notification sistemi yok (alert() kullaniliyor)
- Global search yok (tablo, kolon, metric arama)
- Breadcrumb navigasyon yok (deep sayfalarda kaybolma)
- Loading skeleton yok (spinner var ama skeleton yok)
- Empty state tasarimi tutarsiz

### 7.2 BI Dashboard Arastirma Bulgulari

Dashboard tasarimi icin kritik prensipler (UXPin, NNGroup, ve endustri standartlarina gore):

**3-Saniye Kurali:** Bir dashboard 3 saniyede anlasilabilmeli. Biqly'de:

- Ana sayfa (`Dashboard.tsx`) sadece AI usage gosteriyor, is ozeti yok
- Kullanici en sik yaptigi islemlere hizli erisemiyor
- "Recent queries" veya "favorites" yok ana sayfada

**Gorsel Hiyerarsi:**

- KPI kartlari var (`KPICard.tsx`) ama sadece 512 byte (cok basit)
- Sparkline yok (trend gosterimi icin)
- Renk kodlama tutarsiz: basari icin yesil bazen `#10b981` bazen `#22c55e`

**Progressive Disclosure:**

- Query Builder'da 8 notebook step ayni anda gorunur → yeni kullanici icin korkutucu
- Modeling'de 27 useState ile yonetilen form → wizard/simdi modal ile yonetilmeli

### 7.3 Oncelikli UX Iyilestirmeleri

#### P0: Toast/Notification Sistemi

Su an `alert()`, `window.confirm()`, ve inline error mesajlari karisik kullaniliyor. Bir toast sistemi sart:

```text
src/components/ui/Toast.tsx          → Toast container + provider
src/hooks/useToast.ts               → useToast hook
src/components/ui/Toast.module.css   → Animasyonlu toast stilleri
```

#### P0: Command Palette (Cmd+K)

BI araclarinda (Metabase, Superset, Tableau) global search zorunlu:

- Tablo, kolon, metric, kaydedilmis soru arama
- Hizli navigate (datasource'e git, modeling'e git)
- Recent queries erisimi

#### P1: Keyboard Shortcuts

En azindan sunlar olmali:

- `Cmd+K` → Command palette
- `Cmd+Enter` → Query calistir
- `Cmd+S` → Kaydet
- `Cmd+Z` / `Cmd+Shift+Z` → Undo/redo
- `Escape` → Modal kapat / Filter kapat

#### P1: Loading Skeletons

Spinner yerine skeleton loading:

- Tablo listesi icin satir skeleton
- Chart icin area/bar skeleton
- Form icin input skeleton

#### P1: Breadcrumb Navigasyon

Modeling → Model detay → Metric duzenleme gibi derin sayfalarda:

```text
Datasources > AdventureWorks > Sales Model > Metrics > total_revenue
```

#### P2: Empty State Tasarimi

`EmptyState.tsx` var (791 byte) ama cok basit. Her bolum icin ozel empty state:

- "Henuz datasource eklenmedi" → Datasource ekleme CTA
- "Henuz sorgu calistirilmadi" → Ornek soru onerileri
- "Model bulunamadi" → Model olusturma rehberi

#### P2: Responsive Table

Su an tablolar mobilde kiriliyor. Icinerik:

- Kolon gizleme (responsive breakpoints'e gore)
- Horizontal scroll ile touch destegi
- Satir detay modal (mobilde tam ekran)

#### P2: Kullanici Onboarding

BI araclari kompleks. Onboarding flow:

- Ilk datasource ekleme wizard
- Ilk semantic model olusturma rehber
- Ilk AI query denemesi

### 7.4 BI Dashboard Tasarim Desenleri (Uygulanmasi Gerekenler)

| Desen | Mevcut Durum | Hedef | Oncelik |
| ------- | ------------- | ------- | --------- |
| Toast/Notification | `alert()` kullaniliyor | Merkezi toast sistemi | P0 |
| Command Palette (Cmd+K) | Yok | Global arama + navigate | P0 |
| Undo/Redo | Yok | Query builder + modeling | P1 |
| Keyboard Shortcuts | Yok | En az 5 shortcut | P1 |
| Loading Skeletons | Spinner | Skeleton per bilesen | P1 |
| Breadcrumb | Yok | Deep sayfalarda | P1 |
| Data Export | Yok | CSV/Excel export | P1 |
| Global Filters | Per-component | Tarih range, datasource filtresi | P2 |
| Recent/Favorites | Yok | Ana sayfada son sorgular | P2 |
| Responsive Tables | Kirik | Kolon gizleme + touch | P2 |
| Onboarding | Yok | Adim adim rehber | P2 |

---

## 8. Oncelikli Aksiyon Plani

### Faz 1: Kritik (1-2 hafta)

- [x] **CSRF fix:** `useApi` hook'una CSRF token eklenmeli. `csrfFetch()` merkezi hale getirilmeli.
- [x] **API client birlestirme:** Tek `apiClient` modulu olusturulmali. Response parser, auth header, CSRF, error handling tek noktada.
- [x] **Admin inline style → CSS:** `UserListPage.tsx`, `UserDetailPage.tsx`, `Settings.tsx`, `AuditLogPanel.tsx`, `DatasourceAccessPanel.tsx` toplam ~400 inline style CSS class'a donusturulmeli.
- [x] **Shared admin styles:** `sharedAdminStyles.ts` olusturulmali (3 dosyada kopyalanan style sabitleri icin).
- [x] **`window.confirm()` → `useConfirm`:** 4 dosyada duzeltilmeli.

### Faz 2: Hook Extraction (1 hafta)

- [x] `useDatasources()` hook'u → 4 dosyadaki datasource yukleme paterni
- [x] `useSemanticModels(datasourceId)` hook'u → 4 dosyadaki model yukleme paterni
- [x] `useModelDetail(modelId)` hook'u → 3 dosyadaki model detay paterni
- [x] `useAdminLookups()` hook'u → AuditLogPanel + DatasourceAccessPanel'deki ortak lookup
- [x] `columnRefMatchesTable` → `modeling/utils.ts`'a tasinmali
- [x] `usePasskeyRegistration` hook'u → Settings.tsx'den cikarilmali

### Faz 3: God Component Parcalama (2 hafta)

- [ ] **Modeling.tsx (1,332 satir) → 4 dosya:**
  - `ModelingPalette.tsx` (~270 satir)
  - `JoinEditor.tsx` (~75 satir)
  - `useEntityActions.ts` hook (delete/reactivate/rename pattern)
  - Ana dosya ~600 satira iner

- [ ] **Settings.tsx (955 satir) → 5 dosya:**
  - `PasskeyTable.tsx`
  - `MFASection.tsx`
  - `RecoveryCodesDisplay.tsx`
  - `OTPCodeInput.tsx`
  - Ana dosya ~200 satira iner

- [ ] **QueryBuilder.tsx (950 satir) → 3 dosya:**
  - `NotebookStep.tsx` (wrapper component, 8 kez tekrarlanan patern)
  - `FilterStep.tsx`, `SummarizeStep.tsx`, `SortStep.tsx`, vs.
  - Ana dosya ~300 satira iner

- [ ] **Evaluation.tsx (667 satir) → 3 dosya:**
  - `EvalRunTab.tsx`
  - `EvalHistoryTab.tsx`
  - `EvalRegressionTab.tsx`

- [ ] **UserListPage.tsx (845 satir) → 3 dosya:**
  - `ActiveUsersTab.tsx`
  - `InvitationsTab.tsx`
  - `InviteUserModal.tsx`

- [ ] **SavedQuestions.tsx (816 satir) → 2 dosya:**
  - `QuestionDetailPane.tsx`
  - `SavedQuestionFormModal.tsx` (zaten kismen cikarilmis ama 28 prop → reduce)

### Faz 4: CSS Refactoring (1 hafta)

- [ ] `index.css` (107KB) → domain dosyalarina bolunmeli:
  - `sidebar.css`, `modal.css`, `table-results.css`, `ai-jobs.css`, `workspace.css`, `sharing.css`, `bulk-describe.css`
- [ ] `tableBrowser.css` → 59 `!important` duzeltilmeli (selector specificity ile cozulmeli)
- [ ] Duplicate CSS kurallari kaldirilmali (feedback-*, card-header-row, form-group, past-queries-toggle)
- [ ] Z-index scale: `--z-content: 1-5`, `--z-nav: 40-60`, `--z-popover: 100`, `--z-modal: 1000`, `--z-select: 1100`
- [ ] Hardcoded renkler → CSS variable ile degistirilmeli
- [ ] Monospace font: 6 farkli font-family stack → tek `--font-mono` variable

### Faz 5: Router (1 hafta)

- [ ] `react-router-dom` entegrasyonu
- [ ] Lazy loading route'lar
- [ ] `useNavigate()` hook ile prop drilling bitirmeli
- [ ] Auth route'lari tek yerde tanimlanmali (su an 3 yerde)
- [ ] Route params destegi (`/model/:id`)
- [ ] Query string parsing

### Faz 6: Tip Guvenligi (1 hafta)

- [ ] 22 adet `catch (err: any)` → `catch (err: unknown)`
- [ ] `aiQuery/types.ts` → 8 `any` prop tipi duzeltilmeli
- [ ] `FilterClause.operator` → union type (14 operator)
- [ ] `SemanticJoin.relationship` → union type
- [ ] `SemanticJoin.join_type` → `'LEFT' | 'INNER' | 'RIGHT'`
- [ ] `Relation.relation_type` → backend ile uyumlu (underscore format)
- [ ] Table/TableRow/TableOption → `Pick`/`Omit` ile turetilmeli

### Faz 7: Performans & Memory (1 hafta)

- [ ] Recharts tree-shaking → sadece kullanilan bilesenler import edilmeli (410KB → ~100KB)
- [ ] Locale dosyalari lazy loading (admin/auth section'larini ayri chunk yap)
- [ ] `useApi` → per-request AbortController
- [ ] `startBulkDescribe` → fire-and-forget yerine proper async management
- [ ] Admin style sabitleri → module scope'a tasinmali (her render'da yeniden olusturma)
- [ ] `DEMO_DATA` → ayri dosyaya tasinmali ve lazy yuklenmeli

### Faz 8: UX Iyilestirmeleri (2-3 hafta)

- [ ] Toast/notification sistemi olustur
- [ ] Command Palette (Cmd+K) ekle
- [ ] Keyboard shortcuts sistemi kur
- [ ] Loading skeleton'lari ekle
- [ ] Breadcrumb navigasyonu ekle
- [ ] Query Builder'da progressive disclosure (acordion veya wizard modu)
- [ ] Data export (CSV/Excel) ozelligi
- [ ] Empty state tasarimini iyilestir
- [ ] Ana sayfa redesign: recent queries + favorites + quick actions
- [ ] Responsive table duzeltmeleri

---

## Dosya Boyutu Ozeti (Kaynak Kodu)

| Dosya | Boyut | Satir | Durum |
| ------- | ------- | ------- | ------- |
| `index.css` | 107KB | 5,092 | Parcalanmali |
| `Modeling.tsx` | 54KB | 1,332 | Parcalanmali |
| `TableBrowser.tsx` | 40KB | 1,111 | Parcalanmali |
| `QueryBuilder.tsx` | 39KB | 950 | Parcalanmali |
| `Settings.tsx` | 36KB | 955 | Parcalanmali |
| `UserListPage.tsx` | 30KB | 845 | Parcalanmali |
| `en.ts` | 62KB | 1,458 | Lazy loading |
| `tr.ts` | 66KB | 1,450 | Lazy loading |
| `aiQuery.css` | 31KB | ~800 | Duplicate temizle |
| `SavedQuestions.tsx` | 28KB | 816 | Parcalanmali |
| `Evaluation.tsx` | 29KB | 667 | Parcalanmali |
| `AddMetricModal.tsx` | 28KB | ~600 | Incele |
| `App.tsx` | 21KB | 628 | Router degistir |
| `FewShotExamples.tsx` | 21KB | 559 | Parcalanmali |
| `PromptTemplates.tsx` | 19KB | 555 | Orta |
| `UserDetailPage.tsx` | 19KB | 616 | Parcalanmali |
| `Datasources.tsx` | 21KB | 555 | Parcalanmali |
| `tableBrowser.css` | 15KB | ~400 | !important temizle |
| `auth.css` | 11KB | ~300 | Duplicate temizle |
| `useAIJobs.tsx` | 23KB | ~550 | API birlestir |

---

## Tahmini Etki

| Metrik | Su An | Sonra |
| -------- | ------- | ------- |
| En buyuk dosya | 1,332 satir | ~400 satir |
| API pattern sayisi | 4 | 1 |
| Inline style sayisi | 500+ | ~50 |
| `!important` sayisi | 79 | ~10 |
| `any` tip sayisi | 44 | ~5 |
| Bundle size (chartConfig) | 410KB | ~100KB |
| CSS monolit | 107KB tek dosya | ~15KB x 7 dosya |
| CSRF kapsami | ~30% | ~100% |
| API response parser | 5 farkli | 1 |
