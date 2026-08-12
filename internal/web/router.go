package web

import (
	"io/fs"

	"github.com/SouichiroTsujimoto/unagi/internal/web/about"
	"github.com/SouichiroTsujimoto/unagi/internal/web/admin"
	webarticle "github.com/SouichiroTsujimoto/unagi/internal/web/article"
	"github.com/SouichiroTsujimoto/unagi/internal/web/engagement"
	"github.com/SouichiroTsujimoto/unagi/internal/web/feed"
	"github.com/SouichiroTsujimoto/unagi/internal/web/home"
	"github.com/SouichiroTsujimoto/unagi/internal/web/linkcard"
	webmedia "github.com/SouichiroTsujimoto/unagi/internal/web/media"
	"github.com/SouichiroTsujimoto/unagi/internal/web/sitemap"
	"github.com/labstack/echo/v4"
)

type Handlers struct {
	Home       *home.Handler
	Article    *webarticle.Handler
	About      *about.Handler
	Feed       *feed.Handler
	Sitemap    *sitemap.Handler
	Admin      *admin.Handler
	Media      *webmedia.Handler
	Engagement *engagement.Handler
	LinkCard   *linkcard.Handler
}

func New(h Handlers, staticFiles, islandFiles fs.FS) *echo.Echo {
	router := echo.New()
	router.HideBanner = true
	router.HidePort = true
	router.Use(requestLog())

	router.GET("/static/islands/*", echo.StaticDirectoryHandler(islandFiles, false))
	router.GET("/static/*", echo.StaticDirectoryHandler(staticFiles, false))
	router.GET("/images/*", h.Media.Show)

	router.GET("/", h.Home.Show)
	router.GET("/articles/:slug", h.Article.Show)
	router.GET("/tags/:tag", h.Article.ListByTag)
	router.GET("/about", h.About.Show)
	router.GET("/feed.xml", h.Feed.Show)
	router.GET("/sitemap.xml", h.Sitemap.Show)

	router.GET("/api/articles/:slug/engagement", h.Engagement.Get)
	router.POST("/api/articles/:slug/stickers", h.Engagement.AddEmojiSticker)
	router.POST("/api/articles/:slug/avatar-stickers", h.Engagement.AddAvatarSticker)
	router.POST("/api/articles/:slug/comments", h.Engagement.AddComment)
	router.POST("/api/linkcards", h.LinkCard.Resolve)

	router.GET("/admin/login", h.Admin.LoginPage)
	router.GET("/admin/setup", h.Admin.SetupPage)
	router.POST("/api/admin/setup/begin", h.Admin.BeginSetup)
	router.POST("/api/admin/setup/finish", h.Admin.FinishSetup)
	router.POST("/api/admin/login/begin", h.Admin.BeginLogin)
	router.POST("/api/admin/login/finish", h.Admin.FinishLoginAPI)
	router.POST("/api/admin/recover", h.Admin.Recover)

	adminPages := router.Group("/admin", h.Admin.RequireAuth, h.Admin.RequireCSRF)
	adminPages.GET("", h.Admin.Index)
	adminPages.GET("/articles/new", h.Admin.NewArticlePage)
	adminPages.GET("/articles/:id", h.Admin.EditArticlePage)
	adminPages.GET("/passkeys", h.Admin.PasskeysPage)
	adminPages.POST("/logout", h.Admin.Logout)

	adminAPI := router.Group("/api/admin", h.Admin.RequireAuth, h.Admin.RequireCSRF)
	adminAPI.POST("/articles", h.Admin.CreateArticle)
	adminAPI.PUT("/articles/:id", h.Admin.SaveArticle)
	adminAPI.POST("/articles/:id/publish", h.Admin.PublishArticle)
	adminAPI.POST("/articles/:id/unpublish", h.Admin.UnpublishArticle)
	adminAPI.GET("/articles/:id/stickers", h.Admin.ListStickers)
	adminAPI.DELETE("/articles/:id/stickers", h.Admin.DeleteStickers)
	adminAPI.POST("/preview", h.Admin.Preview)
	adminAPI.POST("/media", h.Media.Upload)
	adminAPI.POST("/passkeys/begin", h.Admin.BeginRegisterPasskey)
	adminAPI.POST("/passkeys/finish", h.Admin.FinishRegisterPasskey)
	adminAPI.DELETE("/passkeys/:id", h.Admin.DeletePasskey)

	return router
}
