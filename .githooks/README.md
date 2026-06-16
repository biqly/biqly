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

| Hook | What it runs | When |
|------|-------------|------|
| `pre-commit` | `make format-frontend` → `git add -u` → `make lint` → `make test` | Before every `git commit` |

The hook automatically stages formatting changes so they're included in the
commit — no leftover unstaged diffs.

## Skipping

If the pre-commit hook blocks a legitimate commit (e.g., WIP, partial fix):

```sh
git commit --no-verify   # skip hooks for this commit only
```

This should be rare — the intent is to catch CI regressions before they reach
the shared branch.
