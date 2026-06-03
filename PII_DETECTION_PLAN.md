# PII Detection and Masking — Implementation Plan

> **Status:** Implemented (bkz. işaretli maddeler; kalan açık maddeler için notlar)
> **Priority:** High
> **Depends on:** Existing permissions system (`permissions` table, `SecurityPolicy`, `PermissionManager`)

## Overview

Datasource sync sonrası otomatik PII tespiti ve role-based masking sistemi.
Kolon adı, sample data ve regex pattern üzerinden 7 PII tipi tespit edilir.
Compiler aşamasında role bazlı masking SQL ifadeleri uygulanır.

**PII Types:** `email`, `phone`, `iban`, `tc_kimlik_no`, `address`, `ip_address`, `credit_card_like`

**Access Policy:**
| Role    | Behavior                      |
|---------|-------------------------------|
| Admin   | Raw value (no masking)        |
| Analyst | Masked value (e.g. `jo***@gmail.com`) |
| Viewer  | Hidden (column excluded / `***`) |

---

## Phase 1 — Data Model & Migration

### 1.1 Migration: PII Column Annotations

- [x] Create migration `038a_add_pii_annotations.up.sql`
  - Add `pii_type` TEXT column to `columns` table (nullable, values: `email`, `phone`, `iban`, `tc_kimlik_no`, `address`, `ip_address`, `credit_card_like`, NULL)
  - Add `pii_confidence` FLOAT column to `columns` table (nullable, 0.0–1.0)
  - Add `pii_detected_at` TIMESTAMPTZ column to `columns` table (nullable)
  - Add `pii_reviewed_by` TEXT column to `columns` table (nullable, admin manual review tracking)
  - Add `pii_masking_strategy` TEXT column to `columns` table (nullable, values: `partial`, `full`, `hash`, custom)
  - Create index `idx_columns_pii_type ON columns(pii_type) WHERE pii_type IS NOT NULL`
- [x] Create migration `038b_add_pii_annotations.down.sql`
  - Drop added columns and index

### 1.2 PII Policy Extension in `permissions` Table

- [x] Create migration `039a_add_pii_policy.up.sql`
  - Add `pii_policy` JSONB column to `permissions` table (default `'{}'`)
  - Schema: `{ "<column_qualified_name>": { "access": "raw" | "masked" | "hidden" } }`
  - Example: `{ "customers.email": { "access": "masked" }, "customers.phone": { "access": "hidden" } }`
- [x] Create migration `039b_add_pii_policy.down.sql`

### 1.3 Go Type Updates

- [x] Update `pkg/metadata/types.go` — Add PII fields to `Column` struct:
  ```go
  PIIType            *string   `json:"pii_type,omitempty" db:"pii_type"`
  PIIConfidence      *float64  `json:"pii_confidence,omitempty" db:"pii_confidence"`
  PIIDetectedAt      *time.Time `json:"pii_detected_at,omitempty" db:"pii_detected_at"`
  PIIReviewedBy      *string   `json:"pii_reviewed_by,omitempty" db:"pii_reviewed_by"`
  PIIMaskingStrategy *string   `json:"pii_masking_strategy,omitempty" db:"pii_masking_strategy"`
  ```
- [x] Update `pkg/metadata/types.go` — Add `PIIPolicy` field to `SecurityPolicy`:
  ```go
  PIIPolicy map[string]PIIColumnAccess `json:"pii_policy,omitempty" db:"pii_policy"`
  ```
- [x] Add `PIIColumnAccess` type:
  ```go
  type PIIColumnAccess struct {
      Access string `json:"access"` // "raw", "masked", "hidden"
  }
  ```
- [x] Update `pkg/security/types.go` — Add `PIIPolicy` to `PermissionPolicy`:
  ```go
  PIIPolicy map[string]string `json:"pii_policy,omitempty"` // qualified_name -> "raw"|"masked"|"hidden"
  ```
- [x] Update metadata scan functions in `internal/metadata/permissions.go` for new `pii_policy` column

---

## Phase 2 — PII Detection Engine

### 2.1 Detection Module

- [x] Create `internal/security/pii/` package
- [x] Create `internal/security/pii/detector.go`
  - Define `PIIDetector` struct with regex pattern set and name heuristic rules
  - Define `PIIResult` struct: `{ Type string, Confidence float64, Source string }`
  - Method `DetectFromColumn(columnName string, sampleData []string) []PIIResult`
