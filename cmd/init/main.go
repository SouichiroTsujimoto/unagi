package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(options{Root: "."}); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
}
