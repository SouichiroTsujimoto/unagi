package web_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
	"github.com/SouichiroTsujimoto/unagi/internal/feature/contentsync"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/linkcard"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/SouichiroTsujimoto/unagi/internal/web"
	"github.com/SouichiroTsujimoto/unagi/internal/web/about"
	"github.com/SouichiroTsujimoto/unagi/internal/web/admin"
	webarticle "github.com/SouichiroTsujimoto/unagi/internal/web/article"
	webauth "github.com/SouichiroTsujimoto/unagi/internal/web/auth"
	webcontentsync "github.com/SouichiroTsujimoto/unagi/internal/web/contentsync"
	webengagement "github.com/SouichiroTsujimoto/unagi/internal/web/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/web/feed"
	"github.com/SouichiroTsujimoto/unagi/internal/web/home"
	"github.com/SouichiroTsujimoto/unagi/internal/web/islands"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	weblinkcard "github.com/SouichiroTsujimoto/unagi/internal/web/linkcard"
	"github.com/SouichiroTsujimoto/unagi/internal/web/sitemap"
	"github.com/SouichiroTsujimoto/unagi/static"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

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
	library := media.New(database, store)
	contentSync, err := contentsync.New(database, articles, library, contentsync.Config{
		Secret:     "router-sync-secret",
		Repository: "SouichiroTsujimoto/unagi-content",
	})
	if err != nil {
		t.Fatal(err)
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := featureauth.New(featureauth.Config{
		SupabaseURL:    "http://127.0.0.1:54321",
		PublishableKey: "sb_publishable_test",
		AdminUserIDs:   []string{"11111111-1111-1111-1111-111111111111"},
		AllowedOrigins: []string{"http://localhost:8080"},
		SiteBaseURL:    "http://localhost:8080",
		SessionTTL:     time.Hour,
		Keyfunc: func(tok *jwt.Token) (any, error) {
			return &priv.PublicKey, nil
		},
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
		Admin:       admin.New(auth, articles, eng, site, log),
		ContentSync: webcontentsync.New(contentSync, log),
		Engagement:  webengagement.New(eng, auth, site, log),
		LinkCard:   weblinkcard.New(cards, log),
		Auth:       webauth.New(auth, site, log),
	}, static.FS(), islands.FS())
	return &echoRouter{handler: router, jwtKey: priv}, articles, eng, auth
}

