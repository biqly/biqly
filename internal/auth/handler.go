package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-webauthn/webauthn/webauthn"
)

type contextKey string

const (
	userIDKey      contextKey = "userID"
	emailKey       contextKey = "email"
	rolesKey       contextKey = "roles"
	workspaceIDKey contextKey = "workspaceID"
)

type AuthHandler struct {
	service  *AuthService
	webAuthn *WebAuthnService
	jwtMgr   *JWTManager
	config   *Config
	limiter  *RateLimiter
}

func NewAuthHandler(service *AuthService, webAuthn *WebAuthnService, jwtMgr *JWTManager, config *Config, limiter *RateLimiter) *AuthHandler {
	return &AuthHandler{
		service:  service,
		webAuthn: webAuthn,
		jwtMgr:   jwtMgr,
		config:   config,
		limiter:  limiter,
	}
}

func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Route("/auth", h.RegisterAuthRoutes)
	r.Route("/internal/auth", h.RegisterInternalRoutes)
}

func (h *AuthHandler) RegisterAuthRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		if h.limiter != nil {
			r.Use(h.limiter.Limit(10, time.Minute, "login"))
		}
		r.Post("/login", h.handleLogin)
	})
	r.Post("/register", h.handleRegister)
	r.Post("/refresh", h.handleRefresh)
	r.Post("/logout", h.handleLogout)
	r.Post("/forgot-password", h.handleForgotPassword)
	r.Post("/reset-password", h.handleResetPassword)
	r.Get("/verify-email", h.handleVerifyEmail)
	r.Post("/resend-verification", h.handleResendVerification)

	r.Get("/oauth/{provider}", h.handleOAuthRedirect)
	r.Get("/oauth/{provider}/callback", h.handleOAuthCallback)

	r.Post("/passkey/login-begin", h.handlePasskeyLoginBegin)
	r.Post("/passkey/login-finish", h.handlePasskeyLoginFinish)

	r.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Get("/me", h.handleMe)
		r.Post("/passkey/register-begin", h.handlePasskeyRegisterBegin)
		r.Post("/passkey/register-finish", h.handlePasskeyRegisterFinish)
		r.Get("/me/passkeys", h.handleMePasskeys)
		r.Delete("/me/passkeys/{id}", h.handleDeletePasskey)
	})
}

func (h *AuthHandler) RegisterInternalRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.internalTokenMiddleware)
		r.Post("/verify", h.handleVerify)
		r.Get("/user/{id}/permissions", h.handleGetUserPermissions)
	})
}

