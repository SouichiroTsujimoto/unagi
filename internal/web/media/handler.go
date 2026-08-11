package media

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	featuremedia "github.com/SouichiroTsujimoto/unagi/internal/feature/media"
)

type Handler struct {
	library *featuremedia.Library
	log     *slog.Logger
}

func New(library *featuremedia.Library, log *slog.Logger) *Handler {
	return &Handler{library: library, log: log}
}

func (h *Handler) Show(c echo.Context) error {
	key := strings.TrimPrefix(c.Param("*"), "/")
	item, rc, err := h.library.Open(c.Request().Context(), key)
	if errors.Is(err, featuremedia.ErrNotFound) || errors.Is(err, featuremedia.ErrInvalidObject) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err != nil {
		h.log.Error("open media", "err", err, "key", key)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	defer rc.Close()

	etag := `"` + item.SHA256 + `"`
	c.Response().Header().Set("ETag", etag)
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	c.Response().Header().Set(echo.HeaderContentType, item.ContentType)
	if item.SizeBytes > 0 {
		c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(item.SizeBytes, 10))
	}
	if match := c.Request().Header.Get("If-None-Match"); match == etag {
		return c.NoContent(http.StatusNotModified)
	}
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), rc)
	return err
}

func (h *Handler) Upload(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file required")
	}
	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid file")
	}
	defer src.Close()

	result, err := h.library.Upload(c.Request().Context(), file.Filename, src, file.Size)
	if errors.Is(err, featuremedia.ErrTooLarge) {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file too large")
	}
	if errors.Is(err, featuremedia.ErrInvalidType) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported image type")
	}
	if err != nil {
		h.log.Error("upload media", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"url":         result.URL,
		"objectKey":   result.Media.ObjectKey,
		"contentType": result.Media.ContentType,
		"sizeBytes":   result.Media.SizeBytes,
		"sha256":      result.Media.SHA256,
		"createdAt":   result.Media.CreatedAt.Format(time.RFC3339),
	})
}
