package home

import (
	"strings"
	"unicode/utf8"

	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
)

// ArticleBrailleThumb returns a static braille ASCII crop from logo-ascii.txt,
// uniquely offset by seed (article slug). Size is in braille characters.
func ArticleBrailleThumb(seed string, cols, rows int) string {
	if cols < 1 {
		cols = 8
	}
	if rows < 1 {
		rows = 5
	}
	lines := terminal.PlainASCIILogoLines()
	if len(lines) == 0 {
		return NoiseBrailleThumb(seed, cols, rows)
	}
	return cropBrailleArt(lines, seed, cols, rows)
}

// NoiseBrailleThumb packs a seeded noise DotGrid into braille characters.
func NoiseBrailleThumb(seed string, cols, rows int) string {
	if cols < 1 {
		cols = 8
	}
	if rows < 1 {
		rows = 5
	}
	// Each braille glyph = 2×4 dots.
	grid := NoiseGrid(seed+"|thumb", cols*2, rows*4)
	var b strings.Builder
	for cy := 0; cy < rows; cy++ {
		if cy > 0 {
			b.WriteByte('\n')
		}
		for cx := 0; cx < cols; cx++ {
			b.WriteRune(packBrailleCell(grid, cx, cy))
		}
	}
	return b.String()
}

func cropBrailleArt(lines []string, seed string, cols, rows int) string {
	runes := make([][]rune, len(lines))
	width := 0
	for i, line := range lines {
		runes[i] = []rune(line)
		if w := len(runes[i]); w > width {
			width = w
		}
	}
	if width == 0 {
		return ""
	}
	if cols > width {
		cols = width
	}
	if rows > len(runes) {
		rows = len(runes)
	}

	h := seedHash(seed)
	maxY := len(runes) - rows
	maxX := width - cols
	if maxY < 0 {
		maxY = 0
	}
	if maxX < 0 {
		maxX = 0
	}
	y0 := int(h % uint32(maxY+1))
	x0 := int((h >> 8) % uint32(maxX+1))

	var b strings.Builder
	for y := 0; y < rows; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		row := runes[y0+y]
		for x := 0; x < cols; x++ {
			xi := x0 + x
			if xi < len(row) {
				b.WriteRune(row[xi])
			} else {
				b.WriteRune('⠀')
			}
		}
	}
	return b.String()
}

func packBrailleCell(g DotGrid, cellX, cellY int) rune {
	// Unicode braille bit order:
	// 1 4
	// 2 5
	// 3 6
	// 7 8
	offsets := [8][2]int{
		{0, 0}, {0, 1}, {0, 2}, {1, 0},
		{1, 1}, {1, 2}, {0, 3}, {1, 3},
	}
	bits := 0
	ox := cellX * 2
	oy := cellY * 4
	for i, d := range offsets {
		if g.At(ox+d[0], oy+d[1]) {
			bits |= 1 << i
		}
	}
	return rune(0x2800 + bits)
}

// thumbLineCount helps tests / layout without parsing the string twice.
func thumbLineCount(art string) int {
	if art == "" {
		return 0
	}
	return strings.Count(art, "\n") + 1
}

func thumbWidth(art string) int {
	width := 0
	for _, line := range strings.Split(art, "\n") {
		if n := utf8.RuneCountInString(line); n > width {
			width = n
		}
	}
	return width
}
