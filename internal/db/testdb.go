package db

import (
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	supabasesql "github.com/SouichiroTsujimoto/unagi/supabase"
	"github.com/uptrace/bun"
)

// TestDSN returns the Postgres DSN used by integration tests.
// Override with UNIGO_TEST_DB_DSN or UNIGO_DB_DSN. Default is local Supabase.
func TestDSN() string {
	for _, key := range []string{"UNIGO_TEST_DB_DSN", "UNIGO_DB_DSN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "postgresql://postgres:postgres@127.0.0.1:54322/postgres"
}

const testAdvisoryLockKey int64 = 748291003

// OpenTest opens a Postgres DB with supabase/migrations applied (except
// Storage, which needs the storage schema). Skips when unreachable.
// Uses a Postgres advisory lock so parallel packages do not TRUNCATE concurrently.
func OpenTest(t *testing.T) *bun.DB {
	t.Helper()
	database, err := Open(Config{Driver: DriverPostgres, DSN: TestDSN()})
	if err != nil {
		t.Skipf("postgres unavailable (%v); run: supabase start", err)
	}
	if _, err := database.Exec("SELECT pg_advisory_lock(?)", testAdvisoryLockKey); err != nil {
		_ = database.Close()
		t.Fatalf("advisory lock: %v", err)
	}
	applyTestSchema(t, database)
	resetTestData(t, database)
	t.Cleanup(func() {
		resetTestData(t, database)
		_, _ = database.Exec("SELECT pg_advisory_unlock(?)", testAdvisoryLockKey)
		_ = database.Close()
	})
	return database
}

func applyTestSchema(t *testing.T, database *bun.DB) {
	t.Helper()
	names, err := fs.Glob(supabasesql.Migrations, "migrations/*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := fs.ReadFile(supabasesql.Migrations, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(body), "storage.buckets") {
			continue
		}
		if _, err := database.Exec(string(body)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
}

func resetTestData(t *testing.T, database *bun.DB) {
	t.Helper()
	_, err := database.Exec(`TRUNCATE TABLE
		article_comments,
		article_stickers,
		article_topics,
		article_revisions,
		articles,
		topics,
		media,
		link_card_cache,
		accounts
	RESTART IDENTITY CASCADE`)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return
		}
		t.Fatalf("reset test data: %v", err)
	}
}
