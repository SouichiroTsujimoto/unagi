// Package db opens a Bun DB. Schema lives in supabase/migrations and is
// applied by the Supabase CLI and GitHub integration, not at process start.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config holds the Postgres connection string.
type Config struct {
	DSN string
}

// WithDefaults fills an empty DSN for local Supabase development.
func (c Config) WithDefaults() Config {
	if strings.TrimSpace(c.DSN) == "" {
		c.DSN = "postgresql://postgres:postgres@127.0.0.1:54322/postgres"
	}
	return c
}

// Label is a short value for the listen banner "db" field.
func (c Config) Label() string {
	c = c.WithDefaults()
	if strings.TrimSpace(c.DSN) != "" {
		return redactDSN(c.DSN)
	}
	return "postgres"
}

func redactDSN(dsn string) string {
	// Avoid printing passwords in the listen banner.
	if i := strings.Index(dsn, "@"); i > 0 {
		if scheme := strings.Index(dsn, "://"); scheme >= 0 && scheme < i {
			user := dsn[scheme+3 : i]
			if colon := strings.Index(user, ":"); colon >= 0 {
				return dsn[:scheme+3] + user[:colon] + ":***" + dsn[i:]
			}
		}
	}
	return dsn
}

// Open connects with pgx and builds Bun. It does not apply SQL migrations.
func Open(cfg Config) (*bun.DB, error) {
	cfg = cfg.WithDefaults()
	return openPostgres(cfg.DSN)
}

func openPostgres(dsn string) (*bun.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("postgres dsn is required")
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return bun.NewDB(sqlDB, pgdialect.New()), nil
}
