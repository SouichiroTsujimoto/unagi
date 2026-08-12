package linkcard

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestExpandYouTubeAndSkipCodeFence(t *testing.T) {
	t.Parallel()
	cards := New(nil)
	ctx := context.Background()

	body := "intro\n\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n\n```\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n```\n"
	out, err := cards.ExpandMarkdown(ctx, body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `article-embed-youtube`) {
		t.Fatalf("expected youtube embed, got %s", out)
	}
	if !strings.Contains(out, "youtube-nocookie.com/embed/dQw4w9WgXcQ") {
		t.Fatalf("expected nocookie embed url, got %s", out)
	}
	fenceIdx := strings.Index(out, "```")
	if fenceIdx < 0 {
		t.Fatal("missing fence")
	}
	if strings.Contains(out[fenceIdx:], "article-embed-youtube") {
		t.Fatal("youtube expanded inside code fence")
	}
}

func TestExpandPendingSkeleton(t *testing.T) {
	t.Parallel()
	cards := New(nil)
	out, err := cards.ExpandMarkdown(context.Background(), "https://example.com/article\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `data-linkcard-url="https://example.com/article"`) {
		t.Fatalf("expected pending card, got %s", out)
	}
	if !strings.Contains(out, "skeleton") {
		t.Fatalf("expected skeleton class, got %s", out)
	}
}

func TestParseOGPAndRenderCard(t *testing.T) {
	t.Parallel()
	base := "https://example.com/page"
	meta := parseOGP([]byte(`<!doctype html><html><head>
<meta property="og:title" content="Hello Card">
<meta property="og:description" content="Desc here">
<meta property="og:image" content="/img.png">
<meta property="og:site_name" content="Example">
</head></html>`), base)
	if meta.Title != "Hello Card" || meta.Description != "Desc here" || meta.SiteName != "Example" {
		t.Fatalf("meta=%+v", meta)
	}
	if meta.Image != "https://example.com/img.png" {
		t.Fatalf("image=%q", meta.Image)
	}
	card := Card{
		URL:         base,
		Provider:    ProviderOGP,
		Title:       meta.Title,
		Description: meta.Description,
		ImageURL:    meta.Image,
		SiteName:    meta.SiteName,
		OK:          true,
	}
	card.HTML = renderOGPCard(card)
	for _, want := range []string{"article-linkcard", "Hello Card", "Desc here", "Example"} {
		if !strings.Contains(card.HTML, want) {
			t.Fatalf("missing %q in %s", want, card.HTML)
		}
	}
}

func TestDetectProviders(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want string
	}{
		{"https://youtu.be/dQw4w9WgXcQ", ProviderYouTube},
		{"https://x.com/jack/status/20", ProviderX},
		{"https://twitter.com/jack/status/20", ProviderX},
		{"https://www.amazon.co.jp/dp/B0EXAMPLE1", ProviderAmazon},
		{"https://github.com/octocat/Hello-World", ProviderGitHub},
		{"https://example.com/a", ProviderOGP},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := detectProvider(u); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.raw, got, tc.want)
		}
	}
}

func TestBlockedLoopback(t *testing.T) {
	t.Parallel()
	cards := New(nil)
	_, err := cards.Resolve(context.Background(), "http://127.0.0.1/")
	if err == nil {
		t.Fatal("expected error for loopback")
	}
}

func TestGitHubLineParse(t *testing.T) {
	t.Parallel()
	s, e := parseGitHubLines("L10-L20")
	if s != 10 || e != 20 {
		t.Fatalf("%d %d", s, e)
	}
	s, e = parseGitHubLines("L3")
	if s != 3 || e != 3 {
		t.Fatalf("%d %d", s, e)
	}
}

func TestCleanAmazonTitle(t *testing.T) {
	t.Parallel()
	got := cleanAmazonTitle("Amazon | TP-Link Omada ギガビット マルチWAN VPNルーター ER605 | TP-Link | 無線・有線LANルーター 通販")
	if !strings.Contains(got, "TP-Link Omada") || !strings.Contains(got, "ER605") {
		t.Fatalf("got %q", got)
	}
	if strings.HasPrefix(strings.ToLower(got), "amazon") {
		t.Fatalf("should strip amazon prefix: %q", got)
	}
}

func TestExtractAmazonImage(t *testing.T) {
	t.Parallel()
	html := []byte(`<img data-old-hires="https://m.media-amazon.com/images/I/41a47e4SRjL._AC_SL1000_.jpg" />`)
	got := extractAmazonImage(html)
	if got == "" {
		t.Fatal("expected image")
	}
}

func TestExpandThenRenderYouTube(t *testing.T) {
	t.Parallel()
	cards := New(nil)
	expanded, err := cards.ExpandMarkdown(context.Background(), "https://youtu.be/dQw4w9WgXcQ\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(expanded, "article-embed-youtube") {
		t.Fatalf("expand failed: %s", expanded)
	}
}
