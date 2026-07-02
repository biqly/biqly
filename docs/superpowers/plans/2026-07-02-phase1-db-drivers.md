# Phase 1 Datasource Drivers (SQLite, Snowflake, Databricks, Oracle) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SQLite, Snowflake, Databricks, and Oracle datasource support end-to-end: Go drivers + dialects + DSN composition + handler validation + adaptive frontend connection form.

**Architecture:** Each database gets a package under `internal/datasource/<driver>/` (embeds `BaseDriver`, implements only `Introspect`) and a dialect in `internal/dialect/<driver>.go` (embeds `BaseDialect`). DSN composition stays centralized in `internal/datasource/dsn.go`. The frontend gains a per-driver form spec in `frontend/src/dbDrivers.ts` that tells the structured form which fields to render.

**Tech Stack:** Go 1.26, `modernc.org/sqlite` (pure Go), `github.com/snowflakedb/gosnowflake`, `github.com/databricks/databricks-sql-go`, `github.com/sijms/go-ora/v2`, React 19 + TypeScript.

**Spec:** `docs/superpowers/specs/2026-07-02-phase1-db-drivers-design.md`

**Field mapping decision (locked):** New drivers reuse the existing structured columns instead of inventing new ones:

| Driver | host | port | username | password | database_name | ssl_mode | connection_params (Extra) |
|---|---|---|---|---|---|---|---|
| sqlite | — | — | — | — | **file path** (required) | — | optional URI params |
| snowflake | **account identifier** (required) | — | required | required | database (required) | — | `warehouse`, `role`, `schema` |
| databricks | server hostname (required) | 443 default | — | **access token** (required) | catalog (optional) | — | `http_path` (required), `schema` |
| oracle | required | 1521 default | required | required | **service name** (required) | `true`/`require` → `ssl=true` | extra query params |

**Deviations from spec (justified, reflected below):**
- Oracle `ExplainSQL` returns `""` (skip dry-run) instead of `EXPLAIN PLAN FOR`: that statement writes to `PLAN_TABLE`, which fails on read-only credentials — same treatment as SQL Server.
- Snowflake relation/PK introspection returns empty: Snowflake's `INFORMATION_SCHEMA` has no `KEY_COLUMN_USAGE` view and `SHOW IMPORTED KEYS` output is version-brittle. Databricks attempts Unity Catalog `information_schema` FKs with graceful degrade (hive_metastore has no `information_schema`).

**Verification gates (every commit):** pre-commit hook runs `make precommit`. During tasks use targeted commands listed per step; the final task runs the full gate.

---

### Task 1: Add Go module dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Fetch the four driver modules**

```bash
cd /Users/baris.dogu/src/biqly/biqly
go get modernc.org/sqlite@latest \
       github.com/snowflakedb/gosnowflake@latest \
       github.com/databricks/databricks-sql-go@latest \
       github.com/sijms/go-ora/v2@latest
go mod tidy
```

Expected: `go.mod` gains the four requires (modernc.org/sqlite pulls `modernc.org/libc` etc. — that is normal).

- [ ] **Step 2: Verify the build still compiles**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add sqlite, snowflake, databricks, oracle driver modules"
```

Note: until a Go file imports these modules, `go mod tidy` in later tasks may drop them; if Step 1's tidy removes them, skip this commit and fold the go.mod change into Task 10's commit instead.

---

### Task 2: DSN composition for the four new drivers

**Files:**
- Modify: `internal/datasource/dsn.go`
- Test: `internal/datasource/dsn_test.go`

`ComposeDSN` currently funnels everything through `prepareDSNParts`, which hard-requires host+port. New drivers that don't fit (sqlite, snowflake, databricks) get their own compose functions dispatched *before* `prepareDSNParts`; oracle fits the host/port model and goes through it.

- [ ] **Step 1: Write failing tests** — append to `internal/datasource/dsn_test.go`:

```go
func TestNormalizeDriverType_newDrivers(t *testing.T) {
	cases := map[string]string{
		"sqlite3": "sqlite", "SQLite": "sqlite",
		"Snowflake": "snowflake",
		"spark":     "databricks", "dbx": "databricks", "Databricks": "databricks",
		"ora": "oracle", "Oracle": "oracle",
	}
	for in, want := range cases {
		if got := datasource.NormalizeDriverType(in); got != want {
			t.Errorf("NormalizeDriverType(%q) = %q, want %q", in, got, want)
		}
	}
	if datasource.DefaultPort("oracle") != 1521 {
		t.Error("oracle default port")
	}
	if datasource.DefaultPort("databricks") != 443 {
		t.Error("databricks default port")
	}
	if datasource.DefaultPort("sqlite") != 0 || datasource.DefaultPort("snowflake") != 0 {
		t.Error("sqlite/snowflake must have no default port")
	}
}

func TestComposeDSN_sqlite(t *testing.T) {
	got, err := datasource.ComposeDSN("sqlite", datasource.ConnectionFields{DatabaseName: "/data/app.db"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "file:/data/app.db?mode=ro" {
		t.Fatalf("got %q", got)
	}
	if _, err := datasource.ComposeDSN("sqlite", datasource.ConnectionFields{}); err == nil {
		t.Fatal("want error for missing file path")
	}
}

func TestComposeDSN_snowflake(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "myorg-acct1", Username: "u", Password: "p@ss",
		DatabaseName: "ANALYTICS",
		Extra:        map[string]string{"warehouse": "WH1", "role": "READER", "schema": "PUBLIC"},
	}
	got, err := datasource.ComposeDSN("snowflake", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "u:p%40ss@myorg-acct1/ANALYTICS/PUBLIC?") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "warehouse=WH1") || !strings.Contains(got, "role=READER") {
		t.Fatalf("got %q", got)
	}
	if _, err := datasource.ComposeDSN("snowflake", datasource.ConnectionFields{Username: "u"}); err == nil {
		t.Fatal("want error for missing account")
	}
	if _, err := datasource.ComposeDSN("snowflake", datasource.ConnectionFields{Host: "a", Username: "u"}); err == nil {
		t.Fatal("want error for missing database")
	}
}

func TestComposeDSN_databricks(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "adb-123.4.azuredatabricks.net", Password: "dapiTOKEN",
		DatabaseName: "main",
		Extra:        map[string]string{"http_path": "/sql/1.0/warehouses/abc", "schema": "default"},
	}
	got, err := datasource.ComposeDSN("databricks", f)
	if err != nil {
		t.Fatal(err)
	}
	want := "token:dapiTOKEN@adb-123.4.azuredatabricks.net:443/sql/1.0/warehouses/abc?catalog=main&schema=default"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := datasource.ComposeDSN("databricks", datasource.ConnectionFields{Host: "h", Password: "t"}); err == nil {
		t.Fatal("want error for missing http_path")
	}
	if _, err := datasource.ComposeDSN("databricks", datasource.ConnectionFields{Host: "h", Extra: map[string]string{"http_path": "/p"}}); err == nil {
		t.Fatal("want error for missing token")
	}
}

func TestComposeDSN_oracle(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "dbhost", Port: 1521, Username: "scott", Password: "tiger",
		DatabaseName: "ORCLPDB1", SSLMode: "require",
	}
	got, err := datasource.ComposeDSN("oracle", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "oracle://scott:tiger@dbhost:1521/ORCLPDB1") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "ssl=true") {
		t.Fatalf("got %q", got)
	}
	if _, err := datasource.ComposeDSN("oracle", datasource.ConnectionFields{Host: "h", Port: 1521}); err == nil {
		t.Fatal("want error for missing service name")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/datasource/ -run 'TestComposeDSN|TestNormalizeDriverType' -v`
Expected: FAIL (`unsupported datasource type`/`unsupported driver` errors).

- [ ] **Step 3: Implement in `internal/datasource/dsn.go`**

Extend `NormalizeDriverType`:

```go
	case "sqlite", "sqlite3":
		return "sqlite"
	case "snowflake":
		return "snowflake"
	case "databricks", "spark", "dbx":
		return "databricks"
	case "oracle", "ora":
		return "oracle"
```

Extend `DefaultPort` (before `default`):

```go
	case "oracle":
		return 1521
	case "databricks":
		return 443
```

Extend `DriverConnectionDefaults` switch:

```go
	case "oracle":
		ssl = "disable"
	}
```

(sqlite, snowflake, databricks get no ssl default and keep port 0/443 from `DefaultPort`.)

Add compose functions:

```go
func composeSQLiteDSN(f ConnectionFields) (string, error) {
	path := strings.TrimSpace(f.DatabaseName)
	if path == "" {
		return "", errors.New("database file path is required for sqlite")
	}
	q := url.Values{}
	q.Set("mode", "ro")
	for k, v := range f.Extra {
		if k = strings.TrimSpace(k); k != "" && strings.ToLower(k) != "mode" {
			q.Set(k, v)
		}
	}
	return "file:" + path + "?" + q.Encode(), nil
}

func composeSnowflakeDSN(f ConnectionFields) (string, error) {
	account := strings.TrimSpace(f.Host)
	if account == "" {
		return "", errors.New("account identifier is required for snowflake")
	}
	user := strings.TrimSpace(f.Username)
	if user == "" {
		return "", errors.New("username is required for snowflake")
	}
	db := strings.TrimSpace(f.DatabaseName)
	if db == "" {
		return "", errors.New("database name is required for snowflake")
	}
	extra := make(map[string]string, len(f.Extra))
	for k, v := range f.Extra {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			extra[k] = strings.TrimSpace(v)
		}
	}
	path := db
	if schema := extra["schema"]; schema != "" {
		path += "/" + schema
	}
	delete(extra, "schema")
	q := url.Values{}
	mergeExtraQuery(q, extra)
	dsn := fmt.Sprintf("%s:%s@%s/%s", url.QueryEscape(user), url.QueryEscape(f.Password), account, path)
	if enc := q.Encode(); enc != "" {
		dsn += "?" + enc
	}
	return dsn, nil
}

