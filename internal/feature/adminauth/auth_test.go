package adminauth_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/adminauth"
)

func TestBootstrapAndSession(t *testing.T) {
	database, err := db.Open(db.Config{
		Driver: db.DriverSQLite,
		DSN:    filepath.Join(t.TempDir(), "auth.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	token := "test-bootstrap-token"
	auth, err := adminauth.New(database, adminauth.Config{
		RPDisplayName:      "unagi",
		RPID:               "localhost",
		RPOrigins:          []string{"http://localhost:8080"},
		BootstrapTokenHash: adminauth.HashToken(token),
		SessionTTL:         time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	need, err := auth.NeedsBootstrap(t.Context())
	if err != nil || !need {
		t.Fatalf("need=%v err=%v", need, err)
	}
	if err := auth.VerifyBootstrapToken(token); err != nil {
		t.Fatal(err)
	}
	if err := auth.VerifyBootstrapToken("nope"); err != adminauth.ErrInvalidToken {
		t.Fatalf("got %v", err)
	}

	sess, raw, err := auth.CreateSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	got, err := auth.LookupSession(t.Context(), raw)
	if err != nil || got.CSRFToken != sess.CSRFToken {
		t.Fatalf("lookup=%+v err=%v", got, err)
	}
	if !auth.ValidOrigin("http://localhost:8080") || auth.ValidOrigin("https://evil.example") {
		t.Fatal("origin check failed")
	}
}
