package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/biqly/biqly/internal/auth"
)

func (h *AuthHandler) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req auth.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	profile, err := h.service.UpdateProfile(r.Context(), userID, req)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, profile)
}

func (h *AuthHandler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req auth.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		h.respondError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}
	err := h.service.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			h.respondError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, auth.ErrNoPasswordSet):
			h.respondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, auth.ErrPasswordReused):
			h.respondError(w, http.StatusBadRequest, err.Error())
		default:
			h.respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleMeGenerateMFABypass(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	code, err := h.service.GenerateMFABypassCode(r.Context(), userID, userID)
	if err != nil {
		if errors.Is(err, auth.ErrSuperAdminRequired) {
			h.respondError(w, http.StatusForbidden, err.Error())
			return
		}
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"bypass_code": code})
}
