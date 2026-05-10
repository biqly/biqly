# Backend Geliştirmeleri → Frontend Değişiklikleri

> `docs/text-to-sql-audit.md` dosyasındaki her backend geliştirmesinin frontend tarafında gerektirdiği UI/UX değişiklikleri.
> Tüm checkbox'lar boş — yapıldıkça işaretleriz.

---

## 1. Disambiguation (Niyet Anlama)

### Backend
AI soruyu belirsiz bulduğunda clarification sorusu döner.

### Frontend Değişiklikleri

#### 1.1 AIQuery.tsx — Clarification UI
- [x] `result.needs_clarification === true` durumunda soru input'unun altında **clarification kartı** göster
- [x] Kartta: `result.clarification_question` metni + önerilen seçenekler (`result.clarification_options?: string[]`)
- [x] Kullanıcı bir seçeneğe tıklayınca `question` state'ine ekle ve tekrar query gönder
- [x] "Skip" butonu — kullanıcı clarification'ı atlayıp mevcut sonuçla devam edebilmeli

```
┌─────────────────────────────────────────────────┐
│  🤔 AI needs clarification                      │
│                                                 │
│  "Best selling" ifadesi belirsiz. Ne demek      │
│  istiyorsunuz?                                  │
│                                                 │
│  ┌─────────────────┐  ┌─────────────────┐       │
│  │ En çok sipariş  │  │ En yüksek gelir │       │
│  └─────────────────┘  └─────────────────┘       │
│                                                 │
│  [Skip — show whatever you have]                │
└─────────────────────────────────────────────────┘
```

#### 1.2 AIQuery.tsx — Multi-Turn Conversation
- [x] `useApi` yanında `useConversation` custom hook oluştur — `messages: {role, content, timestamp}[]` state tutar
- [x] Her AI sorgusu ve yanıtı conversation'a eklenir
- [x] Soru input'unun yanına "New conversation" butonu ekle
- [x] Backend'e `conversation_id` parametresi ekle (gelecekte backend desteklediğinde)
- [x] Conversation history'yi `localStorage`'da sakla

#### 1.3 API Response Type Güncellemesi
- [x] `AIQueryResponse` type'ına `needs_clarification?: boolean` ekle
- [x] `AIQueryResponse` type'ına `clarification_question?: string` ekle
- [x] `AIQueryResponse` type'ına `clarification_options?: string[]` ekle

---

## 2. Self-Consistency / Multi-Candidate Generation

### Backend
Aynı soru için 2-3 LogicalQuery adayı üretilir, en tutarlı seçilir.

### Frontend Değişiklikleri

#### 2.1 AIQuery.tsx — Candidate Comparison UI
- [x] `result.candidates?: LogicalQueryCandidate[]` array'i varsa **karşılaştırma paneli** göster
- [x] Her candidate için: LogicalQuery JSON preview + confidence score
- [x] Kullanıcı "Bu daha iyi" diyebilmeli → seçilen candidate ile execute
- [x] "Auto-select (recommended)" butonu — en yüksek skorlu candidate'ı otomatik seç

```
┌─────────────────────────────────────────────────┐
│  🔄 3 candidates generated                       │
│                                                  │
│  ┌──────────────┐ ✗ ┌──────────────┐ ✓ ┌──────┐ │
│  │ Candidate #1 │   │ Candidate #2 │   │ #3   │ │
│  │ Score: 0.72  │   │ Score: 0.89  │   │ 0.65 │ │
│  │ ...JSON...   │   │ ...JSON...   │   │      │ │
│  └──────────────┘   └──────────────┘   └──────┘ │
│                                                  │
│  [Use Candidate #2 (recommended)]  [Auto-select]│
└─────────────────────────────────────────────────┘
```

#### 2.2 Loading State Güncellemesi
- [x] Backend multi-candidate üretirken "Generating candidates…" loading text göster
- [x] Progress indicator: "Candidate 1/3…", "Candidate 2/3…"
- [x] Backend'den gelen `candidates_count` ile progress göster

#### 2.3 API Response Type Güncellemesi
- [x] `AIQueryResponse` type'ına `candidates?: { logical_query: LogicalQuery; confidence: number; reasoning?: string }[]` ekle

---

## 3. Validation & Re-prompting

### Backend
Validation başarısız → hata mesajı LLM'e geri gönder → düzeltilmiş JSON.

### Frontend Değişiklikleri

#### 3.1 AIQuery.tsx — Retry Feedback UI
- [x] Backend retry yapıyorsa `result.retry_count` göster
- [x] "AI self-corrected (attempt 2/3)" badge'i göster
- [x] Her retry'ın ürettiği LogicalQuery diff'ini toggle ile göster/gizle

