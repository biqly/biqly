# Public Dashboard Sharing (Embedded Analytics — Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Public, unguessable, iframe-embeddable share links for dashboards, with live server-side widget execution, short-TTL caching, rate limiting, and a workspace-admin kill-switch.

**Architecture:** The `internal/dashboard` package gains a share model/repo, token helpers, a widget sanitizer, and a public resolver. Catalog serves anonymous `GET /api/public/dashboards/{token}` (sanitized metadata) plus authenticated share-management endpoints; the query service serves anonymous `POST /api/public/widget-query/{token}/{widgetID}` which executes the widget's **stored** `logical_query` server-side (the query service already has its own `bi_metadata` pool, so no catalog HTTP hop is needed — this is the one deliberate deviation from the spec's "query fetches the widget from a catalog internal endpoint" wording). The kill-switch lives on `workspaces` in the auth DB, checked per-request via a new uncached `AuthClient` internal call. Frame-blocking headers are relaxed only on `/api/public/*` (Go) and `/public/` (nginx). The SPA gains a shell-less `/public/dashboard/:token` route reusing `DashboardWidgetRenderer` via an injected data fetcher.

**Tech Stack:** Go 1.26 + chi + database/sql (Postgres), go-redis v9 (Dragonfly), React 19 + TS + Vite, Recharts, vitest, Helm/Envoy Gateway.

## Global Constraints

- Go 1.26 idioms: `errors.Is`, `errors.AsType[T]`, `new(expr)` for pointer fields, `for i := range n`, `min`/`max`, `slices.Contains`.
- Every commit passes the repo pre-commit hook (`make precommit` = format + lint-go + test-go + check-frontend). Go repo tests need local Postgres: `make dev-up` must be running.
- Run `gofmt -w` on every touched `.go` file before commit.
- All user-facing strings via `useT()`; every new key added to BOTH `frontend/src/i18n/locales/en/core.ts` and `tr/core.ts` (public page keys go in `core`, NOT the lazy `admin` section).
- Share tokens: 32 bytes crypto-random, base64url; only the SHA-256 hex hash is persisted. Plaintext is returned exactly once at creation.
- All public-path failure modes (bad token, revoked, expired, kill-switch off, missing dashboard/widget, text widget) return the **same 404** shape: `writeEntityNotFound(w, "dashboard")`.
- Kill-switch default is **off**: `workspaces.public_sharing_enabled BOOLEAN NOT NULL DEFAULT FALSE`, checked uncached on every public request.
- `logical_query` / `saved_query_id` must never appear in any anonymous response.
- New env vars: `BI_PUBLIC_SHARE_CACHE_TTL` (duration, default `60s`), `BI_PUBLIC_SHARE_RATE_LIMIT` (int, default `60` req/min per token+IP). Document in `docs/configuration.md`.
- The api service never imports `internal/auth` (duplicate small helpers instead, as done for the PAT prefix).

---

### Task 1: bi_metadata migration — `dashboard_public_shares`

**Files:**
- Create: `migrations/069a_create_dashboard_public_shares.up.sql`
- Create: `migrations/069a_create_dashboard_public_shares.down.sql`

**Interfaces:**
- Produces: table `dashboard_public_shares` used by Task 2's repository.

- [ ] **Step 1: Write the up migration**

```sql
-- Public share links for dashboards (embedded analytics phase 1).
-- Token plaintext lives only in the share URL; we persist its SHA-256 hex.
CREATE TABLE IF NOT EXISTS dashboard_public_shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dashboard_id UUID NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
);

-- At most one active (non-revoked) share per dashboard: "rotate" revokes then inserts.
CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_public_shares_active
    ON dashboard_public_shares(dashboard_id) WHERE revoked_at IS NULL;
```

- [ ] **Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS dashboard_public_shares;
```

- [ ] **Step 3: Apply and verify**

Run: `go run ./cmd/migrate up` (uses `BI_METADATA_DB_DSN` from `.env.dev`; `make dev-up` must be running)
Then: `docker exec biqly-postgres-1 psql -U bi_user -d bi_metadata -c '\d dashboard_public_shares'`
Expected: table with the 8 columns and the partial unique index.

- [ ] **Step 4: Commit**

```bash
git add migrations/069a_create_dashboard_public_shares.up.sql migrations/069a_create_dashboard_public_shares.down.sql
git commit -m "feat(dashboard): add dashboard_public_shares table"
```

---

### Task 2: Share token helpers, model, repository (`internal/dashboard`)

**Files:**
- Create: `internal/dashboard/share.go`
- Create: `internal/dashboard/share_repository.go`
- Test: `internal/dashboard/share_test.go`, `internal/dashboard/share_repository_test.go`

**Interfaces:**
- Consumes: Task 1 table.
- Produces:
  - `dashboard.GenerateShareToken() (plaintext string, err error)`
  - `dashboard.HashShareToken(plaintext string) string` (sha256 hex)
  - `type PublicShare struct { ID, DashboardID, WorkspaceID, TokenHash, CreatedBy string; CreatedAt time.Time; RevokedAt, ExpiresAt *time.Time }`
  - `dashboard.NewShareRepository(db *sql.DB) *ShareRepository`
  - `(*ShareRepository) Rotate(ctx, share *PublicShare) error` — revokes any active share for the dashboard, inserts the new one (single tx)
  - `(*ShareRepository) GetActive(ctx, dashboardID, workspaceID string) (*PublicShare, error)` (`sql.ErrNoRows` when none)
  - `(*ShareRepository) Revoke(ctx, dashboardID, workspaceID string) error` (`sql.ErrNoRows` when none active)
  - `(*ShareRepository) FindActiveByTokenHash(ctx, tokenHash string) (*PublicShare, error)` — excludes revoked and expired

- [ ] **Step 1: Write failing token tests** (`share_test.go`)

```go
package dashboard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateShareToken(t *testing.T) {
	a, err := GenerateShareToken()
	require.NoError(t, err)
	b, err := GenerateShareToken()
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.GreaterOrEqual(t, len(a), 43) // 32 bytes base64url, unpadded
	assert.False(t, strings.ContainsAny(a, "+/="), "must be URL-safe")
}

