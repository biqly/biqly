package auth

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

func ValidateEmail(email string) error {
	_, err := NormalizeEmail(email)
	return err
}

func NormalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", errors.New("email is required")
	}
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
	return strings.ToLower(email), nil
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
