package pii

import (
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
)

// Access levels for PII columns.
const (
	AccessRaw    = "raw"
	AccessMasked = "masked"
	AccessHidden = "hidden"
)

// Masking strategies for masked PII columns.
const (
	MaskingStrategyPartial = "partial"
	MaskingStrategyFull    = "full"
)

// HiddenLiteral is the SQL literal emitted in place of hidden PII columns.
const HiddenLiteral = "'***'"

// NormalizeMaskingStrategy canonicalizes a stored strategy value. Unknown
// non-empty values fail closed to full masking.
func NormalizeMaskingStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", MaskingStrategyPartial:
		return MaskingStrategyPartial
	case MaskingStrategyFull:
		return MaskingStrategyFull
	default:
		return MaskingStrategyFull
	}
}

// EffectiveColumnAccess folds the column masking strategy into the access
// level so downstream query compilation has one decision path.
func EffectiveColumnAccess(access, strategy string) string {
	switch access {
	case AccessRaw:
		return AccessRaw
	case AccessMasked:
		if NormalizeMaskingStrategy(strategy) == MaskingStrategyFull {
			return AccessHidden
		}
		return AccessMasked
	case AccessHidden:
		return AccessHidden
	default:
		return AccessHidden
	}
}

// MaskingStrategy renders the SQL expression that masks a PII column.
type MaskingStrategy interface {
	// MaskExpression wraps columnRef (an already-quoted column reference)
	// with the dialect-specific masking expression for piiType.
	MaskExpression(columnRef string, piiType string, d dialect.Dialect) string
}

// DefaultMaskingStrategy implements per-type partial masking, e.g.
// "jo***@gmail.com" for emails. Unknown PII types fail closed to a full mask.
type DefaultMaskingStrategy struct{}

// MaskExpression implements MaskingStrategy.
func (DefaultMaskingStrategy) MaskExpression(columnRef, piiType string, d dialect.Dialect) string {
	// Cast to text first so numeric PII (e.g. TCKN stored as BIGINT) can be
	// sliced with string functions in every dialect.
	col := castText(columnRef, d)
	switch piiType {
	case TypeEmail:
		// jo***@gmail.com
		return concat(d, left(d, col, 2), "'***'", fromChar(d, col, "@"))
	case TypePhone:
		// 055****67
		return concat(d, left(d, col, 3), "'****'", right(d, col, 2))
	case TypeIBAN:
		// TR33****26
		return concat(d, left(d, col, 4), "'****'", right(d, col, 2))
	case TypeTCKimlikNo:
		// 100*****
		return concat(d, left(d, col, 3), "'*****'")
	case TypeAddress:
		// Atatürk Ma...
		return concat(d, left(d, col, 10), "'...'")
	case TypeIPAddress:
		return maskIPExpr(d, col)
	case TypeCreditCardLike:
		// 4111 **** **** 1111
		return concat(d, left(d, col, 4), "' **** **** '", right(d, col, 4))
	default:
		// Fail closed: unknown PII types get a full mask.
		return HiddenLiteral
	}
}

// castText renders a dialect-appropriate cast of col to a string type.
func castText(col string, d dialect.Dialect) string {
	switch d.Name() {
	case "mysql":
		return "CAST(" + col + " AS CHAR)"
	case "sqlserver":
		return "CAST(" + col + " AS NVARCHAR(256))"
	case "clickhouse":
		return "toString(" + col + ")"
	default: // postgres and ANSI-compatible dialects
		return "CAST(" + col + " AS TEXT)"
	}
}

// concat joins string expressions using the dialect's concatenation form.
func concat(d dialect.Dialect, parts ...string) string {
	switch d.Name() {
	case "mysql":
		return "CONCAT(" + strings.Join(parts, ", ") + ")"
	case "clickhouse":
		return "concat(" + strings.Join(parts, ", ") + ")"
	case "sqlserver":
		// CONCAT (2012+) avoids NULL-propagation surprises of "+".
		return "CONCAT(" + strings.Join(parts, ", ") + ")"
	default: // postgres
		return "(" + strings.Join(parts, " || ") + ")"
	}
}

// left returns the first n characters of expr.
func left(d dialect.Dialect, expr string, n int) string {
	if d.Name() == "clickhouse" {
		return "substring(" + expr + ", 1, " + strconv.Itoa(n) + ")"
	}
	return "LEFT(" + expr + ", " + strconv.Itoa(n) + ")"
}

// right returns the last n characters of expr.
func right(d dialect.Dialect, expr string, n int) string {
	if d.Name() == "clickhouse" {
		return "substring(" + expr + ", length(" + expr + ") - " + strconv.Itoa(n-1) + ", " + strconv.Itoa(n) + ")"
	}
	return "RIGHT(" + expr + ", " + strconv.Itoa(n) + ")"
}

// fromChar returns the substring of expr starting at the first occurrence of
// marker (inclusive), e.g. the "@domain.com" tail of an email.
func fromChar(d dialect.Dialect, expr, marker string) string {
	lit := "'" + marker + "'"
	switch d.Name() {
	case "mysql":
		return "SUBSTRING(" + expr + ", LOCATE(" + lit + ", " + expr + "))"
	case "sqlserver":
		return "SUBSTRING(" + expr + ", CHARINDEX(" + lit + ", " + expr + "), LEN(" + expr + "))"
	case "clickhouse":
		return "substring(" + expr + ", position(" + expr + ", " + lit + "))"
	default: // postgres
		return "SUBSTRING(" + expr + " FROM POSITION(" + lit + " IN " + expr + "))"
	}
}

// maskIPExpr replaces every numeric/hex group of an IP with "*".
func maskIPExpr(d dialect.Dialect, expr string) string {
	const pattern = "'[0-9a-fA-F]+'"
	switch d.Name() {
	case "postgres":
		return "REGEXP_REPLACE(" + expr + ", " + pattern + ", '*', 'g')"
	case "mysql":
		return "REGEXP_REPLACE(" + expr + ", " + pattern + ", '*')"
	case "clickhouse":
		return "replaceRegexpAll(" + expr + ", " + pattern + ", '*')"
	default:
		// SQL Server has no native regex: fail closed to a full mask.
		return HiddenLiteral
	}
}
