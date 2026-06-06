package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterAccountSelfRoutes wires §18.1 account-security endpoints under /auth.
// Public:  /auth/unlock-account
// Self:    /auth/me/freeze, /auth/me/unfreeze, /auth/me/account (DELETE), /auth/me/sessions
// Admin:   /auth/admin/users/{id}/force-logout, /auth/admin/users/{id}/restore
//
// Caller must apply authMiddleware/admin-permission guards to the appropriate
// subgroups (matches existing pattern in RegisterAuthRoutes).
func (h *AuthHandler) RegisterAccountSelfRoutes(r chi.Router) {
	r.Post("/me/freeze", h.handleFreezeAccount)
	r.Post("/me/unfreeze", h.handleUnfreezeAccount)
	r.Delete("/me/account", h.handleDeleteAccount)
	r.Get("/me/sessions", h.handleListSessions)
	r.Delete("/me/sessions/{id}", h.handleRevokeSession)
}

func (h *AuthHandler) RegisterAccountPublicRoutes(r chi.Router) {
	r.Post("/unlock-account", h.handleUnlockAccount)
}

func (h *AuthHandler) RegisterAccountAdminRoutes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Post("/admin/users/{id}/force-logout", h.handleAdminForceLogout)
		r.Post("/admin/users/{id}/restore", h.handleAdminRestoreAccount)
		r.Post("/admin/users/{id}/mfa/bypass", h.handleAdminGenerateMFABypass)
	})
}

func (h *AuthHandler) handleAccountFreezeToggle(
	w http.ResponseWriter,
	r *http.Request,
	action func(context.Context, string) error,
	conflictErr error,
	auditAction string,
) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := action(r.Context(), userID); err != nil {
		if errors.Is(err, conflictErr) {
			h.respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditLog(r, &userID, auditAction, nil, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleFreezeAccount(w http.ResponseWriter, r *http.Request) {
	h.handleAccountFreezeToggle(w, r, h.service.FreezeAccount, auth.ErrAccountAlreadyFrozen, auth.AuditAccountFrozen)
}

func (h *AuthHandler) handleUnfreezeAccount(w http.ResponseWriter, r *http.Request) {
	h.handleAccountFreezeToggle(w, r, h.service.UnfreezeAccount, auth.ErrAccountNotFrozen, auth.AuditAccountUnfrozen)
}

func (h *AuthHandler) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req auth.DeleteAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	purgeAt, err := h.service.DeleteAccount(r.Context(), userID, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			h.respondError(w, http.StatusUnauthorized, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditLog(r, &userID, auth.AuditAccountSoftDeleted, nil, nil, map[string]any{"purge_after": purgeAt})
	h.respondJSON(w, http.StatusOK, map[string]any{"purge_after": purgeAt})
}

func (h *AuthHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextUserID(r)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessions, err := h.service.ListActiveSessions(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (h *AuthHandler) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sessionID := chi.URLParam(r, "id")
	if err := h.service.RevokeSession(r.Context(), userID, sessionID); err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditLog(r, &userID, auth.AuditSessionRevoked, ptrStr("session"), &sessionID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleUnlockAccount(w http.ResponseWriter, r *http.Request) {
	var req auth.UnlockAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	userID, err := h.service.UnlockAccount(r.Context(), req.Token)
	if err != nil {
		if errors.Is(err, auth.ErrUnlockTokenInvalid) {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditLog(r, &userID, auth.AuditAccountUnlocked, nil, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleAdminForceLogout(w http.ResponseWriter, r *http.Request) {
	actor, ok := r.Context().Value(userIDKey).(string)
	if !ok || actor == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		h.respondError(w, http.StatusBadRequest, "user id required")
		return
	}
	if err := h.service.AdminForceLogout(r.Context(), targetID); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditLog(r, &actor, auth.AuditAdminForceLogout, ptrStr("user"), &targetID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleAdminRestoreAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := r.Context().Value(userIDKey).(string)
	if !ok || actor == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		h.respondError(w, http.StatusBadRequest, "user id required")
		return
	}
	if err := h.service.RestoreAccount(r.Context(), targetID); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditLog(r, &actor, auth.AuditAccountRestored, ptrStr("user"), &targetID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleAdminGenerateMFABypass(w http.ResponseWriter, r *http.Request) {
	actor, ok := r.Context().Value(userIDKey).(string)
	if !ok || actor == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		h.respondError(w, http.StatusBadRequest, "user id required")
		return
	}

	bypassCode, err := h.service.GenerateMFABypassCode(r.Context(), actor, targetID)
	if err != nil {
		if errors.Is(err, auth.ErrSuperAdminRequired) {
			h.respondError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, auth.ErrUserNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, auth.ErrMFANotEnabled) {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.auditLog(r, &actor, auth.AuditMFABypassGenerated, ptrStr("user"), &targetID, nil)
	h.respondJSON(w, http.StatusOK, map[string]string{"bypass_code": bypassCode})
}

func (h *AuthHandler) auditLog(r *http.Request, userID *string, action string, resource, resourceID *string, metadata any) {
	if h.audit == nil {
		return
	}
	ip := r.RemoteAddr
	if err := h.audit.Log(r.Context(), userID, action, resource, resourceID, metadata, &ip); err != nil {
		slog.WarnContext(r.Context(), "auth audit log failed", "action", action, "error", err)
	}
}

func ptrStr(s string) *string { return &s }
