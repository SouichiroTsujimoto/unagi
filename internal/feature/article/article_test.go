package article

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
)

func openTestArticles(t *testing.T) *Articles {
	t.Helper()
	database, err := db.Open(db.Config{
		Driver: db.DriverSQLite,
		DSN:    "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return New(database)
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
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, jst)
	art := Article{Status: StatusPublished, Published: true, PublishedAt: got.PublishedAt}
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

func TestSeedFromFS(t *testing.T) {
	t.Parallel()
	store := openTestArticles(t)
	fsys := fstest.MapFS{
		"hello-unagi.md": &fstest.MapFile{Data: []byte(`---
title: "unagiへようこそ"
emoji: "🍣"
type: "tech"
topics: ["Go"]
published: true
published_at: 2026-08-01 09:00
---

Hello
`)},
	}
	n, err := store.SeedFromFS(context.Background(), fsys)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("imported=%d", n)
	}
	n2, err := store.SeedFromFS(context.Background(), fsys)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second seed=%d", n2)
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
		`<code`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %s", want, html)
		}
	}
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
