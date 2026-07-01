# Local development & debugging

How to run and debug biqly locally with live-reload. Read this before starting
a debug session or wiring up a hot-reload loop — the targets below already exist;
do not reinvent them.

## Topology

Local dev keeps its own **single Postgres container** (`postgres`) hosting all
biqly system databases — `bi_metadata`, `bi_auth`, `bi_mail` — owned by role
`bi_user` (password `bi_password`) on `localhost:5432`. Production now points
those databases at the shared `prag-postgresql` instance in namespace
`postgresql`, but local dev stays self-contained. There is no separate
`auth-db`/`mail-db` container. `test-datasource` (AdventureWorks,
`localhost:5433`) is a *separate* instance on purpose — it models an external
customer datasource, not a system DB.

On **first init** (empty data dir) the Postgres container creates the three
databases and applies their schemas via psql, mirroring how `bi_metadata` is
seeded — no golang-migrate / `schema_migrations` tracking is used locally:
`init-create-databases.sh` (creates `bi_auth` + `bi_mail`) →
`init-metadata-db.sh` (migrates `bi_metadata`) →
`init-service-dbs.sh` (migrates `bi_auth` + `bi_mail`). So a fresh `make dev-up`
yields all three databases fully migrated. To re-run init after schema changes,
recreate the volume: `docker compose down -v && make dev-up`. The DBs start with
no users — **Sign Up** in the UI to create the first account.

| Service | Container host | Host-native port |
|---|---|---|
| Postgres (bi_metadata / bi_auth / bi_mail) | `postgres:5432` | `localhost:5432` |
| redis (dragonfly) | `redis:6379` | `localhost:6379` |
| nats | `nats:4222` | `localhost:4222` |
| test-datasource | `test-datasource:5432` | `localhost:5433` |

## The dev loop (host-native + live-reload)

Run the Go services on the host (so they rebuild on save) against Dockerized
infra. The frontend dev server proxies `/api` → `:8888` (the api) and
`/api/auth` → `:8889` (the auth service), so both backends must run — otherwise
the browser/console shows `vite http proxy error … ECONNREFUSED /api/auth/...`.

`make watch` runs **all** app services at once (one `air` instance each, own
`tmp/<svc>` dir); Ctrl-C stops them all. Two terminals are enough:

```sh
make dev-up        # Postgres + redis + nats in Docker (no app image builds)
make watch         # ALL services: api :8888, auth :8889, mail :8890 — rebuild on save
make dev-frontend  # Vite dev server — React edits hot-reload in the browser
```

Run a subset with `SVC` (space- or comma-separated `cmd/<svc>` names):

```sh
make watch SVC="api auth"   # only api + auth (skip mail)
```

The default set is `WATCH_SVCS = api auth mail` (catalog/query/ai are embedded in
`cmd/api` locally). The Makefile sources `.env` + `.env.dev` and passes the
build/run commands to `air`, so each binary gets the localhost DSNs. Build output
goes to `tmp/<svc>/` (gitignored). Auth auto-generates an in-memory dev JWT key
when none is configured (a WARN line, not an error).

For breakpoint debugging while keeping live-reload, swap `watch` → `debug-watch`:

```sh
make debug-watch            # api under Delve on :2345; rebuilds on save
make debug-watch SVC=auth   # auth under Delve on :2345
```

Attach the IDE to `localhost:2345` (Delve, `--accept-multiclient --continue`).
After each rebuild Delve restarts, so the IDE **reconnects** automatically — keep
the launch config set to reconnect. Debug one service at a time (single `:2345`);
run the others via plain `watch`. `make debug-catalog/-query/-ai` exist for
one-shot Delve without live-reload.

## Host-native env: `.env.dev`

Host-native targets source `.env` first, then **`.env.dev`** (gitignored) if
present. `.env` carries the in-cluster hostnames (`postgres`, `redis`, `nats`)
that `make docker-up` needs; `.env.dev` overrides DSN hosts to `localhost`. One-time:

```sh
cp .env.dev.example .env.dev
```

Without `.env.dev`, host-native runs try to resolve `postgres`/`redis` (Docker
DNS names) and fail with `no such host`. The compose stack itself sets DB DSNs
inline, so `make docker-up` does not depend on `.env` DB hostnames.

## Full containerized stack

`make docker-up` builds and runs everything (api, auth, mail, frontend) in
Docker, plus seeds AdventureWorks. Heavier; use for end-to-end checks, not the
edit loop. `make docker-down` tears it down (`-v` drops volumes).

## Before merging to main

Branch pushes trigger **no** CI (workflows fire only on `push → main` /
`PR → main`). Develop on a branch, merge to `main` by direct push; the full
suite incl. security (semgrep, codeql) runs on `main`. To confirm green first:

```sh
make verify-main   # local mirror of main CI: vet, lint, test, coverage,
                   # eval-regression, govulncheck, frontend check, helm, semgrep
```

CodeQL is GitHub-hosted and cannot run locally; `semgrep-scan` + `govulncheck`
cover local SAST + dependency-vuln checks.
