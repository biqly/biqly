package mfa_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"github.com/bytedance/sonic"
	"net/http"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/mfa"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// virtualAuthenticator is a software WebAuthn authenticator used to drive the
// full registration + login ceremony against WebAuthnService without a browser.
type virtualAuthenticator struct {
	rpID   string
	origin string
	key    *ecdsa.PrivateKey
	credID []byte
}

const (
	flagUP = 0x01 // user present
	flagUV = 0x04 // user verified
	flagAT = 0x40 // attested credential data included
)

func newVirtualAuthenticator(t *testing.T, rpID, origin string) *virtualAuthenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	credID := make([]byte, 16)
	_, err = rand.Read(credID)
	require.NoError(t, err)
	return &virtualAuthenticator{rpID: rpID, origin: origin, key: key, credID: credID}
}

// coseKey builds the COSE_Key (ES256 / P-256) for the authenticator public key.
// COSE map uses negative and small-positive integer labels; the CTAP2 canonical
// encoder produces deterministically-ordered keys that webauthncose parses.
func (a *virtualAuthenticator) coseKey(t *testing.T) []byte {
	t.Helper()
	x := a.key.PublicKey.X.Bytes() //nolint:staticcheck // coordinate check
	y := a.key.PublicKey.Y.Bytes() //nolint:staticcheck // coordinate check
	xb := make([]byte, 32)
	yb := make([]byte, 32)
	copy(xb[32-len(x):], x)
	copy(yb[32-len(y):], y)

	coseMap := map[int]any{
		1:  2,  // kty: EC2
		3:  -7, // alg: ES256
		-1: 1,  // crv: P-256
		-2: xb, // x
		-3: yb, // y
	}
	enc, err := cbor.CTAP2EncOptions().EncMode()
	require.NoError(t, err)
	out, err := enc.Marshal(coseMap)
	require.NoError(t, err)
	return out
}

// authData assembles authenticatorData. When includeAttested is true the
// attested-credential-data block (AAGUID + credID + COSE key) is appended.
func (a *virtualAuthenticator) authData(t *testing.T, flags byte, signCount uint32, includeAttested bool) []byte {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(a.rpID))

	buf := bytes.NewBuffer(nil)
	buf.Write(rpIDHash[:])
	buf.WriteByte(flags)
	var sc [4]byte
	binary.BigEndian.PutUint32(sc[:], signCount)
	buf.Write(sc[:])

	if includeAttested {
		buf.Write(make([]byte, 16)) // AAGUID (zero)
		var idLen [2]byte
		binary.BigEndian.PutUint16(idLen[:], uint16(len(a.credID))) //nolint:gosec // credID len is 16
		buf.Write(idLen[:])
		buf.Write(a.credID)
		buf.Write(a.coseKey(t))
	}
	return buf.Bytes()
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// registrationRequest builds a CredentialCreationResponse JSON body (attestation
// fmt "none") and wraps it in an *http.Request for FinishRegistration.
func (a *virtualAuthenticator) registrationRequest(t *testing.T, challenge string) *http.Request {
	t.Helper()
	clientDataJSON := marshalJSON(t, map[string]any{
		"type":      "webauthn.create",
		"challenge": challenge,
		"origin":    a.origin,
	})

	authData := a.authData(t, flagUP|flagUV|flagAT, 0, true)
	enc, err := cbor.CTAP2EncOptions().EncMode()
	require.NoError(t, err)
	attObjBytes, err := enc.Marshal(map[string]any{
		"fmt":      "none",
		"attStmt":  map[string]any{},
		"authData": authData,
	})
	require.NoError(t, err)

	return newJSONRequest(t, map[string]any{
		"id":    b64url(a.credID),
		"rawId": b64url(a.credID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64url(clientDataJSON),
			"attestationObject": b64url(attObjBytes),
		},
		"clientExtensionResults": map[string]any{},
	})
}

// assertionRequest builds a CredentialAssertionResponse JSON body signed by
// signKey (callers inject a wrong key to exercise the failure path).
func (a *virtualAuthenticator) assertionRequest(t *testing.T, challenge string, userID []byte, signCount uint32, signKey *ecdsa.PrivateKey) *http.Request {
	t.Helper()
	clientDataJSON := marshalJSON(t, map[string]any{
		"type":      "webauthn.get",
		"challenge": challenge,
		"origin":    a.origin,
	})

	authData := a.authData(t, flagUP|flagUV, signCount, false)
	clientDataHash := sha256.Sum256(clientDataJSON)
	signed := append(append([]byte{}, authData...), clientDataHash[:]...)
	digest := sha256.Sum256(signed)
	sig, err := ecdsa.SignASN1(rand.Reader, signKey, digest[:])
	require.NoError(t, err)

	return newJSONRequest(t, map[string]any{
		"id":    b64url(a.credID),
		"rawId": b64url(a.credID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64url(clientDataJSON),
			"authenticatorData": b64url(authData),
			"signature":         b64url(sig),
			"userHandle":        b64url(userID),
		},
		"clientExtensionResults": map[string]any{},
	})
}

func marshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := sonic.Marshal(v)
	require.NoError(t, err)
	return raw
}

func newJSONRequest(t *testing.T, body map[string]any) *http.Request {
	t.Helper()
	raw := marshalJSON(t, body)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newWebAuthnTestService(t *testing.T) (*mfa.WebAuthnService, *auth.Config, *sql.DB, context.Context) {
	t.Helper()
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()

	cfg := &auth.Config{
		WebAuthnRPID:         "localhost",
		WebAuthnRPName:       "Biqly",
		WebAuthnOrigins:      []string{"https://localhost"},
		WebAuthnChallengeTTL: 60 * time.Second,
	}
	repo := auth.NewUserRepository(db, nil)
	svc, err := mfa.NewWebAuthnService(cfg, repo)
	require.NoError(t, err)

	testutil.ExecAuthSQL(ctx, t, db, "DELETE FROM passkeys", "DELETE FROM webauthn_challenges")
	t.Cleanup(func() {
		testutil.ExecAuthSQL(ctx, t, db,
			"DELETE FROM passkeys",
			"DELETE FROM webauthn_challenges",
		)
	})
	return svc, cfg, db, ctx
}

func insertWebAuthnUser(ctx context.Context, t *testing.T, db *sql.DB, email string) *auth.User {
	t.Helper()
	var id string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'WebAuthn User', 'hash', TRUE) RETURNING id`, email,
	).Scan(&id))
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	})
	return &auth.User{ID: id, Email: email}
}

func TestWebAuthnFullRegistrationAndLoginFlow(t *testing.T) {
	svc, cfg, db, ctx := newWebAuthnTestService(t)
	user := insertWebAuthnUser(ctx, t, db, "webauthn-flow@example.com")

	va := newVirtualAuthenticator(t, cfg.WebAuthnRPID, cfg.WebAuthnOrigins[0])

	// --- Registration ---
	creation, regSession, err := svc.BeginRegistration(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, creation)
	require.NotEmpty(t, regSession.Challenge)

	regReq := va.registrationRequest(t, regSession.Challenge)
	cred, err := svc.FinishRegistration(ctx, user, regSession, regReq, "Test Passkey")
	require.NoError(t, err)
	require.NotNil(t, cred)
	require.Equal(t, va.credID, cred.ID)

	passkeys, err := svc.GetUserPasskeys(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, passkeys, 1, "registration must persist exactly one passkey")

	// --- Login (success) ---
	_, loginSession, err := svc.BeginLogin(ctx, user.Email)
	require.NoError(t, err)
	require.NotEmpty(t, loginSession.Challenge)

	loginReq := va.assertionRequest(t, loginSession.Challenge, []byte(user.ID), 1, va.key)
	gotUser, err := svc.FinishLogin(ctx, loginSession, loginReq)
	require.NoError(t, err)
	require.NotNil(t, gotUser)
	assert.Equal(t, user.ID, gotUser.ID)

	// --- Login with WRONG signature must fail ---
	wrongKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	_, badSession, err := svc.BeginLogin(ctx, user.Email)
	require.NoError(t, err)
	badReq := va.assertionRequest(t, badSession.Challenge, []byte(user.ID), 2, wrongKey)
	_, err = svc.FinishLogin(ctx, badSession, badReq)
	require.Error(t, err, "login with a signature from a different key must fail")

	// --- Challenge single-use: replaying the same session/body must fail ---
	_, reuseSession, err := svc.BeginLogin(ctx, user.Email)
	require.NoError(t, err)
	firstReq := va.assertionRequest(t, reuseSession.Challenge, []byte(user.ID), 3, va.key)
	_, err = svc.FinishLogin(ctx, reuseSession, firstReq)
	require.NoError(t, err)

	secondReq := va.assertionRequest(t, reuseSession.Challenge, []byte(user.ID), 3, va.key)
	_, err = svc.FinishLogin(ctx, reuseSession, secondReq)
	require.Error(t, err, "reusing a consumed challenge must fail (single-use enforcement)")
}
