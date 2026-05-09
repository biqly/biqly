package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("BI_METADATA_DB_DSN"), "Database DSN")
	dir := flag.String("dir", "migrations", "Migrations directory")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("BI_METADATA_DB_DSN is required")
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: migrate [up|down|force <version>]")
		os.Exit(1)
	}

	m, err := migrate.New("file://"+*dir, *dsn)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "force":
		if len(args) < 2 {
			log.Fatal("force requires a version argument")
		}
		var version int
		fmt.Sscanf(args[1], "%d", &version)
		err = m.Force(version)
	default:
		log.Fatalf("Unknown command: %s", args[0])
	}

	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", err)
	}

	if err == migrate.ErrNoChange {
		fmt.Println("No changes to apply")
	} else {
		fmt.Printf("Migration %s completed successfully\n", args[0])
	}
}
