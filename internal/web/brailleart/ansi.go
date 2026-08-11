package brailleart

import (
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
)

// Cell is one braille (or space) glyph with optional truecolor.
type Cell struct {
	Ch      rune
	R, G, B uint8
	Colored bool
}

// Grid is a rectangular ANSI braille art.
type Grid struct {
	Cells [][]Cell
}

func (g Grid) Rows() int { return len(g.Cells) }

func (g Grid) Cols() int {
	w := 0
	for _, row := range g.Cells {
		if len(row) > w {
			w = len(row)
		}
	}
	return w
}

var (
	ansiTrueColorRE = regexp.MustCompile(`\x1b\[38;2;(\d{1,3});(\d{1,3});(\d{1,3})m`)
	ansiResetRE     = regexp.MustCompile(`\x1b\[0m`)
	ansiAnyRE       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// ParseANSI converts truecolor braille ANSI (TUI logo style) into a Grid.
func ParseANSI(raw string) Grid {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimRight(raw, "\n")
	if raw == "" {
		return Grid{}
	}
	var rows [][]Cell
	for _, line := range strings.Split(raw, "\n") {
		rows = append(rows, parseANSILine(line))
	}
	return Grid{Cells: rows}
}

func parseANSILine(line string) []Cell {
	var cells []Cell
	colored := false
	var r, g, b uint8

	i := 0
	for i < len(line) {
		if line[i] == '\x1b' {
			rest := line[i:]
			if m := ansiTrueColorRE.FindStringSubmatchIndex(rest); m != nil && m[0] == 0 {
				rr, _ := strconv.Atoi(rest[m[2]:m[3]])
				gg, _ := strconv.Atoi(rest[m[4]:m[5]])
				bb, _ := strconv.Atoi(rest[m[6]:m[7]])
				r, g, b = clamp8(rr), clamp8(gg), clamp8(bb)
				colored = true
				i += m[1]
				continue
			}
			if m := ansiResetRE.FindStringIndex(rest); m != nil && m[0] == 0 {
				colored = false
				i += m[1]
				continue
			}
			if m := ansiAnyRE.FindStringIndex(rest); m != nil && m[0] == 0 {
				i += m[1]
				continue
			}
			i++
			continue
		}
		ch, size := utf8.DecodeRuneInString(line[i:])
		if ch == utf8.RuneError && size == 1 {
			i++
			continue
		}
		i += size
		if ch == '\n' {
			break
		}
		cell := Cell{Ch: ch}
		if colored {
			cell.Colored = true
			cell.R, cell.G, cell.B = r, g, b
		}
		cells = append(cells, cell)
	}
	return cells
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// Crop returns a sub-grid. Out-of-range cells become blank braille.
func (g Grid) Crop(x0, y0, cols, rows int) Grid {
	if cols < 1 || rows < 1 {
		return Grid{}
	}
	out := make([][]Cell, rows)
	for y := 0; y < rows; y++ {
		row := make([]Cell, cols)
		sy := y0 + y
		for x := 0; x < cols; x++ {
			sx := x0 + x
			if sy >= 0 && sy < len(g.Cells) && sx >= 0 && sx < len(g.Cells[sy]) {
				row[x] = g.Cells[sy][sx]
			} else {
				row[x] = Cell{Ch: '⠀'}
			}
		}
		out[y] = row
	}
	return Grid{Cells: out}
}

// CropBySeed picks a deterministic window from g using seed.
func (g Grid) CropBySeed(seed string, cols, rows int) Grid {
	gw, gh := g.Cols(), g.Rows()
	if gw == 0 || gh == 0 {
		return Grid{}
	}
	if cols > gw {
		cols = gw
	}
	if rows > gh {
		rows = gh
	}
	h := fnv32(seed)
	maxY := gh - rows
	maxX := gw - cols
	if maxY < 0 {
		maxY = 0
	}
	if maxX < 0 {
		maxX = 0
	}
	y0 := int(h % uint32(maxY+1))
	x0 := int((h >> 8) % uint32(maxX+1))
	return g.Crop(x0, y0, cols, rows)
}

// HTML renders a safe <pre> of colored spans (no raw ANSI).
func (g Grid) HTML(class string) string {
	if g.Rows() == 0 {
		return ""
	}
	if class == "" {
		class = "braille-ansi-art"
	}
	var b strings.Builder
	b.WriteString(`<pre class="`)
	b.WriteString(html.EscapeString(class))
	b.WriteString(`" aria-hidden="true">`)
	for y, row := range g.Cells {
		if y > 0 {
			b.WriteByte('\n')
		}
		for _, cell := range row {
			ch := cell.Ch
			if ch == 0 {
				ch = '⠀'
			}
			text := html.EscapeString(string(ch))
			if cell.Colored {
				fmt.Fprintf(&b, `<span style="color:rgb(%d,%d,%d)">%s</span>`, cell.R, cell.G, cell.B, text)
			} else {
				b.WriteString(text)
			}
		}
	}
	b.WriteString(`</pre>`)
	return b.String()
}

func fnv32(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

var (
	defaultOnce sync.Once
	defaultGrid Grid
)

// DefaultGrid returns the site TUI logo as a parsed color grid.
func DefaultGrid() Grid {
	defaultOnce.Do(func() {
		defaultGrid = ParseANSI(terminal.ASCIILogo())
	})
	return defaultGrid
}

// LoadFile reads an ANSI braille file from disk (dev / optional per-article art).
func LoadFile(path string) (Grid, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Grid{}, err
	}
	return ParseANSI(string(data)), nil
}

// LoadFS reads path from an fs.FS (embedded content).
func LoadFS(fsys fs.FS, path string) (Grid, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return Grid{}, err
	}
	return ParseANSI(string(data)), nil
}

// ResolveForSlug picks per-article art:
//  1. images/<slug>-ascii.txt from contentFS (if provided)
//  2. images/<slug>-ascii.txt on disk (dev)
//  3. colored crop (or full art) from the default TUI logo
//
// If cols or rows is <=0, returns the full resolved art without cropping.
func ResolveForSlug(contentFS fs.FS, slug string, cols, rows int) Grid {
	slug = strings.TrimSpace(slug)
	name := slug + "-ascii.txt"
	rel := filepath.ToSlash(filepath.Join("images", name))

	var base Grid
	if contentFS != nil {
		if g, err := LoadFS(contentFS, rel); err == nil && g.Rows() > 0 {
			base = g
		}
	}
	if base.Rows() == 0 {
		if g, err := LoadFile(filepath.Join("images", name)); err == nil && g.Rows() > 0 {
			base = g
		}
	}
	if base.Rows() == 0 {
		base = DefaultGrid()
	}
	if cols < 1 || rows < 1 {
		return base
	}
	if base.Cols() <= cols && base.Rows() <= rows {
		return base
	}
	return base.CropBySeed(slug, cols, rows)
}

// ThumbHTML is full-size colored art for the article list (no crop).
func ThumbHTML(slug string) string {
	return ResolveForSlug(nil, slug, 0, 0).HTML("braille-ansi-art braille-ansi-art--list")
}

// HeroHTML is full-size colored header art for the article page (no crop).
func HeroHTML(slug string) string {
	return ResolveForSlug(nil, slug, 0, 0).HTML("braille-ansi-art braille-ansi-art--hero")
}

// FileHTML loads a disk ANSI file and renders it (lab / compare demos).
func FileHTML(path, class string) string {
	g, err := LoadFile(path)
	if err != nil || g.Rows() == 0 {
		return ""
	}
	if class == "" {
		class = "braille-ansi-art braille-ansi-art--list"
	}
	return g.HTML(class)
}

// CompareSamples are larger ascii-image-converter widths for side-by-side review.
var CompareSamples = []struct {
	Label string
	Path  string
	Width int
}{
	{Label: "width 40", Path: "images/compare-w40-ascii.txt", Width: 40},
	{Label: "width 56", Path: "images/compare-w56-ascii.txt", Width: 56},
	{Label: "width 72", Path: "images/compare-w72-ascii.txt", Width: 72},
}
