package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterPersonalTokenRoutes wires personal-access-token self-service
// endpoints under /auth/me/tokens. Caller must apply authMiddleware to the
// enclosing group (matches RegisterAccountSelfRoutes).
func (h *AuthHandler) RegisterPersonalTokenRoutes(r chi.Router) {
	r.Get("/me/tokens", h.handleListTokens)
	r.Post("/me/tokens", h.handleCreateToken)
	r.Delete("/me/tokens/{id}", h.handleRevokeToken)
}

type createTokenRequest struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expires_in_days,omitempty"`
}

type createTokenResponse struct {
	auth.PersonalAccessToken
	Token string `json:"token"`
}

func (h *AuthHandler) handleListTokens(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	tokens, err := h.service.ListAccessTokens(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (h *AuthHandler) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[createTokenRequest](w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		h.respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresInDays != nil {
		if *req.ExpiresInDays <= 0 {
			h.respondError(w, http.StatusBadRequest, "expires_in_days must be positive")
			return
		}
		expiresAt = new(time.Now().AddDate(0, 0, *req.ExpiresInDays))
	}

	plaintext, rec, err := h.service.CreateAccessToken(r.Context(), userID, name, expiresAt)
	if err != nil {
		if errors.Is(err, auth.ErrPersonalAccessTokensUnavailable) {
			h.respondError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditLog(r, &userID, auth.AuditTokenCreated, new("personal_access_token"), &rec.ID, nil)
	h.respondJSON(w, http.StatusCreated, createTokenResponse{PersonalAccessToken: rec, Token: plaintext})
}

// PATVerifyResponse is the identity resolved from a valid personal access
// token, returned to the api service so it can populate the same request
// context a session JWT would (see internal/http/middleware/jwt.go).
type PATVerifyResponse struct {
	UserID        string   `json:"user_id"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Roles         []string `json:"roles"`
	WorkspaceID   string   `json:"workspace_id"`
}

// handleVerifyPersonalAccessToken resolves a bearer credential presented by
// another service (e.g. the api monolith, on behalf of an MCP/API caller) to
// its owner's current identity. Unlike handleVerify (JWT signature check,
// stateless), this always hits the database: roles and workspace are
// re-resolved live on every call, see Service.VerifyAccessToken.
func (h *AuthHandler) handleVerifyPersonalAccessToken(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[VerifyRequest](w, r)
	if !ok {
		return
	}
	identity, err := h.service.VerifyAccessToken(r.Context(), req.Token)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	h.respondJSON(w, http.StatusOK, PATVerifyResponse{
		UserID:        identity.UserID,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		Roles:         identity.Roles,
		WorkspaceID:   identity.WorkspaceID,
	})
}

func (h *AuthHandler) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	h.handleRevokeByID(w, r, h.service.RevokeAccessToken, auth.ErrPersonalAccessTokenNotFound, "personal_access_token", auth.AuditTokenRevoked)
}
