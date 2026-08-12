// Package auth verifies Supabase Auth (GoTrue) JWTs and mediates OAuth / passkeys.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	CookieName       = "unagi_session"
	PKCECookieName   = "unagi_oauth_pkce"
	CallbackPath     = "/auth/x/callback"
	LogoutPath       = "/auth/x/logout"
	LoginPath        = "/auth/x/login"
	pkceCookieMaxAge = 10 * 60
)

var (
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrNotConfigured = errors.New("auth not configured")
	ErrOAuthFailed   = errors.New("oauth failed")
	ErrInvalidState  = errors.New("invalid oauth state")
)

// Config wires Supabase Auth and admin allowlisting.
type Config struct {
	SupabaseURL    string
	AnonKey        string
	JWTSecret      string
	AdminUserIDs   []string
	AllowedOrigins []string
	SiteBaseURL    string
	SessionTTL     time.Duration // cookie MaxAge hint; JWT exp is authoritative
	SecureCookies  bool
	HTTPClient     *http.Client
}

// Auth is the application auth facade over GoTrue.
type Auth struct {
	config     Config
	adminSet   map[string]struct{}
	httpClient *http.Client
}

// User is a verified Supabase Auth subject.
type User struct {
	ID          string
	Email       string
	Username    string
	DisplayName string
	AvatarURL   string
	IsAdmin     bool
	ExpiresAt   time.Time
	AccessToken string
}

// New validates config and returns Auth.
func New(cfg Config) (*Auth, error) {
	cfg.SupabaseURL = strings.TrimRight(strings.TrimSpace(cfg.SupabaseURL), "/")
	cfg.AnonKey = strings.TrimSpace(cfg.AnonKey)
	cfg.JWTSecret = strings.TrimSpace(cfg.JWTSecret)
	if cfg.SupabaseURL == "" || cfg.AnonKey == "" || cfg.JWTSecret == "" {
		return nil, fmt.Errorf("%w: supabase url, anon key, and jwt secret are required", ErrNotConfigured)
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 7 * 24 * time.Hour
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	adminSet := make(map[string]struct{}, len(cfg.AdminUserIDs))
	for _, id := range cfg.AdminUserIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			adminSet[id] = struct{}{}
		}
	}
	origins := make([]string, 0, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o != "" {
			origins = append(origins, o)
		}
	}
	if len(origins) == 0 && strings.TrimSpace(cfg.SiteBaseURL) != "" {
		origins = []string{strings.TrimRight(strings.TrimSpace(cfg.SiteBaseURL), "/")}
	}
	cfg.AllowedOrigins = origins
	return &Auth{config: cfg, adminSet: adminSet, httpClient: cfg.HTTPClient}, nil
}

func (a *Auth) SessionTTL() time.Duration { return a.config.SessionTTL }
func (a *Auth) SecureCookies() bool       { return a.config.SecureCookies }

// ValidOrigin reports whether origin is an allowed browser Origin.
func (a *Auth) ValidOrigin(origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return true
	}
	if len(a.config.AllowedOrigins) == 0 {
		return true
	}
	for _, want := range a.config.AllowedOrigins {
		if originMatches(origin, want) {
			return true
		}
	}
	return false
}

func originMatches(got, want string) bool {
	g, err1 := url.Parse(got)
	w, err2 := url.Parse(want)
	if err1 != nil || err2 != nil || g.Scheme == "" || w.Scheme == "" {
		return got == want
	}
	if !strings.EqualFold(g.Scheme, w.Scheme) {
		return false
	}
	gh, gp := g.Hostname(), g.Port()
	wh, wp := w.Hostname(), w.Port()
	if gp == "" {
		if g.Scheme == "https" {
			gp = "443"
		} else {
			gp = "80"
		}
	}
	if wp == "" {
		if w.Scheme == "https" {
			wp = "443"
		} else {
			wp = "80"
		}
	}
	if gp != wp {
		return false
	}
	if strings.EqualFold(gh, wh) {
		return true
	}
	return isLoopback(gh) && isLoopback(wh)
}

func isLoopback(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// ParseAccessToken verifies a Supabase access token JWT.
func (a *Auth) ParseAccessToken(raw string) (User, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return User{}, ErrUnauthorized
	}
	claims, err := parseJWT(raw, a.config.JWTSecret)
	if err != nil {
		return User{}, ErrUnauthorized
	}
	user := userFromClaims(claims)
	user.AccessToken = raw
	if _, ok := a.adminSet[user.ID]; ok {
		user.IsAdmin = true
	}
	return user, nil
}

// IsAdminUser reports whether the user id is allowlisted.
func (a *Auth) IsAdminUser(userID string) bool {
	_, ok := a.adminSet[strings.TrimSpace(userID)]
	return ok
}

// BeginPasskeyLogin proxies GoTrue passkey authentication options.
func (a *Auth) BeginPasskeyLogin(ctx context.Context) (json.RawMessage, error) {
	return a.gotrueJSON(ctx, http.MethodPost, "/auth/v1/passkeys/authentication/options", nil, "")
}

