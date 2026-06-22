package index

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDir is the path within migrationsFS that holds the SQL files.
const migrationsDir = "migrations"

// dialect is the goose SQL dialect. The modernc.org/sqlite driver is
// wire-compatible with the sqlite3 dialect.
const dialect = "sqlite3"

func configureGoose() error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(log.New(io.Discard, "", 0))
	return goose.SetDialect(dialect)
}

// Migrate applies all pending up migrations to db.
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := configureGoose(); err != nil {
		return fmt.Errorf("index: configure migrations: %w", err)
	}
	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("index: apply migrations: %w", err)
	}
	return nil
}

// MigrateDown rolls back the most recent migration. Intended for tooling/tests.
func MigrateDown(ctx context.Context, db *sql.DB) error {
	if err := configureGoose(); err != nil {
		return fmt.Errorf("index: configure migrations: %w", err)
	}
	return goose.DownContext(ctx, db, migrationsDir)
}

// MigrationStatus prints the current migration status to goose's logger.
func MigrationStatus(ctx context.Context, db *sql.DB) error {
	if err := configureGoose(); err != nil {
		return err
	}
	goose.SetLogger(log.New(os.Stdout, "", 0))
	return goose.StatusContext(ctx, db, migrationsDir)
}
