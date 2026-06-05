package query

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

var (
	benchmarkFilterSQL  string
	benchmarkFilterArgs []any
)

func BenchmarkCompilerFilterHandler(b *testing.B) {
	compiler := NewCompiler(dialect.PostgresDialect{})
	model := &semantic.SemanticModel{}
	lhsSQL := `"orders"."status"`
	stringValues := []string{"new", "paid", "shipped", "closed"}
	anyStringValues := []any{"new", "paid", "shipped", "closed"}
	anyIntValues := []any{1, 2, 3, 4}
	intValues := []int{1, 2, 3, 4}

	bench := func(b *testing.B, run func(*[]any) (string, error)) {
		b.Helper()
		b.ReportAllocs()
		var sql string
		var args []any
		for b.Loop() {
			args = make([]any, 0, 8)
			var err error
			sql, err = run(&args)
			if err != nil {
				b.Fatal(err)
			}
		}
		benchmarkFilterSQL = sql
		benchmarkFilterArgs = args
	}

	b.Run("current_dispatch_eq_string_slice", func(b *testing.B) {
		filter := Filter{Operator: OpEq, Value: stringValues}
		bench(b, func(args *[]any) (string, error) {
			sql, _, err := compiler.buildFilterPart(filter, lhsSQL, model, args)
			return sql, err
		})
	})

	b.Run("direct_method_eq_string_slice", func(b *testing.B) {
		filter := Filter{Operator: OpEq, Value: stringValues}
		bench(b, func(args *[]any) (string, error) {
			sql, _, err := compiler.buildEqFilter(filter, lhsSQL, model, args)
			return sql, err
		})
	})

	b.Run("typed_helper_eq_string_slice", func(b *testing.B) {
		bench(b, func(args *[]any) (string, error) {
			return benchmarkBuildEqStrings(compiler, lhsSQL, stringValues, args), nil
		})
	})

	b.Run("current_dispatch_in_any_strings", func(b *testing.B) {
		filter := Filter{Operator: OpIn, Value: anyStringValues}
		bench(b, func(args *[]any) (string, error) {
			sql, _, err := compiler.buildFilterPart(filter, lhsSQL, model, args)
			return sql, err
		})
	})

	b.Run("direct_method_in_any_strings", func(b *testing.B) {
		bench(b, func(args *[]any) (string, error) {
			sql, _, err := compiler.buildInFilter(lhsSQL, anyStringValues, args)
			return sql, err
		})
	})

	b.Run("typed_helper_in_strings", func(b *testing.B) {
		bench(b, func(args *[]any) (string, error) {
			return benchmarkBuildInTyped(compiler, lhsSQL, stringValues, args), nil
		})
	})

	b.Run("current_dispatch_in_any_ints", func(b *testing.B) {
		filter := Filter{Operator: OpIn, Value: anyIntValues}
		bench(b, func(args *[]any) (string, error) {
			sql, _, err := compiler.buildFilterPart(filter, lhsSQL, model, args)
			return sql, err
		})
	})

	b.Run("typed_helper_in_ints", func(b *testing.B) {
		bench(b, func(args *[]any) (string, error) {
			return benchmarkBuildInTyped(compiler, lhsSQL, intValues, args), nil
		})
	})
}

func benchmarkBuildEqStrings(c *Compiler, lhsSQL string, vals []string, args *[]any) string {
	if len(vals) == 0 {
		return "1=1"
	}
	parts := make([]string, 0, len(vals))
	for _, valStr := range vals {
		*args = append(*args, valStr)
		parts = append(parts, lhsSQL+" = "+c.dialect.Placeholder(len(*args)))
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

func benchmarkBuildInTyped[T any](c *Compiler, lhsSQL string, vals []T, args *[]any) string {
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		*args = append(*args, v)
		placeholders[i] = c.dialect.Placeholder(len(*args))
	}
	return lhsSQL + " IN (" + strings.Join(placeholders, ", ") + ")"
}
