# Modeling Component Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the Phase 3 modeling palette and join editor out of `Modeling.tsx` while preserving behavior.

**Architecture:** Keep API orchestration, state, and mutations in `Modeling.tsx`. Extract controlled presentational components and a small pure entity filtering helper under `components/modeling`.

**Tech Stack:** React 19, TypeScript 5.7, Vitest, Vite 6

---

### Task 1: Extract entity filtering

**Files:**
- Create: `frontend/src/components/modeling/entityActions.ts`
- Create: `frontend/src/components/modeling/entityActions.test.ts`
- Modify: `frontend/src/components/Modeling.tsx`

- [x] Write a failing Vitest test for active and inactive entity filtering.
- [ ] Run `npm --prefix frontend test -- entityActions.test.ts` and confirm the missing-module failure.
- [x] Add `activeEntities` and `inactiveEntities` pure helpers and replace inline filters.
- [x] Run `npm --prefix frontend test -- entityActions.test.ts`.

### Task 2: Extract the palette

**Files:**
- Create: `frontend/src/components/modeling/ModelingPalette.tsx`
- Modify: `frontend/src/components/Modeling.tsx`

- [x] Move the left-side summary, tabs, schema list, table list, relationship list, dimension list, and metric list into `ModelingPalette`.
- [x] Pass derived values and callbacks from `Modeling.tsx`; keep mutations in the parent.
- [x] Run `npm --prefix frontend run build`.

### Task 3: Extract the join editor

**Files:**
- Create: `frontend/src/components/modeling/JoinEditor.tsx`
- Modify: `frontend/src/components/Modeling.tsx`

- [x] Move the right-side controlled relationship form into `JoinEditor`.
- [x] Pass select options, controlled values, update callback, and save callback from `Modeling.tsx`.
- [x] Run `npm --prefix frontend run build`.

### Task 4: Verify and record completion

**Files:**
- Modify: `frontend/TODO.md`

- [x] Run `npm --prefix frontend test`.
- [x] Run `npm --prefix frontend run build`.
- [ ] Inspect the modeling page in the browser.
- [ ] Mark the completed `Modeling.tsx` Phase 3 item in `frontend/TODO.md`.
