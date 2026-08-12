package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SupabaseStore stores objects in a public Supabase Storage bucket via the service role key.
type SupabaseStore struct {
	baseURL        string
	bucket         string
	serviceRoleKey string
	client         *http.Client
}

// NewSupabaseStore builds a store. baseURL is the Supabase project URL (e.g. http://127.0.0.1:54321).
func NewSupabaseStore(baseURL, bucket, serviceRoleKey string, client *http.Client) (*SupabaseStore, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	bucket = strings.TrimSpace(bucket)
	serviceRoleKey = strings.TrimSpace(serviceRoleKey)
	if baseURL == "" || bucket == "" || serviceRoleKey == "" {
		return nil, fmt.Errorf("supabase storage url, bucket, and service role key are required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &SupabaseStore{
		baseURL:        baseURL,
		bucket:         bucket,
		serviceRoleKey: serviceRoleKey,
		client:         client,
	}, nil
}

func (s *SupabaseStore) Put(ctx context.Context, key string, r io.Reader, contentType string, size int64) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if size > 0 && int64(len(data)) != size {
		// size is a hint; prefer actual length
	}
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, s.bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)
	req.Header.Set("apikey", s.serviceRoleKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("supabase storage put: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *SupabaseStore) Open(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	if !validObjectKey(key) {
		return nil, "", 0, ErrInvalidObject
	}
	url := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.baseURL, s.bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", 0, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	if res.StatusCode == http.StatusNotFound {
		_ = res.Body.Close()
		return nil, "", 0, ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_ = res.Body.Close()
		return nil, "", 0, fmt.Errorf("supabase storage get: status %d", res.StatusCode)
	}
	return res.Body, res.Header.Get("Content-Type"), res.ContentLength, nil
}

func (s *SupabaseStore) Delete(ctx context.Context, key string) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, s.bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)
	req.Header.Set("apikey", s.serviceRoleKey)
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("supabase storage delete: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
