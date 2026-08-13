package media

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

const MaxUploadBytes = 5 << 20 // 5 MiB

var (
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

func markdownURL(key string) string {
	return "/images/" + key
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
