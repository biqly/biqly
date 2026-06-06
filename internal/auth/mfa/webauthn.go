package mfa

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthnUser struct {
	User        *auth.User
	Credentials []webauthn.Credential
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	return []byte(u.User.ID)
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.User.Email
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	if u.User.DisplayName != nil && *u.User.DisplayName != "" {
		return *u.User.DisplayName
	}
	return u.User.Email
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	if u.User.AvatarURL != nil {
		return *u.User.AvatarURL
	}
	return ""
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

type WebAuthnService struct {
	webAuthn *webauthn.WebAuthn
	repo     *auth.UserRepository
	ttl      time.Duration
}

func NewWebAuthnService(cfg *auth.Config, repo *auth.UserRepository) (*WebAuthnService, error) {
	timeout := webauthn.TimeoutConfig{
		Enforce:    true,
		Timeout:    cfg.WebAuthnChallengeTTL,
		TimeoutUVD: cfg.WebAuthnChallengeTTL,
	}
	w, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.WebAuthnRPID,
		RPDisplayName: cfg.WebAuthnRPName,
		RPOrigins:     cfg.WebAuthnOrigins,
		// User Verification = required forces a local gesture (PIN, biometric)
		// so a stolen credential cannot be replayed without the user. ResidentKey
		// preferred enables discoverable credentials where the authenticator
		// supports it; UV=required is the load-bearing constraint here.
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementPreferred,
			UserVerification: protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        timeout,
			Registration: timeout,
		},
	})
	if err != nil {
		return nil, err
	}
	return &WebAuthnService{
		webAuthn: w,
		repo:     repo,
		ttl:      cfg.WebAuthnChallengeTTL,
	}, nil
}

func decodeSessionChallenge(challenge string) ([]byte, error) {
	challengeBytes, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil {
		return base64.URLEncoding.DecodeString(challenge)
	}
	return challengeBytes, nil
}

func sessionChallengeExpiry(session *webauthn.SessionData, ttl time.Duration) time.Time {
	if !session.Expires.IsZero() {
		return session.Expires
	}
	return time.Now().Add(ttl)
}

func (s *WebAuthnService) persistSessionChallenge(ctx context.Context, session *webauthn.SessionData, userID *string) error {
	challengeBytes, err := decodeSessionChallenge(session.Challenge)
	if err != nil {
		return err
	}
	return s.repo.SaveWebAuthnChallenge(ctx, challengeBytes, userID, sessionChallengeExpiry(session, s.ttl))
}

func (s *WebAuthnService) BeginRegistration(ctx context.Context, user *auth.User) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	creds, err := s.repo.GetPasskeysByUserID(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}

	waUser := &WebAuthnUser{
		User:        user,
		Credentials: creds,
	}

	creation, session, err := s.webAuthn.BeginRegistration(waUser)
	if err != nil {
		return nil, nil, err
	}

	if err := s.persistSessionChallenge(ctx, session, &user.ID); err != nil {
		return nil, nil, err
	}

	return creation, session, nil
}

func (s *WebAuthnService) FinishRegistration(ctx context.Context, user *auth.User, session *webauthn.SessionData, request *http.Request, name string) (*webauthn.Credential, error) {
	challengeBytes, err := decodeSessionChallenge(session.Challenge)
	if err != nil {
		return nil, err
	}

	uid, err := s.repo.GetWebAuthnChallenge(ctx, challengeBytes)
	if err != nil {
		return nil, err
	}
	if uid == nil || *uid != user.ID {
		return nil, errors.New("invalid or expired registration challenge")
	}

	creds, err := s.repo.GetPasskeysByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	waUser := &WebAuthnUser{
		User:        user,
		Credentials: creds,
	}

	cred, err := s.webAuthn.FinishRegistration(waUser, *session, request)
	if err != nil {
		return nil, err
	}

	if name == "" {
		name = "Passkey " + time.Now().Format("2006-01-02 15:04")
	}

	err = s.repo.SavePasskey(ctx, user.ID, cred, name)
	if err != nil {
		return nil, err
	}

	return cred, nil
}

func (s *WebAuthnService) BeginLogin(ctx context.Context, emailOrUsername string) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	if emailOrUsername == "" {
		assertion, session, err := s.webAuthn.BeginDiscoverableLogin()
		if err != nil {
			return nil, nil, err
		}

		if err := s.persistSessionChallenge(ctx, session, nil); err != nil {
			return nil, nil, err
		}

		return assertion, session, nil
	}

	user, err := s.repo.GetUserByEmailOrUsername(ctx, emailOrUsername)
	if err != nil {
		return nil, nil, err
	}

	creds, err := s.repo.GetPasskeysByUserID(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(creds) == 0 {
		return nil, nil, errors.New("no registered passkeys found for user")
	}

	waUser := &WebAuthnUser{
		User:        user,
		Credentials: creds,
	}

	assertion, session, err := s.webAuthn.BeginLogin(waUser)
	if err != nil {
		return nil, nil, err
	}

	if err := s.persistSessionChallenge(ctx, session, &user.ID); err != nil {
		return nil, nil, err
	}

	return assertion, session, nil
}

