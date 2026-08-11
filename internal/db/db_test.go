package db

import (
	"path/filepath"
	"testing"
)

func TestOpenSQLiteAppliesMigrations(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(Config{Driver: DriverSQLite, DSN: path})
	if err != nil {
		t.Fatalf("Open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	var n int
	if err := database.NewSelect().
		ColumnExpr("count(*)").
		TableExpr("accounts").
		Scan(t.Context(), &n); err != nil {
		t.Fatalf("query accounts: %v", err)
	}
	if n != 0 {
		t.Fatalf("accounts count = %d, want 0", n)
	}
}

func TestOpenUnsupportedDriver(t *testing.T) {
	t.Parallel()

	_, err := Open(Config{Driver: "mysql", DSN: "x"})
	if err == nil {
		t.Fatal("expected error for unsupported driver")
	}
}

func TestConfigWithDefaults(t *testing.T) {
	t.Parallel()

	got := Config{}.WithDefaults()
	if got.Driver != DriverSQLite || got.DSN != "app.db" {
		t.Fatalf("defaults = %+v", got)
	}
	if got.Label() != "app.db" {
		t.Fatalf("Label = %q", got.Label())
	}
}
