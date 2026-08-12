package linkcard

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

var (
	amazonASINRE   = regexp.MustCompile(`(?i)(?:/dp/|/gp/product/|/gp/aw/d/|/exec/obidos/ASIN/|/o/ASIN/)([A-Z0-9]{10})`)
	amazonASINTok  = regexp.MustCompile(`(?i)^[A-Z0-9]{10}$`)
	amazonHiResRE  = regexp.MustCompile(`(?i)data-old-hires="(https://[^"]+)"`)
	amazonHiResJS  = regexp.MustCompile(`(?i)"hiRes"\s*:\s*"(https://[^"]+)"`)
	amazonTitleSep = regexp.MustCompile(`\s*[|：:]\s*`)
)

func (c *Cards) resolveAmazon(ctx context.Context, raw string, u *url.URL) (Card, error) {
	fetchURL := raw
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host == "amzn.to" || host == "amzn.asia" || host == "a.co" {
		_, finalURL, err := c.fetchBytes(ctx, raw)
		if err != nil {
			return Card{}, err
		}
		fetchURL = finalURL
		if parsed, err := url.Parse(finalURL); err == nil {
			u = parsed
		}
	}

	asin := amazonASIN(u)
	if asin != "" {
		fetchURL = amazonProductURL(u, asin)
		if parsed, err := url.Parse(fetchURL); err == nil {
			u = parsed
		}
	}

	body, finalURL, err := c.fetchBytes(ctx, fetchURL)
	if err != nil {
		if asin == "" {
			return Card{}, err
		}
		return amazonFallbackCard(amazonProductURL(u, asin), asin), nil
	}

	meta := parseOGP(body, finalURL)
	if meta.Image == "" {
		meta.Image = extractAmazonImage(body)
	}
	if meta.Image == "" && asin != "" {
		meta.Image = amazonASINImage(asin)
	}

	title := cleanAmazonTitle(meta.Title)
	if title == "" || strings.EqualFold(title, "Amazon.co.jp") || strings.EqualFold(title, "Amazon.com") {
		if asin != "" {
			title = "Amazon " + asin
		} else {
			title = hostOf(finalURL)
		}
	}

	card := Card{
		URL:         finalURL,
		Provider:    ProviderAmazon,
		Title:       title,
		Description: truncate(meta.Description, 200),
		ImageURL:    meta.Image,
		SiteName:    "Amazon",
		OK:          true,
	}
	if asin != "" {
		card.URL = amazonProductURL(u, asin)
	} else if meta.URL != "" {
		card.URL = meta.URL
	}
	card.HTML = renderOGPCard(card)
	return card, nil
}

func amazonFallbackCard(page, asin string) Card {
	card := Card{
		URL:      page,
		Provider: ProviderAmazon,
		Title:    "Amazon " + asin,
		ImageURL: amazonASINImage(asin),
		SiteName: "Amazon",
		OK:       true,
	}
	card.HTML = renderOGPCard(card)
	return card
}

func amazonASINImage(asin string) string {
	return "https://m.media-amazon.com/images/P/" + asin + ".jpg"
}

func extractAmazonImage(body []byte) string {
	s := string(body)
	if m := amazonHiResRE.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if m := amazonHiResJS.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func cleanAmazonTitle(title string) string {
	title = collapseSpace(title)
	if title == "" {
		return ""
	}
	parts := amazonTitleSep.Split(title, -1)
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		low := strings.ToLower(p)
		if low == "amazon" || strings.HasPrefix(low, "amazon.") || low == "通販" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	if len(cleaned) == 0 {
		return title
	}
	best := cleaned[0]
	for _, p := range cleaned[1:] {
		if len([]rune(p)) > len([]rune(best)) {
			best = p
		}
	}
	return best
}

func amazonASIN(u *url.URL) string {
	if m := amazonASINRE.FindStringSubmatch(u.Path); m != nil {
		return strings.ToUpper(m[1])
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "dp") && amazonASINTok.MatchString(parts[i+1]) {
			return strings.ToUpper(parts[i+1])
		}
	}
	return ""
}

func amazonProductURL(u *url.URL, asin string) string {
	if asin == "" {
		return u.String()
	}
	host := u.Hostname()
	if host == "" {
		host = "www.amazon.co.jp"
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + host + "/dp/" + asin
}