func TestHashShareToken(t *testing.T) {
	h := HashShareToken("fixed-input")
	assert.Equal(t, HashShareToken("fixed-input"), h)
	assert.Len(t, h, 64) // sha256 hex
	assert.NotEqual(t, HashShareToken("other"), h)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dashboard/ -run 'TestGenerateShareToken|TestHashShareToken' -v`
Expected: FAIL — `undefined: GenerateShareToken`.

- [ ] **Step 3: Implement `share.go`**

```go
package dashboard

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// PublicShare is an anonymous, unguessable share link for one dashboard.
// Only the SHA-256 of the URL token is persisted.
type PublicShare struct {
	ID          string     `json:"id"`
	DashboardID string     `json:"dashboard_id"`
	WorkspaceID string     `json:"workspace_id"`
	TokenHash   string     `json:"-"`
	CreatedBy   string     `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// GenerateShareToken returns a new URL-safe share token. The plaintext is
// only ever shown once, at creation time.
func GenerateShareToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashShareToken mirrors the PAT posture (internal/auth/session.go HashToken):
// sha256 hex, duplicated here because catalog/query never import internal/auth.
func HashShareToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
```

- [ ] **Step 4: Run token tests — PASS**, then write failing repository test (`share_repository_test.go`). Mirror the harness of `internal/dashboard/repository_idor_test.go`: `testutil.OpenMetadataDB(t)` + inline `CREATE TABLE IF NOT EXISTS` for `dashboards` (copy the block from that file) **and** for `dashboard_public_shares` (copy Task 1 SQL, table + partial index).

```go
package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShareRepository_Lifecycle(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	// CREATE TABLE IF NOT EXISTS dashboards (...)          <- copy from repository_idor_test.go
	// CREATE TABLE IF NOT EXISTS dashboard_public_shares (...) + partial index  <- copy from migration 069a
	createShareTestTables(t, ctx, db)

	dashRepo := NewRepository(db)
	repo := NewShareRepository(db)
	ws := "11111111-1111-1111-1111-111111111111"
	otherWS := "22222222-2222-2222-2222-222222222222"

	d := &Dashboard{WorkspaceID: new(ws), Name: "shared", Widgets: json.RawMessage(`[]`)}
	require.NoError(t, dashRepo.Create(ctx, d))

	tok, err := GenerateShareToken()
	require.NoError(t, err)
	share := &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok)}
	require.NoError(t, repo.Rotate(ctx, share))
	require.NotEmpty(t, share.ID)

	t.Run("FindActiveByTokenHash finds the live share", func(t *testing.T) {
		got, err := repo.FindActiveByTokenHash(ctx, HashShareToken(tok))
		require.NoError(t, err)
		assert.Equal(t, d.ID, got.DashboardID)
		assert.Equal(t, ws, got.WorkspaceID)
	})

	t.Run("unknown hash is ErrNoRows", func(t *testing.T) {
		_, err := repo.FindActiveByTokenHash(ctx, HashShareToken("nope"))
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Rotate revokes the previous share", func(t *testing.T) {
		tok2, err := GenerateShareToken()
		require.NoError(t, err)
		require.NoError(t, repo.Rotate(ctx, &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok2)}))
		_, err = repo.FindActiveByTokenHash(ctx, HashShareToken(tok))
		assert.ErrorIs(t, err, sql.ErrNoRows, "old token must be dead after rotate")
		got, err := repo.FindActiveByTokenHash(ctx, HashShareToken(tok2))
		require.NoError(t, err)
		assert.Equal(t, d.ID, got.DashboardID)
	})

	t.Run("expired share is not found", func(t *testing.T) {
		tok3, err := GenerateShareToken()
		require.NoError(t, err)
		past := time.Now().Add(-time.Hour)
		require.NoError(t, repo.Rotate(ctx, &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok3), ExpiresAt: &past}))
		_, err = repo.FindActiveByTokenHash(ctx, HashShareToken(tok3))
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("GetActive and Revoke are workspace-scoped (IDOR guard)", func(t *testing.T) {
		tok4, err := GenerateShareToken()
		require.NoError(t, err)
		require.NoError(t, repo.Rotate(ctx, &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok4)}))
		_, err = repo.GetActive(ctx, d.ID, otherWS)
		assert.ErrorIs(t, err, sql.ErrNoRows)
		assert.ErrorIs(t, repo.Revoke(ctx, d.ID, otherWS), sql.ErrNoRows)
		got, err := repo.GetActive(ctx, d.ID, ws)
		require.NoError(t, err)
		assert.Nil(t, got.RevokedAt)
		require.NoError(t, repo.Revoke(ctx, d.ID, ws))
		_, err = repo.GetActive(ctx, d.ID, ws)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}
```

- [ ] **Step 5: Run to verify failure** (`undefined: NewShareRepository`), then implement `share_repository.go`:

```go
package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ShareRepository handles database operations for dashboard public shares.
type ShareRepository struct {
	db *sql.DB
}

// NewShareRepository creates a new dashboard public-share repository.
func NewShareRepository(db *sql.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

// Rotate revokes any active share for the dashboard and inserts the new one
// atomically, keeping the one-active-share-per-dashboard invariant.
func (r *ShareRepository) Rotate(ctx context.Context, s *PublicShare) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rotate share: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE dashboard_public_shares SET revoked_at = now()
		WHERE dashboard_id = $1 AND revoked_at IS NULL
	`, s.DashboardID); err != nil {
		return fmt.Errorf("revoke previous share: %w", err)
	}

	var createdBy sql.NullString
	if s.CreatedBy != "" {
		createdBy = sql.NullString{String: s.CreatedBy, Valid: true}
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO dashboard_public_shares (dashboard_id, workspace_id, token_hash, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, s.DashboardID, s.WorkspaceID, s.TokenHash, createdBy, s.ExpiresAt).Scan(&s.ID, &s.CreatedAt); err != nil {
		return fmt.Errorf("insert share: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rotate share: %w", err)
	}
	return nil
}

// GetActive returns the live share for a dashboard, scoped to the workspace.
func (r *ShareRepository) GetActive(ctx context.Context, dashboardID, workspaceID string) (*PublicShare, error) {
	s := &PublicShare{}
	var createdBy sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, dashboard_id, workspace_id, token_hash, created_by, created_at, revoked_at, expires_at
		FROM dashboard_public_shares
		WHERE dashboard_id = $1 AND workspace_id = $2 AND revoked_at IS NULL
	`, dashboardID, workspaceID).Scan(&s.ID, &s.DashboardID, &s.WorkspaceID, &s.TokenHash, &createdBy, &s.CreatedAt, &s.RevokedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("get active share: %w", err)
	}
	if createdBy.Valid {
		s.CreatedBy = createdBy.String
	}
	return s, nil
}

// Revoke soft-deletes the active share for a dashboard within the workspace.
func (r *ShareRepository) Revoke(ctx context.Context, dashboardID, workspaceID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE dashboard_public_shares SET revoked_at = now()
		WHERE dashboard_id = $1 AND workspace_id = $2 AND revoked_at IS NULL
	`, dashboardID, workspaceID)
	if err != nil {
		return fmt.Errorf("revoke share: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// FindActiveByTokenHash is the anonymous lookup path: live, unexpired shares only.
func (r *ShareRepository) FindActiveByTokenHash(ctx context.Context, tokenHash string) (*PublicShare, error) {
	s := &PublicShare{}
	var createdBy sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, dashboard_id, workspace_id, token_hash, created_by, created_at, revoked_at, expires_at
		FROM dashboard_public_shares
		WHERE token_hash = $1 AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
	`, tokenHash).Scan(&s.ID, &s.DashboardID, &s.WorkspaceID, &s.TokenHash, &createdBy, &s.CreatedAt, &s.RevokedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("find share by token: %w", err)
	}
	if createdBy.Valid {
		s.CreatedBy = createdBy.String
	}
	return s, nil
}
```

- [ ] **Step 6: Run all package tests**

Run: `go test ./internal/dashboard/ -race -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/dashboard/share.go internal/dashboard/share_repository.go internal/dashboard/share_test.go internal/dashboard/share_repository_test.go
git add internal/dashboard/
git commit -m "feat(dashboard): public share token helpers and repository"
```

---

### Task 3: Widget sanitizer + public resolver (`internal/dashboard`)

**Files:**
- Create: `internal/dashboard/public.go`
- Test: `internal/dashboard/public_test.go`

**Interfaces:**
- Consumes: Task 2 repo + `dashboard.Repository`.
- Produces:
  - `dashboard.ErrShareNotFound` — the single sentinel every public failure maps to (handlers translate it to the uniform 404)
  - `dashboard.SanitizeWidgets(raw json.RawMessage) (json.RawMessage, error)` — strips `logical_query` and `saved_query_id` from every widget object
  - `dashboard.NewPublicResolver(db *sql.DB) *PublicResolver`
  - `(*PublicResolver) ResolveDashboard(ctx, plainToken string) (*PublicDashboardView, error)` where `type PublicDashboardView struct { Dashboard *Dashboard; Share *PublicShare }` (Dashboard.Widgets already sanitized)
  - `(*PublicResolver) ResolveWidgetQuery(ctx, plainToken, widgetID string) (*PublicWidgetQuery, error)` where `type PublicWidgetQuery struct { WorkspaceID string; LogicalQuery *logicalquery.LogicalQuery }`

- [ ] **Step 1: Write failing tests** (`public_test.go`)

```go
package dashboard

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWidgetsJSON = `[
  {"id":"w1","type":"chart","title":"Sales","w":6,"h":"medium","chart_type":"bar",
   "config":{"xAxisColumn":"month","yAxisColumns":["total"]},
   "logical_query":{"datasource_id":"ds-1","model_id":"m-1","select":[{"kind":"dimension","field":"month"}],"limit":100},
   "saved_query_id":"sq-1"},
  {"id":"w2","type":"text","title":"Note","w":6,"h":"small","content":"hello"}
]`

func TestSanitizeWidgets(t *testing.T) {
	out, err := SanitizeWidgets(json.RawMessage(testWidgetsJSON))
	require.NoError(t, err)
	var widgets []map[string]any
	require.NoError(t, json.Unmarshal(out, &widgets))
	require.Len(t, widgets, 2)
	for _, w := range widgets {
		assert.NotContains(t, w, "logical_query")
		assert.NotContains(t, w, "saved_query_id")
	}
	assert.Equal(t, "Sales", widgets[0]["title"])
	assert.Equal(t, "hello", widgets[1]["content"])
}

func TestPublicResolver(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	createShareTestTables(t, ctx, db) // helper from Task 2 test

	ws := "11111111-1111-1111-1111-111111111111"
	d := &Dashboard{WorkspaceID: new(ws), Name: "pub", Widgets: json.RawMessage(testWidgetsJSON)}
	require.NoError(t, NewRepository(db).Create(ctx, d))

	tok, err := GenerateShareToken()
	require.NoError(t, err)
	require.NoError(t, NewShareRepository(db).Rotate(ctx, &PublicShare{
		DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok),
	}))

	res := NewPublicResolver(db)

	t.Run("ResolveDashboard returns sanitized widgets", func(t *testing.T) {
		view, err := res.ResolveDashboard(ctx, tok)
		require.NoError(t, err)
		assert.Equal(t, d.ID, view.Dashboard.ID)
		assert.NotContains(t, string(view.Dashboard.Widgets), "logical_query")
		assert.NotContains(t, string(view.Dashboard.Widgets), "saved_query_id")
	})

	t.Run("bad token is ErrShareNotFound", func(t *testing.T) {
		_, err := res.ResolveDashboard(ctx, "bogus")
		assert.ErrorIs(t, err, ErrShareNotFound)
	})

	t.Run("ResolveWidgetQuery returns the stored logical query", func(t *testing.T) {
		q, err := res.ResolveWidgetQuery(ctx, tok, "w1")
		require.NoError(t, err)
		assert.Equal(t, ws, q.WorkspaceID)
		assert.Equal(t, "ds-1", q.LogicalQuery.DatasourceID)
	})

	t.Run("text widget and unknown widget are ErrShareNotFound", func(t *testing.T) {
		_, err := res.ResolveWidgetQuery(ctx, tok, "w2")
		assert.ErrorIs(t, err, ErrShareNotFound)
		_, err = res.ResolveWidgetQuery(ctx, tok, "missing")
		assert.ErrorIs(t, err, ErrShareNotFound)
	})
}
```

- [ ] **Step 2: Run to verify failure** (`undefined: SanitizeWidgets`), then implement `public.go`:

```go
package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/biqly/biqly/pkg/logicalquery"
)

// ErrShareNotFound is the single error every anonymous-path failure collapses
// to, so handlers can return one uniform 404 without leaking which check failed.
var ErrShareNotFound = errors.New("public share not found")

// PublicDashboardView is the anonymous read model: dashboard with widgets
// already sanitized (no logical_query / saved_query_id).
type PublicDashboardView struct {
	Dashboard *Dashboard
	Share     *PublicShare
}

// PublicWidgetQuery is the server-side execution input for one widget.
type PublicWidgetQuery struct {
	WorkspaceID  string
	LogicalQuery *logicalquery.LogicalQuery
}

// SanitizeWidgets strips query internals from the widget config so the
// anonymous client only ever sees render configuration.
func SanitizeWidgets(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`[]`), nil
	}
	var widgets []map[string]any
	if err := json.Unmarshal(raw, &widgets); err != nil {
		return nil, fmt.Errorf("parse widgets: %w", err)
	}
	for _, w := range widgets {
		delete(w, "logical_query")
		delete(w, "saved_query_id")
	}
	out, err := json.Marshal(widgets)
	if err != nil {
		return nil, fmt.Errorf("marshal sanitized widgets: %w", err)
	}
	return out, nil
}

// PublicResolver resolves share tokens to dashboards/widget queries. Both the
// catalog service (metadata) and the query service (execution) use it against
// their own bi_metadata pools.
type PublicResolver struct {
	shares *ShareRepository
	dashes *Repository
}

// NewPublicResolver builds a resolver over a bi_metadata connection.
func NewPublicResolver(db *sql.DB) *PublicResolver {
	return &PublicResolver{shares: NewShareRepository(db), dashes: NewRepository(db)}
}

func (r *PublicResolver) resolve(ctx context.Context, plainToken string) (*Dashboard, *PublicShare, error) {
	share, err := r.shares.FindActiveByTokenHash(ctx, HashShareToken(plainToken))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrShareNotFound
	} else if err != nil {
		return nil, nil, err
	}
	d, err := r.dashes.Get(ctx, share.DashboardID, share.WorkspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrShareNotFound
		}
		return nil, nil, err
	}
	return d, share, nil
}

// ResolveDashboard returns the sanitized dashboard for a share token.
func (r *PublicResolver) ResolveDashboard(ctx context.Context, plainToken string) (*PublicDashboardView, error) {
	d, share, err := r.resolve(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	sanitized, err := SanitizeWidgets(d.Widgets)
	if err != nil {
		return nil, err
	}
	d.Widgets = sanitized
	return &PublicDashboardView{Dashboard: d, Share: share}, nil
}

// ResolveWidgetQuery returns the stored logical query for one widget of a
// shared dashboard. Widgets without a stored query (text) are not found.
func (r *PublicResolver) ResolveWidgetQuery(ctx context.Context, plainToken, widgetID string) (*PublicWidgetQuery, error) {
	d, share, err := r.resolve(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	var widgets []struct {
		ID           string                     `json:"id"`
		LogicalQuery *logicalquery.LogicalQuery `json:"logical_query"`
	}
	if err := json.Unmarshal(d.Widgets, &widgets); err != nil {
		return nil, fmt.Errorf("parse widgets: %w", err)
	}
	for _, w := range widgets {
		if w.ID == widgetID && w.LogicalQuery != nil {
			return &PublicWidgetQuery{WorkspaceID: share.WorkspaceID, LogicalQuery: w.LogicalQuery}, nil
		}
	}
	return nil, ErrShareNotFound
}
```

- [ ] **Step 3: Run tests — PASS**

Run: `go test ./internal/dashboard/ -race -v`

- [ ] **Step 4: Commit**

```bash
gofmt -w internal/dashboard/public.go internal/dashboard/public_test.go
git add internal/dashboard/
git commit -m "feat(dashboard): widget sanitizer and public share resolver"
```

---

### Task 4: Auth service — workspace kill-switch flag

**Files:**
- Create: `migrations/auth/038a_add_workspace_public_sharing.up.sql` + `.down.sql`
- Modify: `internal/auth/workspace/workspace.go` (struct, `Get`/`ListForUser`/`ListAll`/`Create` SELECT+Scan, `Update`, new accessors)
- Modify: `internal/auth/handlers/handler_rbac.go:317-336` (`handleUpdateWorkspace` request struct) and `RegisterInternalRoutes` (line ~201)
- Test: `internal/auth/workspace/public_sharing_test.go`

**Interfaces:**
- Produces:
  - column `workspaces.public_sharing_enabled BOOLEAN NOT NULL DEFAULT FALSE`
  - `Workspace.PublicSharingEnabled bool` (JSON `public_sharing_enabled`)
  - `(*Service) IsPublicSharingEnabled(ctx, workspaceID string) (bool, error)`
  - `(*Service) SetPublicSharingEnabled(ctx, workspaceID, callerID string, enabled bool) error` (owner/admin gated)
  - `(*Service) Update(ctx, id, callerID, name, description string, mfaRequired, publicSharing *bool)` — extended signature
  - Internal HTTP: `GET /internal/auth/workspaces/{id}/public-sharing` → `{"enabled":bool}` (404 on unknown workspace)
  - User-facing: existing workspace update endpoint accepts `"public_sharing_enabled": bool`

- [ ] **Step 1: Migrations** (mirror `migrations/auth/022a_add_workspace_mfa_required.*`)

up:
```sql
ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS public_sharing_enabled BOOLEAN NOT NULL DEFAULT FALSE;
```
down:
```sql
ALTER TABLE workspaces
    DROP COLUMN IF EXISTS public_sharing_enabled;
```
Apply: `go run ./cmd/auth-migrate up`.

- [ ] **Step 2: Write failing service test** (`public_sharing_test.go`, mirror the setup used in `internal/auth/workspace/active_workspace_test.go` for creating the service and a workspace):

```go
func TestPublicSharingFlag(t *testing.T) {
	// setup: svc + owner user + workspace, exactly as in active_workspace_test.go
	enabled, err := svc.IsPublicSharingEnabled(ctx, ws.ID)
	require.NoError(t, err)
	assert.False(t, enabled, "default must be off")

	require.NoError(t, svc.SetPublicSharingEnabled(ctx, ws.ID, ownerID, true))
	enabled, err = svc.IsPublicSharingEnabled(ctx, ws.ID)
	require.NoError(t, err)
	assert.True(t, enabled)

	err = svc.SetPublicSharingEnabled(ctx, ws.ID, nonAdminID, false)
	assert.Error(t, err, "non-admin must not toggle the kill-switch")

	_, err = svc.IsPublicSharingEnabled(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, ErrWorkspaceNotFound)
}
```

- [ ] **Step 3: Implement in `workspace.go`** — accessors mirror `IsMFARequired`/`SetMFARequired` (lines 344-364) verbatim with the new column:

```go
// IsPublicSharingEnabled reports the workspace public-share kill-switch.
func (s *Service) IsPublicSharingEnabled(ctx context.Context, workspaceID string) (bool, error) {
	var enabled bool
	err := s.db.QueryRowContext(ctx,
		`SELECT public_sharing_enabled FROM workspaces WHERE id = $1`,
		workspaceID).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrWorkspaceNotFound
	}
	return enabled, err
}

// SetPublicSharingEnabled toggles public dashboard sharing for a workspace.
func (s *Service) SetPublicSharingEnabled(ctx context.Context, workspaceID, callerID string, enabled bool) error {
	if err := s.requireOwnerOrAdmin(ctx, workspaceID, callerID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE workspaces SET public_sharing_enabled = $1, updated_at = NOW()
		WHERE id = $2
	`, enabled, workspaceID)
	return err
}
```

Then: add `PublicSharingEnabled bool \`json:"public_sharing_enabled"\`` to the `Workspace` struct; add `public_sharing_enabled` to every `SELECT`+`Scan` that carries `mfa_required` (`Get`, `ListForUser`, `ListAll`, `Create` — grep `mfa_required` in the file and extend each in place); extend `Update` (line 185) with a `publicSharing *bool` param calling the setter, mirroring the `mfaRequired` branch.

- [ ] **Step 4: HTTP wiring** — in `handleUpdateWorkspace` (handler_rbac.go:317) add to the decode struct `PublicSharingEnabled *bool \`json:"public_sharing_enabled"\`` and pass to `h.deps.Ws.Update(...)`. In `RegisterInternalRoutes` (handler_rbac.go:201) add:

```go
r.Get("/workspaces/{id}/public-sharing", h.handleInternalWorkspacePublicSharing)
```

```go
func (h *RBACHandler) handleInternalWorkspacePublicSharing(w http.ResponseWriter, r *http.Request) {
	enabled, err := h.deps.Ws.IsPublicSharingEnabled(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, workspace.ErrWorkspaceNotFound) {
			writeError(w, r, http.StatusNotFound, err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/auth/... -race` — PASS (fix every `Scan` mismatch the compiler/tests surface).

- [ ] **Step 6: Commit**

```bash
git add migrations/auth/ internal/auth/
git commit -m "feat(auth): workspace public_sharing_enabled kill-switch"
```

---

### Task 5: AuthClient method + config + public middleware

**Files:**
- Modify: `internal/http/middleware/permission.go` (new `WorkspacePublicSharingEnabled`)
- Modify: `internal/config/config.go` (PublicShare config)
- Create: `internal/http/middleware/public_embed.go`
- Test: `internal/http/middleware/public_embed_test.go`
- Modify: `docs/configuration.md` (document the two env vars)

**Interfaces:**
- Produces:
  - `(*AuthClient) WorkspacePublicSharingEnabled(ctx, workspaceID string) (bool, error)` — **uncached** (kill-switch must be immediate)
  - `config.PublicShareConfig{ CacheTTL time.Duration; RateLimitPerMinute int }` at `cfg.PublicShare`, env `BI_PUBLIC_SHARE_CACHE_TTL` (default 60s), `BI_PUBLIC_SHARE_RATE_LIMIT` (default 60)
  - `bimw.PublicEmbedHeaders(next http.Handler) http.Handler` — deletes `X-Frame-Options`, sets CSP `frame-ancestors *`, CORP `cross-origin`, `X-Robots-Tag: noindex, nofollow`
  - `bimw.NewPublicRateLimiter(client *redis.Client, perMinute int) *PublicRateLimiter` with `(*PublicRateLimiter) Middleware() func(http.Handler) http.Handler` — 429 over limit, keyed by first URL path segment after the route prefix (the token) + client IP, fail-open when redis is nil/down
  - `bimw.PublicRateKey(r *http.Request) string` — extracted pure helper (unit-testable): `sha256hex(chi URLParam "token")[:16] + ":" + clientIP`

- [ ] **Step 1: AuthClient method** (mirror `VerifyPersonalAccessToken`'s header wiring, permission.go:241; GET variant, no cache):

```go
// WorkspacePublicSharingEnabled reads the public-share kill-switch. It is
// deliberately uncached: turning the switch off must kill existing links
// immediately.
func (c *AuthClient) WorkspacePublicSharingEnabled(ctx context.Context, workspaceID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/internal/auth/workspaces/"+url.PathEscape(workspaceID)+"/public-sharing", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("public-sharing lookup: status %d", resp.StatusCode)
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, err
	}
	return body.Enabled, nil
}
```

- [ ] **Step 2: Config** — add to `Config`: `PublicShare PublicShareConfig`; struct + `Load()` entries following the existing pattern (config.go:489):

```go
// PublicShareConfig tunes the anonymous dashboard-share endpoints.
type PublicShareConfig struct {
	CacheTTL           time.Duration
	RateLimitPerMinute int
}
```
```go
		PublicShare: PublicShareConfig{
			CacheTTL:           getEnvAsDuration("BI_PUBLIC_SHARE_CACHE_TTL", 60*time.Second),
			RateLimitPerMinute: getEnvAsInt("BI_PUBLIC_SHARE_RATE_LIMIT", 60),
		},
```
Document both vars in `docs/configuration.md` alongside the other `BI_*` entries.

- [ ] **Step 3: Failing middleware test** (`public_embed_test.go`):

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicEmbedHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	// Simulate the router: strict SecurityHeaders outside, PublicEmbedHeaders inside.
	h := SecurityHeaders(SecurityHeadersConfig{ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'"})(
		PublicEmbedHeaders(inner))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/public/dashboards/tok", nil))

	assert.Empty(t, rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "frame-ancestors *", rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "cross-origin", rec.Header().Get("Cross-Origin-Resource-Policy"))
	assert.Equal(t, "noindex, nofollow", rec.Header().Get("X-Robots-Tag"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), "other hardening must survive")
}

func TestPublicRateLimiter_NilClientPassesThrough(t *testing.T) {
	rl := NewPublicRateLimiter(nil, 1)
	called := 0
	h := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called++ }))
	for range 5 {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	}
	assert.Equal(t, 5, called, "nil redis must fail open")
}
```

- [ ] **Step 4: Implement `public_embed.go`** (INCR+EXPIRE bucket pattern copied from `internal/auth/ratelimit.go` — reimplemented here because api services never import `internal/auth`):

```go
package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// PublicEmbedHeaders relaxes the frame-blocking headers on anonymous embed
// routes only. It runs INSIDE the strict SecurityHeaders middleware, so it
// overrides the already-set values for this route group alone.
func PublicEmbedHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Del("X-Frame-Options")
		h.Set("Content-Security-Policy", "frame-ancestors *")
		h.Set("Cross-Origin-Resource-Policy", "cross-origin")
		h.Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}

