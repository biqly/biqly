package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListOpenAICompatibleModels(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth = %q", got)
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o","owned_by":"openai"},{"id":"gpt-4o-mini"}]}`))
	}))
	t.Cleanup(srv.Close)

	models, err := ListRemoteModelsFromEndpoint(context.Background(), "openai-compatible", srv.URL+"/v1", "test-key", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "gpt-4o" || models[1].ID != "gpt-4o-mini" {
		t.Fatalf("models = %+v", models)
	}
}

func TestListAnthropicModels(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "anthropic-key" {
			t.Fatalf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("missing anthropic-version")
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-4-20250514","type":"model"},{"id":"claude-3-5-haiku-20241022","type":"model"}]}`))
	}))
	t.Cleanup(srv.Close)

	models, err := ListRemoteModelsFromEndpoint(context.Background(), "anthropic", srv.URL+"/v1", "anthropic-key", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %+v", models)
	}
	ids := map[string]bool{models[0].ID: true, models[1].ID: true}
	if !ids["claude-sonnet-4-20250514"] || !ids["claude-3-5-haiku-20241022"] {
		t.Fatalf("models = %+v", models)
	}
}

func TestListAnthropicModelsRequiresAPIKey(t *testing.T) {
	t.Parallel()
	_, err := ListRemoteModelsFromEndpoint(context.Background(), "anthropic", "https://api.anthropic.com/v1", "", 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListRemoteModelsUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := ListRemoteModelsFromEndpoint(context.Background(), "gemini", "https://example.com/v1", "k", 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
