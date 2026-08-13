package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseAccessTokenAndAdminAllowlist(t *testing.T) {
	adminID := "11111111-1111-1111-1111-111111111111"
	srv, sign := newJWKSServer(t)
	a, err := New(Config{
		SupabaseURL:    srv.URL,
		PublishableKey: "sb_publishable_test",
		AdminUserIDs:   []string{adminID},
		SiteBaseURL:    "http://localhost:8080",
		HTTPClient:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}

	tok := sign(adminID, map[string]any{
		"user_name":  "wuhu",
		"full_name":  "Wuhu",
		"avatar_url": "https://example.com/a.png",
	})
	user, err := a.ParseAccessToken(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	if !user.IsAdmin || user.Username != "wuhu" || user.DisplayName != "Wuhu" {
		t.Fatalf("user=%+v", user)
	}

	reader := sign("22222222-2222-2222-2222-222222222222", map[string]any{
		"user_name": "reader",
	})
	u2, err := a.ParseAccessToken(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	if u2.IsAdmin {
		t.Fatal("reader should not be admin")
	}
}

func TestParseAccessTokenRejectsHMAC(t *testing.T) {
	srv, _ := newJWKSServer(t)
	a, err := New(Config{
		SupabaseURL:    srv.URL,
		PublishableKey: "sb_publishable_test",
		HTTPClient:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	raw, err := tok.SignedString([]byte("not-a-jwks-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ParseAccessToken(context.Background(), raw); err == nil {
		t.Fatal("expected hmac token to be rejected")
	}
}

func TestSafeReturnTo(t *testing.T) {
	if SafeReturnTo("/articles/hi") != "/articles/hi" {
		t.Fatal("relative path")
	}
	if SafeReturnTo("https://evil") != "/" {
		t.Fatal("absolute rejected")
	}
	if SafeReturnTo("//evil") != "/" {
		t.Fatal("protocol-relative rejected")
	}
}

func TestValidOrigin(t *testing.T) {
	a, err := New(Config{
		SupabaseURL:    "http://127.0.0.1:54321",
		PublishableKey: "sb_publishable_test",
		AllowedOrigins: []string{"http://localhost:8080"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !a.ValidOrigin("http://127.0.0.1:8080") {
		t.Fatal("loopback should match localhost")
	}
	if a.ValidOrigin("https://evil.example") {
		t.Fatal("foreign origin")
	}
}

func TestGotrueJSONSendsPublishableKeyOnApikeyOnly(t *testing.T) {
	var gotAPIKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("apikey")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	a, err := New(Config{
		SupabaseURL:    srv.URL,
		PublishableKey: "sb_publishable_test",
		HTTPClient:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.BeginPasskeyLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != "sb_publishable_test" {
		t.Fatalf("apikey=%q", gotAPIKey)
	}
	if gotAuth != "" {
		t.Fatalf("authorization should be empty, got %q", gotAuth)
	}
}

func newJWKSServer(t *testing.T) (*httptest.Server, func(sub string, meta map[string]any) string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-es256"
	doc, err := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kty": "EC",
			"kid": kid,
			"alg": "ES256",
			"crv": "P-256",
			"x":   base64.RawURLEncoding.EncodeToString(paddedInt(priv.X, 32)),
			"y":   base64.RawURLEncoding.EncodeToString(paddedInt(priv.Y, 32)),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(doc)
	}))
	t.Cleanup(srv.Close)
	sign := func(sub string, meta map[string]any) string {
		t.Helper()
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"sub":           sub,
			"role":          "authenticated",
			"aud":           "authenticated",
			"user_metadata": meta,
			"exp":           time.Now().Add(time.Hour).Unix(),
			"iat":           time.Now().Unix(),
		})
		tok.Header["kid"] = kid
		raw, err := tok.SignedString(priv)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	return srv, sign
}

func paddedInt(n *big.Int, size int) []byte {
	out := make([]byte, size)
	n.FillBytes(out)
	return out
}
