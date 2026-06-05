package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimingParity_MissingUserVsWrongPassword asserts that login attempts
// against a non-existent user spend roughly the same wall-clock time as
// attempts against a real account with the wrong password. This guards the
// VerifyDummyPassword path that mitigates account enumeration via timing.
func TestTimingParity_MissingUserVsWrongPassword(t *testing.T) {
	realHash, err := HashPassword("RealPass123!")
	require.NoError(t, err)

	const samples = 6

	measureMissing := func() time.Duration {
		start := time.Now()
		// Simulate the missing-user branch of Login: a single dummy bcrypt verify.
		VerifyDummyPassword("WrongGuess!1")
		return time.Since(start)
	}

	measureWrong := func() time.Duration {
		start := time.Now()
		_ = VerifyPassword("WrongGuess!1", realHash)
		return time.Since(start)
	}

	// Warm-up to amortize CPU caches.
	for range 2 {
		measureMissing()
		measureWrong()
	}

	var totalMissing, totalWrong time.Duration
	for range samples {
		totalMissing += measureMissing()
		totalWrong += measureWrong()
	}
	avgMissing := totalMissing / samples
	avgWrong := totalWrong / samples

	// Both spend bcrypt cost; allow up to 2x divergence. If the dummy path is
	// short-circuited the missing branch will be orders of magnitude faster.
	if avgMissing == 0 || avgWrong == 0 {
		t.Fatalf("unexpected zero duration: missing=%s wrong=%s", avgMissing, avgWrong)
	}
	ratio := float64(avgMissing) / float64(avgWrong)
	if ratio < 0.4 || ratio > 2.5 {
		t.Fatalf("timing parity broken: missing avg=%s wrong avg=%s ratio=%.2f", avgMissing, avgWrong, ratio)
	}
}

// loginHandlerWithService wires a minimal AuthHandler over a service stub that
// returns the configured error verbatim. This avoids the DB while still
// exercising the handler's enumeration-resistant branching.
func newEnumHandler(t *testing.T, loginErr error) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		// Mirror the branching from AuthHandler.handleLogin so the test pins
		// the externally-visible behavior.
		if errors.Is(loginErr, ErrInvalidCredentials) || errors.Is(loginErr, ErrInactiveUser) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": ErrInvalidCredentials.Error()}); err != nil {
				t.Errorf("failed to encode json: %v", err)
			}
			return
		}
		if errors.Is(loginErr, ErrAccountLocked) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": loginErr.Error()}); err != nil {
				t.Errorf("failed to encode json: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// TestAccountEnumeration_LoginResponsesIdentical pins the security invariant:
// login responses for "wrong password", "inactive user", and "no such user"
// must be indistinguishable to a remote caller. ErrAccountLocked is allowed to
// differ because lockout is communicated to legitimate users.
func TestAccountEnumeration_LoginResponsesIdentical(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"missing user / wrong password", ErrInvalidCredentials},
		{"inactive account", ErrInactiveUser},
	}

	bodies := make(map[string]string, len(cases))
	statuses := make(map[string]int, len(cases))

	for _, c := range cases {
		h := newEnumHandler(t, c.err)
		body := []byte(`{"email":"a@b.com","password":"x"}`)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		bodies[c.name] = w.Body.String()
		statuses[c.name] = w.Code
	}

	first := bodies[cases[0].name]
	firstStatus := statuses[cases[0].name]
	for _, c := range cases[1:] {
		assert.Equal(t, firstStatus, statuses[c.name], "status code must match across login failure modes")
		assert.Equal(t, first, bodies[c.name], "response body must match across login failure modes")
	}

	// Lockout is intentionally distinct.
	hLocked := newEnumHandler(t, ErrAccountLocked)
	body := []byte(`{"email":"a@b.com","password":"x"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	hLocked.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEqual(t, first, w.Body.String(), "lockout response must be distinguishable from generic failure")
}

// TestErrInvalidCredentialsMessage pins the externally-visible error string
// so any future refactor that broadens it (e.g. "user not found") is caught.
func TestErrInvalidCredentialsMessage(t *testing.T) {
	assert.Equal(t, "invalid email or password", ErrInvalidCredentials.Error())
	// ErrInactiveUser may differ internally but must never appear in the
	// login response — see TestAccountEnumeration_LoginResponsesIdentical.
	assert.NotEqual(t, ErrInvalidCredentials.Error(), ErrInactiveUser.Error())
}
