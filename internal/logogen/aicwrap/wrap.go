package aicwrap

import (
	_ "unsafe"

	_ "github.com/SouichiroTsujimoto/unigo-template/internal/logogen/colorboot"

	"github.com/TheZoraiz/ascii-image-converter/aic_package"
	gookitColor "github.com/gookit/color"
)

// ascii-image-converter snapshots gookit TermColorLevel into an unexported
// package var at init. Under `go test` / non-TTY that snapshot can be "none"
// even after ForceOpenColor. Pin it for our pinned converter version.
//
//go:linkname termColorLevel github.com/TheZoraiz/ascii-image-converter/image_manipulation.termColorLevel
var termColorLevel string

func init() {
	termColorLevel = "millions"
	gookitColor.Enable = true
}

// Convert returns braille truecolor ANSI art for imagePath.
func Convert(imagePath string, width int) (string, error) {
	gookitColor.Enable = true
	termColorLevel = "millions"

	flags := aic_package.DefaultFlags()
	flags.Braille = true
	flags.Colored = true
	flags.Width = width
	return aic_package.Convert(imagePath, flags)
}
