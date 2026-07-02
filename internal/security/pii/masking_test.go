package pii

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/stretchr/testify/assert"
)

var allDialects = []dialect.Dialect{
	dialect.Postgres,
	dialect.MySQL,
	dialect.SQLServer,
	dialect.ClickHouse,
}

func TestMaskExpression_AllTypesAllDialects(t *testing.T) {
	strategy := DefaultMaskingStrategy{}
	for _, d := range allDialects {
		for _, piiType := range AllTypes {
			t.Run(d.Name()+"/"+piiType, func(t *testing.T) {
				expr := strategy.MaskExpression(`"email"`, piiType, d)
				assert.NotEmpty(t, expr)
				// Every mask either references the column or fully hides it.
				if expr != HiddenLiteral {
					assert.Contains(t, expr, `"email"`)
					redacted := strings.Contains(expr, "*") || strings.Contains(expr, "...")
					assert.True(t, redacted, "no redaction marker: %s", expr)
				}
				// Balanced parentheses sanity check.
				assert.Equal(t, strings.Count(expr, "("), strings.Count(expr, ")"), "unbalanced parens: %s", expr)
			})
		}
	}
}

func TestMaskExpression_PostgresShapes(t *testing.T) {
	strategy := DefaultMaskingStrategy{}
	d := dialect.Postgres

	email := strategy.MaskExpression("col", TypeEmail, d)
	assert.Equal(t, "(LEFT(CAST(col AS TEXT), 2) || '***' || SUBSTRING(CAST(col AS TEXT) FROM POSITION('@' IN CAST(col AS TEXT))))", email)

	phone := strategy.MaskExpression("col", TypePhone, d)
	assert.Equal(t, "(LEFT(CAST(col AS TEXT), 3) || '****' || RIGHT(CAST(col AS TEXT), 2))", phone)

	ip := strategy.MaskExpression("col", TypeIPAddress, d)
	assert.Equal(t, "REGEXP_REPLACE(CAST(col AS TEXT), '[0-9a-fA-F]+', '*', 'g')", ip)
}

func TestMaskExpression_MySQLUsesConcatAndLocate(t *testing.T) {
	strategy := DefaultMaskingStrategy{}
	expr := strategy.MaskExpression("col", TypeEmail, dialect.MySQL)
	assert.Contains(t, expr, "CONCAT(")
	assert.Contains(t, expr, "LOCATE('@'")
	assert.NotContains(t, expr, "||")
}

func TestMaskExpression_SQLServerUsesCharindex(t *testing.T) {
	strategy := DefaultMaskingStrategy{}
	expr := strategy.MaskExpression("col", TypeEmail, dialect.SQLServer)
	assert.Contains(t, expr, "CONCAT(")
	assert.Contains(t, expr, "CHARINDEX('@'")

	// No native regex in SQL Server: IP masking fails closed.
	assert.Equal(t, HiddenLiteral, strategy.MaskExpression("col", TypeIPAddress, dialect.SQLServer))
}

func TestMaskExpression_ClickHouseUsesLowercaseFunctions(t *testing.T) {
	strategy := DefaultMaskingStrategy{}
	expr := strategy.MaskExpression("col", TypeCreditCardLike, dialect.ClickHouse)
	assert.Contains(t, expr, "concat(")
	assert.Contains(t, expr, "substring(")
	assert.NotContains(t, expr, "LEFT(")

	ip := strategy.MaskExpression("col", TypeIPAddress, dialect.ClickHouse)
	assert.Contains(t, ip, "replaceRegexpAll(")
}

func TestMaskExpression_UnknownTypeFailsClosed(t *testing.T) {
	strategy := DefaultMaskingStrategy{}
	for _, d := range allDialects {
		assert.Equal(t, HiddenLiteral, strategy.MaskExpression("col", "unknown_type", d))
	}
}

func TestNormalizeMaskingStrategy_Empty(t *testing.T) {
	assert.Equal(t, MaskingStrategyPartial, NormalizeMaskingStrategy(""))
}

func TestNormalizeMaskingStrategy_Partial(t *testing.T) {
	assert.Equal(t, MaskingStrategyPartial, NormalizeMaskingStrategy(MaskingStrategyPartial))
	assert.Equal(t, MaskingStrategyPartial, NormalizeMaskingStrategy("PARTIAL"))
	assert.Equal(t, MaskingStrategyPartial, NormalizeMaskingStrategy(" Partial "))
}

func TestNormalizeMaskingStrategy_Full(t *testing.T) {
	assert.Equal(t, MaskingStrategyFull, NormalizeMaskingStrategy(MaskingStrategyFull))
	assert.Equal(t, MaskingStrategyFull, NormalizeMaskingStrategy("FULL"))
}

