package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// maxMemoryContentRunes bounds a single remembered fact.
const maxMemoryContentRunes = 500

// MemoryEntriesHandler serves the user-scoped AI memory surface: durable
// remembered facts injected into prompts, fully editable and deletable by
// their owner (GDPR).
type MemoryEntriesHandler struct {
	deps *app.AIDeps
}

// NewMemoryEntriesHandler creates a MemoryEntriesHandler.
func NewMemoryEntriesHandler(deps *app.AIDeps) *MemoryEntriesHandler {
	return &MemoryEntriesHandler{deps: deps}
}

type memoryEntryPayload struct {
	Content string `json:"content"`
}

func (p *memoryEntryPayload) validate() string {
	content := strings.TrimSpace(p.Content)
	if content == "" {
		return "content is required"
	}
	if utf8.RuneCountInString(content) > maxMemoryContentRunes {
		return "content must be at most 500 characters"
	}
	return ""
}

type memoryEntryResponse struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func memoryEntryRowToResponse(row metadata.MemoryEntryRow) memoryEntryResponse {
	return memoryEntryResponse{
		ID:        row.ID,
		Content:   row.Content,
		Source:    row.Source,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// List returns the caller's remembered facts.
func (h *MemoryEntriesHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.deps.MetaRepo.ListMemoryEntries(ctx, bimw.WorkspaceID(ctx), bimw.UserID(ctx))
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list memory entries", err)
		return
	}
	out := make([]memoryEntryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, memoryEntryRowToResponse(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

// Create stores a new remembered fact for the caller.
func (h *MemoryEntriesHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[memoryEntryPayload](w, r)
	if !ok {
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	id, err := h.deps.MetaRepo.InsertMemoryEntry(ctx, bimw.WorkspaceID(ctx), bimw.UserID(ctx), strings.TrimSpace(payload.Content), "user")
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create memory entry", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// Update replaces the content of one of the caller's remembered facts.
func (h *MemoryEntriesHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[memoryEntryPayload](w, r)
	if !ok {
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	err := h.deps.MetaRepo.UpdateMemoryEntry(ctx, chi.URLParam(r, "id"), bimw.WorkspaceID(ctx), bimw.UserID(ctx), strings.TrimSpace(payload.Content))
	if err != nil {
		if errors.Is(err, metadata.ErrMemoryEntryNotFound) {
			writeEntityNotFound(w, "memory entry")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update memory entry", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

// Delete removes one of the caller's remembered facts (GDPR right to erasure).
func (h *MemoryEntriesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := h.deps.MetaRepo.DeleteMemoryEntry(ctx, chi.URLParam(r, "id"), bimw.WorkspaceID(ctx), bimw.UserID(ctx))
	if err != nil {
		if errors.Is(err, metadata.ErrMemoryEntryNotFound) {
			writeEntityNotFound(w, "memory entry")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete memory entry", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}
