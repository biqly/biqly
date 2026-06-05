package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/security"
)

// LDAPConfig is the singleton directory sign-in configuration. BindPassword is
// the decrypted value held only in memory for service use — handlers MUST NOT
// serialize it back to clients (it is write-only over the API).
type LDAPConfig struct {
	Enabled         bool      `json:"enabled"`
	AutoCreateUsers bool      `json:"auto_create_users"`
	Host            string    `json:"host"`
	Port            int       `json:"port"`
	Security        string    `json:"security"`
	SkipTLSVerify   bool      `json:"skip_tls_verify"`
	BindDN          string    `json:"bind_dn"`
	BindPassword    string    `json:"-"`
	HasBindPassword bool      `json:"has_bind_password"`
	BaseDN          string    `json:"base_dn"`
	UserFilter      string    `json:"user_filter"`
	EmailAttr       string    `json:"email_attr"`
	DisplayNameAttr string    `json:"display_name_attr"`
	UpdatedAt       time.Time `json:"updated_at"`
	UpdatedBy       *string   `json:"updated_by,omitempty"`
}

// LDAPConfigRepository persists the singleton ldap_config row.
type LDAPConfigRepository struct {
	db  *sql.DB
	enc *security.Encryption
}

func NewLDAPConfigRepository(db *sql.DB, enc *security.Encryption) *LDAPConfigRepository {
	return &LDAPConfigRepository{db: db, enc: enc}
}

// Get returns the configuration with the bind password decrypted for service
// use (login / test connection).
func (r *LDAPConfigRepository) Get(ctx context.Context) (LDAPConfig, error) {
	var (
		c          LDAPConfig
		encPw      string
		updatedBy  sql.NullString
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT enabled, auto_create_users, host, port, security, skip_tls_verify,
		       bind_dn, bind_password_encrypted, base_dn, user_filter,
		       email_attr, display_name_attr, updated_at, updated_by
		FROM ldap_config WHERE id = 1
	`).Scan(
		&c.Enabled, &c.AutoCreateUsers, &c.Host, &c.Port, &c.Security, &c.SkipTLSVerify,
		&c.BindDN, &encPw, &c.BaseDN, &c.UserFilter,
		&c.EmailAttr, &c.DisplayNameAttr, &c.UpdatedAt, &updatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LDAPConfig{}, nil
		}
		return LDAPConfig{}, fmt.Errorf("get ldap config: %w", err)
	}
	encPw = strings.TrimSpace(encPw)
	c.HasBindPassword = encPw != ""
	if c.HasBindPassword {
		c.BindPassword = r.decrypt(encPw)
	}
	if updatedBy.Valid {
		c.UpdatedBy = new(updatedBy.String)
	}
	return c, nil
}

// Update writes the configuration. When in.BindPassword is empty the stored
// password is preserved (so the admin UI never needs to round-trip the secret).
func (r *LDAPConfigRepository) Update(ctx context.Context, in LDAPConfig, updatedBy string) (LDAPConfig, error) {
	encPw := ""
	if strings.TrimSpace(in.BindPassword) != "" {
		enc, err := r.encrypt(in.BindPassword)
		if err != nil {
			return LDAPConfig{}, err
		}
		encPw = enc
	} else {
		// Preserve the existing encrypted password.
		_ = r.db.QueryRowContext(ctx, `SELECT bind_password_encrypted FROM ldap_config WHERE id = 1`).Scan(&encPw)
	}

	if _, err := r.db.ExecContext(ctx, `
		UPDATE ldap_config SET
			enabled = $1, auto_create_users = $2, host = $3, port = $4,
			security = $5, skip_tls_verify = $6, bind_dn = $7,
			bind_password_encrypted = $8, base_dn = $9, user_filter = $10,
			email_attr = $11, display_name_attr = $12,
			updated_at = NOW(), updated_by = NULLIF($13::text, '')::uuid
		WHERE id = 1
	`,
		in.Enabled, in.AutoCreateUsers, strings.TrimSpace(in.Host), in.Port,
		in.Security, in.SkipTLSVerify, strings.TrimSpace(in.BindDN),
		encPw, strings.TrimSpace(in.BaseDN), strings.TrimSpace(in.UserFilter),
		strings.TrimSpace(in.EmailAttr), strings.TrimSpace(in.DisplayNameAttr), updatedBy,
	); err != nil {
		return LDAPConfig{}, fmt.Errorf("update ldap config: %w", err)
	}
	return r.Get(ctx)
}

func (r *LDAPConfigRepository) encrypt(plain string) (string, error) {
	if r.enc == nil {
		return plain, nil
	}
	return r.enc.Encrypt(plain)
}

func (r *LDAPConfigRepository) decrypt(enc string) string {
	if enc == "" || r.enc == nil {
		return enc
	}
	plain, err := r.enc.Decrypt(enc)
	if err != nil {
		if r.enc.IsEncrypted(enc) {
			return ""
		}
		return enc
	}
	return plain
}
