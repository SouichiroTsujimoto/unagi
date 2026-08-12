package linkcard

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func (c *Cards) resolveOGP(ctx context.Context, raw string) (Card, error) {
	body, finalURL, err := c.fetchBytes(ctx, raw)
	if err != nil {
		return Card{}, err
	}
	meta := parseOGP(body, finalURL)
	card := Card{
		URL:         fallback(meta.URL, finalURL),
		Provider:    ProviderOGP,
		Title:       meta.Title,
		Description: truncate(meta.Description, 200),
		ImageURL:    meta.Image,
		SiteName:    meta.SiteName,
		OK:          true,
	}
	if card.Title == "" {
		card.Title = hostOf(card.URL)
	}
	card.HTML = renderOGPCard(card)
	return card, nil
}

type ogpMeta struct {
	Title       string
	Description string
	Image       string
	URL         string
	SiteName    string
}

func parseOGP(body []byte, baseURL string) ogpMeta {
	var meta ogpMeta
	base, _ := url.Parse(baseURL)
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return meta
		case html.StartTagToken, html.SelfClosingTagToken:
			tn, hasAttr := z.TagName()
			name := string(tn)
			switch name {
			case "meta":
				if !hasAttr {
					continue
				}
				attrs := readAttrs(z)
				prop := strings.ToLower(attrs["property"])
				if prop == "" {
					prop = strings.ToLower(attrs["name"])
				}
				content := strings.TrimSpace(attrs["content"])
				if content == "" {
					continue
				}
				switch prop {
				case "og:title":
					if meta.Title == "" {
						meta.Title = content
					}
				case "og:description", "description":
					if meta.Description == "" {
						meta.Description = content
					}
				case "og:image", "og:image:url":
					if meta.Image == "" {
						meta.Image = absolutize(base, content)
					}
				case "og:url":
					if meta.URL == "" {
						meta.URL = absolutize(base, content)
					}
				case "og:site_name":
					if meta.SiteName == "" {
						meta.SiteName = content
					}
				case "twitter:title":
					if meta.Title == "" {
						meta.Title = content
					}
				case "twitter:description":
					if meta.Description == "" {
						meta.Description = content
					}
				case "twitter:image", "twitter:image:src":
					if meta.Image == "" {
						meta.Image = absolutize(base, content)
					}
				}
			case "title":
				if meta.Title == "" {
					if z.Next() == html.TextToken {
						meta.Title = collapseSpace(string(z.Text()))
					}
				}
			case "body":
				// Enough head metadata collected.
				if meta.Title != "" || meta.Description != "" || meta.Image != "" {
					return meta
				}
			}
		}
	}
}

func readAttrs(z *html.Tokenizer) map[string]string {
	out := make(map[string]string)
	for {
		key, val, more := z.TagAttr()
		if len(key) > 0 {
			out[strings.ToLower(string(key))] = string(val)
		}
		if !more {
			break
		}
	}
	return out
}

func absolutize(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if base == nil {
		return u.String()
	}
	return base.ResolveReference(u).String()
}
