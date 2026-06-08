// Package db provides database connection pool management.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver registration
)

// Config holds database connection parameters.
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
	}
}

// NewPool creates a configured, OTel-instrumented database/sql connection pool
// for the internal metadata database. Each query becomes a span (with SQL text)
// under the active trace so DB time is visible in distributed traces.
func NewPool(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := OpenInstrumented("pgx", cfg.DSN, "postgresql", "biqly-postgresql", true)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			_ = closeErr
		}
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}
