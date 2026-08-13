package db

import (
	"testing"
)

func TestOpenPostgresHasSchema(t *testing.T) {
	database := OpenTest(t)

	var n int
	if err := database.NewSelect().
		ColumnExpr("count(*)").
		TableExpr("articles").
		Scan(t.Context(), &n); err != nil {
		t.Fatalf("query articles: %v", err)
	}
	if n != 0 {
		t.Fatalf("articles count = %d, want 0", n)
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
	if got.Driver != DriverPostgres {
		t.Fatalf("driver = %q", got.Driver)
	}
	if got.DSN == "" {
		t.Fatal("expected default DSN")
	}
	if got.Label() == "" {
		t.Fatal("expected Label")
	}
}
