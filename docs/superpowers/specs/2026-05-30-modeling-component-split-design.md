# Modeling Component Split Design

## Scope

Split the first Phase 3 frontend refactor batch out of `frontend/src/components/Modeling.tsx`
without changing user-visible behavior, API contracts, or styling.

## Components

### ModelingPalette

Move the left semantic-model summary panel into
`frontend/src/components/modeling/ModelingPalette.tsx`. The parent keeps state and
server mutations. The palette receives display data and callbacks for schema, table,
join, dimension, and metric actions.

### JoinEditor

Move the right manual-relationship editor into
`frontend/src/components/modeling/JoinEditor.tsx`. The parent keeps join form state,
validation, and save behavior. The editor renders controlled fields and forwards
field updates and save requests.

### Entity Action Helpers

Move repeated active/inactive filtering into
`frontend/src/components/modeling/entityActions.ts`. Keep network mutations in the
page component so API orchestration remains easy to trace.

## Data Flow

`Modeling.tsx` remains the orchestration component. It loads metadata and models,
derives canvas state, owns dialogs and API calls, then passes immutable values and
event callbacks into the extracted presentational components.

## Error Handling

Existing `useApi`, `useConfirm`, and message handling remain unchanged. Extracted
components do not issue requests and do not introduce new error states.

## Verification

Add focused tests for the extracted entity filtering helper. Run the frontend Vitest
suite and production build. Inspect the modeling page in the browser after the build.

