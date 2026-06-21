package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newSigningKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: x509MarshalPKIX(t, &priv.PublicKey),
	})
	return priv, string(pubPEM)
}

func x509MarshalPKIX(t *testing.T, pub *rsa.PublicKey) []byte {
	t.Helper()
	b, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal pkix: %v", err)
	}
	return b
}

func newKeyServer(t *testing.T, pubPEM, audience string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/public-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = sonic.ConfigStd.NewEncoder(w).Encode(map[string]string{
			"public_key": pubPEM,
			"issuer":     "biqly-auth",
			"audience":   audience,
		})
	})
	return httptest.NewServer(mux)
}

func signToken(t *testing.T, priv *rsa.PrivateKey, claims JWTClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestJWTAuth_MissingTokenRejected(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()
	_ = priv

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_BypassPaths(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()
	_ = priv

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider, "/health", "/ready")

	for _, p := range []string{"/health", "/ready"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, p, nil)
		w := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("path %s expected 200, got %d", p, w.Code)
		}
	}

	// Prefix match must not bypass — only exact paths are exempt.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/sub", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("/health/sub expected 401 without token, got %d", w.Code)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthcheck", nil)
	w = httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("/healthcheck expected 401 without token, got %d", w.Code)
	}
}

func TestJWTAuth_ValidTokenPopulatesContext(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-monolith")
	defer srv.Close()

	tokenStr := signToken(t, priv, JWTClaims{
		Email:                 "u@x.com",
		Roles:                 []string{"analyst", "developer"},
		WorkspaceID:           "ws-1",
		AccessibleDatasources: []string{"ds-a", "ds-b"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "biqly-auth",
			Audience:  jwt.ClaimStrings{"biqly-monolith"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	var gotUserID, gotEmail, gotWorkspace string
	var gotRoles, gotDS []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserID(r.Context())
		gotEmail, _ = r.Context().Value(UserEmailKey).(string)
		gotRoles = UserRoles(r.Context())
		gotWorkspace = WorkspaceID(r.Context())
		gotDS = AccessibleDatasources(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if gotUserID != "user-1" || gotEmail != "u@x.com" || gotWorkspace != "ws-1" {
		t.Fatalf("ctx mismatch: user=%q email=%q ws=%q", gotUserID, gotEmail, gotWorkspace)
	}
	if len(gotRoles) != 2 || gotRoles[0] != "analyst" {
		t.Fatalf("roles mismatch: %+v", gotRoles)
	}
	if len(gotDS) != 2 || gotDS[1] != "ds-b" {
		t.Fatalf("ds mismatch: %+v", gotDS)
	}
	ctx := context.WithValue(context.Background(), UserRolesKey, gotRoles)
	if !HasRole(ctx, "developer") {
		t.Fatalf("HasRole(developer) should be true for roles %+v", gotRoles)
	}
}

func TestJWTAuth_ExpiredTokenRejected(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-monolith")
	defer srv.Close()

	tokenStr := signToken(t, priv, JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "biqly-auth",
			Audience:  jwt.ClaimStrings{"biqly-monolith"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
		},
	})

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired, got %d", w.Code)
	}
}

func assertJWTRejected(t *testing.T, claims JWTClaims, label string) {
	t.Helper()
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-monolith")
	defer srv.Close()

	tokenStr := signToken(t, priv, claims)
	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for %s, got %d", label, w.Code)
	}
}

func TestJWTAuth_WrongIssuerRejected(t *testing.T) {
	assertJWTRejected(t, JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "evil",
			Audience:  jwt.ClaimStrings{"biqly-monolith"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}, "wrong issuer")
}

func TestJWTAuth_WrongAudienceRejected(t *testing.T) {
	assertJWTRejected(t, JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "biqly-auth",
			Audience:  jwt.ClaimStrings{"someone-else"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}, "wrong audience")
}

func TestJWTAuth_HS256TokenRejected(t *testing.T) {
	_, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-monolith")
	defer srv.Close()

	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		t.Fatalf("rand: %v", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "user-1",
		Issuer:    "biqly-auth",
		Audience:  jwt.ClaimStrings{"biqly-monolith"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	})
	s, err := tok.SignedString(hmacKey)
	if err != nil {
		t.Fatal(err)
	}

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+s)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for HS256 token, got %d", w.Code)
	}
}

func TestJWTAuth_KeyFetchFailureReturns503(t *testing.T) {
	// Provider pointed at a closed server simulates auth service down.
	closedSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	closedSrv.Close()

	provider := NewPublicKeyProvider(closedSrv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer something")
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when key fetch fails, got %d", w.Code)
	}
}

