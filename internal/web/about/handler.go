package about

import (
	"log/slog"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	log *slog.Logger
}

func New(log *slog.Logger) *Handler {
	return &Handler{log: log}
}

func (handler *Handler) Show(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	if err := Page().Render(c.Request().Context(), c.Response()); err != nil {
		handler.log.Error("render about", "err", err)
		return err
	}
	return nil
}