#### 3.2 Dry-Run / EXPLAIN Result
- [x] Backend `result.validation_result` dönerse: EXPLAIN çıktısını "SQL Plan" collapsible bölümünde göster
- [x] Warning'ler varsa: her warning için `[Fix suggested]` badge + diff view
- [x] Başarılı EXPLAIN sonrası otomatik execute yap (mevcut `runAndExecute` davranışını koru)

#### 3.3 Error Recovery UI
- [x] Validation başarısız + retry limit aşıldıysa: "AI couldn't generate a valid query" mesajı
- [x] Kullanıcıya seçenekler: "Rephrase question" (input'a odaklan), "Switch model", "Manual query builder"
- [x] Hatalı LogicalQuery'yi göster ki kullanıcı neyin yanlış anlaşıldığını görsün

#### 3.4 API Response Type Güncellemesi
- [x] `AIQueryResponse` type'ına `retry_count?: number` ekle
- [x] `AIQueryResponse` type'ına `validation_result?: { explain_output?: string; plan_ok: boolean }` ekle

---

## 4. Context Building & Table Retrieval (Vector Search)

### Backend
Tablo seçimi keyword + vector similarity ile yapılır.

### Frontend Değişiklikleri

#### 4.1 AIQuery.tsx — Gelişmiş Table Router Bilgisi
- [x] `result.table_routing.ranking_method` göster: `"keyword"` veya `"vector"` veya `"hybrid"`
- [x] Her candidate tablo için **relevance score bar** göster (görsel feedback)
- [x] Tablo seçim nedeni: `result.table_routing.reasoning` metnini göster

```
┌─────────────────────────────────────────────────┐
│  📋 Table Routing (hybrid — vector + keyword)    │
│                                                  │
│  public.orders        ████████████████  94%  ✓    │
│  public.customers     ██████████████    82%  ✓    │
│  public.products      ██████████        61%  ✓    │
│  public.categories    ████              23%       │
│                                                  │
│  Reasoning: "orders" matches question tokens,    │
│  FK chain: orders → customers (many-to-one)      │
└─────────────────────────────────────────────────┘
```

#### 4.2 AIQuery.tsx — Interactive Table Routing Override
- [x] Kullanıcı otomatik seçilen tabloları **kendi değiştirebilmeli** — mevcut `selectedTables` multiselect'i ile
- [x] "Auto" vs "Manual" toggle ekle — auto = backend seçer, manual = kullanıcı seçer
- [x] Kullanıcı manual seçim yaptığında table routing bypass edilir

#### 4.3 Sample Data Preview
- [x] Tablo seçildikten sonra "Preview sample data" butonu — `/api/datasources/{id}/tables/{table}/sample` endpoint
- [x] Sample data modal: 5-10 satır preview → kullanıcı veriyi görebilir

---

## 5. Prompt Engineering İyileştirmeleri

### Backend
Few-shot examples, chain-of-thought, sample data in prompt.

### Frontend Değişiklikleri

#### 5.1 AIQuery.tsx — Prompt Transparency
- [x] "Show prompt" collapsible bölüm — backend'den gelen `result.prompt` (mevcut `prompt` field zaten var ama UI'da gösterilmiyor)
- [x] Token count gösterimi: `result.token_count?.{ prompt, completion, total }`
- [x] Prompt boyutu uyarısı: "Prompt is large (45K tokens), may affect quality"

#### 5.2 Few-Shot Example Manager (Yeni Component)
- [x] Admin panelinde "Few-Shot Examples" CRUD sayfası
- [x] Her example: `question` + `logical_query` + `tags[]` + `dialect`
- [x] `GET /api/ai/examples` — örnekleri listele
- [x] `POST /api/ai/examples` — yeni örnek ekle
- [x] `DELETE /api/ai/examples/{id}` — örnek sil
- [ ] Backend'e `example_ids?: string[]` parametresi ile specific few-shot'lar gönder

#### 5.3 Query History → Few-Shot UI
- [ ] Saved Questions sayfasında her kaydedilmiş soru için "Use as AI example" checkbox
- [ ] İşaretlenen sorular `few_shot_examples` tablosuna kaydedilir
- [ ] AI Query sayfasında "Include my past queries as examples" toggle

---

## 6. Evaluation & Testing

### Backend
Golden dataset, LLM-as-judge, execution accuracy.

### Frontend Değişiklikleri

#### 6.1 Evaluation Dashboard (Yeni Component)
- [x] "Evaluation" sayfası route'a ekle: `{ path: '/evaluation', label: 'Evaluation', ... }`
- [x] Golden dataset listesi + her test case'in durumu (pass/fail)
- [x] Accuracy metrics: pie chart (pass rate), trend line (accuracy over time)
- [x] "Run evaluation" butonu — tüm golden dataset'i çalıştır

