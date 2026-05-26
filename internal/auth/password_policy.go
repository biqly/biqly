package auth

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PasswordPolicy captures the validation rules applied to new passwords.
// Defaults reproduce the historic behavior (8 char min, all four classes).
type PasswordPolicy struct {
	MinLength      int  `json:"min_length"`
	MaxLength      int  `json:"max_length"`
	RequireUpper   bool `json:"require_upper"`
	RequireLower   bool `json:"require_lower"`
	RequireDigit   bool `json:"require_digit"`
	RequireSpecial bool `json:"require_special"`
	// MinScore is a 0–4 scale (0 = no scoring, mirrors zxcvbn buckets).
	// 0=Disabled, 1=Weak, 2=Fair, 3=Good, 4=Strong.
	MinScore int `json:"min_score"`
}

func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:      8,
		MaxLength:      128, // bcrypt input cap
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: true,
		MinScore:       2, // Fair: bars trivial common-password choices
	}
}

const specialChars = "!@#$%^&*()_+-=[]{}|;:',.<>/?~`\" \t"

// Validate returns nil when the password satisfies all configured rules and the
// optional identityFields contributors (email, username, display name) are not
// embedded within it. identityFields entries are matched case-insensitively
// after stripping non-alphanumerics — both as substrings of the password and
// as prefixes — to catch trivial composes like "alice2024!".
func (p PasswordPolicy) Validate(password string, identityFields ...string) error {
	length := utf8.RuneCountInString(password)
	if length < p.MinLength {
		return fmt.Errorf("password must be at least %d characters long", p.MinLength)
	}
	if p.MaxLength > 0 && length > p.MaxLength {
		return fmt.Errorf("password must be at most %d characters long", p.MaxLength)
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case strings.ContainsRune(specialChars, char) || unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if p.RequireUpper && !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if p.RequireLower && !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if p.RequireDigit && !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if p.RequireSpecial && !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	normalized := normalizePassword(password)
	for _, field := range identityFields {
		token := normalizePassword(field)
		// Treat tokens shorter than 4 chars as too generic to enforce. Common
		// surnames or initials would otherwise produce unactionable errors.
		if len(token) < 4 {
			continue
		}
		if strings.Contains(normalized, token) {
			return errors.New("password must not contain your email or name")
		}
	}

	if IsCommonPassword(password) {
		return errors.New("password is too common; please choose a less guessable one")
	}

	if p.MinScore > 0 {
		score := PasswordScore(password)
		if score < p.MinScore {
			return errors.New("password is not strong enough; try a longer passphrase with more variety")
		}
	}
	return nil
}

// PasswordScore returns a 0..4 strength bucket using a lightweight heuristic
// (length-weighted class entropy minus penalties for repeats, sequences, and
// common-password matches). Not a full zxcvbn implementation, but good enough
// to reject obvious low-entropy choices on the backend.
func PasswordScore(password string) int {
	if password == "" {
		return 0
	}
	length := utf8.RuneCountInString(password)
	if length == 0 {
		return 0
	}

	classes := 0
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	for _, b := range []bool{hasUpper, hasLower, hasDigit, hasSpecial} {
		if b {
			classes++
		}
	}

	score := 0
	switch {
	case length >= 16:
		score += 2
	case length >= 12:
		score += 1
	}
	switch classes {
	case 4:
		score += 2
	case 3:
		score += 1
	}
	if length >= 20 {
		score++
	}

	if hasRunOrSequence(password) {
		score--
	}
	if IsCommonPassword(password) {
		score -= 2
	}

	if score < 0 {
		score = 0
	}
	if score > 4 {
		score = 4
	}
	return score
}

func hasRunOrSequence(password string) bool {
	runes := []rune(strings.ToLower(password))
	if len(runes) < 4 {
		return false
	}
	// Same character repeated 4+ times.
	run := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			run++
			if run >= 4 {
				return true
			}
		} else {
			run = 1
		}
	}
	// Monotone ASCII sequence (abcd, 1234) of length 4+.
	seq := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1]+1 {
			seq++
			if seq >= 4 {
				return true
			}
		} else {
			seq = 1
		}
	}
	return false
}

func normalizePassword(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// parseBoolEnv interprets common bool spellings; falls back to defaultValue on
// anything unrecognized so a typo doesn't silently flip a policy bit.
func parseBoolEnv(value string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}
