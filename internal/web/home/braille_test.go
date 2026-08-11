package home

import "testing"

func TestDotByteStable(t *testing.T) {
	a := DotByte("hello-unagi")
	b := DotByte("hello-unagi")
	if a != b {
		t.Fatalf("DotByte unstable: %d vs %d", a, b)
	}
}

func TestGlyphIsBrailleBlock(t *testing.T) {
	g := []rune(Glyph("hello-unagi"))
	if len(g) != 1 {
		t.Fatalf("want 1 rune, got %q", Glyph("hello-unagi"))
	}
	if g[0] < 0x2800 || g[0] > 0x28FF {
		t.Fatalf("not braille: U+%04X", g[0])
	}
}

func TestFramesLengthAndDistinct(t *testing.T) {
	frames := Frames("cascade-wave", 8)
	if len(frames) != 8 {
		t.Fatalf("len=%d", len(frames))
	}
	same := true
	for i := 1; i < len(frames); i++ {
		if frames[i] != frames[0] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("all frames identical")
	}
}

func TestNoiseGridSize(t *testing.T) {
	g := NoiseGrid("hello-unagi", 5, 5)
	if g.Cols != 5 || g.Rows != 5 || len(g.On) != 25 {
		t.Fatalf("bad size: %+v len=%d", g, len(g.On))
	}
	h := NoiseGrid("hello-unagi", 5, 5)
	for i := range g.On {
		if g.On[i] != h.On[i] {
			t.Fatal("noise not stable")
		}
	}
}

func TestGlyphArrowScaled(t *testing.T) {
	g := GlyphGrid("arrow-right", 10, 10)
	if len(g.On) != 100 {
		t.Fatalf("len=%d", len(g.On))
	}
	on := 0
	for _, v := range g.On {
		if v {
			on++
		}
	}
	if on < 10 {
		t.Fatalf("arrow too empty: on=%d", on)
	}
}

func TestNoiseFramesAttr(t *testing.T) {
	frames := NoiseFrames("x", 3, 3, 4)
	attr := FramesAttrFromGrids(frames)
	if stringsCount(attr, '|') != 3 {
		t.Fatalf("want 3 separators: %q", attr)
	}
}

func stringsCount(s string, sep rune) int {
	n := 0
	for _, r := range s {
		if r == sep {
			n++
		}
	}
	return n
}
