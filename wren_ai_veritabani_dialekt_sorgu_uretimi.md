# Wren.ai Veritabanı Tiplerine Göre SQL Üretimi: Dialect, Connector ve Semantic Engine Dokümantasyonu

> Amaç: Bu doküman, Wren.ai / Wren Engine’in PostgreSQL, MySQL, SQL Server, BigQuery, Snowflake, ClickHouse, Trino vb. farklı veritabanı tipleri için sorgu üretimini nasıl yönettiğini açıklamak için hazırlanmıştır. Ayrıca Biqly veya benzeri bir Text-to-SQL motorunda aynı yaklaşımın nasıl uygulanabileceğine dair kontrol listesi ve mimari öneri içerir.

---

## 1. Kısa sonuç

Wren.ai’nin yaklaşımı “LLM her veritabanı için ayrı ayrı SQL yazsın” modeli değildir. Daha güvenli ve sürdürülebilir yaklaşım şudur:

1. **Veritabanı bağlantısı / connector seçilir.**
   - PostgreSQL, MySQL, SQL Server, BigQuery, Snowflake, ClickHouse, Trino vb. için ayrı connection/profile bilgisi tutulur.
2. **Fiziksel schema introspect edilir.**
   - Table, column, type, primary key / foreign key ve ilişki bilgileri alınır.
3. **Bu schema MDL / semantic layer’a dönüştürülür.**
   - Agent ham tabloya değil, model, relationship, calculated field, view, instruction ve business definition bilgisine bakar.
4. **LLM genellikle semantic SQL / generic SQL üretir.**
   - Model isimleri, exposed column’lar, relationship’ler ve business kuralları kullanılır.
5. **Wren Engine SQL’i semantic layer üzerinden planlar ve rewrite eder.**
   - Model referansları fiziksel tablo referanslarına çözülür.
   - Calculated field, relationship, view, filter, expression gibi semantic kurallar SQL’e eklenir.
6. **Hedef veritabanı dialect’ine transpile edilir.**
   - Örneğin generic SQL → PostgreSQL SQL, MySQL SQL, SQL Server T-SQL, BigQuery SQL vb.
7. **Hedef connector üzerinden execute edilir.**
   - Query pushdown hedeflenir; yani sorgu mümkün olduğunca verinin olduğu motorda çalıştırılır.
8. **Dry-run / validation / execution feedback ile düzeltme yapılır.**
   - SQL geçersizse hata mesajı LLM’e feedback olarak döner ve sorgu tekrar üretilir.

Wren tarafındaki kritik fikir: **AI’ın asıl görevi dialect ezberlemek değil, doğru semantic niyeti SQL’e çevirmektir. Dialect uyarlaması engine/connector katmanında yapılır.**

---

## 2. Kaynaklardan görünen mimari

Wren dokümantasyonuna göre açık çekirdek içinde şu parçalar öne çıkar:

- **MDL / Modeling Definition Language:** Modelleri, ilişkileri, calculated field’ları, view’ları ve agent metadata’sını tanımlayan semantic contract.
- **Rust semantic engine / Apache DataFusion:** Modeled SQL’i desteklenen kaynaklar üzerinde planlayıp çalıştıran engine.
- **Wren CLI:** Query, plan, validate, context build, profiling ve memory yönetimi için kullanılır.
- **Skills:** AI coding agent’ların Wren’i güvenli ve tekrar üretilebilir şekilde kullanması için workflow tanımları.

Wren dokümanında desteklenen veri kaynakları arasında PostgreSQL, MySQL, BigQuery, Snowflake, DuckDB, ClickHouse, Trino, SQL Server, Databricks, Redshift, Oracle, Athena ve Apache Spark sayılır.

Kaynaklar:

- Wren AI OSS Introduction: https://docs.getwren.ai/oss/introduction
- Wren Engine GitHub README: https://github.com/Canner/wren-engine
- Wren Engine + Apache DataFusion blog: https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

---

## 3. Veritabanı tipine göre ayrım nerede yapılıyor?

