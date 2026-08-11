package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

const MaxUploadBytes = 5 << 20 // 5 MiB

var (
	ErrNotFound      = errors.New("media not found")
	ErrTooLarge      = errors.New("file too large")
	ErrInvalidType   = errors.New("unsupported content type")
	ErrInvalidObject = errors.New("invalid object key")
)

var allowedTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// ObjectStore persists opaque binary objects.
type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader, contentType string, size int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, string, int64, error)
	Delete(ctx context.Context, key string) error
}

type Media struct {
	bun.BaseModel `bun:"table:media,alias:m"`

	ID          int64     `bun:",pk,autoincrement" json:"id"`
	ObjectKey   string    `bun:",notnull,unique" json:"objectKey"`
	ContentType string    `bun:",notnull" json:"contentType"`
	SizeBytes   int64     `bun:",notnull" json:"sizeBytes"`
	SHA256      string    `bun:",notnull" json:"sha256"`
	CreatedAt   time.Time `bun:",notnull" json:"createdAt"`
}

type Library struct {
	db    *bun.DB
	store ObjectStore
}

func New(db *bun.DB, store ObjectStore) *Library {
	return &Library{db: db, store: store}
}

type UploadResult struct {
	Media Media
	URL   string
}

// Upload validates, stores, and records an image.
func (l *Library) Upload(ctx context.Context, filename string, r io.Reader, sizeHint int64) (UploadResult, error) {
	if sizeHint > MaxUploadBytes {
		return UploadResult{}, ErrTooLarge
	}
	limited := io.LimitReader(r, MaxUploadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return UploadResult{}, fmt.Errorf("read upload: %w", err)
	}
	if int64(len(data)) > MaxUploadBytes {
		return UploadResult{}, ErrTooLarge
	}
	contentType := normalizeContentType(http.DetectContentType(data), filename)
	ext, ok := allowedTypes[contentType]
	if !ok {
		return UploadResult{}, ErrInvalidType
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	key, err := randomObjectKey(ext)
	if err != nil {
		return UploadResult{}, err
	}
	if err := l.store.Put(ctx, key, bytes.NewReader(data), contentType, int64(len(data))); err != nil {
		return UploadResult{}, fmt.Errorf("store object: %w", err)
	}

	item := Media{
		ObjectKey:   key,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		SHA256:      hash,
		CreatedAt:   time.Now().UTC(),
	}
	if _, err := l.db.NewInsert().Model(&item).Exec(ctx); err != nil {
		_ = l.store.Delete(ctx, key)
		return UploadResult{}, fmt.Errorf("insert media: %w", err)
	}
	return UploadResult{Media: item, URL: "/images/" + key}, nil
}

// GetByKey loads media metadata.
func (l *Library) GetByKey(ctx context.Context, key string) (Media, error) {
	key = strings.TrimPrefix(key, "/")
	if !validObjectKey(key) {
		return Media{}, ErrInvalidObject
	}
	var item Media
	err := l.db.NewSelect().Model(&item).Where("object_key = ?", key).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Media{}, ErrNotFound
	}
	if err != nil {
		return Media{}, err
	}
	return item, nil
}

// Open streams an object for public delivery.
func (l *Library) Open(ctx context.Context, key string) (Media, io.ReadCloser, error) {
	item, err := l.GetByKey(ctx, key)
	if err != nil {
		return Media{}, nil, err
	}
	rc, contentType, size, err := l.store.Open(ctx, item.ObjectKey)
	if err != nil {
		return Media{}, nil, err
	}
	if contentType != "" {
		item.ContentType = contentType
	}
	if size > 0 {
		item.SizeBytes = size
	}
	return item, rc, nil
}

func randomObjectKey(ext string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]) + ext, nil
}

func validObjectKey(key string) bool {
	if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) {
		return false
	}
	return path.Base(key) == key
}

func normalizeContentType(detected, filename string) string {
	detected = strings.TrimSpace(strings.Split(detected, ";")[0])
	switch detected {
	case "image/jpg":
		return "image/jpeg"
	}
	if detected == "application/octet-stream" || detected == "" {
		ext := strings.ToLower(path.Ext(filename))
		if mt := mime.TypeByExtension(ext); mt != "" {
			return strings.Split(mt, ";")[0]
		}
	}
	return detected
}
