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

func (s *LocalStore) Exists(_ context.Context, key string) (bool, error) {
	if !validObjectKey(key) {
		return false, ErrInvalidObject
	}
	_, err := os.Stat(filepath.Join(s.root, key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStore) List(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !validObjectKey(name) {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	err := os.Remove(filepath.Join(s.root, key))
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}
