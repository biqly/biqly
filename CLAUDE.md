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

## go — language (1.26)

this repo targets go 1.26+. use stdlib error helpers so wrapped errors still match:

### match sentinel / target errors

- use `errors.Is(err, target)` instead of `err == target` — equality fails when `err` is wrapped with `%w`.
- applies to `sql.ErrNoRows`, `io.EOF`, `http.ErrServerClosed`, package sentinels, and other comparable error values.

example:

```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, fmt.Errorf("...")
}
```

### unwrap to a concrete type

- use `errors.AsType[T](err)` instead of `errors.As(err, &target)` — returns `(T, bool)` and avoids pointer-to-interface mistakes.
- keep `errors.As` only when the target type is not fixed at compile time or you are touching code outside a focused migration.

example:

```go
if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
    // use pgErr
}
```

### pointer fields (`new` with expression)

- go 1.26: `new(expr)` allocates a variable of the expression's type, initializes it to `expr`, and returns `*T`.
- use `new(expr)` instead of copying to a local and taking its address — common for optional JSON/API pointer fields.

example:

```go
// before
cf := c.Confidence
tc.Confidence = &cf

// after
tc.Confidence = new(c.Confidence)
```

- `new(T)` still allocates a zero value when you only need a typed nil pointer shell.

### range over integer

- go 1.22+: use `for i := range n` instead of `for i := 0; i < n; i++`.
- applies whenever the loop variable is only used as an index counter.

example:

```go
// before
for i := 0; i < 5; i++ { ... }

// after
for i := range 5 { ... }
```

### benchmark loops

- go 1.24+: use `b.Loop()` in benchmarks instead of manual `for range b.N` or `for i := 0; i < b.N; i++` loops.
- this is the benchmark-specific exception to the general "range over integer" rule.

example:

```go
// before
for range b.N {
    run()
}

// after
for b.Loop() {
    run()
}
```

### min / max built-ins

- go 1.21+: use built-in `min` / `max` instead of simple if-statement clamps or two-value comparisons.
- keep explicit `if` statements when branches have side effects, extra logic, or clearer domain meaning.

example:

```go
// before
if len(encoded) < chunk {
    chunk = len(encoded)
}

// after
chunk = min(chunk, len(encoded))
```

### slice containment check (`slices.Contains`)

- go 1.21+: use `slices.Contains(slice, value)` instead of manual loops or custom helper functions to check if a slice contains a value.
- applies to any comparable slice type.

example:

```go
// before
func contains(values []string, want string) bool {
 for _, v := range values {
  if v == want {
   return true
  }
 }
 return false
}

// after
import "slices"

slices.Contains(values, want)
```

## go — performance rules

when writing or reviewing go code, apply these to minimize performance loss (especially on hot paths):

1. avoid unnecessary allocations; keep values on the stack where possible.
2. minimize `interface{}` / `any` usage on hot paths — prefer concrete types or generics.
3. pre-allocate slice capacity when the size is known (`make([]T, 0, n)`).
4. use `strings.Builder` instead of string concatenation in loops.
5. for json-heavy paths, consider benchmarking alternatives (jsoniter/sonic/easyjson) before adopting.
6. use `sync.Pool` only when measurements justify it — never speculatively.
7. choose map key/value types carefully (small, comparable keys; avoid pointer-heavy values).
8. profile with pprof (cpu + heap) before and after optimizing; don't guess.
9. check escape analysis (`go build -gcflags='-m'`) for hot-path allocations.
10. tune `GOMEMLIMIT` and `GOGC` for deployment environments when memory pressure matters.

these complement, not override, "simplicity first": optimize hot paths with measurements, not speculation.

## lint-enforced coding rules

all code must comply with `make lint-go` and `make lint-frontend` at write-time — do not wait for CI. the full explanatory list of every enabled golangci-lint rule (from `.golangci.yml`) and every ESLint rule (from `frontend/eslint.config.js`) lives in `tasks/lessons.md → Lint-Enforced Coding Rules`. review that section before writing Go or frontend code.

