package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SouichiroTsujimoto/unigo-template/internal/config"
)

func TestFilterBannerFields(t *testing.T) {
	t.Parallel()

	got := filterBannerFields([]string{"version", "go", "", "mode", "unknown-field", "url"})
	want := []string{"version", "go", "", "mode", "url"}
	if len(got) != len(want) {
		t.Fatalf("filterBannerFields len=%d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("filterBannerFields[%d]=%q want %q (%#v)", i, got[i], want[i], got)
		}
	}
}

func TestLoadBannerFieldsMissingUsesDefault(t *testing.T) {
	t.Setenv(config.PathEnv, filepath.Join(t.TempDir(), "missing.toml"))

	got := loadBannerFields()
	def := filterBannerFields(config.Default().Banner.Fields)
	if len(got) != len(def) {
		t.Fatalf("default fields len=%d want %d", len(got), len(def))
	}
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "version") || !strings.Contains(joined, "templ") || !strings.Contains(joined, "git") {
		t.Fatalf("unexpected defaults: %v", got)
	}
	if strings.Contains(joined, "tailwindcss") || strings.Contains(joined, "air") {
		t.Fatalf("defaults should omit tailwindcss/air: %v", got)
	}
}

func TestLoadBannerFieldsFromToml(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unigo.toml")
	content := `
[banner]
fields = ["version", "", "url", "unknown"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.PathEnv, path)

	got := loadBannerFields()
	want := []string{"version", "", "url"}
	if len(got) != len(want) {
		t.Fatalf("fields=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fields[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestGoModRequireVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	content := "module example.com/x\n\nrequire (\n\tgithub.com/a-h/templ v0.3.1020\n\tgithub.com/air-verse/air v1.67.4 // indirect\n)\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := goModRequireVersion(path, "github.com/a-h/templ"); got != "0.3.1020" {
		t.Fatalf("templ version = %q", got)
	}
	if got := goModRequireVersion(path, "github.com/air-verse/air"); got != "1.67.4" {
		t.Fatalf("air version = %q", got)
	}
	if got := moduleBaseName(path); got != "x" {
		t.Fatalf("moduleBaseName = %q", got)
	}
}

func TestDetectDaisyUIVersion(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.Mkdir("tools", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("tools", "daisyui.mjs"), []byte("var version = \"5.7.16\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectDaisyUIVersion(); got != "5.7.16" {
		t.Fatalf("detectDaisyUIVersion = %q", got)
	}
}

func TestDisplayOrDash(t *testing.T) {
	t.Parallel()

	if got := displayOrDash("1.2.3"); got != "1.2.3" {
		t.Fatalf("displayOrDash value = %q", got)
	}
	if got := displayOrDash(""); got != "—" {
		t.Fatalf("displayOrDash empty = %q", got)
	}
}
