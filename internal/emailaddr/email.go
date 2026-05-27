// Package emailaddr provides canonical email normalization and log-safe
// masking shared by the auth and mail packages. It is a dependency-free leaf
// package so both can import it without creating an import cycle.
package emailaddr

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize trims, NFKC-normalizes, lowercases, and canonicalizes well-known
// provider variants so the same logical address always maps to the same row.
// Gmail and Googlemail addresses have dots and +tag suffixes stripped from the
// local part and the domain unified to gmail.com; this prevents trivial
// duplicate-account creation via dot/tag tricks.
func Normalize(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", errors.New("email is required")
	}
	email = norm.NFKC.String(email)
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", errors.New("invalid email address format")
	}
	if addr.Name != "" || addr.Address != email {
		return "", errors.New("invalid email address format")
	}
	if containsUnsupportedText(email) {
		return "", errors.New("invalid email address format")
	}
	lowered := strings.ToLower(email)
	at := strings.LastIndex(lowered, "@")
	if at <= 0 || at == len(lowered)-1 {
		return "", errors.New("invalid email address format")
	}
	local, domain := lowered[:at], lowered[at+1:]
	if domain == "googlemail.com" {
		domain = "gmail.com"
	}
	if domain == "gmail.com" {
		if plus := strings.IndexByte(local, '+'); plus >= 0 {
			local = local[:plus]
		}
		local = strings.ReplaceAll(local, ".", "")
		if local == "" {
			return "", errors.New("invalid email address format")
		}
	}
	return local + "@" + domain, nil
}

// Mask returns a log-safe rendering of an email address that preserves the
// domain and the first/last local-part characters while hiding the rest.
func Mask(email string) string {
	if email == "" {
		return ""
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return "***"
	}
	local, domain := email[:at], email[at+1:]
	switch {
	case len(local) <= 1:
		return "*@" + domain
	case len(local) == 2:
		return local[:1] + "*@" + domain
	default:
		return local[:1] + strings.Repeat("*", len(local)-2) + local[len(local)-1:] + "@" + domain
	}
}

func containsUnsupportedText(value string) bool {
	for _, r := range value {
		if r == '<' || r == '>' || unicode.IsControl(r) {
			return true
		}
	}
	return false
}
