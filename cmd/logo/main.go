package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/SouichiroTsujimoto/unagi/internal/config"
	"github.com/SouichiroTsujimoto/unagi/internal/logogen"
)

func main() {
	file := config.Load()
	image := strings.TrimSpace(file.Banner.Image)
	if image == "" {
		fmt.Fprintln(os.Stderr, "no [banner].image in .unigo.toml; nothing to generate")
		fmt.Fprintln(os.Stderr, "embedded default logo remains in use")
		os.Exit(0)
	}
	if err := logogen.Ensure(image); err != nil {
		fmt.Fprintf(os.Stderr, "logo: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("logo ascii ready: %s\n", logogen.ASCIIPath(image))
}
