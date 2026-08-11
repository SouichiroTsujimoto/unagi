package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/adminauth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/SouichiroTsujimoto/unagi/internal/web"
	"github.com/SouichiroTsujimoto/unagi/internal/web/about"
	"github.com/SouichiroTsujimoto/unagi/internal/web/admin"
	webarticle "github.com/SouichiroTsujimoto/unagi/internal/web/article"
	"github.com/SouichiroTsujimoto/unagi/internal/web/feed"
	"github.com/SouichiroTsujimoto/unagi/internal/web/home"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	webmedia "github.com/SouichiroTsujimoto/unagi/internal/web/media"
	"github.com/SouichiroTsujimoto/unagi/internal/web/sitemap"
	"github.com/SouichiroTsujimoto/unagi/static"
	"github.com/SouichiroTsujimoto/unagi/internal/web/islands"
)

func TestBlogRoutes(t *testing.T) {
	database, err := db.Open(db.Config{
		Driver: db.DriverSQLite,
		DSN:    filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	articles := article.New(database)
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
	auth, err := adminauth.New(database, adminauth.Config{
		RPDisplayName: "unagi",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:8080"},
		SessionTTL:    time.Hour,
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
		Home:    home.New(articles, site, log),
		Article: webarticle.New(articles, site, log),
		About:   about.New(site, log),
		Feed:    feed.New(articles, site, log),
		Sitemap: sitemap.New(articles, site, log),
		Admin:   admin.New(auth, articles, site, log),
		Media:   webmedia.New(library, log),
	}, static.FS(), islands.FS())

	tests := []struct {
		name     string
		path     string
		status   int
		contains []string
	}{
		{name: "home", path: "/", status: 200, contains: []string{"unagiへようこそ", "記事"}},
		{name: "article", path: "/articles/hello-unagi", status: 200, contains: []string{"Hello", "<strong>unagi</strong>"}},
		{name: "missing", path: "/articles/missing", status: 404},
		{name: "tag", path: "/tags/Go", status: 200, contains: []string{"unagiへようこそ"}},
		{name: "unknown tag", path: "/tags/unknown", status: 404},
		{name: "about", path: "/about", status: 200, contains: []string{"About"}},
		{name: "feed", path: "/feed.xml", status: 200, contains: []string{"<rss", "hello-unagi"}},
		{name: "sitemap", path: "/sitemap.xml", status: 200, contains: []string{"<urlset", "/articles/hello-unagi"}},
		{name: "admin login", path: "/admin/login", status: 200, contains: []string{"Passkey"}},
		{name: "admin blocked", path: "/admin", status: 303},
		{name: "accounts gone", path: "/api/accounts", status: 404},
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
