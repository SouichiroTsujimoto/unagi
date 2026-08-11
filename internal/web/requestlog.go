package web

import (
	"log/slog"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

func requestLog() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if strings.HasPrefix(path, "/static/") {
				return next(c)
			}

			start := time.Now()
			err := next(c)
			status := c.Response().Status
			if status == 0 && err != nil {
				status = 500
			}
			slog.Info("request",
				"method", c.Request().Method,
				"path", path,
				"status", status,
				"duration", time.Since(start).Round(time.Millisecond).String(),
			)
			return err
		}
	}
}
