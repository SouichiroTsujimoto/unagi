package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPathMissingUsesDefault(t *testing.T) {
	t.Parallel()

	got := LoadPath(filepath.Join(t.TempDir(), "missing.toml"))
	if !got.Logo() || !got.TUIEnabled() {
		t.Fatalf("defaults: logo=%v tui=%v", got.Logo(), got.TUIEnabled())
	}
	if len(got.Banner.Fields) == 0 {
		t.Fatal("expected default banner fields")
	}
}

func TestLoadPathReadsValues(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "unigo.toml")
	content := `
[dev]
logo = false
tui = false

[banner]
image = "brand/logo.png"
fields = ["version", "", "url"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadPath(path)
	if got.Logo() {
		t.Fatal("expected logo=false")
	}
	if got.TUIEnabled() {
		t.Fatal("expected tui=false")
	}
	if got.Banner.Image != "brand/logo.png" {
		t.Fatalf("image=%q", got.Banner.Image)
	}
	want := []string{"version", "", "url"}
	if len(got.Banner.Fields) != len(want) {
		t.Fatalf("fields=%v want %v", got.Banner.Fields, want)
	}
	for i := range want {
		if got.Banner.Fields[i] != want[i] {
			t.Fatalf("fields[%d]=%q want %q", i, got.Banner.Fields[i], want[i])
		}
	}
}

func TestLoadPathPartialDevKeepsBannerDefault(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "unigo.toml")
	if err := os.WriteFile(path, []byte("[dev]\nlogo = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadPath(path)
	if got.Logo() {
		t.Fatal("expected logo=false")
	}
	if !got.TUIEnabled() {
		t.Fatal("expected tui default true")
	}
	if !strings.Contains(got.DB.DSN, "54322") {
		t.Fatalf("expected default postgres db, got %+v", got.DB)
	}
	if len(got.Banner.Fields) != len(Default().Banner.Fields) {
		t.Fatalf("expected default banner fields, got %v", got.Banner.Fields)
	}
}

func TestLoadPathReadsDB(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "unigo.toml")
	content := `
[db]
dsn = "postgres://localhost/unigo"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadPath(path)
	if got.DB.DSN != "postgres://localhost/unigo" {
		t.Fatalf("db=%+v", got.DB)
	}
}

func TestLoadPathReadsSite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "unigo.toml")
	content := `
[site]
name = "myblog"
base_url = "https://example.com"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadPath(path)
	if got.Site.Name != "myblog" || got.Site.BaseURL != "https://example.com" {
		t.Fatalf("site=%+v", got.Site)
	}
}

func TestLoadPathMissingSiteUsesDefault(t *testing.T) {
	t.Parallel()

	got := LoadPath(filepath.Join(t.TempDir(), "missing.toml"))
	if got.Site.Name != "unagi" {
		t.Fatalf("site name=%q", got.Site.Name)
	}
}
