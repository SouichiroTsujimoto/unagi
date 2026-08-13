package ogimage

import (
	"errors"
	"log/slog"
	"net/http"
	"path"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	featureog "github.com/SouichiroTsujimoto/unagi/internal/feature/ogimage"
	"github.com/labstack/echo/v4"
)

const immutableCacheControl = "public, max-age=31536000, immutable"

type Handler struct {
	images   *featureog.Images
	articles *article.Articles
	log      *slog.Logger
	now      func() time.Time
}

func New(images *featureog.Images, articles *article.Articles, log *slog.Logger) *Handler {
	return &Handler{images: images, articles: articles, log: log, now: time.Now}
}

func (h *Handler) Show(c echo.Context) error {
	item, err := h.articles.Get(c.Request().Context(), c.Param("slug"), h.now())
	if errors.Is(err, article.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "article not found")
	}
	if err != nil {
		h.log.Error("get article for OGP", "err", err, "slug", c.Param("slug"))
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	if path.Base(featureog.Path(item)) != c.Param("version") {
		return echo.NewHTTPError(http.StatusNotFound, "OGP version not found")
	}
	body, err := h.images.RenderPNG(item)
	if err != nil {
		h.log.Error("render article OGP", "err", err, "slug", item.Slug)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	c.Response().Header().Set(echo.HeaderContentType, "image/png")
	c.Response().Header().Set(echo.HeaderCacheControl, immutableCacheControl)
	c.Response().Header().Set("Vercel-CDN-Cache-Control", "max-age=31536000")
	return c.Blob(http.StatusOK, "image/png", body)
}
