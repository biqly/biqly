# Text-to-SQL Audit: Mevcut Durum, Eksikler ve İyileştirme Planı

> Bu doküman, Biqly projesinin text-to-SQL yaklaşımını endüstri standartlarına (Google Cloud, AWS, Vanna AI, Chat2DB, BIRD-bench) göre denetler. Her madde bir checkbox ile işaretlenir — yapıldıkça check ederiz.

**Son güncelleme (2026-05):** Retry + dry-run (EXPLAIN), self-consistency, vektör tablo embedding’leri, çok sağlayıcı (OpenAI / Anthropic), golden eval HTTP API (`POST /api/ai/eval/run`, `GET /api/ai/eval/run/stream`), konuşma bağlamı (`prior_turns`), few-shot + örnek satırlar (Run yolunda), dinamik confidence ve LLM clarification tamamlandı veya kısmen tamamlandı; aşağıdaki maddeler kod durumuna göre işaretlendi.

## 1. Mimari Temel Yaklaşım (Doğru Yapılanlar)

Biqly'nin temel tasarımı **endüstride en güvenli ve önerilen yaklaşımlardan biridir**:

- AI doğrudan SQL üretmez → `LogicalQuery` JSON üretir
- Backend SQL'i derler → dialect-specific, parameterized
- Semantic layer → AI sadece tanımlı dimension/metric kullanabilir
- Validation → JSON schema + semantic model doğrulama
- Table Router → model_id verilmezse metadata'dan tablo seçimi

> **Referans**: AWS "prompt engineering without fine-tuning" pattern, Google Cloud "semantic layer over raw data" yaklaşımı ile uyumlu.

---

## 2. Kritik Eksikler ve Hatalar

### 2.1 Disambiguation (Niyet Anlama)

Mevcut durum: Tablo yönlendirmesi belirsizse veya parse/validation başarısızsa `tryGenerateClarification` ile kısa bir netleştirme sorusu üretilir; API `needs_clarification`, `clarification_question`, `clarification_options` döner. Konuşma geçmişi `prior_turns` ile prompt’a eklenir.

- [x] **LLM-driven disambiguation**: Parse/validation sonrası (ve tablo-router ihtiyacında) ikinci LLM çağrısı ile kısa clarification
- [x] **Multi-turn conversation context**: `prior_turns` + `WithPriorTurns`; frontend `useConversation` ile oturum geçmişi
- [x] **Clarification response type**: `needs_clarification`, `clarification_question`, `clarification_options` şemada ve handler’da
- [x] **Follow-up soru Parsing**: Önceki tur metin + mümkünse `logical_query` JSON’u prompt’ta; tam anlam birleştirme (coreference) LLM’e bırakıldı — iyileştirme alanı

> **Referans**: Google Cloud — "Disambiguation using LLMs: getting the system to respond with a clarifying question when faced with a question that is not clear enough"

### 2.2 Self-Consistency / Multi-Candidate Generation

Mevcut durum: `BI_AI_MULTI_CANDIDATE_COUNT` > 1 iken `tryMultiCandidate` adım sıcaklığı (`base + 0.2*i`), çoğunluk oylaması ve dry-run (varsa) ile seçim. Tek aday + retry yolunda `computeConfidence` (validation/retry cezası).

- [x] **Multi-candidate generation**: `MultiCandidateCount` ile N tamamlama
- [x] **Voting mechanism**: `logicalQueryFingerprint` ile aynı yapıya oy çoğunluğu
- [x] **Dynamic confidence scoring**: Self-consistency’de `winnerCount/n`; tek yolda `computeConfidence`
- [x] **Temperature tuning**: Adaylar için `baseTemperature + 0.2*i` (üst sınır 1)

> **Referans**: Google Cloud — "Self-consistency: generate multiple queries for the same user question, potentially using different prompting techniques or model variants, and pick the best one"

### 2.3 Validation & Re-prompting

Mevcut durum: `ProcessQuestion` içinde parse/validation/dry-run hatasında `BuildRetry` ile LLM’e geri bildirim; `MaxRetries` config ile sınırlı. Run handler’da dialect `ExplainSQL` ile dry-run (SQL Server’da EXPLAIN yoksa no-op).

- [x] **Retry loop**: `BuildRetry(originalPrompt, raw, failureMsg)`
- [x] **Max retry limit**: `BI_AI_MAX_RETRIES` (config)
- [x] **Dry-run validation**: `newSQLDryRunValidator` → `ExplainSQL` + `QueryContext`
- [x] **Error feedback to LLM**: `failureMessageFor` (parse, dry-run, validation uyarıları)

> **Referans**: Google Cloud — "Validation and reprompting: pass back the mistake to the model for a second pass. When provided an example of a mistake, models can typically address what they got wrong."

### 2.4 Context Building & Table Retrieval

