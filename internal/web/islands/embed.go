package islands

import (
	"embed"
	"io/fs"
)

//go:embed *.js
var files embed.FS

func FS() fs.FS {
	return files
}
