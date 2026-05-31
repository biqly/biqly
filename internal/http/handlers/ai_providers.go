package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/app"
)

// AIProvidersHandler serves the admin CRUD API for AI providers and models.
// All routes are admin-gated at the router; mutations refresh the in-memory
// provider cache so model changes take effect without a restart.
type AIProvidersHandler struct {
	deps  *app.AIDeps
	store *ai.ProviderStore
}

// NewAIProvidersHandler builds the handler from AI dependencies.
func NewAIProvidersHandler(deps *app.AIDeps) *AIProvidersHandler {
	return &AIProvidersHandler{deps: deps, store: deps.AIProviderStore}
}

func (h *AIProvidersHandler) refresh(r *http.Request) {
	if h.store == nil {
		return
	}
	if err := h.store.RefreshCache(r.Context()); err != nil {
		slog.WarnContext(r.Context(), "ai provider cache refresh failed", "error", err)
	}
}

func (h *AIProvidersHandler) ready(w http.ResponseWriter) bool {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "AI provider store is not configured")
		return false
	}
	return true
}

// ----- request payloads ------------------------------------------------------

type createProviderRequest struct {
	Name               string `json:"name"`
	ProviderType       string `json:"provider_type"`
	BaseURL            string `json:"base_url"`
	APIKey             string `json:"api_key"`
	IsActive           *bool  `json:"is_active"`
	HTTPTimeoutSeconds int    `json:"http_timeout_seconds"`
	RateLimitPerMinute int    `json:"rate_limit_per_minute"`
}

type updateProviderRequest struct {
	Name               string  `json:"name"`
	ProviderType       string  `json:"provider_type"`
	BaseURL            string  `json:"base_url"`
	APIKey             *string `json:"api_key"`
	IsActive           *bool   `json:"is_active"`
	HTTPTimeoutSeconds *int    `json:"http_timeout_seconds"`
	RateLimitPerMinute *int    `json:"rate_limit_per_minute"`
}

type testProviderRequest struct {
	ModelID string `json:"model_id"`
}

type createAIModelRequest struct {
	ProviderID          string  `json:"provider_id"`
	ModelID             string  `json:"model_id"`
	DisplayName         string  `json:"display_name"`
	Purpose             string  `json:"purpose"`
	MaxTokens           int     `json:"max_tokens"`
	Temperature         float64 `json:"temperature"`
	TopP                float64 `json:"top_p"`
	NumCtx              int     `json:"num_ctx"`
	MaxPromptInputRunes int     `json:"max_prompt_input_runes"`
	IsDefault           bool    `json:"is_default"`
	IsActive            *bool   `json:"is_active"`
}

type updateAIModelRequest struct {
	ModelID             string   `json:"model_id"`
	DisplayName         string   `json:"display_name"`
	Purpose             string   `json:"purpose"`
	MaxTokens           *int     `json:"max_tokens"`
	Temperature         *float64 `json:"temperature"`
	TopP                *float64 `json:"top_p"`
	NumCtx              *int     `json:"num_ctx"`
	MaxPromptInputRunes *int     `json:"max_prompt_input_runes"`
	IsActive            *bool    `json:"is_active"`
}

// ----- providers -------------------------------------------------------------

