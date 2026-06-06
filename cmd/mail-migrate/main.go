// Package main runs database migrations for the mail database.
package main

import (
	"os"

	"github.com/biqly/biqly/internal/dbmigrate"
)

func main() {
	os.Exit(dbmigrate.RunCLI(dbmigrate.CLIConfig{
		DSNEnv:     "BI_MAIL_DB_DSN",
		DSNUsage:   "Database DSN for mail DB",
		DefaultDir: "migrations/mail",
		Usage:      "Usage: mail-migrate [up|down]",
	}))
}
