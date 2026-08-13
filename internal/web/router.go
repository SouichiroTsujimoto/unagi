package web

import (
	"io/fs"
	"net/http"

	"github.com/SouichiroTsujimoto/unagi/internal/web/about"
	"github.com/SouichiroTsujimoto/unagi/internal/web/admin"
	webarticle "github.com/SouichiroTsujimoto/unagi/internal/web/article"
	webauth "github.com/SouichiroTsujimoto/unagi/internal/web/auth"
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
	Auth       *webauth.Handler
}

func New(h Handlers, staticFiles, islandFiles fs.FS) *echo.Echo {
	router := echo.New()
	router.HideBanner = true
	router.HidePort = true
	router.Use(requestLog())

	router.GET("/static/islands/*", echo.StaticDirectoryHandler(islandFiles, false))
	router.GET("/static/*", echo.StaticDirectoryHandler(staticFiles, false))
	router.GET("/healthz", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	router.GET("/", h.Home.Show)
	router.GET("/articles/:slug", h.Article.Show)
	router.GET("/tags/:tag", h.Article.ListByTag)
	router.GET("/about", h.About.Show)
	router.GET("/feed.xml", h.Feed.Show)
	router.GET("/sitemap.xml", h.Sitemap.Show)

	router.GET("/auth/x/login", h.Auth.Login)
	router.GET("/auth/x/callback", h.Auth.Callback)
	router.POST("/auth/x/logout", h.Auth.Logout)
	router.GET("/auth/x/logout", h.Auth.Logout)

	router.GET("/api/articles/:slug/engagement", h.Engagement.Get)
	router.POST("/api/articles/:slug/stickers", h.Engagement.AddEmojiSticker)
	router.POST("/api/articles/:slug/avatar-stickers", h.Engagement.AddAvatarSticker)
	router.POST("/api/articles/:slug/comments", h.Engagement.AddComment)
	router.DELETE("/api/articles/:slug/comments/:id", h.Engagement.DeleteOwnComment)
	router.POST("/api/linkcards", h.LinkCard.Resolve)

	router.GET("/admin/login", h.Admin.LoginPage)

	adminPages := router.Group("/admin", h.Admin.RequireAuth, h.Admin.RequireOrigin)
	adminPages.GET("", h.Admin.Index)
	adminPages.GET("/articles/new", h.Admin.NewArticlePage)
	adminPages.GET("/articles/:id", h.Admin.EditArticlePage)
	adminPages.POST("/logout", h.Admin.Logout)

	adminAPI := router.Group("/api/admin", h.Admin.RequireAuth, h.Admin.RequireOrigin)
	adminAPI.POST("/articles", h.Admin.CreateArticle)
	adminAPI.PUT("/articles/:id", h.Admin.SaveArticle)
	adminAPI.POST("/articles/:id/publish", h.Admin.PublishArticle)
	adminAPI.POST("/articles/:id/unpublish", h.Admin.UnpublishArticle)
	adminAPI.GET("/articles/:id/stickers", h.Admin.ListStickers)
	adminAPI.DELETE("/articles/:id/stickers", h.Admin.DeleteStickers)
	adminAPI.GET("/articles/:id/comments", h.Admin.ListComments)
	adminAPI.PATCH("/articles/:id/comments/:commentID", h.Admin.UpdateComment)
	adminAPI.DELETE("/articles/:id/comments", h.Admin.DeleteComments)
	adminAPI.POST("/preview", h.Admin.Preview)
	adminAPI.POST("/media/sign", h.Media.SignUpload)
	adminAPI.POST("/media/complete", h.Media.CompleteUpload)

	return router
}
