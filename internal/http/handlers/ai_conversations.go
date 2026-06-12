package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
)

type aiConversationRequest struct {
	ID             string                           `json:"id,omitempty"`
	DatasourceID   string                           `json:"datasource_id"`
	ModelID        *string                          `json:"model_id,omitempty"`
	ContextEnabled *bool                            `json:"context_enabled,omitempty"`
	Title          *string                          `json:"title,omitempty"`
	Messages       []metadata.AIConversationMessage `json:"messages,omitempty"`
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
	var req aiConversationRequest
	if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	contextEnabled := true
	if req.ContextEnabled != nil {
		contextEnabled = *req.ContextEnabled
	}
	conv := &metadata.AIConversation{
		ID:             req.ID,
		UserID:         userID,
		DatasourceID:   req.DatasourceID,
		ModelID:        req.ModelID,
		ContextEnabled: contextEnabled,
		Title:          req.Title,
	}
	if err := h.deps.MetaRepo.CreateAIConversation(r.Context(), conv); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "create AI conversation failed", err)
		return
	}
	for i := range req.Messages {
		msg := req.Messages[i]
		msg.ConversationID = conv.ID
		if err := h.deps.MetaRepo.CreateAIConversationMessage(r.Context(), &msg); err != nil {
			writeInternalError(r.Context(), w, http.StatusInternalServerError, "create AI conversation message failed", err)
			return
		}
		conv.Messages = append(conv.Messages, msg)
	}
	writeJSON(w, http.StatusCreated, conv)
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