// PublicRateLimiter throttles anonymous share traffic per token+IP using a
// fixed one-minute Dragonfly bucket (INCR+EXPIRE, same shape as the auth
// service limiter). Fails open when redis is unavailable.
type PublicRateLimiter struct {
	client    *redis.Client
	perMinute int
}

// NewPublicRateLimiter builds a limiter; a nil client disables limiting.
func NewPublicRateLimiter(client *redis.Client, perMinute int) *PublicRateLimiter {
	return &PublicRateLimiter{client: client, perMinute: perMinute}
}

// PublicRateKey derives the throttle key: hashed share token + client IP.
func PublicRateKey(r *http.Request) string {
	tok := chi.URLParam(r, "token")
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])[:16] + ":" + clientIP(r)
}

func clientIP(r *http.Request) string {
	// RealIP middleware (ApplyBaseMiddleware) already rewrites RemoteAddr.
	return r.RemoteAddr
}

// Middleware enforces the per-minute budget.
func (rl *PublicRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rl.client == nil || rl.perMinute <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			bucket := time.Now().Unix() / 60
			key := fmt.Sprintf("pubshare:rl:%s:%d", PublicRateKey(r), bucket)
			ctx := r.Context()
			count, err := rl.client.Incr(ctx, key).Result()
			if err != nil {
				next.ServeHTTP(w, r) // fail open, matching auth limiter
				return
			}
			if count == 1 {
				_ = rl.client.Expire(ctx, key, time.Minute).Err()
			}
			if count > int64(rl.perMinute) {
				w.Header().Set("Retry-After", strconv.Itoa(60))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"too_many_requests"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 5: Run tests, commit**

Run: `go test ./internal/http/middleware/ ./internal/config/ -race` — PASS.

```bash
git add internal/http/middleware/ internal/config/ docs/configuration.md
git commit -m "feat(http): public embed headers, rate limiter, share config, kill-switch client"
```

---

### Task 6: Catalog — share management endpoints + audit

**Files:**
- Create: `internal/http/handlers/dashboard_share.go`
- Modify: `internal/audit/audit.go` (two EventType consts)
- Modify: `internal/app/dependencies.go` (`CatalogDeps`: add `DashboardShareRepo *dashboard.ShareRepository`, `PublicResolver *dashboard.PublicResolver`, `PublicShareRedis *redis.Client`; populate them where `DashboardRepo` is built, redis via the `provideCompositeCache`-style `redis.ParseURL(cfg.Redis.DSN)` pattern from `internal/app/providers.go:63`)
- Modify: `internal/http/catalog_router.go:213` (`registerCatalogDashboardRoutes` gains the three routes; signature gains `authClient *bimw.AuthClient`)
- Test: `internal/http/handlers/dashboard_share_test.go`

**Interfaces:**
- Consumes: Tasks 2, 3, 5.
- Produces (all inside the authenticated `/api` group, workspace-scoped via `dashboardScope`):
  - `POST /api/dashboards/{id}/public-share` → 201 `{"token":"...","url_path":"/public/dashboard/<token>","created_at":"..."}` (creates or rotates)
  - `GET /api/dashboards/{id}/public-share` → 200 `{"active":true,"created_at":"...","expires_at":null}` or `{"active":false}`
  - `DELETE /api/dashboards/{id}/public-share` → 204
  - `audit.EventDashboardShareCreated` / `audit.EventDashboardShareRevoked`
  - `handlers.NewDashboardShareHandler(repo *dashboard.ShareRepository, dashRepo *dashboard.Repository, killSwitch workspaceSharingChecker, auditLogger *audit.Logger) *DashboardShareHandler` where `type workspaceSharingChecker interface { WorkspacePublicSharingEnabled(ctx context.Context, workspaceID string) (bool, error) }` (interface so tests can stub it; `*bimw.AuthClient` satisfies it)

- [ ] **Step 1: Audit consts** in `internal/audit/audit.go` next to the existing EventType block:

```go
	EventDashboardShareCreated EventType = "dashboard_share_created"
	EventDashboardShareRevoked EventType = "dashboard_share_revoked"
```

- [ ] **Step 2: Failing handler tests** (httptest against a chi router with the handler mounted and a fake auth context; mirror the style of existing handler tests in `internal/http/handlers/` — set workspace via `bimw` context keys). Cover: create returns a token once and the URL path; create on a dashboard in another workspace → 404; create when the workspace kill-switch is off → 409 with `"public sharing is disabled for this workspace"`; status reflects active/inactive; delete revokes (subsequent status `active:false`); delete with no active share → 404.

- [ ] **Step 3: Implement `dashboard_share.go`**

```go
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/dashboard"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// workspaceSharingChecker is the kill-switch lookup; *bimw.AuthClient satisfies it.
type workspaceSharingChecker interface {
	WorkspacePublicSharingEnabled(ctx context.Context, workspaceID string) (bool, error)
}

// DashboardShareHandler manages public share links for dashboards.
type DashboardShareHandler struct {
	shares     *dashboard.ShareRepository
	dashes     *dashboard.Repository
	killSwitch workspaceSharingChecker
	auditLog   *audit.Logger
}

// NewDashboardShareHandler creates a DashboardShareHandler.
func NewDashboardShareHandler(shares *dashboard.ShareRepository, dashes *dashboard.Repository, killSwitch workspaceSharingChecker, auditLog *audit.Logger) *DashboardShareHandler {
	return &DashboardShareHandler{shares: shares, dashes: dashes, killSwitch: killSwitch, auditLog: auditLog}
}

// shareScope authorizes the caller for a dashboard and returns its workspace.
// Public shares require a concrete workspace: global dashboards (NULL
// workspace_id) and unscoped super-admin calls are rejected.
func (h *DashboardShareHandler) shareScope(w http.ResponseWriter, r *http.Request) (dashboardID, workspaceID string, ok bool) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return "", "", false
	}
	wsID, ok := dashboardScope(r.Context())
	if !ok || wsID == "" {
		writeEntityNotFound(w, "dashboard")
		return "", "", false
	}
	d, err := h.dashes.Get(r.Context(), id, wsID)
	if err != nil {
		writeEntityNotFound(w, "dashboard")
		return "", "", false
	}
	if d.WorkspaceID == nil || *d.WorkspaceID == "" {
		writeError(w, http.StatusConflict, "global dashboards cannot be shared publicly")
		return "", "", false
	}
	return id, *d.WorkspaceID, true
}

// Create handles POST /api/dashboards/{id}/public-share (create or rotate).
func (h *DashboardShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, wsID, ok := h.shareScope(w, r)
	if !ok {
		return
	}
	enabled, err := h.killSwitch.WorkspacePublicSharingEnabled(ctx, wsID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to check workspace sharing policy", err)
		return
	}
	if !enabled {
		writeError(w, http.StatusConflict, "public sharing is disabled for this workspace")
		return
	}
	token, err := dashboard.GenerateShareToken()
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to generate share token", err)
		return
	}
	share := &dashboard.PublicShare{
		DashboardID: id,
		WorkspaceID: wsID,
		TokenHash:   dashboard.HashShareToken(token),
		CreatedBy:   bimw.UserID(ctx),
	}
	if err := h.shares.Rotate(ctx, share); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create share", err)
		return
	}
	h.auditLog.Log(ctx, audit.Event{
		UserID:    bimw.UserID(ctx),
		EventType: audit.EventDashboardShareCreated,
		Details:   map[string]any{"dashboard_id": id, "share_id": share.ID},
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"url_path":   "/public/dashboard/" + token,
		"created_at": share.CreatedAt.Format(time.RFC3339),
	})
}

// Status handles GET /api/dashboards/{id}/public-share.
func (h *DashboardShareHandler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, wsID, ok := h.shareScope(w, r)
	if !ok {
		return
	}
	share, err := h.shares.GetActive(ctx, id, wsID)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"active": false})
		return
	} else if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load share", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active":     true,
		"created_at": share.CreatedAt.Format(time.RFC3339),
		"expires_at": share.ExpiresAt,
	})
}

// Revoke handles DELETE /api/dashboards/{id}/public-share.
func (h *DashboardShareHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, wsID, ok := h.shareScope(w, r)
	if !ok {
		return
	}
	if err := h.shares.Revoke(ctx, id, wsID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeEntityNotFound(w, "share")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to revoke share", err)
		return
	}
	h.auditLog.Log(ctx, audit.Event{
		UserID:    bimw.UserID(ctx),
		EventType: audit.EventDashboardShareRevoked,
		Details:   map[string]any{"dashboard_id": id},
	})
	w.WriteHeader(http.StatusNoContent)
}
```

(Adapt `writeError` / `writeInternalError` / `writeEntityNotFound` calls to the exact signatures used in `internal/http/handlers/dashboard.go` — they are package-local helpers already imported there.)

- [ ] **Step 4: Register routes** — extend `registerCatalogDashboardRoutes` (catalog_router.go:213):

```go
func registerCatalogDashboardRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	dashHandler := handlers.NewDashboardHandler(deps.DashboardRepo)
	shareHandler := handlers.NewDashboardShareHandler(deps.DashboardShareRepo, deps.DashboardRepo, authClient, deps.AuditLogger)
	r.Route("/dashboards", func(r chi.Router) {
		r.Post("/", dashHandler.Create)
		r.Get("/", dashHandler.List)
		r.Get("/{id}", dashHandler.Get)
		r.Put("/{id}", dashHandler.Update)
		r.Delete("/{id}", dashHandler.Delete)
		r.Post("/{id}/public-share", shareHandler.Create)
		r.Get("/{id}/public-share", shareHandler.Status)
		r.Delete("/{id}/public-share", shareHandler.Revoke)
	})
}
```
Update its call site in `registerCatalogAPIRoutes` (catalog_router.go:65) to pass `authClient` (it is already in scope there).

- [ ] **Step 5: Run, commit**

Run: `go test ./internal/http/... ./internal/audit/... -race` — PASS.

```bash
git add internal/http/ internal/audit/ internal/app/
git commit -m "feat(catalog): dashboard public-share management endpoints"
```

---

### Task 7: Catalog + monolith — anonymous public metadata endpoint

**Files:**
- Create: `internal/http/handlers/public_dashboard.go`
- Create: `internal/http/public_routes.go` (shared registration helper)
- Modify: `internal/http/catalog_router.go:43-48` (nest `/public` before the authed group)
- Modify: `internal/http/router.go:93-119` (same nesting in the monolith, with proxy branch)
- Test: `internal/http/handlers/public_dashboard_test.go`

**Interfaces:**
- Consumes: Tasks 3, 5, 6 deps.
- Produces:
  - `GET /api/public/dashboards/{token}` → 200 `{"id","name","description","widgets":[sanitized]}` | uniform 404
  - `handlers.NewPublicDashboardHandler(resolver *dashboard.PublicResolver, killSwitch workspaceSharingChecker) *PublicDashboardHandler` with method `Get`
  - `http.registerPublicDashboardRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient)` — mounts `GET /dashboards/{token}` with `PublicEmbedHeaders` + rate limiter (built from `deps.PublicShareRedis` + `deps.Config.PublicShare.RateLimitPerMinute`)

- [ ] **Step 1: Failing handler tests.** Cover: valid token → 200, body contains widget `config`/`title` but NOT `logical_query`/`saved_query_id`; invalid token → 404; kill-switch off → **the same 404** (assert identical status + body as invalid token); response carries `Content-Security-Policy: frame-ancestors *` and no `X-Frame-Options` when mounted with the middleware.

- [ ] **Step 2: Implement handler:**

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/biqly/biqly/internal/dashboard"
)

// PublicDashboardHandler serves anonymous, sanitized dashboard metadata.
type PublicDashboardHandler struct {
	resolver   *dashboard.PublicResolver
	killSwitch workspaceSharingChecker
}

// NewPublicDashboardHandler creates a PublicDashboardHandler.
func NewPublicDashboardHandler(resolver *dashboard.PublicResolver, killSwitch workspaceSharingChecker) *PublicDashboardHandler {
	return &PublicDashboardHandler{resolver: resolver, killSwitch: killSwitch}
}

// Get handles GET /api/public/dashboards/{token}. Every failure mode returns
// the same 404 so the endpoint leaks nothing about token validity.
func (h *PublicDashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token, ok := requireURLParam(w, r, "token")
	if !ok {
		return
	}
	view, err := h.resolver.ResolveDashboard(ctx, token)
	if err != nil {
		if errors.Is(err, dashboard.ErrShareNotFound) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to resolve share", err)
		return
	}
	enabled, err := h.killSwitch.WorkspacePublicSharingEnabled(ctx, view.Share.WorkspaceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to check sharing policy", err)
		return
	}
	if !enabled {
		writeEntityNotFound(w, "dashboard")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          view.Dashboard.ID,
		"name":        view.Dashboard.Name,
		"description": view.Dashboard.Description,
		"widgets":     view.Dashboard.Widgets,
	})
}
```

- [ ] **Step 3: Shared registration + router nesting.** New `internal/http/public_routes.go`:

```go
package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// registerPublicDashboardRoutes mounts the anonymous dashboard-metadata
// endpoint. Caller mounts it under /api/public with PublicEmbedHeaders.
func registerPublicDashboardRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	limiter := bimw.NewPublicRateLimiter(deps.PublicShareRedis, deps.Config.PublicShare.RateLimitPerMinute)
	h := handlers.NewPublicDashboardHandler(deps.PublicResolver, authClient)
	r.With(limiter.Middleware()).Get("/dashboards/{token}", h.Get)
}
```

In `catalog_router.go`, restructure the `/api` mount so the public group is a sibling of the authed group **inside the same Route** (chi forbids overlapping top-level wildcards):

```go
	r.Route("/api", func(r chi.Router) {
		r.Route("/public", func(r chi.Router) { // anonymous: NO authMW
			r.Use(bimw.PublicEmbedHeaders)
			registerPublicDashboardRoutes(r, deps.CatalogDeps(), authClient)
		})
		r.Group(func(r chi.Router) {
			r.Use(authMW)
			r.Use(CatalogMetricsMiddleware(GetMetrics()))
			registerCatalogAPIRoutes(r, deps.CatalogDeps(), authClient)
		})
	})
