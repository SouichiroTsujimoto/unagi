package home

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestArticleBrailleThumbSize(t *testing.T) {
	art := ArticleBrailleThumb("hello-unagi", 8, 5)
	if art == "" {
		t.Fatal("empty thumb")
	}
	lines := strings.Split(art, "\n")
	if len(lines) != 5 {
		t.Fatalf("rows=%d want 5; art=%q", len(lines), art)
	}
	for i, line := range lines {
		if utf8.RuneCountInString(line) != 8 {
			t.Fatalf("line %d width=%d want 8: %q", i, utf8.RuneCountInString(line), line)
		}
	}
}

func TestArticleBrailleThumbStableAndDistinct(t *testing.T) {
	a := ArticleBrailleThumb("hello-unagi", 8, 5)
	b := ArticleBrailleThumb("hello-unagi", 8, 5)
	if a != b {
		t.Fatal("unstable")
	}
	c := ArticleBrailleThumb("other-slug", 8, 5)
	if a == c {
		t.Fatal("different seeds produced identical thumbs")
	}
}

func TestNoiseBrailleThumbIsBraille(t *testing.T) {
	art := NoiseBrailleThumb("noise-demo", 4, 3)
	for _, r := range art {
		if r == '\n' {
			continue
		}
		if r < 0x2800 || r > 0x28FF {
			t.Fatalf("non-braille rune U+%04X", r)
		}
	}
	if thumbLineCount(art) != 3 || thumbWidth(art) != 4 {
		t.Fatalf("size lines=%d width=%d", thumbLineCount(art), thumbWidth(art))
	}
}
