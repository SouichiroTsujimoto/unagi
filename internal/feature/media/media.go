package media

import (
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

// PublicCacheControl is the Cache-Control value written on Storage uploads.
// One day is long enough for a return visit, short enough that a same-URL
// replace is visible by the next day. New bytes already get a new sha256 key.
const PublicCacheControl = "public, max-age=86400"

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
	SignUpload(ctx context.Context, key, contentType string) (signedURL, token string, err error)
	Put(ctx context.Context, key string, r io.Reader, contentType string, size int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, string, int64, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
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

type SignResult struct {
	ObjectKey   string
	SignedURL   string
	Token       string
	ContentType string
	MarkdownURL string
}

type UploadResult struct {
	Media Media
	URL   string
}

// ContentAddressedKey builds `{sha256}{ext}` from a hex digest and MIME type.
func ContentAddressedKey(sha256Hex, contentType string) (string, error) {
	sha256Hex = strings.ToLower(strings.TrimSpace(sha256Hex))
	if !validSHA256(sha256Hex) {
		return "", ErrInvalidObject
	}
	ct := normalizeContentType(contentType, sha256Hex)
	ext, ok := allowedTypes[ct]
	if !ok {
		return "", ErrInvalidType
	}
	return sha256Hex + ext, nil
}

// BeginKeyedUpload signs an upload for a caller-chosen content-addressed object key.
func (l *Library) BeginKeyedUpload(ctx context.Context, key, contentType string, sizeBytes int64) (SignResult, error) {
	if sizeBytes > MaxUploadBytes {
		return SignResult{}, ErrTooLarge
	}
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")
	if !validObjectKey(key) {
		return SignResult{}, ErrInvalidObject
	}
	ct := normalizeContentType(contentType, key)
	ext, ok := allowedTypes[ct]
	if !ok {
		return SignResult{}, ErrInvalidType
	}
	if !strings.HasSuffix(key, ext) {
		return SignResult{}, ErrInvalidType
	}
	signedURL, token, err := l.store.SignUpload(ctx, key, ct)
	if err != nil {
		return SignResult{}, fmt.Errorf("sign upload: %w", err)
	}
	return SignResult{
		ObjectKey:   key,
		SignedURL:   signedURL,
		Token:       token,
		ContentType: ct,
		MarkdownURL: markdownURL(key),
	}, nil
}

// Exists reports whether the object is already in the store.
func (l *Library) Exists(ctx context.Context, key string) (bool, error) {
	key = strings.TrimPrefix(key, "/")
	if !validObjectKey(key) {
		return false, ErrInvalidObject
	}
	return l.store.Exists(ctx, key)
}

// Upsert records media metadata for a content-addressed object.
func (l *Library) Upsert(ctx context.Context, item Media) error {
	if !validObjectKey(item.ObjectKey) || !validSHA256(item.SHA256) {
		return ErrInvalidObject
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	_, err := l.db.NewInsert().
		Model(&item).
		On("CONFLICT (object_key) DO UPDATE").
		Set("content_type = EXCLUDED.content_type").
		Set("size_bytes = EXCLUDED.size_bytes").
		Set("sha256 = EXCLUDED.sha256").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert media: %w", err)
	}
	return nil
}

// BeginUpload validates hints, generates an object key, and returns a path-limited signed upload URL.
func (l *Library) BeginUpload(ctx context.Context, filename, contentType string, sizeBytes int64) (SignResult, error) {
	if sizeBytes > MaxUploadBytes {
		return SignResult{}, ErrTooLarge
	}
	ct := normalizeContentType(contentType, filename)
	ext, ok := allowedTypes[ct]
	if !ok {
		return SignResult{}, ErrInvalidType
	}
	key, err := randomObjectKey(ext)
	if err != nil {
		return SignResult{}, err
	}
	signedURL, token, err := l.store.SignUpload(ctx, key, ct)
	if err != nil {
		return SignResult{}, fmt.Errorf("sign upload: %w", err)
	}
	return SignResult{
		ObjectKey:   key,
		SignedURL:   signedURL,
		Token:       token,
		ContentType: ct,
		MarkdownURL: markdownURL(key),
	}, nil
}

// CompleteUpload reads the uploaded object, validates it, and records a media row.
// Invalid objects are deleted from storage.
func (l *Library) CompleteUpload(ctx context.Context, key string) (UploadResult, error) {
	key = strings.TrimPrefix(key, "/")
	if !validObjectKey(key) {
		return UploadResult{}, ErrInvalidObject
	}
	rc, storedType, _, err := l.store.Open(ctx, key)
	if err != nil {
		return UploadResult{}, err
	}
	data, err := io.ReadAll(io.LimitReader(rc, MaxUploadBytes+1))
	_ = rc.Close()
	if err != nil {
		return UploadResult{}, fmt.Errorf("read object: %w", err)
	}
	if int64(len(data)) > MaxUploadBytes {
		_ = l.store.Delete(ctx, key)
		return UploadResult{}, ErrTooLarge
	}
	detected := normalizeContentType(http.DetectContentType(data), key)
	if storedType != "" && detected == "application/octet-stream" {
		detected = normalizeContentType(storedType, key)
	}
	if _, ok := allowedTypes[detected]; !ok {
		_ = l.store.Delete(ctx, key)
		return UploadResult{}, ErrInvalidType
	}
	sum := sha256.Sum256(data)
	item := Media{
		ObjectKey:   key,
		ContentType: detected,
		SizeBytes:   int64(len(data)),
		SHA256:      hex.EncodeToString(sum[:]),
		CreatedAt:   time.Now().UTC(),
	}
	if _, err := l.db.NewInsert().Model(&item).Exec(ctx); err != nil {
		_ = l.store.Delete(ctx, key)
		return UploadResult{}, fmt.Errorf("insert media: %w", err)
	}
	return UploadResult{Media: item, URL: markdownURL(key)}, nil
}

func markdownURL(key string) string {
	return "/images/" + key
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

func validSHA256(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
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