Wren mimarisinde veritabanı tipi ayrımı üç seviyede yapılır:

```text
[Connection/Profile]
       ↓
[Connector / SQLAlchemy / Ibis / native driver]
       ↓
[Dialect transpilation + execution]
```

### 3.1 Connection / profile seviyesi

Her veri kaynağı için connection ayarları farklıdır. Örnekler:

#### PostgreSQL

Wren PostgreSQL connection ekranında şu bilgileri ister:

- display name
- host
- port
- username
- password
- database name
- SSL seçeneği

PostgreSQL için ayrıca Wren’in query execution sırasında temporary view oluşturup silebildiği belirtilir. Bu yüzden kullanıcı izinleri yalnızca `SELECT` ile sınırlı kalmayabilir; advanced analytics/modeling özellikleri için temporary view create/drop yetkileri gerekebilir.

Kaynak: https://docs.getwren.ai/oss/guide/connect/postgresql

#### MySQL

MySQL tarafında Wren şu ayrıntıyı özellikle belirtir:

- **MySQL 8.0 ve üzeri desteklenir.**
- Connection alanları host, port, username, password, database name ve SSL şeklindedir.
- Table selection sonrası seçilen her tablo data model olarak oluşturulur.
- Primary key / foreign key varsa relationship önerileri çıkarılır.

Kaynak: https://docs.getwren.ai/oss/guide/connect/mysql

#### SQL Server

SQL Server commercial documentation tarafında connection alanları şunlardır:

- display name
- host
- port
- username
- password
- database name

Wren Cloud kullanılıyorsa SQL Server firewall tarafında Wren outbound IP adresinin whitelist edilmesi gerekir. SQL Server connection sonrası seçilen tablolar model haline getirilir ve FK/PK üzerinden ilişki önerileri çıkarılır.

Kaynak: https://docs.getwren.ai/cp/guide/connect/sqlserver

---

## 4. Connector / dependency seviyesi

Wren CLI quickstart dokümanında farklı veritabanları için Python package extra’ları listelenir:

```bash
pip install "wrenai[memory,main,postgres]"
pip install "wrenai[memory,main,mysql]"
pip install "wrenai[memory,main,mssql]"
pip install "wrenai[memory,main,bigquery]"
pip install "wrenai[memory,main,snowflake]"
pip install "wrenai[memory,main,clickhouse]"
pip install "wrenai[memory,main,trino]"
pip install "wrenai[memory,main,redshift]"
pip install "wrenai[memory,main,athena]"
pip install "wrenai[memory,main,oracle]"
pip install "wrenai[memory,main,spark]"
```

Bu şu anlama gelir:

- Wren tek bir hard-coded SQL generator kullanmaz.
- Data source tipine göre ilgili connector dependency’si yüklenir.
- Profile içinde `data_source` / `datasource` bilgisi tutulur.
- Agent’ın kullandığı proje belirli bir profile’a bağlanır.
- Query execution, bağlı profile ve data source üzerinden yapılır.

Kaynak: https://docs.getwren.ai/oss/get_started/quickstart

---

## 5. MDL: database bağımlılığını azaltan katman

Dialect farkını yönetmenin en önemli parçası MDL’dir.

Wren’de agent doğrudan şu tabloyu görmez:

```sql
sales.fact_invoice_line
```

Bunun yerine semantic model görür:

```yaml
models:
  - name: InvoiceLines
    table_reference:
      catalog: real_catalog
      schema: sales
      table: fact_invoice_line
    columns:
      - name: invoice_date
        type: date
        description: Invoice date
      - name: net_amount
        type: decimal
        description: Net sales amount excluding tax
```

Bu sayede LLM şunlara odaklanır:

- Hangi model kullanılmalı?
- Hangi kolon business olarak ne anlama geliyor?
- Hangi ilişki ile join yapılmalı?
- Hangi calculated field kullanılmalı?
- Hangi default filter / instruction uygulanmalı?

Dialect ayrıntıları engine tarafına kalır.

