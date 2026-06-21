# Biqly Frontend UI/UX + Design System + Duplication Audit

> Date: 2026-06-20 · Branch: `dev` · Scope: `frontend/` (React 19 + TS + Vite 8 + Tailwind v4 SPA)
> Audit-only. No source code was changed. Backend/API/SQL/auth-security explicitly out of scope.
> Live QA performed against local stack (api `:8888`, auth `:8889`, mail `:8890`; Vite `:3333`) as **Super Admin**, light+dark × 390/768/1440. Screenshots in `/tmp/biqly-qa/`.

---

## Executive Summary

- **Overall UI/UX: strong.** Consistent page-header pattern (breadcrumb › colored eyebrow + bold title + muted description), consistent card/sidebar system, good empty states with CTAs across all 13 inspected screens. Light and dark both render correctly everywhere checked.
- **Design system: mature core, uneven adoption.** A complete primitive layer exists (`components/ui/*`, `lib/*Classes.ts`, `hooks/*`). The debt is *bypass*, not absence: ~21 screens use raw `<button>`+class helpers instead of `ui/Button`, only 8 files use `DataState`, several render inline error `<div>`s instead of `ErrorAlert`.
- **Tailwind/theme: solid bridge, localized risk.** `@theme inline` + `[data-theme]` + `@custom-variant dark` is correctly wired. Risk is concentrated in a handful of **hardcoded theme-agnostic colors** that break in light mode where they render, and ~643 inline `style={{}}` blocks.
- **Responsive: good.** Zero horizontal overflow at 390px on all 13 screens. Sidebar collapses to a hamburger, Admin sub-nav collapses to a Section dropdown, wide tables scroll internally, Modeling canvas degrades to floating panels. One real issue (AI Query mobile, below).
- **Accessibility: good baseline.** `ui/Modal` has focus trap/Escape/dialog semantics; icon buttons largely labeled; the earlier dark-mode + clickable-div + recursive-remount issues are already fixed (todo.md P0, 2026-06-16). Remaining gaps are in bypass screens.
- **Code duplication: the main opportunity.** `errorMessage()` exists but is bypassed by 51 inline sites; `AIQueueStatus` is genuinely defined twice; 23 hand-rolled `cancelled=false` fetch guards and 22 loading-triples await a `useAsyncState`/`useFetch`; several FW helpers were never created.

### Top 5 riskiest areas
1. **Hardcoded theme-agnostic colors** in light mode (`PromptTemplates`, `QueryHistory`, `Glossary`/`GlossaryEnrichPanel`, `App`) — invisible/low-contrast text where these sub-states render.
2. **`AIQueueStatus` type duplicated** (`types/ai.ts:138` + `types/auth.ts:157`) — drift risk on the async-job contract.
3. **Primitive bypass at scale** (raw buttons, inline error divs, low `DataState` adoption) — inconsistent states + a11y regressions creep in.
4. **No `errorMessage` consolidation** — 51 inline `e instanceof Error ? …` sites produce inconsistent, sometimes technical, error text for Turkish users.
5. **AI Query mobile layout** — empty Conversations panel consumes full screen height, burying the prompt.

### Recommended first 3 PRs
1. **Audit-only** (this report + `todo.md` checklist). *(done by this change)*
2. **Theme/token cleanup** — fix the ~6 hardcoded-color sites + drop `var(--token,#hex)` fallbacks. Small, high-safety, removes light-mode risk.
3. **Error/loading standardization** — promote `errorMessage()` to `utils/error.ts`, adopt `ErrorAlert` + `DataState` in the bypass screens.

---

## Audit Plan

- **Pages inspected (13):** Datasources, Metadata, Modeling, Composites, Query Builder, AI Query, Table Browser, Query History, Glossary, Prompt Templates, Dashboard (AI Analytics), Settings, Admin. Routes discovered live from the nav.
- **Components reviewed:** `components/ui/*`, `lib/*Classes.ts`, `hooks/*`, `utils/*`, `i18n/*`, feature dirs (`aiQuery`, `queryBuilder`, `modeling`, `tableBrowser`, `admin`, `settings`, `auth`).
- **Viewports:** 390×844, 768×1024 capability, 1440×900 — light + dark.
- **Code areas scanned:** grep verification of cited file:line references; FW-1..FW-13 existence checks.
- **Commands run:** `npm --prefix frontend run build` (✓ 962ms), grep audits, live `/browse` QA (SPA nav + `data-theme` toggle + `screenshot`), per-screen `scrollWidth-clientWidth` overflow check.
- **Not inspected (limitation):** data-heavy states — populated tables, charts, Row Modal drilldown, generation trace panel, clarification cards, async job tray — the test workspace has one datasource with **no synced metadata/models**, so these render only as empty states. Recommend a follow-up QA pass on a seeded workspace.

