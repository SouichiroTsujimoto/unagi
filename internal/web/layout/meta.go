package layout

import (
	"net/url"
	"strings"
)

// PageMeta holds document-level SEO and social metadata.
type PageMeta struct {
	Title       string
	Description string
	Canonical   string
	OGImage     string
	OGType      string // website | article
	Active      string
	Wide        bool
	Preconnect  string // origin, e.g. https://xxxx.supabase.co
}

// Site holds shared site identity used when building PageMeta.
type Site struct {
	Name        string
	Description string
	BaseURL     string
	MediaOrigin string
}

// OriginOf returns scheme://host from a URL, or empty if it is not an absolute http(s) URL.
func OriginOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// TitleWithSite appends the site name when the page title is not already the site name.
func (s Site) TitleWithSite(pageTitle string) string {
	if pageTitle == "" || pageTitle == s.Name {
		return s.Name
	}
	return pageTitle + " · " + s.Name
}

// AbsoluteURL joins BaseURL with a path.
func (s Site) AbsoluteURL(path string) string {
	base := strings.TrimRight(s.BaseURL, "/")
	if path == "" || path == "/" {
		if base == "" {
			return "/"
		}
		return base + "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if base == "" {
		return path
	}
	return base + path
}