Wren dokümanındaki örneğe göre Wren, MDL içinde `customers` modelini bulur, bunu `table_reference` içinde tanımlanan fiziksel tabloya çözer, modelde expose edilmeyen `email`, `phone` gibi kolonları agent’a görünmez tutar ve `instructions.md` kurallarını uygular.

Kaynak: https://docs.getwren.ai/oss/introduction

---

## 6. SQL üretim pipeline’ı

Aşağıdaki pipeline, Wren yaklaşımının pratik karşılığıdır:

```text
User question
  ↓
Intent/context retrieval
  ↓
Relevant MDL models + columns + relationships + instructions
  ↓
LLM generates semantic/generic SQL
  ↓
Wren Engine validates semantic references
  ↓
SQL rewriter expands models, views, calculated fields, relationships
  ↓
Logical plan / IR
  ↓
Dialect transpilation
  ↓
Connector execution
  ↓
Result + error feedback + memory store
```

### 6.1 Context retrieval

Wren quickstart’ta day-to-day query workflow için agent’ın şu işleri yaptığı anlatılır:

1. `wren memory fetch` ile ilgili context’i getirir.
2. `wren memory recall` ile benzer geçmiş NL-SQL örneklerini bulur.
3. Semantic layer kullanarak SQL yazar.
4. `wren --sql` ile Wren Engine üzerinden execute eder.
5. Başarılı NL-SQL pair’i memory’ye store eder.

Kaynak: https://docs.getwren.ai/oss/get_started/quickstart

### 6.2 Semantic SQL / generic SQL üretimi

Wren Engine blog’unda AI agent’ların farklı veritabanlarıyla konuşurken SQL dialect farkları yüzünden syntax sorunları yaşadığı; Wren Engine’in ise agent’a unified SQL interface sunduğu anlatılır. Agent generic SQL yazar, Wren Engine bu SQL’i hedef database dialect’ine transpile eder.

Kaynak: https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

### 6.3 SQL rewrite

Wren Engine’in eski mimari açıklamasında SQL rewriter’ın Antlr4 Visitor ile WrenMDL ve input SQL’i alıp zenginleştirilmiş SQL ürettiği belirtilir. Bu rewrite sırasında:

- model → physical table mapping yapılır,
- model subquery’leri oluşturulur,
- column expression’ları inşa edilir,
- calculated field’lar açılır,
- relationship/join kuralları uygulanır.

Kaynak: https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

### 6.4 LogicalPlan / DataFusion yönü

Wren Engine blog’unda DataFusion LogicalPlan kullanımının dialect farklarını azaltmak için daha sağlam bir yaklaşım olduğu anlatılır. Örneğin type coercion gibi kurallar logical plan seviyesinde uygulanabilir. Bu, yalnızca string/AST rewrite yerine daha güvenilir bir planlama katmanı sağlar.

Kaynak: https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

---

## 7. Dialect transpilation nasıl sağlanıyor?

Wren Engine blog’unda iki önemli kütüphane adı geçer:

- **SQLGlot:** SQL dialect translation / transpilation için.
- **Ibis:** Farklı veritabanlarını sorgulamak için unified interface olarak.

Blog’daki ifade şu mimariyi ima eder:

```text
Generic / semantic SQL
    ↓
SQL parser / AST / LogicalPlan
    ↓
SQLGlot dialect transpilation
    ↓
Ibis / connector execution
    ↓
Target database
```

SQLGlot’un kendi dokümantasyonunda / GitHub açıklamasında farklı SQL dialect’leri arasında parse, transpile, optimize ve generate işlevleri sunduğu belirtilir. Bu nedenle Wren gibi sistemlerde LLM’in ürettiği generic SQL’in hedef veritabanı syntax’ına dönüştürülmesi için uygun bir bileşendir.

Kaynaklar:

- Wren Engine + DataFusion blog: https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion
- SQLGlot GitHub: https://github.com/tobymao/sqlglot

---

## 8. Veritabanı dialect farkları için örnek dönüşümler