---

## Current Design System Map

| Primitive | File | Usage | Status | Bypassed at | Recommendation |
|---|---|---|---|---|---|
| Button | `ui/Button.tsx` | variants/size/autoWidth | **Good** | ~21 files (`qbAddBtnClass`, `adminBtnPrimaryClass`, `authSubmitBtnClass`, …) | Migrate raw buttons; keep class helpers only for truly bespoke controls |
| Modal | `ui/Modal.tsx` | focus trap + Escape + aria | **Good** | (legacy modals already migrated, 2026-06-16) | — |
| ConfirmDialog | `ui/ConfirmDialog.tsx` | danger/warning/default | **Good** | — | — |
| DataState | `ui/DataState.tsx` | loading/error/empty wrapper | **Partial** | only 8 files adopt | Adopt in AIProviders, RLS, FieldPermission, PII, Roles, ExpressionBuilder |
| DataTable | `ui/DataTable.tsx` | generic `<T>` table | **Good** | — | — |
| EmptyState | `ui/EmptyState.tsx` | title/desc/icon/action | **Good** | — | — |
| ErrorAlert | `ui/ErrorAlert.tsx` | `error` string | **Partial** | inline error `<div>` in ≥5 admin/modeling files | Replace inline error divs |
| LoadingOverlay | `ui/LoadingOverlay.tsx` | overlay spinner | **Good** | — | — |
| Toast | `ui/Toast.tsx` + `useToast` | success/error/info/warning | **Good** | — | — |
| Select / MultiSelect | `ui/Select.tsx`, `ui/MultiSelect.tsx` | searchable, popover/inline | **Good** | — | — |
| Pagination | `ui/Pagination.tsx` + `PaginationControls` | server-driven | **Good** | — | — |
| FormField | `ui/FormField.tsx` | label/error/required | **Good** | auth forms hand-roll some fields | Optional consolidation |
| TagBadge | `ui/TagBadge.tsx` | tone variants | **Good** | — | StatusBadge folded in via `tone` — acceptable |
| KPICard | `ui/KPICard.tsx` | label/value/color | **Good** | — | — |
| Card | `lib/cardClasses.ts` (`cardClass()`) | class-only, no React comp | **Partial** | — | Fine as-is; a thin `Card` wrapper would reduce header/title repetition |
| cn() | `lib/cn.ts` | clsx + tailwind-merge | **Good** | ~43 template-literal/`clsx`-only sites | Prefer `cn()` for conditional classes |
| useApi / useAdminApi | `hooks/useApi.ts` | get/post/put/patch/delete + loading/error/abort | **Good** | — | — |
| useConfirm | `hooks/useConfirm.tsx` | promise-based confirm | **Good** | — | — |
| usePaginatedList | `hooks/usePaginatedList.ts` | AbortController, URL sync | **Good** | — | Reference pattern for `useAsyncState` |
| useToast / useT / useLocale | `hooks/`, `i18n/` | toast + i18n | **Good** | — | — |
| formatters | `utils/formatters.ts` | `formatDateTime`/`formatDateOnly`/`formatDurationMs` | **Good** (FW-10 ✓) | 26+ inline `toLocale*` sites | Adopt at call sites |
| errorMessage | `hooks/usePaginatedListLogic.ts:44` | `(e)=>string` | **Misplaced** | 51 inline sites bypass it | Move to `utils/error.ts`, adopt |
| useAsyncState / useModal / useFetch / useApiResource / buildQueryString / apiConstants | — | — | **Missing** | n/a | Create per FW-2/5/6/7/8/12 |

---

## UI/UX Findings

**UX-1 · Medium · AI Query · `/ai-query` (mobile)**
On 390px the empty **Conversations** panel stacks at full screen height above the chat, so the prompt input and example chips are pushed far below the fold; the hamburger (☰) overlaps the "CONVERSATIONS" header.
- *User impact:* mobile users scroll a blank panel before they can ask anything.
- *Expected:* on mobile, collapse Conversations to a togglable drawer/accordion above a visible chat, or cap its height.
- *Evidence:* `/tmp/biqly-qa/ai-query-mobile-light.png`.
- *Fix:* responsive layout — `lg:grid-cols-[sidebar+chat]`, on `<lg` make Conversations a collapsible section with `max-h`.
- *Regression test:* render `AIQuery` at 390px, assert chat input is within the first viewport height.