## frontend — react + typescript + vite

commands:

1. run development server: `npm --prefix frontend run dev`
2. build frontend: `npm --prefix frontend run build` (runs tsc and vite build)
3. run frontend tests: `npm --prefix frontend run test` (runs vitest)
4. lint frontend (react/typescript): `make lint-frontend` or `npm --prefix frontend run lint` (eslint)

## local dev & debugging (live-reload)

when running or debugging locally with hot-reload, use these targets — do not
reinvent them. full guide: `docs/agents/local-dev.md`.

- `make dev-up` — single Postgres (hosts `bi_metadata` + `bi_auth` + `bi_mail`,
  user `bi_user`, `localhost:5432`, like k8s) + redis + nats in docker. no app builds.
- `make watch` — runs ALL app services with live-reload in ONE command: api (`:8888`),
  auth (`:8889`), mail (`:8890`); each rebuilds on `.go` save, Ctrl-C stops all. the
  frontend proxies `/api` → `:8888` and `/auth` → `:8889`, so both must run (missing
  auth = vite `ECONNREFUSED /auth/...`).
- `make watch SVC="api auth"` — run only the listed services (space- or comma-separated).
- `make debug-watch [SVC=auth]` — single service under Delve on `:2345` (default api);
  run the rest via plain `watch`. IDE reconnects after each rebuild. attach to `localhost:2345`.
- `make dev-frontend` — Vite dev server (React HMR).
- host-native runs need `.env.dev` (`cp .env.dev.example .env.dev`): it overrides
  DSN hosts to `localhost`. without it you get `no such host: postgres`.
- `make verify-main` — local mirror of the gate `main` CI runs (incl. semgrep
  security) before merging dev → main.

## pre-commit checks (required)

before any `git commit`, run the linters AND tests for the code you changed, and **fix every reported issue before staging or committing**. lint failures are blockers — do not commit with open lint errors, defer fixes to a follow-up commit, or push hoping CI will catch them.

1. **go**: `gofmt -w <touched .go files>` + `make lint-go` (golangci-lint) + `make test-go` (go test -race) + `deadcode -test $(go list ./... | grep -v '/frontend')`
2. **react / frontend**: `make lint-frontend` (eslint) + `make test-frontend` (vitest)
3. **frontend full gate** (same as CI): `make check-frontend` (lint + format:check + knip + test + build)
4. **AI eval** (when touching `internal/ai/eval/`, golden cases, eval handlers, or `cmd/eval-live/`): `make eval-regression` — stub provider, no API key; same gate as `.github/workflows/test.yml`. Do **not** run `make eval-live` before commit (real LLM; nightly workflow only).

or run everything in one command: `make precommit` (= `make format-frontend` + `make lint` + `make test`). `make precommit` does **not** include `make eval-regression`; run that separately when AI eval code changes.

`make lint-go` / `golangci-lint run` scans the whole repo, not only files you edited. if you add or enable linters (e.g. `.golangci.yml`), fix all new findings across the codebase in the same change before commit.

Run `gofmt -w` on every touched `.go` file before linting. Formatting drift is a blocker, even when the code compiles.

Frontend Prettier formatting is auto-fixed by `make precommit` (runs `npm --prefix frontend run format` before lint). You can also run it standalone: `npm --prefix frontend run format`. CI runs `format:check` separately — if you used `make precommit` before commit, it will already be clean.

`deadcode -test` must be scoped through `go list` and exclude `/frontend` because `frontend/node_modules` can contain third-party Go packages. Treat findings as blockers to triage before commit, but do not blindly delete: exported APIs, alternate build tags, reflection/linkname paths, and future integration seams may need an explicit keep decision.

Default cleanup strategy for deadcode results: clean up only genuinely dead internal code under `internal/` that is unused in production and tests. Preserve public SDK APIs under `pkg/` and test-only helpers unless a focused review proves they are obsolete.

