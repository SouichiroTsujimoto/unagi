package article

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/linkcard"
)

func openTestArticles(t *testing.T) *Articles {
	t.Helper()
	return New(db.OpenTest(t))
}

func TestParseAndPublicVisibility(t *testing.T) {
	t.Parallel()

	raw := []byte(`---
title: "Hello"
emoji: "📝"
type: "tech"
topics: ["Go", "Zenn"]
published: true
published_at: 2026-08-01 09:00
---

Body with [link](https://example.com).
`)
	got, err := Parse("hello", raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Hello" || got.Emoji != "📝" || got.Type != "tech" {
		t.Fatalf("meta=%+v", got)
	}
	if len(got.Topics) != 2 || got.Topics[0] != "Go" {
		t.Fatalf("topics=%v", got.Topics)
	}
	if got.Published || !got.PublishedAt.IsZero() {
		t.Fatalf("git published fields must be ignored: %+v", got)
	}
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, jst)
	art := Article{Status: StatusPublished, Published: true, PublishedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, jst)}
	if !art.IsPublic(now) {
		t.Fatal("expected public")
	}
	before := time.Date(2026, 7, 1, 0, 0, 0, 0, jst)
	if art.IsPublic(before) {
		t.Fatal("expected scheduled article hidden")
	}
}