func composeDatabricksDSN(f ConnectionFields) (string, error) {
	host := strings.TrimSpace(f.Host)
	if host == "" {
		return "", errors.New("host is required for databricks")
	}
	if f.Password == "" {
		return "", errors.New("access token is required for databricks")
	}
	extra := make(map[string]string, len(f.Extra))
	for k, v := range f.Extra {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			extra[k] = strings.TrimSpace(v)
		}
	}
	httpPath := extra["http_path"]
	if httpPath == "" {
		return "", errors.New("http_path is required for databricks")
	}
	delete(extra, "http_path")
	if !strings.HasPrefix(httpPath, "/") {
		httpPath = "/" + httpPath
	}
	port := f.Port
	if port <= 0 {
		port = 443
	}
	q := url.Values{}
	if catalog := strings.TrimSpace(f.DatabaseName); catalog != "" {
		q.Set("catalog", catalog)
	}
	mergeExtraQuery(q, extra)
	dsn := fmt.Sprintf("token:%s@%s:%d%s", url.QueryEscape(f.Password), host, port, httpPath)
	if enc := q.Encode(); enc != "" {
		dsn += "?" + enc
	}
	return dsn, nil
}

func composeOracleDSN(p dsnParts) (string, error) {
	if p.db == "" {
		return "", errors.New("service name is required for oracle")
	}
	u := url.URL{
		Scheme: "oracle",
		User:   url.UserPassword(p.user, p.pass),
		Host:   fmt.Sprintf("%s:%d", p.host, p.port),
		Path:   "/" + p.db,
	}
	q := url.Values{}
	switch strings.ToLower(p.ssl) {
	case "true", "require", "required":
		q.Set("ssl", "true")
	}
	mergeExtraQuery(q, p.extra)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
```

Rewrite `ComposeDSN` to dispatch hostless drivers first:

```go
// ComposeDSN builds a single connection string for Open/Ping.
func ComposeDSN(driver string, f ConnectionFields) (string, error) {
	switch NormalizeDriverType(driver) {
	case "sqlite":
		return composeSQLiteDSN(f)
	case "snowflake":
		return composeSnowflakeDSN(f)
	case "databricks":
		return composeDatabricksDSN(f)
	}
	p, err := prepareDSNParts(driver, f)
	if err != nil {
		return "", err
	}
	switch p.driver {
	case "postgres":
		return composePostgresDSN(p)
	case "mysql":
		return composeMySQLDSN(p)
	case "sqlserver":
		return composeSQLServerDSN(p)
	case "clickhouse":
		return composeClickHouseDSN(p)
	case "oracle":
		return composeOracleDSN(p)
	default:
		return "", fmt.Errorf("compose DSN: unsupported driver %q", driver)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/datasource/ -run 'TestComposeDSN|TestNormalizeDriverType' -v`
Expected: PASS (all, including existing postgres/mysql/sqlserver cases).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/datasource/dsn.go internal/datasource/dsn_test.go
git add internal/datasource/dsn.go internal/datasource/dsn_test.go
git commit -m "feat(datasource): DSN composition for sqlite, snowflake, databricks, oracle"
```

---

### Task 3: Relax host requirement in the datasources handler

**Files:**
- Modify: `internal/http/handlers/datasources.go` (`resolveCreateDatasourceMode` ~line 60, `buildStructuredModeDraft` ~line 224)
- Test: create `internal/http/handlers/datasources_test.go` (does not exist yet; all handler tests use white-box `package handlers`)

`buildStructuredModeDraft` 400s when `connection.host` is empty and always sets a port — both wrong for sqlite (path only) and snowflake handled via host=account (host set, fine). Only sqlite lacks host. Delegate required-field validation to `ComposeDSN`.

- [ ] **Step 1: Write failing test** — create `internal/http/handlers/datasources_test.go`:

```go
package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

func TestResolveCreateDatasourceMode_structuredWithoutHost(t *testing.T) {
	req := createDatasourceRequest{
		Name: "local file", Type: "sqlite",
		Connection: &connectionRequest{DatabaseName: "/data/app.db"},
	}
	if got := resolveCreateDatasourceMode(&req); got != metadata.DSNModeStructured {
		t.Fatalf("mode = %q, want structured", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/http/handlers/ -run TestResolveCreateDatasourceMode_structuredWithoutHost -v`
Expected: FAIL (`mode = ""`), because `hasConn` only checks `Host`.

- [ ] **Step 3: Implement**

In `resolveCreateDatasourceMode`, replace:

```go
	hasConn := req.Connection != nil && strings.TrimSpace(req.Connection.Host) != ""
```

with:

```go
	hasConn := req.Connection != nil && connectionHasPayload(req.Connection)
```

In `buildStructuredModeDraft`, replace the host-required block:

```go
	host := strings.TrimSpace(c.Host)
	if host == "" {
		return nil, "", http.StatusBadRequest, "connection.host is required", nil
	}
	ds.Host = new(host)

	port := datasource.DefaultPort(driverType)
	if c.Port != nil && *c.Port > 0 {
		port = *c.Port
	}
	ds.Port = new(port)
```

with:

```go
	host := strings.TrimSpace(c.Host)
	if host != "" {
		ds.Host = new(host)
	}

	port := datasource.DefaultPort(driverType)
	if c.Port != nil && *c.Port > 0 {
		port = *c.Port
	}
	if port > 0 {
		ds.Port = new(port)
	}
```

Driver-specific required-field errors (e.g. "host is required", "database file path is required for sqlite") now surface from the existing `ComposeDSN` call a few lines below, which already maps errors to 400.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/http/handlers/ -v -run 'Datasource'`
Expected: PASS, including pre-existing structured-mode tests (postgres still errors on missing host — now via ComposeDSN with the same 400 status; if an existing test asserts the exact message "connection.host is required", update it to the compose error "host is required").

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/http/handlers/
git add internal/http/handlers/
git commit -m "feat(api): allow hostless structured datasources (sqlite file path)"
```

---

### Task 4: SQLite dialect

**Files:**
- Create: `internal/dialect/sqlite.go`
- Test: `internal/dialect/sqlite_test.go`

- [ ] **Step 1: Write failing test** — create `internal/dialect/sqlite_test.go`:

```go
package dialect

import "testing"

func TestSQLiteDialect(t *testing.T) {
	d := SQLite
	if d.Name() != "sqlite" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(3); got != "?" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("main.orders"); got != `"main"."orders"` {
		t.Errorf("QuoteIdent = %q", got)
	}
	if got := d.LimitOffset(10, 5); got != "LIMIT 10 OFFSET 5" {
		t.Errorf("LimitOffset = %q", got)
	}
	truncs := map[string]string{
		"day":     `date("created_at")`,
		"week":    `date("created_at", 'weekday 0', '-6 days')`,
		"month":   `date("created_at", 'start of month')`,
		"quarter": `date("created_at", 'start of month', '-' || ((CAST(strftime('%m', "created_at") AS INTEGER) - 1) % 3) || ' months')`,
		"year":    `date("created_at", 'start of year')`,
	}
	for part, want := range truncs {
		if got := d.DateTrunc(part, "created_at"); got != want {
			t.Errorf("DateTrunc(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.DateTruncPlaceholder("month", "?"); got != "date(?, 'start of month')" {
		t.Errorf("DateTruncPlaceholder = %q", got)
	}
	parts := map[string]string{
		"year":    `CAST(strftime('%Y', "d") AS INTEGER)`,
		"quarter": `(CAST(strftime('%m', "d") AS INTEGER) + 2) / 3`,
		"month":   `CAST(strftime('%m', "d") AS INTEGER)`,
	}
	for part, want := range parts {
		if got := d.CalendarPart(part, "d"); got != want {
			t.Errorf("CalendarPart(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.ILike(`"name"`, "?"); got != `"name" LIKE ?` {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "EXPLAIN QUERY PLAN SELECT 1" {
		t.Errorf("ExplainSQL = %q", got)
	}
	if got := d.Aggregate("count", "*"); got != "COUNT(*)" {
		t.Errorf("Aggregate = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dialect/ -run TestSQLiteDialect -v`
Expected: FAIL to compile (`undefined: SQLite`).

- [ ] **Step 3: Implement** — create `internal/dialect/sqlite.go`:

```go
// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// SQLiteDialect implements the Dialect interface for SQLite.
type SQLiteDialect struct {
	BaseDialect
}

// SQLite is the default global instance of SQLiteDialect.
var SQLite = SQLiteDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "\"",
		QuoteRight: "\"",
	},
}

// Name returns the dialect name.
func (SQLiteDialect) Name() string {
	return "sqlite"
}

// Placeholder returns the parameter placeholder for the given index.
func (SQLiteDialect) Placeholder(_ int) string {
	return "?"
}

// sqliteDateTrunc renders SQLite date() modifiers that truncate expr to part.
func sqliteDateTrunc(part, expr string) string {
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "day":
		return fmt.Sprintf("date(%s)", expr)
	case "week":
		// 'weekday 0' advances to the next Sunday (or stays), -6 days lands on Monday.
		return fmt.Sprintf("date(%s, 'weekday 0', '-6 days')", expr)
	case "month":
		return fmt.Sprintf("date(%s, 'start of month')", expr)
	case "quarter":
		return fmt.Sprintf("date(%s, 'start of month', '-' || ((CAST(strftime('%%m', %s) AS INTEGER) - 1) %% 3) || ' months')", expr, expr)
	case "year":
		return fmt.Sprintf("date(%s, 'start of year')", expr)
	default:
		return fmt.Sprintf("datetime(%s)", expr)
	}
}

// DateTrunc returns the date truncation expression.
func (d SQLiteDialect) DateTrunc(part, column string) string {
	return sqliteDateTrunc(part, d.QuoteIdent(column))
}

// DateTruncPlaceholder truncates a bind-parameter timestamp.
func (SQLiteDialect) DateTruncPlaceholder(part, placeholder string) string {
	return sqliteDateTrunc(part, placeholder)
}

// CalendarPart returns strftime-based integer buckets for year/quarter/month.
func (d SQLiteDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"CAST(strftime('%%Y', %s) AS INTEGER)",
		"(CAST(strftime('%%m', %s) AS INTEGER) + 2) / 3",
		"CAST(strftime('%%m', %s) AS INTEGER)",
	)
}

// ILike returns a case-insensitive LIKE expression. SQLite LIKE is
// case-insensitive for ASCII by default.
func (SQLiteDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s LIKE %s", column, placeholder)
}

// ExplainSQL uses SQLite's EXPLAIN QUERY PLAN form.
func (SQLiteDialect) ExplainSQL(sql string) string {
	return "EXPLAIN QUERY PLAN " + sql
}

var _ Dialect = SQLiteDialect{}
```

Note the `%%` escapes: `CalendarPartLookup` runs the format strings through `fmt.Sprintf`, so literal `%m`/`%Y` must be written `%%m`/`%%Y`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/dialect/ -run TestSQLiteDialect -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/dialect/sqlite.go internal/dialect/sqlite_test.go
git add internal/dialect/sqlite.go internal/dialect/sqlite_test.go
git commit -m "feat(dialect): sqlite dialect"
```

---

### Task 5: Snowflake dialect

**Files:**
- Create: `internal/dialect/snowflake.go`
- Test: `internal/dialect/snowflake_test.go`

- [ ] **Step 1: Write failing test** — create `internal/dialect/snowflake_test.go`:

```go
package dialect

import "testing"

func TestSnowflakeDialect(t *testing.T) {
	d := Snowflake
	if d.Name() != "snowflake" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(2); got != "?" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("sales.orders"); got != `"sales"."orders"` {
		t.Errorf("QuoteIdent = %q", got)
	}
	if got := d.LimitOffset(10, 5); got != "LIMIT 10 OFFSET 5" {
		t.Errorf("LimitOffset = %q", got)
	}
	if got := d.DateTrunc("month", "created_at"); got != `DATE_TRUNC('month', "created_at")` {
		t.Errorf("DateTrunc = %q", got)
	}
	if got := d.CalendarPart("quarter", "d"); got != `CAST(EXTRACT(QUARTER FROM "d") AS INTEGER)` {
		t.Errorf("CalendarPart = %q", got)
	}
	if got := d.ILike(`"name"`, "?"); got != `"name" ILIKE ?` {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "EXPLAIN SELECT 1" {
		t.Errorf("ExplainSQL = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dialect/ -run TestSnowflakeDialect -v`
Expected: compile FAIL (`undefined: Snowflake`).

- [ ] **Step 3: Implement** — create `internal/dialect/snowflake.go`:

```go
// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import "fmt"

// SnowflakeDialect implements the Dialect interface for Snowflake.
type SnowflakeDialect struct {
	BaseDialect
}

// Snowflake is the default global instance of SnowflakeDialect.
var Snowflake = SnowflakeDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "\"",
		QuoteRight: "\"",
	},
}

// Name returns the dialect name.
func (SnowflakeDialect) Name() string {
	return "snowflake"
}

// Placeholder returns the parameter placeholder for the given index.
func (SnowflakeDialect) Placeholder(_ int) string {
	return "?"
}

// DateTrunc returns the date truncation expression.
func (d SnowflakeDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("DATE_TRUNC('%s', %s)", part, d.QuoteIdent(column))
}

// CalendarPart returns CAST(EXTRACT(...)) AS INTEGER buckets.
func (d SnowflakeDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"CAST(EXTRACT(YEAR FROM %s) AS INTEGER)",
		"CAST(EXTRACT(QUARTER FROM %s) AS INTEGER)",
		"CAST(EXTRACT(MONTH FROM %s) AS INTEGER)",
	)
}

// ILike returns Snowflake's native case-insensitive LIKE.
func (SnowflakeDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", column, placeholder)
}

var _ Dialect = SnowflakeDialect{}
```

(`LimitOffset`, `DateTruncPlaceholder` — `DATE_TRUNC('p', CAST(? AS TIMESTAMP))` is valid Snowflake — `ExplainSQL`, `Aggregate`, `WindowFunc` all come from `BaseDialect`.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/dialect/ -run TestSnowflakeDialect -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/dialect/snowflake.go internal/dialect/snowflake_test.go
git add internal/dialect/snowflake.go internal/dialect/snowflake_test.go
git commit -m "feat(dialect): snowflake dialect"
```

---

### Task 6: Databricks dialect

**Files:**
- Create: `internal/dialect/databricks.go`
- Test: `internal/dialect/databricks_test.go`

- [ ] **Step 1: Write failing test** — create `internal/dialect/databricks_test.go`:

```go
package dialect

import "testing"

func TestDatabricksDialect(t *testing.T) {
	d := Databricks
	if d.Name() != "databricks" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(2); got != "?" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("sales.orders"); got != "`sales`.`orders`" {
		t.Errorf("QuoteIdent = %q", got)
	}
	if got := d.LimitOffset(10, 5); got != "LIMIT 10 OFFSET 5" {
		t.Errorf("LimitOffset = %q", got)
	}
	if got := d.DateTrunc("month", "created_at"); got != "date_trunc('MONTH', `created_at`)" {
		t.Errorf("DateTrunc = %q", got)
	}
	if got := d.DateTruncPlaceholder("month", "?"); got != "date_trunc('MONTH', CAST(? AS TIMESTAMP))" {
		t.Errorf("DateTruncPlaceholder = %q", got)
	}
	if got := d.CalendarPart("year", "d"); got != "year(`d`)" {
		t.Errorf("CalendarPart = %q", got)
	}
	if got := d.ILike("`name`", "?"); got != "`name` ILIKE ?" {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "EXPLAIN SELECT 1" {
		t.Errorf("ExplainSQL = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dialect/ -run TestDatabricksDialect -v`
Expected: compile FAIL (`undefined: Databricks`).

- [ ] **Step 3: Implement** — create `internal/dialect/databricks.go`:

```go
// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strings"
)

// DatabricksDialect implements the Dialect interface for Databricks (Spark SQL).
type DatabricksDialect struct {
	BaseDialect
}

// Databricks is the default global instance of DatabricksDialect.
var Databricks = DatabricksDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "`",
		QuoteRight: "`",
	},
}

// Name returns the dialect name.
func (DatabricksDialect) Name() string {
	return "databricks"
}

// Placeholder returns the parameter placeholder for the given index.
func (DatabricksDialect) Placeholder(_ int) string {
	return "?"
}

// DateTrunc returns Spark's date_trunc with an upper-case format unit.
func (d DatabricksDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("date_trunc('%s', %s)", strings.ToUpper(part), d.QuoteIdent(column))
}

// DateTruncPlaceholder truncates a bind-parameter timestamp.
func (DatabricksDialect) DateTruncPlaceholder(part, placeholder string) string {
	return fmt.Sprintf("date_trunc('%s', CAST(%s AS TIMESTAMP))", strings.ToUpper(part), placeholder)
}

// CalendarPart returns Spark's year/quarter/month functions.
func (d DatabricksDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column, "year(%s)", "quarter(%s)", "month(%s)")
}

// ILike returns Spark SQL's native case-insensitive LIKE.
func (DatabricksDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("%s ILIKE %s", column, placeholder)
}

var _ Dialect = DatabricksDialect{}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/dialect/ -run TestDatabricksDialect -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/dialect/databricks.go internal/dialect/databricks_test.go
git add internal/dialect/databricks.go internal/dialect/databricks_test.go
git commit -m "feat(dialect): databricks dialect"
```

---

### Task 7: Oracle dialect

**Files:**
- Create: `internal/dialect/oracle.go`
- Test: `internal/dialect/oracle_test.go`

- [ ] **Step 1: Write failing test** — create `internal/dialect/oracle_test.go`:

```go
package dialect

import "testing"

func TestOracleDialect(t *testing.T) {
	d := Oracle
	if d.Name() != "oracle" {
		t.Errorf("Name() = %q", d.Name())
	}
	if got := d.Placeholder(3); got != ":3" {
		t.Errorf("Placeholder = %q", got)
	}
	if got := d.QuoteIdent("sales.orders"); got != `"sales"."orders"` {
		t.Errorf("QuoteIdent = %q", got)
	}
	limits := []struct {
		limit, offset int
		want          string
	}{
		{10, 5, "OFFSET 5 ROWS FETCH NEXT 10 ROWS ONLY"},
		{10, 0, "FETCH FIRST 10 ROWS ONLY"},
		{0, 5, "OFFSET 5 ROWS"},
		{0, 0, ""},
	}
	for _, tc := range limits {
		if got := d.LimitOffset(tc.limit, tc.offset); got != tc.want {
			t.Errorf("LimitOffset(%d,%d) = %q, want %q", tc.limit, tc.offset, got, tc.want)
		}
	}
	truncs := map[string]string{
		"day":     `TRUNC("d", 'DD')`,
		"week":    `TRUNC("d", 'IW')`,
		"month":   `TRUNC("d", 'MM')`,
		"quarter": `TRUNC("d", 'Q')`,
		"year":    `TRUNC("d", 'YYYY')`,
	}
	for part, want := range truncs {
		if got := d.DateTrunc(part, "d"); got != want {
			t.Errorf("DateTrunc(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.DateTruncPlaceholder("month", ":1"); got != "TRUNC(CAST(:1 AS TIMESTAMP), 'MM')" {
		t.Errorf("DateTruncPlaceholder = %q", got)
	}
	parts := map[string]string{
		"year":    `EXTRACT(YEAR FROM "d")`,
		"quarter": `TO_NUMBER(TO_CHAR("d", 'Q'))`,
		"month":   `EXTRACT(MONTH FROM "d")`,
	}
	for part, want := range parts {
		if got := d.CalendarPart(part, "d"); got != want {
			t.Errorf("CalendarPart(%s) = %q, want %q", part, got, want)
		}
	}
	if got := d.ILike(`"name"`, ":1"); got != `UPPER("name") LIKE UPPER(:1)` {
		t.Errorf("ILike = %q", got)
	}
	if got := d.ExplainSQL("SELECT 1"); got != "" {
		t.Errorf("ExplainSQL = %q, want empty (skip dry-run)", got)
	}
	if got := d.SelectWithLimit([]string{`"a"`}, `"t"`, 5); got != `SELECT "a" FROM "t" FETCH FIRST 5 ROWS ONLY` {
		t.Errorf("SelectWithLimit = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dialect/ -run TestOracleDialect -v`
Expected: compile FAIL (`undefined: Oracle`).

- [ ] **Step 3: Implement** — create `internal/dialect/oracle.go`:

```go
// Package dialect defines SQL dialect interfaces for different database engines.
package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

// OracleDialect implements the Dialect interface for Oracle Database (12c+).
type OracleDialect struct {
	BaseDialect
}

// Oracle is the default global instance of OracleDialect.
var Oracle = OracleDialect{
	BaseDialect: BaseDialect{
		QuoteLeft:  "\"",
		QuoteRight: "\"",
		// EXPLAIN PLAN FOR writes to PLAN_TABLE, which fails on read-only
		// credentials — skip dry-run like SQL Server does.
		ExplainDisabled: true,
	},
}

// Name returns the dialect name.
func (OracleDialect) Name() string {
	return "oracle"
}

// Placeholder returns the numbered bind placeholder for the given index.
func (OracleDialect) Placeholder(index int) string {
	return ":" + strconv.Itoa(index)
}

// LimitOffset generates Oracle 12c+ OFFSET/FETCH clauses.
func (OracleDialect) LimitOffset(limit, offset int) string {
	switch {
	case limit > 0 && offset > 0:
		return "OFFSET " + strconv.Itoa(offset) + " ROWS FETCH NEXT " + strconv.Itoa(limit) + " ROWS ONLY"
	case limit > 0:
		return "FETCH FIRST " + strconv.Itoa(limit) + " ROWS ONLY"
	case offset > 0:
		return "OFFSET " + strconv.Itoa(offset) + " ROWS"
	default:
		return ""
	}
}

func oracleTruncFormat(part string) string {
	switch strings.ToLower(strings.TrimSpace(part)) {
	case "day":
		return "DD"
	case "week":
		return "IW"
	case "month":
		return "MM"
	case "quarter":
		return "Q"
	case "year":
		return "YYYY"
	default:
		return "DD"
	}
}

// DateTrunc returns TRUNC(col, 'fmt').
func (d OracleDialect) DateTrunc(part, column string) string {
	return fmt.Sprintf("TRUNC(%s, '%s')", d.QuoteIdent(column), oracleTruncFormat(part))
}

// DateTruncPlaceholder truncates a bind-parameter timestamp.
func (OracleDialect) DateTruncPlaceholder(part, placeholder string) string {
	return fmt.Sprintf("TRUNC(CAST(%s AS TIMESTAMP), '%s')", placeholder, oracleTruncFormat(part))
}

// CalendarPart returns EXTRACT-based buckets; Oracle EXTRACT has no QUARTER,
// so quarter uses TO_CHAR(d, 'Q').
func (d OracleDialect) CalendarPart(part, column string) string {
	return CalendarPartLookup(d, part, column,
		"EXTRACT(YEAR FROM %s)",
		"TO_NUMBER(TO_CHAR(%s, 'Q'))",
		"EXTRACT(MONTH FROM %s)",
	)
}

// ILike returns a case-insensitive LIKE via UPPER on both sides.
func (OracleDialect) ILike(column, placeholder string) string {
	return fmt.Sprintf("UPPER(%s) LIKE UPPER(%s)", column, placeholder)
}

// SelectWithLimit formats an Oracle SELECT capped with FETCH FIRST.
func (OracleDialect) SelectWithLimit(columns []string, table string, limit int) string {
	sql := "SELECT " + strings.Join(columns, ", ") + " FROM " + table
	if limit > 0 {
		sql += " FETCH FIRST " + strconv.Itoa(limit) + " ROWS ONLY"
	}
	return sql
}

var _ Dialect = OracleDialect{}
```

- [ ] **Step 4: Run all dialect tests**

Run: `go test ./internal/dialect/ -v`
Expected: PASS (new + existing).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/dialect/oracle.go internal/dialect/oracle_test.go
git add internal/dialect/oracle.go internal/dialect/oracle_test.go
git commit -m "feat(dialect): oracle dialect"
```

---

### Task 8: Query-layer dialect branches (calendar cast + expression compiler)

**Files:**
- Modify: `internal/query/calendar_grain_filter.go` (`(*Compiler).castColumnAsDate`, ~line 249)
- Modify: `internal/query/expr_compiler.go` (`dialectFunctions` ~line 16, `normalizeExprDialect` ~line 61)
- Test: `internal/query/calendar_grain_filter_test.go`, `internal/query/expr_compiler_test.go` (append to existing files)

Verified signatures: `castColumnAsDate` is a `*Compiler` method switching on `c.dialect.Name()`; `normalizeExprDialect(d dialect.Dialect) dialect.Dialect` type-switches on concrete dialect structs to fix zero-value quote chars; `dialectFunctions` is `map[string]map[string]string` keyed by upper function name then `d.Name()`.

- [ ] **Step 1: Write failing tests**

Append to `internal/query/calendar_grain_filter_test.go`:

```go
func TestCastColumnAsDate_sqlite(t *testing.T) {
	c := NewCompiler(dialect.SQLite)
	if got := c.castColumnAsDate("d"); got != `date("d")` {
		t.Errorf("got %q", got)
	}
}

func TestCastColumnAsDate_newDriversDefault(t *testing.T) {
	for _, d := range []dialect.Dialect{dialect.Snowflake, dialect.Databricks, dialect.Oracle} {
		c := NewCompiler(d)
		got := c.castColumnAsDate("d")
		want := "CAST(" + d.QuoteIdent("d") + " AS DATE)"
		if got != want {
			t.Errorf("%s: got %q, want %q", d.Name(), got, want)
		}
	}
}
```

(Add `"github.com/biqly/biqly/internal/dialect"` to imports if not present; match the existing import path.)

Append to `internal/query/expr_compiler_test.go`:

```go
func TestNormalizeExprDialect_newDrivers(t *testing.T) {
	cases := []struct {
		zero dialect.Dialect
		want string
	}{
		{dialect.SQLiteDialect{}, "sqlite"},
		{dialect.SnowflakeDialect{}, "snowflake"},
		{dialect.DatabricksDialect{}, "databricks"},
		{dialect.OracleDialect{}, "oracle"},
	}
	for _, tc := range cases {
		got := normalizeExprDialect(tc.zero)
		if got.Name() != tc.want {
			t.Errorf("normalizeExprDialect(%T).Name() = %q, want %q", tc.zero, got.Name(), tc.want)
		}
		// Zero-value structs must be replaced by the canonical instance so
		// QuoteIdent works; verify a quote round-trip does not produce "".
		if q := got.QuoteIdent("x"); q == "x" || q == "" {
			t.Errorf("%T: QuoteIdent broken after normalize: %q", tc.zero, q)
		}
	}
}

func TestDialectFunctions_oracleSubstr(t *testing.T) {
	m, ok := dialectFunctions["SUBSTRING"]
	if !ok || m["oracle"] != "SUBSTR" {
		t.Errorf("SUBSTRING oracle mapping = %v", m)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/query/ -run 'TestCastColumnAsDate|TestNormalizeExprDialect|TestDialectFunctions_oracle' -v`
Expected: FAIL — sqlite falls to default CAST; zero-value dialects keep empty quotes (databricks QuoteIdent falls back to `"` default, but the sqlite case and the SUBSTRING map definitely fail). If the databricks/oracle normalize sub-cases pass trivially due to BaseDialect quote fallbacks, that is fine — the point of Step 3 is canonical-instance replacement, matching the existing convention.

- [ ] **Step 3: Implement**

In `(*Compiler).castColumnAsDate` (`internal/query/calendar_grain_filter.go` ~line 249), add a case before `default`:

```go
	case "sqlite":
		return fmt.Sprintf("date(%s)", quoted)
```

(SQLite `CAST(x AS DATE)` yields numeric affinity — wrong; `date()` returns the ISO date string. snowflake/databricks/oracle keep the default `CAST(%s AS DATE)`, which is valid on all three.)

In `normalizeExprDialect` (`internal/query/expr_compiler.go` ~line 61), add cases to the type switch mirroring the existing ones:

```go
	case dialect.SQLiteDialect:
		if concrete.QuoteLeft == "" {
			return dialect.SQLite
		}
	case dialect.SnowflakeDialect:
		if concrete.QuoteLeft == "" {
			return dialect.Snowflake
		}
	case dialect.DatabricksDialect:
		if concrete.QuoteLeft == "" {
			return dialect.Databricks
		}
	case dialect.OracleDialect:
		if concrete.QuoteLeft == "" {
			return dialect.Oracle
		}
```

In `dialectFunctions`, extend the existing `"SUBSTRING"` entry (Oracle has SUBSTR, not SUBSTRING):

```go
	"SUBSTRING": {
		"clickhouse": "substring",
		"oracle":     "SUBSTR",
	},
```

Documented limitation (already in the spec): `DATE_TRUNC` inside *calculated expressions* has no sqlite/oracle mapping in `dialectFunctions` — dimension-level time grains work fine (dialect DateTrunc), only free-form expressions using DATE_TRUNC will emit the unmapped name on those two engines.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/query/ -v -run 'TestCastColumnAsDate|TestNormalizeExprDialect|TestDialectFunctions'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/query/
git add internal/query/
git commit -m "feat(query): dialect branches for sqlite/snowflake/databricks/oracle"
```

---

### Task 9: PII masking dialect branches

**Files:**
- Modify: `internal/security/pii/masking.go` (helpers `castText`, `left`, `right`, `fromChar`, `maskIPExpr`, lines ~100-175)
- Test: `internal/security/pii/masking_test.go` (append; file is `package pii`)

Verified signatures in `masking.go`: `castText(col string, d dialect.Dialect)`, `concat(d, parts...)`, `left(d dialect.Dialect, expr string, n int)`, `right(d, expr, n)`, `fromChar(d dialect.Dialect, expr, marker string)` (marker is the bare char; the function builds the `'...'` literal), `maskIPExpr(d dialect.Dialect, expr string)`. All switch on `d.Name()`.

- [ ] **Step 1: Write failing tests** — append to `internal/security/pii/masking_test.go`:

```go
func TestCastText_newDrivers(t *testing.T) {
	cases := []struct {
		d    dialect.Dialect
		want string
	}{
		{dialect.Oracle, "CAST(x AS VARCHAR2(256))"},
		{dialect.Databricks, "CAST(x AS STRING)"},
		{dialect.SQLite, "CAST(x AS TEXT)"},
		{dialect.Snowflake, "CAST(x AS TEXT)"},
	}
	for _, tc := range cases {
		if got := castText("x", tc.d); got != tc.want {
			t.Errorf("castText(%s) = %q, want %q", tc.d.Name(), got, tc.want)
		}
	}
}

func TestLeftRight_substrDialects(t *testing.T) {
	for _, d := range []dialect.Dialect{dialect.Oracle, dialect.SQLite} {
		if got := left(d, "x", 3); got != "SUBSTR(x, 1, 3)" {
			t.Errorf("left(%s) = %q", d.Name(), got)
		}
		if got := right(d, "x", 2); got != "SUBSTR(x, -2, 2)" {
			t.Errorf("right(%s) = %q", d.Name(), got)
		}
	}
}

func TestFromChar_newDrivers(t *testing.T) {
	cases := []struct {
		d    dialect.Dialect
		want string
	}{
		{dialect.SQLite, "SUBSTR(x, INSTR(x, '@'))"},
		{dialect.Oracle, "SUBSTR(x, INSTR(x, '@'))"},
		{dialect.Snowflake, "SUBSTR(x, POSITION('@', x))"},
		{dialect.Databricks, "substring(x, instr(x, '@'))"},
	}
	for _, tc := range cases {
		if got := fromChar(tc.d, "x", "@"); got != tc.want {
			t.Errorf("fromChar(%s) = %q, want %q", tc.d.Name(), got, tc.want)
		}
	}
}

func TestMaskIPExpr_newDrivers(t *testing.T) {
	const pattern = "'[0-9a-fA-F]+'"
	for _, d := range []dialect.Dialect{dialect.Snowflake, dialect.Databricks, dialect.Oracle} {
		want := "REGEXP_REPLACE(x, " + pattern + ", '*')"
		if got := maskIPExpr(d, "x"); got != want {
			t.Errorf("maskIPExpr(%s) = %q, want %q", d.Name(), got, want)
		}
	}
	// SQLite has no native regex: must fail closed to the hidden literal.
	if got := maskIPExpr(dialect.SQLite, "x"); got != HiddenLiteral {
		t.Errorf("maskIPExpr(sqlite) = %q, want %q", got, HiddenLiteral)
	}
}
```

(Add `"github.com/biqly/biqly/internal/dialect"` to the test file imports if missing.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/security/pii/ -run 'TestCastText_new|TestLeftRight_substr|TestFromChar_new|TestMaskIPExpr_new' -v`
Expected: FAIL — oracle/databricks castText hit the postgres default; left/right emit LEFT/RIGHT; fromChar emits the postgres FROM/POSITION-IN form; maskIPExpr for snowflake/databricks/oracle falls to HiddenLiteral (sqlite sub-case passes already via the default branch).

- [ ] **Step 3: Implement in `internal/security/pii/masking.go`**

`castText` — add cases before `default` (TEXT is invalid on Oracle and Databricks; sqlite/snowflake keep the ANSI default):

```go
	case "oracle":
		return "CAST(" + col + " AS VARCHAR2(256))"
	case "databricks":
		return "CAST(" + col + " AS STRING)"
```

`left` — oracle/sqlite lack LEFT():

```go
func left(d dialect.Dialect, expr string, n int) string {
	switch d.Name() {
	case "clickhouse":
		return "substring(" + expr + ", 1, " + strconv.Itoa(n) + ")"
	case "oracle", "sqlite":
		return "SUBSTR(" + expr + ", 1, " + strconv.Itoa(n) + ")"
	default:
		return "LEFT(" + expr + ", " + strconv.Itoa(n) + ")"
	}
}
```

`right` — oracle/sqlite lack RIGHT(); negative start counts from the end in both:

```go
func right(d dialect.Dialect, expr string, n int) string {
	switch d.Name() {
	case "clickhouse":
		return "substring(" + expr + ", length(" + expr + ") - " + strconv.Itoa(n-1) + ", " + strconv.Itoa(n) + ")"
	case "oracle", "sqlite":
		return "SUBSTR(" + expr + ", -" + strconv.Itoa(n) + ", " + strconv.Itoa(n) + ")"
	default:
		return "RIGHT(" + expr + ", " + strconv.Itoa(n) + ")"
	}
}
```

(snowflake/databricks have native LEFT/RIGHT — default OK.)

`fromChar` — add cases to the switch (the postgres `SUBSTRING(x FROM POSITION(c IN x))` form is unsupported on all four):

```go
	case "sqlite", "oracle":
		return "SUBSTR(" + expr + ", INSTR(" + expr + ", " + lit + "))"
	case "snowflake":
		return "SUBSTR(" + expr + ", POSITION(" + lit + ", " + expr + "))"
	case "databricks":
		return "substring(" + expr + ", instr(" + expr + ", " + lit + "))"
```

(SQLite's functions are case-insensitive, so upper-case SUBSTR/INSTR compiles fine and shares the oracle branch.)

`maskIPExpr` — snowflake/databricks/oracle support 3-arg REGEXP_REPLACE that replaces all matches by default:

```go
	case "mysql", "snowflake", "databricks", "oracle":
		return "REGEXP_REPLACE(" + expr + ", " + pattern + ", '*')"
```

(Fold into/alongside the existing mysql case. sqlite stays on the default branch → HiddenLiteral, fail closed.)

- [ ] **Step 4: Run the full pii package tests**

Run: `go test ./internal/security/pii/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/security/pii/
git add internal/security/pii/
git commit -m "feat(pii): masking expressions for sqlite/snowflake/databricks/oracle"
```

---

### Task 10: SQLite driver package

**Files:**
- Create: `internal/datasource/sqlite/driver.go`
- Create: `internal/datasource/sqlite/introspect.go`
- Test: `internal/datasource/sqlite/driver_test.go`

- [ ] **Step 1: Create `internal/datasource/sqlite/driver.go`** (mirror `postgres/driver.go`):

```go
// Package sqlite implements the SQLite datasource driver.
package sqlite

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "modernc.org/sqlite" // register sqlite driver
)

// Driver implements the datasource.Driver interface for SQLite.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new SQLite driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("sqlite", "sqlite", dialect.SQLite, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables, columns and relations.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
```

This mirrors `postgres/driver.go` exactly: pointer embed of `*datasource.BaseDriver`, and `ComposeIntrospection(ctx, db, d.Type(), steps)` — note the `d.Type()` dbSystem argument.

- [ ] **Step 2: Create `internal/datasource/sqlite/introspect.go`**

SQLite has a single logical schema (`main`); tables come from `sqlite_master`; columns/FKs come from the parameter-bindable pragma table-valued functions.

```go
package sqlite

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
)

func introspectSchemas(_ context.Context, _ *sql.DB) ([]datasource.SchemaInfo, error) {
	return []datasource.SchemaInfo{{Name: "main"}}, nil
}

const tablesQuery = `
SELECT name, type
FROM sqlite_master
WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
ORDER BY name`

func introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	return datasource.QueryAll(ctx, db, tablesQuery, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		var typ string
		if err := rows.Scan(&t.TableName, &typ); err != nil {
			return t, err
		}
		t.SchemaName = "main"
		t.TableType = "BASE TABLE"
		if typ == "view" {
			t.TableType = "VIEW"
		}
		return t, nil
	})
}

const columnsQuery = `
SELECT ti.name, ti.type, ti."notnull", ti.dflt_value, ti.cid, ti.pk
FROM pragma_table_info(?) AS ti
ORDER BY ti.cid`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	tables, err := introspectTables(ctx, db)
	if err != nil {
		return nil, err
	}
	var out []datasource.ColumnInfo
	for _, t := range tables {
		cols, err := datasource.QueryAll(ctx, db, columnsQuery, []any{t.TableName}, func(rows *sql.Rows) (datasource.ColumnInfo, error) {
			var c datasource.ColumnInfo
			var notNull, pk, cid int
			var dflt sql.NullString
			if err := rows.Scan(&c.ColumnName, &c.DataType, &notNull, &dflt, &cid, &pk); err != nil {
				return c, err
			}
			c.SchemaName = "main"
			c.TableName = t.TableName
			c.Nullable = notNull == 0
			c.OrdinalPosition = cid + 1
			if dflt.Valid {
				c.ColumnDefault = dflt.String
			}
			c.IsPrimaryKey = pk > 0
			return c, nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, cols...)
	}
	return out, nil
}

const fkQuery = `
SELECT fk.id, fk."table", fk."from", fk."to"
FROM pragma_foreign_key_list(?) AS fk
ORDER BY fk.id, fk.seq`

func introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	tables, err := introspectTables(ctx, db)
	if err != nil {
		return nil, err
	}
	var out []datasource.RelationInfo
	for _, t := range tables {
		rels, err := datasource.QueryAll(ctx, db, fkQuery, []any{t.TableName}, func(rows *sql.Rows) (datasource.RelationInfo, error) {
			var r datasource.RelationInfo
			var id int
			if err := rows.Scan(&id, &r.ToTable, &r.FromColumn, &r.ToColumn); err != nil {
				return r, err
			}
			r.ConstraintName = fmt.Sprintf("fk_%s_%d", t.TableName, id)
			r.FromSchema = "main"
			r.FromTable = t.TableName
			r.ToSchema = "main"
			r.RelationshipType = datasource.DefaultRelationshipType
			return r, nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, rels...)
	}
	return out, nil
}
```

(Add `"fmt"` to the imports. Field names verified against `internal/datasource/datasource.go`: `TableInfo{SchemaName, TableName, TableType, RowEstimate, Comment}`, `ColumnInfo{SchemaName, TableName, ColumnName, DataType, Nullable bool, OrdinalPosition, ColumnDefault string, IsPrimaryKey}`, `RelationInfo{ConstraintName, FromSchema, FromTable, FromColumn, ToSchema, ToTable, ToColumn, RelationshipType}`.)

- [ ] **Step 3: Write E2E test** — create `internal/datasource/sqlite/driver_test.go` (pure-Go driver ⇒ real temp DB in CI):

```go
package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriver_IntrospectRealDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d := NewDriver()
	ctx := context.Background()

	rw, err := d.Open(ctx, "file:"+path)
	require.NoError(t, err)
	_, err = rw.ExecContext(ctx, `CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = rw.ExecContext(ctx, `CREATE TABLE orders (
		id INTEGER PRIMARY KEY,
		customer_id INTEGER NOT NULL REFERENCES customers(id),
		total REAL DEFAULT 0
	)`)
	require.NoError(t, err)
	require.NoError(t, rw.Close())

	db, err := d.Open(ctx, "file:"+path+"?mode=ro")
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, d.Ping(ctx, "file:"+path+"?mode=ro"))

	res, err := d.Introspect(ctx, db)
	require.NoError(t, err)
	require.Len(t, res.Schemas, 1)
	require.Equal(t, "main", res.Schemas[0].Name)
	require.Len(t, res.Tables, 2)
	require.Len(t, res.Relations, 1)
	require.Equal(t, "orders", res.Relations[0].FromTable)
	require.Equal(t, "customers", res.Relations[0].ToTable)

	var cols int
	for _, c := range res.Columns {
		if c.Table == "orders" {
			cols++
		}
	}
	require.Equal(t, 3, cols)
}
```

(Adjust field accessors to the real `IntrospectionResult` shape.)

- [ ] **Step 4: Run test**

Run: `go test ./internal/datasource/sqlite/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go mod tidy
gofmt -w internal/datasource/sqlite/
git add internal/datasource/sqlite/ go.mod go.sum
git commit -m "feat(datasource): sqlite driver with pragma-based introspection"
```

---

### Task 11: Snowflake driver package

**Files:**
- Create: `internal/datasource/snowflake/driver.go`
- Create: `internal/datasource/snowflake/introspect.go`
- Test: `internal/datasource/snowflake/driver_test.go`

- [ ] **Step 1: Create `internal/datasource/snowflake/driver.go`**:

```go
// Package snowflake implements the Snowflake datasource driver.
package snowflake

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/snowflakedb/gosnowflake" // register snowflake driver
)

// Driver implements the datasource.Driver interface for Snowflake.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new Snowflake driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("snowflake", "snowflake", dialect.Snowflake, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables and columns. Snowflake's
// INFORMATION_SCHEMA exposes no KEY_COLUMN_USAGE, so relations and primary-key
// flags are not populated (Phase 1 limitation).
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
```

- [ ] **Step 2: Create `internal/datasource/snowflake/introspect.go`**:

```go
package snowflake

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
)

const schemasQuery = `
SELECT schema_name
FROM information_schema.schemata
WHERE schema_name NOT IN ('INFORMATION_SCHEMA')
ORDER BY schema_name`

func introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	return datasource.QueryAll(ctx, db, schemasQuery, nil, datasource.ScanSchemaName)
}

const tablesQuery = `
SELECT table_schema, table_name,
       CASE table_type WHEN 'VIEW' THEN 'VIEW' ELSE 'BASE TABLE' END,
       row_count, COALESCE(comment, '')
FROM information_schema.tables
WHERE table_schema NOT IN ('INFORMATION_SCHEMA')
ORDER BY table_schema, table_name`

func introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	return datasource.QueryAll(ctx, db, tablesQuery, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Comment)
		return t, err
	})
}

const columnsQuery = `
SELECT table_schema, table_name, column_name, data_type,
       CASE is_nullable WHEN 'YES' THEN 1 ELSE 0 END,
       ordinal_position, character_maximum_length,
       numeric_precision, numeric_scale, COALESCE(column_default, '')
FROM information_schema.columns
WHERE table_schema NOT IN ('INFORMATION_SCHEMA')
ORDER BY table_schema, table_name, ordinal_position`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	return datasource.QueryAll(ctx, db, columnsQuery, nil, datasource.ScanStandardColumnInfo)
}

func introspectRelations(_ context.Context, _ *sql.DB) ([]datasource.RelationInfo, error) {
	// Snowflake INFORMATION_SCHEMA has no KEY_COLUMN_USAGE; FK metadata is only
	// available via SHOW IMPORTED KEYS, whose output is version-brittle.
	return nil, nil
}
```

**Note on scan helpers (verified against `introspect_scan.go`):** `ScanStandardColumnInfo` expects exactly 10 columns in order: schema, table, name, data_type, nullable(0/1 int), ordinal, char_max_length, numeric_precision, numeric_scale, default(string). `ScanForeignKeyRelation` expects 7: constraint_name FIRST, then from_schema/from_table/from_column/to_schema/to_table/to_column. `ScanTableInfo` scans only 4 columns (no comment) — use an inline scan func when the query selects a comment column, like postgres does.

- [ ] **Step 3: Write mock-conn test** — create `internal/datasource/snowflake/driver_test.go` following the pattern in `internal/datasource/clickhouse/driver_test.go` (fake `database/sql/driver` conn keyed by query substring). Copy that file's mock scaffolding and register rows for the three queries:

```go
package snowflake

// Mirror clickhouse/driver_test.go's mockConn/mockRows scaffolding.
// Register results:
//   "information_schema.schemata" -> [["ANALYTICS"]]
//   "information_schema.tables"   -> [["ANALYTICS", "ORDERS", "BASE TABLE", int64(42), "Orders table"]]
//   "information_schema.columns"  -> [["ANALYTICS", "ORDERS", "ID", "NUMBER", int64(0), int64(1), nil, nil, nil, ""]]
// Then assert:
//   res.Schemas == 1, res.Tables == 1, res.Columns == 1, res.Relations empty
//   Driver.Type() == "snowflake", Dialect().Name() == "snowflake"
```

Write the full test by copying the clickhouse test structure verbatim and substituting the queries/rows above.

- [ ] **Step 4: Run test**

Run: `go test ./internal/datasource/snowflake/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go mod tidy
gofmt -w internal/datasource/snowflake/
git add internal/datasource/snowflake/ go.mod go.sum
git commit -m "feat(datasource): snowflake driver (information_schema introspection)"
```

---

### Task 12: Databricks driver package

**Files:**
- Create: `internal/datasource/databricks/driver.go`
- Create: `internal/datasource/databricks/introspect.go`
- Test: `internal/datasource/databricks/driver_test.go`

- [ ] **Step 1: Create `internal/datasource/databricks/driver.go`**:

```go
// Package databricks implements the Databricks SQL warehouse datasource driver.
package databricks

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/databricks/databricks-sql-go" // register databricks driver
)

// Driver implements the datasource.Driver interface for Databricks.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new Databricks driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("databricks", "databricks", dialect.Databricks, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables, columns and (when Unity Catalog is
// enabled) foreign-key relations.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
```

- [ ] **Step 2: Create `internal/datasource/databricks/introspect.go`**:

```go
package databricks

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/biqly/biqly/internal/datasource"
)

const schemasQuery = `
SELECT schema_name
FROM information_schema.schemata
WHERE schema_name NOT IN ('information_schema')
ORDER BY schema_name`

func introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	return datasource.QueryAll(ctx, db, schemasQuery, nil, datasource.ScanSchemaName)
}

const tablesQuery = `
SELECT table_schema, table_name,
       CASE table_type WHEN 'VIEW' THEN 'VIEW' ELSE 'BASE TABLE' END,
       CAST(NULL AS BIGINT), COALESCE(comment, '')
FROM information_schema.tables
WHERE table_schema NOT IN ('information_schema')
ORDER BY table_schema, table_name`

func introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	return datasource.QueryAll(ctx, db, tablesQuery, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Comment)
		return t, err
	})
}

const columnsQuery = `
SELECT table_schema, table_name, column_name, data_type,
       CASE is_nullable WHEN 'YES' THEN 1 ELSE 0 END,
       ordinal_position, character_maximum_length,
       numeric_precision, numeric_scale, COALESCE(column_default, '')
FROM information_schema.columns
WHERE table_schema NOT IN ('information_schema')
ORDER BY table_schema, table_name, ordinal_position`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	return datasource.QueryAll(ctx, db, columnsQuery, nil, datasource.ScanStandardColumnInfo)
}

const relationsQuery = `
SELECT rc.constraint_name,
       kcu.table_schema, kcu.table_name, kcu.column_name,
       rk.table_schema, rk.table_name, rk.column_name
FROM information_schema.referential_constraints rc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_catalog = rc.constraint_catalog
 AND kcu.constraint_schema = rc.constraint_schema
 AND kcu.constraint_name = rc.constraint_name
JOIN information_schema.key_column_usage rk
  ON rk.constraint_catalog = rc.unique_constraint_catalog
 AND rk.constraint_schema = rc.unique_constraint_schema
 AND rk.constraint_name = rc.unique_constraint_name
 AND rk.ordinal_position = kcu.position_in_unique_constraint
ORDER BY rc.constraint_name, kcu.ordinal_position`

func introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	rels, err := datasource.QueryAll(ctx, db, relationsQuery, nil, datasource.ScanForeignKeyRelation)
	if err != nil {
		// hive_metastore catalogs have no information_schema constraint views;
		// degrade to no relations instead of failing the whole sync.
		slog.WarnContext(ctx, "databricks relation introspection unavailable", "error", err)
		return nil, nil
	}
	return rels, nil
}
```

Align `ScanForeignKeyRelation`'s expected 7-column layout with the SELECT list — verified: it scans constraint_name first, then from_schema/from_table/from_column/to_schema/to_table/to_column (see `introspect_scan.go:38`). The relations query above matches this order.

- [ ] **Step 3: Write mock-conn test** — create `internal/datasource/databricks/driver_test.go`, again copying the clickhouse mock pattern. Cover:
  - happy path: schemas/tables/columns rows returned; relations query returns one FK row → `res.Relations` has 1 entry
  - degrade path: relations query returns an error → `Introspect` still succeeds with `res.Relations` empty
  - `Driver.Type() == "databricks"`, `Dialect().Name() == "databricks"`

- [ ] **Step 4: Run test**

Run: `go test ./internal/datasource/databricks/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go mod tidy
gofmt -w internal/datasource/databricks/
git add internal/datasource/databricks/ go.mod go.sum
git commit -m "feat(datasource): databricks driver with unity-catalog FK degrade"
```

---

### Task 13: Oracle driver package

**Files:**
- Create: `internal/datasource/oracle/driver.go`
- Create: `internal/datasource/oracle/introspect.go`
- Test: `internal/datasource/oracle/driver_test.go`

- [ ] **Step 1: Create `internal/datasource/oracle/driver.go`**:

```go
// Package oracle implements the Oracle Database datasource driver.
package oracle

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/sijms/go-ora/v2" // register oracle driver
)

