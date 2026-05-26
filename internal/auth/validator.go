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

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:',.<>/?~`", char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}
