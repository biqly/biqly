package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	clickhouseMaxOpenConns = 10
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
			slog.Error("failed to close connection after ping", "driver", driverName, "error", closeErr)
		}
	}()
	return db.PingContext(ctx)
}

// OpenPool opens a pooled *sql.DB and applies MaxOpen/MaxIdle. ctx is reserved for future checks.
func OpenPool(ctx context.Context, driverName, dsn string, limits PoolLimits) (*sql.DB, error) {
	_ = ctx
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s connection: %w", driverName, err)
	}
	db.SetMaxOpenConns(limits.MaxOpen)
	db.SetMaxIdleConns(limits.MaxIdle)
	return db, nil
}
