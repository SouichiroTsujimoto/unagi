package sitemap

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
)

type Handler struct {
	articles *article.Articles
	site     layout.Site
	log      *slog.Logger
}

func New(articles *article.Articles, site layout.Site, log *slog.Logger) *Handler {
	return &Handler{articles: articles, site: site, log: log}
}

type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []url    `xml:"url"`
}

type url struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
}

func (handler *Handler) Show(c echo.Context) error {
	ctx := c.Request().Context()
	now := time.Now()
	entries := []url{
		{Loc: handler.site.AbsoluteURL("/"), ChangeFreq: "weekly"},
		{Loc: handler.site.AbsoluteURL("/about"), ChangeFreq: "monthly"},
	}
	posts, err := handler.articles.List(ctx, now)
	if err != nil {
		handler.log.Error("list articles", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	for _, post := range posts {
		entry := url{Loc: handler.site.AbsoluteURL(post.Path()), ChangeFreq: "monthly"}
		if !post.PublishedAt.IsZero() {
			entry.LastMod = post.PublishedAt.Format("2006-01-02")
		}
		entries = append(entries, entry)
	}
	topics, err := handler.articles.Topics(ctx, now)
	if err != nil {
		handler.log.Error("list topics", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	for _, topic := range topics {
		entries = append(entries, url{
			Loc:        handler.site.AbsoluteURL(article.TopicPath(topic)),
			ChangeFreq: "weekly",
		})
	}

	doc := urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/xml; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	if _, err := c.Response().Write([]byte(xml.Header)); err != nil {
		return err
	}
	enc := xml.NewEncoder(c.Response())
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		handler.log.Error("encode sitemap", "err", err)
		return err
	}
	return nil
}
