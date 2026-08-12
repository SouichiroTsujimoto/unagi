package linkcard

import (
	"net/url"
	"regexp"
	"strings"
)

var youtubeIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func resolveYouTube(raw string, u *url.URL) (Card, error) {
	id := youtubeVideoID(u)
	if id == "" {
		return Card{URL: raw, Provider: ProviderYouTube, OK: false}, nil
	}
	pageURL := "https://www.youtube.com/watch?v=" + id
	return Card{
		URL:      pageURL,
		Provider: ProviderYouTube,
		Title:    "YouTube",
		SiteName: "YouTube",
		HTML:     renderYouTube(id, pageURL),
		OK:       true,
	}, nil
}

func youtubeVideoID(u *url.URL) string {
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	path := strings.Trim(u.Path, "/")
	switch host {
	case "youtu.be":
		id := strings.Split(path, "/")[0]
		if youtubeIDRE.MatchString(id) {
			return id
		}
	case "youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com":
		if v := u.Query().Get("v"); youtubeIDRE.MatchString(v) {
			return v
		}
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			switch parts[0] {
			case "embed", "shorts", "live", "v":
				if youtubeIDRE.MatchString(parts[1]) {
					return parts[1]
				}
			}
		}
	}
	return ""
}
