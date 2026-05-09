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
	DSN            string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:            dsn,
		MaxOpenConns:   25,
		MaxIdleConns:   5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
	}
}

// NewPool creates a configured database/sql connection pool.
func NewPool(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// Close safely closes a database connection pool.
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
