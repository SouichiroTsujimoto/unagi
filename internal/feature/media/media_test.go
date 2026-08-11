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

func TestUploadAndOpenPNG(t *testing.T) {
	database, err := db.Open(db.Config{
		Driver: db.DriverSQLite,
		DSN:    filepath.Join(t.TempDir(), "media.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatal(err)
	}
	lib := media.New(database, store)

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	result, err := lib.Upload(context.Background(), "dot.png", bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if result.URL == "" || result.Media.ContentType != "image/png" {
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

	_, err = lib.Upload(context.Background(), "x.txt", bytes.NewReader([]byte("hello")), 5)
	if err != media.ErrInvalidType {
		t.Fatalf("want invalid type, got %v", err)
	}
}

func TestRejectOversize(t *testing.T) {
	if err := media.ErrTooLarge; err == nil {
		t.Fatal("sentinel")
	}
	database, err := db.Open(db.Config{Driver: db.DriverSQLite, DSN: filepath.Join(t.TempDir(), "m.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "o"))
	if err != nil {
		t.Fatal(err)
	}
	lib := media.New(database, store)
	_, err = lib.Upload(context.Background(), "big.png", bytes.NewReader(nil), media.MaxUploadBytes+1)
	if err != media.ErrTooLarge {
		t.Fatalf("got %v", err)
	}
}