Aşağıdaki örnekler Wren’in birebir çıktısı olarak değil, Wren’in kullandığı yaklaşıma uygun dialect uyarlama örnekleri olarak düşünülmelidir.

### 8.1 LIMIT / TOP / FETCH

Generic SQL:

```sql
SELECT customer_name, total_revenue
FROM CustomerRevenue
ORDER BY total_revenue DESC
LIMIT 10;
```

PostgreSQL / MySQL:

```sql
SELECT customer_name, total_revenue
FROM customer_revenue
ORDER BY total_revenue DESC
LIMIT 10;
```

SQL Server:

```sql
SELECT TOP 10 customer_name, total_revenue
FROM customer_revenue
ORDER BY total_revenue DESC;
```

Oracle / ANSI FETCH style:

```sql
SELECT customer_name, total_revenue
FROM customer_revenue
ORDER BY total_revenue DESC
FETCH FIRST 10 ROWS ONLY;
```

### 8.2 Date truncation

Generic semantic expression:

```sql
SELECT DATE_TRUNC('month', order_date) AS order_month, SUM(amount) AS revenue
FROM Orders
GROUP BY DATE_TRUNC('month', order_date);
```

PostgreSQL:

```sql
SELECT DATE_TRUNC('month', order_date) AS order_month, SUM(amount) AS revenue
FROM orders
GROUP BY DATE_TRUNC('month', order_date);
```

SQL Server:

```sql
SELECT DATEFROMPARTS(YEAR(order_date), MONTH(order_date), 1) AS order_month,
       SUM(amount) AS revenue
FROM orders
GROUP BY DATEFROMPARTS(YEAR(order_date), MONTH(order_date), 1);
```

MySQL:

```sql
SELECT DATE_FORMAT(order_date, '%Y-%m-01') AS order_month,
       SUM(amount) AS revenue
FROM orders
GROUP BY DATE_FORMAT(order_date, '%Y-%m-01');
```

BigQuery:

```sql
SELECT DATE_TRUNC(order_date, MONTH) AS order_month,
       SUM(amount) AS revenue
FROM orders
GROUP BY DATE_TRUNC(order_date, MONTH);
```

### 8.3 Identifier quoting

Semantic model:

```text
Order Details
```

PostgreSQL:

```sql
"Order Details"
```

MySQL:

```sql
`Order Details`
```

SQL Server:

```sql
[Order Details]
```

BigQuery:

```sql
`project.dataset.Order Details`
```

### 8.4 String concatenation

Generic intent:

```text
first_name + ' ' + last_name
```

PostgreSQL:

```sql
first_name || ' ' || last_name
```

MySQL:

```sql
CONCAT(first_name, ' ', last_name)
```

SQL Server:

```sql
CONCAT(first_name, ' ', last_name)
```

### 8.5 Boolean handling

PostgreSQL:

```sql
WHERE is_active = TRUE
```

MySQL:

```sql
WHERE is_active = 1
```

SQL Server:

```sql
WHERE is_active = 1
```

Bu tip farkların LLM prompt’una gömülmesi mümkün olsa da, Wren yaklaşımında daha doğru yer engine / dialect / connector katmanıdır.

---

## 9. Wren’in dialect stratejisindeki önemli tasarım kararları

### 9.1 LLM’e minimum dialect yükü vermek

LLM’e “SQL Server için TOP kullan, PostgreSQL için LIMIT kullan” gibi yüzlerce kural ezberletmek kırılgan olur. Wren’in unified SQL interface fikri bu yükü azaltır.

### 9.2 Semantic model üzerinden sorgu üretmek

LLM fiziksel tablo karmaşasını değil, semantic modeli kullanır. Bu hem accuracy hem güvenlik sağlar.

Örnek:

```text
Kötü yaklaşım:
LLM tüm database schema’yı görür ve doğrudan dbo.FactDailyReportSnapshot tablosuna SQL yazar.

İyi yaklaşım:
LLM sadece RevenueSnapshot modelini, exposed kolonları, allowed relationships ve business descriptions bilgilerini görür.
```

