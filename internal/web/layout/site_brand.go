package layout

import "strconv"

// Hollow circle 5×5 (row-major) for the site brand mark.
const siteBrandCirclePattern = "" +
	".###." +
	"#...#" +
	"..##." +
	".#..." +
	"..###"

func siteBrandDotClass(i int) string {
	if i >= 0 && i < len(siteBrandCirclePattern) && siteBrandCirclePattern[i] == '#' {
		return "braille-dot is-on"
	}
	return "braille-dot"
}

func siteBrandDotStyle(i int) string {
	return "--i:" + strconv.Itoa(i)
}
