package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/SouichiroTsujimoto/unagi/internal/logogen/aicwrap"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: braillegen <image> <out-ascii.txt> [width]")
		os.Exit(2)
	}
	src, out := os.Args[1], os.Args[2]
	width := 28
	if len(os.Args) > 3 {
		w, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		width = w
	}
	art, err := aicwrap.Convert(src, width)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte(art+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", out, "len", len(art))
}
