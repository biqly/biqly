package security

import (
	"regexp"
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

var (
	dsnUserPassRegex = regexp.MustCompile(`(?i)([^:\s/]+):([^@\s:]+)@`)
	dsnPasswordRegex = regexp.MustCompile(`(?i)\b(pass|password)\b\s*=\s*([^\s;&]+)`)
)

// RedactDSN returns a copy of dsn safe to log: any embedded password is
// replaced with "***". Supports URL-style DSNs (postgres://, mysql://, etc.)
// and key=value DSNs (libpq style).
func RedactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	// Redact user:pass@ format
	dsn = dsnUserPassRegex.ReplaceAllString(dsn, "${1}:***@")
	// Redact pass= or password= format
	dsn = dsnPasswordRegex.ReplaceAllString(dsn, "${1}=***")
	return dsn
}
