package layout

import "strconv"

// arrow-up 5×5 (row-major), same glyph as the braille lab cascade.
const backToTopPattern = "" +
	"..#.." +
	".###." +
	"#.#.#" +
	"..#.." +
	"..#.."

func backToTopDotClass(i int) string {
	if i >= 0 && i < len(backToTopPattern) && backToTopPattern[i] == '#' {
		return "braille-dot is-on"
	}
	return "braille-dot"
}

func backToTopDotStyle(i int) string {
	return "--i:" + strconv.Itoa(i)
}
