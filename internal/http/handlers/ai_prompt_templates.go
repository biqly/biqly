package handlers

import (
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

type updatePromptTemplateRequest struct {
	Content string `json:"content"`
}

type restorePromptTemplateRequest struct {
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

// AIPromptTemplatesHandler serves admin CRUD for locale-specific static prompt sections.
type AIPromptTemplatesHandler struct {
	deps *app.AIDeps
}

// NewAIPromptTemplatesHandler creates a prompt templates handler.
func NewAIPromptTemplatesHandler(deps *app.AIDeps) *AIPromptTemplatesHandler {
	return &AIPromptTemplatesHandler{deps: deps}
}

// ListPromptTemplates returns all rows, optionally filtered by ?locale=en|tr.
func (h *AIPromptTemplatesHandler) ListPromptTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.deps.MetaRepo.ListPromptTemplates(ctx)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list prompt templates", err)
		return
	}
	locFilter := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("locale")))
	if locFilter != "" {
		filtered := rows[:0]
		for _, row := range rows {
			if row.Locale == locFilter {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	writeJSON(w, http.StatusOK, rows)
}

// UpdatePromptTemplate upserts content for name/locale path params.
func (h *AIPromptTemplatesHandler) UpdatePromptTemplate(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	localeStr := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "locale")))
	if name == "" || localeStr == "" {
		writeError(w, http.StatusBadRequest, "name and locale are required")
		return
	}
	loc, okLoc := promptTemplateLocale(localeStr)
	if !okLoc {
		writeError(w, http.StatusBadRequest, "unsupported locale")
		return
	}
	if !isKnownPromptTemplateName(name) {
		writeError(w, http.StatusBadRequest, "unknown prompt template name")
		return
	}
	input, ok := decodeJSON[updatePromptTemplateRequest](w, r)
	if !ok {
		return
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	ctx := r.Context()
	if err := h.deps.MetaRepo.UpsertPromptTemplate(ctx, name, loc, content); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update prompt template", err)
		return
	}
	prompt.InvalidatePromptTemplateCache(name, loc)
	writeOK(w)
}

// RestorePromptTemplate resets one template from embedded defaults.
func (h *AIPromptTemplatesHandler) RestorePromptTemplate(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[restorePromptTemplateRequest](w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(input.Name)
	localeStr := strings.TrimSpace(strings.ToLower(input.Locale))
	if name == "" || localeStr == "" {
		writeError(w, http.StatusBadRequest, "name and locale are required")
		return
	}
	loc, okLoc := promptTemplateLocale(localeStr)
	if !okLoc {
		writeError(w, http.StatusBadRequest, "unsupported locale")
		return
	}
	if !isKnownPromptTemplateName(name) {
		writeError(w, http.StatusBadRequest, "unknown prompt template name")
		return
	}
	ctx := r.Context()
	if err := prompt.RestorePromptTemplateFromEmbed(ctx, h.deps.MetaRepo, name, loc); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to restore prompt template", err)
		return
	}
	writeOK(w)
}

// ReseedPromptTemplates replaces all templates from embedded files.
func (h *AIPromptTemplatesHandler) ReseedPromptTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := prompt.ReseedAllPromptTemplatesFromEmbed(ctx, h.deps.MetaRepo); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to reseed prompt templates", err)
		return
	}
	writeOK(w)
}

func isKnownPromptTemplateName(name string) bool {
	for _, n := range prompt.KnownPromptTemplateNames() {
		if n == name {
			return true
		}
	}
	return false
}

func promptTemplateLocale(raw string) (i18n.Locale, bool) {
	return i18n.ParseSupportedLocale(raw)
}

// PromptTemplateRow is the API wire type for list responses.
type PromptTemplateRow = metadata.PromptTemplate
