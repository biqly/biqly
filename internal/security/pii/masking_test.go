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
