package engagement

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
)

const visitorCookie = "unagi_visitor"

type Handler struct {
	engagement *engagement.Engagement
	site       layout.Site
	log        *slog.Logger
	now        func() time.Time
}

func New(eng *engagement.Engagement, site layout.Site, log *slog.Logger) *Handler {
	return &Handler{
		engagement: eng,
		site:       site,
		log:        log,
		now:        time.Now,
	}
}

func (h *Handler) Get(c echo.Context) error {
	snap, err := h.engagement.GetSnapshot(c.Request().Context(), c.Param("slug"), h.now())
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
	visitor, err := h.ensureVisitor(c)
	if err != nil {
		h.log.Error("ensure visitor", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	sticker, err := h.engagement.AddEmojiSticker(c.Request().Context(), c.Param("slug"), h.now(), engagement.AddEmojiInput{
		Emoji:       body.Emoji,
		X:           body.X,
		Y:           body.Y,
		VisitorHash: engagement.HashVisitor(visitor),
	})
	return h.mapWriteError(c, err, sticker)
}

func (h *Handler) AddAvatarSticker(c echo.Context) error {
	if err := h.requireAllowedOrigin(c); err != nil {
		return err
	}
	_, err := h.engagement.AddAvatarSticker(c.Request().Context(), c.Param("slug"), h.now())
	return h.mapLoginError(c, err)
}

type commentBody struct {
	Body string `json:"body"`
}

func (h *Handler) AddComment(c echo.Context) error {
	if err := h.requireAllowedOrigin(c); err != nil {
		return err
	}
	var body commentBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	_, err := h.engagement.AddComment(c.Request().Context(), c.Param("slug"), h.now(), body.Body)
	return h.mapLoginError(c, err)
}

func (h *Handler) mapWriteError(c echo.Context, err error, sticker engagement.Sticker) error {
	switch {
	case err == nil:
		return c.JSON(http.StatusCreated, sticker)
	case errors.Is(err, engagement.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "article not found")
	case errors.Is(err, engagement.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, engagement.ErrLimitExceeded):
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	default:
		h.log.Error("write engagement", "err", err, "slug", c.Param("slug"))
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
}

func (h *Handler) mapLoginError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, engagement.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "article not found")
	case errors.Is(err, engagement.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, engagement.ErrLoginRequired):
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error":     "login_required",
			"message":   "Xアカウントでログインしてください",
			"loginPath": engagement.LoginPath,
		})
	default:
		h.log.Error("login-gated engagement", "err", err, "slug", c.Param("slug"))
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
}

func (h *Handler) requireAllowedOrigin(c echo.Context) error {
	origin := c.Request().Header.Get("Origin")
	if origin == "" {
		return nil
	}
	if !h.validOrigin(origin) {
		return echo.NewHTTPError(http.StatusForbidden, "invalid origin")
	}
	return nil
}

func (h *Handler) validOrigin(origin string) bool {
	base := strings.TrimSpace(h.site.BaseURL)
	if base == "" {
		return true
	}
	want, err := url.Parse(base)
	if err != nil || want.Scheme == "" || want.Host == "" {
		return false
	}
	got, err := url.Parse(origin)
	if err != nil || got.Scheme == "" || got.Host == "" {
		return false
	}
	if got.Scheme != want.Scheme {
		return false
	}
	return sameHostPort(got.Hostname(), got.Port(), want.Hostname(), want.Port(), want.Scheme)
}

func sameHostPort(gotHost, gotPort, wantHost, wantPort, scheme string) bool {
	if gotPort == "" {
		if scheme == "https" {
			gotPort = "443"
		} else {
			gotPort = "80"
		}
	}
	if wantPort == "" {
		if scheme == "https" {
			wantPort = "443"
		} else {
			wantPort = "80"
		}
	}
	if gotPort != wantPort {
		return false
	}
	if strings.EqualFold(gotHost, wantHost) {
		return true
	}
	return isLoopbackHost(gotHost) && isLoopbackHost(wantHost)
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (h *Handler) ensureVisitor(c echo.Context) (string, error) {
	if cookie, err := c.Cookie(visitorCookie); err == nil && isVisitorToken(cookie.Value) {
		return cookie.Value, nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	secure := strings.HasPrefix(strings.ToLower(h.site.BaseURL), "https://")
	c.SetCookie(&http.Cookie{
		Name:     visitorCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   60 * 60 * 24 * 400,
	})
	return token, nil
}

func isVisitorToken(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}
