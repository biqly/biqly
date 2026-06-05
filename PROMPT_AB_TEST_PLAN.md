# Prompt A/B Testing Plan

A/B testing framework for AI prompt templates. Allows running multiple prompt versions simultaneously with configurable traffic splits and automatic metric collection.

---

## Context

### Current Infrastructure

The project already has:

- **`ai_prompt_templates`** table: versioned templates keyed by `(name, locale, version)` with `is_active` flag — only one version is active per name+locale
- **`internal/ai/prompt/prompt_store.go`**: `PromptTemplateStore` interface with `Template()` and `Snapshot()` methods, `dbPromptStore` with in-memory cache + invalidation
- **`internal/ai/prompt/prompt_templates.go`**: `KnownPromptTemplateNames()` returns `["system_rules", "output_format", "retry", "clarification", "ambiguity", "prompt_layout"]`
- **`internal/ai/schema.go`**: `AIMetadata` already carries `PromptTemplateVersions map[string]int`, `TokenUsage`, `CostUSD`, `Confidence`
- **`ai_query_history`** table: already tracks `confidence_score`, `latency_ms`, `cost_usd`, `token_count`, `user_rating`, `warnings`
- **`ai_feedback`** table: stores user feedback with `rating` and `categories`
- **`internal/metadata/curated_ai.go`**: `ModelSuccessRateRow` already computes success rate, avg confidence, feedback counts
- **`internal/ai/eval/`**: 3-tier eval framework (structural, execution, LLM judge) with `QuestionResult.PromptTemplateVersions`

### Gap

Currently only **one** prompt version is active per name+locale. There is no way to split traffic between versions, compare their performance, or determine statistical significance.

---

## Phase 1 — Experiment Data Model (4h)

Define the core types for experiments, variants, and traffic allocation.

- [x] Create `internal/ai/abtest/types.go`:
  ```
  ExperimentStatus: "draft" | "running" | "paused" | "completed"

  Experiment {
    ID              string
    Name            string           — human-readable: "System Rules v12 vs v13"
    Description     string
    TemplateName    string           — which prompt section: "system_rules", "output_format", etc.
    Locale          string           — "en" or "tr"
    Status          ExperimentStatus
    StartedAt       *time.Time
    EndedAt         *time.Time
    CreatedBy       string
    CreatedAt       time.Time
    UpdatedAt       time.Time
  }

  Variant {
    ID              string
    ExperimentID    string
    Name            string           — "v12", "v13", "control", "treatment"
    TemplateVersion int              — which version from ai_prompt_templates
    TrafficPct      int              — 0-100, must sum to 100 across variants
    IsControl       bool             — one variant must be control
  }

  ExperimentMetrics {
    ExperimentID    string
    VariantID       string
    PeriodStart     time.Time
    PeriodEnd       time.Time
    TotalQueries    int
    SuccessRate     float64          — queries without errors / total
    ValidatorPassRate float64        — structural validation passed / total
    AvgConfidence   float64
    UserCorrectionRate float64       — user_rating="negative" / rated queries
    PositiveFeedbackRate float64     — user_rating="positive" / rated queries
    ExecutionSuccessRate float64     — SQL executed without error / compiled
    AvgCostUSD      float64
    AvgLatencyMs    float64
    TotalTokens     int
  }
  ```
- [x] Define traffic allocation rule: variant selection is deterministic per user (hash `user_id + experiment_id` → bucket) so users see consistent variants
- [x] Unit tests for traffic allocation logic

## Phase 2 — Database Schema (3h)

Extend the existing metadata DB with experiment tables.

- [x] Create migration `migrations/042a_add_ab_experiments.up.sql`:
  ```sql
  CREATE TABLE ab_experiments (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    template_name TEXT NOT NULL,        -- references ai_prompt_templates.name
    locale      TEXT NOT NULL DEFAULT 'en',
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','running','paused','completed')),
    started_at  TIMESTAMPTZ,
    ended_at    TIMESTAMPTZ,
    created_by  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE INDEX idx_ab_exp_status ON ab_experiments(status);

  CREATE TABLE ab_variants (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    experiment_id   TEXT NOT NULL REFERENCES ab_experiments(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    template_version INT NOT NULL,
    traffic_pct     INT NOT NULL DEFAULT 50 CHECK (traffic_pct >= 0 AND traffic_pct <= 100),
    is_control      BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(experiment_id, name)
  );

  -- Add experiment context to existing ai_query_history
  ALTER TABLE ai_query_history ADD COLUMN IF NOT EXISTS ab_experiment_id TEXT;
  ALTER TABLE ai_query_history ADD COLUMN IF NOT EXISTS ab_variant_id TEXT;
  ```
