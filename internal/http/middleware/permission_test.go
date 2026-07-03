package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

type authStub struct {
	server         *httptest.Server
	permCalls      atomic.Int32
	dsCalls        atomic.Int32
	permAllowed    bool
	dsAllowed      bool
	permStatusCode int
	dsStatusCode   int
	lastPermBody   []byte
	lastDSBody     []byte
}

func newAuthStub(t *testing.T) *authStub {
	t.Helper()
	s := &authStub{
		permAllowed:    true,
		dsAllowed:      true,
		permStatusCode: http.StatusOK,
		dsStatusCode:   http.StatusOK,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/check-permission", func(w http.ResponseWriter, r *http.Request) {
		s.permCalls.Add(1)
		var err error
		s.lastPermBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		if s.permStatusCode != http.StatusOK {
			w.WriteHeader(s.permStatusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, sonic.ConfigStd.NewEncoder(w).Encode(map[string]bool{"allowed": s.permAllowed}))
	})
	mux.HandleFunc("/internal/auth/check-datasource-access", func(w http.ResponseWriter, r *http.Request) {
		s.dsCalls.Add(1)
		var err error
		s.lastDSBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		if s.dsStatusCode != http.StatusOK {
			w.WriteHeader(s.dsStatusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, sonic.ConfigStd.NewEncoder(w).Encode(map[string]bool{"allowed": s.dsAllowed}))
	})
	s.server = httptest.NewServer(mux)
	return s
}

func (s *authStub) Close() { s.server.Close() }

func ctxWithUser(roles []string) context.Context {
	ctx := context.WithValue(context.Background(), UserIDKey, "u1")
	ctx = context.WithValue(ctx, UserRolesKey, roles)
	return ctx
}

func TestRequirePermission_NilClientPassThrough(t *testing.T) {
	mw := RequirePermission(nil, "query:execute")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nil client should pass through, got %d", w.Code)
	}
}

func TestRequirePermission_SuperAdminBypass(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequirePermission(client, "query:execute")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req = req.WithContext(ctxWithUser([]string{RoleSuperAdmin}))
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("super_admin should bypass: %d", w.Code)
	}
	if stub.permCalls.Load() != 0 {
		t.Fatalf("super_admin should not call auth service, got %d calls", stub.permCalls.Load())
	}
}

