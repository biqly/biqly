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
- [x] **Column-level retrieval**: Geniş tablolarda soru anahtar kelimeleri + kolon embedding benzerliği birleşik skorla kolon alt kümesi seçilir (`column_retrieval.go`); tam embedding kapsamı şartı kaldırıldı
- [x] **Schema partitioning**: Çok şemalı datasource’larda otomatik şema kümeleme — soru skoruna göre en ilgili 1–2 şema (`schema_partition.go`, `TableRoutingDebug.schema_partitions`)
- [x] **Sample data in prompt**: `POST /api/ai/query/run` yolunda `WithSampleData` (`/ai/query` ve `/ai/query/preview` örnek satır içermez)
- [x] **Query history as context**: Few-shot + `include_past_queries` / `example_ids`

> **Referans**: AWS — "Vector embeddings created from a central data catalog can be supplied to an LLM to generate relevant and precise SQL responses"

---

## 3. Prompt Engineering İyileştirmeleri

### 3.1 Few-Shot Examples

- [x] **Dynamic few-shot selection**: Geçmiş / kayıtlı örnekler `loadFewShotExamples*` ile prompt’a
- [x] **Curated example library**: `GET/POST/DELETE /api/ai/examples` + UI `FewShotExamples`
- [x] **Dialect-specific examples**: `PromptBuilder` dialect rehberi — aynı LogicalQuery’nin postgres/mysql/sqlserver/clickhouse derlemesi; `WithTargetDialect` handler’lardan
- [x] **Failure examples**: `## Examples — Common Mistakes` — SQL üretme, uydurma alan, having/filters, grain filtreleri vb.

### 3.2 Prompt Structure

- [x] **Chain-of-thought prompting**: `## Planning Steps` (8 adım: niyet → tablo/join → metric → dimension → filter/group_by → having/window → order/limit → JSON); isteğe bağlı `## Reasoning` öneki + `stripReasoningPreamble` ile parse
- [x] **Structured output instruction**: JSON-only ve şema kuralları prompt’ta
- [x] **Business glossary section**: `## Business Glossary` prompt bölümü; katalog eşanlamlıları + `business_glossary_terms` (CRUD `/api/ai/glossary`); soru token eşleşmesi ile seçim — tam vektör RAG henüz yok (P3)
- [x] **Date/time relative reference handling**: Tarih grain, ISO filtre, `having`/window talimatları prompt’ta

### 3.3 Prompt Size Management

- [x] **Progressive context loading**: Retry’da context tier yükselir (compact → standard → expanded): prompt rune bütçesi ×1.35/×1.75, daha fazla few-shot / prior turn / glossary; tam prompt `BuildRetry` ile yeniden kurulur
- [x] **Token counting**: `EstimateTokens` (~4 char/token); `slog` + API `prompt_stats` / `token_usage` (tahmini prompt+completion)
- [x] **Context window adaptation**: Model registry + `BI_AI_NUM_CTX` + `EffectiveMaxPromptRunes`; `/api/ai/settings` exposes `context_window_tokens`, `effective_max_prompt_runes`

---

## 4. Evaluation & Testing

### 4.1 Evaluation Framework

- [x] **Golden dataset**: `DefaultGoldenCases()` + `LogicalQueryEqual` + testler; büyük (50–100) set henüz yok
- [x] **Execution accuracy test**: `ResultSetEqual` + `MemoryResultExecutor`; eval runner `EvalModeExecution`; HTTP eval varsayılan logical+execution
- [x] **LLM-as-a-judge**: `JudgeLogicalQuery` + `EvalModeJudge`; `POST /api/ai/eval/run?judge=1`
- [x] **Benchmark suite**: `BenchmarkCases()` (`biqly-benchmark-v1`); `?suite=benchmark`; stub regression testi
- [x] **Regression testing**: CI + `make eval-regression` stub gate; canlı LLM için `BI_AI_GOLDEN_EVAL=1` ayrı

### 4.2 Metrics

- [x] **Exact match accuracy**: Eval UI / `POST /api/ai/eval/run` pass_rate
- [x] **Execution accuracy**: Eval runner execution pass rate; in-memory golden executor
- [x] **Failure rate**: `outcome_status` + `GetAIMetricsDashboard`; dashboard KPI; Prometheus `bi_ai_failure_rate`
- [x] **Average retry count**: `retry_count` persist + structured log; dashboard + Prometheus `bi_ai_avg_retry_count`
- [x] **User satisfaction tracking**: `POST /api/ai/feedback` (basit)