#### 6.2 AIQuery.tsx — User Feedback
- [ ] Her AI sonucunun yanında 👍/👎 butonları
- [ ] Thumbs up → store in `ai_query_history` as `user_rating: positive`
- [ ] Thumbs down → show feedback form: "What went wrong?" (free text + categories)
- [ ] Feedback data'yi backend'e gönder → `POST /api/ai/feedback`

#### 6.3 AIQuery.tsx — Result Comparison
- [ ] "Compare with expected" toggle — golden dataset'teki beklenen sonuçla karşılaştır
- [ ] Diff view: expected vs actual LogicalQuery

---

## 7. Dynamic Confidence Scoring

### Backend
Hardcoded 0.8 kaldırılıp dynamic scoring yapılır.

### Frontend Değişiklikleri

#### 7.1 AIQuery.tsx — Confidence Visualization
- [x] Mevcut basit `Confidence: 80%` yazısını **progress bar** ile değiştir
- [x] Confidence seviyelerine göre renk: >0.8 yeşil, 0.5-0.8 sarı, <0.5 kırmızı
- [x] Confidence breakdown göster: `table_routing_confidence`, `llm_confidence`, `validation_confidence`
- [x] Düşük confidence uyarısı: action suggestion ile ("Try being more specific", "Select tables manually")

```
┌─────────────────────────────────────────┐
│  Confidence: ████████████████░░░  84%    │
│                                          │
│  Table routing:  ██████████████████  95% │
│  LLM output:    ████████████░░░░░  78%   │
│  Validation:    ██████████████████ 100%  │
└─────────────────────────────────────────┘
```

#### 7.2 API Response Type Güncellemesi
- [x] `AIQueryResponse` type'ına `confidence_breakdown?: { table_routing: number; llm: number; validation: number }` ekle

---

## 8. Multi-Model Orchestration

### Backend
Farklı LLM provider'ları ve model routing.

### Frontend Değişiklikleri

#### 8.1 AIQuery.tsx — Model Selector
- [x] Soru input'unun yanına **model selector dropdown** ekle
- [x] Seçenekler: `OpenAI GPT-4o`, `GPT-4o-mini`, `Claude 3.5`, `Local Llama` vb.
- [x] `POST /api/ai/query` request'ine `model?: string` parametresi ekle
- [x] Her modelin yanında cost indicator: `$` / `$$` / `$$$`

#### 8.2 AIQuery.tsx — Model Fallback Indicator
- [ ] Backend fallback yaptığında `result.model_used` ile hangi model kullanıldığını göster
- [ ] "Primary model (GPT-4o) failed, used fallback (GPT-4o-mini)" uyarısı

#### 8.3 Streaming Responses
- [x] `EventSource` veya `fetch` streaming ile LogicalQuery JSON'ı parça parça göster
- [x] Typing effect: LogicalQuery alanları yavaşça dolsun (UX iyileştirmesi)
- [x] "Generating…" yerine "Reading schema…" → "Selecting tables…" → "Building query…" aşama göstergesi

---

## 9. Conversation Memory

### Backend
Session bazlı conversation, context carry-over.

### Frontend Değişiklikleri

#### 9.1 AIQuery.tsx — Chat UI
- [x] Mevcut tek-question UI'ı **chat-style UI**'a dönüştür
- [x] Mesaj baloncukları: kullanıcı (sağ) → AI (sol)
- [x] Her AI yanıtı: LogicalQuery + SQL + result tablosu baloncuk içinde
- [x] Scrollable mesaj listesi

```
┌──────────────────────────────────────┐
│                                      │
│        "Geçen ayki toplam gelir?"    │  ← user
│                                      │
│  ┌──────────────────────────────┐    │
│  │ SQL: SELECT SUM(...)         │    │  ← AI
│  │ Result: 15 tablo             │    │
│  │ Confidence: 91%              │    │
│  └──────────────────────────────┘    │
│                                      │
│    "Bunu product bazında kır"        │  ← user (follow-up)
│                                      │
│  ┌──────────────────────────────┐    │
│  │ (same query + GROUP BY       │    │  ← AI (context carry-over)
│  │  product_name)               │    │
│  └──────────────────────────────┘    │
│                                      │
│  ┌─────────────────────────────┐     │
│  │  Ask a follow-up...         │ [→] │
│  └─────────────────────────────┘     │
└──────────────────────────────────────┘
```

#### 9.2 Conversation Sidebar
- [x] Sol sidebar'da "Recent conversations" listesi
- [x] Her conversation: ilk soru + timestamp
- [x] Conversation silme / rename
- [x] `localStorage` veya backend'de conversation persistence

#### 9.3 Context Indicator
- [x] "Context: 3 previous questions in this conversation" badge
- [ ] Follow-up sorularda applied filters'i göster: "Inheriting: date >= 2026-01-01"

---

