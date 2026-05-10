# Text-to-SQL Referans Mimariler

> Endüstride başarılı text-to-SQL projelerinin mimari örüntüleri. Biqly'nin mevcut ve gelecek mimarisini bu referanslara göre konumlandırıyoruz.

## 1. Google Cloud Text-to-SQL Architecture

```text
User Question
     │
     ▼
┌──────────────────┐
│  Intent Analysis  │  ← Disambiguation (clarification question)
│  & Entity         │  ← Entity resolution
│  Resolution       │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Context Builder  │  ← Intelligent retrieval (vector search)
│                   │  ← Schema annotations
│                   │  ← Business rules
│                   │  ← Query history (few-shot)
│                   │  ← Data samples
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  SQL Generation   │  ← SQL-aware foundation model
│  (multi-candidate)│  ← Multiple prompt strategies
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Validation       │  ← Syntax check (dry run)
│  & Re-prompting   │  ← Self-consistency voting
│                   │  ← LLM re-prompt on error
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Execution        │  ← Safe execution
│  & Response       │  ← Result formatting
└──────────────────┘
```

**Biqly'deki karşılığı:**

- Context Builder: `PromptBuilder` + `TableRouter` ✓
- SQL Generation: LLM → LogicalQuery ✓
- Validation: `Validator` ✓
- Re-prompting: **EKSİK** ✗
- Disambiguation: **EKSİK** ✗
- Self-consistency: **EKSİK** ✗

---

## 2. AWS Text2SQL Pattern: RAG-enhanced

```text
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  Data Catalog │────▶│  Vector Store    │────▶│  LLM Prompt  │
│  (Glue/Custom)│    │  (Embeddings)    │     │  + Context   │
└─────────────┘     └──────────────────┘     └──────┬──────┘
                                                       │
                                                       ▼
                                               ┌──────────────┐
                                               │  SQL Output  │
                                               │  + Validation │
                                               └──────────────┘
```

**Anahtar fark:** AWS, tablo/column seçimini vector similarity ile yapıyor. Biqly keyword-matching yapıyor.

**Biqly'ye uygulanabilir:**

- [ ] Metadata'daki tablo/column açıklamalarını embed'le
- [ ] Kullanıcı sorusunu embed'le
- [ ] Cosine similarity ile en uygun tabloları seç
- [ ] Mevcut keyword-matching'i fallback olarak tut

---

## 3. Vanna AI Pattern: RAG + Training from Query History

```text
┌─────────────────┐
│  Training Data   │  ← Successful Q→SQL pairs
│  (DDL + Q&A)     │  ← DDL statements
└────────┬────────┘
         │ Vectorize & Store
         ▼
┌─────────────────┐
│  Vector Store    │  ← ChromaDB / similar
└────────┬────────┘
         │ Similarity Search
         ▼
┌─────────────────┐
│  Prompt Builder  │  ← DDL + similar Q&A pairs
│                  │  ← Question
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  LLM             │  → SQL
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Run & Validate  │  → If error → re-prompt with error
└────────┬────────┘
         │ Success
         ▼
┌─────────────────┐
│  Store Q→SQL     │  ← Auto-train from success
└─────────────────┘
```

**Anahtar insight:** Başarılı sorgular otomatik olarak eğitim verisi olur.

**Biqly'ye uygulanabilir:**

- [ ] `ai_query_history` tablosundaki başarılı sorguları few-shot olarak kullan
- [ ] Başarısız → retry → başarılı döngüsünü kaydet
- [ ] DDL bilgilerini prompt'a ekle (CREATE TABLE statements)

---

## 4. Biqly'nin Hedef Mimari (Önerilen)

```text
User Question + (optional) conversation context
         │
         ▼
┌───────────────────────────┐
│  1. Table/Context Router  │
│     ├─ Vector search      │  (P2 - embedding based)
│     ├─ Keyword matching   │  (mevcut)
│     ├─ FK graph expansion │  (mevcut)
│     └─ Confidence check   │
└────────────┬──────────────┘
             │
             ▼
┌───────────────────────────┐
│  2. Disambiguation        │  (P0 - yeni)
│     ├─ Ambiguity detect   │
│     ├─ Clarification gen  │
│     └─ Multi-turn context │
└────────────┬──────────────┘
             │ Clear intent
             ▼
┌──────────────────────────┐
│  3. Prompt Builder       │
│     ├─ Semantic context  │  (mevcut)
│     ├─ Few-shot examples │  (P1 - query history)
│     ├─ Sample data       │  (P1)
│     └─ DDL statements    │  (P1)
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│  4. Multi-Candidate Gen  │  (P2 - self-consistency)
│     ├─ Candidate 1       │
│     ├─ Candidate 2       │
│     └─ Candidate 3       │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│  5. Validate & Re-prompt │
│     ├─ JSON parse        │  (mevcut)
│     ├─ Semantic validate │  (mevcut)
│     ├─ EXPLAIN check     │  (P0)
│     └─ Retry on failure  │  (P0 - yeni)
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│  6. Compile & Execute    │  (mevcut)
│     ├─ Dialect compile   │
│     ├─ Security check    │
│     └─ Execute + cache   │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│  7. Post-Execution       │
│     ├─ Store in history  │  (mevcut)
│     ├─ Auto-train update │  (P1 - yeni)
│     └─ Confidence score  │  (P0 - dynamic)
└──────────────────────────┘
```

---

## 5. Pattern Karşılaştırma Tablosu

| Pattern | Google Cloud | AWS | Vanna AI | Biqly (mevcut) | Biqly (hedef) |
| --- | --- | --- | --- | --- | --- |
| SQL generation | Raw SQL | Raw SQL | Raw SQL | **LogicalQuery JSON** | **LogicalQuery JSON** |
| Table retrieval | Vector + keyword | Vector (RAG) | Vector (RAG) | Keyword only | Keyword + Vector |
| Disambiguation | LLM-driven | - | - | - | LLM-driven |
| Validation | Dry-run | - | Re-prompt | JSON + Semantic | JSON + Semantic + EXPLAIN + Re-prompt |
| Self-consistency | Multi-candidate | - | - | - | Multi-candidate |
| Few-shot | Curated | - | Auto from history | 1 static example | Dynamic from history |
| Multi-model | Yes | Yes | Yes | OpenAI only | Multi-provider |
| Safety | Post-validation | - | - | **Pre-validation (LogicalQuery)** | **Pre-validation** |

**Sonuç:** Biqly'nin "LogicalQuery-first" yaklaşımı endüstride benzersiz ve en güvenli. Eksikler execution-phase'da değil, **generation-phase kalitesinde** (disambiguation, re-prompting, self-consistency).
