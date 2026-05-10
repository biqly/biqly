# Text-to-SQL Audit: Mevcut Durum, Eksikler ve İyileştirme Planı

> Bu doküman, Biqly projesinin text-to-SQL yaklaşımını endüstri standartlarına (Google Cloud, AWS, Vanna AI, Chat2DB, BIRD-bench) göre denetler. Her madde bir checkbox ile işaretlenir — yapıldıkça check ederiz.

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

Mevcut durum: AI'ya tek seferde soru gönderilir, yanıt alınır. Belirsiz sorularda `empty select` uyarısı verilir ama kullanıcıya **clarification sorusu sorulmaz**.

- [ ] **LLM-driven disambiguation**: İlk LLM çağrısında soru belirsizse, ikinci bir LLM çağrısı ile "ne demek istediniz?" sorusu üret
- [ ] **Multi-turn conversation context**: Kullanıcının önceki sorularını context'e ekle
- [ ] **Clarification response type**: API'de `needs_clarification: true` ve `clarification_question: "..."` alanları tanımla
- [ ] **Follow-up soru Parsing**: Kullanıcı "hayır, revenue olsun" dediğinde önceki context ile birleştir

> **Referans**: Google Cloud — "Disambiguation using LLMs: getting the system to respond with a clarifying question when faced with a question that is not clear enough"

### 2.2 Self-Consistency / Multi-Candidate Generation

Mevcut durum: Tek LLM çağrısı, tek sonuç. Confidence score sabit `0.8` (hardcoded).

- [ ] **Multi-candidate generation**: Aynı soru için 2-3 farklı LLM çağrısı yapıp en tutarlı sonucu seç
- [ ] **Voting mechanism**: Birden fazla aday arasından en çok tekrar eden LogicalQuery'yi seç
- [ ] **Dynamic confidence scoring**: Hardcoded `0.8` yerine, candidate'ler arası tutarlılık + semantic alignment skorla hesapla
- [ ] **Temperature tuning**: İlk çağrı `temperature: 0.1`, ikinci çağrı `temperature: 0.3` gibi farklı parametrelerle dene

> **Referans**: Google Cloud — "Self-consistency: generate multiple queries for the same user question, potentially using different prompting techniques or model variants, and pick the best one"

### 2.3 Validation & Re-prompting

Mevcut durum: Validation başarısız olursa `warnings` ile döner ama **LLM'e geri gönderilip düzeltme yapılmaz**.

- [ ] **Retry loop**: Validation başarısız olursa, hata mesajını LLM'e geri gönder ve düzeltilmiş JSON iste
- [ ] **Max retry limit**: En fazla 2-3 retry denemesi
- [ ] **Dry-run validation**: Compile edilen SQL'i `EXPLAIN` ile çalıştırıp syntax kontrolü yap
- [ ] **Error feedback to LLM**: "Bu dimension yok, mevcut dimensionlar şunlar: ..." şeklinde LLM'e geri bildirim

> **Referans**: Google Cloud — "Validation and reprompting: pass back the mistake to the model for a second pass. When provided an example of a mistake, models can typically address what they got wrong."

### 2.4 Context Building & Table Retrieval

Mevcut `TableRouter` keyword-matching tabanlı. Bu iyi bir başlangıç ama büyük şemalarda yetersiz kalır.

- [ ] **Vector embedding tablo seçimi**: Tablo/column açıklamalarını embed edip semantic similarity ile tablo seç
- [ ] **Column-level retrieval**: Tablo seçildikten sonra, sadece ilgili column'ları getir (şu an tüm columnlar gidiyor)
- [ ] **Schema partitioning**: Büyük datasource'larda schema bazlı filtreleme
- [ ] **Sample data in prompt**: Tablodan 2-3 örnek satır prompt'a ekle (mevcut `describe` endpoint'i var ama AI query'de kullanılmıyor)
- [ ] **Query history as context**: Kullanıcının geçmiş başarılı sorgularını few-shot example olarak prompt'a ekle

> **Referans**: AWS — "Vector embeddings created from a central data catalog can be supplied to an LLM to generate relevant and precise SQL responses"

---

## 3. Prompt Engineering İyileştirmeleri

### 3.1 Few-Shot Examples

Mevcut prompt'ta tek bir örnek var (`orders per customer name`). Bu yeterli değil.

- [ ] **Dynamic few-shot selection**: Kullanıcının sorusuna en benzer 2-3 geçmiş sorguyu prompt'a ekle
- [ ] **Curated example library**: En sık kullanılan sorgu tipleri için elle seçilmiş örnekler
- [ ] **Dialect-specific examples**: Her dialect için farklı SQL syntax örnekleri
- [ ] **Failure examples**: "Bunu yapma" örnekleri — hallucination'ı azaltır

### 3.2 Prompt Structure

- [ ] **Chain-of-thought prompting**: "Önce hangi tablolar gerekli düşün, sonra hangi kolonlar, sonra filtreler..." adımları ekle
- [ ] **Structured output instruction**: JSON Schema'yı prompt'ta ver (zaten var ama daha vurgulu olabilir)
- [ ] **Business glossary section**: Sektör-specific terimlerin açıklamalarını prompt'a ekle
- [ ] **Date/time relative reference handling**: "Geçen ay", "bu çeyrek" gibi ifadeler için daha güçlü talimat

