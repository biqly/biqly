// Package main runs database migrations for the auth database.
package main

import (
	"os"

	"github.com/biqly/biqly/internal/dbmigrate"
)

func main() {
	os.Exit(dbmigrate.RunCLI(dbmigrate.CLIConfig{
		DSNEnv:     "BI_AUTH_DB_DSN",
		DSNUsage:   "Database DSN for auth DB",
		DefaultDir: "migrations/auth",
		Usage:      "Usage: auth-migrate [up|down]",
	}))
}