Mevcut `TableRouter` keyword + (yapılandırıldıysa) önceden hesaplanmış tablo embedding’leri ile hibrit skor. Auto-model için boyut/metric kotası (`maxAutoModelDimensions` vb.) ile prompt sınırı.

- [x] **Vector embedding tablo seçimi**: `POST /api/ai/metadata/embed`, `EmbeddingReader` + cosine benzerliği
- [x] **Türkçe metadata açıklamaları**: AI Describe varsayılan olarak Türkçe iş açıklaması üretir; teknik tablo/kolon adlarını açıklamada koruyarak schema ile köprü kurar
- [ ] **Column-level retrieval**: Kolonlar öncelik + kota ile budanıyor; soruya göre semantik kolon alt kümesi seçimi henüz yok
- [ ] **Schema partitioning**: Tablo kapsamı UI ile manuel daraltma var; otomatik şema kümeleme yok
- [x] **Sample data in prompt**: `POST /api/ai/query/run` yolunda `WithSampleData` (`/ai/query` ve `/ai/query/preview` örnek satır içermez)
- [x] **Query history as context**: Few-shot + `include_past_queries` / `example_ids`

> **Referans**: AWS — "Vector embeddings created from a central data catalog can be supplied to an LLM to generate relevant and precise SQL responses"

---

## 3. Prompt Engineering İyileştirmeleri

### 3.1 Few-Shot Examples

- [x] **Dynamic few-shot selection**: Geçmiş / kayıtlı örnekler `loadFewShotExamples*` ile prompt’a
- [x] **Curated example library**: `GET/POST/DELETE /api/ai/examples` + UI `FewShotExamples`
- [ ] **Dialect-specific examples**: Çıktı LogicalQuery; derleyici dialect üstleniyor — prompt’ta dialect’e özel SQL örnekleri yok
- [ ] **Failure examples**: Prompt’ta “bunu yapma” örnek seti yok

### 3.2 Prompt Structure

- [ ] **Chain-of-thought prompting**: Açık adım adım “önce tablolar…” zinciri yok; kurallar uzun metin olarak var
- [x] **Structured output instruction**: JSON-only ve şema kuralları prompt’ta
- [ ] **Business glossary section**: Harici sözlük / RAG yok
- [x] **Date/time relative reference handling**: Tarih grain, ISO filtre, `having`/window talimatları prompt’ta

### 3.3 Prompt Size Management

- [ ] **Progressive context loading**: Retry’da otomatik genişletme yok
- [ ] **Token counting**: Yaklaşık token log’u yok (yalnızca rune kotası `maxPromptRunes`)
- [ ] **Context window adaptation**: Model kartına göre dinamik bütçe yok

---

## 4. Evaluation & Testing

### 4.1 Evaluation Framework

- [x] **Golden dataset**: `DefaultGoldenCases()` + `LogicalQueryEqual` + testler; büyük (50–100) set henüz yok
- [ ] **Execution accuracy test**: Golden set yalnızca LogicalQuery eşleşmesi; üretilen SQL’in sonuç kümesi karşılaştırması yok
- [ ] **LLM-as-a-judge**: Yok
- [ ] **Benchmark suite**: İç BIRD-benzeri tam pipeline benchmark yok
- [ ] **Regression testing**: Prompt değişiminde zorunlu eval gate (CI) yok; `BI_AI_GOLDEN_EVAL=1` ile isteğe bağlı live test var

### 4.2 Metrics

- [x] **Exact match accuracy**: Eval UI / `POST /api/ai/eval/run` pass_rate
- [ ] **Execution accuracy**: Yok
- [ ] **Failure rate**: Merkezi metrik panosu yok
- [ ] **Average retry count**: Log/telemetri yok
- [x] **User satisfaction tracking**: `POST /api/ai/feedback` (basit)

---

## 5. Nice-to-Have / Gelecek Özellikler

### 5.1 Conversation Memory

- [x] **Session-based conversation**: `useConversation` (localStorage) + `conversation_id` isteğe bağlı
- [x] **Context carry-over**: `prior_turns` ile önceki soru/JSON modele gidiyor
- [ ] **Implicit filter persistence**: “Geçen ay” filtresini otomatik taşıyan ayrı bir state makinesi yok

### 5.2 Advanced SQL Features

- [ ] **Subquery support**: LogicalQuery’de iç içe alt sorgu modeli yok
- [x] **Window functions**: `SelectTypeWindow` + derleyici
- [x] **CTE support**: `LogicalQuery.CTEs` + derleyici
- [x] **HAVING clause**: Derleyici + validator
- [ ] **CASE/WHEN in select**: Henüz yok

### 5.3 Multi-Model Orchestration