### 9.3 Dialect transpilation’ı deterministic yapmak

Dialect çevrimi modelin “tahminine” bırakılırsa hata oranı artar. SQLGlot / DataFusion / Ibis gibi araçlarla deterministic dönüşüm yapmak daha güvenlidir.

### 9.4 Dry-run ve validate zorunlu olmalı

Wren CLI tarafında `dry-plan` ve `dry-run` gibi komutlar vardır. Bu, generated SQL’i execute etmeden önce planlama/validation yapmak için önemlidir.

Kaynak: https://docs.getwren.ai/oss/get_started/quickstart

### 9.5 Connector capability farkları bilinmeli

Her veritabanı aynı şeyi desteklemez:

- MySQL < 8.0 CTE desteği sorunludur.
- BigQuery `UNNEST` kullanımında farklı kurallar uygular.
- DATE/TIMESTAMP coercion farklılıkları olabilir.
- Identifier quoting farklıdır.
- Pagination syntax farklıdır.
- Temporary view/table permission ihtiyacı farklıdır.

Wren Engine blog’unda MySQL legacy CTE desteği ve BigQuery UNNEST farkı örnek zorluklar olarak verilir.

Kaynak: https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

---

## 10. Biqly için uygulanabilir mimari

Senin Biqly gibi bir Text-to-SQL motorunda Wren yaklaşımını şu şekilde modelleyebilirsin.

### 10.1 DataSource abstraction

```go
type DataSourceType string

const (
    DataSourcePostgres  DataSourceType = "postgres"
    DataSourceMySQL     DataSourceType = "mysql"
    DataSourceSQLServer DataSourceType = "sqlserver"
    DataSourceBigQuery  DataSourceType = "bigquery"
    DataSourceSnowflake DataSourceType = "snowflake"
    DataSourceClickHouse DataSourceType = "clickhouse"
)

type DataSource struct {
    ID              string
    Name            string
    Type            DataSourceType
    ConnectionRef   string
    DefaultCatalog  string
    DefaultSchema   string
    Capabilities    DialectCapabilities
}
```

### 10.2 DialectCapabilities

```go
type DialectCapabilities struct {
    SupportsCTE              bool
    SupportsWindowFunctions  bool
    SupportsDateTrunc        bool
    SupportsILike            bool
    SupportsLimitOffset      bool
    SupportsTop              bool
    SupportsFetchFirst       bool
    IdentifierQuoteStyle     string // double_quote, backtick, bracket
    BooleanStyle             string // true_false, one_zero, bit
    TimeZoneSupport          string
    TemporaryObjectStrategy  string // temp_view, temp_table, none
}
```

### 10.3 Query generation contract

LLM’e doğrudan dialect-specific SQL yazdırmak yerine iki aşamalı tasarım daha sağlam olur:

```text
Natural language
  ↓
LogicalQuery JSON
  ↓
Semantic SQL / AST
  ↓
Dialect compiler
  ↓
Native SQL
```

Örnek LogicalQuery:

```json
{
  "model": "Orders",
  "select": [
    { "field": "order_month", "expression": "month(order_date)" },
    { "metric": "total_revenue" }
  ],
  "filters": [
    { "field": "order_date", "op": ">=", "value": "2025-01-01" }
  ],
  "group_by": ["order_month"],
  "order_by": [{ "field": "total_revenue", "direction": "desc" }],
  "limit": 10
}
```

Sonra compiler bunu hedef dialect’e çevirir.

---

## 11. Biqly için önerilen compiler katmanları

```text
Biqly AI Service
  ├── PromptBuilder
  ├── ContextRetriever
  ├── LogicalQueryGenerator
  └── SQLReviewLoop

Biqly Semantic Layer
  ├── ModelRegistry
  ├── RelationshipResolver
  ├── MetricRegistry
  ├── DimensionRegistry
  └── PolicyEngine

Biqly Query Engine
  ├── LogicalQueryValidator
  ├── SemanticPlanner
  ├── DialectCompiler
  │     ├── PostgresCompiler
  │     ├── MySQLCompiler
  │     ├── SQLServerCompiler
  │     ├── BigQueryCompiler
  │     └── ClickHouseCompiler
  ├── SQLDryRunner
  └── Executor
```

