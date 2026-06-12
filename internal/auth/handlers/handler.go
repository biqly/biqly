package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/mfa"
	"github.com/biqly/biqly/internal/auth/oauth"
	bimw "github.com/biqly/biqly/internal/http/middleware"
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

func contextUserID(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(userIDKey).(string)
	return userID, ok && userID != ""
}

func (h *AuthHandler) requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := contextUserID(r)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	return userID, true
}

type AuthHandler struct {
	service  *auth.Service
	webAuthn *mfa.WebAuthnService
	jwtMgr   *auth.JWTManager
	config   *auth.Config
	limiter  *auth.RateLimiter
	mfa      *mfa.Service
	gdpr     *GDPRExporter
	audit    *auth.AuditService
}

func (h *AuthHandler) SetGDPRExporter(g *GDPRExporter)      { h.gdpr = g }
func (h *AuthHandler) SetAuditService(a *auth.AuditService) { h.audit = a }

func NewAuthHandler(service *auth.Service, webAuthn *mfa.WebAuthnService, jwtMgr *auth.JWTManager, config *auth.Config, limiter *auth.RateLimiter) *AuthHandler {
	return &AuthHandler{
		service:  service,
		webAuthn: webAuthn,
		jwtMgr:   jwtMgr,
		config:   config,
		limiter:  limiter,
	}
}

func (h *AuthHandler) RegisterAuthRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		if h.limiter != nil {
			r.Use(h.limiter.Limit(10, time.Minute, "login"))
		}
		r.Post("/login", h.handleLogin)
	})
	r.Group(func(r chi.Router) {
		if h.limiter != nil {
			r.Use(h.limiter.Limit(5, time.Minute, "register"))
		}
		r.Post("/register", h.handleRegister)
	})
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
	r.Get("/csrf", h.handleCSRF)

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
		r.Patch("/me/profile", h.handleUpdateProfile)
		r.Post("/me/password", h.handleChangePassword)
		r.Post("/me/mfa/bypass", h.handleMeGenerateMFABypass)
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
		invitationPagination := bimw.Paginate(bimw.PaginationConfig{
			DefaultPage:     1,
			DefaultPageSize: 10,
			MaxPageSize:     100,
		})
		r.Post("/admin/invitations", h.handleAdminInviteUser)
		r.With(invitationPagination).Get("/admin/invitations", h.handleAdminListInvitations)
		r.Delete("/admin/invitations/{id}", h.handleAdminRevokeInvitation)
		r.Post("/admin/invitations/{id}/resend", h.handleAdminResendInvitation)
		r.Get("/admin/platform-settings", h.handleAdminGetPlatformSettings)
		r.Put("/admin/platform-settings", h.handleAdminUpdatePlatformSettings)
		r.Get("/admin/ldap-config", h.handleAdminGetLDAPConfig)
		r.Put("/admin/ldap-config", h.handleAdminUpdateLDAPConfig)
		r.Post("/admin/ldap-config/test", h.handleAdminTestLDAP)
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
	req, ok := decodeJSON[auth.RegisterRequest](w, r)
	if !ok {
		return
	}

	resp, err := h.service.Register(r.Context(), req, new(r.UserAgent()), new(r.RemoteAddr))
	if errors.Is(err, auth.ErrSelfSignupDisabled) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondTokenResponse(w, r, http.StatusCreated, resp)
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[auth.LoginRequest](w, r)
	if !ok {
		return
	}

	resp, err := h.service.Login(r.Context(), req, new(r.UserAgent()), new(r.RemoteAddr))
	if errors.Is(err, auth.ErrInvalidCredentials) || errors.Is(err, auth.ErrInactiveUser) {
		// Return identical message for both to prevent account enumeration:
		// an attacker probing emails must not be able to distinguish
		// "wrong password" from "inactive account" from "no such user".
		h.respondError(w, http.StatusUnauthorized, auth.ErrInvalidCredentials.Error())
		return
	}
	switch {
	case errors.Is(err, auth.ErrAccountLocked):
		h.respondError(w, http.StatusTooManyRequests, err.Error())
		return
	case errors.Is(err, auth.ErrMFARequired):
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	case errors.Is(err, auth.ErrAccountFrozen), errors.Is(err, auth.ErrAccountDeleted):
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	case err != nil:
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondTokenResponse(w, r, http.StatusOK, resp)
}

