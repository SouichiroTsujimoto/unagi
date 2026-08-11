package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
)

// LocalStore stores objects under a directory for development.
type LocalStore struct {
	root string
}

func NewLocalStore(root string) (*LocalStore, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir media: %w", err)
	}
	return &LocalStore{root: root}, nil
}

func (s *LocalStore) Put(_ context.Context, key string, r io.Reader, _ string, _ int64) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	path := filepath.Join(s.root, key)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStore) Open(_ context.Context, key string) (io.ReadCloser, string, int64, error) {
	if !validObjectKey(key) {
		return nil, "", 0, ErrInvalidObject
	}
	path := filepath.Join(s.root, key)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", 0, ErrNotFound
		}
		return nil, "", 0, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, "", 0, err
	}
	return f, "", st.Size(), nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	err := os.Remove(filepath.Join(s.root, key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// GCSStore stores objects in a Google Cloud Storage bucket via ADC.
type GCSStore struct {
	bucket *storage.BucketHandle
	prefix string
}

func NewGCSStore(client *storage.Client, bucket, prefix string) *GCSStore {
	prefix = strings.Trim(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return &GCSStore{
		bucket: client.Bucket(bucket),
		prefix: prefix,
	}
}

func (s *GCSStore) Put(ctx context.Context, key string, r io.Reader, contentType string, _ int64) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	w := s.bucket.Object(s.prefix + key).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

func (s *GCSStore) Open(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	if !validObjectKey(key) {
		return nil, "", 0, ErrInvalidObject
	}
	obj := s.bucket.Object(s.prefix + key)
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	return r, r.Attrs.ContentType, r.Attrs.Size, nil
}

func (s *GCSStore) Delete(ctx context.Context, key string) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	err := s.bucket.Object(s.prefix + key).Delete(ctx)
	if err == storage.ErrObjectNotExist {
		return nil
	}
	return err
}
