package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	auth *featureauth.Auth
	site layout.Site
	log  *slog.Logger
}

func New(a *featureauth.Auth, site layout.Site, log *slog.Logger) *Handler {
	return &Handler{auth: a, site: site, log: log}
}

func (h *Handler) Login(c echo.Context) error {
	returnTo := c.QueryParam("return_to")
	if returnTo == "" {
		returnTo = c.QueryParam("returnTo")
	}
	authorizeURL, verifier, err := h.auth.StartXOAuth(returnTo)
	if err != nil {
		h.log.Error("start x oauth", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "oauth unavailable")
	}
	encoded, err := featureauth.EncodePKCEPayload(verifier, returnTo)
	if err != nil {
		return err
	}
	h.setPKCECookie(c, encoded)
	return c.Redirect(http.StatusFound, authorizeURL)
}

func (h *Handler) Callback(c echo.Context) error {
	if errMsg := c.QueryParam("error"); errMsg != "" {
		h.log.Warn("oauth error", "error", errMsg, "desc", c.QueryParam("error_description"))
		return echo.NewHTTPError(http.StatusBadRequest, "oauth failed")
	}
	code := c.QueryParam("code")
	pkceCookie, err := c.Cookie(featureauth.PKCECookieName)
	if err != nil || pkceCookie.Value == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing oauth state")
	}
	verifier, returnTo, err := featureauth.DecodePKCEPayload(pkceCookie.Value)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid oauth state")
	}
	h.clearPKCECookie(c)
	user, err := h.auth.FinishXOAuth(c.Request().Context(), code, verifier)
	if err != nil {
		if errors.Is(err, featureauth.ErrInvalidState) {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid oauth state")
		}
		h.log.Error("finish x oauth", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, "oauth failed")
	}
	h.setSessionCookie(c, user)
	return c.Redirect(http.StatusSeeOther, returnTo)
}

func (h *Handler) Logout(c echo.Context) error {
	h.clearSessionCookie(c)
	returnTo := featureauth.SafeReturnTo(c.QueryParam("return_to"))
	if c.Request().Method == http.MethodPost {
		return c.NoContent(http.StatusNoContent)
	}
	return c.Redirect(http.StatusSeeOther, returnTo)
}

func (h *Handler) setSessionCookie(c echo.Context, user featureauth.User) {
	maxAge := int(h.auth.SessionTTL().Seconds())
	if !user.ExpiresAt.IsZero() {
		sec := int(time.Until(user.ExpiresAt).Seconds())
		if sec > 0 {
			maxAge = sec
		}
	}
	c.SetCookie(&http.Cookie{
		Name:     featureauth.CookieName,
		Value:    user.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.auth.SecureCookies(),
		MaxAge:   maxAge,
	})
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

func (h *Handler) setPKCECookie(c echo.Context, value string) {
	c.SetCookie(&http.Cookie{
		Name:     featureauth.PKCECookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.auth.SecureCookies(),
		MaxAge:   featureauth.PKCECookieMaxAgeSeconds(),
	})
}

func (h *Handler) clearPKCECookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     featureauth.PKCECookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.auth.SecureCookies(),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// SessionFromRequest is used by tests and handlers sharing the cookie.
func (h *Handler) SessionFromRequest(c echo.Context) (featureauth.User, error) {
	cookie, err := c.Cookie(featureauth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return featureauth.User{}, featureauth.ErrUnauthorized
	}
	return h.auth.ParseAccessToken(cookie.Value)
}
