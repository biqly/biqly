-- LDAP / directory sign-in configuration (singleton row, like platform_settings).
-- Connection parameters and attribute mappings are managed at runtime via the
-- admin API; the bind password is stored AES-encrypted (BI_AUTH_ENCRYPTION_KEY).
CREATE TABLE ldap_config (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    -- When true, a successful LDAP bind for an unknown email auto-creates a
    -- local (passwordless) user. When false, the user must already exist.
    auto_create_users BOOLEAN NOT NULL DEFAULT TRUE,
    host TEXT NOT NULL DEFAULT '',
    port INT NOT NULL DEFAULT 389,
    -- none | starttls | ldaps
    security TEXT NOT NULL DEFAULT 'starttls',
    skip_tls_verify BOOLEAN NOT NULL DEFAULT FALSE,
    -- Service account used to search for the user's DN (search+bind).
    bind_dn TEXT NOT NULL DEFAULT '',
    bind_password_encrypted TEXT NOT NULL DEFAULT '',
    base_dn TEXT NOT NULL DEFAULT '',
    -- %s is replaced (after RFC 4515 escaping) with the submitted username.
    user_filter TEXT NOT NULL DEFAULT '(uid=%s)',
    email_attr TEXT NOT NULL DEFAULT 'mail',
    display_name_attr TEXT NOT NULL DEFAULT 'cn',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

INSERT INTO ldap_config (id) VALUES (1);
