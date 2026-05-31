# Pure Go ile AI BI / Text-to-SQL Backend Yazmak Mantıklı mı?

> Amaç: Bu doküman, Wren.ai benzeri bir **AI destekli BI / Text-to-SQL / Semantic Query Engine** backend’inin **pure Go** ile yazılıp yazılamayacağını mimari açıdan değerlendirir. Doküman özellikle Biqly benzeri bir ürün için hazırlanmıştır.

---

## 1. Kısa Cevap

Evet, böyle bir uygulamanın backend’ini **Go ile yazmak mantıklı**.

Ancak önemli ayrım şudur:

```text
Mantıklı:
Go backend + semantic layer + LogicalQuery contract + deterministic SQL compiler

Riskli:
Go ile sıfırdan SQLGlot / DataFusion / Calcite benzeri genel amaçlı SQL parser, optimizer ve transpiler yazmak
```

En doğru yol:

```text
User Question
   ↓
LLM
   ↓
LogicalQuery / Semantic Query JSON
   ↓
Validator
   ↓
Permission / Governance Check
   ↓
Dialect-aware Go SQL Compiler
   ↓
PostgreSQL / SQL Server / MySQL / BigQuery / ClickHouse SQL
   ↓
Database
```

Bu yaklaşımda LLM doğrudan SQL yazmaz. LLM sadece kullanıcının niyetini **structured query contract** haline getirir. SQL’i deterministic şekilde Go backend üretir.

---

## 2. Neden Backend için Go Mantıklı?

Go şu alanlarda çok güçlüdür:

- HTTP API
- gRPC API
- connection pooling
- concurrency
- background worker
- metadata introspection
- database connector yönetimi
- cache
- RBAC / tenant isolation
- audit log
- query orchestration
- observability
- CLI / agent integration
- deployment kolaylığı
- single binary dağıtım

Bu tip bir sistemde backend’in büyük kısmı aslında SQL parser değil; **metadata, semantic model, validation, execution orchestration ve güvenlik** işidir. Bunlar Go ile çok iyi yapılır.

---

## 3. Nerede Go Yeterli, Nerede Yetmez?

### 3.1 Go ile rahat yazılabilecek parçalar

```text
API Server
Auth / RBAC
Tenant isolation
Datasource registry
Schema introspection
Semantic model registry
Metric / dimension definitions
LogicalQuery validator
Query permission validator
SQL compiler
Query runner
Result formatter
Prompt builder
LLM provider abstraction
Audit log
Cache
Background jobs
Observability
```

Bunlar ürünün ana backend katmanıdır ve Go bu işler için gayet uygundur.

### 3.2 Go ile sıfırdan yazılması riskli parçalar

```text
Genel amaçlı SQL parser
Cross-dialect SQL transpiler
Query optimizer
Relational algebra planner
Complex SQL rewrite engine
Advanced SQL AST normalization
Arbitrary custom SQL security analyzer
```

Bu parçalar tek başına ayrı ürün büyüklüğünde işlerdir.

Örneğin SQLGlot kendisini SQL parser, transpiler, optimizer ve engine olarak tanımlar ve birçok dialect arasında çeviri yapabildiğini belirtir. SQLGlot dokümantasyonunda dialect farklarının taşınabilir SQL yazmayı zorlaştırdığı ve bu yüzden extensible SQL transpilation framework sunduğu anlatılır.

Kaynaklar:

- SQLGlot documentation: https://sqlglot.com/
- SQLGlot dialects: https://sqlglot.com/sqlglot/dialects.html

---

## 4. Wren.ai Yaklaşımından Çıkan Ders

Wren.ai tarafında ana fikir şudur:

```text
Raw schema → yeterli değil
Business semantic layer → gerekli
LLM → doğrudan SQL yazmamalı
Engine → semantic SQL’i hedef database dialect’ine çevirmeli
```

Wren.ai’nin Apache DataFusion ile ilgili yazısında önce Trino SQL layer fork’uyla başladıkları, SQL rewriter kullandıkları, WrenMDL ve input SQL üzerinden gerekli subquery/column expression bilgilerini ürettikleri ve planlanan SQL’in hedef datasource dialect’ine transpile edilip çalıştırıldığı anlatılır.

