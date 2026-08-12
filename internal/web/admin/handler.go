package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/adminauth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

const csrfHeader = "X-CSRF-Token"

type Handler struct {
	auth       *adminauth.Auth
	articles   *article.Articles
	engagement *engagement.Engagement
	site       layout.Site
	log        *slog.Logger
}

func New(auth *adminauth.Auth, articles *article.Articles, eng *engagement.Engagement, site layout.Site, log *slog.Logger) *Handler {
	return &Handler{auth: auth, articles: articles, engagement: eng, site: site, log: log}
}

func (h *Handler) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := h.sessionFromRequest(c)
		if err != nil {
			if wantsJSON(c) {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}
			return c.Redirect(http.StatusSeeOther, "/admin/login")
		}
		c.Set("admin_session", sess)
		return next(c)
	}
}

func (h *Handler) RequireCSRF(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodHead {
			return next(c)
		}
		origin := c.Request().Header.Get("Origin")
		if origin != "" && !h.auth.ValidOrigin(origin) {
			return echo.NewHTTPError(http.StatusForbidden, "invalid origin")
		}
		sess, _ := c.Get("admin_session").(adminauth.Session)
		token := c.Request().Header.Get(csrfHeader)
		if token == "" {
			token = c.FormValue("csrf")
		}
		if token == "" || token != sess.CSRFToken {
			return echo.NewHTTPError(http.StatusForbidden, "invalid csrf token")
		}
		return next(c)
	}
}

func (h *Handler) sessionFromRequest(c echo.Context) (adminauth.Session, error) {
	cookie, err := c.Cookie(adminauth.CookieName)
	if err != nil || cookie.Value == "" {
		return adminauth.Session{}, adminauth.ErrUnauthorized
	}
	return h.auth.LookupSession(c.Request().Context(), cookie.Value)
}

func wantsJSON(c echo.Context) bool {
	return strings.Contains(c.Request().Header.Get("Accept"), "application/json") ||
		strings.HasPrefix(c.Path(), "/api/")
}

func (h *Handler) setSessionCookie(c echo.Context, raw string) {
	c.SetCookie(&http.Cookie{
		Name:     adminauth.CookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.auth.SecureCookies(),
		Expires:  time.Now().Add(h.auth.SessionTTL()),
	})
}

func (h *Handler) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     adminauth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
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
	if _, err := h.sessionFromRequest(c); err == nil {
		return c.Redirect(http.StatusSeeOther, "/admin")
	}
	need, _ := h.auth.NeedsBootstrap(c.Request().Context())
	if need {
		return c.Redirect(http.StatusSeeOther, "/admin/setup")
	}
	return h.render(c, LoginPage(h.site))
}

func (h *Handler) SetupPage(c echo.Context) error {
	need, err := h.auth.NeedsBootstrap(c.Request().Context())
	if err != nil {
		return err
	}
	if !need {
		return c.Redirect(http.StatusSeeOther, "/admin/login")
	}
	return h.render(c, SetupPage(h.site))
}

func (h *Handler) Index(c echo.Context) error {
	items, err := h.articles.ListAll(c.Request().Context())
	if err != nil {
		h.log.Error("list all articles", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	sess := c.Get("admin_session").(adminauth.Session)
	return h.render(c, IndexPage(h.site, items, sess.CSRFToken))
}

func (h *Handler) NewArticlePage(c echo.Context) error {
	sess := c.Get("admin_session").(adminauth.Session)
	return h.render(c, EditPage(h.site, article.Article{Type: "tech"}, sess.CSRFToken, true))
}

func (h *Handler) EditArticlePage(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	item, err := h.articles.GetByID(c.Request().Context(), id)
	if errors.Is(err, article.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err != nil {
		return err
	}
	sess := c.Get("admin_session").(adminauth.Session)
	return h.render(c, EditPage(h.site, item, sess.CSRFToken, false))
}

func (h *Handler) PasskeysPage(c echo.Context) error {
	creds, err := h.auth.ListCredentials(c.Request().Context())
	if err != nil {
		return err
	}
	sess := c.Get("admin_session").(adminauth.Session)
	return h.render(c, PasskeysPage(h.site, creds, sess.CSRFToken))
}

func (h *Handler) Logout(c echo.Context) error {
	if cookie, err := c.Cookie(adminauth.CookieName); err == nil {
		_ = h.auth.DestroySession(c.Request().Context(), cookie.Value)
	}
	h.clearSessionCookie(c)
	return c.Redirect(http.StatusSeeOther, "/admin/login")
}
