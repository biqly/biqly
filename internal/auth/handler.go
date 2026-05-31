package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/biqly/biqly/internal/http/response"
	"github.com/biqly/biqly/internal/mail"
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
	mfa      *MFAService
	gdpr     *GDPRExporter
	audit    *AuditService
}

func (h *AuthHandler) SetGDPRExporter(g *GDPRExporter) { h.gdpr = g }
func (h *AuthHandler) SetAuditService(a *AuditService) { h.audit = a }

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
	r.Group(func(r chi.Router) {
		if h.limiter != nil {
			r.Use(h.limiter.Limit(5, time.Minute, "magic-link"))
		}
		r.Post("/magic-link/request", h.handleMagicLinkRequest)
		r.Post("/magic-link/consume", h.handleMagicLinkConsume)
	})
	r.Get("/verify-email", h.handleVerifyEmail)
	r.Post("/resend-verification", h.handleResendVerification)
	r.Get("/email-change/confirm", h.handleConfirmEmailChange)
	r.Post("/email-change/confirm", h.handleConfirmEmailChange)

	r.Get("/invitations/{token}", h.handleGetInvitation)
	r.Post("/invitations/{token}/claim", h.handleClaimInvitation)

	r.Get("/oauth/{provider}", h.handleOAuthRedirect)
	r.Get("/oauth/{provider}/callback", h.handleOAuthCallback)
	r.Group(func(r chi.Router) {
		if h.limiter != nil {
			r.Use(h.limiter.Limit(20, time.Minute, "oauth-exchange"))
		}
		r.Post("/oauth/exchange", h.handleOAuthExchange)
	})

	r.Post("/passkey/login-begin", h.handlePasskeyLoginBegin)
	r.Post("/passkey/login-finish", h.handlePasskeyLoginFinish)

	r.Get("/password-policy", h.handlePasswordPolicy)

	h.RegisterAccountPublicRoutes(r)

	r.Group(func(r chi.Router) {
		if h.limiter != nil {
			r.Use(h.limiter.Limit(10, time.Minute, "mfa-login"))
		}
		r.Post("/mfa/login", h.handleMFALogin)
	})

	r.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Get("/me", h.handleMe)
		r.Get("/me/export", h.handleMeExport)
		r.Post("/me/active-workspace", h.handleSetActiveWorkspace)
		r.Post("/me/email-change/request", h.handleRequestEmailChange)
		r.Post("/passkey/register-begin", h.handlePasskeyRegisterBegin)
		r.Post("/passkey/register-finish", h.handlePasskeyRegisterFinish)
		r.Get("/me/passkeys", h.handleMePasskeys)
		r.Delete("/me/passkeys/{id}", h.handleDeletePasskey)
		r.Patch("/me/passkeys/{id}", h.handleUpdatePasskey)
		r.Get("/mfa/status", h.handleMFAStatus)
		r.Post("/mfa/enroll", h.handleMFAEnroll)
		r.Post("/mfa/verify", h.handleMFAVerify)
		r.Post("/mfa/disable", h.handleMFADisable)
		r.Post("/mfa/recovery/regenerate", h.handleMFARegenerateRecovery)
		r.Post("/admin/invitations", h.handleAdminInviteUser)
		r.Get("/admin/invitations", h.handleAdminListInvitations)
		r.Delete("/admin/invitations/{id}", h.handleAdminRevokeInvitation)
		r.Post("/admin/invitations/{id}/resend", h.handleAdminResendInvitation)
		r.Post("/admin/users/{id}/resend-verification", h.handleAdminResendUserVerification)
		h.RegisterAccountSelfRoutes(r)
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
		// Return identical message for both to prevent account enumeration:
		// an attacker probing emails must not be able to distinguish
		// "wrong password" from "inactive account" from "no such user".
		h.respondError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		return
	} else if errors.Is(err, ErrAccountLocked) {
		h.respondError(w, http.StatusTooManyRequests, err.Error())
		return
	} else if errors.Is(err, ErrMFARequired) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	} else if errors.Is(err, ErrAccountFrozen) || errors.Is(err, ErrAccountDeleted) {
		h.respondError(w, http.StatusForbidden, err.Error())
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
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExpired) || errors.Is(err, ErrSessionRevoked) ||
		errors.Is(err, ErrSessionAbsoluteExpired) || errors.Is(err, ErrSessionIdleExpired) {
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

func (h *AuthHandler) handleSetActiveWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req SetActiveWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.service.SetActiveWorkspace(r.Context(), userID, req.WorkspaceID)
	if err != nil {
		if errors.Is(err, ErrNotWorkspaceOwner) {
			h.respondError(w, http.StatusForbidden, "not a member of workspace")
			return
		}
		h.respondError(w, http.StatusBadRequest, err.Error())
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
	response.WriteJSON(w, status, data)
}

// respondError writes a JSON error. For server-side failures (5xx) the
// detailed message is sent to logs/telemetry only and the client receives a
// generic, user-friendly message so internal details never leak. Client
// errors (4xx) keep their message, which is intentional validation feedback.
func (h *AuthHandler) respondError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		slog.Error("auth handler error", "detail", message, "status", status)
		message = "internal server error"
	}
	response.WriteError(w, status, message)
}

