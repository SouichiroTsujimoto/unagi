package media_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
)

func TestSupabaseStoreSendsSecretKeyOnApikeyOnly(t *testing.T) {
	var gotAPIKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("apikey")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	store, err := media.NewSupabaseStore(srv.URL, "images", "sb_secret_test", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "dot.png", strings.NewReader("png"), "image/png", 3); err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != "sb_secret_test" {
		t.Fatalf("apikey=%q", gotAPIKey)
	}
	if gotAuth != "" {
		t.Fatalf("authorization should be empty, got %q", gotAuth)
	}

	if err := store.Delete(context.Background(), "dot.png"); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Fatalf("delete authorization should be empty, got %q", gotAuth)
	}
}