- [x] Create down migration `migrations/042b_add_ab_experiments.down.sql`
- [ ] Verify migration runs cleanly against existing DB
  - Blocked locally on 2026-06-05: Docker daemon was unavailable, `localhost:5432` refused connections, and the installed `libpq` `initdb` could not start a temporary Postgres server because the matching `postgres` binary is missing. Static migration-shape verification passed with `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate ./internal/ai/abtest -count=1`.

## Phase 3 — Repository Layer (4h)

CRUD operations for experiments and variants.

- [x] Create `internal/ai/abtest/repository.go`:
  - `CreateExperiment(ctx, experiment *Experiment) error`
  - `UpdateExperiment(ctx, experiment *Experiment) error`
  - `GetExperiment(ctx, id string) (*Experiment, error)`
  - `ListExperiments(ctx, status string) ([]Experiment, error)`
  - `AddVariant(ctx, variant *Variant) error`
  - `UpdateVariant(ctx, variant *Variant) error`
  - `ListVariants(ctx, experimentID string) ([]Variant, error)`
  - `DeleteVariant(ctx, id string) error`
  - `GetRunningExperimentsForTemplate(ctx, templateName, locale string) ([]Experiment, error)`
- [x] Validate traffic_pct sums to 100 on create/update
- [x] Validate template_version actually exists in `ai_prompt_templates`
- [x] Validate exactly one variant has `is_control = true`
- [x] Unit tests with mock DB

## Phase 4 — Traffic Router (3h)

Determine which variant a request gets.

- [ ] Create `internal/ai/abtest/router.go`:
  ```
  type TrafficRouter struct { repo Repository }

  func (r *TrafficRouter) ResolveVariant(
    ctx context.Context,
    userID string,
    templateName string,
    locale string,
    defaultVersion int,
  ) (variant Variant, err error)
  ```
- [ ] Algorithm:
  1. Look up running experiments for `(templateName, locale)`
  2. If no running experiment → return synthetic variant with `TemplateVersion = defaultVersion`
  3. Hash `userID + experimentID` → uint32 → mod 100 → pick variant from cumulative traffic ranges
  4. Return selected variant
- [ ] Cache running experiments in-memory with TTL (30s) — experiments change rarely
- [ ] Thread-safe cache with `sync.RWMutex`
- [ ] Unit tests: verify deterministic allocation, traffic split accuracy, no-experiment fallback

## Phase 5 — Integration into Prompt Rendering Pipeline (5h)

The key integration point. When building a prompt, check if the template has an active experiment and use the variant's version instead of the active one.

- [ ] Modify `internal/ai/prompt/prompt_store.go`:
  - Add `SnapshotForUser(ctx, userID, locale, name) PromptTemplateSnapshot` method to `PromptTemplateStore` interface
  - In `dbPromptStore.SnapshotForUser`:
    1. Call `abtest.TrafficRouter.ResolveVariant(ctx, userID, name, locale, defaultVersion)`
    2. If experiment is active: load the specific `template_version` from `ai_prompt_templates` (add `GetPromptTemplateByVersion` to repo)
    3. If no experiment: fall back to current `Snapshot()` behavior (active version)
    4. Return snapshot with the variant's content + version
- [ ] Add `GetPromptTemplateByVersion(ctx, name, locale string, version int) (string, error)` to `internal/metadata/ai_prompt_templates.go`
- [ ] Modify `internal/ai/service.go` — the main `ProcessQuestion` flow:
  1. Extract `userID` from context (already available via auth middleware)
  2. Use `SnapshotForUser` instead of `Snapshot` when building the prompt
  3. After generation, record `ab_experiment_id` and `ab_variant_id` in the `AIMetadata` response
- [ ] Modify `internal/http/handlers/` AI query handler:
  1. When inserting into `ai_query_history`, include `ab_experiment_id` and `ab_variant_id`
- [ ] Invalidate variant cache when experiment status changes (reuse `InvalidatePromptTemplateCache` pattern)
- [ ] Integration test: verify prompt rendering uses variant version when experiment is running

## Phase 6 — Metrics Collection & Aggregation (6h)

Aggregate existing `ai_query_history` data by experiment/variant.

- [ ] Create `internal/ai/abtest/metrics.go`:
  ```
  type MetricsCollector struct { repo *metadata.Repository }

  func (m *MetricsCollector) ComputeMetrics(
    ctx context.Context,
    experimentID string,
    periodStart, periodEnd time.Time,
  ) ([]ExperimentMetrics, error)
  ```