// Driver implements the datasource.Driver interface for Oracle Database (12c+).
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new Oracle driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("oracle", "oracle", dialect.Oracle, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables, columns and relations from the ALL_*
// dictionary views, restricted to non-Oracle-maintained users.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
```

- [ ] **Step 2: Create `internal/datasource/oracle/introspect.go`**:

```go
package oracle

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
)

const userFilter = `SELECT username FROM all_users WHERE oracle_maintained = 'N'`

const schemasQuery = `
SELECT username FROM all_users
WHERE oracle_maintained = 'N'
ORDER BY username`

func introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	return datasource.QueryAll(ctx, db, schemasQuery, nil, datasource.ScanSchemaName)
}

const tablesQuery = `
SELECT t.owner, t.table_name, 'BASE TABLE', t.num_rows, COALESCE(c.comments, '')
FROM all_tables t
LEFT JOIN all_tab_comments c ON c.owner = t.owner AND c.table_name = t.table_name
WHERE t.owner IN (` + userFilter + `)
UNION ALL
SELECT v.owner, v.view_name, 'VIEW', NULL, ''
FROM all_views v
WHERE v.owner IN (` + userFilter + `)
ORDER BY 1, 2`

func introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	return datasource.QueryAll(ctx, db, tablesQuery, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Comment)
		return t, err
	})
}