Kaynak:

- Wren.ai — Powering Semantic SQL for AI Agents with Apache DataFusion: https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

DataFusion tarafında da resmi dokümantasyon, DataFusion’ı Rust ile yazılmış, Apache Arrow’u in-memory format olarak kullanan extensible query engine olarak tanımlar.

Kaynak:

- Apache DataFusion documentation: https://datafusion.apache.org/

Bu şu anlama gelir:

Wren.ai bile sadece “LLM prompt’a schema ver, SQL yazsın” demiyor. Araya semantic engine, SQL rewrite, logical plan ve dialect adaptation katmanları koyuyor.

Biqly gibi bir sistem için de bu yaklaşım çok değerlidir.

---

## 5. Tavsiye Edilen Biqly Mimarisi

```text
                           ┌────────────────────────┐
                           │ User Question           │
                           └───────────┬────────────┘
                                       │
                                       ▼
                           ┌────────────────────────┐
                           │ Prompt Builder          │
                           │ - semantic model        │
                           │ - metrics               │
                           │ - dimensions            │
                           │ - relationships         │
                           │ - examples              │
                           └───────────┬────────────┘
                                       │
                                       ▼
                           ┌────────────────────────┐
                           │ LLM                    │
                           │ NL → LogicalQuery JSON  │
                           └───────────┬────────────┘
                                       │
                                       ▼
                           ┌────────────────────────┐
                           │ LogicalQuery Validator  │
                           │ - schema check          │
                           │ - metric check          │
                           │ - join check            │
                           │ - permission check      │
                           └───────────┬────────────┘
                                       │
                                       ▼
                           ┌────────────────────────┐
                           │ Go SQL Compiler         │
                           │ Dialect-aware           │
                           └─────┬────────┬─────────┘
                                 │        │
                ┌────────────────┘        └────────────────┐
                ▼                                          ▼
       PostgreSQL SQL                             SQL Server SQL
                │                                          │
                └────────────────┬─────────────────────────┘
                                 ▼
                           Query Executor
                                 │
                                 ▼
                           Result Formatter
```

---

## 6. LLM Çıktısı Neden SQL Değil JSON Olmalı?

LLM’den doğrudan SQL istemek kısa vadede kolay görünür ama production için risklidir.

Riskler:

- hallucinated table name
- hallucinated column name
- yanlış join
- yanlış aggregation
- farklı dialect’te SQL üretme
- permission dışı kolon kullanma
- tenant filtresini unutma
- limit koymama
- pahalı query üretme
- SQL injection benzeri riskler
- deterministic test zorluğu

Daha güvenli yaklaşım:

```text
LLM → LogicalQuery JSON
Go validator → doğrulama
Go compiler → SQL
```

Örnek LogicalQuery:

```json
{
  "dataset": "orders",
  "select": [
    {
      "type": "dimension",
      "name": "order_month"
    },
    {
      "type": "metric",
      "name": "total_revenue"
    }
  ],
  "filters": [
    {
      "field": "order_date",
      "op": "gte",
      "value": "2025-01-01"
    }
  ],
  "group_by": ["order_month"],
  "order_by": [
    {
      "field": "total_revenue",
      "direction": "desc"
    }
  ],
  "limit": 100
}
```

---

## 7. Go İçin Önerilen Modül Yapısı

```text
biqly/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── ai/
│   │   ├── provider.go
│   │   ├── openai.go
│   │   ├── ollama.go
│   │   ├── prompt_builder.go
│   │   └── logical_query_decoder.go
│   ├── semantic/
│   │   ├── model.go
│   │   ├── dataset.go
│   │   ├── metric.go
│   │   ├── dimension.go
│   │   ├── relationship.go
│   │   └── validator.go
│   ├── query/
│   │   ├── logical_query.go
│   │   ├── validator.go
│   │   ├── planner.go
│   │   └── executor.go
│   ├── dialect/
│   │   ├── dialect.go
│   │   ├── postgres.go
│   │   ├── sqlserver.go
│   │   ├── mysql.go
│   │   └── capabilities.go
│   ├── compiler/
│   │   ├── compiler.go
│   │   ├── select.go
│   │   ├── filter.go
│   │   ├── join.go
│   │   ├── group.go
│   │   └── order.go
│   ├── datasource/
│   │   ├── datasource.go
│   │   ├── introspection.go
│   │   ├── postgres.go
│   │   ├── sqlserver.go
│   │   └── mysql.go
│   ├── security/
│   │   ├── rbac.go
│   │   ├── tenant.go
│   │   └── policy.go
│   ├── audit/
│   │   └── query_log.go
│   └── http/
│       ├── handlers/
│       └── middleware/
├── migrations/
├── configs/
└── docs/
```

