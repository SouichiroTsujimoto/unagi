package layout

import "strings"

// PageMeta holds document-level SEO and social metadata.
type PageMeta struct {
	Title       string
	Description string
	Canonical   string
	OGImage     string
	OGType      string // website | article
	Active      string
	Wide        bool
}

// Site holds shared site identity used when building PageMeta.
type Site struct {
	Name        string
	Description string
	BaseURL     string
	Author      string
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
