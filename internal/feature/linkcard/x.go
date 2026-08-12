package linkcard

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var xStatusRE = regexp.MustCompile(`(?i)^/(?:#!/)?(?:\w+)/status(?:es)?/(\d+)`)

func (c *Cards) resolveX(ctx context.Context, raw string, u *url.URL) (Card, error) {
	id := xStatusID(u)
	if id == "" {
		return c.resolveOGP(ctx, raw)
	}
	pageURL := "https://x.com/i/status/" + id
	endpoint := "https://publish.twitter.com/oembed?omit_script=true&dnt=true&url=" + url.QueryEscape(pageURL)
	body, _, err := c.fetchBytes(ctx, endpoint)
	if err != nil {
		return Card{}, err
	}
	var payload struct {
		URL          string `json:"url"`
		AuthorName   string `json:"author_name"`
		AuthorURL    string `json:"author_url"`
		HTML         string `json:"html"`
		ProviderName string `json:"provider_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Card{}, err
	}
	text := stripHTMLText(payload.HTML)
	card := Card{
		URL:         fallback(payload.URL, pageURL),
		Provider:    ProviderX,
		Title:       fallback(payload.AuthorName, "X"),
		Description: truncate(text, 280),
		SiteName:    fallback(payload.ProviderName, "X"),
		OK:          true,
	}
	card.HTML = renderXCard(card)
	return card, nil
}

func xStatusID(u *url.URL) string {
	m := xStatusRE.FindStringSubmatch(u.Path)
	if m == nil {
		return ""
	}
	return m[1]
}

func stripHTMLText(fragment string) string {
	z := html.NewTokenizer(strings.NewReader(fragment))
	var b strings.Builder
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return collapseSpace(b.String())
		case html.TextToken:
			b.WriteString(string(z.Text()))
		case html.StartTagToken, html.SelfClosingTagToken:
			tn, _ := z.TagName()
			switch string(tn) {
			case "br", "p", "div":
				b.WriteByte(' ')
			}
		}
	}
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
