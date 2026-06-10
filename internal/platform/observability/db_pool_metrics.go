package observability

import (
	"database/sql"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DBPoolSnapshot is a point-in-time view of sql.DB pool statistics.
type DBPoolSnapshot struct {
	OpenConnections int
	InUse           int
	Idle            int
	WaitCount       int64
	WaitDuration    time.Duration
}

// DBPoolStatsProvider returns current pool statistics on each Prometheus scrape.
type DBPoolStatsProvider func() DBPoolSnapshot

var registeredDBPools sync.Map

// SnapshotFromDB reads sql.DB.Stats() into a DBPoolSnapshot.
func SnapshotFromDB(db *sql.DB) DBPoolSnapshot {
	if db == nil {
		return DBPoolSnapshot{}
	}
	s := db.Stats()
	return DBPoolSnapshot{
		OpenConnections: s.OpenConnections,
		InUse:           s.InUse,
		Idle:            s.Idle,
		WaitCount:       s.WaitCount,
		WaitDuration:    s.WaitDuration,
	}
}

// DBPoolStatsFromDB returns a provider that reads live stats from db on scrape.
func DBPoolStatsFromDB(db *sql.DB) DBPoolStatsProvider {
	return func() DBPoolSnapshot { return SnapshotFromDB(db) }
}

// RegisterDBPoolMetrics exposes pool gauges/counters for one named pool (metadata, auth, datasource).
func RegisterDBPoolMetrics(reg prometheus.Registerer, poolName string, provider DBPoolStatsProvider) {
	if reg == nil || provider == nil || poolName == "" {
		return
	}
	if _, loaded := registeredDBPools.LoadOrStore(poolName, true); loaded {
		return
	}
	labels := prometheus.Labels{"pool": poolName}
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "biqly_db_pool_open_connections",
		Help:        "Current open connections in the database pool.",
		ConstLabels: labels,
	}, func() float64 { return float64(provider().OpenConnections) }))
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "biqly_db_pool_in_use",
		Help:        "Current in-use connections in the database pool.",
		ConstLabels: labels,
	}, func() float64 { return float64(provider().InUse) }))
	reg.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "biqly_db_pool_wait_count_total",
		Help:        "Cumulative count of connections waited for in the database pool.",
		ConstLabels: labels,
	}, func() float64 { return float64(provider().WaitCount) }))
	reg.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name:        "biqly_db_pool_wait_duration_seconds_total",
		Help:        "Cumulative time spent waiting for a database pool connection.",
		ConstLabels: labels,
	}, func() float64 { return provider().WaitDuration.Seconds() }))
}
