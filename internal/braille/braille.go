// Package braille builds deterministic dot patterns shared across renderers.
package braille

import (
	"hash/fnv"
	"strings"
)

// DotGrid is a row-major on/off bitmap.
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

// SeedHash returns the stable FNV-1a seed used by article dot patterns.
func SeedHash(seed string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return h.Sum32()
}

// NoiseGrid fills a deterministic pseudo-random pattern from seed (~45% on).
func NoiseGrid(seed string, cols, rows int) DotGrid {
	grid := DotGrid{Cols: cols, Rows: rows, On: make([]bool, cols*rows)}
	hash := SeedHash(seed)
	for i := range grid.On {
		hash = hash*16777619 + uint32(i)*97 + 0x9e3779b9
		grid.On[i] = (hash>>16)&0xff > 140
	}
	return grid
}