func (h *AuthHandler) secureCookie(r *http.Request) bool {
	return r.URL.Scheme == "https" || r.Header.Get("X-Forwarded-Proto") == "https" || h.config.Port != 8889
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

	secureCookie := h.secureCookie(r)
	//nolint:gosec // G124: false positive as Secure is set dynamically based on HTTPS
	stateCookie := &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     "oauth_state_" + providerName,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   secureCookie,
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
	secureCookie := h.secureCookie(r)
	//nolint:gosec // G124: false positive as Secure is set dynamically based on HTTPS
	clearCookie := &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     "oauth_state_" + providerName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
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

	code, err := h.service.IssueOAuthCallbackCode(r.Context(), resp)
	if err != nil {
		h.respondError(w, http.StatusServiceUnavailable, "oauth callback exchange unavailable")
		return
	}

	frontendBase := strings.TrimRight(h.config.FrontendBaseURL, "/")
	if frontendBase == "" {
		frontendBase = "http://localhost:3333"
	}
	callbackURL, err := url.Parse(frontendBase + "/auth/callback")
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "invalid frontend callback URL")
		return
	}
	q := callbackURL.Query()
	q.Set("code", code)
	callbackURL.RawQuery = q.Encode()
	http.Redirect(w, r, callbackURL.String(), http.StatusTemporaryRedirect)
}

type OAuthExchangeRequest struct {
	Code string `json:"code"`
}

