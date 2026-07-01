# Design Spec: Frontend Phase 1 — Quick Wins Refactoring

**Date:** 2026-05-19
**Status:** In Progress
**Topic:** Reducing code duplication and improving maintainability in the Biqly frontend.

## 1. Overview
The Biqly frontend currently suffers from significant code duplication in core data-fetching patterns and UI components. Phase 1 focuses on "Quick Wins" that remove redundant logic, standardize common patterns, and clean up dead code.

## 2. Proposed Changes

### 2.1 Shared Hooks Extraction
- **`useDatasourceSelector`**: 
  - Location: `frontend/src/hooks/useDatasourceSelector.ts`
  - Purpose: Fetch datasources, handle active ID state, and sync with `ds` query parameter.
  - Affected Files: `AIQuery.tsx`, `QueryBuilder.tsx`, `Modeling.tsx`, `Metadata.tsx`, `Datasources.tsx`.
- **`useSemanticModels`**:
  - Location: `frontend/src/hooks/useSemanticModels.ts`
  - Purpose: Fetch semantic models for a given `datasourceId`.
  - Affected Files: `QueryBuilder.tsx`, `Modeling.tsx`, `AIQuery.tsx`.

### 2.2 UI Component Unification
- **`ToggleButtonGroup`**:
  - Location: `frontend/src/components/ui/ToggleButtonGroup.tsx`
  - Purpose: Generic button group for toggles.
  - Affected Files: `ThemeToggle.tsx`, `LanguageSwitcher.tsx`, `ChartTypeSelector.tsx`.
- **Modal Standardization**:
  - Replace custom backdrop/card implementations with the existing `<Modal>` component.
  - Affected Files: `FewShotExamples.tsx`, `Modeling.tsx`, `Metadata.tsx`.

### 2.3 Utilities & Formatting
- **`rowsToChartData`**:
  - Location: `frontend/src/utils/chartData.ts`
  - Purpose: Convert result rows to Recharts-compatible objects.
- **`localeNumberTag`**:
  - Location: `frontend/src/utils/formatters.ts`
  - Purpose: Centralize locale string mappings (e.g., `tr` -> `tr-TR`).

### 2.4 Cleanup & Dependencies
- **Add `clsx`**: Install and use for all `className` concatenations.
- **Dead Code Removal**: 
  - `WindowFunction` and `CompiledQuery` from `internal/ai/ai.ts` (if confirmed unused).
  - `useStreamingApi.ts` (if confirmed unused).

## 3. Architecture & Data Flow
- Hooks will return `{ data, loading, error, ...setter }` patterns.
- `useDatasourceSelector` will utilize `useQueryParam` (if exists) or standard URLSearchParams.

## 4. Success Criteria
- ~600 lines of code removed.
- Zero functional regressions in data fetching or UI interactions.
- All modals behave consistently (backdrop clicks, etc.).

## 5. Self-Review Notes
- **Ambiguity Check**: Ensure `useDatasourceSelector` handles the default "first datasource" selection logic exactly as the current implementations do.
- **Dependency**: `clsx` needs to be added to `package.json`.
- **Scope**: Limited to non-breaking structural improvements.
