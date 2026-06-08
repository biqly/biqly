package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	clickhouseMaxOpenConns = 10

	// DefaultConnMaxLifetime caps how long any connection lives before being
	// recycled. Prevents stale TCP sessions and forces pool refresh across
	// upstream DB restarts / DNS changes. Exported so the metadata DB pool
	// can share the same default.
	DefaultConnMaxLifetime = 30 * time.Minute
	// DefaultConnMaxIdleTime closes connections that have been idle too long
	// so the pool shrinks during quiet periods.
	DefaultConnMaxIdleTime = 10 * time.Minute
)

// PoolLimits configures sql.DB pool sizes for a datasource driver.
type PoolLimits struct {
	MaxOpen int
	MaxIdle int
}

// DefaultPoolLimits matches PostgreSQL, MySQL, and SQL Server defaults.
func DefaultPoolLimits() PoolLimits {
	return PoolLimits{MaxOpen: defaultMaxOpenConns, MaxIdle: defaultMaxIdleConns}
}

// ClickHousePoolLimits uses a smaller open-connections cap suited to ClickHouse client usage.
func ClickHousePoolLimits() PoolLimits {
	return PoolLimits{MaxOpen: clickhouseMaxOpenConns, MaxIdle: defaultMaxIdleConns}
}

// Ping opens a short-lived *sql.DB, pings once, and closes it.
func Ping(ctx context.Context, driverName, dsn string) error {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("failed to open %s connection: %w", driverName, err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.ErrorContext(ctx, "failed to close connection after ping", "driver", driverName, "error", closeErr)
		}
	}()
	return db.PingContext(ctx)
}

// OpenPool opens a pooled, OTel-instrumented *sql.DB and applies MaxOpen/MaxIdle
// plus the package-default ConnMaxLifetime/ConnMaxIdleTime. ctx is reserved for
// future checks. SQL text is suppressed in spans (recordStatement=false) because
// these connect to user-supplied data sources whose queries may embed business
// data — only timing and operation kind are recorded.
func OpenPool(ctx context.Context, driverName, dsn string, limits PoolLimits) (*sql.DB, error) {
	_ = ctx
	system := dbSystemForDriver(driverName)
	db, err := platformdb.OpenInstrumented(driverName, dsn, system, "datasource:"+system, false)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s connection: %w", driverName, err)
	}
	db.SetMaxOpenConns(limits.MaxOpen)
	db.SetMaxIdleConns(limits.MaxIdle)
	db.SetConnMaxLifetime(DefaultConnMaxLifetime)
	db.SetConnMaxIdleTime(DefaultConnMaxIdleTime)
	return db, nil
}

// dbSystemForDriver maps a registered sql driver name to the OTel db.system
// value. Unknown drivers fall back to the driver name itself.
func dbSystemForDriver(driverName string) string {
	switch driverName {
	case "pgx", "postgres", "postgresql":
		return "postgresql"
	case "mysql":
		return "mysql"
	case "clickhouse":
		return "clickhouse"
	case "sqlserver", "mssql":
		return "mssql"
	default:
		return driverName
	}
}
