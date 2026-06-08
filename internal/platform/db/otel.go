package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
)

// skipMigrationSpans drops spans for golang-migrate's bookkeeping queries
// against schema_migrations. These run at startup, are not request work, and the
// initial "relation does not exist" probe would otherwise surface as a noisy
// error span.
func skipMigrationSpans(_ context.Context, _ otelsql.Method, query string, _ []driver.NamedValue) bool {
	return !strings.Contains(strings.ToLower(query), "schema_migrations")
}

// OTelOptions returns the standard otelsql instrumentation options for a pool.
//
// system is the db.system span attribute (postgresql, mysql, clickhouse, mssql).
// peerService, when non-empty, is set as peer.service so the database appears as
// a node in Jaeger's service dependency graph (it has no server-side spans of
// its own). recordStatement controls whether the SQL text is captured as
// db.statement: pass true for internal databases (metadata, auth, mail) where
// the schema is ours, and false for user-supplied data sources so query text —
// which may embed business data — never lands in exported spans.
func OTelOptions(system, peerService string, recordStatement bool) []otelsql.Option {
	attrs := []attribute.KeyValue{attribute.String("db.system", system)}
	if peerService != "" {
		attrs = append(attrs, attribute.String("peer.service", peerService))
	}
	return []otelsql.Option{
		otelsql.WithAttributes(attrs...),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			DisableQuery:         !recordStatement,
			OmitConnResetSession: true,
			OmitRows:             true,
			SpanFilter:           skipMigrationSpans,
		}),
	}
}

// OpenInstrumented opens an otelsql-instrumented *sql.DB. Each query, exec, and
// ping becomes a span under the active trace, so DB time is visible alongside
// application spans. See OTelOptions for the peerService / recordStatement
// guidance.
func OpenInstrumented(driverName, dsn, system, peerService string, recordStatement bool) (*sql.DB, error) {
	return otelsql.Open(driverName, dsn, OTelOptions(system, peerService, recordStatement)...)
}