## 10. Advanced SQL Features (HAVING, CTE, Window Functions)

### Backend
LogicalQuery genişletilir.

### Frontend Değişiklikleri

#### 10.1 QueryBuilder.tsx — Advanced Query Modes
- [x] "Simple" / "Advanced" toggle
- [x] Advanced modda:
  - [x] HAVING clause builder (aggregation sonrası filtre)
  - [x] Window function selector (ROW_NUMBER, RANK, LAG/LEAD)
  - [x] CTE builder ("WITH ... AS")
- [x] Her yeni LogicalQuery alanı için form UI

#### 10.2 AIQuery.tsx — Advanced Feature Detection
- [ ] Kullanıcı "ranking", "top 5", "previous value" gibi ifadeler kullandığında AI window function kullanır
- [ ] Backend ürettiğinde frontend "Uses: Window Function (ROW_NUMBER)" badge göster
- [ ] SQL preview'de window function vurgulanarak gösterilsin

---

## 11. Data Visualization Hints

### Backend
Sonuçlara göre chart tipi önerisi.

### Frontend Değişiklikleri

#### 11.1 AIQuery.tsx — Auto Chart
- [x] `result.visualization_hint?: { chart_type: 'bar' | 'line' | 'pie' | 'table'; reason: string }` göster
- [x] Otomatik chart type seçimi backend önerisine göre
- [x] "Why this chart?" tooltip: "Bar chart recommended because data has categorical dimension + single metric"

#### 11.2 ResultTable Improvements
- [x] Kolon header'ına tıklayınca sort (client-side)
- [x] Kolon header'ına right-click → "Filter by this value" → AI'ya follow-up soru gönder
- [ ] Cell değerine tıklayınca drill-down: "Show details for [value]" → yeni AI query

---

## 12. Cost Tracking & Observability

### Backend
Token bazlı maliyet, LLM latency tracking.

### Frontend Değişiklikleri

#### 12.1 AIQuery.tsx — Cost & Performance Display
- [x] Her AI yanıtı sonrası: `⏱ 1.2s · 🪙 $0.003 (847 tokens)`
- [x] `result.token_usage?: { prompt: number; completion: number; total: number }`
- [x] `result.cost_usd?: number` göster
- [x] Tooltip: detaylı token breakdown

#### 12.2 Dashboard.tsx — AI Usage Analytics
- [ ] "AI Usage" section: total queries, success rate, avg latency, total cost
- [ ] Time-series chart: daily AI query count
- [ ] Top-10 soru listesi (most frequent)

---

## 13. useApi.ts & Shared Type Güncellemeleri

### 13.1 Shared Types
- [x] `frontend/src/types/` klasörü oluştur
- [x] `types/ai.ts`: `AIQueryRequest`, `AIQueryResponse`, `LogicalQuery`, `TableRoutingResult`, `ConversationMessage`
- [ ] `types/query.ts`: `LogicalQueryPayload`, `QueryResult`, `CompiledQuery`
- [x] `types/metadata.ts`: `Datasource`, `Table`, `Column`, `Relation`
- [ ] `types/semantic.ts`: `SemanticModel`, `Dimension`, `Metric`, `Join`

### 13.2 useApi Güncellemeleri
- [x] `useConversation` hook: conversation state management
- [x] `useStreamingApi` hook: SSE/streaming response handling
- [x] Error handling iyileştirmesi: network error retry, timeout handling

---

## Öncelik Matrisi

| # | Backend Geliştirme | Frontend Geliştirme | Öncelik | Tahmini Efor |
|---|---|---|---|---|
| 1 | Disambiguation | Clarification UI + Multi-turn | P0 | 3 gün |
| 2 | Re-prompting | Retry feedback UI | P0 | 1 gün |
| 3 | Dynamic confidence | Confidence bar + breakdown | P0 | 1 gün |
| 4 | EXPLAIN validation | SQL Plan view | P0 | 1 gün |
| 5 | Multi-candidate | Comparison panel | P1 | 3 gün |
| 6 | Few-shot from history | Example manager + feedback | P1 | 3 gün |
| 7 | Sample data in prompt | Sample preview modal | P1 | 1 gün |
| 8 | Vector table retrieval | Router visualization + override | P1 | 2 gün |
| 9 | Conversation memory | Chat UI + sidebar | P2 | 5 gün |
| 10 | Multi-model support | Model selector + cost display | P2 | 2 gün |
| 11 | Streaming | Streaming hook + typing effect | P2 | 2 gün |
| 12 | Advanced SQL | Advanced query builder | P2 | 3 gün |
| 13 | Evaluation dashboard | New page + charts | P2 | 3 gün |
| 14 | Visualization hints | Auto chart + drill-down | P3 | 2 gün |
| 15 | Cost tracking | Cost badge + analytics | P3 | 1 gün |
