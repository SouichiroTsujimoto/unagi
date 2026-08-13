package web

import (
	"io/fs"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

const staticImageCacheControl = "public, max-age=604800"

func staticHandler(files fs.FS) echo.HandlerFunc {
	inner := echo.StaticDirectoryHandler(files, false)
	return func(c echo.Context) error {
		if cc := staticCacheControl(c.Param("*")); cc != "" {
			c.Response().Header().Set(echo.HeaderCacheControl, cc)
		}
		return inner(c)
	}
}

func staticCacheControl(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".webp", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg":
		return staticImageCacheControl
	default:
		return ""
	}
}