- [ ] **Model routing**: Soru zorluğuna göre model seçimi yok
- [ ] **Model fallback**: Birincil başarısızsa otomatik yedek modele geçiş yok
- [x] **Streaming responses**: Golden eval için `GET /api/ai/eval/run/stream` (SSE metin); ana `/ai/query` hâlâ tek JSON cevap
- [ ] **Cost tracking**: Token maliyet takibi yok

### 5.4 Data Visualization Hints

- [ ] **Chart type suggestion**
- [ ] **Auto-pivot**
- [ ] **Anomaly detection**

### 5.5 RAG Integration

- [ ] **Documentation RAG**
- [ ] **Column description enrichment**: `describe` pipeline’ı var; otomatik zenginleştirme + AI query ile tam entegrasyon sınırlı
- [ ] **Schema change detection**: AI context’i için özel tetikleyici yok

---

## 6. Code-Level Sorunlar

### 6.1 Table Router

- [ ] **Hardcoded domain logic**: AdventureWorks’e özel heuristikler hâlâ kodda
- [ ] **Turkish token synonyms hardcoded**: `tokenSynonyms` vb. kod içi
- [ ] **Score calculation magic numbers**: Ağırlıklar sabit
- [x] **Missing unit tests for edge cases**: `table_router_test.go` genişledi; çok büyük şema/Türkçe köşe vakaları için hâlâ açık iş

### 6.2 AI Client

- [x] **Single provider lock-in**: `Provider` arayüzü + OpenAI + Anthropic
- [x] **No retry on API failure**: 429 / 502–504 ve ağ zaman aşımı için 4 denemeye kadar exponential backoff (`client.go`, `anthropic.go`)
- [ ] **No token counting**: Yaklaşık token log’u yok

### 6.3 Confidence Scoring

- [x] **Hardcoded 0.8**: Kaldırıldı; `computeConfidence` ve self-consistency oranı
- [x] **TableRouter confidence**: `minRouteConfidence` ile alt eşik; embedding hibrit skor

### 6.4 Schema Awareness

- [ ] **No schema prefix in joins**: Çok şemalı join sınırlamaları devam ediyor
- [ ] **No cross-schema query support**: LogicalQuery’de açık şema alanı yok

---

## 7. Öncelik Sırası (Önerilen Implementation Order)

### P0 — Hemen Yapılmalı

1. [x] Retry/re-prompt loop (validation başarısız → LLM'e geri gönder)
2. [x] Dynamic confidence scoring (hardcoded 0.8 kaldır)
3. [x] Disambiguation endpoint (clarification question üretme)
4. [x] Dry-run / EXPLAIN validation

### P1 — Kısa Vadede

5. [x] Few-shot example library (dinamik)
6. [x] Sample data in prompt (mevcut describe endpoint'ini kullan)
7. [x] Golden evaluation dataset
8. [x] Multi-provider support (adapter pattern)

### P2 — Orta Vadede

9. [x] Self-consistency / multi-candidate
10. [x] Vector embedding table retrieval
11. [x] Conversation memory / multi-turn
12. [x] Advanced SQL features (HAVING, window functions; CTE deferred to P3)

### P3 — Uzun Vadede

13. [ ] LLM-as-a-judge evaluation
14. [ ] Model routing (basit → ucuz, karmaşık → güçlü)
15. [ ] RAG integration for business glossary
16. [x] Streaming responses *(golden eval SSE; ana sorgu akışı hâlâ JSON)*

---

## 8. Referans Projeler ve Kaynaklar

| Proje | Yaklaşım | Biqly'ye Uygulanabilir Kısmı |
|---|---|---|
| **Vanna AI** | RAG + few-shot from query history | Query history'den dinamik few-shot |
| **Chat2DB** | Multi-model, dialect-aware prompting | Multi-provider adapter |
| **Google Cloud Text-to-SQL** | Self-consistency, disambiguation, dry-run | Retry loop, clarification, EXPLAIN |
| **AWS Text2SQL** | RAG from data catalog, vector embeddings | Vector tablo seçimi |
| **BIRD-bench** | Evaluation benchmark | Internal golden dataset |

### Kaynak Bağlantılar

- [Google Cloud: Techniques for improving text-to-SQL](https://cloud.google.com/blog/products/databases/techniques-for-improving-text-to-sql)
- [AWS: Best practices for Text2SQL and Generative AI](https://aws.amazon.com/blogs/machine-learning/generating-value-from-enterprise-data-best-practices-for-text2sql-and-generative-ai/)
- [Google Cloud: Architectural Patterns for Text-to-SQL](https://medium.com/google-cloud/architectural-patterns-for-text-to-sql-leveraging-llms-for-enhanced-bigquery-interactions-59756a749e15)
- [Vanna AI](https://github.com/vanna-ai/vanna)
- [Chat2DB](https://github.com/chat2db/Chat2DB)
