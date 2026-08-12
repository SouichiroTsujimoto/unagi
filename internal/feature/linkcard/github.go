package linkcard

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	githubBlobRE  = regexp.MustCompile(`(?i)^/([^/]+)/([^/]+)/blob/([^/]+)/(.+)$`)
	githubLinesRE = regexp.MustCompile(`(?i)^L(\d+)(?:-L(\d+))?$`)
)

func (c *Cards) resolveGitHub(ctx context.Context, raw string, u *url.URL) (Card, error) {
	// Line-anchored file URLs keep a code embed; everything else is an OGP blog card.
	if m := githubBlobRE.FindStringSubmatch(u.Path); m != nil {
		start, end := parseGitHubLines(u.Fragment)
		if start > 0 {
			owner, repo, ref, filePath := m[1], m[2], m[3], m[4]
			code, err := c.fetchGitHubRaw(ctx, owner, repo, ref, filePath, start, end)
			if err == nil && strings.TrimSpace(code) != "" {
				title := fmt.Sprintf("%s/%s/%s", owner, repo, filePath)
				if end > start {
					title = fmt.Sprintf("%s#L%d-L%d", title, start, end)
				} else {
					title = fmt.Sprintf("%s#L%d", title, start)
				}
				card := Card{
					URL:      raw,
					Provider: ProviderGitHub,
					Title:    title,
					SiteName: "GitHub",
					OK:       true,
				}
				lang := strings.TrimPrefix(path.Ext(filePath), ".")
				card.HTML = renderGitHubSnippet(card, code, lang)
				return card, nil
			}
		}
	}

	pageURL := raw
	if i := strings.Index(pageURL, "#"); i >= 0 {
		pageURL = pageURL[:i]
	}
	card, err := c.resolveOGP(ctx, pageURL)
	if err != nil {
		return Card{}, err
	}
	card.Provider = ProviderGitHub
	card.URL = fallback(card.URL, pageURL)
	card.SiteName = fallback(card.SiteName, "GitHub")
	card.HTML = renderOGPCard(card)
	return card, nil
}

func (c *Cards) fetchGitHubRaw(ctx context.Context, owner, repo, ref, filePath string, start, end int) (string, error) {
	seg := strings.Split(filePath, "/")
	for i, s := range seg {
		seg[i] = url.PathEscape(s)
	}
	rawURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/%s/%s",
		url.PathEscape(owner),
		url.PathEscape(repo),
		url.PathEscape(ref),
		strings.Join(seg, "/"),
	)
	body, _, err := c.fetchBytes(ctx, rawURL)
	if err != nil {
		return "", err
	}
	text := string(body)
	if start <= 0 {
		lines := strings.Split(text, "\n")
		if len(lines) > 40 {
			text = strings.Join(lines[:40], "\n") + "\n…"
		}
		return text, nil
	}
	lines := strings.Split(text, "\n")
	if start > len(lines) {
		return "", fmt.Errorf("start line out of range")
	}
	if end < start {
		end = start
	}
	if end > len(lines) {
		end = len(lines)
	}
	if end-start > 80 {
		end = start + 80
	}
	return strings.Join(lines[start-1:end], "\n"), nil
}

func parseGitHubLines(fragment string) (start, end int) {
	fragment = strings.TrimSpace(fragment)
	m := githubLinesRE.FindStringSubmatch(fragment)
	if m == nil {
		return 0, 0
	}
	start, _ = strconv.Atoi(m[1])
	if m[2] != "" {
		end, _ = strconv.Atoi(m[2])
	} else {
		end = start
	}
	return start, end
}
