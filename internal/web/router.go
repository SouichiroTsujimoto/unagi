package web

import (
	"io/fs"

	"github.com/labstack/echo/v4"
	"github.com/SouichiroTsujimoto/unigo-template/internal/web/about"
	"github.com/SouichiroTsujimoto/unigo-template/internal/web/home"
)

func New(homeHandler *home.Handler, aboutHandler *about.Handler, staticFiles, islandFiles fs.FS) *echo.Echo {
	router := echo.New()
	router.HideBanner = true
	router.HidePort = true
	router.Use(requestLog())

	// More specific prefix first: islands are embedded from internal/web/islands.
	router.GET("/static/islands/*", echo.StaticDirectoryHandler(islandFiles, false))
	router.GET("/static/*", echo.StaticDirectoryHandler(staticFiles, false))
	router.GET("/", homeHandler.Show)
	router.GET("/about", aboutHandler.Show)
	router.GET("/api/accounts", homeHandler.List)
	router.POST("/api/accounts", homeHandler.Create)
	router.DELETE("/api/accounts/:id", homeHandler.Delete)

	return router
}
