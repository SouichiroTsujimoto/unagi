package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
)

func TestLoadConfigBuildsRuntimeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unigo.toml")
	if err := os.WriteFile(path, []byte(`
[site]
name = "unagi"
description = "ignored"
base_url = "http://localhost:8080"

[db]
dsn = "postgresql://toml"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UNIGO_CONFIG", path)
	t.Setenv("UNIGO_DB_DSN", "postgresql://runtime")
	t.Setenv("UNIGO_SITE_BASE_URL", "https://example.com/")
	t.Setenv("UNIGO_SUPABASE_URL", "https://project.supabase.co/")
	t.Setenv("UNIGO_SUPABASE_PUBLISHABLE_KEY", "sb_publishable_test")
	t.Setenv("UNIGO_SUPABASE_SECRET_KEY", "sb_secret_test")
	t.Setenv("UNIGO_ADMIN_USER_IDS", "one, two")
	t.Setenv("UNIGO_CONTENT_SYNC_SECRET", "sync-secret")
	t.Setenv("UNIGO_SITE_NAME", "ignored")
	t.Setenv("UNIGO_SITE_DESCRIPTION", "ignored")
	t.Setenv("UNIGO_MEDIA_BUCKET", "ignored")
	t.Setenv("UNIGO_ALLOWED_ORIGINS", "https://ignored.example")
	t.Setenv("UNIGO_SESSION_TTL", "1h")
	t.Setenv("UNIGO_SECURE_COOKIES", "false")
	t.Setenv("UNIGO_DEV_MODE", "true")

	cfg := LoadConfig("1.2.3")
	if cfg.DB.DSN != "postgresql://runtime" {
		t.Fatalf("dsn=%q", cfg.DB.DSN)
	}
	if cfg.Site.Name != "unagi" || cfg.Site.Description != siteDescription {
		t.Fatalf("site=%+v", cfg.Site)
	}
	if cfg.Site.BaseURL != "https://example.com" || cfg.Site.MediaOrigin != "https://project.supabase.co" {
		t.Fatalf("site URLs=%+v", cfg.Site)
	}
	if cfg.MediaPublicBase != "https://project.supabase.co/storage/v1/object/public/"+media.Bucket {
		t.Fatalf("media base=%q", cfg.MediaPublicBase)
	}
	if !cfg.Auth.SecureCookies || cfg.Auth.SessionTTL != 7*24*time.Hour {
		t.Fatalf("auth=%+v", cfg.Auth)
	}
	if len(cfg.Auth.AllowedOrigins) != 1 || cfg.Auth.AllowedOrigins[0] != "https://example.com" {
		t.Fatalf("origins=%v", cfg.Auth.AllowedOrigins)
	}
	if len(cfg.Auth.AdminUserIDs) != 2 || cfg.ContentSyncSecret != "sync-secret" {
		t.Fatalf("admins=%v sync=%q", cfg.Auth.AdminUserIDs, cfg.ContentSyncSecret)
	}
	if cfg.DevAdminBypass {
		t.Fatal("development admin bypass enabled for production URL")
	}
}

func TestDevelopmentAdminBypassRequiresDevModeAndLoopbackHTTP(t *testing.T) {
	t.Setenv("UNIGO_DEV_MODE", "true")
	for _, baseURL := range []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
	} {
		if !developmentAdminBypass(baseURL) {
			t.Fatalf("bypass disabled for %q", baseURL)
		}
	}
	for _, baseURL := range []string{
		"https://localhost:8080",
		"https://example.com",
		"http://example.com",
	} {
		if developmentAdminBypass(baseURL) {
			t.Fatalf("bypass enabled for %q", baseURL)
		}
	}

	t.Setenv("UNIGO_DEV_MODE", "false")
	if developmentAdminBypass("http://localhost:8080") {
		t.Fatal("bypass enabled outside development mode")
	}
}

func TestAddressFromPort(t *testing.T) {
	t.Setenv("PORT", "3000")
	if got := AddressFromPort(":8080"); got != ":3000" {
		t.Fatalf("address=%q", got)
	}
	t.Setenv("PORT", ":4000")
	if got := AddressFromPort(":8080"); got != ":4000" {
		t.Fatalf("address=%q", got)
	}
}
