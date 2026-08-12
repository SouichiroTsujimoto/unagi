package engagement

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
)

func openTestEngagement(t *testing.T) (*Engagement, *article.Articles) {
	t.Helper()
	database, err := db.Open(db.Config{
		Driver: db.DriverSQLite,
		DSN:    "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	articles := article.New(database)
	return New(database, articles), articles
}

func publishHello(t *testing.T, articles *article.Articles) article.Article {
	t.Helper()
	ctx := context.Background()
	created, err := articles.Create(ctx, article.SaveInput{
		Slug:        "hello",
		Title:       "Hello",
		Emoji:       "🍣",
		Type:        "tech",
		Topics:      []string{"Go"},
		BodyMD:      "body\n",
		PublishedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)),
	})
	if err != nil {
		t.Fatal(err)
	}
	published, err := articles.Publish(ctx, created.ID, created.RevisionID, created.PublishedAt)
	if err != nil {
		t.Fatal(err)
	}
	return published
}

func TestGetSnapshotAndAddEmoji(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)

	snap, err := eng.GetSnapshot(ctx, "hello", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Stickers) != 0 || len(snap.Comments) != 0 {
		t.Fatalf("expected empty snapshot, got %+v", snap)
	}
	if len(snap.AllowedEmoji) == 0 || snap.LoginPath != LoginPath {
		t.Fatalf("meta=%+v", snap)
	}

	sticker, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji:       "🍣",
		X:           0.25,
		Y:           0.75,
		VisitorHash: HashVisitor("visitor-1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sticker.Kind != KindEmoji || sticker.Value != "🍣" || sticker.X != 0.25 || sticker.Y != 0.75 {
		t.Fatalf("sticker=%+v", sticker)
	}

	snap, err = eng.GetSnapshot(ctx, "hello", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Stickers) != 1 {
		t.Fatalf("stickers=%d", len(snap.Stickers))
	}
}

func TestAddEmojiValidationAndLimits(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()
	hash := HashVisitor("visitor-limit")

	_, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji:       "💣",
		X:           0.5,
		Y:           0.5,
		VisitorHash: hash,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want invalid emoji, got %v", err)
	}

	_, err = eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji:       "👍",
		X:           1.5,
		Y:           0.5,
		VisitorHash: hash,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want invalid coord, got %v", err)
	}

	for i := 0; i < MaxStickersPerVisitorArticle; i++ {
		_, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
			Emoji:       "👍",
			X:           0.1,
			Y:           float64(i) / 20,
			VisitorHash: hash,
		})
		if err != nil {
			t.Fatalf("place %d: %v", i, err)
		}
	}
	_, err = eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji:       "👍",
		X:           0.2,
		Y:           0.2,
		VisitorHash: hash,
	})
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("want visitor limit, got %v", err)
	}
}

func TestDraftArticleNotFound(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	ctx := context.Background()
	if _, err := articles.Create(ctx, article.SaveInput{
		Slug:   "draft",
		Title:  "Draft",
		Emoji:  "📝",
		Type:   "tech",
		BodyMD: "secret\n",
	}); err != nil {
		t.Fatal(err)
	}
	_, err := eng.GetSnapshot(ctx, "draft", time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestLoginRequiredStubs(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()

	_, err := eng.AddAvatarSticker(ctx, "hello", now)
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("avatar: %v", err)
	}
	_, err = eng.AddComment(ctx, "hello", now, "nice post")
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("comment: %v", err)
	}
	_, err = eng.AddComment(ctx, "hello", now, "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty comment: %v", err)
	}
}

func TestSeedCommentAppearsInSnapshot(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	post := publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()

	if _, err := eng.SeedVisibleComment(ctx, post.ID, "hello from x", "wuhu", "wuhu", "https://example.com/a.png", now); err != nil {
		t.Fatal(err)
	}
	snap, err := eng.GetSnapshot(ctx, "hello", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Comments) != 1 || snap.Comments[0].Body != "hello from x" {
		t.Fatalf("comments=%+v", snap.Comments)
	}
}

func TestAdminDeleteStickers(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	post := publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()
	hash := HashVisitor("admin-delete")

	a, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{Emoji: "🍣", X: 0.1, Y: 0.2, VisitorHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	b, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{Emoji: "👍", X: 0.3, Y: 0.4, VisitorHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{Emoji: "👀", X: 0.5, Y: 0.6, VisitorHash: hash}); err != nil {
		t.Fatal(err)
	}

	list, err := eng.ListStickersByArticleID(ctx, post.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}
	if list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("order=%+v", list)
	}

	n, err := eng.DeleteStickersByIDs(ctx, post.ID, []int64{a.ID, b.ID})
	if err != nil || n != 2 {
		t.Fatalf("deleted=%d err=%v", n, err)
	}
	list, err = eng.ListStickersByArticleID(ctx, post.ID)
	if err != nil || len(list) != 1 || list[0].Value != "👀" {
		t.Fatalf("after select delete: %+v err=%v", list, err)
	}

	n, err = eng.DeleteAllStickers(ctx, post.ID)
	if err != nil || n != 1 {
		t.Fatalf("delete all=%d err=%v", n, err)
	}
	list, err = eng.ListStickersByArticleID(ctx, post.ID)
	if err != nil || len(list) != 0 {
		t.Fatalf("expected empty, got %+v err=%v", list, err)
	}
}
