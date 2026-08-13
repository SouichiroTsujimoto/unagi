package session

import (
	"net/http"
	"strings"
	"time"

	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
	"github.com/labstack/echo/v4"
)

func User(c echo.Context, auth *featureauth.Auth) (featureauth.User, error) {
	if auth == nil {
		return featureauth.User{}, featureauth.ErrUnauthorized
	}
	cookie, err := c.Cookie(featureauth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return featureauth.User{}, featureauth.ErrUnauthorized
	}
	return auth.ParseAccessToken(c.Request().Context(), cookie.Value)
}

func RequireAllowedOrigin(c echo.Context, auth *featureauth.Auth) error {
	origin := c.Request().Header.Get("Origin")
	if origin == "" {
		return nil
	}
	if auth == nil || !auth.ValidOrigin(origin) {
		return echo.NewHTTPError(http.StatusForbidden, "invalid origin")
	}
	return nil
}

func ClearCookie(c echo.Context, name string, secure bool) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}
