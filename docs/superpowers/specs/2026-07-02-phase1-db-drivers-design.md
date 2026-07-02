# Phase 1 Datasource Drivers: SQLite, Snowflake, Databricks, Oracle

Date: 2026-07-02
Status: Approved

## Goal

Add four new datasource drivers — SQLite, Snowflake, Databricks, Oracle — to
backend and frontend, following the existing driver/dialect architecture with
full structured-form support per driver.

Deferred (out of scope): IBM Db2 (requires CGO + proprietary clidriver),
MongoDB / Cassandra / Elasticsearch (non-SQL, no `database/sql` join-capable
model; would need a separate non-SQL query path).

## Approach

Follow the existing pattern one-to-one (approach A):

- One package per driver under `internal/datasource/<driver>/` embedding
  `BaseDriver` and implementing only `Introspect` via `ComposeIntrospection`.
- One dialect per driver under `internal/dialect/<driver>.go`.
- Registration in `internal/app/dependencies.go`.

Rejected alternative: a config-driven "generic ANSI" driver — less code but
leaks per-driver differences (Oracle `:1` placeholders, Databricks backtick
quoting) through the abstraction.

## Backend

### Go dependencies (all pure Go, `database/sql` compatible)

| Driver | Module | sql driver name |
|---|---|---|
| SQLite | `modernc.org/sqlite` | `sqlite` |
| Snowflake | `github.com/snowflakedb/gosnowflake` | `snowflake` |
| Databricks | `github.com/databricks/databricks-sql-go` | `databricks` |
| Oracle | `github.com/sijms/go-ora/v2` | `oracle` |

### Dialects

| Aspect | SQLite | Snowflake | Databricks | Oracle |
|---|---|---|---|---|
| QuoteIdent | `"x"` | `"x"` | `` `x` `` | `"X"` (upper-fold unquoted refs) |
| Placeholder | `?` | `?` | `?` | `:1`, `:2`, … |
| Limit/Offset | `LIMIT n OFFSET m` | `LIMIT n OFFSET m` | `LIMIT n OFFSET m` | `OFFSET m ROWS FETCH NEXT n ROWS ONLY` (12c+) |
| DateTrunc | `strftime`/`date(col, 'start of month')` | `DATE_TRUNC('month', col)` | `date_trunc('MONTH', col)` | `TRUNC(col, 'MM')` |
| CalendarPart | `CAST(strftime('%Y', col) AS INTEGER)` | `EXTRACT(YEAR FROM col)` | `year(col)` etc. | `EXTRACT(YEAR FROM col)` |
| ILike | `col LIKE ?` (LIKE is case-insensitive for ASCII by default) | native `ILIKE` | native `ILIKE` | `UPPER(col) LIKE UPPER(:n)` |
| ExplainSQL | `EXPLAIN QUERY PLAN <sql>` | `EXPLAIN <sql>` | `EXPLAIN <sql>` | `EXPLAIN PLAN FOR <sql>` |

Oracle minimum supported version: 12c (required for `OFFSET/FETCH`).

### Introspection

- **SQLite**: `sqlite_master` for tables; `PRAGMA table_info(...)` for columns
  and PKs; `PRAGMA foreign_key_list(...)` for relations. Single synthetic
  schema `main`.
- **Oracle**: `ALL_TABLES`, `ALL_TAB_COLUMNS`, `ALL_CONSTRAINTS` +
  `ALL_CONS_COLUMNS` (owner-scoped; skip `SYS`/`SYSTEM` and other built-in
  schemas).
- **Snowflake**: `INFORMATION_SCHEMA` of the connected database (tables,
  columns, `TABLE_CONSTRAINTS` / `REFERENTIAL_CONSTRAINTS` — FKs are
  informational but present in metadata).
- **Databricks**: `information_schema` of the connected catalog (Unity
  Catalog; FKs informational).

### DSN composition (`internal/datasource/dsn.go`)