- [ ] Aggregation query (per variant):
  ```sql
  SELECT
    ab_variant_id,
    COUNT(*) AS total_queries,
    -- success rate: no errors (confidence >= 0.7 AND no critical warnings)
    COUNT(*) FILTER (WHERE confidence_score >= 0.7
      AND (warnings IS NULL OR cardinality(warnings) = 0))
      / COUNT(*)::float AS success_rate,
    -- validator pass rate
    COUNT(*) FILTER (WHERE confidence_score >= 0.7)
      / COUNT(*)::float AS validator_pass_rate,
    -- user correction rate
    COUNT(*) FILTER (WHERE user_rating = 'negative')
      / NULLIF(COUNT(*) FILTER (WHERE user_rating IS NOT NULL), 0)::float AS user_correction_rate,
    -- execution success rate
    COUNT(*) FILTER (WHERE sql_text IS NOT NULL)
      / NULLIF(COUNT(*), 0)::float AS execution_success_rate,
    AVG(confidence_score) AS avg_confidence,
    AVG(cost_usd) AS avg_cost_usd,
    AVG(latency_ms) AS avg_latency_ms,
    SUM(token_count) AS total_tokens
  FROM ai_query_history
  WHERE ab_experiment_id = $1
    AND created_at BETWEEN $2 AND $3
  GROUP BY ab_variant_id
  ```
- [ ] Create `internal/ai/abtest/significance.go`:
  - Implement two-proportion z-test for comparing success rates
  - Implement two-sample t-test for comparing avg cost / latency
  - Return `SignificanceResult { IsSignificant bool, PValue float64, Confidence float64 }`
  - Use standard library `math` — no external stats dependency
- [ ] Wire into `MetricsCollector.ComputeMetrics` — compute significance vs control for each treatment variant
- [ ] Unit tests for significance calculation with known values

## Phase 7 — Admin API Endpoints (5h)

REST API for managing experiments and viewing results.

- [ ] Create `internal/http/handlers/ab_experiment.go`:

  **Experiment CRUD:**
  - `POST /api/ai/ab-experiments` — create experiment (admin only)
  - `GET /api/ai/ab-experiments` — list experiments (filterable by status)
  - `GET /api/ai/ab-experiments/{id}` — get experiment details + variants
  - `PUT /api/ai/ab-experiments/{id}` — update experiment (name, description)
  - `PUT /api/ai/ab-experiments/{id}/status` — transition status: `draft→running`, `running→paused`, `paused→running`, `running→completed`

  **Variant management:**
  - `POST /api/ai/ab-experiments/{id}/variants` — add variant
  - `PUT /api/ai/ab-experiments/{id}/variants/{variantId}` — update variant (traffic %, name)
  - `DELETE /api/ai/ab-experiments/{id}/variants/{variantId}` — remove variant (only in draft)

  **Metrics:**
  - `GET /api/ai/ab-experiments/{id}/metrics` — compute and return metrics for date range
  - `GET /api/ai/ab-experiments/{id}/timeseries` — daily metrics breakdown for charts

- [ ] Add routes to `internal/http/catalog_router.go` under `/api/ai/ab-experiments`
- [ ] Admin-only middleware: all endpoints require `admin` role
- [ ] Validation: cannot start experiment if variants don't sum to 100%, cannot modify running experiment variants (only pause → modify → resume)
- [ ] Integration tests for each endpoint

## Phase 8 — Frontend Admin UI (8h)

Management interface for creating and monitoring experiments.

- [ ] Create `frontend/src/components/admin/ABExperimentList.tsx`:
  - Table of experiments with status badge, template name, variant count
  - Filter by status
  - "New Experiment" button
- [ ] Create `frontend/src/components/admin/ABExperimentDetail.tsx`:
  - Experiment config (name, template, locale, status)
  - Variant list with traffic % sliders (must sum to 100)
  - Status transition buttons (Start, Pause, Resume, Complete)
  - Metrics comparison table:
    | Metric | Control (v12) | Treatment (v13) | Δ | Significant? |
    |--------|--------------|-----------------|---|-------------|
    | Success Rate | 82% | 87% | +5% | ✓ p=0.03 |
    | Avg Confidence | 0.78 | 0.83 | +0.05 | ✓ p=0.01 |
    | Cost/query | $0.012 | $0.014 | +$0.002 | ✗ |
  - Time-series charts (daily success rate, cost, confidence per variant)
- [ ] Create `frontend/src/components/admin/ABExperimentForm.tsx`:
  - Template name dropdown (from `KnownPromptTemplateNames`)
  - Locale selector
  - Variant builder: name + template version dropdown + traffic %
  - Live validation (traffic sums to 100)