type echoRouter struct {
	handler http.Handler
	jwtKey  *ecdsa.PrivateKey
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
		{name: "admin login", path: "/admin/login", status: 200, contains: []string{"Xでログイン", "/auth/x/login?return_to=/admin"}},
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

	raw := signReaderJWT(t, router.jwtKey, "22222222-2222-2222-2222-222222222222", "wuhu", "wuhu", "https://example.com/a.png")
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

func TestAdminAuthBoundaries(t *testing.T) {
	router, _, _, _ := newTestRouter(t)
	reader := featureauth.CookieName + "=" + signReaderJWT(t, router.jwtKey, "22222222-2222-2222-2222-222222222222", "reader", "reader", "")
	admin := featureauth.CookieName + "=" + signReaderJWT(t, router.jwtKey, "11111111-1111-1111-1111-111111111111", "souic", "souic", "")

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Cookie", reader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "管理者として許可されていません") {
		t.Fatalf("reader /admin status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	req.Header.Set("Cookie", reader)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "管理者として許可されていません") {
		t.Fatalf("reader /admin/login status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin" {
		t.Fatalf("admin /admin/login status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "記事管理") {
		t.Fatalf("admin /admin status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/login/begin", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("passkey begin status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/login/finish", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("passkey finish status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/media/sign", strings.NewReader(`{"filename":"dot.png","contentType":"image/png","sizeBytes":12}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Cookie", reader)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reader media sign status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminEditorRoutesGoneAndPublishRemains(t *testing.T) {
	router, articles, _, _ := newTestRouter(t)
	admin := featureauth.CookieName + "=" + signReaderJWT(t, router.jwtKey, "11111111-1111-1111-1111-111111111111", "souic", "souic", "")

	gone := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/articles/new"},
		{http.MethodPost, "/api/admin/articles"},
		{http.MethodPut, "/api/admin/articles/1"},
		{http.MethodPost, "/api/admin/preview"},
		{http.MethodPost, "/api/admin/media/sign"},
		{http.MethodPost, "/api/admin/media/complete"},
	}
	for _, tt := range gone {
		req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Origin", "http://localhost:8080")
		req.Header.Set("Cookie", admin)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d body=%s", tt.method, tt.path, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/articles/1", nil)
	req.Header.Set("Cookie", admin)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "コメント") || strings.Contains(rec.Body.String(), "article-editor") {
		t.Fatalf("manage page status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/articles/1/unpublish", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unpublish status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/admin/articles/1/publish", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/articles/1/comments", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("comments status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/articles/1/stickers", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", admin)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stickers status=%d body=%s", rec.Code, rec.Body.String())
	}

	item, err := articles.GetByID(context.Background(), 1)
	if err != nil || !item.Published {
		t.Fatalf("article after toggle: %+v err=%v", item, err)
	}
}

func TestContentSyncRequiresHMAC(t *testing.T) {
	router, _, _, _ := newTestRouter(t)
	body := []byte(`{"repository":"SouichiroTsujimoto/unagi-content","commit_sha":"aaaaaaa","run_id":"router-1","articles":[],"images":[]}`)

	req := httptest.NewRequest(http.MethodPost, "/api/content-sync/dry-run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned status=%d body=%s", rec.Code, rec.Body.String())
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := contentsync.Sign("router-sync-secret", http.MethodPost, "/api/content-sync/dry-run", ts, "router-1", "SouichiroTsujimoto/unagi-content", body)
	req = httptest.NewRequest(http.MethodPost, "/api/content-sync/dry-run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(contentsync.HeaderTimestamp, ts)
	req.Header.Set(contentsync.HeaderRunID, "router-1")
	req.Header.Set(contentsync.HeaderRepository, "SouichiroTsujimoto/unagi-content")
	req.Header.Set(contentsync.HeaderSignature, "sha256="+sig)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed dry-run status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestContentSyncImagesAndApply(t *testing.T) {
	router, _, _, _ := newTestRouter(t)
	body := []byte(`{"repository":"SouichiroTsujimoto/unagi-content","commit_sha":"bbbbbbb","run_id":"router-apply","articles":[],"images":[]}`)

	req := signedSyncRequest(t, "/api/content-sync/images", "router-apply", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"uploads":[]`) {
		t.Fatalf("images status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = signedSyncRequest(t, "/api/content-sync/sync", "router-apply", body)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"article_count":0`) {
		t.Fatalf("sync status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = signedSyncRequest(t, "/api/content-sync/sync", "router-apply", body)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate sync status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func signedSyncRequest(t *testing.T, path, runID string, body []byte) *http.Request {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	repository := "SouichiroTsujimoto/unagi-content"
	sig := contentsync.Sign("router-sync-secret", http.MethodPost, path, ts, runID, repository, body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(contentsync.HeaderTimestamp, ts)
	req.Header.Set(contentsync.HeaderRunID, runID)
	req.Header.Set(contentsync.HeaderRepository, repository)
	req.Header.Set(contentsync.HeaderSignature, "sha256="+sig)
	return req
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
		!strings.Contains(loc, "provider=x") ||
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

func TestXAuthCallbackSuccess(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	token := signReaderJWT(t, priv, "22222222-2222-2222-2222-222222222222", "reader", "Reader", "")
	gotrue := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/token" || r.URL.Query().Get("grant_type") != "pkce" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": token})
	}))
	t.Cleanup(gotrue.Close)

	auth, err := featureauth.New(featureauth.Config{
		SupabaseURL:    gotrue.URL,
		PublishableKey: "sb_publishable_test",
		SiteBaseURL:    "http://localhost:8080",
		SessionTTL:     time.Hour,
		HTTPClient:     gotrue.Client(),
		Keyfunc: func(tok *jwt.Token) (any, error) {
			return &priv.PublicKey, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := webauth.New(auth, layout.Site{BaseURL: "http://localhost:8080"}, log)
	router := http.NewServeMux()
	router.HandleFunc("GET /auth/x/callback", func(w http.ResponseWriter, r *http.Request) {
		e := echo.New()
		e.GET("/auth/x/callback", handler.Callback)
		e.ServeHTTP(w, r)
	})

	payload, err := featureauth.EncodePKCEPayload("verifier", "/articles/hello-unagi")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/x/callback?code=oauth-code", nil)
	req.AddCookie(&http.Cookie{Name: featureauth.PKCECookieName, Value: payload})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/articles/hello-unagi" {
		t.Fatalf("callback status=%d location=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	var session *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == featureauth.CookieName {
			session = cookie
			break
		}
	}
	if session == nil || session.Value == "" || !session.HttpOnly {
		t.Fatalf("session cookies=%v", rec.Result().Cookies())
	}
}

func signReaderJWT(t *testing.T, key *ecdsa.PrivateKey, sub, username, display, avatar string) string {
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
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