```

In the monolith `router.go` `/api` route (line ~93), add the same `/public` sub-route BEFORE the existing authed content, with the proxy branch mirroring the catalog conditional:

```go
		r.Route("/public", func(r chi.Router) {
			r.Use(bimw.PublicEmbedHeaders)
			if deps.Config.Services.CatalogURL != "" {
				registerUpstreamProxy(r, upstreamProxySpec{
					targetURL: deps.Config.Services.CatalogURL, envVarName: "BI_CATALOG_SERVICE_URL",
					serviceLabel: "catalog service", paths: []string{"/dashboards/{token}"},
				})
			} else {
				registerPublicDashboardRoutes(r, deps.CatalogDeps(), authClient)
			}
		})
```
(The existing `r.Use(authMW)` at the top of the `/api` route must move into a `r.Group(func(r chi.Router) { r.Use(authMW); ... })` wrapping the current body — chi requires `Use` before routes, so the public sub-route is registered first, then the authed group.)

- [ ] **Step 4: Run, commit**

Run: `go test ./internal/http/... -race && go build ./...` — PASS.

```bash
git add internal/http/
git commit -m "feat(catalog): anonymous public dashboard metadata endpoint"
```

---

### Task 8: Query service + monolith — anonymous widget execution endpoint

**Files:**
- Create: `internal/http/handlers/public_widget_query.go`
- Modify: `internal/http/public_routes.go` (add `registerPublicWidgetQueryRoutes`)
- Modify: `internal/app/query_dependencies.go` (`QueryDeps` gains `PublicResolver *dashboard.PublicResolver`, `PublicShareRedis *redis.Client`, built from the existing `db` handle + `cfg.Redis.DSN`)
- Modify: `internal/http/query_router.go:39-53` (nest `/public` before authed group)
- Modify: `internal/http/router.go` (monolith `/api/public` block gains the query branch)
- Test: `internal/http/handlers/public_widget_query_test.go`

**Interfaces:**
- Consumes: Task 3 resolver, Task 5 middleware/config, `internalQueryRunner.RunWithModel` (existing, `internal/http/handlers/internal_ports.go:37`).
- Produces:
  - `POST /api/public/widget-query/{token}/{widgetID}` → 200 `query result payload` (same shape as `/api/query/run`) | uniform 404 | 429
  - `handlers.NewPublicWidgetQueryHandler(resolver *dashboard.PublicResolver, runner internalQueryRunner, killSwitch workspaceSharingChecker, cache *redis.Client, cacheTTL time.Duration) *PublicWidgetQueryHandler`
  - Path is `/api/public/widget-query/...` (NOT under `/api/public/dashboards/...`) so the Envoy gateway can prefix-route it to the query service while `/api/public/dashboards` goes to catalog.

- [ ] **Step 1: Failing handler tests.** Use a stub `internalQueryRunner` (implement the interface returning a canned `*core.RunResult`) and a stub kill-switch. Cover: valid token+widget → 200 with the runner's result and the runner received the STORED logical query (not anything from the request body — send a malicious body and assert it is ignored); unknown widget / text widget / bad token / kill-switch off → identical 404s; second call within TTL does NOT re-invoke the runner when a redis client is present (skip this subtest when no redis; with nil cache client every call executes).

- [ ] **Step 2: Implement `public_widget_query.go`:**

```go
package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/biqly/biqly/internal/dashboard"
)

