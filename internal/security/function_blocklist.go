package security

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// defaultDeniedFunctions are functions that may read files, access remote
// systems, manipulate large objects, or execute code. They are denied for
// every datasource and cannot be removed by datasource configuration.
var defaultDeniedFunctions = []string{
	"xp_cmdshell",
	"openrowset",
	"pg_read_file",
	"pg_execute_server_program",
	"load_file",
	"dblink",
	"dblink_connect",
	"dblink_exec",
	"lo_import",
	"lo_export",
	"read_csv",
	"read_parquet",
	"postgres_scan",
	"mysql_scan",
	"eval",
	"evalfunction",
}

var functionNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// DefaultDeniedFunctions returns a copy of the immutable default deny list.
func DefaultDeniedFunctions() []string {
	return slices.Clone(defaultDeniedFunctions)
}

// NormalizeFunctionBlocklist validates, normalizes, and deduplicates SQL
// function names. Names are compared case-insensitively.
func NormalizeFunctionBlocklist(functions []string) ([]string, error) {
	normalized := make([]string, 0, len(functions))
	for _, function := range functions {
		name := strings.ToLower(strings.TrimSpace(function))
		if !functionNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid SQL function name %q", function)
		}
		if !slices.Contains(normalized, name) {
			normalized = append(normalized, name)
		}
	}
	return normalized, nil
}

// NormalizeCustomFunctionBlocklist returns only datasource additions. Entries
// already covered by immutable defaults are omitted from the stored custom list.
func NormalizeCustomFunctionBlocklist(functions []string) ([]string, error) {
	normalized, err := NormalizeFunctionBlocklist(functions)
	if err != nil {
		return nil, err
	}
	custom := make([]string, 0, len(normalized))
	for _, name := range normalized {
		if !slices.Contains(defaultDeniedFunctions, name) {
			custom = append(custom, name)
		}
	}
	return custom, nil
}

// EffectiveDeniedFunctions combines immutable defaults with datasource-specific
// additions. Callers cannot remove an entry from DefaultDeniedFunctions.
func EffectiveDeniedFunctions(custom []string) ([]string, error) {
	normalized, err := NormalizeCustomFunctionBlocklist(custom)
	if err != nil {
		return nil, err
	}
	effective := DefaultDeniedFunctions()
	for _, name := range normalized {
		if !slices.Contains(effective, name) {
			effective = append(effective, name)
		}
	}
	return effective, nil
}

func checkFunctionBlocklist(sql string, denied []string) error {
	deniedSet := make(map[string]struct{}, len(denied))
	for _, function := range denied {
		deniedSet[function] = struct{}{}
	}
	for i := 0; i < len(sql); {
		switch {
		case readonlyLineCommentAt(sql, i, len(sql)):
			i = skipReadonlyLineComment(sql, i, len(sql))
		case readonlyBlockCommentAt(sql, i, len(sql)):
			i = skipReadonlyBlockComment(sql, i, len(sql))
		case sql[i] == '\'':
			i = skipFunctionStringLiteral(sql, i, '\'')
		case sql[i] == '"':
			name, next := readQuotedFunctionIdentifier(sql, i)
			if _, blocked := deniedSet[strings.ToLower(name)]; blocked && functionCallAt(sql, next) {
				return fmt.Errorf("function %q is blocked", name)
			}
			i = next
		case isFunctionIdentifierStart(sql[i]):
			start := i
			i++
			for i < len(sql) && isFunctionIdentifierPart(sql[i]) {
				i++
			}
			name := strings.ToLower(sql[start:i])
			if _, blocked := deniedSet[name]; blocked && functionCallAt(sql, i) {
				return fmt.Errorf("function %q is blocked", name)
			}
		default:
			i++
		}
	}
	return nil
}

func isFunctionIdentifierStart(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isFunctionIdentifierPart(b byte) bool {
	return isFunctionIdentifierStart(b) || b >= '0' && b <= '9' || b == '$'
}

func readQuotedFunctionIdentifier(sql string, start int) (string, int) {
	var name strings.Builder
	for i := start + 1; i < len(sql); i++ {
		if sql[i] != '"' {
			name.WriteByte(sql[i])
			continue
		}
		if i+1 < len(sql) && sql[i+1] == '"' {
			name.WriteByte('"')
			i++
			continue
		}
		return name.String(), i + 1
	}
	return name.String(), len(sql)
}

func skipFunctionStringLiteral(sql string, start int, quote byte) int {
	for i := start + 1; i < len(sql); i++ {
		if sql[i] != quote {
			continue
		}
		if i+1 < len(sql) && sql[i+1] == quote {
			i++
			continue
		}
		return i + 1
	}
	return len(sql)
}

func functionCallAt(sql string, start int) bool {
	for i := start; i < len(sql); {
		switch {
		case sql[i] == ' ' || sql[i] == '\t' || sql[i] == '\r' || sql[i] == '\n':
			i++
		case readonlyLineCommentAt(sql, i, len(sql)):
			i = skipReadonlyLineComment(sql, i, len(sql))
		case readonlyBlockCommentAt(sql, i, len(sql)):
			i = skipReadonlyBlockComment(sql, i, len(sql))
		default:
			return sql[i] == '('
		}
	}
	return false
}