`prepareDSNParts` currently requires host + port; SQLite (file path) and
Snowflake (account identifier) don't fit. Refactor: move the host/port
requirement into the drivers that need it, keep `ComposeDSN` as the single
entry point.

Per-driver structured fields (extras carried in `ConnectionFields.Extra`):

| Driver | Required | Optional (Extra) | DSN shape |
|---|---|---|---|
| SQLite | `path` (Extra) | — | `file:/path/to.db?mode=ro` |
| Snowflake | `account` (Extra), user, pass | `warehouse`, `role`, db, schema | `user:pass@account/db/schema?warehouse=…&role=…` |
| Databricks | host, `http_path` (Extra), `token` (password field) | `catalog`, `schema` (db field = catalog) | `token:<token>@host:443/<http_path>?catalog=…&schema=…` |
| Oracle | host, port (1521), user, pass, `service` (db field) | `ssl=true` | `oracle://user:pass@host:1521/service` |

`NormalizeDriverType` gains aliases (`sqlite3`→`sqlite`, `spark`/`dbx`→
`databricks`, `ora`→`oracle`). `DefaultPort`: oracle 1521, databricks 443,
snowflake/sqlite 0 (not host/port based).

### Registration & validation

- Register all four in `internal/app/dependencies.go`.
- `internal/http/handlers/datasources.go` DSN-mode validation is
  driver-agnostic and needs no change; structured-mode required-field errors
  come from the compose functions.
- Security pipeline (ReadOnlyChecker, query timeout, row limit, AES DSN
  encryption, fingerprinting) applies unchanged — it operates on compiled SQL
  and stored DSNs, not on driver specifics. SQLite DSNs get `mode=ro` to
  enforce read-only at the connection level.

## Frontend

### Driver metadata (`frontend/src/dbDrivers.ts`)

- Extend `DRIVER_IDS` with `sqlite`, `snowflake`, `databricks`, `oracle`.
- Add four SVG logos to `frontend/src/assets/db-logos/`.
- `driverDefaultPort`: oracle 1521, databricks 443, others none.
- `driverDsnPlaceholder`: realistic example DSN per driver.
- New per-driver field descriptor so the form can adapt: which of
  host/port/username/password/database/ssl apply, plus extra fields
  (`path`, `account`, `warehouse`, `role`, `http_path`, `catalog`).

### Form (`Datasources.tsx` + `DatasourceFormModal.tsx`)

Structured mode adapts per driver:

- **SQLite**: single "Database file path" input; no host/port/user/pass/ssl.
- **Snowflake**: account, username, password, database, schema, warehouse,
  role; no host/port.
- **Databricks**: server hostname, HTTP path, access token (password input),
  catalog, schema; port hidden (443 default).
- **Oracle**: host, port (1521), username, password, service name, ssl toggle.

Raw DSN mode remains available for all drivers. i18n keys
`datasources.drivers.{sqlite,snowflake,databricks,oracle}` plus new field
labels in both `en` and `tr` locales.

## Testing

- Dialect unit tests following existing `internal/dialect/*_test.go`
  golden-style patterns (quote, placeholder, limit/offset, date functions,
  ilike, explain) for all four dialects.
- DSN compose unit tests in `internal/datasource/dsn_test.go` (happy path +
  required-field errors) for all four drivers.
- SQLite end-to-end introspection test with a real temp-file database
  (pure Go, runs in CI): create tables with PK/FK, assert
  `IntrospectionResult`.
- Snowflake/Databricks/Oracle introspection: covered by compile-time
  interface checks and DSN/dialect tests only; no cloud/container
  integration tests in Phase 1.
- Frontend: extend existing driver-metadata/form tests for new driver IDs and
  per-driver field visibility.

## Success criteria

- All four drivers selectable in the UI with adapted structured forms.
- `ComposeDSN` produces valid DSNs; connection test (`POST
  /api/datasources/{id}/test`) works against a real SQLite file.
- Metadata sync + query compile/run path works end-to-end for SQLite locally.
- `make precommit` clean (lint-go, test-go, check-frontend).
