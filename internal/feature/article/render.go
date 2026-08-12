package article

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	messageBlockRE = regexp.MustCompile(`(?s):::message(?:\s+(\w+))?\r?\n(.*?)\r?\n:::`)
	detailsBlockRE = regexp.MustCompile(`(?s):::details\s+([^\r\n]+)\r?\n(.*?)\r?\n:::`)
)

var markdown = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Footnote,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.WithAllClasses(true),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
		html.WithUnsafe(),
	),
)

var policy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").OnElements("div", "aside", "pre", "code", "span", "details", "figure", "figcaption", "a")
	p.AllowAttrs("open").OnElements("details")
	p.AllowElements("aside", "details", "summary", "figure", "figcaption", "iframe", "noscript")
	p.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6", "li", "sup", "a")
	p.AllowAttrs("href", "rel", "target").OnElements("a")
	p.AllowAttrs("src", "alt", "loading", "decoding").OnElements("img")
	p.AllowAttrs("src", "title", "loading", "referrerpolicy", "allow", "allowfullscreen").OnElements("iframe")
	p.AllowAttrs("data-linkcard-url", "aria-busy", "aria-hidden").OnElements("figure", "span", "div")
	p.AllowURLSchemes("https", "http")
	p.RequireNoFollowOnLinks(false)
	return p
}()

// Render converts Markdown body to sanitized HTML.
func Render(body string) (string, error) {
	body, err := expandZennBlocks(body)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(body), &buf); err != nil {
		return "", fmt.Errorf("goldmark: %w", err)
	}
	return string(policy.SanitizeBytes(buf.Bytes())), nil
}

// RewriteImageURLs replaces Markdown `/images/<key>` links with the public Storage base.
func RewriteImageURLs(body, publicBase string) string {
	publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if publicBase == "" || body == "" {
		return body
	}
	return strings.ReplaceAll(body, "/images/", publicBase+"/")
}

func expandZennBlocks(body string) (string, error) {
	var err error
	body = messageBlockRE.ReplaceAllStringFunc(body, func(block string) string {
		if err != nil {
			return block
		}
		m := messageBlockRE.FindStringSubmatch(block)
		if m == nil {
			return block
		}
		kind := strings.TrimSpace(m[1])
		if kind == "" {
			kind = "info"
		}
		inner, renderErr := renderFragment(m[2])
		if renderErr != nil {
			err = renderErr
			return block
		}
		class := "article-message"
		if kind != "info" {
			class += " article-message-" + kind
		}
		return fmt.Sprintf("<aside class=\"%s\">%s</aside>\n", class, inner)
	})
	if err != nil {
		return "", err
	}

	body = detailsBlockRE.ReplaceAllStringFunc(body, func(block string) string {
		if err != nil {
			return block
		}
		m := detailsBlockRE.FindStringSubmatch(block)
		if m == nil {
			return block
		}
		inner, renderErr := renderFragment(m[2])
		if renderErr != nil {
			err = renderErr
			return block
		}
		title := htmlEscape(strings.TrimSpace(m[1]))
		return fmt.Sprintf("<details class=\"article-details\"><summary>%s</summary>%s</details>\n", title, inner)
	})
	if err != nil {
		return "", err
	}
	return body, nil
}

func renderFragment(md string) (string, error) {
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(strings.TrimSpace(md)+"\n"), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(s)
}