### 3.3 Prompt Size Management

- [ ] **Progressive context loading**: İlk çağrıda minimal context, retry'da daha fazla
- [ ] **Token counting**: Prompt token sayısını logla ve optimize et
- [ ] **Context window adaptation**: Model'e göre prompt boyutunu ayarla

---

## 4. Evaluation & Testing

### 4.1 Evaluation Framework

Mevcut: Testler var ama text-to-SQL kalite ölçümü yok.

- [ ] **Golden dataset**: 50-100 doğal dil sorusu + beklenen LogicalQuery çifti
- [ ] **Execution accuracy test**: Üretilen SQL'in doğru sonuç döndüğünü doğrula
- [ ] **LLM-as-a-judge**: Başka bir LLM ile üretilen sorgunun kalitesini değerlendir
- [ ] **Benchmark suite**: BIRD-bench benzeri internal benchmark oluştur
- [ ] **Regression testing**: Her prompt değişikliğinde tüm test setini çalıştır

### 4.2 Metrics

- [ ] **Exact match accuracy**: Üretilen LogicalQuery'nin beklenenle birebir eşleşme oranı
- [ ] **Execution accuracy**: SQL'in aynı sonuç kümesini döndürme oranı
- [ ] **Failure rate**: Validation/retry sonrası hâlâ başarısız olan sorguların oranı
- [ ] **Average retry count**: Başarılı bir sorgu için ortalama retry sayısı
- [ ] **User satisfaction tracking**: Kullanıcının sonucu kabul/red oranı

---

## 5. Nice-to-Have / Gelecek Özellikler

### 5.1 Conversation Memory

- [ ] **Session-based conversation**: Kullanıcının oturum bazlı soru geçmişi
- [ ] **Context carry-over**: "Bunu product bazında kır" → önceki sorguyu product dimension'ı ile tekrar çalıştır
- [ ] **Implicit filter persistence**: "Geçen ayın verisi" → sonraki sorularda da aynı filtre uygula

### 5.2 Advanced SQL Features

- [ ] **Subquery support**: LogicalQuery'de iç içe sorgu desteği
- [ ] **Window functions**: ROW_NUMBER, RANK, LAG/LEAD desteği
- [ ] **CTE support**: WITH clause desteği (okunabilirlik ve karmaşık sorgular için)
- [ ] **HAVING clause**: Aggregation sonrası filtreleme
- [ ] **CASE/WHEN in select**: Koşullu kolon üretimi

### 5.3 Multi-Model Orchestration

- [ ] **Model routing**: Basit sorular → hızlı/ucuz model, karmaşık sorular → güçlü model
- [ ] **Model fallback**: Ana model başarısız olursa yedek modele geç
- [ ] **Streaming responses**: LLM yanıtı streaming olarak client'a ilet
- [ ] **Cost tracking**: Token bazlı maliyet takibi

### 5.4 Data Visualization Hints

- [ ] **Chart type suggestion**: Sonuçlara göre "bu veri bar chart için uygun" önerisi
- [ ] **Auto-pivot**: Pivot tablo önerisi
- [ ] **Anomaly detection**: Sonuçlarda anormallik tespiti ve vurgulama

### 5.5 RAG Integration

- [ ] **Documentation RAG**: Şirket içi veri sözlüğü / wiki'yi RAG ile prompt'a ekle
- [ ] **Column description enrichment**: AI ile otomatik column açıklaması üretimi (mevcut `describe` endpoint'ini AI query pipeline'ına entegre et)
- [ ] **Schema change detection**: Metadata sync'te değişen kolonları tespit et ve AI context'ini güncelle

---

## 6. Code-Level Sorunlar

### 6.1 Table Router

- [ ] **Hardcoded domain logic**: `isCategoryOrProductQuestion`, `isRevenueLikeQuestion` gibi fonksiyonlar AdventureWorks'e özel — bunları generic yap veya configurable yap
- [ ] **Turkish token synonyms hardcoded**: `tokenSynonyms` map'i code'da gömülü — bunu config veya DB'ye taşı
- [ ] **Score calculation magic numbers**: `score += 14`, `score += 8` gibi hardcoded ağırlıklar — bunları tune edilebilir yap
- [ ] **Missing unit tests for edge cases**: Türkçe sorular, çok tablolu şemalar, boş metadata

### 6.2 AI Client

- [ ] **Single provider lock-in**: Sadece OpenAI-compatible API desteği — Anthropic, Google, local modeller için adapter pattern gerek
- [ ] **No retry on API failure**: Network timeout veya rate limit durumunda retry mekanizması yok
- [ ] **No token counting**: Prompt/yanıt token sayısı loglanmıyor

### 6.3 Confidence Scoring

- [ ] **Hardcoded 0.8**: `service.go:62` — `Confidence: 0.8` sabit değer, dynamic scoring gerek
- [ ] **TableRouter confidence**: `routeConfidence` fonksiyonu var ama sadece relative scoring yapıyor, absolute threshold yok

### 6.4 Schema Awareness

- [ ] **No schema prefix in joins**: `buildJoins` her zaman `model.BaseSchema` kullanıyor → farklı schema'lardaki tabloları join edemez
- [ ] **No cross-schema query support**: LogicalQuery'de schema bilgisi yok

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
16. [ ] Streaming responses

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
