package auth

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type supabaseClaims struct {
	jwt.RegisteredClaims
	Email        string          `json:"email"`
	Phone        string          `json:"phone"`
	AppMetadata  json.RawMessage `json:"app_metadata"`
	UserMetadata json.RawMessage `json:"user_metadata"`
	Role         string          `json:"role"`
	SessionID    string          `json:"session_id"`
}

type userMeta struct {
	UserName          string `json:"user_name"`
	PreferredUsername string `json:"preferred_username"`
	ScreenName        string `json:"screen_name"`
	FullName          string `json:"full_name"`
	Name              string `json:"name"`
	AvatarURL         string `json:"avatar_url"`
	Picture           string `json:"picture"`
	Email             string `json:"email"`
}

func parseJWT(raw, secret string) (supabaseClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &supabaseClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected alg %s", t.Method.Alg())
		}
		return []byte(secret), nil
	}, jwt.WithLeeway(30*time.Second))
	if err != nil {
		return supabaseClaims{}, err
	}
	claims, ok := token.Claims.(*supabaseClaims)
	if !ok || !token.Valid {
		return supabaseClaims{}, fmt.Errorf("invalid token")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return supabaseClaims{}, fmt.Errorf("missing sub")
	}
	return *claims, nil
}

func userFromClaims(claims supabaseClaims) User {
	meta := userMeta{}
	_ = json.Unmarshal(claims.UserMetadata, &meta)
	username := firstNonEmpty(meta.UserName, meta.PreferredUsername, meta.ScreenName)
	display := firstNonEmpty(meta.FullName, meta.Name, username)
	avatar := firstNonEmpty(meta.AvatarURL, meta.Picture)
	email := firstNonEmpty(claims.Email, meta.Email)
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}
	return User{
		ID:          claims.Subject,
		Email:       email,
		Username:    username,
		DisplayName: display,
		AvatarURL:   avatar,
		ExpiresAt:   exp,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
