package terminal

import (
	"strings"
	"testing"
)

func TestRenderASCIILogo(t *testing.T) {
	t.Parallel()

	got := renderASCIILogo()
	if got == "" {
		t.Fatal("expected non-empty logo")
	}
	if !strings.Contains(got, "⣿") {
		t.Fatalf("expected braille art, got %q", got[:min(80, len(got))])
	}
	// Colored output should include ANSI escape sequences.
	if !strings.Contains(got, "\x1b[") {
		t.Fatal("expected ANSI color sequences in logo output")
	}
}
