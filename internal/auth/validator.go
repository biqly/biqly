package auth

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func ValidateEmail(email string) error {
	_, err := NormalizeEmail(email)
	return err
}

// NormalizeEmail trims, NFKC-normalizes, lowercases, and canonicalizes
// well-known provider variants so the same logical address always maps to
// the same row. Gmail and Googlemail addresses have dots and +tag suffixes
// stripped from the local part and the domain unified to gmail.com; this
// prevents trivial duplicate-account creation via dot/tag tricks.
func NormalizeEmail(email string) (string, error) {
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

func ValidateDisplayName(displayName string) error {
	_, err := SanitizeDisplayName(displayName)
	return err
}

func SanitizeDisplayName(displayName string) (string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", nil
	}
	if utf8.RuneCountInString(displayName) > 120 {
		return "", errors.New("display name is too long")
	}
	if containsUnsupportedText(displayName) {
		return "", errors.New("display name contains unsupported characters")
	}
	return displayName, nil
}

func containsUnsupportedText(value string) bool {
	for _, r := range value {
		if r == '<' || r == '>' || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// ValidatePassword applies the default password policy. Callers with access
// to a configured policy (e.g. AuthService) should prefer that policy's
// Validate method so deployment-specific rules and identity-containment
// checks are enforced.
func ValidatePassword(password string) error {
	return DefaultPasswordPolicy().Validate(password)
}
