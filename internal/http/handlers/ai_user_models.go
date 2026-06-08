package handlers

import (
	"context"
	"github.com/bytedance/sonic"
	"net/http"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/auth/rbac"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

type selectableModelSummary struct {
	ID           string `json:"id"`
	Purpose      string `json:"purpose"`
	ModelID      string `json:"model_id"`
	DisplayName  string `json:"display_name"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
}

func applyUserAIModelAccess(
	ctx context.Context,
	store *ai.ProviderStore,
	auth *bimw.AuthClient,
	userID string,
	candidateIDs []string,
	preferences map[string]string,
) ([]string, map[string]string, bool, error) {
	if userID == "" || auth == nil {
		return candidateIDs, preferences, false, nil
	}
	access, err := auth.UserAIAccess(ctx, userID)
	if err != nil {
		return candidateIDs, preferences, false, err
	}
	if access == nil {
		return candidateIDs, preferences, false, nil
	}
	for purpose, modelID := range access.Preferences {
		preferences[purpose] = modelID
	}
	if !access.Restricted {
		return candidateIDs, preferences, access.Restricted, nil
	}
	providerModels, err := store.ActiveModelUUIDsByProviders(ctx, access.ProviderIDs)
	if err != nil {
		return candidateIDs, preferences, access.Restricted, err
	}
	rbacAccess := &rbac.UserAIAccess{
		Restricted:  true,
		ModelIDs:    access.ModelIDs,
		ProviderIDs: access.ProviderIDs,
	}
	return rbac.FilterAllowedModelIDs(rbacAccess, providerModels, candidateIDs), preferences, access.Restricted, nil
}

func userSelectableModels(
	ctx context.Context,
	store *ai.ProviderStore,
	auth *bimw.AuthClient,
	userID string,
) (models []selectableModelSummary, preferences map[string]string, restricted bool, err error) {
	preferences = map[string]string{}
	if store == nil {
		return nil, preferences, false, nil
	}
	rows, err := store.ListAllActiveModels(ctx)
	if err != nil {
		return nil, preferences, false, err
	}
	candidateIDs := make([]string, 0, len(rows))
	for _, m := range rows {
		candidateIDs = append(candidateIDs, m.ID)
	}
	var aerr error
	candidateIDs, preferences, restricted, aerr = applyUserAIModelAccess(ctx, store, auth, userID, candidateIDs, preferences)
	if aerr != nil {
		return nil, preferences, restricted, aerr
	}
	allowed := make(map[string]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		allowed[id] = struct{}{}
	}
	models = make([]selectableModelSummary, 0, len(candidateIDs))
	for _, m := range rows {
		if _, ok := allowed[m.ID]; !ok {
			continue
		}
		models = append(models, selectableModelSummary{
			ID:           m.ID,
			Purpose:      m.Purpose,
			ModelID:      m.ModelID,
			DisplayName:  m.DisplayName,
			ProviderName: m.ProviderName,
			ProviderType: m.ProviderType,
		})
	}
	return models, preferences, restricted, nil
}

type userAIModelsResponse struct {
	Models      []selectableModelSummary `json:"models"`
	Preferences map[string]string        `json:"preferences"`
	Restricted  bool                     `json:"restricted"`
	DBManaged   bool                     `json:"db_managed"`
}

type userAIPreferenceInput struct {
	Purpose string `json:"purpose"`
	ModelID string `json:"model_id"`
}

type putUserAIPrefsRequest struct {
	Preferences []userAIPreferenceInput `json:"preferences"`
}

// UserAIModels lists models the caller may select and their saved preferences.
func (h *AIHandler) UserAIModels(w http.ResponseWriter, r *http.Request) {
	if h.deps.AIProviderStore == nil {
		writeJSON(w, http.StatusOK, userAIModelsResponse{
			Preferences: map[string]string{},
		})
		return
	}
	userID := bimw.UserID(r.Context())
	models, prefs, restricted, err := userSelectableModels(r.Context(), h.deps.AIProviderStore, h.authClient, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, userAIModelsResponse{
		Models:      models,
		Preferences: prefs,
		Restricted:  restricted,
		DBManaged:   true,
	})
}

// PutUserAIPreferences saves per-purpose model choices after access checks.
func (h *AIHandler) PutUserAIPreferences(w http.ResponseWriter, r *http.Request) {
	if h.deps.AIProviderStore == nil {
		writeError(w, http.StatusBadRequest, "ai provider store is not configured")
		return
	}
	if h.authClient == nil {
		writeError(w, http.StatusServiceUnavailable, "auth is not configured")
		return
	}
	userID := bimw.UserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req putUserAIPrefsRequest
	if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	models, _, _, err := userSelectableModels(r.Context(), h.deps.AIProviderStore, h.authClient, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	allowed := make(map[string]struct{}, len(models))
	for _, m := range models {
		allowed[m.ID] = struct{}{}
	}
	for _, p := range req.Preferences {
		if p.Purpose == "" || p.ModelID == "" {
			continue
		}
		if !ai.UserSelectablePurpose(p.Purpose) {
			writeError(w, http.StatusBadRequest, "purpose not user-selectable: "+p.Purpose)
			return
		}
		if _, ok := allowed[p.ModelID]; !ok {
			writeError(w, http.StatusForbidden, "model not allowed for this user")
			return
		}
	}
	for _, p := range req.Preferences {
		if p.Purpose == "" || p.ModelID == "" {
			continue
		}
		if err := h.authClient.SetUserAIPreference(r.Context(), userID, p.Purpose, p.ModelID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	prefs, err := h.authClient.ListUserAIPreferences(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]string{}
	for _, p := range prefs {
		out[p.Purpose] = p.ModelID
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": out})
}

// DeleteUserAIPreference clears a single purpose preference.
func (h *AIHandler) DeleteUserAIPreference(w http.ResponseWriter, r *http.Request) {
	if h.authClient == nil {
		writeError(w, http.StatusServiceUnavailable, "auth is not configured")
		return
	}
	userID := bimw.UserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	purpose, ok := requireURLParam(w, r, "purpose")
	if !ok || !ai.ValidPurpose(purpose) {
		if ok {
			writeError(w, http.StatusBadRequest, "invalid purpose")
		}
		return
	}
	if err := h.authClient.DeleteUserAIPreference(r.Context(), userID, purpose); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
