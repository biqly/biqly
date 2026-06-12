package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

func TestResolveDatasourceScope_AuthDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = false

	ctx := context.Background()
	got, scoped, err := resolveDatasourceScope(ctx, cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scoped {
		t.Error("expected scoped to be false")
	}
	if got != nil {
		t.Errorf("expected nil map, got %v", got)
	}
}

func TestResolveDatasourceScope_NoUser(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true

	ctx := context.Background()
	got, scoped, err := resolveDatasourceScope(ctx, cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scoped {
		t.Error("expected scoped to be false")
	}
	if got != nil {
		t.Errorf("expected nil map, got %v", got)
	}
}

func TestResolveDatasourceScope_SuperAdmin(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-1")
	ctx = context.WithValue(ctx, bimw.UserRolesKey, []string{bimw.RoleSuperAdmin})

	got, scoped, err := resolveDatasourceScope(ctx, cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scoped {
		t.Error("expected scoped to be false")
	}
	if got != nil {
		t.Errorf("expected nil map, got %v", got)
	}
}

func TestResolveDatasourceScope_MissingWorkspace_Required(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-1")

	got, scoped, err := resolveDatasourceScope(ctx, cfg, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scoped {
		t.Error("expected scoped to be false")
	}
	if got != nil {
		t.Errorf("expected nil map, got %v", got)
	}
}

func TestResolveDatasourceScope_UserOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/internal/auth/user/user-1/datasources" {
			w.WriteHeader(http.StatusOK)
			_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string][]string{
				"datasource_ids": {"ds-1", "ds-2"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.ServiceURL = server.URL
	cfg.Auth.InternalToken = "secret"

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-1")

	got, scoped, err := resolveDatasourceScope(ctx, cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scoped {
		t.Error("expected scoped to be true")
	}
	expected := map[string]struct{}{
		"ds-1": {},
		"ds-2": {},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestResolveDatasourceScope_WorkspaceIntersect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/internal/auth/user/user-1/datasources" {
			w.WriteHeader(http.StatusOK)
			_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string][]string{
				"datasource_ids": {"ds-1", "ds-2", "ds-3"},
			})
			return
		}
		if req.URL.Path == "/internal/auth/workspaces/ws-1/datasources" {
			w.WriteHeader(http.StatusOK)
			_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string][]string{
				"datasource_ids": {"ds-2", "ds-3", "ds-4"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.ServiceURL = server.URL
	cfg.Auth.InternalToken = "secret"

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-1")
	ctx = context.WithValue(ctx, bimw.WorkspaceIDKey, "ws-1")

	got, scoped, err := resolveDatasourceScope(ctx, cfg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scoped {
		t.Error("expected scoped to be true")
	}
	expected := map[string]struct{}{
		"ds-2": {},
		"ds-3": {},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestResolveDatasourceScope_ListUserError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.ServiceURL = server.URL
	cfg.Auth.InternalToken = "secret"

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-1")

	_, _, err := resolveDatasourceScope(ctx, cfg, false)
	if err == nil {
		t.Error("expected error but got nil")
	}
}

func TestResolveDatasourceScope_ListWorkspaceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/internal/auth/user/user-1/datasources" {
			w.WriteHeader(http.StatusOK)
			_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string][]string{
				"datasource_ids": {"ds-1"},
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.ServiceURL = server.URL
	cfg.Auth.InternalToken = "secret"

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "user-1")
	ctx = context.WithValue(ctx, bimw.WorkspaceIDKey, "ws-1")

	_, _, err := resolveDatasourceScope(ctx, cfg, false)
	if err == nil {
		t.Error("expected error but got nil")
	}
}