### Neden ayrı compiler?

Çünkü şu farkları tek prompt ile yönetmek zordur:

| Konu | PostgreSQL | MySQL | SQL Server | BigQuery |
|---|---|---|---|---|
| Limit | `LIMIT` | `LIMIT` | `TOP` / `OFFSET FETCH` | `LIMIT` |
| Identifier quote | `"x"` | `` `x` `` | `[x]` | `` `x` `` |
| Date month | `DATE_TRUNC` | `DATE_FORMAT` | `DATEFROMPARTS` | `DATE_TRUNC(date, MONTH)` |
| Boolean | `TRUE/FALSE` | `1/0` | `BIT` | `TRUE/FALSE` |
| Case-insensitive LIKE | `ILIKE` | collation/LOWER | collation/LOWER | `LOWER` |
| String concat | `||` / `CONCAT` | `CONCAT` | `CONCAT` / `+` | `CONCAT` |

---

## 12. Modelini kontrol ettirmek için AI Review Prompt

Aşağıdaki prompt’u başka bir yapay zekaya vererek kendi Text-to-SQL motorunu Wren yaklaşımına göre kontrol ettirebilirsin.

```text
You are a senior Text-to-SQL architect. Review my query generation engine against Wren.ai-style architecture.

Evaluate whether the system separates these responsibilities correctly:

1. Data source connection/profile
2. Schema introspection
3. Semantic model / MDL-like layer
4. Relationship and metric definitions
5. Context retrieval for the LLM
6. Natural language to LogicalQuery or semantic SQL
7. SQL validation and dry-run
8. Dialect-specific SQL compilation/transpilation
9. Connector execution
10. Error feedback and memory/learning loop

Focus especially on multi-database support for PostgreSQL, MySQL, SQL Server, BigQuery, Snowflake, ClickHouse, and Trino.

Check for these risks:

- The LLM is asked to memorize dialect-specific SQL syntax.
- Raw physical schema is exposed directly without semantic filtering.
- The engine does not know connector capabilities.
- SQL Server TOP/OFFSET, PostgreSQL LIMIT, MySQL LIMIT, and BigQuery LIMIT are handled in prompt text instead of deterministic compiler logic.
- Date/time functions are not abstracted.
- Identifier quoting is not dialect-aware.
- Boolean handling is not dialect-aware.
- Temporary table/view permissions are not considered.
- Dry-run/validation does not happen before execution.
- Execution errors are not fed back to the LLM in a controlled retry loop.

Give me:

1. Architecture gaps
2. High-risk bugs
3. Recommended abstractions
4. Minimum implementation plan
5. Test matrix by dialect
6. Examples of compiler rules I should add
```

---

## 13. Test matrix önerisi

Biqly tarafında multi-dialect SQL üretimini kontrol etmek için en az şu testleri yazmalısın:

### 13.1 Basit select

```text
Question: Son 10 müşteriyi getir.
Expected:
- PostgreSQL/MySQL/BigQuery: LIMIT 10
- SQL Server: TOP 10 veya OFFSET/FETCH
```

### 13.2 Aggregation

```text
Question: Aylık ciroyu göster.
Expected:
- Date truncation dialect-specific olmalı.
- GROUP BY expression doğru olmalı.
```

### 13.3 Join

```text
Question: Müşteri bazında toplam sipariş tutarı.
Expected:
- Join relationship semantic layer’dan gelmeli.
- LLM join condition uydurmamalı.
```

### 13.4 Identifier quoting

```text
Question: Order Details tablosundan veri getir.
Expected:
- PostgreSQL: "Order Details"
- MySQL/BigQuery: `Order Details`
- SQL Server: [Order Details]
```

### 13.5 Boolean

```text
Question: Aktif müşterileri getir.
Expected:
- PostgreSQL/BigQuery: TRUE
- SQL Server/MySQL: 1 veya dialect config’e göre uygun karşılık
```

