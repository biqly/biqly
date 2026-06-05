package db

import (
	"database/sql"
	"strings"
	"time"
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
	return new(value.String)
}

// TimePtrFromNull converts a sql.NullTime into a *time.Time, returning nil
// when the value is not valid.
func TimePtrFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return new(value.Time)
}

// IntPtrFromNull converts a sql.NullInt64 into a *int, returning nil
// when the value is not valid.
func IntPtrFromNull(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	return new(int(value.Int64))
}

// NullIfNilIntPtr returns nil when the int pointer is nil; otherwise it returns
// the dereferenced int. Suitable for passing as a SQL parameter that maps to a
// nullable integer column.
func NullIfNilIntPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// NullStringArray helps scan PostgreSQL arrays into Go slices.
type NullStringArray struct {
	S *[]string
}

// Scan implements the sql.Scanner interface for NullStringArray.
func (n *NullStringArray) Scan(src any) error {
	if src == nil {
		*n.S = []string{}
		return nil
	}
	switch v := src.(type) {
	case string:
		*n.S = ParseStringArray(v)
	case []byte:
		*n.S = ParseStringArray(string(v))
	default:
		*n.S = []string{}
	}
	return nil
}

// ParseStringArray parses PostgreSQL string arrays: {"a","b","c"}
func ParseStringArray(s string) []string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.Trim(p, "\"")
	}
	return parts
}