// PublicWidgetQueryHandler executes the stored logical query of one widget of
// a publicly shared dashboard. No query input is accepted from the visitor.
type PublicWidgetQueryHandler struct {
	resolver   *dashboard.PublicResolver
	runner     internalQueryRunner
	killSwitch workspaceSharingChecker
	cache      *redis.Client
	cacheTTL   time.Duration
}

// NewPublicWidgetQueryHandler creates a PublicWidgetQueryHandler.
func NewPublicWidgetQueryHandler(resolver *dashboard.PublicResolver, runner internalQueryRunner, killSwitch workspaceSharingChecker, cache *redis.Client, cacheTTL time.Duration) *PublicWidgetQueryHandler {
	return &PublicWidgetQueryHandler{resolver: resolver, runner: runner, killSwitch: killSwitch, cache: cache, cacheTTL: cacheTTL}
}

func (h *PublicWidgetQueryHandler) cacheKey(token, widgetID string) string {
	return "pubshare:wq:" + dashboard.HashShareToken(token)[:32] + ":" + widgetID
}

// Run handles POST /api/public/widget-query/{token}/{widgetID}.
func (h *PublicWidgetQueryHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token, ok := requireURLParam(w, r, "token")
	if !ok {
		return
	}
	widgetID, ok := requireURLParam(w, r, "widgetID")
	if !ok {
		return
	}

	wq, err := h.resolver.ResolveWidgetQuery(ctx, token, widgetID)
	if err != nil {
		if errors.Is(err, dashboard.ErrShareNotFound) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to resolve widget", err)
		return
	}
	enabled, err := h.killSwitch.WorkspacePublicSharingEnabled(ctx, wq.WorkspaceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to check sharing policy", err)
		return
	}
	if !enabled {
		writeEntityNotFound(w, "dashboard")
		return
	}

	// Short-TTL cache shields the customer datasource from anonymous traffic.
	if h.cache != nil {
		if raw, err := h.cache.Get(ctx, h.cacheKey(token, widgetID)).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(raw)
			return
		}
	}

	result, se := h.runner.RunWithModel(ctx, wq.LogicalQuery, nil)
	if se != nil {
		writeServiceError(ctx, w, se)
		return
	}
	if h.cache != nil && result != nil && result.Result != nil {
		if payload, err := sonic.Marshal(result.Result); err == nil {
			_ = h.cache.Set(ctx, h.cacheKey(token, widgetID), payload, h.cacheTTL).Err()
		}
	}
	writeJSON(w, http.StatusOK, result.Result)
}
```

(Match `writeServiceError`'s exact signature from `internal/http/handlers/query.go:75` — it is `writeServiceError(r.Context(), w, se)` there; use the same form.)

- [ ] **Step 3: Registration.** Add to `public_routes.go`:

```go
// registerPublicWidgetQueryRoutes mounts the anonymous widget execution
// endpoint. Caller mounts it under /api/public with PublicEmbedHeaders.
func registerPublicWidgetQueryRoutes(r chi.Router, deps *app.QueryDeps, authClient *bimw.AuthClient) {
	limiter := bimw.NewPublicRateLimiter(deps.PublicShareRedis, deps.Config.PublicShare.RateLimitPerMinute)
	h := handlers.NewPublicWidgetQueryHandler(deps.PublicResolver, deps.QueryService, authClient, deps.PublicShareRedis, deps.Config.PublicShare.CacheTTL)
	r.With(limiter.Middleware()).Post("/widget-query/{token}/{widgetID}", h.Run)
}
```

Nest in `query_router.go` exactly as Task 7 did for catalog (public sub-route first, then existing authed content wrapped in `r.Group`). In the monolith `/api/public` block from Task 7, add:

```go
			if deps.Config.Services.QueryURL != "" {
				registerUpstreamProxy(r, upstreamProxySpec{
					targetURL: deps.Config.Services.QueryURL, envVarName: "BI_QUERY_SERVICE_URL",
					serviceLabel: "query service", paths: []string{"/widget-query/{token}/{widgetID}"},
				})
			} else {
				registerPublicWidgetQueryRoutes(r, deps.QueryDeps(), authClient)
			}
