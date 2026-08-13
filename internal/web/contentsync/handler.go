package contentsync

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	feature "github.com/SouichiroTsujimoto/unagi/internal/feature/contentsync"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	sync *feature.Sync
	log  *slog.Logger
}

func New(sync *feature.Sync, log *slog.Logger) *Handler {
	return &Handler{sync: sync, log: log}
}

func (h *Handler) DryRun(c echo.Context) error {
	snap, err := h.authorizedSnapshot(c)
	if err != nil {
		return err
	}
	result, err := h.sync.DryRun(c.Request().Context(), snap)
	return h.writeResult(c, result, err)
}

func (h *Handler) Apply(c echo.Context) error {
	snap, err := h.authorizedSnapshot(c)
	if err != nil {
		return err
	}
	result, err := h.sync.Apply(c.Request().Context(), snap)
	return h.writeResult(c, result, err)
}

func (h *Handler) Images(c echo.Context) error {
	snap, err := h.authorizedSnapshot(c)
	if err != nil {
		return err
	}
	uploads, err := h.sync.PlanUploads(c.Request().Context(), snap)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"uploads": uploads})
}

func (h *Handler) authorizedSnapshot(c echo.Context) (feature.Snapshot, error) {
	if h.sync == nil || !h.sync.Configured() {
		return feature.Snapshot{}, echo.NewHTTPError(http.StatusServiceUnavailable, "content sync is not configured")
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 8<<20))
	if err != nil {
		return feature.Snapshot{}, echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	req := c.Request()
	err = feature.VerifyHMAC(
		h.sync.Secret(),
		req.Method,
		req.URL.Path,
		req.Header.Get(feature.HeaderTimestamp),
		req.Header.Get(feature.HeaderRunID),
		req.Header.Get(feature.HeaderRepository),
		req.Header.Get(feature.HeaderSignature),
		body,
		time.Now(),
	)
	if err != nil {
		return feature.Snapshot{}, mapError(err)
	}
	var snap feature.Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		return feature.Snapshot{}, echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	if snap.RunID == "" {
		snap.RunID = req.Header.Get(feature.HeaderRunID)
	}
	if snap.Repository == "" {
		snap.Repository = req.Header.Get(feature.HeaderRepository)
	}
	return snap, nil
}

func (h *Handler) writeResult(c echo.Context, result feature.Result, err error) error {
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func mapError(err error) error {
	switch {
	case errors.Is(err, feature.ErrNotConfigured):
		return echo.NewHTTPError(http.StatusServiceUnavailable, "content sync is not configured")
	case errors.Is(err, feature.ErrUnauthorized):
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, feature.ErrForbiddenRepo):
		return echo.NewHTTPError(http.StatusForbidden, "repository is not allowed")
	case errors.Is(err, feature.ErrStaleTimestamp):
		return echo.NewHTTPError(http.StatusUnauthorized, "timestamp expired")
	case errors.Is(err, feature.ErrDuplicateRun):
		return echo.NewHTTPError(http.StatusConflict, "run already applied")
	case errors.Is(err, feature.ErrMissingImage):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, feature.ErrInvalidSnapshot):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
}
