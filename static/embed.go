package static

import (
	"embed"
	"io/fs"
)

//go:embed app.css theme.js *.png vendor
var files embed.FS

func FS() fs.FS {
	return files
}
