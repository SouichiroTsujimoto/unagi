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
	var gotAPIKey, gotAuth, gotPath, gotMethod, gotCacheControl string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("apikey")
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/object/") && !strings.Contains(r.URL.Path, "/upload/sign/") {
			gotCacheControl = r.Header.Get("Cache-Control")
		}
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

	if err := store.Put(context.Background(), "dot.png", strings.NewReader("png"), "image/png", 3); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Fatalf("put authorization should be empty, got %q", gotAuth)
	}
	if gotCacheControl != media.PublicCacheControl {
		t.Fatalf("put cache-control=%q", gotCacheControl)
	}

	if err := store.Delete(context.Background(), "dot.png"); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Fatalf("delete authorization should be empty, got %q", gotAuth)
	}
}
