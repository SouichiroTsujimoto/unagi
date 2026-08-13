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
	var gotAPIKey, gotAuth, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("apikey")
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/object/upload/sign/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"/object/upload/sign/images/dot.png?token=abc","token":"abc"}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	store, err := media.NewSupabaseStore(srv.URL, "images", "sb_secret_test", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	signedURL, token, err := store.SignUpload(context.Background(), "dot.png", "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != "sb_secret_test" {
		t.Fatalf("apikey=%q", gotAPIKey)
	}
	if gotAuth != "" {
		t.Fatalf("authorization should be empty, got %q", gotAuth)
	}
	if gotMethod != http.MethodPost || !strings.HasSuffix(gotPath, "/object/upload/sign/images/dot.png") {
		t.Fatalf("sign request %s %s", gotMethod, gotPath)
	}
	if token != "abc" || signedURL != srv.URL+"/storage/v1/object/upload/sign/images/dot.png?token=abc" {
		t.Fatalf("signedURL=%q token=%q", signedURL, token)
	}

	exists, err := store.Exists(context.Background(), "dot.png")
	if err != nil {
		t.Fatal(err)
	}
	if !exists || gotMethod != http.MethodHead || !strings.HasSuffix(gotPath, "/object/images/dot.png") {
		t.Fatalf("exists=%v request=%s %s", exists, gotMethod, gotPath)
	}
	if gotAuth != "" {
		t.Fatalf("head authorization should be empty, got %q", gotAuth)
	}
}
