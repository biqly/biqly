# Frontend Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce code duplication by ~600 lines and standardize core patterns across the Biqly frontend.

**Architecture:** Extract duplicated logic into reusable custom hooks, utilities, and a generic toggle component. Standardize modal usage across the app.

**Tech Stack:** React 19, TypeScript, Vitest.

---

## File Structure & Responsibility Mapping

### New Files
- `frontend/src/hooks/useDatasourceSelector.ts`: Fetching and syncing datasources with URL params.
- `frontend/src/hooks/useSemanticModels.ts`: Fetching semantic models for a datasource.
- `frontend/src/components/ui/ToggleButtonGroup.tsx`: Generic, accessible toggle buttons.
- `frontend/src/utils/chartData.ts`: Unifying row transformation for charts.

### Modified Files
- `frontend/src/hooks/useApi.ts`: Consolidating error parsing (Phase 2 candidate, but will touch for basic fixes).
- `frontend/src/pages/AIQuery.tsx`: Using new hooks/components.
- `frontend/src/pages/Modeling.tsx`: Using new hooks/components.
- `frontend/src/pages/QueryBuilder.tsx`: Using new hooks/components.
- `frontend/src/pages/Metadata.tsx`: Using new hooks/components.
- `frontend/src/pages/Datasources.tsx`: Using new hooks/components.

---

### Task 1: Project Setup - Dependencies & Clean Cleanup

**Files:**
- Modify: `frontend/package.json`
- Modify: `internal/ai/ai.ts` (Remove dead types)
- Delete: `frontend/src/hooks/useStreamingApi.ts`

- [x] **Step 1: Add `clsx` dependency**
  - Run: `npm install clsx` in `frontend/`
- [x] **Step 2: Verify `useStreamingApi.ts` is unused and delete it**
  - Run: `grep -r "useStreamingApi" frontend/src` (should return no imports)
  - Action: Delete the file.
- [ ] **Step 3: Remove dead types from `internal/ai/ai.ts`**
  - Search and remove `WindowFunction` and `CompiledQuery` if they have no references.
- [ ] **Step 4: Commit cleanup**
  - Command: `git commit -m "chore: initial cleanup and add clsx dependency"`

---

### Task 2: Implement `rowsToChartData` Utility

**Files:**
- Create: `frontend/src/utils/chartData.ts`
- Create: `frontend/src/utils/chartData.test.ts`

- [x] **Step 1: Write tests for `rowsToChartData`**
```typescript
import { expect, test } from 'vitest'
import { rowsToChartData } from './chartData'

test('converts rows to chart objects', () => {
  const rows = [['Jan', 100], ['Feb', 200]]
  const result = rowsToChartData(rows)
  expect(result).toEqual([
    { name: 'Jan', value: 100 },
    { name: 'Feb', value: 200 }
  ])
})
```
- [ ] **Step 2: Run test (Verify failure)**
  - Run: `npm test src/utils/chartData.test.ts`
- [x] **Step 3: Implement `rowsToChartData`**
```typescript
export function rowsToChartData(rows: any[][] | undefined) {
  return rows?.map((row) => {
    const obj: { name: string; value?: number } = { name: String(row[0]) }
    if (row[1] !== undefined) obj.value = Number(row[1]) || 0
    return obj
  }) || []
}
```
- [x] **Step 4: Verify test passes**
- [ ] **Step 5: Commit**
  - Command: `git commit -m "feat: add rowsToChartData utility"`

---

### Task 3: Implement `useDatasourceSelector` Hook

**Files:**
- Create: `frontend/src/hooks/useDatasourceSelector.ts`

- [ ] **Step 1: Implement the hook**
  - Logic: Use `useApi`, `useState`, and `useQueryParam('ds')`.
  - Fetch `/api/datasources` on mount.
- [ ] **Step 2: Refactor `QueryBuilder.tsx` to use `useDatasourceSelector`**
  - Remove redundant `useEffect` and `setDatasources` state.
- [ ] **Step 3: Verify QueryBuilder still loads datasources correctly**
- [ ] **Step 4: Commit**
  - Command: `git commit -m "refactor: use useDatasourceSelector in QueryBuilder"`

---

### Task 4: Implement `useSemanticModels` Hook

**Files:**
- Create: `frontend/src/hooks/useSemanticModels.ts`

- [x] **Step 1: Implement the hook**
  - Logic: Fetch `/api/semantic/models?datasource_id=...` whenever `datasourceId` changes.
- [x] **Step 2: Refactor `AIQuery.tsx` to use `useSemanticModels`**
  - Remove local fetching logic.
- [ ] **Step 3: Verify AIQuery still loads models correctly**
- [ ] **Step 4: Commit**
  - Command: `git commit -m "refactor: use useSemanticModels in AIQuery"`

---

### Task 5: Generic `ToggleButtonGroup` Component

**Files:**
- Create: `frontend/src/components/ui/ToggleButtonGroup.tsx`

- [x] **Step 1: Implement component**
  - Props: `options`, `value`, `onChange`, `getLabel`, `ariaLabel`.
- [ ] **Step 2: Replace `LanguageSwitcher.tsx` logic with `ToggleButtonGroup`**
- [ ] **Step 3: Replace `ThemeToggle.tsx` logic with `ToggleButtonGroup`**
- [ ] **Step 4: Commit**
  - Command: `git commit -m "refactor: use ToggleButtonGroup for language and theme switchers"`

---

### Task 6: Final Modal Standardization

**Files:**
- Modify: `frontend/src/pages/Modeling.tsx`
- Modify: `frontend/src/pages/Metadata.tsx`

- [x] **Step 1: Replace custom backdrops in `Modeling.tsx` with `<Modal>`**
- [x] **Step 2: Replace custom backdrops in `Metadata.tsx` with `<Modal>`**
- [ ] **Step 3: Verify modal functionality (close on ESC / backdrop click)**
- [ ] **Step 4: Commit**
  - Command: `git commit -m "refactor: standardize modal usage across pages"`
