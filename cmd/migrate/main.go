// Package main runs database migrations.
package main

import (
	"errors"
	"flag"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("BI_METADATA_DB_DSN"), "Database DSN")
	dir := flag.String("dir", "migrations", "Migrations directory")
	flag.Parse()

	if *dsn == "" {
		slog.Error("BI_METADATA_DB_DSN is required")
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		slog.Error("Usage: migrate [up|down|force <version>]")
		os.Exit(1)
	}

	m, err := migrate.New("file://"+*dir, *dsn)
	if err != nil {
		slog.Error("failed to create migrate instance", "error", err)
		os.Exit(1)
	}

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "force":
		if len(args) < 2 {
			slog.Error("force requires a version argument")
			os.Exit(1)
		}
		version, convErr := strconv.Atoi(args[1])
		if convErr != nil {
			slog.Error("invalid version", "error", convErr)
			os.Exit(1)
		}
		err = m.Force(version)
	default:
		slog.Error("unknown command", "command", args[0])
		os.Exit(1)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("no changes to apply")
	} else {
		slog.Info("migration completed", "command", args[0])
	}
}
