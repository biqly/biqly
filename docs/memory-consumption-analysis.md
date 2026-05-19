# Biqly Backend Memory Consumption Analysis

Go backend'inde gereksiz memory consumption olusturan noktaların detaylı analizi.
Her bulgu severity ve hot-path olup olmadigina gore siniflandirilmistir.

---

## Contents

1. [CRITICAL: Executor Per-Row Triple Allocation](#1-critical-executor-per-row-triple-allocation)
2. [HIGH: Encryption Cipher Objects Recreated Per Call](#2-high-encryption-cipher-objects-recreated-per-call)
3. [HIGH: io.ReadAll on Unbounded LLM Response Bodies](#3-high-ioreadall-on-unbounded-llm-response-bodies)
4. [HIGH: SyncMetadata Missing Context Cancellation](#4-high-syncmetadata-missing-context-cancellation)
5. [HIGH: No sync.Pool for Prompt Builder strings.Builder](#5-high-no-syncpool-for-prompt-builder-stringsbuilder)
6. [HIGH: BuildRetry Duplicates Entire ~80K Prompt](#6-high-buildretry-duplicates-entire-80k-prompt)
7. [HIGH: strings.ToUpper Full SQL Copy in Security Check](#7-high-stringstoupper-full-sql-copy-in-security-check)
8. [HIGH: sb.String() Full Copy for Rune Counting](#8-high-sbstring-full-copy-for-rune-counting)
9. [MEDIUM: fmt.Sprintf vs strconv.Itoa in Hot Paths](#9-medium-fmtsprintf-vs-strconvitoa-in-hot-paths)
10. [MEDIUM: Slice/Map Allocations Without Capacity Hints](#10-medium-slicemap-allocations-without-capacity-hints)
11. [MEDIUM: json.Marshal Double-Allocation Pattern](#11-medium-jsonmarshal-double-allocation-pattern)
12. [MEDIUM: strings.NewReplacer Allocated Per Call](#12-medium-stringsnewreplacer-allocated-per-call)
13. [MEDIUM: tokenSet Allocates Map Hundreds of Times Per Route](#13-medium-tokenset-allocates-map-hundreds-of-times-per-route)
14. [MEDIUM: relationAdjacency Rebuilt 3x Per Route Call](#14-medium-relationadjacency-rebuilt-3x-per-route-call)
15. [MEDIUM: []byte/string Conversions in Validation Path](#15-medium-bytestring-conversions-in-validation-path)
16. [MEDIUM: Cache.Key Uses Reflection-Based Formatting](#16-medium-cachekey-uses-reflection-based-formatting)
17. [MEDIUM: buildAIHistoryEntry Embeds 80K+ Rune Prompt](#17-medium-buildaihistoryentry-embeds-80k-rune-prompt)
18. [MEDIUM: QuerySlice Generic Helper No Capacity Hint](#18-medium-queryslice-generic-helper-no-capacity-hint)
19. [MEDIUM: Duplicate dimMap Construction in CompileWithPermissions](#19-medium-duplicated-dimap-construction-in-compilewithpermissions)
20. [MEDIUM: String Concatenation With += in Loops](#20-medium-string-concatenation-with--in-loops)
21. [MEDIUM: []rune Allocation Before Truncation Check](#21-medium-rune-allocation-before-truncation-check)
22. [LOW: map[string]bool vs map[string]struct{}](#22-low-mapstringbool-vs-mapstringstruct)
23. [LOW: writeJSON Reflection on Every HTTP Response](#23-low-writejson-reflection-on-every-http-response)
24. [LOW: Operator Slices Allocated Per Validate Call](#24-low-operator-slices-allocated-per-validate-call)
25. [LOW: UpsertColumns One-At-A-Time in Loop](#25-low-upsertcolumns-one-at-a-time-in-loop)
26. [LOW: Health Check []byte Allocation](#26-low-health-check-byte-allocation)
27. [Summary Table](#summary-table)

---

## 1. CRITICAL: Executor Per-Row Triple Allocation

**File:** `internal/query/executor.go:66-88`

```go
var resultRows [][]any           // NO capacity hint
for rows.Next() {
    vals := make([]any, len(colTypes))    // allocation #1 per row
    valPtrs := make([]any, len(colTypes))  // allocation #2 per row
    for i := range vals {
        valPtrs[i] = &vals[i]
    }
    rows.Scan(valPtrs...)
    row := make([]any, len(vals))  // allocation #3 per row
    copy(row, vals)
    resultRows = append(resultRows, row)
}
```

**Impact:** 10,000 row x 20 column sonuc icin **30,000 heap allocation** per query.
`vals` ve `valPtrs` sadece scan hedefi olarak kullanilip atiliyor.

**Fix:**

```go
resultRows := make([][]any, 0, e.maxRows)
vals := make([]any, len(colTypes))
valPtrs := make([]any, len(colTypes))
for i := range vals {
    valPtrs[i] = &vals[i]
}
for rows.Next() {
    rows.Scan(valPtrs...)
    row := make([]any, len(vals))
    copy(row, vals)
    resultRows = append(resultRows, row)
}
```

Bu degisiklik 2/3 per-row allocation'i ortadan kaldirir.

---

## 2. HIGH: Encryption Cipher Objects Recreated Per Call

**File:** `internal/security/encryption.go:49-67, 70-97`

```go
func (e *Encryption) Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(e.key)   // ~240 bytes, her cagrida yeni
    gcm, err := cipher.NewGCM(block)     // ~240 bytes, her cagrida yeni
    nonce := make([]byte, gcm.NonceSize()) // her cagrida yeni
    // ...
}
```

**Impact:** Her datasource resolution ve query execution'da ~1 KB short-lived allocation.
AES key degismedigi icin cipher ve GCM immutable.

**Fix:** `block` ve `gcm`'i `NewEncryption`'da bir kez olusturup struct field olarak sakla:

```go
type Encryption struct {
    key  []byte
    block cipher.Block
    aead  cipher.AEAD
}

func NewEncryption(key []byte) (*Encryption, error) {
    block, err := aes.NewCipher(key)
    if err != nil { return nil, err }
    aead, err := cipher.NewGCM(block)
    if err != nil { return nil, err }
    return &Encryption{key: key, block: block, aead: aead}, nil
}
```

---

## 3. HIGH: io.ReadAll on Unbounded LLM Response Bodies

**File:** `internal/ai/http_transport.go:11-13`

```go
func readResponseBody(resp *http.Response) ([]byte, error) {
    body, readErr := io.ReadAll(resp.Body)  // no size limit
    closeErr := resp.Body.Close()
    // ...
}
```

**Impact:** Her LLM API call (OpenAI, Anthropic, embeddings) buradan geciyor.
`io.ReadAll` doubling strategy ile buyuyor: 100 KB response icin 200 KB peak allocation.
`multiCandidateCount > 1` ile N katina cikiyor. Malicious endpoint OOM'a yol acabilir.

**Fix:**

```go
func readResponseBody(resp *http.Response) ([]byte, error) {
    defer resp.Body.Close()
    limited := io.LimitReader(resp.Body, maxResponseSize) // e.g. 10MB
    return io.ReadAll(limited)
}
```

---

## 4. HIGH: SyncMetadata Missing Context Cancellation

**File:** `internal/http/handlers/datasources.go:478-600`

```go
for _, s := range result.Schemas {   // no ctx.Err() check
    schemaID, err := h.deps.MetaRepo.UpsertSchema(ctx, ds.ID, schema)
}
for _, t := range result.Tables {    // no ctx.Err() check
    tableID, err := h.deps.MetaRepo.UpsertTable(ctx, ds.ID, table)
}
for _, c := range result.Columns {   // no ctx.Err() check
    h.deps.MetaRepo.UpsertColumns(ctx, ds.ID, []metadata.Column{col})
}
```

**Impact:** Buyuk veritabani introspeksonunda (500 tablo, 10K kolon) client disconnect
olsa bile goroutine calismaya devam eder. `result` 5-50 MB tutabilir.
Zombie goroutine + memory leak.

**Fix:** Her loop'un basina:

```go
if ctx.Err() != nil {
    return
}
```

---

## 5. HIGH: No sync.Pool for Prompt Builder strings.Builder

**File:** `internal/ai/prompt.go:66-67`

```go
var sb strings.Builder  // 80K+ rune prompt icin ~160 KB peak
```

**Impact:** Prompt 80,000 rune'a kadar buyuyebilir. Builder'in internal byte slice doubling
ile peak ~160 KB. Her AI query en az bir tane, retry'lar ekstra. `ai` paketinde hic
`sync.Pool` yok. Concurrent yuk altinda GC pressure onemli.

**Fix:**

```go
var promptBuilderPool = sync.Pool{
    New: func() any { return new(strings.Builder) },
}

func (b *PromptBuilder) Build(...) string {
    sb := promptBuilderPool.Get().(*strings.Builder)
    sb.Reset()
    defer promptBuilderPool.Put(sb)
    // ...
}
```

---

## 6. HIGH: BuildRetry Duplicates Entire ~80K Prompt

**File:** `internal/ai/prompt.go:387-398`

```go
func (b *PromptBuilder) BuildRetry(originalPrompt, lastResponse, validationError string) string {
    var sb strings.Builder
    sb.WriteString(originalPrompt)  // 80K rune duplicate
    sb.WriteString("\n\n## Previous Attempt (incorrect)\n")
    sb.WriteString(lastResponse)    // another multi-KB
    // ...
}
```

**Impact:** Retry prompt nearly 2x original. `maxRetries=3` icin 4 prompt string ayni anda
memory'de: base + 3 retry, her biri ~80-160 KB = ~640 KB transient data per failed request.

**Fix:** Retry context'i ayri bir section olarak ekleyip original prompt'u refere et
(veya en azindan builder pool kullan).

---

## 7. HIGH: strings.ToUpper Full SQL Copy in Security Check

**File:** `internal/security/readonly.go:38`

```go
trimmed := strings.TrimSpace(strings.ToUpper(sql))  // full copy of entire SQL
```

**File:** `internal/query/compiler.go:67`

```go
upperSQL := strings.ToUpper(cq.SQL)  // full copy again
```

**Impact:** Her query execution'da compiled SQL'in tamami upper-case olarak kopyalaniyor.
2-4 KB SQL icin 2-4 KB gereksiz allocation. Iki farkli yerde ayni pattern.

**Fix:** Case-insensitive regex `(?i)` kullan veya sadece kucuk keyword'leri upper yap:

```go
pattern := regexp.MustCompile(`(?i)\b(DROP|ALTER|TRUNCATE)\b`)
```

---

## 8. HIGH: sb.String() Full Copy for Rune Counting

**File:** `internal/ai/prompt.go:93, 120`

```go
headRunes := utf8.RuneCountInString(sb.String())   // materializes entire builder
metricsBudget := maxPromptRunes - utf8.RuneCountInString(sb.String()) - ...
```

**Impact:** `sb.String()` builder'in tamami yeni bir string olarak kopyalar.
100 dimension'dan sonra ~10 KB allocation sadece rune saymak icin, her prompt build'de 2 kez.

**Fix:**

```go
utf8.RuneCount(sb.Bytes())  // no-copy, reads buffer directly
```

---

## 9. MEDIUM: fmt.Sprintf vs strconv.Itoa in Hot Paths

Her query compilation'da onlarca kez cagrilan yerlerde `fmt.Sprintf` kullanilmis.
`fmt.Sprintf` reflection ve format parsing yapiyor; `strconv.Itoa` direkt donusum.

### 9a. Dialect Placeholder (per bind parameter)

**File:** `internal/dialect/postgres.go:31`, `internal/dialect/sqlserver.go:31`

```go
// Bad
return fmt.Sprintf("$%d", index)
// Good
return "$" + strconv.Itoa(index)
```

**Impact:** 20 parametrelik query'de 20 gereksiz `fmt.Sprintf` call.

### 9b. Dialect LimitOffset

**File:** `internal/dialect/base.go:27,30`, `internal/dialect/sqlserver.go:37,39`

```go
// Bad
parts = append(parts, fmt.Sprintf("LIMIT %d", limit))
// Good
parts = append(parts, "LIMIT "+strconv.Itoa(limit))
```

### 9c. Compiler buildFilterPart (per filter)

**File:** `internal/query/compiler.go:670-710`

```go
// Bad - 10+ branches with fmt.Sprintf for simple concatenation
return fmt.Sprintf("%s = %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
return fmt.Sprintf("%s IS NULL", lhsSQL), nil, nil  // literally just lhsSQL + " IS NULL"

// Good
return lhsSQL + " = " + c.dialect.Placeholder(len(*args)), nil, nil
return lhsSQL + " IS NULL", nil, nil
```

**Impact:** Her filter icin `[]any` allocation + interface boxing + format parsing.

### 9d. Compiler buildSelect (per select item)

**File:** `internal/query/compiler.go:338,350,361,375`

```go
// Bad
parts = append(parts, fmt.Sprintf("%s AS %s", col, c.dialect.QuoteIdent(alias)))
// Good
parts = append(parts, col+" AS "+c.dialect.QuoteIdent(alias))
```

### 9e. LIKE Pattern Construction

**File:** `internal/query/compiler.go:697,700,703`

```go
// Bad
*args = append(*args, fmt.Sprintf("%%%v%%", f.Value))
// Good (if string)
*args = append(*args, "%"+f.Value.(string)+"%")
```

### 9f. Sample Query LIMIT

**File:** `internal/ai/sample.go:74`

```go
// Bad
qb.WriteString(fmt.Sprint(limit))
// Good
qb.WriteString(strconv.Itoa(limit))
```

### 9g. Eval Memory Executor

**File:** `internal/ai/eval_memory.go:64,66,68,105`

```go
// Bad
return fmt.Sprint(val) == fmt.Sprint(f.Value)
// Good - type switch with strconv
```

---

## 10. MEDIUM: Slice/Map Allocations Without Capacity Hints

Boyutu bilinen veya tahmin edilebilen her yerde `var x []T` veya `make(map[K]V)` kullanilmis.
Bu pattern Go'da doubling growth ile multiple reallocation'a yol aciyor.

### 10a. Compiler (per query compilation)

**File:** `internal/query/compiler.go` ve `internal/query/compiler_nested.go`

| Line | Code | Fix |
|------|------|-----|
| 47 | `dimMap := make(map[string]string)` | `make(map[string]string, len(model.Dimensions)+len(model.Metrics))` |
| 325 | `var parts []string` in buildSelect | `make([]string, 0, len(items))` |
| 499 | `var parts []string` in buildHaving | `make([]string, 0, len(filters))` |
| 566 | `var clauses []string` in buildJoins | `make([]string, 0, len(joinNames))` |
| 609 | `var parts []string` in buildWhere | `make([]string, 0, len(filters))` |
| 758 | `var parts []string` in buildGroupBy | `make([]string, 0, len(groupBy))` |
| 774 | `var parts []string` in buildOrderBy | `make([]string, 0, len(orderBy))` |

**File:** `internal/query/compiler_nested.go`

| Line | Code | Fix |
|------|------|-----|
| 86 | `dimMap := make(map[string]semantic.Dimension)` | `make(..., len(model.Dimensions))` |
| 99 | `metricMap := make(map[string]semantic.Metric)` | `make(..., len(model.Metrics))` |
| 103 | `joinMap := make(map[string]semantic.Join)` | `make(..., len(model.Joins))` |

**File:** `internal/query/compiler_case.go`

| Line | Code | Fix |
|------|------|-----|
| 21 | `var parts []string` | `make([]string, 0, len(item.Case.Branches)+3)` |
| 82 | `var parts []string` | `make([]string, 0, len(filters))` |

### 10b. Prompt Builder (per AI query)

**File:** `internal/ai/prompt.go`

| Line | Code | Fix |
|------|------|-----|
| 100 | `var allowedDims []semantic.Dimension` | `make([]semantic.Dimension, 0, len(model.Dimensions))` |
| 108 | `var allowedMetrics []semantic.Metric` | `make([]semantic.Metric, 0, len(model.Metrics))` |
| 133 | `var allowedJoins []semantic.Join` | `make([]semantic.Join, 0, len(model.Joins))` |
| 232 | `var filtered []FewShotExample` | `make([]FewShotExample, 0, len(examples))` |
| 263 | `var displayDims, otherDims []semantic.Dimension` | Pre-estimate from `len(dims)` |

### 10c. Table Router (per routing operation)

**File:** `internal/ai/table_router.go`

| Line | Code | Fix |
|------|------|-----|
| 53 | `var out []bundleColumn` | Pre-compute total column count |
| 1675 | `tokens := make(map[string]bool)` | Estimate from word count |
| 1248 | `adj := make(map[string][]string)` | `make(..., 2*len(relations))` |
| 1267 | `parent := make(map[string]string)` | `make(..., len(adj))` |
| 1268 | `seen := make(map[string]bool)` | `make(..., len(adj))` |

### 10d. Validator (per validation)

**File:** `internal/query/validator.go`

| Line | Code | Fix |
|------|------|-----|
| 31 | `dimMap := make(map[string]bool)` | `make(map[string]bool, len(model.Dimensions))` |
| 84 | `allowedFields := make(map[string]bool)` | `make(..., len(model.Dimensions)+len(model.Metrics))` |

### 10e. Planner (per query plan)

**File:** `internal/query/planner.go`

| Line | Code | Fix |
|------|------|-----|
| 36 | `var requiredJoins []string` | `make([]string, 0, len(model.Joins))` |
| 39 | `dimMap := make(map[string]semantic.Dimension)` | `make(..., len(model.Dimensions))` |
| 45 | `metricMap := make(map[string]semantic.Metric)` | `make(..., len(model.Metrics))` |
| 51 | `tables := make(map[string]bool)` | `make(..., len(model.Joins)+1)` |

### 10f. QuerySlice Generic (all repository queries)

**File:** `internal/platform/db/query.go:17`

```go
var out []T  // grows from nil
// Fix:
out := make([]T, 0, 64)  // reasonable default
```

### 10g. splitDot Hot Path

**File:** `internal/query/physical_ref.go:140`

```go
var result []string  // max 3 elements always
// Fix:
result := make([]string, 0, 3)
```

---

## 11. MEDIUM: json.Marshal Double-Allocation Pattern

`json.Marshal` -> `[]byte` -> `string(b)` pattern'i birden fazla yerde, her seferinde
ayni verinin 2x memory'de bulunmasina neden oluyor.

### 11a. Embedding Encoding

**File:** `internal/metadata/repository.go:316-325`

```go
func encodeEmbedding(vec []float32) (string, error) {
    b, err := json.Marshal(vec)  // 1536 float32 = ~10-15 KB
    return string(b), nil        // another 10-15 KB copy
}
```

### 11b. nullableJSON

**File:** `internal/metadata/repository.go:708-718`

```go
func nullableJSON(value any) (*string, error) {
    encoded, err := json.Marshal(value)  // allocation
    s := string(encoded)                 // second copy
    return &s, nil
}
```

**Impact:** AI prompt context (50+ KB) icin 100 KB peak per history write.

### 11c. Query Fingerprint

**File:** `internal/query/fingerprint.go:63-68`

```go
raw, err := json.Marshal(c)              // allocation
sum := sha256.Sum256(raw)
return hex.EncodeToString(sum[:])
```

### 11d. Sample Data in Prompt

**File:** `internal/ai/prompt.go:213-217`

```go
data, err := json.Marshal(s.Rows)  // intermediate []byte
sb.Write(data)                      // copies into builder, data becomes garbage

// Fix: use streaming encoder
enc := json.NewEncoder(&sb)
enc.Encode(s.Rows)  // writes directly into builder
```

---

## 12. MEDIUM: strings.NewReplacer Allocated Per Call

**File:** `internal/ai/table_router.go:1699-1715`

```go
func normalizeText(text string) string {
    replacer := strings.NewReplacer(  // allocated every call
        "İ", "i", "I", "i", "ı", "i",
        "Ş", "s", "ş", "s", /* ... */
    )
    text = strings.ToLower(replacer.Replace(text))
    // ...
}
```

**Impact:** 100 tablo x 10 kolon = 1000+ cagri, her biri `strings.NewReplacer` allocate ediyor.

**Fix:** Package-level `var`:

```go
var turkishReplacer = strings.NewReplacer(
    "İ", "i", "I", "i", "ı", "i",
    "Ş", "s", "ş", "s", "Ğ", "g", "ğ", "g",
    "Ü", "u", "ü", "u", "Ö", "o", "ö", "o",
    "Ç", "c", "ç", "c",
)
```

**Ayni sorun:** `internal/http/handlers/ai.go:695-720` `handlerTokenSet` fonksiyonunda da
ayni `strings.NewReplacer` her cagrida yeniden olusturuluyor.

---

## 13. MEDIUM: tokenSet Allocates Map Hundreds of Times Per Route

**File:** `internal/ai/table_router.go:1673-1682`

```go
func tokenSet(text string) map[string]bool {
    normalized := normalizeText(text)
    tokens := make(map[string]bool)  // new map every call
    // ...
}
```

**Impact:** `tokenSet` called from `scoreTable` (per table), `weightedTokenScore` (per column),
`selectAutomaticTables`, `appendEntityResolverTables`, vb.
100 tablo x 10 kolon = ~1000 map allocation. Question token'leri her seferinde ayni.

**Fix:** Question token'lerini bir kez hesaplayip pass et:

```go
type routingContext struct {
    questionTokens map[string]bool
    questionText   string
}
```

---

## 14. MEDIUM: relationAdjacency Rebuilt 3x Per Route Call

**File:** `internal/ai/table_router.go:1247-1259`

```go
func relationAdjacency(relations []metadata.Relation) map[string][]string {
    adj := make(map[string][]string)
    // ...
}
```

**Called from:** `appendEntityResolverTables`, `appendQuestionEntityTables`,
`expandSelectedWithJoinBridges` - hepsi ayni `Route` call icinde.

**Impact:** 200 relation icin 3x = 3 adjacency map, her biri ~400 entry.

**Fix:** `Route` icinde bir kez hesaplayip diger fonksiyonlara parametre olarak gec.

---

## 15. MEDIUM: []byte/string Conversions in Validation Path

**File:** `internal/ai/validator.go:37`

```go
if err := json.Unmarshal([]byte(cleaned), &lq); err != nil {  // full copy of LLM response
```

**Impact:** Her retry ve multi-candidate attempt icin tam LLM response kopyalaniyor.
5 retry x 5 candidate = 25 full copy.

**Fix:**

```go
if err := json.NewDecoder(strings.NewReader(cleaned)).Decode(&lq); err != nil {
```

---

## 16. MEDIUM: Cache.Key Uses Reflection-Based Formatting

**File:** `internal/platform/redis/redis.go:46-56`

```go
func (c *Cache) Key(datasourceID, modelID string, lq query.LogicalQuery, userScope string) string {
    data := struct {
        Query query.LogicalQuery `json:"q"`  // full struct copy
    }{datasourceID, modelID, lq, userScope}
    hash := sha256.Sum256([]byte(fmt.Sprintf("%+v", data)))  // reflection + large string
    return fmt.Sprintf("bi:query:%x", hash)
}
```

**Impact:** `%+v` reflection-based formatting, complex query icin 4-8 KB string.
`[]byte(...)` ile 2x memory. Per-query overhead.

**Fix:** `json.Marshal` veya binary fingerprint kullan.

---

## 17. MEDIUM: buildAIHistoryEntry Embeds 80K+ Rune Prompt

**File:** `internal/http/handlers/history.go:62-70`

```go
PromptContext: map[string]any{
    "prompt": resp.Prompt,  // up to 80K runes = ~80-320 KB
},
AIResponse: map[string]any{
    "response":     resp,              // entire Response object
    "raw_response": resp.RawResponse,  // full LLM response
},
```

**Impact:** Her AI query'de 80+ KB history entry memory'de tutulur DB write bitene kadar.

**Fix:** Prompt'u truncate et veya streaming JSON encoding kullan.

---

## 18. MEDIUM: QuerySlice Generic Helper No Capacity Hint

**File:** `internal/platform/db/query.go:17`

```go
var out []T  // nil, grows via append
```

**Impact:** Tum repository list query'leri buradan geciyor. 1000 row icin ~10 reallocation.
5000 column introspection icin ~13 reallocation.

**Fix:**

```go
out := make([]T, 0, 64)  // reasonable default, eliminates first 6 growth steps
```

---

## 19. MEDIUM: Duplicate dimMap Construction in CompileWithPermissions

**File:** `internal/query/compiler.go:47-53` ve `internal/query/compiler_nested.go:86`

```go
// In CompileWithPermissions:
dimMap := make(map[string]string)  // first build
for _, d := range model.Dimensions { dimMap[d.Name] = d.ColumnRef }

// Then Compile -> compileStatement:
dimMap := make(map[string]semantic.Dimension)  // second build, different type
for _, d := range model.Dimensions { dimMap[d.Name] = d }
```

**Impact:** Her permission-gated query icin dimension map 2 kez insaa ediliyor.

**Fix:** Shared helper veya map'i pass et.

---

## 20. MEDIUM: String Concatenation With += in Loops

**File:** `internal/ai/prompt.go:276-283, 298-305, 332-336`

```go
line := fmt.Sprintf("- %s (type: %s, column: %s)", d.Name, d.Type, d.ColumnRef)
if d.TimeGrain != "" {
    line = strings.TrimSuffix(line, ")") + fmt.Sprintf(", time_grain: %s)", d.TimeGrain)
}
if syn != "" {
    line += fmt.Sprintf(", synonyms: %s", syn)  // new allocation
}
line += "\n"  // another new allocation
```

**Impact:** 100 dimension x 4 intermediate string = 400 allocation per prompt build.
`strings.TrimSuffix` + concat = 3 allocation where 1 `fmt.Sprintf` would suffice.

**Fix:** Tek bir `fmt.Sprintf` veya direkt `sb.WriteString`:

```go
sb.WriteString(fmt.Sprintf("- %s (type: %s, column: %s%s%s)\n",
    d.Name, d.Type, d.ColumnRef,
    formatTimeGrain(d.TimeGrain),
    formatSynonyms(syn),
))
```

**Same pattern:** `internal/ai/glossary.go:272-279`

---

## 21. MEDIUM: []rune Allocation Before Truncation Check

**File:** `internal/ai/describe.go:250-258`, `internal/ai/glossary.go:289-290`

```go
func truncateStringRunes(s string, maxRunes int) string {
    runes := []rune(s)           // allocates full rune slice BEFORE checking
    if len(runes) <= maxRunes {  // often true, allocation wasted
        return s
    }
    return string(runes[:maxRunes]) + "…"
}
```

**Impact:** 100 table cell x ~2 KB rune slice = ~200 KB wasted allocation when no truncation needed.

**Fix:**

```go
func truncateStringRunes(s string, maxRunes int) string {
    if utf8.RuneCountInString(s) <= maxRunes {
        return s
    }
    // Only now allocate
    runes := []rune(s)
    return string(runes[:maxRunes]) + "…"
}
```

---

## 22. LOW: map[string]bool vs map[string]struct{}

**Files:** `internal/query/compiler.go:129,238,242,261,564`,
`internal/datasource/postgres/introspect.go:114`

```go
tables := make(map[string]bool)      // 1 byte per value
// Better:
tables := make(map[string]struct{})  // 0 bytes per value
```

**Impact:** 100 entry icin ~100 byte tasarruf. Per-query accumulation.
`struct{}` compiler optimization ile zero-allocation value.

---

## 23. LOW: writeJSON Reflection on Every HTTP Response

**File:** `internal/http/handlers/helpers.go:24-36`

```go
func writeJSON(w http.ResponseWriter, status int, data any) {
    v := reflect.ValueOf(data)  // reflection on every response
    if v.Kind() == reflect.Slice && v.IsNil() {
        data = reflect.MakeSlice(v.Type(), 0, 0).Interface()
    }
    json.NewEncoder(w).Encode(data)
}
```

**Impact:** ~48 bytes per call, prevents inlining.

**Fix:** Type-specific fast path veya `any` check:

```go
if data != nil {
    if sl, ok := data.([]someType); ok && sl == nil {
        data = []someType{}
    }
}
```

---

## 24. LOW: Operator Slices Allocated Per Validate Call

**File:** `internal/query/validator.go:67, 92-96`

```go
havingOps := []string{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpBetween, OpIsNull, OpIsNotNull}
validOps := []string{OpEq, OpNeq, /* 14 elements */}
```

**Fix:** Package-level `var`:

```go
var havingOps = []string{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpBetween, OpIsNull, OpIsNotNull}
var validOps = []string{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith, OpBetween, OpIsNull, OpIsNotNull}
```

---

## 25. LOW: UpsertColumns One-At-A-Time in Loop

**File:** `internal/http/handlers/datasources.go:576`

```go
for _, c := range result.Columns {
    h.deps.MetaRepo.UpsertColumns(ctx, ds.ID, []metadata.Column{col})  // 1-element slice per iteration
}
```

**Impact:** 10K column = 10K single-element slice allocation + 10K DB round-trip.

**Fix:** Batch:

```go
batch := make([]metadata.Column, 0, 100)
for _, c := range result.Columns {
    batch = append(batch, col)
    if len(batch) >= 100 {
        h.deps.MetaRepo.UpsertColumns(ctx, ds.ID, batch)
        batch = batch[:0]
    }
}
if len(batch) > 0 {
    h.deps.MetaRepo.UpsertColumns(ctx, ds.ID, batch)
}
```

---

## 26. LOW: Health Check []byte Allocation

**File:** `internal/http/router.go:42`

```go
r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"status":"ok"}`))  // allocates every call
})
```

**Fix:**

```go
var healthOK = []byte(`{"status":"ok"}`)

r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write(healthOK)
})
```

---

## Summary Table

| # | Severity | File | Issue | Hot Path |
| --- | ---------- | ------ | ------- | ---------- |
| 1 | CRITICAL | `query/executor.go:66-88` | Per-row triple allocation (30K allocs/query) | Yes |
| 2 | HIGH | `security/encryption.go:50-82` | Cipher recreated per call | Yes |
| 3 | HIGH | `ai/http_transport.go:12` | io.ReadAll unbounded, no limit | Yes |
| 4 | HIGH | `handlers/datasources.go:478-600` | Missing ctx cancellation, zombie goroutines | Sync |
| 5 | HIGH | `ai/prompt.go:66` | No sync.Pool for strings.Builder | Yes |
| 6 | HIGH | `ai/prompt.go:387-398` | BuildRetry duplicates 80K prompt | Yes |
| 7 | HIGH | `security/readonly.go:38`, `compiler.go:67` | strings.ToUpper full SQL copy | Yes |
| 8 | HIGH | `ai/prompt.go:93,120` | sb.String() full copy for rune count | Yes |
| 9 | MEDIUM | Multiple files | fmt.Sprintf vs strconv.Itoa (50+ occurrences) | Yes |
| 10 | MEDIUM | Multiple files | Slice/map without capacity hints (40+ locations) | Yes |
| 11 | MEDIUM | `metadata/repository.go`, `fingerprint.go`, `prompt.go` | json.Marshal double-allocation | Yes |
| 12 | MEDIUM | `table_router.go:1699`, `handlers/ai.go:695` | strings.NewReplacer per call | Yes |
| 13 | MEDIUM | `table_router.go:1673` | tokenSet map allocated 1000x per route | Yes |
| 14 | MEDIUM | `table_router.go:1247` | relationAdjacency rebuilt 3x | Yes |
| 15 | MEDIUM | `ai/validator.go:37` | []byte conversion per validation attempt | Yes |
| 16 | MEDIUM | `platform/redis/redis.go:54` | Reflection-based cache key | Yes |
| 17 | MEDIUM | `handlers/history.go:62-70` | 80K+ prompt embedded in history | Yes |
| 18 | MEDIUM | `platform/db/query.go:17` | QuerySlice no capacity hint (all queries) | Yes |
| 19 | MEDIUM | `compiler.go:47` + `compiler_nested.go:86` | Duplicate dimMap construction | Yes |
| 20 | MEDIUM | `prompt.go:276-336`, `glossary.go:272-279` | += string concat in loops | Yes |
| 21 | MEDIUM | `describe.go:250`, `glossary.go:289` | []rune before truncation check | Yes |
| 22 | LOW | Multiple files | map[string]bool vs map[string]struct{} | Yes |
| 23 | LOW | `handlers/helpers.go:24` | Reflection on every HTTP response | Yes |
| 24 | LOW | `query/validator.go:67,92` | Operator slices per Validate call | Yes |
| 25 | LOW | `handlers/datasources.go:576` | UpsertColumns one-at-a-time | Sync |
| 26 | LOW | `http/router.go:42` | Health check []byte per call | Health |

---

## Priority Fixes (Estimated Impact)

| Priority | Fix | Estimated Reduction |
| ---------- | ----- | ------------------- |
| 1 | Executor per-row allocation reuse | ~20,000 allocs/query |
| 2 | Encryption cipher caching | ~1 KB/query |
| 3 | Prompt builder sync.Pool | ~160 KB/concurrent request |
| 4 | sb.String() -> sb.Bytes() for rune count | ~20 KB/prompt build |
| 5 | io.ReadAll with size limit | Prevents OOM |
| 6 | Capacity hints on all compiler slices/maps | ~25 growth cycles/compilation |
| 7 | fmt.Sprintf -> strconv/concat in dialect/compiler | ~40 []any allocs/query |
| 8 | SyncMetadata ctx cancellation | Prevents multi-MB goroutine leaks |
| 9 | strings.NewReplacer package-level var | ~1000 allocations/route |
| 10 | tokenSet question tokens computed once | ~1000 map allocations/route |

---

*Analysis generated on 2026-05-19. Based on codebase scan of all Go files under `internal/`.*