- [x] Create `internal/security/pii/patterns.go`
  - Compile regex patterns for each PII type:
    - `email`: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
    - `phone`: Turkish phone `^(\+90|0)?[0-9]{10}$` + international variants
    - `iban`: `^TR[0-9]{24}$` + generic IBAN `^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$`
    - `tc_kimlik_no`: `^[1-9][0-9]{10}$` + TCKN checksum validation
    - `address`: keyword heuristic (kolon adında "address", "adres", "addr" geçen + text type + length > 20)
    - `ip_address`: `^(\d{1,3}\.){3}\d{1,3}$` + IPv6
    - `credit_card_like`: Luhn algorithm + `^[0-9]{13,19}$` pattern
- [x] Create `internal/security/pii/name_heuristics.go`
  - Column name → PII type mapping by keywords:
    - email: `email`, `e_mail`, `e-posta`, `mail`
    - phone: `phone`, `tel`, `telephone`, `gsm`, `mobile`, `cep`
    - iban: `iban`, `bank_account`
    - tc_kimlik_no: `tc`, `tckn`, `tc_kimlik`, `identity`, `national_id`
    - address: `address`, `adres`, `addr`
    - ip_address: `ip`, `ip_address`, `ip_addr`
    - credit_card_like: `card_number`, `cc_number`, `credit_card`, `pan`
- [x] Create `internal/security/pii/detector_test.go`
  - Unit tests for each PII type (positive + negative cases)
  - Test confidence scoring logic
  - Test edge cases (NULL samples, empty strings, mixed data)

### 2.2 Detection Logic

Detection flow for a single column:
1. **Name heuristic** → if column name matches PII keywords, set base confidence 0.5
2. **Regex scan on sample data** → match ratio determines confidence boost (0.0–0.5)
3. **Final confidence** = name_score + sample_score
4. **Threshold** → only flag if confidence >= 0.6

- [x] Implement confidence scoring algorithm in `detector.go`
- [x] Add configurable detection threshold (default: 0.6)

---

## Phase 3 — Sync Integration (Auto-Detection on Sync)

### 3.1 Post-Sync PII Scan Hook

- [x] Locate datasource sync flow in `internal/http/handlers/datasources.go` (`SyncMetadata` handler)
- [x] Create `internal/security/pii/scanner.go`
  - `PIIScanner` struct with `Detector` + metadata repository dependency
  - `ScanDatasource(ctx, datasourceID)` method:
    1. Fetch all columns for the datasource
    2. For each column, fetch sample data (top N non-NULL values)
    3. Run `Detector.DetectFromColumn()` on each
    4. Update `columns` table with PII annotations (type, confidence, detected_at)
    5. Return scan summary (count per PII type)
- [x] Create `internal/security/pii/scanner_test.go`
  - Integration-style test with mock repository

### 3.2 Sample Data Fetching

- [x] Add method to `internal/metadata/repository.go`:
  > **Not:** Repository yalnızca metadata DB'sini tuttuğu için sample fetch `internal/security/pii/sampler.go` içinde `NewDBSampleFetcher(db, dialect)` olarak uygulandı (canlı datasource bağlantısı + dialect üzerinden).
  ```go
  GetColumnSampleData(ctx context.Context, datasourceID, schemaName, tableName, columnName string, limit int) ([]string, error)
  ```
  - Uses the datasource connection to run: `SELECT <col> FROM <schema>.<table> WHERE <col> IS NOT NULL LIMIT <limit>`
  - Limit default: 50 rows
- [ ] Add alternative: use driver-specific `information_schema` stats if available
  > **Açık:** Spekülatif optimizasyon — mevcut `SELECT … LIMIT 50` örnekleme tüm driver'larda çalışıyor; ihtiyaç doğarsa eklenecek.
- [x] Update sample data to be fetched via the existing driver registry (`internal/datasource/`)

### 3.3 Sync Handler Integration

- [x] Update `SyncMetadata` handler in `internal/http/handlers/datasources.go`
  - After schema/column sync completes, call `PIIScanner.ScanDatasource()`
  - Run PII scan in background goroutine (non-blocking) if sync is async
  - If sync is sync, run PII scan as final step before response
- [x] Add `scan_pii` query parameter to sync endpoint (default: true)
- [x] Add logging for PII scan results per datasource

### 3.4 PII Rescan Endpoint

