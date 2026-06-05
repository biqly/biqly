package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/go-chi/chi/v5"
)

var errBadInput = errors.New("name is required")

// fakeProviderStore implements providerStoreAPI for handler tests. Each method
// delegates to an optional func field; nil funcs return zero values.
type fakeProviderStore struct {
	providers    []ai.ProviderRow
	models       []ai.ModelRow
	getErr       error
	createErr    error
	deleteErr    error
	refreshCount int

	createProviderID string
	createModelID    string
	testResult       ai.ConnectionTestResult
	remoteModels     []ai.RemoteModelOption
	remoteModelsErr  error
	setDefaultErr    error
	updateErr        error
}

func (f *fakeProviderStore) ListProviders(context.Context) ([]ai.ProviderRow, error) {
	return f.providers, nil
}
func (f *fakeProviderStore) GetProvider(_ context.Context, id string) (ai.ProviderRow, error) {
	if f.getErr != nil {
		return ai.ProviderRow{}, f.getErr
	}
	return ai.ProviderRow{ID: id, Name: "p"}, nil
}
func (f *fakeProviderStore) CreateProvider(context.Context, *ai.CreateProviderInput) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.createProviderID, nil
}
func (f *fakeProviderStore) UpdateProvider(context.Context, string, *ai.UpdateProviderInput) error {
	return f.updateErr
}
func (f *fakeProviderStore) DeleteProvider(context.Context, string) error { return f.deleteErr }
func (f *fakeProviderStore) TestConnection(context.Context, string, string) (ai.ConnectionTestResult, error) {
	return f.testResult, nil
}
func (f *fakeProviderStore) ListRemoteModels(context.Context, string) ([]ai.RemoteModelOption, error) {
	if f.remoteModelsErr != nil {
		return nil, f.remoteModelsErr
	}
	return f.remoteModels, nil
}
func (f *fakeProviderStore) ListModels(context.Context, string, string) ([]ai.ModelRow, error) {
	return f.models, nil
}
func (f *fakeProviderStore) ActiveModels(context.Context) ([]ai.ModelRow, error) {
	return f.models, nil
}
func (f *fakeProviderStore) CreateModel(context.Context, *ai.CreateModelInput) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.createModelID, nil
}
func (f *fakeProviderStore) UpdateModel(context.Context, string, *ai.UpdateModelInput) error {
	return f.updateErr
}
func (f *fakeProviderStore) DeleteModel(context.Context, string) error { return f.deleteErr }
func (f *fakeProviderStore) SetDefaultModel(context.Context, string) error {
	return f.setDefaultErr
}
func (f *fakeProviderStore) RefreshCache(context.Context) error {
	f.refreshCount++
	return nil
}

func newTestRouter(store providerStoreAPI) *chi.Mux {
	h := &AIProvidersHandler{store: store}
	r := chi.NewRouter()
	r.Get("/ai/providers", h.ListProviders)
	r.Post("/ai/providers", h.CreateProvider)
	r.Get("/ai/providers/active-models", h.ActiveModels)
	r.Get("/ai/providers/{id}", h.GetProvider)
	r.Put("/ai/providers/{id}", h.UpdateProvider)
	r.Delete("/ai/providers/{id}", h.DeleteProvider)
	r.Post("/ai/providers/{id}/test", h.TestProvider)
	r.Get("/ai/providers/{id}/remote-models", h.ListProviderRemoteModels)
	r.Get("/ai/models", h.ListModels)
	r.Post("/ai/models", h.CreateModel)
	r.Put("/ai/models/{id}", h.UpdateModel)
	r.Delete("/ai/models/{id}", h.DeleteModel)
	r.Post("/ai/models/{id}/default", h.SetDefaultModel)
	return r
}

func do(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequestWithContext(context.Background(), method, path, rdr)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAIProviders_ReadyGuard(t *testing.T) {
	// Nil store → 503 on every endpoint.
	r := newTestRouter(nil)
	rec := do(t, r, http.MethodGet, "/ai/providers", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAIProviders_List(t *testing.T) {
	store := &fakeProviderStore{providers: []ai.ProviderRow{{ID: "1", Name: "OpenAI"}, {ID: "2", Name: "Anthropic"}}}
	rec := do(t, newTestRouter(store), http.MethodGet, "/ai/providers", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var got []ai.ProviderRow
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(got))
	}
}

func TestAIProviders_CreateRefreshesCache(t *testing.T) {
	store := &fakeProviderStore{createProviderID: "new-id"}
	rec := do(t, newTestRouter(store), http.MethodPost, "/ai/providers",
		`{"name":"OpenAI","provider_type":"openai","base_url":"https://api.openai.com/v1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	if store.refreshCount != 1 {
		t.Fatalf("expected cache refresh after create, got %d", store.refreshCount)
	}
}

func TestAIProviders_CreateValidationError(t *testing.T) {
	// Any store error on create surfaces as 400 bad request.
	store := &fakeProviderStore{createErr: errBadInput}
	rec := do(t, newTestRouter(store), http.MethodPost, "/ai/providers", `{"name":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if store.refreshCount != 0 {
		t.Fatalf("cache must not refresh on failed create")
	}
}

func TestAIProviders_GetNotFound(t *testing.T) {
	store := &fakeProviderStore{getErr: ai.ErrProviderNotFound}
	rec := do(t, newTestRouter(store), http.MethodGet, "/ai/providers/abc", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAIProviders_DeleteNotFound(t *testing.T) {
	store := &fakeProviderStore{deleteErr: ai.ErrProviderNotFound}
	rec := do(t, newTestRouter(store), http.MethodDelete, "/ai/providers/abc", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAIProviders_DeleteSuccess(t *testing.T) {
	store := &fakeProviderStore{}
	rec := do(t, newTestRouter(store), http.MethodDelete, "/ai/providers/abc", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if store.refreshCount != 1 {
		t.Fatalf("expected cache refresh after delete, got %d", store.refreshCount)
	}
}

func TestAIProviders_TestConnection(t *testing.T) {
	store := &fakeProviderStore{testResult: ai.ConnectionTestResult{Status: "connected", LatencyMS: 42, Model: "gpt-4o"}}
	rec := do(t, newTestRouter(store), http.MethodPost, "/ai/providers/abc/test", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var got ai.ConnectionTestResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != "connected" || got.LatencyMS != 42 {
		t.Fatalf("unexpected test result: %+v", got)
	}
}

func TestAIProviders_CreateModel(t *testing.T) {
	store := &fakeProviderStore{createModelID: "m-1"}
	rec := do(t, newTestRouter(store), http.MethodPost, "/ai/models",
		`{"provider_id":"p1","model_id":"gpt-4o","purpose":"query"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	if store.refreshCount != 1 {
		t.Fatalf("expected cache refresh after model create")
	}
}

func TestAIProviders_SetDefaultNotFound(t *testing.T) {
	store := &fakeProviderStore{setDefaultErr: ai.ErrModelNotFound}
	rec := do(t, newTestRouter(store), http.MethodPost, "/ai/models/xyz/default", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAIProviders_SetDefaultSuccess(t *testing.T) {
	store := &fakeProviderStore{}
	rec := do(t, newTestRouter(store), http.MethodPost, "/ai/models/xyz/default", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if store.refreshCount != 1 {
		t.Fatalf("expected cache refresh after set-default")
	}
}

func TestAIProviders_UpdateModelNotFound(t *testing.T) {
	store := &fakeProviderStore{updateErr: ai.ErrModelNotFound}
	rec := do(t, newTestRouter(store), http.MethodPut, "/ai/models/xyz",
		`{"model_id":"gpt-4o","purpose":"query"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
