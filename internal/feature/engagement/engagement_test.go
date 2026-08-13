package engagement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
)

func openTestEngagement(t *testing.T) (*Engagement, *article.Articles) {
	t.Helper()
	database := db.OpenTest(t)
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

func testAuthor(id string) Author {
	return Author{
		XUserID:     id,
		Username:    "wuhu",
		DisplayName: "wuhu",
		AvatarURL:   "https://example.com/a.png",
	}
}

func TestHighResolutionXAvatarURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "normal profile image",
			raw:  "https://pbs.twimg.com/profile_images/123/avatar_normal.jpg",
			want: "https://pbs.twimg.com/profile_images/123/avatar_400x400.jpg",
		},
		{
			name: "query preserved",
			raw:  "https://pbs.twimg.com/profile_images/123/avatar_normal.jpg?format=jpg",
			want: "https://pbs.twimg.com/profile_images/123/avatar_400x400.jpg?format=jpg",
		},
		{
			name: "already high resolution",
			raw:  "https://pbs.twimg.com/profile_images/123/avatar_400x400.jpg",
			want: "https://pbs.twimg.com/profile_images/123/avatar_400x400.jpg",
		},
		{
			name: "other host",
			raw:  "https://example.com/avatar_normal.jpg",
			want: "https://example.com/avatar_normal.jpg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := highResolutionXAvatarURL(tt.raw); got != tt.want {
				t.Fatalf("highResolutionXAvatarURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetSnapshotAndAddEmoji(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)

	snap, err := eng.GetSnapshot(ctx, "hello", now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Stickers) != 0 || len(snap.Comments) != 0 || snap.Authenticated {
		t.Fatalf("expected empty snapshot, got %+v", snap)
	}
	if len(snap.AllowedEmoji) == 0 || snap.LoginPath != LoginPath {
		t.Fatalf("meta=%+v", snap)
	}

	sticker, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji: "🍣",
		X:     0.25,
		Y:     0.75,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sticker.Kind != KindEmoji || sticker.Value != "🍣" || sticker.X != 0.25 || sticker.Y != 0.75 {
		t.Fatalf("sticker=%+v", sticker)
	}

	viewer := &Viewer{Username: "wuhu", DisplayName: "wuhu", AvatarURL: "https://example.com/a.png"}
	snap, err = eng.GetSnapshot(ctx, "hello", now, viewer)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Stickers) != 1 || !snap.Authenticated || snap.Viewer == nil || snap.LogoutPath == "" {
		t.Fatalf("snap=%+v", snap)
	}
}

func TestAddEmojiValidationAndBoardLimit(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()

	_, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji: "💣",
		X:     0.5,
		Y:     0.5,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want invalid emoji, got %v", err)
	}

	_, err = eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji: "👍",
		X:     1.5,
		Y:     0.5,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want invalid coord, got %v", err)
	}

	for i := 0; i < MaxStickersPerArticle; i++ {
		_, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
			Emoji: "👍",
			X:     0.1,
			Y:     float64(i) / float64(MaxStickersPerArticle+1),
		})
		if err != nil {
			t.Fatalf("place %d: %v", i, err)
		}
	}
	_, err = eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{
		Emoji: "👍",
		X:     0.2,
		Y:     0.2,
	})
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("want board limit, got %v", err)
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
	_, err := eng.GetSnapshot(ctx, "draft", time.Now(), nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestAddAvatarAndComment(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()
	author := testAuthor("42")
	author.AvatarURL = "https://pbs.twimg.com/profile_images/123/avatar_normal.jpg"
	wantAvatarURL := "https://pbs.twimg.com/profile_images/123/avatar_400x400.jpg"

	_, err := eng.AddAvatarSticker(ctx, "hello", now, Author{}, 0.5, 0.5)
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("avatar without author: %v", err)
	}

	sticker, err := eng.AddAvatarSticker(ctx, "hello", now, author, 0.4, 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if sticker.Kind != KindAvatar || sticker.Value != wantAvatarURL || sticker.Username != "wuhu" {
		t.Fatalf("sticker=%+v", sticker)
	}

	_, err = eng.AddComment(ctx, "hello", now, author, "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty comment: %v", err)
	}
	comment, err := eng.AddComment(ctx, "hello", now, author, "nice post")
	if err != nil {
		t.Fatal(err)
	}
	if comment.Body != "nice post" || comment.Username != "wuhu" || comment.AvatarURL != wantAvatarURL {
		t.Fatalf("comment=%+v", comment)
	}

	viewer := &Viewer{AvatarURL: author.AvatarURL, XUserID: author.XUserID}
	snap, err := eng.GetSnapshot(ctx, "hello", now, viewer)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Stickers) != 1 || len(snap.Comments) != 1 {
		t.Fatalf("snap=%+v", snap)
	}
	if snap.Viewer == nil || snap.Viewer.AvatarURL != wantAvatarURL {
		t.Fatalf("viewer=%+v", snap.Viewer)
	}
}