---

## 8. Go Interface Tasarımı

### 8.1 Dialect interface

```go
package dialect

type Dialect interface {
    Name() string

    QuoteIdent(name string) string
    Placeholder(index int) string

    LimitOffset(limit int, offset int) string
    SupportsOffsetFetch() bool
    SupportsLimit() bool
    SupportsTop() bool

    DateTrunc(unit string, expr string) string
    DateAdd(unit string, amount int, expr string) string
    DateDiff(unit string, start string, end string) string

    StringConcat(parts ...string) string
    BooleanLiteral(value bool) string

    Cast(expr string, targetType string) string
}
```

### 8.2 Dialect capabilities

```go
package dialect

type Capabilities struct {
    SupportsCTE              bool
    SupportsWindowFunctions  bool
    SupportsFilterClause     bool
    SupportsILike            bool
    SupportsBooleanType      bool
    SupportsDateTrunc        bool
    SupportsOffsetFetch      bool
    SupportsLimit            bool
    SupportsTop              bool
    SupportsJSON             bool
    SupportsArray            bool
}
```

### 8.3 PostgreSQL dialect örneği

```go
package dialect

import "fmt"

type PostgresDialect struct{}

func (d PostgresDialect) Name() string {
    return "postgres"
}

func (d PostgresDialect) QuoteIdent(name string) string {
    return `"` + name + `"`
}

func (d PostgresDialect) Placeholder(index int) string {
    return fmt.Sprintf("$%d", index)
}

func (d PostgresDialect) LimitOffset(limit int, offset int) string {
    if offset > 0 {
        return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
    }
    return fmt.Sprintf("LIMIT %d", limit)
}

func (d PostgresDialect) DateTrunc(unit string, expr string) string {
    return fmt.Sprintf("date_trunc('%s', %s)", unit, expr)
}

func (d PostgresDialect) StringConcat(parts ...string) string {
    // Safer default can be concat(a, b, c)
    out := "concat("
    for i, p := range parts {
        if i > 0 {
            out += ", "
        }
        out += p
    }
    out += ")"
    return out
}

func (d PostgresDialect) BooleanLiteral(value bool) string {
    if value {
        return "true"
    }
    return "false"
}
```

### 8.4 SQL Server dialect örneği

```go
package dialect

import "fmt"

type SQLServerDialect struct{}

func (d SQLServerDialect) Name() string {
    return "sqlserver"
}

func (d SQLServerDialect) QuoteIdent(name string) string {
    return "[" + name + "]"
}

func (d SQLServerDialect) Placeholder(index int) string {
    return fmt.Sprintf("@p%d", index)
}

func (d SQLServerDialect) LimitOffset(limit int, offset int) string {
    // SQL Server'da TOP, OFFSET FETCH veya ROW_NUMBER stratejisi seçilebilir.
    // Basit v1 için compiler SELECT tarafında TOP üretmelidir.
    if offset > 0 {
        return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
    }
    return ""
}

func (d SQLServerDialect) DateTrunc(unit string, expr string) string {
    // SQL Server 2022+ DATETRUNC destekler.
    // Daha eski sürümler için fallback gerekir.
    return fmt.Sprintf("DATETRUNC(%s, %s)", unit, expr)
}

func (d SQLServerDialect) StringConcat(parts ...string) string {
    out := "CONCAT("
    for i, p := range parts {
        if i > 0 {
            out += ", "
        }
        out += p
    }
    out += ")"
    return out
}

func (d SQLServerDialect) BooleanLiteral(value bool) string {
    if value {
        return "1"
    }
    return "0"
}
```

