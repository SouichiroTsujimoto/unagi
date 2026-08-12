package home

import (
	"hash/fnv"
	"strconv"
	"strings"
)

// DotGrid is a row-major on/off bitmap for the braille icon lab.
type DotGrid struct {
	Cols int
	Rows int
	On   []bool
}

func (g DotGrid) At(x, y int) bool {
	if x < 0 || y < 0 || x >= g.Cols || y >= g.Rows {
		return false
	}
	return g.On[y*g.Cols+x]
}

func (g DotGrid) Pack01() string {
	var b strings.Builder
	b.Grow(len(g.On))
	for _, on := range g.On {
		if on {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

func seedHash(seed string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return h.Sum32()
}

func emptyGrid(cols, rows int) DotGrid {
	return DotGrid{Cols: cols, Rows: rows, On: make([]bool, cols*rows)}
}

// NoiseGrid fills a deterministic pseudo-random pattern from seed (~45% on).
func NoiseGrid(seed string, cols, rows int) DotGrid {
	g := emptyGrid(cols, rows)
	h := seedHash(seed)
	for i := range g.On {
		h = h*16777619 + uint32(i)*97 + 0x9e3779b9
		g.On[i] = (h>>16)&0xff > 140
	}
	return g
}

// NoiseFrames returns evolving noise with a clear traveling scan column.
func NoiseFrames(seed string, cols, rows, n int) []DotGrid {
	if n < 1 {
		n = 1
	}
	base := NoiseGrid(seed, cols, rows)
	out := make([]DotGrid, n)
	for i := 0; i < n; i++ {
		frame := emptyGrid(cols, rows)
		scanX := i % cols
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				srcX := (x + i) % cols
				on := base.On[y*cols+srcX]
				if x == scanX {
					on = true
				} else if x == (scanX-1+cols)%cols {
					on = false
				}
				frame.On[y*cols+x] = on
			}
		}
		out[i] = frame
	}
	return out
}

func FramesAttrFromGrids(frames []DotGrid) string {
	parts := make([]string, len(frames))
	for i, f := range frames {
		parts[i] = f.Pack01()
	}
	return strings.Join(parts, "|")
}

// GlyphGrid returns a named symbol, scaled to cols×rows with nearest-neighbor.
func GlyphGrid(name string, cols, rows int) DotGrid {
	src := glyphSource(name)
	if src.Cols == 0 {
		return NoiseGrid(name, cols, rows)
	}
	return scaleNearest(src, cols, rows)
}

func glyphSource(name string) DotGrid {
	switch name {
	case "arrow-right":
		return parseGlyph(5, 5, ""+
			"..#.."+
			"...#."+
			"#####"+
			"...#."+
			"..#..")
	case "arrow-up":
		return parseGlyph(5, 5, ""+
			"..#.."+
			".###."+
			"#.#.#"+
			"..#.."+
			"..#..")
	case "arrow-left":
		return parseGlyph(5, 5, ""+
			"..#.."+
			".#..."+
			"#####"+
			".#..."+
			"..#..")
	case "arrow-down":
		return parseGlyph(5, 5, ""+
			"..#.."+
			"..#.."+
			"#.#.#"+
			".###."+
			"..#..")
	case "smile":
		return parseGlyph(5, 5, ""+
			".#.#."+
			".#.#."+
			"....."+
			"#...#"+
			".###.")
	case "wink":
		return parseGlyph(5, 5, ""+
			".#..."+
			".#.#."+
			"....."+
			"#...#"+
			".###.")
	case "face":
		return parseGlyph(5, 5, ""+
			".###."+
			"#.#.#"+
			"#####"+
			"#...#"+
			".###.")
	case "heart":
		return parseGlyph(5, 5, ""+
			".#.#."+
			"#####"+
			"#####"+
			".###."+
			"..#..")
	case "x":
		return parseGlyph(5, 5, ""+
			"#...#"+
			".#.#."+
			"..#.."+
			".#.#."+
			"#...#")
	case "check":
		return parseGlyph(5, 5, ""+
			"....#"+
			"...#."+
			"#.#.."+
			".#..."+
			".....")
	case "share":
		// Tray with upward arrow (share affordance).
		return parseGlyph(5, 5, ""+
			".####"+
			"...##"+
			"..#.#"+
			".#..#"+
			"#....")
	default:
		return DotGrid{}
	}
}

func parseGlyph(cols, rows int, pattern string) DotGrid {
	g := emptyGrid(cols, rows)
	pattern = strings.ReplaceAll(pattern, "\n", "")
	for i := 0; i < len(g.On) && i < len(pattern); i++ {
		g.On[i] = pattern[i] == '#'
	}
	return g
}

func scaleNearest(src DotGrid, cols, rows int) DotGrid {
	if src.Cols == cols && src.Rows == rows {
		out := emptyGrid(cols, rows)
		copy(out.On, src.On)
		return out
	}
	out := emptyGrid(cols, rows)
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			sx := (x*src.Cols + cols/2) / cols
			sy := (y*src.Rows + rows/2) / rows
			if sx >= src.Cols {
				sx = src.Cols - 1
			}
			if sy >= src.Rows {
				sy = src.Rows - 1
			}
			out.On[y*cols+x] = src.At(sx, sy)
		}
	}
	return out
}

