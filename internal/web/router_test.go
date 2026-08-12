package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/linkcard"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/SouichiroTsujimoto/unagi/internal/web"
	"github.com/SouichiroTsujimoto/unagi/internal/web/about"
	"github.com/SouichiroTsujimoto/unagi/internal/web/admin"
	webarticle "github.com/SouichiroTsujimoto/unagi/internal/web/article"
	webauth "github.com/SouichiroTsujimoto/unagi/internal/web/auth"
	webengagement "github.com/SouichiroTsujimoto/unagi/internal/web/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/web/feed"
	"github.com/SouichiroTsujimoto/unagi/internal/web/home"
	"github.com/SouichiroTsujimoto/unagi/internal/web/islands"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	weblinkcard "github.com/SouichiroTsujimoto/unagi/internal/web/linkcard"
	webmedia "github.com/SouichiroTsujimoto/unagi/internal/web/media"
	"github.com/SouichiroTsujimoto/unagi/internal/web/sitemap"
	"github.com/SouichiroTsujimoto/unagi/static"
	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "super-secret-jwt-token-with-at-least-32-characters-long"

func newTestRouter(t *testing.T) (*echoRouter, *article.Articles, *engagement.Engagement, *featureauth.Auth) {
	t.Helper()
	database := db.OpenTest(t)

	articles := article.New(database)
	cards := linkcard.New(database)
	articles.SetEmbeds(cards)
	eng := engagement.New(database, articles)
	ctx := context.Background()
	created, err := articles.Create(ctx, article.SaveInput{
		Slug:        "hello-unagi",
		Title:       "unagiへようこそ",
		Emoji:       "🍣",
		Type:        "tech",
		Topics:      []string{"Go", "ブログ"},
		BodyMD:      "Hello **unagi**.\n",
		PublishedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := articles.Publish(ctx, created.ID, created.RevisionID, created.PublishedAt); err != nil {
		t.Fatal(err)
	}

	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "media"))
	if err != nil {
		t.Fatal(err)
	}
	library := media.New(database, store, "https://cdn.example/images")
	auth, err := featureauth.New(featureauth.Config{
		SupabaseURL:    "http://127.0.0.1:54321",
		AnonKey:        "test-anon",
		JWTSecret:      testJWTSecret,
		AdminUserIDs:   []string{"11111111-1111-1111-1111-111111111111"},
		AllowedOrigins: []string{"http://localhost:8080"},
		SiteBaseURL:    "http://localhost:8080",
		SessionTTL:     time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	site := layout.Site{
		Name:        "unagi",
		Description: "個人用のミニマルな技術ブログ",
		BaseURL:     "http://localhost:8080",
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := web.New(web.Handlers{
		Home:       home.New(articles, site, log),
		Article:    webarticle.New(articles, site, log),
		About:      about.New(site, log),
		Feed:       feed.New(articles, site, log),
		Sitemap:    sitemap.New(articles, site, log),
		Admin:      admin.New(auth, articles, eng, site, log),
		Media:      webmedia.New(library, log),
		Engagement: webengagement.New(eng, auth, site, log),
		LinkCard:   weblinkcard.New(cards, log),
		Auth:       webauth.New(auth, site, log),
	}, static.FS(), islands.FS())
	return &echoRouter{handler: router}, articles, eng, auth
}

type echoRouter struct {
	handler http.Handler
}

func (r *echoRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}

func TestBlogRoutes(t *testing.T) {
	router, _, _, _ := newTestRouter(t)

	tests := []struct {
		name     string
		path     string
		status   int
		contains []string
	}{
		{name: "home", path: "/", status: 200, contains: []string{"<title>Posts · unagi</title>", "Hello from", "wuhu1sland", "unagiへようこそ", `aria-label="unagi トップへ"`}},
		{name: "article", path: "/articles/hello-unagi", status: 200, contains: []string{"Hello", "<strong>unagi</strong>", "article-engagement", `slug="hello-unagi"`, "article-linkcards"}},
		{name: "missing", path: "/articles/missing", status: 404},
		{name: "tag", path: "/tags/Go", status: 200, contains: []string{"unagiへようこそ"}},
		{name: "unknown tag", path: "/tags/unknown", status: 404},
		{name: "about", path: "/about", status: 200, contains: []string{"<title>Me · unagi</title>", "me", "unagi", "学部3回生", "https://x.com/wuhu1sland", "https://github.com/SouichiroTsujimoto", `aria-label="unagi トップへ"`}},
		{name: "feed", path: "/feed.xml", status: 200, contains: []string{"<rss", "hello-unagi"}},
		{name: "sitemap", path: "/sitemap.xml", status: 200, contains: []string{"<urlset", "/articles/hello-unagi"}},
		{name: "admin login", path: "/admin/login", status: 200, contains: []string{"passkey"}},
		{name: "admin blocked", path: "/admin", status: 303},
		{name: "accounts gone", path: "/api/accounts", status: 404},
		{name: "healthz", path: "/healthz", status: 200},
		{name: "images gone", path: "/images/foo.png", status: 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status=%d want %d body=%s", rec.Code, tt.status, rec.Body.String())
			}
			body := rec.Body.String()
			for _, want := range tt.contains {
				if !strings.Contains(body, want) {
					t.Fatalf("missing %q in %s", want, body)
				}
			}
		})
	}
}

func TestEngagementRoutes(t *testing.T) {
	router, _, _, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/articles/hello-unagi/engagement", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", rec.Code, rec.Body.String())
	}
	var snap engagement.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.AllowedEmoji) == 0 || snap.LoginPath != engagement.LoginPath || snap.Authenticated {
		t.Fatalf("snap=%+v", snap)
	}

	payload := []byte(`{"emoji":"🍣","x":0.4,"y":0.6}`)
	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/stickers", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post sticker status=%d body=%s", rec.Code, rec.Body.String())
	}

	bad := []byte(`{"emoji":"nope","x":0.1,"y":0.1}`)
	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/stickers", bytes.NewReader(bad))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad emoji status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/avatar-stickers", bytes.NewReader([]byte(`{"x":0.2,"y":0.3}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("avatar status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"login_required"`) {
		t.Fatalf("avatar body=%s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/comments", bytes.NewReader([]byte(`{"body":"hi"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("comment status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/stickers", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("evil origin status=%d body=%s", rec.Code, rec.Body.String())
	}

	loopback := []byte(`{"emoji":"🎉","x":0.2,"y":0.8}`)
	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/stickers", bytes.NewReader(loopback))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("loopback origin status=%d body=%s", rec.Code, rec.Body.String())
	}

	raw := signReaderJWT(t, "22222222-2222-2222-2222-222222222222", "wuhu", "wuhu", "https://example.com/a.png")
	cookie := featureauth.CookieName + "=" + raw

	req = httptest.NewRequest(http.MethodGet, "/api/articles/hello-unagi/engagement", nil)
	req.Header.Set("Cookie", cookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed get status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if !snap.Authenticated || snap.Viewer == nil || snap.Viewer.Username != "wuhu" {
		t.Fatalf("authed snap=%+v", snap)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/avatar-stickers", bytes.NewReader([]byte(`{"x":0.55,"y":0.45}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Cookie", cookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("avatar create status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/articles/hello-unagi/comments", bytes.NewReader([]byte(`{"body":"hello from x"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Cookie", cookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("comment create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created engagement.Comment
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !created.Mine || created.ID == 0 {
		t.Fatalf("created=%+v", created)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/articles/hello-unagi/comments/"+itoa(created.ID), nil)
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Cookie", cookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("comment delete status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/articles/hello-unagi/comments/"+itoa(created.ID), nil)
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Cookie", cookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("comment delete again status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func TestXAuthRoutes(t *testing.T) {
	router, _, _, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/x/login?return_to=/articles/hello-unagi", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("login status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "http://127.0.0.1:54321/auth/v1/authorize") ||
		!strings.Contains(loc, "provider=twitter") ||
		!strings.Contains(loc, "code_challenge=") {
		t.Fatalf("location=%q", loc)
	}
	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, featureauth.PKCECookieName+"=") || !strings.Contains(setCookie, "HttpOnly") {
		t.Fatalf("oauth cookie=%q", setCookie)
	}

	req = httptest.NewRequest(http.MethodGet, "/auth/x/callback", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("callback without state: status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/auth/x/logout?return_to=/articles/hello-unagi", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/articles/hello-unagi" {
		t.Fatalf("logout status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func signReaderJWT(t *testing.T, sub, username, display, avatar string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  sub,
		"role": "authenticated",
		"aud":  "authenticated",
		"user_metadata": map[string]any{
			"user_name":  username,
			"full_name":  display,
			"avatar_url": avatar,
		},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