const columnsQuery = `
SELECT c.owner, c.table_name, c.column_name, c.data_type,
       CASE c.nullable WHEN 'Y' THEN 1 ELSE 0 END,
       c.column_id, c.char_length, c.data_precision, c.data_scale, ''
FROM all_tab_columns c
WHERE c.owner IN (` + userFilter + `)
ORDER BY c.owner, c.table_name, c.column_id`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	cols, err := datasource.QueryAll(ctx, db, columnsQuery, nil, datasource.ScanStandardColumnInfo)
	if err != nil {
		return nil, err
	}
	pks, err := primaryKeyColumns(ctx, db)
	if err != nil {
		return nil, err
	}
	for i := range cols {
		if pks[pkKey{cols[i].SchemaName, cols[i].TableName, cols[i].ColumnName}] {
			cols[i].IsPrimaryKey = true
		}
	}
	return cols, nil
}

type pkKey struct{ schema, table, column string }

const pkQuery = `
SELECT cc.owner, cc.table_name, cc.column_name
FROM all_constraints ac
JOIN all_cons_columns cc
  ON cc.owner = ac.owner AND cc.constraint_name = ac.constraint_name
WHERE ac.constraint_type = 'P'
  AND ac.owner IN (` + userFilter + `)`

func primaryKeyColumns(ctx context.Context, db *sql.DB) (map[pkKey]bool, error) {
	rows, err := datasource.QueryAll(ctx, db, pkQuery, nil, func(r *sql.Rows) (pkKey, error) {
		var k pkKey
		err := r.Scan(&k.schema, &k.table, &k.column)
		return k, err
	})
	if err != nil {
		return nil, err
	}
	set := make(map[pkKey]bool, len(rows))
	for _, k := range rows {
		set[k] = true
	}
	return set, nil
}

