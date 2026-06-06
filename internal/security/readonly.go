package security

import (
	"errors"
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
		"XP_CMDSHELL", "OPENROWSET", "PG_READ_FILE",
		"LOAD_FILE", "DBLINK", "LO_IMPORT",
		"SET", "RESET", "COPY", "DO", "LOCK", "VACUUM", "REINDEX",
	}
	dangerousKeywords = dangerous
	dangerousKeywordPatterns = make([]*regexp.Regexp, 0, len(dangerous)+1)
	for _, kw := range dangerous {
		dangerousKeywordPatterns = append(dangerousKeywordPatterns,
			regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(kw)+`\b`))
	}
	dangerousKeywords = append(dangerousKeywords, "BULK INSERT")
	dangerousKeywordPatterns = append(dangerousKeywordPatterns,
		regexp.MustCompile(`(?i)\bBULK\s+INSERT\b`))
}

// ReadOnlyChecker validates that SQL is safe to execute.
type ReadOnlyChecker struct{}

// NewReadOnlyChecker creates a new read-only SQL checker.
func NewReadOnlyChecker() *ReadOnlyChecker {
	return &ReadOnlyChecker{}
}

// Check verifies the SQL is a safe SELECT/WITH/EXPLAIN query.
func (*ReadOnlyChecker) Check(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return errors.New("empty query")
	}

	cleaned, err := stripSQLLiteralsAndComments(trimmed)
	if err != nil {
		return fmt.Errorf("strip sql literals and comments: %w", err)
	}
	cleanedTrim := strings.TrimSpace(cleaned)
	if cleanedTrim == "" {
		return errors.New("empty query after stripping comments")
	}

	upper := strings.ToUpper(cleanedTrim)
	if !strings.HasPrefix(upper, "SELECT") &&
		!strings.HasPrefix(upper, "WITH") &&
		!strings.HasPrefix(upper, "EXPLAIN") {
		head := cleanedTrim
		if len(head) > 50 {
			head = head[:50]
		}
		return fmt.Errorf("only SELECT, WITH or EXPLAIN queries are allowed, got: %s", head)
	}

	for i, pattern := range dangerousKeywordPatterns {
		if pattern.MatchString(cleaned) {
			return fmt.Errorf("query contains dangerous keyword: %s", dangerousKeywords[i])
		}
	}

	if hasMultipleStatements(cleaned) {
		return errors.New("multiple statements are not allowed")
	}

	return nil
}

// hasMultipleStatements reports whether the cleaned SQL contains more than one
// top-level statement. Trailing semicolons are allowed; any semicolon followed
// by further non-whitespace content is treated as a statement separator.
func hasMultipleStatements(cleaned string) bool {
	stripped := strings.TrimRight(cleaned, " \t\r\n;")
	return strings.Contains(stripped, ";")
}

// stripSQLLiteralsAndComments removes single-quoted strings, double-quoted
// identifiers, line comments (--), and block comments (/* */) from sql so the
// remaining text can be safely scanned for keywords and statement separators.
// String/identifier content is replaced with empty placeholders that preserve
// surrounding token boundaries.
//
//nolint:gocognit
func writeStrippedSQLLiteralsAndComments(sql string, out *strings.Builder) error {
	i := 0
	n := len(sql)
	for i < n {
		c := sql[i]

		if c == '-' && i+1 < n && sql[i+1] == '-' {
			for i < n && sql[i] != '\n' {
				i++
			}
			continue
		}

		if c == '/' && i+1 < n && sql[i+1] == '*' {
			i += 2
			for i+1 < n && (sql[i] != '*' || sql[i+1] != '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			continue
		}

		if c == '\'' {
			var err error
			i, err = skipStringLiteral(sql, i, n, '\'', out)
			if err != nil {
				return err
			}
			continue
		}

		if c == '"' {
			var err error
			i, err = skipStringLiteral(sql, i, n, '"', out)
			if err != nil {
				return err
			}
			continue
		}

		if err := writeBuilderByte(out, c); err != nil {
			return err
		}
		i++
	}
	return nil
}

func skipStringLiteral(sql string, i, n int, quote byte, out *strings.Builder) (int, error) {
	if err := writeBuilderByte(out, quote); err != nil {
		return i, err
	}
	i++
	for i < n {
		if sql[i] == quote {
			if i+1 < n && sql[i+1] == quote {
				i += 2
				continue
			}
			if err := writeBuilderByte(out, quote); err != nil {
				return i, err
			}
			i++
			break
		}
		i++
	}
	return i, nil
}

func writeBuilderByte(out *strings.Builder, b byte) error {
	return out.WriteByte(b)
}

func stripSQLLiteralsAndComments(sql string) (string, error) {
	var out strings.Builder
	out.Grow(len(sql))
	if err := writeStrippedSQLLiteralsAndComments(sql, &out); err != nil {
		return "", err
	}
	return out.String(), nil
}
