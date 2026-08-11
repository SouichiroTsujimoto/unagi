package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SouichiroTsujimoto/unigo-template/internal/config"
)

func writeTestConfig(t *testing.T, fields string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "unigo.toml")
	content := "[banner]\nfields = [" + fields + "]\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.PathEnv, path)
}

func TestPrintListenBanner(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeTestConfig(t, `"version", "go", "mode", "db", "url"`)

	if err := os.WriteFile("go.mod", []byte("module github.com/example/demo-app\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var full strings.Builder
	PrintListenBanner(&full, "full", BannerInfo{
		Version: "0.1.0",
		DBPath:  "app.db",
		Mode:    "http",
		URLs:    []string{"http://localhost:8080"},
	})
	gotFull := full.String()
	for _, want := range []string{"demo-app", "version:", "0.1.0", "go:", "mode:", "http", "db:", "app.db", "http://localhost:8080"} {
		if !strings.Contains(gotFull, want) {
			t.Fatalf("banner missing %q: %q", want, gotFull)
		}
	}
	if strings.Contains(gotFull, "tailwindcss:") || strings.Contains(gotFull, "air:") {
		t.Fatalf("banner should omit unconfigured fields: %q", gotFull)
	}

	var compact strings.Builder
	PrintListenBanner(&compact, "compact", BannerInfo{
		Version: "0.1.0",
		DBPath:  "app.db",
		Mode:    "http",
		URLs:    []string{"http://localhost:8080"},
	})
	gotCompact := compact.String()
	if strings.Contains(gotCompact, "⣿") {
		t.Fatalf("compact banner should omit ascii logo: %q", gotCompact)
	}
	if !strings.Contains(gotCompact, "http://localhost:8080") {
		t.Fatalf("compact banner missing url: %q", gotCompact)
	}
}

func TestBannerStyleFromLogo(t *testing.T) {
	t.Parallel()
	if got := BannerStyleFromLogo(true); got != "full" {
		t.Fatalf("logo on = %q, want full", got)
	}
	if got := BannerStyleFromLogo(false); got != "compact" {
		t.Fatalf("logo off = %q, want compact", got)
	}
}

func TestRenderListenBannerPlainOmitsBorder(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeTestConfig(t, `"version", "url"`)

	info := BannerInfo{
		Version: "0.1.0",
		URLs:    []string{"http://localhost:8080"},
	}
	bordered := RenderListenBanner("compact", info)
	plain := RenderListenBannerPlain("compact", info)
	if !strings.Contains(bordered, "╭") || !strings.Contains(bordered, "╰") {
		t.Fatalf("bordered banner should include box drawing: %q", bordered)
	}
	if strings.Contains(plain, "╭") || strings.Contains(plain, "╰") || strings.Contains(plain, "│") {
		t.Fatalf("plain banner should omit box border: %q", plain)
	}
	if !strings.Contains(plain, "http://localhost:8080") {
		t.Fatalf("plain banner missing url: %q", plain)
	}
}

func TestShouldPrintListenBanner(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("UNIGO_DEV_TUI", "")

	if !ShouldPrintListenBanner() {
		t.Fatal("expected banner without tmp/")
	}
	if err := os.Mkdir("tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if !ShouldPrintListenBanner() {
		t.Fatal("expected first banner with tmp/")
	}
	if ShouldPrintListenBanner() {
		t.Fatal("expected banner suppressed after gate file")
	}
}

func TestShouldPrintListenBannerSuppressedInDevTUI(t *testing.T) {
	t.Setenv("UNIGO_DEV_TUI", "1")
	if ShouldPrintListenBanner() {
		t.Fatal("dev TUI should own the banner")
	}
}
