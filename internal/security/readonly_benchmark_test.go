package security

import (
	"strings"
	"sync"
	"testing"
)

var (
	benchmarkReadonlyResult string
	benchmarkReadonlyPool   = sync.Pool{
		New: func() any {
			return new(strings.Builder)
		},
	}
)

func BenchmarkStripSQLLiteralsAndComments(b *testing.B) {
	queries := []struct {
		name string
		sql  string
	}{
		{
			name: "short",
			sql:  `SELECT id, name FROM customers WHERE status = 'active' AND note <> 'DROP TABLE ignored'`,
		},
		{
			name: "comments",
			sql:  "SELECT id, total FROM orders -- DELETE is inside a comment\nWHERE status = 'paid' /* UPDATE ignored */ AND total > 100",
		},
		{
			name: "long",
			sql: strings.Repeat(
				`SELECT "customer""name", 'literal with SELECT and DROP', total FROM orders WHERE status = 'paid'; `,
				32,
			),
		},
	}

	for _, query := range queries {
		b.Run(query.name+"/current", func(b *testing.B) {
			b.ReportAllocs()
			var result string
			for b.Loop() {
				result = stripSQLLiteralsAndComments(query.sql)
			}
			benchmarkReadonlyResult = result
		})

		b.Run(query.name+"/pooled_builder", func(b *testing.B) {
			b.ReportAllocs()
			var result string
			for b.Loop() {
				result = benchmarkStripSQLLiteralsAndCommentsWithPool(query.sql)
			}
			benchmarkReadonlyResult = result
		})
	}
}

func benchmarkStripSQLLiteralsAndCommentsWithPool(sql string) string {
	out, ok := benchmarkReadonlyPool.Get().(*strings.Builder)
	if !ok {
		out = &strings.Builder{}
	}
	out.Reset()
	out.Grow(len(sql))
	writeStrippedSQLLiteralsAndComments(sql, out)
	result := out.String()
	benchmarkReadonlyPool.Put(out)
	return result
}
