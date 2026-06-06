package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/biqly/biqly/internal/auth/ldap"
)

// ErrLDAPNotConfigured is returned by the admin/config helpers when the LDAP
// subsystem has not been wired into the service.
var ErrLDAPNotConfigured = errors.New("ldap is not configured")

// SetLDAP wires the directory config repository and authenticator so the Login
// flow can fall back to LDAP and the admin API can manage the configuration.
func (s *Service) SetLDAP(repo *LDAPConfigRepository, authenticator ldap.Authenticator) {
	s.ldapConfig = repo
	s.ldapAuth = authenticator
}

func (s *Service) ldapReady() bool {
	return s.ldapConfig != nil && s.ldapAuth != nil
}

// GetLDAPConfig returns the stored configuration (bind password decrypted for
// service use; handlers must not serialize it back to clients).
func (s *Service) GetLDAPConfig(ctx context.Context) (LDAPConfig, error) {
	if s.ldapConfig == nil {
		return LDAPConfig{}, ErrLDAPNotConfigured
	}
	return s.ldapConfig.Get(ctx)
}

// UpdateLDAPConfig persists the configuration.
func (s *Service) UpdateLDAPConfig(ctx context.Context, in LDAPConfig, updatedBy string) (LDAPConfig, error) {
	if s.ldapConfig == nil {
		return LDAPConfig{}, ErrLDAPNotConfigured
	}
	return s.ldapConfig.Update(ctx, in, updatedBy)
}

// TestLDAPConnection probes the directory with the supplied configuration. When
// the bind password is blank the stored one is used (so the admin UI can test
// without re-entering the secret).
func (s *Service) TestLDAPConnection(ctx context.Context, in LDAPConfig) error {
	if !s.ldapReady() {
		return ErrLDAPNotConfigured
	}
	if strings.TrimSpace(in.BindPassword) == "" {
		if cur, err := s.ldapConfig.Get(ctx); err == nil {
			in.BindPassword = cur.BindPassword
		}
	}
	return s.ldapAuth.TestConnection(ctx, settingsFromLDAPConfig(in))
}

// LDAPEnabled reports whether directory sign-in is turned on.
func (s *Service) LDAPEnabled(ctx context.Context) bool {
	if !s.ldapReady() {
		return false
	}
	cfg, err := s.ldapConfig.Get(ctx)
	return err == nil && cfg.Enabled
}

func settingsFromLDAPConfig(c LDAPConfig) ldap.Settings {
	return ldap.Settings{
		Host:            c.Host,
		Port:            c.Port,
		Security:        c.Security,
		SkipTLSVerify:   c.SkipTLSVerify,
		BindDN:          c.BindDN,
		BindPassword:    c.BindPassword,
		BaseDN:          c.BaseDN,
		UserFilter:      c.UserFilter,
		EmailAttr:       c.EmailAttr,
		DisplayNameAttr: c.DisplayNameAttr,
	}
}

// tryLDAP attempts directory authentication as a fallback after local password
// auth has failed. It returns:
//   - (user, nil)  when the directory authenticated the user (resolved/JIT-created);
//   - (nil, nil)   when LDAP is disabled, did not authenticate, or the user is
//     valid in the directory but not provisioned locally (auto-create off) —
//     the caller then returns a generic invalid-credentials error;
//   - (nil, err)   on a connectivity/configuration error.
func (s *Service) tryLDAP(ctx context.Context, username, password string) (*User, error) {
	if !s.ldapReady() {
		return nil, nil //nolint:nilnil // LDAP disabled; caller treats as not authenticated
	}
	cfg, err := s.ldapConfig.Get(ctx)
	if err != nil || !cfg.Enabled {
		return nil, err
	}

	res, err := s.ldapAuth.Authenticate(ctx, settingsFromLDAPConfig(cfg), username, password)
	if errors.Is(err, ldap.ErrInvalidCredentials) {
		return nil, nil //nolint:nilnil // invalid credentials are not an infrastructure error
	}
	if err != nil {
		return nil, err
	}

	email := strings.TrimSpace(res.Email)
	if email == "" {
		email = username
	}
	normEmail, nerr := NormalizeEmail(email)
	if nerr != nil {
		normEmail = strings.TrimSpace(strings.ToLower(email))
	}

	user, gerr := s.userRepo.GetUserByEmail(ctx, normEmail)
	if errors.Is(gerr, ErrUserNotFound) {
		if !cfg.AutoCreateUsers {
			return nil, nil //nolint:nilnil // LDAP user missing and auto-create disabled
		}
		displayName, derr := SanitizeDisplayName(res.DisplayName)
		if derr != nil || strings.TrimSpace(displayName) == "" {
			displayName = normEmail
		}
		return s.userRepo.CreateDirectoryUser(ctx, normEmail, displayName)
	}
	if gerr != nil {
		return nil, gerr
	}
	return user, nil
}