// ListProviders returns all providers with masked keys and model counts.
func (h *AIProvidersHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	rows, err := h.store.ListProviders(r.Context())
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list providers", err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// GetProvider returns a single provider.
func (h *AIProvidersHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	row, err := h.store.GetProvider(r.Context(), id)
	if errors.Is(err, ai.ErrProviderNotFound) {
		writeEntityNotFound(w, "provider")
		return
	}
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to get provider", err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// CreateProvider adds a provider.
func (h *AIProvidersHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	in, ok := decodeJSON[createProviderRequest](w, r)
	if !ok {
		return
	}
	id, err := h.store.CreateProvider(r.Context(), ai.CreateProviderInput{
		Name:               in.Name,
		ProviderType:       in.ProviderType,
		BaseURL:            in.BaseURL,
		APIKey:             in.APIKey,
		IsActive:           in.IsActive,
		HTTPTimeoutSeconds: in.HTTPTimeoutSeconds,
		RateLimitPerMinute: in.RateLimitPerMinute,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.refresh(r)
	row, err := h.store.GetProvider(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// UpdateProvider patches a provider.
func (h *AIProvidersHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	in, ok := decodeJSON[updateProviderRequest](w, r)
	if !ok {
		return
	}
	err := h.store.UpdateProvider(r.Context(), id, ai.UpdateProviderInput{
		Name:               in.Name,
		ProviderType:       in.ProviderType,
		BaseURL:            in.BaseURL,
		APIKey:             in.APIKey,
		IsActive:           in.IsActive,
		HTTPTimeoutSeconds: in.HTTPTimeoutSeconds,
		RateLimitPerMinute: in.RateLimitPerMinute,
	})
	if errors.Is(err, ai.ErrProviderNotFound) {
		writeEntityNotFound(w, "provider")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.refresh(r)
	row, err := h.store.GetProvider(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// DeleteProvider removes a provider and its models.
func (h *AIProvidersHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	err := h.store.DeleteProvider(r.Context(), id)
	if errors.Is(err, ai.ErrProviderNotFound) {
		writeEntityNotFound(w, "provider")
		return
	}
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to delete provider", err)
		return
	}
	h.refresh(r)
	w.WriteHeader(http.StatusNoContent)
}

// TestProvider probes provider connectivity with a tiny prompt.
func (h *AIProvidersHandler) TestProvider(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	in, ok := decodeJSONAllowEmpty[testProviderRequest](w, r)
	if !ok {
		return
	}
	result, err := h.store.TestConnection(r.Context(), id, in.ModelID)
	if errors.Is(err, ai.ErrProviderNotFound) {
		writeEntityNotFound(w, "provider")
		return
	}
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to test provider", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ----- models ----------------------------------------------------------------

// ListModels returns models filtered by optional provider_id and purpose.
func (h *AIProvidersHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	rows, err := h.store.ListModels(r.Context(), r.URL.Query().Get("provider_id"), r.URL.Query().Get("purpose"))
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list models", err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// ActiveModels returns the current default model for each purpose.
func (h *AIProvidersHandler) ActiveModels(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	rows, err := h.store.ActiveModels(r.Context())
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list active models", err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// CreateModel adds a model to a provider.
func (h *AIProvidersHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	in, ok := decodeJSON[createAIModelRequest](w, r)
	if !ok {
		return
	}
	id, err := h.store.CreateModel(r.Context(), ai.CreateModelInput{
		ProviderID:          in.ProviderID,
		ModelID:             in.ModelID,
		DisplayName:         in.DisplayName,
		Purpose:             in.Purpose,
		MaxTokens:           in.MaxTokens,
		Temperature:         in.Temperature,
		TopP:                in.TopP,
		NumCtx:              in.NumCtx,
		MaxPromptInputRunes: in.MaxPromptInputRunes,
		IsDefault:           in.IsDefault,
		IsActive:            in.IsActive,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.refresh(r)
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// UpdateModel patches a model.
func (h *AIProvidersHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	in, ok := decodeJSON[updateAIModelRequest](w, r)
	if !ok {
		return
	}
	err := h.store.UpdateModel(r.Context(), id, ai.UpdateModelInput{
		ModelID:             in.ModelID,
		DisplayName:         in.DisplayName,
		Purpose:             in.Purpose,
		MaxTokens:           in.MaxTokens,
		Temperature:         in.Temperature,
		TopP:                in.TopP,
		NumCtx:              in.NumCtx,
		MaxPromptInputRunes: in.MaxPromptInputRunes,
		IsActive:            in.IsActive,
	})
	if errors.Is(err, ai.ErrModelNotFound) {
		writeEntityNotFound(w, "model")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.refresh(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteModel removes a model.
func (h *AIProvidersHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	err := h.store.DeleteModel(r.Context(), id)
	if errors.Is(err, ai.ErrModelNotFound) {
		writeEntityNotFound(w, "model")
		return
	}
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to delete model", err)
		return
	}
	h.refresh(r)
	w.WriteHeader(http.StatusNoContent)
}

// SetDefaultModel marks a model as the default for its purpose.
func (h *AIProvidersHandler) SetDefaultModel(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	err := h.store.SetDefaultModel(r.Context(), id)
	if errors.Is(err, ai.ErrModelNotFound) {
		writeEntityNotFound(w, "model")
		return
	}
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to set default model", err)
		return
	}
	h.refresh(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
