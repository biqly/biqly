package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
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

func newAuthStub() *authStub {
	s := &authStub{
		permAllowed:    true,
		dsAllowed:      true,
		permStatusCode: http.StatusOK,
		dsStatusCode:   http.StatusOK,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/check-permission", func(w http.ResponseWriter, r *http.Request) {
		s.permCalls.Add(1)
		s.lastPermBody, _ = io.ReadAll(r.Body)
		if s.permStatusCode != http.StatusOK {
			w.WriteHeader(s.permStatusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"allowed": s.permAllowed})
	})
	mux.HandleFunc("/internal/auth/check-datasource-access", func(w http.ResponseWriter, r *http.Request) {
		s.dsCalls.Add(1)
		s.lastDSBody, _ = io.ReadAll(r.Body)
		if s.dsStatusCode != http.StatusOK {
			w.WriteHeader(s.dsStatusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"allowed": s.dsAllowed})
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
	stub := newAuthStub()
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
	stub := newAuthStub()
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
	stub := newAuthStub()
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
	stub := newAuthStub()
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

func TestRequirePermission_UpstreamError(t *testing.T) {
	stub := newAuthStub()
	defer stub.Close()
	stub.permStatusCode = http.StatusInternalServerError

	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequirePermission(client, "query:execute")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestRequirePermission_PropagatesWorkspaceScope(t *testing.T) {
	stub := newAuthStub()
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
	if err := json.Unmarshal(stub.lastPermBody, &body); err != nil {
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
	stub := newAuthStub()
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
	stub := newAuthStub()
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
	stub := newAuthStub()
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

func TestRequireDatasourceAccess_FromURLParam(t *testing.T) {
	stub := newAuthStub()
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
	if err := json.Unmarshal(stub.lastDSBody, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["datasource_id"] != "ds-1" || body["level"] != "read" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestRequireDatasourceAccess_FromQueryString(t *testing.T) {
	stub := newAuthStub()
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
	_ = json.Unmarshal(stub.lastDSBody, &body)
	if body["datasource_id"] != "qs-1" {
		t.Fatalf("expected datasource_id=qs-1, got %+v", body)
	}
}

func TestRequireDatasourceAccess_FromJSONBodyAndRestores(t *testing.T) {
	stub := newAuthStub()
	defer stub.Close()
	client := NewAuthClient(stub.server.URL, "tok")

	type payload struct {
		DatasourceID string `json:"datasource_id"`
		Question     string `json:"question"`
	}

	captured := make(chan payload, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "decode failed", http.StatusBadRequest)
			return
		}
		captured <- p
		w.WriteHeader(http.StatusOK)
	})

	mw := RequireDatasourceAccess(client, "read")
	wrapped := mw(handler)

	body := payload{DatasourceID: "body-1", Question: "merhaba"}
	buf, _ := json.Marshal(body)
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
	stub := newAuthStub()
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
	stub := newAuthStub()
	defer stub.Close()
	stub.dsStatusCode = http.StatusBadGateway

	client := NewAuthClient(stub.server.URL, "tok")
	mw := RequireDatasourceAccess(client, "read")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x?datasource_id=ds-1", nil)
	req = req.WithContext(ctxWithUser([]string{"analyst"}))
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestRequireDatasourceAccess_CachedAcrossCalls(t *testing.T) {
	stub := newAuthStub()
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
		_ = json.NewEncoder(w).Encode(map[string][]string{"datasource_ids": {"a", "b", "c"}})
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
		_ = json.NewEncoder(w).Encode(map[string][]string{"datasource_ids": {"d1", "d2"}})
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