- [ ] Add CSS in `frontend/src/styles/ab-experiment.css` (BEM naming)
- [ ] Add i18n strings via `useT()` hook
- [ ] Vitest component tests for ABExperimentDetail

## Phase 9 — Automated Decision Helper (3h)

Help admins decide when to promote a variant.

- [ ] Create `internal/ai/abtest/recommender.go`:
  ```
  type Recommendation struct {
    WinnerVariantID  string
    Reason           string
    Metrics          map[string]float64  — delta vs control
    Significance     SignificanceResult
    SampleSize       int
    MinSampleReached bool
  }

  func Recommend(ctx, experimentID string) (*Recommendation, error)
  ```
- [ ] Logic:
  1. Check if minimum sample size reached (configurable, default 100 per variant)
  2. Compare each treatment vs control on primary metric (success_rate by default)
  3. If significant at p < 0.05 AND practical significance (≥ 2% improvement) → recommend winner
  4. If degrading → recommend stopping
  5. Otherwise → "continue collecting data"
- [ ] Expose via `GET /api/ai/ab-experiments/{id}/recommendation`
- [ ] Show recommendation badge in frontend detail view
- [ ] Config: `BI_AB_MIN_SAMPLE_SIZE` (default 100), `BI_AB_SIGNIFICANCE_THRESHOLD` (default 0.05)

## Phase 10 — Testing & Polish (3h)

- [ ] End-to-end test: create experiment → start → run queries → verify traffic split → check metrics
- [ ] Test variant cache invalidation on status change
- [ ] Test deterministic user assignment (same user → same variant)
- [ ] Verify existing prompt rendering unaffected when no experiments are running
- [ ] Load test: verify traffic router adds < 1ms to prompt build latency
- [ ] Run `make lint-go` + `make test-go`
- [ ] Run `make lint-frontend` + `make test-frontend`
- [ ] Update README API endpoints section with A/B experiment routes

---

## File Map

```
internal/ai/abtest/
├── types.go           — Experiment, Variant, ExperimentMetrics
├── repository.go      — DB CRUD for experiments + variants
├── router.go          — TrafficRouter (deterministic variant selection)
├── metrics.go         — MetricsCollector (aggregation queries)
├── significance.go    — Statistical significance tests
├── recommender.go     — Automated winner recommendation
├── router_test.go
├── significance_test.go
└── metrics_test.go

internal/ai/prompt/
├── prompt_store.go    — +SnapshotForUser method
└── (modified)         — interface extension

internal/metadata/
├── ai_prompt_templates.go  — +GetPromptTemplateByVersion
└── (existing)              — ai_query_history already has metric columns

internal/http/handlers/
├── ab_experiment.go   — All A/B experiment endpoints
└── (modified)         — AI query handler records experiment context

migrations/
├── XXX_add_ab_experiments.up.sql
└── XXX_add_ab_experiments.down.sql

frontend/src/
├── components/admin/ABExperimentList.tsx
├── components/admin/ABExperimentDetail.tsx
├── components/admin/ABExperimentForm.tsx
└── styles/ab-experiment.css
```

## Estimated Total: ~44h

## Key Design Decisions

1. **Deterministic assignment**: Same user always sees same variant (hash-based, not random per request)
2. **No new metrics tables**: Reuse `ai_query_history` with added `ab_experiment_id` / `ab_variant_id` columns
3. **Non-invasive**: When no experiments are running, the prompt pipeline is completely unchanged
4. **Admin-only**: Experiment management restricted to admin role
5. **Template-scoped**: Each experiment targets one `(template_name, locale)` pair
6. **Significance-aware**: Automated recommendation only when statistical significance is reached

## Dependencies on Existing Code

- `internal/ai/prompt/prompt_store.go` — `PromptTemplateStore` interface (add `SnapshotForUser`)
- `internal/ai/prompt/prompt_templates.go` — `KnownPromptTemplateNames()`
- `internal/metadata/ai_prompt_templates.go` — `GetPromptTemplateVersion`, add `GetPromptTemplateByVersion`
- `internal/ai/schema.go` — `AIMetadata` (add `ABExperimentID`, `ABVariantID`)
- `internal/ai/service.go` — `ProcessQuestion` flow (pass userID to prompt builder)
- `internal/metadata/curated_ai.go` — existing metric aggregation patterns
- `internal/ai/eval/eval_runner.go` — `QuestionResult.PromptTemplateVersions` (extend with variant info)
- `internal/http/handlers/` — AI query handler (record experiment context in history)
- `internal/http/catalog_router.go` — route registration
