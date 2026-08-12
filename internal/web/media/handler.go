package media

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	featuremedia "github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	library *featuremedia.Library
	log     *slog.Logger
}

func New(library *featuremedia.Library, log *slog.Logger) *Handler {
	return &Handler{library: library, log: log}
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
