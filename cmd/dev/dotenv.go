package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// loadDotEnv loads project-root .env for local development.
// Missing file is fine. Existing process env is not overwritten
// (shell export and CI env win over .env).
func loadDotEnv() {
	err := godotenv.Load(".env")
	if err == nil || os.IsNotExist(err) {
		return
	}
	fmt.Fprintf(os.Stderr, "dev: .env: %v\n", err)
}