---

## 5. Nice-to-Have / Gelecek Özellikler

### 5.1 Conversation Memory

- [x] **Session-based conversation**: `useConversation` (localStorage) + `conversation_id` isteğe bağlı
- [x] **Context carry-over**: `prior_turns` ile önceki soru/JSON modele gidiyor
- [x] **Implicit filter persistence**: `FilterSessionState` + `ClassifyFollowUpIntent` (refine / replace / new); önceki tur `filters`/`having` refine’da programatik merge + prompt’ta “Active Filters” bölümü

### 5.2 Advanced SQL Features

- [x] **Subquery support**: `SubqueryBody`, `Filter.subquery` (IN/NOT IN), `from_subquery` / `from_cte`
- [x] **Window functions**: `SelectTypeWindow` + derleyici
- [x] **CTE support**: `LogicalQuery.CTEs` + derleyici
- [x] **HAVING clause**: Derleyici + validator
- [x] **CASE/WHEN in select**: `SelectTypeCase` + `CaseExpr` / derleyici

### 5.3 Multi-Model Orchestration

- [ ] **Model routing**: Soru zorluğuna göre model seçimi yok
- [ ] **Model fallback**: Birincil başarısızsa otomatik yedek modele geçiş yok
- [x] **Streaming responses**: Golden eval için `GET /api/ai/eval/run/stream` (SSE metin); ana `/ai/query` hâlâ tek JSON cevap
- [ ] **Cost tracking**: Token maliyet takibi yok

### 5.4 Data Visualization Hints

- [x] **Chart type suggestion**: `EnrichResult` → `chart_suggestions`; AI `/ai/query/run` → `visualization_hint`; frontend otomatik chart seçimi
- [x] **Auto-pivot**: `pivot_hint` (2 kategorik boyut + metrik); UI’da pivot önerisi
- [x] **Anomaly detection**: `anomalies[]` (metrik sütunlarda IQR); UI’da aykırı sayacı

### 5.5 RAG Integration

- [ ] **Documentation RAG**
- [ ] **Column description enrichment**: `describe` pipeline’ı var; otomatik zenginleştirme + AI query ile tam entegrasyon sınırlı
- [ ] **Schema change detection**: AI context’i için özel tetikleyici yok

---

## 6. Code-Level Sorunlar

### 6.1 Table Router

- [x] **Hardcoded domain logic**: Tablo boost kuralları `routing_weights_default.json` içinde genel substring kalıpları (`orderdetail`, `category`, `product`); AdventureWorks’e özel tablo adı yok
- [x] **Turkish token synonyms hardcoded**: `routing_lexicon_default.json` + `BI_AI_ROUTING_LEXICON_PATH` ile override
- [x] **Score calculation magic numbers**: `routing_weights_default.json` + `BI_AI_ROUTING_WEIGHTS_PATH` ile override
- [x] **Missing unit tests for edge cases**: `table_router_test.go` genişledi; çok büyük şema/Türkçe köşe vakaları için hâlâ açık iş

### 6.2 AI Client

- [x] **Single provider lock-in**: `Provider` arayüzü + OpenAI + Anthropic
- [x] **No retry on API failure**: 429 / 502–504 ve ağ zaman aşımı için 4 denemeye kadar exponential backoff (`client.go`, `anthropic.go`)
- [x] **Token counting**: OpenAI/Anthropic `usage` parse; `llm completion` slog (`est_prompt_tokens`, `tokens_from_api`, prompt/completion/total); service prefers API usage over estimate

### 6.3 Confidence Scoring

- [x] **Hardcoded 0.8**: Kaldırıldı; `computeConfidence` ve self-consistency oranı
- [x] **TableRouter confidence**: `minRouteConfidence` ile alt eşik; embedding hibrit skor

### 6.4 Schema Awareness

- [x] **No schema prefix in joins**: `semantic.Join` `from_schema`/`to_schema`; compiler `buildJoins` şema bazlı qualify
- [x] **No cross-schema query support**: `default_schema`, `table_schemas`, üç parçalı `column_ref` (`schema.table.column`)

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
15. [ ] RAG integration for business glossary *(v1: keyword retrieval + harici tablo tamam; embedding RAG açık)*
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
