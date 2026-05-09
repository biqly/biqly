package postgres

import "github.com/biqly/biqly/internal/dialect"

// Ensure Driver implements dialect.Dialect at compile time.
var _ dialect.Dialect = dialect.PostgresDialect{}