- [x] Add HTTP endpoint `POST /api/datasources/{id}/scan-pii`
  - Triggers PII scan on existing datasource without full sync
  - Returns scan summary
- [x] Register route in `internal/http/catalog_router.go`
  - Under datasource access middleware with "write" permission

---

## Phase 4 — Masking Engine (Compiler-Level)

### 4.1 Masking Strategies per Dialect

- [x] Create `internal/security/pii/masking.go`
  - Define `MaskingStrategy` interface:
    ```go
    type MaskingStrategy interface {
        MaskExpression(columnRef string, piiType string, dialect dialect.Dialect) string
    }
    ```
  - Implement `DefaultMaskingStrategy` with per-type templates:

| PII Type          | Masking SQL Pattern                                        |
|-------------------|-----------------------------------------------------------|
| `email`           | `LEFT(col, 2) || '***' || SUBSTRING(col FROM POSITION('@' IN col))` |
| `phone`           | `LEFT(col, 3) || '****' || RIGHT(col, 2)`                |
| `iban`            | `LEFT(col, 4) || '****' || RIGHT(col, 2)`                |
| `tc_kimlik_no`    | `LEFT(col, 3) || '*****'`                                 |
| `address`         | `LEFT(col, 10) || '...'`                                  |
| `ip_address`      | `REGEXP_REPLACE(col, '[0-9]+', '*')` (dialect-specific)  |
| `credit_card_like`| `LEFT(col, 4) || ' **** **** ' || RIGHT(col, 4)`         |

- [x] Handle dialect differences:
  - **PostgreSQL:** `||`, `SUBSTRING`, `POSITION`, `REGEXP_REPLACE`
  - **MySQL:** `CONCAT`, `SUBSTRING`, `LOCATE`, `REGEXP_REPLACE`
  - **SQL Server:** `+`, `SUBSTRING`, `CHARINDEX`, no native regex (use `REPLACE` pattern)
  - **ClickHouse:** `concat`, `substring`, `position`, `replaceRegexpOne`
- [x] Create `internal/security/pii/masking_test.go`
  - Test each PII type × each supported dialect
  - Verify generated SQL expressions are valid

### 4.2 Compiler Integration

- [x] Update `internal/query/compiler.go`
  - Add `PIIMaskingConfig` parameter to `CompileWithPermissions`:
    ```go
    type PIIMaskingConfig struct {
        ColumnAccess map[string]string // qualified col name -> "raw"|"masked"|"hidden"
    }
    ```
  - In `buildSelect()`, before emitting column reference:
    1. Check if column is in `PIIMaskingConfig.ColumnAccess`
    2. If `"hidden"` → replace with `'***' AS <alias>` (or omit column entirely)
    3. If `"masked"` → wrap with masking SQL expression from `MaskingStrategy`
    4. If `"raw"` or absent → no transformation
  - Preserve column alias so result column names stay consistent
- [x] Update `CompileWithPermissions` signature:
  ```go
  func (c *Compiler) CompileWithPermissions(
      ctx context.Context,
      lq *LogicalQuery,
      model *semantic.SemanticModel,
      rowFilters []security.RowFilter,
      piiConfig *PIIMaskingConfig,
  ) (*CompiledQuery, error)
  ```
- [x] Ensure backward compatibility: nil `PIIMaskingConfig` = no masking

### 4.3 Filter & WHERE Clause Handling

- [x] When a column is `"hidden"` for a user, that column must also be excluded from WHERE filters
  - Update validator to reject filter references to hidden PII columns
  - Or: silently replace filter value with `'***'` (configurable behavior)
- [x] Ensure `GROUP BY` and `ORDER BY` also use masked expressions when applicable
  - Grouping on masked values may change semantics → consider blocking or warning

---

## Phase 5 — Permission Policy Enforcement

### 5.1 Role-Based Default PII Policy

- [x] Create `internal/security/pii/policy.go`
  - `DefaultPIIPolicy(role string, piiType string) string`:
    - Admin → `"raw"` for all types
    - Analyst → `"masked"` for all types
    - Viewer → `"hidden"` for sensitive types (`tc_kimlik_no`, `credit_card_like`, `iban`), `"masked"` for others
  - Allow per-column overrides in `permissions.pii_policy`
- [x] Create `internal/security/pii/policy_test.go`

### 5.2 Permission Manager Integration

- [x] Update `internal/security/permissions.go`
  - Add method: `GetPIIPolicy(policy *PermissionPolicy) map[string]string`
  - Returns merged default + explicit PII policy for the user
