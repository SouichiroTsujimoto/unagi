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

func TestNoiseGridDiffersBySeed(t *testing.T) {
	a := NoiseGrid("kea-dhcp-lease-metrics", 5, 5)
	b := NoiseGrid("astrobit-moonbit-components", 5, 5)
	same := true
	for i := range a.On {
		if a.On[i] != b.On[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical noise")
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

func TestGlyphShareShape(t *testing.T) {
	g := GlyphGrid("share", 5, 5)
	if len(g.On) != 25 {
		t.Fatalf("len=%d", len(g.On))
	}
	// Tip of up-arrow and tray corners should be on.
	if !g.At(2, 0) || !g.At(0, 4) || !g.At(4, 4) {
		t.Fatalf("share glyph missing expected dots: %v", g.On)
	}
}

func TestNoiseFramesAttr(t *testing.T) {
	frames := NoiseFrames("x", 3, 3, 4)
	attr := FramesAttrFromGrids(frames)
	if stringsCount(attr, '|') != 3 {
		t.Fatalf("want 3 separators: %q", attr)
	}
}

func TestGlyphScrollFramesMovesRightWithGap(t *testing.T) {
	frames := GlyphScrollFrames("arrow-right", 5, 5, 0) // n<1 → cols+1
	if len(frames) != 6 {
		t.Fatalf("len=%d want 6", len(frames))
	}
	base := GlyphGrid("arrow-right", 5, 5)
	// Frame 1: one column right; left column is the gap (empty).
	for y := 0; y < 5; y++ {
		if frames[1].On[y*5+0] {
			t.Fatalf("frame1 left col should be gap at y=%d", y)
		}
		for x := 1; x < 5; x++ {
			want := base.On[y*5+(x-1)]
			if frames[1].On[y*5+x] != want {
				t.Fatalf("frame1 mismatch at %d,%d", x, y)
			}
		}
	}
	// Frame 5: shift=5 → only the rightmost viewport column can show glyph col0;
	// the gap sits between wrapped copies (strip index 5).
	on := 0
	for _, v := range frames[5].On {
		if v {
			on++
		}
	}
	if on == 0 {
		t.Fatal("frame5 should still show wrapped glyph fragment")
	}
	// Wherever glyph appears on both sides of a wrap, at least one empty col remains.
	// At shift=5: src=(x-5)%6 → x0→1, x1→2, x2→3, x3→4, x4→5(gap)
	if frames[5].On[0*5+4] { // any row, gap column in viewport
		// gap is at x=4 for all rows
	}
	for y := 0; y < 5; y++ {
		if frames[5].On[y*5+4] {
			t.Fatalf("frame5 x=4 should be gap at y=%d", y)
		}
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
