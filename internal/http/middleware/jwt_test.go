package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
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

func newKeyServer(t *testing.T, pubPEM, issuer, audience string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/auth/public-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"public_key": pubPEM,
			"issuer":     issuer,
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
	srv := newKeyServer(t, pub, "biqly-auth", "biqly")
	defer srv.Close()
	_ = priv

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_BypassPaths(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-auth", "biqly")
	defer srv.Close()
	_ = priv

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider, "/health", "/ready")

	for _, p := range []string{"/health", "/ready", "/health/sub"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		w := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("path %s expected 200, got %d", p, w.Code)
		}
	}
}

func TestJWTAuth_ValidTokenPopulatesContext(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-auth", "biqly-monolith")
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

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
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
	srv := newKeyServer(t, pub, "biqly-auth", "biqly-monolith")
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

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired, got %d", w.Code)
	}
}

func TestJWTAuth_WrongIssuerRejected(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-auth", "biqly-monolith")
	defer srv.Close()

	tokenStr := signToken(t, priv, JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "evil",
			Audience:  jwt.ClaimStrings{"biqly-monolith"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong issuer, got %d", w.Code)
	}
}

func TestJWTAuth_WrongAudienceRejected(t *testing.T) {
	priv, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-auth", "biqly-monolith")
	defer srv.Close()

	tokenStr := signToken(t, priv, JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "biqly-auth",
			Audience:  jwt.ClaimStrings{"someone-else"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong audience, got %d", w.Code)
	}
}

func TestJWTAuth_HS256TokenRejected(t *testing.T) {
	_, pub := newSigningKey(t)
	srv := newKeyServer(t, pub, "biqly-auth", "biqly-monolith")
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
	s, _ := tok.SignedString(hmacKey)

	provider := NewPublicKeyProvider(srv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+s)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for HS256 token, got %d", w.Code)
	}
}

func TestJWTAuth_KeyFetchFailureReturns503(t *testing.T) {
	// Provider pointed at a closed server simulates auth service down.
	closedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	closedSrv.Close()

	provider := NewPublicKeyProvider(closedSrv.URL, "tok")
	mw := JWTAuth(provider)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		if got := extractBearer(req); got != want {
			t.Errorf("extractBearer(%q) = %q, want %q", header, got, want)
		}
	}
}