- [x] Update `PermissionInjector.CheckFieldAccess()` to also check PII hidden columns
- [x] Ensure fail-closed: if PII policy is missing for a PII-annotated column, default to `"masked"`

### 5.3 Query Execution Flow

Full flow with PII masking:

```
User Question → AI → LogicalQuery
  → Validator (check denied fields + PII hidden columns)
  → Compiler.CompileWithPermissions(rowFilters, piiConfig)
    → buildSelect: apply masking expressions
    → buildWhere: reject/transform hidden column refs
    → buildGroupBy: warn on masked columns
  → ReadOnlyChecker
  → Executor
  → Result (PII columns already masked in SQL)
```

- [x] Wire PII policy resolution into the query execution handler
  - `internal/http/handlers/query.go` or `internal/http/handlers/ai.go`
  - Resolve user's PII policy from `permissions` table
  - Pass `PIIMaskingConfig` to compiler

---

## Phase 6 — HTTP API

### 6.1 PII Column Management

- [x] Create `internal/http/handlers/pii.go`
- [x] `GET /api/datasources/{id}/pii-columns`
  - List all PII-annotated columns for a datasource
  - Response: `[{ column_id, schema, table, column, pii_type, confidence, masking_strategy }]`
- [x] `PATCH /api/metadata/columns/{id}/pii`
  - Manually set/override PII type and masking strategy
  - Body: `{ "pii_type": "email", "pii_masking_strategy": "partial", "pii_reviewed_by": "admin@biqly.com" }`
  - Set `pii_confidence = 1.0` for manual review
- [x] `DELETE /api/metadata/columns/{id}/pii`
  - Clear PII annotation (mark as reviewed and safe)
  - Set `pii_type = NULL, pii_confidence = NULL`
- [x] `POST /api/datasources/{id}/scan-pii`
  - Trigger PII scan (Phase 3.4)
  - Response: `{ "scanned_columns": 42, "detected": { "email": 3, "phone": 2, ... } }`
- [x] Register routes in `internal/http/catalog_router.go`

### 6.2 PII Policy in Permission Upsert

- [x] Update `upsertPermissionRequest` in `internal/http/handlers/permissions.go`
  - Add `PIIPolicy map[string]PIIColumnAccess `json:"pii_policy,omitempty"``
- [x] Update `Upsert` handler to persist `pii_policy` JSONB
- [x] Update `GetByKeys` response to include `pii_policy`
- [x] Update `internal/metadata/permissions.go` scan/upsert for new column

---

## Phase 7 — Frontend (Admin UI)

### 7.1 PII Detection Results View

- [x] Add PII tab/section to Datasource detail page
  > **Not:** Admin paneline datasource seçicili "PII Detection" sekmesi olarak eklendi (`PIIDetectionPanel.tsx`).
- [x] Table view: detected PII columns with type, confidence, sample preview (masked for admin safety)
  > **Not:** Sample preview hariç (ek endpoint gerektirir); tip/güven/strateji/inceleme kolonları mevcut.
- [x] Color-coded confidence indicator (green > 0.8, yellow > 0.6, red < 0.6)
- [x] "Rescan PII" button
- [x] "Review" action per column → confirm/dismiss/change type

### 7.2 PII Policy Editor

- [x] Add PII policy section to Permission editor page
- [x] Per-column dropdown: Raw / Masked / Hidden
- [x] Default role template selector (Admin/Analyst/Viewer)
- [x] Bulk apply: "Apply Viewer defaults to all PII columns"

### 7.3 Semantic Model PII Indicators

- [x] Show PII badge on dimension fields that have PII annotations
  > **Not:** Field Permissions panelindeki alan listesinde; canvas editöründe değil.
- [x] Tooltip with PII type and masking strategy
- [ ] Warning when publishing model with unreviewed PII columns
  > **Açık:** Publish akışına backend doğrulaması gerektirir; ayrı bir iş olarak ele alınmalı.

---

## Phase 8 — Audit & Compliance

### 8.1 Audit Logging

- [x] Log PII detection results to audit system (`internal/audit/`)
  - Event: `pii.scan_completed` with per-type counts
- [x] Log PII policy changes
  - Event: `pii.policy_updated` with diff
- [x] Log PII masking applied to queries
  - Event: `pii.masking_applied` with column list

### 8.2 Compliance Report

