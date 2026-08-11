// Package logogen turns a banner source image into a committed ASCII logo file.
// Imported only by cmd/logo so ascii-image-converter stays out of bin/server.
package logogen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SouichiroTsujimoto/unigo-template/internal/logogen/aicwrap"
)

const (
	asciiSuffix     = "-ascii.txt"
	asciiHashSuffix = "-ascii.txt.sha256"
	asciiLogoWidth  = 32
)

// ASCIIPath maps brand/logo.png → brand/logo-ascii.txt.
func ASCIIPath(imagePath string) string {
	return strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + asciiSuffix
}

// HashPath maps brand/logo.png → brand/logo-ascii.txt.sha256.
func HashPath(imagePath string) string {
	return strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + asciiHashSuffix
}

// Ensure creates or refreshes the sibling ASCII file for imagePath.
func Ensure(imagePath string) error {
	imagePath = strings.TrimSpace(imagePath)
	if imagePath == "" {
		return nil
	}
	imagePath = filepath.Clean(imagePath)

	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("banner image %q: %w", imagePath, err)
	}

	sum, err := fileSHA256(imagePath)
	if err != nil {
		return err
	}

	asciiPath := ASCIIPath(imagePath)
	hashPath := HashPath(imagePath)
	if fresh(asciiPath, hashPath, sum) {
		return nil
	}

	_ = os.Remove(asciiPath)
	_ = os.Remove(hashPath)

	art, err := aicwrap.Convert(imagePath, asciiLogoWidth)
	if err != nil {
		return fmt.Errorf("ascii-image-converter: %w", err)
	}
	art = strings.TrimRight(art, "\n")
	if err := os.MkdirAll(filepath.Dir(asciiPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for ascii logo: %w", err)
	}
	if err := os.WriteFile(asciiPath, []byte(art+"\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", asciiPath, err)
	}
	if err := os.WriteFile(hashPath, []byte(sum+"\n"), 0o644); err != nil {
		return fmt.Errorf("write logo hash: %w", err)
	}
	return nil
}

func fresh(asciiPath, hashPath, wantSum string) bool {
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
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
