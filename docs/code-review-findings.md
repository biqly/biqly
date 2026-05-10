# Kod İnceleme Sonuçları — Düzeltme Listesi

> 4 paralel agent tarafından backend ve frontend kodu incelendi. Aşağıda öncelik sırasına göre tüm bulgular.
> Tüm checkbox'lar boş — düzelttikçe işaretleriz.

---

## P0 — CRITICAL (Güvenlik / Crash / Veri Kaybı)

### Backend

- [ ] **SQL Injection — `WindowSpec.Frame`** (`compiler.go:505`): `Frame` string raw SQL'e inject ediliyor, hiçbir validasyon yok. Kötü niyetli `Frame: "ROWS BETWEEN 1 PRECEDING AND 1 PRECEDING) OR 1=1 --"` SQL injection'a yol açar. Allowlist regex ile sadece geçerli frame keyword'leri kabul et.
- [ ] **SQL Injection — NTILE bucket** (`compiler.go:461-465`): `WindowSpec.Expression` user-supplied olup NTILE içine raw inject ediliyor. Sadece pozitif integer kabul etmeli.
- [ ] **SQL Injection — `pqStringArray`** (`ai_examples.go:238-251`): User-supplied tag'ler PostgreSQL array literal'e escape edilmeden yazılıyor. `lib/pq.Array()` kullan veya `"` → `\\"` escape yap.
- [ ] **Eval endpoint'leri açık** (`router.go:91-92`): `/api/ai/eval/run` ve `/api/ai/eval/run/stream` kimlik doğrulaması olmadan LLM çağrısı yapıyor. Denial-of-wallet riski. Admin route veya API-key gating ekle.
- [ ] **DSN plaintext kaydediliyor** (`datasources.go:60`): `DSNEncrypted` field'ı raw DSN'i kaydediyor. `TODO: encrypt this` yorumu var ama henüz yapılmamış.
- [ ] **`CompileWithPermissions` panic riski** (`compiler.go:176-185`): Tüm `RowFilter.Field`'lar unknown ise `filterParts` boş kalır ve `filterParts[0]` index-out-of-range panic verir.
- [ ] **`CompileWithPermissions` yanlış WHERE clause** (`compiler.go:185`): No-existing-WHERE branch'i `IS NOT NULL` hardcoded ediyor, actual filter value'leri ignore ediyor.
- [ ] **SQL syntax hatası — `usage_date` undefined** (`ai_examples.go:198`): `SELECT DATE(created_at)` alias vermeden `ORDER BY usage_date` kullanılıyor. PostgreSQL her zaman 500 döner. `AS usage_date` ekle.
- [ ] **`DayUsage.Date` type mismatch** (`ai_examples.go:180`): `DATE(created_at)` pgx'ten `time.Time` gelir ama field `string`. Scan error verir.

### Frontend

- [ ] **Stale `question` state** (`AIQuery.tsx:387-390`): `setQuestion(q)` sonrası `requestBody()` eski `question` değerini okur (React batching). Fix: `requestBody(q)` ile parametre geç.
- [ ] **Feedback API her zaman 400 verir** (`AIQuery.tsx:414-434`): `datasource_id` gönderilmiyor ama backend zorunlu. `datasource_id: datasourceId` ekle.
- [ ] **İlk mesaj conversation'a kayboluyor** (`useConversation.ts:57-59`): `createConversation()` çağrılıp fonksiyondan dönülüyor, mesaj yeni conversation'a eklenmiyor. Fix: create sonrası yeni ID ile mesajı ekle.
- [ ] **Concurrent `useApi` loading state çakışması** (`useApi.ts:66-85`): Tek `loading` state'i paylaşıldığı için paralel çağrılar birbirini eziyor. Per-request loading tracking veya ayrı `useApi` instance'ları kullan.
- [ ] **Candidate seçimi no-op** (`AIQuery.tsx:580`): `onUse={() => {}}` — candidate comparison UI hiçbir şey yapmıyor. Fix: seçilen candidate ile re-execute.
- [ ] **Few-shot edit endpoint yok** (`FewShotExamples.tsx:70`): POST `/api/ai/examples/${editId}` çağrılıyor ama backend'de PUT/PATCH yok. Edit her zaman başarısız.
- [ ] **Few-shot create yanlış response shape** (`FewShotExamples.tsx:82-83`): Backend `{ id: "uuid" }` döner, frontend tüm `FewShotExample` objesi bekler. Table'da boş satırlar görünür.

---

## P1 — HIGH (Doğruluk / Performans / Kullanılabilirlik)

### Backend