do not commit until lint and tests pass cleanly (zero errors) for the stacks you touched — go paths, frontend paths, or both.

styling & coding conventions:

1. use Tailwind CSS utilities for new or substantially changed UI when they keep the code simpler and more consistent. Existing vanilla CSS with BEM naming conventions in `frontend/src/styles/` may remain; avoid broad rewrites unless the task explicitly asks for them.
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

## deployment & infrastructure

See [Configuration Parameters](docs/configuration.md) for a comprehensive list of environment variables and dynamic database overrides.

the app runs in the **`biqly` kubernetes namespace** in the default kubeconfig cluster.

### deploy folder (`deploy/`)

```text
deploy/
├── helm/biqly/            # main helm chart (umbrella)
│   ├── Chart.yaml
│   ├── values.yaml        # base values (namespace: biqly, image registry, gateway)
│   ├── values-dev.yaml    # dev overrides
│   ├── values-prod.yaml   # prod overrides (used by argocd)
│   ├── templates/         # namespace, postgresql (cnp), nats, dragonfly, otel, alertmanager, etc.
│   └── charts/            # sub-charts: api, auth, ai, query, catalog, frontend, mail
└── argocd/                # argocd application, project, and image-updater config
    ├── application.yaml   # deploys deploy/helm/biqly → namespace biqly, values-prod.yaml
    ├── project.yaml
    └── image-updater.yaml # argocd image updater annotation config
```

key points:

- namespace is declared in `values.yaml` (`global.namespace.name: biqly`) and created by the chart.
- argocd syncs from `main` branch, helm path `deploy/helm/biqly`, with `values-prod.yaml`.
- image tags are bumped automatically by argocd image updater (commits like `chore(deploy): bump image tags`).
- to apply changes locally: `helm upgrade --install biqly deploy/helm/biqly -n biqly -f deploy/helm/biqly/values-dev.yaml`
- to inspect the cluster: `kubectl -n biqly get pods`

### ci / github actions (`.github/workflows/`)

| workflow | trigger | purpose |
| --- | --- | --- |
| `ci.yml` | push/pr to `main` | backend (go test + lint + build) + frontend quality gate + docker build & push |
| `test.yml` | push/pr to `main` | go test only (lighter gate, also runs on prs) |
| `build-*.yml` | push/pr to `main` | per-service docker builds (auth, ai, query, catalog, mail, migrate) |
| `semgrep.yml` | push/pr to `main` | sast security scan |

notes:

- `ci.yml` skips when only `deploy/**` changes.
- docker images are pushed to `ghcr.io/biqly/*` and tagged with the git sha.
- golangci-lint version is pinned in `ci.yml` (`v2.12.2`) — match locally with `make lint-go`.

## Agent skills

### Skill discovery

when the user asks for something and the right workflow is unclear, **start with** `.agents/skills/find-skills/SKILL.md` — read and follow it before improvising.

local project skills live under `.agents/skills/<skill-name>/SKILL.md`. workflow:

1. read `find-skills` to map the request to a domain/task
2. scan `.agents/skills/` for a matching local skill (list directories or read each skill's `description` in its frontmatter)
3. if a local skill fits, read its `SKILL.md` and follow it fully before coding or answering
4. if no local skill fits, use `find-skills` to search remotely (`npx skills find [query]`) or install from [skills.sh](https://skills.sh/)

also follow `.agents/skills/using-superpowers/SKILL.md`: invoke any skill that might apply (even ~1% chance) before responding or editing code.

priority: repo-local `.agents/skills/` first, then global/remote skills via `npx skills`.

### Issue tracker

Issues and PRDs live on GitHub (`biqly/biqly`); use `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Five canonical triage roles map 1:1 to GitHub label strings (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: `CONTEXT.md` at repo root + `docs/adr/` for decisions. See `docs/agents/domain.md`.