const relationsQuery = `
SELECT ac.constraint_name,
       cc.owner, cc.table_name, cc.column_name,
       rc.owner, rc.table_name, rc.column_name
FROM all_constraints ac
JOIN all_cons_columns cc
  ON cc.owner = ac.owner AND cc.constraint_name = ac.constraint_name
JOIN all_cons_columns rc
  ON rc.owner = ac.r_owner AND rc.constraint_name = ac.r_constraint_name
 AND rc.position = cc.position
WHERE ac.constraint_type = 'R'
  AND ac.owner IN (` + userFilter + `)
ORDER BY ac.constraint_name, cc.position``

func introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	return datasource.QueryAll(ctx, db, relationsQuery, nil, datasource.ScanForeignKeyRelation)
}
```

Notes for the implementer:
- `all_tab_columns.data_default` is intentionally skipped (LONG type — brittle to scan through database/sql); the columns query selects `''` for the default slot to satisfy `ScanStandardColumnInfo`'s 10-column layout.
- `ScanStandardColumnInfo` (see `introspect_scan.go:19`) scans: schema, table, name, data_type, nullable(0/1), ordinal, char_max, num_precision, num_scale, default — the queries above match this order exactly.
- The PK post-processing loop sets `ColumnInfo.IsPrimaryKey` (field verified in `datasource.go:46`).

