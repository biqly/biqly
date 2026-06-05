# Semantic Drift Detection Plan

Detect and surface mismatches between a semantic model's field references and the actual datasource schema after metadata sync.

---

## Phase 1 — Drift Types & Data Model (4h)

- [x] Define drift types in `internal/semantic/drift/types.go`:
  ```
  DriftTypeColumnDropped   — column referenced by dimension/metric no longer exists
  DriftTypeColumnAdded      — new column in base table not yet modeled (informational)
  DriftTypeTypeChanged      — column data_type changed since last sync
  DriftTypeTableDropped     — base_table or join table no longer exists
  DriftTypeSchemaDropped    — base_schema no longer exists
  DriftTypeJoinBroken       — join references non-existent table/column
  DriftTypeMetricBroken     — metric expression references dropped column
  ```
- [x] Define `DriftReport` struct:
  ```
  DriftReport {
    ModelID       string
    DatasourceID  string
    DetectedAt    time.Time
    SyncID        string   (FK to audit event)
    Drifts        []DriftItem
    Severity      string   "critical" | "warning" | "info"
  }
  DriftItem {
    Type        DriftType
    Field       string   — dimension/metric name
    ColumnRef   string   — schema.table.column
    OldValue    string   — e.g. old data type
    NewValue    string   — e.g. new data type, or "dropped"
    Description string   — human-readable
  }
  ```
- [x] Define severity rules:
  - **critical**: `ColumnDropped`, `TableDropped`, `SchemaDropped` — queries will fail
  - **warning**: `TypeChanged`, `JoinBroken`, `MetricBroken` — may produce wrong results
  - **info**: `ColumnAdded` — no impact, FYI

## Phase 2 — Drift Detector Engine (6h)

- [x] Create `internal/semantic/drift/detector.go` with `Detector` struct:
  ```
  func (d *Detector) Compare(ctx, model SemanticModel, columns []metadata.Column, tables []metadata.Table) (*DriftReport, error)
  ```
- [x] Implement comparison logic:
  1. Build lookup: `map[SchemaTableColumn]metadata.Column` from introspected columns
  2. Build lookup: `map[SchemaTable]bool` from introspected tables
  3. For each dimension in model: resolve `column_ref` → lookup in column map
     - Not found → `DriftTypeColumnDropped`
     - Found but `data_type` differs → `DriftTypeTypeChanged`
  4. For each metric: parse expression tokens → check referenced columns exist
     - Any missing → `DriftTypeMetricBroken`
  5. For each join: check target table + columns exist
     - Missing → `DriftTypeJoinBroken`
  6. Check `base_schema` + `base_table` exist in table map
     - Missing → `DriftTypeSchemaDropped` / `DriftTypeTableDropped`
  7. Compare column count in base table vs modeled dimensions
     - New unmapped columns → `DriftTypeColumnAdded` (info)
- [x] Add `columnRef` parser utility: split `"schema.table.column"` into components
- [x] Unit tests: cover each drift type with mock model + column data

## Phase 3 — Storage & Persistence (3h)

- [x] Create migration `migrations/041a_add_drift_reports.up.sql`:
  ```sql
  CREATE TABLE drift_reports (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    model_id        TEXT NOT NULL REFERENCES semantic_models(id),
    datasource_id   TEXT NOT NULL,
    sync_event_id   TEXT,
    severity        TEXT NOT NULL DEFAULT 'info',
    drifts          JSONB NOT NULL DEFAULT '[]',
    resolved        BOOLEAN NOT NULL DEFAULT false,
    resolved_by     TEXT,
    resolved_at     TIMESTAMPTZ,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE INDEX idx_drift_reports_model ON drift_reports(model_id);
  CREATE INDEX idx_drift_reports_unresolved ON drift_reports(resolved) WHERE NOT resolved;
  ```
- [x] Add repository methods in `internal/semantic/drift/repository.go`:
  - `InsertReport(ctx, report *DriftReport) error`
  - `ListUnresolvedByModel(ctx, modelID string) ([]DriftReport, error)`
  - `ListUnresolvedByDatasource(ctx, dsID string) ([]DriftReport, error)`
  - `ResolveReport(ctx, id, resolvedBy string) error`
  - `GetLatestByModel(ctx, modelID string) (*DriftReport, error)`
- [x] Integration test: insert + query round-trip

## Phase 4 — Sync Hook Integration (5h)

- [x] Extend `SyncMetadata` handler in `internal/http/handlers/datasources.go`:
  1. After `UpsertColumns` completes successfully, capture the final column/table state
  2. Load all active semantic models for this datasource: `semanticRepo.ListModels(ctx, ds.ID)`
  3. For each model, run `detector.Compare(ctx, model, freshColumns, freshTables)`
  4. If report has `critical` or `warning` drifts:
     - Persist via `driftRepo.InsertReport`
     - Emit audit event: `EventDriftDetected` with model ID + drift summary
     - Queue notification (Phase 5)
  5. If only `info` drifts: persist silently (no notification)
