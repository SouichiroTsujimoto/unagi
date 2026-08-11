// Package db opens a Bun DB and applies SQL migrations.
// Feature packages depend on *bun.DB; driver/dialect selection stays here.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// Config selects the database backend.
type Config struct {
	Driver string // sqlite | postgres
	DSN    string // sqlite path or postgres URL / Cloud SQL DSN
}

// WithDefaults fills empty driver/DSN for local SQLite development.
func (c Config) WithDefaults() Config {
	if strings.TrimSpace(c.Driver) == "" {
		c.Driver = DriverSQLite
	}
	c.Driver = strings.ToLower(strings.TrimSpace(c.Driver))
	if strings.TrimSpace(c.DSN) == "" {
		if c.Driver == DriverSQLite {
			c.DSN = "app.db"
		}
	}
	return c
}

// Label is a short value for the listen banner "db" field.
func (c Config) Label() string {
	c = c.WithDefaults()
	if strings.TrimSpace(c.DSN) != "" {
		return c.DSN
	}
	return c.Driver
}

// Open connects with the configured driver, builds Bun, and migrates.
func Open(cfg Config) (*bun.DB, error) {
	cfg = cfg.WithDefaults()

	switch cfg.Driver {
	case DriverSQLite:
		return openSQLite(cfg.DSN)
	case DriverPostgres:
		return openPostgres(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported db driver %q (use sqlite or postgres)", cfg.Driver)
	}
}

func openSQLite(dsn string) (*bun.DB, error) {
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	db := bun.NewDB(sqlDB, sqlitedialect.New())
	if err := applyMigrations(db, DriverSQLite); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func openPostgres(dsn string) (*bun.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("postgres dsn is required")
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db := bun.NewDB(sqlDB, pgdialect.New())
	if err := applyMigrations(db, DriverPostgres); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
