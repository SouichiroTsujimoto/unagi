package logogen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASCIIPath(t *testing.T) {
	t.Parallel()
	if got := ASCIIPath("brand/logo.png"); got != "brand/logo-ascii.txt" {
		t.Fatalf("got %q", got)
	}
	if got := HashPath("brand/logo.png"); got != "brand/logo-ascii.txt.sha256" {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureGenerateReuseAndRefresh(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(image, pngSolid(20, 180, 180), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Ensure(image); err != nil {
		t.Fatal(err)
	}
	ascii := ASCIIPath(image)
	first, err := os.ReadFile(ascii)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || !strings.Contains(string(first), "\x1b[") {
		t.Fatalf("expected colored ascii, len=%d", len(first))
	}

	info1, err := os.Stat(ascii)
	if err != nil {
		t.Fatal(err)
	}
	if err := Ensure(image); err != nil {
		t.Fatal(err)
	}
	info2, err := os.Stat(ascii)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatal("expected reuse when image hash matches")
	}

	if err := os.WriteFile(image, pngSolid(200, 40, 40), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(image); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(ascii)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("expected ascii to change after image change")
	}
}

func TestEnsureEmptyNoop(t *testing.T) {
	t.Parallel()
	if err := Ensure(""); err != nil {
		t.Fatal(err)
	}
}

// pngSolid returns a tiny valid 8x8 RGB PNG filled with one color.
func pngSolid(r, g, b byte) []byte {
	const w, h = 8, 8
	// Manually build via standard library in a helper file would pull image/png;
	// keep a precomputed path by writing through compress in test binary.
	return mustEncodePNG(w, h, r, g, b)
}
