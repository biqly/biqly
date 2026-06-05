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
			for range b.N {
				result = stripSQLLiteralsAndComments(query.sql)
			}
			benchmarkReadonlyResult = result
		})

		b.Run(query.name+"/pooled_builder", func(b *testing.B) {
			b.ReportAllocs()
			var result string
			for range b.N {
				result = benchmarkStripSQLLiteralsAndCommentsWithPool(query.sql)
			}
			benchmarkReadonlyResult = result
		})
	}
}

func benchmarkStripSQLLiteralsAndCommentsWithPool(sql string) string {
	out := benchmarkReadonlyPool.Get().(*strings.Builder)
	out.Reset()
	out.Grow(len(sql))
	defer benchmarkReadonlyPool.Put(out)

	i := 0
	n := len(sql)
	for i < n {
		c := sql[i]

		if c == '-' && i+1 < n && sql[i+1] == '-' {
			for i < n && sql[i] != '\n' {
				i++
			}
			continue
		}

		if c == '/' && i+1 < n && sql[i+1] == '*' {
			i += 2
			for i+1 < n && (sql[i] != '*' || sql[i+1] != '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			continue
		}

		if c == '\'' {
			out.WriteByte('\'')
			i++
			for i < n {
				if sql[i] == '\'' {
					if i+1 < n && sql[i+1] == '\'' {
						i += 2
						continue
					}
					out.WriteByte('\'')
					i++
					break
				}
				i++
			}
			continue
		}

		if c == '"' {
			out.WriteByte('"')
			i++
			for i < n {
				if sql[i] == '"' {
					if i+1 < n && sql[i+1] == '"' {
						i += 2
						continue
					}
					out.WriteByte('"')
					i++
					break
				}
				i++
			}
			continue
		}

		out.WriteByte(c)
		i++
	}
	return out.String()
}
