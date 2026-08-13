package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStore stores objects under a directory for development and tests.
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

func (s *LocalStore) SignUpload(_ context.Context, key, _ string) (string, string, error) {
	if !validObjectKey(key) {
		return "", "", ErrInvalidObject
	}
	return "local://upload/" + key, "local", nil
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
