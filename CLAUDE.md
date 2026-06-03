# claude.md

## user-global rules

these rules apply in every project and override any default skill template or conflicting project-local rule.

1. don't assume. don't hide confusion. surface tradeoffs.
2. minimum code that solves the problem. nothing speculative.
3. touch only what you must. clean up only your own mess.
4. define success criteria. loop until verified.
5. when something is asked, answer compactly unless a detailed answer is explicitly requested.

## gograph — go repository intelligence
<!-- gograph-start: do not remove -->
rules (enforced when gograph mcp server is connected):

1. never use grep, rg, find, or glob to search for go symbols, functions, structs, or types. use gograph_query instead.
2. before editing any go symbol, run gograph_plan `<symbol>` to understand callers, tests, and risk.
3. after editing go code, run gograph_review --uncommitted to verify test coverage and blast radius.
4. to understand a function, use gograph_context `<symbol>` (replaces 4–5 separate tool calls).
5. run gograph_capabilities at the start of any go coding session.
<!-- gograph-end: do not remove -->

## frontend — react + typescript + vite

commands:

1. run development server: `npm --prefix frontend run dev`
2. build frontend: `npm --prefix frontend run build` (runs tsc and vite build)
3. run frontend tests: `npm --prefix frontend run test` (runs vitest)

styling & coding conventions:

1. use vanilla css with bem naming conventions (located in `frontend/src/styles/`). avoid tailwind css.
2. React 19 + TypeScript + Vite 6: components import/use class names as plain strings.
3. use clean react hooks and functional components.
4. ensure all interactive components are fully accessible (semantic html, proper `aria-*` tags, keyboard navigation, unique ids).
5. translate text using `useT()` hook for i18n support.

## workflow orchestration

### plan mode default

- enter plan mode for any non-trivial task (3+ steps or architectural decisions).
- if something goes sideways, stop and re-plan immediately — don't keep pushing.
- use plan mode for verification steps, not just building.
- write detailed specs upfront to reduce ambiguity.

### subagent strategy

- use subagents liberally to keep the main context window clean.
- offload research, exploration, and parallel analysis to subagents.
- for complex problems, throw more compute at it via subagents.
- one task per subagent for focused execution.

### self-improvement loop

- after any correction from the user: update tasks/lessons.md with the pattern.
- write rules that prevent the same mistake from recurring.
- ruthlessly iterate on lessons until mistake rate drops.
- review tasks/lessons.md at session start for relevant context.

### verification before done

- never mark a task complete without proving it works.
- diff behavior between main and your changes when relevant.
- ask: "would a staff engineer approve this?"
- run tests, check logs, demonstrate correctness.

### demand elegance (balanced)

- for non-trivial changes: pause and ask "is there a more elegant way?"
- if a fix feels hacky: "knowing everything i know now, implement the elegant solution."
- skip this for simple, obvious fixes — don't over-engineer.
- challenge your own work before presenting it.

### autonomous bug fixing

- when given a bug report: just fix it. don't ask for hand-holding.
- point at logs, errors, failing tests — then resolve them.
- zero context switching required from the user.
- fix failing ci tests without being told how.

## task management

1. **plan first**: write plan to tasks/todo.md with checkable items.
2. **verify plan**: check in before starting implementation.
3. **track progress**: mark items complete as you go.
4. **explain changes**: high-level summary at each step.
5. **document results**: add a review section to tasks/todo.md.
6. **capture lessons**: update tasks/lessons.md after corrections.

## core principles

- **simplicity first**: make every change as simple as possible. minimal code impact.
- **no laziness**: find root causes. no temporary fixes. senior developer standards.
- **minimal impact**: touch only what is necessary. avoid introducing bugs.
