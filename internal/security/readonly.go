package security

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	dangerousKeywords        []string
	dangerousKeywordPatterns []*regexp.Regexp
)

func init() {
	dangerous := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "ALTER",
		"TRUNCATE", "CREATE", "GRANT", "REVOKE", "MERGE",
		"CALL", "EXEC", "EXECUTE",
	}
	dangerousKeywords = dangerous
	dangerousKeywordPatterns = make([]*regexp.Regexp, 0, len(dangerous))
	for _, kw := range dangerous {
		dangerousKeywordPatterns = append(dangerousKeywordPatterns,
			regexp.MustCompile(`\b`+regexp.QuoteMeta(kw)+`\b`))
	}
}

// ReadOnlyChecker validates that SQL is safe to execute.
type ReadOnlyChecker struct{}

// NewReadOnlyChecker creates a new read-only SQL checker.
func NewReadOnlyChecker() *ReadOnlyChecker {
	return &ReadOnlyChecker{}
}

// Check verifies the SQL is a safe SELECT query.
func (c *ReadOnlyChecker) Check(sql string) error {
	trimmed := strings.TrimSpace(strings.ToUpper(sql))

	// Must start with SELECT
	if !strings.HasPrefix(trimmed, "SELECT") {
		return fmt.Errorf("only SELECT queries are allowed, got: %s", trimmed[:min(50, len(trimmed))])
	}

	for i, pattern := range dangerousKeywordPatterns {
		if pattern.MatchString(trimmed) {
			return fmt.Errorf("query contains dangerous keyword: %s", dangerousKeywords[i])
		}
	}

	// Reject multiple statements (semicolon check)
	if strings.Contains(sql, ";") && !strings.HasSuffix(strings.TrimSpace(sql), ";") {
		return fmt.Errorf("multiple statements are not allowed")
	}

	return nil
}
