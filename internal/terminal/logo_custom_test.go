package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestASCIIPathForImage(t *testing.T) {
	t.Parallel()
	if got := asciiPathForImage("brand/logo.png"); got != "brand/logo-ascii.txt" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadCustomASCIILogoUsesCommittedFileWithoutImage(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "logo.png")
	ascii := filepath.Join(dir, "logo-ascii.txt")
	if err := os.WriteFile(ascii, []byte("CUSTOM\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Image absent: still use committed ascii (deploy / clone without source image).
	got := loadCustomASCIILogo(image)
	if got != "CUSTOM" {
		t.Fatalf("got %q", got)
	}
}
