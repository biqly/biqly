package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

type aiConversationRequest struct {
	ID              string                           `json:"id,omitempty"`
	DatasourceID    string                           `json:"datasource_id"`
	ModelID         *string                          `json:"model_id,omitempty"`
	ContextEnabled  *bool                            `json:"context_enabled,omitempty"`
	Title           *string                          `json:"title,omitempty"`
	SnapshotVersion int64                            `json:"snapshot_version"`
	Messages        []metadata.AIConversationMessage `json:"messages,omitempty"`
}

func (h *AIHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID := historyUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user required")
		return
	}
	conversations, err := h.deps.MetaRepo.ListAIConversations(r.Context(), userID, 50)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list AI conversations failed", err)
		return
	}
	writeJSON(w, http.StatusOK, conversations)
}

func (h *AIHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	userID := historyUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user required")
		return
	}
	req, ok := decodeJSON[aiConversationRequest](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}

	// Messages with remote_id are required for idempotent writes.
	for _, m := range req.Messages {
		if m.RemoteID == "" {
			writeError(w, http.StatusBadRequest, "messages require remote_id for idempotent writes")
			return
		}
	}

	contextEnabled := true
	if req.ContextEnabled != nil {
		contextEnabled = *req.ContextEnabled
	}
	conv := metadata.AIConversation{
		ID:              req.ID,
		UserID:          userID,
		DatasourceID:    req.DatasourceID,
		ModelID:         req.ModelID,
		ContextEnabled:  contextEnabled,
		Title:           req.Title,
		SnapshotVersion: req.SnapshotVersion,
		Messages:        req.Messages,
	}

	// Idempotency key: require the header, or generate one for backward compat.
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = "auto-" + userID + "-" + conv.ID
	}

	payloadHash := computeConversationPayloadHash(userID, *req)

	result, err := h.deps.MetaRepo.SaveAIConversationSnapshot(r.Context(), userID, metadata.ConversationSnapshotWrite{
		Conversation:    conv,
		ExpectedVersion: req.SnapshotVersion,
		IdempotencyKey:  idempotencyKey,
		PayloadHash:     payloadHash,
	})
	if err != nil {
		switch {
		case errors.Is(err, metadata.ErrConversationVersionConflict):
			writeError(w, http.StatusConflict, "conversation version conflict")
		case errors.Is(err, metadata.ErrConversationMessageConflict):
			writeError(w, http.StatusConflict, "conversation message conflict")
		case errors.Is(err, metadata.ErrIdempotencyKeyConflict):
			writeError(w, http.StatusConflict, "idempotency key conflict")
		default:
			writeInternalError(r.Context(), w, http.StatusInternalServerError, "save AI conversation snapshot failed", err)
		}
		return
	}

	writeJSON(w, result.StatusCode, result.Conversation)
}

func (h *AIHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	userID := historyUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user required")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "conversation id is required")
		return
	}
	deleted, err := h.deps.MetaRepo.DeleteAIConversation(r.Context(), id, userID)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "delete AI conversation failed", err)
		return
	}
	if !deleted {
		writeEntityNotFound(w, "conversation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// computeConversationPayloadHash produces a stable hash of the request payload
// so a retried request with the same body returns the stored response, while a
// different body for the same idempotency key is rejected as a conflict.
func computeConversationPayloadHash(userID string, req aiConversationRequest) string {
	h := sha256.New()
	h.Write([]byte(userID))
	h.Write([]byte(req.DatasourceID))
	if req.ModelID != nil {
		h.Write([]byte(*req.ModelID))
	}
	h.Write([]byte(req.TitlePtr()))
	for _, m := range req.Messages {
		h.Write([]byte(m.RemoteID))
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TitlePtr returns the title pointer value or empty string for hashing.
func (r aiConversationRequest) TitlePtr() string {
	if r.Title != nil {
		return *r.Title
	}
	return ""
}
