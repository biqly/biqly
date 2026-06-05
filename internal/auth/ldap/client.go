// Package ldap implements directory sign-in via the search+bind pattern:
// a service account searches for the user's DN, then we re-bind as that DN with
// the user-supplied password to verify credentials. It is deliberately
// decoupled from internal/auth (no import cycle) — callers pass a plain
// Settings value mapped from their stored configuration.
package ldap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
)

// Security modes for the LDAP connection.
const (
	SecurityNone     = "none"
	SecurityStartTLS = "starttls"
	SecurityLDAPS    = "ldaps"
)

// dialTimeout bounds connection + the overall bind/search so a slow or
// unreachable directory cannot hang a login request.
const dialTimeout = 8 * time.Second

// ErrInvalidCredentials is returned when the directory is reachable but the
// username/password did not authenticate (no entry, or bind rejected). Callers
// MUST map this to a generic auth failure — never surface it verbatim, to avoid
// account enumeration.
var ErrInvalidCredentials = errors.New("ldap: invalid credentials")

// Settings is the connection + search configuration for one directory.
type Settings struct {
	Host            string
	Port            int
	Security        string // none | starttls | ldaps
	SkipTLSVerify   bool
	BindDN          string
	BindPassword    string
	BaseDN          string
	UserFilter      string // contains a single %s placeholder
	EmailAttr       string
	DisplayNameAttr string
}

// Result carries the directory attributes resolved for an authenticated user.
type Result struct {
	DN          string
	Email       string
	DisplayName string
}

// Authenticator verifies credentials and tests connectivity against a directory.
type Authenticator interface {
	Authenticate(ctx context.Context, s Settings, username, password string) (*Result, error)
	TestConnection(ctx context.Context, s Settings) error
}

// Client is the default go-ldap-backed Authenticator.
type Client struct{}

// New returns a ready-to-use LDAP client.
func New() *Client { return &Client{} }

func (s Settings) addr() string {
	port := s.Port
	if port == 0 {
		if s.Security == SecurityLDAPS {
			port = 636
		} else {
			port = 389
		}
	}
	return net.JoinHostPort(strings.TrimSpace(s.Host), strconv.Itoa(port))
}

func (s Settings) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName:         strings.TrimSpace(s.Host),
		InsecureSkipVerify: s.SkipTLSVerify, //nolint:gosec // operator opt-in for self-signed dirs
		MinVersion:         tls.VersionTLS12,
	}
}

// dial opens a connection honouring the configured security mode and applies
// the overall deadline derived from ctx (capped by dialTimeout).
func (s Settings) dial(ctx context.Context) (*ldapv3.Conn, error) {
	if strings.TrimSpace(s.Host) == "" {
		return nil, errors.New("ldap: host is not configured")
	}
	d := &net.Dialer{Timeout: dialTimeout}
	var (
		conn *ldapv3.Conn
		err  error
	)
	switch s.Security {
	case SecurityLDAPS:
		conn, err = ldapv3.DialURL("ldaps://"+s.addr(),
			ldapv3.DialWithDialer(d), ldapv3.DialWithTLSConfig(s.tlsConfig()))
	default:
		conn, err = ldapv3.DialURL("ldap://"+s.addr(), ldapv3.DialWithDialer(d))
	}
	if err != nil {
		return nil, fmt.Errorf("ldap: connect %s: %w", s.addr(), err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetTimeout(time.Until(deadline))
	} else {
		conn.SetTimeout(dialTimeout)
	}
	if s.Security == SecurityStartTLS {
		if err := conn.StartTLS(s.tlsConfig()); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("ldap: starttls: %w", err)
		}
	}
	return conn, nil
}

// bindService binds with the configured service account (or anonymously when no
// bind DN is set, for directories that permit anonymous search).
func (s Settings) bindService(conn *ldapv3.Conn) error {
	if strings.TrimSpace(s.BindDN) == "" {
		if err := conn.UnauthenticatedBind(""); err != nil {
			return fmt.Errorf("ldap: anonymous bind: %w", err)
		}
		return nil
	}
	if err := conn.Bind(s.BindDN, s.BindPassword); err != nil {
		return fmt.Errorf("ldap: service bind: %w", err)
	}
	return nil
}

func (s Settings) filterFor(username string) string {
	f := strings.TrimSpace(s.UserFilter)
	if f == "" {
		f = "(uid=%s)"
	}
	// RFC 4515 escaping of the user-controlled value prevents LDAP filter
	// injection (e.g. a username of "*" or ")(uid=*").
	return fmt.Sprintf(f, ldapv3.EscapeFilter(username))
}

// Authenticate runs the search+bind flow and returns the directory attributes
// for the user. A reachable directory that rejects the credentials yields
// ErrInvalidCredentials; connectivity/config problems yield other errors.
func (*Client) Authenticate(ctx context.Context, s Settings, username, password string) (*Result, error) {
	// Reject empty passwords up front: many servers treat an empty password as
	// an unauthenticated (anonymous) bind that "succeeds" — which would be an
	// auth bypass.
	if strings.TrimSpace(username) == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	conn, err := s.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	if err := s.bindService(conn); err != nil {
		return nil, err
	}

	emailAttr := attrOr(s.EmailAttr, "mail")
	nameAttr := attrOr(s.DisplayNameAttr, "cn")
	res, err := conn.Search(&ldapv3.SearchRequest{
		BaseDN:       strings.TrimSpace(s.BaseDN),
		Scope:        ldapv3.ScopeWholeSubtree,
		DerefAliases: ldapv3.NeverDerefAliases,
		SizeLimit:    2,
		TimeLimit:    int(dialTimeout / time.Second),
		Filter:       s.filterFor(username),
		Attributes:   []string{emailAttr, nameAttr},
	})
	if err != nil {
		return nil, fmt.Errorf("ldap: search: %w", err)
	}
	if len(res.Entries) != 1 {
		// 0 = no such user; >1 = ambiguous filter. Both are auth failures.
		return nil, ErrInvalidCredentials
	}
	entry := res.Entries[0]

	// Re-bind as the user to verify the password. Use a fresh connection so the
	// service-account binding is not reused.
	userConn, err := s.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = userConn.Close() }()
	if err := userConn.Bind(entry.DN, password); err != nil {
		if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultInvalidCredentials) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("ldap: user bind: %w", err)
	}

	return &Result{
		DN:          entry.DN,
		Email:       strings.TrimSpace(entry.GetAttributeValue(emailAttr)),
		DisplayName: strings.TrimSpace(entry.GetAttributeValue(nameAttr)),
	}, nil
}

// TestConnection verifies the host is reachable, TLS negotiates, and the
// service account (or anonymous) bind succeeds. It does not require a user.
func (*Client) TestConnection(ctx context.Context, s Settings) error {
	conn, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	return s.bindService(conn)
}

func attrOr(v, def string) string {
	if t := strings.TrimSpace(v); t != "" {
		return t
	}
	return def
}