func (s *WebAuthnService) FinishLogin(ctx context.Context, session *webauthn.SessionData, request *http.Request) (*auth.User, error) {
	challengeBytes, err := decodeSessionChallenge(session.Challenge)
	if err != nil {
		return nil, err
	}

	uid, err := s.repo.GetWebAuthnChallenge(ctx, challengeBytes)
	if err != nil {
		return nil, err
	}

	// Read and restore request body to inspect incoming credential ID and backup flags
	var bodyBytes []byte
	if request.Body != nil {
		var readErr error
		bodyBytes, readErr = io.ReadAll(request.Body)
		if readErr != nil {
			return nil, readErr
		}
		request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	if uid == nil { //nolint:nestif // discoverable assertion resolves user from credential id
		handler := func(rawID, _ []byte) (webauthn.User, error) {
			discoveredUID, err := s.repo.GetUserIDByCredentialID(ctx, rawID)
			if err != nil {
				return nil, errors.New("no user found for passkey")
			}

			user, err := s.repo.GetUserByID(ctx, discoveredUID)
			if err != nil {
				return nil, err
			}

			creds, err := s.repo.GetPasskeysByUserID(ctx, user.ID)
			if err != nil {
				return nil, err
			}
			applyAssertionBackupFlags(creds, bodyBytes)

			return &WebAuthnUser{
				User:        user,
				Credentials: creds,
			}, nil
		}

		waUser, cred, err := s.webAuthn.FinishPasskeyLogin(handler, *session, request)
		if err != nil {
			return nil, err
		}

		user, ok := waUser.(*WebAuthnUser)
		if !ok {
			return nil, errors.New("invalid passkey user")
		}

		err = s.repo.UpdatePasskeySignCount(ctx, cred.ID, cred.Authenticator.SignCount)
		if err != nil {
			return nil, err
		}

		err = s.repo.UpdateLastLogin(ctx, user.User.ID)
		if err != nil {
			return nil, err
		}

		return user.User, nil
	}

	user, err := s.repo.GetUserByID(ctx, *uid)
	if err != nil {
		return nil, err
	}

	creds, err := s.repo.GetPasskeysByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	applyAssertionBackupFlags(creds, bodyBytes)

	waUser := &WebAuthnUser{
		User:        user,
		Credentials: creds,
	}

	cred, err := s.webAuthn.FinishLogin(waUser, *session, request)
	if err != nil {
		return nil, err
	}

	err = s.repo.UpdatePasskeySignCount(ctx, cred.ID, cred.Authenticator.SignCount)
	if err != nil {
		return nil, err
	}

	err = s.repo.UpdateLastLogin(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func applyAssertionBackupFlags(creds []webauthn.Credential, bodyBytes []byte) {
	backupEligible, backupState, ok := AssertionBackupFlags(bodyBytes)
	if !ok {
		return
	}

	for i := range creds {
		creds[i].Flags.BackupEligible = backupEligible
		creds[i].Flags.BackupState = backupState
	}
}

func AssertionBackupFlags(bodyBytes []byte) (backupEligible bool, backupState bool, ok bool) {
	if len(bodyBytes) > 0 { //nolint:nestif // backup flags are optional in assertion payload
		var reqPayload struct {
			Response struct {
				AuthenticatorData string `json:"authenticatorData"`
			} `json:"response"`
		}
		if err := json.Unmarshal(bodyBytes, &reqPayload); err == nil && reqPayload.Response.AuthenticatorData != "" {
			authData, err := base64.RawURLEncoding.DecodeString(reqPayload.Response.AuthenticatorData)
			if err != nil {
				authData, err = base64.URLEncoding.DecodeString(reqPayload.Response.AuthenticatorData)
			}
			if err == nil && len(authData) > 32 {
				flags := protocol.AuthenticatorFlags(authData[32])
				return flags.HasBackupEligible(), flags.HasBackupState(), true
			}
		}
	}
	return false, false, false
}

func (s *WebAuthnService) GetUserPasskeys(ctx context.Context, userID string) ([]auth.PasskeyInfo, error) {
	return s.repo.GetUserPasskeys(ctx, userID)
}

func (s *WebAuthnService) DeletePasskey(ctx context.Context, userID string, passkeyID string) error {
	return s.repo.DeletePasskey(ctx, userID, passkeyID)
}

func (s *WebAuthnService) UpdatePasskeyName(ctx context.Context, userID string, passkeyID string, name string) error {
	return s.repo.UpdatePasskeyName(ctx, userID, passkeyID, name)
}
