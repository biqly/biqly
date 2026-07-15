# Public Dashboard Sharing (Embedded Analytics — Phase 1) — Design

- **Date:** 2026-07-15
- **Status:** Approved (brainstorming session with Baris)
- **Scope:** Phase 1 of the embedded-analytics roadmap: public share links for dashboards, iframe-embeddable.

## Context & Goal

Biqly has no public/anonymous access path today: every `/api/*` route requires a Bearer credential, and iframe embedding is blocked at three layers (Go CSP `frame-ancestors 'none'` in `internal/http/router.go`, `X-Frame-Options: DENY` in `internal/http/middleware/security_headers.go`, and nginx config in `frontend/nginx.conf` / the frontend Helm chart).

Long-term goal is Metabase-style embedded analytics in three phases:

1. **Phase 1 (this spec):** public share links for dashboards — unguessable tokenized URL, iframe-embeddable, read-only.
2. **Phase 2 (future):** signed B2B2C embeds — customer backend mints short-lived signed JWTs with locked parameters (row-level scoping); reuses the shell-less route and public API skeleton from Phase 1.
3. **Phase 3 (future):** React SDK + white-label theming.

Nothing from Phases 2–3 is built now, but Phase 1 must not preclude them.

## Decisions Made

| Question | Decision |
| --- | --- |
| Primary use case | All three (customer-product embeds, internal embeds, public links), phased |
| Phase 1 scope | Public share link, dashboards only (no single-chart shares) |
| Data freshness | Live query execution of stored widget queries + short-TTL Dragonfly cache + rate limiting |
| Governance | Anyone with dashboard edit permission can create/revoke links; workspace admin kill-switch `public_sharing_enabled` (default **off**) |
| Architecture | Existing SPA gains a shell-less public route; catalog + query services gain public endpoints; headers relaxed only on public paths |

## 1. Data Model

New table `dashboard_public_shares` (bi_metadata, catalog-owned migration):

| Column | Notes |
| --- | --- |
| `id` | UUID PK |
| `dashboard_id` | FK → `dashboards`, `ON DELETE CASCADE` |
| `workspace_id` | denormalized from the dashboard for fast scoping |
| `token_hash` | SHA-256 of the share token; plaintext never stored (same posture as PAT, `internal/auth/pat.go`) |
| `created_by` | user ID, audit |
| `created_at` | timestamp |
| `revoked_at` | nullable; soft revoke |
| `expires_at` | nullable; optional expiry |

- **One active share per dashboard**: partial unique index on `dashboard_id WHERE revoked_at IS NULL`.
- "Rotate link" = revoke current + create new.
- Token: 32 bytes crypto-random, base64url-encoded, lives only in the URL.
- Workspace settings gain `public_sharing_enabled` (bool, default **false**) — the admin kill-switch.

## 2. Backend API

### Management endpoints (authenticated, catalog service, require dashboard edit permission)

- `POST /api/dashboards/{id}/public-share` — create or rotate; returns the token exactly once.
- `GET /api/dashboards/{id}/public-share` — share status (exists, created_at, expires_at).
- `DELETE /api/dashboards/{id}/public-share` — revoke.

### Public endpoints (anonymous)

- `GET /api/public/dashboards/{token}` (catalog) — returns a **sanitized** dashboard: name plus each widget's render config (id, type, title, chart_type, config, size). `logical_query` and `saved_query_id` are **never sent to the client**.
- `POST /api/public/dashboards/{token}/widgets/{widgetId}/run` (query) — reads the widget's stored `logical_query` server-side from the dashboard record and executes it. No query input is accepted from the visitor. The query service fetches the widget definition from a catalog `X-Internal-Token` internal endpoint (existing service-to-service pattern).

Rationale: today `DashboardWidgetRenderer.tsx` posts the widget's `logical_query` from the client to `/api/query/run`. The public path must not accept client-supplied queries — execution is strictly of the stored definition.

### Security layers on the public path

- Token validation on **every request**: hash lookup + revoked/expired check + workspace `public_sharing_enabled` check (kill-switch takes effect immediately for existing links).
- Invalid / revoked / disabled / expired all return the **same 404** (no enumeration signal).
- Rate limiting: Dragonfly counter per token+IP (default 60 req/min, configurable).
- Result cache: Dragonfly, key = share+widget, configurable TTL (default 60s) — shields the customer datasource from anonymous traffic.
- Queries run scoped to the dashboard's workspace and its datasource access; existing workspace scoping is re-applied — no privilege expansion for the anonymous principal.
- `X-Robots-Tag: noindex` on public responses; share create/revoke logged to the existing audit trail.

## 3. Header / Ingress Changes

Relaxed **only** on public paths; everything else keeps today's strict headers.

- Go services: on the `/api/public/*` route group, drop `X-Frame-Options` and set CSP `frame-ancestors *` (these are JSON APIs; the actual iframe target is the frontend).
- Nginx (`frontend/nginx.conf` + `deploy/helm/biqly/charts/frontend/config/default.conf`): new `location /public/` block serving the same `index.html` but without `X-Frame-Options` and with `Content-Security-Policy: frame-ancestors *`.
- CORS untouched: the iframe loads our origin, so public API calls are same-origin.

## 4. Frontend

- New route `/public/dashboard/:token` in `App.tsx`, **outside** the app shell and `AuthGuard` (no sidebar/header/breadcrumbs). Renders the dashboard grid plus a small biqly footer badge.
- Targeted refactor of `DashboardWidgetRenderer`: the data-fetch function becomes an injected prop (currently it POSTs to `/api/query/run` itself). The in-app path keeps current behavior; the public page injects the public run endpoint. One render codebase.
- Dashboard toolbar gains a "Public link" modal (following the `ShareButton` pattern): enable/disable toggle, copy link, **copy iframe snippet**, rotate link.
- Workspace admin settings page gains the `public_sharing_enabled` toggle.
- Public page is read-only: no filters, drill-down, or interactions in Phase 1. i18n via `useT()`; accessible (semantic HTML, aria, keyboard).
- Error state: invalid link shows a neutral "This dashboard does not exist or sharing has been disabled" page.

## 5. Testing

- **Go:** share repo CRUD + IDOR tests (following `internal/dashboard/repository_idor_test.go`); handler tests for token validation, kill-switch, expiry; "revoked → uniform 404" test; rate-limit and cache behavior tests; sanitization test asserting `logical_query` never appears in public responses.
- **Frontend:** vitest for the public page (render, error state) and the share modal.
- **Verification:** embed in a real iframe against the dev cluster end-to-end; `make verify-main` (includes semgrep) before merging.

## Out of Scope (Phase 1)

- Signed/JWT embeds, per-embed parameter locking, row-level scoping beyond workspace (Phase 2).
- React SDK, white-label theming (Phase 3).
- Single-chart / saved-question shares.
- Interactive filters or drill-down on the public page.
- Chart image (PNG/PDF) export.