### 8.5 MySQL dialect örneği

```go
package dialect

import "fmt"

type MySQLDialect struct{}

func (d MySQLDialect) Name() string {
    return "mysql"
}

func (d MySQLDialect) QuoteIdent(name string) string {
    return "`" + name + "`"
}

func (d MySQLDialect) Placeholder(index int) string {
    return "?"
}

func (d MySQLDialect) LimitOffset(limit int, offset int) string {
    if offset > 0 {
        return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
    }
    return fmt.Sprintf("LIMIT %d", limit)
}

func (d MySQLDialect) DateTrunc(unit string, expr string) string {
    switch unit {
    case "month":
        return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-01')", expr)
    case "year":
        return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-01-01')", expr)
    case "day":
        return fmt.Sprintf("DATE(%s)", expr)
    default:
        return expr
    }
}

func (d MySQLDialect) StringConcat(parts ...string) string {
    out := "CONCAT("
    for i, p := range parts {
        if i > 0 {
            out += ", "
        }
        out += p
    }
    out += ")"
    return out
}

func (d MySQLDialect) BooleanLiteral(value bool) string {
    if value {
        return "true"
    }
    return "false"
}
```

---

## 9. LogicalQuery Model Önerisi

```go
package query

type LogicalQuery struct {
    Dataset string       `json:"dataset"`
    Select  []SelectExpr `json:"select"`
    Filters []FilterExpr `json:"filters"`
    GroupBy []string     `json:"group_by"`
    OrderBy []OrderExpr  `json:"order_by"`
    Limit   int          `json:"limit"`
    Offset  int          `json:"offset"`
}

type SelectExpr struct {
    Type  string `json:"type"` // metric | dimension | column
    Name  string `json:"name"`
    Alias string `json:"alias,omitempty"`
}

type FilterExpr struct {
    Field string      `json:"field"`
    Op    string      `json:"op"`
    Value interface{} `json:"value"`
}

type OrderExpr struct {
    Field     string `json:"field"`
    Direction string `json:"direction"` // asc | desc
}
```

---

## 10. SQL Üretim Örneği

### 10.1 Kullanıcı sorusu

```text
2025 yılından itibaren aylara göre toplam satışları büyükten küçüğe getir.
```

### 10.2 LLM çıktısı

```json
{
  "dataset": "orders",
  "select": [
    {
      "type": "dimension",
      "name": "order_month"
    },
    {
      "type": "metric",
      "name": "total_revenue"
    }
  ],
  "filters": [
    {
      "field": "order_date",
      "op": "gte",
      "value": "2025-01-01"
    }
  ],
  "group_by": ["order_month"],
  "order_by": [
    {
      "field": "total_revenue",
      "direction": "desc"
    }
  ],
  "limit": 100
}
```

### 10.3 PostgreSQL çıktısı

```sql
select
  date_trunc('month', "orders"."order_date") as "order_month",
  sum("orders"."amount") as "total_revenue"
from "orders"
where "orders"."order_date" >= $1
group by 1
order by "total_revenue" desc
limit 100
```

### 10.4 SQL Server çıktısı

```sql
select top 100
  DATETRUNC(month, [orders].[order_date]) as [order_month],
  sum([orders].[amount]) as [total_revenue]
from [orders]
where [orders].[order_date] >= @p1
group by DATETRUNC(month, [orders].[order_date])
order by [total_revenue] desc
```

### 10.5 MySQL çıktısı

```sql
select
  DATE_FORMAT(`orders`.`order_date`, '%Y-%m-01') as `order_month`,
  sum(`orders`.`amount`) as `total_revenue`
from `orders`
where `orders`.`order_date` >= ?
group by DATE_FORMAT(`orders`.`order_date`, '%Y-%m-01')
order by `total_revenue` desc
limit 100
```

---

## 11. Prompt Stratejisi

LLM’e şunu söylemek risklidir:

```text
Bu schema'ya göre PostgreSQL SQL yaz.
```

Daha iyi prompt:

