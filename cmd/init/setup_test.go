package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameModule(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "internal", "app")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	goMod := "module github.com/SouichiroTsujimoto/unagi\n\ngo 1.26.0\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	appGo := `package app

import "github.com/SouichiroTsujimoto/unagi/internal/feature/article"
`
	if err := os.WriteFile(filepath.Join(src, "app.go"), []byte(appGo), 0o644); err != nil {
		t.Fatal(err)
	}
	templGo := `package home
import "github.com/SouichiroTsujimoto/unagi/internal/feature/article"
`
	if err := os.WriteFile(filepath.Join(src, "page_templ.go"), []byte(templGo), 0o644); err != nil {
		t.Fatal(err)
	}

	to := "github.com/example/app"
	if err := renameModule(root, skeletonModule, to); err != nil {
		t.Fatal(err)
	}

	modData, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modData), "module "+to) {
		t.Fatalf("go.mod not updated: %s", modData)
	}
	appData, err := os.ReadFile(filepath.Join(src, "app.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appData), to+"/internal/feature/article") {
		t.Fatalf("app.go not updated: %s", appData)
	}
	templData, err := os.ReadFile(filepath.Join(src, "page_templ.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(templData), to+"/internal/feature/article") {
		t.Fatalf("_templ.go not updated: %s", templData)
	}
}