func Complement(g DotGrid) DotGrid {
	out := emptyGrid(g.Cols, g.Rows)
	for i, on := range g.On {
		out.On[i] = !on
	}
	return out
}

// --- classic 2×4 Braille Patterns (kept for A/B/D cards) ---

func DotByte(seed string) uint8 {
	return uint8(seedHash(seed))
}

func brailleRune(bits uint8) rune {
	return rune(0x2800 + int(bits))
}

func Glyph(seed string) string {
	return string(brailleRune(DotByte(seed)))
}

func SwapPair(seed string) (rest, hover string) {
	b := DotByte(seed)
	return string(brailleRune(b)), string(brailleRune(^b))
}

func DotOn(seed string, i int) bool {
	if i < 0 || i > 7 {
		return false
	}
	return DotByte(seed)&(1<<uint(i)) != 0
}

func Frames(seed string, n int) []string {
	if n < 1 {
		n = 1
	}
	base := DotByte(seed)
	out := make([]string, n)
	for i := 0; i < n; i++ {
		rotated := bitsRotateLeft(base, i%8)
		pulse := uint8(1 << (i % 8))
		out[i] = string(brailleRune(rotated | pulse))
	}
	return out
}

func FramesAttr(seed string) string {
	return strings.Join(Frames(seed, 8), ",")
}

func Phases(seed string) (dormant, pioneer, climax string) {
	b := DotByte(seed)
	return string(brailleRune(b & 0x09)),
		string(brailleRune(b&0x3F | 0x12)),
		string(brailleRune(b))
}

func PhasesAttr(seed string) string {
	a, b, c := Phases(seed)
	return strings.Join([]string{a, b, c}, ",")
}

func bitsRotateLeft(v uint8, n int) uint8 {
	n &= 7
	return v<<n | v>>(8-n)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func brailleDotClass(on bool) string {
	if on {
		return "braille-dot is-on"
	}
	return "braille-dot"
}

// GlyphScrollFrames shifts a glyph one column right each frame on a looping strip.
// The strip is the glyph plus one empty column, so wraps keep a 1-cell gap
// between successive copies instead of joining tip-to-tail.
// n defaults to cols+1 (one full period).
func GlyphScrollFrames(name string, cols, rows, n int) []DotGrid {
	period := cols + 1 // glyph columns + 1-gap
	if n < 1 {
		n = period
	}
	base := GlyphGrid(name, cols, rows)
	out := make([]DotGrid, n)
	for i := 0; i < n; i++ {
		frame := emptyGrid(cols, rows)
		shift := i % period
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				// Viewport x maps onto looping strip [glyph | empty].
				src := (x - shift) % period
				if src < 0 {
					src += period
				}
				if src >= cols {
					continue // gap column
				}
				frame.On[y*cols+x] = base.On[y*cols+src]
			}
		}
		out[i] = frame
	}
	return out
}