func TestExtractBearer(t *testing.T) {
	cases := map[string]string{
		"Bearer abc":     "abc",
		"bearer xyz":     "xyz",
		"BeArEr  spaced": "spaced",
		"":               "",
		"abc":            "",
		"Bearer ":        "",
	}
	for header, want := range cases {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		if got := extractBearer(req); got != want {
			t.Errorf("extractBearer(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestRequireVerifiedEmail(t *testing.T) {
	allowed := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireVerifiedEmail()

	// 1. GET passes through regardless of verification state.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	mw(allowed).ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), EmailVerifiedKey, false)))
	if w.Code != http.StatusOK {
		t.Fatalf("GET should bypass verification gate, got %d", w.Code)
	}

	// 2. POST with unverified email returns 403 email_verification_required.
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", nil)
	w = httptest.NewRecorder()
	mw(allowed).ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), EmailVerifiedKey, false)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST unverified should be 403, got %d", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, "email_verification_required") {
		t.Fatalf("expected error code in body, got %q", got)
	}

	// 3. POST with verified email passes through.
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x", nil)
	w = httptest.NewRecorder()
	mw(allowed).ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), EmailVerifiedKey, true)))
	if w.Code != http.StatusOK {
		t.Fatalf("POST verified should be 200, got %d", w.Code)
	}
}

func TestOptionalJWTAuth_ValidTokenPopulatesIdentity(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()

	tokenStr := signToken(t, priv, JWTClaims{
		Email: "u@x.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-42",
			Issuer:    "biqly-auth",
			Audience:  jwt.ClaimStrings{"biqly"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	mw := OptionalJWTAuth(NewPublicKeyProvider(srv.URL, "tok"))
	var gotUserID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/user-preferences", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUserID != "user-42" {
		t.Fatalf("expected user-42, got %q", gotUserID)
	}
}

func TestOptionalJWTAuth_NoTokenPassesWithoutIdentity(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()
	_ = priv

	mw := OptionalJWTAuth(NewPublicKeyProvider(srv.URL, "tok"))
	var sawUserID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawUserID = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/user-models", nil)
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected pass-through 200, got %d", w.Code)
	}
	if sawUserID != "" {
		t.Fatalf("expected empty userID, got %q", sawUserID)
	}
}

func TestContextSettersAndGetters(t *testing.T) {
	t.Run("WithUserID/UserID round-trip", func(t *testing.T) {
		ctx := WithUserID(context.Background(), "u-42")
		if got := UserID(ctx); got != "u-42" {
			t.Fatalf("UserID = %q, want %q", got, "u-42")
		}
		// Empty string when not set
		if got := UserID(context.Background()); got != "" {
			t.Fatalf("UserID on bare ctx = %q, want \"\"", got)
		}
	})

	t.Run("WithUserEmail/UserEmail round-trip", func(t *testing.T) {
		ctx := WithUserEmail(context.Background(), "a@b.com")
		if got := UserEmail(ctx); got != "a@b.com" {
			t.Fatalf("UserEmail = %q, want %q", got, "a@b.com")
		}
		if got := UserEmail(context.Background()); got != "" {
			t.Fatalf("UserEmail on bare ctx = %q, want \"\"", got)
		}
	})

	t.Run("WithUserRoles/UserRoles round-trip", func(t *testing.T) {
		roles := []string{"admin", "viewer"}
		ctx := WithUserRoles(context.Background(), roles)
		got := UserRoles(ctx)
		if len(got) != 2 || got[0] != "admin" || got[1] != "viewer" {
			t.Fatalf("UserRoles = %+v, want %+v", got, roles)
		}
		if got := UserRoles(context.Background()); got != nil {
			t.Fatalf("UserRoles on bare ctx = %+v, want nil", got)
		}
	})

	t.Run("WithWorkspaceID/WorkspaceID round-trip", func(t *testing.T) {
		ctx := WithWorkspaceID(context.Background(), "ws-99")
		if got := WorkspaceID(ctx); got != "ws-99" {
			t.Fatalf("WorkspaceID = %q, want %q", got, "ws-99")
		}
		if got := WorkspaceID(context.Background()); got != "" {
			t.Fatalf("WorkspaceID on bare ctx = %q, want \"\"", got)
		}
	})

	t.Run("WithEmailVerified/EmailVerified round-trip", func(t *testing.T) {
		ctx := WithEmailVerified(context.Background(), true)
		if got := EmailVerified(ctx); got != true {
			t.Fatalf("EmailVerified = %v, want true", got)
		}
		ctx = WithEmailVerified(context.Background(), false)
		if got := EmailVerified(ctx); got != false {
			t.Fatalf("EmailVerified = %v, want false", got)
		}
		if got := EmailVerified(context.Background()); got != false {
			t.Fatalf("EmailVerified on bare ctx = %v, want false", got)
		}
	})

	t.Run("AccessibleDatasources on bare context", func(t *testing.T) {
		if got := AccessibleDatasources(context.Background()); got != nil {
			t.Fatalf("AccessibleDatasources on bare ctx = %+v, want nil", got)
		}
	})

	t.Run("HasRole false on bare context", func(t *testing.T) {
		if HasRole(context.Background(), "anything") {
			t.Fatal("HasRole on bare ctx should be false")
		}
	})
}

func TestOptionalJWTAuth_NonJWTBearerNotRejected(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()
	_ = priv

	mw := OptionalJWTAuth(NewPublicKeyProvider(srv.URL, "tok"))
	reached := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if UserID(r.Context()) != "" {
			t.Fatalf("admin-key bearer must not populate identity")
		}
		w.WriteHeader(http.StatusOK)
	})

	// An admin API key is not a JWT — the middleware must let it through so an
	// inner AdminKeyMiddleware can validate it, rather than rejecting it.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/providers", nil)
	req.Header.Set("Authorization", "Bearer some-admin-api-key")
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	if !reached || w.Code != http.StatusOK {
		t.Fatalf("expected pass-through 200, reached=%v code=%d", reached, w.Code)
	}
}

