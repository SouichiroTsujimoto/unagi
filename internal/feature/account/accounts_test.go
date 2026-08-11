package account_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/account"
	"github.com/SouichiroTsujimoto/unagi/internal/db"
)

func TestAccountsLifecycle(t *testing.T) {
	database, err := db.Open(db.Config{
		Driver: db.DriverSQLite,
		DSN:    filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	ctx := context.Background()
	accounts := account.New(database)

	created, err := accounts.Create(ctx, " Hello@Example.com ")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if created.Email != "hello@example.com" {
		t.Fatalf("created email = %q, want %q", created.Email, "hello@example.com")
	}

	if _, err := accounts.Create(ctx, "hello@example.com"); !errors.Is(err, account.ErrEmailExists) {
		t.Fatalf("duplicate create error = %v, want %v", err, account.ErrEmailExists)
	}

	listed, err := accounts.List(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed accounts = %#v, want created account", listed)
	}

	if err := accounts.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if err := accounts.Delete(ctx, created.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("second delete error = %v, want %v", err, account.ErrNotFound)
	}
}
