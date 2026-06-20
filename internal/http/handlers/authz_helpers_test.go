package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestAIJobOwnedBy(t *testing.T) {
	tests := []struct {
		name      string
		job       *metadata.AIJob
		userID    string
		sessionID string
		want      bool
	}{
		{
			name:      "matches by user id",
			job:       &metadata.AIJob{UserID: strPtr("u-1")},
			userID:    "u-1",
			sessionID: "",
			want:      true,
		},
		{
			name:      "matches by client session id for legacy jobs",
			job:       &metadata.AIJob{ClientSessionID: "sess-1"},
			userID:    "",
			sessionID: "sess-1",
			want:      true,
		},
		{
			name:      "does not match different user id",
			job:       &metadata.AIJob{UserID: strPtr("u-1")},
			userID:    "u-other",
			sessionID: "",
			want:      false,
		},
		{
			name:      "does not match different session id",
			job:       &metadata.AIJob{ClientSessionID: "sess-1"},
			userID:    "",
			sessionID: "sess-other",
			want:      false,
		},
		{
			name:      "empty user id and session id never matches",
			job:       &metadata.AIJob{UserID: strPtr("u-1"), ClientSessionID: "sess-1"},
			userID:    "",
			sessionID: "",
			want:      false,
		},
		{
			name:      "nil user id pointer falls back to session id",
			job:       &metadata.AIJob{ClientSessionID: "sess-1"},
			userID:    "u-1",
			sessionID: "sess-1",
			want:      true,
		},
		{
			name:      "nil user id pointer with no session match",
			job:       &metadata.AIJob{ClientSessionID: "sess-1"},
			userID:    "u-1",
			sessionID: "sess-other",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aiJobOwnedBy(tt.job, tt.userID, tt.sessionID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCanViewAIHistoryDetails(t *testing.T) {
	t.Run("nil auth client allows viewing", func(t *testing.T) {
		ctx := ctxWithIdentity("u-1")
		assert.True(t, canViewAIHistoryDetails(ctx, nil, "u-1"))
	})

	t.Run("super admin bypasses permission check", func(t *testing.T) {
		ctx := ctxWithIdentity("u-admin", bimw.RoleSuperAdmin)
		// Non-nil auth client, but super admin should short-circuit before calling it.
		assert.True(t, canViewAIHistoryDetails(ctx, &bimw.AuthClient{}, "u-admin"))
	})

	t.Run("empty user id denies", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, canViewAIHistoryDetails(ctx, &bimw.AuthClient{}, ""))
	})

	t.Run("permission granted via auth service", func(t *testing.T) {
		srv := newFakeAuthPermissionServer(t, true)
		defer srv.Close()

		client := bimw.NewAuthClient(srv.URL, "internal-token")
		ctx := ctxWithIdentity("u-1")
		ctx = bimw.WithWorkspaceID(ctx, "ws-1")
		assert.True(t, canViewAIHistoryDetails(ctx, client, "u-1"))
	})

	t.Run("permission denied via auth service", func(t *testing.T) {
		srv := newFakeAuthPermissionServer(t, false)
		defer srv.Close()

		client := bimw.NewAuthClient(srv.URL, "internal-token")
		ctx := ctxWithIdentity("u-1")
		ctx = bimw.WithWorkspaceID(ctx, "ws-1")
		assert.False(t, canViewAIHistoryDetails(ctx, client, "u-1"))
	})

	t.Run("auth service error denies (fail closed)", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := bimw.NewAuthClient(srv.URL, "internal-token")
		ctx := ctxWithIdentity("u-1")
		ctx = bimw.WithWorkspaceID(ctx, "ws-1")
		assert.False(t, canViewAIHistoryDetails(ctx, client, "u-1"))
	})
}

func TestPIIAccessForColumn(t *testing.T) {
	column := metadata.Column{
		SchemaName: "public",
		TableName:  "customers",
		ColumnName: "email",
	}

	t.Run("nil config returns not found", func(t *testing.T) {
		_, _, ok := piiAccessForColumn(nil, column)
		assert.False(t, ok)
	})

	t.Run("matches fully qualified ref via ColumnInfo", func(t *testing.T) {
		cfg := &query.PIIMaskingConfig{
			ColumnInfo: map[string]query.PIIColumnInfo{
				"public.customers.email": {Access: "masked", PIIType: "email", Strategy: "partial"},
			},
		}
		access, piiType, ok := piiAccessForColumn(cfg, column)
		require.True(t, ok)
		assert.Equal(t, "masked", access)
		assert.Equal(t, "email", piiType)
	})

	t.Run("matches table.column ref via ColumnInfo", func(t *testing.T) {
		cfg := &query.PIIMaskingConfig{
			ColumnInfo: map[string]query.PIIColumnInfo{
				"customers.email": {Access: "raw", PIIType: "email", Strategy: "partial"},
			},
		}
		access, _, ok := piiAccessForColumn(cfg, column)
		require.True(t, ok)
		assert.Equal(t, "raw", access)
	})

	t.Run("matches short column name ref via ColumnInfo", func(t *testing.T) {
		cfg := &query.PIIMaskingConfig{
			ColumnInfo: map[string]query.PIIColumnInfo{
				"email": {Access: "hidden", PIIType: "email", Strategy: "full"},
			},
		}
		access, _, ok := piiAccessForColumn(cfg, column)
		require.True(t, ok)
		assert.Equal(t, "hidden", access)
	})

	t.Run("ColumnAccess masked with full strategy folds to hidden", func(t *testing.T) {
		cfg := &query.PIIMaskingConfig{
			ColumnAccess:     map[string]string{"customers.email": "masked"},
			ColumnTypes:      map[string]string{"customers.email": "email"},
			ColumnStrategies: map[string]string{"customers.email": "full"},
		}
		access, piiType, ok := piiAccessForColumn(cfg, column)
		require.True(t, ok)
		assert.Equal(t, pii.AccessHidden, access, "masked+full strategy folds to hidden")
		assert.Equal(t, "email", piiType)
	})

	t.Run("unknown column returns not found", func(t *testing.T) {
		cfg := &query.PIIMaskingConfig{
			ColumnInfo:   map[string]query.PIIColumnInfo{},
			ColumnAccess: map[string]string{},
			ColumnTypes:  map[string]string{},
		}
		_, _, ok := piiAccessForColumn(cfg, column)
		assert.False(t, ok)
	})

	t.Run("ColumnInfo takes precedence over ColumnAccess", func(t *testing.T) {
		cfg := &query.PIIMaskingConfig{
			ColumnInfo: map[string]query.PIIColumnInfo{
				"customers.email": {Access: "hidden", PIIType: "email", Strategy: "full"},
			},
			ColumnAccess: map[string]string{"customers.email": "raw"},
		}
		access, _, ok := piiAccessForColumn(cfg, column)
		require.True(t, ok)
		assert.Equal(t, "hidden", access, "ColumnInfo should win over ColumnAccess")
	})
}

// newFakeAuthPermissionServer returns an httptest server that responds to
// /internal/auth/check-permission with the given allowed value.
func newFakeAuthPermissionServer(t *testing.T, allowed bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		out, _ := sonic.Marshal(map[string]bool{"allowed": allowed})
		_, _ = w.Write(out)
	}))
}
