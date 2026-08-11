// Package colorboot forces truecolor before ascii-image-converter initializes.
package colorboot

import (
	"os"

	gookitColor "github.com/gookit/color"
)

func init() {
	if os.Getenv("TERM") == "" {
		_ = os.Setenv("TERM", "xterm-256color")
	}
	if os.Getenv("COLORTERM") == "" {
		_ = os.Setenv("COLORTERM", "truecolor")
	}
	_ = gookitColor.ForceOpenColor()
	gookitColor.Enable = true
}