func TestCommentUserLimit(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()
	author := testAuthor("99")

	for i := 0; i < MaxCommentsPerUserArticle; i++ {
		if _, err := eng.AddComment(ctx, "hello", now, author, "c"); err != nil {
			t.Fatalf("comment %d: %v", i, err)
		}
	}
	_, err := eng.AddComment(ctx, "hello", now, author, "overflow")
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("want user limit, got %v", err)
	}

	other := testAuthor("100")
	if _, err := eng.AddComment(ctx, "hello", now, other, "ok"); err != nil {
		t.Fatal(err)
	}
}

func TestAdminCommentModeration(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	post := publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()
	author := testAuthor("7")

	visible, err := eng.AddComment(ctx, "hello", now, author, "keep me")
	if err != nil {
		t.Fatal(err)
	}
	hiddenSeed, err := eng.SeedCommentWithUser(ctx, post.ID, author, "hide me", CommentStatusVisible, now)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := eng.SetCommentStatus(ctx, post.ID, hiddenSeed.ID, CommentStatusHidden)
	if err != nil || updated.Status != CommentStatusHidden {
		t.Fatalf("hide=%+v err=%v", updated, err)
	}

	snap, err := eng.GetSnapshot(ctx, "hello", now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Comments) != 1 || snap.Comments[0].ID != visible.ID {
		t.Fatalf("public comments=%+v", snap.Comments)
	}

	list, err := eng.ListCommentsByArticleID(ctx, post.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("admin list=%d err=%v", len(list), err)
	}

	if _, err := eng.SetCommentStatus(ctx, post.ID, hiddenSeed.ID, CommentStatusVisible); err != nil {
		t.Fatal(err)
	}
	n, err := eng.DeleteCommentsByIDs(ctx, post.ID, []int64{hiddenSeed.ID})
	if err != nil || n != 1 {
		t.Fatalf("deleted=%d err=%v", n, err)
	}
	n, err = eng.DeleteAllComments(ctx, post.ID)
	if err != nil || n != 1 {
		t.Fatalf("delete all=%d err=%v", n, err)
	}
}

func TestDeleteOwnComment(t *testing.T) {
	t.Parallel()
	eng, articles := openTestEngagement(t)
	publishHello(t, articles)
	ctx := context.Background()
	now := time.Now()
	author := testAuthor("42")
	other := testAuthor("99")

	mine, err := eng.AddComment(ctx, "hello", now, author, "my comment")
	if err != nil {
		t.Fatal(err)
	}
	if !mine.Mine {
		t.Fatalf("created comment should be mine: %+v", mine)
	}
	theirs, err := eng.AddComment(ctx, "hello", now, other, "other comment")
	if err != nil {
		t.Fatal(err)
	}

	viewer := &Viewer{Username: author.Username, DisplayName: author.DisplayName, AvatarURL: author.AvatarURL, XUserID: author.XUserID}
	snap, err := eng.GetSnapshot(ctx, "hello", now, viewer)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Comments) != 2 {
		t.Fatalf("comments=%d", len(snap.Comments))
	}
	var sawMine, sawTheirs bool
	for _, c := range snap.Comments {
		if c.ID == mine.ID {
			sawMine = c.Mine
		}
		if c.ID == theirs.ID {
			sawTheirs = c.Mine
		}
	}
	if !sawMine || sawTheirs {
		t.Fatalf("mine flags wrong: %+v", snap.Comments)
	}

	if err := eng.DeleteOwnComment(ctx, "hello", now, author, theirs.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found for foreign comment, got %v", err)
	}
	if err := eng.DeleteOwnComment(ctx, "hello", now, author, mine.ID); err != nil {
		t.Fatal(err)
	}
	snap, err = eng.GetSnapshot(ctx, "hello", now, viewer)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Comments) != 1 || snap.Comments[0].ID != theirs.ID {
		t.Fatalf("after delete: %+v", snap.Comments)
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
	snap, err := eng.GetSnapshot(ctx, "hello", now, nil)
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

	a, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{Emoji: "🍣", X: 0.1, Y: 0.2})
	if err != nil {
		t.Fatal(err)
	}
	b, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{Emoji: "👍", X: 0.3, Y: 0.4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.AddEmojiSticker(ctx, "hello", now, AddEmojiInput{Emoji: "👀", X: 0.5, Y: 0.6}); err != nil {
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
