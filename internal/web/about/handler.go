package about

import (
	"log/slog"

	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	site layout.Site
	log  *slog.Logger
}

func New(site layout.Site, log *slog.Logger) *Handler {
	return &Handler{site: site, log: log}
}

func (handler *Handler) Show(c echo.Context) error {
	meta := layout.PageMeta{
		Title:       handler.site.TitleWithSite("Me"),
		Description: handler.site.Description,
		Canonical:   handler.site.AbsoluteURL("/about"),
		OGImage:     handler.site.AbsoluteURL("/static/wuhu1sland-2.webp"),
		OGType:      "website",
		SiteName:    handler.site.Name,
		Active:      "about",
	}
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	if err := Page(meta, handler.site).Render(c.Request().Context(), c.Response()); err != nil {
		handler.log.Error("render about", "err", err)
		return err
	}
	return nil
}