### 13.6 String search

```text
Question: Adında 'ali' geçen müşteriler.
Expected:
- PostgreSQL: ILIKE mümkünse kullanılabilir.
- MySQL/SQL Server: collation veya LOWER(column) LIKE LOWER(?) stratejisi.
```

### 13.7 Permission / temp object

```text
Question: Karmaşık model/view gerektiren analiz.
Expected:
- PostgreSQL temporary view/table gerekiyorsa permission kontrolü yapılmalı.
- Read-only datasource ise fallback plan olmalı.
```

---

## 14. Önerilen minimum implementation plan

### Faz 1 — Dialect config

- `datasource_type` alanını zorunlu yap.
- Her datasource için `DialectCapabilities` oluştur.
- Identifier quoting, limit, boolean, date truncation kurallarını compiler’a taşı.

### Faz 2 — LogicalQuery standardizasyonu

- LLM çıktısını SQL yerine önce JSON contract’a çevir.
- JSON schema validation uygula.
- Unknown model/field/metric durumunda fail-fast yap.

### Faz 3 — Semantic planner

- Model → physical table mapping yap.
- Relationship resolver ekle.
- Metric ve calculated field expansion ekle.
- Policy/default filter uygulamasını burada yap.

### Faz 4 — Dialect compiler

- PostgreSQL compiler
- MySQL compiler
- SQL Server compiler
- BigQuery compiler
- Ortak AST / SQL builder kullan.

### Faz 5 — Dry-run ve retry loop

- SQL üretildikten sonra `EXPLAIN`, `PREPARE`, `LIMIT 0` veya dialect’e uygun dry-run çalıştır.
- Hata mesajını sanitize edip LLM’e geri ver.
- En fazla 2 retry uygula.

### Faz 6 — Memory / examples

- Başarılı NL → LogicalQuery → SQL örneklerini sakla.
- Datasource type ve dialect bilgisini memory kaydına ekle.
- Retrieval sırasında aynı dialect örneklerine daha yüksek ağırlık ver.

---

## 15. Sonuç

Wren.ai’nin veritabanı tiplerine göre SQL üretim stratejisi üç ana prensibe dayanır:

1. **Semantic-first:** LLM ham schema değil, MDL/context görür.
2. **Unified SQL interface:** Agent mümkün olduğunca generic/semantic SQL yazar.
3. **Dialect-aware engine:** SQL rewrite, logical plan, transpilation ve connector execution engine tarafında yapılır.

Biqly tarafında aynı yaklaşımın karşılığı şudur:

```text
Natural Language
  → LogicalQuery
  → Semantic Planner
  → Dialect Compiler
  → Dry Run
  → Execute
  → Feedback / Memory
```

Bu yapı kurulursa PostgreSQL, MySQL, SQL Server, BigQuery, Snowflake, ClickHouse gibi farklı veritabanlarını desteklemek için LLM prompt’unu şişirmek yerine deterministic compiler ve connector katmanını genişletmen yeterli olur.

---

## 16. Kaynakça

1. Wren AI OSS Introduction  
   https://docs.getwren.ai/oss/introduction

2. Wren Engine GitHub README  
   https://github.com/Canner/wren-engine

3. Powering Semantic SQL for AI Agents with Apache DataFusion  
   https://www.getwren.ai/post/powering-semantic-sql-for-ai-agents-with-apache-datafusion

4. Wren AI Quick Start: Wren CLI with jaffle_shop  
   https://docs.getwren.ai/oss/get_started/quickstart

5. Wren AI PostgreSQL connector guide  
   https://docs.getwren.ai/oss/guide/connect/postgresql

6. Wren AI MySQL connector guide  
   https://docs.getwren.ai/oss/guide/connect/mysql

7. Wren AI SQL Server connector guide  
   https://docs.getwren.ai/cp/guide/connect/sqlserver

8. SQLGlot GitHub  
   https://github.com/tobymao/sqlglot
