# Semantic Model Builder Plan

## Goal

Create a guided semantic-model setup flow so a datasource with synced metadata can be made queryable without hand-calling semantic CRUD endpoints.

## Current State

- Semantic model storage, CRUD, validation, publish, rollback, dimensions, metrics, and joins already exist.
- Metadata sync already stores tables, columns, primary keys, foreign keys, descriptions, and relation records.
- Query Builder already expects a selected semantic model and disables model selection when none exists.
- The missing product flow is generating a sensible draft semantic model from metadata, letting the user review it, and publishing it.
- Wren AI's semantic-layer framing matches this direction: reliable Text-to-SQL needs explicit entities, metrics, dimensions, and relationships instead of letting the LLM infer table joins every time.

## Phase 1 - Auto Draft From Metadata

- [x] Add this plan.
- [x] Add `POST /api/semantic/models/generate`.
- [x] Request body:
  - `datasource_id`
  - optional `base_schema`
  - optional `base_table`
  - optional `publish`
- [x] Generation rules:
  - If no base table is provided, pick the table with the strongest relationship signal, then row estimate, then stable name order.
  - Create one draft model for the selected base table.
  - Add dimensions for base-table descriptive/date/boolean columns.
  - Mark likely display dimensions such as `name`, `title`, `label`, `code`, and `number`.
  - Add `count` metric for the base table.
  - Add `sum`/`avg` metrics for numeric non-key columns.
  - Add joins from synced FK relations connected to the selected table.
  - Reuse table and column descriptions as semantic descriptions where available.
- [x] Validate the generated model before returning it.
- [x] Publish only when requested and validation passes.

## Phase 2 - Query Builder CTA

- [x] When datasource has no semantic model, show a compact setup CTA near the disabled model selector.
- [x] CTA calls the generation endpoint with the selected datasource.
- [x] Refresh model list and select the created model.
- [x] Show validation warnings/errors inline.
- [x] Keep the existing "draft model" warning if the generated model is not published.

## Phase 3 - Review Surface

- [x] Add a dedicated semantic model review drawer/page.
- [ ] Allow toggling dimensions/metrics before publish.
- [ ] Allow base-table selection when multiple candidates exist.
- [x] Show FK-derived joins in a modeling canvas.
- [x] Allow manually adding semantic joins when physical FK metadata is missing.
- [x] Add publish action after manual join creation.
- [ ] Add rollback action from the UI.

## Phase 4 - AI-Assisted Enrichment

- [ ] Optionally enrich labels, descriptions, synonyms, and metric names using the existing AI metadata describe flow.
- [ ] Keep generated LogicalQuery safe: AI still never emits raw SQL.
- [ ] Persist user-approved names/synonyms for future routing and prompt context.

## Verification

- [x] Unit-test generator heuristics.
- [ ] Handler test for generated model payload and validation errors.
- [x] Frontend build.
- [x] Browser-check no-model datasource flow.
- [x] Browser-check modeling canvas with table cards, join line, and manual relationship panel.
