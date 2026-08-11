package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	sitecontent "github.com/SouichiroTsujimoto/unagi"
	"cloud.google.com/go/storage"
	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/adminauth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/SouichiroTsujimoto/unagi/internal/httpserver"
	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
	"github.com/SouichiroTsujimoto/unagi/internal/web"
	"github.com/SouichiroTsujimoto/unagi/internal/web/about"
	"github.com/SouichiroTsujimoto/unagi/internal/web/admin"
	webarticle "github.com/SouichiroTsujimoto/unagi/internal/web/article"
	"github.com/SouichiroTsujimoto/unagi/internal/web/feed"
	"github.com/SouichiroTsujimoto/unagi/internal/web/home"
	"github.com/SouichiroTsujimoto/unagi/internal/web/islands"
	"github.com/SouichiroTsujimoto/unagi/internal/web/layout"
	webmedia "github.com/SouichiroTsujimoto/unagi/internal/web/media"
	"github.com/SouichiroTsujimoto/unagi/internal/web/sitemap"
	"github.com/SouichiroTsujimoto/unagi/static"
)

type Config struct {
	Address          string
	DB               db.Config
	Domains          []string
	ACMEEmail        string
	CertMagicStorage string
	Version          string
	Banner           string
	Site             layout.Site
	Auth             adminauth.Config
	MediaBackend     string // local | gcs
	MediaLocalDir    string
	GCSBucket        string
	GCSPrefix        string
}

func Run(config Config) error {
	log := terminal.NewLogger()
	slog.SetDefault(log)

	database, err := db.Open(config.DB)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	articles := article.New(database)
	articlesFS, err := sitecontent.Articles()
	if err != nil {
		return fmt.Errorf("articles fs: %w", err)
	}
	if n, err := articles.SeedFromFS(context.Background(), articlesFS); err != nil {
		return fmt.Errorf("seed articles: %w", err)
	} else if n > 0 {
		log.Info("seeded articles from embed", "count", n)
	}

	objectStore, closer, err := openObjectStore(config)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer()
	}
	mediaLib := media.New(database, objectStore)

	auth, err := adminauth.New(database, config.Auth)
	if err != nil {
		return fmt.Errorf("admin auth: %w", err)
	}

	site := config.Site
	if site.Name == "" {
		site.Name = "unagi"
	}
	if site.Description == "" {
		site.Description = "個人用のミニマルな技術ブログ"
	}

	handler := web.New(web.Handlers{
		Home:    home.New(articles, site, log),
		Article: webarticle.New(articles, site, log),
		About:   about.New(site, log),
		Feed:    feed.New(articles, site, log),
		Sitemap: sitemap.New(articles, site, log),
		Admin:   admin.New(auth, articles, site, log),
		Media:   webmedia.New(mediaLib, log),
	}, static.FS(), islands.FS())

	return httpserver.Run(handler, httpserver.Config{
		Address:          config.Address,
		Domains:          config.Domains,
		ACMEEmail:        config.ACMEEmail,
		CertMagicStorage: config.CertMagicStorage,
		DBPath:           config.DB.WithDefaults().Label(),
		Version:          config.Version,
		Banner:           config.Banner,
	}, log)
}

func openObjectStore(config Config) (media.ObjectStore, func(), error) {
	backend := strings.ToLower(strings.TrimSpace(config.MediaBackend))
	if backend == "" {
		backend = "local"
	}
	switch backend {
	case "local":
		dir := config.MediaLocalDir
		if dir == "" {
			dir = "data/media"
		}
		store, err := media.NewLocalStore(dir)
		return store, nil, err
	case "gcs":
		if strings.TrimSpace(config.GCSBucket) == "" {
			return nil, nil, fmt.Errorf("gcs bucket is required")
		}
		client, err := storage.NewClient(context.Background())
		if err != nil {
			return nil, nil, fmt.Errorf("gcs client: %w", err)
		}
		return media.NewGCSStore(client, config.GCSBucket, config.GCSPrefix), func() { _ = client.Close() }, nil
	default:
		return nil, nil, fmt.Errorf("unsupported media backend %q", backend)
	}
}

// Ensure data dir exists for local defaults.
func EnsureDataDirs(paths ...string) error {
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
	}
	return nil
}
