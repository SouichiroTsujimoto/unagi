package brailleart

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseANSITrueColor(t *testing.T) {
	raw := "\x1b[38;2;10;20;30m⣿\x1b[0m\x1b[38;2;255;0;0m⣿\x1b[0m"
	g := ParseANSI(raw)
	if g.Rows() != 1 || g.Cols() != 2 {
		t.Fatalf("size rows=%d cols=%d", g.Rows(), g.Cols())
	}
	if !g.Cells[0][0].Colored || g.Cells[0][0].R != 10 || g.Cells[0][0].G != 20 || g.Cells[0][0].B != 30 {
		t.Fatalf("cell0=%+v", g.Cells[0][0])
	}
	if g.Cells[0][1].R != 255 || g.Cells[0][1].G != 0 {
		t.Fatalf("cell1=%+v", g.Cells[0][1])
	}
}

func TestHTMLEscapesAndColors(t *testing.T) {
	g := ParseANSI("\x1b[38;2;1;2;3m⣿\x1b[0m")
	html := g.HTML("braille-ansi-art--list")
	if !strings.Contains(html, `style="color:rgb(1,2,3)"`) {
		t.Fatalf("missing color: %s", html)
	}
	if strings.Contains(html, "\x1b") {
		t.Fatal("raw ANSI leaked into HTML")
	}
	if !strings.Contains(html, "braille-ansi-art--list") {
		t.Fatal("missing class")
	}
}

func TestThumbHTMLIsFullSize(t *testing.T) {
	html := ThumbHTML("hello-unagi")
	if html == "" {
		t.Fatal("empty")
	}
	// Custom file for hello-unagi exists; full art should be many lines.
	if strings.Count(html, "\n") < 8 {
		t.Fatalf("expected full-size art, newlines=%d", strings.Count(html, "\n"))
	}
}

func TestCropBySeedStable(t *testing.T) {
	var raw strings.Builder
	for y := 0; y < 12; y++ {
		if y > 0 {
			raw.WriteByte('\n')
		}
		for x := 0; x < 24; x++ {
			r := (x * 11) % 256
			g := (y * 19) % 256
			b := (x*y + 40) % 256
			fmt.Fprintf(&raw, "\x1b[38;2;%d;%d;%dm⣿\x1b[0m", r, g, b)
		}
	}
	g := ParseANSI(raw.String())
	a := g.CropBySeed("hello-unagi", 8, 5)
	b := g.CropBySeed("hello-unagi", 8, 5)
	if a.HTML("") != b.HTML("") {
		t.Fatal("unstable crop")
	}
	c := g.CropBySeed("other", 8, 5)
	if a.HTML("") == c.HTML("") {
		t.Fatal("expected distinct crops")
	}
}

func TestDefaultGridFromLogo(t *testing.T) {
	g := DefaultGrid()
	if g.Rows() < 8 || g.Cols() < 16 {
		t.Fatalf("logo grid too small: %dx%d", g.Cols(), g.Rows())
	}
	colored := 0
	for _, row := range g.Cells {
		for _, c := range row {
			if c.Colored {
				colored++
			}
		}
	}
	if colored < 100 {
		t.Fatalf("expected many colored cells, got %d", colored)
	}
}
