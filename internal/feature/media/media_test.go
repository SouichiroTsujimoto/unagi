package media_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
)

func TestContentAddressedUpload(t *testing.T) {
	database := db.OpenTest(t)
	store, err := media.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	library := media.New(database, store)
	digest := strings.Repeat("a", 64)
	key, err := media.ContentAddressedKey(digest, "image/png")
	if err != nil {
		t.Fatal(err)
	}

	signed, err := library.BeginKeyedUpload(context.Background(), key, "image/png", 3)
	if err != nil {
		t.Fatal(err)
	}
	if signed.ObjectKey != key || signed.ContentType != "image/png" || signed.SignedURL == "" {
		t.Fatalf("signed=%+v", signed)
	}
	exists, err := library.Exists(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("object should not exist before upload")
	}

	if err := library.Upsert(context.Background(), media.Media{
		ObjectKey:   key,
		ContentType: "image/png",
		SizeBytes:   3,
		SHA256:      digest,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), key, strings.NewReader("abc"), "image/png", 3); err != nil {
		t.Fatal(err)
	}

	gone := strings.Repeat("b", 64) + ".png"
	if err := store.Put(context.Background(), gone, strings.NewReader("x"), "image/png", 1); err != nil {
		t.Fatal(err)
	}
	n, err := library.PruneExcept(context.Background(), []string{key})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned=%d", n)
	}
	if ok, _ := store.Exists(context.Background(), gone); ok {
		t.Fatal("expected pruned object gone")
	}
	if ok, _ := store.Exists(context.Background(), key); !ok {
		t.Fatal("kept object missing")
	}
}

func TestContentAddressedUploadRejectsInvalidInput(t *testing.T) {
	database := db.OpenTest(t)
	store, err := media.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	library := media.New(database, store)

	if _, err := media.ContentAddressedKey("invalid", "image/png"); err != media.ErrInvalidObject {
		t.Fatalf("digest error=%v", err)
	}
	if _, err := library.BeginKeyedUpload(context.Background(), "../escape.png", "image/png", 3); err != media.ErrInvalidObject {
		t.Fatalf("key error=%v", err)
	}
	if _, err := library.BeginKeyedUpload(context.Background(), strings.Repeat("a", 64)+".png", "image/png", media.MaxUploadBytes+1); err != media.ErrTooLarge {
		t.Fatalf("size error=%v", err)
	}
}