- [ ] **Step 3: Write mock-conn test** — create `internal/datasource/oracle/driver_test.go` copying the clickhouse mock pattern. Register rows keyed by query substrings (`all_users`, `all_tables`, `all_tab_columns`, `constraint_type = 'P'`, `constraint_type = 'R'`) and assert:
  - schemas/tables/columns/relations counts
  - a column present in the PK result gets `IsPrimaryKey == true`
  - `Driver.Type() == "oracle"`, `Dialect().Name() == "oracle"`

- [ ] **Step 4: Run test**

Run: `go test ./internal/datasource/oracle/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go mod tidy
gofmt -w internal/datasource/oracle/
git add internal/datasource/oracle/ go.mod go.sum
git commit -m "feat(datasource): oracle driver (ALL_* dictionary introspection)"
```

---

### Task 14: Register drivers in the app

**Files:**
- Modify: `internal/app/dependencies.go` (~lines 375-378, existing `reg.Register(...)` block)

- [ ] **Step 1: Add registrations** — in the block that registers postgres/mysql/sqlserver/clickhouse, append:

```go
	reg.Register(sqlite.NewDriver())
	reg.Register(snowflake.NewDriver())
	reg.Register(databricks.NewDriver())
	reg.Register(oracle.NewDriver())
```

with imports:

