package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/uptrace/bun"
	bunmigrate "github.com/uptrace/bun/migrate"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrationFiles embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrationFiles embed.FS

func applyMigrations(db *bun.DB, driver string) error {
	migrations := bunmigrate.NewMigrations()
	root, err := migrationRoot(driver)
	if err != nil {
		return err
	}
	if err := migrations.Discover(root); err != nil {
		return fmt.Errorf("discover migrations: %w", err)
	}

	migrator := bunmigrate.NewMigrator(
		db,
		migrations,
		bunmigrate.WithMarkAppliedOnSuccess(true),
	)
	ctx := context.Background()
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}
	if _, err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func migrationRoot(driver string) (fs.FS, error) {
	switch driver {
	case DriverSQLite:
		return fs.Sub(sqliteMigrationFiles, "migrations/sqlite")
	case DriverPostgres:
		return fs.Sub(postgresMigrationFiles, "migrations/postgres")
	default:
		return nil, fmt.Errorf("no migrations for driver %q", driver)
	}
}