func (h *AuthHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()
	resp, err := h.service.Register(r.Context(), req, &ua, &ip)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusCreated, resp)
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()
	resp, err := h.service.Login(r.Context(), req, &ua, &ip)
	if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrInactiveUser) {
		h.respondError(w, http.StatusUnauthorized, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()
	resp, err := h.service.Refresh(r.Context(), req, &ua, &ip)
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExpired) || errors.Is(err, ErrSessionRevoked) {
		h.respondError(w, http.StatusUnauthorized, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.Logout(r.Context(), req.RefreshToken)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resp, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

type VerifyRequest struct {
	Token string `json:"token"`
}

type VerifyResponse struct {
	UserID                string   `json:"user_id"`
	Email                 string   `json:"email"`
	Roles                 []string `json:"roles"`
	WorkspaceID           string   `json:"workspace_id"`
	AccessibleDatasources []string `json:"accessible_datasources"`
}

func (h *AuthHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims, err := h.jwtMgr.ValidateToken(req.Token)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	resp := VerifyResponse{
		UserID:                claims.Subject,
		Email:                 claims.Email,
		Roles:                 claims.Roles,
		WorkspaceID:           claims.WorkspaceID,
		AccessibleDatasources: claims.AccessibleDatasources,
	}

	h.respondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) handleGetUserPermissions(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		h.respondError(w, http.StatusBadRequest, "user id is required")
		return
	}

	perms, err := h.service.rbacRepo.GetUserPermissions(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

func (h *AuthHandler) AuthMiddleware() func(http.Handler) http.Handler {
	return h.authMiddleware
}

func (h *AuthHandler) InternalTokenMiddleware() func(http.Handler) http.Handler {
	return h.internalTokenMiddleware
}

func (h *AuthHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.respondError(w, http.StatusUnauthorized, "authorization header required")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			h.respondError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		claims, err := h.jwtMgr.ValidateToken(parts[1])
		if err != nil {
			h.respondError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, userIDKey, claims.Subject)
		ctx = context.WithValue(ctx, emailKey, claims.Email)
		ctx = context.WithValue(ctx, rolesKey, claims.Roles)
		ctx = context.WithValue(ctx, workspaceIDKey, claims.WorkspaceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *AuthHandler) internalTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenHeader := r.Header.Get("X-Internal-Token")
		if tokenHeader == "" {
			// Fall back to Authorization Bearer key if used
			authHeader := r.Header.Get("Authorization")
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				tokenHeader = parts[1]
			}
		}

		if tokenHeader != h.config.InternalToken {
			h.respondError(w, http.StatusForbidden, "forbidden: invalid internal token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *AuthHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}

func (h *AuthHandler) handleOAuthRedirect(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, err := NewOAuthProvider(providerName, h.config)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	state, err := h.generateState()
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	//nolint:gosec
	stateCookie := &http.Cookie{
		Name:     "oauth_state_" + providerName,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   r.URL.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, stateCookie)

	authURL := provider.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, err := NewOAuthProvider(providerName, h.config)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	stateParam := r.URL.Query().Get("state")
	codeParam := r.URL.Query().Get("code")

	cookie, err := r.Cookie("oauth_state_" + providerName)
	if err != nil || cookie.Value != stateParam || stateParam == "" {
		h.respondError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	// Clear the state cookie
	//nolint:gosec
	clearCookie := &http.Cookie{
		Name:     "oauth_state_" + providerName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.URL.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, clearCookie)

	token, err := provider.ExchangeCode(r.Context(), codeParam)
	if err != nil {
		h.respondError(w, http.StatusBadGateway, fmt.Sprintf("failed to exchange code: %v", err))
		return
	}

	userInfo, err := provider.GetUserInfo(r.Context(), token)
	if err != nil {
		h.respondError(w, http.StatusBadGateway, fmt.Sprintf("failed to get user info: %v", err))
		return
	}

	ua := r.UserAgent()
	ip := r.RemoteAddr
	resp, err := h.service.LoginOrRegisterOAuth(r.Context(), providerName, token, userInfo, &ua, &ip)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	frontendCallback := "http://localhost:3333/auth/callback"
	redirectURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&user_id=%s&email=%s",
		frontendCallback, resp.AccessToken, resp.RefreshToken, resp.UserID, resp.Email)
	//nolint:gosec
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) generateState() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

type PasskeyLoginBeginRequest struct {
	Email string `json:"email"`
}

func (h *AuthHandler) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.service.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	creation, session, err := h.webAuthn.BeginRegistration(r.Context(), user)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sessionB64 := base64.StdEncoding.EncodeToString(sessionJSON)
	//nolint:gosec
	http.SetCookie(w, &http.Cookie{
		Name:     "webauthn_register_session",
		Value:    sessionB64,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   r.URL.Scheme == "https" || h.config.Port != 8889,
		SameSite: http.SameSiteLaxMode,
	})

	h.respondJSON(w, http.StatusOK, creation)
}

func (h *AuthHandler) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	cookie, err := r.Cookie("webauthn_register_session")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "registration session cookie missing")
		return
	}

	sessionJSON, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session cookie format")
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session data")
		return
	}

	name := r.URL.Query().Get("name")

	user, err := h.service.userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = h.webAuthn.FinishRegistration(r.Context(), user, &session, r, name)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	//nolint:gosec
	http.SetCookie(w, &http.Cookie{
		Name:     "webauthn_register_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	h.respondJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *AuthHandler) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	var req PasskeyLoginBeginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	assertion, session, err := h.webAuthn.BeginLogin(r.Context(), req.Email)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sessionB64 := base64.StdEncoding.EncodeToString(sessionJSON)
	//nolint:gosec
	http.SetCookie(w, &http.Cookie{
		Name:     "webauthn_login_session",
		Value:    sessionB64,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   r.URL.Scheme == "https" || h.config.Port != 8889,
		SameSite: http.SameSiteLaxMode,
	})

	h.respondJSON(w, http.StatusOK, assertion)
}

func (h *AuthHandler) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("webauthn_login_session")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "login session cookie missing")
		return
	}

	sessionJSON, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session cookie format")
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session data")
		return
	}

	user, err := h.webAuthn.FinishLogin(r.Context(), &session, r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	//nolint:gosec
	http.SetCookie(w, &http.Cookie{
		Name:     "webauthn_login_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	ip := r.RemoteAddr
	ua := r.UserAgent()
	resp, err := h.service.CreateTokenResponseForUser(r.Context(), user, &ua, &ip)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) handleMePasskeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	passkeys, err := h.webAuthn.GetUserPasskeys(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, passkeys)
}

func (h *AuthHandler) handleDeletePasskey(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	passkeyID := chi.URLParam(r, "id")
	if passkeyID == "" {
		h.respondError(w, http.StatusBadRequest, "passkey id is required")
		return
	}

	err := h.webAuthn.DeletePasskey(r.Context(), userID, passkeyID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

func (h *AuthHandler) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.ForgotPassword(r.Context(), req.Email)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a password reset link has been sent."})
}

func (h *AuthHandler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.ResetPassword(r.Context(), req.Token, req.Password)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Password reset successful."})
}

func (h *AuthHandler) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	err := h.service.VerifyEmail(r.Context(), token)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Email verified successfully."})
}

func (h *AuthHandler) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.ResendVerificationEmail(r.Context(), req.Email)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "If the email is not verified, a new verification link has been sent."})
}
