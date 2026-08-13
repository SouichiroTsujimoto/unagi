package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	auth       *featureauth.Auth
	articles   *article.Articles
	engagement *engagement.Engagement
	site       layout.Site
	log        *slog.Logger
}

func New(auth *featureauth.Auth, articles *article.Articles, eng *engagement.Engagement, site layout.Site, log *slog.Logger) *Handler {
	return &Handler{auth: auth, articles: articles, engagement: eng, site: site, log: log}
}

func (h *Handler) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := h.userFromRequest(c)
		if err != nil {
			if wantsJSON(c) {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			return c.Redirect(http.StatusSeeOther, "/admin/login")
		}
		if !user.IsAdmin {
			if wantsJSON(c) {
				return echo.NewHTTPError(http.StatusForbidden, "forbidden")
			}
			return h.forbidden(c)
		}
		c.Set("admin_user", user)
		return next(c)
	}
}

// RequireOrigin rejects cross-site mutating requests (SameSite=Lax cookie companion).
func (h *Handler) RequireOrigin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodHead {
			return next(c)
		}
		origin := c.Request().Header.Get("Origin")
		if origin != "" && !h.auth.ValidOrigin(origin) {
			return echo.NewHTTPError(http.StatusForbidden, "invalid origin")
		}
		return next(c)
	}
}

func (h *Handler) userFromRequest(c echo.Context) (featureauth.User, error) {
	cookie, err := c.Cookie(featureauth.CookieName)
	if err != nil || cookie.Value == "" {
		return featureauth.User{}, featureauth.ErrUnauthorized
	}
	return h.auth.ParseAccessToken(c.Request().Context(), cookie.Value)
}

func wantsJSON(c echo.Context) bool {
	return strings.Contains(c.Request().Header.Get("Accept"), "application/json") ||
		strings.HasPrefix(c.Path(), "/api/")
}

func (h *Handler) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     featureauth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.auth.SecureCookies(),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (h *Handler) render(c echo.Context, component templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	return component.Render(c.Request().Context(), c.Response())
}

func (h *Handler) LoginPage(c echo.Context) error {
	user, err := h.userFromRequest(c)
	if err == nil && user.IsAdmin {
		return c.Redirect(http.StatusSeeOther, "/admin")
	}
	if err == nil {
		return h.forbidden(c)
	}
	return h.render(c, LoginPage(h.site))
}

func (h *Handler) forbidden(c echo.Context) error {
	c.Response().Status = http.StatusForbidden
	return h.render(c, ForbiddenPage(h.site))
}

func (h *Handler) Index(c echo.Context) error {
	items, err := h.articles.ListAll(c.Request().Context())
	if err != nil {
		h.log.Error("list all articles", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return h.render(c, IndexPage(h.site, items))
}

func (h *Handler) ArticlePage(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	item, err := h.articles.GetByID(c.Request().Context(), id)
	if errors.Is(err, article.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err != nil {
		return err
	}
	return h.render(c, ManagePage(h.site, item))
}

func (h *Handler) Logout(c echo.Context) error {
	h.clearSessionCookie(c)
	return c.Redirect(http.StatusSeeOther, "/admin/login")
}
