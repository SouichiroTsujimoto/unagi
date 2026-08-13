package engagement

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/web/session"
)

type Handler struct {
	engagement *engagement.Engagement
	auth       *featureauth.Auth
	log        *slog.Logger
	now        func() time.Time
}

func New(eng *engagement.Engagement, a *featureauth.Auth, log *slog.Logger) *Handler {
	return &Handler{
		engagement: eng,
		auth:       a,
		log:        log,
		now:        time.Now,
	}
}

func (h *Handler) Get(c echo.Context) error {
	viewer := h.viewerFromRequest(c)
	snap, err := h.engagement.GetSnapshot(c.Request().Context(), c.Param("slug"), h.now(), viewer)
	if errors.Is(err, engagement.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "article not found")
	}
	if err != nil {
		h.log.Error("get engagement", "err", err, "slug", c.Param("slug"))
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, snap)
}

type emojiStickerBody struct {
	Emoji string  `json:"emoji"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

func (h *Handler) AddEmojiSticker(c echo.Context) error {
	if err := h.requireAllowedOrigin(c); err != nil {
		return err
	}
	var body emojiStickerBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	sticker, err := h.engagement.AddEmojiSticker(c.Request().Context(), c.Param("slug"), h.now(), engagement.AddEmojiInput{
		Emoji: body.Emoji,
		X:     body.X,
		Y:     body.Y,
	})
	return h.mapWriteError(c, err, sticker, nil)
}

type avatarStickerBody struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (h *Handler) AddAvatarSticker(c echo.Context) error {
	if err := h.requireAllowedOrigin(c); err != nil {
		return err
	}
	author, err := h.requireAuthor(c)
	if err != nil {
		return err
	}
	var body avatarStickerBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	sticker, err := h.engagement.AddAvatarSticker(c.Request().Context(), c.Param("slug"), h.now(), author, body.X, body.Y)
	return h.mapWriteError(c, err, sticker, nil)
}

type commentBody struct {
	Body string `json:"body"`
}

func (h *Handler) AddComment(c echo.Context) error {
	if err := h.requireAllowedOrigin(c); err != nil {
		return err
	}
	author, err := h.requireAuthor(c)
	if err != nil {
		return err
	}
	var body commentBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	comment, err := h.engagement.AddComment(c.Request().Context(), c.Param("slug"), h.now(), author, body.Body)
	return h.mapWriteError(c, err, engagement.Sticker{}, &comment)
}

func (h *Handler) DeleteOwnComment(c echo.Context) error {
	if err := h.requireAllowedOrigin(c); err != nil {
		return err
	}
	author, err := h.requireAuthor(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid comment id")
	}
	err = h.engagement.DeleteOwnComment(c.Request().Context(), c.Param("slug"), h.now(), author, id)
	switch {
	case err == nil:
		return c.NoContent(http.StatusNoContent)
	case errors.Is(err, engagement.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "comment not found")
	case errors.Is(err, engagement.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, engagement.ErrLoginRequired):
		return h.loginRequired(c)
	default:
		h.log.Error("delete own comment", "err", err, "slug", c.Param("slug"), "comment_id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
}

func (h *Handler) requireAuthor(c echo.Context) (engagement.Author, error) {
	user, err := h.userFromRequest(c)
	if errors.Is(err, featureauth.ErrUnauthorized) {
		return engagement.Author{}, h.loginRequired(c)
	}
	if err != nil {
		h.log.Error("lookup auth session", "err", err)
		return engagement.Author{}, echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return engagement.Author{
		XUserID:     user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
	}, nil
}

func (h *Handler) viewerFromRequest(c echo.Context) *engagement.Viewer {
	user, err := h.userFromRequest(c)
	if err != nil {
		return nil
	}
	return &engagement.Viewer{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		XUserID:     user.ID,
	}
}

func (h *Handler) userFromRequest(c echo.Context) (featureauth.User, error) {
	return session.User(c, h.auth)
}

func (h *Handler) loginRequired(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, map[string]any{
		"error":     "login_required",
		"message":   "Xアカウントでログインしてください",
		"loginPath": engagement.LoginPath,
	})
}

func (h *Handler) mapWriteError(c echo.Context, err error, sticker engagement.Sticker, comment *engagement.Comment) error {
	switch {
	case err == nil && comment != nil:
		return c.JSON(http.StatusCreated, *comment)
	case err == nil:
		return c.JSON(http.StatusCreated, sticker)
	case errors.Is(err, engagement.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "article not found")
	case errors.Is(err, engagement.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, engagement.ErrLimitExceeded):
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	case errors.Is(err, engagement.ErrLoginRequired):
		return h.loginRequired(c)
	default:
		h.log.Error("write engagement", "err", err, "slug", c.Param("slug"))
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
}

func (h *Handler) requireAllowedOrigin(c echo.Context) error {
	return session.RequireAllowedOrigin(c, h.auth)
}
