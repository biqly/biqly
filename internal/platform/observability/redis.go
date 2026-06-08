package observability

import (
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
)

// InstrumentRedis enables OpenTelemetry tracing on a go-redis client. Each
// command becomes a client span under the active trace, and peerService names
// the cache as a node in Jaeger's service dependency graph. It is best-effort:
// callers log and continue on error rather than failing startup.
func InstrumentRedis(rdb redis.UniversalClient, peerService string) error {
	return redisotel.InstrumentTracing(rdb,
		redisotel.WithAttributes(attribute.String("peer.service", peerService)))
}