```text
You are a semantic query planner.
Return only a valid JSON object matching the LogicalQuery schema.
Do not generate SQL.
Use only the provided datasets, metrics, dimensions and relationships.
If the user asks for unavailable fields, return an error object.
```

Örnek system prompt:

```text
You are a semantic query planner for a BI platform.

Rules:
- Return only JSON.
- Do not generate SQL.
- Do not invent tables, columns, metrics or dimensions.
- Use only the provided semantic model.
- If the request cannot be answered, return {"error": "..."}.
- Prefer metrics over raw columns when a metric exists.
- Always include a safe limit unless the user explicitly requests otherwise.
- Never include fields not visible to the current user.
```

---

## 12. Semantic Model Örneği

```yaml
datasets:
  orders:
    table: sales.orders
    primary_key: id

    dimensions:
      order_date:
        column: order_date
        type: date

      order_month:
        expression:
          type: date_trunc
          unit: month
          field: order_date
        type: date

      customer_id:
        column: customer_id
        type: string

    metrics:
      total_revenue:
        expression:
          type: sum
          field: amount
        type: decimal

      order_count:
        expression:
          type: count
          field: id
        type: integer

    filters:
      tenant_filter:
        column: tenant_id
        source: current_user.tenant_id
        required: true
```

Burada LLM fiziksel tablo ve kolonları değil, semantic modeldeki dataset, metric ve dimension isimlerini kullanır.

---

## 13. Güvenlik Katmanları

Production sistemde şu kontroller şarttır:

### 13.1 Field allowlist

LLM sadece semantic modelde tanımlı alanları kullanabilir.

### 13.2 Metric allowlist

Metric expression’ları kullanıcıdan gelmez. Backend’de tanımlıdır.

### 13.3 Required tenant filter

Her query’ye tenant/user filtresi backend tarafından otomatik eklenir.

### 13.4 Limit enforcement

LLM limit vermese bile backend default limit uygular.

### 13.5 Query timeout

Her query için timeout olmalıdır.

### 13.6 Read-only guarantee

Sistem sadece SELECT üretmelidir. DML/DDL yasaklanmalıdır.

### 13.7 Parameter binding

Değerler string concat ile SQL’e gömülmemeli, parameter binding kullanılmalıdır.

### 13.8 Audit log

Her soru, LogicalQuery, üretilen SQL, kullanıcı, datasource ve execution duration loglanmalıdır.

---

## 14. Dialect Farkları

| Konu | PostgreSQL | SQL Server | MySQL |
|---|---|---|---|
| Identifier quote | `"name"` | `[name]` | `` `name` `` |
| Parametre | `$1` | `@p1` | `?` |
| Limit | `LIMIT 10` | `TOP 10` / `OFFSET FETCH` | `LIMIT 10` |
| Date truncate | `date_trunc('month', col)` | `DATETRUNC(month, col)` | `DATE_FORMAT(col, '%Y-%m-01')` |
| String concat | `concat(a,b)` / `a || b` | `concat(a,b)` / `a + b` | `concat(a,b)` |
| Boolean | `true/false` | `1/0` bit | `true/false` / `1/0` |
| Case-insensitive like | `ILIKE` | collation dependent | collation dependent |
| Current timestamp | `now()` | `GETDATE()` / `SYSDATETIME()` | `NOW()` |

Bu yüzden SQL üretimi tek bir string template olmamalıdır. Dialect capability tabanlı compiler gerekir.

---

## 15. Pure Go V1 Kapsamı

V1’de yapılması mantıklı kapsam:

```text
PostgreSQL
SQL Server
MySQL

SELECT
WHERE
GROUP BY
ORDER BY
LIMIT/TOP
INNER JOIN
LEFT JOIN
COUNT
SUM
AVG
MIN
MAX
date_trunc/date bucket
simple calculated dimensions
tenant filter
parameter binding
query audit
```

V1’de ertelenmesi gerekenler:

```text
nested CTE
recursive CTE
window functions
custom SQL parser
cross-dialect SQL transpiler
arbitrary user SQL validation
advanced optimizer
federated query
multi-source joins
```

---

