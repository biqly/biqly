package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthnUser struct {
	User        *User
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
	repo     *UserRepository
}

func NewWebAuthnService(cfg *Config, repo *UserRepository) (*WebAuthnService, error) {
	w, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.WebAuthnRPID,
		RPDisplayName: cfg.WebAuthnRPName,
		RPOrigins:     cfg.WebAuthnOrigins,
	})
	if err != nil {
		return nil, err
	}
	return &WebAuthnService{
		webAuthn: w,
		repo:     repo,
	}, nil
}

func (s *WebAuthnService) BeginRegistration(ctx context.Context, user *User) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
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

	challengeBytes, err := base64.RawURLEncoding.DecodeString(session.Challenge)
	if err != nil {
		challengeBytes, err = base64.URLEncoding.DecodeString(session.Challenge)
		if err != nil {
			return nil, nil, err
		}
	}

	expiresAt := session.Expires
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(5 * time.Minute)
	}
	err = s.repo.SaveWebAuthnChallenge(ctx, challengeBytes, &user.ID, expiresAt)
	if err != nil {
		return nil, nil, err
	}

	return creation, session, nil
}

func (s *WebAuthnService) FinishRegistration(ctx context.Context, user *User, session *webauthn.SessionData, request *http.Request, name string) (*webauthn.Credential, error) {
	challengeBytes, err := base64.RawURLEncoding.DecodeString(session.Challenge)
	if err != nil {
		challengeBytes, err = base64.URLEncoding.DecodeString(session.Challenge)
		if err != nil {
			return nil, err
		}
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

	challengeBytes, err := base64.RawURLEncoding.DecodeString(session.Challenge)
	if err != nil {
		challengeBytes, err = base64.URLEncoding.DecodeString(session.Challenge)
		if err != nil {
			return nil, nil, err
		}
	}

	expiresAt := session.Expires
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(5 * time.Minute)
	}
	err = s.repo.SaveWebAuthnChallenge(ctx, challengeBytes, &user.ID, expiresAt)
	if err != nil {
		return nil, nil, err
	}

	return assertion, session, nil
}

func (s *WebAuthnService) FinishLogin(ctx context.Context, session *webauthn.SessionData, request *http.Request) (*User, error) {
	challengeBytes, err := base64.RawURLEncoding.DecodeString(session.Challenge)
	if err != nil {
		challengeBytes, err = base64.URLEncoding.DecodeString(session.Challenge)
		if err != nil {
			return nil, err
		}
	}

	uid, err := s.repo.GetWebAuthnChallenge(ctx, challengeBytes)
	if err != nil {
		return nil, err
	}
	if uid == nil {
		return nil, errors.New("invalid or expired login challenge")
	}

	user, err := s.repo.GetUserByID(ctx, *uid)
	if err != nil {
		return nil, err
	}

	creds, err := s.repo.GetPasskeysByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

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

func (s *WebAuthnService) GetUserPasskeys(ctx context.Context, userID string) ([]PasskeyInfo, error) {
	return s.repo.GetUserPasskeys(ctx, userID)
}

func (s *WebAuthnService) DeletePasskey(ctx context.Context, userID string, passkeyID string) error {
	return s.repo.DeletePasskey(ctx, userID, passkeyID)
}
