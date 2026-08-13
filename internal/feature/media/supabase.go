package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SupabaseStore stores objects in a public Supabase Storage bucket via a secret API key.
type SupabaseStore struct {
	baseURL   string
	bucket    string
	secretKey string
	client    *http.Client
}

// NewSupabaseStore builds a store. baseURL is the Supabase project URL (e.g. http://127.0.0.1:54321).
func NewSupabaseStore(baseURL, bucket, secretKey string, client *http.Client) (*SupabaseStore, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	bucket = strings.TrimSpace(bucket)
	secretKey = strings.TrimSpace(secretKey)
	if baseURL == "" || bucket == "" || secretKey == "" {
		return nil, fmt.Errorf("supabase storage url, bucket, and secret key are required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &SupabaseStore{
		baseURL:   baseURL,
		bucket:    bucket,
		secretKey: secretKey,
		client:    client,
	}, nil
}

func (s *SupabaseStore) authorize(req *http.Request) {
	// Secret keys are not JWTs. Send them on apikey only.
	// https://supabase.com/docs/guides/getting-started/migrating-to-new-api-keys
	req.Header.Set("apikey", s.secretKey)
}

func (s *SupabaseStore) SignUpload(ctx context.Context, key, _ string) (string, string, error) {
	if !validObjectKey(key) {
		return "", "", ErrInvalidObject
	}
	url := fmt.Sprintf("%s/storage/v1/object/upload/sign/%s/%s", s.baseURL, s.bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", "", err
	}
	s.authorize(req)
	res, err := s.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", "", fmt.Errorf("supabase storage sign: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", fmt.Errorf("supabase storage sign: decode: %w", err)
	}
	if payload.URL == "" {
		return "", "", fmt.Errorf("supabase storage sign: empty url")
	}
	signed := payload.URL
	if strings.HasPrefix(signed, "/") {
		signed = s.baseURL + "/storage/v1" + signed
	}
	return signed, payload.Token, nil
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
	s.authorize(req)
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

func (s *SupabaseStore) Exists(ctx context.Context, key string) (bool, error) {
	if !validObjectKey(key) {
		return false, ErrInvalidObject
	}
	url := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.baseURL, s.bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return true, nil
	}
	return false, fmt.Errorf("supabase storage head: status %d", res.StatusCode)
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
	s.authorize(req)
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
