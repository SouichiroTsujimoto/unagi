package article

import (
	"errors"
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
	slug := c.Param("slug")
	post, err := handler.articles.Get(c.Request().Context(), slug, handler.now())
	if errors.Is(err, article.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "article not found")
	}
	if err != nil {
		handler.log.Error("get article", "err", err, "slug", slug)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	meta := layout.PageMeta{
		Title:       handler.site.TitleWithSite(post.Title),
		Description: firstNonEmpty(post.Summary, handler.site.Description),
		Canonical:   handler.site.AbsoluteURL(post.Path()),
		OGImage:     handler.site.AbsoluteURL("/static/wuhu1sland-1.png"),
		OGType:      "article",
		Active:      "home",
		Wide:        true,
		Preconnect:  handler.site.MediaOrigin,
		Assets: layout.AssetArticleShare |
			layout.AssetArticleLinkcards |
			layout.AssetArticleEngagement,
	}
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	if err := ShowPage(meta, post).Render(c.Request().Context(), c.Response()); err != nil {
		handler.log.Error("render article", "err", err, "slug", slug)
		return err
	}
	return nil
}

func (handler *Handler) ListByTag(c echo.Context) error {
	tag := c.Param("tag")
	items, err := handler.articles.ListByTopic(c.Request().Context(), tag, handler.now())
	if err != nil {
		handler.log.Error("list by tag", "err", err, "tag", tag)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	if len(items) == 0 {
		ok, err := handler.articles.TopicExists(c.Request().Context(), tag)
		if err != nil {
			handler.log.Error("topic exists", "err", err, "tag", tag)
			return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
		}
		if !ok {
			return echo.NewHTTPError(http.StatusNotFound, "tag not found")
		}
	}

	meta := layout.PageMeta{
		Title:       handler.site.TitleWithSite(tag),
		Description: tag + "に関する記事一覧",
		Canonical:   handler.site.AbsoluteURL(article.TopicPath(tag)),
		OGImage:     handler.site.AbsoluteURL("/static/wuhu1sland-1.png"),
		OGType:      "website",
		Active:      "home",
	}
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	if err := TagPage(meta, tag, items).Render(c.Request().Context(), c.Response()); err != nil {
		handler.log.Error("render tag", "err", err, "tag", tag)
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