- [x] Add `EventDriftDetected` and `EventDriftResolved` to `internal/audit/audit.go` EventType constants
- [x] Add `DriftDetector` and `DriftRepo` to handler dependencies (`HandlersDeps` struct)
- [x] Ensure sync hook runs inside existing transaction context so it rolls back on failure
- [x] Unit test: mock sync flow, verify drift report created for dropped column scenario

## Phase 5 — Notification (4h)

- [x] Add `SendDriftAlert` method to `internal/mail/client.go`:
  ```
  func (c *APIClient) SendDriftAlert(ctx context.Context, email string, report DriftReport) error
  ```
- [x] Add email template in `internal/mail/templates.go`:
  - Subject: `"[Biqly] Schema drift detected in model: {model_name}"`
  - Body: list of drift items with severity badges, link to model editor
- [x] Implement `internal/semantic/drift/notifier.go`:
  ```
  type Notifier struct { mailClient *mail.APIClient; userRepo UserRepository }
  func (n *Notifier) NotifyOwner(ctx, report DriftReport, model SemanticModel) error
  ```
  - Resolve owner email from `model.CreatedBy` → user repo lookup
  - Fall back to workspace admins if `CreatedBy` is nil
  - Send via `mailClient.SendDriftAlert`
- [x] Wire `Notifier.NotifyOwner` into sync hook (Phase 4, step 4)
- [x] Add `SendDriftAlert` to `MockEmailSender` in `internal/mail/mock.go`
- [x] Test: verify email payload with mock sender

## Phase 6 — API Endpoints (4h)

- [x] `GET /api/v1/models/{id}/drift` — list unresolved drift reports for model
- [x] `GET /api/v1/datasources/{id}/drift` — list unresolved drifts across all models for datasource
- [x] `POST /api/v1/drift/{id}/resolve` — mark drift as resolved (sets `resolved=true`, `resolved_by`)
- [x] Add routes in `internal/http/catalog_router.go`
- [x] Add handler in `internal/http/handlers/drift.go`
- [x] Request/response types in `internal/http/handlers/drift_types.go`
- [x] Integration tests for each endpoint

## Phase 7 — Frontend UI (6h)

- [x] Add drift badge to model list items — yellow dot for warning, red for critical
- [x] Add drift panel component in `frontend/src/components/admin/DriftPanel.tsx`:
  - Table of drift items with severity, field name, description
  - "Resolve" button per item (calls POST resolve endpoint)
  - "View in schema" link
- [x] Add drift section to model detail page — visible when unresolved drifts exist
- [x] Add drift column to datasource detail — shows count of affected models
- [x] Add i18n strings via `useT()` hook
- [x] CSS in `frontend/src/styles/drift.css` (BEM naming)
- [ ] Tests: vitest component tests for DriftPanel

## Phase 8 — Scheduled Re-check (3h)

- [x] Create `internal/semantic/drift/scheduler.go`:
  - Runs on configurable interval (default: 6h)
  - For each active datasource: call `detector.Compare` with latest stored columns
  - Persist new reports for any previously-unseen drifts
  - Send notifications for new critical/warning drifts
- [x] Use existing `AIJob` pattern as reference for background task management
- [x] Add config: `BI_DRIFT_CHECK_INTERVAL` env var (default `6h`)
- [x] Wire scheduler into app startup (`internal/app/`)
- [x] Graceful shutdown via context cancellation

## Phase 9 — Testing & Polish (3h)

- [ ] End-to-end test: sync datasource with breaking schema change → verify drift report + notification
- [ ] End-to-end test: resolve drift via API → verify report marked resolved
- [ ] Load test: datasource with 50 models × 100 columns — verify sync+drift completes in <5s
- [x] Add `make lint-go` and `make test-go` verification
- [ ] Update `docs/` if applicable

---

## File Map

```
internal/semantic/drift/
├── types.go          — DriftReport, DriftItem, DriftType
├── detector.go       — Compare logic
├── repository.go     — DB persistence
├── notifier.go       — Email notification
├── scheduler.go      — Periodic re-check
└── detector_test.go

internal/http/handlers/
├── drift.go          — HTTP handlers
└── drift_types.go    — Request/response structs

internal/mail/
├── templates/drift_alert.html  — Email template
└── client.go         — +SendDriftAlert method

migrations/
└── 041a_add_drift_reports.up.sql

frontend/src/
├── components/admin/DriftPanel.tsx
└── styles/drift.css
```

## Estimated Total: ~38h

## Dependencies on Existing Code

- `internal/http/handlers/datasources.go` — `SyncMetadata` handler (hook point)
- `internal/metadata/repository.go` — `ListColumns`, `ListTables`
- `internal/semantic/repository.go` — `ListModels`, `GetFullModel`
- `pkg/semantic/types.go` — `SemanticModel`, `Dimension`, `Metric`, `Join`
- `internal/audit/audit.go` — `EventType` constants, `Logger`
- `internal/mail/client.go` — `APIClient.Send*` pattern
- `internal/metadata/ai_jobs.go` — background job pattern reference
