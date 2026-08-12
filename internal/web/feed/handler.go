package feed

import (
	"encoding/xml"
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
}

func New(articles *article.Articles, site layout.Site, log *slog.Logger) *Handler {
	return &Handler{articles: articles, site: site, log: log}
}

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate,omitempty"`
	Description string `xml:"description,omitempty"`
}

func (handler *Handler) Show(c echo.Context) error {
	items, err := handler.articles.List(c.Request().Context(), time.Now())
	if err != nil {
		handler.log.Error("list articles", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	feedItems := make([]rssItem, 0, len(items))
	var lastBuild time.Time
	for _, post := range items {
		link := handler.site.AbsoluteURL(post.Path())
		item := rssItem{
			Title:       displayTitle(post),
			Link:        link,
			GUID:        link,
			Description: post.Summary,
		}
		if !post.PublishedAt.IsZero() {
			item.PubDate = post.PublishedAt.UTC().Format(time.RFC1123Z)
			if post.PublishedAt.After(lastBuild) {
				lastBuild = post.PublishedAt
			}
		}
		feedItems = append(feedItems, item)
	}

	doc := rss{
		Version: "2.0",
		Channel: rssChannel{
			Title:       handler.site.Name,
			Link:        handler.site.AbsoluteURL("/"),
			Description: handler.site.Description,
			Language:    "ja",
			Items:       feedItems,
		},
	}
	if !lastBuild.IsZero() {
		doc.Channel.LastBuildDate = lastBuild.UTC().Format(time.RFC1123Z)
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/rss+xml; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	if _, err := c.Response().Write([]byte(xml.Header)); err != nil {
		return err
	}
	enc := xml.NewEncoder(c.Response())
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		handler.log.Error("encode feed", "err", err)
		return err
	}
	return nil
}

func displayTitle(post article.Article) string {
	if post.Emoji == "" {
		return post.Title
	}
	return post.Emoji + " " + post.Title
}
