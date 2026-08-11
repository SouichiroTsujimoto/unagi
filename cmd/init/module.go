package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func readModulePath(goModPath string) (string, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after), nil
		}
	}
	return "", fmt.Errorf("module path not found in %s", goModPath)
}

func renameModule(root, from, to string) error {
	if from == "" || to == "" {
		return fmt.Errorf("module paths must not be empty")
	}
	if from == to {
		return nil
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "tmp", "bin", "tools", "node_modules":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		// Include *_templ.go so go.mod / imports stay consistent until templ generate rewrites them.
		switch filepath.Ext(d.Name()) {
		case ".go", ".mod", ".md", ".templ", ".toml":
		default:
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(data, []byte(from)) {
			return nil
		}
		updated := bytes.ReplaceAll(data, []byte(from), []byte(to))
		return os.WriteFile(path, updated, 0o644)
	})
}
