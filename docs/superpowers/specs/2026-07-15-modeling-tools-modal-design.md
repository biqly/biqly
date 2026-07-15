# Modeling Tools Modal Design

## Goal

Replace the permanent Semantic Layer and Manual Relationship side panels with one shared, accessible modal so the modeling canvas can use the full available width.

## Approved interaction

- The canvas exposes two compact launchers: **Semantic layer** and **Add relationship**.
- Both launchers open the same modal directly on the relevant tab.
- The modal has a quiet navigation rail on desktop and a two-column tab strip on smaller screens.
- Switching tabs does not close the modal or reset existing semantic-model or relationship-form state.
- A table card's existing add-relationship action opens the same modal on the relationship tab.
- Escape, backdrop click, the close button, focus trapping, and focus restoration come from the shared `Modal` primitive.

## Visual direction

The canvas remains the visual hero. The launchers use the existing indigo accent, a restrained raised-card treatment, concise labels, and model/relationship counts. The modal is deliberately broad, with a pale navigation surface and one scrollable work area; decoration stays secondary to dense modeling data.

## Component boundaries

- `Modeling.tsx` owns a single `semantic | relationship | null` modal state and keeps all existing data/action wiring.
- `ModelingToolsModal.tsx` owns modal chrome, launchers, tab semantics, responsive navigation, and active-content selection.
- `ModelingPalette.tsx` and `JoinEditor.tsx` retain their current business content but no longer render collapsible sidebar controls.
- `useModelingPageState.ts` stops owning viewport-specific panel state.
- `modelingClasses.ts` replaces sidebar-grid classes with full-canvas, launcher, modal, and tab styles.

## Success criteria

- The modeling canvas spans the shell width at all supported breakpoints.
- Either launcher opens one shared modal on the intended tab.
- Only the active tab panel is rendered and its state comes from existing page state.
- The table-card relationship action opens the relationship tab.
- No semantic-model, relationship-save, zoom, pan, or table-card behavior changes.
- Focus and keyboard behavior remain accessible.
- Focused tests and `make check-frontend` pass.