func TestJWTAuthWithAdminBypass_AdminKeyBypasses(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()
	_ = priv

	adminKey := "s3cret-admin-api-key"
	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuthWithAdminBypass(provider, []string{adminKey})

	// Case 1: Authorization: Bearer <adminKey>
	var gotUserID string
	var gotRoles []string
	var gotEmailVerified bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserID(r.Context())
		gotRoles = UserRoles(r.Context())
		gotEmailVerified = EmailVerified(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Bearer adminKey: expected 200, got %d", w.Code)
	}
	if gotUserID != "admin" {
		t.Fatalf("Bearer adminKey: expected UserID='admin', got %q", gotUserID)
	}
	if len(gotRoles) != 1 || gotRoles[0] != RoleSuperAdmin {
		t.Fatalf("Bearer adminKey: expected roles=[%s], got %+v", RoleSuperAdmin, gotRoles)
	}
	if !gotEmailVerified {
		t.Fatalf("Bearer adminKey: expected EmailVerified=true")
	}

	// Case 2: X-API-Key: <adminKey>
	gotUserID = ""
	gotRoles = nil
	gotEmailVerified = false

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("X-API-Key", adminKey)
	w = httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("X-API-Key adminKey: expected 200, got %d", w.Code)
	}
	if gotUserID != "admin" {
		t.Fatalf("X-API-Key adminKey: expected UserID='admin', got %q", gotUserID)
	}
	if len(gotRoles) != 1 || gotRoles[0] != RoleSuperAdmin {
		t.Fatalf("X-API-Key adminKey: expected roles=[%s], got %+v", RoleSuperAdmin, gotRoles)
	}
	if !gotEmailVerified {
		t.Fatalf("X-API-Key adminKey: expected EmailVerified=true")
	}

	// Case 3: Invalid admin key and no valid JWT -> Rejected
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer invalid-admin-key")
	w = httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid adminKey expected 401, got %d", w.Code)
	}
}

// TestJWTAuthWithAdminBypass_SecondKeyAccepted pins that every configured key
// is a valid bypass credential — BI_API_KEY and BI_ADMIN_API_KEY gate
// different layers but both must pass the JWT-enforcement edge. Empty entries
// (an unset key) must not allow empty credentials through.
func TestJWTAuthWithAdminBypass_SecondKeyAccepted(t *testing.T) {
	_, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuthWithAdminBypass(provider, []string{"", "admin-only-key"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer admin-only-key")
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("second key: expected 200, got %d", w.Code)
	}

	// The blank first entry must not be matchable.
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("X-API-Key", "")
	w = httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("empty credential: expected 401, got %d", w.Code)
	}
}

func TestJWTAuthWithAdminBypass_FallbackToJWT(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly")
	defer srv.Close()

	tokenStr := signToken(t, priv, JWTClaims{
		Email:                 "u@x.com",
		Roles:                 []string{"developer"},
		WorkspaceID:           "ws-1",
		AccessibleDatasources: []string{"ds-a"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "biqly-auth",
			Audience:  jwt.ClaimStrings{"biqly"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuthWithAdminBypass(provider, []string{"s3cret-admin-api-key"})

	var gotUserID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("JWT fallback expected 200, got %d", w.Code)
	}
	if gotUserID != "user-1" {
		t.Fatalf("JWT fallback expected UserID='user-1', got %q", gotUserID)
	}
}
