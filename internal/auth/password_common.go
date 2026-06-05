package auth

import (
	_ "embed"
	"strings"
	"sync"
	"unicode"
)

//go:embed password_common.txt
var commonPasswordList string

var (
	commonOnce sync.Once
	commonSet  map[string]struct{}
)

func loadCommonPasswords() {
	commonSet = make(map[string]struct{}, 256)
	for _, line := range strings.Split(commonPasswordList, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		commonSet[line] = struct{}{}
	}
}

// IsCommonPassword reports whether the password matches a well-known weak
// password. It checks the literal lower-cased value as well as a "core" form
// (alphabetic prefix only) so trivial composes like "Password123!" or
// "qwerty2024" are still caught.
func IsCommonPassword(password string) bool {
	commonOnce.Do(loadCommonPasswords)
	if password == "" {
		return false
	}
	candidates := []string{
		strings.ToLower(password),
		stripTrailingNonLetters(password),
		stripLeetSpeak(password),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, ok := commonSet[c]; ok {
			return true
		}
	}
	return false
}

func stripTrailingNonLetters(password string) string {
	runes := []rune(strings.ToLower(password))
	end := len(runes)
	for end > 0 && !unicode.IsLetter(runes[end-1]) {
		end--
	}
	if end == 0 {
		return ""
	}
	return string(runes[:end])
}

// stripLeetSpeak undoes the most common 1337 substitutions so "p@ssw0rd" is
// recognized as "password". Conservative — only the highest-frequency
// substitutions to avoid false positives on genuinely strong passwords.
func stripLeetSpeak(password string) string {
	runes := make([]rune, 0, len(password))
	for _, r := range strings.ToLower(password) {
		switch r {
		case '0':
			runes = append(runes, 'o')
		case '1':
			runes = append(runes, 'i')
		case '3':
			runes = append(runes, 'e')
		case '4':
			runes = append(runes, 'a')
		case '5':
			runes = append(runes, 's')
		case '7':
			runes = append(runes, 't')
		case '@':
			runes = append(runes, 'a')
		case '$':
			runes = append(runes, 's')
		case '!':
			runes = append(runes, 'i')
		default:
			runes = append(runes, r)
		}
	}
	return string(runes)
}
