package contentsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/uptrace/bun"
)

const testRepo = "SouichiroTsujimoto/unagi-content"

func testSync(t *testing.T) (*Sync, *article.Articles, *media.Library, *media.LocalStore) {
	t.Helper()
	database := db.OpenTest(t)
	articles := article.New(database)
	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatal(err)
	}
	lib := media.New(database, store)
	s, err := New(database, articles, lib, Config{Secret: "test-secret", Repository: testRepo})
	if err != nil {
		t.Fatal(err)
	}
	return s, articles, lib, store
}

func pngSHA(t *testing.T) (data []byte, sum string) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data = buf.Bytes()
	h := sha256.Sum256(data)
	return data, hex.EncodeToString(h[:])
}

func md(title, body string) string {
	return "---\ntitle: \"" + title + "\"\nemoji: \"🍣\"\ntype: \"tech\"\ntopics: [\"Go\"]\n---\n\n" + body + "\n"
}

func snap(runID, commit string, articles []ArticleIn, images []ImageIn) Snapshot {
	return Snapshot{
		Repository: testRepo,
		CommitSHA:  commit,
		RunID:      runID,
		Articles:   articles,
		Images:     images,
	}
}

func TestVerifyHMAC(t *testing.T) {
	t.Parallel()
	body := []byte(`{"ok":true}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := Sign("secret", "POST", "/api/content-sync/sync", ts, "run-1", testRepo, body)
	if err := VerifyHMAC("secret", "POST", "/api/content-sync/sync", ts, "run-1", testRepo, "sha256="+sig, body, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := VerifyHMAC("secret", "POST", "/api/content-sync/sync", ts, "run-1", testRepo, sig, []byte("nope"), time.Now()); err != ErrUnauthorized {
		t.Fatalf("tamper: %v", err)
	}
	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	oldSig := Sign("secret", "POST", "/api/content-sync/sync", old, "run-1", testRepo, body)
	if err := VerifyHMAC("secret", "POST", "/api/content-sync/sync", old, "run-1", testRepo, oldSig, body, time.Now()); err != ErrStaleTimestamp {
		t.Fatalf("stale: %v", err)
	}
}

func TestDryRunAndApply(t *testing.T) {
	s, articles, _, store := testSync(t)
	ctx := context.Background()
	data, sum := pngSHA(t)
	key := sum + ".png"
	if err := store.Put(ctx, key, bytes.NewReader(data), "image/png", int64(len(data))); err != nil {
		t.Fatal(err)
	}

	first := snap("run-1", "aaaaaaaa", []ArticleIn{{
		Path:     "articles/hello-unagi.md",
		Markdown: md("Hello", "see ![](/images/dot.png)"),
	}}, []ImageIn{{
		Path:        "images/dot.png",
		SHA256:      sum,
		Size:        int64(len(data)),
		ContentType: "image/png",
	}})

	planned, err := s.DryRun(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if planned.Created != 1 || planned.Updated != 0 || planned.Deleted != 0 {
		t.Fatalf("dry-run=%+v", planned)
	}
	list, err := articles.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("dry-run wrote %d articles", len(list))
	}

	applied, err := s.Apply(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Created != 1 {
		t.Fatalf("apply=%+v", applied)
	}
	item, err := articles.GetBySlugAdmin(ctx, "hello-unagi")
	if err != nil {
		t.Fatal(err)
	}
	if item.Published || item.Status != article.StatusDraft {
		t.Fatalf("new article must be draft: %+v", item)
	}
	if !strings.Contains(item.BodyMD, "/images/"+key) || strings.Contains(item.BodyMD, "/images/dot.png") {
		t.Fatalf("body not rewritten: %s", item.BodyMD)
	}

	if _, err := articles.Publish(ctx, item.ID, item.RevisionID, time.Date(2026, 8, 1, 9, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60))); err != nil {
		t.Fatal(err)
	}
	publishedAt := item.PublishedAt
	_ = publishedAt

	same, err := s.Apply(ctx, snap("run-2", "bbbbbbbb", first.Articles, first.Images))
	if err != nil {
		t.Fatal(err)
	}
	if same.Unchanged != 1 || same.Updated != 0 {
		t.Fatalf("noop=%+v", same)
	}

	changed := first
	changed.RunID = "run-3"
	changed.CommitSHA = "cccccccc"
	changed.Articles[0].Markdown = md("Hello v2", "see ![](/images/dot.png)\n\nmore")
	updated, err := s.Apply(ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Updated != 1 {
		t.Fatalf("update=%+v", updated)
	}
	pub, err := articles.Get(ctx, "hello-unagi", time.Date(2026, 8, 12, 0, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)))
	if err != nil {
		t.Fatal(err)
	}
	if pub.Title != "Hello v2" {
		t.Fatalf("published revision not advanced: %+v", pub)
	}
	again, err := articles.GetByID(ctx, pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.PublishedAt.IsZero() {
		t.Fatal("published_at cleared")
	}
}

func TestApplyKeepsUnpublishedAndDeletes(t *testing.T) {
	s, articles, _, store := testSync(t)
	ctx := context.Background()
	data, sum := pngSHA(t)
	if err := store.Put(ctx, sum+".png", bytes.NewReader(data), "image/png", int64(len(data))); err != nil {
		t.Fatal(err)
	}
	images := []ImageIn{{Path: "images/dot.png", SHA256: sum, Size: int64(len(data)), ContentType: "image/png"}}

	created, err := articles.Create(ctx, article.SaveInput{Slug: "keep-draft", Title: "Old", Type: "tech", BodyMD: "old\n"})
	if err != nil {
		t.Fatal(err)
	}
	gone, err := articles.Create(ctx, article.SaveInput{Slug: "remove-me", Title: "Gone", Type: "tech", BodyMD: "bye\n"})
	if err != nil {
		t.Fatal(err)
	}
	eng := engagement.New(s.db, articles)
	if _, err := articles.Publish(ctx, gone.ID, gone.RevisionID, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.AddEmojiSticker(ctx, "remove-me", time.Now(), engagement.AddEmojiInput{Emoji: "🍣", X: 0.2, Y: 0.3}); err != nil {
		t.Fatal(err)
	}

	out, err := s.Apply(ctx, snap("run-del", "dddddddd", []ArticleIn{{
		Path:     "articles/keep-draft.md",
		Markdown: md("New title", "updated"),
	}}, images))
	if err != nil {
		t.Fatal(err)
	}
	if out.Updated != 1 || out.Deleted != 1 {
		t.Fatalf("counts=%+v", out)
	}
	kept, err := articles.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Published || kept.Title != "New title" {
		t.Fatalf("draft changed publish state: %+v", kept)
	}
	if _, err := articles.GetBySlugAdmin(ctx, "remove-me"); err != article.ErrNotFound {
		t.Fatalf("deleted article remains: %v", err)
	}
}

func TestApplyRejectsDuplicateAndMissingImage(t *testing.T) {
	s, _, _, store := testSync(t)
	ctx := context.Background()
	data, sum := pngSHA(t)
	first := snap("run-dup", "eeeeeeee", []ArticleIn{{
		Path:     "articles/hello-unagi.md",
		Markdown: md("Hello", "no image"),
	}}, nil)
	if _, err := s.Apply(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(ctx, first); err != ErrDuplicateRun {
		t.Fatalf("dup=%v", err)
	}

	missing := snap("run-img", "ffffffff", []ArticleIn{{
		Path:     "articles/pic.md",
		Markdown: md("Pic", "![](/images/dot.png)"),
	}}, []ImageIn{{
		Path:        "images/dot.png",
		SHA256:      sum,
		Size:        int64(len(data)),
		ContentType: "image/png",
	}})
	if _, err := s.Apply(ctx, missing); err == nil || !strings.Contains(err.Error(), "not in storage") {
		t.Fatalf("missing image: %v", err)
	}
	if err := store.Put(ctx, sum+".png", bytes.NewReader(data), "image/png", int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(ctx, missing); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRollsBackOnFailure(t *testing.T) {
	s, articles, _, _ := testSync(t)
	ctx := context.Background()
	good := article.SyncContent{Slug: "ok", SourcePath: "articles/ok.md", SourceHash: "a", Title: "Ok", Type: "tech", BodyMD: "ok\n"}
	bad := article.SyncContent{Slug: "ok", SourcePath: "articles/ok.md", SourceHash: "b", Title: "Dup", Type: "tech", BodyMD: "dup\n"}
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := articles.ApplySnapshotTx(ctx, tx, []article.SyncContent{good, bad})
		return err
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}
	list, err := articles.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("partial apply leaked: %+v", list)
	}
}

func TestPlanUploads(t *testing.T) {
	s, _, _, store := testSync(t)
	ctx := context.Background()
	data, sum := pngSHA(t)
	shot := snap("run-up", "1111111", nil, []ImageIn{{
		Path:        "images/dot.png",
		SHA256:      sum,
		Size:        int64(len(data)),
		ContentType: "image/png",
	}})
	uploads, err := s.PlanUploads(ctx, shot)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploads) != 1 || uploads[0].ObjectKey != sum+".png" || uploads[0].SignedURL == "" {
		t.Fatalf("uploads=%+v", uploads)
	}
	if err := store.Put(ctx, sum+".png", bytes.NewReader(data), "image/png", int64(len(data))); err != nil {
		t.Fatal(err)
	}
	again, err := s.PlanUploads(ctx, shot)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("expected no uploads, got %+v", again)
	}
}

func TestForbiddenRepoAndInvalidFrontmatter(t *testing.T) {
	s, _, _, _ := testSync(t)
	ctx := context.Background()
	_, err := s.DryRun(ctx, Snapshot{Repository: "evil/repo", CommitSHA: "aaaaaaa", RunID: "r", Articles: []ArticleIn{{
		Path:     "articles/hello-unagi.md",
		Markdown: md("Hello", "body"),
	}}})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("repo: %v", err)
	}
	_, err = s.DryRun(ctx, snap("r2", "bbbbbbb", []ArticleIn{{
		Path:     "articles/hello-unagi.md",
		Markdown: "no frontmatter\n",
	}}, nil))
	if err == nil || !strings.Contains(err.Error(), "invalid content snapshot") {
		t.Fatalf("frontmatter: %v", err)
	}
}