func TestCreatePublishAndList(t *testing.T) {
	t.Parallel()
	store := openTestArticles(t)
	ctx := context.Background()

	created, err := store.Create(ctx, SaveInput{
		Slug:        "hello",
		Title:       "Hello",
		Emoji:       "📝",
		Type:        "tech",
		Topics:      []string{"Go"},
		BodyMD:      "# Hello\n\npublished body\n",
		PublishedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, jst),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(ctx, created.ID, created.RevisionID, created.PublishedAt); err != nil {
		t.Fatal(err)
	}

	draft, err := store.Create(ctx, SaveInput{
		Slug:   "draft",
		Title:  "Draft",
		Emoji:  "📝",
		Type:   "tech",
		Topics: []string{"Go"},
		BodyMD: "secret\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = draft

	future, err := store.Create(ctx, SaveInput{
		Slug:        "future",
		Title:       "Future",
		Emoji:       "📝",
		Type:        "tech",
		Topics:      []string{"Go"},
		BodyMD:      "later\n",
		PublishedAt: time.Date(2099, 1, 1, 9, 0, 0, 0, jst),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(ctx, future.ID, future.RevisionID, future.PublishedAt); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 12, 12, 0, 0, 0, jst)
	list, err := store.List(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Slug != "hello" {
		t.Fatalf("list=%v", list)
	}
	if len(list[0].Topics) != 1 || list[0].Topics[0] != "Go" {
		t.Fatalf("list topics=%v", list[0].Topics)
	}
	got, err := store.Get(ctx, "hello", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Hello" || got.HTML == "" || len(got.Topics) != 1 || got.Topics[0] != "Go" {
		t.Fatalf("get=%+v", got)
	}
	if _, err := store.Get(ctx, "draft", now); err != ErrNotFound {
		t.Fatalf("draft err=%v", err)
	}
	if _, err := store.Get(ctx, "future", now); err != ErrNotFound {
		t.Fatalf("future err=%v", err)
	}
	byTopic, err := store.ListByTopic(ctx, "Go", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(byTopic) != 1 {
		t.Fatalf("topic=%v", byTopic)
	}
}

func TestPublishKeepsOriginalPublishedAt(t *testing.T) {
	t.Parallel()
	store := openTestArticles(t)
	ctx := context.Background()
	first := time.Date(2026, 8, 1, 9, 0, 0, 0, jst)
	created, err := store.Create(ctx, SaveInput{
		Slug:   "keep-date",
		Title:  "Keep",
		Type:   "tech",
		BodyMD: "one\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(ctx, created.ID, created.RevisionID, first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Unpublish(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	item, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(ctx, item.ID, item.RevisionID, time.Date(2026, 8, 12, 0, 0, 0, 0, jst)); err != nil {
		t.Fatal(err)
	}
	again, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.PublishedAt.Equal(first) {
		t.Fatalf("published_at changed: %v want %v", again.PublishedAt, first)
	}
}

func TestRenderZennBlocksAndCode(t *testing.T) {
	t.Parallel()

	html, err := Render(`:::message
hello **world**
:::

:::message alert
careful
:::

:::details Open me
hidden *text*
:::

` + "```go\npackage main\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="article-message"`,
		`article-message-alert`,
		`<details class="article-details">`,
		`<summary>Open me</summary>`,
		`hello`,
		`class="chroma"`,
		`class="kn"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %s", want, html)
		}
	}
}

func TestRenderLinkCards(t *testing.T) {
	t.Parallel()

	md := "see\n\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n\n@[card](https://example.com/)\n"
	exp := stubExpander{html: `<figure class="article-embed article-embed-youtube"><div class="article-embed-frame"><span class="article-embed-frame-skel skeleton" aria-hidden="true"></span><iframe class="article-embed-frame-media" title="YouTube video" src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ" loading="lazy" referrerpolicy="strict-origin-when-cross-origin" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe></div></figure>
<figure class="article-linkcard"><a class="article-linkcard-link" href="https://example.com/" rel="noopener noreferrer" target="_blank"><span class="article-linkcard-thumb"><span class="article-linkcard-thumb-skel skeleton" aria-hidden="true"></span><img class="article-linkcard-thumb-img" src="https://example.com/og.png" alt="" loading="lazy" decoding="async"/></span><span class="article-linkcard-body"><span class="article-linkcard-title">Example</span><span class="article-linkcard-meta">example.com</span></span></a></figure>
`}
	a := &Articles{embeds: exp}
	html, err := a.RenderHTML(context.Background(), md)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`article-embed-youtube`,
		`class="article-embed-frame-media"`,
		`youtube-nocookie.com/embed/dQw4w9WgXcQ`,
		`article-linkcard`,
		`class="article-linkcard-thumb-img"`,
		`iframe`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %s", want, html)
		}
	}
}

type stubExpander struct{ html string }

func (s stubExpander) ExpandMarkdown(ctx context.Context, body string) (string, error) {
	_ = ctx
	_ = body
	return s.html, nil
}

func TestSaveRevisionKeepsPublishedStable(t *testing.T) {
	t.Parallel()
	store := openTestArticles(t)
	ctx := context.Background()
	created, err := store.Create(ctx, SaveInput{
		Slug:   "stable",
		Title:  "V1",
		Type:   "tech",
		BodyMD: "one\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(ctx, created.ID, created.RevisionID, time.Date(2026, 8, 1, 0, 0, 0, 0, jst)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveRevision(ctx, created.ID, SaveInput{
		Slug:   "stable",
		Title:  "V2",
		Type:   "tech",
		BodyMD: "two\n",
	}); err != nil {
		t.Fatal(err)
	}
	pub, err := store.Get(ctx, "stable", time.Date(2026, 8, 12, 0, 0, 0, 0, jst))
	if err != nil {
		t.Fatal(err)
	}
	if pub.Title != "V1" || strings.TrimSpace(pub.BodyMD) != "one" {
		t.Fatalf("published changed: %+v", pub)
	}
}

func TestRenderHTMLYouTubeLive(t *testing.T) {
	t.Parallel()
	a := New(nil, WithEmbeds(linkcard.New(nil)))
	html, err := a.RenderHTML(context.Background(), "hello\n\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n\nbye\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "youtube-nocookie.com/embed/dQw4w9WgXcQ") {
		t.Fatalf("got %s", html)
	}
	if strings.Contains(html, "<p><figure") {
		t.Fatalf("figure wrapped in p: %s", html)
	}
}

func TestRenderHTMLPendingCard(t *testing.T) {
	t.Parallel()
	a := New(nil, WithEmbeds(linkcard.New(nil)))
	html, err := a.RenderHTML(context.Background(), "https://example.com/post\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `data-linkcard-url="https://example.com/post"`) {
		t.Fatalf("pending attr stripped: %s", html)
	}
	if !strings.Contains(html, "skeleton") {
		t.Fatalf("missing skeleton: %s", html)
	}
}

func TestRenderMarkdownImagesLazy(t *testing.T) {
	t.Parallel()

	html, err := Render("![one](/images/a.png)\n\n![two](/images/b.png)\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `src="/images/a.png"`) || !strings.Contains(html, `src="/images/b.png"`) {
		t.Fatalf("missing src in %s", html)
	}
	if !strings.Contains(html, `decoding="async"`) {
		t.Fatalf("missing decoding in %s", html)
	}
	if strings.Count(html, `loading="lazy"`) != 1 {
		t.Fatalf("want one lazy image, got %s", html)
	}
	a := strings.Index(html, `src="/images/a.png"`)
	b := strings.Index(html, `src="/images/b.png"`)
	lazy := strings.Index(html, `loading="lazy"`)
	if a < 0 || b < 0 || lazy < 0 || lazy < a || lazy > b+len(`src="/images/b.png"`)+40 {
		t.Fatalf("lazy should be on the second image: %s", html)
	}
}

func TestRewriteImageURLs(t *testing.T) {
	t.Parallel()

	got := RewriteImageURLs("see ![](/images/dot.png)", "https://example.supabase.co/storage/v1/object/public/images")
	if got != "see ![](https://example.supabase.co/storage/v1/object/public/images/dot.png)" {
		t.Fatalf("got %q", got)
	}
	if RewriteImageURLs("none", "") != "none" {
		t.Fatal("empty base should be a no-op")
	}
}