**UX-2 · Low · Prompt Templates · `/prompt-templates`**
Page is a single large monospace text block; for very long templates the only structure is "Section" select + character count. Reads as a wall of text.
- *Fix (polish):* monospace line numbers or collapse-by-section; not blocking.

**UX-3 · Info · All data screens**
Could only be exercised in empty state (no synced metadata/models). Empty states are good (clear copy + CTA). Populated tables, charts, Row Modal, generation trace, clarification cards, async job tray need a seeded-workspace QA pass.

Positive: page-header consistency, example-chip empty state on AI Query, Admin's grouped sub-nav, and "Local API · localhost:8888" status indicator are all well done. Light/dark parity is clean on every inspected screen.

---

## Design System Findings

**DS-1 · Medium · Buttons**
Expected: `ui/Button` with variant/size. Actual: ~21 files render raw `<button>` + a bespoke class helper (`qbAddBtnClass` in 6 queryBuilder steps; `adminBtnPrimaryClass`/`adminBtnGhostClass` in admin panels; `authSubmitBtnClass`/`authLinkBtnClass` in auth pages; `rowModalBackClass`).
- *Shared primitive exists:* yes (`ui/Button.tsx`).
- *Refactor:* migrate to `<Button variant=… size=…>`; retain helpers only for genuinely unique controls (e.g., expr-mode toggles).
- *Priority:* P1.

**DS-2 · Low-Med · Primary CTA color inconsistency**
Admin "Invite User" renders **green/emerald** while every other primary CTA ("+ New Dashboard", "+ New datasource", "+ New") is **indigo/violet** (`text-accent`).
- *Evidence:* `/tmp/biqly-qa/admin-desktop-dark.png`, `admin-mobile-light.png`.
- *Fix:* use the standard accent for the primary action (or define a documented semantic for green = "create user"); align across panels.
- *Priority:* P1.

**DS-3 · Medium · ErrorAlert / DataState adoption**
Inline error `<div>`s in ≥5 files (`AIProvidersPanel`, `AIJobsAdminPanel`, `ExpressionBuilder`, `FieldPermissionPanel`, `PIIDetectionPanel`) and only 8 files use `DataState`.
- *Refactor:* replace inline error markup with `ErrorAlert`; wrap list/table panels in `DataState`.
- *Priority:* P1.

**DS-4 · Low · Card header pattern**
`Card` is class-only (`cardClass()`); header/title/subtitle markup repeats. A thin `Card`/`CardHeader` wrapper would reduce repetition. Optional, P2.

---

## Tailwind / Theme Findings

**TW-1 · Medium · Hardcoded theme-agnostic colors (light-mode risk)**
| File:line | Pattern | Risk |
|---|---|---|
| `PromptTemplates.tsx:115,118` | `color: 'rgba(255,255,255,0.35)'` on `{{`/`}}` markers | white text → invisible on light card |
| `PromptTemplates.tsx:116` | `color: '#f43f5e'` keyword highlight | fixed pink, not token |
| `GlossaryEnrichPanel.tsx:100,116` | `border-(--danger,#d9534f)`, `text-(--danger,#d9534f)` | hardcoded fallback hex |
| `QueryHistory.tsx:259,268…331` | `var(--table-header-bg,#f9fafb)`, `var(--table-header-fg,#4b5563)` ×9 | fallback hex; bites when rows render |
| `App.tsx:498` | `hover:bg-(--bg-hover,#f3f4f6)` | fallback hex |
- *Type:* Hardcoded Color / Token Bypass.
- *Light/dark risk:* yes — these are theme-agnostic and only "happen to work" in one theme.
- *Token/helper:* `text-foreground-faint`, `text-error`, `border-error`, `bg-card`/table-header tokens, `hover:bg-canvas-subtle`.
- *Migration:* replace literals with tokens; remove `var(--token,#hex)` fallbacks (the token is always defined).
- *Accept as-is:* chart palette hex in `utils/constants.ts` + Recharts `fill=` props (data-viz, not UI surfaces).
- *Priority:* P0 (small, removes a real light-mode defect class).

**TW-2 · Medium · Inline `style={{}}` blocks**
643 inline-style blocks across ~75 files (heaviest: `DashboardBuilder`, `AIUsageDashboard`, `QueryHistory`, admin panels, modeling). ~40–50% migratable to utilities/class helpers; the rest are legitimately dynamic (computed widths, gradients).
- *Priority:* P2 (incremental, per-file).

