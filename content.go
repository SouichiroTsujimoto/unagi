// Package content embeds Zenn-compatible article Markdown and images.
package content

import (
	"embed"
	"io/fs"
)

//go:embed articles/*.md
//go:embed images
var files embed.FS

// FS returns the embedded content filesystem (articles/ and images/).
func FS() embed.FS {
	return files
}

// Articles returns the articles/ subtree.
func Articles() (fs.FS, error) {
	return fs.Sub(files, "articles")
}

// Images returns the images/ subtree.
func Images() (fs.FS, error) {
	return fs.Sub(files, "images")
}