```go
	"github.com/biqly/biqly/internal/datasource/databricks"
	"github.com/biqly/biqly/internal/datasource/oracle"
	"github.com/biqly/biqly/internal/datasource/sqlite"
	"github.com/biqly/biqly/internal/datasource/snowflake"
```

(Match the module path used by existing imports in the file; keep import grouping/gofmt happy.)

- [ ] **Step 2: Build and run app tests**

Run: `go build ./... && go test ./internal/app/ ./internal/datasource/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
gofmt -w internal/app/dependencies.go
git add internal/app/dependencies.go
git commit -m "feat(app): register sqlite, snowflake, databricks, oracle drivers"
```

---

### Task 15: Frontend driver metadata (dbDrivers.ts + logos + i18n)

**Files:**
- Modify: `frontend/src/dbDrivers.ts`
- Create: `frontend/src/assets/db-logos/sqlite.svg`, `snowflake.svg`, `databricks.svg`, `oracle.svg`
- Modify: `frontend/src/i18n/locales/en/core.ts` (`datasources.drivers` at line ~224, `datasources.fields` at ~231)
- Modify: `frontend/src/i18n/locales/tr/core.ts` (mirrored keys, drivers at ~232)

Note: frontend code style has **no semicolons** — match the existing files. A `datasources.fields` i18n block already exists (host/port/database/username/password/ssl_mode/...) — extend it, don't duplicate.

- [ ] **Step 1: Add logos**

Download brand SVGs from simple-icons (MIT-licensed path data) for three; Oracle was removed from simple-icons (trademark) → use a generic database-glyph SVG in Oracle red:

```bash
cd /Users/baris.dogu/src/biqly/biqly/frontend/src/assets/db-logos
curl -fsSL https://cdn.simpleicons.org/sqlite -o sqlite.svg
curl -fsSL https://cdn.simpleicons.org/snowflake -o snowflake.svg
curl -fsSL https://cdn.simpleicons.org/databricks -o databricks.svg
cat > oracle.svg << 'SVG'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#C74634"><path d="M8.5 5h7a7 7 0 1 1 0 14h-7a7 7 0 1 1 0-14zm0 2.5a4.5 4.5 0 1 0 0 9h7a4.5 4.5 0 1 0 0-9z"/></svg>
SVG
```

Verify each file starts with `<svg`: `head -c 100 *.svg`

- [ ] **Step 2: Extend `frontend/src/dbDrivers.ts`**

Extend the existing structures (verified current shape: `DRIVER_IDS` const array + derived `DriverId` union, `DRIVER_LOGOS` record, `normalizeDriverType`, `driverDefaultPort`, `driverStructuredDefaults` returning `{ port: string; ssl_mode: string }`, `driverDsnPlaceholder`):

```ts
import databricksLogo from './assets/db-logos/databricks.svg'
import oracleLogo from './assets/db-logos/oracle.svg'
import sqliteLogo from './assets/db-logos/sqlite.svg'
import snowflakeLogo from './assets/db-logos/snowflake.svg'
```

```ts
export const DRIVER_IDS = [
  'postgres',
  'mysql',
  'sqlserver',
  'clickhouse',
  'sqlite',
  'snowflake',
  'databricks',
  'oracle',
] as const
```

`DRIVER_LOGOS`: add `sqlite: sqliteLogo, snowflake: snowflakeLogo, databricks: databricksLogo, oracle: oracleLogo`.

`normalizeDriverType`: add cases `'sqlite3' → 'sqlite'`, `'spark'/'dbx' → 'databricks'`, `'ora' → 'oracle'` (bare names already pass through the default branch).

`driverDefaultPort`: add `case 'oracle': return 1521` and `case 'databricks': return 443`.

`driverStructuredDefaults`: add cases (mirroring backend `DriverConnectionDefaults`):

```ts
    case 'oracle':
      return { port: portStr, ssl_mode: 'disable' }
    case 'databricks':
      return { port: portStr, ssl_mode: '' }
```

(sqlite/snowflake fall to the default `{ port: '', ssl_mode: '' }`.)

`driverDsnPlaceholder`: add raw-DSN placeholders:

```ts
    case 'sqlite':
      return 'file:/path/to/database.db?mode=ro'
    case 'snowflake':
      return 'user:pass@account/dbname/schema?warehouse=WH'
    case 'databricks':
      return 'token:dapi***@host:443/sql/1.0/warehouses/abc?catalog=main'
    case 'oracle':
      return 'oracle://user:pass@host:1521/service_name'
```

Add the per-driver form spec at the end of the file:

```ts
export interface DriverExtraField {
  key: string
  labelKey: TranslationKey
  required?: boolean
  placeholder?: string
}

export interface DriverFormSpec {
  host: boolean
  hostLabelKey?: TranslationKey
  port: boolean
  username: boolean
  password: boolean
  passwordLabelKey?: TranslationKey
  database: boolean
  databaseLabelKey?: TranslationKey
  databaseRequired?: boolean
  ssl: boolean
  extras: DriverExtraField[]
}

const FULL_FORM: DriverFormSpec = {
  host: true,
  port: true,
  username: true,
  password: true,
  database: true,
  ssl: true,
  extras: [],
}

const FORM_SPECS: Partial<Record<DriverId, DriverFormSpec>> = {
  sqlite: {
    host: false,
    port: false,
    username: false,
    password: false,
    database: true,
    databaseLabelKey: 'datasources.fields.file_path',
    databaseRequired: true,
    ssl: false,
    extras: [],
  },
  snowflake: {
    host: true,
    hostLabelKey: 'datasources.fields.account',
    port: false,
    username: true,
    password: true,
    database: true,
    databaseRequired: true,
    ssl: false,
    extras: [
      { key: 'warehouse', labelKey: 'datasources.fields.warehouse' },
      { key: 'role', labelKey: 'datasources.fields.role' },
      { key: 'schema', labelKey: 'datasources.fields.schema' },
    ],
  },
  databricks: {
    host: true,
    port: true,
    username: false,
    password: true,
    passwordLabelKey: 'datasources.fields.token',
    database: true,
    databaseLabelKey: 'datasources.fields.catalog',
    ssl: false,
    extras: [
      {
        key: 'http_path',
        labelKey: 'datasources.fields.http_path',
        required: true,
        placeholder: '/sql/1.0/warehouses/...',
      },
      { key: 'schema', labelKey: 'datasources.fields.schema' },
    ],
  },
  oracle: {
    host: true,
    port: true,
    username: true,
    password: true,
    database: true,
    databaseLabelKey: 'datasources.fields.service_name',
    databaseRequired: true,
    ssl: true,
    extras: [],
  },
}

export function driverFormSpec(id: string): DriverFormSpec {
  const d = normalizeDriverType(id)
  if ((DRIVER_IDS as readonly string[]).includes(d)) {
    return FORM_SPECS[d as DriverId] ?? FULL_FORM
  }
  return FULL_FORM
}
```

