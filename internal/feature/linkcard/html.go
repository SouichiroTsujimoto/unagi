package linkcard

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

func htmlEscape(s string) string {
	return html.EscapeString(s)
}

func renderOGPCard(card Card) string {
	href := htmlEscape(card.URL)
	title := htmlEscape(fallback(card.Title, card.URL))
	desc := htmlEscape(card.Description)
	site := htmlEscape(fallback(card.SiteName, hostOf(card.URL)))
	providerClass := "article-linkcard"
	if card.Provider != "" && card.Provider != ProviderOGP {
		providerClass += " article-linkcard-" + htmlEscape(card.Provider)
	}

	var b strings.Builder
	b.WriteString(`<figure class="` + providerClass + `">`)
	b.WriteString(`<a class="article-linkcard-link" href="` + href + `" rel="noopener noreferrer" target="_blank">`)
	if card.ImageURL != "" {
		b.WriteString(`<span class="article-linkcard-thumb">`)
		b.WriteString(`<span class="article-linkcard-thumb-skel skeleton" aria-hidden="true"></span>`)
		b.WriteString(`<img class="article-linkcard-thumb-img" src="` + htmlEscape(card.ImageURL) + `" alt="" loading="lazy" decoding="async"/>`)
		b.WriteString(`</span>`)
	}
	b.WriteString(`<span class="article-linkcard-body">`)
	b.WriteString(`<span class="article-linkcard-title">` + title + `</span>`)
	if desc != "" {
		b.WriteString(`<span class="article-linkcard-desc">` + desc + `</span>`)
	}
	b.WriteString(`<span class="article-linkcard-meta">` + site + `</span>`)
	b.WriteString(`</span></a></figure>`)
	return b.String()
}

func renderPendingCard(rawURL string) string {
	esc := htmlEscape(rawURL)
	var b strings.Builder
	b.WriteString(`<figure class="article-linkcard article-linkcard-pending" data-linkcard-url="` + esc + `" aria-busy="true">`)
	b.WriteString(`<div class="article-linkcard-link">`)
	b.WriteString(`<div class="article-linkcard-thumb skeleton" aria-hidden="true"></div>`)
	b.WriteString(`<div class="article-linkcard-body">`)
	b.WriteString(`<div class="skeleton article-linkcard-skel-line article-linkcard-skel-title" aria-hidden="true"></div>`)
	b.WriteString(`<div class="skeleton article-linkcard-skel-line article-linkcard-skel-desc" aria-hidden="true"></div>`)
	b.WriteString(`<div class="skeleton article-linkcard-skel-line article-linkcard-skel-meta" aria-hidden="true"></div>`)
	b.WriteString(`</div></div>`)
	b.WriteString(`<noscript><p><a href="` + esc + `" rel="noopener noreferrer">` + esc + `</a></p></noscript>`)
	b.WriteString(`</figure>`)
	return b.String()
}

func renderYouTube(videoID, pageURL string) string {
	src := "https://www.youtube-nocookie.com/embed/" + url.PathEscape(videoID)
	return fmt.Sprintf(
		`<figure class="article-embed article-embed-youtube"><div class="article-embed-frame"><span class="article-embed-frame-skel skeleton" aria-hidden="true"></span><iframe class="article-embed-frame-media" title="YouTube video" src="%s" loading="lazy" referrerpolicy="strict-origin-when-cross-origin" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe></div><figcaption class="article-embed-caption"><a href="%s" rel="noopener noreferrer" target="_blank">YouTube で開く</a></figcaption></figure>`,
		htmlEscape(src),
		htmlEscape(pageURL),
	)
}

func renderXCard(card Card) string {
	href := htmlEscape(card.URL)
	title := htmlEscape(fallback(card.Title, "Xのポスト"))
	desc := htmlEscape(card.Description)
	site := htmlEscape(fallback(card.SiteName, "X"))
	var b strings.Builder
	b.WriteString(`<figure class="article-embed article-embed-x">`)
	b.WriteString(`<a class="article-embed-x-link" href="` + href + `" rel="noopener noreferrer" target="_blank">`)
	b.WriteString(`<span class="article-embed-x-label">` + site + `</span>`)
	if desc != "" {
		b.WriteString(`<span class="article-embed-x-text">` + desc + `</span>`)
	} else {
		b.WriteString(`<span class="article-embed-x-text">` + title + `</span>`)
	}
	if card.Title != "" && desc != "" {
		b.WriteString(`<span class="article-embed-x-author">` + title + `</span>`)
	}
	b.WriteString(`</a></figure>`)
	return b.String()
}

func renderGitHubSnippet(card Card, code string, lang string) string {
	href := htmlEscape(card.URL)
	title := htmlEscape(fallback(card.Title, card.URL))
	meta := htmlEscape(fallback(card.SiteName, "GitHub"))
	var b strings.Builder
	b.WriteString(`<figure class="article-embed article-embed-github">`)
	b.WriteString(`<a class="article-embed-github-head" href="` + href + `" rel="noopener noreferrer" target="_blank">`)
	b.WriteString(`<span class="article-embed-github-title">` + title + `</span>`)
	b.WriteString(`<span class="article-embed-github-meta">` + meta + `</span>`)
	b.WriteString(`</a>`)
	if strings.TrimSpace(code) != "" {
		class := "chroma"
		if lang != "" {
			class += " language-" + htmlEscape(lang)
		}
		b.WriteString(`<pre class="article-embed-github-code"><code class="` + class + `">` + htmlEscape(code) + `</code></pre>`)
	} else if card.Description != "" {
		b.WriteString(`<p class="article-embed-github-desc">` + htmlEscape(card.Description) + `</p>`)
	}
	b.WriteString(`</figure>`)
	return b.String()
}

func fallback(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	h := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if h == "" {
		return raw
	}
	return h
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