// FinishPasskeyLogin verifies a passkey assertion via GoTrue and returns the session user.
func (a *Auth) FinishPasskeyLogin(ctx context.Context, challengeID string, credential json.RawMessage) (User, error) {
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" || len(credential) == 0 {
		return User{}, ErrUnauthorized
	}
	body, err := json.Marshal(map[string]any{
		"challenge_id":        challengeID,
		"credential_response": json.RawMessage(credential),
	})
	if err != nil {
		return User{}, err
	}
	raw, err := a.gotrueJSON(ctx, http.MethodPost, "/auth/v1/passkeys/authentication/verify", body, "")
	if err != nil {
		return User{}, err
	}
	tok, err := extractAccessToken(raw)
	if err != nil {
		return User{}, err
	}
	user, err := a.ParseAccessToken(tok)
	if err != nil {
		return User{}, err
	}
	if !user.IsAdmin {
		return User{}, ErrForbidden
	}
	return user, nil
}

// StartXOAuth returns the Supabase authorize URL and PKCE verifier cookie value.
func (a *Auth) StartXOAuth(returnTo string) (authorizeURL, verifier string, err error) {
	verifier, challenge, err := newPKCE()
	if err != nil {
		return "", "", err
	}
	redirectTo := strings.TrimRight(a.config.SiteBaseURL, "/") + CallbackPath
	q := url.Values{}
	q.Set("provider", "twitter")
	q.Set("redirect_to", redirectTo)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if safe := SafeReturnTo(returnTo); safe != "" {
		// Stash return path in redirect_to query for our callback to read via state cookie instead.
		_ = safe
	}
	authorizeURL = a.config.SupabaseURL + "/auth/v1/authorize?" + q.Encode()
	return authorizeURL, verifier, nil
}

// FinishXOAuth exchanges an auth code for tokens using the PKCE verifier.
func (a *Auth) FinishXOAuth(ctx context.Context, code, verifier string) (User, error) {
	code = strings.TrimSpace(code)
	verifier = strings.TrimSpace(verifier)
	if code == "" || verifier == "" {
		return User{}, ErrInvalidState
	}
	body, err := json.Marshal(map[string]string{
		"auth_code":     code,
		"code_verifier": verifier,
	})
	if err != nil {
		return User{}, err
	}
	raw, err := a.gotrueJSON(ctx, http.MethodPost, "/auth/v1/token?grant_type=pkce", body, "")
	if err != nil {
		return User{}, fmt.Errorf("%w: %v", ErrOAuthFailed, err)
	}
	tok, err := extractAccessToken(raw)
	if err != nil {
		return User{}, fmt.Errorf("%w: %v", ErrOAuthFailed, err)
	}
	return a.ParseAccessToken(tok)
}

func (a *Auth) gotrueJSON(ctx context.Context, method, path string, body []byte, bearer string) (json.RawMessage, error) {
	var rdr io.Reader
	if body != nil {
		rdr = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, a.config.SupabaseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", a.config.AnonKey)
	req.Header.Set("Authorization", "Bearer "+a.config.AnonKey)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("gotrue %s: status %d: %s", path, res.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.RawMessage(data), nil
}

func extractAccessToken(raw json.RawMessage) (string, error) {
	var envelope struct {
		AccessToken string `json:"access_token"`
		Session     *struct {
			AccessToken string `json:"access_token"`
		} `json:"session"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", err
	}
	if envelope.AccessToken != "" {
		return envelope.AccessToken, nil
	}
	if envelope.Session != nil && envelope.Session.AccessToken != "" {
		return envelope.Session.AccessToken, nil
	}
	return "", fmt.Errorf("missing access_token")
}

func newPKCE() (verifier, challenge string, err error) {
	var b [32]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// SafeReturnTo allows only relative in-site paths.
func SafeReturnTo(returnTo string) string {
	returnTo = strings.TrimSpace(returnTo)
	if returnTo == "" || !strings.HasPrefix(returnTo, "/") || strings.HasPrefix(returnTo, "//") {
		return "/"
	}
	if strings.ContainsAny(returnTo, "\r\n") {
		return "/"
	}
	return returnTo
}

// EncodePKCEPayload stores verifier + returnTo for the oauth cookie.
func EncodePKCEPayload(verifier, returnTo string) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"v": verifier,
		"r": SafeReturnTo(returnTo),
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodePKCEPayload parses the oauth cookie.
func DecodePKCEPayload(raw string) (verifier, returnTo string, err error) {
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return "", "", ErrInvalidState
	}
	var payload struct {
		V string `json:"v"`
		R string `json:"r"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", "", ErrInvalidState
	}
	if strings.TrimSpace(payload.V) == "" {
		return "", "", ErrInvalidState
	}
	return payload.V, SafeReturnTo(payload.R), nil
}

// PKCECookieMaxAgeSeconds is the oauth pending cookie lifetime.
func PKCECookieMaxAgeSeconds() int { return pkceCookieMaxAge }