**TW-3 · Low · Magic arbitrary values**
Repeated `text-[0.8125rem]` (token `text-caption` already exists), `gap-[0.65rem]`, `p-[0.35rem]`, `rounded-[0.3rem]`, arbitrary `font-['…']`. Add 3–4 spacing/radius tokens to `@theme` and adopt `text-caption`.
- *Priority:* P2.

**TW-4 · Low · `cn()` bypass**
~43 sites use template-literal conditional classNames or bare `clsx` without `twMerge`, risking unresolved Tailwind conflicts. Prefer `cn()`. P2.

**TW-5 · Info · index.css debt**
~789 lines; ~365 structurally irreducible (theme vars, `@theme` bridge, pseudo-element select/checkbox/scrollbar/range, canvas grid, keyframes, auth gradients). ~150 lines are legacy BEM `.btn`/`.card` (~170 call sites) — deprecate gradually as DS-1 progresses. No dead-selector blocker found.

---

## Responsive Findings

**RES-1 · Medium · AI Query mobile** — see UX-1. Empty Conversations panel full-height; hamburger overlap.

**RES-2 · Pass · Admin mobile** — left sub-nav collapses to a "Section" dropdown; DataTable scrolls horizontally inside its container (Status column off-screen but reachable). Good pattern.

**RES-3 · Pass · Modeling mobile** — side panels collapse to floating "Tables"/"Joins" buttons; canvas + zoom controls usable. Acceptable for a desktop-oriented tool.

**RES-4 · Pass · Table Browser / Datasources / Metadata / Glossary / Query Builder / Dashboard mobile** — clean stacking, hamburger sidebar, stacked selectors, no overflow.

**RES-5 · Pass · Global** — `scrollWidth - clientWidth = 0` at 390px on all 13 screens (no horizontal overflow).

---

## Accessibility Findings

**A11Y-1 · Low · AI Query mobile focus order** — "Skip to content" appears (good) but the full-height empty panel means keyboard/screen-reader users traverse an empty region before the prompt. Fix with UX-1.

**A11Y-2 · Low · Color-only signaling (verify with data)** — status uses `TagBadge` tones (Active/Unverified/2FA off) with text labels — good. Re-verify chart legends (color-only) once a populated dashboard exists.

**A11Y-3 · Info · Inherited-good** — `ui/Modal` focus trap/Escape/aria, labeled icon buttons, and the 2026-06-16 fixes (dark-mode binding, clickable-div→button, recursive remount) are in place. Remaining a11y gaps live in the DS-1/DS-3 bypass screens (raw buttons / inline error divs lacking `role="alert"`).

---

## Code Duplication Findings

**DUP-1 · Type · `AIQueueStatus` defined twice** — `types/ai.ts:138` and `types/auth.ts:157`. Drift risk on the async-job wire contract. Keep one (`types/ai.ts`), re-export from the other. **P1.** (FW-9)

**DUP-2 · Helper · `errorMessage` not consolidated** — exported from `hooks/usePaginatedListLogic.ts:44` but **51** inline `e instanceof Error ? e.message : String(e)` sites bypass it. Move to `utils/error.ts`, import everywhere. **P1.** (FW-1)

**DUP-3 · Hook · async fetch state** — 22 `loading`-triples + **23** hand-rolled `let cancelled = false` effect guards vs the AbortController pattern in `usePaginatedList`. Extract `useAsyncState`/`useFetch`. **P1.** (FW-2/FW-8)

**DUP-4 · Helper · date/number formatting** — `formatters.ts` exists (FW-10 ✓) but 26+ inline `toLocale*` calls remain. Adopt at call sites. **P2.** (FW-10 adoption)

**DUP-5 · Helper · query string + API paths** — 7 inline `new URLSearchParams` sites; no central API-path constants. Create `buildQueryString` + `apiConstants`. **P2.** (FW-6/FW-5)

**DUP-6 · Hook · modal + confirm-mutation** — repeated `[xOpen,setXOpen]`/`[editing,setEditing]` and `confirm→try/catch→toast` blocks. Extract `useModal` + `useConfirmedMutation` (builds on existing `useConfirm`). **P2.** (FW-12/FW-13)

**DUP-7 · Style · inline styles + admin panel styles** — see TW-2; admin panels repeat inline layout styles (FW-3 partial). **P2.**

---

## Proposed Shared Abstractions

> Verify-first: `errorMessage` and `formatDate` already exist — finish placement/adoption, don't re-create. `useConfirm` exists — `useConfirmedMutation` wraps it.