- [x] Create `GET /api/compliance/pii-summary` endpoint
  - Per-datasource: total columns, PII detected, reviewed count, unreviewed count
  - Per-type breakdown
- [x] Export as CSV/PDF for compliance documentation
  > **Not:** CSV export (`?format=csv`) uygulandı; PDF kapsam dışı bırakıldı.

---

## Phase 9 — Testing & Quality

### 9.1 Unit Tests

- [x] `internal/security/pii/detector_test.go` — all PII type patterns
- [x] `internal/security/pii/masking_test.go` — all dialects × all types
- [x] `internal/security/pii/policy_test.go` — role defaults, overrides, fail-closed
- [x] `internal/security/pii/scanner_test.go` — mock repository scan
- [x] `internal/query/compiler_test.go` — compiler tests with PII masking config
  > **Not:** `internal/query/pii_masking_test.go` olarak eklendi.

### 9.2 Integration Tests

- [ ] End-to-end: create datasource → sync → PII detected → query with masking
  > **Açık:** Canlı DB gerektirir; derleyici seviyesinde rol bazlı kapsama `TestCompileWithPermissions_RoleBasedAccess` ile mevcut.
- [x] Role-based: query as admin (raw) vs analyst (masked) vs viewer (hidden)
  > **Not:** Derleyici seviyesinde (`TestCompileWithPermissions_RoleBasedAccess`); canlı DB ile değil.
- [x] Verify no PII leakage in query results for masked/hidden roles
  > **Not:** Üretilen SQL üzerinde doğrulandı (hidden kolon referansı hiç içermiyor).
- [ ] Test with real Postgres/MySQL/SQLServer/ClickHouse connections
  > **Açık:** Gerçek DB bağlantıları gerektirir (CI altyapısı).

### 9.3 Golden Tests

- [x] Add PII masking golden test cases to `internal/query/golden_test.go`
- [x] Cover each PII type × each dialect with expected SQL output
  > **Not:** Tüm tipler Postgres'te tam SQL ile; e-posta tipi 4 dialect'te tam SQL ile; tüm tip×dialect ifadeleri `masking_test.go`'da.

---

## Phase 10 — Documentation & Configuration

### 10.1 Configuration

- [x] Add PII detection config to application config:
  > **Not:** Kod tabanı konvansiyonuna uygun olarak env tabanlı eklendi (`BI_PII_*` değişkenleri, `internal/config/config.go` → `PIIConfig`).
  ```yaml
  pii:
    enabled: true
    detection_threshold: 0.6
    sample_data_limit: 50
    auto_scan_on_sync: true
    default_masking_strategy: "partial"
  ```
- [ ] Make PII types configurable (allow adding custom types + patterns)
  > **Açık:** Spekülatif — 7 yerleşik tip mevcut ihtiyacı karşılıyor; özel tip/pattern desteği talep olursa eklenecek.

### 10.2 Documentation

- [x] Update `AGENTS.md` with PII-related code locations
- [x] Update `README.md` with PII feature description
- [x] Create `docs/pii-detection-masking.md` with architecture, API, and usage guide
- [x] Add migration notes for upgrade path
  > **Not:** `docs/pii-detection-masking.md` içinde.

---

## Execution Order (Suggested)

1. **Phase 1** (Data Model) — ~2h
2. **Phase 2** (Detection Engine) — ~4h
3. **Phase 3** (Sync Integration) — ~3h
4. **Phase 4** (Masking Engine + Compiler) — ~6h
5. **Phase 5** (Permission Enforcement) — ~3h
6. **Phase 6** (HTTP API) — ~3h
7. **Phase 7** (Frontend) — ~6h
8. **Phase 8** (Audit) — ~2h
9. **Phase 9** (Testing) — ~4h
10. **Phase 10** (Docs & Config) — ~2h

**Total estimate: ~35h**

---

## Key Design Decisions

1. **PII detection at column level, not row level** — Biqly is a BI tool; column-level is appropriate for aggregate analytics
2. **Masking in SQL, not in application** — Database-side masking prevents PII from ever reaching the app server
3. **Fail-closed** — If PII policy is ambiguous, default to most restrictive (hidden)
4. **No raw PII in AI prompts** — PII-annotated columns should use masked sample data in prompt context
5. **Backward compatible** — Existing datasources without PII annotations work unchanged
6. **Manual review** — Auto-detection is a suggestion; admins must review and confirm