- [ ] **`eval.go:filterKeys` value exclude ediyor** (`eval.go:87-93`): Filtre değerini karşılaştırmaya dahil etmiyor — yanlış filtre değerleri eşleşiyor. Fix: `field + "|" + operator + "|" + fmt.Sprint(value)`.
- [ ] **`eval.go:selectKeys` alias exclude ediyor** (`eval.go:68-73`): Farklı alias'lar eşleşiyor.
- [ ] **`openai-compatible` boş BaseURL → malformed URL** (`provider.go:26` + `client.go:29`): Default BaseURL sadece `"openai"` provider için set ediliyor. `"openai-compatible"` provider için boş kalır.
- [ ] **Generation error retry yok** (`service.go:151-153`): API error'lar retry loop'a girmiyor, sadece parse/validation failure retry ediliyor.
- [ ] **Embedding API retry yok** (`embedder.go`): Tek bir 429/502 tüm embedding job'ı öldürür.
- [ ] **SQL Server `LimitOffset` offset-only case geçersiz T-SQL** (`sqlserver.go:38-52`): `OFFSET N ROWS` olmadan `FETCH NEXT` gerekiyor. Dead code da var.
- [ ] **MySQL `DateTrunc` `part` parametresini ignore ediyor** (`mysql.go:50-53`): Her zaman full datetime format döner, truncation yapılmaz.
- [ ] **ClickHouse `DateTrunc` "day" part crash** (`clickhouse.go:50-54`): `toStartOfDay()` fonksiyonu ClickHouse'ta yok. `toDate()` olmalı.
- [ ] **Streaming eval client disconnect ignore** (`ai_eval.go:189-205`): Client disconnect olduğunda LLM çağrıları devam eder. `ctx.Done()` check ekle.
- [ ] **Default encryption key reject edilmiyor** (`config.go:106`): `"change-this-to-a-secure-32-byte-key!!"` hala kabul ediliyor. Startup'ta reject et.
- [ ] **Request body size limit yok**: Tüm handler'larda `MaxBytesReader` yok. OOM riski.
- [ ] **`NewService` provider error yutuyor** (`service.go:28`): Unknown provider silent fallback yapıyor, config typo maskelenir.
- [ ] **`DeleteExample` 404 kontrolü yok** (`ai_examples.go:118-131`): Olmayan ID silindiğinde 200 döner. `RowsAffected()` check ekle.

### Frontend

- [ ] **EventSource streaming loading state set edilmiyor** (`useStreamingApi.ts:101-135`): `setLoading(true)` ve `setData(null)` hiç çağrılmıyor.
- [ ] **EventSource error sessiz düşüyor** (`useStreamingApi.ts:125-130`): Accumulated boşsa hiçbir UI feedback yok.
- [ ] **Eval streaming POST on GET endpoint** (`Evaluation.tsx:219`): `streaming.start(url, {})` body gönderince POST yapılır ama backend GET bekler → 405. Fix: `undefined` geç.
- [ ] **Race condition `addMessage`'da** (`useConversation.ts:55-74`): Hızlı arka arkaya mesajlarda stale closure sorunu. Functional state update kullan: `setConversations(prev => ...)`.
- [ ] **Conversation sidebar sadece active conversation gösteriyor** (`AIQuery.tsx:463-478`): Diğer conversation'lar listelenmiyor, geçiş yapılamıyor.
- [ ] **Type mismatch: `TableRoutingCandidate.selected`**: Backend'de `selected` field yok, checkmark render olmaz.
- [ ] **Type mismatch: `confidence_breakdown`**: Backend'de bu field yok, breakdown UI dead code.

---

## P2 — MEDIUM (Tutarlılık / UX)

### Backend