```

- [ ] **Step 4: Run, commit**

Run: `go test ./internal/http/... ./internal/app/... -race && go build ./...` — PASS.
Then run gograph review: `gograph_review` with `uncommitted=true` (or after commit, on the change set) to confirm blast radius and test coverage.

```bash
git add internal/http/ internal/app/
git commit -m "feat(query): anonymous public widget-query endpoint with cache and rate limit"
```

---

### Task 9: Frontend — public API client + widget renderer fetch injection

**Files:**
- Create: `frontend/src/api/publicDashboard.ts`
- Modify: `frontend/src/components/dashboard/DashboardWidgetRenderer.tsx:282-319`
- Test: `frontend/src/api/publicDashboard.test.ts`, extend `frontend/src/components/dashboard/` renderer tests if present

**Interfaces:**
- Produces:
  - `getPublicDashboard(token: string): Promise<PublicDashboard>` where `interface PublicDashboard { id: string; name: string; description?: string; widgets: DashboardWidget[] }`
  - `runPublicWidget(token: string, widgetId: string): Promise<QueryResultPayload>`
  - `DashboardWidgetRenderer` gains optional prop `fetchData?: (widget: DashboardWidget) => Promise<QueryResultPayload | null>`; when provided it is used for every non-text widget (sanitized widgets have no `logical_query`); when absent, behavior is exactly today's.

- [ ] **Step 1: Failing API client test** (vitest, mock `fetch`): `getPublicDashboard` GETs `/api/public/dashboards/<token>` with NO `Authorization` header even when a global token is set (`setGlobalAccessToken('x')` then assert); `runPublicWidget` POSTs `/api/public/widget-query/<token>/<widgetId>`.

- [ ] **Step 2: Implement `publicDashboard.ts`:**

```ts
import { apiFetch } from './apiClient'
import type { QueryResultPayload } from '../types/ai'
import type { DashboardWidget } from '../components/dashboard/DashboardWidgetRenderer'

export interface PublicDashboard {
  id: string
  name: string
  description?: string
  widgets: DashboardWidget[]
}

// token: '' forces anonymous requests: the public page must behave identically
// for signed-in and signed-out visitors.
export function getPublicDashboard(token: string): Promise<PublicDashboard> {
  return apiFetch<PublicDashboard>('GET', `/api/public/dashboards/${encodeURIComponent(token)}`, undefined, {
    token: '',
  })
}