func (h *AuthHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSONAllowEmpty[auth.RefreshRequest](w, r)
	if !ok {
		return
	}
	req.RefreshToken = h.refreshTokenFromRequest(r, req.RefreshToken)
	if req.RefreshToken == "" {
		h.respondError(w, http.StatusBadRequest, "refresh token required")
		return
	}

	resp, err := h.service.Refresh(r.Context(), req, new(r.UserAgent()), new(r.RemoteAddr))
	if errors.Is(err, auth.ErrSessionNotFound) || errors.Is(err, auth.ErrSessionExpired) || errors.Is(err, auth.ErrSessionRevoked) ||
		errors.Is(err, auth.ErrSessionAbsoluteExpired) || errors.Is(err, auth.ErrSessionIdleExpired) {
		h.clearRefreshTokenCookie(w, r)
		h.respondError(w, http.StatusUnauthorized, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondTokenResponse(w, r, http.StatusOK, resp)
}

func (h *AuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSONAllowEmpty[auth.RefreshRequest](w, r)
	if !ok {
		return
	}
	token := h.refreshTokenFromRequest(r, req.RefreshToken)
	if token == "" {
		h.clearRefreshTokenCookie(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err := h.service.Logout(r.Context(), token)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.clearRefreshTokenCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
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
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	req, ok := decodeJSON[auth.SetActiveWorkspaceRequest](w, r)
	if !ok {
		return
	}

	resp, err := h.service.SetActiveWorkspace(r.Context(), userID, req.WorkspaceID)
	if err != nil {
		if errors.Is(err, auth.ErrNotWorkspaceOwner) {
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
	req, ok := decodeJSON[VerifyRequest](w, r)
	if !ok {
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

	perms, err := h.service.RBACRepo().GetUserPermissions(r.Context(), userID)
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

		want := h.config.InternalToken
		if want == "" || tokenHeader == "" || subtle.ConstantTimeCompare([]byte(tokenHeader), []byte(want)) != 1 {
			h.respondError(w, http.StatusForbidden, "forbidden: invalid internal token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (*AuthHandler) respondJSON(w http.ResponseWriter, status int, data any) {
	response.WriteJSON(w, status, data)
}

// respondError writes a JSON error. For server-side failures (5xx) the
// detailed message is sent to logs/telemetry only and the client receives a
// generic, user-friendly message so internal details never leak. Client
// errors (4xx) keep their message, which is intentional validation feedback.
func (*AuthHandler) respondError(w http.ResponseWriter, status int, message string) {
	response.WriteError(w, status, message)
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	auth.WriteResponseCookie(w, r, h.config.Port, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter, r *http.Request, name string) {
	h.setSessionCookie(w, r, name, "", -1)
}

const refreshTokenCookieName = "biqly_refresh_token" //nolint:gosec // G101: cookie name, not a credential

func (h *AuthHandler) refreshTokenMaxAge() int {
	secs := int(h.config.JWTRefreshTTL.Seconds())
	if secs < 1 {
		return 1
	}
	return secs
}

func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	h.setSessionCookie(w, r, refreshTokenCookieName, token, h.refreshTokenMaxAge())
}

func (h *AuthHandler) clearRefreshTokenCookie(w http.ResponseWriter, r *http.Request) {
	h.clearSessionCookie(w, r, refreshTokenCookieName)
}

func (*AuthHandler) refreshTokenFromRequest(r *http.Request, bodyToken string) string {
	if bodyToken != "" {
		return bodyToken
	}
	cookie, err := r.Cookie(refreshTokenCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}

// respondTokenResponse stores the refresh token in an httpOnly cookie and omits
// it from the JSON body so browser clients never persist it in localStorage.
func (h *AuthHandler) respondTokenResponse(w http.ResponseWriter, r *http.Request, status int, resp *auth.TokenResponse) {
	if resp != nil && resp.RefreshToken != "" {
		h.setRefreshTokenCookie(w, r, resp.RefreshToken)
		resp.RefreshToken = ""
	}
	h.respondJSON(w, status, resp)
}

func (h *AuthHandler) handleOAuthRedirect(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, err := oauth.NewOAuthProvider(providerName, h.config)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	state, err := h.generateState()
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	bindToken, err := h.generateState()
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to generate bind token")
		return
	}

	if err := h.service.StoreOAuthState(r.Context(), providerName, bindToken, state); err != nil {
		slog.Error("failed to store oauth state", "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to initiate oauth")
		return
	}

	h.setSessionCookie(w, r, "oauth_bind_"+providerName, bindToken, 300)

	authURL := provider.GetAuthURL(state)
	if err := oauth.ValidateAuthURL(providerName, authURL); err != nil {
		slog.Error("invalid oauth auth url", "provider", providerName, "error", err)
		h.respondError(w, http.StatusInternalServerError, "invalid oauth redirect")
		return
	}
	//nolint:gosec // authURL host validated against provider allowlist
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider, err := oauth.NewOAuthProvider(providerName, h.config)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	stateParam := r.URL.Query().Get("state")
	codeParam := r.URL.Query().Get("code")

	cookie, err := r.Cookie("oauth_bind_" + providerName)
	if err != nil || cookie.Value == "" || stateParam == "" {
		h.respondError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	matched, err := h.service.VerifyOAuthState(r.Context(), providerName, cookie.Value, stateParam)
	if err != nil || !matched {
		h.respondError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	// Clear the state cookie
	h.clearSessionCookie(w, r, "oauth_bind_"+providerName)

	token, err := provider.ExchangeCode(r.Context(), codeParam)
	if err != nil {
		slog.ErrorContext(r.Context(), "oauth code exchange failed", "provider", providerName, "err", err)
		h.respondError(w, http.StatusBadGateway, "oauth provider unavailable")
		return
	}

	userInfo, err := provider.GetUserInfo(r.Context(), token)
	if err != nil {
		slog.ErrorContext(r.Context(), "oauth userinfo failed", "provider", providerName, "err", err)
		h.respondError(w, http.StatusBadGateway, "oauth provider unavailable")
		return
	}

	resp, err := h.service.LoginOrRegisterOAuth(r.Context(), providerName, token, userInfo, new(r.UserAgent()), new(r.RemoteAddr))
	if errors.Is(err, auth.ErrSelfSignupDisabled) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	}
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
	req, ok := decodeJSON[OAuthExchangeRequest](w, r)
	if !ok {
		return
	}
	if req.Code == "" {
		h.respondError(w, http.StatusBadRequest, "code is required")
		return
	}

	resp, err := h.service.RedeemOAuthCallbackCode(r.Context(), req.Code)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidOAuthCallbackCode) {
			h.respondError(w, http.StatusBadRequest, "invalid or expired code")
			return
		}
		if errors.Is(err, auth.ErrOAuthExchangeUnavailable) {
			h.respondError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondTokenResponse(w, r, http.StatusOK, resp)
}

func (*AuthHandler) generateState() (string, error) {
	b := make([]byte, 32)
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
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.service.UserRepo().GetUserByID(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	creation, session, err := h.webAuthn.BeginRegistration(r.Context(), user)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	sessionJSON, err := sonic.ConfigStd.Marshal(session)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sessionB64 := base64.StdEncoding.EncodeToString(sessionJSON)
	h.setSessionCookie(w, r, "webauthn_register_session", signProtectedCookie(h.config.InternalToken, sessionB64), 300)

	h.respondJSON(w, http.StatusOK, creation)
}

func (h *AuthHandler) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	cookie, err := r.Cookie("webauthn_register_session")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "registration session cookie missing")
		return
	}

	sessionB64, ok := verifyProtectedCookie(h.config.InternalToken, cookie.Value)
	if !ok {
		h.respondError(w, http.StatusBadRequest, "invalid session cookie")
		return
	}
	sessionJSON, err := base64.StdEncoding.DecodeString(sessionB64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session cookie format")
		return
	}

	var session webauthn.SessionData
	if err := sonic.ConfigStd.Unmarshal(sessionJSON, &session); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session data")
		return
	}

	name := r.URL.Query().Get("name")

	user, err := h.service.UserRepo().GetUserByID(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = h.webAuthn.FinishRegistration(r.Context(), user, &session, r, name)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.clearSessionCookie(w, r, "webauthn_register_session")

	h.respondJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *AuthHandler) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[PasskeyLoginBeginRequest](w, r)
	if !ok {
		return
	}

	assertion, session, err := h.webAuthn.BeginLogin(r.Context(), req.Email)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	sessionJSON, err := sonic.ConfigStd.Marshal(session)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sessionB64 := base64.StdEncoding.EncodeToString(sessionJSON)
	h.setSessionCookie(w, r, "webauthn_login_session", signProtectedCookie(h.config.InternalToken, sessionB64), 300)

	h.respondJSON(w, http.StatusOK, assertion)
}

func (h *AuthHandler) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("webauthn_login_session")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "login session cookie missing")
		return
	}

	sessionB64, ok := verifyProtectedCookie(h.config.InternalToken, cookie.Value)
	if !ok {
		h.respondError(w, http.StatusBadRequest, "invalid session cookie")
		return
	}
	sessionJSON, err := base64.StdEncoding.DecodeString(sessionB64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session cookie format")
		return
	}

	var session webauthn.SessionData
	if err := sonic.ConfigStd.Unmarshal(sessionJSON, &session); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid session data")
		return
	}

	user, err := h.webAuthn.FinishLogin(r.Context(), &session, r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.clearSessionCookie(w, r, "webauthn_login_session")

	resp, err := h.service.CreateTokenResponseForUser(r.Context(), user, new(r.UserAgent()), new(r.RemoteAddr))
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondTokenResponse(w, r, http.StatusOK, resp)
}

func (h *AuthHandler) handleMePasskeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
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
	userID, ok := h.requireUserID(w, r)
	if !ok {
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
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	passkeyID := chi.URLParam(r, "id")
	if passkeyID == "" {
		h.respondError(w, http.StatusBadRequest, "passkey id is required")
		return
	}

	req, ok := decodeJSON[struct {
		Name string `json:"name"`
	}](w, r)
	if !ok {
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

func (h *AuthHandler) handleEmailAction(
	w http.ResponseWriter,
	r *http.Request,
	action func(context.Context, string) error,
	okMessage string,
	errStatus int,
) {
	req, ok := decodeJSON[struct {
		Email string `json:"email"`
	}](w, r)
	if !ok {
		return
	}
	if err := action(r.Context(), req.Email); err != nil {
		h.respondError(w, errStatus, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"message": okMessage})
}

func (h *AuthHandler) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	h.handleEmailAction(
		w, r,
		h.service.ForgotPassword,
		"If the email exists, a password reset link has been sent.",
		http.StatusInternalServerError,
	)
}

func (h *AuthHandler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[ResetPasswordRequest](w, r)
	if !ok {
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
	req, ok := decodeJSON[auth.MagicLinkRequest](w, r)
	if !ok {
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
	req, ok := decodeJSON[auth.MagicLinkConsumeRequest](w, r)
	if !ok {
		return
	}
	resp, err := h.service.ConsumeMagicLink(r.Context(), req.Token, new(r.UserAgent()), new(r.RemoteAddr))
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrMagicLinkInvalid), errors.Is(err, auth.ErrMagicLinkUsed):
			h.respondError(w, http.StatusBadRequest, auth.ErrMagicLinkInvalid.Error())
		case errors.Is(err, auth.ErrInactiveUser):
			h.respondError(w, http.StatusUnauthorized, auth.ErrInactiveUser.Error())
		case errors.Is(err, auth.ErrAccountFrozen), errors.Is(err, auth.ErrAccountDeleted):
			h.respondError(w, http.StatusUnauthorized, err.Error())
		default:
			h.respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	h.respondTokenResponse(w, r, http.StatusOK, resp)
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
	h.handleEmailAction(
		w, r,
		h.service.ResendVerificationEmail,
		"If the email is not verified, a new verification link has been sent.",
		http.StatusBadRequest,
	)
}

func (h *AuthHandler) handleRequestEmailChange(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	req, ok := decodeJSON[auth.RequestEmailChangeRequest](w, r)
	if !ok {
		return
	}

	change, err := h.service.RequestEmailChange(r.Context(), userID, req.NewEmail)
	if errors.Is(err, auth.ErrUserAlreadyExists) {
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
		req, ok := decodeJSON[ConfirmEmailChangeRequest](w, r)
		if !ok {
			return
		}
		token = req.Token
	}

	change, err := h.service.ConfirmEmailChange(r.Context(), token)
	if errors.Is(err, auth.ErrEmailChangePending) {
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
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	req, ok := decodeJSON[auth.InviteUserRequest](w, r)
	if !ok {
		return
	}

	err := h.service.InviteUser(r.Context(), userID, req.Email, req.RoleName)
	switch {
	case errors.Is(err, auth.ErrNotSuperAdmin):
		h.respondError(w, http.StatusForbidden, auth.ErrNotSuperAdmin.Error())
		return
	case errors.Is(err, auth.ErrRoleNotFound):
		h.respondError(w, http.StatusNotFound, auth.ErrRoleNotFound.Error())
		return
	case err != nil:
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
	switch {
	case errors.Is(err, auth.ErrInvitationNotFound):
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	case errors.Is(err, auth.ErrInvitationExpired), errors.Is(err, auth.ErrInvitationClaimed):
		h.respondError(w, http.StatusGone, err.Error())
		return
	case err != nil:
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

	req, ok := decodeJSON[auth.ClaimInvitationRequest](w, r)
	if !ok {
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	resp, err := h.service.ClaimInvitation(r.Context(), token, req.Password, req.DisplayName, ua, ip)
	switch {
	case errors.Is(err, auth.ErrInvitationNotFound):
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	case errors.Is(err, auth.ErrInvitationExpired), errors.Is(err, auth.ErrInvitationClaimed):
		h.respondError(w, http.StatusGone, err.Error())
		return
	case err != nil:
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondTokenResponse(w, r, http.StatusOK, resp)
}

func decodeInvitationRouteToken(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	return url.PathUnescape(raw)
}

func (h *AuthHandler) handleAdminListInvitations(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	invites, err := h.service.ListInvitations(r.Context(), userID)
	if errors.Is(err, auth.ErrNotSuperAdmin) {
		h.respondError(w, http.StatusForbidden, err.Error())
		return
	} else if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	search := strings.ToLower(r.URL.Query().Get("search"))
	status := r.URL.Query().Get("status")

	var filtered []*auth.Invitation
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
	pagination := bimw.PaginationFromContext(r.Context())
	start := pagination.Offset
	pageSize := pagination.PageSize

	var paginated []*auth.Invitation
	if start < total {
		end := min(start+pageSize, total)
		paginated = filtered[start:end]
	} else {
		paginated = []*auth.Invitation{}
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"invitations": paginated,
		"total":       total,
	})
}

func (h *AuthHandler) handleAdminInvitationAction(
	w http.ResponseWriter,
	r *http.Request,
	action func(context.Context, string, string) error,
	okMessage string,
) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		h.respondError(w, http.StatusBadRequest, "id is required")
		return
	}
	err := action(r.Context(), userID, id)
	switch {
	case errors.Is(err, auth.ErrNotSuperAdmin):
		h.respondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, auth.ErrInvitationNotFound):
		h.respondError(w, http.StatusNotFound, err.Error())
	case err != nil:
		h.respondError(w, http.StatusInternalServerError, err.Error())
	default:
		h.respondJSON(w, http.StatusOK, map[string]string{"message": okMessage})
	}
}

func (h *AuthHandler) handleAdminRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	h.handleAdminInvitationAction(w, r, h.service.RevokeInvitation, "invitation revoked successfully")
}

func (h *AuthHandler) handleAdminResendInvitation(w http.ResponseWriter, r *http.Request) {
	h.handleAdminInvitationAction(w, r, h.service.ResendInvitation, "invitation resent successfully")
}

func (h *AuthHandler) handleAdminResendUserVerification(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireUserID(w, r); !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		h.respondError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.service.AdminResendUserVerification(r.Context(), id)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, auth.ErrEmailAlreadyVerified) {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "verification email sent"})
}
