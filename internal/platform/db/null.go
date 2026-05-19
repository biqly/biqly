package db

import (
	"database/sql"
	"strings"
)

// NullIfEmptyPtr returns nil when the string pointer is nil or its trimmed
// value is empty; otherwise it returns the trimmed string. Suitable for
// passing as a SQL parameter that maps to a nullable text column.
func NullIfEmptyPtr(p *string) any {
	if p == nil {
		return nil
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return nil
	}
	return s
}

// NullIfEmpty returns nil when the trimmed string is empty; otherwise it
// returns the trimmed string. Suitable for passing as a SQL parameter that
// maps to a nullable text column.
func NullIfEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

// StringPtrFromNull converts a sql.NullString into a *string, returning nil
// when the value is not valid.
func StringPtrFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
