# Modeling Tools Modal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the modeling canvas sidebars with one shared, tabbed modal and two direct canvas launchers.

**Architecture:** Keep model data and mutations in `useModelingPageState` and `Modeling.tsx`. Introduce a focused `ModelingToolsModal` presentation component, turn the existing palette/editor into modal content, and replace the three-column shell with a full-width canvas shell.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, Vitest, Testing Library, existing `ui/Modal` and i18n system.

---

### Task 1: Lock the modal contract with tests

**Files:**
- Create: `frontend/src/components/modeling/ModelingToolsModal.test.tsx`
- Create: `frontend/src/components/modeling/ModelingToolsModal.tsx`

- [x] **Step 1: Write a failing interaction test**

Render the launcher and modal with Testing Library. Assert that the Semantic layer launcher calls `onOpen('semantic')`, Add relationship calls `onOpen('relationship')`, the dialog exposes two tabs, the active tab has `aria-selected="true"`, only active panel content is present, and clicking the other tab calls `onTabChange`.

- [x] **Step 2: Run the focused test and verify the missing component failure**

Run: `npm --prefix frontend run test -- ModelingToolsModal.test.tsx`

Expected: FAIL because `ModelingToolsModal` does not exist.

- [x] **Step 3: Implement the presentation contract**

Create and export these exact interfaces:

```tsx
export type ModelingToolsTab = 'semantic' | 'relationship'

export function ModelingToolLaunchers(props: {
  tableCount: number
  relationshipCount: number
  onOpen: (tab: ModelingToolsTab) => void
  t: Translate
}): ReactNode

export function ModelingToolsModal(props: {
  activeTab: ModelingToolsTab | null
  onTabChange: (tab: ModelingToolsTab) => void
  onClose: () => void
  semanticContent: ReactNode
  relationshipContent: ReactNode
  t: Translate
}): ReactNode
```

Use the shared `Modal`; give navigation `role="tablist"`, each button `role="tab"`, stable `aria-controls`, and selected content `role="tabpanel"`.

- [x] **Step 4: Run the focused test and verify it passes**

Run: `npm --prefix frontend run test -- ModelingToolsModal.test.tsx`

Expected: PASS.

### Task 2: Move existing tools into the shared modal

**Files:**
- Modify: `frontend/src/components/Modeling.tsx`
- Modify: `frontend/src/components/modeling/ModelingPalette.tsx`
- Modify: `frontend/src/components/modeling/JoinEditor.tsx`
- Modify: `frontend/src/components/modeling/useModelingPageState.ts`

- [x] **Step 1: Replace panel booleans with one page-local modal state**

Add `const [toolsTab, setToolsTab] = useState<ModelingToolsTab | null>(null)` in `Modeling.tsx`. Remove `paletteOpen`, `editorOpen`, their setters/toggles, and `closeMobilePanels` from `useModelingPageState`.

- [x] **Step 2: Make palette and editor content-only**

Remove `open` and `onToggle` from both prop interfaces. Replace collapsible `<aside>` wrappers and rail buttons with semantic content containers while retaining existing headings, forms, actions, and callbacks.

- [x] **Step 3: Render full-width canvas and one modal**

Render `ModelingToolLaunchers` as an absolute canvas overlay, keep `ModelingCanvas` data behavior unchanged, route its relationship action to `setToolsTab('relationship')`, and render one `ModelingToolsModal` with the existing `ModelingPalette` and `JoinEditor` trees as tab content.

- [x] **Step 4: Run typecheck**

Run: `make typecheck-frontend`

Expected: PASS with no stale panel prop or state references.

### Task 3: Replace sidebar styling and localize new chrome

**Files:**
- Modify: `frontend/src/lib/modelingClasses.ts`
- Modify: `frontend/src/i18n/locales/en/core.ts`
- Modify: `frontend/src/i18n/locales/tr/core.ts`

- [x] **Step 1: Replace shell contract**

Change `modelingShellClass` to a full-width, single-cell canvas shell. Add focused class exports for the launcher group, launcher button/icon/meta, modal layout, navigation rail, tabs, and scrollable panel. Remove sidebar/mobile scrim classes without consumers.

- [x] **Step 2: Add English and Turkish copy**

Add keys for modal title/subtitle, semantic and relationship tab descriptions, launcher count labels, and tab aria label. Reuse existing `semantic_layer`, `add_relationship`, `manual_title`, and panel descriptions where wording already fits.

- [x] **Step 3: Format and run static gates**

Run: `npm --prefix frontend run format`

Run: `make typecheck-frontend && make lint-frontend`

Expected: PASS.

### Task 4: Verify behavior and quality

**Files:**
- Modify: `tasks/todo.md`

- [x] **Step 1: Run focused modeling tests**

Run: `npm --prefix frontend run test -- ModelingToolsModal.test.tsx ModelingTableCard.test.tsx TableDetailModal.test.tsx`

Expected: PASS.

- [x] **Step 2: Run exact frontend CI gate**

Run: `make check-frontend`

Expected: PASS for ESLint, Tailwind diagnostics, Prettier, knip, Vitest, TypeScript, and Vite build.

- [x] **Step 3: Check diff scope and whitespace**

Run: `git diff --check`

Run: `git status --short`

Expected: no whitespace errors; only scoped frontend/docs/task files plus the unrelated pre-existing `internal/app/catalog_dependencies.go` change.

- [x] **Step 4: Perform visual QA and record results**

Verify full-width canvas, both direct launchers, both modal tabs, responsive navigation, Escape/close behavior, and the table-card relationship path. Record exact evidence and environment limitations in `tasks/todo.md`.