func (h *AuthHandler) handleOAuthExchange(w http.ResponseWriter, r *http.Request) {
	var req OAuthExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		h.respondError(w, http.StatusBadRequest, "code is required")
		return
	}

	resp, err := h.service.RedeemOAuthCallbackCode(r.Context(), req.Code)
	if err != nil {
		if errors.Is(err, ErrInvalidOAuthCallbackCode) {
			h.respondError(w, http.StatusBadRequest, "invalid or expired code")
			return
		}
		if errors.Is(err, ErrOAuthExchangeUnavailable) {
			h.respondError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
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
	secureCookie := h.secureCookie(r)
	//nolint:gosec // G124: false positive as Secure is set dynamically based on HTTPS
	http.SetCookie(w, &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     "webauthn_register_session",
		Value:    sessionB64,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   secureCookie,
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

	secureCookie := h.secureCookie(r)
	//nolint:gosec // G124: false positive as Secure is set dynamically based on HTTPS
	http.SetCookie(w, &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     "webauthn_register_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
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
	secureCookie := h.secureCookie(r)
	//nolint:gosec // G124: false positive as Secure is set dynamically based on HTTPS
	http.SetCookie(w, &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     "webauthn_login_session",
		Value:    sessionB64,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   secureCookie,
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

	secureCookie := h.secureCookie(r)
	//nolint:gosec // G124: false positive as Secure is set dynamically based on HTTPS
	http.SetCookie(w, &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     "webauthn_login_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
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

func (h *AuthHandler) handleUpdatePasskey(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	err := h.webAuthn.UpdatePasskeyName(r.Context(), userID, passkeyID, req.Name)
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

type ConfirmEmailChangeRequest struct {
	Token string `json:"token"`
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

func (h *AuthHandler) handleMagicLinkRequest(w http.ResponseWriter, r *http.Request) {
	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Errors here are either malformed email (400) or downstream failures we
	// want to mask; in either case the response shape stays uniform to keep
	// the endpoint enumeration-resistant.
	ip := r.RemoteAddr
	if err := h.service.RequestMagicLink(r.Context(), req.Email, ip); err != nil {
		if errors.Is(err, mail.ErrEmailRateLimited) || strings.Contains(err.Error(), "invalid email") {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Log but respond with success so the caller cannot tell whether the
		// address exists; internal errors must not leak through this path.
		h.respondJSON(w, http.StatusOK, map[string]string{"message": "If an account exists, a sign-in link has been emailed."})
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"message": "If an account exists, a sign-in link has been emailed."})
}

func (h *AuthHandler) handleMagicLinkConsume(w http.ResponseWriter, r *http.Request) {
	var req MagicLinkConsumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ua := r.UserAgent()
	ip := r.RemoteAddr
	resp, err := h.service.ConsumeMagicLink(r.Context(), req.Token, &ua, &ip)
	if err != nil {
		switch {
		case errors.Is(err, ErrMagicLinkInvalid), errors.Is(err, ErrMagicLinkUsed):
			h.respondError(w, http.StatusBadRequest, ErrMagicLinkInvalid.Error())
		case errors.Is(err, ErrInactiveUser):
			h.respondError(w, http.StatusUnauthorized, ErrInactiveUser.Error())
		case errors.Is(err, ErrAccountFrozen), errors.Is(err, ErrAccountDeleted):
			h.respondError(w, http.StatusUnauthorized, err.Error())
		default:
			h.respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	h.respondJSON(w, http.StatusOK, resp)
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

func (h *AuthHandler) handleRequestEmailChange(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req RequestEmailChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	change, err := h.service.RequestEmailChange(r.Context(), userID, req.NewEmail)
	if errors.Is(err, ErrUserAlreadyExists) {
		h.respondError(w, http.StatusConflict, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusAccepted, map[string]string{
		"status":     "pending_confirmation",
		"new_email":  change.NewEmail,
		"not_before": change.NotBefore.Format(time.RFC3339),
		"expires_at": change.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *AuthHandler) handleConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		var req ConfirmEmailChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		token = req.Token
	}

	change, err := h.service.ConfirmEmailChange(r.Context(), token)
	if errors.Is(err, ErrEmailChangePending) {
		h.respondJSON(w, http.StatusAccepted, map[string]string{
			"status":     "pending_confirmation",
			"new_email":  change.NewEmail,
			"not_before": change.NotBefore.Format(time.RFC3339),
		})
		return
	} else if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{
		"status":    "completed",
		"new_email": change.NewEmail,
	})
}

func (h *AuthHandler) handleAdminInviteUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req InviteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.InviteUser(r.Context(), userID, req.Email, req.RoleName)
	if errors.Is(err, ErrNotSuperAdmin) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	} else if errors.Is(err, ErrRoleNotFound) {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "user invited successfully"})
}

func (h *AuthHandler) handleGetInvitation(w http.ResponseWriter, r *http.Request) {
	token, err := decodeInvitationRouteToken(chi.URLParam(r, "token"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid token")
		return
	}
	if token == "" {
		h.respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	invite, err := h.service.GetInvitation(r.Context(), token)
	if errors.Is(err, ErrInvitationNotFound) {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	} else if errors.Is(err, ErrInvitationExpired) || errors.Is(err, ErrInvitationClaimed) {
		h.respondError(w, http.StatusGone, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"id":         invite.ID,
		"email":      invite.Email,
		"role_id":    invite.RoleID,
		"role_name":  invite.RoleName,
		"invited_by": invite.InvitedBy,
		"expires_at": invite.ExpiresAt,
	})
}

func (h *AuthHandler) handleClaimInvitation(w http.ResponseWriter, r *http.Request) {
	token, err := decodeInvitationRouteToken(chi.URLParam(r, "token"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid token")
		return
	}
	if token == "" {
		h.respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	var req ClaimInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	resp, err := h.service.ClaimInvitation(r.Context(), token, req.Password, req.DisplayName, ua, ip)
	if errors.Is(err, ErrInvitationNotFound) {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	} else if errors.Is(err, ErrInvitationExpired) || errors.Is(err, ErrInvitationClaimed) {
		h.respondError(w, http.StatusGone, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

func decodeInvitationRouteToken(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	return url.PathUnescape(raw)
}

func (h *AuthHandler) handleAdminListInvitations(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	invites, err := h.service.ListInvitations(r.Context(), userID)
	if errors.Is(err, ErrNotSuperAdmin) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	search := strings.ToLower(r.URL.Query().Get("search"))
	status := r.URL.Query().Get("status")

	var filtered []*Invitation
	for _, inv := range invites {
		matchesSearch := true
		if search != "" {
			emailMatch := strings.Contains(strings.ToLower(inv.Email), search)
			roleMatch := strings.Contains(strings.ToLower(inv.RoleName), search)
			matchesSearch = emailMatch || roleMatch
		}

		matchesStatus := true
		if status != "" && status != "all" {
			isClaimed := inv.ClaimedAt != nil
			isExpired := !isClaimed && time.Now().After(inv.ExpiresAt)
			isPending := !isClaimed && !isExpired

			switch status {
			case "claimed":
				matchesStatus = isClaimed
			case "expired":
				matchesStatus = isExpired
			case "pending":
				matchesStatus = isPending
			}
		}

		if matchesSearch && matchesStatus {
			filtered = append(filtered, inv)
		}
	}

	total := len(filtered)
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")
	page := 1
	pageSize := 10
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
		pageSize = ps
	}
	start := (page - 1) * pageSize
	var paginated []*Invitation
	if start < total {
		end := start + pageSize
		if end > total {
			end = total
		}
		paginated = filtered[start:end]
	} else {
		paginated = []*Invitation{}
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"invitations": paginated,
		"total":       total,
	})
}

func (h *AuthHandler) handleAdminRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		h.respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.service.RevokeInvitation(r.Context(), userID, id)
	if errors.Is(err, ErrNotSuperAdmin) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	} else if errors.Is(err, ErrInvitationNotFound) {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "invitation revoked successfully"})
}

func (h *AuthHandler) handleAdminResendInvitation(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		h.respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.service.ResendInvitation(r.Context(), userID, id)
	if errors.Is(err, ErrNotSuperAdmin) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	} else if errors.Is(err, ErrInvitationNotFound) {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "invitation resent successfully"})
}

func (h *AuthHandler) handleAdminResendUserVerification(w http.ResponseWriter, r *http.Request) {
	_, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		h.respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.service.AdminResendUserVerification(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrEmailAlreadyVerified) {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "verification email sent"})
}