export function runPublicWidget(token: string, widgetId: string): Promise<QueryResultPayload> {
  return apiFetch<QueryResultPayload>(
    'POST',
    `/api/public/widget-query/${encodeURIComponent(token)}/${encodeURIComponent(widgetId)}`,
    {},
    { token: '' },
  )
}
```

(Verify `apiFetch`'s parameter order in `src/api/apiClient.ts` before writing — mirror an existing call like `createShare` in `src/api/admin.ts:623`.)

- [ ] **Step 3: Renderer refactor.** In `DashboardWidgetRenderer.tsx`, change the signature and effect:

```tsx
export function DashboardWidgetRenderer({
  widget,
  fetchData,
}: {
  widget: DashboardWidget
  fetchData?: (widget: DashboardWidget) => Promise<QueryResultPayload | null>
}) {
  const { postData, loading: apiLoading, error: apiError, abort } = useApi()
  const [extLoading, setExtLoading] = useState(false)
  const [extError, setExtError] = useState<string | null>(null)
  const [data, setData] = useResetStateOnDepsChange<ChartRow[] | null>(null, [
    widget.id,
    widget.logical_query,
    widget.type,
  ])
  const [columns, setColumns] = useState<QueryResultPayload['columns']>([])

  useEffect(() => {
    const applyResult = (res: QueryResultPayload | null) => {
      if (!res) {
        setData([])
        return
      }
      setColumns(res.columns)
      const mapped = res.rows.map((row) => {
        const obj: ChartRow = {}
        res.columns.forEach((col, idx) => {
          obj[col.name] = row[idx]
        })
        return obj
      })
      setData(mapped)
    }

    if (widget.type === 'text') {
      return
    }

    let active = true
    if (fetchData) {
      setExtLoading(true)
      setExtError(null)
      fetchData(widget)
        .then((res) => {
          if (!active) {
            return
          }
          applyResult(res)
        })
        .catch((e: unknown) => {
          if (active) {
            setExtError(e instanceof Error ? e.message : String(e))
            setData([])
          }
        })
        .finally(() => {
          if (active) {
            setExtLoading(false)
          }
        })
      return () => {
        active = false
        setData(null)
      }
    }

    if (!widget.logical_query) {
      return
    }
    void postData<QueryResultPayload>('/api/query/run', widget.logical_query).then((res) => {
      if (!active) {
        return
      }
      applyResult(res)
    })
    return () => {
      active = false
      abort()
      setData(null)
    }
  }, [widget, widget.id, widget.logical_query, widget.type, fetchData, postData, abort, setData])

  const loading = fetchData ? extLoading : apiLoading
  const error = fetchData ? extError : apiError
```

Keep the rest of the component (the `loading`/`error` usages downstream already reference those names — adjust the destructuring aliases accordingly). Existing in-app call sites (`DashboardBuilder.tsx`) pass no `fetchData` and are unaffected.

- [ ] **Step 4: Run, commit**

Run: `npm --prefix frontend run test` and `make typecheck-frontend` — PASS.

```bash
git add frontend/src/api/publicDashboard.ts frontend/src/api/publicDashboard.test.ts frontend/src/components/dashboard/DashboardWidgetRenderer.tsx
git commit -m "feat(frontend): public dashboard API client and injectable widget data fetcher"
```

---

### Task 10: Frontend — public dashboard page + route

**Files:**
- Create: `frontend/src/components/public/PublicDashboardPage.tsx`
- Modify: `frontend/src/App.tsx` (lazy import + route outside AuthGuard/shell)
- Modify: `frontend/src/i18n/locales/en/core.ts` + `tr/core.ts` (new `publicDashboard` slice)
- Test: `frontend/src/components/public/PublicDashboardPage.test.tsx`

**Interfaces:**
- Consumes: Task 9 client + renderer prop.
- Produces: route `/public/dashboard/:token` — shell-less, read-only grid, neutral error state, "Powered by biqly" footer.

- [ ] **Step 1: i18n keys** (both locales; en shown, add Turkish equivalents in `tr/core.ts` at the mirrored position):

```ts
  publicDashboard: {
    loading: 'Loading dashboard…',
    not_found_title: 'Dashboard unavailable',
    not_found_desc: 'This dashboard does not exist or sharing has been disabled.',
    powered_by: 'Powered by',
  },
```

- [ ] **Step 2: Failing page test** (vitest + testing-library): mocks `getPublicDashboard` — success renders the dashboard name and one widget title; 404 rejection renders `not_found_title`; no sidebar/nav landmarks in the DOM.

- [ ] **Step 3: Implement the page:**

```tsx
import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'

import { getPublicDashboard, runPublicWidget, type PublicDashboard } from '../../api/publicDashboard'
import { useT } from '../../i18n/hooks'
import { cn } from '../../lib/cn'
import { cardClass } from '../../lib/cardClasses'
import { DashboardWidgetRenderer, type DashboardWidget } from '../dashboard/DashboardWidgetRenderer'

function widgetHeightPx(h: DashboardWidget['h']): number {
  if (typeof h === 'number') {
    return h
  }
  return h === 'small' ? 220 : h === 'large' ? 520 : 360
}

export default function PublicDashboardPage() {
  const t = useT()
  const { token = '' } = useParams()
  const [dash, setDash] = useState<PublicDashboard | null>(null)
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading')

  useEffect(() => {
    let active = true
    setState('loading')
    getPublicDashboard(token)
      .then((d) => {
        if (active) {
          setDash(d)
          setState('ready')
        }
      })
      .catch(() => {
        if (active) {
          setState('error')
        }
      })
    return () => {
      active = false
    }
  }, [token])

  if (state === 'loading') {
    return (
      <div className="text-foreground-faint flex min-h-screen items-center justify-center">
        {t('publicDashboard.loading')}
      </div>
    )
  }
  if (state === 'error' || !dash) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-2 p-8 text-center">
        <h1 className="text-xl font-bold">{t('publicDashboard.not_found_title')}</h1>
        <p className="text-foreground-faint">{t('publicDashboard.not_found_desc')}</p>
      </main>
    )
  }

  return (
    <main className="bg-background text-foreground min-h-screen p-6">
      <header className="mb-6">
        <h1 className="text-[1.8rem] font-bold">{dash.name}</h1>
        {dash.description && <p className="text-foreground-faint mt-0.5 text-[0.9rem]">{dash.description}</p>}
      </header>
      <div className="grid grid-cols-12 gap-6">
        {dash.widgets.map((widget) => (
          <div
            key={widget.id}
            className={cn(cardClass({ elevated: true }), 'relative flex flex-col p-5')}
            style={{ gridColumn: `span ${widget.w || 6}`, minHeight: `${widgetHeightPx(widget.h)}px` }}
          >
            <div className="mb-3 flex items-center justify-between border-b border-(--border-light,#f1f5f9) pb-1">
              <h2 className="text-foreground m-0 text-base font-semibold">{widget.title}</h2>
            </div>
            <div className="flex-1 overflow-hidden">
              <DashboardWidgetRenderer widget={widget} fetchData={(w) => runPublicWidget(token, w.id)} />
            </div>
          </div>
        ))}
      </div>
      <footer className="text-foreground-faint mt-8 text-center text-xs">
        {t('publicDashboard.powered_by')} <span className="font-semibold">biqly</span>
      </footer>
    </main>
  )
}
```

(Adjust the grid-span classes/utilities to compile with the repo's Tailwind v4 config; reuse the exact class strings from `DashboardBuilder.tsx:462-515` where they differ.)

- [ ] **Step 4: Route in `App.tsx`** — sibling of the `/auth` branch, BEFORE `path="*"`:

```tsx
const PublicDashboardPage = lazy(() => import('./components/public/PublicDashboardPage'))
```
```tsx
      <Route
        path="/public/dashboard/:token"
        element={
          <Suspense fallback={<LoadingScreen />}>
            <PublicDashboardPage />
          </Suspense>
        }
      />
```

- [ ] **Step 5: Run, commit**

Run: `npm --prefix frontend run test && make check-frontend` — PASS.

```bash
git add frontend/src/
git commit -m "feat(frontend): shell-less public dashboard page"
```

---

### Task 11: Frontend — share modal + workspace settings toggle

**Files:**
- Create: `frontend/src/api/dashboardShare.ts`, `frontend/src/components/dashboard/PublicShareModal.tsx`
- Modify: `frontend/src/components/DashboardBuilder.tsx:342-393` (Share button in the header actions div)
- Modify: `frontend/src/api/admin.ts:376-384` (`updateWorkspace` gains `publicSharingEnabled?: boolean` → body `public_sharing_enabled`)
- Modify: `frontend/src/components/workspaces/WorkspaceSettingsPage.tsx` (checkbox mirroring the MFA one at lines 60/91/122/297-300)
- Modify: `frontend/src/i18n/locales/en/core.ts` + `tr/core.ts` (share-modal keys), `en/admin.ts` + `tr/admin.ts` (workspace toggle label)
- Test: `frontend/src/components/dashboard/PublicShareModal.test.tsx`

**Interfaces:**
- Consumes: Task 6 endpoints.
- Produces:
  - `getDashboardPublicShare(id): Promise<{active: boolean; created_at?: string}>`, `createDashboardPublicShare(id): Promise<{token: string; url_path: string}>`, `revokeDashboardPublicShare(id): Promise<void>` (authenticated, via `apiFetch` with the global token)
  - `<PublicShareModal dashboardId={id} open={open} onClose={fn} />` — shows current status; create/rotate shows the full URL (`window.location.origin + url_path`) and an iframe snippet, each with a copy button; revoke turns it off; a 409 from create renders the "sharing disabled by admin" message
  - Workspace settings gains a `public_sharing_enabled` checkbox

- [ ] **Step 1: API client** (`dashboardShare.ts`):

```ts
import { apiFetch } from './apiClient'

