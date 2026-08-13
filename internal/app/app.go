package app

import (
	"fmt"
	"log/slog"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	featureauth "github.com/SouichiroTsujimoto/unagi/internal/feature/auth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/contentsync"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/linkcard"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/ogimage"
	"github.com/SouichiroTsujimoto/unagi/internal/httpserver"
	"github.com/SouichiroTsujimoto/unagi/internal/terminal"
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
	weblinkcard "github.com/SouichiroTsujimoto/unagi/internal/web/linkcard"
	webogimage "github.com/SouichiroTsujimoto/unagi/internal/web/ogimage"
	"github.com/SouichiroTsujimoto/unagi/internal/web/sitemap"
	"github.com/SouichiroTsujimoto/unagi/static"
)

func Run(config Config) error {
	config = config.withDefaults()
	log := terminal.NewLogger()
	slog.SetDefault(log)

	database, err := db.Open(config.DB)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	cards := linkcard.New(database)
	articles := article.New(
		database,
		article.WithMediaPublicBase(config.MediaPublicBase),
		article.WithEmbeds(cards),
	)
	eng := engagement.New(database, articles)
	ogImages, err := ogimage.New()
	if err != nil {
		return fmt.Errorf("OGP images: %w", err)
	}

	objectStore, err := openObjectStore(config)
	if err != nil {
		return err
	}
	mediaLib := media.New(database, objectStore)
	contentSync, err := contentsync.New(database, articles, mediaLib, contentsync.Config{
		Secret: config.ContentSyncSecret,
	})
	if err != nil {
		return fmt.Errorf("content sync: %w", err)
	}

	auth, err := featureauth.New(config.Auth)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	site := config.Site
	if config.DevAdminBypass {
		log.Warn("admin authentication bypass enabled for local development")
	}

	handler := web.New(web.Handlers{
		Home:        home.New(articles, site, log),
		Article:     webarticle.New(articles, site, log),
		About:       about.New(site, log),
		Feed:        feed.New(articles, site, log),
		Sitemap:     sitemap.New(articles, site, log),
		Admin:       admin.New(auth, articles, eng, ogImages, site, log, config.DevAdminBypass),
		ContentSync: webcontentsync.New(contentSync, log),
		Engagement:  webengagement.New(eng, auth, log),
		LinkCard:    weblinkcard.New(cards, log),
		OGImage:     webogimage.New(ogImages, articles, log),
		Auth:        webauth.New(auth, site, log),
	}, static.FS(), islands.FS())

	return httpserver.Run(handler, httpserver.Config{
		Address: config.Address,
		DBPath:  config.DB.WithDefaults().Label(),
		Version: config.Version,
		Banner:  config.Banner,
	}, log)
}

func openObjectStore(config Config) (media.ObjectStore, error) {
	return media.NewSupabaseStore(config.Auth.SupabaseURL, config.SupabaseSecretKey, nil)
}
