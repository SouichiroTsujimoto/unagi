package media_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBeginAndCompletePNG(t *testing.T) {
	database := db.OpenTest(t)
	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatal(err)
	}
	lib := media.New(database, store)
	data := pngBytes(t)

	signed, err := lib.BeginUpload(context.Background(), "dot.png", "image/png", int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if signed.MarkdownURL != "/images/"+signed.ObjectKey || signed.ContentType != "image/png" {
		t.Fatalf("signed=%+v", signed)
	}
	if signed.SignedURL == "" || signed.Token == "" {
		t.Fatal("missing signed url or token")
	}

	if err := store.Put(context.Background(), signed.ObjectKey, bytes.NewReader(data), "image/png", int64(len(data))); err != nil {
		t.Fatal(err)
	}
	result, err := lib.CompleteUpload(context.Background(), signed.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "/images/"+result.Media.ObjectKey || result.Media.ContentType != "image/png" || result.Media.SHA256 == "" {
		t.Fatalf("result=%+v", result)
	}

	item, rc, err := lib.Open(context.Background(), result.Media.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if item.SHA256 == "" {
		t.Fatal("missing sha")
	}

	_, err = lib.BeginUpload(context.Background(), "x.txt", "text/plain", 5)
	if err != media.ErrInvalidType {
		t.Fatalf("want invalid type, got %v", err)
	}
}

func TestRejectOversize(t *testing.T) {
	database := db.OpenTest(t)
	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "o"))
	if err != nil {
		t.Fatal(err)
	}
	lib := media.New(database, store)
	_, err = lib.BeginUpload(context.Background(), "big.png", "image/png", media.MaxUploadBytes+1)
	if err != media.ErrTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestCompleteRejectsMissingAndInvalid(t *testing.T) {
	database := db.OpenTest(t)
	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "o"))
	if err != nil {
		t.Fatal(err)
	}
	lib := media.New(database, store)

	_, err = lib.CompleteUpload(context.Background(), "../escape.png")
	if err != media.ErrInvalidObject {
		t.Fatalf("escape: %v", err)
	}
	_, err = lib.CompleteUpload(context.Background(), "missing.png")
	if err != media.ErrNotFound {
		t.Fatalf("missing: %v", err)
	}

	key := "bad.txt"
	if err := store.Put(context.Background(), key, bytes.NewReader([]byte("hello")), "text/plain", 5); err != nil {
		t.Fatal(err)
	}
	_, err = lib.CompleteUpload(context.Background(), key)
	if err != media.ErrInvalidType {
		t.Fatalf("invalid: %v", err)
	}
	if _, _, _, err := store.Open(context.Background(), key); err != media.ErrNotFound {
		t.Fatalf("want deleted, got %v", err)
	}
}
