package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseAccessTokenAndAdminAllowlist(t *testing.T) {
	secret := "super-secret-jwt-token-with-at-least-32-characters-long"
	adminID := "11111111-1111-1111-1111-111111111111"
	a, err := New(Config{
		SupabaseURL:  "http://127.0.0.1:54321",
		AnonKey:      "anon",
		JWTSecret:    secret,
		AdminUserIDs: []string{adminID},
		SiteBaseURL:  "http://localhost:8080",
	})
	if err != nil {
		t.Fatal(err)
	}

	tok := signTestJWT(t, secret, adminID, map[string]any{
		"user_name":  "wuhu",
		"full_name":  "Wuhu",
		"avatar_url": "https://example.com/a.png",
	})
	user, err := a.ParseAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if !user.IsAdmin || user.Username != "wuhu" || user.DisplayName != "Wuhu" {
		t.Fatalf("user=%+v", user)
	}

	reader := signTestJWT(t, secret, "22222222-2222-2222-2222-222222222222", map[string]any{
		"user_name": "reader",
	})
	u2, err := a.ParseAccessToken(reader)
	if err != nil {
		t.Fatal(err)
	}
	if u2.IsAdmin {
		t.Fatal("reader should not be admin")
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
		AnonKey:        "anon",
		JWTSecret:      "super-secret-jwt-token-with-at-least-32-characters-long",
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

func signTestJWT(t *testing.T, secret, sub string, meta map[string]any) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":           sub,
		"role":          "authenticated",
		"aud":           "authenticated",
		"user_metadata": meta,
		"exp":           time.Now().Add(time.Hour).Unix(),
		"iat":           time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
