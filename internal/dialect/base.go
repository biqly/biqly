package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

// identSegmentQuoter is implemented by every Dialect for qualified identifier quoting.
type identSegmentQuoter interface {
	QuoteIdentSegment(part string) string
}

// QuoteIdentQualified splits identifier on '.' and quotes each segment.
func QuoteIdentQualified(q identSegmentQuoter, identifier string) string {
	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = q.QuoteIdentSegment(part)
	}
	return strings.Join(quoted, ".")
}

// StandardLimitOffset is the LIMIT/OFFSET clause shared by PostgreSQL, MySQL, and ClickHouse.
func StandardLimitOffset(limit, offset int) string {
	var parts []string
	if limit > 0 {
		parts = append(parts, "LIMIT "+strconv.Itoa(limit))
	}
	if offset > 0 {
		parts = append(parts, "OFFSET "+strconv.Itoa(offset))
	}
	return strings.Join(parts, " ")
}

// CastTypeUpper returns the upper-cased SQL type name used uniformly by all built-in dialects.
func CastTypeUpper(sqlType string) string {
	return strings.ToUpper(sqlType)
}

// AggregateStandardSQL formats COUNT/SUM/AVG/MIN/MAX in the conventional SQL style (PostgreSQL family).
func AggregateStandardSQL(d Dialect, fn, column string) string {
	if strings.ToLower(fn) == "count" && column == "*" {
		return "COUNT(*)"
	}
	quotedCol := d.QuoteIdent(column)
	switch strings.ToLower(fn) {
	case "count":
		return fmt.Sprintf("COUNT(%s)", quotedCol)
	case "count_distinct":
		return fmt.Sprintf("COUNT(DISTINCT %s)", quotedCol)
	case "sum":
		return fmt.Sprintf("SUM(%s)", quotedCol)
	case "avg":
		return fmt.Sprintf("AVG(%s)", quotedCol)
	case "min":
		return fmt.Sprintf("MIN(%s)", quotedCol)
	case "max":
		return fmt.Sprintf("MAX(%s)", quotedCol)
	default:
		return fmt.Sprintf("COUNT(%s)", quotedCol)
	}
}

// AggregateClickHouseSQL formats aggregates using ClickHouse-native spellings where they differ.
func AggregateClickHouseSQL(d Dialect, fn, column string) string {
	if strings.ToLower(fn) == "count" && column == "*" {
		return "count()"
	}
	quotedCol := d.QuoteIdent(column)
	switch strings.ToLower(fn) {
	case "count":
		return fmt.Sprintf("count(%s)", quotedCol)
	case "count_distinct":
		return fmt.Sprintf("uniq(%s)", quotedCol)
	case "sum":
		return fmt.Sprintf("sum(%s)", quotedCol)
	case "avg":
		return fmt.Sprintf("avg(%s)", quotedCol)
	case "min":
		return fmt.Sprintf("min(%s)", quotedCol)
	case "max":
		return fmt.Sprintf("max(%s)", quotedCol)
	default:
		return fmt.Sprintf("count(%s)", quotedCol)
	}
}