- [ ] **Cross-schema join desteği yok** (`compiler.go:620-621`): Tüm join'ler `BaseSchema` ile prefix'leniyor. `Join` struct'ına `Schema` field ekle.
- [ ] **CTE tanımlı ama compile edilmiyor** (`logical.go:19-30`): `CTEs` field JSON'da parse ediliyor ama `Compile()` tamamen ignore ediyor.
- [ ] **LAG/LEAD window function desteği yok**: `WindowSpec` yeterli parametre içermez.
- [ ] **Multi-candidate sequential** (`service.go:246-266`): 5 candidate = 5x latency. `errgroup` ile paralelize et.
- [ ] **Confidence denominator yanlış** (`service.go:287`): `n` (requested) yerine `successCount` kullanılmalı.
- [ ] **`resolveFilterLHS` complex metric expression bozar** (`compiler.go:662-664`): `QuoteIdent("COALESCE(col1, 0)")` yanlış quoting yapar.
- [ ] **`WHERE` detection fragile** (`compiler.go:163`): String literal içindeki `WHERE` false positive verir.
- [ ] **CORS all origins + credentials** (`router.go:26-33`): Production'da bilinen origin'lere restrict et.
- [ ] **Semantic CRUD input validation yok**: `name`, `base_table`, `column_ref` gibi zorunlu alanlar kontrol edilmiyor.
- [ ] **Query history pagination yok** (`query.go:192-199`): Tüm history döner, büyüdükçe yavaşlar.
- [ ] **`DeleteDatasource` 404 yerine 500** (`datasources.go:113-118`): Not-found case ayrıştır.
- [ ] **`sample.go` empty table name passes validation** (`sample.go:30`): Boş tablo adı geçersiz SQL üretir.
- [ ] **`sample.go` empty cols → `SELECT FROM ...`** (`sample.go:36`): Geçersiz SQL.
- [ ] **`embed_metadata.go` ListColumns called before check** (`embed_metadata.go:48-49`): Boş tablo listesi kontrolünden önce tüm column'lar fetch edilir.
- [ ] **System prompt duplicated** (`client.go:80` vs `anthropic.go:18`): Bir değişirse diğeri gecikir. Shared constant yap.
- [ ] **HTTP timeout hardcoded 60s** (`client.go:35`, `anthropic.go:39`): Config'e taşı.
- [ ] **`exampleIDs` parametre ignore** (`ai.go:475-484`): Frontend specific ID gönderir ama backend hiç kullanmaz.

### Frontend

- [ ] **`chartData` her render'da recomputed** (`AIQuery.tsx:441-445`): `useMemo` kullan.
- [ ] **`SampleDataModal` error'da loading kalmıyor** (`AIQuery.tsx:231`): `.catch()` handler yok.
- [ ] **Eval streaming output raw text** (`Evaluation.tsx:249-253`): JSON parse edip structured render yapılmalı.
- [ ] **Eval streaming `evalData` populate edilmiyor** (`Evaluation.tsx:217-220`): KPI kartları ve chart hiç dolmaz.
- [ ] **`extractUUIDFromPath` chi kullanmıyor** (`ai_examples.go:290-297`): `chi.URLParam(r, "id")` kullanılmalı.
- [ ] **Dashboard hardcoded sample data** (`Dashboard.tsx:8-151`): API entegrasyonu yok, kozmetik.
- [ ] **React error boundary yok**: Component crash → beyaz ekran.
- [ ] **`Typing effect` codepoint/char mismatch** (`useStreamingApi.ts:69`): Emoji supplementary plane karakterlerde bozuk render.
- [ ] **`fallbackPost` 50ms setTimeout race** (`useStreamingApi.ts:239-244`): Typing effect ile yarışır.

---

## P3 — LOW (Kod Kalitesi)

- [ ] **Schema.go dead response fields**: `Candidates`, `RetryCount`, `ValidationResult`, `ModelUsed`, `TokenUsage`, `CostUSD`, `LatencyMs`, `VisualizationHint` hiç populate edilmiyor.
- [ ] **PostgresDialect compile-time interface check yok**: Diğer dialect'lerde var.
- [ ] **ClickHouse lowercase function names**: `count()` vs `COUNT()` tutarsız.
- [ ] **SQL Server `ILike` collation-dependent**: Case-sensitive collation ile çalışmaz.
- [ ] **`HAVING OpBetween` value validation yok** (`validator.go:64-78`): 2-element array kontrolü compile'a kalıyor.
- [ ] **`OrderBy` field validation yok** (`validator.go:120-128`): Unknown field'ler sessizce geçer.
- [ ] **Full LLM prompt/response DB'de saklanıyor** (`history.go:116-125`): Potentially sensitive, DB bloat.
- [ ] **Multiple `eslint-disable-line` suppressions** (`AIQuery.tsx`): `get` dependency ekle, suppressions kaldır.
- [ ] **`useEffect` unstable nested dependency** (`AIQuery.tsx:447`): `visualization_hint` değişmeden tekrar çalışabilir.

---

## İstatistik

| Severity | Backend | Frontend | Toplam |
|----------|---------|----------|--------|
| **P0 Critical** | 9 | 7 | **16** |
| **P1 High** | 13 | 7 | **20** |
| **P2 Medium** | 16 | 9 | **25** |
| **P3 Low** | 8 | 2 | **10** |
| **Toplam** | **46** | **25** | **71** |

## Önerilen Sıralama

1. P0 Security (SQL injection'lar + eval auth + DSN encryption)
2. P0 Crashes (panic guard, SQL syntax error, type mismatch)
3. P0 Frontend data bugs (stale question, lost messages, no-op handlers)
4. P1 Eval correctness (filter/select key comparison)
5. P1 Provider bugs (BaseURL, retry)
6. P1 Dialect bugs (MySQL DateTrunc, SQL Server LimitOffset, ClickHouse)
7. P2+ geri kalanı sırasıyla
