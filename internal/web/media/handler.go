package media

import (
	"encoding/json"
	"errors"
	"io"
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

type signRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type completeRequest struct {
	ObjectKey string `json:"objectKey"`
}

func (h *Handler) SignUpload(c echo.Context) error {
	var req signRequest
	if err := decodeJSON(c, &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	result, err := h.library.BeginUpload(c.Request().Context(), req.Filename, req.ContentType, req.SizeBytes)
	if errors.Is(err, featuremedia.ErrTooLarge) {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file too large")
	}
	if errors.Is(err, featuremedia.ErrInvalidType) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported image type")
	}
	if err != nil {
		h.log.Error("sign media upload", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"objectKey":   result.ObjectKey,
		"signedUrl":   result.SignedURL,
		"token":       result.Token,
		"contentType": result.ContentType,
		"url":         result.MarkdownURL,
	})
}

func (h *Handler) CompleteUpload(c echo.Context) error {
	var req completeRequest
	if err := decodeJSON(c, &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	result, err := h.library.CompleteUpload(c.Request().Context(), req.ObjectKey)
	if errors.Is(err, featuremedia.ErrTooLarge) {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file too large")
	}
	if errors.Is(err, featuremedia.ErrInvalidType) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported image type")
	}
	if errors.Is(err, featuremedia.ErrInvalidObject) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid object key")
	}
	if errors.Is(err, featuremedia.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "object not found")
	}
	if err != nil {
		h.log.Error("complete media upload", "err", err)
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

func decodeJSON(c echo.Context, dest any) error {
	dec := json.NewDecoder(c.Request().Body)
	if err := dec.Decode(dest); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
