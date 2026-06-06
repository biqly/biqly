package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/biqly/biqly/internal/auth"
)

type updatePlatformSettingsRequest struct {
	SelfSignupEnabled bool `json:"self_signup_enabled"`
}

func (h *AuthHandler) handleAdminGetPlatformSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	isSuper, err := h.service.IsSuperAdmin(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !isSuper {
		h.respondError(w, http.StatusForbidden, auth.ErrNotSuperAdmin.Error())
		return
	}
	settings, err := h.service.GetPlatformSettings(r.Context())
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, settings)
}

func (h *AuthHandler) handleAdminUpdatePlatformSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req updatePlatformSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	settings, err := h.service.UpdatePlatformSettings(r.Context(), userID, req.SelfSignupEnabled)
	if errors.Is(err, auth.ErrNotSuperAdmin) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, settings)
}
