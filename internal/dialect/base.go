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
	parts := make([]string, 0, 2)
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

type aggregateSpelling struct {
	countStar      string
	count          string
	countDistinct  string
	sum            string
	avg            string
	min            string
	max            string
	defaultUnknown string
}

func aggregateSQL(d interface{ QuoteIdent(string) string }, fn, column string, spell aggregateSpelling) string {
	fnLower := strings.ToLower(strings.TrimSpace(fn))
	if fnLower == "custom" || fnLower == "none" || fnLower == "" {
		return column
	}
	if fnLower == "count" && column == "*" {
		return spell.countStar
	}
	quotedCol := d.QuoteIdent(column)
	switch fnLower {
	case "count":
		return fmt.Sprintf(spell.count, quotedCol)
	case "count_distinct":
		return fmt.Sprintf(spell.countDistinct, quotedCol)
	case "sum":
		return fmt.Sprintf(spell.sum, quotedCol)
	case "avg":
		return fmt.Sprintf(spell.avg, quotedCol)
	case "min":
		return fmt.Sprintf(spell.min, quotedCol)
	case "max":
		return fmt.Sprintf(spell.max, quotedCol)
	default:
		return fmt.Sprintf(spell.defaultUnknown, quotedCol)
	}
}

// AggregateStandardSQL formats COUNT/SUM/AVG/MIN/MAX in the conventional SQL style (PostgreSQL family).
func AggregateStandardSQL(d interface{ QuoteIdent(string) string }, fn, column string) string {
	return aggregateSQL(d, fn, column, aggregateSpelling{
		countStar:      "COUNT(*)",
		count:          "COUNT(%s)",
		countDistinct:  "COUNT(DISTINCT %s)",
		sum:            "SUM(%s)",
		avg:            "AVG(%s)",
		min:            "MIN(%s)",
		max:            "MAX(%s)",
		defaultUnknown: "COUNT(%s)",
	})
}

// AggregateClickHouseSQL formats aggregates using ClickHouse-native spellings where they differ.
func AggregateClickHouseSQL(d interface{ QuoteIdent(string) string }, fn, column string) string {
	return aggregateSQL(d, fn, column, aggregateSpelling{
		countStar:      "count()",
		count:          "count(%s)",
		countDistinct:  "uniq(%s)",
		sum:            "sum(%s)",
		avg:            "avg(%s)",
		min:            "min(%s)",
		max:            "max(%s)",
		defaultUnknown: "count(%s)",
	})
}
