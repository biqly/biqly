# Git Hooks

This directory contains Git hooks that run automatically before/after certain
Git actions.

## Activation

To enable the hooks, configure Git to use this directory:

```sh
git config core.hooksPath .githooks
```

Or use the Makefile target:

```sh
make setup-githooks
```

## Available hooks

**`pre-commit`** runs before every `git commit`, in order:

1. `make format-frontend` then `git add -u` (stage the formatting changes)
2. `make lint-go` and `make test-go` (Go gate)
3. `make check-frontend` (frontend CI gate)

`check-frontend` is the **exact** `npm run check` gate CI runs (lint + tailwind +
format:check + knip + test + `build`, where `build` runs `tsc --noEmit`). Running
it here means type errors, unused code (knip), and format drift fail locally
instead of in CI. The hook automatically stages formatting changes so they're
included in the commit — no leftover unstaged diffs.

For a faster type-only check during development, run `make typecheck-frontend`.

## Skipping

If the pre-commit hook blocks a legitimate commit (e.g., WIP, partial fix):

```sh
git commit --no-verify   # skip hooks for this commit only
```

This should be rare — the intent is to catch CI regressions before they reach
the shared branch.