func TestNormalizeMaskingStrategy_UnknownFailsClosed(t *testing.T) {
	assert.Equal(t, MaskingStrategyFull, NormalizeMaskingStrategy("surprise"))
	assert.Equal(t, MaskingStrategyFull, NormalizeMaskingStrategy("whatever"))
}

func TestEffectiveColumnAccess_Raw(t *testing.T) {
	assert.Equal(t, AccessRaw, EffectiveColumnAccess(AccessRaw, ""))
	assert.Equal(t, AccessRaw, EffectiveColumnAccess(AccessRaw, MaskingStrategyFull))
	assert.Equal(t, AccessRaw, EffectiveColumnAccess(AccessRaw, MaskingStrategyPartial))
	assert.Equal(t, AccessRaw, EffectiveColumnAccess(AccessRaw, "surprise"))
}

func TestEffectiveColumnAccess_MaskedWithPartialStrategy(t *testing.T) {
	assert.Equal(t, AccessMasked, EffectiveColumnAccess(AccessMasked, MaskingStrategyPartial))
	assert.Equal(t, AccessMasked, EffectiveColumnAccess(AccessMasked, ""))
}

func TestEffectiveColumnAccess_MaskedWithFullStrategy(t *testing.T) {
	assert.Equal(t, AccessHidden, EffectiveColumnAccess(AccessMasked, MaskingStrategyFull))
}

func TestEffectiveColumnAccess_MaskedWithUnknownStrategy(t *testing.T) {
	// Unknown strategies normalize to full, which maps masked to hidden
	assert.Equal(t, AccessHidden, EffectiveColumnAccess(AccessMasked, "surprise"))
}

func TestEffectiveColumnAccess_Hidden(t *testing.T) {
	assert.Equal(t, AccessHidden, EffectiveColumnAccess(AccessHidden, ""))
	assert.Equal(t, AccessHidden, EffectiveColumnAccess(AccessHidden, MaskingStrategyPartial))
	assert.Equal(t, AccessHidden, EffectiveColumnAccess(AccessHidden, MaskingStrategyFull))
}

func TestEffectiveColumnAccess_UnknownAccessFailsClosed(t *testing.T) {
	assert.Equal(t, AccessHidden, EffectiveColumnAccess("bogus", ""))
	assert.Equal(t, AccessHidden, EffectiveColumnAccess("", ""))
}

func TestCastText_newDrivers(t *testing.T) {
	cases := []struct {
		d    dialect.Dialect
		want string
	}{
		{dialect.Oracle, "CAST(x AS VARCHAR2(256))"},
		{dialect.Databricks, "CAST(x AS STRING)"},
		{dialect.SQLite, "CAST(x AS TEXT)"},
		{dialect.Snowflake, "CAST(x AS TEXT)"},
	}
	for _, tc := range cases {
		if got := castText("x", tc.d); got != tc.want {
			t.Errorf("castText(%s) = %q, want %q", tc.d.Name(), got, tc.want)
		}
	}
}

func TestLeftRight_substrDialects(t *testing.T) {
	for _, d := range []dialect.Dialect{dialect.Oracle, dialect.SQLite} {
		if got := left(d, "x", 3); got != "SUBSTR(x, 1, 3)" {
			t.Errorf("left(%s) = %q", d.Name(), got)
		}
		if got := right(d, "x", 2); got != "SUBSTR(x, -2, 2)" {
			t.Errorf("right(%s) = %q", d.Name(), got)
		}
	}
}

func TestFromChar_newDrivers(t *testing.T) {
	cases := []struct {
		d    dialect.Dialect
		want string
	}{
		{dialect.SQLite, "SUBSTR(x, INSTR(x, '@'))"},
		{dialect.Oracle, "SUBSTR(x, INSTR(x, '@'))"},
		{dialect.Snowflake, "SUBSTR(x, POSITION('@', x))"},
		{dialect.Databricks, "substring(x, instr(x, '@'))"},
	}
	for _, tc := range cases {
		if got := fromChar(tc.d, "x", "@"); got != tc.want {
			t.Errorf("fromChar(%s) = %q, want %q", tc.d.Name(), got, tc.want)
		}
	}
}

func TestMaskIPExpr_newDrivers(t *testing.T) {
	const pattern = "'[0-9a-fA-F]+'"
	for _, d := range []dialect.Dialect{dialect.Snowflake, dialect.Databricks, dialect.Oracle} {
		want := "REGEXP_REPLACE(x, " + pattern + ", '*')"
		if got := maskIPExpr(d, "x"); got != want {
			t.Errorf("maskIPExpr(%s) = %q, want %q", d.Name(), got, want)
		}
	}
	// SQLite has no native regex: must fail closed to the hidden literal.
	if got := maskIPExpr(dialect.SQLite, "x"); got != HiddenLiteral {
		t.Errorf("maskIPExpr(sqlite) = %q, want %q", got, HiddenLiteral)
	}
}
