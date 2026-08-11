package terminal

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPlainASCIILogoLines(t *testing.T) {
	lines := PlainASCIILogoLines()
	if len(lines) < 8 {
		t.Fatalf("too few lines: %d", len(lines))
	}
	for i, line := range lines {
		if strings.ContainsRune(line, '\x1b') {
			t.Fatalf("line %d still has ANSI", i)
		}
		if utf8.RuneCountInString(line) < 8 {
			t.Fatalf("line %d too short: %q", i, line)
		}
	}
}