func TestRequirePermission_NoUserUnauthorized(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequirePermission(client, "query:execute")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequirePermission_AllowedAndCached(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	stub.permAllowed = true

	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequirePermission(client, "query:execute")

	for range 3 {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req = req.WithContext(ctxWithUser([]string{"analyst"}))
		w := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	}
	if got := stub.permCalls.Load(); got != 1 {
		t.Fatalf("expected 1 auth service call (cached), got %d", got)
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	stub.permAllowed = false

	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequirePermission(client, "model:publish")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req = req.WithContext(ctxWithUser([]string{"viewer"}))
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "model:publish") {
		t.Fatalf("error should name the permission, got %q", w.Body.String())
	}
}

func assertMiddlewareUpstreamError(t *testing.T, configure func(*authStub), mw func(*AuthClient) func(http.Handler) http.Handler, path string) {
	t.Helper()
	stub := newAuthStub(t)
	defer stub.Close()
	configure(stub)
	client := NewAuthClient(stub.server.URL, "tok")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	mw(client)(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestRequirePermission_UpstreamError(t *testing.T) {
	assertMiddlewareUpstreamError(t, func(stub *authStub) {
		stub.permStatusCode = http.StatusInternalServerError
	}, func(client *AuthClient) func(http.Handler) http.Handler {
		return RequirePermission(client, "query:execute")
	}, "/x")
}

func TestRequirePermission_PropagatesWorkspaceScope(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequirePermission(client, "query:execute")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	ctx := ctxWithUser([]string{"analyst"})
	ctx = context.WithValue(ctx, WorkspaceIDKey, "ws-42")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)

	var body map[string]string
	if err := sonic.ConfigStd.Unmarshal(stub.lastPermBody, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["scope_type"] != "workspace" || body["scope_id"] != "ws-42" {
		t.Fatalf("unexpected scope: %+v", body)
	}
}

func TestRequireDatasourceAccess_NilClientPassThrough(t *testing.T) {
	mw := RequireDatasourceAccess(nil, "read")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nil client should pass through, got %d", w.Code)
	}
}

func TestRequireDatasourceAccess_SuperAdminBypass(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")

	r := chi.NewRouter()
	r.With(RequireDatasourceAccess(client, "read")).Get("/ds/{datasourceID}", okHandler().ServeHTTP)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ds/abc", nil)
	req = req.WithContext(ctxWithUser([]string{RoleSuperAdmin}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("super_admin should bypass: %d", w.Code)
	}
	if stub.dsCalls.Load() != 0 {
		t.Fatalf("super_admin should not call auth service, got %d", stub.dsCalls.Load())
	}
}

func TestRequireDatasourceAccess_NoUser(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")

	r := chi.NewRouter()
	r.With(RequireDatasourceAccess(client, "read")).Get("/ds/{datasourceID}", okHandler().ServeHTTP)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ds/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireDatasourceAccess_MissingDatasourceID(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequireDatasourceAccess(client, "read")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no datasource id, got %d", w.Code)
	}
}

func TestExtractDatasourceIDFromNestedLogicalQuery(t *testing.T) {
	body := `{"logical_query":{"datasource_id":"ds9"}}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if got := extractDatasourceID(req); got != "ds9" {
		t.Fatalf("extractDatasourceID nested = %q, want ds9", got)
	}
	// The probe must restore the body for the downstream handler.
	rest, _ := io.ReadAll(req.Body)
	if string(rest) != body {
		t.Fatalf("body not restored: %q", string(rest))
	}
}

func TestRequireResolvedDatasourceAccess_AllowedThenDenied(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	resolve := func(_ context.Context, id string) (string, error) { return "ds-for-" + id, nil }

	r := chi.NewRouter()
	r.With(RequireResolvedDatasourceAccess(client, "write", resolve)).Get("/models/{id}", okHandler().ServeHTTP)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/models/m1", nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when access allowed, got %d", w.Code)
	}

	// Use a distinct model id so the auth client's per-datasource cache doesn't
	// serve the earlier "allowed" result.
	stub.dsAllowed = false
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/models/m2", nil)
	req2 = req2.WithContext(ctxWithUser([]string{"analyst"}))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when access denied, got %d", w2.Code)
	}
}

func TestRequireResolvedDatasourceAccess_ResolverErrorIsNotFound(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	resolve := func(context.Context, string) (string, error) { return "", errors.New("no such model") }

	r := chi.NewRouter()
	r.With(RequireResolvedDatasourceAccess(client, "read", resolve)).Get("/models/{id}", okHandler().ServeHTTP)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/models/x", nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on resolver error, got %d", w.Code)
	}
}

func TestRequireResolvedDatasourceAccess_NilClientPassThrough(t *testing.T) {
	mw := RequireResolvedDatasourceAccess(nil, "read", func(context.Context, string) (string, error) {
		return "", nil
	})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nil client should pass through, got %d", w.Code)
	}
}

func TestRequireDatasourceAccess_FromURLParam(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")

	r := chi.NewRouter()
	r.With(RequireDatasourceAccess(client, "read")).Get("/ds/{datasourceID}", okHandler().ServeHTTP)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ds/ds-1", nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := sonic.ConfigStd.Unmarshal(stub.lastDSBody, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["datasource_id"] != "ds-1" || body["level"] != "read" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestRequireDatasourceAccess_FromQueryString(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequireDatasourceAccess(client, "write")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x?datasource_id=qs-1", nil)
	req = req.WithContext(ctxWithUser([]string{"developer"}))
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	require.NoError(t, sonic.ConfigStd.Unmarshal(stub.lastDSBody, &body))
	if body["datasource_id"] != "qs-1" {
		t.Fatalf("expected datasource_id=qs-1, got %+v", body)
	}
}

func TestRequireDatasourceAccess_FromJSONBodyAndRestores(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")

	type payload struct {
		DatasourceID string `json:"datasource_id"`
		Question     string `json:"question"`
	}

	captured := make(chan payload, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "decode failed", http.StatusBadRequest)
			return
		}
		captured <- p
		w.WriteHeader(http.StatusOK)
	})

	mw := RequireDatasourceAccess(client, "read")
	wrapped := mw(handler)

	body := payload{DatasourceID: "body-1", Question: "merhaba"}
	buf, err := sonic.ConfigStd.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/query", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	select {
	case got := <-captured:
		if got.DatasourceID != "body-1" || got.Question != "merhaba" {
			t.Fatalf("body lost after middleware: %+v", got)
		}
	default:
		t.Fatal("handler never decoded body")
	}
}

func TestRequireDatasourceAccess_Denied(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	stub.dsAllowed = false

	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequireDatasourceAccess(client, "read")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x?datasource_id=ds-1", nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireDatasourceAccess_UpstreamError(t *testing.T) {
	assertMiddlewareUpstreamError(t, func(stub *authStub) {
		stub.dsStatusCode = http.StatusBadGateway
	}, func(client *AuthClient) func(http.Handler) http.Handler {
		return RequireDatasourceAccess(client, "read")
	}, "/x?datasource_id=ds-1")
}

func TestRequireDatasourceAccess_CachedAcrossCalls(t *testing.T) {
	stub := newAuthStub(t)
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequireDatasourceAccess(client, "read")

	for range 4 {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x?datasource_id=same-ds", nil)
		req = req.WithContext(ctxWithUser([]string{"analyst"}))
		w := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	}
	if got := stub.dsCalls.Load(); got != 1 {
		t.Fatalf("expected single auth call (cached), got %d", got)
	}
}

func TestAuthClient_ListUserDatasources(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/datasources", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string][]string{"datasource_ids": {"a", "b", "c"}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	got, err := client.ListUserDatasources(context.Background(), "u1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("unexpected list: %+v", got)
	}
}

func TestAuthClient_ListWorkspaceDatasources_Caches(t *testing.T) {
	var calls int
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/workspaces/ws1/datasources", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		calls++
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string][]string{"datasource_ids": {"d1", "d2"}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	for i := range 3 {
		got, err := client.ListWorkspaceDatasources(context.Background(), "ws1")
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if len(got) != 2 || got[0] != "d1" {
			t.Fatalf("call %d: unexpected list %+v", i, got)
		}
	}
	if calls != 1 {
		t.Fatalf("expected 1 upstream call (cached), got %d", calls)
	}

	client.InvalidateWorkspaceDatasourceCache("ws1")
	if _, err := client.ListWorkspaceDatasources(context.Background(), "ws1"); err != nil {
		t.Fatalf("post-invalidate: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected refetch after invalidation, got %d total calls", calls)
	}
}

func TestAuthClient_ListWorkspaceDatasources_EmptyWorkspace(t *testing.T) {
	client := NewAuthClient("http://unused", "tok")
	got, err := client.ListWorkspaceDatasources(context.Background(), "")
	if err != nil {
		t.Fatalf("empty workspace: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for empty workspace, got %+v", got)
	}
}

func TestSharedAuthClient_ReturnsSameInstance(t *testing.T) {
	c1 := SharedAuthClient("http://auth:8080", "tok-1")
	c2 := SharedAuthClient("http://auth:8080", "tok-1")
	if c1 != c2 {
		t.Fatal("SharedAuthClient should return the same instance for identical args")
	}
	// Different token should create a different client.
	c3 := SharedAuthClient("http://auth:8080", "tok-2")
	if c1 == c3 {
		t.Fatal("SharedAuthClient should return different instance for different token")
	}
}

func TestSharedAuthClient_URLTrimmed(t *testing.T) {
	c1 := SharedAuthClient("http://auth:8080/", "tok")
	c2 := SharedAuthClient("http://auth:8080", "tok")
	if c1 != c2 {
		t.Fatal("SharedAuthClient should normalize trailing slash in URL")
	}
}

func TestAuthClient_UserAIAccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-access", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"restricted":   true,
			"model_ids":    []string{"gpt-4", "claude-3"},
			"provider_ids": []string{"openai"},
			"preferences":  map[string]string{"model": "gpt-4"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	access, err := client.UserAIAccess(context.Background(), "u1")
	if err != nil {
		t.Fatalf("UserAIAccess: %v", err)
	}
	if access == nil {
		t.Fatal("expected non-nil UserAIAccess")
	}
	if !access.Restricted {
		t.Fatal("expected restricted=true")
	}
	if len(access.ModelIDs) != 2 || access.ModelIDs[0] != "gpt-4" {
		t.Fatalf("unexpected model_ids: %+v", access.ModelIDs)
	}
	if len(access.ProviderIDs) != 1 || access.ProviderIDs[0] != "openai" {
		t.Fatalf("unexpected provider_ids: %+v", access.ProviderIDs)
	}
	if access.Preferences["model"] != "gpt-4" {
		t.Fatalf("unexpected preferences: %+v", access.Preferences)
	}
}

func TestAuthClient_UserAIAccess_EmptyUserID(t *testing.T) {
	client := NewAuthClient("http://unused", "tok")
	access, err := client.UserAIAccess(context.Background(), "")
	if err != nil {
		t.Fatalf("UserAIAccess(''): %v", err)
	}
	if access != nil {
		t.Fatal("expected nil for empty userID")
	}
}

func TestAuthClient_UserAIAccess_Non200Status(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-access", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	_, err := client.UserAIAccess(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestAuthClient_UserAIAccess_NilPreferences(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-access", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"restricted":   false,
			"model_ids":    []string{},
			"provider_ids": []string{},
			"preferences":  nil,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	access, err := client.UserAIAccess(context.Background(), "u1")
	if err != nil {
		t.Fatalf("UserAIAccess: %v", err)
	}
	if access.Preferences == nil {
		t.Fatal("expected non-nil (empty) preferences map, got nil")
	}
}

func TestAuthClient_ListUserAIPreferences(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"preferences": []map[string]string{
				{"purpose": "chat", "model_id": "gpt-4"},
				{"purpose": "code", "model_id": "claude-3"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	prefs, err := client.ListUserAIPreferences(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ListUserAIPreferences: %v", err)
	}
	if len(prefs) != 2 {
		t.Fatalf("expected 2 preferences, got %d", len(prefs))
	}
	if prefs[0].Purpose != "chat" || prefs[0].ModelID != "gpt-4" {
		t.Fatalf("unexpected pref[0]: %+v", prefs[0])
	}
	if prefs[1].Purpose != "code" || prefs[1].ModelID != "claude-3" {
		t.Fatalf("unexpected pref[1]: %+v", prefs[1])
	}
}

func TestAuthClient_ListUserAIPreferences_EmptyUserID(t *testing.T) {
	client := NewAuthClient("http://unused", "tok")
	prefs, err := client.ListUserAIPreferences(context.Background(), "")
	if err != nil {
		t.Fatalf("ListUserAIPreferences(''): %v", err)
	}
	if prefs != nil {
		t.Fatal("expected nil for empty userID")
	}
}

func TestAuthClient_ListUserAIPreferences_Non200Status(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	_, err := client.ListUserAIPreferences(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestAuthClient_SetUserAIPreference(t *testing.T) {
	var gotMethod, gotToken, gotContentType string
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotToken = r.Header.Get("X-Internal-Token")
		gotContentType = r.Header.Get("Content-Type")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	err := client.SetUserAIPreference(context.Background(), "u1", "chat", "gpt-4")
	if err != nil {
		t.Fatalf("SetUserAIPreference: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
	if gotToken != "tok" {
		t.Fatalf("expected token 'tok', got %q", gotToken)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", gotContentType)
	}
	var bodyMap map[string]string
	require.NoError(t, sonic.ConfigStd.Unmarshal(gotBody, &bodyMap))
	if bodyMap["purpose"] != "chat" || bodyMap["model_id"] != "gpt-4" {
		t.Fatalf("unexpected body: %+v", bodyMap)
	}
}

func TestAuthClient_SetUserAIPreference_NoContentSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	err := client.SetUserAIPreference(context.Background(), "u1", "chat", "gpt-4")
	if err != nil {
		t.Fatalf("SetUserAIPreference(204): %v", err)
	}
}

func TestAuthClient_SetUserAIPreference_ErrorStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	err := client.SetUserAIPreference(context.Background(), "u1", "chat", "gpt-4")
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
}

func TestAuthClient_DeleteUserAIPreference(t *testing.T) {
	var gotMethod, gotToken string
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences/chat", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotToken = r.Header.Get("X-Internal-Token")
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	err := client.DeleteUserAIPreference(context.Background(), "u1", "chat")
	if err != nil {
		t.Fatalf("DeleteUserAIPreference: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	if gotToken != "tok" {
		t.Fatalf("expected token 'tok', got %q", gotToken)
	}
}

func TestAuthClient_DeleteUserAIPreference_NoContentSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences/chat", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	err := client.DeleteUserAIPreference(context.Background(), "u1", "chat")
	if err != nil {
		t.Fatalf("DeleteUserAIPreference(204): %v", err)
	}
}

func TestAuthClient_DeleteUserAIPreference_ErrorStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1/ai-preferences/chat", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	err := client.DeleteUserAIPreference(context.Background(), "u1", "chat")
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
}

func TestAuthClient_GetUserEmail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string]string{"email": "user@example.com"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	email, err := client.GetUserEmail(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetUserEmail: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("expected 'user@example.com', got %q", email)
	}
}

func TestAuthClient_GetUserEmail_Non200Status(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/user/u1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewAuthClient(srv.URL, "tok")
	_, err := client.GetUserEmail(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
