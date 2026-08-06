// Command migrate manages the hashdupes index database schema. It is a thin
// wrapper over the goose-backed migrations embedded in internal/index.
//
// Usage:
//
//	migrate [-db PATH] <up|down|status>
//
// With no -db, the default per-user database path is used.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"hashdupes/internal/appdir"
	"hashdupes/internal/index"
)

func main() {
	dbFlag := flag.String("db", "", "path to the index database (default: per-user app dir)")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	cmd := flag.Arg(0)

	path := *dbFlag
	if path == "" {
		p, err := appdir.DefaultDBPath()
		if err != nil {
			fatal("resolve db path: %v", err)
		}
		path = p
	}

	ctx := context.Background()
	db, err := index.OpenDB(ctx, path)
	if err != nil {
		fatal("open db: %v", err)
	}
	defer db.Close()

	switch cmd {
	case "up":
		if err := index.Migrate(ctx, db); err != nil {
			fatal("migrate up: %v", err)
		}
		fmt.Printf("migrations applied (%s)\n", path)
	case "down":
		if err := index.MigrateDown(ctx, db); err != nil {
			fatal("migrate down: %v", err)
		}
		fmt.Printf("rolled back one migration (%s)\n", path)
	case "status":
		if err := index.MigrationStatus(ctx, db); err != nil {
			fatal("migrate status: %v", err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: migrate [-db PATH] <up|down|status>\n")
	flag.PrintDefaults()
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}
