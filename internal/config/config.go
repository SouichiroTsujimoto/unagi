// Package config loads project settings from .unigo.toml.
package config

import (
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	// DefaultPath is the project-root config file.
	DefaultPath = ".unigo.toml"
	// PathEnv overrides DefaultPath when set.
	PathEnv = "UNIGO_CONFIG"
)

// File is the on-disk .unigo.toml schema.
type File struct {
	Dev    Dev    `toml:"dev"`
	DB     DB     `toml:"db"`
	Banner Banner `toml:"banner"`
}

// DB holds database connection defaults.
type DB struct {
	Driver string `toml:"driver"` // sqlite | postgres
	DSN    string `toml:"dsn"`
}

// Dev holds development-launcher defaults.
type Dev struct {
	Logo *bool `toml:"logo"`
	TUI  *bool `toml:"tui"`
}

// Banner holds listen-banner field configuration.
type Banner struct {
	Fields []string `toml:"fields"`
	// Image is an optional path to a source logo image (png/jpg/…).
	// When set, go tool ascii-image-converter generates a sibling
	// <name>-ascii.txt (committed); the embedded default logo is never removed.
	Image string `toml:"image"`
}

// Default returns built-in settings when the file is missing or empty.
func Default() File {
	logo := true
	tui := true
	return File{
		Dev: Dev{
			Logo: &logo,
			TUI:  &tui,
		},
		DB: DB{
			Driver: "sqlite",
			DSN:    "app.db",
		},
		Banner: Banner{
			Fields: []string{
				"version",
				"go",
				"templ",
				"",
				"mode",
				"db",
				"css",
				"git",
				"",
				"url",
				"pid",
			},
		},
	}
}

// Path returns the config file path (env override or default).
func Path() string {
	if v := strings.TrimSpace(os.Getenv(PathEnv)); v != "" {
		return v
	}
	return DefaultPath
}

// Load reads .unigo.toml (or UNIGO_CONFIG). Missing/invalid files yield Default().
func Load() File {
	return LoadPath(Path())
}

// LoadPath reads a specific config file path.
func LoadPath(path string) File {
	data, err := os.ReadFile(path)
	if err != nil {
		return Default()
	}
	var file File
	if err := toml.Unmarshal(data, &file); err != nil {
		return Default()
	}
	return file.mergeDefaults()
}

func (f File) mergeDefaults() File {
	def := Default()
	if f.Dev.Logo == nil {
		f.Dev.Logo = def.Dev.Logo
	}
	if f.Dev.TUI == nil {
		f.Dev.TUI = def.Dev.TUI
	}
	if strings.TrimSpace(f.DB.Driver) == "" {
		f.DB.Driver = def.DB.Driver
	}
	if strings.TrimSpace(f.DB.DSN) == "" {
		f.DB.DSN = def.DB.DSN
	}
	if len(f.Banner.Fields) == 0 {
		f.Banner.Fields = append([]string(nil), def.Banner.Fields...)
	}
	return f
}

// Logo returns the configured logo switch (default true).
func (f File) Logo() bool {
	if f.Dev.Logo == nil {
		return true
	}
	return *f.Dev.Logo
}

// TUIEnabled returns the configured TUI switch (default true).
func (f File) TUIEnabled() bool {
	if f.Dev.TUI == nil {
		return true
	}
	return *f.Dev.TUI
}
