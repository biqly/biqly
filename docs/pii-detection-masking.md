# PII Detection and Masking

Automatic detection of personally identifiable information (PII) in datasource
columns and role-based masking of those columns at SQL compilation time.

## Overview

After every metadata sync (and on demand), Biqly scans each column's name and
sample data against 7 PII types:

| Type | Detection |
| --- | --- |
| `email` | RFC-style regex on samples + name keywords (`email`, `e_posta`, `mail`, …) |
| `phone` | Turkish (`+90`/`0` + 10 digits) and E.164 international formats |
| `iban` | `TR` + 24 digits and generic IBAN format |
| `tc_kimlik_no` | 11-digit format **plus checksum validation** |
| `address` | Name keyword + long free-text samples (name match required) |
| `ip_address` | IPv4/IPv6 parsing |
| `credit_card_like` | 13–19 digits **plus Luhn checksum** |

Confidence = name-keyword score (0.5) + sample match ratio × 0.5. Columns at
or above the threshold (default 0.6) are annotated in the `columns` table
(`pii_type`, `pii_confidence`, `pii_detected_at`). Detection is a suggestion:
admins review each finding in the Admin → PII Detection panel (confirm,
change type, or dismiss). Reviewed columns (`pii_reviewed_by`) are never
overwritten by later scans.

## Access policy

Per-role defaults, overridable per column via `permissions.pii_policy`:

| Role | Default |
| --- | --- |
| admin / super_admin | `raw` — unmasked values |
| analyst | `masked` — e.g. `jo***@gmail.com` |
| viewer / unknown | `hidden` for sensitive types (`tc_kimlik_no`, `credit_card_like`, `iban`), `masked` otherwise |

Fail-closed rules:

- Unknown roles get viewer behavior.
- Invalid policy values resolve to `hidden`.
- Unknown PII types mask to a constant `'***'`.
- A PII policy resolution error fails the query instead of running unmasked.

## Masking is compiled into SQL

Masking happens database-side — raw PII never reaches the app server for
masked/hidden users:

- `masked` columns are wrapped with dialect-specific expressions
  (PostgreSQL `||`/`SUBSTRING`, MySQL `CONCAT`/`LOCATE`, SQL Server
  `CONCAT`/`CHARINDEX`, ClickHouse `concat`/`substring`). SQL Server has no
  native regex, so IP masking falls back to a full `'***'` mask there.
- `hidden` columns are projected as the literal `'***'` (alias preserved);
  the physical column is never referenced. Filters on hidden columns are
  rejected with `HIDDEN_PII_FIELD`.
- `GROUP BY` / `ORDER BY` reuse the masked expression so grouping cannot leak
  raw values.

## Architecture

```
Sync / scan-pii endpoint
  └─ pii.Scanner (internal/security/pii/scanner.go)
       ├─ pii.Detector (detector.go, patterns.go, name_heuristics.go)
       ├─ pii.NewDBSampleFetcher (sampler.go) — SELECT <col> … LIMIT 50 on the live datasource
       └─ metadata.Repository PII methods (internal/metadata/pii.go)

Query execution
  └─ core.QueryService.CompileWithContext
       └─ core.PIIPolicyService (internal/core/pii_policy.go)
            ├─ identity: JWT user + roles (internal/app/pii_identity.go)
            ├─ pii.BuildColumnAccessMaps (policy.go) — role defaults + permissions.pii_policy overrides
            └─ query.PIIMaskingConfig → Compiler (internal/query/pii_masking.go)
                 └─ pii.DefaultMaskingStrategy (masking.go) — dialect SQL
```

## API

| Method | Path | Notes |
| --- | --- | --- |
| `POST` | `/api/datasources/{id}/sync-metadata` | Runs PII scan after sync; `?scan_pii=false` skips |
| `POST` | `/api/datasources/{id}/scan-pii` | Scan without full sync; returns `{scanned_columns, detected}` |
| `GET` | `/api/datasources/{id}/pii-columns` | List annotated columns |
| `PATCH` | `/api/metadata/columns/{id}/pii` | Manual review: `{pii_type, pii_masking_strategy?, pii_reviewed_by}` (confidence pinned to 1.0) |
| `DELETE` | `/api/metadata/columns/{id}/pii?reviewed_by=…` | Clear annotation; with `reviewed_by` future scans won't re-flag |
| `PUT` | `/api/permissions` | Accepts `pii_policy: {"schema.table.column": {"access": "raw"\|"masked"\|"hidden"}}` |
| `GET` | `/api/compliance/pii-summary` | Per-datasource totals + per-type breakdown; `?format=csv` |

Audit events: `pii.scan_completed`, `pii.policy_updated` (with old/new diff),
`pii.masking_applied` (per query compile with the affected column list).

## Configuration (env)

| Variable | Default | Description |
| --- | --- | --- |
| `BI_PII_ENABLED` | `true` | Toggle the whole subsystem |
| `BI_PII_DETECTION_THRESHOLD` | `0.6` | Min confidence to flag a column |
| `BI_PII_SAMPLE_DATA_LIMIT` | `50` | Sample rows fetched per column |
| `BI_PII_AUTO_SCAN_ON_SYNC` | `true` | Scan automatically after metadata sync |
| `BI_PII_DEFAULT_MASKING_STRATEGY` | `partial` | Default masking strategy name |

## Migration notes (upgrade path)

- `038a_add_pii_annotations` adds nullable `pii_*` columns to `columns` plus
  a partial index and CHECK constraints — existing rows are untouched and
  remain non-PII until the first scan.
- `039a_add_pii_policy` adds `permissions.pii_policy JSONB NOT NULL DEFAULT '{}'`.
- Both are backward compatible: datasources without annotations and policies
  without `pii_policy` behave exactly as before (no masking).
- After upgrading, run `POST /api/datasources/{id}/scan-pii` (or a metadata
  sync) per datasource to populate annotations, then review them in
  Admin → PII Detection.

## Frontend

- **Admin → PII Detection** (`PIIDetectionPanel.tsx`): detections per
  datasource with color-coded confidence (green > 0.8, yellow ≥ 0.6,
  red < 0.6), rescan button, confirm/change-type/dismiss actions.
- **Admin → Field Permissions** (`FieldPermissionPanel.tsx`): per-column PII
  access dropdown (role default / raw / masked / hidden), bulk "apply role
  defaults", and a PII badge on dimension fields backed by annotated columns.
