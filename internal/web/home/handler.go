package home

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	articles *article.Articles
	site     layout.Site
	log      *slog.Logger
	now      func() time.Time
}

func New(articles *article.Articles, site layout.Site, log *slog.Logger) *Handler {
	return &Handler{
		articles: articles,
		site:     site,
		log:      log,
		now:      time.Now,
	}
}

func (handler *Handler) Show(c echo.Context) error {
	items, err := handler.articles.List(c.Request().Context(), handler.now())
	if err != nil {
		handler.log.Error("list articles", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	meta := layout.PageMeta{
		Title:       handler.site.TitleWithSite("Posts"),
		Description: handler.site.Description,
		Canonical:   handler.site.AbsoluteURL("/"),
		OGImage:     handler.site.AbsoluteURL("/static/wuhu1sland-1.png"),
		OGType:      "website",
		Active:      "home",
	}
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	if err := Page(meta, handler.site, items).Render(c.Request().Context(), c.Response()); err != nil {
		handler.log.Error("render home", "err", err)
		return err
	}
	return nil
}
