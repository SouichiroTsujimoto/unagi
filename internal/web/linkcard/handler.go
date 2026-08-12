package linkcard

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/linkcard"
)

const maxBatchURLs = 20

type Handler struct {
	cards *linkcard.Cards
	log   *slog.Logger
}

func New(cards *linkcard.Cards, log *slog.Logger) *Handler {
	return &Handler{cards: cards, log: log}
}

type resolveBody struct {
	URLs []string `json:"urls"`
}

type resolveItem struct {
	URL  string `json:"url"`
	HTML string `json:"html"`
}

type resolveResponse struct {
	Cards []resolveItem `json:"cards"`
}

func (h *Handler) Resolve(c echo.Context) error {
	var body resolveBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	if len(body.URLs) == 0 {
		return c.JSON(http.StatusOK, resolveResponse{Cards: []resolveItem{}})
	}
	if len(body.URLs) > maxBatchURLs {
		return echo.NewHTTPError(http.StatusBadRequest, "too many urls")
	}

	out := make([]resolveItem, 0, len(body.URLs))
	seen := make(map[string]struct{}, len(body.URLs))
	for _, raw := range body.URLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}

		card, err := h.cards.Resolve(c.Request().Context(), raw)
		if err != nil || !card.OK || strings.TrimSpace(card.HTML) == "" {
			esc := raw
			out = append(out, resolveItem{
				URL:  raw,
				HTML: `<p><a href="` + htmlEscapeAttr(esc) + `" rel="noopener noreferrer">` + htmlEscapeAttr(esc) + `</a></p>`,
			})
			continue
		}
		out = append(out, resolveItem{URL: raw, HTML: card.HTML})
	}
	return c.JSON(http.StatusOK, resolveResponse{Cards: out})
}

func htmlEscapeAttr(s string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(s)
}