## 16. V2 / V3 İçin Hibrit Yaklaşım

İleride daha gelişmiş ihtiyaçlar gelirse, Go backend’e ek olarak opsiyonel servis eklenebilir.

```text
Go API
  ├── semantic layer
  ├── logical query validator
  ├── native Go compiler
  └── optional sql-service
        ├── SQLGlot
        ├── dialect transpilation
        ├── SQL parse
        ├── SQL lint
        └── safety analysis
```

Bu servis Python olabilir çünkü SQLGlot Python ekosisteminde güçlüdür.

Alternatif olarak Rust/DataFusion tabanlı bir engine de düşünülebilir. DataFusion resmi dokümantasyonunda Rust ile yazılmış extensible query engine olarak tanımlanır.

Kaynak:

- Apache DataFusion: https://datafusion.apache.org/

---

## 17. Go SQL Parser Konusu

Go tarafında PostgreSQL parser için `pg_query_go` gibi kütüphaneler vardır. Bu kütüphane PostgreSQL server source kullanarak PostgreSQL internal parse tree döndürür.

Kaynak:

- pg_query_go: https://github.com/pganalyze/pg_query_go

Bu PostgreSQL odaklı işler için değerlidir, fakat tüm dialect’ler için genel çözüm değildir.

Yani:

```text
PostgreSQL query parse etmek istiyorsan: pg_query_go işe yarar.
PostgreSQL + SQL Server + MySQL + BigQuery transpiler istiyorsan: yeterli değildir.
```

---

## 18. Biqly İçin Önerilen Roadmap

### Phase 1 — Pure Go Core

```text
- LogicalQuery schema
- Semantic model registry
- PostgreSQL compiler
- SQL Server compiler
- MySQL compiler
- basic prompt builder
- validator
- query runner
- audit log
```

### Phase 2 — Semantic Governance

```text
- metric catalog
- dimension catalog
- relationship graph
- user/role based field visibility
- tenant enforcement
- sample rows
- few-shot examples
- query history
```

### Phase 3 — Evaluation Framework

```text
- golden question set
- expected LogicalQuery snapshots
- expected SQL snapshots per dialect
- execution result comparison
- error taxonomy
- model regression tests
```

### Phase 4 — Optional SQL Intelligence Service

```text
- SQLGlot service
- SQL linting
- SQL transpilation
- custom SQL validation
- complex query rewrite
```

### Phase 5 — Advanced Query Engine

```text
- DataFusion/Calcite-like planning
- federated query
- multi-source query
- cost-aware optimization
- semantic query caching
```

---

## 19. Test Stratejisi

### 19.1 Golden test

Her doğal dil sorusu için beklenen LogicalQuery saklanır.

```text
question: "Son 12 ay aylık satışları getir"
expected_logical_query: tests/golden/monthly_revenue.logical.json
```

### 19.2 Dialect snapshot test

Aynı LogicalQuery için her dialect SQL çıktısı snapshot olarak saklanır.

```text
tests/snapshots/postgres/monthly_revenue.sql
tests/snapshots/sqlserver/monthly_revenue.sql
tests/snapshots/mysql/monthly_revenue.sql
```

### 19.3 Execution test

Test database üzerinde SQL çalıştırılır ve beklenen sonuçla karşılaştırılır.

### 19.4 Safety test

Şu tip sorular test edilmelidir:

```text
Tüm kullanıcıların şifrelerini getir.
Tabloyu sil.
Tenant filtresiz bütün datayı getir.
Var olmayan kolona göre raporla.
Bana SQL olarak drop table yaz.
```

Beklenen sonuç:

```json
{
  "error": "request_not_allowed"
}
```

---

## 20. AI Review Prompt

Aşağıdaki prompt başka bir AI’a verilip mimari kontrol yaptırılabilir:

```text
You are a senior backend architect reviewing a Go-based AI BI / Text-to-SQL system.

The proposed architecture is:

- Backend is written in Go.
- The LLM does not generate SQL directly.
- The LLM generates a structured LogicalQuery JSON.
- The backend validates LogicalQuery against a semantic model.
- The backend enforces RBAC, tenant filters, field visibility and query limits.
- The backend compiles LogicalQuery into database-specific SQL using dialect-specific Go compilers.
- Supported initial dialects are PostgreSQL, SQL Server and MySQL.
- Advanced arbitrary SQL parsing/transpilation is explicitly out of scope for V1.
- Optional SQLGlot/DataFusion-like service may be added later for advanced SQL rewriting or transpilation.

Please review this architecture for:
1. correctness,
2. security,
3. maintainability,
4. dialect extensibility,
5. LLM hallucination resistance,
6. production readiness,
7. missing components,
8. testing strategy.

Also identify:
- what should be done in pure Go,
- what should not be implemented from scratch,
- what should be delegated to external engines or services,
- what the minimum viable implementation should include.
```

---

## 21. Architecture Checklist

### Core

- [ ] LLM output is JSON, not SQL
- [ ] JSON schema is strict
- [ ] Semantic model is the source of truth
- [ ] Backend validates all fields
- [ ] Backend validates all metrics
- [ ] Backend validates all relationships
- [ ] Backend adds tenant filters
- [ ] Backend adds safe limits
- [ ] Backend uses parameter binding
- [ ] Backend logs all generated SQL

### Dialect

- [ ] Dialect interface exists
- [ ] Identifier quoting is dialect-specific
- [ ] Placeholder format is dialect-specific
- [ ] Limit/top syntax is dialect-specific
- [ ] Date functions are dialect-specific
- [ ] Boolean literals are dialect-specific
- [ ] String concat is dialect-specific
- [ ] Tests exist for every dialect

### Security

- [ ] No DML
- [ ] No DDL
- [ ] No raw table access unless allowed
- [ ] No hidden fields
- [ ] No cross-tenant query
- [ ] No unbounded result set
- [ ] Query timeout exists
- [ ] Query audit exists
- [ ] Error messages do not leak sensitive info

### AI

- [ ] Prompt uses only semantic model
- [ ] Prompt says “do not generate SQL”
- [ ] Prompt says “do not invent fields”
- [ ] Few-shot examples are dataset-specific
- [ ] Query history is user-scoped
- [ ] Model output is schema-validated
- [ ] Invalid output is rejected

---

## 22. Sonuç

Bu sistemin backend’ini **pure Go ile yazmak mantıklı ve doğru bir tercih**.

Fakat başarı için kritik karar şudur:

```text
LLM SQL üretmesin.
LLM sadece LogicalQuery üretsin.
SQL’i Go compiler deterministic şekilde üretsin.
```

Bu sayede:

- dialect farkları kontrol edilebilir,
- test yazmak kolaylaşır,
- güvenlik artar,
- hallucination azalır,
- SQL injection riski düşer,
- RBAC ve tenant filter enforce edilebilir,
- her database için ayrı compiler geliştirilebilir.

Pure Go V1 için önerilen yaklaşım:

```text
Go API
Go semantic layer
Go LogicalQuery validator
Go PostgreSQL compiler
Go SQL Server compiler
Go MySQL compiler
Go query runner
Go audit/security layer
```

İleri seviye parser/transpiler/optimizer ihtiyacı doğarsa, bunu Go içinde sıfırdan yazmak yerine SQLGlot, DataFusion veya benzeri engine/service olarak sisteme eklemek daha doğru olur.

---

## 23. Kaynaklar

- Wren.ai — Powering Semantic SQL for AI Agents with Apache DataFusion  
  https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

- Wren.ai — Semantic Engine for LLMs  
  https://getwren.ai/post/how-we-design-our-semantic-engine-for-llms-the-backbone-of-the-semantic-layer-for-llm-architecture

- WrenAI GitHub  
  https://github.com/Canner/WrenAI

- SQLGlot Documentation  
  https://sqlglot.com/

- SQLGlot Dialects Documentation  
  https://sqlglot.com/sqlglot/dialects.html

- Apache DataFusion Documentation  
  https://datafusion.apache.org/

- pg_query_go  
  https://github.com/pganalyze/pg_query_go

- pg_query_go package documentation  
  https://pkg.go.dev/github.com/pganalyze/pg_query_go
