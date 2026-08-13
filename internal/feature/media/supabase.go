package media

import (
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

func (s *SupabaseStore) Exists(ctx context.Context, key string) (bool, error) {
	if !validObjectKey(key) {
		return false, ErrInvalidObject
	}
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, s.bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	s.authorize(req)
	res, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		return true, nil
	case res.StatusCode == http.StatusNotFound, res.StatusCode == http.StatusBadRequest, res.StatusCode == http.StatusForbidden:
		return false, nil
	default:
		return false, fmt.Errorf("supabase storage head: status %d", res.StatusCode)
	}
}
