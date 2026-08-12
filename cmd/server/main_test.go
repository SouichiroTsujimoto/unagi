package main

import (
	"testing"

	"github.com/SouichiroTsujimoto/unagi/internal/config"
	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
)

func TestLayoutSiteEnvOverrides(t *testing.T) {
	t.Setenv("UNIGO_SITE_NAME", "prod-unagi")
	t.Setenv("UNIGO_SITE_DESCRIPTION", "production notes")
	t.Setenv("UNIGO_SITE_BASE_URL", "https://example.com/")
	t.Setenv("UNIGO_SITE_AUTHOR", "souic")

	site := layoutSite(config.File{
		Site: config.Site{
			Name:        "unagi",
			Description: "local",
			BaseURL:     "http://localhost:8080",
			Author:      "",
		},
	})
	if site.Name != "prod-unagi" {
		t.Fatalf("name=%q", site.Name)
	}
	if site.Description != "production notes" {
		t.Fatalf("description=%q", site.Description)
	}
	if site.BaseURL != "https://example.com" {
		t.Fatalf("baseURL=%q", site.BaseURL)
	}
	if site.Author != "souic" {
		t.Fatalf("author=%q", site.Author)
	}
}

func TestAuthConfigFromEnv(t *testing.T) {
	t.Setenv("UNIGO_SITE_BASE_URL", "https://example.com")
	t.Setenv("UNIGO_SUPABASE_URL", "https://abc.supabase.co")
	t.Setenv("UNIGO_SUPABASE_ANON_KEY", "anon")
	t.Setenv("UNIGO_SUPABASE_JWT_SECRET", "super-secret-jwt-token-with-at-least-32-characters-long")
	t.Setenv("UNIGO_ADMIN_USER_IDS", "11111111-1111-1111-1111-111111111111")
	t.Setenv("UNIGO_SECURE_COOKIES", "")
	t.Setenv("UNIGO_ALLOWED_ORIGINS", "")

	site := layoutSite(config.Default())
	cfg := authConfig(site)
	if cfg.SupabaseURL != "https://abc.supabase.co" {
		t.Fatalf("url=%q", cfg.SupabaseURL)
	}
	if !cfg.SecureCookies {
		t.Fatal("expected secure cookies from https base url")
	}
	if len(cfg.AdminUserIDs) != 1 {
		t.Fatalf("admins=%v", cfg.AdminUserIDs)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://example.com" {
		t.Fatalf("origins=%v", cfg.AllowedOrigins)
	}
	if site.AbsoluteURL("/feed.xml") != "https://example.com/feed.xml" {
		t.Fatalf("feed=%q", site.AbsoluteURL("/feed.xml"))
	}
	_ = featureauth.CallbackPath
}
