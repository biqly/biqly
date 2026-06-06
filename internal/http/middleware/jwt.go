package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const (
	UserIDKey        ctxKey = "auth.user_id"
	UserEmailKey     ctxKey = "auth.email"
	UserRolesKey     ctxKey = "auth.roles"
	WorkspaceIDKey   ctxKey = "auth.workspace_id"
	AccessibleDSKey  ctxKey = "auth.accessible_datasources"
	PermissionsKey   ctxKey = "auth.permissions"
	EmailVerifiedKey ctxKey = "auth.email_verified"
)

type JWTClaims struct {
	Email                 string   `json:"email"`
	Roles                 []string `json:"roles"`
	WorkspaceID           string   `json:"workspace_id,omitempty"`
	AccessibleDatasources []string `json:"accessible_datasources,omitempty"`
	EmailVerified         bool     `json:"email_verified,omitempty"`
	jwt.RegisteredClaims
}

// PublicKeyProvider fetches the JWT public key from auth service /internal/auth/public-key.
type PublicKeyProvider struct {
	authServiceURL string
	internalToken  string
	httpClient     *http.Client

	mu        sync.RWMutex
	publicKey *rsa.PublicKey
	issuer    string
	audience  string
	fetchedAt time.Time
	ttl       time.Duration
}

func NewPublicKeyProvider(authServiceURL, internalToken string) *PublicKeyProvider {
	return &PublicKeyProvider{
		authServiceURL: strings.TrimRight(authServiceURL, "/"),
		internalToken:  internalToken,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		ttl:            1 * time.Hour,
	}
}

// jwtConfig groups the public key with the expected issuer/audience.
type jwtConfig struct {
	key      *rsa.PublicKey
	issuer   string
	audience string
}

func (p *PublicKeyProvider) getConfig(ctx context.Context) (*jwtConfig, error) {
	p.mu.RLock()
	if p.publicKey != nil && time.Since(p.fetchedAt) < p.ttl {
		cfg := &jwtConfig{key: p.publicKey, issuer: p.issuer, audience: p.audience}
		p.mu.RUnlock()
		return cfg, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.publicKey != nil && time.Since(p.fetchedAt) < p.ttl {
		return &jwtConfig{key: p.publicKey, issuer: p.issuer, audience: p.audience}, nil
	}

	url := p.authServiceURL + "/internal/auth/public-key"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build public-key request: %w", err)
	}
	req.Header.Set("X-Internal-Token", p.internalToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch public key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("public-key endpoint returned %d", resp.StatusCode)
	}

	var body struct {
		PublicKey string `json:"public_key"`
		Issuer    string `json:"issuer"`
		Audience  string `json:"audience"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode public-key response: %w", err)
	}

	key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(body.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	p.publicKey = key
	p.issuer = body.Issuer
	p.audience = body.Audience
	p.fetchedAt = time.Now()
	return &jwtConfig{key: key, issuer: body.Issuer, audience: body.Audience}, nil
}

// JWTAuth verifies the Bearer JWT against the auth service's public key.
// Bypass paths are exempted (e.g. /health, /ready).
func JWTAuth(provider *PublicKeyProvider, bypassPaths ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range bypassPaths {
				if p != "" && strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			cfg, err := provider.getConfig(r.Context())
			if err != nil {
				writeAuthError(w, http.StatusServiceUnavailable, "auth key unavailable")
				return
			}

			claims, err := verifyJWTClaims(cfg, tokenStr)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			next.ServeHTTP(w, r.WithContext(applyJWTClaims(r.Context(), claims)))
		})
	}
}

// OptionalJWTAuth populates the user identity from a valid Bearer JWT when one
// is present, but never rejects the request. Requests with no token, an
// unverifiable token (e.g. an admin API key), or a temporarily unavailable
// auth key simply proceed without identity. This lets services that are not
// JWT-enforced (BI_AUTH_ENABLED=false) still resolve the caller for per-user
// features while keeping admin-key and unauthenticated routes working.
func OptionalJWTAuth(provider *PublicKeyProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				next.ServeHTTP(w, r)
				return
			}
			cfg, err := provider.getConfig(r.Context())
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			claims, err := verifyJWTClaims(cfg, tokenStr)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(applyJWTClaims(r.Context(), claims)))
		})
	}
}

func verifyJWTClaims(cfg *jwtConfig, tokenStr string) (*JWTClaims, error) {
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
	}
	if cfg.issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(cfg.issuer))
	}
	if cfg.audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(cfg.audience))
	}
	parser := jwt.NewParser(parserOpts...)

	claims := &JWTClaims{}
	tok, err := parser.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return cfg.key, nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func applyJWTClaims(ctx context.Context, claims *JWTClaims) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, claims.Subject)
	ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
	ctx = context.WithValue(ctx, UserRolesKey, claims.Roles)
	ctx = context.WithValue(ctx, WorkspaceIDKey, claims.WorkspaceID)
	ctx = context.WithValue(ctx, AccessibleDSKey, claims.AccessibleDatasources)
	ctx = context.WithValue(ctx, EmailVerifiedKey, claims.EmailVerified)
	return ctx
}

func extractBearer(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
		return strings.TrimSpace(v[len(prefix):])
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Warn("write auth error response failed", "status", status, "error", err)
	}
}

// Context helpers

func UserID(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

func UserRoles(ctx context.Context) []string {
	v, _ := ctx.Value(UserRolesKey).([]string)
	return v
}

func WorkspaceID(ctx context.Context) string {
	v, _ := ctx.Value(WorkspaceIDKey).(string)
	return v
}

func AccessibleDatasources(ctx context.Context) []string {
	v, _ := ctx.Value(AccessibleDSKey).([]string)
	return v
}

func HasRole(ctx context.Context, role string) bool {
	return slices.Contains(UserRoles(ctx), role)
}

// EmailVerified reports whether the JWT was issued for a user whose email
// ownership has been confirmed. Callers should treat this as the source of
// truth for the request lifetime; users.email_verified flips can take up to
// one access-token TTL to propagate.
func EmailVerified(ctx context.Context) bool {
	v, _ := ctx.Value(EmailVerifiedKey).(bool)
	return v
}

// RequireVerifiedEmail blocks the request when the JWT lacks the
// email_verified claim. Apply after JWTAuth on write routes (POST/PUT/PATCH/DELETE)
// to enforce a read-only experience for unverified accounts. The 403 response
// uses a stable error code so the frontend can prompt the user to verify.
func RequireVerifiedEmail() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GET/HEAD/OPTIONS are read-only and allowed even pre-verification
			// so the user can still browse the app before confirming.
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if !EmailVerified(r.Context()) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "email_verification_required",
					"hint":  "Confirm your email address to enable write actions.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
