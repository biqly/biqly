package security

import (
	"fmt"
	"strings"
)

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

	// Reject dangerous keywords
	dangerous := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "ALTER",
		"TRUNCATE", "CREATE", "GRANT", "REVOKE", "MERGE",
		"CALL", "EXEC", "EXECUTE",
	}

	for _, kw := range dangerous {
		if strings.Contains(trimmed, kw) {
			return fmt.Errorf("query contains dangerous keyword: %s", kw)
		}
	}

	// Reject multiple statements (semicolon check)
	if strings.Contains(sql, ";") && !strings.HasSuffix(strings.TrimSpace(sql), ";") {
		return fmt.Errorf("multiple statements are not allowed")
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