- **`errorMessage(e: unknown): string`** — *purpose:* one safe error-string extractor. *Replaces:* 51 inline sites. *Move from* `hooks/usePaginatedListLogic.ts:44` *to* `utils/error.ts`, re-export. *Tests:* Error, string, object, undefined.
- **`useAsyncState<T>()`** → `{ data, loading, error, run, reset }` with AbortController. *Replaces:* 22 loading-triples + 23 `cancelled` guards. *Model on* `usePaginatedList`. *Tests:* resolve, reject→errorMessage, unmount-abort.
- **`useFetch<T>(fetcher, deps)`** — declarative wrapper over `useAsyncState` for read-on-mount screens. (FW-2/FW-7 unify into these two.)
- **`useModal<T>()`** → `{ open, openWith(data), close, data }`. *Replaces:* modal open/editing pairs.
- **`useConfirmedMutation(fn, opts)`** — confirm → try/catch → toast.success/error(errorMessage). *Builds on* `useConfirm` + `useToast`.
- **`buildQueryString(params)`** + **`apiConstants.ts`** — central URL/query construction.
- **`StatusBadge`** — thin wrapper over `TagBadge` mapping domain status→tone+label (single source for status colors).
- **`AdminPanelShell` / `AdminFormSection`** — standard admin layout + form section (removes inline admin styles; FW-3).
- **`Card` / `CardHeader`** — optional thin wrapper over `cardClass()` to dedupe header/title/subtitle.
- *AI Query:* keep `ClarificationCard`/`GenerationTracePanel`/`AIJobTracker` as-is; only fix the mobile layout (UX-1).

---

## Prioritized Action Plan

### P0 — light-mode correctness (small, safe)
- **A1 · TW-1:** replace ~6 hardcoded theme-agnostic colors with tokens; drop `var(--token,#hex)` fallbacks. Files: `PromptTemplates.tsx`, `QueryHistory.tsx`, `GlossaryEnrichPanel.tsx`, `Glossary.tsx`, `App.tsx`. Risk: low. Verify: light-theme screenshot of each. PR ≤ ~40 lines.

### P1 — standardization (high value)
- **A2 · DUP-1/FW-9:** dedupe `AIQueueStatus` to `types/ai.ts`. Risk: low. Verify: `tsc`.
- **A3 · DUP-2/FW-1:** `errorMessage` → `utils/error.ts`; adopt at 51 sites (mechanical). Verify: build + lint.
- **A4 · DS-1:** raw `<button>` → `ui/Button` (queryBuilder/admin/auth). Risk: visual; verify screenshots.
- **A5 · DS-3:** inline error divs → `ErrorAlert`; adopt `DataState` in list panels.
- **A6 · DS-2:** unify primary CTA color (Invite User).
- **A7 · UX-1/RES-1:** AI Query mobile Conversations layout.

### P2 — refactor / polish
- **A8 · DUP-3/FW-2/FW-8:** `useAsyncState`/`useFetch`; migrate fetch screens.
- **A9 · DUP-5/6, FW-5/6/12/13:** `buildQueryString`, `apiConstants`, `useModal`, `useConfirmedMutation`.
- **A10 · TW-2/TW-3/TW-4:** inline-style→utility, magic-value tokens, `cn()` adoption.
- **A11 · FW-3:** `AdminPanelShell`/`AdminFormSection`.
- **A12 · TW-5:** deprecate BEM `.btn`/`.card` as DS-1 lands.
- **A13 · DUP-4/FW-10:** adopt `formatters` at 26+ inline `toLocale*` sites.

---

## Suggested PR Breakdown
1. **Audit-only** — this report + `todo.md` checklist. *(this change)*
2. **Theme/token cleanup** — A1.
3. **Error/loading/empty standardization** — A3 + A5.
4. **Type + primitive bypass** — A2 + A4 + A6.
5. **AI Query mobile** — A7.
6. **Hook/helper duplication** — A8 + A9.
7. **Admin standardization** — A11 + admin inline-style removal.
8. **Polish** — A10 + A12 + A13.

---

## Verification Commands
```bash
make check-frontend                      # full CI gate (eslint+tailwind+format+knip+test+build/tsc)
npm --prefix frontend run test           # vitest
npm --prefix frontend run build          # tsc --noEmit + vite build  (baseline: ✓ 962ms)
npm --prefix frontend run lint
npm --prefix frontend run format:check
git diff --check
```
Manual visual QA (per PR touching UI): light + dark screenshot of each changed screen at 390 / 768 / 1440 via the `/browse` skill; re-run the seeded-workspace pass for data-heavy states (UX-3).
