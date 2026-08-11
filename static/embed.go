package static

import (
	"embed"
	"io/fs"
)

//go:embed app.css theme.js go-chan-*.png vendor
var files embed.FS

func FS() fs.FS {
	return files
}
