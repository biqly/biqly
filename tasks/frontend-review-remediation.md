# Frontend Security/Correctness Review — Remediation

Full frontend scan (2026-07-03), 5 parallel reviewers + manual verification of every P0/P1.
Verified with `make check-frontend` (eslint + tailwind + format:check + knip + vitest + tsc build) — **all green**.

Note: no XSS surface anywhere (`dangerouslySetInnerHTML`/`innerHTML`/`javascript:` absent repo-wide;
DB cell values render as React text). Access token is in-memory only. CSRF + refresh are single-flighted.

## Fixed

### P0
- **#1 Cross-account conversation leak** — `clearAuth()` now wipes `biqly_conversations` on logout
  (`AuthProvider.tsx`, via new `clearStoredConversations()` in `useConversation.ts`), and
  `loadConversationSnapshot` no longer re-uploads local-only conversations under the current token.
  Closes both the read-leak (B sees A's conversations) and write-leak (A's content saved under B).

### P1
- **#2 Stale permissions after workspace switch** — `setActiveWorkspace` now `await loadPermissions(newToken)`.
- **#3 AI job polling** — `pollJob` tolerates up to `MAX_POLL_FAILURES` consecutive transient
  errors (status 0 / 429 / ≥500), resetting on success; only terminates on real 4xx / 404 / terminal
  state. Counter cleaned up in `stopPolling`.
- **#4 Metadata table-switch race** — `toggleTable` guards the columns fetch with a request-id ref.
- **#5 Model-detail response race** — `useModelDetail` / `useSemanticModels` drop stale responses via a request-id ref.
- **#6 Table-browser rows keyboard-inaccessible** — rows in `TableBrowserModelContent` and
  `TableBrowserRowModal` now `role="button"` + `tabIndex` + Enter/Space handler + aria-label
  (new i18n key `table_browser.open_row_details`, en/tr).

### P2
- Modal body-scroll lock — `ui/Modal.tsx` locks `document.body` overflow while open (restores on close).
- `usePasskeyRegistration` — guards `window.PublicKeyCredential` existence before dereferencing.
- Mention autocomplete keyboard index — `flatEntries` now follows grouped DOM order.
- Canvas drag stuck — `useModelingCanvas` ends drag/pan on `window` blur and force-ends on unmount.
- Invite modal stale state — `UserListPage` conditionally mounts `InviteUserModal` so it remounts per open.
- TimeGrains error message — uses `request()`'s own `{error}` instead of the stale hook `error` state.

### P3
- "Remember me" — removed the decorative, unwired checkbox from `SignInCredentialsForm`.

## Not changed (informational, from reviewers)
- `i18n/locale.ts` + `i18n/hooks.ts` duplicate `i18n/index.tsx` state (dead/duplicate, no runtime import) — consolidation candidate.
- `useApi.abort()` only aborts the latest controller; no live call site uses it.
- `AuthProvider` context value not memoized (stylistic; provider re-renders are state-driven).
- `useApi`/`apiClient` `useAdminKey` option is dead (legacy admin-key path removed); stale comments in `aiProviders.ts`/`aiAdmin.ts`.
- `bulkProgressUtils.ts` / `bulkProgress.tsx` duplicate types/helpers.