- [ ] **Step 3: Add i18n keys**

`frontend/src/i18n/locales/en/core.ts` — extend `datasources.drivers` (before `unknown`):

```ts
      sqlite: 'SQLite',
      snowflake: 'Snowflake',
      databricks: 'Databricks',
      oracle: 'Oracle',
```

Extend the existing `datasources.fields` block:

```ts
      file_path: 'Database file path',
      account: 'Account identifier',
      warehouse: 'Warehouse',
      role: 'Role',
      schema: 'Schema',
      token: 'Access token',
      catalog: 'Catalog',
      http_path: 'HTTP path',
      service_name: 'Service name',
```

`frontend/src/i18n/locales/tr/core.ts` — mirrored in the same blocks:

```ts
      sqlite: 'SQLite',
      snowflake: 'Snowflake',
      databricks: 'Databricks',
      oracle: 'Oracle',
```

```ts
      file_path: 'Veritabanı dosya yolu',
      account: 'Hesap tanımlayıcısı',
      warehouse: 'Warehouse',
      role: 'Rol',
      schema: 'Şema',
      token: 'Erişim anahtarı',
      catalog: 'Katalog',
      http_path: 'HTTP yolu',
      service_name: 'Servis adı',
```

- [ ] **Step 4: Typecheck**

Run: `make typecheck-frontend`
Expected: PASS. (knip may flag the not-yet-consumed `driverFormSpec`/`DriverExtraField` exports — consumption arrives in Task 16, so defer `make check-frontend` until then and commit Tasks 15+16 together if the pre-commit hook blocks on knip.)

- [ ] **Step 5: Commit (or fold into Task 16's commit if knip blocks)**

```bash
git add frontend/src/dbDrivers.ts frontend/src/assets/db-logos/ frontend/src/i18n/locales/en/core.ts frontend/src/i18n/locales/tr/core.ts
git commit -m "feat(frontend): driver metadata, form specs, logos, i18n for 4 new drivers"
```

---

### Task 16: Adaptive structured connection form

**Files:**
- Modify: `frontend/src/components/Datasources.tsx` (`StructuredForm` line ~49, `emptyStructured` ~64, `connectionSummary` ~75, `draftPayload` ~164, `edit` ~252, `setDriver` ~269, `canSubmit` ~354)
- Modify: `frontend/src/components/datasources/DatasourceFormModal.tsx` (`StructuredForm` line ~28, structured field JSX lines ~190-269)

Verified current shape: `StructuredForm` is duplicated in both files with fields `host, port, username, password, database_name, ssl_mode` (all strings). The modal receives `structured` + `onStructuredChange` props. `canSubmit` in Datasources.tsx gates on `structured.host.trim() !== ''`. Payload keys are `host, port, username, password, database_name, ssl_mode` and the backend also accepts `connection_params` (map). Labels already come from `datasources.fields.*`.

- [ ] **Step 1: Extend the duplicated `StructuredForm` interface in BOTH files**

```ts
interface StructuredForm {
  host: string
  port: string
  username: string
  password: string
  database_name: string
  ssl_mode: string
  extras: Record<string, string>
}
```

In `Datasources.tsx` update `emptyStructured()` to include `extras: {}`, and in `edit(ds)` seed `extras: {}` (existing connection params are not round-tripped to the form in Phase 1 — password-style opacity; note this in the code with a short comment only if the file already comments similar decisions).

- [ ] **Step 2: Render fields from the spec in `DatasourceFormModal.tsx`**

Import the spec: add `driverFormSpec` to the existing `../../dbDrivers` import. Derive `const spec = driverFormSpec(form.type)` next to `driverConnHints`.

Wrap each structured field's `form-group` div in a spec conditional, keeping the existing markup/classes:

- host row: render the host `form-group` only `{spec.host && (...)}`, label `{t(spec.hostLabelKey ?? 'datasources.fields.host')}`
- port `form-group`: `{spec.port && (...)}`
- database `form-group`: `{spec.database && (...)}`, label `{t(spec.databaseLabelKey ?? 'datasources.fields.database')}`
- username `form-group`: `{spec.username && (...)}`
- password `form-group`: `{spec.password && (...)}`, label `{t(spec.passwordLabelKey ?? 'datasources.fields.password')}`
- ssl `form-group`: `{spec.ssl && (...)}`

Layout note: host+port share a `modalFormRowClass()` row, as do database+username. When one of a pair is hidden the remaining field simply fills the row — keep the row wrapper but conditionally render each child; if BOTH children of a row are hidden, hide the row wrapper too (e.g. `{(spec.host || spec.port) && <div className={modalFormRowClass()}>...`).

After the ssl block (inside the structured `<>` fragment), render extras:

```tsx
{spec.extras.map((f) => (
  <div key={f.key} className={legacyFormClass('form-group')}>
    <label htmlFor={`ds-extra-${f.key}`}>{t(f.labelKey)}</label>
    <input
      id={`ds-extra-${f.key}`}
      value={structured.extras[f.key] ?? ''}
      onChange={(e) =>
        onStructuredChange({
          ...structured,
          extras: { ...structured.extras, [f.key]: e.target.value },
        })
      }
      placeholder={f.placeholder}
      autoComplete="off"
    />
    {!f.required && (
      <small className="text-foreground-muted mt-1 block text-[0.75rem] leading-[1.35]">
        {t('common.optional')}
      </small>
    )}
  </div>
))}
```

- [ ] **Step 3: Make `draftPayload` in `Datasources.tsx` spec-driven**

Replace the `if (!structured.host.trim()) return null` gate and the connection-object construction:

```ts
    const spec = driverFormSpec(form.type)
    if (spec.host && !structured.host.trim()) {
      return null
    }
    if (spec.databaseRequired && !structured.database_name.trim()) {
      return null
    }
    if (spec.extras.some((f) => f.required && !(structured.extras[f.key] ?? '').trim())) {
      return null
    }
    const portStr = spec.port ? structured.port.trim() : ''
    let port: number | undefined
    if (portStr !== '') {
      const n = parseInt(portStr, 10)
      if (Number.isNaN(n) || n <= 0) {
        return null
      }
      port = n
    }

    const connection: Record<string, unknown> = {}
    if (spec.host) {
      connection.host = structured.host.trim()
    }
    if (spec.username) {
      connection.username = structured.username
    }
    if (spec.password) {
      connection.password = structured.password
    }
    if (spec.database) {
      connection.database_name = structured.database_name
    }
    if (port !== undefined) {
      connection.port = port
    }
    if (spec.ssl) {
      const ssl = structured.ssl_mode.trim()
      if (ssl) {
        connection.ssl_mode = ssl
      }
    }
    const extras = Object.fromEntries(
      Object.entries(structured.extras).filter(([, v]) => v.trim() !== ''),
    )
    if (Object.keys(extras).length > 0) {
      connection.connection_params = extras
    }
```

(The rest of `draftPayload` — name/raw handling and the returned object — stays unchanged.)

- [ ] **Step 4: Make `canSubmit` spec-driven**

Replace the structured branch of `canSubmit` (~line 354):

```ts
  const spec = driverFormSpec(form.type)
  const canSubmit =
    form.name.trim() !== '' &&
    (connMode === 'raw'
      ? editingId !== null || form.dsn.trim() !== ''
      : (!spec.host || structured.host.trim() !== '') &&
        (!spec.databaseRequired || structured.database_name.trim() !== '') &&
        spec.extras.every((f) => !f.required || (structured.extras[f.key] ?? '').trim() !== '') &&
        (structured.port.trim() === '' ||
          (!Number.isNaN(parseInt(structured.port, 10)) && parseInt(structured.port, 10) > 0)))
```

- [ ] **Step 5: Fix `connectionSummary` for hostless drivers**

In the structured branch (~line 82), after the `if (host)` case add:

```ts
    if (db) {
      return { line1: db }
    }
```

(SQLite rows then show the file path instead of an empty line.)

- [ ] **Step 6: Reset extras on driver switch**

In `setDriver` (~line 269):

```ts
  const setDriver = (type: string) => {
    setForm({ ...form, type })
    const defaults = driverStructuredDefaults(type)
    setStructured({ ...structured, port: defaults.port, ssl_mode: defaults.ssl_mode, extras: {} })
  }
```

- [ ] **Step 7: Run frontend gate**

Run: `make check-frontend`
Expected: PASS (eslint + tailwind + format:check + knip + vitest + tsc). Fix format drift with `npm --prefix frontend run format`.

- [ ] **Step 8: Manual smoke test**

```bash
make dev-up        # if not already running
make watch SVC="api auth"   # terminal 1
make dev-frontend           # terminal 2
```

In the browser create-datasource modal verify: SQLite shows only the file-path field; Snowflake shows account/user/pass/db + warehouse/role/schema; Databricks shows host/port/token/catalog + required http_path; Oracle shows the full form with service-name label; postgres/mysql forms unchanged. Save button disabled until required per-driver fields are filled.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/components/Datasources.tsx frontend/src/components/datasources/DatasourceFormModal.tsx
git commit -m "feat(frontend): adaptive per-driver connection form"
```

---

### Task 17: Full verification gate

**Files:** none (verification only)

- [ ] **Step 1: Run the complete pre-commit gate**

Run: `make precommit`
Expected: format + lint-go + test-go (race) + check-frontend all PASS.

- [ ] **Step 2: Deadcode check**

Run: `deadcode -test $(go list ./... | grep -v '/frontend')`
Expected: no new findings from this work (new exported dialect vars/driver constructors are consumed by `dependencies.go` and tests).

- [ ] **Step 3: If anything failed** — fix, re-run, and amend/commit the fixes with a `fix:` commit before declaring done.

- [ ] **Step 4: Final commit (if fixes were needed) and summary**

Report: drivers registered, forms adaptive, documented limitations (Snowflake FKs empty, Oracle EXPLAIN skipped, DATE_TRUNC-in-expressions unsupported on sqlite/oracle, sqlite IP-masking fails closed).
