package security

import (
	"net/url"
	"regexp"
	"strings"
)

// ConnectionDSN returns a driver-ready DSN from stored metadata.
// When enc is non-nil and the stored value matches the encrypted heuristic, it is decrypted;
// otherwise stored is returned unchanged (plaintext when encryption is off or legacy rows).
func ConnectionDSN(enc *Encryption, stored string) (string, error) {
	if enc != nil && enc.IsEncrypted(stored) {
		return enc.Decrypt(stored)
	}
	return stored, nil
}

var dsnPasswordKVRegex = regexp.MustCompile(`(?i)(password\s*=\s*)([^\s;]+)`)

// RedactDSN returns a copy of dsn safe to log: any embedded password is
// replaced with "***". Supports URL-style DSNs (postgres://, mysql://, etc.)
// and key=value DSNs (libpq style).
func RedactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		if u.User != nil {
			user := u.User.Username()
			if user != "" {
				u.User = url.UserPassword(user, "***")
			} else {
				u.User = nil
			}
			return u.String()
		}
	}
	if strings.Contains(dsn, "password") || strings.Contains(dsn, "PASSWORD") {
		return dsnPasswordKVRegex.ReplaceAllString(dsn, "${1}***")
	}
	return dsn
}