export interface PublicShareStatus {
  active: boolean
  created_at?: string
  expires_at?: string | null
}

export interface CreatedPublicShare {
  token: string
  url_path: string
  created_at: string
}

export function getDashboardPublicShare(id: string): Promise<PublicShareStatus> {
  return apiFetch<PublicShareStatus>('GET', `/api/dashboards/${encodeURIComponent(id)}/public-share`)
}

export function createDashboardPublicShare(id: string): Promise<CreatedPublicShare> {
  return apiFetch<CreatedPublicShare>('POST', `/api/dashboards/${encodeURIComponent(id)}/public-share`, {})
}

export function revokeDashboardPublicShare(id: string): Promise<void> {
  return apiFetch<void>('DELETE', `/api/dashboards/${encodeURIComponent(id)}/public-share`)
}
```

- [ ] **Step 2: i18n keys** (`en/core.ts` under a new `publicShare` slice + Turkish mirror):

```ts
  publicShare: {
    title: 'Public link',
    description: 'Anyone with this link can view the dashboard. Data stays read-only.',
    enable: 'Create public link',
    rotate: 'Rotate link',
    revoke: 'Disable public link',
    copy_link: 'Copy link',
    copy_iframe: 'Copy iframe code',
    copied: 'Copied',
    disabled_by_admin: 'Public sharing is disabled for this workspace. Ask a workspace admin to enable it.',
    active_since: 'Public since {{date}}',
    token_notice: 'The link is shown once per rotation — copy it now.',
  },
```
And in `en/admin.ts` `workspaces` slice (+ `tr/admin.ts`): `public_sharing_enabled: 'Allow public dashboard links'`.

- [ ] **Step 3: Failing modal test**: status=inactive shows the enable button; clicking enable (mock create) shows link + iframe snippet containing `/public/dashboard/`; 409 rejection shows `disabled_by_admin`; active state shows rotate + revoke.

- [ ] **Step 4: Implement `PublicShareModal.tsx`** on the shared `Modal` component (`frontend/src/components/ui/Modal.tsx`, props `{open, title, children, onClose}`), using `useFetch`-style load of status on open, `navigator.clipboard.writeText` for copy, `buttonClass` helpers for buttons. Iframe snippet string:

```ts
const iframeSnippet = (url: string) =>
  `<iframe src="${url}" width="100%" height="600" frameborder="0" title="biqly dashboard"></iframe>`
```

- [ ] **Step 5: Wire the trigger** in `DashboardBuilder.tsx` non-edit branch of the header actions:

```tsx
<button type="button" className={buttonClass('secondary')} onClick={() => setShareOpen(true)}>
  {t('publicShare.title')}
</button>
```
plus `const [shareOpen, setShareOpen] = useState(false)` and `<PublicShareModal dashboardId={dashboardId} open={shareOpen} onClose={() => setShareOpen(false)} />`.

- [ ] **Step 6: Workspace settings toggle** — in `WorkspaceSettingsPage.tsx` clone the MFA checkbox block (lines 297-300) for `public_sharing_enabled`, add `editPublicSharing` state (init from `ws.public_sharing_enabled` at line 91), pass through `updateWorkspace` (extend `frontend/src/api/admin.ts:376` signature + body). Add `public_sharing_enabled?: boolean` to the workspace type in `frontend/src/types/auth.ts` (find the `mfa_required` field and mirror it).

- [ ] **Step 7: Run, commit**

Run: `npm --prefix frontend run test && make check-frontend` — PASS.

```bash
git add frontend/src/
git commit -m "feat(frontend): dashboard public-share modal and workspace kill-switch toggle"
```

---

### Task 12: nginx + Helm routing

**Files:**
- Modify: `frontend/nginx.conf` (new `/public/` location before `location /`)
- Modify: `deploy/helm/biqly/charts/frontend/config/default.conf` (same)
- Modify: `deploy/helm/biqly/charts/catalog/values.yaml` (route paths gain `/api/public/dashboards`)
- Modify: `deploy/helm/biqly/charts/query/values.yaml` (route paths gain `/api/public/widget-query`)

**Interfaces:**
- Consumes: route topology from Tasks 7–8, page from Task 10.
- Produces: iframe-embeddable `/public/dashboard/*` page; gateway routes public API prefixes to the right services.

- [ ] **Step 1: nginx blocks** — add to BOTH files, ABOVE the existing `location /` block (embeddable SPA shell; everything else keeps `DENY`):

```nginx
    # Public dashboard embeds: same SPA, framing allowed, never indexed.
    location /public/ {
        add_header X-Content-Type-Options "nosniff" always; # nosemgrep: generic.nginx.security.header-x-frame-options.header-x-frame-options
        add_header Content-Security-Policy "frame-ancestors *" always; # nosemgrep: generic.nginx.security.header-x-frame-options.header-x-frame-options
        add_header Referrer-Policy "strict-origin-when-cross-origin" always; # nosemgrep: generic.nginx.security.header-x-frame-options.header-x-frame-options
        add_header X-Robots-Tag "noindex, nofollow" always; # nosemgrep: generic.nginx.security.header-x-frame-options.header-x-frame-options
        try_files $uri $uri/ /index.html;
    }
```
(Copy the exact `# nosemgrep:` comment style used on the sibling blocks in each file — check what rule IDs they reference and reuse them.)

- [ ] **Step 2: Helm route paths.** In each service's values file, find the existing HTTPRoute `paths`/`matches` list (e.g. catalog lists `/api/dashboards`) and append `/api/public/dashboards` (catalog) and `/api/public/widget-query` (query). Follow the exact YAML shape already present in the file.

- [ ] **Step 3: Rebuild chart deps and template-check**

Run: `helm dependency build deploy/helm/biqly && helm template biqly deploy/helm/biqly -f deploy/helm/biqly/values-dev.yaml | grep -A3 'api/public'`
Expected: both new path prefixes appear in the rendered HTTPRoutes.

- [ ] **Step 4: Commit**

```bash
git add frontend/nginx.conf deploy/helm/biqly/
git commit -m "feat(deploy): route and frame-header config for public dashboard embeds"
```

---

### Task 13: End-to-end verification

**Files:**
- Create (scratch only, not committed): an iframe test page in the session scratchpad.

- [ ] **Step 1: Boot the stack**

Run: `make dev-up` (already running), `make watch` (api `:8888` + auth `:8889` + mail), `make dev-frontend` (Vite `:3333`).

- [ ] **Step 2: Manual API flow** (with a real signed-in token from the dev UI):
  1. Enable the kill-switch: workspace settings toggle in the UI (or `PUT /api/auth/workspaces/{id}` with `{"public_sharing_enabled":true}`).
  2. Create a dashboard with one chart widget in the UI; `POST /api/dashboards/{id}/public-share` → capture `token`.
  3. `curl -i http://localhost:3333/api/public/dashboards/<token>` → 200, body has widgets, NO `logical_query` substring, headers include `Content-Security-Policy: frame-ancestors *` and no `X-Frame-Options`.
  4. `curl -i -X POST http://localhost:3333/api/public/widget-query/<token>/<widgetId>` → 200 rows.
  5. `curl -i http://localhost:3333/api/public/dashboards/WRONG` → 404; disable kill-switch → step 3 also 404 with an identical body; re-enable.
  6. Open `http://localhost:3333/public/dashboard/<token>` in a browser — dashboard renders with charts, no sidebar; sign out and reload — still renders.

- [ ] **Step 3: iframe smoke test** — write to the scratchpad and open:

```html
<!doctype html><html><body style="background:#eee">
<h1>Host app</h1>
<iframe src="http://localhost:3333/public/dashboard/TOKEN_HERE" width="100%" height="600" frameborder="0" title="biqly dashboard"></iframe>
</body></html>
```
Serve it from a DIFFERENT origin (`python3 -m http.server 8000` in the scratchpad dir) and confirm the dashboard renders inside the iframe.

- [ ] **Step 4: Full gate**

Run: `make precommit` and `make verify-main` — both PASS. Fix anything they surface before declaring done.

---

## Self-Review Notes

- **Spec deviation (deliberate, single):** the spec said the query service fetches widget definitions from a catalog internal endpoint; exploration showed the query service already owns a `bi_metadata` pool, so both services use `dashboard.PublicResolver` directly. One fewer network hop, one fewer internal endpoint, identical security properties. Flagged to the user at plan presentation.
- **Spec coverage check:** data model → T1/T2; management API → T6; public metadata + sanitize + uniform 404 + kill-switch → T3/T7; server-side execution + cache + rate limit → T5/T8; header relaxation (Go + nginx) → T5/T7/T8/T12; frontend route/page/renderer/modal/toggle → T9/T10/T11; Helm/gateway → T12; tests woven through; e2e + `make verify-main` → T13. Audit events → T6. Robots noindex → T5 (API) + T12 (page). `X-Robots-Tag` on the page served via nginx block.
- **Type consistency:** `PublicResolver.ResolveWidgetQuery` returns `*PublicWidgetQuery` consumed by T8 handler; `fetchData` prop type in T9 matches T10 usage; `workspaceSharingChecker` defined once in T6, reused in T7/T8 (same package).
