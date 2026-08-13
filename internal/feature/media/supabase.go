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

const Bucket = "images"

// SupabaseStore stores objects in a public Supabase Storage bucket via a secret API key.
type SupabaseStore struct {
	baseURL   string
	secretKey string
	client    *http.Client
}

// NewSupabaseStore builds a store. baseURL is the Supabase project URL (e.g. http://127.0.0.1:54321).
func NewSupabaseStore(baseURL, secretKey string, client *http.Client) (*SupabaseStore, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	secretKey = strings.TrimSpace(secretKey)
	if baseURL == "" || secretKey == "" {
		return nil, fmt.Errorf("supabase storage url and secret key are required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &SupabaseStore{
		baseURL:   baseURL,
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
	url := fmt.Sprintf("%s/storage/v1/object/upload/sign/%s/%s", s.baseURL, Bucket, key)
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
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, Bucket, key)
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

func (s *SupabaseStore) List(ctx context.Context) ([]string, error) {
	const page = 100
	var out []string
	for offset := 0; offset < 10000; offset += page {
		body, err := json.Marshal(map[string]any{
			"prefix": "",
			"limit":  page,
			"offset": offset,
		})
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("%s/storage/v1/object/list/%s", s.baseURL, Bucket)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		s.authorize(req)
		req.Header.Set("Content-Type", "application/json")
		res, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("supabase storage list: status %d: %s", res.StatusCode, strings.TrimSpace(string(payload)))
		}
		var items []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal(payload, &items); err != nil {
			return nil, fmt.Errorf("supabase storage list: decode: %w", err)
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if item.ID == "" || !validObjectKey(item.Name) {
				continue
			}
			out = append(out, item.Name)
		}
		if len(items) < page {
			break
		}
	}
	return out, nil
}

func (s *SupabaseStore) Delete(ctx context.Context, key string) error {
	if !validObjectKey(key) {
		return ErrInvalidObject
	}
	body, err := json.Marshal(map[string][]string{"prefixes": {key}})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/storage/v1/object/%s", s.baseURL, Bucket)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	s.authorize(req)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("supabase storage delete: status %d: %s", res.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}
