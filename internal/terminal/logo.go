package terminal

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SouichiroTsujimoto/unagi/internal/config"
)

//go:embed logo-ascii.txt
var logoASCIIFile embed.FS

const (
	asciiSuffix     = "-ascii.txt"
	asciiHashSuffix = "-ascii.txt.sha256"
)

// ASCIILogo returns the banner logo art (ANSI truecolor braille).
// Prefer a custom ASCII file derived from [banner].image; otherwise the embedded default.
func ASCIILogo() string {
	if custom := loadCustomASCIILogo(config.Load().Banner.Image); custom != "" {
		return custom
	}
	return embeddedASCIILogo()
}

func renderASCIILogo() string {
	return ASCIILogo()
}

// EnsureBannerLogo generates or refreshes the custom ASCII logo when [banner].image is set.
// Runs `go run ./cmd/logo` so ascii-image-converter is not linked into bin/server.
// Safe to call when image is unset (no-op). Never modifies the embedded default logo.
func EnsureBannerLogo() error {
	image := strings.TrimSpace(config.Load().Banner.Image)
	if image == "" {
		return nil
	}
	image = filepath.Clean(image)
	if _, err := os.Stat(image); err != nil {
		return err
	}
	sum, err := fileSHA256(image)
	if err != nil {
		return err
	}
	if asciiFresh(asciiPathForImage(image), asciiHashPathForImage(image), sum) {
		return nil
	}
	cmd := exec.Command("go", "run", "./cmd/logo")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func embeddedASCIILogo() string {
	data, err := logoASCIIFile.ReadFile("logo-ascii.txt")
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// PlainASCIILogo returns the banner braille art with ANSI colors stripped.
func PlainASCIILogo() string {
	return strings.TrimRight(ansiEscapeRE.ReplaceAllString(ASCIILogo(), ""), "\n")
}

// PlainASCIILogoLines returns non-empty lines of PlainASCIILogo.
func PlainASCIILogoLines() []string {
	plain := PlainASCIILogo()
	if plain == "" {
		return nil
	}
	raw := strings.Split(plain, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func loadCustomASCIILogo(imagePath string) string {
	imagePath = strings.TrimSpace(imagePath)
	if imagePath == "" {
		return ""
	}
	imagePath = filepath.Clean(imagePath)
	asciiPath := asciiPathForImage(imagePath)

	sum, imgErr := fileSHA256(imagePath)
	if imgErr == nil {
		if !asciiFresh(asciiPath, asciiHashPathForImage(imagePath), sum) {
			// Stale or missing: try to regenerate when `go` is available (dev).
			_ = EnsureBannerLogo()
		}
	}

	data, err := os.ReadFile(asciiPath)
	if err != nil {
		return ""
	}
	// When the source image is present, only trust ascii that matches its hash.
	if imgErr == nil && !asciiFresh(asciiPath, asciiHashPathForImage(imagePath), sum) {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}

func asciiPathForImage(imagePath string) string {
	return strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + asciiSuffix
}

func asciiHashPathForImage(imagePath string) string {
	return strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + asciiHashSuffix
}

func asciiFresh(asciiPath, hashPath, wantSum string) bool {
	if _, err := os.Stat(asciiPath); err != nil {
		return false
	}
	data, err := os.ReadFile(hashPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == wantSum
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
