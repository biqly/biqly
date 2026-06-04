package query

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

var dialectFunctions = map[string]map[string]string{
	"COALESCE": {
		"clickhouse": "coalesce",
	},
	"CONCAT": {
		"clickhouse": "concat",
	},
	"ROUND": {
		"clickhouse": "round",
	},
	"NULLIF": {
		"clickhouse": "nullif",
	},
	"SUBSTRING": {
		"clickhouse": "substring",
	},
	"REPLACE": {
		"clickhouse": "replace",
	},
	"DATE_TRUNC": {
		"postgres":   "DATE_TRUNC",
		"mysql":      "DATE_FORMAT",
		"sqlserver":  "DATETRUNC",
		"clickhouse": "toStartOfInterval",
	},
}

// CompileExpr emits safe SQL from a canonical semantic expression AST.
func CompileExpr(expr pkgsemantic.ExprNode, d dialect.Dialect, resolver *SchemaResolver) string {
	d = normalizeExprDialect(d)
	return compileExpr(expr, d, resolver)
}

func normalizeExprDialect(d dialect.Dialect) dialect.Dialect {
	switch concrete := d.(type) {
	case dialect.PostgresDialect:
		if concrete.QuoteLeft == "" {
			return dialect.Postgres
		}
	case dialect.MySQLDialect:
		if concrete.QuoteLeft == "" {
			return dialect.MySQL
		}
	case dialect.SQLServerDialect:
		if concrete.QuoteLeft == "" {
			return dialect.SQLServer
		}
	case dialect.ClickHouseDialect:
		if concrete.QuoteLeft == "" {
			return dialect.ClickHouse
		}
	}
	return d
}

func compileExpr(expr pkgsemantic.ExprNode, d dialect.Dialect, resolver *SchemaResolver) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case pkgsemantic.LiteralExpr:
		return literalSQL(e.Value)
	case pkgsemantic.ColumnRefExpr:
		return columnRefSQL(e, d, resolver)
	case pkgsemantic.MetricRefExpr:
		return d.QuoteIdent(e.Name)
	case pkgsemantic.DimensionRefExpr:
		return d.QuoteIdent(e.Name)
	case pkgsemantic.BinaryExpr:
		return binaryExprSQL(e, d, resolver)
	case pkgsemantic.UnaryExpr:
		return unaryExprSQL(e, d, resolver)
	case pkgsemantic.FunctionCallExpr:
		return functionCallSQL(e, d, resolver)
	case pkgsemantic.CaseExpr:
		return caseExprSQL(e, d, resolver)
	default:
		return ""
	}
}

func literalSQL(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'"
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
	}
}

func columnRefSQL(ref pkgsemantic.ColumnRefExpr, d dialect.Dialect, resolver *SchemaResolver) string {
	return d.QuoteIdent(columnRefPath(ref, resolver))
}

func columnRefPath(ref pkgsemantic.ColumnRefExpr, resolver *SchemaResolver) string {
	columnRef := ref.Column
	if ref.Table != "" {
		columnRef = ref.Table + "." + ref.Column
	}
	if resolver != nil {
		return resolver.PhysicalColumnRef(columnRef)
	}
	return columnRef
}

func binaryExprSQL(expr pkgsemantic.BinaryExpr, d dialect.Dialect, resolver *SchemaResolver) string {
	left := compileExpr(expr.Left, d, resolver)
	right := compileExpr(expr.Right, d, resolver)
	if expr.Op == pkgsemantic.OpConcat {
		switch d.Name() {
		case "mysql":
			return "CONCAT(" + left + ", " + right + ")"
		case "sqlserver":
			return "(" + left + " + " + right + ")"
		case "clickhouse":
			return "concat(" + left + ", " + right + ")"
		default:
			return "(" + left + " || " + right + ")"
		}
	}
	return "(" + left + " " + binaryOpSQL(expr.Op) + " " + right + ")"
}

func unaryExprSQL(expr pkgsemantic.UnaryExpr, d dialect.Dialect, resolver *SchemaResolver) string {
	inner := compileExpr(expr.Expr, d, resolver)
	switch expr.Op {
	case pkgsemantic.OpNot:
		return "(NOT " + inner + ")"
	case pkgsemantic.OpNegate:
		return "(-" + inner + ")"
	default:
		return "(" + strings.ToUpper(string(expr.Op)) + " " + inner + ")"
	}
}

func functionCallSQL(expr pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver) string {
	if sql, ok := dateTruncSQL(expr, d, resolver); ok {
		return sql
	}

	args := make([]string, 0, len(expr.Args))
	for _, arg := range expr.Args {
		args = append(args, compileExpr(arg, d, resolver))
	}
	name := functionNameSQL(expr.Name, d)
	return name + "(" + strings.Join(args, ", ") + ")"
}

func dateTruncSQL(expr pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver) (string, bool) {
	if !strings.EqualFold(expr.Name, "DATE_TRUNC") || len(expr.Args) != 2 {
		return "", false
	}

	part, ok := literalString(expr.Args[0])
	if !ok {
		return "", false
	}

	column, ok := dateTruncColumnRef(expr.Args[1], resolver)
	if !ok {
		return "", false
	}
	return d.DateTrunc(part, column), true
}

func literalString(expr pkgsemantic.ExprNode) (string, bool) {
	lit, ok := expr.(pkgsemantic.LiteralExpr)
	if !ok {
		return "", false
	}
	value, ok := lit.Value.(string)
	return value, ok
}

func dateTruncColumnRef(expr pkgsemantic.ExprNode, resolver *SchemaResolver) (string, bool) {
	ref, ok := expr.(pkgsemantic.ColumnRefExpr)
	if !ok {
		return "", false
	}
	return columnRefPath(ref, resolver), true
}

func caseExprSQL(expr pkgsemantic.CaseExpr, d dialect.Dialect, resolver *SchemaResolver) string {
	var b strings.Builder
	b.WriteString("CASE")
	for _, condition := range expr.Conditions {
		b.WriteString(" WHEN ")
		b.WriteString(compileExpr(condition.When, d, resolver))
		b.WriteString(" THEN ")
		b.WriteString(compileExpr(condition.Then, d, resolver))
	}
	if expr.ElseExpr != nil {
		b.WriteString(" ELSE ")
		b.WriteString(compileExpr(expr.ElseExpr, d, resolver))
	}
	b.WriteString(" END")
	return b.String()
}

func binaryOpSQL(op pkgsemantic.BinaryOp) string {
	switch op {
	case pkgsemantic.OpAdd:
		return "+"
	case pkgsemantic.OpSubtract:
		return "-"
	case pkgsemantic.OpMultiply:
		return "*"
	case pkgsemantic.OpDivide:
		return "/"
	case pkgsemantic.OpModulo:
		return "%"
	case pkgsemantic.OpEq:
		return "="
	case pkgsemantic.OpNeq:
		return "<>"
	case pkgsemantic.OpLt:
		return "<"
	case pkgsemantic.OpLte:
		return "<="
	case pkgsemantic.OpGt:
		return ">"
	case pkgsemantic.OpGte:
		return ">="
	case pkgsemantic.OpAnd:
		return "AND"
	case pkgsemantic.OpOr:
		return "OR"
	case pkgsemantic.OpConcat:
		return "||"
	default:
		return strings.ToUpper(string(op))
	}
}

func functionNameSQL(name string, d dialect.Dialect) string {
	upper := strings.ToUpper(name)
	if byDialect, ok := dialectFunctions[upper]; ok {
		if mapped := byDialect[d.Name()]; mapped != "" {
			return mapped
		}
	}
	return upper
}
